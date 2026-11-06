package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-sql/sqlexp"
	"sqlterm/internal/result"
)

var fakeDriverID atomic.Int64

type pagingPlan struct {
	columns    [][]string
	sets       [][][]driver.Value
	finalErr   error
	nextCalls  atomic.Int64
	closeCalls atomic.Int64
}

type pagingDriver struct{ state *pagingState }
type pagingConn struct {
	state          *pagingState
	returnMessages *sqlexp.ReturnMessage
}
type pagingState struct {
	mu        sync.Mutex
	plans     map[string][]*pagingPlan
	queries   []string
	queryArgs [][]driver.NamedValue
	execs     []string
}
type pagingRows struct {
	plan *pagingPlan
	set  int
	row  int
}

type pagingResult int64

func (d *pagingDriver) Open(string) (driver.Conn, error) { return &pagingConn{state: d.state}, nil }
func (c *pagingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (c *pagingConn) Close() error              { return nil }
func (c *pagingConn) Begin() (driver.Tx, error) { return nil, errors.New("transactions not supported") }
func (c *pagingConn) CheckNamedValue(value *driver.NamedValue) error {
	messages, ok := value.Value.(*sqlexp.ReturnMessage)
	if !ok {
		return nil
	}
	sqlexp.ReturnMessageInit(messages)
	c.returnMessages = messages
	return driver.ErrRemoveArgument
}
func (c *pagingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	plans := c.state.plans[query]
	if len(plans) == 0 {
		return nil, fmt.Errorf("unexpected query %q", query)
	}
	plan := plans[0]
	c.state.plans[query] = plans[1:]
	c.state.queries = append(c.state.queries, query)
	c.state.queryArgs = append(c.state.queryArgs, append([]driver.NamedValue(nil), args...))
	if c.returnMessages != nil {
		messages := c.returnMessages
		c.returnMessages = nil
		if err := sqlexp.ReturnMessageEnqueue(ctx, messages, sqlexp.MsgNext{}); err != nil {
			return nil, err
		}
		if err := sqlexp.ReturnMessageEnqueue(ctx, messages, sqlexp.MsgNextResultSet{}); err != nil {
			return nil, err
		}
	}
	return &pagingRows{plan: plan}, nil
}
func (c *pagingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.execs = append(c.state.execs, query)
	return pagingResult(0), nil
}
func (r pagingResult) LastInsertId() (int64, error) { return 0, nil }
func (r pagingResult) RowsAffected() (int64, error) { return int64(r), nil }
func (r *pagingRows) Columns() []string             { return r.plan.columns[r.set] }
func (r *pagingRows) Close() error {
	r.plan.closeCalls.Add(1)
	return nil
}
func (r *pagingRows) Next(dest []driver.Value) error {
	r.plan.nextCalls.Add(1)
	rows := r.plan.sets[r.set]
	if r.row < len(rows) {
		copy(dest, rows[r.row])
		r.row++
		return nil
	}
	if r.set == len(r.plan.sets)-1 && r.plan.finalErr != nil {
		e := r.plan.finalErr
		r.plan.finalErr = nil
		return e
	}
	return io.EOF
}
func (r *pagingRows) HasNextResultSet() bool { return r.set+1 < len(r.plan.sets) }
func (r *pagingRows) NextResultSet() error {
	if !r.HasNextResultSet() {
		return io.EOF
	}
	r.set++
	r.row = 0
	return nil
}

func newPagingSession(t *testing.T, engine string, plans map[string][]*pagingPlan) (*Session, *pagingState) {
	t.Helper()
	state := &pagingState{plans: plans}
	name := fmt.Sprintf("sqlterm-paging-%d", fakeDriverID.Add(1))
	sql.Register(name, &pagingDriver{state: state})
	database, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Conn(context.Background())
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(); database.Close() })
	return &Session{DB: database, Conn: conn, Adapter: NewAdapter(engine)}, state
}

func plan(values ...int) *pagingPlan {
	rows := make([][]driver.Value, len(values))
	for i, value := range values {
		rows[i] = []driver.Value{int64(value)}
	}
	return &pagingPlan{columns: [][]string{{"value"}}, sets: [][][]driver.Value{rows}}
}

func waitEvent(t *testing.T, events <-chan Event, kind string, stores map[*result.Store]struct{}) Event {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if event.Store != nil {
				stores[event.Store] = struct{}{}
			}
			if event.Kind == kind {
				return event
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %q event", kind)
		}
	}
}

func closeEventStores(events <-chan Event, stores map[*result.Store]struct{}) {
	for len(events) > 0 {
		event := <-events
		if event.Store != nil {
			stores[event.Store] = struct{}{}
		}
	}
	for store := range stores {
		store.Close()
	}
}

func TestRunPagingUsesExplicitBlockDemandWithoutOverfetch(t *testing.T) {
	cases := []struct {
		name       string
		block      int
		values     []int
		pauseRows  []int
		pauseNexts []int64
	}{
		{name: "size one exact multiple", block: 1, values: []int{1, 2}, pauseRows: []int{1, 2}, pauseNexts: []int64{1, 2}},
		{name: "size two exact multiple", block: 2, values: []int{1, 2}, pauseRows: []int{2}, pauseNexts: []int64{2}},
		{name: "size two non exact multiple", block: 2, values: []int{1, 2, 3}, pauseRows: []int{2}, pauseNexts: []int64{2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			queryPlan := plan(tc.values...)
			session, _ := newPagingSession(t, "postgres", map[string][]*pagingPlan{"SELECT paged": {queryPlan}})
			pager, err := NewPager(tc.block)
			if err != nil {
				t.Fatal(err)
			}
			if pager.Request(PageToken{}) {
				t.Fatal("accepted demand before a pause")
			}
			events := make(chan Event, 32)
			stores := make(map[*result.Store]struct{})
			var earlyAccepted atomic.Bool
			done := make(chan error, 1)
			go func() {
				done <- session.Run(WithPager(context.Background(), pager), []Bound{{SQL: "SELECT paged"}}, func(event Event) {
					if event.Kind == "columns" {
						earlyAccepted.Store(pager.Request(event.Token))
					}
					events <- event
				})
			}()
			for i, rows := range tc.pauseRows {
				paused := waitEvent(t, events, "paused", stores)
				if paused.Set != 0 || paused.Rows != rows {
					t.Fatalf("pause = set %d rows %d, want set 0 rows %d", paused.Set, paused.Rows, rows)
				}
				if got := queryPlan.nextCalls.Load(); got != tc.pauseNexts[i] {
					t.Fatalf("driver Next calls at pause = %d, want %d", got, tc.pauseNexts[i])
				}
				for store := range stores {
					if got := store.Count(); got != rows {
						t.Fatalf("stored rows at pause = %d, want %d", got, rows)
					}
				}
				if !pager.Request(paused.Token) {
					t.Fatal("rejected current demand")
				}
				if pager.Request(paused.Token) {
					t.Fatal("accepted repeated demand for one pause")
				}
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("paged run did not finish")
			}
			if earlyAccepted.Load() {
				t.Fatal("accepted demand after columns but before a pause")
			}
			if got, want := queryPlan.nextCalls.Load(), int64(len(tc.values)+1); got != want {
				t.Fatalf("final driver Next calls = %d, want %d", got, want)
			}
			if queryPlan.closeCalls.Load() != 1 {
				t.Fatalf("rows closed %d times", queryPlan.closeCalls.Load())
			}
			if pager.Request(PageToken{}) {
				t.Fatal("accepted demand after execution")
			}
			closeEventStores(events, stores)
		})
	}
}

func TestRunPagingRejectsStalePauseDemand(t *testing.T) {
	queryPlan := plan(1, 2)
	session, _ := newPagingSession(t, "postgres", map[string][]*pagingPlan{"SELECT generations": {queryPlan}})
	pager, _ := NewPager(1)
	events := make(chan Event, 16)
	stores := make(map[*result.Store]struct{})
	done := make(chan error, 1)
	go func() {
		done <- session.Run(WithPager(context.Background(), pager), []Bound{{SQL: "SELECT generations"}}, func(event Event) { events <- event })
	}()
	first := waitEvent(t, events, "paused", stores)
	if !pager.Request(first.Token) {
		t.Fatal("could not advance first pause")
	}
	second := waitEvent(t, events, "paused", stores)
	if first.Set != second.Set || first.Token == second.Token {
		t.Fatalf("pause identity was not renewed: first=%#v second=%#v", first.Token, second.Token)
	}
	if pager.Request(first.Token) {
		t.Fatal("delayed first-pause token advanced the second pause")
	}

	var staleAccepted atomic.Int64
	var staleRequests sync.WaitGroup
	staleRequests.Add(32)
	for range 32 {
		go func() {
			defer staleRequests.Done()
			if pager.Request(first.Token) {
				staleAccepted.Add(1)
			}
		}()
	}
	staleRequests.Wait()
	if staleAccepted.Load() != 0 {
		t.Fatalf("accepted %d concurrent stale requests", staleAccepted.Load())
	}
	if got := queryPlan.nextCalls.Load(); got != 2 {
		t.Fatalf("driver Next calls before second demand = %d, want 2", got)
	}

	var currentAccepted atomic.Int64
	var currentRequests sync.WaitGroup
	currentRequests.Add(32)
	for range 32 {
		go func() {
			defer currentRequests.Done()
			if pager.Request(second.Token) {
				currentAccepted.Add(1)
			}
		}()
	}
	currentRequests.Wait()
	if currentAccepted.Load() != 1 {
		t.Fatalf("accepted %d current requests, want 1", currentAccepted.Load())
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := queryPlan.nextCalls.Load(); got != 3 {
		t.Fatalf("final driver Next calls = %d, want 3", got)
	}
	closeEventStores(events, stores)
}

func TestRunPagingCancellationReleasesPausedRows(t *testing.T) {
	queryPlan := plan(1, 2)
	session, _ := newPagingSession(t, "postgres", map[string][]*pagingPlan{"SELECT cancel": {queryPlan}})
	pager, _ := NewPager(1)
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 16)
	stores := make(map[*result.Store]struct{})
	done := make(chan error, 1)
	go func() {
		done <- session.Run(WithPager(ctx, pager), []Bound{{SQL: "SELECT cancel"}}, func(event Event) { events <- event })
	}()
	paused := waitEvent(t, events, "paused", stores)
	start := make(chan struct{})
	var requested atomic.Bool
	var racers sync.WaitGroup
	racers.Add(2)
	go func() { defer racers.Done(); <-start; requested.Store(pager.Request(paused.Token)) }()
	go func() { defer racers.Done(); <-start; cancel() }()
	close(start)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation (request won=%v)", err, requested.Load())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled paused run did not finish")
	}
	racers.Wait()
	if queryPlan.closeCalls.Load() != 1 {
		t.Fatalf("rows closed %d times", queryPlan.closeCalls.Load())
	}
	if pager.Request(paused.Token) {
		t.Fatal("accepted demand after cancellation")
	}
	closeEventStores(events, stores)
}

func TestPagerRejectsInvalidSizeAndReuse(t *testing.T) {
	if pager, err := NewPager(0); err == nil || pager != nil {
		t.Fatalf("NewPager(0) = %#v, %v", pager, err)
	}
	pager, err := NewPager(1)
	if err != nil {
		t.Fatal(err)
	}
	session, _ := newPagingSession(t, "postgres", map[string][]*pagingPlan{})
	if err = session.Run(WithPager(context.Background(), pager), nil, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if err = session.Run(WithPager(context.Background(), pager), nil, func(Event) {}); err == nil {
		t.Fatal("reused single-execution pager")
	}
}

func TestRunPagingPreservesErrorsCloseAndUnpagedBehavior(t *testing.T) {
	t.Run("final row error closes rows", func(t *testing.T) {
		wantErr := errors.New("row failure")
		queryPlan := plan(1)
		queryPlan.finalErr = wantErr
		session, _ := newPagingSession(t, "postgres", map[string][]*pagingPlan{"SELECT broken": {queryPlan}})
		pager, _ := NewPager(2)
		var stores []Event
		err := session.Run(WithPager(context.Background(), pager), []Bound{{SQL: "SELECT broken"}}, func(event Event) { stores = append(stores, event) })
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if queryPlan.nextCalls.Load() != 2 || queryPlan.closeCalls.Load() != 1 {
			t.Fatalf("Next=%d Close=%d", queryPlan.nextCalls.Load(), queryPlan.closeCalls.Load())
		}
		for _, event := range stores {
			if event.Store != nil {
				event.Store.Close()
			}
		}
	})
	t.Run("nil pager remains eager", func(t *testing.T) {
		queryPlan := plan(1, 2, 3)
		session, _ := newPagingSession(t, "postgres", map[string][]*pagingPlan{"SELECT eager": {queryPlan}})
		paused := false
		var stores []Event
		err := session.Run(context.Background(), []Bound{{SQL: "SELECT eager"}}, func(event Event) {
			paused = paused || event.Kind == "paused"
			stores = append(stores, event)
		})
		if err != nil {
			t.Fatal(err)
		}
		if paused || queryPlan.nextCalls.Load() != 4 {
			t.Fatalf("paused=%v Next=%d", paused, queryPlan.nextCalls.Load())
		}
		for _, event := range stores {
			if event.Store != nil {
				event.Store.Close()
			}
		}
	})
}

func TestRunPagingSQLServerUsesTheSharedDemandGate(t *testing.T) {
	queryPlan := plan(1, 2)
	session, state := newPagingSession(t, "sqlserver", map[string][]*pagingPlan{"SELECT server": {queryPlan}})
	pager, _ := NewPager(1)
	events := make(chan Event, 16)
	stores := make(map[*result.Store]struct{})
	done := make(chan error, 1)
	go func() {
		done <- session.Run(WithPager(context.Background(), pager), []Bound{{SQL: "SELECT server", Args: []any{"argument"}}}, func(event Event) { events <- event })
	}()
	for _, wantRows := range []int{1, 2} {
		paused := waitEvent(t, events, "paused", stores)
		if paused.Set != 0 || paused.Rows != wantRows {
			t.Fatalf("pause = set %d rows %d", paused.Set, paused.Rows)
		}
		if got := queryPlan.nextCalls.Load(); got != int64(wantRows) {
			t.Fatalf("driver Next calls at pause = %d, want %d", got, wantRows)
		}
		if !pager.Request(paused.Token) {
			t.Fatal("could not advance SQL Server result")
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if queryPlan.nextCalls.Load() != 3 || queryPlan.closeCalls.Load() != 1 {
		t.Fatalf("Next=%d Close=%d", queryPlan.nextCalls.Load(), queryPlan.closeCalls.Load())
	}
	state.mu.Lock()
	queries := append([]string(nil), state.queries...)
	var args []driver.NamedValue
	if len(state.queryArgs) == 1 {
		args = append(args, state.queryArgs[0]...)
	}
	state.mu.Unlock()
	if fmt.Sprint(queries) != "[SELECT server]" || len(args) != 1 || args[0].Value != "argument" {
		t.Fatalf("driver calls = queries %v args %#v", queries, args)
	}
	closeEventStores(events, stores)
}

func TestRunPagingOrdersResultSetsStatementsAndInvocationCleanup(t *testing.T) {
	first := &pagingPlan{
		columns: [][]string{{"a"}, {"b"}},
		sets: [][][]driver.Value{
			{{int64(1)}, {int64(2)}},
			{{int64(3)}},
		},
	}
	second := plan(4)
	session, state := newPagingSession(t, "postgres", map[string][]*pagingPlan{
		"SELECT first":  {first},
		"SELECT second": {second},
	})
	pager, _ := NewPager(1)
	events := make(chan Event, 32)
	stores := make(map[*result.Store]struct{})
	done := make(chan error, 1)
	go func() {
		done <- session.Run(WithPager(context.Background(), pager), []Bound{{SQL: "SELECT first"}, {SQL: "SELECT second"}}, func(event Event) { events <- event })
	}()
	for _, wantSet := range []int{0, 0, 1, 2} {
		paused := waitEvent(t, events, "paused", stores)
		if paused.Set != wantSet {
			t.Fatalf("paused set = %d, want %d", paused.Set, wantSet)
		}
		state.mu.Lock()
		queries := append([]string(nil), state.queries...)
		state.mu.Unlock()
		if wantSet < 2 && len(queries) != 1 {
			t.Fatalf("later statement started early: %v", queries)
		}
		if !pager.Request(paused.Token) {
			t.Fatalf("could not advance set %d", wantSet)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	state.mu.Lock()
	queries := append([]string(nil), state.queries...)
	state.mu.Unlock()
	if fmt.Sprint(queries) != "[SELECT first SELECT second]" {
		t.Fatalf("query order = %v", queries)
	}
	closeEventStores(events, stores)

	invocationPlan := plan(7)
	invocation, invocationState := newPagingSession(t, "postgres", map[string][]*pagingPlan{"SELECT invocation": {invocationPlan}})
	invocationPager, _ := NewPager(1)
	invocationEvents := make(chan Event, 16)
	invocationStores := make(map[*result.Store]struct{})
	invocationDone := make(chan error, 1)
	go func() {
		inv := Invocation{SQL: "SELECT invocation", Before: []Bound{{SQL: "SET before"}}, After: "SET after"}
		invocationDone <- invocation.RunInvocation(WithPager(context.Background(), invocationPager), inv, func(event Event) { invocationEvents <- event })
	}()
	paused := waitEvent(t, invocationEvents, "paused", invocationStores)
	invocationState.mu.Lock()
	execsBeforeDemand := append([]string(nil), invocationState.execs...)
	invocationState.mu.Unlock()
	if fmt.Sprint(execsBeforeDemand) != "[SET before]" {
		t.Fatalf("invocation execs while paused = %v", execsBeforeDemand)
	}
	if !invocationPager.Request(paused.Token) {
		t.Fatal("could not advance invocation")
	}
	if err := <-invocationDone; err != nil {
		t.Fatal(err)
	}
	invocationState.mu.Lock()
	execs := append([]string(nil), invocationState.execs...)
	invocationState.mu.Unlock()
	if fmt.Sprint(execs) != "[SET before SET after]" {
		t.Fatalf("invocation exec order = %v", execs)
	}
	closeEventStores(invocationEvents, invocationStores)
}

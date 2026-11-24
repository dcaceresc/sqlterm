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

	"github.com/golang-sql/sqlexp"
)

var fakeDriverID atomic.Int64

type pagingPlan struct {
	columns    [][]string
	types      [][]string
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
	plan     *pagingPlan
	set      int
	row      int
	ctx      context.Context
	messages *sqlexp.ReturnMessage
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
	messages := c.returnMessages
	if messages != nil {
		c.returnMessages = nil
		if err := sqlexp.ReturnMessageEnqueue(ctx, messages, sqlexp.MsgNext{}); err != nil {
			return nil, err
		}
		if err := sqlexp.ReturnMessageEnqueue(ctx, messages, sqlexp.MsgNextResultSet{}); err != nil {
			return nil, err
		}
	}
	return &pagingRows{plan: plan, ctx: ctx, messages: messages}, nil
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
func (r *pagingRows) ColumnTypeDatabaseTypeName(index int) string {
	if r.set < len(r.plan.types) && index < len(r.plan.types[r.set]) {
		return r.plan.types[r.set][index]
	}
	return ""
}
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
	if r.messages != nil {
		if err := sqlexp.ReturnMessageEnqueue(r.ctx, r.messages, sqlexp.MsgNext{}); err != nil {
			return err
		}
		return sqlexp.ReturnMessageEnqueue(r.ctx, r.messages, sqlexp.MsgNextResultSet{})
	}
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

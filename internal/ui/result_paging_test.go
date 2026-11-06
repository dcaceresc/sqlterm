package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"sqlterm/internal/result"
)

func TestResultPagingWaitsForExplicitNavigationAndRenewsDemand(t *testing.T) {
	u := runningUI(t, config.Config{})
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	conn, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	rows := sqlmock.NewRows([]string{"id"})
	for i := 1; i <= 2001; i++ {
		rows.AddRow(i)
	}
	mock.ExpectQuery("SELECT id FROM paging_test").WillReturnRows(rows).RowsWillBeClosed()
	session := &db.Session{DB: database, Conn: conn, Profile: config.Profile{Engine: "postgres"}, Adapter: db.NewAdapter("postgres")}
	tab := u.active()
	u.app.QueueUpdateDraw(func() {
		u.run(tab, "SELECT id FROM paging_test", func(ctx context.Context, emit func(db.Event)) error {
			return session.Run(ctx, []db.Bound{{SQL: "SELECT id FROM paging_test"}}, emit)
		})
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && len(tab.Results) == 1 && tab.Results[0].Count() == 1000 && tab.Execution.Phase == "Paused"
	})

	// Draws, status pulses and result refreshes are passive and must not drain.
	for i := 0; i < 3; i++ {
		u.app.QueueUpdateDraw(func() { u.showResults(); u.refreshResultInfo(); u.refreshBars() })
		time.Sleep(20 * time.Millisecond)
	}
	u.app.QueueUpdateDraw(func() {
		if got := tab.Results[0].Count(); got != 1000 {
			t.Errorf("passive refresh fetched rows: %d", got)
		}
		u.app.SetFocus(u.table)
		u.table.Select(1000, 0)
		dispatch(u, tcell.KeyDown, 0)
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && tab.Results[0].Count() == 2000 && tab.Execution.Phase == "Paused"
	})

	// The first navigation only satisfies its exact pause token. A renewed pause
	// needs a new navigation intent. Wheel-down only counts in the results grid.
	time.Sleep(30 * time.Millisecond)
	u.app.QueueUpdateDraw(func() {
		if got := tab.Results[0].Count(); got != 2000 {
			t.Errorf("renewed pause drained automatically: %d", got)
		}
		u.app.SetFocus(tab.Editor.Area)
		x, y, _, _ := u.table.GetInnerRect()
		if u.resultWheelDemand(tcell.NewEventMouse(x, y, tcell.WheelDown, 0), tview.MouseScrollDown) {
			t.Error("wheel outside result focus requested rows")
		}
		u.app.SetFocus(u.table)
		u.table.SetOffset(1990, 0)
		if !u.resultWheelDemand(tcell.NewEventMouse(x, y, tcell.WheelDown, 0), tview.MouseScrollDown) {
			t.Error("wheel-down near loaded end did not request rows")
		}
	})
	waitUI(t, u, func() bool { return !tab.Busy })
	u.app.QueueUpdateDraw(func() {
		if got := tab.Results[0].Count(); got != 2001 {
			t.Errorf("final row count = %d, want 2001", got)
		}
		if tab.Execution.Phase != "Completed" {
			t.Errorf("phase = %q, want Completed", tab.Execution.Phase)
		}
	})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResultPagingDownwardNavigationKeysRequestOneBlock(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	conn, _ := database.Conn(context.Background())
	defer conn.Close()
	rows := sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2).AddRow(3).AddRow(4).AddRow(5)
	mock.ExpectQuery("SELECT navigation").WillReturnRows(rows).RowsWillBeClosed()
	session := &db.Session{DB: database, Conn: conn, Profile: config.Profile{Engine: "postgres"}, Adapter: db.NewAdapter("postgres")}
	tab := u.active()
	u.app.QueueUpdateDraw(func() {
		u.run(tab, "SELECT navigation", func(ctx context.Context, emit func(db.Event)) error {
			return session.Run(ctx, []db.Bound{{SQL: "SELECT navigation"}}, emit)
		})
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && len(tab.Results) == 1 && tab.Results[0].Count() == 1 && tab.Execution.Phase == "Paused"
	})
	keys := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyDown, 0, 0),
		tcell.NewEventKey(tcell.KeyCtrlN, 0, 0),
		tcell.NewEventKey(tcell.KeyPgDn, 0, 0),
		tcell.NewEventKey(tcell.KeyEnd, 0, 0),
		tcell.NewEventKey(tcell.KeyRune, 'j', 0),
	}
	for i, key := range keys {
		wantRows := i + 2
		u.app.QueueUpdateDraw(func() {
			u.app.SetFocus(u.table)
			u.table.Select(tab.Results[0].Count(), 0)
			forward := u.key(key)
			if forward != nil {
				u.table.InputHandler()(forward, func(p tview.Primitive) { u.app.SetFocus(p) })
			}
		})
		if wantRows <= 5 {
			waitForPagingState(t, u, func() bool { return tab.Busy && tab.Results[0].Count() == wantRows && tab.Execution.Phase == "Paused" })
		} else {
			waitUI(t, u, func() bool { return !tab.Busy })
		}
	}
	if got := tab.Results[0].Count(); got != 5 {
		t.Fatalf("navigation fetched %d rows, want 5", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResultPagingCancellationAndIndependentTabs(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	firstDB, firstMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer firstDB.Close()
	firstConn, _ := firstDB.Conn(context.Background())
	defer firstConn.Close()
	firstMock.ExpectQuery("SELECT first").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)).RowsWillBeClosed()
	firstSession := &db.Session{DB: firstDB, Conn: firstConn, Profile: config.Profile{Engine: "postgres"}, Adapter: db.NewAdapter("postgres")}
	first := u.active()
	u.app.QueueUpdateDraw(func() {
		u.run(first, "SELECT first", func(ctx context.Context, emit func(db.Event)) error {
			return firstSession.Run(ctx, []db.Bound{{SQL: "SELECT first"}}, emit)
		})
	})
	waitForPagingState(t, u, func() bool {
		return first.Busy && len(first.Results) == 1 && first.Results[0].Count() == 1 && first.Execution.Phase == "Paused"
	})
	u.app.QueueUpdateDraw(func() {
		before := len(u.tabs)
		u.closeTab()
		if len(u.tabs) != before || !strings.Contains(first.Status, "Cancel execution") {
			t.Error("paused tab closed without cancellation confirmation")
		}
	})

	secondDB, secondMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer secondDB.Close()
	secondConn, _ := secondDB.Conn(context.Background())
	defer secondConn.Close()
	secondMock.ExpectQuery("SELECT second").WillReturnRows(sqlmock.NewRows([]string{"id"})).RowsWillBeClosed()
	secondSession := &db.Session{DB: secondDB, Conn: secondConn, Profile: config.Profile{Engine: "postgres"}, Adapter: db.NewAdapter("postgres")}
	u.app.QueueUpdateDraw(func() {
		u.newTab(nil)
		second := u.active()
		u.run(second, "SELECT second", func(ctx context.Context, emit func(db.Event)) error {
			return secondSession.Run(ctx, []db.Bound{{SQL: "SELECT second"}}, emit)
		})
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if !first.Busy || first.Execution.Phase != "Paused" || first.Results[0].Count() != 1 {
			t.Error("second tab affected paused first tab")
		}
		u.current = 0
		u.showTab()
		dispatch(u, tcell.KeyF10, 0)
	})
	waitUI(t, u, func() bool { return !first.Busy })
	u.app.QueueUpdateDraw(func() {
		if first.Execution.Phase != "Cancelled" || first.Results[0].Count() != 1 || first.Execution.Pause != nil {
			t.Errorf("cancel did not retain partial rows and clear pause: phase=%s rows=%d", first.Execution.Phase, first.Results[0].Count())
		}
	})
	if err := firstMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := secondMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResultPagingOnlyActivePausedSetCanAdvance(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	conn, _ := database.Conn(context.Background())
	defer conn.Close()
	mock.ExpectQuery("SELECT sets").WillReturnRows(
		sqlmock.NewRows([]string{"first"}).AddRow("one"),
		sqlmock.NewRows([]string{"second"}).AddRow("two"),
	).RowsWillBeClosed()
	session := &db.Session{DB: database, Conn: conn, Profile: config.Profile{Engine: "postgres"}, Adapter: db.NewAdapter("postgres")}
	tab := u.active()
	u.app.QueueUpdateDraw(func() {
		u.run(tab, "SELECT sets", func(ctx context.Context, emit func(db.Event)) error {
			return session.Run(ctx, []db.Bound{{SQL: "SELECT sets"}}, emit)
		})
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && len(tab.Results) == 1 && tab.Execution.Pause != nil && tab.Execution.Pause.Set == 0
	})
	u.app.QueueUpdateDraw(func() {
		u.app.SetFocus(u.table)
		u.table.Select(1, 0)
		dispatch(u, tcell.KeyEnd, 0)
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && len(tab.Results) == 2 && tab.Execution.Pause != nil && tab.Execution.Pause.Set == 1
	})
	u.app.QueueUpdateDraw(func() {
		tab.SelectedSet = 0
		u.showResults()
		u.app.SetFocus(u.table)
		u.table.Select(1, 0)
		if u.requestResultBlock(tab) {
			t.Error("old result set advanced the active paused set")
		}
	})
	time.Sleep(30 * time.Millisecond)
	u.app.QueueUpdateDraw(func() {
		if !tab.Busy || tab.Execution.Pause == nil || tab.Execution.Pause.Set != 1 {
			t.Error("inactive set selection changed pause")
		}
		tab.SelectedSet = 1
		u.showResults()
		u.app.SetFocus(u.table)
		u.table.Select(1, 0)
		dispatch(u, tcell.KeyEnd, 0)
	})
	waitUI(t, u, func() bool { return !tab.Busy })
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResultPagingPausedStatusFitsSupportedTerminalSizes(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	screen := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(screen)
	defer screen.Fini()
	store, err := result.New([]string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Append([]result.Cell{{Text: "1"}})
	tab := u.active()
	tab.Results = []*result.Store{store}
	tab.Busy = true
	tab.Execution = &execution{ID: 1, Source: "Query 1", Scope: "Execute script", Started: time.Now(), Phase: "Paused", Pause: &resultPause{Set: 0, Rows: 1}}
	u.showResults()
	u.app.SetFocus(u.table)
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		screen.SetSize(size[0], size[1])
		u.app.ForceDraw()
		text := screenText(screen)
		if !strings.Contains(text, "F10") || (!strings.Contains(text, "Paused") && !strings.Contains(text, "pause")) || !strings.Contains(text, "1 received") {
			t.Fatalf("paused paging guidance missing at %dx%d: %s", size[0], size[1], text)
		}
	}
}

func TestQueuedPauseFromReplacedExecutionIsIgnored(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	tab := u.active()
	oldPager, _ := db.NewPager(1)
	old := &execution{ID: 1, Pager: oldPager, Phase: "Running"}
	tab.Execution = &execution{ID: 2, Phase: "Running"}
	u.setResultPause(tab, old, db.Event{Kind: "paused", Set: 0, Rows: 1})
	if old.Pause != nil || tab.Execution.Phase != "Running" {
		t.Fatal("queued pause from old execution affected replacement")
	}
}

func TestResultBlockSizeSettingPersistsAndRollsBack(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1000})
	if err := config.Save(u.configPath, u.cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(u.configPath)
	if err != nil {
		t.Fatal(err)
	}
	u.app.QueueUpdateDraw(func() {
		u.resultBlockSizePrompt()
		form := u.dialogContent.(*tview.Form)
		form.GetFormItem(0).(*tview.InputField).SetText("-1")
		form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	waitUI(t, u, func() bool { return u.dialogContent == nil })
	afterInvalid, err := os.ReadFile(u.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if u.cfg.ResultBlockSize != 1000 || string(afterInvalid) != string(before) {
		t.Fatal("invalid setting changed config or file")
	}

	u.app.QueueUpdateDraw(func() {
		u.resultBlockSizePrompt()
		form := u.dialogContent.(*tview.Form)
		form.GetFormItem(0).(*tview.InputField).SetText("2500")
		form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	waitUI(t, u, func() bool { return u.dialogContent == nil })
	loaded, err := config.Load(u.configPath)
	if err != nil || loaded.ResultBlockSize != 2500 || u.cfg.ResultBlockSize != 2500 {
		t.Fatalf("setting was not persisted: cfg=%d loaded=%d err=%v", u.cfg.ResultBlockSize, loaded.ResultBlockSize, err)
	}

	failedPath := filepath.Join(t.TempDir(), "config-directory")
	if err := os.Mkdir(failedPath, 0700); err != nil {
		t.Fatal(err)
	}
	u.configPath = failedPath
	u.app.QueueUpdateDraw(func() {
		u.resultBlockSizePrompt()
		form := u.dialogContent.(*tview.Form)
		form.GetFormItem(0).(*tview.InputField).SetText("3000")
		form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	waitUI(t, u, func() bool { return u.dialogContent == nil })
	if u.cfg.ResultBlockSize != 2500 {
		t.Fatal("failed save changed in-memory config")
	}
}

func waitForPagingState(t *testing.T, u *UI, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ready := false
		u.app.QueueUpdateDraw(func() { ready = predicate() })
		if ready {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	var state string
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		rows, phase := 0, "<nil>"
		if len(tab.Results) > 0 {
			rows = tab.Results[0].Count()
		}
		if tab.Execution != nil {
			phase = tab.Execution.Phase
		}
		state = fmt.Sprintf("busy=%v rows=%d phase=%s", tab.Busy, rows, phase)
	})
	t.Fatalf("paging state timed out: %s", state)
}

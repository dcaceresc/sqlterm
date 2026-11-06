package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	// Closing a paused tab releases its read instead of refusing; the required
	// release-then-close sequence is covered by
	// TestClosingPausedTabReleasesDriverAndCloses.

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

// mockTabSession attaches a sqlmock connection to tab and returns the session
// used to run paged statements through the simulated TUI.
func mockTabSession(t *testing.T, u *UI, tab *Tab) (*db.Session, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	session := &db.Session{DB: database, Conn: conn, Profile: config.Profile{Engine: "postgres"}, Adapter: db.NewAdapter("postgres")}
	u.app.QueueUpdateDraw(func() {
		tab.Session = session
		tab.Profile = config.Profile{Name: "demo", Engine: "postgres", Database: "academia"}
	})
	return session, mock
}

func startStatement(u *UI, tab *Tab, session *db.Session, sql string) {
	u.app.QueueUpdateDraw(func() {
		u.run(tab, sql, func(ctx context.Context, emit func(db.Event)) error {
			return session.Run(ctx, []db.Bound{{SQL: sql}}, emit)
		})
	})
}

// pauseStatement runs one statement and waits until it holds the tab paused
// after the first configured block of rows.
func pauseStatement(t *testing.T, u *UI, tab *Tab, session *db.Session, sql string, rows int) {
	t.Helper()
	startStatement(u, tab, session, sql)
	waitForPagingState(t, u, func() bool {
		return tab.Busy && len(tab.Results) == 1 && tab.Results[0].Count() == rows && tab.Execution.Phase == "Paused"
	})
}

func TestNewExecutionReleasesPausedReadAndRunsReplacement(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	tab := u.active()
	session, mock := mockTabSession(t, u, tab)
	mock.ExpectQuery("SELECT old").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("old").AddRow("old")).RowsWillBeClosed()
	mock.ExpectQuery("SELECT replacement").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("new").AddRow("new")).RowsWillBeClosed()
	pauseStatement(t, u, tab, session, "SELECT old", 1)
	previous := 0
	u.app.QueueUpdateDraw(func() {
		previous = tab.Execution.ID
		tab.Editor.SetText("SELECT replacement")
		dispatch(u, tcell.KeyF5, 0)
		if !strings.Contains(tab.Status, "Releasing paused") {
			t.Errorf("no transitional status while releasing the paused read: %q", tab.Status)
		}
		if !tab.Busy {
			t.Error("busy cleared before the paused task finished")
		}
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && tab.Execution != nil && tab.Execution.ID != previous && tab.Execution.Phase == "Paused" &&
			len(tab.Results) == 1 && tab.Results[0].Count() == 1
	})
	u.app.QueueUpdateDraw(func() {
		row, err := tab.Results[0].Row(0)
		if err != nil || row[0].Text != "new" {
			t.Errorf("replacement results missing: %v", err)
		}
		if tab.Execution.SQL != "SELECT replacement" {
			t.Errorf("execution identity did not switch: %q", tab.Execution.SQL)
		}
		if tab.Execution.Pause == nil || tab.Execution.Pause.Rows != 1 {
			t.Error("replacement did not pause on its own first block")
		}
	})
	u.app.QueueUpdateDraw(func() { dispatch(u, tcell.KeyF10, 0) })
	waitUI(t, u, func() bool { return !tab.Busy })
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeReleasesPausedReadBeforeReadingParameters(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	tab := u.active()
	session, mock := mockTabSession(t, u, tab)
	mock.ExpectQuery("SELECT old").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("old").AddRow("old")).RowsWillBeClosed()
	mock.ExpectQuery("FROM pg_proc p CROSS JOIN LATERAL").WillReturnRows(sqlmock.NewRows([]string{"name", "type", "mode", "default"}).AddRow("arg", "text", "IN", false))
	mock.ExpectQuery("CALL \"public\"\\.\"refresh\"").WithArgs("").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("invoked")).RowsWillBeClosed()
	pauseStatement(t, u, tab, session, "SELECT old", 1)
	previous := 0
	u.app.QueueUpdateDraw(func() {
		previous = tab.Execution.ID
		tab.Object = &db.Object{Schema: "public", Name: "refresh", Kind: "procedure"}
		u.app.SetFocus(tab.Editor.Area)
		dispatch(u, tcell.KeyF9, 0)
		if !strings.Contains(tab.Status, "Releasing paused") {
			t.Errorf("no transitional status before reading parameters: %q", tab.Status)
		}
	})
	waitUI(t, u, func() bool { return u.dialogContent != nil })
	u.app.QueueUpdateDraw(func() {
		panel, ok := u.dialogContent.(*tview.Flex)
		if !ok {
			t.Errorf("parameter form missing after the paused read was released: %T", u.dialogContent)
			return
		}
		form := panel.GetItem(0).(*tview.Form)
		form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && tab.Execution != nil && tab.Execution.Phase == "Paused" &&
			len(tab.Results) == 1 && tab.Results[0].Count() == 1
	})
	u.app.QueueUpdateDraw(func() {
		if tab.Execution.ID == previous {
			t.Error("invocation did not replace the paused execution")
		}
		row, err := tab.Results[0].Row(0)
		if err != nil || row[0].Text != "invoked" {
			t.Errorf("invocation results missing after release: %v", err)
		}
	})
	u.app.QueueUpdateDraw(func() { dispatch(u, tcell.KeyF10, 0) })
	waitUI(t, u, func() bool { return !tab.Busy })
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseChangeAndDisconnectReleasePausedRead(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	tab := u.active()
	session, mock := mockTabSession(t, u, tab)
	mock.ExpectQuery("SELECT old").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)).RowsWillBeClosed()
	mock.ExpectQuery("SELECT datname FROM pg_database").WillReturnRows(sqlmock.NewRows([]string{"datname"}).AddRow("otra"))
	pauseStatement(t, u, tab, session, "SELECT old", 1)
	u.app.QueueUpdateDraw(func() {
		dispatch(u, tcell.KeyF4, 0)
		if !strings.Contains(tab.Status, "Releasing paused") {
			t.Errorf("database change did not announce the release: %q", tab.Status)
		}
	})
	waitUI(t, u, func() bool { return u.dialogContent != nil && !tab.Busy })
	u.app.QueueUpdateDraw(func() {
		u.dismiss()
		u.disconnect()
	})
	waitUI(t, u, func() bool { return tab.Session == nil && !tab.Busy })
	u.app.QueueUpdateDraw(func() {
		if tab.Status != "Disconnected" {
			t.Errorf("disconnect status after release = %q", tab.Status)
		}
	})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClosingPausedTabReleasesDriverAndCloses(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	tab := u.active()
	session, mock := mockTabSession(t, u, tab)
	mock.ExpectQuery("SELECT closable").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)).RowsWillBeClosed()
	pauseStatement(t, u, tab, session, "SELECT closable", 1)
	u.app.QueueUpdateDraw(func() {
		u.closeTab()
		if !strings.Contains(tab.Status, "Releasing paused") {
			t.Errorf("close did not announce the release: %q", tab.Status)
		}
	})
	waitUI(t, u, func() bool { return u.active() != tab && !tab.Busy })
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	// A tab that is still working keeps the previous close guarantee.
	working := u.active()
	u.app.QueueUpdateDraw(func() {
		working.Busy, working.Cancel = true, func() {}
		working.Execution = &execution{ID: 99, Phase: "Receiving rows"}
		u.closeTab()
		if !strings.Contains(working.Status, "Cancel execution before closing this tab") || len(u.tabs) != 1 {
			t.Errorf("working tab closed without cancellation: %q", working.Status)
		}
		working.Busy, working.Cancel, working.Execution = false, nil, nil
	})
}

func TestQuitReleasesPausedReadAndStops(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{ResultBlockSize: 1})
	screen := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(screen)
	screen.SetSize(120, 40)
	done := make(chan error, 1)
	go func() { done <- u.Run() }()
	tab := u.active()
	session, mock := mockTabSession(t, u, tab)
	mock.ExpectQuery("SELECT quitting").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)).RowsWillBeClosed()
	u.app.QueueUpdateDraw(func() { u.dismiss() })
	pauseStatement(t, u, tab, session, "SELECT quitting", 1)
	u.app.QueueUpdateDraw(func() { u.quit() })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("quit did not release the paused read and stop")
	}
	if tab.Busy {
		t.Error("busy left behind after quit")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPauseFreezesElapsedAndStopsPulseRefresh(t *testing.T) {
	u := runningUI(t, config.Config{ResultBlockSize: 1})
	tab := u.active()
	session, mock := mockTabSession(t, u, tab)
	mock.ExpectQuery("SELECT pulse").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)).RowsWillBeClosed()
	pauseStatement(t, u, tab, session, "SELECT pulse", 1)
	var draws int32
	u.app.QueueUpdateDraw(func() {
		u.app.SetAfterDrawFunc(func(tcell.Screen) { atomic.AddInt32(&draws, 1) })
	})
	// Let any in-flight pulse land before measuring an idle paused tab.
	time.Sleep(400 * time.Millisecond)
	var frozen string
	var started time.Time
	u.app.QueueUpdateDraw(func() {
		started, frozen = tab.Execution.Started, u.resultInfo.GetText(true)
		if !strings.Contains(frozen, "1 received") || !strings.Contains(frozen, "F10") || !strings.Contains(frozen, "Paused") {
			t.Errorf("paused indicator missing received count or guidance: %q", frozen)
		}
	})
	atomic.StoreInt32(&draws, 0)
	time.Sleep(900 * time.Millisecond)
	if n := atomic.LoadInt32(&draws); n != 0 {
		t.Errorf("paused execution redrew %d times without user action", n)
	}
	u.app.QueueUpdateDraw(func() {
		if got := u.resultInfo.GetText(true); got != frozen {
			t.Errorf("paused elapsed time kept changing:\n%q\n%q", frozen, got)
		}
		// Copy stays restricted while the reader is still open.
		u.gridKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0))
		if !strings.Contains(tab.Status, "Press F10 to stop receiving") {
			t.Errorf("copy while paused no longer explained: %q", tab.Status)
		}
		u.app.SetFocus(u.table)
		u.table.Select(1, 0)
		dispatch(u, tcell.KeyDown, 0)
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && tab.Execution.Phase == "Paused" && len(tab.Results) == 1 && tab.Results[0].Count() == 2
	})
	u.app.QueueUpdateDraw(func() {
		if tab.Execution.Started != started {
			t.Error("resuming the next block reset the start time")
		}
		if resumed := u.resultInfo.GetText(true); resumed == frozen {
			t.Error("elapsed duration did not continue after resuming the next block")
		}
	})
	u.app.QueueUpdateDraw(func() { dispatch(u, tcell.KeyF10, 0) })
	waitUI(t, u, func() bool { return !tab.Busy })
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// holdPaused drives a real task that pauses after its first block and then keeps
// the driver busy until the returned release func is called, so tests can act
// inside the window between requesting the release and the task finishing.
func holdPaused(t *testing.T, u *UI, tab *Tab) (cancelled chan struct{}, release func()) {
	t.Helper()
	cancelled = make(chan struct{})
	released, done := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(released) }) }
	t.Cleanup(func() {
		// Cancel the held read (if it is still paused) and let the driver finish
		// before the runningUI cleanup joins every task.
		u.app.QueueUpdateDraw(func() {
			if tab.Cancel != nil {
				tab.Cancel()
			}
		})
		release()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("held driver never finished")
		}
	})
	u.app.QueueUpdateDraw(func() {
		u.run(tab, "SELECT held", func(ctx context.Context, emit func(db.Event)) error {
			defer close(done)
			emit(db.Event{Kind: "paused", Set: 0, Rows: 1})
			<-ctx.Done()
			close(cancelled)
			<-released
			return ctx.Err()
		})
	})
	waitForPagingState(t, u, func() bool {
		return tab.Busy && tab.Execution != nil && tab.Execution.Phase == "Paused" && tab.Execution.Pause != nil
	})
	return cancelled, release
}

func waitForCancelledDriver(t *testing.T, cancelled chan struct{}) {
	t.Helper()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("paused read was not cancelled")
	}
}

func TestClosingPausedTabRemovesTheRequestedTabAfterRelease(t *testing.T) {
	u := runningUI(t, config.Config{})
	paused := u.active()
	cancelled, release := holdPaused(t, u, paused)
	var other *Tab
	u.app.QueueUpdateDraw(func() {
		paused.Editor.SetText("SELECT held")
		paused.Saved = "SELECT held"
		u.newTab(nil)
		other = u.active()
		other.Editor.SetText("SELECT other")
		other.Saved = "SELECT other"
	})
	session, _ := mockTabSession(t, u, other)
	store, err := result.New([]string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append([]result.Cell{{Text: "survivor"}}); err != nil {
		t.Fatal(err)
	}
	u.app.QueueUpdateDraw(func() { other.Results = []*result.Store{store} })

	// Close the paused tab, then switch tabs inside the release window before the
	// deferred removal runs.
	u.app.QueueUpdateDraw(func() {
		u.current = 0
		u.showTab()
		u.closeTab()
		if !strings.Contains(paused.Status, "Releasing paused") {
			t.Errorf("close did not announce the release: %q", paused.Status)
		}
	})
	waitForCancelledDriver(t, cancelled)
	u.app.QueueUpdateDraw(func() {
		u.current = 1
		u.showTab()
	})
	release()
	waitUI(t, u, func() bool { return len(u.tabs) == 1 })
	if u.tabs[0] != other || u.current != 0 || u.active() != other {
		t.Fatalf("closed tab removal kept the wrong identity: tabs=%d current=%d", len(u.tabs), u.current)
	}
	if other.Session != session || len(other.Results) != 1 || other.Results[0] != store || other.Results[0].Count() != 1 {
		t.Error("surviving tab lost its session or results")
	}
	if other.Editor.Area.GetText() != "SELECT other" {
		t.Error("surviving tab lost its SQL")
	}
}

func TestClosingTabKeepsPausedTabInAnotherTabIntact(t *testing.T) {
	u := runningUI(t, config.Config{})
	paused := u.active()
	_, _ = holdPaused(t, u, paused)
	var closing *Tab
	u.app.QueueUpdateDraw(func() {
		u.newTab(nil)
		closing = u.active()
		closing.Editor.SetText("SELECT closing")
		closing.Saved = "SELECT closing"
	})
	store, err := result.New([]string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append([]result.Cell{{Text: "closing"}}); err != nil {
		t.Fatal(err)
	}
	u.app.QueueUpdateDraw(func() {
		closing.Results = []*result.Store{store}
		u.closeTab()
	})
	waitUI(t, u, func() bool { return len(u.tabs) == 1 })
	if u.tabs[0] != paused || u.current != 0 || u.active() != paused {
		t.Fatalf("closing the active tab removed the paused one: tabs=%d current=%d", len(u.tabs), u.current)
	}
	if !paused.Busy || paused.Execution == nil || paused.Execution.Phase != "Paused" || paused.Execution.Pause == nil {
		t.Error("paused tab in another tab was disturbed")
	}
}

func TestClosingLastTabCreatesFreshTab(t *testing.T) {
	u := runningUI(t, config.Config{})
	paused := u.active()
	cancelled, release := holdPaused(t, u, paused)
	u.app.QueueUpdateDraw(func() { u.closeTab() })
	waitForCancelledDriver(t, cancelled)
	release()
	waitUI(t, u, func() bool { return u.active() != paused && !paused.Busy })
	if len(u.tabs) != 1 || u.current != 0 {
		t.Fatalf("last tab close left %d tabs at %d", len(u.tabs), u.current)
	}
	fresh := u.active()
	if fresh == paused || fresh.Busy || len(fresh.Results) != 0 {
		t.Error("fresh tab after closing the last tab is not clean")
	}
	u.app.QueueUpdateDraw(func() { u.closeTab() })
	waitUI(t, u, func() bool { return !u.active().Busy && len(u.tabs) == 1 })
	if u.active() == fresh || u.current != 0 {
		t.Errorf("closing the idle last tab did not replace it: current=%d", u.current)
	}
}

func TestCloseTabRefusesRunningStatementBeforeUnsavedPrompt(t *testing.T) {
	u := runningUI(t, config.Config{})
	tab := u.active()
	started, release := make(chan struct{}), make(chan struct{})
	u.app.QueueUpdateDraw(func() {
		tab.Editor.SetText("SELECT unsaved")
		u.run(tab, "SELECT unsaved", func(ctx context.Context, emit func(db.Event)) error {
			close(started)
			<-release
			return ctx.Err()
		})
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not start")
	}
	u.app.QueueUpdateDraw(func() {
		u.closeTab()
	})
	if u.dialogContent != nil {
		t.Error("running statement asked to discard unsaved SQL before refusing")
	}
	if !strings.Contains(tab.Status, "Cancel execution before closing this tab") {
		t.Errorf("running statement close status = %q", tab.Status)
	}
	if len(u.tabs) != 1 {
		t.Error("running statement closed the tab")
	}
	close(release)
	waitUI(t, u, func() bool { return !tab.Busy })
	// An idle tab with unsaved SQL still asks before discarding it.
	u.app.QueueUpdateDraw(func() { u.closeTab() })
	if u.dialogContent == nil {
		t.Error("idle tab with unsaved SQL closed without asking")
	} else {
		u.app.QueueUpdateDraw(func() { u.dismiss() })
		if len(u.tabs) != 1 {
			t.Error("dismissing the prompt closed the tab")
		}
	}
}

func TestBusyExecutionStillRefusesSecondStart(t *testing.T) {
	u := runningUI(t, config.Config{})
	tab := u.active()
	started, release := make(chan struct{}), make(chan struct{})
	starts := 0
	u.app.QueueUpdateDraw(func() {
		u.run(tab, "SELECT busy", func(ctx context.Context, emit func(db.Event)) error {
			starts++
			close(started)
			<-release
			return ctx.Err()
		})
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not start")
	}
	u.app.QueueUpdateDraw(func() {
		tab.Editor.SetText("SELECT second")
		dispatch(u, tcell.KeyF5, 0)
		if !strings.Contains(tab.Status, "Session busy") {
			t.Errorf("working tab did not refuse a second start: %q", tab.Status)
		}
	})
	if starts != 1 {
		t.Fatalf("second statement started while the first was running: %d", starts)
	}
	close(release)
	u.app.QueueUpdateDraw(func() { dispatch(u, tcell.KeyF10, 0) })
	waitUI(t, u, func() bool { return !tab.Busy })
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
		rows, phase, failure := 0, "<nil>", ""
		if len(tab.Results) > 0 {
			rows = tab.Results[0].Count()
		}
		if tab.Execution != nil {
			phase, failure = tab.Execution.Phase, tab.Execution.Error
		}
		state = fmt.Sprintf("busy=%v rows=%d phase=%s error=%q", tab.Busy, rows, phase, failure)
	})
	t.Fatalf("paging state timed out: %s", state)
}

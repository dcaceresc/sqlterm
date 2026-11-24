package ui

import (
	"context"
	"database/sql/driver"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gdamore/tcell/v2"

	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"sqlterm/internal/result"
)

func TestResultStreamReadsPastFormerBlockSizeWithoutNavigation(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	rows := sqlmock.NewRows([]string{"id"})
	for i := range 1001 {
		rows.AddRow(driver.Value(int64(i)))
	}
	const query = "SELECT id FROM fixture"
	mock.ExpectQuery(query).WillReturnRows(rows).RowsWillBeClosed()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	u := runningUI(t, cfg)
	session := &db.Session{Conn: conn, Adapter: db.NewAdapter("postgres")}
	u.app.QueueUpdateDraw(func() {
		u.run(u.active(), query, func(ctx context.Context, emit func(db.Event)) error {
			return session.Run(ctx, []db.Bound{{SQL: query}}, emit)
		})
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if s := u.currentStore(); s == nil || s.Count() != 1001 || u.active().Execution.Phase != "Completed" {
			t.Error("execution did not reach EOF without navigation")
		}
	})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResultStreamKeepsFirstRowVisibleAsStoreGrows(t *testing.T) {
	u, screen := modeTestUI(t)
	screen.SetSize(80, 24)
	s, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	tab := u.active()
	tab.Execution = &execution{Phase: "Receiving rows"}
	tab.Results = []*result.Store{s}
	if err := s.Append([]result.Cell{{Text: "first-row-marker"}}); err != nil {
		t.Fatal(err)
	}
	u.showResults()
	u.app.SetFocus(u.table)
	u.app.ForceDraw()
	if !strings.Contains(screenText(screen), "first-row-marker") {
		t.Fatal("first row not visible in initial frame")
	}
	for range 100 {
		if err := s.Append([]result.Cell{{Text: "later-row-marker"}}); err != nil {
			t.Fatal(err)
		}
	}
	u.app.ForceDraw()
	if row, _ := u.table.GetOffset(); row != 0 || !strings.Contains(screenText(screen), "first-row-marker") {
		t.Fatalf("stream scrolled away from its first row (offset %d)", row)
	}
	tab.Execution.Phase = "Completed"
	u.showResults()
	u.table.SetOffset(50, 0).Select(51, 1)
	u.app.ForceDraw()
	if row, _ := u.table.GetOffset(); row == 0 {
		t.Fatal("completed grid no longer permits browsing later rows")
	}
}

func TestResultStreamReportsProgressWithoutNavigation(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	more := make(chan struct{}, 1)
	finish := make(chan struct{}, 1)
	defer close(more)
	defer close(finish)
	u.app.QueueUpdateDraw(func() {
		u.run(u.active(), "SELECT 1", func(ctx context.Context, emit func(db.Event)) error {
			emit(db.Event{Kind: "columns", Store: s})
			if err := s.Append([]result.Cell{{Text: "first"}}); err != nil {
				return err
			}
			emit(db.Event{Kind: "rows", Rows: 1})
			<-more
			if err := s.Append([]result.Cell{{Text: "second"}}); err != nil {
				return err
			}
			emit(db.Event{Kind: "rows", Rows: 2})
			<-finish
			return nil
		})
	})
	waitUI(t, u, func() bool { return strings.Contains(u.table.GetTitle(), "1 received rows") })
	var initial string
	u.app.QueueUpdateDraw(func() {
		initial = u.status.GetText(true)
		u.app.SetFocus(u.table)
		dispatchSelector(u, tcell.KeyEnd, 0)
		if key := u.gridKey(tcell.NewEventKey(tcell.KeyDown, 0, 0)); key != nil {
			t.Error("running results did not keep the first rows fixed")
		}
	})
	more <- struct{}{}
	waitUI(t, u, func() bool {
		return strings.Contains(u.table.GetTitle(), "2 received rows") && u.status.GetText(true) != initial
	})
	finish <- struct{}{}
	waitUI(t, u, func() bool { return !u.active().Busy && u.active().Execution.Phase == "Completed" })
}

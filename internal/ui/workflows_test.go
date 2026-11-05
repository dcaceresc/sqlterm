package ui

import (
	"context"
	"encoding/csv"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"os"
	"path/filepath"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"sqlterm/internal/result"
	"strings"
	"testing"
	"time"
)

func runningUI(t *testing.T, c config.Config) *UI {
	t.Helper()
	u := New(filepath.Join(t.TempDir(), "config.json"), c)
	s := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(s)
	s.SetSize(120, 40)
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	u.app.QueueUpdateDraw(func() {})
	t.Cleanup(func() {
		u.app.QueueUpdateDraw(func() { u.closing = true; u.cancel() })
		u.wg.Wait()
		u.app.QueueUpdateDraw(func() { u.app.Stop() })
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("UI did not stop")
		}
		for _, tab := range u.tabs {
			u.dispose(tab)
		}
	})
	return u
}
func waitUI(t *testing.T, u *UI, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		ready := false
		u.app.QueueUpdateDraw(func() { ready = predicate() })
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("UI operation did not complete")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func TestCopyResultsWithAndWithoutHeadersUsesDisplayedOrder(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"name", "value"})
	if err != nil {
		t.Fatal(err)
	}
	s.Append([]result.Cell{{Text: "first"}, {Text: "1"}})
	s.Append([]result.Cell{{Text: "second"}, {Null: true}})
	copied := make(chan string, 4)
	u.app.QueueUpdateDraw(func() {
		u.active().Results = []*result.Store{s}
		u.active().Order = []int{1, 0}
		u.showResults()
		u.app.SetFocus(u.table)
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		u.copyResults("all", true)
	})
	var text string
	select {
	case text = <-copied:
	case <-time.After(3 * time.Second):
		t.Fatal("copy did not reach clipboard")
	}
	waitUI(t, u, func() bool { return !u.active().Busy })
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 3 || rows[0][0] != "name" || rows[1][0] != "second" || rows[1][1] != `\N` {
		t.Fatal(rows, err)
	}
	u.app.QueueUpdateDraw(func() { u.copyResults("row", false) })
	select {
	case text = <-copied:
	case <-time.After(3 * time.Second):
		t.Fatal("row copy timed out")
	}
	waitUI(t, u, func() bool { return !u.active().Busy })
	if text != "second\t\\N\n" {
		t.Fatal(text)
	}
	u.app.QueueUpdateDraw(func() {
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
		front, _ := u.pages.GetFrontPage()
		if front != "dialog" || u.closing {
			t.Error("Ctrl+C in results must show copy options")
		}
		u.dismiss()
		u.writeClipboard = func(string) error { return errors.New("clipboard locked") }
		u.copyResults("all", false)
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if !strings.Contains(u.active().Status, "clipboard locked") {
			t.Error("clipboard error not shown")
		}
	})
}
func TestCopyRowsSurvivesNULAndClipboardBackendPanic(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"name", "binary"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Append([]result.Cell{{Text: "normal"}, {Text: "\x00row\x00version"}}); err != nil {
		t.Fatal(err)
	}
	copied := make(chan string, 1)
	u.app.QueueUpdateDraw(func() {
		u.active().Results = []*result.Store{s}
		u.showResults()
		u.writeClipboard = func(text string) error {
			if strings.ContainsRune(text, 0) {
				panic("embedded NUL rejected by native conversion")
			}
			copied <- text
			return nil
		}
	})
	for _, scope := range []string{"row", "all"} {
		for _, headers := range []bool{false, true} {
			u.app.QueueUpdateDraw(func() { u.copyResults(scope, headers) })
			select {
			case text := <-copied:
				if !strings.Contains(text, `\0row\0version`) {
					t.Fatalf("NUL not escaped: %q", text)
				}
				if strings.HasPrefix(text, "name\tbinary\n") != headers {
					t.Fatalf("incorrect headers: %q", text)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("clipboard did not receive row/results")
			}
			waitUI(t, u, func() bool { return !u.active().Busy })
		}
	}
	u.app.QueueUpdateDraw(func() {
		u.writeClipboard = func(string) error { panic("native clipboard failure") }
		u.copyResults("all", true)
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if !strings.Contains(u.active().Status, "clipboard backend rejected") || u.closing {
			t.Error("clipboard panic must leave UI open and show error")
		}
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		u.copyResults("cell", false)
	})
	select {
	case text := <-copied:
		if text != "normal" {
			t.Fatalf("cell copy after failure: %q", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("UI did not recover after clipboard panic")
	}
	waitUI(t, u, func() bool { return !u.active().Busy })
	row, err := s.Row(0)
	if err != nil || row[1].Text != "\x00row\x00version" {
		t.Fatal("copy changed stored results", err)
	}
}

func TestBlankDatabaseBrowsesDatabasesThenLoadsSelectedObjects(t *testing.T) {
	p := config.Profile{Name: "server", Engine: "sqlserver", Host: "localhost", User: "sa"}
	u := runningUI(t, config.Config{Profiles: []config.Profile{p}, Keys: map[string]string{"apply": "F4"}})
	first, m1, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	conn1, _ := first.Conn(context.Background())
	second, m2, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	conn2, _ := second.Conn(context.Background())
	m1.ExpectQuery("SELECT name FROM sys.databases").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("DEV").AddRow("QA"))
	m1.ExpectQuery("SELECT name FROM sys.databases").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("DEV").AddRow("QA"))
	m2.ExpectQuery("SELECT s.name").WillReturnRows(sqlmock.NewRows([]string{"schema", "name", "kind", "id", "signature", "scalar"}).AddRow("dbo", "Orders", "table", "1", "", false))
	u.app.QueueUpdateDraw(func() {
		u.openSession = func(_ context.Context, profile config.Profile, _ string) (*db.Session, error) {
			if profile.Database == "" {
				profile.Database = "master"
				return &db.Session{DB: first, Conn: conn1, Profile: profile, Adapter: db.NewAdapter("sqlserver")}, nil
			}
			if profile.Database != "DEV" {
				t.Error("incorrect database selected")
			}
			return &db.Session{DB: second, Conn: conn2, Profile: profile, Adapter: db.NewAdapter("sqlserver")}, nil
		}
		u.connect(u.active(), p, "password")
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if !u.active().SelectingDatabase || u.objects.GetItemCount() != 2 {
			t.Error("blank profile did not show databases")
		}
		label, _ := u.objects.GetItemText(0)
		if label != "DEV" {
			t.Error(label)
		}
		u.search.SetText("DEV")
		u.execute()
		if !strings.Contains(u.active().Status, "F4") {
			t.Error("executed SQL in bootstrap database")
		}
		u.key(tcell.NewEventKey(tcell.KeyF4, 0, 0))
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		_, page := u.pages.GetFrontPage()
		list, ok := page.(*centered).child.(*tview.List)
		if !ok {
			t.Error("F4 did not show database picker")
			return
		}
		list.GetItemSelectedFunc(0)()
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if u.active().SelectingDatabase || u.active().Profile.Database != "DEV" || len(u.active().Catalog) != 1 {
			t.Error("selected database objects missing")
		}
		if u.search.GetText() != "" || u.objects.GetItemCount() != 1 {
			t.Error("database search filter hid the selected database objects")
		}
		if u.cfg.Profiles[0].Database != "" {
			t.Error("selection overwrote saved server profile")
		}
	})
	if err = m1.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
	if err = m2.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
func TestConnectionButtonUsesUnsavedFieldsAndDoesNotSave(t *testing.T) {
	p := config.Profile{Name: "saved", Engine: "sqlserver", Host: "old", User: "u", CredentialID: "saved-secret"}
	u := runningUI(t, config.Config{Profiles: []config.Profile{p}})
	opened := make(chan config.Profile, 2)
	closed := make(chan string, 2)
	var testMocks []sqlmock.Sqlmock
	u.app.QueueUpdateDraw(func() {
		u.vault = &memoryVault{values: map[string]string{"saved-secret": "stored password"}}
		u.openSession = func(_ context.Context, profile config.Profile, password string) (*db.Session, error) {
			opened <- profile
			database, mock, err := sqlmock.New()
			if err != nil {
				return nil, err
			}
			mock.ExpectClose()
			testMocks = append(testMocks, mock)
			closed <- password
			return &db.Session{DB: database, Profile: profile}, nil
		}
		u.profileForm(0)
		f := currentForm(u)
		f.GetFormItem(0).(*tview.InputField).SetText("unsaved")
		f.GetFormItem(2).(*tview.InputField).SetText("new-server")
		f.GetFormItem(3).(*tview.InputField).SetText("1444")
		f.GetFormItem(4).(*tview.InputField).SetText("")
		f.GetFormItem(6).(*tview.InputField).SetText(" new password ")
		f.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	select {
	case profile := <-opened:
		if profile.Host != "new-server" || profile.Port != 1444 || profile.Database != "" || profile.Name != "unsaved" {
			t.Fatal(profile)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("test did not open connection")
	}
	if password := <-closed; password != " new password " {
		t.Fatal("test used incorrect password")
	}
	waitUI(t, u, func() bool {
		return strings.Contains(currentForm(u).GetFormItem(10).(*tview.TextView).GetText(false), "successful")
	})
	u.app.QueueUpdateDraw(func() {
		if u.active().Session != nil || u.cfg.Profiles[0].Host != "old" || u.cfg.Profiles[0].Name != "saved" {
			t.Error("test saved or replaced connection")
		}
		f := currentForm(u)
		f.GetFormItem(6).(*tview.InputField).SetText("")
		f.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	<-opened
	if password := <-closed; password != "stored password" {
		t.Error("test did not reuse saved password")
	}
	waitUI(t, u, func() bool {
		return strings.Contains(currentForm(u).GetFormItem(10).(*tview.TextView).GetText(false), "successful")
	})
	if _, err := os.Stat(u.configPath); !os.IsNotExist(err) {
		t.Error("test wrote a profile file")
	}
	for _, mock := range testMocks {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error("test connection was not closed", err)
		}
	}
}
func TestConnectionButtonCanCancelAndRedactsErrors(t *testing.T) {
	u := runningUI(t, config.Config{})
	started := make(chan struct{})
	u.app.QueueUpdateDraw(func() {
		u.profileForm(-1)
		f := currentForm(u)
		f.GetFormItem(2).(*tview.InputField).SetText("localhost")
		f.GetFormItem(5).(*tview.InputField).SetText("u")
		f.GetFormItem(6).(*tview.InputField).SetText("secret-value")
		u.openSession = func(ctx context.Context, _ config.Profile, _ string) (*db.Session, error) {
			close(started)
			<-ctx.Done()
			return nil, errors.New("cancelled while sending secret-value")
		}
		f.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("test did not start")
	}
	u.app.QueueUpdateDraw(func() {
		f := currentForm(u)
		if f.GetButton(1).GetLabel() != "Cancel test" {
			t.Error("test cannot be cancelled")
		}
		f.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	waitUI(t, u, func() bool { return currentForm(u).GetButton(1).GetLabel() == "Test connection" })
	u.app.QueueUpdateDraw(func() {
		text := currentForm(u).GetFormItem(10).(*tview.TextView).GetText(false)
		if !strings.Contains(text, "failed") || strings.Contains(text, "secret-value") {
			t.Error("test failed to report or redact error")
		}
		if len(u.cfg.Profiles) != 0 {
			t.Error("test saved a profile")
		}
	})
}

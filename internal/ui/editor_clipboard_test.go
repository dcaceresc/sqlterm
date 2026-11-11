package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/result"
)

func TestEditorSQLCopySelectionIsLiteralAndPreservesEditor(t *testing.T) {
	for _, tt := range []struct{ mode, text, want string }{
		{"NORMAL", "select * from tabla", "select * from tabla"},
		{"INSERT", "SELECT 'café';\r\n-- note\nSELECT 2;", "SELECT 'café';\r\n-- note\nSELECT 2;"},
		{"VISUAL", "-- SQL, not spreadsheet data", "-- SQL, not spreadsheet data"},
		{"NORMAL", "SELECT '\x00';", `SELECT '\0';`},
	} {
		t.Run(tt.mode+tt.text[:min(6, len(tt.text))], func(t *testing.T) {
			u := runningUI(t, config.Config{})
			copied := make(chan string, 1)
			original := "prefix\n" + tt.text + "\nsuffix"
			var from, to int
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				tab.Editor.SetText(original)
				tab.Saved = original
				tab.Path = filepath.Join(t.TempDir(), "query.sql")
				tab.Editor.Clipboard = "internal Vim clipboard"
				start := len("prefix\n")
				if tt.mode == "VISUAL" {
					tab.Editor.Area.Select(start, start)
					tab.Editor.Handle(tcell.NewEventKey(tcell.KeyRune, 'v', 0))
					for range tt.text {
						tab.Editor.Handle(tcell.NewEventKey(tcell.KeyRune, 'l', 0))
					}
				} else {
					if tt.mode == "INSERT" {
						tab.Editor.Handle(tcell.NewEventKey(tcell.KeyRune, 'i', 0))
					}
					tab.Editor.Area.Select(start, start+len(tt.text))
				}
				_, from, to = tab.Editor.Area.GetSelection()
				u.writeClipboard = func(text string) error { copied <- text; return nil }
				u.app.SetFocus(tab.Editor.Area)
				if u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)) != nil {
					t.Error("Ctrl+C must not reach the application exit handler")
				}
			})
			select {
			case got := <-copied:
				if got != tt.want {
					t.Fatalf("copied %q, want %q", got, tt.want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("SQL selection did not reach the system clipboard writer")
			}
			waitUI(t, u, func() bool { return strings.HasPrefix(u.active().Status, "Copied SQL selection to clipboard") })
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				_, gotFrom, gotTo := tab.Editor.Area.GetSelection()
				if tab.Editor.Area.GetText() != original || tab.Saved != original || filepath.Base(tab.Path) != "query.sql" || tab.Editor.Mode != tt.mode || gotFrom != from || gotTo != to || tab.Editor.Clipboard != "internal Vim clipboard" {
					t.Error("copy changed editor state")
				}
				if tab.Busy || tab.Cancel != nil || tab.Session != nil || u.dialogContent != nil || u.app.GetFocus() != tab.Editor.Area {
					t.Error("copy started SQL work, opened a result menu or changed focus")
				}
				if strings.Contains(tt.text, "\x00") && !strings.Contains(tab.Status, `NUL escaped as \0`) {
					t.Error("NUL escaping was not reported")
				}
			})
		})
	}
}

func TestEditorSQLCopyWithoutSelectionPreservesClipboard(t *testing.T) {
	for _, text := range []string{"select * from tabla", ""} {
		u := runningUI(t, config.Config{})
		var calls atomic.Int32
		u.app.QueueUpdateDraw(func() {
			tab := u.active()
			tab.Editor.SetText(text)
			tab.Saved = text
			tab.Editor.Area.Select(0, 0)
			u.writeClipboard = func(string) error { calls.Add(1); return nil }
			u.app.SetFocus(tab.Editor.Area)
			u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
			if tab.Status != "Select SQL text before copying" || calls.Load() != 0 || tab.Editor.Area.GetText() != text || tab.Saved != text || u.dialogContent != nil {
				t.Error("missing selection must show notice without copying or changing SQL")
			}
		})
	}
}

func TestEditorSQLCopyUsesSpanishMessages(t *testing.T) {
	u := runningUI(t, config.Config{Language: "es"})
	copied := make(chan string, 1)
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("SELECT '\x00';")
		tab.Saved = tab.Editor.Area.GetText()
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		u.app.SetFocus(tab.Editor.Area)
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
		if tab.Status != "Selecciona texto SQL antes de copiar" {
			t.Error("missing-selection notice not translated")
		}
		tab.Editor.Area.Select(0, len(tab.Saved))
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
	})
	select {
	case got := <-copied:
		if got != `SELECT '\0';` {
			t.Errorf("copied %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("copy did not reach writer")
	}
	waitUI(t, u, func() bool {
		return u.active().Status == `Selección SQL copiada al portapapeles (NUL representado como \0)`
	})
	u.app.QueueUpdateDraw(func() {
		u.writeClipboard = func(string) error { return errors.New("clipboard locked") }
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
	})
	waitUI(t, u, func() bool { return u.active().Status == "No se pudo copiar el SQL: clipboard locked" })
}

func TestEditorSQLCopyDuringQueryDoesNotChangeQueryState(t *testing.T) {
	u := runningUI(t, config.Config{})
	copied := make(chan string, 1)
	var cancelled atomic.Int32
	var pending atomic.Int32
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("select * from tabla")
		tab.Saved = tab.Editor.Area.GetText()
		tab.Editor.Area.Select(0, len(tab.Saved))
		store, err := result.New([]string{"value"})
		if err != nil {
			t.Error(err)
			return
		}
		tab.Results = []*result.Store{store}
		tab.Busy = true
		tab.Cancel = func() { cancelled.Add(1) }
		tab.Pending = func() { pending.Add(1) }
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		u.app.SetFocus(tab.Editor.Area)
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
	})
	select {
	case got := <-copied:
		if got != "select * from tabla" {
			t.Error(got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("query state blocked SQL copy")
	}
	waitUI(t, u, func() bool { return u.active().Status == "Copied SQL selection to clipboard" })
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		if !tab.Busy || tab.Cancel == nil || tab.Pending == nil || cancelled.Load() != 0 || pending.Load() != 0 || tab.Results[0].Count() != 0 {
			t.Error("copy changed running query/cancellation state")
		}
		tab.Busy = false
		tab.Cancel = nil
		tab.Pending = nil
	})
}

func TestEditorSQLCopyReportsErrorsAndPanics(t *testing.T) {
	for _, panicBackend := range []bool{false, true} {
		u := runningUI(t, config.Config{})
		u.app.QueueUpdateDraw(func() {
			tab := u.active()
			tab.Editor.SetText("select 1;")
			tab.Saved = tab.Editor.Area.GetText()
			tab.Editor.Area.Select(0, len(tab.Saved))
			u.writeClipboard = func(string) error {
				if panicBackend {
					panic("backend failed")
				}
				return errors.New("clipboard locked")
			}
			u.app.SetFocus(tab.Editor.Area)
			u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
		})
		waitUI(t, u, func() bool { return strings.HasPrefix(u.active().Status, "Could not copy SQL: ") })
		u.app.QueueUpdateDraw(func() {
			tab := u.active()
			want := "clipboard locked"
			if panicBackend {
				want = "clipboard backend rejected the text"
			}
			if !strings.Contains(tab.Status, want) || tab.Busy || tab.Editor.Area.GetText() != tab.Saved || u.closing || u.dialogContent != nil {
				t.Error("copy failure must be visible without changing SQL or closing application")
			}
		})
	}
}

func TestEditorSQLCopyKeepsUndoHistory(t *testing.T) {
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		e.SetText("select 1;")
		e.Area.Replace(7, 8, "2")
		e.Area.Select(0, len(e.Area.GetText()))
		u.writeClipboard = func(string) error { return nil }
		u.app.SetFocus(e.Area)
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
	})
	waitUI(t, u, func() bool { return u.active().Status == "Copied SQL selection to clipboard" })
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		e.Area.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlZ, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		if e.Area.GetText() != "select 1;" {
			t.Error("copy changed the undo history")
		}
	})
}

func TestEditorSQLCopyRejectsDuplicateAndSurvivesClosedTab(t *testing.T) {
	u := runningUI(t, config.Config{})
	entered := make(chan string, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var calls atomic.Int32
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("select 1;")
		tab.Saved = tab.Editor.Area.GetText()
		tab.Editor.Area.Select(0, len(tab.Saved))
		u.writeClipboard = func(text string) error {
			n := calls.Add(1)
			entered <- text
			if n == 1 {
				<-release
			}
			return nil
		}
		u.app.SetFocus(tab.Editor.Area)
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
		if tab.Status != "SQL clipboard copy already in progress" || tab.Busy {
			t.Error("duplicate copy must be bounded independently of query state")
		}
	})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("copy did not start")
	}
	u.app.QueueUpdateDraw(func() {
		u.closeTab()
		tab := u.active()
		tab.Editor.SetText("select 2;")
		tab.Editor.Area.Select(0, len(tab.Editor.Area.GetText()))
		tab.Status = "replacement tab status"
	})
	releaseOnce.Do(func() { close(release) })
	done := make(chan struct{})
	go func() { u.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("copy did not finish after original tab closed")
	}
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		if tab.Status != "replacement tab status" || calls.Load() != 1 {
			t.Error("completion affected another tab or duplicate reached clipboard")
		}
		u.app.SetFocus(tab.Editor.Area)
		u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
	})
	select {
	case got := <-entered:
		if got != "select 2;" {
			t.Error("copy guard did not reset")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subsequent copy blocked")
	}
	waitUI(t, u, func() bool { return u.active().Status == "Copied SQL selection to clipboard" })
}

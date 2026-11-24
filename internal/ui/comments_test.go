package ui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestCommentCurrentLineSelectionAndDocumentActions(t *testing.T) {
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("select ñ;\nselect 2;")
		tab.Saved = tab.Editor.Area.GetText()
		tab.Path = filepath.Join(t.TempDir(), "query.sql")
		originalPath := tab.Path
		u.app.SetFocus(tab.Editor.Area)
		u.commentSQL(false)
		if tab.Editor.Area.GetText() != "-- select ñ;\nselect 2;" || tab.Status == "Select SQL text before commenting" {
			t.Error("no selection must comment the current line without a notice")
		}
		u.commentSQL(false)
		if tab.Editor.Area.GetText() != tab.Saved {
			t.Error("repeated current-line toggle must restore SQL")
		}
		tab.Editor.Area.Select(0, 6)
		u.commentSQL(false)
		if tab.Editor.Area.GetText() != "-- select ñ;\nselect 2;" {
			t.Error("comment action did not comment selection")
		}
		u.commentSQL(false)
		if tab.Editor.Area.GetText() != tab.Saved {
			t.Error("second comment action must uncomment the same selection")
		}
		u.commentSQL(false)
		if tab.Saved != "select ñ;\nselect 2;" || tab.Path != originalPath || tab.Busy || tab.Session != nil {
			t.Error("comment action changed saved state or executed SQL")
		}
		u.commentSQL(true)
		if tab.Editor.Area.GetText() != "select ñ;\n-- select 2;" {
			t.Error("whole-document action must toggle every line")
		}
		if tab.Saved != "select ñ;\nselect 2;" || tab.Path != originalPath || tab.Busy || tab.Session != nil {
			t.Error("document action changed saved state or executed SQL")
		}
		if u.dialogContent != nil || u.app.GetFocus() != tab.Editor.Area {
			t.Error("comment action must return to editor")
		}
	})
}

func TestCommentShortcutDoesNotAffectOtherPanelsOrDialogs(t *testing.T) {
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("select 1;")
		tab.Editor.Area.Select(0, 6)
		u.app.SetFocus(u.search)
		event := tcell.NewEventKey(tcell.KeyRune, '}', tcell.ModCtrl)
		u.key(event)
		if tab.Editor.Area.GetText() != "select 1;" {
			t.Error("shortcut edited SQL outside editor")
		}
		u.prompt("Test", "", false, func(string) {})
		u.key(event)
		if tab.Editor.Area.GetText() != "select 1;" || u.dialogContent == nil {
			t.Error("shortcut must not edit SQL or dismiss dialog")
		}
		u.dismiss()
		u.app.SetFocus(tab.Editor.Area)
		key := tcell.NewEventKey(tcell.KeyRune, '/', 0)
		if got := u.key(key); got != key {
			t.Error("plain / must be passed through to the editor")
		}
	})
}

func TestFormerCommentShortcutsPassThrough(t *testing.T) {
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("select 1;\nselect 2;")
		tab.Saved = tab.Editor.Area.GetText()
		u.app.SetFocus(tab.Editor.Area)
		for _, event := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyRune, '}', tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyRune, '}', tcell.ModCtrl|tcell.ModShift),
			tcell.NewEventKey(tcell.KeyRune, '}', 0),
			tcell.NewEventKey(tcell.KeyRune, '}', tcell.ModShift),
			tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModAlt),
			tcell.NewEventKey(tcell.KeyRune, '}', tcell.ModCtrl|tcell.ModAlt),
		} {
			if u.key(event) != event || tab.Editor.Area.GetText() != tab.Saved {
				t.Errorf("unbound event %v must pass through without editing SQL", event)
			}
		}
	})
}

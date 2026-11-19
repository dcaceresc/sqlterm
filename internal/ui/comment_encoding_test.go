package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestCommentControlCodeEncoding(t *testing.T) {
	for _, modifiers := range []tcell.ModMask{0, tcell.ModCtrl, tcell.ModShift, tcell.ModCtrl | tcell.ModShift} {
		t.Run(tcell.NewEventKey(tcell.KeyCtrlRightSq, 0, modifiers).Name(), func(t *testing.T) {
			u := runningUI(t, config.Config{})
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				const original = "select 1;\nselect 2;"
				tab.Editor.SetText(original)
				tab.Saved = original
				tab.Editor.Area.Select(0, 6)
				u.app.SetFocus(tab.Editor.Area)
				event := tcell.NewEventKey(tcell.KeyCtrlRightSq, 0, modifiers)
				want := "-- select 1;\nselect 2;"
				if modifiers&tcell.ModShift != 0 {
					want = "-- select 1;\n-- select 2;"
				}
				if u.key(event) != nil || tab.Editor.Area.GetText() != want {
					t.Errorf("encoded shortcut: got %q, want %q", tab.Editor.Area.GetText(), want)
				}
				u.key(event)
				if tab.Editor.Area.GetText() != original || tab.Saved != original || tab.Busy || tab.Session != nil {
					t.Error("repeated shortcut must restore SQL without saving or executing")
				}
				u.app.SetFocus(u.search)
				if u.key(event) != event || tab.Editor.Area.GetText() != original {
					t.Error("encoded shortcut must remain scoped to editor")
				}
				u.prompt("Test", "", false, func(string) {})
				u.key(event)
				if tab.Editor.Area.GetText() != original || u.dialogContent == nil {
					t.Error("encoded shortcut must not edit SQL in dialogs")
				}
			})
		})
	}
}

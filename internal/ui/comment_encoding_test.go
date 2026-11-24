package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestFormerCommentControlCodesPassThrough(t *testing.T) {
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
				if got := u.key(event); got != event || tab.Editor.Area.GetText() != original || tab.Saved != original || tab.Busy || tab.Session != nil {
					t.Errorf("former comment event %v must pass through without editing, saving, or executing SQL", event)
				}
				u.app.SetFocus(u.search)
				if u.key(event) != event || tab.Editor.Area.GetText() != original {
					t.Error("former comment event must not affect the search panel")
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

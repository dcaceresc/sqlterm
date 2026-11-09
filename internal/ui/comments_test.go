package ui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func TestCommentSelectionShortcutAndPalette(t *testing.T) {
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("select ñ;\nselect 2;")
		tab.Saved = tab.Editor.Area.GetText()
		tab.Path = filepath.Join(t.TempDir(), "query.sql")
		originalPath := tab.Path
		u.app.SetFocus(tab.Editor.Area)
		event := tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModAlt)
		if u.key(event) != nil {
			t.Error("Alt+/ must be consumed in editor")
		}
		if tab.Editor.Area.GetText() != tab.Saved || tab.Status != "Select SQL text before commenting" {
			t.Error("no selection must show a notice without editing")
		}
		tab.Editor.Area.Select(0, 6)
		if u.key(event) != nil || tab.Editor.Area.GetText() != "-- select ñ;\nselect 2;" {
			t.Error("Alt+/ did not comment selection")
		}
		u.key(event)
		if tab.Editor.Area.GetText() != tab.Saved {
			t.Error("second Alt+/ must uncomment the same selection")
		}
		u.key(event)
		if tab.Saved != "select ñ;\nselect 2;" || tab.Path != originalPath || tab.Busy || tab.Session != nil {
			t.Error("comment action changed saved state or executed SQL")
		}
		u.key(tcell.NewEventKey(tcell.KeyCtrlP, 0, 0))
		panel, ok := u.dialogContent.(*tview.Flex)
		if !ok {
			t.Error("Ctrl+P must remain the palette")
			return
		}
		list := panel.GetItem(1).(*tview.List)
		found := false
		for i := 0; i < list.GetItemCount(); i++ {
			name, _ := list.GetItemText(i)
			if name == "Comment whole document" {
				list.SetCurrentItem(i)
				u.app.SetFocus(list)
				list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
				found = true
				break
			}
		}
		if !found || tab.Editor.Area.GetText() != "select ñ;\n-- select 2;" {
			t.Error("palette must offer whole-document commenting")
		}
		if u.dialogContent != nil || u.app.GetFocus() != tab.Editor.Area {
			t.Error("palette action must return to editor")
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
		event := tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModAlt)
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
		tab.Editor.Search = func() { tab.Status = "search called" }
		key := tcell.NewEventKey(tcell.KeyRune, '/', 0)
		if out := u.key(key); out != nil {
			tab.Editor.Handle(out)
		}
		if tab.Status != "search called" {
			t.Error("plain / must retain editor search")
		}
	})
}

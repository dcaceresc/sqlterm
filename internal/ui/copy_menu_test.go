package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/result"
)

func copyMenuUI(t *testing.T) *UI {
	t.Helper()
	u, _ := modeTestUI(t)
	s, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	u.active().Results = []*result.Store{s}
	u.showResults()
	u.app.SetFocus(u.table)
	dispatch(u, tcell.KeyCtrlC, 0)
	u.app.ForceDraw()
	if u.copyPopup == nil {
		t.Fatal("copy menu did not open")
	}
	return u
}

func TestCopyMenuCtrlCCancelsAndReopensWithoutExiting(t *testing.T) {
	u := copyMenuUI(t)
	// Returning the original Ctrl+C event lets tview stop its event loop.
	if k := u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)); k != nil {
		t.Fatal("Ctrl+C escaped to tview's exit handler")
	}
	if name, _ := u.pages.GetFrontPage(); name != "main" || u.copyPopup != nil || !u.table.HasFocus() || u.closing {
		t.Fatal("cancel must close the menu and restore results focus")
	}
	dispatch(u, tcell.KeyCtrlC, 0)
	if u.copyPopup == nil || u.closing {
		t.Fatal("copy menu did not reopen")
	}
	dispatch(u, tcell.KeyEscape, 0)
	if u.copyPopup != nil || !u.table.HasFocus() {
		t.Fatal("Escape must restore results focus")
	}
}

func TestCopyMenuClickOutsideClosesAndFocusesTarget(t *testing.T) {
	for _, panel := range []string{"editor", "objects"} {
		t.Run(panel, func(t *testing.T) {
			u := copyMenuUI(t)
			var target tview.Primitive = u.active().Editor.Area
			if panel == "objects" {
				target = u.objects
			}
			x, y, _, _ := target.GetRect()
			x, y = x+2, y+2
			if u.copyPopup.InRect(x, y) {
				t.Fatal("test click must be outside the popup")
			}
			event := tcell.NewEventMouse(x, y, tcell.Button1, 0)
			for _, action := range []tview.MouseAction{tview.MouseLeftDown, tview.MouseLeftUp, tview.MouseLeftClick} {
				forward, forwardAction := u.app.GetMouseCapture()(event, action)
				if forward != nil {
					u.pages.MouseHandler()(forwardAction, forward, func(p tview.Primitive) { u.app.SetFocus(p) })
				}
			}
			if name, _ := u.pages.GetFrontPage(); name != "main" || u.copyPopup != nil || !target.HasFocus() {
				t.Fatal("outside click must remove menu and focus clicked panel")
			}
			dispatch(u, tcell.KeyCtrlC, 0)
			if u.copyPopup == nil || u.closing {
				t.Fatal("copy menu must reopen after changing panels")
			}
		})
	}
}

func TestCtrlCWithoutResultsDoesNotExit(t *testing.T) {
	u, _ := modeTestUI(t)
	for range 2 {
		if k := u.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)); k != nil || u.closing {
			t.Fatal("Ctrl+C without results must not exit the application")
		}
	}
}

func TestCopyMenuPanelShortcutsDismissPopup(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyF6, tcell.KeyF7, tcell.KeyF8} {
		t.Run(tcell.KeyNames[key], func(t *testing.T) {
			u := copyMenuUI(t)
			dispatch(u, key, 0)
			target := tview.Primitive(u.table)
			if key == tcell.KeyF6 {
				target = u.search
			} else if key == tcell.KeyF7 {
				target = u.active().Editor.Area
			}
			if name, _ := u.pages.GetFrontPage(); name != "main" || u.copyPopup != nil || !target.HasFocus() {
				t.Fatal("panel shortcut left the copy menu visible or focused")
			}
		})
	}
}

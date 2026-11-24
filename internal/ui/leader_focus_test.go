package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestLeaderRetainsEditorFocus(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		e.SetText("SELECT 1;")
		for _, key := range []rune{' ', 'w'} {
			dispatchModified(u, tcell.KeyRune, key, 0)
			if u.dialogContent != nil || u.app.GetFocus() != e.Area || e.Cursor() != 0 || e.Area.GetText() != "SELECT 1;" || e.Mode != "NORMAL" {
				t.Errorf("%q must preserve editor focus/text/cursor without modal: dialog=%T", key, u.dialogContent)
				return
			}
		}
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		if u.app.GetFocus() != e.Area || e.Area.GetText() != "SELECT 1;" {
			t.Fatal("Escape must preserve editor")
		}
	})
}

func TestLeaderNativeActionInteractions(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		e.SetText("SELECT 1;")
		send := func(sequence string) {
			for _, r := range sequence {
				dispatchModified(u, tcell.KeyRune, r, 0)
			}
		}
		send("l")
		position := e.Cursor()
		send(" w")
		dispatchModified(u, tcell.KeyBackspace2, 0, 0)
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		if e.Cursor() != position || e.Area.GetText() != "SELECT 1;" || u.app.GetFocus() != e.Area || u.leaderPanel != nil {
			t.Error("leader navigation changed editor state")
		}
		for _, key := range []string{" ca", " cc", " cs"} {
			send(key)
			if e.Area.GetText() != "SELECT 1;" || u.leaderPanel == nil || u.dialogContent != nil || u.app.GetFocus() != e.Area {
				t.Errorf("%q must leave SQL, focus, and menu unchanged: %q", key, e.Area.GetText())
			}
			dispatchModified(u, tcell.KeyEscape, 0, 0)
		}
		previouslyEnabled := u.cfg.Editor.LineNumbers == nil || *u.cfg.Editor.LineNumbers
		send(" ul")
		if u.cfg.Editor.LineNumbers == nil || *u.cfg.Editor.LineNumbers == previouslyEnabled || u.dialogContent != nil || u.leaderPanel != nil {
			t.Error("line numbers did not toggle in place")
		}
		send(" ui")
		if u.dialogContent == nil || u.app.GetFocus() == e.Area || u.leaderPanel != nil {
			t.Error("language action did not focus its dialog")
		}
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		if u.app.GetFocus() != e.Area {
			t.Error("dialog dismissal did not restore editor focus")
		}
		send(" wr")
		if u.app.GetFocus() != u.table || u.leaderPanel != nil {
			t.Error("maximize results did not transfer focus")
		}
		u.app.SetFocus(e.Area)
		send(" ")
		u.app.SetFocus(u.search)
		send("z")
		if u.search.GetText() != "z" || u.leaderPanel != nil {
			t.Error("leader intercepted input after focus transfer")
		}
		u.app.SetFocus(e.Area)
		if u.leaderPanel != nil {
			t.Error("leader reappeared")
		}
	})
}

func TestLeaderGlobalHelpShortcut(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		dispatchModified(u, tcell.KeyRune, ' ', 0)
		dispatchModified(u, tcell.KeyF1, 0, 0)
		if u.leaderPanel != nil || u.dialogContent == nil {
			t.Error("F1 must close the command hints and open help")
		}
	})
}

func TestLeaderRendersOnRight(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		u, screen := modeTestUI(t)
		screen.SetSize(size[0], size[1])
		u.app.SetFocus(u.active().Editor.Area)
		u.leaderMenu()
		u.app.ForceDraw()
		found := false
		for y := 0; y < size[1]; y++ {
			var row strings.Builder
			for x := 0; x < size[0]; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				row.WriteRune(r)
			}
			if x := strings.Index(row.String(), "Files"); x >= 0 {
				found = true
				if len([]rune(row.String()[:x])) < size[0]/2 {
					t.Errorf("%v: command panel is not on the right", size)
				}
			}
		}
		if !found || u.dialogContent != nil || u.app.GetFocus() != u.active().Editor.Area {
			t.Errorf("%v: expected nonmodal right panel with editor focus", size)
		}
	}
}

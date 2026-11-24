package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestModalLineSelectionCopyPreservesVisualMode(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	for _, tt := range []struct{ sequence, want string }{
		{"V", "select ñ;\n"},
		{"ggVG", "select ñ;\nselect 2;"},
	} {
		t.Run(tt.sequence, func(t *testing.T) {
			u := runningNeovimUI(t)
			copied := make(chan string, 1)
			var from, to int
			const original = "select ñ;\nselect 2;"
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				tab.Editor.SetText(original)
				tab.Saved = original
				u.app.SetFocus(tab.Editor.Area)
				for _, r := range tt.sequence {
					dispatchModified(u, tcell.KeyRune, r, 0)
				}
				text, start, end := tab.Editor.Area.GetSelection()
				from, to = start, end
				if text != tt.want || tab.Editor.Mode != "VISUAL" {
					t.Errorf("selection before copy=%q mode=%s", text, tab.Editor.Mode)
				}
				u.writeClipboard = func(text string) error { copied <- text; return nil }
				dispatchModified(u, tcell.KeyCtrlC, 0, 0)
			})
			select {
			case got := <-copied:
				if got != tt.want {
					t.Errorf("clipboard=%q want=%q", got, tt.want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("selection did not reach clipboard")
			}
			waitUI(t, u, func() bool { return strings.HasPrefix(u.active().Status, "Copied SQL selection to clipboard") })
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				text, start, end := tab.Editor.Area.GetSelection()
				if tab.Editor.Mode != "VISUAL" || text != tt.want || start != from || end != to ||
					tab.Editor.Area.GetText() != original || tab.Saved != original || tab.Busy || tab.Session != nil {
					t.Error("copy changed mode, selection, SQL, saved state or query state")
				}
			})
		})
	}
}

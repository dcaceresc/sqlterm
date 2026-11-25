package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func testTechnicalMessagesCopy(t *testing.T) {
	u := runningUI(t, config.Config{})
	copied := make(chan string, 2)
	for i, want := range []string{"'=notice\nsecond line", "=notice\nsecond line"} {
		u.app.QueueUpdateDraw(func() {
			u.active().Messages = []string{"=notice", "second line"}
			u.writeClipboard = func(text string) error { copied <- text; return nil }
			u.app.SetFocus(u.table)
			dispatchModified(u, tcell.KeyRune, ' ', 0)
			dispatchModified(u, tcell.KeyRune, 'm', 0)
			panel, ok := u.dialogContent.(*tview.Flex)
			if !ok || panel.GetItemCount() != 2 || panel.GetItem(0).(*tview.TextView).GetText(false) != "=notice\nsecond line" {
				t.Error("Results menu did not open the read-only Technical Messages dialog")
				return
			}
			buttons := panel.GetItem(1).(*tview.Form)
			buttons.GetButton(i).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		})
		if got := receiveCopy(t, copied); got != want {
			t.Fatalf("copied messages %q, want %q", got, want)
		}
		waitUI(t, u, func() bool { return !u.active().Busy })
		u.app.QueueUpdateDraw(func() {
			dispatchModified(u, tcell.KeyEscape, 0, 0)
			if u.dialogContent != nil || !u.table.HasFocus() {
				t.Error("Escape did not restore focus to Results")
			}
		})
	}
}

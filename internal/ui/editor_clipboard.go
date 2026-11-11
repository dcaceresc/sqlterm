package ui

import (
	"fmt"

	"sqlterm/internal/clipboardtext"
)

// copySQLSelection is independent of connection work: copying the editor must
// neither acquire Tab.Busy nor change a query's cancellation/pending operation.
func (u *UI) copySQLSelection() {
	if u.closing {
		return
	}
	tab := u.active()
	text, _, _ := tab.Editor.Area.GetSelection()
	if text == "" {
		u.message(u.tr("Select SQL text before copying"))
		return
	}
	if u.copyingSQL {
		u.message(u.tr("SQL clipboard copy already in progress"))
		return
	}
	// Capture immutable text and the writer on the UI loop, before tab switches
	// or edits. Only the completion callback touches UI-owned state.
	writer := u.writeClipboard
	u.copyingSQL = true
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		escaped, err := clipboardtext.Write(writer, text)
		u.post(func() {
			u.copyingSQL = false
			if u.closing {
				return
			}
			for _, current := range u.tabs {
				if current != tab {
					continue
				}
				if err != nil {
					tab.Status = fmt.Sprintf(u.tr("Could not copy SQL: %s"), err.Error())
				} else {
					tab.Status = u.tr("Copied SQL selection to clipboard")
					if escaped {
						tab.Status += u.tr(" (NUL escaped as \\0)")
					}
				}
				u.refreshBars()
				return
			}
		})
	}()
}

package ui

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/clipboardtext"
	"sqlterm/internal/result"
)

func (u *UI) inspectText(title, text string) {
	view := tview.NewTextView().SetText(text).SetWrap(false).SetScrollable(true)
	view.SetBorder(true).SetTitle(u.tr(" ") + tview.Escape(title) + " ")
	buttons := tview.NewForm().SetHorizontal(true)
	copyValue := func(value string) {
		t := u.active()
		u.task(t, "Copy full value", func(ctx context.Context) (func(), error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			escaped, err := clipboardtext.Write(u.writeClipboard, value)
			return func() {
				if err == nil {
					t.Status = "Copied full value"
					if escaped {
						t.Status += ` (NUL escaped as \0)`
					}
				}
			}, err
		})
	}
	buttons.AddButton(u.tr("Copy safe for spreadsheets"), func() { copyValue(result.SpreadsheetSafeText(text)) }).
		AddButton(u.tr("Copy literal · not safe for untrusted spreadsheet data"), func() { copyValue(text) }).
		AddButton(u.tr("Close"), u.dismiss)
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(view, 0, 1, true).AddItem(buttons, 3, 0, false)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		_, index := buttons.GetFocusedItemIndex()
		switch k.Key() {
		case tcell.KeyTab:
			switch {
			case view.HasFocus():
				u.app.SetFocus(buttons)
			case buttons.HasFocus() && index == buttons.GetButtonCount()-1:
				u.app.SetFocus(view)
			default:
				return k
			}
		case tcell.KeyBacktab:
			switch {
			case view.HasFocus():
				buttons.SetFocus(buttons.GetButtonCount() - 1)
				u.app.SetFocus(buttons)
			case buttons.HasFocus() && index <= 0:
				u.app.SetFocus(view)
			default:
				return k
			}
		default:
			return k
		}
		return nil
	})
	u.popup(panel, 100, 26)
}

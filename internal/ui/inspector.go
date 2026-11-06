package ui

import (
	"context"
	"io"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/clipboardtext"
	"sqlterm/internal/result"
)

// cellRecordWriter writes one clipboard record for the inspected cell.
// spreadsheetSafe selects the default safe mode or the explicit literal mode.
type cellRecordWriter func(ctx context.Context, w io.Writer, spreadsheetSafe bool) error

// inspectText shows read-only text without cell context, so it offers only the
// value copy buttons.
func (u *UI) inspectText(title, text string) {
	u.inspect(title, text, "", nil)
}

// inspectCellText also offers copying the inspected cell as one TSV record:
// column name, tab, value. A call site without cell context passes no header and
// a nil writer and then gets only the value buttons, never a wrong cell copy.
func (u *UI) inspectCellText(title, header string, write cellRecordWriter, text string) {
	u.inspect(title, text, header, write)
}

func (u *UI) inspect(title, text, header string, write cellRecordWriter) {
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
	headerButtons := tview.NewForm().SetHorizontal(true)
	// An empty column name is still a real header; only a missing writer means
	// the dialog has no cell context (for example technical messages).
	hasHeader := write != nil
	if hasHeader {
		copyRecord := func(spreadsheetSafe bool) {
			t := u.active()
			if t.Busy {
				u.message(u.tr("Wait until execution finishes"))
				return
			}
			u.task(t, "Copy cell with header", func(ctx context.Context) (func(), error) {
				var b strings.Builder
				if err := write(ctx, &b, spreadsheetSafe); err != nil {
					return nil, err
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				escaped, err := clipboardtext.Write(u.writeClipboard, b.String())
				return func() {
					if err == nil {
						t.Status = u.tr("Copied 1 cell with header to clipboard")
						if escaped {
							t.Status += ` (NUL escaped as \0)`
						}
					}
				}, err
			})
		}
		headerButtons.AddButton(u.tr("Copy column name and value"), func() { copyRecord(true) }).
			AddButton(u.tr("Copy name and value literally · not safe for untrusted spreadsheet data"), func() { copyRecord(false) })
		// Narrow terminals wrap the literal variant onto a second line; the form
		// keeps its three rows and scrolls the focused button into view.
		panel.AddItem(headerButtons, 3, 0, false)
	}
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		_, valueIndex := buttons.GetFocusedItemIndex()
		_, headerIndex := headerButtons.GetFocusedItemIndex()
		if k.Key() == tcell.KeyTab {
			switch {
			case view.HasFocus():
				u.app.SetFocus(buttons)
			case buttons.HasFocus() && valueIndex == buttons.GetButtonCount()-1:
				if hasHeader {
					u.app.SetFocus(headerButtons)
				} else {
					u.app.SetFocus(view)
				}
			case hasHeader && headerButtons.HasFocus() && headerIndex == headerButtons.GetButtonCount()-1:
				u.app.SetFocus(view)
			default:
				return k
			}
			return nil
		}
		if k.Key() == tcell.KeyBacktab {
			switch {
			case view.HasFocus():
				if hasHeader {
					headerButtons.SetFocus(headerButtons.GetButtonCount() - 1)
					u.app.SetFocus(headerButtons)
				} else {
					buttons.SetFocus(buttons.GetButtonCount() - 1)
					u.app.SetFocus(buttons)
				}
			case hasHeader && headerButtons.HasFocus() && headerIndex <= 0:
				buttons.SetFocus(buttons.GetButtonCount() - 1)
				u.app.SetFocus(buttons)
			case buttons.HasFocus() && valueIndex <= 0:
				u.app.SetFocus(view)
			default:
				return k
			}
			return nil
		}
		return k
	})
	u.popup(panel, 100, 26)
}

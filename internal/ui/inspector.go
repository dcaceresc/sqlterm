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
	plain := tview.NewTextView().SetText(text).SetWrap(false).SetScrollable(true)
	plain.SetBorder(true).SetTitle(u.tr(" ") + tview.Escape(title) + " ")
	var view tview.Primitive = plain
	buttons := tview.NewForm().SetHorizontal(true)
	copyText := func(value, action, status string) {
		t := u.active()
		u.task(t, action, func(ctx context.Context) (func(), error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			escaped, err := clipboardtext.Write(u.writeClipboard, value)
			return func() {
				if err == nil {
					t.Status = status
					if escaped {
						t.Status += ` (NUL escaped as \0)`
					}
				}
			}, err
		})
	}
	copyValue := func(value string) { copyText(value, "Copy full value", "Copied full value") }
	var cell *cellTextView
	if write != nil {
		area := tview.NewTextArea().SetText(text, false).SetWrap(false)
		u.style(area)
		area.SetBorder(true).SetTitle(plain.GetTitle() + "· " + u.tr("v/V select · / find · Ctrl+C copy") + " ")
		cell = &cellTextView{TextArea: area, text: text, copyValue: func(value string) {
			copyText(value, u.tr("Copy selected text"), u.tr("Copied selected text"))
		}}
		view = cell
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
	if cell != nil {
		search := &cellTextSearch{InputField: tview.NewInputField().SetLabel(u.tr("Find: "))}
		searchOpen := false
		search.close = func() {
			panel.RemoveItem(search)
			searchOpen = false
			u.app.SetFocus(cell)
		}
		cell.openSearch = func() {
			if !searchOpen {
				searchOpen = true
				search.SetLabel(u.tr("Find: "))
				panel.AddItem(search, 1, 0, false)
				u.style(search.InputField)
			}
			u.app.SetFocus(search)
		}
		search.SetDoneFunc(func(key tcell.Key) {
			if key != tcell.KeyEnter {
				search.close()
				return
			}
			query := search.GetText()
			if query == "" {
				search.close()
				return
			}
			_, _, end := cell.GetSelection()
			match := strings.Index(text[end:], query)
			if match >= 0 {
				match += end
			} else {
				match = strings.Index(text, query)
			}
			if match < 0 {
				search.SetLabel(u.tr("No matches: "))
				return
			}
			cell.Select(match, match+len(query))
			search.close()
		})
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

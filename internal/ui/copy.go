package ui

import (
	"context"
	"fmt"
	"github.com/rivo/tview"
	"sqlterm/internal/clipboardtext"
	"strings"
)

func (u *UI) copyMenu() {
	if u.currentStore() == nil {
		u.message("No results to copy")
		return
	}
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true).SetTitle(" Copy results ")
	for _, opt := range []struct {
		label, scope string
		headers      bool
	}{{"Cell", "cell", false}, {"Row without headers", "row", false}, {"Row with headers", "row", true}, {"All results without headers", "all", false}, {"All results with headers", "all", true}} {
		o := opt
		l.AddItem(o.label, "", 0, func() { u.dismiss(); u.app.SetFocus(u.table); u.copyResults(o.scope, o.headers) })
	}
	u.popup(l, 70, 9)
}
func (u *UI) copyResults(scope string, headers bool) {
	t := u.active()
	store := u.currentStore()
	if store == nil {
		u.message("No results to copy")
		return
	}
	if t.Busy {
		u.message("Wait until execution finishes")
		return
	}
	row, col := u.table.GetSelection()
	var rows []int
	if scope != "all" {
		if row < 1 || row > store.Count() {
			u.message("Select a result cell first")
			return
		}
		n := row - 1
		if len(t.Order) > 0 {
			n = t.Order[n]
		}
		rows = []int{n}
	} else if t.Order != nil {
		rows = append([]int{}, t.Order...)
	}
	u.task(t, "Copy results", func(ctx context.Context) (func(), error) {
		var text string
		if scope == "cell" {
			r, err := store.Row(rows[0])
			if err != nil {
				return nil, err
			}
			if col < 0 || col >= len(r) {
				return nil, fmt.Errorf("select a column first")
			}
			text = r[col].Text
			if r[col].Null {
				text = `\N`
			}
		} else {
			var b strings.Builder
			if err := store.WriteTSV(ctx, &b, rows, headers); err != nil {
				return nil, err
			}
			text = b.String()
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		escaped, err := clipboardtext.Write(u.writeClipboard, text)
		if err != nil {
			return nil, fmt.Errorf("clipboard: %w", err)
		}
		return func() {
			t.Status = "Copied results to clipboard"
			if escaped {
				t.Status += ` (NUL escaped as \0)`
			}
		}, nil
	})
}

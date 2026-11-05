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
		u.message(u.tr("No results to copy"))
		return
	}
	l := tview.NewList().ShowSecondaryText(false)
	t := u.active()
	s := u.currentStore()
	source := t.Profile.Database
	if e := t.Execution; e != nil {
		source = fmt.Sprintf("#%d %s / %s", e.ID, e.Source, e.Database)
	}
	l.SetBorder(true).SetTitle(u.tr(" Copy · ") + tview.Escape(source) + " ")
	for _, opt := range []struct {
		label, scope string
		headers      bool
	}{{"Cell · 1 value", "cell", false}, {"Row · 1 row without headers", "row", false}, {"Row · 1 row with headers", "row", true}, {fmt.Sprintf(u.tr("Received results · %d rows without headers"), s.Count()), "all", false}, {fmt.Sprintf(u.tr("Received results · %d rows with headers"), s.Count()), "all", true}} {
		o := opt
		l.AddItem(u.tr(o.label), "", 0, func() { u.dismiss(); u.app.SetFocus(u.table); u.copyResults(o.scope, o.headers) })
	}
	if t.CopyAnchor > 0 {
		row, _ := u.table.GetSelection()
		selectedCount := max(row, t.CopyAnchor) - min(row, t.CopyAnchor) + 1
		for _, headers := range []bool{false, true} {
			h := headers
			label := "Selection without headers"
			if h {
				label = "Selection with headers"
			}
			l.AddItem(fmt.Sprintf("%s · %d", u.tr(label), selectedCount), "", 0, func() { u.dismiss(); u.app.SetFocus(u.table); u.copyResults("selection", h) })
		}
	}
	u.popup(l, 90, 4+l.GetItemCount())
	u.copyPopup = l
}
func (u *UI) copyResults(scope string, headers bool) {
	t := u.active()
	store := u.currentStore()
	if store == nil {
		u.message(u.tr("No results to copy"))
		return
	}
	if t.Busy {
		u.message(u.tr("Wait until execution finishes"))
		return
	}
	row, col := u.table.GetSelection()
	var rows []int
	if scope == "selection" {
		if t.CopyAnchor < 1 || row < 1 {
			u.message(u.tr("Press v in results to start a row selection"))
			return
		}
		for n := min(t.CopyAnchor, row) - 1; n < max(t.CopyAnchor, row); n++ {
			if n < 0 || n >= store.Count() {
				continue
			}
			id := n
			if len(t.Order) > 0 {
				id = t.Order[n]
			}
			rows = append(rows, id)
		}
	} else if scope != "all" {
		if row < 1 || row > store.Count() {
			u.message(u.tr("Select a result cell first"))
			return
		}
		n := row - 1
		if len(t.Order) > 0 {
			if n >= len(t.Order) {
				u.message(u.tr("Select a result row first"))
				return
			}
			n = t.Order[n]
		}
		rows = []int{n}
	} else if t.Order != nil {
		rows = append([]int{}, t.Order...)
	}
	count := store.Count()
	if scope != "all" {
		count = len(rows)
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
			t.Status = fmt.Sprintf(u.tr("Copied %d rows to clipboard"), count)
			if scope == "cell" {
				t.Status = u.tr("Copied 1 cell to clipboard")
			}
			if escaped {
				t.Status += ` (NUL escaped as \0)`
			}
		}, nil
	})
}

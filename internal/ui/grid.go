package ui

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/result"
	"strings"
)

type gridContent struct {
	tview.TableContentReadOnly
	store   *result.Store
	order   []int
	palette *Palette
}

func (g *gridContent) GetRowCount() int {
	if g.store == nil {
		return 0
	}
	return g.store.Count() + 1
}
func (g *gridContent) GetColumnCount() int {
	if g.store == nil {
		return 0
	}
	return len(g.store.Columns)
}
func (g *gridContent) GetCell(row, col int) *tview.TableCell {
	p := resolveTheme("")
	if g.palette != nil {
		p = *g.palette
	}
	if g.store == nil || col < 0 || col >= len(g.store.Columns) {
		return nil
	}
	if row == 0 {
		return tview.NewTableCell(tview.Escape(g.store.Columns[col])).SetTextColor(p.Accent).SetBackgroundColor(p.Background).SetSelectable(false).SetMaxWidth(36)
	}
	n := row - 1
	if len(g.order) > 0 {
		if n >= len(g.order) {
			return nil
		}
		n = g.order[n]
	}
	r, e := g.store.Row(n)
	if e != nil {
		return tview.NewTableCell("[read error]")
	}
	if col >= len(r) {
		return nil
	}
	v := r[col]
	if v.Null {
		return tview.NewTableCell("NULL").SetTextColor(p.Muted).SetBackgroundColor(p.Background).SetMaxWidth(36)
	}
	text := strings.ReplaceAll(strings.ReplaceAll(v.Text, "\r", "\\r"), "\n", "\\n")
	return tview.NewTableCell(tview.Escape(text)).SetTextColor(p.Foreground).SetBackgroundColor(p.Background).SetMaxWidth(36)
}
func (u *UI) currentStore() *result.Store {
	t := u.active()
	if len(t.Results) == 0 || t.SelectedSet >= len(t.Results) {
		return nil
	}
	return t.Results[t.SelectedSet]
}
func (u *UI) showResults() {
	t := u.active()
	s := u.currentStore()
	u.table.SetContent(&gridContent{store: s, order: t.Order, palette: &u.palette})
	u.table.SetOffset(0, 0).Select(1, 0)
	if s == nil {
		u.table.SetTitle(" Results ")
	} else {
		u.table.SetTitle(fmt.Sprintf(" Results · set %d/%d · %d rows ", t.SelectedSet+1, len(t.Results), s.Count()))
	}
}
func (u *UI) viewCell(row, col int) {
	s := u.currentStore()
	if s == nil || row < 1 {
		return
	}
	n := row - 1
	if len(u.active().Order) > 0 {
		if n >= len(u.active().Order) {
			return
		}
		n = u.active().Order[n]
	}
	r, e := s.Row(n)
	if e != nil {
		u.message(e.Error())
		return
	}
	if col >= len(r) {
		return
	}
	v := r[col]
	text := v.Text
	if v.Null {
		text = "SQL NULL"
	}
	u.notice(s.Columns[col], text)
}
func (u *UI) gridKey(k *tcell.EventKey) *tcell.EventKey {
	if k.Key() != tcell.KeyRune {
		return k
	}
	t := u.active()
	switch k.Rune() {
	case 'y', 'Y':
		u.copyResults("all", k.Rune() == 'Y')
		return nil
	case 'c':
		u.copyResults("cell", false)
		return nil
	case 'r', 'R':
		u.copyResults("row", k.Rune() == 'R')
		return nil
	case 'm':
		u.notice("Messages", strings.Join(t.Messages, "\n"))
		return nil
	case '[', ']':
		if len(t.Results) > 0 {
			d := 1
			if k.Rune() == '[' {
				d = -1
			}
			t.SelectedSet = (t.SelectedSet + d + len(t.Results)) % len(t.Results)
			t.Order = nil
			u.showResults()
		}
		return nil
	case 'e':
		u.export()
		return nil
	case 's', 'S':
		s := u.currentStore()
		if s == nil {
			return nil
		}
		if t.Busy {
			u.message("Wait until execution finishes")
			return nil
		}
		_, col := u.table.GetSelection()
		desc := k.Rune() == 'S'
		u.task(t, "Sort results", func(ctx context.Context) (func(), error) {
			order, e := s.OrderContext(ctx, col, desc)
			if e != nil {
				return nil, e
			}
			return func() {
				t.Order = order
				if u.active() == t {
					u.showResults()
				}
			}, nil
		})
		return nil
	case '/':
		u.prompt("Find in results", "", false, func(text string) {
			s := u.currentStore()
			if s == nil {
				return
			}
			row, _ := u.table.GetSelection()
			u.task(t, "Search results", func(ctx context.Context) (func(), error) {
				n, e := s.Find(ctx.Done(), text, row)
				if e != nil {
					return nil, e
				}
				return func() {
					if n < 0 {
						t.Status = "No matching row"
						return
					}
					display := n
					if len(t.Order) > 0 {
						for i, id := range t.Order {
							if id == n {
								display = i
								break
							}
						}
					}
					if u.active() == t {
						u.table.Select(display+1, 0)
						u.app.SetFocus(u.table)
					}
				}, e
			})
		})
		return nil
	}
	return k
}
func (u *UI) export() {
	t := u.active()
	s := u.currentStore()
	if s == nil {
		u.message("No results to export")
		return
	}
	if t.Busy {
		u.message("Wait until execution finishes")
		return
	}
	u.prompt("Export complete set to new .csv or .json file", "results.csv", false, func(path string) {
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		u.task(t, "Export", func(ctx context.Context) (func(), error) { return nil, s.ExportContext(ctx, path, format) })
	})
}

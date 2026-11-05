package ui

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/result"
	"strconv"
	"strings"
)

type gridContent struct {
	tview.TableContentReadOnly
	store       *result.Store
	order       []int
	palette     *Palette
	widths      map[int]int
	selectedRow func() int
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
	width := 36
	if g.widths[col] > 0 {
		width = g.widths[col]
	}
	if row == 0 {
		return tview.NewTableCell(tview.Escape(g.store.Columns[col])).SetTextColor(p.Accent).SetBackgroundColor(p.Background).SetSelectable(false).SetMaxWidth(width)
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
	bg := p.Background
	if g.selectedRow != nil && g.selectedRow() == row {
		bg = p.Surface
	}
	if v.Null {
		return tview.NewTableCell("NULL").SetTextColor(p.Muted).SetBackgroundColor(bg).SetMaxWidth(width)
	}
	text := strings.NewReplacer("\r", `\r`, "\n", `\n`, "\t", `\t`, "\x00", `\0`).Replace(v.Text)
	if text == "" {
		text = `""`
	}
	cell := tview.NewTableCell(tview.Escape(text)).SetTextColor(p.Foreground).SetBackgroundColor(bg).SetMaxWidth(width)
	if _, err := strconv.ParseFloat(v.Text, 64); err == nil && strings.TrimSpace(v.Text) == v.Text {
		cell.SetAlign(tview.AlignRight)
	}
	return cell
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
	u.table.SetContent(&gridContent{store: s, order: t.Order, palette: &u.palette, widths: t.ColumnWidths, selectedRow: func() int { r, _ := u.table.GetSelection(); return r }})
	u.table.SetOffset(0, 0).Select(1, 0)
	u.refreshResultInfo()
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
	if col < 0 || col >= len(r) {
		return
	}
	v := r[col]
	text := v.Text
	if v.Null {
		text = "SQL NULL"
	}
	u.inspectText(fmt.Sprintf(u.tr("Cell · row %d · column %d: %s"), row, col+1, s.Columns[col]), text)
}
func (u *UI) gridKey(k *tcell.EventKey) *tcell.EventKey {
	if k.Key() != tcell.KeyRune {
		return k
	}
	t := u.active()
	switch k.Rune() {
	case '+', '-':
		_, col := u.table.GetSelection()
		if t.ColumnWidths == nil {
			t.ColumnWidths = map[int]int{}
		}
		w := t.ColumnWidths[col]
		if w == 0 {
			w = 36
		}
		if k.Rune() == '+' {
			w += 4
		} else {
			w = max(8, w-4)
		}
		t.ColumnWidths[col] = min(160, w)
		u.table.SetContent(&gridContent{store: u.currentStore(), order: t.Order, palette: &u.palette, widths: t.ColumnWidths, selectedRow: func() int { r, _ := u.table.GetSelection(); return r }})
		return nil
	case 'v':
		if t.CopyAnchor > 0 {
			t.CopyAnchor = 0
		} else {
			r, _ := u.table.GetSelection()
			t.CopyAnchor = r
		}
		u.refreshResultInfo()
		return nil
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
		u.showMessages()
		return nil
	case '[', ']':
		if len(t.Results) > 0 {
			d := 1
			if k.Rune() == '[' {
				d = -1
			}
			t.SelectedSet = (t.SelectedSet + d + len(t.Results)) % len(t.Results)
			t.Order = nil
			t.SortDescription = ""
			t.CopyAnchor = 0
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
			u.message(u.tr("Wait until execution finishes"))
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
				direction := "ASC"
				if desc {
					direction = "DESC"
				}
				t.SortDescription = fmt.Sprintf(u.tr("local received rows: %s %s (lexical)"), s.Columns[col], direction)
				if u.active() == t {
					u.showResults()
				}
			}, nil
		})
		return nil
	case '/':
		u.prompt(u.tr("Find in results"), "", false, func(text string) {
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
		u.message(u.tr("No results to export"))
		return
	}
	if t.Busy {
		u.message(u.tr("Wait until execution finishes"))
		return
	}
	u.prompt(u.tr("Export complete set to new .csv or .json file"), "results.csv", false, func(path string) {
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		u.task(t, "Export", func(ctx context.Context) (func(), error) { return nil, s.ExportContext(ctx, path, format) })
	})
}

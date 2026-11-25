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
	store     *result.Store
	order     []int
	palette   *Palette
	widths    map[int]int
	selection func() (int, int)
	marked    func(row, col int) bool
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
	return len(g.store.Columns) + 1
}
func (g *gridContent) GetCell(row, col int) *tview.TableCell {
	p := resolveTheme("")
	if g.palette != nil {
		p = *g.palette
	}
	if g.store == nil || col < 0 || col > len(g.store.Columns) {
		return nil
	}
	bg := p.Background
	if g.selection != nil {
		r, c := g.selection()
		if (r == row && c == col) || (r == 0 && c > 0 && c == col) || (c == 0 && r > 0 && r == row) {
			bg = p.Selection
		}
	}
	if g.marked != nil && g.marked(row, col) {
		bg = p.Selection
	}
	if col == 0 {
		label := "#"
		if row > 0 {
			label = strconv.Itoa(row)
		}
		return tview.NewTableCell(label).SetAlign(tview.AlignRight).SetTextColor(p.Accent).SetBackgroundColor(bg).SetSelectable(row > 0)
	}
	col--
	width := 36
	if g.widths[col] > 0 {
		width = g.widths[col]
	}
	if row == 0 {
		return tview.NewTableCell(tview.Escape(g.store.Columns[col])).SetTextColor(p.Accent).SetBackgroundColor(bg).SetMaxWidth(width)
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
	u.setGridContent(t, s)
	u.table.SetOffset(0, 0).Select(1, 1)
	u.refreshResultInfo()
}
func (u *UI) resultPreviewLocked() bool {
	t := u.active()
	if t.Execution == nil {
		return false
	}
	switch t.Execution.Phase {
	case "Running", "Receiving rows", "Cancelling":
		return true
	}
	return false
}

func (u *UI) gridKey(k *tcell.EventKey) *tcell.EventKey {
	if u.resultPreviewLocked() {
		return nil
	}
	if u.selectGridKey(k) {
		return nil
	}
	if k.Key() == tcell.KeyCtrlN {
		return tcell.NewEventKey(tcell.KeyDown, 0, k.Modifiers())
	}
	if k.Key() != tcell.KeyRune {
		return k
	}
	t := u.active()
	switch k.Rune() {
	case '+', '-':
		_, col := u.table.GetSelection()
		if col == 0 {
			return nil
		}
		col--
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
		u.setGridContent(t, u.currentStore())
		return nil
	case 'v':
		u.toggleGridRowRange()
		return nil
	case 'y':
		u.copyGridSelection()
		return nil
	case 'Y':
		u.copyGridHeaders(false)
		return nil
	case 'c':
		if t.Busy {
			u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
			return nil
		}
		u.copyResults("cell", false)
		return nil
	case 'r', 'R':
		if t.Busy {
			u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
			return nil
		}
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
			t.clearGridSelection()
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
			u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
			return nil
		}
		_, col := u.table.GetSelection()
		if col == 0 {
			return nil
		}
		col--
		desc := k.Rune() == 'S'
		u.task(t, "Sort results", func(ctx context.Context) (func(), error) {
			order, e := s.OrderContext(ctx, col, desc)
			if e != nil {
				return nil, e
			}
			return func() {
				t.clearGridSelection()
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
						u.table.Select(display+1, 1)
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
		u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
		return
	}
	selection, hasSelection := u.explicitGridCopyScope()
	all := gridCopyScope{first: gridPoint{1, 1}, last: gridPoint{s.Count(), len(s.Columns)}}
	useSelection := hasSelection
	order := t.Order
	menu := tview.NewList().ShowSecondaryText(false)
	scopeLabel := func() string {
		if useSelection {
			return "Export scope: Selection"
		}
		return "Export scope: Entire result"
	}
	refreshScope := func() { menu.SetTitle(" " + u.tr(scopeLabel()) + " ") }
	open := func(initial, format string, spreadsheetSafe bool) {
		scope := all
		if useSelection {
			scope = selection
		}
		u.exportPrompt(t, s, scope, order, scopeLabel(), initial, format, spreadsheetSafe)
	}
	menu.SetBorder(true)
	refreshScope()
	menu.AddItem(u.tr("Spreadsheet-safe CSV · prefixes formula-like text"), "", 0, func() {
		open("results.csv", "csv", true)
	})
	menu.AddItem(u.tr("Literal CSV · not safe for untrusted spreadsheet data"), "", 0, func() {
		open("results.csv", "csv", false)
	})
	menu.AddItem(u.tr("JSON · literal values unchanged"), "", 0, func() {
		open("results.json", "json", false)
	})
	if hasSelection {
		menu.AddItem(u.tr("Change scope: Entire result"), "", 0, func() {
			useSelection = !useSelection
			refreshScope()
			label := "Change scope: Entire result"
			if !useSelection {
				label = "Change scope: Selection"
			}
			menu.SetItemText(3, u.tr(label), "")
		})
	}
	menu.AddItem(u.tr("Excel (.xlsx) · values stored as text"), "", 0, func() {
		open("results.xlsx", "xlsx", false)
	})
	u.popup(menu, 82, 8)
}

func (u *UI) exportPrompt(t *Tab, s *result.Store, scope gridCopyScope, order []int, title, initial, format string, spreadsheetSafe bool) {
	u.pickFile(initial, u.lastExportDir, true, "Export", title, func(path string) {
		u.task(t, "Export", func(ctx context.Context) (func(), error) {
			var rows []int
			start, end := scope.first.row-1, scope.last.row
			switch {
			case order != nil:
				if start < 0 || end < start || end > len(order) {
					return nil, fmt.Errorf("export: result order no longer matches scope")
				}
				rows = order[start:end]
			case start != 0 || end != s.Count():
				rows = make([]int, end-start)
				for i := range rows {
					rows[i] = start + i
				}
			}
			if err := s.ExportProjectionContext(ctx, path, format, spreadsheetSafe, rows, scope.first.col-1, scope.last.col); err != nil {
				return nil, err
			}
			return func() {
				u.lastExportDir = filepath.Dir(path)
				if err := writeCache(u.configPath+".export-dir.json", u.lastExportDir); err != nil {
					t.Status += " · " + u.tr("Could not remember export folder") + ": " + err.Error()
				}
			}, nil
		})
	})
}

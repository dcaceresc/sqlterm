package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/result"
)

type gridPoint struct{ row, col int }

// Los límites son coordenadas visibles de datos; la fila y columna cero no se seleccionan.
type gridSelection struct {
	anchor, end gridPoint
	extending   bool
	singleCell  bool
	mouseRange  bool
}

func (s gridSelection) contains(row, col int) bool {
	return s.anchor.row > 0 && s.anchor.col > 0 &&
		row >= min(s.anchor.row, s.end.row) && row <= max(s.anchor.row, s.end.row) &&
		col >= min(s.anchor.col, s.end.col) && col <= max(s.anchor.col, s.end.col)
}

func (t *Tab) clearGridSelection() {
	t.CopyAnchor = 0
	t.gridSelection = gridSelection{}
}

func (u *UI) setGridContent(t *Tab, s *result.Store) {
	u.table.SetContent(&gridContent{
		store: s, order: t.Order, palette: &u.palette, widths: t.ColumnWidths,
		selection: u.table.GetSelection,
		marked: func(row, col int) bool {
			if t.CopyAnchor > 0 && col > 0 {
				focus, _ := u.table.GetSelection()
				return focus > 0 && row >= min(t.CopyAnchor, focus) && row <= max(t.CopyAnchor, focus)
			}
			return t.gridSelection.contains(row, col)
		},
	})
}

func (u *UI) canSelectGrid() bool {
	s := u.currentStore()
	return s != nil && len(s.Columns) > 0 && s.Count() > 0 && !u.resultPreviewLocked()
}

func (u *UI) markGridPoint(row, col int) {
	if !u.canSelectGrid() || row < 0 || col < 0 || row+col == 0 {
		return
	}
	s := u.currentStore()
	if row > s.Count() || col > len(s.Columns) {
		return
	}
	end := gridPoint{max(1, row), max(1, col)}
	if row == 0 {
		end.row = s.Count()
	}
	if col == 0 {
		end.col = len(s.Columns)
	}
	u.setGridSelection(gridPoint{max(1, row), max(1, col)}, end, false)
	u.active().gridSelection.singleCell = row > 0 && col > 0
}

func (u *UI) setGridSelection(anchor, end gridPoint, extending bool) {
	t := u.active()
	t.CopyAnchor = 0
	t.gridSelection = gridSelection{anchor: anchor, end: end, extending: extending}
	u.refreshResultInfo()
}

func (u *UI) gridFocusChanged(row, col int) {
	t := u.active()
	if t != nil && t.gridSelection.mouseRange && t.gridSelection.end != (gridPoint{row, col}) {
		t.clearGridSelection()
	}
	u.refreshResultInfo()
}

func (u *UI) toggleGridRowRange() {
	t := u.active()
	if t.CopyAnchor > 0 {
		t.CopyAnchor = 0
	} else if row, _ := u.table.GetSelection(); row > 0 && u.canSelectGrid() {
		t.clearGridSelection()
		t.CopyAnchor = row
	}
	u.refreshResultInfo()
}

func (u *UI) selectAllGrid() {
	if !u.canSelectGrid() {
		return
	}
	s := u.currentStore()
	u.setGridSelection(gridPoint{1, 1}, gridPoint{s.Count(), len(s.Columns)}, false)
	u.app.SetFocus(u.table)
}

func (u *UI) selectGridKey(k *tcell.EventKey) bool {
	t := u.active()
	r, c := u.table.GetSelection()
	if k.Modifiers() == tcell.ModShift {
		var dr, dc int
		switch k.Key() {
		case tcell.KeyUp:
			dr = -1
		case tcell.KeyDown:
			dr = 1
		case tcell.KeyLeft:
			dc = -1
		case tcell.KeyRight:
			dc = 1
		default:
			return false
		}
		if u.canSelectGrid() {
			s := u.currentStore()
			anchor := gridPoint{max(1, r), max(1, c)}
			if prev := t.gridSelection; prev.extending && prev.end == (gridPoint{r, c}) {
				anchor = prev.anchor
			}
			end := gridPoint{min(s.Count(), max(1, r+dr)), min(len(s.Columns), max(1, c+dc))}
			u.setGridSelection(anchor, end, true)
			u.table.Select(end.row, end.col)
		}
		return true
	}
	switch k.Key() {
	case tcell.KeyEnter:
		u.markGridPoint(r, c)
		return true
	case tcell.KeyCtrlA:
		u.selectAllGrid()
		return true
	case tcell.KeyEscape:
		if t.CopyAnchor > 0 || t.gridSelection.anchor.row > 0 {
			t.clearGridSelection()
			u.refreshResultInfo()
			return true
		}
	}
	return false
}

func (u *UI) gridMouseCapture() func(*tcell.EventMouse, tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	var owner *Tab
	var store *result.Store
	var start gridPoint
	var dragged bool
	return func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if action == tview.MouseLeftUp {
			defer func() { owner, store = nil, nil }()
		}
		if action == tview.MouseLeftDown {
			owner, store, dragged = nil, nil, false
		}
		if action == tview.MouseLeftClick && dragged {
			dragged = false
			return nil, action
		}
		switch action {
		case tview.MouseMove:
			if owner == nil || event.Buttons()&tcell.Button1 == 0 {
				return event, action
			}
		case tview.MouseLeftUp:
			if owner == nil {
				return event, action
			}
		case tview.MouseLeftDown, tview.MouseLeftClick:
		default:
			return event, action
		}
		if u.dialogContent != nil || !u.canSelectGrid() {
			return event, action
		}
		x, y := event.Position()
		left, top, width, height := u.table.GetInnerRect()
		if x < left || y < top || x >= left+width || y >= top+height {
			return event, action
		}
		r, c := u.table.CellAt(x, y)
		p := gridPoint{r, c}
		s := u.currentStore()
		switch action {
		case tview.MouseLeftDown:
			if r > 0 && c > 0 && r <= s.Count() && c <= len(s.Columns) {
				owner, store, start = u.active(), s, p
			}
		case tview.MouseMove, tview.MouseLeftUp:
			if owner == u.active() && store == s &&
				r > 0 && c > 0 && r <= s.Count() && c <= len(s.Columns) && (dragged || p != start) {
				if !dragged {
					u.app.SetFocus(u.table)
				}
				dragged = true
				u.table.Select(r, c)
				u.setGridSelection(start, p, false)
				owner.gridSelection.mouseRange = true
				if action == tview.MouseMove {
					return nil, action
				}
			}
		case tview.MouseLeftClick:
			if r > 0 && c > 0 && r <= s.Count() && c <= len(s.Columns) {
				u.active().clearGridSelection()
				u.refreshResultInfo()
			} else {
				u.markGridPoint(r, c)
			}
		}
		return event, action
	}
}

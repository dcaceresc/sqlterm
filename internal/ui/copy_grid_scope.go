package ui

import (
	"context"
	"fmt"
	"strings"

	"sqlterm/internal/clipboardtext"
	"sqlterm/internal/result"
)

type gridCopyScope struct {
	first, last gridPoint
	singleCell  bool
}

func (u *UI) explicitGridCopyScope() (gridCopyScope, bool) {
	t := u.active()
	if s := t.gridSelection; s.anchor.row > 0 {
		return gridCopyScope{
			first:      gridPoint{min(s.anchor.row, s.end.row), min(s.anchor.col, s.end.col)},
			last:       gridPoint{max(s.anchor.row, s.end.row), max(s.anchor.col, s.end.col)},
			singleCell: s.singleCell,
		}, true
	}
	if row, _ := u.table.GetSelection(); t.CopyAnchor > 0 && row > 0 {
		if s := u.currentStore(); s != nil {
			return gridCopyScope{first: gridPoint{min(t.CopyAnchor, row), 1}, last: gridPoint{max(t.CopyAnchor, row), len(s.Columns)}}, true
		}
	}
	return gridCopyScope{}, false
}

func (u *UI) copyGridHeaders(only bool) {
	if u.active().Busy {
		u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
		return
	}
	if scope, ok := u.explicitGridCopyScope(); ok {
		u.copyGridScope(scope, true, only)
		return
	}
	s := u.currentStore()
	r, c := u.table.GetSelection()
	if s == nil || r < 0 || c < 0 || (r == 0 && c == 0) {
		u.message(u.tr("Select a result cell first"))
		return
	}
	first := gridPoint{max(1, r), max(1, c)}
	last := first
	if r == 0 {
		last.row = s.Count()
	}
	if c == 0 {
		last.col = len(s.Columns)
	}
	u.copyGridScope(gridCopyScope{first: first, last: last}, true, only)
}

func (u *UI) copyGridScope(scope gridCopyScope, headers, only bool) {
	t, s := u.active(), u.currentStore()
	if s == nil || scope.first.col < 1 || scope.last.col > len(s.Columns) ||
		(!only && (scope.first.row < 1 || scope.last.row < scope.first.row || scope.last.row > s.Count() ||
			(t.Order != nil && scope.last.row > len(t.Order)))) {
		u.message(u.tr("Select a result cell first"))
		return
	}
	first, last := scope.first.col-1, scope.last.col-1
	// El orden publicado no se modifica en sitio; el trabajo usa la vista capturada al copiar.
	order := t.Order
	u.task(t, "Copy results", func(ctx context.Context) (func(), error) {
		var b strings.Builder
		if scope.singleCell && !headers {
			row := scope.first.row - 1
			if order != nil {
				row = order[row]
			}
			cells, err := s.Row(row)
			if err != nil {
				return nil, err
			}
			if first >= len(cells) {
				return nil, fmt.Errorf("missing column %d in row %d", first, row)
			}
			if cell := cells[first]; cell.Null {
				b.WriteString(`\N`)
			} else {
				b.WriteString(result.SpreadsheetSafeText(cell.Text))
			}
		} else {
			var rows []int
			switch {
			case only:
				rows = []int{}
			case order != nil:
				rows = order[scope.first.row-1 : scope.last.row]
			case scope.first.row != 1 || scope.last.row != s.Count():
				rows = make([]int, scope.last.row-scope.first.row+1)
				for i := range rows {
					rows[i] = scope.first.row - 1 + i
				}
			}
			if err := s.WriteSpreadsheetProjection(ctx, &b, rows, first, last, headers); err != nil {
				return nil, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		escaped, err := clipboardtext.Write(u.writeClipboard, b.String())
		if err != nil {
			return nil, fmt.Errorf("clipboard: %w", err)
		}
		return func() {
			switch {
			case only:
				t.Status = u.tr("Copied headers to clipboard")
			case scope.singleCell && !headers:
				t.Status = u.tr("Copied 1 cell to clipboard")
			default:
				t.Status = fmt.Sprintf(u.tr("Copied %d rows to clipboard"), scope.last.row-scope.first.row+1)
			}
			if escaped {
				t.Status += ` (NUL escaped as \0)`
			}
		}, nil
	})
}

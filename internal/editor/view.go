package editor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// View draws the gutter around the original TextArea. SQL, undo, selection,
// cursor and input handlers remain owned by that TextArea.
type View struct {
	*tview.TextArea
	editor        *Editor
	numbers, wrap bool
	muted, active tcell.Color
	text          string
	width         int
	rows          []viewRow
	complete      bool
}

type viewRow struct {
	line         int
	continuation bool
}

func (e *Editor) SetWordWrap(enabled bool) {
	if e.View.wrap == enabled {
		return
	}
	_, start, end := e.Area.GetSelection()
	e.View.wrap = enabled
	e.View.rows = nil
	e.Area.SetWrap(enabled).SetWordWrap(true).SetOffset(0, 0).Select(start, end)
	_, _, row, col := e.Area.GetCursor()
	_, _, width, height := e.Area.GetInnerRect()
	columnOffset := 0
	if !enabled {
		columnOffset = max(0, col-max(1, width)+1)
	}
	e.Area.SetOffset(max(0, row-max(1, height)+1), columnOffset)
}
func (e *Editor) WordWrap() bool                              { return e.View.wrap }
func (e *Editor) SetLineNumbers(enabled bool)                 { e.View.numbers = enabled }
func (v *View) SetLineNumberColors(muted, active tcell.Color) { v.muted, v.active = muted, active }

// Position reports the SQL location, independent of soft wrapping.
func (e *Editor) Position() (row, column int) {
	text := e.Area.GetText()
	pos := max(0, min(len(text), e.Cursor()))
	prefix := text[:pos]
	row = strings.Count(prefix, "\n")
	start := strings.LastIndexByte(prefix, '\n') + 1
	column = uniseg.StringWidth(strings.ReplaceAll(prefix[start:], "\t", strings.Repeat(" ", tview.TabSize)))
	return
}

func (v *View) Draw(screen tcell.Screen) {
	text := v.GetText()
	gutter := 0
	digits := max(2, len(strconv.Itoa(strings.Count(text, "\n")+1)))
	if v.numbers {
		gutter = digits + 2
	}
	// Padding is part of TextArea's own geometry, including mouse coordinates,
	// wrapping, cursor placement and IntelliSense anchoring.
	v.SetBorderPadding(0, 0, gutter, 0)
	v.TextArea.Draw(screen)
	if gutter == 0 {
		return
	}
	x, y, width, height := v.GetInnerRect()
	offset, _ := v.GetOffset()
	if width <= 0 || height <= 0 {
		return
	}
	if v.rows == nil || v.text != text || v.width != width || (!v.complete && offset+height >= len(v.rows)) {
		v.rows, v.complete = layoutRows(text, width, v.wrap, offset+height+256)
		v.text, v.width = text, width
	}
	current, _ := v.editor.Position()
	for i := 0; i < height; i++ {
		style := tcell.StyleDefault.Foreground(v.muted).Background(v.GetBackgroundColor())
		label := ""
		if n := offset + i; n < len(v.rows) {
			r := v.rows[n]
			marker := " "
			if r.continuation {
				marker = "↪"
			}
			label = fmt.Sprintf("%*d%s", digits, r.line, marker)
			if v.HasFocus() && r.line == current+1 {
				style = style.Foreground(v.active).Bold(true)
			}
		}
		for col := 0; col < gutter; col++ {
			screen.SetContent(x-gutter+col, y+i, ' ', nil, style)
		}
		for col, r := range []rune(label) {
			if col < gutter-1 {
				screen.SetContent(x-gutter+col, y+i, r, nil, style)
			}
		}
		screen.SetContent(x-1, y+i, '│', nil, style)
	}
}

// Use the same Unicode line-break and grapheme rules as TextArea. Stop after
// the viewport plus a lookahead instead of laying out an entire large script.
func layoutRows(text string, width int, wrap bool, limit int) ([]viewRow, bool) {
	starts, physical := []int{0}, []int{0}
	remaining, state, pos := text, -1, 0
	lineWidth, sinceBreak, lastGrapheme, lastBreak := 0, 0, 0, 0
	for len(remaining) > 0 {
		cluster, rest, boundaries, next := uniseg.StepString(remaining, state)
		state = next
		remaining = rest
		pos += len(cluster)
		cw := boundaries >> uniseg.ShiftWidth
		if cluster == "\t" {
			cw = tview.TabSize
		}
		lineWidth += cw
		sinceBreak += cw
		if !wrap || lineWidth <= width {
			if boundaries&uniseg.MaskLine == uniseg.LineMustBreak && (len(rest) > 0 || uniseg.HasTrailingLineBreakInString(cluster)) {
				starts = append(starts, pos)
				physical = append(physical, pos)
				lineWidth, sinceBreak, lastGrapheme, lastBreak = 0, 0, 0, 0
				if len(starts) > limit {
					break
				}
				continue
			}
		} else if lastBreak == 0 {
			if lastGrapheme != 0 {
				starts = append(starts, lastGrapheme)
				lineWidth = cw
				lastBreak = 0
			}
		} else {
			starts = append(starts, lastBreak)
			lineWidth = sinceBreak
			lastBreak = 0
		}
		if boundaries&uniseg.MaskLine == uniseg.LineCanBreak {
			lastBreak = pos
			sinceBreak = 0
		}
		lastGrapheme = pos
		if len(starts) > limit {
			break
		}
	}
	rows := make([]viewRow, len(starts))
	line := 0
	for i, start := range starts {
		for line+1 < len(physical) && physical[line+1] <= start {
			line++
		}
		rows[i] = viewRow{line: line + 1, continuation: start != physical[line]}
	}
	return rows, len(remaining) == 0
}

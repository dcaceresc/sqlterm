package editor

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// View dibuja la grilla publicada por Neovim cuando está activo.
// Sin una sesión nativa, solo dibuja el widget de texto.
type View struct {
	*Area
	editor        *Editor
	numbers, wrap bool
	active        tcell.Color
}

func (e *Editor) SetWordWrap(enabled bool) {
	if e.native != nil {
		e.View.wrap = enabled
		e.nativeEdit(func() error { return e.native.session.Configure(e.View.numbers, enabled) })
		return
	}
	if e.View.wrap == enabled {
		return
	}
	_, start, end := e.Area.GetSelection()
	e.View.wrap = enabled
	e.Area.SetWrap(enabled).SetWordWrap(true).SetOffset(0, 0).Select(start, end)
	_, _, row, col := e.Area.GetCursor()
	_, _, width, height := e.Area.GetInnerRect()
	columnOffset := 0
	if !enabled {
		columnOffset = max(0, col-max(1, width)+1)
	}
	e.Area.SetOffset(max(0, row-max(1, height)+1), columnOffset)
}
func (e *Editor) WordWrap() bool { return e.View.wrap }
func (e *Editor) SetLineNumbers(enabled bool) {
	e.View.numbers = enabled
	if e.native != nil {
		e.nativeEdit(func() error { return e.native.session.Configure(enabled, e.View.wrap) })
	}
}
func (v *View) SetActiveColor(active tcell.Color) { v.active = active }

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
	if v.editor.native != nil {
		v.drawNeovim(screen)
		return
	}
	v.TextArea.Draw(screen)
}

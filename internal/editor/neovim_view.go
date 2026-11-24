package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/nvim"
)

// drawNeovim pinta solo frames publicados; nunca consulta RPC bajo el bloqueo de dibujo.
func (v *View) drawNeovim(screen tcell.Screen) {
	v.SetBorderPadding(0, 0, 0, 0)
	v.Box.DrawForSubclass(screen, v)
	x, y, width, height := v.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	n := v.editor.native
	if n.session == nil {
		tview.Print(screen, "Neovim unavailable", x, y, width, tview.AlignLeft, v.active)
		return
	}
	if width != n.width || height != n.height {
		select {
		case n.resize <- [2]int{width, height}:
			n.width, n.height = width, height
		default:
		}
	}
	frame := n.session.Frame()
	for row := 0; row < min(height, frame.Height); row++ {
		for col := 0; col < min(width, frame.Width); col++ {
			cell := frame.Cell(row, col)
			if cell.Text == "" {
				continue
			}
			first, size := utf8.DecodeRuneInString(cell.Text)
			screen.SetContent(x+col, y+row, first, []rune(cell.Text[size:]), nativeStyle(frame, cell.HighlightID))
		}
	}
	if n.session.Err() != nil {
		tview.Print(screen, "Neovim exited; SQL execution is blocked", x, y+height-1, width, tview.AlignLeft, v.active)
	}
	if v.HasFocus() && frame.CursorCol < width && frame.CursorRow < height {
		screen.ShowCursor(x+frame.CursorCol, y+frame.CursorRow)
	}
}

func nativeStyle(frame nvim.Frame, id int) tcell.Style {
	h, defaults := frame.Highlight(id), frame.DefaultColors()
	fg, bg := h.Foreground, h.Background
	if fg < 0 {
		fg = defaults.Foreground
	}
	if bg < 0 {
		bg = defaults.Background
	}
	style := tcell.StyleDefault
	if fg >= 0 {
		style = style.Foreground(tcell.NewHexColor(int32(fg)))
	}
	if bg >= 0 {
		style = style.Background(tcell.NewHexColor(int32(bg)))
	}
	return style.Bold(h.Bold).Italic(h.Italic).Reverse(h.Reverse || h.Standout).
		Underline(h.Underline || h.Undercurl || h.Underdouble || h.Underdotted || h.Underdashed).
		StrikeThrough(h.Strikethrough)
}

func (a *Area) GetCursor() (int, int, int, int) {
	if n := a.editor.native; n != nil && n.session != nil {
		f := n.session.Frame()
		return f.CursorRow, f.CursorCol, f.CursorRow, f.CursorCol
	}
	return a.TextArea.GetCursor()
}

func (a *Area) GetOffset() (int, int) {
	if a.editor.native != nil {
		return 0, 0
	}
	return a.TextArea.GetOffset()
}

func (a *Area) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	if a.editor.native == nil {
		return func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
			used, capture := a.TextArea.MouseHandler()(action, event, func(p tview.Primitive) {
				if p == a.TextArea {
					focus(a)
				} else {
					focus(p)
				}
			})
			if capture == a.TextArea {
				capture = a
			}
			return used, capture
		}
	}
	return a.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !a.InRect(x, y) {
			return false, nil
		}
		innerX, innerY, width, height := a.GetInnerRect()
		col, row := x-innerX, y-innerY
		if col < 0 || row < 0 || col >= width || row >= height {
			return false, nil
		}
		button, operation := "", ""
		switch action {
		case tview.MouseLeftDown:
			button, operation = "left", "press"
			a.editor.native.dragging = true
		case tview.MouseLeftUp:
			button, operation = "left", "release"
			a.editor.native.dragging = false
		case tview.MouseMove:
			if a.editor.native.dragging {
				button, operation = "left", "drag"
			}
		case tview.MouseScrollUp:
			button, operation = "wheel", "up"
		case tview.MouseScrollDown:
			button, operation = "wheel", "down"
		}
		if button == "" {
			return false, nil
		}
		var modifiers []string
		if event.Modifiers()&tcell.ModShift != 0 {
			modifiers = append(modifiers, "S")
		}
		if event.Modifiers()&tcell.ModCtrl != 0 {
			modifiers = append(modifiers, "C")
		}
		if event.Modifiers()&tcell.ModAlt != 0 {
			modifiers = append(modifiers, "A")
		}
		focus(a)
		a.editor.nativeEdit(func() error {
			return a.editor.native.session.InputMouse(button, operation, strings.Join(modifiers, "-"), row, col)
		})
		if a.editor.native.dragging {
			return true, a
		}
		return true, nil
	})
}

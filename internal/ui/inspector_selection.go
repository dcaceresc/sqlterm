package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type cellTextView struct {
	*tview.TextArea
	text           string
	mode           rune
	anchor, cursor int
	copyValue      func(string)
	openSearch     func()
}

func (v *cellTextView) PasteHandler() func(string, func(tview.Primitive)) { return nil }

func (v *cellTextView) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		if action == tview.MouseLeftDown {
			v.mode = 0
		}
		consumed, capture := v.TextArea.MouseHandler()(action, event, func(p tview.Primitive) {
			if p == v.TextArea {
				p = v
			}
			focus(p)
		})
		if capture == v.TextArea {
			capture = v
		}
		return consumed, capture
	}
}

func (v *cellTextView) stopVisual() bool {
	if v.mode == 0 {
		return false
	}
	v.mode = 0
	v.Select(v.cursor, v.cursor)
	return true
}

func (v *cellTextView) drawSelection() {
	start, end := min(v.anchor, v.cursor), max(v.anchor, v.cursor)
	if v.mode == 'V' {
		start = strings.LastIndexByte(v.text[:start], '\n') + 1
		if n := strings.IndexByte(v.text[end:], '\n'); n >= 0 {
			end += n + 1
		} else {
			end = len(v.text)
		}
	} else if end < len(v.text) {
		_, size := utf8.DecodeRuneInString(v.text[end:])
		end += size
	}
	v.Select(start, end)
}

func (v *cellTextView) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return func(k *tcell.EventKey, focus func(tview.Primitive)) {
		if k.Key() == tcell.KeyRune && k.Modifiers() == tcell.ModNone {
			switch k.Rune() {
			case '/':
				v.stopVisual()
				v.openSearch()
				return
			case 'v', 'V':
				if v.mode == k.Rune() {
					v.stopVisual()
					return
				}
				if v.mode == 0 {
					_, _, v.cursor = v.GetSelection()
					v.anchor = v.cursor
				}
				v.mode = k.Rune()
				v.drawSelection()
				return
			}
		}
		switch k.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
			if v.mode != 0 {
				v.Select(v.cursor, v.cursor)
				k = tcell.NewEventKey(k.Key(), 0, k.Modifiers()&^tcell.ModShift)
			}
			v.TextArea.InputHandler()(k, focus)
			if v.mode != 0 {
				_, _, v.cursor = v.GetSelection()
				v.drawSelection()
			}
		}
	}
}

type cellTextSearch struct {
	*tview.InputField
	close func()
}

func (s *cellTextSearch) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		return s.InputField.MouseHandler()(action, event, func(p tview.Primitive) {
			if p == s.InputField {
				p = s
			}
			focus(p)
		})
	}
}

func (u *UI) cellInspectorKey(key tcell.Key) bool {
	switch focus := u.app.GetFocus().(type) {
	case *cellTextView:
		if key == tcell.KeyEscape {
			return focus.stopVisual()
		}
		if text, _, _ := focus.GetSelection(); text != "" {
			focus.copyValue(text)
		}
		return true
	case *cellTextSearch:
		if key == tcell.KeyEscape {
			focus.close()
			return true
		}
	}
	return false
}

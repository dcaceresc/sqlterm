package editor

import (
	"github.com/rivo/tview"
	"strings"
)

// Area mantiene el contrato de la interfaz; con Neovim sus datos son solo una proyección.
type Area struct {
	*tview.TextArea
	editor *Editor
}

func (a *Area) GetText() string {
	if n := a.editor.native; n != nil {
		return n.snapshot.Text
	}
	return a.TextArea.GetText()
}

func (a *Area) SetText(text string, cursorAtEnd bool) *Area {
	if n := a.editor.native; n != nil {
		a.editor.nativeEdit(func() error { return n.session.SetText(text) })
		if cursorAtEnd && a.editor.Err() == nil {
			a.Select(len(text), len(text))
		}
	} else {
		a.TextArea.SetText(text, cursorAtEnd)
	}
	return a
}

func (a *Area) GetSelection() (string, int, int) {
	if n := a.editor.native; n != nil {
		text, selected, err := n.snapshot.SelectedText()
		if err == nil && selected {
			return text, n.snapshot.Selection.Start, n.snapshot.Selection.End
		}
		return "", n.snapshot.Cursor, n.snapshot.Cursor
	}
	return a.TextArea.GetSelection()
}

func (a *Area) Select(start, end int) *Area {
	if n := a.editor.native; n != nil {
		a.editor.nativeEdit(func() error { return n.session.Select(start, end) })
	} else {
		a.TextArea.Select(start, end)
	}
	return a
}

func (a *Area) Replace(start, end int, text string) *Area {
	if n := a.editor.native; n != nil {
		if a.editor.Err() == nil {
			insert := strings.HasPrefix(n.snapshot.Mode, "i")
			a.editor.nativeEdit(func() error {
				if err := n.session.Replace(start, end, text); err != nil {
					return err
				}
				if err := n.session.SetCursor(start + len(text)); err != nil {
					return err
				}
				if insert {
					return n.session.Input("i")
				}
				return nil
			})
		}
	} else {
		a.TextArea.Replace(start, end, text)
	}
	return a
}

// PasteHandler recibe un bloque literal, nunca una sucesión de comandos modales.
func (a *Area) PasteHandler() func(string, func(tview.Primitive)) {
	if a.editor.native == nil {
		return a.TextArea.PasteHandler()
	}
	return func(text string, _ func(tview.Primitive)) {
		n := a.editor.native
		a.editor.nativeEdit(func() error { return n.session.Paste(text) })
	}
}

package nvim

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Input encola teclas con notación de Neovim (por ejemplo <Esc> o <LT>), no texto pegado.
// Espera una barrera no rápida en el bucle de entrada, no depende de recibir un flush.
func (s *Session) Input(keys string) error {
	if !validText(keys) {
		return ErrUnsupportedText
	}
	return s.run(func() error {
		n, err := s.v.Input(keys)
		if err != nil {
			return err
		}
		if err = s.inputBarrier(); err != nil {
			return err
		}
		if n != len(keys) {
			return fmt.Errorf("nvim: accepted %d of %d input bytes", n, len(keys))
		}
		return nil
	})
}

func (s *Session) inputBarrier() error {
	mode, err := s.v.Mode()
	if err != nil || mode.Blocking {
		return err
	}
	return s.v.ExecLua(`return true`, nil)
}

// InputMouse encola un evento de ratón en coordenadas de grilla de base cero.
func (s *Session) InputMouse(button, action, modifier string, row, col int) error {
	return s.run(func() error {
		if err := s.v.InputMouse(button, action, modifier, 0, row, col); err != nil {
			return err
		}
		return s.inputBarrier()
	})
}

// Paste pasa texto literal por nvim_paste, sin simular teclas ni entrar en Insert.
func (s *Session) Paste(text string) error {
	if !validText(text) {
		return ErrUnsupportedText
	}
	return s.run(func() error {
		p, err := s.snapshot()
		if err != nil {
			return err
		}
		switch p.Mode {
		case "n", "v", "V", "i", "ic", "ix", "R", "Rc", "Rx":
		default:
			return ErrUnsupportedMode
		}
		if text == "" {
			return nil
		}
		accepted, err := s.v.Paste(text, false, -1)
		if err == nil && !accepted {
			return errors.New("nvim: paste rejected")
		}
		return err
	})
}

// Snapshot consulta en un solo RPC el buffer SQL, el cursor y la selección real de Neovim.
func (s *Session) Snapshot() (Snapshot, error) {
	// El canal local evita compartir un resultado que aún se escribe tras un timeout.
	result := make(chan Snapshot, 1)
	err := s.run(func() error {
		p, err := s.snapshot()
		if err == nil {
			result <- p
		}
		return err
	})
	if err != nil {
		return Snapshot{}, err
	}
	return <-result, nil
}

func (s *Session) snapshot() (Snapshot, error) {
	state, err := s.v.Mode()
	if err != nil {
		return Snapshot{}, err
	}
	if state.Blocking {
		return Snapshot{}, ErrInputPending
	}
	var raw []any
	if err := s.v.ExecLua(snapshotLua, &raw, int(s.buffer)); err != nil {
		return Snapshot{}, err
	}
	if len(raw) == 1 && raw[0] == false {
		return Snapshot{}, ErrForeignBuffer
	}
	if len(raw) != 8 {
		return Snapshot{}, errors.New("nvim: invalid snapshot response")
	}
	text, _ := raw[1].(string)
	mode, _ := raw[3].(string)
	cursor := integer(raw[2])
	if mode == "" {
		return Snapshot{}, errors.New("nvim: missing snapshot mode")
	}
	if !validText(text) || !boundary(text, cursor) {
		return Snapshot{}, ErrUnsupportedText
	}
	return Snapshot{Text: text, Cursor: cursor, Mode: mode, ChangedTick: integer(raw[4]), Selection: Selection{SelectionKind(integer(raw[5])), integer(raw[6]), integer(raw[7])}}, nil
}

// SetText sustituye todo el buffer y coloca el cursor en el inicio en modo Normal.
func (s *Session) SetText(text string) error { return s.move("set", 0, 0, text) }

// Replace sustituye un rango semiabierto de bytes y coloca el cursor en el inicio del rango.
func (s *Session) Replace(start, end int, text string) error {
	return s.move("replace", start, end, text)
}

// SetCursor mueve el cursor por desplazamiento UTF-8 y sale de Visual o Insert.
func (s *Session) SetCursor(offset int) error { return s.move("cursor", offset, offset, "") }

// Select activa una selección Visual charwise. Un intervalo vacío solo mueve el cursor.
// Si Neovim no puede representar exactamente el intervalo (p. ej. solo un salto de línea), devuelve ErrUnsupportedSelection.
func (s *Session) Select(start, end int) error { return s.move("select", start, end, "") }

func (s *Session) move(op string, start, end int, text string) error {
	if !validText(text) {
		return ErrUnsupportedText
	}
	return s.run(func() error {
		p, err := s.snapshot()
		if err != nil {
			return err
		}
		switch p.Mode[0] {
		case 'n', 'v', 'V', '\x16', 's', 'S', '\x13', 'i', 'R':
		default:
			return ErrUnsupportedMode
		}
		if start > end {
			return errors.New("nvim: reversed byte range")
		}
		row, col, err := bytePosition(p.Text, start)
		if err != nil {
			return err
		}
		endrow, endcol, err := bytePosition(p.Text, end)
		if err != nil {
			return err
		}
		if op == "select" && start < end {
			_, size := utf8.DecodeLastRuneInString(p.Text[:end])
			endrow, endcol, err = bytePosition(p.Text, end-size)
			if err != nil {
				return err
			}
		} else if op == "select" {
			op = "cursor"
		}
		var ok bool
		if err = s.v.ExecLua(moveLua, &ok, int(s.buffer), op, row, col, endrow, endcol, strings.Split(text, "\n")); err != nil {
			return err
		}
		if !ok {
			return ErrForeignBuffer
		}
		if op == "select" || op == "cursor" {
			actual, err := s.snapshot()
			if err != nil {
				return err
			}
			if op == "select" && actual.Selection != (Selection{Kind: Charwise, Start: start, End: end}) {
				// No dejar una selección aproximada que podría ejecutar SQL distinto.
				if err := s.v.ExecLua(moveLua, nil, int(s.buffer), "cursor", row, col, row, col, []string{}); err != nil {
					return err
				}
				return ErrUnsupportedSelection
			}
			if op == "cursor" && actual.Cursor != start {
				return fmt.Errorf("nvim: cannot place cursor at byte offset %d", start)
			}
		}
		return nil
	})
}

package editor

import (
	"errors"
	"strings"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/nvim"
)

type nativeEditor struct {
	session       *nvim.Session
	snapshot      nvim.Snapshot
	err           error
	resize        chan [2]int
	width, height int
	dragging      bool
}

// EnableNeovim sustituye la autoridad de edición sin recrear el panel ni sus callbacks.
func (e *Editor) EnableNeovim(options nvim.Options) error {
	if e.native != nil {
		return errors.New("Neovim editor is already configured")
	}
	text := e.Area.TextArea.GetText()
	n := &nativeEditor{snapshot: nvim.Snapshot{Text: text, Mode: "n"}, resize: make(chan [2]int, 1)}
	e.native = n
	s, err := nvim.Start(options)
	if err != nil {
		n.err = err
		return err
	}
	n.session = s
	if err = s.SetText(text); err == nil {
		err = s.Configure(e.View.numbers, e.View.wrap)
	}
	if err == nil {
		err = e.Refresh()
	}
	if err != nil {
		n.err = err
		_ = s.Close()
		return err
	}
	go func() {
		for {
			select {
			case <-s.Done():
				if e.OnRedraw != nil {
					e.OnRedraw()
				}
				return
			case <-s.Changed():
				if e.OnRedraw != nil {
					e.OnRedraw()
				}
			case size := <-n.resize:
				_ = s.Resize(size[0], size[1])
			}
		}
	}()
	return nil
}

func (e *Editor) IsNeovim() bool { return e.native != nil }

func (e *Editor) SetNeovimTheme(theme nvim.Theme) error {
	if e.native == nil || e.native.session == nil {
		return nil
	}
	return e.native.session.SetTheme(theme)
}

// Err evita que una selección desconocida o un hijo muerto se conviertan en ejecutar todo.
func (e *Editor) Err() error {
	if n := e.native; n != nil {
		if n.err != nil {
			return n.err
		}
		if n.session == nil {
			return nvim.ErrClosed
		}
		if err := n.session.Err(); err != nil {
			return err
		}
		_, _, err := n.snapshot.SelectedText()
		return err
	}
	return nil
}

// Refresh se llama en la ruta de entrada/acciones, nunca desde Draw.
func (e *Editor) Refresh() error {
	n := e.native
	if n == nil {
		return nil
	}
	if n.session == nil {
		return n.err
	}
	if n.err != nil && !errors.Is(n.err, nvim.ErrInputPending) {
		return n.err
	}
	snapshot, err := n.session.Snapshot()
	if err != nil {
		n.err = err
		return err
	}
	n.err = nil
	n.snapshot = snapshot
	oldMode := e.Mode
	e.Mode = nativeMode(snapshot.Mode)
	if e.Area.TextArea.GetText() != snapshot.Text {
		e.Area.TextArea.SetText(snapshot.Text, false)
	}
	_, from, to := e.Area.GetSelection()
	e.Area.TextArea.Select(from, to)
	if oldMode != e.Mode && e.OnMode != nil {
		e.OnMode(e.Mode)
	}
	return e.Err()
}

func nativeMode(mode string) string {
	if mode == "" {
		return "NORMAL"
	}
	switch mode[0] {
	case 'i':
		return "INSERT"
	case 'R':
		return "REPLACE"
	case 'v', 'V', 's', 'S':
		return "VISUAL"
	case 22, 19:
		return "VISUAL BLOCK"
	case 'c', 'r', '!':
		return "COMMAND"
	default:
		return "NORMAL"
	}
}

func (e *Editor) nativeEdit(work func() error) {
	n := e.native
	if n.session == nil || n.session.Err() != nil {
		if e.OnError != nil {
			e.OnError(e.Err())
		}
		return
	}
	err := work()
	if err == nil {
		n.err = nil
		err = e.Refresh()
	}
	if err != nil {
		n.err = err
		if e.OnError != nil && !errors.Is(err, nvim.ErrInputPending) {
			e.OnError(err)
		}
	}
}

func (e *Editor) Close() error {
	if n := e.native; n != nil && n.session != nil {
		return n.session.Close()
	}
	return nil
}

// Complete aplica una sugerencia sin reiniciar la sesión de inserción.
func (e *Editor) Complete(start, end int, text string) {
	if n := e.native; n != nil && e.Mode == "INSERT" {
		e.nativeEdit(func() error { return n.session.CompleteInsert(start, end, text) })
		return
	}
	e.Area.Replace(start, end, text)
}

// TakeExQuit cancela la línea Ex antes de ceder el cierre al cliente.
func (e *Editor) TakeExQuit() (matched, all bool, err error) {
	if e.native == nil || e.Mode != "COMMAND" {
		return false, false, nil
	}
	line, err := e.native.session.ExCommandLine()
	if err != nil {
		return false, false, err
	}
	switch strings.TrimSuffix(strings.TrimSpace(line), "!") {
	case "q", "qu", "qui", "quit":
	case "qa", "qal", "qall", "quita", "quital", "quitall":
		all = true
	default:
		return false, false, nil
	}
	e.nativeEdit(func() error { return e.native.session.Input("<Esc>") })
	return true, all, e.Err()
}

func (e *Editor) nativeKey(k *tcell.EventKey) {
	if key := nativeKeyName(k); key != "" {
		e.nativeEdit(func() error { return e.native.session.Input(key) })
	}
}

func nativeKeyName(k *tcell.EventKey) string {
	prefix := ""
	mods := k.Modifiers()
	if mods&tcell.ModCtrl != 0 {
		prefix += "C-"
	}
	if mods&tcell.ModAlt != 0 {
		prefix += "M-"
	}
	if k.Key() == tcell.KeyRune {
		text := string(k.Rune())
		if prefix == "" {
			if text == "<" {
				return "<LT>"
			}
			return text
		}
		return "<" + prefix + text + ">"
	}
	keys := map[tcell.Key]string{
		tcell.KeyEscape: "Esc", tcell.KeyEnter: "CR", tcell.KeyTab: "Tab", tcell.KeyBacktab: "S-Tab",
		tcell.KeyBackspace: "BS", tcell.KeyBackspace2: "BS", tcell.KeyDelete: "Del", tcell.KeyInsert: "Insert",
		tcell.KeyUp: "Up", tcell.KeyDown: "Down", tcell.KeyLeft: "Left", tcell.KeyRight: "Right",
		tcell.KeyHome: "Home", tcell.KeyEnd: "End", tcell.KeyPgUp: "PageUp", tcell.KeyPgDn: "PageDown",
		tcell.KeyCtrlSpace: "Space", tcell.KeyCtrlRightSq: "]", tcell.KeyCtrlBackslash: "\\",
	}
	name := keys[k.Key()]
	if name == "" && k.Key() >= tcell.KeyCtrlA && k.Key() <= tcell.KeyCtrlZ {
		name = string(rune('a' + k.Key() - tcell.KeyCtrlA))
		if !strings.Contains(prefix, "C-") {
			prefix = "C-" + prefix
		}
	}
	if name == "" && k.Key() >= tcell.KeyF1 && k.Key() <= tcell.KeyF64 {
		name = tcell.KeyNames[k.Key()]
	}
	if k.Key() == tcell.KeyCtrlSpace || k.Key() == tcell.KeyCtrlRightSq || k.Key() == tcell.KeyCtrlBackslash {
		if !strings.Contains(prefix, "C-") {
			prefix = "C-" + prefix
		}
	}
	if mods&tcell.ModShift != 0 && k.Key() != tcell.KeyBacktab {
		prefix += "S-"
	}
	if name == "" {
		return ""
	}
	return "<" + prefix + name + ">"
}

package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
	"sqlterm/internal/nvim"
)

// NewWithNeovim conserva la interfaz SQLTerm y usa un hijo aislado por pestaña SQL.
func NewWithNeovim(path string, cfg config.Config) (*UI, error) {
	u := New(path, cfg)
	u.nativeEditors = true
	for _, tab := range u.tabs {
		if err := u.activateNeovim(tab); err != nil {
			u.cancel()
			for _, open := range u.tabs {
				u.dispose(open)
			}
			u.nativeWG.Wait()
			return nil, err
		}
	}
	return u, nil
}

func (u *UI) activateNeovim(t *Tab) error {
	t.Editor.OnError = func(err error) {
		t.Status = fmt.Sprintf("Neovim: %v", err)
		u.refreshBars()
	}
	t.Editor.OnRedraw = func() {
		select {
		case <-u.ctx.Done():
			return
		default:
		}
		if !u.nativeEventPending.CompareAndSwap(false, true) {
			return
		}
		u.app.QueueEvent(tcell.NewEventInterrupt(nil))
	}
	if err := t.Editor.EnableNeovim(nvim.Options{Width: 80, Height: 20}); err != nil {
		return err
	}
	return t.Editor.SetNeovimTheme(u.palette.neovimTheme())
}

func (p Palette) neovimTheme() nvim.Theme {
	return nvim.Theme{
		Foreground: p.Foreground.Hex(), Background: p.Background.Hex(), Surface: p.Surface.Hex(),
		Muted: p.Muted.Hex(), Accent: p.Accent.Hex(), Selection: p.Selection.Hex(), SelectedText: p.SelectedText.Hex(),
		Keyword: p.Syntax.Keyword.Hex(), String: p.Syntax.String.Hex(), Number: p.Syntax.Number.Hex(),
	}
}

func (u *UI) editorReady(t *Tab) bool {
	if err := t.Editor.Refresh(); err != nil {
		t.Status = fmt.Sprintf("Neovim: %v", err)
		u.refreshBars()
		return false
	}
	return true
}

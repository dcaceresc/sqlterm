package ui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestCommentsUseMutedColorAcrossThemes(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 35)
	u.pages.SetRect(0, 0, 120, 35)
	e := u.active().Editor
	e.SetText("-- note\nSELECT 1;")
	e.Area.Select(0, 0)
	for _, theme := range palettes {
		u.applyTheme(theme.ID)
		u.pages.Draw(s)
		s.Show()
		x, y, _, _ := e.Area.GetInnerRect()
		_, _, style, _ := s.GetContent(x, y)
		fg, bg, _ := style.Decompose()
		if fg != theme.Muted || bg != theme.Background {
			t.Errorf("%s comment style %v/%v", theme.ID, fg, bg)
		}
		_, _, style, _ = s.GetContent(x, y+1)
		fg, bg, _ = style.Decompose()
		if fg != theme.Foreground || bg != theme.Background {
			t.Errorf("%s ordinary SQL style changed", theme.ID)
		}
	}
}

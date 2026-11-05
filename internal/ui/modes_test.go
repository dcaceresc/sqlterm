package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/config"
	"strings"
	"testing"
)

type cursorScreen struct {
	tcell.SimulationScreen
	shape tcell.CursorStyle
	color tcell.Color
}

func (s *cursorScreen) SetCursorStyle(shape tcell.CursorStyle, colors ...tcell.Color) {
	s.shape = shape
	s.color = tcell.ColorDefault
	if len(colors) > 0 {
		s.color = colors[0]
	}
	s.SimulationScreen.SetCursorStyle(shape, colors...)
}
func modeTestUI(t *testing.T) (*UI, *cursorScreen) {
	t.Helper()
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	s := &cursorScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8")}
	u.app.SetScreen(s)
	s.SetSize(120, 35)
	t.Cleanup(func() { u.cancel(); s.Fini() })
	return u, s
}
func modeKey(u *UI, key tcell.Key, r rune) {
	u.active().Editor.Area.InputHandler()(tcell.NewEventKey(key, r, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
}
func badgeBackground(u *UI, s *cursorScreen) tcell.Color {
	x, y, _, _ := u.status.GetRect()
	_, _, style, _ := s.GetContent(x+1, y)
	_, bg, _ := style.Decompose()
	return bg
}
func TestModesChangeBadgeBorderAndCursorInEveryTheme(t *testing.T) {
	u, s := modeTestUI(t)
	for _, p := range palettes {
		u.applyTheme(p.ID)
		e := u.active().Editor
		e.SetText("SELECT 1;")
		u.app.ForceDraw()
		normal := badgeBackground(u, s)
		if normal != p.Accent || s.shape != tcell.CursorStyleSteadyBlock || s.color != p.Accent {
			t.Fatalf("%s: NORMAL not styled", p.ID)
		}
		modeKey(u, tcell.KeyRune, 'i')
		u.app.ForceDraw()
		insert := badgeBackground(u, s)
		if e.Mode != "INSERT" || insert == normal || insert != p.Insert || s.shape != tcell.CursorStyleSteadyBar || s.color != p.Insert || e.Area.GetBorderColor() != p.Insert {
			t.Fatalf("%s: INSERT not visibly different", p.ID)
		}
		modeKey(u, tcell.KeyEscape, 0)
		u.app.ForceDraw()
		if badgeBackground(u, s) != normal || s.shape != tcell.CursorStyleSteadyBlock {
			t.Fatal("Esc did not restore NORMAL appearance")
		}
		modeKey(u, tcell.KeyRune, 'v')
		u.app.ForceDraw()
		visual := badgeBackground(u, s)
		if visual == normal || visual == insert || visual != p.Visual || s.shape != tcell.CursorStyleSteadyBlock || e.Area.GetBorderColor() != p.Visual {
			t.Fatalf("%s: VISUAL not styled", p.ID)
		}
		if e.Area.GetText() != "SELECT 1;" {
			t.Fatal("changing mode altered SQL")
		}
		modeKey(u, tcell.KeyEscape, 0)
	}
}
func TestModeAppearanceFollowsFocusTabAndTheme(t *testing.T) {
	u, s := modeTestUI(t)
	first := u.active()
	first.Editor.SetText("SELECT 1;")
	modeKey(u, tcell.KeyRune, 'i')
	u.app.ForceDraw()
	u.app.SetFocus(u.search)
	u.app.ForceDraw()
	if s.shape != tcell.CursorStyleDefault || s.color != tcell.ColorReset || first.Editor.Area.GetBorderColor() != u.palette.Border || u.objects.GetBorderColor() != u.palette.Accent {
		t.Fatal("inactive editor kept active cursor or border")
	}
	u.app.SetFocus(first.Editor.Area)
	u.app.ForceDraw()
	if s.shape != tcell.CursorStyleSteadyBar {
		t.Fatal("returning to INSERT did not restore bar cursor")
	}
	u.applyTheme("light")
	u.app.ForceDraw()
	if badgeBackground(u, s) != u.palette.Insert || s.color != u.palette.Insert {
		t.Fatal("theme change lost mode styling")
	}
	u.prompt("File", "query.sql", false, func(string) {})
	u.app.ForceDraw()
	if s.shape != tcell.CursorStyleDefault || s.color != tcell.ColorReset {
		t.Fatal("editor cursor style leaked into dialog")
	}
	u.dismiss()
	u.app.ForceDraw()
	if s.shape != tcell.CursorStyleSteadyBar {
		t.Fatal("dismiss did not restore insert cursor")
	}
	u.newTab(nil)
	u.app.ForceDraw()
	if u.active().Editor.Mode != "NORMAL" || s.shape != tcell.CursorStyleSteadyBlock {
		t.Fatal("new tab inherited incorrect mode")
	}
	u.current = 0
	u.showTab()
	u.app.ForceDraw()
	if first.Editor.Mode != "INSERT" || s.shape != tcell.CursorStyleSteadyBar {
		t.Fatal("switching tabs lost insert appearance")
	}
}
func TestModeStatusFitsWidthAndPreservesPosition(t *testing.T) {
	p := resolveTheme("tokyo-night")
	for _, width := range []int{1, 8, 24, 40, 80, 120} {
		status := modeStatus(p, "INSERT", "客户报告[2026].sql", "PROD/长名称 / production", "query failed\nsecond line", 12, 34, width)
		if got := tview.TaggedStringWidth(status); got != width {
			t.Fatalf("status width %d, want %d", got, width)
		}
		if strings.Contains(status, "\n") {
			t.Fatal("status contains newline")
		}
		if width >= 24 && !strings.Contains(status, "12:34") {
			t.Fatal("cursor position clipped instead of context")
		}
	}
	u, s := modeTestUI(t)
	u.active().Editor.SetText("SELECT 1;\nSELECT 2;")
	u.app.ForceDraw() // Lay out the text before positioning a cursor on line two.
	u.active().Editor.Area.Select(12, 12)
	for _, width := range []int{80, 120} {
		s.SetSize(width, 35)
		u.app.ForceDraw()
		if got := tview.TaggedStringWidth(u.status.GetText(false)); got != width {
			t.Fatalf("resized status width %d want %d", got, width)
		}
		if !strings.Contains(u.status.GetText(true), "2:3") {
			t.Fatalf("position did not update with cursor: %q", u.status.GetText(true))
		}
	}
}

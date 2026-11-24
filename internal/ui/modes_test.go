package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/config"
	"strings"
	"testing"
	"time"
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
		u.active().Editor.Mode = "INSERT"
		u.app.ForceDraw()
		insert := badgeBackground(u, s)
		if e.Mode != "INSERT" || insert == normal || insert != p.Insert || s.shape != tcell.CursorStyleSteadyBar || s.color != p.Insert || e.Area.GetBorderColor() != p.Insert {
			t.Fatalf("%s: INSERT not visibly different", p.ID)
		}
		u.active().Editor.Mode = "NORMAL"
		u.app.ForceDraw()
		if badgeBackground(u, s) != normal || s.shape != tcell.CursorStyleSteadyBlock {
			t.Fatal("NORMAL appearance was not restored")
		}
		u.active().Editor.Mode = "VISUAL"
		u.app.ForceDraw()
		visual := badgeBackground(u, s)
		if visual == normal || visual == insert || visual != p.Visual || s.shape != tcell.CursorStyleSteadyBlock || e.Area.GetBorderColor() != p.Visual {
			t.Fatalf("%s: VISUAL not styled", p.ID)
		}
		if e.Area.GetText() != "SELECT 1;" {
			t.Fatal("changing mode altered SQL")
		}
		u.active().Editor.Mode = "NORMAL"
	}
}
func TestModeAppearanceFollowsFocusTabAndTheme(t *testing.T) {
	u, s := modeTestUI(t)
	first := u.active()
	first.Editor.SetText("SELECT 1;")
	u.active().Editor.Mode = "INSERT"
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
func TestModeStatusShowsMessageAndExecutionTime(t *testing.T) {
	u, screen := modeTestUI(t)
	tab := u.active()
	tab.Profile.Folder, tab.Profile.Name, tab.Profile.Database = "DEV", "test-profile", "test-database"
	tab.Editor.SetText("SELECT 1;")
	u.message("query failed")
	for _, width := range []int{80, 120} {
		screen.SetSize(width, 35)
		for _, state := range []struct {
			execution *execution
			want      string
		}{
			{nil, "NORMAL query failed"},
			{&execution{Phase: "Completed", Duration: 1200 * time.Millisecond}, "NORMAL query failed 1.2s"},
		} {
			tab.Execution = state.execution
			u.app.ForceDraw()
			if got := tview.TaggedStringWidth(u.status.GetText(false)); got != width {
				t.Errorf("status width = %d, want %d", got, width)
			}
			if got := strings.Join(strings.Fields(u.status.GetText(true)), " "); got != state.want {
				t.Errorf("status = %q, want %q", got, state.want)
			}
			text := screenText(screen)
			lines := strings.Split(text, "\n")
			if !strings.Contains(strings.ToLower(lines[34]), "f1 help") || !strings.Contains(strings.ToLower(lines[34]), "f2 connect") {
				t.Error("help keys no longer visible in the bottom row")
			}
			if strings.Contains(lines[32], "query failed") {
				t.Error("operation message appears on a separate row")
			}
		}
	}
}

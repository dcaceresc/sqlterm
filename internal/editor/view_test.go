package editor

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func viewScreen(t *testing.T, e *Editor, width, height int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Fini)
	s.SetSize(width, height)
	e.Area.SetBorder(true)
	e.View.SetRect(0, 0, width, height)
	e.View.Draw(s)
	return s
}
func TestWrapTogglePreservesSelectionModeAndViewport(t *testing.T) {
	e := New()
	sql := strings.Repeat("SELECT long_column_name FROM some_table;\n", 50)
	e.SetText(sql)
	s := viewScreen(t, e, 32, 10)
	start := strings.LastIndex(sql, "SELECT")
	e.Area.Select(start, start+6)
	e.Mode = "INSERT"
	for _, enabled := range []bool{true, false, true} {
		e.SetWordWrap(enabled)
		e.View.Draw(s)
		selection, from, to := e.Area.GetSelection()
		if selection != "SELECT" || from != start || to != start+6 || e.Mode != "INSERT" {
			t.Fatal("wrap changed selection or mode")
		}
		_, _, row, _ := e.Area.GetCursor()
		offset, _ := e.Area.GetOffset()
		_, _, _, height := e.Area.GetInnerRect()
		if row < offset || row >= offset+height {
			t.Fatal("wrap hid cursor", row, offset, height)
		}
		logical, column := e.Position()
		if logical != 49 || column != 6 {
			t.Fatal("position uses visual rows", logical, column)
		}
	}
}

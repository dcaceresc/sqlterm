package editor

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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
func gutterText(e *Editor, s tcell.Screen, screenRow int) string {
	x, _, _, _ := e.Area.GetInnerRect()
	var b strings.Builder
	for col := 1; col < x-1; col++ {
		r, comb, _, _ := s.GetContent(col, screenRow)
		b.WriteRune(r)
		for _, r := range comb {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func TestLineNumbersScrollEditAndUndo(t *testing.T) {
	e := New()
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("SELECT %d;", i+1)
	}
	sql := strings.Join(lines, "\n")
	e.SetText(sql)
	s := viewScreen(t, e, 42, 9)
	if label := gutterText(e, s, 1); label != "1" {
		t.Fatal("first line", label)
	}
	e.Area.SetOffset(398, 0)
	e.View.Draw(s)
	if label := gutterText(e, s, 1); label != "399" {
		t.Fatal("scrolled line", label)
	}
	e.Area.Replace(0, 0, "-- new line\n")
	e.Area.SetOffset(0, 0)
	e.View.Draw(s)
	if got := len(e.View.rows); got < 8 {
		t.Fatal("gutter disappeared after edit")
	}
	keys(e, "u")
	e.View.Draw(s)
	if e.Area.GetText() != sql {
		t.Fatal("gutter changed SQL or undo")
	}
}

func TestWrappedGutterMatchesTextAreaLogicalLines(t *testing.T) {
	for _, sql := range []string{"SELECT alpha, beta, gamma, delta FROM demo_table WHERE id = 1;\nSELECT 2;\n", "SELECT '\u4e2d\u6587 \u00f1 e\u0301 👩‍💻';\r\n\tSELECT lots_of_characters_without_spaces_here;\n", strings.Repeat("x", 100) + "\n\nSELECT 3;"} {
		e := New()
		e.SetText(sql)
		s := viewScreen(t, e, 27, 10)
		e.SetWordWrap(true)
		e.View.Draw(s)
		for pos := 0; pos <= len(sql); {
			e.Area.Select(pos, pos)
			e.View.Draw(s)
			_, _, row, _ := e.Area.GetCursor()
			_, _, actual := e.Area.GetSelection()
			logical := strings.Count(sql[:actual], "\n") + 1
			// TextArea can display an end-of-line caret on the next screen
			// row when the preceding row fills the viewport exactly.
			atBreak := actual < len(sql) && (sql[actual] == '\r' || sql[actual] == '\n')
			if !atBreak && row < len(e.View.rows) && e.View.rows[row].line != logical {
				t.Fatalf("pos %d: visual row %d numbered %d, want %d in %q", pos, row, e.View.rows[row].line, logical, sql)
			}
			if pos == len(sql) {
				break
			}
			_, size := utf8.DecodeRuneInString(sql[pos:])
			pos += size
		}
		if e.Area.GetText() != sql {
			t.Fatal("wrapping changed SQL")
		}
	}
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

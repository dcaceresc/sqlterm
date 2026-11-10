package editor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

var testSyntax = SyntaxColors{Keyword: tcell.ColorBlue, String: tcell.ColorGreen, Number: tcell.ColorRed}

func syntaxScreen(t *testing.T, e *Editor, width int) tcell.SimulationScreen {
	t.Helper()
	e.SetLineNumbers(false)
	e.Area.SetBackgroundColor(tcell.ColorBlack)
	e.Area.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack))
	e.View.SetSyntaxColors(testSyntax)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Fini)
	s.SetSize(width, 20)
	e.View.SetRect(0, 0, width, 20)
	e.View.Draw(s)
	return s
}

func TestSyntaxLiteralBoundaries(t *testing.T) {
	for _, tc := range []struct {
		sql, token string
		color      tcell.Color
	}{
		{"select SELECTED col42 @select #select", "select", testSyntax.Keyword},
		{"select SELECTED col42 @select #select", "SELECTED", tcell.ColorWhite},
		{"SELECT [SELECT], \"FROM\", `WHERE`", "SELECT]", tcell.ColorWhite},
		{"SELECT [SELECT], \"FROM\", `WHERE`", "FROM", tcell.ColorWhite},
		{"SELECT [SELECT], \"FROM\", `WHERE`", "WHERE", tcell.ColorWhite},
		{"SELECT 'WHERE 42 -- ''text'''", "WHERE", testSyntax.String},
		{"SELECT N'café 日本語'", "N'", testSyntax.String},
		{"SELECT 'first\nWHERE 42\nlast'", "WHERE", testSyntax.String},
		{"SELECT $body$first\nSELECT 123$body$", "123", testSyntax.String},
		{"SELECT 'unfinished", "unfinished", testSyntax.String},
		{"SELECT 42, 3.14, .5, 1e-3, 0xFF", "42", testSyntax.Number},
		{"SELECT 42, 3.14, .5, 1e-3, 0xFF", "3.14", testSyntax.Number},
		{"SELECT 42, 3.14, .5, 1e-3, 0xFF", ".5", testSyntax.Number},
		{"SELECT 42, 3.14, .5, 1e-3, 0xFF", "1e-3", testSyntax.Number},
		{"SELECT 42, 3.14, .5, 1e-3, 0xFF", "0xFF", testSyntax.Number},
		{"SELECT col42, @id123", "42", tcell.ColorWhite},
		{"SELECT 1 -- SELECT 'text' 42", "42", tcell.ColorWhite},
		{"/* outer /* SELECT */ 'text' 42 */ SELECT", "42", tcell.ColorWhite},
	} {
		t.Run(tc.sql+"/"+tc.token, func(t *testing.T) {
			e := New()
			e.SetText(tc.sql)
			s := syntaxScreen(t, e, 120)
			start := strings.Index(tc.sql, tc.token)
			prefix := tc.sql[:start]
			row := strings.Count(prefix, "\n")
			line := strings.LastIndex(prefix, "\n") + 1
			col := uniseg.StringWidth(prefix[line:])
			for i := 0; i < len(tc.token); i++ {
				_, _, st, _ := s.GetContent(col+i, row)
				fg, _, _ := st.Decompose()
				if fg != tc.color {
					t.Fatalf("cell %d: %v want %v", i, fg, tc.color)
				}
			}
		})
	}
}

func TestSyntaxWrapScrollUnicodeSelectionAndUndo(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		e := New()
		text := "SELECT '日本 e\u0301 👩‍💻\t" + strings.Repeat("x", 45) + "', 42;\nSELECT 7;"
		e.SetText(text)
		e.SetWordWrap(wrap)
		s := syntaxScreen(t, e, 24)
		// Compare glyphs to TextArea's unmodified renderer, including wide clusters.
		plain := tcell.NewSimulationScreen("UTF-8")
		if err := plain.Init(); err != nil {
			t.Fatal(err)
		}
		defer plain.Fini()
		plain.SetSize(24, 20)
		e.Area.Draw(plain)
		for y := 0; y < 20; y++ {
			for x := 0; x < 24; x++ {
				r, c, _, _ := s.GetContent(x, y)
				pr, pc, _, _ := plain.GetContent(x, y)
				if r != pr || !reflect.DeepEqual(c, pc) {
					t.Fatalf("wrap=%v glyph changed at %d,%d", wrap, x, y)
				}
			}
		}
		if wrap {
			for y := 0; y < 10; y++ {
				for x := 0; x < 24; x++ {
					r, _, st, _ := s.GetContent(x, y)
					fg, _, _ := st.Decompose()
					if r == 'x' && fg != testSyntax.String {
						t.Fatalf("wrapped string at %d,%d not colored", x, y)
					}
				}
			}
		} else {
			e.Area.SetOffset(0, 24)
			e.View.Draw(s)
			r, _, st, _ := s.GetContent(0, 0)
			fg, _, _ := st.Decompose()
			if r != 'x' || fg != testSyntax.String {
				t.Fatal("horizontal scroll lost string color", r, fg)
			}
		}
		secondLine := 0
		for row, layout := range e.View.rows {
			if layout.line == 2 {
				secondLine = row
				break
			}
		}
		e.Area.SetOffset(secondLine, 0)
		e.View.Draw(s)
		_, _, st, _ := s.GetContent(0, 0)
		fg, _, _ := st.Decompose()
		if fg != testSyntax.Keyword {
			t.Fatal("vertical scroll lost keyword color")
		}
		e.Area.SetOffset(0, 0)
		selected := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)
		e.Area.SetSelectedStyle(selected).Select(1, 5)
		e.View.Draw(s)
		for x := 1; x < 5; x++ {
			_, _, st, _ := s.GetContent(x, 0)
			if st != selected {
				t.Fatal("selection overwritten", st)
			}
		}
		before, from, to := e.Area.GetSelection()
		e.View.Draw(s)
		after, a, b := e.Area.GetSelection()
		if e.Area.GetText() != text || before != after || from != a || to != b {
			t.Fatal("draw mutated editor")
		}
		e.Area.Replace(0, 6, "'text'")
		e.Area.Select(0, 0)
		e.View.Draw(s)
		_, _, st, _ = s.GetContent(0, 0)
		fg, _, _ = st.Decompose()
		if fg != testSyntax.String {
			t.Fatal("edit did not invalidate syntax cache")
		}
		keys(e, "u")
		e.View.Draw(s)
		if e.Area.GetText() != text {
			t.Fatal("draw changed undo history")
		}
	}
}

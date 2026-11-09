package editor

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCommentLinesRenderMuted(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		for _, numbers := range []bool{false, true} {
			e := New()
			e.SetText("  -- comment café 日本語 with a long tail that wraps across several visible rows\nSELECT 1;\n-- last comment")
			e.SetWordWrap(wrap)
			e.SetLineNumbers(numbers)
			normal, muted, background := tcell.ColorWhite, tcell.ColorGray, tcell.ColorBlack
			selected := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlue)
			e.Area.SetTextStyle(tcell.StyleDefault.Foreground(normal).Background(background)).SetSelectedStyle(selected)
			e.Area.SetBackgroundColor(background)
			e.View.SetLineNumberColors(muted, tcell.ColorGreen)
			s := tcell.NewSimulationScreen("UTF-8")
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			s.SetSize(32, 18)
			e.View.SetRect(0, 0, 32, 18)
			e.View.Draw(s)
			s.Show()
			x, y, width, _ := e.Area.GetInnerRect()
			rows, _ := layoutRows(e.Area.GetText(), width, wrap, 100)
			for row, logical := range rows {
				_, _, style, _ := s.GetContent(x+2, y+row)
				fg, _, _ := style.Decompose()
				want := muted
				if logical.line == 2 {
					want = normal
				}
				if fg != want {
					t.Errorf("wrap=%v numbers=%v row=%d line=%d foreground=%v want=%v", wrap, numbers, row, logical.line, fg, want)
				}
			}
			e.Area.Select(2, 4)
			e.View.Draw(s)
			s.Show()
			_, _, style, _ := s.GetContent(x+2, y)
			if style != selected {
				t.Errorf("comment color overwrote selected style: %v", style)
			}
			e.Area.Select(0, 0)
			e.View.SetLineNumberColors(tcell.ColorPurple, tcell.ColorGreen)
			e.View.Draw(s)
			s.Show()
			_, _, style, _ = s.GetContent(x+2, y)
			fg, _, _ := style.Decompose()
			if fg != tcell.ColorPurple {
				t.Error("muted color did not follow theme change")
			}
			if !wrap {
				e.Area.SetOffset(1, 4)
				e.View.Draw(s)
				s.Show()
				_, _, style, _ = s.GetContent(x, y)
				fg, _, _ = style.Decompose()
				if fg != normal {
					t.Error("scrolling applied comment color to ordinary SQL")
				}
			}
			s.Fini()
		}
	}
}

func TestCommentColorDoesNotChangeTextOrUndo(t *testing.T) {
	e := New()
	text := "-- note\nSELECT 'unchanged';"
	e.SetText(text)
	e.Area.Select(0, len("-- note"))
	e.CommentLines(false)
	before := e.Area.GetText()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)
	e.View.SetRect(0, 0, 80, 24)
	e.View.Draw(s)
	if e.Area.GetText() != before {
		t.Fatal("drawing mutated SQL")
	}
	keys(e, "u")
	if e.Area.GetText() != text {
		t.Fatal("drawing changed undo history")
	}
}

func TestCommentColorIgnoresQuotedCommentMarkers(t *testing.T) {
	text := "SELECT 'first\n-- inside string\nlast';\nSELECT $$first\n-- inside dollar string\nlast$$;\n-- actual comment"
	e := New()
	e.SetText(text)
	e.SetLineNumbers(false)
	e.Area.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack))
	e.Area.SetBackgroundColor(tcell.ColorBlack)
	e.View.SetLineNumberColors(tcell.ColorGray, tcell.ColorGreen)
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)
	e.View.SetRect(0, 0, 80, 24)
	e.View.Draw(s)
	s.Show()
	for row, line := range strings.Split(text, "\n") {
		_, _, style, _ := s.GetContent(0, row)
		fg, _, _ := style.Decompose()
		want := tcell.ColorWhite
		if line == "-- actual comment" {
			want = tcell.ColorGray
		}
		if fg != want {
			t.Errorf("line %q foreground=%v want=%v", line, fg, want)
		}
	}
}

package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/result"
)

func TestGridMouseHighlightsOnlySelectedScope(t *testing.T) {
	u, screen := modeTestUI(t)
	s, err := result.New([]string{"A", "B"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for range 2 {
		s.Append([]result.Cell{{Text: "one"}, {Text: "two"}})
	}
	u.active().Results = []*result.Store{s}
	u.showResults()
	u.app.SetFocus(u.table)
	u.app.ForceDraw()
	point := func(row, col int) (int, int) {
		x, y, w, h := u.table.GetInnerRect()
		for dy := range h {
			for dx := range w {
				if r, c := u.table.CellAt(x+dx, y+dy); r == row && c == col {
					return x + dx, y + dy
				}
			}
		}
		t.Fatalf("cell (%d,%d) not rendered", row, col)
		return 0, 0
	}
	for _, target := range [][2]int{{1, 2}, {0, 1}, {2, 0}} {
		r, c := target[0], target[1]
		x, y := point(r, c)
		u.table.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.Button1, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		u.app.ForceDraw()
		for row := range 3 {
			for col := range 3 {
				x, y := point(row, col)
				_, _, style, _ := screen.GetContent(x, y)
				_, bg, _ := style.Decompose()
				want := (r == row && c == col) || (r == 0 && c == col) || (c == 0 && r == row)
				if (bg == u.palette.Selection) != want {
					t.Errorf("click (%d,%d): wrong highlight on (%d,%d)", r, c, row, col)
				}
			}
		}
	}
}

func TestGridCopiesCellRowAndColumnWithoutDialog(t *testing.T) {
	for _, tc := range []struct {
		name     string
		row, col int
		key      tcell.Key
		rune     rune
		want     string
	}{
		{"cell", 1, 1, tcell.KeyCtrlC, 0, "one"},
		{"column", 0, 2, tcell.KeyRune, 'y', "two\nfour\n"},
		{"row", 2, 0, tcell.KeyCtrlC, 0, "three\tfour\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := runningUI(t, config.Config{})
			copied := make(chan string, 1)
			u.app.QueueUpdateDraw(func() {
				s, err := result.New([]string{"A", "B"})
				if err != nil {
					t.Error(err)
					return
				}
				u.active().Results = []*result.Store{s}
				s.Append([]result.Cell{{Text: "one"}, {Text: "two"}})
				s.Append([]result.Cell{{Text: "three"}, {Text: "four"}})
				u.writeClipboard = func(text string) error { copied <- text; return nil }
				u.showResults()
				u.app.SetFocus(u.table)
				u.table.Select(tc.row, tc.col)
				dispatchSelector(u, tcell.KeyEnter, 0)
				if u.dialogContent != nil {
					t.Error("Enter opened a dialog")
				}
				dispatchSelector(u, tc.key, tc.rune)
				if u.dialogContent != nil {
					t.Error("copy opened a dialog")
				}
			})
			if got := receiveCopy(t, copied); got != tc.want {
				t.Fatalf("copied %q, want %q", got, tc.want)
			}
		})
	}
}

package ui

import (
	"testing"
	"time"

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
	capture := u.gridMouseCapture()
	for _, target := range [][2]int{{1, 2}, {0, 1}, {2, 0}} {
		r, c := target[0], target[1]
		x, y := point(r, c)
		event, action := capture(tcell.NewEventMouse(x, y, tcell.Button1, 0), tview.MouseLeftClick)
		u.table.MouseHandler()(action, event, func(p tview.Primitive) { u.app.SetFocus(p) })
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
		if r > 0 && c > 0 {
			if _, selected := u.explicitGridCopyScope(); selected {
				t.Error("a plain click left an explicit selection")
			}
			dispatchModified(u, tcell.KeyDown, 0, 0)
			u.app.ForceDraw()
			x, y := point(r, c)
			_, _, style, _ := screen.GetContent(x, y)
			if _, bg, _ := style.Decompose(); bg == u.palette.Selection {
				t.Error("the old focus remained highlighted")
			}
		}
	}
}

func TestGridExplicitSelection(t *testing.T) {
	u, screen := modeTestUI(t)
	s, err := result.New([]string{"A", "B", "C"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for range 3 {
		if err := s.Append([]result.Cell{{Text: "one"}, {Text: "two"}, {Text: "three"}}); err != nil {
			t.Fatal(err)
		}
	}
	u.active().Results = []*result.Store{s, s}
	u.showResults()
	u.app.SetFocus(u.table)
	u.app.ForceDraw()
	point := func(row, col int) (int, int) {
		t.Helper()
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
	marked := func(row, col int) bool {
		u.app.ForceDraw()
		x, y := point(row, col)
		_, _, style, _ := screen.GetContent(x, y)
		_, bg, _ := style.Decompose()
		return bg == u.palette.Selection
	}
	shift := func(key tcell.Key) { dispatchModified(u, key, 0, tcell.ModShift) }
	t.Run("reverse rectangle and independent focus", func(t *testing.T) {
		u.table.Select(3, 3)
		shift(tcell.KeyLeft)
		shift(tcell.KeyUp)
		dispatchModified(u, tcell.KeyLeft, 0, 0)
		if !marked(3, 3) || !marked(2, 2) || marked(1, 3) || marked(3, 0) {
			t.Fatal("incorrect rectangular highlight")
		}
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		if marked(3, 3) {
			t.Fatal("Escape did not clear rectangle")
		}
	})
	t.Run("Enter marks a column and v extends rows", func(t *testing.T) {
		u.table.Select(0, 2)
		dispatchModified(u, tcell.KeyEnter, 0, 0)
		dispatchModified(u, tcell.KeyRight, 0, 0)
		if !marked(3, 2) {
			t.Fatal("marked column followed focus")
		}
		u.table.Select(1, 1)
		dispatchModified(u, tcell.KeyRune, 'v', 0)
		dispatchModified(u, tcell.KeyDown, 0, 0)
		if !marked(1, 3) || !marked(2, 3) || marked(3, 3) {
			t.Fatal("incorrect row range")
		}
		u.table.Select(0, 1)
		if marked(1, 3) || marked(2, 3) {
			t.Fatal("row range was highlighted while focus was on a header")
		}
		dispatchModified(u, tcell.KeyRune, 'v', 0)
		if marked(1, 3) {
			t.Fatal("v did not clear range")
		}
	})
	t.Run("select all and switch result", func(t *testing.T) {
		dispatchModified(u, tcell.KeyCtrlA, 0, 0)
		if !marked(3, 3) || !marked(1, 2) || marked(2, 0) {
			t.Fatal("Select All did not mark exactly the data cells")
		}
		dispatchModified(u, tcell.KeyRune, ']', 0)
		if marked(3, 3) {
			t.Fatal("selection leaked to next result")
		}
	})

}

func TestGridDragAndSortWithLiveEvents(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"A", "B", "C"})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"three", "one", "two"} {
		if err := s.Append([]result.Cell{{Text: text}, {Text: text}, {Text: text}}); err != nil {
			t.Fatal(err)
		}
	}
	var events []tcell.Event
	drained := make(chan struct{}, 1)
	u.app.QueueUpdateDraw(func() {
		u.active().Results = []*result.Store{s}
		u.showResults()
		u.app.SetFocus(u.table)
		u.app.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
			if k.Key() == tcell.KeyF8 {
				drained <- struct{}{}
			}
			return u.key(k)
		})
	})
	u.app.QueueUpdateDraw(func() {
		x, y, w, h := u.table.GetInnerRect()
		for _, target := range []struct {
			row, col int
			buttons  tcell.ButtonMask
		}{
			{1, 1, tcell.Button1}, {2, 2, tcell.Button1},
			{3, 3, tcell.ButtonNone}, {1, 1, tcell.ButtonNone},
		} {
			found := false
			for dy := 0; dy < h && !found; dy++ {
				for dx := range w {
					if r, c := u.table.CellAt(x+dx, y+dy); r == target.row && c == target.col {
						var mods tcell.ModMask
						if target.buttons != tcell.ButtonNone {
							mods = tcell.ModShift
						}
						events = append(events, tcell.NewEventMouse(x+dx, y+dy, target.buttons, mods))
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("cell (%d,%d) not rendered", target.row, target.col)
			}
		}
	})
	for _, event := range events {
		u.app.QueueEvent(event)
	}
	u.app.QueueEvent(tcell.NewEventKey(tcell.KeyF8, 0, 0))
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("UI did not process mouse events")
	}
	marked := func(row, col int) bool {
		_, bg, _ := u.table.GetCell(row, col).Style.Decompose()
		return bg == u.palette.Selection
	}
	copied := make(chan string, 1)
	u.app.QueueUpdateDraw(func() {
		if !marked(1, 3) || !marked(3, 1) {
			t.Error("drag highlight did not survive releasing Shift")
		}
		dispatchModified(u, tcell.KeyUp, 0, 0)
		if marked(1, 3) || marked(3, 1) || !marked(2, 3) {
			t.Error("moving focus did not leave only the focused cell highlighted")
		}
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		dispatchModified(u, tcell.KeyCtrlC, 0, 0)
	})
	if got := receiveCopy(t, copied); got != "one" {
		t.Errorf("copied %q after moving focus, want one", got)
	}
	for _, event := range events {
		u.app.QueueEvent(event)
	}
	u.app.QueueEvent(tcell.NewEventKey(tcell.KeyF8, 0, 0))
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("UI did not process second drag")
	}
	u.app.QueueUpdateDraw(func() {
		if !marked(1, 3) || !marked(3, 1) {
			t.Error("second mouse rectangle was not marked")
		}
		dispatchModified(u, tcell.KeyRune, 's', 0)
	})
	waitUI(t, u, func() bool { return u.active().SortDescription != "" })
	u.app.QueueUpdateDraw(func() {
		if marked(1, 2) || marked(2, 1) {
			t.Error("sorting did not clear the selection")
		}
	})
}

func TestGridKeyboardRectangleRemainsCopyTargetAfterDown(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"Letter", "Value"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range [][2]string{{"A", "uno"}, {"B", "dos"}, {"C", "tres"}} {
		if err := s.Append([]result.Cell{{Text: row[0]}, {Text: row[1]}}); err != nil {
			s.Close()
			t.Fatal(err)
		}
	}
	copied := make(chan string, 1)
	u.app.QueueUpdateDraw(func() {
		u.active().Results = []*result.Store{s}
		u.showResults()
		u.app.SetFocus(u.table)
		u.table.Select(1, 1)
		dispatchModified(u, tcell.KeyRight, 0, tcell.ModShift)
		dispatchModified(u, tcell.KeyDown, 0, tcell.ModShift)
		dispatchModified(u, tcell.KeyDown, 0, 0)
		if row, col := u.table.GetSelection(); row != 3 || col != 2 {
			t.Errorf("focus is (%d,%d), want (3,2)", row, col)
		}
		for _, cell := range [][2]int{{1, 1}, {2, 2}} {
			_, bg, _ := u.table.GetCell(cell[0], cell[1]).Style.Decompose()
			if bg != u.palette.Selection {
				t.Errorf("rectangle cell %v lost its highlight", cell)
			}
		}
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		dispatchModified(u, tcell.KeyRune, 'y', 0)
		if u.dialogContent != nil {
			t.Error("copy opened a dialog")
		}
	})
	if got, want := receiveCopy(t, copied), "A\tuno\nB\tdos\n"; got != want {
		t.Errorf("copied %q, want %q", got, want)
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

func TestGridCopyExplicitSelection(t *testing.T) {
	for _, tc := range []struct {
		name, action, want string
		keys               []tcell.Key
		y                  bool
	}{
		{"rectangle via Ctrl+C", "", "'@José\t\\N\n\"a\tb\nc\"\t\n", []tcell.Key{tcell.KeyLeft, tcell.KeyUp}, false},
		{"all via y", "", "skip\t'@José\t\\N\nskip\t\"a\tb\nc\"\t\n", []tcell.Key{tcell.KeyCtrlA}, true},
		{"rectangle with headers", "Copy with Headers", "'=dup\t'=dup\n'@José\t\\N\n\"a\tb\nc\"\t\n", []tcell.Key{tcell.KeyLeft, tcell.KeyUp}, false},
		{"headers only", "Copy Headers", "'=dup\t'=dup\n", []tcell.Key{tcell.KeyLeft, tcell.KeyUp}, false},
		{"one cell vertical header", "Copy with Headers", "'=dup\n\"a\tb\nc\"\n", nil, false},
		{"Y copies rectangle", "Y", "'=dup\t'=dup\n'@José\t\\N\n\"a\tb\nc\"\t\n", []tcell.Key{tcell.KeyLeft, tcell.KeyUp}, false},
		{"Y copies focused cell without a mark", "Y", "'=dup\n\"a\tb\nc\"\n", nil, false},
		{"v selects full rows for Y", "vY", "other\t'=dup\t'=dup\nskip\t'@José\t\\N\nskip\t\"a\tb\nc\"\t\n", []tcell.Key{tcell.KeyLeft}, false},
		{"Results menu copies only headers", "menu", "'=dup\t'=dup\n", []tcell.Key{tcell.KeyLeft, tcell.KeyUp}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var u *UI
			if tc.action == "menu" {
				u = runningNeovimUI(t)
			} else {
				u = runningUI(t, config.Config{})
			}
			s, err := result.New([]string{"other", "=dup", "=dup"})
			if err != nil {
				t.Fatal(err)
			}
			for _, cells := range [][]result.Cell{
				{{Text: "skip"}, {Text: "a\tb\nc"}, {}},
				{{Text: "skip"}, {Text: "@José"}, {Null: true}},
			} {
				if err := s.Append(cells); err != nil {
					t.Fatal(err)
				}
			}
			copied := make(chan string, 1)
			found := false
			u.app.QueueUpdateDraw(func() {
				u.active().Results = []*result.Store{s}
				u.active().Order = []int{1, 0}
				u.showResults()
				u.app.SetFocus(u.table)
				u.writeClipboard = func(text string) error { copied <- text; return nil }
				col := 3
				if len(tc.keys) == 0 {
					col = 2
				}
				u.table.Select(2, col)
				for _, key := range tc.keys {
					var mods tcell.ModMask
					if key != tcell.KeyCtrlA {
						mods = tcell.ModShift
					}
					dispatchModified(u, key, 0, mods)
				}
				switch {
				case tc.action == "Y" || tc.action == "vY":
					if tc.action == "vY" {
						dispatchModified(u, tcell.KeyRune, 'v', 0)
						dispatchModified(u, tcell.KeyUp, 0, 0)
					}
					found = true
					dispatchModified(u, tcell.KeyRune, 'Y', tcell.ModShift)
				case tc.action == "menu":
					u.app.SetFocus(u.active().Editor.Area)
					dispatchModified(u, tcell.KeyRune, ' ', 0)
					if u.leaderPanel == nil {
						t.Error("Space did not open Results menu navigation")
						return
					}
					dispatchModified(u, tcell.KeyRune, 'r', 0)
					dispatchModified(u, tcell.KeyRune, 'h', 0)
					found = u.leaderPanel == nil
				case tc.action != "":
					for _, c := range u.commands() {
						if c.Name == tc.action && (c.Available == nil || c.Available()) {
							found = true
							c.Run()
							break
						}
					}
				case tc.y:
					found = true
					dispatchModified(u, tcell.KeyRune, 'y', 0)
				default:
					found = true
					dispatchModified(u, tcell.KeyCtrlC, 0, 0)
				}
				if u.dialogContent != nil {
					t.Error("copy opened a dialog")
				}
			})
			if !found {
				t.Fatalf("action %q unavailable", tc.action)
			}
			if got := receiveCopy(t, copied); got != tc.want {
				t.Fatalf("copied %q, want %q", got, tc.want)
			}
		})
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/result"
)

func TestLeaderRender(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		u, screen := modeTestUI(t)
		screen.SetSize(size[0], size[1])
		u.leaderMenu()
		for _, submenu := range []bool{false, true} {
			if submenu {
				u.leaderPanel.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'w', 0))
			}
			u.app.ForceDraw()
			var rendered strings.Builder
			for y := 0; y < size[1]; y++ {
				for x := 0; x < size[0]; x++ {
					r, _, _, _ := screen.GetContent(x, y)
					rendered.WriteRune(r)
				}
				rendered.WriteByte('\n')
			}
			want := "Files"
			if submenu {
				want = "Balanced"
			}
			if !strings.Contains(rendered.String(), want) {
				t.Fatalf("%v submenu=%v: %s", size, submenu, rendered.String())
			}
			t.Logf("rendered %dx%d submenu=%v: %s visible", size[0], size[1], submenu, want)
		}
	}
}

func TestLeaderMaps(t *testing.T) {
	u, _ := modeTestUI(t)
	expected := map[rune]string{'f': "ensS", 'b': "d", 'c': "xAptTiq", 'd': "cbdre", 's': "odgh", 'r': "yYhvem", 'u': "wlCi", 'w': "bedmrc><+-", 'q': "q"}
	groups := u.leaderGroups()
	for key, keys := range expected {
		var got strings.Builder
		for _, c := range groups[key] {
			got.WriteRune(c.Key)
			if c.Run == nil || c.Name == "" {
				t.Errorf("missing command %c/%c", key, c.Key)
			}
		}
		if got.String() != keys {
			t.Errorf("%c: %q want %q", key, got.String(), keys)
		}
	}
}

func TestLeaderAvailabilityAndCase(t *testing.T) {
	u, _ := modeTestUI(t)
	groups := u.leaderGroups()
	for _, test := range []struct {
		group, key rune
		name       string
	}{{'f', 'e', "Open SQL"}, {'f', 'n', "New session"}, {'b', 'd', "Close tab"}, {'u', 'w', "Word wrap"}, {'u', 'l', "Line numbers"}, {'u', 'C', "Themes"}, {'u', 'i', "Language / Idioma"}, {'f', 's', "Save SQL"}, {'f', 'S', "Save SQL as"}, {'c', 'A', "Apply definition"}, {'c', 't', "New routine template"}, {'c', 'T', "New table template"}} {
		found := false
		for _, c := range groups[test.group] {
			if c.Key == test.key {
				found = c.Name == test.name
			}
		}
		if !found {
			t.Errorf("%c/%c: want %s", test.group, test.key, test.name)
		}
	}
	u.leaderMenu()
	capture := u.leaderPanel.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', 0))
	capture(tcell.NewEventKey(tcell.KeyRune, 'x', 0))
	if u.active().Busy || u.active().Execution != nil {
		t.Fatal("unavailable execution ran")
	}
	if u.active().Status != "Unavailable in this context: Execute SQL" {
		t.Fatalf("unavailable status: %q", u.active().Status)
	}
}

func TestLeaderExcludedModesAndPanes(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		for _, mode := range []string{"INSERT", "VISUAL", "V-LINE", "REPLACE"} {
			e.Mode = mode
			u.app.SetFocus(e.Area)
			u.key(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
			if u.leaderPanel != nil {
				t.Errorf("Space opened menu in %s", mode)
			}
		}
		e.Mode = "NORMAL"
		for _, pane := range []tview.Primitive{u.objects, u.search, u.kind} {
			u.app.SetFocus(pane)
			u.key(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
			if u.leaderPanel != nil {
				t.Errorf("Space opened menu in %T", pane)
			}
		}
		u.app.SetFocus(e.Area)
		u.key(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModAlt))
		if u.leaderPanel != nil {
			t.Error("modified Space opened menu")
		}
	})
}

func TestLeaderResultsFromGrid(t *testing.T) {
	u, screen := modeTestUI(t)
	u.app.SetFocus(u.table)
	dispatchModified(u, tcell.KeyRune, ' ', tcell.ModAlt)
	if u.leaderPanel != nil {
		t.Fatal("modified Space opened the Results menu")
	}
	open := func() {
		t.Helper()
		dispatchModified(u, tcell.KeyRune, ' ', 0)
		u.app.ForceDraw()
		if u.leaderPanel == nil || u.dialogContent != nil || u.app.GetFocus() != u.table {
			t.Fatal("Space did not keep a nonmodal Results menu on the focused grid")
		}
		var rendered strings.Builder
		width, height := screen.Size()
		for y := 0; y < height; y++ {
			for x := width / 2; x < width; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				rendered.WriteRune(r)
			}
			rendered.WriteByte('\n')
		}
		menu := rendered.String()
		for _, label := range []string{"h  Copy Headers", "Y  Copy with Headers", "v  Toggle row range"} {
			if !strings.Contains(menu, label) {
				t.Errorf("Results menu is missing %q", label)
			}
		}
		if strings.Contains(menu, "f  Files") || strings.Contains(menu, "Inspect cell") {
			t.Error("expected Results menu without the cell inspector")
		}
	}
	for _, cancel := range []tcell.Key{tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyEscape} {
		open()
		dispatchModified(u, tcell.KeyRune, 'r', 0)
		dispatchModified(u, cancel, 0, 0)
		if u.leaderPanel != nil || u.app.GetFocus() != u.table {
			t.Fatalf("%v did not cancel the Results menu in place", cancel)
		}
	}
	open()
	u.app.SetFocus(u.search)
	dispatchModified(u, tcell.KeyRune, 'h', 0)
	if u.leaderPanel != nil || u.search.GetText() != "h" {
		t.Fatal("Results menu captured input after focus moved")
	}
	u.app.SetFocus(u.table)
	open()
	u.app.SetFocus(u.search)
	u.app.ForceDraw()
	if u.leaderPanel != nil {
		t.Fatal("Results menu was not dismissed on redraw after focus moved")
	}
	t.Run("technical messages still copy", testTechnicalMessagesCopy)
}

func TestGridInspectorShortcutRemoved(t *testing.T) {
	u, _ := modeTestUI(t)
	s, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.Append([]result.Cell{{Text: "original"}}); err != nil {
		t.Fatal(err)
	}
	u.active().Results = []*result.Store{s}
	u.showResults()
	u.app.SetFocus(u.table)
	status := u.active().Status
	check := func() {
		t.Helper()
		row, col := u.table.GetSelection()
		if u.dialogContent != nil || !u.table.HasFocus() || row != 1 || col != 1 || u.active().Status != status || u.active().Busy {
			t.Error("removed i changed dialog, focus, cell or execution state")
		}
	}
	dispatchModified(u, tcell.KeyRune, 'i', 0)
	check()
	if u.dialogContent != nil {
		dispatchModified(u, tcell.KeyEscape, 0, 0)
	}
	dispatchModified(u, tcell.KeyRune, ' ', 0)
	dispatchModified(u, tcell.KeyRune, 'i', 0)
	check()
	if u.leaderPanel == nil {
		t.Error("removed i consumed the Results menu")
	}
}

func TestLeaderLayoutLeaves(t *testing.T) {
	for _, ckey := range "bedmrc><+-" {
		u, _ := modeTestUI(t)
		u.leaderMenu()
		capture := u.leaderPanel.GetInputCapture()
		capture(tcell.NewEventKey(tcell.KeyRune, 'w', 0))
		capture(tcell.NewEventKey(tcell.KeyRune, ckey, 0))
		if u.leaderPanel != nil {
			t.Fatalf("%c opened another dialog", ckey)
		}
		l := u.cfg.Layout
		ok := false
		switch ckey {
		case 'b':
			ok = l.EditorPercent == 45 && l.Maximized == "" && !l.HideCatalog
		case 'e':
			ok = l.EditorPercent == 70
		case 'd':
			ok = l.EditorPercent == 25
		case 'm':
			ok = l.Maximized == "editor" && l.HideCatalog
		case 'r':
			ok = l.Maximized == "results" && l.HideCatalog
		case 'c':
			ok = l.HideCatalog
		case '>':
			ok = l.CatalogWidth == 34
		case '<':
			ok = l.CatalogWidth == 26
		case '+':
			ok = l.EditorPercent == 55
		case '-':
			ok = l.EditorPercent == 35
		}
		if !ok {
			t.Errorf("%c: %+v", ckey, l)
		}
	}
}

func TestLeaderNavigationAndRemovedKeys(t *testing.T) {
	u, _ := modeTestUI(t)
	u.leaderMenu()
	root := u.leaderPanel
	capture := root.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'f', 0))
	text := root
	if !strings.Contains(text.GetText(true), "Save SQL as") {
		t.Fatal(text.GetText(true))
	}
	before := text.GetText(true)
	capture(tcell.NewEventKey(tcell.KeyRune, 'z', 0))
	if text.GetText(true) != before {
		t.Fatal("invalid key changed menu")
	}
	capture(tcell.NewEventKey(tcell.KeyBackspace2, 0, 0))
	if !strings.Contains(text.GetText(true), "Files") {
		t.Fatal("missing root")
	}
	for _, key := range []tcell.Key{tcell.KeyF11, tcell.KeyCtrlP} {
		u.cfg.Keys = map[string]string{"help": tcell.KeyNames[key]}
		u.key(tcell.NewEventKey(key, 0, 0))
		if u.leaderPanel != root {
			t.Fatalf("removed key %v changed dialog", key)
		}
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if u.leaderPanel != nil {
		t.Fatal("Escape did not dismiss")
	}
	t.Run("editor menu still ignores shifted keys outside Results", func(t *testing.T) {
		u := runningNeovimUI(t)
		u.app.QueueUpdateDraw(func() {
			dispatchModified(u, tcell.KeyRune, ' ', 0)
			root := u.leaderPanel
			if root == nil {
				t.Error("Space did not open the editor menu")
				return
			}
			before := root.GetText(true)
			dispatchModified(u, tcell.KeyRune, 'Y', tcell.ModShift)
			if u.leaderPanel != root || root.GetText(true) != before || u.dialogContent != nil {
				t.Error("Shift+Y changed the editor root menu")
			}
		})
	})
}

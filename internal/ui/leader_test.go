package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestLeaderRender(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		u, screen := modeTestUI(t)
		screen.SetSize(size[0], size[1])
		u.leaderMenu()
		for _, submenu := range []bool{false, true} {
			if submenu {
				u.dialogContent.(*tview.TextView).GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'w', 0))
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
	expected := map[rune]string{'f': "osS", 'b': "nd", 'c': "csaxAptTiq", 'd': "cbdre", 's': "odgh", 'r': "yiebm", 'u': "wntl", 'w': "bedmrc><+-", 'q': "q"}
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
	}{{'f', 's', "Save SQL"}, {'f', 'S', "Save SQL as"}, {'c', 'a', "Comment whole document"}, {'c', 'A', "Apply definition"}, {'c', 't', "New routine template"}, {'c', 'T', "New table template"}} {
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
	capture := u.dialogContent.(*tview.TextView).GetInputCapture()
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
			if u.dialogContent != nil {
				t.Errorf("Space opened menu in %s", mode)
			}
		}
		e.Mode = "NORMAL"
		for _, pane := range []tview.Primitive{u.table, u.objects, u.search, u.kind} {
			u.app.SetFocus(pane)
			u.key(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
			if u.dialogContent != nil {
				t.Errorf("Space opened menu in %T", pane)
			}
		}
		u.app.SetFocus(e.Area)
		u.key(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModAlt))
		if u.dialogContent != nil {
			t.Error("modified Space opened menu")
		}
	})
}

func TestLeaderLayoutLeaves(t *testing.T) {
	for _, ckey := range "bedmrc><+-" {
		u, _ := modeTestUI(t)
		u.leaderMenu()
		capture := u.dialogContent.(*tview.TextView).GetInputCapture()
		capture(tcell.NewEventKey(tcell.KeyRune, 'w', 0))
		capture(tcell.NewEventKey(tcell.KeyRune, ckey, 0))
		if u.dialogContent != nil {
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
	root := u.dialogContent
	capture := root.(interface {
		GetInputCapture() func(*tcell.EventKey) *tcell.EventKey
	}).GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'f', 0))
	text := root.(*tview.TextView)
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
		if u.dialogContent != root {
			t.Fatalf("removed key %v changed dialog", key)
		}
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if u.dialogContent != nil {
		t.Fatal("Escape did not dismiss")
	}
}

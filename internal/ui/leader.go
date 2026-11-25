package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (u *UI) leaderGroups() map[rune][]command {
	byName := make(map[string]command)
	for _, c := range u.commands() {
		byName[c.Name] = c
	}
	group := func(keys string, names ...string) []command {
		var out []command
		for i, key := range keys {
			c := byName[names[i]]
			c.Key = key
			out = append(out, c)
		}
		return out
	}
	return map[rune][]command{
		'f': group("ensS", "Open SQL", "New session", "Save SQL", "Save SQL as"),
		'b': group("d", "Close tab"),
		'c': group("xAptTiq", "Execute SQL", "Apply definition", "Invoke procedure / function", "New routine template", "New table template", "Insert object name", "Generate SELECT"),
		'd': group("cbdre", "Connections", "Change database", "Disconnect", "Refresh catalog", "Execution confirmations"),
		's': group("odgh", "Search objects", "View definition", "Grouped / flat catalog", "History"),
		'r': group("yYhvem", "Copy results", "Copy with Headers", "Copy Headers", "Toggle row range", "Export results", "Technical messages"),
		'u': group("wlCi", "Word wrap", "Line numbers", "Themes", "Language / Idioma"),
		'w': u.layoutCommands(),
		'q': group("q", "Quit"),
	}
}

type leaderOverlay struct {
	*tview.TextView
	origin tview.Primitive
}

func (u *UI) leaderMenu() {
	u.openLeader(u.active().Editor.Area, 0)
}

func (u *UI) openLeader(origin tview.Primitive, initialGroup rune) {
	groups := u.leaderGroups()
	byName := make(map[string]command)
	for _, c := range u.commands() {
		byName[c.Name] = c
	}
	labels := map[rune]string{'f': "Files", 'b': "Tabs", 'c': "Code", 'd': "Connections", 's': "Explore", 'r': "Results", 'u': "Preferences", 'w': "Layout", 'q': "Quit"}
	text := tview.NewTextView()
	text.SetBorder(true)
	current := initialGroup
	title := " Space · Esc cancel · Backspace parent "
	if initialGroup == 'r' {
		title = " Results · Esc or Backspace cancel "
	}
	text.SetTitle(u.tr(title))
	var entries []command
	rebuild := func() {
		entries = nil
		text.Clear()
		if current == 0 {
			for _, key := range "fbcdsruwq" {
				fmt.Fprintf(text, "%c  %s\n", key, u.tr(labels[key]))
			}
			entries = []command{byName["Help"], byName["Cancel operation"]}
			entries[0].Key = '?'
			entries[1].Key = 'x'
		} else {
			entries = groups[current]
		}
		for _, c := range entries {
			detail := ""
			if c.Available != nil && !c.Available() {
				detail = " · " + u.tr("Unavailable in this context")
			}
			fmt.Fprintf(text, "%c  %s%s\n", c.Key, u.tr(c.Name), detail)
		}
	}
	rebuild()
	text.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		switch k.Key() {
		case tcell.KeyEscape:
			u.leaderPanel = nil
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if initialGroup != 0 {
				u.leaderPanel = nil
				return nil
			}
			current = 0
			rebuild()
			return nil
		}
		shiftedResultsCopy := current == 'r' && k.Rune() == 'Y' && k.Modifiers() == tcell.ModShift
		if k.Key() != tcell.KeyRune || (k.Modifiers() != 0 && !shiftedResultsCopy) {
			return nil
		}
		if current == 0 {
			if _, ok := groups[k.Rune()]; ok {
				current = k.Rune()
				rebuild()
				return nil
			}
		}
		for _, c := range entries {
			if c.Key == k.Rune() {
				u.leaderPanel = nil
				if c.Available != nil && !c.Available() {
					u.message(u.tr("Unavailable in this context: ") + c.Name)
					return nil
				}
				c.Run()
				return nil
			}
		}
		return nil
	})
	u.style(text)
	text.SetWrap(false)
	u.leaderPanel = &leaderOverlay{TextView: text, origin: origin}
}

// drawLeader muestra las próximas teclas a la derecha sin cambiar el foco ni cubrir toda la sesión.
func (u *UI) drawLeader(screen tcell.Screen) {
	panel := u.leaderPanel
	if panel == nil {
		return
	}
	if !panel.origin.HasFocus() {
		u.leaderPanel = nil
		return
	}
	width, height := screen.Size()
	if width < 4 || height < 5 {
		return
	}
	panelWidth := min(46, width/2)
	panelHeight := min(14, height-4)
	panel.SetRect(width-panelWidth, height-panelHeight-2, panelWidth, panelHeight)
	panel.Draw(screen)
}

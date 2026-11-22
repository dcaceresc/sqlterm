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
		'f': group("osS", "Open SQL", "Save SQL", "Save SQL as"),
		'b': group("nd", "New session", "Close tab"),
		'c': group("csaxAptTiq", "Comment selection", "Comment selection", "Comment whole document", "Execute SQL", "Apply definition", "Invoke procedure / function", "New routine template", "New table template", "Insert object name", "Generate SELECT"),
		'd': group("cbdre", "Connections", "Change database", "Disconnect", "Refresh catalog", "Execution confirmations"),
		's': group("odgh", "Search objects", "View definition", "Grouped / flat catalog", "History"),
		'r': group("yiebm", "Copy results", "Inspect cell", "Export results", "Result block size", "Technical messages"),
		'u': group("wntl", "Word wrap", "Line numbers", "Themes", "Language / Idioma"),
		'w': u.layoutCommands(),
		'q': group("q", "Quit"),
	}
}

func (u *UI) leaderMenu() {
	groups := u.leaderGroups()
	byName := make(map[string]command)
	for _, c := range u.commands() {
		byName[c.Name] = c
	}
	labels := map[rune]string{'f': "Files", 'b': "Tabs", 'c': "Code", 'd': "Connections", 's': "Explore", 'r': "Results", 'u': "Preferences", 'w': "Layout", 'q': "Quit"}
	text := tview.NewTextView()
	text.SetBorder(true)
	current := rune(0)
	var entries []command
	rebuild := func() {
		entries = nil
		text.Clear()
		text.SetTitle(u.tr(" Space · Esc cancel · Backspace parent "))
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
			u.dismiss()
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			current = 0
			rebuild()
			return nil
		}
		if k.Key() != tcell.KeyRune || k.Modifiers() != 0 {
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
				u.dismiss()
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
	u.popup(text, 72, 16)
}

package ui

import "sqlterm/internal/config"

func (u *UI) configureEditor(t *Tab) {
	t.Editor.SetWordWrap(u.cfg.Editor.WordWrap)
	numbers := u.cfg.Editor.LineNumbers == nil || *u.cfg.Editor.LineNumbers
	t.Editor.SetLineNumbers(numbers)
}

func (u *UI) toggleEditorOption(wrap bool) {
	next := u.cfg
	if wrap {
		next.Editor.WordWrap = !next.Editor.WordWrap
	} else {
		enabled := next.Editor.LineNumbers != nil && !*next.Editor.LineNumbers
		next.Editor.LineNumbers = &enabled
	}
	if err := config.Save(u.configPath, next); err != nil {
		u.message(err.Error())
		return
	}
	u.cfg = next
	u.hideSuggestions()
	for _, t := range u.tabs {
		u.configureEditor(t)
	}
	label, state := "Line numbers", "Off"
	if wrap {
		label = "Word wrap"
		if next.Editor.WordWrap {
			state = "On"
		}
	} else if next.Editor.LineNumbers == nil || *next.Editor.LineNumbers {
		state = "On"
	}
	u.message(u.tr(label) + ": " + u.tr(state))
}

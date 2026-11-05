package ui

import (
	"github.com/gdamore/tcell/v2"
	"strings"
)

// Configured shortcuts are global aliases. Restrict them to special keys so
// ordinary typing can never accidentally execute SQL.
func (u *UI) customKey(k *tcell.EventKey) bool {
	if k.Key() == tcell.KeyRune {
		return false
	}
	normalize := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "+", "-")) }
	name := normalize(tcell.KeyNames[k.Key()])
	if name == "" {
		return false
	}
	for action, binding := range u.cfg.Keys {
		if normalize(binding) != name {
			continue
		}
		switch action {
		case "database":
			u.switchDatabase()
		case "complete":
			u.complete()
		case "theme":
			u.chooseTheme()
		case "run":
			u.execute()
		case "connect":
			u.chooseConnection()
		case "invoke":
			u.invokeSelected()
		case "cancel":
			if u.active().Cancel != nil {
				u.active().Cancel()
			}
		case "help":
			u.showHelp()
		case "save":
			u.saveFile()
		case "open":
			u.openFile()
		case "new":
			u.newTab(u.active())
		case "apply":
			u.applyDefinition()
		case "refresh":
			u.refreshCatalog()
		default:
			return false
		}
		return true
	}
	return false
}

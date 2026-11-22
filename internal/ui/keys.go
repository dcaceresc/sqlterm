package ui

import (
	"github.com/gdamore/tcell/v2"
	"strings"
)

// Los alias globales excluyen las teclas retiradas y la escritura normal.
func (u *UI) customKey(k *tcell.EventKey) bool {
	if k.Key() == tcell.KeyRune || k.Key() == tcell.KeyF11 || k.Key() == tcell.KeyCtrlP {
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
			u.cancelOperation()
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

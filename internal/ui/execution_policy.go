package ui

import (
	"strings"

	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

// This controls an optional UX confirmation, never SQL permissions or safety.
func confirmExecution(policy, folder string, statements []string) bool {
	if policy == "all" {
		return true
	}
	if policy != "production" || !strings.EqualFold(strings.Split(folder, "/")[0], "PROD") {
		return false
	}
	for _, sql := range statements {
		words := strings.Fields(strings.ToUpper(strings.TrimSpace(sql)))
		if len(words) == 0 {
			continue
		}
		if words[0] != "SELECT" && words[0] != "SHOW" && words[0] != "EXPLAIN" {
			return true
		}
		if strings.Contains(" "+strings.Join(words, " ")+" ", " INTO ") {
			return true
		}
		for _, token := range strings.FieldsFunc(strings.Join(words, " "), func(r rune) bool { return !(r >= 'A' && r <= 'Z' || r == '_') }) {
			switch token {
			case "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP", "TRUNCATE", "MERGE", "CALL", "EXEC", "EXECUTE", "GRANT", "REVOKE", "DENY":
				return true
			}
		}
	}
	return false
}

func (u *UI) executionPolicyMenu() {
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(u.tr(" Execution confirmations · not a security control "))
	for _, item := range []struct{ value, label, detail string }{{"", "Off (existing behavior)", "F5 executes directly; Apply definition still requires review"}, {"production", "Production writes (heuristic)", "Confirm likely writes in PROD folders; SQL detection cannot guarantee safety"}, {"all", "Every F5 execution", "Confirm every script or selection, including reads"}} {
		choice := item
		list.AddItem(u.tr(choice.label), u.tr(choice.detail), 0, func() {
			next := u.cfg
			next.ConfirmExecution = choice.value
			if err := config.Save(u.configPath, next); err != nil {
				u.message(err.Error())
				return
			}
			u.cfg = next
			u.dismiss()
		})
	}
	u.popup(list, 100, 12)
}

package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

type selectorSearch struct {
	*tview.InputField
	results tview.Primitive
}

func newSelectorSearch(label string, results tview.Primitive) *selectorSearch {
	return &selectorSearch{InputField: tview.NewInputField().SetLabel(label), results: results}
}

func (s *selectorSearch) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		return s.InputField.MouseHandler()(action, event, func(p tview.Primitive) {
			if p == s.InputField {
				p = s
			}
			focus(p)
		})
	}
}

func (u *UI) clearSelectorSearch() bool {
	search, ok := u.app.GetFocus().(*selectorSearch)
	if !ok {
		return false
	}
	search.SetText("")
	u.app.SetFocus(search.results)
	return true
}

func (s *selectorSearch) key(u *UI, k *tcell.EventKey, hasResult bool) *tcell.EventKey {
	if s.HasFocus() {
		switch k.Key() {
		case tcell.KeyDown, tcell.KeyTab, tcell.KeyBacktab:
			u.app.SetFocus(s.results)
			return nil
		case tcell.KeyEnter:
			if hasResult {
				u.app.SetFocus(s.results)
				s.results.InputHandler()(k, func(p tview.Primitive) { u.app.SetFocus(p) })
			}
			return nil
		}
	} else if k.Key() == tcell.KeyRune && k.Rune() == '/' {
		u.app.SetFocus(s)
		return nil
	}
	return k
}

func (u *UI) databaseSelector(t *Tab, names []string) {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(u.tr(" Change database · / search · Enter reconnect "))
	search := newSelectorSearch(u.tr("Find database: "), list)
	detail := tview.NewTextView().SetWrap(true)
	filtered := []string{}
	update := func(i int) {
		if i < 0 || i >= len(filtered) {
			detail.SetText(u.tr("No matching databases"))
			return
		}
		detail.SetText(fmt.Sprintf(u.tr("%s → %s\nOpens a new connection. SQL is preserved. The old session closes only after success. Transactions, temporary tables and variables do not carry over."), t.Profile.Database, filtered[i]))
	}
	list.SetChangedFunc(func(i int, _, _ string, _ rune) { update(i) })
	rebuild := func(filter string) {
		list.Clear()
		filtered = nil
		for _, name := range names {
			if fuzzy(strings.ToLower(filter), strings.ToLower(name)) {
				filtered = append(filtered, name)
			}
		}
		for _, name := range filtered {
			n := name
			label := name
			if name == t.Profile.Database {
				label += " (" + u.tr("Current") + ")"
			}
			list.AddItem(tview.Escape(label), "", 0, func() { u.dismiss(); p := t.Profile; p.Database = n; u.connect(t, p, t.Password) })
		}
		update(0)
	}
	search.SetChangedFunc(rebuild)
	rebuild("")
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 1, 0, false).AddItem(list, 0, 1, true).AddItem(detail, 4, 0, false)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		return search.key(u, k, len(filtered) > 0)
	})
	u.popup(panel, 90, 24)
}

func profileID(p config.Profile) string {
	if p.CredentialID != "" {
		return p.CredentialID
	}
	return p.Engine + "|" + p.DisplayName() + "|" + p.Host
}
func stringIndex(values []string, value string) int {
	for i, s := range values {
		if s == value {
			return i
		}
	}
	return -1
}
func (u *UI) recordRecent(p config.Profile) {
	id := profileID(p)
	next := u.cfg
	next.RecentConnections = []string{id}
	for _, key := range u.cfg.RecentConnections {
		if key != id && len(next.RecentConnections) < 10 {
			next.RecentConnections = append(next.RecentConnections, key)
		}
	}
	if err := config.Save(u.configPath, next); err == nil {
		u.cfg = next
	} else {
		u.active().Messages = append(u.active().Messages, "Could not save recent connections: "+err.Error())
	}
}

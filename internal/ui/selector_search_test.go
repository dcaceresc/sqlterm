package ui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func dispatchSelector(u *UI, key tcell.Key, r rune) {
	if k := u.key(tcell.NewEventKey(key, r, 0)); k != nil {
		u.pages.InputHandler()(k, func(p tview.Primitive) { u.app.SetFocus(p) })
	}
}

func TestSelectorsFocusResultsAndSearchOnDemand(t *testing.T) {
	for _, name := range []string{"connections", "themes", "databases"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			u := New(path, config.Config{Profiles: []config.Profile{{Name: "Development", Engine: "postgres", Host: "localhost", User: "user"}}})
			defer u.cancel()
			switch name {
			case "connections":
				dispatchSelector(u, tcell.KeyF2, 0)
			case "themes":
				dispatchSelector(u, tcell.KeyF3, 0)
			case "databases":
				u.databaseSelector(u.active(), []string{"Development"})
			}
			results := u.app.GetFocus()
			switch results.(type) {
			case *tview.TreeView, *tview.List:
			default:
				t.Fatalf("initial focus is %T, want results", results)
			}
			dispatchSelector(u, tcell.KeyRune, '/')
			if u.app.GetFocus() == results {
				t.Fatal("slash did not focus search")
			}
			for _, r := range "zzzzzz" {
				dispatchSelector(u, tcell.KeyRune, r)
			}
			dispatchSelector(u, tcell.KeyEnter, 0)
			if u.dialogContent == nil {
				t.Fatal("Enter selected a nonexistent item")
			}
			dispatchSelector(u, tcell.KeyEscape, 0)
			if u.dialogContent == nil || u.app.GetFocus() != results {
				t.Fatal("Escape did not return to results")
			}
			if list, ok := results.(*tview.List); ok && list.GetItemCount() == 0 {
				t.Fatal("Escape did not clear filter")
			}
			dispatchSelector(u, tcell.KeyEscape, 0)
			if u.dialogContent != nil {
				t.Fatal("second Escape did not close selector")
			}
		})
	}
}

func TestThemeSearchPreviewAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	u := New(path, config.Config{Theme: "nord"})
	defer u.cancel()
	for _, id := range []string{"dracula", "solarized-light", "catppuccin-mocha", "everforest"} {
		if resolveTheme(id).ID != id {
			t.Errorf("missing palette %q", id)
		}
	}
	dispatchSelector(u, tcell.KeyF3, 0)
	dispatchSelector(u, tcell.KeyRune, '/')
	for _, r := range "solarized" {
		dispatchSelector(u, tcell.KeyRune, r)
	}
	if u.palette.ID != "solarized-light" || u.cfg.Theme != "nord" {
		t.Fatal("filter should preview without saving")
	}
	dispatchSelector(u, tcell.KeyEscape, 0)
	if u.palette.ID != "nord" {
		t.Fatal("clearing filter should restore initial preview")
	}
	dispatchSelector(u, tcell.KeyRune, '/')
	for _, r := range "dracula" {
		dispatchSelector(u, tcell.KeyRune, r)
	}
	dispatchSelector(u, tcell.KeyEnter, 0)
	loaded, err := config.Load(path)
	if err != nil || loaded.Theme != "dracula" || u.dialogContent != nil {
		t.Fatalf("Enter did not save and close: theme=%q, err=%v", loaded.Theme, err)
	}
}

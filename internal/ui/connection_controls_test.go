package ui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func TestConnectionFoldersToggleOnlyWithEnter(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Folders: []string{"DEV/Child"}})
	defer u.cancel()
	u.chooseConnection()
	_, page := u.pages.GetFrontPage()
	panel := page.(*centered).child.(*tview.Flex)
	tree := panel.GetItem(1).(*tview.TreeView)
	u.app.SetFocus(tree)
	folder := tree.GetRoot().GetChildren()[0]
	send := func(key tcell.Key, r rune) {
		tree.SetCurrentNode(folder)
		u.pages.InputHandler()(tcell.NewEventKey(key, r, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	}
	for _, expanded := range []bool{false, true} {
		for _, key := range []tcell.Key{tcell.KeyLeft, tcell.KeyRight, tcell.KeyRune} {
			folder.SetExpanded(expanded)
			send(key, ' ')
			if folder.IsExpanded() != expanded {
				t.Errorf("%v must not toggle folder expansion from %v", key, expanded)
			}
		}
		folder.SetExpanded(expanded)
		send(tcell.KeyEnter, 0)
		if folder.IsExpanded() == expanded {
			t.Errorf("Enter must toggle folder expansion from %v", expanded)
		}
	}
}

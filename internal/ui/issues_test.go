package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func TestEditorPreferencesKeepBufferAndApplyToNewTabs(t *testing.T) {
	u, s := modeTestUI(t)
	first := u.active()
	sql := "SELECT a_very_long_column_name FROM a_very_long_table_name;\nSELECT 2;"
	first.Editor.SetText(sql)
	u.app.ForceDraw()
	first.Editor.Area.Select(7, 29)
	if k := u.key(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt)); k != nil {
		t.Fatal("Alt+Z not consumed")
	}
	u.app.ForceDraw()
	text, from, to := first.Editor.Area.GetSelection()
	if !first.Editor.WordWrap() || first.Editor.Area.GetText() != sql || text != sql[7:29] || from != 7 || to != 29 {
		t.Fatal("wrap changed SQL or selection")
	}
	cfg, err := config.Load(u.configPath)
	if err != nil || !cfg.Editor.WordWrap {
		t.Fatal("wrap not persisted", err)
	}
	u.newTab(nil)
	second := u.active()
	if !second.Editor.WordWrap() {
		t.Fatal("new tab did not inherit wrap")
	}
	u.toggleEditorOption(true)
	if first.Editor.WordWrap() || second.Editor.WordWrap() {
		t.Fatal("wrap toggle not applied to existing tabs")
	}
	u.toggleEditorOption(false)
	u.app.ForceDraw()
	cfg, err = config.Load(u.configPath)
	if err != nil || cfg.Editor.LineNumbers == nil || *cfg.Editor.LineNumbers {
		t.Fatal("line number preference not saved", err)
	}
	if !strings.Contains(screenText(s), "Editor") {
		t.Fatal("editor hidden by preference change")
	}
}

func TestEditorPreferenceSaveFailureDoesNotApply(t *testing.T) {
	u, _ := modeTestUI(t)
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	u.configPath = filepath.Join(blocked, "config.json")
	u.toggleEditorOption(true)
	if u.cfg.Editor.WordWrap || u.active().Editor.WordWrap() {
		t.Fatal("failed save changed wrap")
	}
}

func TestEditorChangesKeepQuitConfirmationVisible(t *testing.T) {
	u, s := modeTestUI(t)
	s.SetSize(80, 24)
	u.active().Editor.SetText("SELECT 1;")
	u.confirm("Discard unsaved SQL and quit?", func() {})
	u.app.ForceDraw()
	text := screenText(s)
	for _, label := range []string{"Discard", "Cancel", "Confirm"} {
		if !strings.Contains(text, label) {
			t.Fatalf("missing %q in quit confirmation:\n%s", label, text)
		}
	}
}

func TestFavoritesSectionIsFirstAcrossFoldersAndPersists(t *testing.T) {
	u, s := modeTestUI(t)
	u.cfg.Profiles = []config.Profile{{Name: "Same", Folder: "DEV", Engine: "sqlserver", Host: "dev-host", User: "u", CredentialID: "dev-id"}, {Name: "Same", Folder: "PROD", Engine: "postgres", Host: "prod-host", User: "u", CredentialID: "prod-id"}}
	u.chooseConnection()
	panel := u.dialogContent.(*tview.Flex)
	search := panel.GetItem(0).(*tview.InputField)
	tree := panel.GetItem(1).(*tview.TreeView)
	buttons := panel.GetItem(3).(*tview.Form)
	search.SetText("prod-host")
	u.app.ForceDraw()
	if ref := tree.GetCurrentNode().GetReference().(connectionRef); ref.ProfileIndex != 1 || ref.FolderNode {
		t.Fatal("wrong profile selected")
	}
	buttons.GetButton(buttons.GetButtonCount()-1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	search.SetText("")
	u.app.ForceDraw()
	first := tree.GetRoot().GetChildren()[0]
	ref := first.GetReference().(connectionRef)
	if !ref.FavoriteGroup || len(first.GetChildren()) != 1 {
		t.Fatal("favorites not above DEV/QA/PROD")
	}
	leaf := first.GetChildren()[0]
	ref = leaf.GetReference().(connectionRef)
	if ref.ProfileIndex != 1 || ref.Folder != "PROD" {
		t.Fatal("same-name favorite points at wrong destination")
	}
	if !strings.Contains(screenText(s), "Favorites") {
		t.Fatal("favorites section not visible")
	}
	cfg, err := config.Load(u.configPath)
	if err != nil || len(cfg.Favorites) != 1 || cfg.Favorites[0] != "prod-id" {
		t.Fatal("favorite not saved", err)
	}
	reopened := New(u.configPath, cfg)
	defer reopened.cancel()
	reopened.connectionFolder = "DEV"
	reopened.chooseConnection()
	otherTree := reopened.dialogContent.(*tview.Flex).GetItem(1).(*tview.TreeView)
	if ref := otherTree.GetCurrentNode().GetReference().(connectionRef); ref.FolderNode || ref.ProfileIndex != 1 {
		t.Fatal("reopening connections did not prioritize saved favorite")
	}
	u.app.SetFocus(tree)
	tree.SetCurrentNode(leaf)
	u.pages.InputHandler()(tcell.NewEventKey(tcell.KeyRune, '*', 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	u.app.ForceDraw()
	if len(u.cfg.Favorites) != 0 {
		t.Fatal("* did not remove favorite")
	}
	for _, node := range tree.GetRoot().GetChildren() {
		if node.GetReference().(connectionRef).FavoriteGroup {
			t.Fatal("empty favorites section remained")
		}
	}
	if len(u.cfg.Profiles) != 2 || u.cfg.Profiles[0].CredentialID != "dev-id" || u.cfg.Profiles[1].CredentialID != "prod-id" {
		t.Fatal("favorite action changed profiles or credentials")
	}
}

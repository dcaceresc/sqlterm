package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/config"
	"strings"
	"testing"
)

func TestConnectionTreeFolderFilteringAndProfileIdentity(t *testing.T) {
	c := config.Config{Folders: []string{"DEV", "QA", "PROD", "TEAM/Empty"}, Profiles: []config.Profile{{Name: "Shared", Folder: "PROD", Host: "prod"}, {Name: "Shared", Folder: "DEV/API", Host: "dev"}, {Name: "Local"}}}
	root := connectionRoot(c, "")
	folders := map[string]bool{}
	leaves := map[int]string{}
	root.Walk(func(n, parent *tview.TreeNode) bool {
		ref := n.GetReference().(connectionRef)
		if ref.FolderNode {
			folders[ref.Folder] = true
		} else {
			leaves[ref.ProfileIndex] = ref.Folder
		}
		return true
	})
	for _, path := range []string{"DEV", "DEV/API", "QA", "PROD", "TEAM", "TEAM/Empty"} {
		if !folders[path] {
			t.Fatal("missing folder", path)
		}
	}
	if len(leaves) != 3 || leaves[0] != "PROD" || leaves[1] != "DEV/API" {
		t.Fatal("profile indexes changed with sorting")
	}
	filtered := connectionRoot(c, "prod")
	count := 0
	filtered.Walk(func(n, parent *tview.TreeNode) bool {
		ref := n.GetReference().(connectionRef)
		if !ref.FolderNode {
			count++
			if ref.ProfileIndex != 0 {
				t.Fatal("wrong environment in filter")
			}
		}
		return true
	})
	if count != 1 {
		t.Fatal("filter lost profile")
	}
}
func TestFoldersRenameMoveAndCredentialIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	dev := config.Profile{Name: "Shared", Folder: "DEV/API", Engine: "sqlserver", Host: "dev", User: "u", CredentialID: "dev"}
	prod := dev
	prod.Folder = "PROD"
	prod.Host = "prod"
	prod.CredentialID = "prod"
	u := New(path, config.Config{Profiles: []config.Profile{dev, prod}})
	defer u.cancel()
	vault := &memoryVault{values: map[string]string{"dev": "dev password", "prod": "prod password"}}
	u.vault = vault
	u.active().Profile = dev
	if err := u.deleteFolder("DEV"); err == nil {
		t.Fatal("deleted folder containing connections")
	}
	if err := u.renameFolder("DEV", "Development"); err != nil {
		t.Fatal(err)
	}
	if u.cfg.Profiles[0].Folder != "Development/API" || u.active().Profile.Folder != "Development/API" || u.cfg.Profiles[0].CredentialID != "dev" {
		t.Fatal("rename lost subtree or active identity")
	}
	if err := u.rememberPassword(prod, "new prod password"); err != nil {
		t.Fatal(err)
	}
	if vault.values["dev"] != "dev password" || u.cfg.Profiles[0].CredentialID != "dev" {
		t.Fatal("same-name profile changed another environment's credential")
	}
	u.profileForm(0)
	f := currentForm(u)
	f.GetFormItem(9).(*tview.InputField).SetText("QA")
	saveForm(u, f)
	if u.cfg.Profiles[0].Folder != "QA" || u.cfg.Profiles[0].CredentialID != "dev" || u.active().Profile.Folder != "QA" {
		t.Fatal("moving a profile lost credential or active identity")
	}
	loaded, err := config.Load(path)
	if err != nil || loaded.Profiles[0].Folder != "QA" {
		t.Fatal("folder not persisted", err)
	}
	if err = u.addFolder("TEAM/Empty"); err != nil {
		t.Fatal(err)
	}
	if err = u.renameFolder("TEAM", "Workspace"); err != nil {
		t.Fatal(err)
	}
	if err = u.deleteFolder("Workspace"); err != nil {
		t.Fatal(err)
	}
	for _, p := range config.FolderPaths(u.cfg) {
		if strings.HasPrefix(p, "Workspace") {
			t.Fatal("empty folder not removed")
		}
	}
	if err = u.renameFolder("QA", "QA/Nested"); err == nil {
		t.Fatal("moved a folder into itself")
	}
	if err = u.renameFolder("QA", "PROD"); err == nil {
		t.Fatal("overwrote existing environment")
	}
}
func TestAddProfileInSelectedFolderAndKeyboardNavigation(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	u.connectionFolder = "PROD"
	u.chooseConnection()
	_, page := u.pages.GetFrontPage()
	panel := page.(*centered).child.(*tview.Flex)
	tree := panel.GetItem(1).(*tview.TreeView)
	ref := tree.GetCurrentNode().GetReference().(connectionRef)
	if ref.Folder != "PROD" {
		t.Fatal("folder selection not restored")
	}
	u.pages.InputHandler()(tcell.NewEventKey(tcell.KeyRune, '+', 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	f := currentForm(u)
	if f.GetFormItem(9).(*tview.InputField).GetText() != "PROD" {
		t.Fatal("new profile not assigned to selected folder")
	}
}

package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

func folderGroupKey(u *UI, key rune) {
	u.pages.InputHandler()(tcell.NewEventKey(tcell.KeyRune, key, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
}

func selectFolderGroup(t *testing.T, u *UI, path string) *tview.TreeView {
	t.Helper()
	panel := u.dialogContent.(*tview.Flex)
	tree := panel.GetItem(1).(*tview.TreeView)
	tree.GetRoot().Walk(func(n, _ *tview.TreeNode) bool {
		ref := n.GetReference().(connectionRef)
		if ref.FolderNode && !ref.FavoriteGroup && ref.Folder == path {
			tree.SetCurrentNode(n)
		}
		return true
	})
	if tree.GetCurrentNode().GetReference().(connectionRef).Folder != path {
		t.Fatalf("folder %q not visible", path)
	}
	return tree
}

func folderGroupForm(t *testing.T, u *UI) (*tview.Form, *tview.InputField, *tview.DropDown, *tview.DropDown) {
	t.Helper()
	f, ok := u.dialogContent.(*tview.Form)
	if !ok || f.GetFormItemCount() != 3 {
		t.Fatalf("expected Name/Parent group/Color form, got %T", u.dialogContent)
	}
	name := f.GetFormItem(0).(*tview.InputField)
	parent := f.GetFormItem(1).(*tview.DropDown)
	color := f.GetFormItem(2).(*tview.DropDown)
	if name.GetLabel() != "Name" || parent.GetLabel() != "Parent group" || color.GetLabel() != "Color" {
		t.Fatal("incorrect group fields")
	}
	return f, name, parent, color
}

func TestConnectionFolderGroupMove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	initial := []byte(`{"folders":["QA","QA/Existing","DEV/API","DEV/Empty"],"folder_colors":{"DEV":"green","DEV/API":"purple","DEV/Empty":"blue"},"profiles":[{"name":"Main","engine":"postgres","host":"db","user":"u","folder":"DEV/API","credential_id":"saved"}],"confirm_execution":"all"}`)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u := New(path, c)
	defer u.cancel()
	u.active().Profile = c.Profiles[0]
	session := &db.Session{Adapter: db.NewAdapter("postgres")}
	u.active().Session = session
	u.active().Editor.SetText("SELECT 1;")
	u.chooseConnection()
	selectFolderGroup(t, u, "DEV")
	folderGroupKey(u, 'e')
	f, name, parent, color := folderGroupForm(t, u)
	if _, label := color.GetCurrentOption(); name.GetText() != "DEV" || label != "Green" {
		t.Fatalf("edit fields not prefilled: %q %q", name.GetText(), label)
	}
	for i, want := range []string{"(Root)", "QA", "QA/Existing"} {
		parent.SetCurrentOption(i)
		if _, got := parent.GetCurrentOption(); got != want {
			t.Fatalf("parent option %d = %q, want %q", i, got, want)
		}
	}
	parent.SetCurrentOption(3)
	if i, label := parent.GetCurrentOption(); i >= 0 || label != "" {
		t.Fatal("self or descendant exposed as an extra parent")
	}
	parent.SetCurrentOption(1)
	for value, message := range map[string]string{"Existing": "target folder already exists", "Bad/Leaf": "folder name cannot contain / or \\", " ": "folder name is required", "..": "folder components cannot be empty, . or .."} {
		name.SetText(value)
		saveForm(u, f)
		stored, err := os.ReadFile(path)
		if u.dialogContent != f || !strings.Contains(f.GetTitle(), message) || err != nil || !bytes.Equal(stored, initial) || u.active().Profile != c.Profiles[0] {
			t.Fatalf("rejection for %q: title=%q, readErr=%v", value, f.GetTitle(), err)
		}
	}
	name.SetText("Team")
	color.SetCurrentOption(1)
	u.configPath = filepath.Join(path, "cannot-save.json")
	saveForm(u, f)
	stored, err := os.ReadFile(path)
	if u.dialogContent != f || err != nil || !bytes.Equal(stored, initial) || u.active().Profile != c.Profiles[0] || !reflect.DeepEqual(u.cfg, c) || u.cfg.FolderColors["DEV"] != "green" {
		t.Fatal("failed config save changed persisted or open profile state")
	}
	u.configPath = path
	color.SetCurrentOption(2)
	saveForm(u, f)
	if u.active().Profile.Folder != "QA/Team/API" || u.active().Profile.CredentialID != "saved" || u.active().Editor.Area.GetText() != "SELECT 1;" || u.active().Session != session {
		t.Fatal("move lost open profile state")
	}
	loaded, err := config.Load(path)
	stored, readErr := os.ReadFile(path)
	if err != nil || readErr != nil || loaded.Profiles[0].Folder != "QA/Team/API" || loaded.ConfirmExecution != "all" || bytes.Contains(stored, []byte(`"DEV`)) {
		t.Fatalf("move not persisted, policy changed, or old paths remain: %s %v %v", stored, err, readErr)
	}
	for _, want := range []string{`"QA/Team": "green"`, `"QA/Team/API": "purple"`, `"QA/Team/Empty": "blue"`} {
		if !bytes.Contains(stored, []byte(want)) {
			t.Fatalf("missing moved color %s: %s", want, stored)
		}
	}
	selectFolderGroup(t, u, "QA/Team")
	folderGroupKey(u, 'r')
	f, _, parent, _ = folderGroupForm(t, u)
	parent.SetCurrentOption(0)
	saveForm(u, f)
	if u.active().Profile.Folder != "Team/API" {
		t.Fatal("legacy r did not move to root using the same editor")
	}
	u.dismiss()
	dispatchModified(u, tcell.KeyF5, 0, 0)
	if _, ok := u.dialogContent.(*tview.Modal); !ok || u.active().Busy {
		t.Fatal("F5 no longer confirms a read under the saved all policy")
	}
	u.dismiss()
	u.chooseConnection()
	tree := selectFolderGroup(t, u, "Team/API")
	tree.SetCurrentNode(tree.GetCurrentNode().GetChildren()[0])
	folderGroupKey(u, 'e')
	profile := currentForm(u)
	if profile.GetFormItem(9).(*tview.InputField).GetText() != "Team/API" {
		t.Fatal("e did not preserve profile editing")
	}
}

func TestTabHeaderFolderColorAndTreeTheme(t *testing.T) {
	u, s := modeTestUI(t)
	u.cfg.Folders = []string{"PROD/Deep", "QA", "DEV"}
	u.cfg.FolderColors = map[string]string{"PROD": "green", "PROD/Deep": "purple"}
	u.active().Profile = config.Profile{Name: "db", Folder: "PROD/Deep/Child"}
	u.chooseConnection()
	selectFolderGroup(t, u, "QA")
	u.applyTheme("nord")
	u.app.ForceDraw()
	rows := strings.Split(screenText(s), "\n")
	for _, check := range []struct {
		text  string
		row   int
		color tcell.Color
	}{
		{"PROD │ db", 0, tcell.ColorPurple},
		{"PROD/", -1, tcell.ColorGreen},
		{"Deep/", -1, tcell.ColorPurple},
		{"DEV/", -1, u.palette.Accent},
	} {
		found := false
		for y, row := range rows {
			if check.row >= 0 && y != check.row {
				continue
			}
			if i := strings.Index(row, check.text); i >= 0 {
				x := len([]rune(row[:i]))
				_, _, style, _ := s.GetContent(x, y)
				fg, _, _ := style.Decompose()
				if fg.Hex() != check.color.Hex() {
					t.Fatalf("%s color = %v, want %v", check.text, fg, check.color)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing rendered label %q", check.text)
		}
	}
}

func TestConnectionFolderGroupRedAndOrange(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Folders: []string{"DEV"}, Language: "es"})
	defer u.cancel()
	u.chooseConnection()
	for _, tc := range []struct {
		index int
		label string
		color tcell.Color
	}{
		{6, "Rojo", tcell.ColorRed},
		{7, "Naranja", tcell.ColorOrange},
	} {
		selectFolderGroup(t, u, "DEV")
		folderGroupKey(u, 'e')
		form, ok := u.dialogContent.(*tview.Form)
		if !ok {
			t.Fatalf("expected folder form, got %T", u.dialogContent)
		}
		color := form.GetFormItem(2).(*tview.DropDown)
		color.SetCurrentOption(tc.index)
		if _, label := color.GetCurrentOption(); label != tc.label {
			t.Fatalf("color option = %q, want %q", label, tc.label)
		}
		saveForm(u, form)
		u.active().Profile.Folder = "DEV"
		if _, _, got := u.connectionContext(); got != tc.color {
			t.Errorf("workspace color = %v, want %v", got, tc.color)
		}
	}
}

func TestConnectionFolderGroupCreateColorAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	u := New(path, config.Config{Folders: []string{"PROD"}})
	defer u.cancel()
	u.chooseConnection()
	selectFolderGroup(t, u, "PROD")
	folderGroupKey(u, 'n')
	f, name, parent, color := folderGroupForm(t, u)
	if _, label := parent.GetCurrentOption(); label != "PROD" {
		t.Fatalf("creation parent = %q", label)
	}
	name.SetText("Empty")
	color.SetCurrentOption(1)
	if _, label := color.GetCurrentOption(); label != "Blue" {
		t.Fatalf("color option = %q", label)
	}
	saveForm(u, f)
	selectFolderGroup(t, u, "PROD/Empty")
	u.active().Profile.Folder = "PROD/Empty/Deep"
	if env, _, fg := u.connectionContext(); env != "PROD" || fg != tcell.ColorBlue {
		t.Fatalf("workspace got %q %v", env, fg)
	}
	folderGroupKey(u, 'e')
	f, _, _, color = folderGroupForm(t, u)
	color.SetCurrentOption(0)
	saveForm(u, f)
	if _, _, fg := u.connectionContext(); fg != tcell.ColorOrangeRed {
		t.Fatalf("clearing explicit color did not restore PROD: %v", fg)
	}
	selectFolderGroup(t, u, "PROD/Empty")
	folderGroupKey(u, 'n')
	f, name, _, color = folderGroupForm(t, u)
	name.SetText("Child")
	color.SetCurrentOption(2)
	saveForm(u, f)
	selectFolderGroup(t, u, "PROD/Empty")
	u.active().Profile.Folder = "PROD/Uncolored"
	if _, _, fg := u.connectionContext(); fg != tcell.ColorOrangeRed {
		t.Fatalf("legacy PROD color = %v", fg)
	}
	folderGroupKey(u, 'd')
	m := u.dialogContent.(*tview.Modal)
	m.SetFocus(1)
	u.pages.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	stored, err := os.ReadFile(path)
	if err != nil || bytes.Contains(stored, []byte("Empty")) || bytes.Contains(stored, []byte("folder_colors")) {
		t.Fatalf("deletion left folder or colors: %s %v", stored, err)
	}
}

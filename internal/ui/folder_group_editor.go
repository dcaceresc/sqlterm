package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

var folderColorOptions = []struct {
	id, label string
	color     tcell.Color
}{
	{"", "Default", tcell.ColorDefault},
	{"blue", "Blue", tcell.ColorBlue},
	{"green", "Green", tcell.ColorGreen},
	{"orange-red", "Orange-red", tcell.ColorOrangeRed},
	{"purple", "Purple", tcell.ColorPurple},
	{"yellow", "Yellow", tcell.ColorYellow},
	{"red", "Red", tcell.ColorRed},
	{"orange", "Orange", tcell.ColorOrange},
}

func folderColor(id string) (tcell.Color, bool) {
	for _, option := range folderColorOptions[1:] {
		if id == option.id {
			return option.color, true
		}
	}
	return tcell.ColorDefault, false
}

func (u *UI) folderGroupForm(old, initialParent string) {
	title, initialName := "New folder", ""
	if old != "" {
		title = "Edit folder"
		initialParent, initialName = "", old
		if i := strings.LastIndex(old, "/"); i >= 0 {
			initialParent, initialName = old[:i], old[i+1:]
		}
	}
	parents, parentLabels := []string{""}, []string{u.tr("(Root)")}
	selectedParent := 0
	for _, path := range config.FolderPaths(u.cfg) {
		if old != "" && folderContains(old, path) {
			continue
		}
		if path == initialParent {
			selectedParent = len(parents)
		}
		parents = append(parents, path)
		parentLabels = append(parentLabels, tview.Escape(path))
	}
	colors := make([]string, len(folderColorOptions))
	selectedColor := 0
	for i, option := range folderColorOptions {
		colors[i] = u.tr(option.label)
		if option.id == u.cfg.FolderColors[old] {
			selectedColor = i
		}
	}
	f := tview.NewForm().AddInputField(u.tr("Name"), initialName, 45, nil, nil)
	f.AddDropDown(u.tr("Parent group"), parentLabels, selectedParent, nil)
	f.AddDropDown(u.tr("Color"), colors, selectedColor, nil)
	f.SetBorder(true).SetTitle(" " + u.tr(title) + " ")
	f.AddButton(u.tr("Save"), func() {
		parent, _ := f.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
		color, _ := f.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
		if parent < 0 || color < 0 {
			return
		}
		name := f.GetFormItem(0).(*tview.InputField).GetText()
		if err := u.saveFolderGroup(old, name, parents[parent], folderColorOptions[color].id); err != nil {
			f.SetTitle(" " + tview.Escape(u.tr(err.Error())) + " ")
			return
		}
		u.chooseConnection()
	}).AddButton(u.tr("Cancel"), u.chooseConnection)
	u.popup(f, 74, 13)
}

func (u *UI) saveFolderGroup(old, name, parent, color string) error {
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf(`folder name cannot contain / or \`)
	}
	leaf, err := config.NormalizeFolder(name)
	if err != nil {
		return err
	}
	if leaf == "" {
		return fmt.Errorf("folder name is required")
	}
	path := leaf
	if parent != "" {
		if old != "" && folderContains(old, parent) {
			return fmt.Errorf("cannot move a folder inside itself")
		}
		if stringIndex(config.FolderPaths(u.cfg), parent) < 0 {
			return fmt.Errorf("parent group no longer exists")
		}
		path = parent + "/" + leaf
	}
	if old == "" {
		return u.addFolderWithColor(path, color)
	}
	if err := u.updateFolder(old, path, &color); err != nil {
		return err
	}
	u.connectionFolder = path
	return nil
}

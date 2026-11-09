package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// sqlFileDialog keeps navigation separate from the document until selection.
type sqlFileDialog struct {
	*tview.Flex
	u          *UI
	dir        string
	path, name *tview.InputField
	list       *tview.List
	status     *tview.TextView
	save       bool
	done       func(string)
}

func (u *UI) pickSQLFile(initial string, save bool, done func(string)) {
	dir, err := os.Getwd()
	if err != nil {
		u.message(err.Error())
		return
	}
	name := ""
	if initial != "" {
		absolute, err := filepath.Abs(initial)
		if err != nil {
			u.message(err.Error())
			return
		}
		dir, name = filepath.Dir(absolute), filepath.Base(absolute)
	}
	if save && name == "" {
		name = "query.sql"
	}
	p := &sqlFileDialog{Flex: tview.NewFlex().SetDirection(tview.FlexRow), u: u, save: save, done: done}
	p.path = tview.NewInputField().SetLabel(u.tr("Folder") + ": ")
	p.name = tview.NewInputField().SetLabel(u.tr("File name") + ": ").SetText(name)
	p.list = tview.NewList().ShowSecondaryText(false)
	p.status = tview.NewTextView().SetDynamicColors(false)
	action := "Open"
	title := "Open SQL file"
	if save {
		action, title = "Save", "Save SQL as"
	}
	buttons := tview.NewForm().AddButton(u.tr(action), p.submit).AddButton(u.tr("Cancel"), u.dismiss)
	buttons.SetButtonsAlign(tview.AlignRight)
	p.SetBorder(true).SetTitle(" " + u.tr(title) + " ")
	p.AddItem(p.path, 1, 0, false).AddItem(p.list, 0, 1, true).AddItem(p.name, 1, 0, false).AddItem(p.status, 2, 0, false).AddItem(buttons, 3, 0, false)
	p.path.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			p.navigate(p.path.GetText())
		}
	})
	p.name.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			p.submit()
		}
	})
	focus := []tview.Primitive{p.path, p.list, p.name, buttons.GetButton(0), buttons.GetButton(1)}
	p.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if key.Key() == tcell.KeyTab || key.Key() == tcell.KeyBacktab {
			for i, item := range focus {
				if item.HasFocus() {
					step := 1
					if key.Key() == tcell.KeyBacktab {
						step = len(focus) - 1
					}
					u.app.SetFocus(focus[(i+step)%len(focus)])
					return nil
				}
			}
		}
		return key
	})
	p.navigate(dir)
	// The dialog wraps a Flex, so style its children explicitly: the shared
	// theme walker only descends into concrete tview container types.
	u.style(p.Flex)
	u.popup(p, 96, 28)
	u.app.SetFocus(p.list)
}

func (p *sqlFileDialog) navigate(dir string) {
	if !filepath.IsAbs(dir) && p.dir != "" {
		dir = filepath.Join(p.dir, dir)
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		p.status.SetText(err.Error())
		return
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		p.status.SetText(err.Error())
		return
	}
	p.dir = absolute
	p.path.SetText(absolute)
	p.list.Clear()
	parent := filepath.Dir(absolute)
	p.list.AddItem("[..]", "", 0, func() { p.navigate(parent) })
	// At a Windows volume root, expose available drives without a native dialog.
	if parent == absolute && filepath.VolumeName(absolute) != "" {
		for drive := 'A'; drive <= 'Z'; drive++ {
			root := string(drive) + ":\\"
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				p.list.AddItem(root, "", 0, func() { p.navigate(root) })
			}
		}
	}
	for _, directories := range []bool{true, false} {
		for _, entry := range entries {
			path := filepath.Join(absolute, entry.Name())
			info, err := os.Stat(path)
			if err != nil || info.IsDir() != directories {
				continue
			}
			name := entry.Name()
			if directories {
				p.list.AddItem(tview.Escape("["+name+"]"), "", 0, func() { p.navigate(path) })
			} else {
				p.list.AddItem(tview.Escape(name), "", 0, func() {
					p.name.SetText(name)
					if p.save {
						p.u.app.SetFocus(p.name)
					} else {
						p.submit()
					}
				})
			}
		}
	}
	p.status.SetText(p.u.tr("Tab: next field · Enter: open folder / select · Esc: cancel"))
}

func (p *sqlFileDialog) submit() {
	name := p.name.GetText()
	if strings.TrimSpace(name) == "" {
		p.status.SetText(p.u.tr("Choose a file name"))
		return
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.dir, path)
	}
	path = filepath.Clean(path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		p.navigate(path)
		return
	}
	p.u.dismiss()
	p.done(path)
}

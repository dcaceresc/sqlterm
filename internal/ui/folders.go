package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sort"
	"sqlterm/internal/config"
	"strings"
)

type connectionRef struct {
	Folder       string
	ProfileIndex int
	FolderNode   bool
}

func connectionRoot(c config.Config, filter string) *tview.TreeNode {
	rootLabel, recentLabel := "Connections", "recent"
	if c.Language == "es" {
		rootLabel, recentLabel = "Conexiones", "reciente"
	}
	root := tview.NewTreeNode(rootLabel).SetReference(connectionRef{FolderNode: true, ProfileIndex: -1}).SetExpanded(true)
	nodes := map[string]*tview.TreeNode{"": root}
	var ensure func(string) *tview.TreeNode
	ensure = func(path string) *tview.TreeNode {
		if n, ok := nodes[path]; ok {
			return n
		}
		parent, name := "", path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			parent, name = path[:i], path[i+1:]
		}
		n := tview.NewTreeNode(tview.Escape(name) + "/").SetReference(connectionRef{Folder: path, ProfileIndex: -1, FolderNode: true}).SetExpanded(true)
		ensure(parent).AddChild(n)
		nodes[path] = n
		return n
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	for _, path := range config.FolderPaths(c) {
		if filter == "" || fuzzy(filter, strings.ToLower(path)) {
			ensure(path)
		}
	}
	indexes := make([]int, len(c.Profiles))
	for i := range indexes {
		indexes[i] = i
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		a, b := c.Profiles[indexes[i]], c.Profiles[indexes[j]]
		fa, fb := stringIndex(c.Favorites, profileID(a)) >= 0, stringIndex(c.Favorites, profileID(b)) >= 0
		if fa != fb {
			return fa
		}
		ra, rb := stringIndex(c.RecentConnections, profileID(a)), stringIndex(c.RecentConnections, profileID(b))
		if ra != rb {
			if ra < 0 {
				return false
			}
			if rb < 0 {
				return true
			}
			return ra < rb
		}
		return strings.ToLower(c.Profiles[indexes[i]].DisplayName()) < strings.ToLower(c.Profiles[indexes[j]].DisplayName())
	})
	for _, i := range indexes {
		p := c.Profiles[i]
		haystack := strings.ToLower(p.DisplayName() + " " + p.Engine + " " + p.Host + " " + p.Database)
		if !fuzzy(filter, haystack) {
			continue
		}
		text := p.Name
		if stringIndex(c.Favorites, profileID(p)) >= 0 {
			text = "★ " + text
		}
		if stringIndex(c.RecentConnections, profileID(p)) >= 0 {
			text += " (" + recentLabel + ")"
		}
		ensure(p.Folder).AddChild(tview.NewTreeNode(tview.Escape(text)).SetReference(connectionRef{Folder: p.Folder, ProfileIndex: i}))
	}
	return root
}
func (u *UI) connectionBrowser() {
	tree := tview.NewTreeView()
	tree.SetBorder(true).SetTitle(u.tr(" Connections · Enter connect / expand "))
	search := tview.NewInputField().SetLabel(u.tr("Find: "))
	details := tview.NewTextView().SetWrap(true)
	details.SetBorder(true).SetTitle(u.tr(" Selected connection "))
	rebuild := func(filter string) {
		target := u.connectionFolder
		root := connectionRoot(u.cfg, filter)
		tree.SetRoot(root).SetCurrentNode(root)
		if filter != "" {
			var first *tview.TreeNode
			root.Walk(func(n, parent *tview.TreeNode) bool {
				if ref, ok := n.GetReference().(connectionRef); ok && !ref.FolderNode && first == nil {
					first = n
				}
				return true
			})
			if first != nil {
				tree.SetCurrentNode(first)
			}
		}
		if filter == "" {
			root.Walk(func(n, parent *tview.TreeNode) bool {
				if ref, ok := n.GetReference().(connectionRef); ok && ref.FolderNode && ref.Folder == target {
					tree.SetCurrentNode(n)
				}
				return true
			})
		}
		u.style(tree)
	}
	tree.SetChangedFunc(func(n *tview.TreeNode) {
		if ref, ok := n.GetReference().(connectionRef); ok {
			u.connectionFolder = ref.Folder
			if !ref.FolderNode && ref.ProfileIndex >= 0 && ref.ProfileIndex < len(u.cfg.Profiles) {
				p := u.cfg.Profiles[ref.ProfileIndex]
				database := p.Database
				if database == "" {
					database = u.tr("Choose after connecting")
				}
				details.SetText(fmt.Sprintf(u.tr("%s · %s\n%s:%d · %s\nUser: %s · * toggle favorite"), p.DisplayName(), p.Engine, p.Host, p.EffectivePort(), database, p.User))
			} else {
				details.SetText(u.tr("Folder: ") + ref.Folder + "\n" + u.tr("←/→ collapse/expand · Enter connects profiles") + "\n" + u.tr("/ search · + add · n folder · e edit"))
			}
		}
	})
	tree.SetSelectedFunc(func(n *tview.TreeNode) {
		ref, ok := n.GetReference().(connectionRef)
		if !ok {
			return
		}
		if ref.FolderNode {
			n.SetExpanded(!n.IsExpanded())
			return
		}
		if ref.ProfileIndex >= 0 && ref.ProfileIndex < len(u.cfg.Profiles) {
			p := u.cfg.Profiles[ref.ProfileIndex]
			u.dismiss()
			u.password(p, func(password string) { u.connect(u.active(), p, password) })
		}
	})
	search.SetChangedFunc(rebuild)
	rebuild("")
	add := func() { u.dismiss(); u.profileForm(-1) }
	edit := func() {
		n := tree.GetCurrentNode()
		if n != nil {
			if ref, ok := n.GetReference().(connectionRef); ok && !ref.FolderNode {
				u.dismiss()
				u.profileForm(ref.ProfileIndex)
				return
			}
		}
		u.dismiss()
		u.pickProfileEdit()
	}
	newFolder := func() {
		initial := ""
		if u.connectionFolder != "" {
			initial = u.connectionFolder + "/"
		}
		u.prompt(u.tr("New folder"), initial, false, func(path string) {
			if err := u.addFolder(path); err != nil {
				u.notice(u.tr("Folder"), err.Error())
				return
			}
			u.chooseConnection()
		})
	}
	selectedFolder := func() (string, bool) {
		n := tree.GetCurrentNode()
		if n == nil {
			return "", false
		}
		ref, ok := n.GetReference().(connectionRef)
		return ref.Folder, ok && ref.FolderNode && ref.Folder != ""
	}
	rename := func() {
		old, ok := selectedFolder()
		if !ok {
			return
		}
		u.prompt(u.tr("Rename folder"), old, false, func(path string) {
			if err := u.renameFolder(old, path); err != nil {
				u.notice(u.tr("Folder"), err.Error())
				return
			}
			u.chooseConnection()
		})
	}
	remove := func() {
		path, ok := selectedFolder()
		if !ok {
			return
		}
		u.confirm(u.tr("Delete empty folder ")+path+"? Connections must be moved first.", func() {
			if err := u.deleteFolder(path); err != nil {
				u.notice(u.tr("Folder"), err.Error())
				return
			}
			u.chooseConnection()
		})
	}
	buttons := tview.NewForm().SetHorizontal(true).AddButton(u.tr("+ Add profile"), add).AddButton(u.tr("New folder"), newFolder).AddButton(u.tr("Edit profile"), edit)
	buttons.SetBorderPadding(0, 0, 0, 0)
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 1, 0, true).AddItem(tree, 0, 1, false).AddItem(details, 5, 0, false).AddItem(buttons, 3, 0, false)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		focus := u.app.GetFocus()
		if focus == search {
			if k.Key() == tcell.KeyEnter || k.Key() == tcell.KeyTab {
				u.app.SetFocus(tree)
				return nil
			}
			return k
		}
		if k.Key() == tcell.KeyTab {
			if tree.HasFocus() {
				u.app.SetFocus(buttons)
				return nil
			}
			if _, i := buttons.GetFocusedItemIndex(); i == buttons.GetButtonCount()-1 {
				u.app.SetFocus(search)
				return nil
			}
			return k
		}
		if k.Key() == tcell.KeyBacktab {
			if tree.HasFocus() {
				u.app.SetFocus(search)
				return nil
			}
			if _, i := buttons.GetFocusedItemIndex(); i == 0 {
				u.app.SetFocus(tree)
				return nil
			}
		}
		if tree.HasFocus() && (k.Key() == tcell.KeyLeft || k.Key() == tcell.KeyRight) {
			if n := tree.GetCurrentNode(); n != nil {
				n.SetExpanded(k.Key() == tcell.KeyRight)
			}
			return nil
		}
		if tree.HasFocus() && k.Key() == tcell.KeyRune {
			switch k.Rune() {
			case '*':
				if n := tree.GetCurrentNode(); n != nil {
					if ref, ok := n.GetReference().(connectionRef); ok && !ref.FolderNode {
						p := u.cfg.Profiles[ref.ProfileIndex]
						id := profileID(p)
						next := u.cfg
						next.Favorites = append([]string{}, u.cfg.Favorites...)
						if i := stringIndex(next.Favorites, id); i >= 0 {
							next.Favorites = append(next.Favorites[:i], next.Favorites[i+1:]...)
						} else {
							next.Favorites = append(next.Favorites, id)
						}
						if err := config.Save(u.configPath, next); err != nil {
							details.SetText(err.Error())
						} else {
							u.cfg = next
							rebuild(search.GetText())
						}
					}
				}
				return nil
			case '+':
				add()
				return nil
			case 'e':
				edit()
				return nil
			case 'n':
				newFolder()
				return nil
			case 'r':
				rename()
				return nil
			case 'd':
				remove()
				return nil
			case '/':
				u.app.SetFocus(search)
				return nil
			}
		}
		return k
	})
	u.popup(panel, 110, 28)
}
func folderContains(parent, path string) bool {
	return parent == path || strings.HasPrefix(path, parent+"/")
}
func (u *UI) saveFolderConfig(next config.Config) error {
	if err := config.Save(u.configPath, next); err != nil {
		return err
	}
	for _, tab := range u.tabs {
		for i, p := range u.cfg.Profiles {
			if tab.Profile.Name == p.Name && tab.Profile.Folder == p.Folder {
				tab.Profile.Folder = next.Profiles[i].Folder
				break
			}
		}
	}
	u.cfg = next
	u.refreshBars()
	return nil
}
func (u *UI) addFolder(raw string) error {
	path, err := config.NormalizeFolder(raw)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("folder name is required")
	}
	for _, s := range config.FolderPaths(u.cfg) {
		if s == path {
			return fmt.Errorf("folder already exists")
		}
	}
	next := u.cfg
	next.Folders = append(append([]string{}, u.cfg.Folders...), path)
	if err = u.saveFolderConfig(next); err != nil {
		return err
	}
	u.connectionFolder = path
	return nil
}
func (u *UI) renameFolder(old, raw string) error {
	path, err := config.NormalizeFolder(raw)
	if err != nil {
		return err
	}
	if old == "" || path == "" {
		return fmt.Errorf("folder name is required")
	}
	if path == old {
		return nil
	}
	if folderContains(old, path) {
		return fmt.Errorf("cannot move a folder inside itself")
	}
	exists := false
	for _, s := range config.FolderPaths(u.cfg) {
		if s == path {
			return fmt.Errorf("target folder already exists")
		}
		if s == old {
			exists = true
		}
	}
	if !exists {
		return fmt.Errorf("folder no longer exists")
	}
	replace := func(s string) string {
		if folderContains(old, s) {
			return path + strings.TrimPrefix(s, old)
		}
		return s
	}
	next := u.cfg
	next.Folders = append([]string{}, u.cfg.Folders...)
	for i, s := range next.Folders {
		next.Folders[i] = replace(s)
	}
	next.Profiles = append([]config.Profile{}, u.cfg.Profiles...)
	for i := range next.Profiles {
		next.Profiles[i].Folder = replace(next.Profiles[i].Folder)
	}
	if err = u.saveFolderConfig(next); err != nil {
		return err
	}
	u.connectionFolder = replace(u.connectionFolder)
	return nil
}
func (u *UI) deleteFolder(path string) error {
	if path == "" {
		return fmt.Errorf("select a folder first")
	}
	for _, p := range u.cfg.Profiles {
		if folderContains(path, p.Folder) {
			return fmt.Errorf("folder contains connections; move them using Edit profile first")
		}
	}
	next := u.cfg
	next.Folders = []string{}
	for _, s := range u.cfg.Folders {
		if !folderContains(path, s) {
			next.Folders = append(next.Folders, s)
		}
	}
	if err := u.saveFolderConfig(next); err != nil {
		return err
	}
	if folderContains(path, u.connectionFolder) {
		u.connectionFolder = ""
	}
	return nil
}

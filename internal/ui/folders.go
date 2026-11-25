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
	Folder        string
	ProfileIndex  int
	FolderNode    bool
	FavoriteGroup bool
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
	favoritesLabel := "★ Favorites"
	if c.Language == "es" {
		favoritesLabel = "★ Favoritos"
	}
	favorites := tview.NewTreeNode(favoritesLabel).SetExpanded(true).SetReference(connectionRef{FolderNode: true, ProfileIndex: -1, FavoriteGroup: true})
	for _, i := range indexes {
		p := c.Profiles[i]
		if stringIndex(c.Favorites, profileID(p)) < 0 || !fuzzy(filter, strings.ToLower(p.DisplayName()+" "+p.Engine+" "+p.Host+" "+p.Database)) {
			continue
		}
		label := "★ " + p.Name
		if p.Folder != "" {
			label += " · " + p.Folder
		}
		favorites.AddChild(tview.NewTreeNode(tview.Escape(label)).SetReference(connectionRef{Folder: p.Folder, ProfileIndex: i}))
	}
	if len(favorites.GetChildren()) > 0 {
		root.AddChild(favorites)
	}
	for _, path := range config.FolderPaths(c) {
		if filter == "" || fuzzy(filter, strings.ToLower(path)) {
			ensure(path)
		}
	}
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
	tree.SetBorder(true).SetTitle(u.tr(" Connections · / search · Enter connect / expand "))
	search := newSelectorSearch(u.tr("Find: "), tree)
	var hasMatches bool
	details := tview.NewTextView().SetWrap(true)
	details.SetBorder(true).SetTitle(u.tr(" Selected connection "))
	preferredIndex := -1
	rebuild := func(filter string) {
		target := u.connectionFolder
		root := connectionRoot(u.cfg, filter)
		tree.SetRoot(root).SetCurrentNode(root)
		var first *tview.TreeNode
		root.Walk(func(n, parent *tview.TreeNode) bool {
			if ref, ok := n.GetReference().(connectionRef); ok && !ref.FolderNode && first == nil {
				first = n
			}
			return true
		})
		hasMatches = first != nil
		if first != nil {
			tree.SetCurrentNode(first)
		} else if filter != "" {
			details.SetText(u.tr("No matching connections"))
		}
		if filter == "" {
			hasFavorites := false
			if len(root.GetChildren()) > 0 {
				first := root.GetChildren()[0]
				if ref, ok := first.GetReference().(connectionRef); ok && ref.FavoriteGroup && len(first.GetChildren()) > 0 {
					hasFavorites = true
					tree.SetCurrentNode(first.GetChildren()[0])
				}
			}
			root.Walk(func(n, parent *tview.TreeNode) bool {
				if ref, ok := n.GetReference().(connectionRef); ok && !hasFavorites && ref.FolderNode && !ref.FavoriteGroup && ref.Folder == target && target != "" {
					tree.SetCurrentNode(n)
				}
				return true
			})
		}
		if preferredIndex >= 0 {
			picked := false
			root.Walk(func(n, parent *tview.TreeNode) bool {
				if ref, ok := n.GetReference().(connectionRef); ok && !ref.FolderNode && ref.ProfileIndex == preferredIndex && !picked {
					tree.SetCurrentNode(n)
					picked = true
				}
				return true
			})
			preferredIndex = -1
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
				details.SetText(u.tr("Folder: ") + ref.Folder + "\n" + u.tr("Enter collapses/expands folders or connects profiles") + "\n" + u.tr("/ search · + add · n folder · e edit"))
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
	selectedFolder := func() (string, bool) {
		n := tree.GetCurrentNode()
		if n == nil {
			return "", false
		}
		ref, ok := n.GetReference().(connectionRef)
		return ref.Folder, ok && ref.FolderNode && !ref.FavoriteGroup && ref.Folder != ""
	}
	edit := func() {
		if path, ok := selectedFolder(); ok {
			u.folderGroupForm(path, "")
			return
		}
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
		parent := ""
		if n := tree.GetCurrentNode(); n != nil {
			if ref, ok := n.GetReference().(connectionRef); ok {
				parent = ref.Folder
			}
		}
		u.folderGroupForm("", parent)
	}
	rename := func() {
		if path, ok := selectedFolder(); ok {
			u.folderGroupForm(path, "")
		}
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
	favorite := func() {
		n := tree.GetCurrentNode()
		if n == nil {
			return
		}
		ref, ok := n.GetReference().(connectionRef)
		if !ok || ref.FolderNode {
			return
		}
		next := u.cfg
		id := profileID(u.cfg.Profiles[ref.ProfileIndex])
		next.Favorites = append([]string{}, u.cfg.Favorites...)
		if i := stringIndex(next.Favorites, id); i >= 0 {
			next.Favorites = append(next.Favorites[:i], next.Favorites[i+1:]...)
		} else {
			next.Favorites = append(next.Favorites, id)
		}
		if err := config.Save(u.configPath, next); err != nil {
			details.SetText(err.Error())
			return
		}
		u.cfg = next
		preferredIndex = ref.ProfileIndex
		rebuild(search.GetText())
	}
	buttons := tview.NewForm().SetHorizontal(true).AddButton(u.tr("+ Add profile"), add).AddButton(u.tr("New folder"), newFolder).AddButton(u.tr("Edit"), edit).AddButton("★ "+u.tr("Favorite"), favorite)
	buttons.SetBorderPadding(0, 0, 0, 0)
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 1, 0, false).AddItem(tree, 0, 1, true).AddItem(details, 5, 0, false).AddItem(buttons, 3, 0, false)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		wasSearching := search.HasFocus()
		if k = search.key(u, k, hasMatches || search.GetText() == ""); k == nil || wasSearching {
			return k
		}
		if k.Key() == tcell.KeyEnter && tree.HasFocus() && search.GetText() != "" && !hasMatches {
			return nil
		}
		if k.Key() == tcell.KeyTab {
			if tree.HasFocus() {
				u.app.SetFocus(buttons)
				return nil
			}
			if _, i := buttons.GetFocusedItemIndex(); i == buttons.GetButtonCount()-1 {
				u.app.SetFocus(tree)
				return nil
			}
			return k
		}
		if k.Key() == tcell.KeyBacktab {
			if _, i := buttons.GetFocusedItemIndex(); !tree.HasFocus() && i == 0 {
				u.app.SetFocus(tree)
				return nil
			}
		}
		if tree.HasFocus() && (k.Key() == tcell.KeyLeft || k.Key() == tcell.KeyRight || (k.Key() == tcell.KeyRune && k.Rune() == ' ')) {
			return nil
		}
		if tree.HasFocus() && k.Key() == tcell.KeyRune {
			switch k.Rune() {
			case '*':
				favorite()
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
	return u.addFolderWithColor(raw, "")
}

func (u *UI) addFolderWithColor(raw, color string) error {
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
	if color != "" {
		next.FolderColors = rewriteFolderColors(next.FolderColors, func(s string) string { return s })
		next.FolderColors[path] = color
	}
	if err = u.saveFolderConfig(next); err != nil {
		return err
	}
	u.connectionFolder = path
	return nil
}
func (u *UI) renameFolder(old, raw string) error {
	return u.updateFolder(old, raw, nil)
}

func (u *UI) updateFolder(old, raw string, color *string) error {
	path, err := config.NormalizeFolder(raw)
	if err != nil {
		return err
	}
	if old == "" || path == "" {
		return fmt.Errorf("folder name is required")
	}
	if path != old && folderContains(old, path) {
		return fmt.Errorf("cannot move a folder inside itself")
	}
	exists := false
	for _, s := range config.FolderPaths(u.cfg) {
		if s == path && path != old {
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
	next.Profiles = append([]config.Profile(nil), u.cfg.Profiles...)
	ids := map[string]string{}
	for i := range next.Profiles {
		p := &next.Profiles[i]
		previousID := profileID(*p)
		p.Folder = replace(p.Folder)
		if id := profileID(*p); id != previousID {
			ids[previousID] = id
		}
	}
	next.Favorites = rewriteFolderProfileIDs(next.Favorites, ids)
	next.RecentConnections = rewriteFolderProfileIDs(next.RecentConnections, ids)
	next.FolderColors = rewriteFolderColors(next.FolderColors, replace)
	if color != nil {
		if *color == "" {
			delete(next.FolderColors, path)
		} else {
			next.FolderColors[path] = *color
		}
	}
	if err = u.saveFolderConfig(next); err != nil {
		return err
	}
	u.connectionFolder = replace(u.connectionFolder)
	return nil
}
func rewriteFolderProfileIDs(values []string, moved map[string]string) []string {
	if len(moved) == 0 {
		return values
	}
	next := append([]string(nil), values...)
	for i, id := range next {
		if updated, ok := moved[id]; ok {
			next[i] = updated
		}
	}
	return next
}

func rewriteFolderColors(colors map[string]string, replace func(string) string) map[string]string {
	next := make(map[string]string, len(colors))
	for path, color := range colors {
		if path = replace(path); path != "" {
			next[path] = color
		}
	}
	return next
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
	next.FolderColors = rewriteFolderColors(next.FolderColors, func(s string) string {
		if folderContains(path, s) {
			return ""
		}
		return s
	})
	if err := u.saveFolderConfig(next); err != nil {
		return err
	}
	if folderContains(path, u.connectionFolder) {
		u.connectionFolder = ""
	}
	return nil
}

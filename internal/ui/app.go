package ui

import (
	"context"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/config"
	"sqlterm/internal/credentials"
	"sqlterm/internal/db"
	"sqlterm/internal/editor"
	"sqlterm/internal/result"
	"strings"
	"sync"
)

type Tab struct {
	Editor              *editor.Editor
	Session             *db.Session
	Profile             config.Profile
	Password            string
	Path, Saved, Status string
	Original            string
	Object              *db.Object
	Catalog             []db.Object
	Columns             map[string][]string
	Databases           []string
	SelectingDatabase   bool
	Results             []*result.Store
	Messages            []string
	SelectedSet         int
	Order               []int
	Busy                bool
	Cancel              context.CancelFunc
}
type UI struct {
	app                   *tview.Application
	pages                 *tview.Pages
	layout                *tview.Flex
	editorHost            *tview.Flex
	tabsBar, status, help *tview.TextView
	objects               *tview.List
	search                *tview.InputField
	kind                  *tview.DropDown
	table                 *tview.Table
	cfg                   config.Config
	vault                 credentials.Store
	openSession           func(context.Context, config.Profile, string) (*db.Session, error)
	writeClipboard        func(string) error
	palette               Palette
	dialogCancel          func()
	connectionFolder      string
	configPath            string
	tabs                  []*Tab
	current               int
	leader                bool
	closing               bool
	suggestions           *suggestionState
	suppressSuggestions   bool
	skipSuggestionMove    bool
	wg                    sync.WaitGroup
	ctx                   context.Context
	cancel                context.CancelFunc
}

func New(path string, cfg config.Config) *UI {
	ctx, cancel := context.WithCancel(context.Background())
	u := &UI{app: tview.NewApplication(), cfg: cfg, configPath: path, ctx: ctx, cancel: cancel, vault: credentials.System{}}
	u.openSession = db.Connect
	u.writeClipboard = clipboard.WriteAll
	if u.cfg.Folders == nil {
		u.cfg.Folders = []string{"DEV", "QA", "PROD"}
	}
	u.palette = resolveTheme(cfg.Theme)
	u.tabsBar = tview.NewTextView().SetDynamicColors(true)
	u.status = tview.NewTextView().SetDynamicColors(true)
	u.help = tview.NewTextView().SetText(defaultHelp)
	u.search = tview.NewInputField().SetLabel("Find: ").SetChangedFunc(func(string) { u.filterObjects() })
	u.kind = tview.NewDropDown().SetLabel("Type: ").SetOptions([]string{"all", "table", "view", "procedure", "function"}, func(string, int) { u.filterObjects() }).SetCurrentOption(0)
	u.objects = tview.NewList().ShowSecondaryText(false)
	u.objects.SetBorder(true).SetTitle(" Objects · Enter definition ")
	left := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.search, 1, 0, false).AddItem(u.kind, 1, 0, false).AddItem(u.objects, 0, 1, false)
	u.editorHost = tview.NewFlex()
	u.table = tview.NewTable().SetFixed(1, 0).SetSelectable(true, true)
	u.table.SetBorder(true).SetTitle(" Results ")
	u.table.SetSelectedFunc(func(row, col int) { u.viewCell(row, col) })
	u.table.SetInputCapture(u.gridKey)
	center := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.editorHost, 0, 3, true).AddItem(u.table, 0, 2, false)
	body := tview.NewFlex().AddItem(left, 34, 0, false).AddItem(center, 0, 1, true)
	u.layout = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.tabsBar, 1, 0, false).AddItem(body, 0, 1, true).AddItem(u.status, 1, 0, false).AddItem(u.help, 1, 0, false)
	u.pages = tview.NewPages().AddPage("main", u.layout, true, true)
	u.app.SetRoot(u.pages, true).EnableMouse(true).EnablePaste(true).SetInputCapture(u.key)
	u.app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if state := u.suggestions; state != nil {
			if x, y := event.Position(); state.List.InRect(x, y) {
				consumed, _ := state.List.MouseHandler()(action, event, func(tview.Primitive) {})
				if consumed {
					return nil, action
				}
			}
		}
		return event, action
	})
	// tview holds its application lock while drawing. These hooks use widget
	// focus flags, never Application.GetFocus/SetFocus.
	u.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
		u.status.SetRect(0, max(0, height-2), width, 1)
		u.refreshBars()
		return false
	}).SetAfterDrawFunc(u.updateCursor)
	u.newTab(nil)
	u.applyTheme(cfg.Theme)
	return u
}
func (u *UI) Run() error {
	u.chooseConnection()
	err := u.app.Run()
	u.cancel()
	u.wg.Wait()
	for _, t := range u.tabs {
		u.dispose(t)
	}
	return err
}
func (u *UI) active() *Tab { return u.tabs[u.current] }
func (u *UI) newTab(from *Tab) {
	t := &Tab{Editor: editor.New(), Columns: map[string][]string{}, Status: "Disconnected"}
	t.Editor.Area.SetBorder(true)
	u.style(t.Editor.Area)
	t.Editor.Area.SetChangedFunc(func() { u.refreshBars() })
	t.Editor.Area.SetMovedFunc(func() {
		u.refreshBars()
		if u.active() == t {
			u.updateSuggestions(false)
		}
	})
	t.Editor.OnMode = func(mode string) {
		u.refreshBars()
		if u.active() == t && mode != "INSERT" {
			u.hideSuggestions()
		}
	}
	t.Editor.Search = func() { u.searchEditor() }
	if from != nil {
		t.Profile = from.Profile
		t.Password = from.Password
	}
	u.tabs = append(u.tabs, t)
	u.current = len(u.tabs) - 1
	u.showTab()
	if from != nil && t.Profile.Name != "" {
		p := t.Profile
		if from.SelectingDatabase {
			p.Database = ""
		}
		u.connect(t, p, t.Password)
	}
}
func (u *UI) showTab() {
	u.hideSuggestions()
	t := u.active()
	u.editorHost.Clear().AddItem(t.Editor.Area, 0, 1, true)
	u.filterObjects()
	u.showResults()
	u.refreshBars()
	u.app.SetFocus(t.Editor.Area)
}
func (u *UI) refreshBars() {
	if len(u.tabs) == 0 {
		return
	}
	t := u.active()
	if !u.leader {
		if u.table.HasFocus() {
			u.help.SetText(" Ctrl+C copy menu · y/Y all without/with headers · r/R row · c cell · e export · Ctrl+Q quit ")
		} else {
			u.help.SetText(defaultHelp)
		}
	}
	var b strings.Builder
	for i, x := range u.tabs {
		name := fmt.Sprintf("Query %d", i+1)
		if x.Path != "" {
			name = filepath.Base(x.Path)
		}
		if x.Editor.Area.GetText() != x.Saved {
			name += "*"
		}
		if i == u.current {
			fmt.Fprintf(&b, "[%s:%s] %d:%s [-:-] ", colorTag(u.palette.Background), colorTag(u.palette.Accent), i+1, tview.Escape(name))
		} else {
			fmt.Fprintf(&b, " %d:%s ", i+1, tview.Escape(name))
		}
	}
	u.tabsBar.SetText(b.String())
	profile := t.Profile.DisplayName()
	if profile == "" {
		profile = "no connection"
	}
	busy := ""
	if t.Busy {
		busy = " · working"
	}
	file := fmt.Sprintf("Query %d", u.current+1)
	if t.Path != "" {
		file = filepath.Base(t.Path)
	}
	if t.Editor.Area.GetText() != t.Saved {
		file += "*"
	}
	_, _, row, column := t.Editor.Area.GetCursor()
	_, _, width, _ := u.status.GetInnerRect()
	if width == 0 {
		width = 80
	}
	database := t.Profile.Database
	if t.SelectingDatabase {
		database = "(select with F4)"
	}
	u.status.SetText(modeStatus(u.palette, t.Editor.Mode, file, profile+" / "+database, t.Status+busy, row+1, column+1, width))
	u.refreshModeBorders()
}
func (u *UI) message(s string) { u.active().Status = s; u.refreshBars() }
func (u *UI) post(f func())    { u.app.QueueUpdateDraw(f) }

// Each tab permits one connection operation. Other tabs can work independently.
func (u *UI) task(t *Tab, label string, work func(context.Context) (func(), error)) {
	if t.Busy || u.closing {
		u.message("Session busy; cancel or wait")
		return
	}
	t.Busy = true
	t.Status = label
	ctx, cancel := context.WithCancel(u.ctx)
	t.Cancel = cancel
	u.refreshBars()
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		defer cancel()
		apply, err := work(ctx)
		u.post(func() {
			t.Busy = false
			t.Cancel = nil
			if err != nil {
				t.Status = err.Error()
			} else {
				t.Status = label + " complete"
			}
			if apply != nil {
				apply()
			}
			u.refreshBars()
		})
	}()
}
func (u *UI) dispose(t *Tab) {
	for _, s := range t.Results {
		s.Close()
	}
	if t.Session != nil {
		t.Session.Close()
	}
	t.Password = ""
}
func (u *UI) quit() {
	if u.closing {
		return
	}
	dirty := false
	for _, t := range u.tabs {
		dirty = dirty || t.Editor.Area.GetText() != t.Saved
	}
	finish := func() {
		u.closing = true
		u.cancel()
		u.message("Closing sessions…")
		go func() { u.wg.Wait(); u.post(func() { u.app.Stop() }) }()
	}
	if dirty {
		u.confirm("Discard unsaved SQL and quit?", finish)
	} else {
		finish()
	}
}
func (u *UI) closeTab() {
	t := u.active()
	if t.Busy {
		u.message("Cancel execution before closing this tab")
		return
	}
	closeIt := func() {
		u.dispose(t)
		u.tabs = append(u.tabs[:u.current], u.tabs[u.current+1:]...)
		if len(u.tabs) == 0 {
			u.newTab(nil)
		} else {
			u.current = min(u.current, len(u.tabs)-1)
			u.showTab()
		}
	}
	if t.Editor.Area.GetText() != t.Saved {
		u.confirm("Discard unsaved SQL in this tab?", closeIt)
	} else {
		closeIt()
	}
}
func (u *UI) key(k *tcell.EventKey) *tcell.EventKey {
	if u.suggestionKey(k) {
		return nil
	}
	if k.Key() == tcell.KeyF3 && !u.closing {
		if name, _ := u.pages.GetFrontPage(); name == "main" && u.customKey(k) {
			return nil
		}
		u.chooseTheme()
		return nil
	}
	name, _ := u.pages.GetFrontPage()
	if name != "main" {
		return k
	}
	if u.closing {
		return nil
	}
	if k.Key() == tcell.KeyF4 {
		u.switchDatabase()
		return nil
	}
	if u.customKey(k) {
		return nil
	}
	if u.leader && k.Key() != tcell.KeyRune {
		u.leader = false
		u.help.SetText(defaultHelp)
		if k.Key() == tcell.KeyEscape {
			return nil
		}
	}
	switch k.Key() {
	case tcell.KeyCtrlC:
		if u.table.HasFocus() {
			u.copyMenu()
		} else {
			u.quit()
		}
		return nil
	case tcell.KeyCtrlQ:
		u.quit()
		return nil
	case tcell.KeyF1:
		u.showHelp()
		return nil
	case tcell.KeyF2:
		u.chooseConnection()
		return nil
	case tcell.KeyF5:
		u.execute()
		return nil
	case tcell.KeyF6:
		u.app.SetFocus(u.search)
		return nil
	case tcell.KeyF7:
		u.app.SetFocus(u.active().Editor.Area)
		return nil
	case tcell.KeyF8:
		u.app.SetFocus(u.table)
		return nil
	case tcell.KeyF9:
		u.invokeSelected()
		return nil
	case tcell.KeyF10:
		if u.active().Cancel != nil {
			u.active().Cancel()
		}
		return nil
	case tcell.KeyCtrlN:
		u.newTab(u.active())
		return nil
	case tcell.KeyCtrlS:
		u.saveFile()
		return nil
	case tcell.KeyCtrlO:
		u.openFile()
		return nil
	case tcell.KeyCtrlSpace:
		u.complete()
		return nil
	case tcell.KeyTab:
		if u.app.GetFocus() == u.search {
			u.app.SetFocus(u.kind)
		} else if u.app.GetFocus() == u.kind {
			u.app.SetFocus(u.objects)
		} else if u.app.GetFocus() == u.objects {
			u.app.SetFocus(u.active().Editor.Area)
		} else if u.app.GetFocus() == u.active().Editor.Area && u.active().Editor.Mode != "INSERT" {
			u.app.SetFocus(u.table)
		} else {
			return k
		}
		return nil
	}
	if k.Modifiers()&tcell.ModAlt != 0 && k.Key() == tcell.KeyRune {
		if k.Rune() >= '1' && k.Rune() <= '9' {
			n := int(k.Rune() - '1')
			if n < len(u.tabs) {
				u.current = n
				u.showTab()
			}
			return nil
		}
	}
	if u.app.GetFocus() == u.active().Editor.Area && u.active().Editor.Mode == "NORMAL" && k.Key() == tcell.KeyRune {
		if u.leader {
			u.leader = false
			u.help.SetText(defaultHelp)
			switch k.Rune() {
			case 'c':
				u.chooseConnection()
			case 'f':
				u.app.SetFocus(u.search)
			case 'x':
				u.execute()
			case 'p':
				u.invokeSelected()
			case 'a':
				u.applyDefinition()
			case 'r':
				u.refreshCatalog()
			case 'n':
				u.newTab(u.active())
			case 'w':
				u.saveFile()
			case 'o':
				u.openFile()
			case 'q':
				u.closeTab()
			case 'h':
				u.showHistory()
			case 'e':
				u.export()
			case 'b':
				u.switchDatabase()
			case 'd':
				u.disconnect()
			case 't':
				u.template()
			case 'T':
				u.chooseTheme()
			case '?':
				u.showHelp()
			}
			return nil
		}
		if k.Rune() == ' ' {
			u.leader = true
			u.help.SetText(" c connect · f find · x run · p invoke · a apply · r refresh · n new · w save · o open · q close · h history · e export · b database · d disconnect · t template · T themes ")
			return nil
		}
	}
	return k
}

const defaultHelp = " F1 help  F2 connect  F3 themes  F4 database  F5 run  F6 objects  F7 editor  F8 results  F9 invoke  F10 cancel "

func (u *UI) filterObjects() {
	if len(u.tabs) == 0 {
		return
	}
	u.objects.Clear()
	t := u.active()
	needle := strings.ToLower(u.search.GetText())
	u.kind.SetDisabled(t.SelectingDatabase)
	if t.SelectingDatabase {
		u.objects.SetTitle(" Databases · Enter select · F4 ")
		for _, name := range t.Databases {
			n := name
			if fuzzy(needle, strings.ToLower(n)) {
				u.objects.AddItem(n, "database", 0, func() { p := t.Profile; p.Database = n; u.connect(t, p, t.Password) })
			}
		}
		return
	}
	u.objects.SetTitle(" Objects · Enter definition ")
	_, kind := u.kind.GetCurrentOption()
	for _, object := range t.Catalog {
		o := object
		if kind != "" && kind != "all" && o.Kind != kind {
			continue
		}
		if !fuzzy(needle, strings.ToLower(o.Label())) {
			continue
		}
		u.objects.AddItem(o.Kind+" "+o.Label(), "", 0, func() { u.openObject(o) })
	}
}
func fuzzy(needle, haystack string) bool {
	r := []rune(needle)
	i := 0
	for _, c := range haystack {
		if i < len(r) && r[i] == c {
			i++
		}
	}
	return i == len(r)
}
func (u *UI) selectedObject() (db.Object, bool) {
	if u.active().SelectingDatabase {
		return db.Object{}, false
	}
	n := u.objects.GetCurrentItem()
	if n < 0 || n >= u.objects.GetItemCount() {
		return db.Object{}, false
	}
	label, _ := u.objects.GetItemText(n)
	for _, o := range u.active().Catalog {
		if label == o.Kind+" "+o.Label() {
			return o, true
		}
	}
	return db.Object{}, false
}
func (u *UI) historyPath() string { return filepath.Join(filepath.Dir(u.configPath), "history.json") }

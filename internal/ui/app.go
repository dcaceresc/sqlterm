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
	Execution           *execution
	SortDescription     string
	CatalogStatus       string
	CatalogSelection    string
	CatalogFilter       string
	CatalogKind         int
	Collapsed           map[string]bool
	ColumnWidths        map[int]int
	CopyAnchor          int
	Pending             func()
}
type UI struct {
	app                                                *tview.Application
	pages                                              *tview.Pages
	layout                                             *tview.Flex
	editorHost                                         *tview.Flex
	body, center, sidebar, resultsHost                 *tview.Flex
	contextBar, resultInfo, resultDetail, objectDetail *tview.TextView
	catalogRows                                        map[int]db.Object
	dialogReturn                                       tview.Primitive
	lastWidth, lastHeight                              int
	lastLayout                                         string
	nextExecution                                      int
	restoringCatalog                                   bool
	tabsBar, status, help                              *tview.TextView
	objects                                            *tview.List
	search                                             *tview.InputField
	kind                                               *tview.DropDown
	table                                              *tview.Table
	cfg                                                config.Config
	vault                                              credentials.Store
	openSession                                        func(context.Context, config.Profile, string) (*db.Session, error)
	writeClipboard                                     func(string) error
	palette                                            Palette
	dialogCancel                                       func()
	dialogContent                                      tview.Primitive
	copyPopup                                          *tview.List
	connectionFolder                                   string
	configPath                                         string
	tabs                                               []*Tab
	current                                            int
	closing                                            bool
	suggestions                                        *suggestionState
	suppressSuggestions                                bool
	skipSuggestionMove                                 bool
	wg                                                 sync.WaitGroup
	ctx                                                context.Context
	cancel                                             context.CancelFunc
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
	u.contextBar = tview.NewTextView().SetDynamicColors(true)
	u.resultInfo = tview.NewTextView().SetDynamicColors(true)
	u.resultDetail = tview.NewTextView()
	u.objectDetail = tview.NewTextView()
	u.status = tview.NewTextView().SetDynamicColors(true)
	u.help = tview.NewTextView().SetText(u.tr(defaultHelp))
	u.search = tview.NewInputField().SetLabel(u.tr("Find: ")).SetChangedFunc(func(string) { u.filterObjects() })
	u.kind = tview.NewDropDown().SetLabel(u.tr("Type: ")).SetOptions(u.kindOptions(), func(string, int) { u.filterObjects() }).SetCurrentOption(0)
	u.objects = tview.NewList().ShowSecondaryText(false)
	u.objects.SetChangedFunc(func(i int, _, _ string, _ rune) {
		if o, ok := u.catalogRows[i]; ok && len(u.tabs) > 0 {
			u.active().CatalogSelection = o.Kind + " " + o.Label()
			u.objectDetail.SetText(o.Label())
		}
	})
	u.objects.SetBorder(true).SetTitle(u.tr(" Objects · Enter definition "))
	left := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.search, 1, 0, false).AddItem(u.kind, 1, 0, false).AddItem(u.objects, 0, 1, false)
	left.AddItem(u.objectDetail, 1, 0, false)
	u.sidebar = left
	u.editorHost = tview.NewFlex()
	u.table = tview.NewTable().SetFixed(1, 0).SetSelectable(true, true)
	u.table.SetBorder(true).SetTitle(u.tr(" Results "))
	u.table.SetSelectedFunc(func(row, col int) { u.viewCell(row, col) })
	u.table.SetInputCapture(u.gridKey)
	u.table.SetSelectionChangedFunc(func(int, int) { u.refreshResultInfo() })
	u.table.SetSeparator('│')
	u.resultsHost = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.resultInfo, 2, 0, false).AddItem(u.table, 0, 1, true).AddItem(u.resultDetail, 1, 0, false)
	center := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.editorHost, 0, 1, true).AddItem(u.resultsHost, 0, 1, false)
	u.center = center
	body := tview.NewFlex().AddItem(left, 34, 0, false).AddItem(center, 0, 1, true)
	u.body = body
	u.layout = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.contextBar, 1, 0, false).AddItem(u.tabsBar, 1, 0, false).AddItem(body, 0, 1, true).AddItem(u.status, 1, 0, false).AddItem(u.help, 1, 0, false)
	u.pages = tview.NewPages().AddPage("main", u.layout, true, true)
	u.app.SetRoot(u.pages, true).EnableMouse(true).EnablePaste(true).SetInputCapture(u.key)
	u.app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if u.dialogContent != nil && (action == tview.MouseLeftDown || action == tview.MouseMiddleDown || action == tview.MouseRightDown) {
			if x, y := event.Position(); !u.dialogContains(x, y) {
				u.dismiss()
			}
		}
		if state := u.suggestions; state != nil {
			if x, y := event.Position(); state.List.InRect(x, y) {
				consumed, _ := state.List.MouseHandler()(action, event, func(tview.Primitive) {})
				if consumed {
					return nil, action
				}
			}
		}
		if u.resultWheelDemand(event, action) {
			return nil, action
		}
		return event, action
	})
	// tview holds its application lock while drawing. These hooks use widget
	// focus flags, never Application.GetFocus/SetFocus.
	u.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
		u.adaptLayout(width, height)
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
	u.configureEditor(t)
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
	u.editorHost.Clear().AddItem(t.Editor.View, 0, 1, true)
	u.restoringCatalog = true
	u.search.SetText(t.CatalogFilter)
	u.kind.SetCurrentOption(t.CatalogKind)
	u.restoringCatalog = false
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
	u.help.SetText(u.tr(defaultHelp))
	var b strings.Builder
	for i, x := range u.tabs {
		name := u.queryName(x)
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
		profile = u.tr("No connection")
	}
	busy := ""
	if t.Busy {
		busy = " · working"
	}
	file := u.queryName(t)
	if t.Editor.Area.GetText() != t.Saved {
		file += "*"
	}
	row, column := t.Editor.Position()
	_, _, width, _ := u.status.GetInnerRect()
	if width == 0 {
		width = 80
	}
	database := t.Profile.Database
	if t.SelectingDatabase {
		database = "(select with F4)"
	}
	u.status.SetText(modeStatus(u.palette, t.Editor.Mode, file, profile+" / "+database, u.tr(t.Status)+busy, row+1, column+1, width))
	u.refreshWorkspace()
	u.refreshModeBorders()
}
func (u *UI) message(s string) { u.active().Status = s; u.refreshBars() }
func (u *UI) post(f func())    { u.app.QueueUpdateDraw(f) }

// Each tab permits one connection operation. Other tabs can work independently.
func (u *UI) task(t *Tab, label string, work func(context.Context) (func(), error)) {
	if t.Busy || u.closing {
		u.message(u.tr("Session busy; cancel or wait"))
		return
	}
	t.Busy = true
	t.Status = u.tr(label)
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
				t.Status = u.tr("Completed") + ": " + u.tr(label)
			}
			if apply != nil {
				apply()
			}
			next := t.Pending
			t.Pending = nil
			u.refreshBars()
			// Work queued while this task held the tab starts only now, after the
			// task and its driver connection have actually finished.
			if next != nil && !u.closing {
				next()
			}
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
		u.message(u.tr("Closing sessions…"))
		go func() { u.wg.Wait(); u.post(func() { u.app.Stop() }) }()
	}
	if dirty {
		u.confirm(u.tr("Discard unsaved SQL and quit?"), finish)
	} else {
		finish()
	}
}
func (u *UI) closeTab() {
	t := u.active()
	// A tab with a running (non-paused) statement is refused before any unsaved
	// SQL prompt, as before. A paused read is released first and closed after.
	if t.Busy && !pausedExecution(t) {
		u.message(u.tr("Cancel execution before closing this tab"))
		return
	}
	var closeIt func()
	closeIt = func() {
		if t.Busy {
			if u.releasePausedExecution(t, closeIt) {
				return
			}
			u.message(u.tr("Cancel execution before closing this tab"))
			return
		}
		// The release defers this removal past further input, so the tab is
		// located by identity instead of the index that is active now.
		index := -1
		for i, tab := range u.tabs {
			if tab == t {
				index = i
				break
			}
		}
		if index < 0 {
			return
		}
		u.dispose(t)
		u.tabs = append(u.tabs[:index], u.tabs[index+1:]...)
		if len(u.tabs) == 0 {
			u.newTab(nil)
			return
		}
		if index < u.current {
			u.current--
		}
		u.current = min(u.current, len(u.tabs)-1)
		u.showTab()
	}
	if t.Editor.Area.GetText() != t.Saved {
		u.confirm(u.tr("Discard unsaved SQL in this tab?"), closeIt)
	} else {
		closeIt()
	}
}
func (u *UI) key(k *tcell.EventKey) *tcell.EventKey {
	if u.dialogContent != nil {
		switch k.Key() {
		case tcell.KeyCtrlC, tcell.KeyEscape:
			wasCopy := u.copyPopup != nil
			u.dismiss()
			if wasCopy {
				u.app.SetFocus(u.table)
			}
			return nil
		case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8, tcell.KeyF11, tcell.KeyCtrlP, tcell.KeyCtrlQ:
			u.dismiss()
		}
	}
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
		if k.Key() == tcell.KeyCtrlC {
			u.dismiss()
			return nil
		}
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
	if k.Key() == tcell.KeyRune && k.Modifiers()&tcell.ModAlt != 0 && (k.Rune() == 'z' || k.Rune() == 'Z') {
		u.toggleEditorOption(true)
		return nil
	}
	switch k.Key() {
	case tcell.KeyCtrlC:
		if u.active().Busy && u.currentStore() != nil {
			u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
			return nil
		}
		u.copyMenu()
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
		u.cancelOperation()
		return nil
	case tcell.KeyF11:
		u.layoutMenu()
		return nil
	case tcell.KeyCtrlP:
		u.commandPalette()
		return nil
	case tcell.KeyCtrlN:
		if u.table.HasFocus() {
			return k
		}
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
		if k.Rune() == ' ' {
			u.commandPalette()
			return nil
		}
	}
	return k
}

const defaultHelp = " F1 help  F2 connect  F3 themes  F4 database  F5 run  F6 objects  F7 editor  F8 results  F9 invoke  F10 cancel "

func (u *UI) filterObjects() {
	u.rebuildCatalog()
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
	o, ok := u.catalogRows[n]
	return o, ok
}
func (u *UI) historyPath() string { return filepath.Join(filepath.Dir(u.configPath), "history.json") }

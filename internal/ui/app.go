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
	"sqlterm/internal/diagnostic"
	"sqlterm/internal/editor"
	"sqlterm/internal/result"
	"sync"
	"sync/atomic"
)

type Tab struct {
	Editor              *editor.Editor
	Session             *db.Session
	Profile             config.Profile
	Password            string
	Path, Saved, Status string
	fileCRLF            bool
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
	gridSelection       gridSelection
}
type UI struct {
	app                                *tview.Application
	pages                              *tview.Pages
	layout                             *tview.Flex
	editorHost                         *tview.Flex
	body, center, sidebar, resultsHost *tview.Flex
	tabsHeader, objectDetail           *tview.TextView
	headerHits                         []tabHeaderHit
	headerPress                        tabHeaderHit
	catalogRows                        map[int]db.Object
	dialogReturn                       tview.Primitive
	lastWidth, lastHeight              int
	lastLayout                         string
	nextExecution                      int
	restoringCatalog                   bool
	status, help                       *tview.TextView
	objects                            *tview.List
	search                             *tview.InputField
	kind                               *tview.DropDown
	table                              *tview.Table
	cfg                                config.Config
	vault                              credentials.Store
	openSession                        func(context.Context, config.Profile, string) (*db.Session, error)
	writeClipboard                     func(string) error
	copyingSQL                         bool
	palette                            Palette
	dialogCancel                       func()
	leaderPanel                        *tview.TextView
	dialogContent                      tview.Primitive
	connectionFolder                   string
	lastExportDir                      string
	lastSavedSQLDir                    string
	configPath                         string
	tabs                               []*Tab
	current                            int
	closing                            bool
	nativeEditors                      bool
	nativeWG                           sync.WaitGroup
	nativeEventPending                 atomic.Bool
	suggestions                        *suggestionState
	suppressSuggestions                bool
	skipSuggestionMove                 bool
	wg                                 sync.WaitGroup
	ctx                                context.Context
	cancel                             context.CancelFunc
}

func New(path string, cfg config.Config) *UI {
	ctx, cancel := context.WithCancel(context.Background())
	u := &UI{app: tview.NewApplication(), cfg: cfg, configPath: path, ctx: ctx, cancel: cancel, vault: credentials.System{}}
	u.openSession = func(ctx context.Context, profile config.Profile, password string) (*db.Session, error) {
		session, err := db.Connect(ctx, profile, password)
		return session, diagnostic.Redact(err, password)
	}
	u.writeClipboard = clipboard.WriteAll
	if u.cfg.Folders == nil {
		u.cfg.Folders = []string{"DEV", "QA", "PROD"}
	}
	u.palette = resolveTheme(cfg.Theme)
	u.tabsHeader = tview.NewTextView().SetDynamicColors(true).SetWrap(false).SetScrollable(false)
	u.tabsHeader.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if x, y := event.Position(); u.tabsHeader.InRect(x, y) {
			return tview.MouseConsumed, nil
		}
		return action, event
	})
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
	u.table = tview.NewTable().SetFixed(1, 1).SetSelectable(true, true)
	u.table.SetBorder(true).SetTitle(u.tr(" Results "))
	u.table.SetInputCapture(u.gridKey)
	u.table.SetSelectionChangedFunc(func(int, int) { u.refreshResultInfo() })
	u.table.SetSeparator('│')
	u.resultsHost = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.table, 0, 1, true)
	center := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.editorHost, 0, 1, true).AddItem(u.resultsHost, 0, 1, false)
	u.center = center
	body := tview.NewFlex().AddItem(left, 34, 0, false).AddItem(center, 0, 1, true)
	u.body = body
	u.layout = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.tabsHeader, 1, 0, false).AddItem(body, 0, 1, true).AddItem(u.status, 1, 0, false).AddItem(u.help, 1, 0, false)
	u.pages = tview.NewPages().AddPage("main", u.layout, true, true)
	u.app.SetRoot(u.pages, true).EnableMouse(true).EnablePaste(true).SetInputCapture(u.key)
	gridMouse := u.gridMouseCapture()
	u.app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if u.tabHeaderMouse(event, action) {
			return nil, action
		}
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
		if u.resultPreviewLocked() && (action == tview.MouseScrollDown || action == tview.MouseScrollUp || action == tview.MouseScrollLeft || action == tview.MouseScrollRight) && u.table.InRect(event.Position()) {
			return nil, action
		}
		return gridMouse(event, action)
	})
	// tview holds its application lock while drawing. These hooks use widget
	// focus flags, never Application.GetFocus/SetFocus.
	u.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
		u.nativeEventPending.Store(false)
		u.adaptLayout(width, height)
		if u.resultPreviewLocked() {
			// tview activa el seguimiento del final al dibujar una tabla corta.
			// Restablecer el desplazamiento evita que salte al crecer el resultado.
			u.table.SetOffset(0, 0)
		}
		u.status.SetRect(0, max(0, height-2), width, 1)
		u.refreshBars()
		u.drawTabHeader(width)
		return false
	}).SetAfterDrawFunc(func(screen tcell.Screen) {
		u.updateCursor(screen)
		u.drawLeader(screen)
	})
	_ = readCache(u.configPath+".export-dir.json", &u.lastExportDir)
	u.restoreWorkspace()
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
	u.nativeWG.Wait()
	return err
}
func (u *UI) active() *Tab { return u.tabs[u.current] }
func (u *UI) newTab(from *Tab) {
	t := &Tab{Editor: editor.New(), Columns: map[string][]string{}, Status: "Disconnected"}
	if len(u.tabs) == 0 {
		t.Status = "Welcome to SQLTerm"
	}
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
	if from != nil {
		t.Profile = from.Profile
		t.Password = from.Password
	}
	u.tabs = append(u.tabs, t)
	u.current = len(u.tabs) - 1
	if u.nativeEditors {
		if err := u.activateNeovim(t); err != nil {
			t.Status = fmt.Sprintf("Neovim: %v", err)
		}
	}
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
	_, _, width, _ := u.status.GetInnerRect()
	if width == 0 {
		width = 80
	}
	message := u.tr(t.Status)
	if t.Status == "Connected" || t.Status == "Disconnected" {
		message = ""
	}
	u.status.SetText(modeStatus(u.palette, t.Editor.Mode, message, t.Execution.elapsedText(), width))
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
	password := t.Password
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		defer cancel()
		apply, err := work(ctx)
		if err != nil {
			_ = diagnostic.LogError(u.configPath, label, err, password)
		}
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
			u.refreshBars()
		})
	}()
}
func (u *UI) dispose(t *Tab) {
	if t.Editor.IsNeovim() {
		u.nativeWG.Add(1)
		go func() { defer u.nativeWG.Done(); _ = t.Editor.Close() }()
	}
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
	finish := func() {
		u.closing = true
		u.cancel()
		u.message(u.tr("Closing sessions…"))
		go func() { u.wg.Wait(); u.post(func() { u.app.Stop() }) }()
	}
	if err := u.cacheWorkspace(); err != nil {
		u.confirm(u.tr("Could not cache SQL tabs")+": "+tview.Escape(err.Error())+"\n"+u.tr("Quit without updating the SQL cache?"), finish)
		return
	}
	finish()
}
func (u *UI) closeTab() { u.closeTabTarget(u.active()) }

func (u *UI) closeTabTarget(t *Tab) {
	// A tab with a running (non-paused) statement is refused before any unsaved
	// SQL prompt, as before. A paused read is released first and closed after.
	if t.Busy {
		u.message(u.tr("Cancel execution before closing this tab"))
		return
	}
	var closeIt func()
	closeIt = func() {
		if t.Busy {

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
func saveAsShortcut(k *tcell.EventKey) bool {
	modifiers := k.Modifiers()
	if k.Key() == tcell.KeyCtrlS {
		return modifiers == tcell.ModShift || modifiers == tcell.ModCtrl|tcell.ModShift
	}
	return k.Key() == tcell.KeyRune && (k.Rune() == 's' || k.Rune() == 'S') && modifiers == tcell.ModCtrl|tcell.ModShift
}

func (u *UI) lazyVimTabKey(k *tcell.EventKey) bool {
	if k.Key() != tcell.KeyRune || k.Modifiers()&^tcell.ModShift != 0 {
		return false
	}
	var delta int
	switch k.Rune() {
	case 'H':
		delta = -1
	case 'L':
		delta = 1
	default:
		return false
	}
	e := u.active().Editor
	// No interceptar la segunda tecla de un comando pendiente de Neovim.
	if u.app.GetFocus() != e.Area || !e.IsNeovim() || e.Mode != "NORMAL" || e.Err() != nil {
		return false
	}
	if count := len(u.tabs); count > 1 {
		u.current = (u.current + delta + count) % count
		u.showTab()
	}
	return true
}

func (u *UI) key(k *tcell.EventKey) *tcell.EventKey {
	if u.leaderPanel != nil {
		if u.app.GetFocus() == u.active().Editor.Area {
			switch k.Key() {
			case tcell.KeyCtrlC:
				u.leaderPanel = nil
				return nil
			case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8:
			default:
				return u.leaderPanel.GetInputCapture()(k)
			}
		}
		u.leaderPanel = nil
	}
	if u.dialogContent != nil {
		switch k.Key() {
		case tcell.KeyCtrlC, tcell.KeyEscape:
			if u.cellInspectorKey(k.Key()) {
				return nil
			}
			if k.Key() == tcell.KeyEscape && u.clearSelectorSearch() {
				return nil
			}
			u.dismiss()
			return nil
		case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8:
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
	if saveAsShortcut(k) {
		u.saveFileAs()
		return nil
	}
	if k.Key() == tcell.KeyF4 {
		u.switchDatabase()
		return nil
	}
	if u.customKey(k) {
		return nil
	}
	if u.lazyVimTabKey(k) {
		return nil
	}
	if e := u.active().Editor; k.Key() == tcell.KeyRune && k.Rune() == ':' && k.Modifiers()&^tcell.ModShift == 0 &&
		u.app.GetFocus() == e.Area && e.IsNeovim() && (e.Mode == "NORMAL" || e.Mode == "VISUAL") && e.Err() == nil {
		u.message(u.tr("Ex commands are disabled. F1: help · Space b d: close tab · Space q q: quit"))
		return nil
	}
	if k.Key() == tcell.KeyRune && k.Modifiers()&tcell.ModAlt != 0 && (k.Rune() == 'z' || k.Rune() == 'Z') {
		u.toggleEditorOption(true)
		return nil
	}
	if e := u.active().Editor; k.Key() == tcell.KeyRune && k.Rune() == ' ' && k.Modifiers() == 0 && u.app.GetFocus() == e.Area && e.IsNeovim() && e.Mode == "NORMAL" {
		u.leaderMenu()
		return nil
	}
	switch k.Key() {
	case tcell.KeyCtrlC:
		if u.app.GetFocus() == u.active().Editor.Area {
			u.copySQLSelection()
			return nil
		}
		if u.active().Busy && u.currentStore() != nil {
			u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
			return nil
		}
		if u.table.HasFocus() {
			u.copyGridSelection()
		}
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

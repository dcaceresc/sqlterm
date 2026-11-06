package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

type execution struct {
	ID                                               int
	SQL, EditorSQL, Source, Profile, Database, Scope string
	Started                                          time.Time
	Duration                                         time.Duration
	Phase, Error                                     string
	Pager                                            *db.Pager
	Pause                                            *resultPause
	// pulsePaused stops the elapsed-time timer while the read is paused, and
	// pausedFor keeps the displayed duration frozen at that pause moment.
	pulsePaused atomic.Bool
	pausedFor   time.Duration
}

func (u *UI) queryName(t *Tab) string {
	if t.Path != "" {
		return filepath.Base(t.Path)
	}
	if t.Object != nil {
		return t.Object.Label() + " (" + u.tr("definition") + ")"
	}
	for i, tab := range u.tabs {
		if tab == t {
			return fmt.Sprintf(u.tr("Query %d"), i+1)
		}
	}
	return u.tr("Query")
}

func (u *UI) refreshWorkspace() {
	t := u.active()
	environment := strings.Split(t.Profile.Folder, "/")[0]
	if environment == "" {
		environment = "LOCAL"
	}
	state, database := "Disconnected", t.Profile.Database
	if t.Session != nil {
		state = "Connected"
	}
	if t.SelectingDatabase {
		database = u.tr("Choose database · F4")
	}
	color := u.palette.Accent
	if strings.EqualFold(environment, "PROD") {
		color = tcell.ColorOrangeRed
	}
	profile := t.Profile.Name
	if profile == "" {
		profile = u.tr("No connection")
	}
	u.contextBar.SetText(fmt.Sprintf("[::b] SQLTerm │ [%s]%s[-] │ %s │ %s │ %s[-:-:-]", colorTag(color), tview.Escape(environment), tview.Escape(profile), tview.Escape(database), u.tr(state)))
	u.refreshResultInfo()
	if o, ok := u.selectedObject(); ok {
		u.objectDetail.SetText(o.Label())
	}
	if u.table.HasFocus() {
		u.help.SetText(u.tr(" F1 Help  ↓/PgDn/End/j Scroll more  F10 Stop/keep  Ctrl+C Copy  e Export "))
	} else if u.objects.HasFocus() || u.search.HasFocus() || u.kind.HasFocus() {
		u.help.SetText(u.tr(" F1 Help  Enter Definition  Ctrl+P Actions  F7 Editor  F8 Results  F11 Layout "))
	} else if u.lastWidth > 0 && u.lastWidth < 110 {
		u.help.SetText(u.tr(" F1 Help  F5 Run  F6 Objects  F7 Editor  F8 Results  F10 Cancel  F11 Layout "))
	}
}

func (u *UI) refreshResultInfo() {
	if u.resultInfo == nil || len(u.tabs) == 0 {
		return
	}
	t := u.active()
	e := t.Execution
	scope := "Execute script"
	if selected, _, _ := t.Editor.Area.GetSelection(); selected != "" {
		scope = "Execute selection"
	}
	info := u.tr("Not executed") + " · F5 " + u.tr(scope) + " · " + u.tr("Destination above")
	if e != nil {
		duration := e.Duration
		switch e.Phase {
		case "Running", "Receiving rows", "Cancelling":
			duration = time.Since(e.Started)
		case "Paused":
			// A paused read is idle: keep the duration it reached when it paused.
			duration = e.pausedFor
		}
		stale := ""
		if e.EditorSQL != t.Editor.Area.GetText() {
			stale = " · " + u.tr("Results of previous SQL")
		}
		partial := ""
		if e.Phase == "Cancelled" || e.Phase == "Error" {
			for _, store := range t.Results {
				if store.Count() > 0 {
					partial = " · " + u.tr("Partial results")
					break
				}
			}
		}
		guidance := ""
		if e.Phase == "Paused" && e.Pause != nil {
			guidance = fmt.Sprintf(u.tr(" · %d received · scroll at end for more · F10 stop and keep rows"), e.Pause.Rows)
		}
		info = fmt.Sprintf("#%d %s │ %s / %s │ %s · %s%s%s\n%s%s · %s · F5 · Ctrl+P", e.ID, e.Source, e.Profile, e.Database, e.Started.Format("15:04:05"), duration.Round(time.Millisecond), stale, partial, u.tr(e.Phase), guidance, u.tr(e.Scope))
	}
	u.resultInfo.SetText(tview.Escape(info))
	s := u.currentStore()
	prefix := ""
	if u.table.HasFocus() {
		prefix = "> "
	}
	if s == nil {
		u.table.SetTitle(" " + prefix + u.tr("Results") + " ")
		u.resultDetail.SetText(u.tr("No received rows"))
		return
	}
	u.table.SetTitle(fmt.Sprintf(u.tr(" %sResults · set %d/%d · %d received rows "), prefix, t.SelectedSet+1, len(t.Results), s.Count()))
	rowOffset, _ := u.table.GetOffset()
	_, _, _, height := u.table.GetInnerRect()
	visible := max(0, min(s.Count()-rowOffset, height-1))
	u.resultInfo.SetText(tview.Escape(info + fmt.Sprintf(u.tr(" · %d visible rows"), visible)))
	row, col := u.table.GetSelection()
	_, offset := u.table.GetOffset()
	name := ""
	if col >= 0 && col < len(s.Columns) {
		name = s.Columns[col]
	}
	order := t.SortDescription
	if order == "" {
		order = u.tr("server order")
	}
	u.resultDetail.SetText(fmt.Sprintf(u.tr("Row %d/%d │ Column %d/%d: %s │ Scroll %d/%d │ %s"), row, s.Count(), col+1, len(s.Columns), name, offset+1, len(s.Columns), order))
	if t.CopyAnchor > 0 {
		u.resultDetail.SetText(fmt.Sprintf(u.tr("Selected %d rows (%d–%d) · Ctrl+C copy selection · v clear"), max(t.CopyAnchor, row)-min(t.CopyAnchor, row)+1, min(t.CopyAnchor, row), max(t.CopyAnchor, row)))
	}
}

func (u *UI) cancelOperation() {
	t := u.active()
	if t.Cancel != nil {
		// F10 stops everything: it also abandons a replacement that was queued
		// while the paused read is being released.
		t.Pending = nil
		if t.Execution != nil && (t.Execution.Phase == "Running" || t.Execution.Phase == "Receiving rows" || t.Execution.Phase == "Paused") {
			t.Execution.Phase = "Cancelling"
			t.Execution.Pause = nil
			t.Execution.pulsePaused.Store(false)
		}
		t.Status = u.tr("Cancelling · waiting for driver")
		t.Cancel()
		u.refreshBars()
	}
}

func (u *UI) executionPulse(t *Tab, e *execution) func() {
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// A paused read must look idle: no periodic redraw or elapsed tick.
				if e.pulsePaused.Load() {
					continue
				}
				u.post(func() {
					if u.active() == t && t.Execution == e {
						u.refreshResultInfo()
					}
				})
			}
		}
	}()
	return func() { close(stop); <-done }
}

// Called during drawing: only widget focus flags are safe under tview's lock.
func (u *UI) adaptLayout(width, height int) {
	if len(u.tabs) == 0 {
		return
	}
	focus := "editor"
	if u.table.HasFocus() {
		focus = "results"
	} else if u.objects.HasFocus() || u.search.HasFocus() || u.kind.HasFocus() {
		focus = "objects"
	}
	if u.dialogContent != nil && u.lastLayout != "" && width == u.lastWidth && height == u.lastHeight {
		return
	}
	l := u.cfg.Layout
	key := fmt.Sprintf("%dx%d/%+v/%s", width, height, l, focus)
	if key == u.lastLayout {
		return
	}
	u.lastLayout, u.lastWidth, u.lastHeight = key, width, height
	// Preserve the current widget focus when rebuilding containers.
	u.body.Clear()
	u.center.Clear()
	if width < 95 || height < 27 {
		if focus == "objects" {
			u.body.AddItem(u.sidebar, 0, 1, false)
		} else if focus == "results" {
			u.body.AddItem(u.resultsHost, 0, 1, false)
		} else {
			u.body.AddItem(u.editorHost, 0, 1, false)
		}
		return
	}
	catalogWidth := l.CatalogWidth
	if catalogWidth == 0 {
		catalogWidth = 30
	}
	catalogWidth = min(max(20, catalogWidth), width/2)
	if focus == "objects" || (!l.HideCatalog && l.Maximized == "") {
		u.body.AddItem(u.sidebar, catalogWidth, 0, false)
	}
	if l.Maximized != "" {
		if focus == "results" {
			u.center.AddItem(u.resultsHost, 0, 1, false)
		} else {
			u.center.AddItem(u.editorHost, 0, 1, false)
		}
	} else {
		percent := l.EditorPercent
		if percent == 0 {
			percent = 45
		}
		percent = max(20, min(80, percent))
		u.center.AddItem(u.editorHost, 0, percent, false).AddItem(u.resultsHost, 0, 100-percent, false)
	}
	u.body.AddItem(u.center, 0, 1, false)
}

func (u *UI) saveLayout(l config.Layout) {
	next := u.cfg
	next.Layout = l
	if err := config.Save(u.configPath, next); err != nil {
		u.message(err.Error())
		return
	}
	u.cfg = next
	u.lastLayout = ""
	if l.Maximized == "editor" {
		u.app.SetFocus(u.active().Editor.Area)
	} else if l.Maximized == "results" {
		u.app.SetFocus(u.table)
	}
}

func (u *UI) layoutMenu() {
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true).SetTitle(u.tr(" Layout · saved per workspace "))
	add := func(label string, change func(*config.Layout)) {
		l.AddItem(u.tr(label), "", 0, func() { next := u.cfg.Layout; change(&next); u.dismiss(); u.saveLayout(next) })
	}
	add("Balanced", func(v *config.Layout) { v.EditorPercent = 45; v.Maximized = ""; v.HideCatalog = false })
	add("Edit", func(v *config.Layout) { v.EditorPercent = 70; v.Maximized = "" })
	add("Explore data", func(v *config.Layout) { v.EditorPercent = 25; v.Maximized = "" })
	add("Maximize editor", func(v *config.Layout) { v.Maximized = "editor"; v.HideCatalog = true })
	add("Maximize results", func(v *config.Layout) { v.Maximized = "results"; v.HideCatalog = true })
	add("Show / hide catalog", func(v *config.Layout) { v.HideCatalog = !v.HideCatalog })
	add("Wider catalog (+4)", func(v *config.Layout) {
		if v.CatalogWidth == 0 {
			v.CatalogWidth = 30
		}
		v.CatalogWidth += 4
	})
	add("Narrower catalog (-4)", func(v *config.Layout) {
		if v.CatalogWidth == 0 {
			v.CatalogWidth = 30
		}
		v.CatalogWidth = max(20, v.CatalogWidth-4)
	})
	add("More editor (+10%)", func(v *config.Layout) {
		if v.EditorPercent == 0 {
			v.EditorPercent = 45
		}
		v.EditorPercent = min(80, v.EditorPercent+10)
	})
	add("More results (+10%)", func(v *config.Layout) {
		if v.EditorPercent == 0 {
			v.EditorPercent = 45
		}
		v.EditorPercent = max(20, v.EditorPercent-10)
	})
	u.popup(l, 65, 14)
}

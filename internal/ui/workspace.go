package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
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
	// pulsePaused detiene los redibujados periódicos durante la pausa.
	// pausedFor conserva el tiempo transcurrido hasta esa pausa.
	pulsePaused atomic.Bool
	pausedFor   time.Duration
}

func (e *execution) elapsedText() string {
	if e == nil {
		return ""
	}
	elapsed := e.Duration
	switch e.Phase {
	case "Running", "Receiving rows", "Cancelling":
		elapsed = time.Since(e.Started)
	case "Paused":
		elapsed = e.pausedFor
	}
	return elapsed.Round(time.Millisecond).String()
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

func (u *UI) connectionContext() (environment, details string, color tcell.Color) {
	t := u.active()
	environment, _, _ = strings.Cut(t.Profile.Folder, "/")
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
	color = u.palette.Accent
	if strings.EqualFold(environment, "PROD") {
		color = tcell.ColorOrangeRed
	}
	profile := t.Profile.Name
	if profile == "" {
		profile = u.tr("No connection")
	}
	return environment, fmt.Sprintf(" │ %s │ %s │ %s", profile, database, u.tr(state)), color
}

func (u *UI) refreshWorkspace() {
	u.refreshResultInfo()
	if o, ok := u.selectedObject(); ok {
		u.objectDetail.SetText(o.Label())
	}
	if u.table.HasFocus() {
		u.help.SetText(u.tr(" F1 Help  ↓/PgDn/End/j Scroll more  F10 Stop/keep  Ctrl+C Copy  e Export "))
	} else if u.objects.HasFocus() || u.search.HasFocus() || u.kind.HasFocus() {
		u.help.SetText(u.tr(" F1 Help  Enter Definition  F7 Editor  F8 Results  Space w Layout in NORMAL editor "))
	} else if u.lastWidth > 0 && u.lastWidth < 110 {
		u.help.SetText(u.tr(" F1 Help  F2 Connect  F5 Run  F6 Objects  F7 Editor  F8 Results  F10 Cancel  Space w Layout "))
	}
}

func (u *UI) refreshResultInfo() {
	if len(u.tabs) == 0 {
		return
	}
	t := u.active()
	title := u.tr("Results")
	if u.table.HasFocus() {
		title = "> " + title
	}
	if e := t.Execution; e != nil {
		switch e.Phase {
		case "Paused":
			title += " · " + u.tr("Paused")
		case "Cancelled", "Error":
			for _, store := range t.Results {
				if store.Count() > 0 {
					title += " · " + u.tr("Partial results")
					break
				}
			}
		}
		if e.EditorSQL != t.Editor.Area.GetText() {
			title += " · " + u.tr("Results of previous SQL")
		}
	}
	s := u.currentStore()
	if s != nil {
		title += " · " + fmt.Sprintf(u.tr("set %d/%d · %d received rows"), t.SelectedSet+1, len(t.Results), s.Count())
		if t.CopyAnchor > 0 {
			row, _ := u.table.GetSelection()
			first, last := min(t.CopyAnchor, row), max(t.CopyAnchor, row)
			title += " · " + fmt.Sprintf(u.tr("Selected %d rows (%d–%d) · Ctrl+C copy selection · v clear"), last-first+1, first, last)
		} else if t.SortDescription != "" {
			title += " · " + t.SortDescription
		}
	}
	u.table.SetTitle(" " + title + " ")
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

func (u *UI) layoutCommands() []command {
	var commands []command
	keys := []rune{'b', 'e', 'd', 'm', 'r', 'c', '>', '<', '+', '-'}
	add := func(label string, change func(*config.Layout)) {
		commands = append(commands, command{Name: label, Key: keys[len(commands)], Run: func() { next := u.cfg.Layout; change(&next); u.saveLayout(next) }})
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
	return commands
}

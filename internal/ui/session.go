package ui

import (
	"context"
	"fmt"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"sqlterm/internal/history"
	"strings"
	"time"
)

func (u *UI) connect(t *Tab, p config.Profile, password string) {
	if t.Busy {
		u.message(u.tr("Session busy"))
		return
	}
	u.task(t, "Connect", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		s, e := u.openSession(ctx, p, password)
		if e != nil {
			return func() { u.connectionFailure(p, e) }, e
		}
		selectDatabase := strings.TrimSpace(p.Database) == ""
		var databases []string
		if selectDatabase {
			var err error
			databases, err = s.Adapter.Databases(ctx, s.Conn)
			if err != nil {
				s.Close()
				return nil, err
			}
		}
		return func() {
			if t.Session != nil {
				t.Session.Close()
			}
			t.Session = s
			for _, r := range t.Results {
				r.Close()
			}
			t.Results = nil
			t.Order = nil
			t.Execution = nil
			t.SortDescription = ""
			t.Messages = nil
			t.SelectedSet = 0
			t.CopyAnchor = 0
			t.ColumnWidths = nil
			t.Profile = s.Profile
			u.recordRecent(p)
			t.Password = password
			t.Object = nil
			t.Catalog = nil
			t.Columns = map[string][]string{}
			t.SelectingDatabase = selectDatabase
			t.Databases = databases
			t.Original = ""
			if u.active() == t {
				u.hideSuggestions()
				u.search.SetText(u.tr(""))
				u.kind.SetCurrentOption(0)
				u.filterObjects()
				u.showResults()
			}
			if selectDatabase {
				t.Status = "Choose a database in the explorer or press F4"
			} else {
				u.refreshCatalogTab(t)
			}
		}, nil
	})
}
func (u *UI) disconnect() {
	u.hideSuggestions()
	t := u.active()
	if t.Busy {
		u.message(u.tr("Cancel execution before disconnecting"))
		return
	}
	if t.Session != nil {
		t.Session.Close()
		t.Session = nil
	}
	t.Password = ""
	t.Catalog = nil
	t.Columns = map[string][]string{}
	t.Databases = nil
	t.SelectingDatabase = false
	t.Status = "Disconnected"
	u.filterObjects()
	u.refreshBars()
}
func (u *UI) refreshCatalog() { u.refreshCatalogTab(u.active()) }
func (u *UI) refreshCatalogTab(t *Tab) {
	if t.Busy {
		u.message(u.tr("Session busy"))
		return
	}
	if t.Session == nil {
		u.message(u.tr("Connect first"))
		return
	}
	t.CatalogStatus = "Loading catalog…"
	if u.active() == t {
		u.filterObjects()
	}
	u.task(t, "Refresh catalog", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if t.SelectingDatabase {
			names, err := t.Session.Adapter.Databases(ctx, t.Session.Conn)
			if err != nil {
				return nil, err
			}
			return func() {
				t.Databases = names
				if u.active() == t {
					u.filterObjects()
				}
			}, nil
		}
		objects, e := t.Session.Adapter.Catalog(ctx, t.Session.Conn)
		if e != nil {
			return func() {
				t.CatalogStatus = "Could not load catalog · Ctrl+P Refresh catalog"
				if u.active() == t {
					u.filterObjects()
				}
			}, e
		}
		return func() {
			t.CatalogStatus = ""
			t.Catalog = objects
			t.Columns = map[string][]string{}
			if u.active() == t {
				u.filterObjects()
			}
		}, nil
	})
}
func (u *UI) openObject(o db.Object) {
	t := u.active()
	if t.Session == nil {
		return
	}
	open := func() {
		u.task(t, "Read definition", func(ctx context.Context) (func(), error) {
			ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			text, e := t.Session.Adapter.Definition(ctx, t.Session.Conn, o)
			if e != nil {
				return nil, e
			}
			var cols []string
			if o.Kind == "table" || o.Kind == "view" {
				cols, _ = t.Session.Adapter.Columns(ctx, t.Session.Conn, o)
			}
			return func() {
				t.Editor.SetText(text)
				t.Saved = text
				t.Original = text
				t.Path = ""
				if o.Kind != "table" {
					t.Object = &o
				} else {
					t.Object = nil
				}
				if cols != nil {
					t.Columns[o.Label()] = cols
				}
				if u.active() == t {
					u.app.SetFocus(t.Editor.Area)
				}
			}, nil
		})
	}
	if t.Editor.Area.GetText() != t.Saved {
		u.confirm(u.tr("Replace unsaved editor contents with ")+o.Label()+"?", open)
	} else {
		open()
	}
}
func (u *UI) switchDatabase() {
	t := u.active()
	if t.Session == nil {
		u.message(u.tr("Connect first"))
		return
	}
	u.task(t, "List databases", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		names, e := t.Session.Adapter.Databases(ctx, t.Session.Conn)
		if e != nil {
			return nil, e
		}
		return func() { t.Databases = names; u.databaseSelector(t, names) }, nil
	})
}
func (u *UI) execute() {
	t := u.active()
	if t.Busy {
		u.message(u.tr("Session busy"))
		return
	}
	if t.Session == nil {
		u.message(u.tr("Connect first"))
		return
	}
	if t.SelectingDatabase {
		u.message(u.tr("Choose a database first (F4)"))
		return
	}
	text, _, _ := t.Editor.Area.GetSelection()
	if text == "" {
		text = t.Editor.Area.GetText()
	}
	statements, e := db.SplitScript(t.Profile.Engine, text)
	if e != nil {
		u.message(e.Error())
		return
	}
	if len(statements) == 0 {
		u.message(u.tr("No SQL to execute"))
		return
	}
	bound := make([]db.Bound, len(statements))
	for i, s := range statements {
		bound[i] = db.Bound{SQL: s}
	}
	session := t.Session
	run := func() {
		if t.Session != session {
			u.message(u.tr("Destination changed; review SQL and execute again"))
			return
		}
		u.run(t, text, func(ctx context.Context, emit func(db.Event)) error { return session.Run(ctx, bound, emit) })
	}
	if confirmExecution(u.cfg.ConfirmExecution, t.Profile.Folder, statements) {
		u.confirm(u.tr("Execute SQL on ")+t.Profile.DisplayName()+" / "+t.Profile.Database+"? This policy uses SQL heuristics; it is not a read-only or security guarantee.", run)
	} else {
		run()
	}
}
func (u *UI) run(t *Tab, sqlText string, work func(context.Context, func(db.Event)) error, after ...func()) {
	if t.Busy {
		u.message(u.tr("Session busy"))
		return
	}
	for _, s := range t.Results {
		s.Close()
	}
	t.Results = nil
	t.Messages = nil
	t.Order = nil
	t.SelectedSet = 0
	t.CopyAnchor = 0
	t.ColumnWidths = nil
	t.SortDescription = ""
	if u.active() == t {
		u.showResults()
	}
	if u.cfg.History {
		if e := history.Add(u.historyPath(), history.Entry{At: time.Now(), Profile: t.Profile.DisplayName(), Database: t.Profile.Database, SQL: sqlText}); e != nil {
			u.notice(u.tr("History error"), e.Error())
			return
		}
	}
	u.nextExecution++
	scope := "Execute script"
	if selected, _, _ := t.Editor.Area.GetSelection(); selected != "" {
		scope = "Execute selection"
	}
	pager, err := db.NewPager(u.cfg.EffectiveResultBlockSize())
	if err != nil {
		u.message(err.Error())
		return
	}
	e := &execution{ID: u.nextExecution, SQL: sqlText, EditorSQL: t.Editor.Area.GetText(), Source: u.queryName(t), Profile: t.Profile.DisplayName(), Database: t.Profile.Database, Scope: scope, Started: time.Now(), Phase: "Running", Pager: pager}
	t.Execution = e
	u.task(t, "Execute", func(ctx context.Context) (func(), error) {
		ctx = db.WithPager(ctx, pager)
		started := time.Now()
		stopPulse := u.executionPulse(t, e)
		defer stopPulse()
		err := work(ctx, func(ev db.Event) {
			u.post(func() {
				if t.Execution != e {
					return
				}
				switch ev.Kind {
				case "columns":
					t.Results = append(t.Results, ev.Store)
					if u.active() == t {
						u.showResults()
					}
				case "rows":
					if e.Phase != "Cancelling" && e.Phase != "Paused" {
						e.Phase = "Receiving rows"
					}
					if u.active() == t {
						u.refreshResultInfo()
					}
				case "paused":
					u.setResultPause(t, e, ev)
				case "message":
					if len(t.Messages) < 2000 {
						t.Messages = append(t.Messages, ev.Message)
					}
					t.Status = ev.Message
					u.refreshBars()
				}
			})
		})
		elapsed := time.Since(started)
		cancelled := ctx.Err() == context.Canceled
		return func() {
			if t.Execution != e {
				return
			}
			e.Duration = elapsed
			e.Pause = nil
			e.Phase = "Completed"
			if err != nil {
				e.Phase = "Error"
				e.Error = err.Error()
				if cancelled {
					e.Phase = "Cancelled"
				}
				t.Messages = append(t.Messages, err.Error())
			}
			if err == nil {
				t.Status = fmt.Sprintf(u.tr("Complete in %s · %d result sets"), elapsed.Round(time.Millisecond), len(t.Results))
				for _, f := range after {
					f()
				}
			}
			if u.active() == t {
				u.showResults()
			}
		}, err
	})
}
func (u *UI) invokeSelected() {
	t := u.active()
	if t.SelectingDatabase {
		u.message(u.tr("Choose a database first (F4)"))
		return
	}
	if t.Session == nil {
		u.message(u.tr("Connect first"))
		return
	}
	o, ok := u.selectedObject()
	if u.app.GetFocus() == t.Editor.Area && t.Object != nil {
		o = *t.Object
		ok = true
	}
	if !ok || (o.Kind != "procedure" && o.Kind != "function") {
		u.message(u.tr("Select a procedure or function in Objects first"))
		return
	}
	u.task(t, "Read parameters", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		params, e := t.Session.Adapter.Parameters(ctx, t.Session.Conn, o)
		if e != nil {
			return nil, e
		}
		return func() { u.parameterForm(t, o, params) }, nil
	})
}
func (u *UI) parameterForm(t *Tab, o db.Object, params []db.Parameter) {
	// Scrollable rows keep routines with many parameters usable on small screens.
	values := make([]*tview.InputField, len(params))
	modes := make([]*tview.DropDown, len(params))
	list := tview.NewFlex().SetDirection(tview.FlexRow)
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(u.tr(" ") + o.Label() + " · Tab navigate ")
	for i, p := range params {
		values[i] = tview.NewInputField().SetLabel(p.Name + " (" + p.Type + " " + p.Direction + ") ").SetFieldWidth(30)
		options := []string{"value", "NULL"}
		if p.Default {
			options = append(options, "default")
		}
		initial := 0
		if p.Direction == "OUT" {
			initial = 1
		}
		modes[i] = tview.NewDropDown().SetLabel(u.tr("Mode")).SetOptions(options, nil).SetCurrentOption(initial)
		form.AddFormItem(values[i])
		form.AddFormItem(modes[i])
	}
	form.AddButton(u.tr("Execute"), func() {
		vals := make([]db.Value, len(params))
		for i := range params {
			_, mode := modes[i].GetCurrentOption()
			vals[i] = db.Value{Text: values[i].GetText(), Null: mode == "NULL", Omit: mode == "default"}
		}
		inv, e := t.Session.Adapter.Invoke(o, params, vals)
		if e != nil {
			form.SetTitle(u.tr(" ") + e.Error() + " ")
			return
		}
		u.dismiss()
		u.run(t, "-- Invoke "+o.Label()+" (parameter values excluded from history)", func(ctx context.Context, emit func(db.Event)) error { return t.Session.RunInvocation(ctx, inv, emit) })
	}).AddButton(u.tr("Cancel"), u.dismiss)
	list.AddItem(form, 0, 1, true)
	u.popup(list, 110, min(32, 8+len(params)*4))
}
func (u *UI) complete() { u.updateSuggestions(true) }

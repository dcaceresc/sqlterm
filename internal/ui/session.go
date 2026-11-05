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
		u.message("Session busy")
		return
	}
	u.task(t, "Connect", func(ctx context.Context) (func(), error) {
		s, e := db.Connect(ctx, p, password)
		if e != nil {
			return func() { u.connectionFailure(p, e) }, e
		}
		return func() {
			if t.Session != nil {
				t.Session.Close()
			}
			t.Session = s
			t.Profile = s.Profile
			t.Password = password
			t.Object = nil
			t.Catalog = nil
			t.Columns = map[string][]string{}
			if u.active() == t {
				u.filterObjects()
			}
			u.refreshCatalogTab(t)
		}, nil
	})
}
func (u *UI) disconnect() {
	t := u.active()
	if t.Busy {
		u.message("Cancel execution before disconnecting")
		return
	}
	if t.Session != nil {
		t.Session.Close()
		t.Session = nil
	}
	t.Password = ""
	t.Catalog = nil
	t.Columns = map[string][]string{}
	t.Status = "Disconnected"
	u.filterObjects()
	u.refreshBars()
}
func (u *UI) refreshCatalog() { u.refreshCatalogTab(u.active()) }
func (u *UI) refreshCatalogTab(t *Tab) {
	if t.Session == nil {
		u.message("Connect first")
		return
	}
	u.task(t, "Refresh catalog", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		objects, e := t.Session.Adapter.Catalog(ctx, t.Session.Conn)
		if e != nil {
			return nil, e
		}
		return func() {
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
		u.confirm("Replace unsaved editor contents with "+o.Label()+"?", open)
	} else {
		open()
	}
}
func (u *UI) switchDatabase() {
	t := u.active()
	if t.Session == nil {
		u.message("Connect first")
		return
	}
	u.task(t, "List databases", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		names, e := t.Session.Adapter.Databases(ctx, t.Session.Conn)
		if e != nil {
			return nil, e
		}
		return func() {
			l := tview.NewList()
			l.SetBorder(true).SetTitle(" Change database · opens a fresh connection ")
			for _, name := range names {
				n := name
				l.AddItem(n, "", 0, func() { u.dismiss(); p := t.Profile; p.Database = n; u.connect(t, p, t.Password) })
			}
			u.popup(l, 80, 22)
		}, nil
	})
}
func (u *UI) execute() {
	t := u.active()
	if t.Session == nil {
		u.message("Connect first")
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
		u.message("No SQL to execute")
		return
	}
	bound := make([]db.Bound, len(statements))
	for i, s := range statements {
		bound[i] = db.Bound{SQL: s}
	}
	u.run(t, text, func(ctx context.Context, emit func(db.Event)) error { return t.Session.Run(ctx, bound, emit) })
}
func (u *UI) run(t *Tab, sqlText string, work func(context.Context, func(db.Event)) error, after ...func()) {
	if t.Busy {
		u.message("Session busy")
		return
	}
	for _, s := range t.Results {
		s.Close()
	}
	t.Results = nil
	t.Messages = nil
	t.Order = nil
	t.SelectedSet = 0
	if u.active() == t {
		u.showResults()
	}
	if u.cfg.History {
		if e := history.Add(u.historyPath(), history.Entry{At: time.Now(), Profile: t.Profile.DisplayName(), Database: t.Profile.Database, SQL: sqlText}); e != nil {
			u.notice("History error", e.Error())
			return
		}
	}
	u.task(t, "Execute", func(ctx context.Context) (func(), error) {
		started := time.Now()
		err := work(ctx, func(ev db.Event) {
			u.post(func() {
				switch ev.Kind {
				case "columns":
					t.Results = append(t.Results, ev.Store)
					if u.active() == t {
						u.showResults()
					}
				case "rows":
					if u.active() == t {
						u.table.SetTitle(fmt.Sprintf(" Results · set %d/%d · %d rows ", t.SelectedSet+1, len(t.Results), t.Results[t.SelectedSet].Count()))
					}
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
		return func() {
			if err != nil {
				t.Messages = append(t.Messages, err.Error())
			}
			if err == nil {
				t.Status = fmt.Sprintf("Complete in %s · %d result sets", elapsed.Round(time.Millisecond), len(t.Results))
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
	if t.Session == nil {
		u.message("Connect first")
		return
	}
	o, ok := u.selectedObject()
	if u.app.GetFocus() == t.Editor.Area && t.Object != nil {
		o = *t.Object
		ok = true
	}
	if !ok || (o.Kind != "procedure" && o.Kind != "function") {
		u.message("Select a procedure or function in Objects first")
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
	form.SetBorder(true).SetTitle(" " + o.Label() + " · Tab navigate ")
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
		modes[i] = tview.NewDropDown().SetLabel("Mode").SetOptions(options, nil).SetCurrentOption(initial)
		form.AddFormItem(values[i])
		form.AddFormItem(modes[i])
	}
	form.AddButton("Execute", func() {
		vals := make([]db.Value, len(params))
		for i := range params {
			_, mode := modes[i].GetCurrentOption()
			vals[i] = db.Value{Text: values[i].GetText(), Null: mode == "NULL", Omit: mode == "default"}
		}
		inv, e := t.Session.Adapter.Invoke(o, params, vals)
		if e != nil {
			form.SetTitle(" " + e.Error() + " ")
			return
		}
		u.dismiss()
		u.run(t, "-- Invoke "+o.Label()+" (parameter values excluded from history)", func(ctx context.Context, emit func(db.Event)) error { return t.Session.RunInvocation(ctx, inv, emit) })
	}).AddButton("Cancel", u.dismiss)
	list.AddItem(form, 0, 1, true)
	u.popup(list, 110, min(32, 8+len(params)*4))
}
func (u *UI) complete() {
	t := u.active()
	if t.Session == nil {
		u.message("Connect first")
		return
	}
	if t.Busy {
		u.message("Wait for catalog/query operation")
		return
	}
	// Load missing column metadata on demand; canceled work never changes editor text.
	var missing []db.Object
	for _, o := range t.Catalog {
		if (o.Kind == "table" || o.Kind == "view") && mentionsObject(t.Editor.Area.GetText(), o) {
			if _, ok := t.Columns[o.Label()]; !ok {
				missing = append(missing, o)
			}
		}
	}
	show := func() {
		prefix, start := t.Editor.Prefix()
		items := map[string]string{}
		keywords := "SELECT FROM WHERE JOIN LEFT RIGHT INNER OUTER ON GROUP BY ORDER HAVING INSERT INTO VALUES UPDATE SET DELETE CREATE ALTER DROP AS DISTINCT NULL IS NOT AND OR WITH BEGIN COMMIT ROLLBACK CASE WHEN THEN ELSE END UNION ALL EXISTS IN LIKE ASC DESC"
		switch t.Profile.Engine {
		case "sqlserver":
			keywords += " TOP OUTPUT DECLARE EXEC EXECUTE GO PRINT TRY CATCH MERGE"
		case "mysql":
			keywords += " LIMIT OFFSET CALL DELIMITER SHOW EXPLAIN REPLACE"
		case "postgres":
			keywords += " LIMIT OFFSET CALL RETURNING DO LANGUAGE EXPLAIN"
		}
		for _, k := range strings.Fields(keywords) {
			items[k] = "keyword"
		}
		for _, o := range t.Catalog {
			name := t.Session.Adapter.Quote(o.Schema) + "." + t.Session.Adapter.Quote(o.Name)
			if start > 0 && t.Editor.Area.GetText()[start-1] == '.' {
				name = t.Session.Adapter.Quote(o.Name)
			}
			items[name] = o.Kind + " " + o.Signature
			for _, c := range t.Columns[o.Label()] {
				items[t.Session.Adapter.Quote(c)] = "column · " + o.Label()
			}
		}
		u.completionList(t, prefix, start, items)
	}
	if len(missing) == 0 {
		show()
		return
	}
	u.task(t, "Load columns", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		columns := map[string][]string{}
		for _, o := range missing {
			c, e := t.Session.Adapter.Columns(ctx, t.Session.Conn, o)
			if e != nil {
				return nil, e
			}
			columns[o.Label()] = c
		}
		return func() {
			for k, v := range columns {
				t.Columns[k] = v
			}
			show()
		}, nil
	})
}
func mentionsObject(sql string, o db.Object) bool {
	return strings.Contains(strings.ToLower(sql), strings.ToLower(o.Name))
}

package ui

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"os"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"sqlterm/internal/history"
	"strings"
)

func (u *UI) saveFile() {
	t := u.active()
	save := func(path string) {
		if path == "" {
			return
		}
		write := func() {
			if e := os.WriteFile(path, []byte(t.Editor.Area.GetText()), 0600); e != nil {
				u.message(e.Error())
				return
			}
			t.Path = path
			t.Saved = t.Editor.Area.GetText()
			u.message("Saved " + path)
		}
		if _, e := os.Stat(path); e == nil && path != t.Path {
			u.confirm("Overwrite "+path+"?", write)
		} else {
			write()
		}
	}
	if t.Path != "" {
		save(t.Path)
	} else {
		u.prompt("Save SQL file", "query.sql", false, save)
	}
}
func (u *UI) openFile() {
	t := u.active()
	u.prompt("Open SQL file", "", false, func(path string) {
		open := func() {
			info, e := os.Stat(path)
			if e != nil {
				u.message(e.Error())
				return
			}
			if info.Size() > 8*1024*1024 {
				u.message("Editor file limit is 8 MiB")
				return
			}
			b, e := os.ReadFile(path)
			if e != nil {
				u.message(e.Error())
				return
			}
			t.Editor.SetText(string(b))
			t.Saved = string(b)
			t.Path = path
			t.Object = nil
			u.refreshBars()
		}
		if t.Editor.Area.GetText() != t.Saved {
			u.confirm("Replace unsaved SQL?", open)
		} else {
			open()
		}
	})
}
func (u *UI) searchEditor() {
	t := u.active()
	u.prompt("Find forward (wraps)", "", false, func(needle string) {
		if needle == "" {
			return
		}
		text := t.Editor.Area.GetText()
		p := t.Editor.Cursor()
		next := min(len(text), p+1)
		n := strings.Index(text[next:], needle)
		if n >= 0 {
			n += next
		} else {
			n = strings.Index(text, needle)
		}
		if n < 0 {
			u.message("No match")
			return
		}
		t.Editor.Area.Select(n, n)
		u.app.SetFocus(t.Editor.Area)
	})
}
func (u *UI) showHistory() {
	if !u.cfg.History {
		u.confirm("Enable local SQL history? Statements may contain sensitive literals. Invocation parameter values are excluded.", func() {
			next := u.cfg
			next.History = true
			if e := config.Save(u.configPath, next); e != nil {
				u.message(e.Error())
				return
			}
			u.cfg = next
			u.showHistory()
		})
		return
	}
	entries, e := history.Load(u.historyPath())
	if e != nil {
		u.message(e.Error())
		return
	}
	l := tview.NewList()
	l.SetBorder(true).SetTitle(" History · Enter loads SQL into current tab ")
	l.AddItem("Clear history", "", 'd', func() {
		u.confirm("Delete all local SQL history?", func() {
			if e := history.Clear(u.historyPath()); e != nil {
				u.message(e.Error())
			}
		})
	})
	l.AddItem("Disable history", "", 'x', func() {
		next := u.cfg
		next.History = false
		if e := config.Save(u.configPath, next); e != nil {
			u.message(e.Error())
			return
		}
		u.cfg = next
		u.dismiss()
	})
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		summary := strings.ReplaceAll(entry.SQL, "\n", " ")
		if len(summary) > 100 {
			summary = summary[:100] + "…"
		}
		l.AddItem(entry.At.Format("01-02 15:04")+" · "+entry.Profile+" / "+entry.Database, summary, 0, func() {
			u.dismiss()
			t := u.active()
			load := func() { t.Editor.SetText(entry.SQL); t.Path = ""; t.Object = nil; u.refreshBars() }
			if t.Editor.Area.GetText() != t.Saved {
				u.confirm("Replace unsaved SQL with history entry?", load)
			} else {
				load()
			}
		})
	}
	u.popup(l, 110, 28)
}
func (u *UI) template() {
	t := u.active()
	if t.SelectingDatabase {
		u.message("Choose a database first (F4)")
		return
	}
	if t.Session == nil {
		u.message("Connect first")
		return
	}
	if t.Profile.Engine == "mysql" && t.Profile.Database == "" {
		u.message("Choose a database with Space b first")
		return
	}
	u.prompt("New procedure name (unqualified)", "new_procedure", false, func(name string) {
		if name == "" {
			return
		}
		a := t.Session.Adapter
		schema := "public"
		if a.Engine() == "sqlserver" {
			schema = "dbo"
		}
		if a.Engine() == "mysql" {
			schema = t.Profile.Database
		}
		q := a.Quote(schema) + "." + a.Quote(name)
		text := ""
		switch a.Engine() {
		case "sqlserver":
			text = "CREATE PROCEDURE " + q + "\n    @value int\nAS\nBEGIN\n    SET NOCOUNT ON;\n    SELECT @value AS value;\nEND;"
		case "mysql":
			text = "CREATE PROCEDURE " + q + "(IN p_value INT)\nBEGIN\n    SELECT p_value AS value;\nEND"
		default:
			text = "CREATE OR REPLACE PROCEDURE " + q + "(IN p_value integer)\nLANGUAGE plpgsql\nAS $body$\nBEGIN\n    RAISE NOTICE 'value: %', p_value;\nEND;\n$body$;"
		}
		load := func() { t.Editor.SetText(text); t.Path = ""; t.Saved = ""; t.Object = nil; u.refreshBars() }
		if t.Editor.Area.GetText() != t.Saved {
			u.confirm("Replace unsaved SQL with template?", load)
		} else {
			load()
		}
	})
}
func (u *UI) applyDefinition() {
	t := u.active()
	if t.SelectingDatabase {
		u.message("Choose a database first (F4)")
		return
	}
	if t.Session == nil {
		u.message("Connect first")
		return
	}
	if t.Busy {
		u.message("Session busy")
		return
	}
	text := t.Editor.Area.GetText()
	plan, e := db.PrepareDefinition(t.Session.Adapter, t.Object, text)
	if e != nil {
		u.message(e.Error())
		return
	}
	preview := "CHANGES\n" + changeBlock(t.Original, text) + "\n\nSQL TO EXECUTE\n"
	for _, st := range plan {
		preview += st.SQL + "\n\n"
	}
	if t.Profile.Engine == "mysql" && len(plan) > 1 {
		preview = "MYSQL: DROP + CREATE is not atomic. If CREATE fails, the routine remains absent. Grants may need to be restored.\n\n" + preview
	}
	if t.Profile.Engine == "postgres" && t.Object != nil && t.Object.Kind != "view" {
		preview = "POSTGRESQL: changing input argument types creates another overload; the original remains.\n\n" + preview
	}
	v := tview.NewTextView().SetText(preview).SetScrollable(true).SetWrap(false)
	v.SetBorder(true).SetTitle(" Review definition · scroll / Tab to buttons ")
	buttons := tview.NewForm().AddButton("Cancel", u.dismiss).AddButton("Apply to server", func() {
		u.dismiss()
		u.run(t, text, func(ctx context.Context, emit func(db.Event)) error {
			for _, st := range plan {
				r, e := t.Session.Conn.ExecContext(ctx, st.SQL)
				if e != nil {
					return e
				}
				if n, e := r.RowsAffected(); e == nil {
					emit(db.Event{Kind: "message", Message: fmt.Sprintf("%d affected rows", n)})
				}
			}
			return nil
		}, func() { t.Original = text; u.refreshCatalogTab(t) })
	})
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(v, 0, 1, true).AddItem(buttons, 3, 0, false)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		if k.Key() == tcell.KeyTab {
			if u.app.GetFocus() == v {
				u.app.SetFocus(buttons)
				return nil
			} else if _, button := buttons.GetFocusedItemIndex(); button == 1 {
				u.app.SetFocus(v)
				return nil
			}
		}
		if k.Key() == tcell.KeyBacktab {
			if _, button := buttons.GetFocusedItemIndex(); button == 0 {
				u.app.SetFocus(v)
				return nil
			}
		}
		return k
	})
	u.popup(panel, 115, 32)
}

// Display a compact, exact contiguous replacement, preserving surrounding lines.
func changeBlock(old, new string) string {
	a, b := strings.Split(old, "\n"), strings.Split(new, "\n")
	start := 0
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	if start == endA && start == endB {
		return "(no textual changes)"
	}
	var s strings.Builder
	fmt.Fprintf(&s, "@@ old line %d, new line %d @@\n", start+1, start+1)
	for _, line := range a[start:endA] {
		s.WriteString("- " + line + "\n")
	}
	for _, line := range b[start:endB] {
		s.WriteString("+ " + line + "\n")
	}
	return s.String()
}

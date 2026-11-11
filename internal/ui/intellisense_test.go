package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/db"
	"strings"
	"testing"
)

func dispatch(u *UI, key tcell.Key, r rune) {
	k := u.key(tcell.NewEventKey(key, r, 0))
	if k != nil {
		u.app.GetFocus().InputHandler()(k, func(p tview.Primitive) { u.app.SetFocus(p) })
	}
}
func TestAutomaticIntelliSenseDoesNotStealFocusAndAcceptsTab(t *testing.T) {
	u, _ := modeTestUI(t)
	tab := u.active()
	tab.Session = &db.Session{Adapter: db.NewAdapter("sqlserver")}
	tab.Profile.Engine = "sqlserver"
	o := db.Object{Schema: "dbo", Name: "Customers", Kind: "table"}
	tab.Catalog = []db.Object{o}
	tab.Columns[o.Label()] = []string{"Name", "Number"}
	sql := "SELECT c.Na FROM dbo.Customers AS c"
	tab.Editor.SetText(sql)
	u.app.ForceDraw()
	tab.Editor.Area.Select(len("SELECT c.Na"), len("SELECT c.Na"))
	dispatch(u, tcell.KeyRune, 'i')
	dispatch(u, tcell.KeyRune, 'm')
	u.app.ForceDraw()
	if u.suggestions == nil || !tab.Editor.Area.HasFocus() || tab.Editor.Mode != "INSERT" {
		t.Fatal("automatic suggestions missing or stole editor focus")
	}
	if len(u.suggestions.Items) != 1 || u.suggestions.Items[0].Text != "[Name]" {
		t.Fatal(u.suggestions.Items)
	}
	dispatch(u, tcell.KeyTab, 0)
	u.app.ForceDraw()
	if !strings.Contains(tab.Editor.Area.GetText(), "c.[Name] FROM") || u.suggestions != nil || tab.Editor.Mode != "INSERT" {
		t.Fatal("completion did not replace the prefix in place")
	}
	dispatch(u, tcell.KeyEscape, 0)
	if tab.Editor.Mode != "NORMAL" {
		t.Fatal("Escape did not restore normal mode")
	}
}
func TestAutomaticProcedureParameterGuidanceAndAcceptance(t *testing.T) {
	for _, trigger := range []string{" ", " @", " @i"} {
		for _, accept := range []struct {
			name string
			key  tcell.Key
		}{{"Tab", tcell.KeyTab}, {"Enter", tcell.KeyEnter}} {
			t.Run(trigger+"/"+accept.name, func(t *testing.T) {
				u, _ := modeTestUI(t)
				tab := u.active()
				tab.Profile.Engine = "sqlserver"
				tab.Catalog = []db.Object{{Schema: "dbo", Name: "Report", Kind: "procedure", Signature: "@id int, @name nvarchar"}}
				sql := "EXEC dbo.Report"
				tab.Editor.SetText(sql)
				u.app.ForceDraw()
				tab.Editor.Area.Select(len(sql), len(sql))
				dispatch(u, tcell.KeyRune, 'i')
				for _, r := range trigger {
					dispatch(u, tcell.KeyRune, r)
				}
				u.app.ForceDraw()
				if u.suggestions == nil || !tab.Editor.Area.HasFocus() || tab.Editor.Mode != "INSERT" {
					t.Fatal("procedure parameter popup missing or stole focus")
				}
				items := u.suggestions.Items
				if items[0].Text != "@id = " || items[0].Detail != "parameter · @id int" {
					t.Fatalf("wrong parameter name/type: %#v", items)
				}
				if tab.Editor.Area.GetText() != sql+trigger || tab.Session != nil || tab.Busy {
					t.Fatal("showing guidance changed SQL or started execution")
				}
				dispatch(u, accept.key, 0)
				u.app.ForceDraw()
				if tab.Editor.Area.GetText() != sql+" @id = " || u.suggestions != nil || tab.Editor.Mode != "INSERT" {
					t.Fatal("parameter acceptance did not replace only the prefix")
				}
				if tab.Session != nil || tab.Busy {
					t.Fatal("accepting guidance started execution")
				}
				dispatch(u, tcell.KeyRune, '1')
				if u.suggestions != nil {
					t.Fatal("parameter popup opened while entering a value")
				}
				dispatch(u, tcell.KeyRune, ',')
				if u.suggestions == nil || u.suggestions.Items[0].Text != "@id = " {
					t.Fatal("parameter popup did not reopen at the next argument")
				}
				if tab.Editor.Area.GetText() != sql+" @id = 1," || tab.Session != nil || tab.Busy {
					t.Fatal("next argument guidance changed SQL or started execution")
				}
			})
		}
	}
}

func TestCompletionNavigationDoesNotCreateTab(t *testing.T) {
	u, _ := modeTestUI(t)
	tab := u.active()
	tab.Session = &db.Session{Adapter: db.NewAdapter("postgres")}
	tab.Profile.Engine = "postgres"
	tab.Editor.SetText("SE")
	u.app.ForceDraw()
	tab.Editor.Area.Select(2, 2)
	dispatch(u, tcell.KeyRune, 'i')
	u.complete()
	if u.suggestions == nil {
		t.Fatal("Ctrl+Space completion missing")
	}
	count := len(u.tabs)
	dispatch(u, tcell.KeyCtrlN, 0)
	if len(u.tabs) != count {
		t.Fatal("completion navigation created a session")
	}
	dispatch(u, tcell.KeyEscape, 0)
	if u.suggestions != nil || tab.Editor.Mode != "NORMAL" {
		t.Fatal("completion did not dismiss")
	}
}
func TestOfflineKeywordCompletionDoesNotNeedAConnection(t *testing.T) {
	u, _ := modeTestUI(t)
	dispatch(u, tcell.KeyRune, 'i')
	dispatch(u, tcell.KeyRune, 'S')
	dispatch(u, tcell.KeyRune, 'E')
	u.app.ForceDraw()
	if u.suggestions == nil || u.suggestions.Items[0].Text != "SELECT" || !u.active().Editor.Area.HasFocus() {
		t.Fatal("offline SQL keyword completion missing")
	}
	dispatch(u, tcell.KeyTab, 0)
	if u.active().Editor.Area.GetText() != "SELECT" || u.active().Session != nil {
		t.Fatal("offline completion changed connection state")
	}
}

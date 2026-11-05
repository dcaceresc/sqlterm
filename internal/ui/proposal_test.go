package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"sqlterm/internal/result"
)

func TestExecutionProvenanceAndConfirmedCancellation(t *testing.T) {
	u := runningUI(t, config.Config{})
	tab := u.active()
	store, err := result.New([]string{"name"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Append([]result.Cell{{Text: "received"}}); err != nil {
		t.Fatal(err)
	}
	driverCancelled, release := make(chan struct{}), make(chan struct{})
	u.app.QueueUpdateDraw(func() {
		tab.Profile = config.Profile{Name: "demo", Folder: "PROD/API", Database: "Academia"}
		tab.Editor.SetText("SELECT name FROM students;")
		u.run(tab, tab.Editor.Area.GetText(), func(ctx context.Context, emit func(db.Event)) error {
			emit(db.Event{Kind: "columns", Store: store})
			emit(db.Event{Kind: "rows"})
			<-ctx.Done()
			close(driverCancelled)
			<-release
			return ctx.Err()
		})
	})
	waitUI(t, u, func() bool { return len(tab.Results) == 1 && tab.Execution.Phase == "Receiving rows" })
	u.app.QueueUpdateDraw(func() {
		u.cancelOperation()
		if tab.Execution.Phase != "Cancelling" || !tab.Busy {
			t.Error("cancellation announced before driver finished")
		}
	})
	select {
	case <-driverCancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("driver did not receive cancellation")
	}
	close(release)
	waitUI(t, u, func() bool { return !tab.Busy })
	u.app.QueueUpdateDraw(func() {
		if tab.Execution.Phase != "Cancelled" || tab.Execution.Database != "Academia" || tab.Execution.Source != "Query 1" || tab.Results[0] != store {
			t.Error("lost provenance or partial rows")
		}
		tab.Editor.SetText("SELECT other FROM somewhere;")
		u.refreshWorkspace()
		info := u.resultInfo.GetText(true)
		if !strings.Contains(info, "Results of previous SQL") || !strings.Contains(info, "Partial results") || !strings.Contains(info, "Academia") {
			t.Error(info)
		}
		if !strings.Contains(u.contextBar.GetText(true), "PROD") {
			t.Error("environment missing")
		}
	})
}

func TestDialogsRestoreOpeningControlAndDoNotStealContext(t *testing.T) {
	u, _ := modeTestUI(t)
	for _, control := range []tview.Primitive{u.table, u.objects, u.search, u.active().Editor.Area} {
		u.app.SetFocus(control)
		u.prompt("File", "query.sql", false, func(string) {})
		u.notice("Nested", "Details")
		u.dismiss()
		if u.app.GetFocus() != control {
			t.Fatalf("did not restore %T", control)
		}
	}
}

func TestResponsiveLayoutPreservesSQLAndSavedPreferences(t *testing.T) {
	u, s := modeTestUI(t)
	tab := u.active()
	tab.Profile = config.Profile{Name: "demo", Folder: "QA", Database: "Reportes"}
	tab.Editor.SetText("SELECT 42;")
	u.saveLayout(config.Layout{Maximized: "results", HideCatalog: true, CatalogWidth: 38, EditorPercent: 25})
	if !u.table.HasFocus() {
		t.Fatal("maximize results did not focus results")
	}
	loaded, err := config.Load(u.configPath)
	if err != nil || loaded.Layout.Maximized != "results" {
		t.Fatal("layout not persisted", err)
	}
	for _, size := range [][2]int{{80, 24}, {120, 40}, {180, 50}} {
		s.SetSize(size[0], size[1])
		for _, key := range []tcell.Key{tcell.KeyF6, tcell.KeyF7, tcell.KeyF8} {
			dispatch(u, key, 0)
			u.app.ForceDraw()
			text := screenText(s)
			if !strings.Contains(text, "SQLTerm") || !strings.Contains(text, "QA") || !strings.Contains(text, "Reportes") {
				t.Fatalf("lost context at %v: %s", size, text)
			}
			if size[0] == 80 {
				for _, item := range []struct {
					key   tcell.Key
					title string
				}{{tcell.KeyF6, "> Objects"}, {tcell.KeyF7, "> Editor"}, {tcell.KeyF8, "> Results"}} {
					if item.key == key && !strings.Contains(text, item.title) {
						t.Fatalf("missing active panel %s", item.title)
					}
				}
			}
		}
	}
	if tab.Editor.Area.GetText() != "SELECT 42;" {
		t.Fatal("layout changed SQL")
	}
}

func TestCatalogHierarchySearchAndTabFilterRestoration(t *testing.T) {
	u, _ := modeTestUI(t)
	first := u.active()
	first.Catalog = []db.Object{{Schema: "dbo", Name: "VeryLongStudentsName", Kind: "table"}, {Schema: "dbo", Name: "calculate", Kind: "procedure"}, {Schema: "audit", Name: "events", Kind: "view"}}
	u.search.SetText("dbotable")
	if len(u.catalogRows) != 1 {
		t.Fatal("schema/type search did not find object")
	}
	for i := range u.catalogRows {
		u.objects.SetCurrentItem(i)
	}
	if o, ok := u.selectedObject(); !ok || o.Name != "VeryLongStudentsName" {
		t.Fatal("object identity lost")
	}
	u.newTab(nil)
	if u.search.GetText() != "" {
		t.Fatal("new tab inherited search")
	}
	u.current = 0
	u.showTab()
	if u.search.GetText() != "dbotable" {
		t.Fatal("tab switch lost filter")
	}
	if o, ok := u.selectedObject(); !ok || o.Name != "VeryLongStudentsName" {
		t.Fatal("tab switch lost selection")
	}
}

func TestFailedDatabaseChangePreservesSessionSQLAndResults(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	old := &db.Session{Adapter: db.NewAdapter("postgres")}
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Status = "Completed"
		tab.Session = old
		tab.Profile = config.Profile{Name: "demo", Engine: "postgres", Database: "Academia"}
		tab.Editor.SetText("unsaved SQL")
		tab.Results = []*result.Store{s}
		u.openSession = func(context.Context, config.Profile, string) (*db.Session, error) {
			return nil, errors.New("connection refused")
		}
		p := tab.Profile
		p.Database = "Reportes"
		u.connect(tab, p, "")
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		if tab.Session != old || tab.Profile.Database != "Academia" || tab.Editor.Area.GetText() != "unsaved SQL" || tab.Results[0] != s {
			t.Error("failed database change damaged previous context")
		}
		tab.Session = nil
	})
}

func TestExecutionConfirmationPolicies(t *testing.T) {
	for _, tc := range []struct {
		policy, folder, sql string
		want                bool
	}{{"", "PROD", "DELETE FROM t", false}, {"production", "DEV", "DELETE FROM t", false}, {"production", "PROD/API", "DELETE FROM t", true}, {"production", "PROD", "SELECT 1", false}, {"production", "PROD", "SELECT * INTO t FROM s", true}, {"production", "PROD", "WITH x AS (SELECT 1) DELETE FROM t", true}, {"all", "DEV", "SELECT 1", true}} {
		if got := confirmExecution(tc.policy, tc.folder, []string{tc.sql}); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}

func TestProposalBilingualWorkspaceScreens(t *testing.T) {
	for _, lang := range []string{"en", "es"} {
		t.Run(lang, func(t *testing.T) {
			u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Language: lang})
			defer u.cancel()
			screen := tcell.NewSimulationScreen("UTF-8")
			u.app.SetScreen(screen)
			defer screen.Fini()
			tab := u.active()
			tab.Status = "Completed"
			tab.Profile = config.Profile{Name: "demo-sql", Folder: "DEV", Database: "Academia", Engine: "sqlserver"}
			tab.Session = &db.Session{Adapter: db.NewAdapter("sqlserver")}
			tab.Editor.SetText("SELECT TOP (100)\n    IdAlumno, Nombre, Estado\nFROM dbo.Alumnos\nORDER BY IdAlumno;")
			tab.Catalog = []db.Object{{Schema: "dbo", Name: "Alumnos", Kind: "table"}, {Schema: "dbo", Name: "Cursos", Kind: "table"}, {Schema: "dbo", Name: "Matriculas", Kind: "table"}, {Schema: "dbo", Name: "ResumenAlumnos", Kind: "view"}, {Schema: "dbo", Name: "BuscarAlumno", Kind: "procedure"}}
			store, err := result.New([]string{"IdAlumno", "Nombre", "Estado", "FechaIngreso"})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			for i, name := range []string{"Ana Demo", "Luis Prueba", "María Ejemplo", "Jorge Simulado", "Lucía Demostración"} {
				if err := store.Append([]result.Cell{{Text: fmt.Sprint(1001 + i)}, {Text: name}, {Text: "Activo"}, {Text: "2026-11-05"}}); err != nil {
					t.Fatal(err)
				}
			}
			tab.Results = []*result.Store{store}
			tab.Execution = &execution{ID: 7, EditorSQL: tab.Editor.Area.GetText(), Source: "Alumnos.sql", Profile: "DEV/demo-sql", Database: "Academia", Scope: "Execute script", Started: time.Date(2026, 11, 5, 14, 30, 0, 0, time.Local), Duration: 136 * time.Millisecond, Phase: "Completed"}
			u.filterObjects()
			u.showResults()
			u.app.SetFocus(u.table)
			u.table.Select(1, 1)
			for _, size := range [][2]int{{80, 24}, {120, 40}, {180, 50}} {
				screen.SetSize(size[0], size[1])
				u.app.ForceDraw()
				text := screenText(screen)
				wanted := "Results"
				if lang == "es" {
					wanted = "Resultados"
				}
				for _, label := range []string{wanted, "DEV", "demo-sql", "Academia", "Ana Demo"} {
					if !strings.Contains(text, label) {
						t.Errorf("%v missing %q: %s", size, label, text)
					}
				}
				if strings.Contains(text, "%!") {
					t.Fatal("invalid localized format", text)
				}
				if dir := os.Getenv("SQLTERM_UI_SNAPSHOTS"); dir != "" {
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("propuesta-ui-%dx%d-%s.txt", size[0], size[1], lang)), []byte(text), 0600); err != nil {
						t.Fatal(err)
					}
					type pixel struct {
						Text   string
						FG, BG string
					}
					cells, w, h := screen.GetContents()
					pixels := make([]pixel, len(cells))
					for i, c := range cells {
						fg, bg, _ := c.Style.Decompose()
						if fg.Hex() < 0 {
							fg = u.palette.Foreground
						}
						if bg.Hex() < 0 {
							bg = u.palette.Background
						}
						pixels[i] = pixel{Text: string(c.Runes), FG: colorTag(fg), BG: colorTag(bg)}
					}
					data, err := json.Marshal(struct {
						Width, Height int
						Cells         []pixel
					}{w, h, pixels})
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("propuesta-ui-%dx%d-%s.json", size[0], size[1], lang)), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			u.languageMenu()
			list := u.dialogContent.(*tview.List)
			index := 0
			if lang == "en" {
				index = 1
			}
			list.GetItemSelectedFunc(index)()
			loaded, err := config.Load(u.configPath)
			if err != nil || loaded.Language == lang {
				t.Fatal("language preference not saved", err)
			}
		})
	}
}

func TestRangeCopyAndInspectorKeepOriginalValues(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"name"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"first", "second", "third"} {
		if err = s.Append([]result.Cell{{Text: value}}); err != nil {
			t.Fatal(err)
		}
	}
	copied := make(chan string, 2)
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Results = []*result.Store{s}
		tab.Order = []int{2, 0, 1}
		u.showResults()
		u.table.Select(3, 0)
		tab.CopyAnchor = 2
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		u.copyResults("selection", true)
	})
	select {
	case text := <-copied:
		if text != "name\nfirst\nsecond\n" {
			t.Fatalf("wrong received selection/order: %q", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("range copy timed out")
	}
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if !strings.Contains(u.active().Status, "Copied 2 rows") {
			t.Error("incorrect copy scope")
		}
		u.app.SetFocus(u.table)
		u.viewCell(1, 0)
		panel := u.dialogContent.(*tview.Flex)
		buttons := panel.GetItem(1).(*tview.Form)
		buttons.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	select {
	case text := <-copied:
		if text != "third" {
			t.Fatal("inspector did not copy complete original value", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("inspector copy timed out")
	}
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		u.dismiss()
		if !u.table.HasFocus() {
			t.Error("inspector did not restore results focus")
		}
	})
}

func TestSearchableSelectorsAndContextHelp(t *testing.T) {
	u, _ := modeTestUI(t)
	u.cfg.Profiles = []config.Profile{{Name: "Academia DEV", Folder: "DEV", Engine: "sqlserver", Host: "dev-host", User: "user", Database: "Academia", CredentialID: "private-secret-reference"}, {Name: "Reports PROD", Folder: "PROD", Engine: "postgres", Host: "prod-host", User: "user"}}
	u.chooseConnection()
	panel := u.dialogContent.(*tview.Flex)
	search := panel.GetItem(0).(*tview.InputField)
	tree := panel.GetItem(1).(*tview.TreeView)
	if !search.HasFocus() {
		t.Fatal("connection search must have initial focus")
	}
	search.SetText("prod")
	u.app.ForceDraw()
	ref := tree.GetCurrentNode().GetReference().(connectionRef)
	if ref.ProfileIndex != 1 || ref.FolderNode {
		t.Fatal("filter did not select matching profile")
	}
	if details := panel.GetItem(2).(*tview.TextView).GetText(false); !strings.Contains(details, "prod-host") {
		t.Fatal("missing selected connection details", details)
	}
	u.dismiss()
	u.app.SetFocus(u.table)
	u.showHelp()
	help := u.dialogContent.(*tview.Flex)
	section := help.GetItem(1).(*tview.DropDown)
	if _, name := section.GetCurrentOption(); name != "Results" {
		t.Fatal("help did not open results context")
	}
	help.GetItem(0).(*tview.InputField).SetText("TSV")
	text := help.GetItem(2).(*tview.TextView).GetText(false)
	if !strings.Contains(text, "TSV") || strings.Contains(text, "New session") {
		t.Fatal("help filtering failed", text)
	}
	u.dismiss()
	u.commandPalette()
	commands := u.dialogContent.(*tview.Flex)
	commands.GetItem(0).(*tview.InputField).SetText("Generate SELECT")
	list := commands.GetItem(1).(*tview.List)
	if list.GetItemCount() != 1 {
		t.Fatal("command search failed")
	}
	_, detail := list.GetItemText(0)
	if !strings.Contains(detail, "Unavailable") {
		t.Fatal("inapplicable command not explained")
	}
}

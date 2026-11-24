package ui

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

func TestOpenTableShowsDefinitionAndKeepsGenerateSelect(t *testing.T) {
	for _, engine := range []string{"sqlserver", "postgres", "mysql"} {
		t.Run(engine, func(t *testing.T) {
			u := runningUI(t, config.Config{FlatCatalog: true})
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			conn, err := database.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			a := db.NewAdapter(engine)
			o := db.Object{Schema: "sales", Name: "orders", Kind: "table", ID: "42"}
			target := a.Quote(o.Schema) + "." + a.Quote(o.Name)
			want := "CREATE TABLE " + target + " (\n    id int NOT NULL\n);"
			if engine == "mysql" {
				mock.ExpectQuery("SHOW CREATE TABLE").WillReturnRows(sqlmock.NewRows([]string{"Table", "Create Table"}).AddRow(o.Name, want))
			} else {
				columns, constraints, indexes := "FROM sys.columns", "FROM sys.key_constraints", "FROM sys.indexes"
				if engine == "postgres" {
					columns, constraints, indexes = "FROM pg_attribute", "FROM pg_constraint", "FROM pg_index"
				}
				mock.ExpectQuery(columns).WithArgs(o.ID).WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow("id int NOT NULL"))
				mock.ExpectQuery(constraints).WithArgs(o.ID).WillReturnRows(sqlmock.NewRows([]string{"ddl"}))
				mock.ExpectQuery(indexes).WithArgs(o.ID).WillReturnRows(sqlmock.NewRows([]string{"ddl"}))
			}
			mock.ExpectQuery("SELECT (name|attname|COLUMN_NAME)").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("id"))
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				tab.Session = &db.Session{DB: database, Conn: conn, Adapter: a}
				tab.Catalog = []db.Object{o}
				u.rebuildCatalog()
				u.objects.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(_ tview.Primitive) {})
			})
			waitUI(t, u, func() bool { return !u.active().Busy })
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				if tab.Editor.Area.GetText() != want || tab.Saved != want || tab.Original != want || tab.Path != "" || tab.Object != nil || len(tab.Columns[o.Label()]) != 1 {
					t.Error("table definition or editor state incorrect")
				}
				tab.Editor.SetText("")
				u.insertObjectName(true)
				if got := tab.Editor.Area.GetText(); got != "SELECT * FROM "+target+";" {
					t.Errorf("Generate SELECT = %q", got)
				}
			})
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

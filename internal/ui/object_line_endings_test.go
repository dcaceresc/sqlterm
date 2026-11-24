package ui

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/db"
)

type objectDefinitionAdapter struct {
	db.Adapter
	text string
}

func (a objectDefinitionAdapter) Definition(context.Context, *sql.Conn, db.Object) (string, error) {
	return a.text, nil
}

func (a objectDefinitionAdapter) Columns(context.Context, *sql.Conn, db.Object) ([]string, error) {
	return nil, nil
}

func TestNeovimOpensDatabaseDefinitionsWithoutCarriageReturns(t *testing.T) {
	for _, tc := range []struct{ kind, ddl string }{
		{"procedure", "CREATE PROCEDURE dbo.example AS"},
		{"view", "CREATE VIEW dbo.example AS"},
		{"function", "CREATE FUNCTION dbo.example() RETURNS int AS"},
		{"table", "CREATE TABLE dbo.example (id int);"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			u := runningNeovimUI(t)
			o := db.Object{Kind: tc.kind, Schema: "dbo", Name: "example"}
			want := tc.ddl + "\n-- definition\n"
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				tab.Session = &db.Session{Adapter: objectDefinitionAdapter{Adapter: db.NewAdapter("sqlserver"), text: tc.ddl + "\r\n-- definition\r\n"}}
				u.cfg.FlatCatalog = true
				tab.Catalog = []db.Object{o}
				u.rebuildCatalog()
				u.objects.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
			})
			waitUI(t, u, func() bool { return !u.active().Busy })
			u.app.QueueUpdateDraw(func() {
				tab := u.active()
				if text := tab.Editor.Area.GetText(); text != want || tab.Saved != want || tab.Original != want || tab.Path != "" {
					t.Errorf("text=%q saved=%q original=%q path=%q; want %q in the editor, saved and original", text, tab.Saved, tab.Original, tab.Path, want)
				}
				if tc.kind == "table" && tab.Object != nil || tc.kind != "table" && (tab.Object == nil || *tab.Object != o) {
					t.Errorf("unexpected object for %s: %+v", tc.kind, tab.Object)
				}
			})
		})
	}
}

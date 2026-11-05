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

func TestSelectorDialogsDismissAndReopen(t *testing.T) {
	for _, selector := range []tcell.Key{tcell.KeyF2, tcell.KeyF3, tcell.KeyF4} {
		for _, exit := range []string{"editor click", "objects click", "F6", "F7", "F8", "Escape", "Ctrl+C", "same selector"} {
			t.Run(tcell.KeyNames[selector]+"/"+exit, func(t *testing.T) {
				u := runningUI(t, config.Config{})
				var mock sqlmock.Sqlmock
				if selector == tcell.KeyF4 {
					connDB, m, err := sqlmock.New()
					if err != nil {
						t.Fatal(err)
					}
					mock = m
					conn, err := connDB.Conn(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					for range 2 {
						mock.ExpectQuery("SELECT name FROM sys.databases").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("DEV"))
					}
					u.app.QueueUpdateDraw(func() {
						u.active().Session = &db.Session{DB: connDB, Conn: conn, Adapter: db.NewAdapter("sqlserver")}
					})
				}
				original := u.palette.ID
				u.app.QueueUpdateDraw(func() { dispatch(u, selector, 0) })
				waitUI(t, u, func() bool { return u.dialogContent != nil && !u.active().Busy })
				var previous tview.Primitive
				u.app.QueueUpdateDraw(func() {
					previous = u.dialogContent
					if selector == tcell.KeyF3 {
						u.dialogContent.(*tview.List).SetCurrentItem(1)
						if u.palette.ID == original {
							t.Error("theme preview fixture did not change theme")
						}
					}
					if exit == "editor click" || exit == "objects click" {
						target := tview.Primitive(u.active().Editor.Area)
						if exit == "objects click" {
							target = u.objects
						}
						x, y, _, _ := target.GetRect()
						event := tcell.NewEventMouse(x+2, y+2, tcell.Button1, 0)
						if u.dialogContains(x+2, y+2) {
							t.Error("fixture click is inside dialog")
						}
						for _, action := range []tview.MouseAction{tview.MouseLeftDown, tview.MouseLeftUp, tview.MouseLeftClick} {
							forward, forwardedAction := u.app.GetMouseCapture()(event, action)
							if forward != nil {
								u.pages.MouseHandler()(forwardedAction, forward, func(p tview.Primitive) { u.app.SetFocus(p) })
							}
						}
						if !target.HasFocus() {
							t.Error("clicked panel did not receive focus")
						}
					} else {
						key := map[string]tcell.Key{"F6": tcell.KeyF6, "F7": tcell.KeyF7, "F8": tcell.KeyF8, "Escape": tcell.KeyEscape, "Ctrl+C": tcell.KeyCtrlC, "same selector": selector}[exit]
						dispatch(u, key, 0)
					}
					if exit != "same selector" {
						if name, _ := u.pages.GetFrontPage(); name != "main" || u.dialogContent != nil || u.closing {
							t.Error("dialog remained visible after cancellation")
						}
						if selector == tcell.KeyF3 && u.palette.ID != original {
							t.Error("theme preview not reverted")
						}
						dispatch(u, selector, 0)
					}
				})
				waitUI(t, u, func() bool { return u.dialogContent != nil && u.dialogContent != previous && !u.active().Busy })
				if mock != nil {
					if err := mock.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

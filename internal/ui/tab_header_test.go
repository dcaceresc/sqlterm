package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

func clickTabHeader(u *UI, x int) {
	for _, step := range []struct {
		action tview.MouseAction
		button tcell.ButtonMask
	}{{tview.MouseLeftDown, tcell.Button1}, {tview.MouseLeftUp, tcell.ButtonNone}, {tview.MouseLeftClick, tcell.ButtonNone}} {
		event, action := u.app.GetMouseCapture()(tcell.NewEventMouse(x, 0, step.button, 0), step.action)
		if event != nil {
			u.pages.MouseHandler()(action, event, func(p tview.Primitive) { u.app.SetFocus(p) })
		}
	}
}

func firstHeaderClose(t *testing.T, s *cursorScreen) int {
	t.Helper()
	cells, width, _ := s.GetContents()
	for x, cell := range cells[:width] {
		if string(cell.Runes) == "×" {
			return x
		}
	}
	t.Fatalf("no visible close button: %q", strings.SplitN(screenText(s), "\n", 2)[0])
	return -1
}

func TestInteractiveTabHeaderSelectionAndContext(t *testing.T) {
	u, s := modeTestUI(t)
	s.SetSize(80, 25)
	first := u.active()
	first.Path = "界[red].sql"
	u.newTab(nil)
	second := u.active()
	second.Profile = config.Profile{Name: "local", Folder: "PROD/API", Database: "master"}
	second.Session = &db.Session{Adapter: db.NewAdapter("sqlserver")}
	u.app.ForceDraw()
	row := strings.SplitN(screenText(s), "\n", 2)[0]
	context := "SQLTerm │ PROD │ local │ master │ Connected"
	if !strings.HasPrefix(row, " 1:界 [red].sql ×") || !strings.Contains(row, "2:Query 2 ×") || !strings.HasSuffix(row, context) {
		t.Fatalf("tabs and right-aligned connection not in row 0: %q", row)
	}
	_, _, style, _ := s.GetContent(80-len([]rune("PROD │ local │ master │ Connected")), 0)
	if fg, _, _ := style.Decompose(); fg != tcell.ColorOrangeRed {
		t.Errorf("PROD color = %v", fg)
	}
	clickTabHeader(u, 79)
	if u.active() != second || len(u.tabs) != 2 || !second.Editor.Area.HasFocus() {
		t.Fatal("connection context acted as a tab or stole focus")
	}
	clickTabHeader(u, 4)
	if u.active() != first || !first.Editor.Area.HasFocus() || len(u.tabs) != 2 {
		t.Fatal("wide label did not select and focus its tab")
	}
	u.app.ForceDraw()
	clickTabHeader(u, firstHeaderClose(t, s))
	if u.active() != second || len(u.tabs) != 1 {
		t.Fatal("close button following a wide label did not close that tab")
	}
}

func TestInteractiveTabHeaderClosesInactiveWithSafeguards(t *testing.T) {
	u, s := modeTestUI(t)
	first := u.active()
	first.Editor.SetText("SELECT 1;")
	u.newTab(nil)
	second := u.active()
	u.app.ForceDraw()
	x := firstHeaderClose(t, s)
	first.Busy = true
	clickTabHeader(u, x)
	if u.dialogContent != nil || u.active() != second || len(u.tabs) != 2 || second.Status != "Cancel execution before closing this tab" || first.Editor.Area.GetText() != "SELECT 1;" {
		t.Fatal("busy inactive tab must refuse close before prompting")
	}
	first.Busy = false
	u.notice("Test", "Do not click through")
	u.app.ForceDraw()
	clickTabHeader(u, x)
	if u.dialogContent != nil || u.active() != second || len(u.tabs) != 2 || !second.Editor.Area.HasFocus() {
		t.Fatal("outside click should dismiss the old modal without reaching the tab or stealing focus")
	}
	u.app.ForceDraw()
	clickTabHeader(u, x)
	u.app.ForceDraw()
	if u.dialogContent == nil || u.active() != second || len(u.tabs) != 2 || !strings.Contains(screenText(s), "Discard unsaved SQL in this tab?") {
		t.Fatal("dirty inactive tab must prompt without being activated")
	}
	dispatch(u, tcell.KeyEscape, 0)
	if u.dialogContent != nil || u.active() != second || len(u.tabs) != 2 || first.Editor.Area.GetText() != "SELECT 1;" {
		t.Fatal("cancel changed the tabs or discarded SQL")
	}
	u.app.ForceDraw()
	clickTabHeader(u, x)
	dispatch(u, tcell.KeyTab, 0)
	dispatch(u, tcell.KeyEnter, 0)
	if u.dialogContent != nil || u.active() != second || !second.Editor.Area.HasFocus() || len(u.tabs) != 1 {
		t.Fatal("confirm did not close only the inactive target")
	}
}

func TestInteractiveTabHeaderOverflow(t *testing.T) {
	u, s := modeTestUI(t)
	for i := 0; i < 7; i++ {
		u.active().Path = "inactive-long-name.sql"
		u.newTab(nil)
	}
	last := u.active()
	last.Path = strings.Repeat("界", 30) + ".sql"
	s.SetSize(30, 24)
	u.app.ForceDraw()
	row := strings.SplitN(screenText(s), "\n", 2)[0]
	if !strings.HasPrefix(row, " 8:") || !strings.Contains(row, "… ×") || !strings.Contains(row, "SQLTerm") {
		t.Fatalf("active tab and connection should share the narrow row: %q", row)
	}
	clickTabHeader(u, 29)
	if u.active() != last || len(u.tabs) != 8 {
		t.Fatal("clipped context acted as a tab")
	}
	clickTabHeader(u, firstHeaderClose(t, s))
	if len(u.tabs) != 7 || u.active().Path != "inactive-long-name.sql" {
		t.Fatal("visible close control did not close the active tab")
	}
}

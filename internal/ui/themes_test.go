package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"os"
	"path/filepath"
	"sqlterm/internal/config"
	"sqlterm/internal/result"
	"strings"
	"testing"
)

func TestThemePreviewCancelAndSavePreserveWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	u := New(path, config.Config{Profiles: []config.Profile{{Name: "db", Folder: "DEV", Engine: "postgres", Host: "localhost", User: "u", CredentialID: "secret-id"}}})
	defer u.cancel()
	originalDefaults := tview.Styles
	tab := u.active()
	tab.Editor.SetText("SELECT 1;")
	tab.Editor.Area.Select(7, 7)
	store, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Append([]result.Cell{{Null: true}})
	tab.Results = []*result.Store{store}
	u.showResults()
	u.chooseTheme()
	list := currentThemeList(u)
	list.SetCurrentItem(4)
	if u.palette.ID != "light" {
		t.Fatal("no live preview")
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("preview wrote settings")
	}
	u.pages.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if u.palette.ID != "tokyo-night" || u.cfg.Theme != "" {
		t.Fatal("cancel did not restore original theme")
	}
	u.chooseTheme()
	list = currentThemeList(u)
	list.SetCurrentItem(2)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	loaded, err := config.Load(path)
	if err != nil || loaded.Theme != "nord" || loaded.Profiles[0].CredentialID != "secret-id" {
		t.Fatal("theme persistence lost settings", err)
	}
	if tab.Editor.Area.GetText() != "SELECT 1;" || tab.Editor.Cursor() != 7 || tab.Results[0] != store {
		t.Fatal("theme change changed SQL, cursor or result set")
	}
	fg, bg, _ := tab.Editor.Area.GetTextStyle().Decompose()
	if fg != u.palette.Foreground || bg != u.palette.Background {
		t.Fatal("editor did not change colors")
	}
	headingFG, _, _ := u.table.GetCell(0, 0).Style.Decompose()
	nullFG, _, _ := u.table.GetCell(1, 0).Style.Decompose()
	if headingFG != u.palette.Accent || nullFG != u.palette.Muted {
		t.Fatal("results did not change colors")
	}
	u.newTab(nil)
	fg, bg, _ = u.active().Editor.Area.GetTextStyle().Decompose()
	if fg != u.palette.Foreground || bg != u.palette.Background {
		t.Fatal("new tabs lost selected theme")
	}
	if tview.Styles != originalDefaults {
		t.Fatal("changed process-global widget styles")
	}
}
func currentThemeList(u *UI) *tview.List {
	_, page := u.pages.GetFrontPage()
	return page.(*centered).child.(*tview.List)
}
func TestThemeSaveFailureCanBeCanceled(t *testing.T) {
	block := filepath.Join(t.TempDir(), "file")
	os.WriteFile(block, []byte("block"), 0600)
	u := New(filepath.Join(block, "config.json"), config.Config{Theme: "nord"})
	defer u.cancel()
	u.chooseTheme()
	l := currentThemeList(u)
	l.SetCurrentItem(4)
	l.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if u.cfg.Theme != "nord" {
		t.Fatal("failed write changed saved choice")
	}
	u.dismiss()
	if u.palette.ID != "nord" {
		t.Fatal("failed preview not restored")
	}
}
func TestAllThemesRenderConnectionsAndMaskedPassword(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 35)
	u.pages.SetRect(0, 0, 120, 35)
	for _, p := range palettes {
		u.applyTheme(p.ID)
		u.chooseConnection()
		u.pages.Draw(s)
		s.Show()
		_, _, style, _ := s.GetContent(119, 34)
		_, bg, _ := style.Decompose()
		if bg != p.Background {
			t.Fatalf("%s has an unthemed background", p.ID)
		}
		x, y, _, _ := u.search.GetRect()
		_, _, style, _ = s.GetContent(x, y)
		_, bg, _ = style.Decompose()
		if bg != p.Background {
			t.Fatalf("%s has an unthemed input label", p.ID)
		}
		u.dismiss()
		u.profileForm(-1)
		f := currentForm(u)
		f.GetFormItem(6).(*tview.InputField).SetText("test-secret-must-stay-hidden")
		u.pages.Draw(s)
		s.Show()
		if text := screenText(s); len(text) == 0 || strings.Contains(text, "test-secret-must-stay-hidden") {
			t.Fatal("empty screen or exposed password")
		}
		if f.GetBackgroundColor() != p.Background {
			t.Fatal("form not themed")
		}
		u.dismiss()
	}
}
func TestThemeListTextUsesPaletteBackground(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 30)
	u.pages.SetRect(0, 0, 100, 30)
	u.chooseTheme()
	l := currentThemeList(u)
	l.SetCurrentItem(4)
	u.pages.Draw(s)
	s.Show()
	x, y, _, _ := l.GetInnerRect()
	_, _, style, _ := s.GetContent(x, y)
	fg, bg, _ := style.Decompose()
	if fg != u.palette.Foreground || bg != u.palette.Background {
		t.Fatal("unselected theme text does not match palette")
	}
	u.dismiss()
}

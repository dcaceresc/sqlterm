package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestSQLSyntaxColors(t *testing.T) {
	for _, rule := range []struct{ name, sql, token, color string }{
		{"keywords", "select name FROM customers WHERE id IS NULL", "select", "#7aa2f7"},
		{"strings", "SELECT 'Hola ''mundo'''", "'Hola", "#9ece6a"},
		{"numbers", "SELECT 42, 3.14, .5, 1e-3, 0xFF", "42", "#ff9e64"},
	} {
		t.Run(rule.name, func(t *testing.T) {
			u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
			defer u.cancel()
			e := u.active().Editor
			e.SetText(rule.sql)
			e.Area.Select(0, 0)
			s := tcell.NewSimulationScreen("UTF-8")
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			defer s.Fini()
			s.SetSize(120, 35)
			u.pages.SetRect(0, 0, 120, 35)
			u.pages.Draw(s)
			x, y, _, _ := e.Area.GetInnerRect()
			start := strings.Index(rule.sql, rule.token)
			for i := 0; i < len(rule.token); i++ {
				_, _, style, _ := s.GetContent(x+start+i, y)
				fg, _, _ := style.Decompose()
				if fg != tcell.GetColor(rule.color) {
					t.Fatalf("%s cell %d: color %v, want %s", rule.name, i, fg, rule.color)
				}
			}
			if e.Area.GetText() != rule.sql {
				t.Fatal("highlighting changed SQL")
			}
		})
	}
}

func TestSyntaxColorsFollowThemesAndNewTabs(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 35)
	u.pages.SetRect(0, 0, 120, 35)
	for _, theme := range palettes {
		u.applyTheme(theme.ID)
		for _, newTab := range []bool{false, true} {
			if newTab {
				u.newTab(nil)
			}
			e := u.active().Editor
			e.SetText("SELECT 'text', 42;")
			e.Area.Select(0, 0)
			u.pages.Draw(s)
			x, y, _, _ := e.Area.GetInnerRect()
			for _, token := range []struct {
				col   int
				color tcell.Color
			}{{0, theme.Syntax.Keyword}, {7, theme.Syntax.String}, {15, theme.Syntax.Number}} {
				_, _, st, _ := s.GetContent(x+token.col, y)
				fg, bg, _ := st.Decompose()
				if fg != token.color || bg != theme.Background {
					t.Fatalf("%s new=%v col=%d: %v/%v", theme.ID, newTab, token.col, fg, bg)
				}
			}
		}
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestLeaderHelpDescribesHierarchy(t *testing.T) {
	u, _ := modeTestUI(t)
	for _, language := range []string{"en", "es"} {
		u.cfg.Language = language
		text := u.legacyHelp()
		if language == "es" {
			text = u.spanishHelp()["All shortcuts"]
		}
		if strings.Contains(text, "Action keys in the palette:") || strings.Contains(text, "Dentro de la paleta,") {
			t.Errorf("%s help advertises retired flat mappings", language)
		}
	}
}

func TestLeaderSpanishLabels(t *testing.T) {
	u, screen := modeTestUI(t)
	u.cfg.Language = "es"
	screen.SetSize(80, 24)
	u.leaderMenu()
	u.app.ForceDraw()
	for _, label := range []string{"Archivos", "Pestañas", "Código", "Explorar", "Preferencias", "Distribución"} {
		if !strings.Contains(screenText(screen), label) {
			t.Errorf("missing translated group %q", label)
		}
	}
	capture := u.dialogContent.(*tview.TextView).GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'w', 0))
	u.app.ForceDraw()
	if !strings.Contains(screenText(screen), "Maximizar editor") {
		t.Fatal("layout actions must be translated")
	}
}

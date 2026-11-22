package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

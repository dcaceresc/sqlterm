package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestPlainWidgetDoesNotEmulateVim(t *testing.T) {
	e := New()
	for _, r := range "ivg" {
		e.Area.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, 0), func(tview.Primitive) {})
	}
	if got := e.Area.GetText(); got != "ivg" || e.Mode != "NORMAL" {
		t.Fatalf("plain widget: text=%q mode=%s", got, e.Mode)
	}
}

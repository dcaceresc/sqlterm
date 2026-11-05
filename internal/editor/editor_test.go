package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"testing"
)

func send(e *Editor, k *tcell.EventKey) {
	if out := e.Handle(k); out != nil {
		e.Area.InputHandler()(out, func(tview.Primitive) {})
	}
}
func keys(e *Editor, s string) {
	for _, r := range s {
		send(e, tcell.NewEventKey(tcell.KeyRune, r, 0))
	}
}
func TestVimEditingUnicodeAndUndo(t *testing.T) {
	e := New()
	e.SetText("SELECT ñ;\nSELECT 2;")
	keys(e, "wi")
	keys(e, "big_")
	send(e, tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if e.Area.GetText() != "SELECT big_ñ;\nSELECT 2;" {
		t.Fatal(e.Area.GetText())
	}
	keys(e, "u")
	send(e, tcell.NewEventKey(tcell.KeyCtrlR, 0, 0))
	if e.Area.GetText() != "SELECT big_ñ;\nSELECT 2;" {
		t.Fatal("redo failed")
	}
	keys(e, "ggwvl")
	selected, _, _ := e.Area.GetSelection()
	if selected != "b" {
		t.Fatalf("visual selected %q", selected)
	}
	keys(e, "d")
	if e.Mode != "NORMAL" || e.Clipboard != "b" {
		t.Fatal("visual cut failed")
	}
	keys(e, "p")
	if e.Area.GetText() != "SELECT big_ñ;\nSELECT 2;" {
		t.Fatal("paste failed")
	}
}
func TestUnicodeMovementAndPrefix(t *testing.T) {
	e := New()
	e.SetText("SELECT corazón")
	e.Area.Select(len(e.Area.GetText()), len(e.Area.GetText()))
	prefix, start := e.Prefix()
	if prefix != "corazón" || start != 7 {
		t.Fatal(prefix, start)
	}
	keys(e, "hhhh")
	p := e.Cursor()
	if string(e.Area.GetText()[p:]) != "azón" {
		t.Fatal(e.Area.GetText()[p:])
	}
}
func TestPrefixIncludesUnclosedIdentifierQuote(t *testing.T) {
	for _, sql := range []string{"SELECT dbo.[cust", "SELECT public.\"cust", "SELECT app.`cust"} {
		e := New()
		e.SetText(sql)
		e.Area.Select(len(sql), len(sql))
		prefix, start := e.Prefix()
		if prefix != "cust" || start != len(sql)-5 {
			t.Fatal(sql, prefix, start)
		}
	}
}

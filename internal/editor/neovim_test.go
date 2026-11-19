package editor

import (
	"os/exec"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/nvim"
)

func testNativeEditor(t *testing.T) *Editor {
	t.Helper()
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("Neovim is not installed")
	}
	e := New()
	if err := e.EnableNeovim(nvim.Options{Dir: t.TempDir(), Width: 60, Height: 12}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
	})
	return e
}

func TestNeovimEditorUsesNativeModesSelectionAndPaste(t *testing.T) {
	e := testNativeEditor(t)
	const original = "select ñ;\nselect 2;"
	e.SetText(original)
	nativeKeys(e, "GggVG")
	if selected, _, _ := e.Area.GetSelection(); selected != original || e.Mode != "VISUAL" {
		t.Fatalf("native ggVG selected=%q mode=%s err=%v", selected, e.Mode, e.Err())
	}
	e.Area.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	e.SetText("")
	e.Area.PasteHandler()("i:quit ggVG\n\t界🙂", func(tview.Primitive) {})
	if e.Area.GetText() != "i:quit ggVG\n\t界🙂" || e.Mode != "NORMAL" || e.Err() != nil {
		t.Fatalf("paste text=%q mode=%s err=%v", e.Area.GetText(), e.Mode, e.Err())
	}
	nativeKeys(e, "u")
	if e.Area.GetText() != "" {
		t.Fatalf("native paste must undo in one step: %q", e.Area.GetText())
	}
}

func TestNeovimEditorHostEditsUseAuthoritativeBuffer(t *testing.T) {
	e := testNativeEditor(t)
	e.SetText("SELECT name;")
	e.Area.Replace(7, 11, "ñ")
	if e.Area.GetText() != "SELECT ñ;" || e.Err() != nil {
		t.Fatalf("replace text=%q err=%v", e.Area.GetText(), e.Err())
	}
	e.Area.Select(0, len("SELECT"))
	if selected, _, _ := e.Area.GetSelection(); selected != "SELECT" {
		t.Fatalf("host selection=%q err=%v", selected, e.Err())
	}
	nativeKeys(e, "y")
	if e.Mode != "NORMAL" {
		t.Fatalf("native yank mode=%s", e.Mode)
	}
}

func TestNeovimEditorErrorsNeverFallBackToLegacy(t *testing.T) {
	e := New()
	if err := e.EnableNeovim(nvim.Options{Binary: "sqlterm-nvim-does-not-exist", Width: 60, Height: 12}); err == nil {
		t.Fatal("missing Neovim must be reported")
	}
	nativeKeys(e, "iSELECT")
	if e.Area.GetText() != "" || e.Err() == nil {
		t.Fatal("failed native startup must not fall back to emulated editing")
	}
}

func nativeKeys(e *Editor, sequence string) {
	for _, r := range sequence {
		e.Area.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, 0), func(tview.Primitive) {})
	}
}

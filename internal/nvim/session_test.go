package nvim

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMissingBinary(t *testing.T) {
	s, err := Start(Options{Binary: filepath.Join(t.TempDir(), "missing-nvim"), Width: 40, Height: 10})
	if err == nil {
		s.Close()
		t.Fatal("missing executable accepted")
	}
}

func TestChildExit(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("Neovim is not installed")
	}
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	s, err := Start(Options{Width: 40, Height: 10, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := s.Input(":qa!<CR>"); err != nil && !errors.Is(err, ErrExited) {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("child exit not observed")
	}
	if _, err := s.Snapshot(); !errors.Is(err, ErrExited) {
		t.Fatalf("snapshot after child exit: %v", err)
	}
}

func TestEmbeddedNeovim(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	binary, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("Neovim is not installed")
	}
	// El hijo usa un directorio seguro del repositorio; XDG solo apunta al scratch de la prueba.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", scratch)
	t.Setenv("XDG_DATA_HOME", scratch)
	t.Setenv("XDG_STATE_HOME", scratch)
	t.Setenv("XDG_CACHE_HOME", scratch)
	t.Setenv("NVIM_APPNAME", "nvim")
	if err := os.Mkdir(filepath.Join(scratch, "nvim"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "nvim", "init.lua"), []byte(`vim.opt.number = true; vim.opt.tabstop = 31`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIMINIT", "set tabstop=32")
	s, err := Start(Options{Binary: binary, Dir: cwd, Width: 44, Height: 10, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() Snapshot { t.Helper(); p, err := s.Snapshot(); must(err); return p }
	check := func(text, mode string, cursor int) {
		t.Helper()
		p := snapshot()
		if p.Text != text || p.Mode != mode || p.Cursor != cursor {
			t.Fatalf("snapshot = %+v; want %q %q %d", p, text, mode, cursor)
		}
	}
	selected := func(want string, kind SelectionKind) {
		t.Helper()
		p := snapshot()
		got, ok, err := p.SelectedText()
		must(err)
		if !ok || got != want || p.Selection.Kind != kind {
			t.Fatalf("selection = %q, %v (%+v); want %q %v", got, ok, p, want, kind)
		}
	}
	initial := s.Frame()
	if initial.Width != 44 || initial.Height != 10 || initial.Sequence == 0 {
		t.Fatalf("initial frame: %+v", initial)
	}
	var isolation []bool
	must(s.v.ExecLua(`return { not vim.o.swapfile, vim.o.shada == '', not vim.o.undofile, not vim.o.number, vim.o.tabstop == 8 }`, &isolation))
	for _, ok := range isolation {
		if !ok {
			t.Fatalf("isolation: %v", isolation)
		}
	}
	const original = "áβ\nselect 2;\n"
	must(s.SetText(original))
	check(original, "n", 0)
	must(s.Input("G"))
	must(s.Input("g"))
	if _, err := s.Snapshot(); !errors.Is(err, ErrInputPending) {
		t.Fatalf("pending g snapshot: %v", err)
	}
	must(s.Input("g"))
	check(original, "n", 0)
	must(s.Input("V"))
	selected("áβ\n", Linewise)
	must(s.Input("<Esc>ggVG"))
	selected(original, Linewise)
	must(s.Input("<Esc>ggv"))
	selected("á", Charwise)
	must(s.Input("<Esc><C-v>"))
	_, _, err = snapshot().SelectedText()
	if !errors.Is(err, ErrBlockSelection) {
		t.Fatalf("block selection: %v", err)
	}
	must(s.Input("<Esc>ggd"))
	if p := snapshot(); !strings.HasPrefix(p.Mode, "no") {
		t.Fatalf("pending operator: %+v", p)
	}
	must(s.Input("w"))
	check("\nselect 2;\n", "n", 0)
	must(s.Input("u<Esc><Esc>"))
	check(original, "n", 0)
	must(s.Input("iX"))
	check("X"+original, "i", 1)
	must(s.Input("<Esc>u"))
	check(original, "n", 0)
	must(s.Select(2, len("áβ\ns")))
	selected("β\ns", Charwise)
	must(s.Replace(2, len("áβ\ns"), "🌍\n"))
	check("á🌍\nelect 2;\n", "n", 2)
	must(s.SetText("x\n"))
	must(s.SetCursor(len("x\n")))
	check("x\n", "n", len("x\n"))
	if err := s.Select(1, 2); !errors.Is(err, ErrUnsupportedSelection) {
		t.Fatalf("newline-only visual range: %v", err)
	}
	check("x\n", "n", 1)
	must(s.SetText("a\u0301b"))
	must(s.Input("v"))
	selected("a\u0301", Charwise)
	if err := s.Select(0, 1); !errors.Is(err, ErrUnsupportedSelection) {
		t.Fatalf("partial composed glyph: %v", err)
	}
	check("a\u0301b", "n", 0)
	must(s.SetText(""))
	must(s.Paste("<Esc> α\n🌍\n"))
	if p := snapshot(); p.Text != "<Esc> α\n🌍\n" || p.Mode != "n" {
		t.Fatalf("normal literal paste: %+v", p)
	}
	must(s.SetText("αβ"))
	must(s.Input("v"))
	must(s.Paste("🌍\nα"))
	if p := snapshot(); p.Text != "🌍\nαβ" || p.Mode != "n" {
		t.Fatalf("visual literal paste: %+v", p)
	}
	beforeResize := s.Frame().Sequence
	must(s.Resize(52, 12))
	if f := s.Frame(); f.Width != 52 || f.Height != 12 || f.Sequence <= beforeResize {
		t.Fatalf("resize: %+v", f)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("process not reaped")
	}
	if err := s.Input("j"); !errors.Is(err, ErrClosed) {
		t.Fatalf("input after close: %v", err)
	}
}

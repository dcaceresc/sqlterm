package ui

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestCellInspectorLineAndSearchCopy(t *testing.T) {
	for _, tc := range []struct{ name, keys, want string }{
		{"line", "V", "first\n"},
		{"search", "/β", "β"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
			defer u.cancel()
			copied := make(chan string, 1)
			u.writeClipboard = func(text string) error { copied <- text; return nil }
			u.inspectCellText("Cell", "name", func(context.Context, io.Writer, bool) error { return nil }, "first\nαβγ\n")
			dispatchSelector(u, tcell.KeyRune, 'i')
			dispatchSelector(u, tcell.KeyRune, 'x')
			for _, key := range tc.keys {
				dispatchSelector(u, tcell.KeyRune, key)
			}
			if tc.name == "search" {
				dispatchSelector(u, tcell.KeyEnter, 0)
			}
			dispatchSelector(u, tcell.KeyCtrlC, 0)
			if text := receiveCopy(t, copied); text != tc.want {
				t.Fatalf("copied %q, want %q", text, tc.want)
			}
		})
	}
}

func TestCellInspectorVisualCopy(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	copied := make(chan string, 1)
	u.writeClipboard = func(text string) error { copied <- text; return nil }
	u.inspectCellText("Cell", "name", func(context.Context, io.Writer, bool) error { return nil }, "αβγ\nsecond\n")
	dispatchSelector(u, tcell.KeyCtrlC, 0)
	if u.dialogContent == nil {
		t.Fatal("copy with no selection closed the inspector")
	}
	select {
	case text := <-copied:
		t.Fatalf("unexpected copy: %q", text)
	default:
	}
	dispatchSelector(u, tcell.KeyRune, 'v')
	dispatchSelector(u, tcell.KeyRight, 0)
	dispatchSelector(u, tcell.KeyCtrlC, 0)
	if text := receiveCopy(t, copied); text != "αβ" {
		t.Fatalf("visual copied %q", text)
	}
	if u.dialogContent == nil {
		t.Fatal("copy closed the inspector")
	}
}

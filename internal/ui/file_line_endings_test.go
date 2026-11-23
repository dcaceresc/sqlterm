package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func TestNeovimOpensAndSavesSQLLineEndings(t *testing.T) {
	for _, tc := range []struct{ name, newline string }{{"CRLF", "\r\n"}, {"LF", "\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "query.sql")
			original := "SELECT 1;" + tc.newline + "SELECT 2;" + tc.newline
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			u := runningNeovimUI(t)
			u.app.QueueUpdateDraw(func() {
				fileDialogKey(u, tcell.KeyCtrlO)
				picker := u.dialogContent.(*sqlFileDialog)
				picker.navigate(dir)
				picker.list.SetCurrentItem(1)
				fileDialogKey(u, tcell.KeyEnter)
				tab := u.active()
				const expected = "SELECT 1;\nSELECT 2;\n"
				if got := tab.Editor.Area.GetText(); got != expected || tab.Saved != expected || tab.Path != path {
					t.Errorf("open: text=%q saved=%q path=%q", got, tab.Saved, tab.Path)
				}
				if got, err := os.ReadFile(path); err != nil || string(got) != original {
					t.Errorf("open modified the file: %q, %v", got, err)
				}
				if err := tab.Editor.SetText("SELECT 3;\nSELECT 4;\n"); err != nil {
					t.Error(err)
					return
				}
				fileDialogKey(u, tcell.KeyCtrlS)
			})
			want := "SELECT 3;" + tc.newline + "SELECT 4;" + tc.newline
			if got, err := os.ReadFile(path); err != nil || string(got) != want {
				t.Fatalf("save: %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestReopenPreservesCRLFOnSave(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	path := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(path, []byte("SELECT 1;\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	u := New(configPath, config.Config{})
	fileDialogKey(u, tcell.KeyCtrlO)
	picker := u.dialogContent.(*sqlFileDialog)
	picker.name.SetText(path)
	picker.submit()
	_ = u.active().Editor.SetText("SELECT 2;\n")
	quitTestWorkspace(t, u)
	reopened := New(configPath, config.Config{})
	defer reopened.cancel()
	fileDialogKey(reopened, tcell.KeyCtrlS)
	if got, err := os.ReadFile(path); err != nil || string(got) != "SELECT 2;\r\n" {
		t.Fatalf("save after reopening: %q, %v", got, err)
	}
}

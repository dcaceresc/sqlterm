package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
)

func runningNeovimUI(t *testing.T) *UI {
	t.Helper()
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("Neovim is not installed")
	}
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		u.nativeEditors = true
		if err := u.activateNeovim(u.active()); err != nil {
			t.Error(err)
		}
		u.app.SetFocus(u.active().Editor.Area)
	})
	return u
}

func TestNeovimUIStartupWelcomeKeepsSQLEmpty(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		if u.active().Status != "Welcome to SQLTerm" || e.Area.GetText() != "" || u.active().Saved != "" || u.active().Path != "" || e.Mode != "NORMAL" {
			t.Errorf("startup: status=%q text=%q saved=%q path=%q mode=%s", u.active().Status, e.Area.GetText(), u.active().Saved, u.active().Path, e.Mode)
		}
		u.newTab(nil)
		if u.active().Status != "Disconnected" {
			t.Errorf("new tab must not repeat startup welcome: %q", u.active().Status)
		}
	})
}

func TestNeovimUISpaceLeaderAndRemovedBindings(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		const text = "SELECT 1;"
		if err := e.SetText(text); err != nil {
			t.Error(err)
			return
		}
		dispatchModified(u, tcell.KeyRune, ' ', 0)
		if u.dialogContent == nil || e.Cursor() != 0 || e.Mode != "NORMAL" || e.Area.GetText() != text {
			t.Errorf("Space must open actions without editing: dialog=%T cursor=%d mode=%s text=%q", u.dialogContent, e.Cursor(), e.Mode, e.Area.GetText())
			return
		}
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		dispatchModified(u, tcell.KeyCtrlP, 0, 0)
		if u.dialogContent != nil {
			t.Error("Ctrl+P must not open actions")
		}
	})
}

func TestNeovimUISpacePreservesInsertAndSearch(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		dispatchModified(u, tcell.KeyRune, 'i', 0)
		dispatchModified(u, tcell.KeyRune, ' ', 0)
		if u.dialogContent != nil || e.Mode != "INSERT" || e.Area.GetText() != " " {
			t.Errorf("INSERT Space: dialog=%T mode=%s text=%q", u.dialogContent, e.Mode, e.Area.GetText())
		}
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		u.app.SetFocus(u.search)
		u.search.SetText("")
		dispatchModified(u, tcell.KeyRune, ' ', 0)
		if u.dialogContent != nil || u.search.GetText() != " " {
			t.Errorf("search Space: dialog=%T text=%q", u.dialogContent, u.search.GetText())
		}
	})
}

func TestNeovimUICommentShortcutConsumesWindowsKeyEncodings(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	for _, tt := range []struct {
		name string
		key  tcell.Key
		r    rune
		sql  string
	}{
		{"Windows control character", tcell.KeyGS, 29, "SELECT 1;\nSELECT 2;"},
		{"Windows modified bracket", tcell.KeyRune, ']', "SELECT 1;\nSELECT 2;"},
		{"Modified brace", tcell.KeyRune, '}', "SELECT 1;\nSELECT 2;"},
		{"VT control key", tcell.KeyCtrlRightSq, 0, "SELECT 1;\nSELECT 2;"},
		{"UTF-8 last line without newline", tcell.KeyRune, ']', "SELECT 'ñ' AS café"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u := runningNeovimUI(t)
			u.app.QueueUpdateDraw(func() {
				e := u.active().Editor
				sql := tt.sql
				if err := e.SetText(sql); err != nil {
					t.Error(err)
					return
				}
				dispatchModified(u, tcell.KeyRune, 'V', 0)
				dispatchModified(u, tt.key, tt.r, tcell.ModCtrl)
				if got := e.Area.GetText(); got != "-- "+sql || e.Mode != "VISUAL" || e.Err() != nil {
					t.Errorf("Ctrl+}: text=%q mode=%s err=%v", got, e.Mode, e.Err())
					return
				}
				dispatchModified(u, tt.key, tt.r, tcell.ModCtrl)
				if got := e.Area.GetText(); got != sql || e.Err() != nil || u.dialogContent != nil {
					t.Errorf("uncomment: text=%q err=%v dialog=%T", got, e.Err(), u.dialogContent)
				}
			})
		})
	}
}

func TestNeovimUICompletionKeepsInsertMode(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
		t.Run(tcell.KeyNames[key], func(t *testing.T) {
			u := runningNeovimUI(t)
			u.app.QueueUpdateDraw(func() {
				e := u.active().Editor
				for _, r := range "iselect" {
					dispatchModified(u, tcell.KeyRune, r, 0)
				}
				if u.suggestions == nil || e.Mode != "INSERT" {
					t.Errorf("expected INSERT with SQL suggestions: mode=%s err=%v", e.Mode, e.Err())
					return
				}
				dispatchModified(u, key, 0, 0)
				if e.Mode != "INSERT" || e.Area.GetText() != "SELECT" || u.suggestions != nil || e.Err() != nil {
					t.Errorf("completion: mode=%s text=%q suggestions=%t err=%v", e.Mode, e.Area.GetText(), u.suggestions != nil, e.Err())
				}
				for _, r := range " 1" {
					dispatchModified(u, tcell.KeyRune, r, 0)
				}
				if e.Area.GetText() != "SELECT 1" || e.Mode != "INSERT" {
					t.Errorf("continued typing: mode=%s text=%q", e.Mode, e.Area.GetText())
				}
				dispatchModified(u, tcell.KeyEscape, 0, 0)
				if e.Mode != "NORMAL" || e.Area.GetText() != "SELECT 1" {
					t.Error("Escape must still return to NORMAL without changing SQL")
				}
			})
		})
	}
}

func TestNeovimUIBracketedPasteIsLiteralAndKeepsNormal(t *testing.T) {
	u := runningNeovimUI(t)
	const text = "i:quit ggVG\n\t界🙂 SELECT 1;"
	dispatched := make(chan struct{})
	u.app.QueueUpdateDraw(func() {
		u.active().Editor.SetText("")
		u.app.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
			if key.Key() == tcell.KeyF12 {
				close(dispatched)
				return nil
			}
			return u.key(key)
		})
	})
	u.app.QueueEvent(tcell.NewEventPaste(true))
	for _, r := range text {
		key := tcell.KeyRune
		if r == '\n' {
			key, r = tcell.KeyEnter, 0
		}
		if r == '\t' {
			key, r = tcell.KeyTab, 0
		}
		u.app.QueueEvent(tcell.NewEventKey(key, r, 0))
	}
	u.app.QueueEvent(tcell.NewEventPaste(false))
	u.app.QueueEvent(tcell.NewEventKey(tcell.KeyF12, 0, 0))
	select {
	case <-dispatched:
	case <-time.After(5 * time.Second):
		t.Fatal("paste was not dispatched")
	}
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		if e.Area.GetText() != text || e.Mode != "NORMAL" || e.Err() != nil {
			t.Errorf("paste text=%q mode=%s err=%v", e.Area.GetText(), e.Mode, e.Err())
		}
		dispatchModified(u, tcell.KeyRune, 'u', 0)
		if e.Area.GetText() != "" {
			t.Errorf("paste must undo in one step: %q", e.Area.GetText())
		}
	})
}

func TestNeovimUISaveSelectionAndTabIsolation(t *testing.T) {
	u := runningNeovimUI(t)
	path := filepath.Join(t.TempDir(), "native.sql")
	u.app.QueueUpdateDraw(func() {
		first := u.active()
		first.Editor.SetText("SELECT ñ;\nSELECT 2;")
		for _, r := range "ggVG" {
			dispatchModified(u, tcell.KeyRune, r, 0)
		}
		if selected, _, _ := first.Editor.Area.GetSelection(); selected != "SELECT ñ;\nSELECT 2;" {
			t.Errorf("native selection=%q err=%v", selected, first.Editor.Err())
		}
		first.Path = path
		u.saveFile()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "SELECT ñ;\nSELECT 2;" || first.Saved != string(data) {
			t.Errorf("saved=%q err=%v", data, err)
		}
		u.newTab(nil)
		if u.active() == first || !u.active().Editor.IsNeovim() || u.active().Editor.Area.GetText() != "" {
			t.Error("new SQL tab must have its own Neovim session and empty buffer")
		}
		u.active().Editor.SetText("SELECT 'other';")
		if first.Editor.Area.GetText() != string(data) {
			t.Error("new tab changed first buffer")
		}
	})
}

func TestNeovimUIBlocksStaleOrBlockwiseExecutionAndSave(t *testing.T) {
	u := runningNeovimUI(t)
	path := filepath.Join(t.TempDir(), "keep.sql")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Path = path
		tab.Editor.SetText("SELECT 1;")
		dispatchModified(u, tcell.KeyCtrlV, 0, 0)
		u.execute()
		if !strings.Contains(tab.Status, "blockwise") {
			t.Errorf("blockwise execution status=%q", tab.Status)
		}
		u.saveFile()
		data, _ := os.ReadFile(path)
		if string(data) != "original" {
			t.Error("unsupported selection must not overwrite SQL")
		}
		dispatchModified(u, tcell.KeyEscape, 0, 0)
		if err := tab.Editor.Close(); err != nil {
			t.Error(err)
		}
		u.execute()
		if !strings.Contains(tab.Status, "Neovim:") {
			t.Errorf("closed editor execution status=%q", tab.Status)
		}
		u.saveFile()
		data, _ = os.ReadFile(path)
		if string(data) != "original" {
			t.Error("dead Neovim must not overwrite SQL from cached text")
		}
	})
}

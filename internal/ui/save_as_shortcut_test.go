package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func dispatchModified(u *UI, key tcell.Key, r rune, modifiers tcell.ModMask) {
	event := u.key(tcell.NewEventKey(key, r, modifiers))
	if event != nil {
		u.app.GetFocus().InputHandler()(event, func(p tview.Primitive) { u.app.SetFocus(p) })
	}
}

func TestCtrlShiftSSaveAsShortcutOpensAndCancelsWithoutSaving(t *testing.T) {
	variants := []struct {
		name      string
		key       tcell.Key
		r         rune
		modifiers tcell.ModMask
	}{
		{"CtrlS key with Shift", tcell.KeyCtrlS, 0, tcell.ModShift},
		{"lowercase rune with CtrlShift", tcell.KeyRune, 's', tcell.ModCtrl | tcell.ModShift},
		{"uppercase rune with CtrlShift", tcell.KeyRune, 'S', tcell.ModCtrl | tcell.ModShift},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			currentDir := t.TempDir()
			rememberedDir := t.TempDir()
			path := filepath.Join(currentDir, "active.sql")
			original := "select 'saved'"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}

			u, _ := modeTestUI(t)
			tab := u.active()
			tab.Path = path
			tab.Saved = original
			tab.Editor.SetText("select 'dirty'")
			u.lastSavedSQLDir = rememberedDir
			u.app.SetFocus(tab.Editor.Area)

			dispatchModified(u, variant.key, variant.r, variant.modifiers)
			p, ok := u.dialogContent.(*sqlFileDialog)
			if !ok {
				t.Fatalf("Ctrl+Shift+S opened %T, want Save SQL as dialog", u.dialogContent)
			}
			if !p.save || p.dir != rememberedDir || p.name.GetText() != "active.sql" {
				t.Errorf("Ctrl+Shift+S Save As state: save=%v dir=%q name=%q", p.save, p.dir, p.name.GetText())
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != original {
				t.Fatalf("Ctrl+Shift+S saved before confirmation: contents=%q err=%v", contents, err)
			}

			dispatchModified(u, variant.key, variant.r, variant.modifiers)
			if u.dialogContent != p {
				t.Errorf("Ctrl+Shift+S in modal replaced the existing dialog: got %T", u.dialogContent)
			}

			dispatch(u, tcell.KeyEscape, 0)
			contents, err = os.ReadFile(path)
			if err != nil || string(contents) != original {
				t.Errorf("cancel changed file: contents=%q err=%v", contents, err)
			}
			if u.dialogContent != nil || tab.Path != path || tab.Saved != original || tab.Editor.Area.GetText() != "select 'dirty'" {
				t.Errorf("cancel changed editor state: dialog=%T path=%q saved=%q editor=%q", u.dialogContent, tab.Path, tab.Saved, tab.Editor.Area.GetText())
			}
		})
	}
}

func TestSaveOpenAndF12ShortcutsRemainDistinct(t *testing.T) {
	dir := t.TempDir()
	activePath := filepath.Join(dir, "active.sql")
	otherPath := filepath.Join(dir, "other.sql")
	if err := os.WriteFile(activePath, []byte("old active"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte("other unchanged"), 0600); err != nil {
		t.Fatal(err)
	}

	u, _ := modeTestUI(t)
	tab := u.active()
	tab.Path = activePath
	tab.Saved = "old active"
	tab.Editor.SetText("new active")
	dispatch(u, tcell.KeyCtrlS, 0)

	activeContents, activeErr := os.ReadFile(activePath)
	otherContents, otherErr := os.ReadFile(otherPath)
	if activeErr != nil || string(activeContents) != "new active" || tab.Saved != "new active" {
		t.Errorf("plain Ctrl+S direct save: contents=%q err=%v saved=%q", activeContents, activeErr, tab.Saved)
	}
	if otherErr != nil || string(otherContents) != "other unchanged" {
		t.Errorf("plain Ctrl+S changed another file: contents=%q err=%v", otherContents, otherErr)
	}
	if u.dialogContent != nil {
		t.Fatalf("plain Ctrl+S opened dialog %T for an existing path", u.dialogContent)
	}

	dispatch(u, tcell.KeyF12, 0)
	if u.dialogContent != nil {
		t.Fatalf("unconfigured F12 opened default Save As dialog %T", u.dialogContent)
	}

	dispatch(u, tcell.KeyCtrlO, 0)
	p, ok := u.dialogContent.(*sqlFileDialog)
	if !ok || p.save {
		t.Fatalf("Ctrl+O opened %T with save=%v, want Open SQL dialog", u.dialogContent, ok && p.save)
	}
	if p.dir != dir || p.name.GetText() != "active.sql" {
		t.Errorf("Ctrl+O state: dir=%q name=%q", p.dir, p.name.GetText())
	}
}

func TestConfiguredF12AliasStillRunsApply(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Keys: map[string]string{"apply": "F12"}})
	defer u.cancel()
	u.active().Editor.SetText("select 'dirty'")

	dispatch(u, tcell.KeyF12, 0)

	if u.dialogContent != nil {
		t.Fatalf("configured apply:F12 opened Save As dialog %T", u.dialogContent)
	}
	if u.active().Status != "Connect first" {
		t.Errorf("configured apply:F12 status=%q, want apply action status", u.active().Status)
	}
}

func TestConfiguredPlainCtrlSAliasKeepsPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "active.sql")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Keys: map[string]string{"apply": "Ctrl-S"}})
	defer u.cancel()
	u.active().Path = path
	u.active().Saved = "saved"
	u.active().Editor.SetText("dirty")

	dispatch(u, tcell.KeyCtrlS, 0)

	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "saved" {
		t.Errorf("configured plain Ctrl+S alias saved file: contents=%q err=%v", contents, err)
	}
	if u.active().Status != "Connect first" {
		t.Errorf("configured plain Ctrl+S status=%q, want apply action status", u.active().Status)
	}
}

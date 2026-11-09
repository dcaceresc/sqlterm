package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func TestSQLFileDialogUsesSelectedTheme(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sample.sql"), []byte("select 1"), 0600); err != nil {
		t.Fatal(err)
	}
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 35)
	u.pages.SetRect(0, 0, 120, 35)
	u.active().Path = filepath.Join(dir, "sample.sql")
	for _, theme := range palettes {
		for _, save := range []bool{false, true} {
			u.applyTheme(theme.ID)
			if save {
				u.saveFileAs()
			} else {
				u.openFile()
			}
			p := u.dialogContent.(*sqlFileDialog)
			u.pages.Draw(screen)
			screen.Show()
			// Check rendered colors, not only widget configuration.
			check := func(label string, x, y int, wantFG, wantBG tcell.Color) {
				t.Helper()
				_, _, style, _ := screen.GetContent(x, y)
				fg, bg, _ := style.Decompose()
				if fg != wantFG || bg != wantBG {
					t.Errorf("%s save=%v %s: got %v/%v, want %v/%v", theme.ID, save, label, fg, bg, wantFG, wantBG)
				}
			}
			x, y, _, _ := p.path.GetInnerRect()
			check("folder label", x, y, theme.Accent, theme.Background)
			check("folder value", x+len("Folder: "), y, theme.Foreground, theme.Surface)
			x, y, _, _ = p.list.GetInnerRect()
			check("selected entry", x, y, theme.SelectedText, theme.Selection)
			check("file entry", x, y+1, theme.Foreground, theme.Background)
			x, y, _, _ = p.name.GetInnerRect()
			check("name label", x, y, theme.Accent, theme.Background)
			check("name value", x+len("File name: "), y, theme.Foreground, theme.Surface)
			x, y, _, _ = p.status.GetInnerRect()
			check("navigation hint", x, y, theme.Foreground, theme.Background)
			buttons := p.GetItem(p.GetItemCount() - 1).(*tview.Form)
			buttonX, buttonY, buttonWidth, _ := buttons.GetButton(0).GetInnerRect()
			check("action button text", buttonX+buttonWidth/2, buttonY, theme.Foreground, theme.Surface)
			u.dismiss()
		}
	}
}

func TestSQLFileDialog(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "queries")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sub, "sample.sql")
	if err := os.WriteFile(source, []byte("select 42"), 0600); err != nil {
		t.Fatal(err)
	}
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		u.active().Path = filepath.Join(dir, "original.sql")
		u.openFile()
		p, ok := u.dialogContent.(*sqlFileDialog)
		if !ok {
			t.Error("open must show a terminal file dialog")
			return
		}
		p.navigate(sub)
		p.name.SetText("sample.sql")
		p.submit()
		if u.active().Path != source || u.active().Editor.Area.GetText() != "select 42" || u.active().Saved != "select 42" {
			t.Error("open did not load selected SQL")
		}
		u.active().Editor.SetText("select 43")
		u.saveFileAs()
		p = u.dialogContent.(*sqlFileDialog)
		p.navigate(dir)
		p.name.SetText("copy.sql")
		p.submit()
		b, err := os.ReadFile(filepath.Join(dir, "copy.sql"))
		if err != nil || string(b) != "select 43" {
			t.Errorf("save as: %q %v", b, err)
		}
		u.active().Editor.SetText("select 44")
		u.saveFile()
		b, err = os.ReadFile(filepath.Join(dir, "copy.sql"))
		if err != nil || string(b) != "select 44" {
			t.Errorf("direct save: %q %v", b, err)
		}
		u.app.SetFocus(u.active().Editor.Area)
		u.saveFileAs()
		fileDialogKey(u, tcell.KeyEscape)
		if u.dialogContent != nil || u.active().Path != filepath.Join(dir, "copy.sql") || u.app.GetFocus() != u.active().Editor.Area {
			t.Error("cancel must preserve path and restore focus")
		}
	})
}

// Route keys through the same global and widget handlers as the application.
func fileDialogKey(u *UI, key tcell.Key) {
	event := tcell.NewEventKey(key, 0, 0)
	if event = u.key(event); event != nil {
		u.pages.InputHandler()(event, func(p tview.Primitive) { u.app.SetFocus(p) })
	}
}

func TestSQLFileDialogProtections(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.sql")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(dir, "large.sql")
	if err := os.WriteFile(large, make([]byte, 8*1024*1024+1), 0600); err != nil {
		t.Fatal(err)
	}
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Editor.SetText("replacement")
		tab.Path = filepath.Join(dir, "current.sql")
		tab.Saved = "saved"
		for _, confirm := range []bool{false, true} {
			u.saveFileAs()
			p := u.dialogContent.(*sqlFileDialog)
			p.name.SetText(target)
			p.name.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
			if _, ok := u.dialogContent.(*tview.Modal); !ok {
				t.Error("overwrite needs confirmation")
				return
			}
			if confirm {
				fileDialogKey(u, tcell.KeyRight)
			}
			fileDialogKey(u, tcell.KeyEnter)
			b, err := os.ReadFile(target)
			want := "original"
			if confirm {
				want = "replacement"
			}
			if err != nil || string(b) != want {
				t.Errorf("overwrite confirm=%v: %q %v", confirm, b, err)
			}
			if !confirm && (tab.Path != filepath.Join(dir, "current.sql") || tab.Saved != "saved") {
				t.Error("rejected overwrite changed document")
			}
		}
		tab.Editor.SetText("dirty")
		fileDialogKey(u, tcell.KeyCtrlO)
		p := u.dialogContent.(*sqlFileDialog)
		p.name.SetText(target)
		p.submit()
		if _, ok := u.dialogContent.(*tview.Modal); !ok {
			t.Error("dirty open needs confirmation")
			return
		}
		fileDialogKey(u, tcell.KeyEnter)
		if tab.Editor.Area.GetText() != "dirty" || tab.Path != target || tab.Saved != "replacement" {
			t.Error("rejected open changed document")
		}
		for _, path := range []string{large, filepath.Join(dir, "missing.sql")} {
			u.openFile()
			p = u.dialogContent.(*sqlFileDialog)
			p.name.SetText(path)
			p.submit()
			fileDialogKey(u, tcell.KeyRight)
			fileDialogKey(u, tcell.KeyEnter)
			if tab.Editor.Area.GetText() != "dirty" || tab.Path != target || tab.Saved != "replacement" {
				t.Error("failed open changed document")
			}
		}
		u.saveFileAs()
		p = u.dialogContent.(*sqlFileDialog)
		p.name.SetText(filepath.Join(dir, "missing", "query.sql"))
		p.submit()
		if tab.Path != target || tab.Saved != "replacement" {
			t.Error("failed save changed document")
		}
		b, err := os.ReadFile(target)
		if err != nil || string(b) != "replacement" {
			t.Error("failed operations changed existing file")
		}
	})
}

func TestSQLFileDialogNavigationKeys(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "child")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.sql"), []byte("select 1"), 0600); err != nil {
		t.Fatal(err)
	}
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		u.active().Path = filepath.Join(dir, "current.sql")
		fileDialogKey(u, tcell.KeyCtrlO)
		p := u.dialogContent.(*sqlFileDialog)
		// Directories precede files: index 0 is parent, index 1 is child.
		p.list.SetCurrentItem(1)
		fileDialogKey(u, tcell.KeyEnter)
		if p.path.GetText() != sub {
			t.Error("Enter did not navigate to child")
		}
		p.list.SetCurrentItem(0)
		fileDialogKey(u, tcell.KeyEnter)
		if p.path.GetText() != dir {
			t.Errorf("parent did not navigate up: path=%q dir=%q want=%q listFocused=%v focus=%T item=%d", p.path.GetText(), p.dir, dir, p.list.HasFocus(), u.app.GetFocus(), p.list.GetCurrentItem())
		}
		fileDialogKey(u, tcell.KeyBacktab)
		if !p.path.HasFocus() {
			t.Error("Shift+Tab must focus path")
		}
		p.path.SetText(sub)
		fileDialogKey(u, tcell.KeyEnter)
		if p.path.GetText() != sub {
			t.Error("path field did not navigate")
		}
		fileDialogKey(u, tcell.KeyTab)
		p.list.SetCurrentItem(1)
		fileDialogKey(u, tcell.KeyEnter)
		if u.active().Editor.Area.GetText() != "select 1" {
			t.Error("Enter did not open file")
		}
	})
}

func TestSQLFileDialogCancelAndErrors(t *testing.T) {
	u := runningUI(t, config.Config{})
	u.app.QueueUpdateDraw(func() {
		u.active().Editor.SetText("unsaved SQL")
		u.saveFile()
		p, ok := u.dialogContent.(*sqlFileDialog)
		if !ok {
			t.Error("first save must open picker")
			return
		}
		original := p.dir
		p.name.SetText("")
		p.submit()
		if u.dialogContent != p || p.status.GetText(false) != "Choose a file name" {
			t.Error("empty name must keep the dialog open")
		}
		p.navigate(filepath.Join(t.TempDir(), "missing"))
		if p.dir != original {
			t.Error("failed navigation changed directory")
		}
		// From the list, Tab visits name, Save, then Cancel.
		for i := 0; i < 3; i++ {
			fileDialogKey(u, tcell.KeyTab)
		}
		fileDialogKey(u, tcell.KeyEnter)
		if u.dialogContent != nil {
			t.Error("Cancel button must be keyboard accessible")
			u.dismiss()
		}
		if u.active().Path != "" || u.active().Saved != "" || u.active().Editor.Area.GetText() != "unsaved SQL" {
			t.Error("cancel changed document")
		}
	})
}

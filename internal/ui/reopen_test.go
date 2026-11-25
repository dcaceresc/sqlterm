package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/result"
)

func TestReopenRestoresSQLWithoutSavingFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	file := filepath.Join(dir, "report.sql")
	if err := os.WriteFile(file, []byte("select 1"), 0600); err != nil {
		t.Fatal(err)
	}
	u := New(path, config.Config{})
	u.active().Path, u.active().Saved = file, "select 1"
	_ = u.active().Editor.SetText("select 2")
	u.newTab(nil)
	_ = u.active().Editor.SetText("select 'ñ'\n-- draft")
	u.current = 0
	u.showTab()
	quitTestWorkspace(t, u)

	reopened := New(path, config.Config{})
	defer reopened.cancel()
	if len(reopened.tabs) != 2 || reopened.current != 0 {
		t.Fatalf("restored %d tabs, active %d; want 2 tabs, active 0", len(reopened.tabs), reopened.current)
	}
	if tab := reopened.active(); tab.Path != file || tab.Saved != "select 1" || tab.Editor.Area.GetText() != "select 2" || tab.Session != nil {
		t.Fatal("file-backed draft did not restore as disconnected and unsaved")
	}
	if tab := reopened.tabs[1]; tab.Path != "" || tab.Editor.Area.GetText() != "select 'ñ'\n-- draft" || tab.Session != nil {
		t.Fatal("untitled draft did not restore as disconnected")
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "select 1" {
		t.Fatalf("quitting modified the SQL file: %q, %v", b, err)
	}

	_ = reopened.active().Editor.SetText("select 3")
	quitTestWorkspace(t, reopened)
	again := New(path, config.Config{})
	defer again.cancel()
	if got := again.active().Editor.Area.GetText(); got != "select 3" {
		t.Fatalf("second quit did not replace cached draft: %q", got)
	}
}

func TestQuitOffersExplicitExitAfterNeovimCloses(t *testing.T) {
	u := runningNeovimUI(t)
	u.app.QueueUpdateDraw(func() {
		if err := u.active().Editor.Close(); err != nil {
			t.Error(err)
		}
		for _, key := range " qq" {
			dispatchModified(u, tcell.KeyRune, key, 0)
		}
		if _, ok := u.dialogContent.(*tview.Modal); !ok || u.closing {
			t.Errorf("Space q q must offer a choice without exiting, got %T", u.dialogContent)
			return
		}
		fileDialogKey(u, tcell.KeyEnter)
		if u.closing || u.dialogContent != nil {
			t.Error("cancel did not keep the application open")
		}
	})
}

func quitThroughLeader(u *UI) {
	u.leaderMenu()
	for _, key := range "qq" {
		u.leaderPanel.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, key, 0))
	}
}

func quitTestWorkspace(t *testing.T, u *UI) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(screen)
	finished := make(chan error, 1)
	go func() { finished <- u.Run() }()
	u.app.QueueUpdateDraw(func() {
		u.dismiss()
		quitThroughLeader(u)
		if !u.closing {
			t.Error("Space q q did not start normal shutdown")
			u.app.Stop()
		}
	})
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("quit timed out")
	}
}

func TestExportBrowsesAndRemembersSuccessfulDestination(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	destination, other := t.TempDir(), t.TempDir()
	u := runningUIAtPath(t, configPath, config.Config{})
	store, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	choose := func(u *UI) *sqlFileDialog {
		u.export()
		menu := u.dialogContent.(*tview.List)
		menu.SetCurrentItem(2)
		fileDialogKey(u, tcell.KeyEnter)
		picker, ok := u.dialogContent.(*sqlFileDialog)
		if !ok {
			t.Errorf("export opened %T, want the file browser", u.dialogContent)
		}
		return picker
	}
	u.app.QueueUpdateDraw(func() {
		u.active().Results = []*result.Store{store}
		if p := choose(u); p != nil {
			if p.name.GetText() != "results.json" || !p.save {
				t.Error("JSON export did not suggest results.json in save mode")
			}
			p.navigate(destination)
			p.submit()
		}
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	file := filepath.Join(destination, "results.json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	u.app.QueueUpdateDraw(func() {
		if p := choose(u); p != nil {
			if p.dir != destination {
				t.Errorf("next export started in %q", p.dir)
			}
			p.navigate(other)
			fileDialogKey(u, tcell.KeyEscape)
		}
	})
	reopened := New(configPath, config.Config{})
	defer reopened.cancel()
	reopened.active().Results = []*result.Store{store}
	if p := choose(reopened); p != nil && p.dir != destination {
		t.Errorf("reopened export started in %q, want %q", p.dir, destination)
	}
	if after, err := os.ReadFile(file); err != nil || string(after) != string(before) {
		t.Errorf("cancel changed original export: %q, %v", after, err)
	}
	if _, err := os.Stat(filepath.Join(other, "results.json")); !os.IsNotExist(err) {
		t.Errorf("cancel created another export: %v", err)
	}
}

func TestExportSelectionAndEntireResultInDisplayOrder(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mark, toggle bool
		format       int
		file, want   string
	}{
		{"selection JSON", true, false, 2, "results.json", "{\"columns\":[\"=dup\",\"=dup\"],\"rows\":[[\"@Compañía\",null],[\"Café\",\"\"]]}\n"},
		{"selection safe CSV", true, false, 0, "results.csv", "\xef\xbb\xbf'=dup,'=dup\n'@Compañía,\\N\nCafé,\n"},
		{"selection literal CSV", true, false, 1, "results.csv", "\xef\xbb\xbf=dup,=dup\n@Compañía,\\N\nCafé,\n"},
		{"all by default despite focus", false, false, 2, "results.json", "{\"columns\":[\"other\",\"=dup\",\"=dup\"],\"rows\":[[\"third\",\"@Compañía\",null],[\"second\",\"Café\",\"\"],[\"first\",\"one\",\"end\"]]}\n"},
		{"override selection", true, true, 2, "results.json", "{\"columns\":[\"other\",\"=dup\",\"=dup\"],\"rows\":[[\"third\",\"@Compañía\",null],[\"second\",\"Café\",\"\"],[\"first\",\"one\",\"end\"]]}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := runningUI(t, config.Config{})
			destination := t.TempDir()
			store, err := result.New([]string{"other", "=dup", "=dup"})
			if err != nil {
				t.Fatal(err)
			}
			for _, cells := range [][]result.Cell{
				{{Text: "first"}, {Text: "one"}, {Text: "end"}},
				{{Text: "second"}, {Text: "Café"}, {}},
				{{Text: "third"}, {Text: "@Compañía"}, {Null: true}},
			} {
				if err := store.Append(cells); err != nil {
					t.Fatal(err)
				}
			}
			u.app.QueueUpdateDraw(func() {
				u.active().Results = []*result.Store{store}
				u.active().Order = []int{2, 1, 0}
				u.showResults()
				u.app.SetFocus(u.table)
				u.table.Select(2, 3)
				if tc.mark {
					dispatchModified(u, tcell.KeyLeft, 0, tcell.ModShift)
					dispatchModified(u, tcell.KeyUp, 0, tcell.ModShift)
				}
				dispatchModified(u, tcell.KeyRune, 'e', 0)
				menu := u.dialogContent.(*tview.List)
				if tc.toggle {
					if menu.GetItemCount() < 4 {
						t.Error("scope cannot be changed")
						return
					}
					menu.SetCurrentItem(3)
					fileDialogKey(u, tcell.KeyEnter)
				}
				menu.SetCurrentItem(tc.format)
				fileDialogKey(u, tcell.KeyEnter)
				picker, ok := u.dialogContent.(*sqlFileDialog)
				if !ok {
					t.Errorf("expected file picker, got %T", u.dialogContent)
					return
				}
				picker.navigate(destination)
				picker.submit()
			})
			waitUI(t, u, func() bool { return !u.active().Busy })
			got, err := os.ReadFile(filepath.Join(destination, tc.file))
			if err != nil || string(got) != tc.want {
				t.Fatalf("export = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

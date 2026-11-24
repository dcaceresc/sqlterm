package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

func TestAsyncTaskFailureIsLoggedOnceWithoutSeparateSQLOrResults(t *testing.T) {
	dir := t.TempDir()
	u := runningUIAtPath(t, filepath.Join(dir, "custom.json"), config.Config{})
	password := "secret/value"
	u.app.QueueUpdateDraw(func() {
		u.active().Password = password
		u.active().Editor.SetText("SELECT private_data FROM customers")
		u.active().Messages = []string{"result: confidential row"}
		u.task(u.active(), "Execute", func(context.Context) (func(), error) {
			return nil, errors.New("backend failed with secret%2Fvalue")
		})
	})
	waitUI(t, u, func() bool { return !u.active().Busy })

	data, err := os.ReadFile(filepath.Join(dir, "sqlterm-errors.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "operation=Execute") != 1 || !strings.Contains(text, "backend failed") {
		t.Fatalf("task failure was not logged once:\n%s", text)
	}
	for _, absent := range []string{password, "secret%2Fvalue", "SELECT private_data", "confidential row"} {
		if strings.Contains(text, absent) {
			t.Fatalf("log contains unrelated or secret state %q:\n%s", absent, text)
		}
	}
}

func TestConnectionTestFailureIsLoggedOnceAndStillShown(t *testing.T) {
	dir := t.TempDir()
	u := runningUIAtPath(t, filepath.Join(dir, "config.json"), config.Config{})
	password := "connection secret"
	u.app.QueueUpdateDraw(func() {
		u.profileForm(-1)
		f := currentForm(u)
		f.GetFormItem(2).(*tview.InputField).SetText("localhost")
		f.GetFormItem(5).(*tview.InputField).SetText("user")
		f.GetFormItem(6).(*tview.InputField).SetText(password)
		u.openSession = func(context.Context, config.Profile, string) (*db.Session, error) {
			return nil, errors.New("dial failed for connection+secret")
		}
		f.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	waitUI(t, u, func() bool {
		return strings.Contains(currentForm(u).GetFormItem(10).(*tview.TextView).GetText(false), "failed")
	})

	data, err := os.ReadFile(filepath.Join(dir, "sqlterm-errors.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "operation=Test connection") != 1 || !strings.Contains(text, "dial failed") {
		t.Fatalf("connection failure was not logged once:\n%s", text)
	}
	if strings.Contains(text, password) || strings.Contains(text, "connection+secret") {
		t.Fatalf("connection password leaked:\n%s", text)
	}
}

func TestLogWriteFailureDoesNotReplaceTaskError(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	u := runningUIAtPath(t, filepath.Join(blocked, "config.json"), config.Config{})
	u.app.QueueUpdateDraw(func() {
		u.task(u.active(), "Execute", func(context.Context) (func(), error) {
			return nil, errors.New("original backend failure")
		})
	})
	waitUI(t, u, func() bool { return !u.active().Busy })
	u.app.QueueUpdateDraw(func() {
		if !strings.Contains(u.active().Status, "original backend failure") {
			t.Errorf("original UI error replaced: %q", u.active().Status)
		}
	})
}

func runningUIAtPath(t *testing.T, path string, c config.Config) *UI {
	t.Helper()
	u := New(path, c)
	s := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(s)
	s.SetSize(120, 40)
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	u.app.QueueUpdateDraw(func() {})
	t.Cleanup(func() {
		u.app.QueueUpdateDraw(func() { u.closing = true; u.cancel() })
		u.wg.Wait()
		u.app.QueueUpdateDraw(func() { u.app.Stop() })
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("UI did not stop")
		}
		for _, tab := range u.tabs {
			u.dispose(tab)
		}
	})
	return u
}

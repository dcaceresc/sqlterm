package ui

import (
	"context"
	"github.com/gdamore/tcell/v2"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"strings"
	"testing"
	"time"
)

func TestClosingLastTabCreatesFreshTab(t *testing.T) {
	u := runningUI(t, config.Config{})
	fresh := u.active()
	u.app.QueueUpdateDraw(func() { u.closeTab() })
	waitUI(t, u, func() bool { return !u.active().Busy && len(u.tabs) == 1 })
	if u.active() == fresh || u.current != 0 {
		t.Errorf("closing the idle last tab did not replace it: current=%d", u.current)
	}
}

func TestCloseTabRefusesRunningStatementBeforeUnsavedPrompt(t *testing.T) {
	u := runningUI(t, config.Config{})
	tab := u.active()
	started, release := make(chan struct{}), make(chan struct{})
	u.app.QueueUpdateDraw(func() {
		tab.Editor.SetText("SELECT unsaved")
		u.run(tab, "SELECT unsaved", func(ctx context.Context, emit func(db.Event)) error {
			close(started)
			<-release
			return ctx.Err()
		})
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not start")
	}
	u.app.QueueUpdateDraw(func() {
		u.closeTab()
	})
	if u.dialogContent != nil {
		t.Error("running statement asked to discard unsaved SQL before refusing")
	}
	if !strings.Contains(tab.Status, "Cancel execution before closing this tab") {
		t.Errorf("running statement close status = %q", tab.Status)
	}
	if len(u.tabs) != 1 {
		t.Error("running statement closed the tab")
	}
	close(release)
	waitUI(t, u, func() bool { return !tab.Busy })
	// An idle tab with unsaved SQL still asks before discarding it.
	u.app.QueueUpdateDraw(func() { u.closeTab() })
	if u.dialogContent == nil {
		t.Error("idle tab with unsaved SQL closed without asking")
	} else {
		u.app.QueueUpdateDraw(func() { u.dismiss() })
		if len(u.tabs) != 1 {
			t.Error("dismissing the prompt closed the tab")
		}
	}
}

func TestBusyExecutionStillRefusesSecondStart(t *testing.T) {
	u := runningUI(t, config.Config{})
	tab := u.active()
	started, release := make(chan struct{}), make(chan struct{})
	starts := 0
	u.app.QueueUpdateDraw(func() {
		u.run(tab, "SELECT busy", func(ctx context.Context, emit func(db.Event)) error {
			starts++
			close(started)
			<-release
			return ctx.Err()
		})
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not start")
	}
	u.app.QueueUpdateDraw(func() {
		tab.Editor.SetText("SELECT second")
		dispatch(u, tcell.KeyF5, 0)
		if !strings.Contains(tab.Status, "Session busy") {
			t.Errorf("working tab did not refuse a second start: %q", tab.Status)
		}
	})
	if starts != 1 {
		t.Fatalf("second statement started while the first was running: %d", starts)
	}
	close(release)
	u.app.QueueUpdateDraw(func() { dispatch(u, tcell.KeyF10, 0) })
	waitUI(t, u, func() bool { return !tab.Busy })
}

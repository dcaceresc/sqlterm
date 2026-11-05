package ui

import (
	"context"
	"errors"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"path/filepath"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"strings"
	"testing"
	"time"
)

func screenText(s tcell.SimulationScreen) string {
	cells, w, _ := s.GetContents()
	var b strings.Builder
	for i, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		} else {
			b.WriteByte(' ')
		}
		if (i+1)%w == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func TestCertificateFailureOffersProfileEditingWithoutChangingTLS(t *testing.T) {
	p := config.Profile{Name: "local", Engine: "sqlserver", Host: "localhost", User: "sa", TLS: "verify"}
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Profiles: []config.Profile{p}})
	defer u.cancel()
	u.connectionFailure(p, errors.New("x509: certificate signed by unknown authority"))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 35)
	u.pages.SetRect(0, 0, 120, 35)
	u.pages.Draw(screen)
	screen.Show()
	text := screenText(screen)
	for _, label := range []string{"TLS certificate verification failed", "Edit profile", "Trust server certificate", "Encrypt connection"} {
		if !strings.Contains(text, label) {
			t.Fatalf("missing %q:\n%s", label, text)
		}
	}
	if u.cfg.Profiles[0].TLS != "verify" {
		t.Fatal("silently downgraded TLS")
	}
}
func TestReviewCanReachApplyUsingKeyboard(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	u.active().Session = &db.Session{Adapter: db.NewAdapter("postgres")}
	u.active().Profile.Engine = "postgres"
	u.active().Editor.SetText("CREATE FUNCTION public.f() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;")
	u.applyDefinition()
	send := func() {
		u.pages.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	}
	send()
	button, ok := u.app.GetFocus().(*tview.Button)
	if !ok || button.GetLabel() != "Cancel" {
		t.Fatalf("cannot reach Cancel: %T", u.app.GetFocus())
	}
	send()
	button, ok = u.app.GetFocus().(*tview.Button)
	if !ok || button.GetLabel() != "Apply to server" {
		t.Fatalf("cannot reach Apply: %T", u.app.GetFocus())
	}
}
func TestTerminalLayoutAndVisibleFilePrompt(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if e := screen.Init(); e != nil {
		t.Fatal(e)
	}
	defer screen.Fini()
	screen.SetSize(140, 42)
	u.pages.SetRect(0, 0, 140, 42)
	draw := func() string { u.pages.Draw(screen); screen.Show(); return screenText(screen) }
	u.chooseConnection()
	if text := draw(); !strings.Contains(text, "Connections") || !strings.Contains(text, "Add profile") {
		t.Fatal(text)
	}
	u.dismiss()
	if text := draw(); !strings.Contains(text, "Objects") || !strings.Contains(text, "NORMAL") || !strings.Contains(text, "Results") {
		t.Fatal(text)
	}
	u.prompt("Save SQL file", "query.sql", false, func(string) {})
	if text := draw(); !strings.Contains(text, "query.sql") {
		t.Fatal("file path was masked:\n" + text)
	}
	u.dismiss()
	u.prompt("Password", "top-secret", true, func(string) {})
	if text := draw(); strings.Contains(text, "top-secret") {
		t.Fatal("password visible")
	}
}
func TestQuitCancelsAndJoinsBackgroundWork(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	screen := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(screen)
	screen.SetSize(120, 40)
	done := make(chan error, 1)
	go func() { done <- u.Run() }()
	started := make(chan struct{})
	finished := make(chan struct{})
	u.app.QueueUpdateDraw(func() {
		u.dismiss()
		u.task(u.active(), "Long task", func(ctx context.Context) (func(), error) {
			close(started)
			<-ctx.Done()
			close(finished)
			return nil, ctx.Err()
		})
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("task did not start")
	}
	u.app.QueueUpdateDraw(func() { u.quit() })
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("quit deadlocked with worker")
	}
	select {
	case <-finished:
	default:
		t.Fatal("worker leaked")
	}
}

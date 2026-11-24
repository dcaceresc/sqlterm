//go:build !js && !wasip1

package terminal_test

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	pasteOn  = "\x1b[?2004h"
	pasteOff = "\x1b[?2004l"
	mouseOn  = "\x1b[?1006h"
	mouseOff = "\x1b[?1006l"
)

func TestTerminfoInputDispatch(t *testing.T) {
	t.Setenv("LC_ALL", "C.UTF-8")
	t.Setenv("LINES", "24")
	t.Setenv("COLUMNS", "80")
	t.Setenv("TCELL_ALTSCREEN", "")

	const literal = "i:quit ggVG\n\tSELECT '界'; -- 🙂"
	// Cada fragmento es una lectura real de la Tty: se parten el delimitador y
	// un carácter UTF-8, que termina justo antes del delimitador de cierre.
	tty := &scriptedTTY{
		input: []string{
			"\x1b[1;5C\x1b[20",
			"0~i:quit ggVG\r\tSELECT '界'; -- " + "🙂"[:2],
			"🙂"[2:] + "\x1b[201~\x16\x1b[12~\x1b[<4;7;4M\x1b[21~",
		},
		size:    tcell.WindowSize{Width: 80, Height: 24},
		drained: make(chan struct{}),
	}
	// El terminfo se fija explícitamente para no depender de TERM ni de una consola real.
	profile, err := tcell.LookupTerminfo("xterm-truecolor")
	if err != nil {
		t.Fatal(err)
	}
	screen, err := tcell.NewTerminfoScreenFromTtyTerminfo(tty, profile)
	if err != nil {
		t.Fatal(err)
	}

	app := tview.NewApplication().SetScreen(screen).EnablePaste(true).EnableMouse(true)
	pane := &inputRecorder{Box: tview.NewBox(), keysReady: make(chan struct{})}
	var captured []keyPress
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		captured = append(captured, press(event))
		return event
	})
	app.SetRoot(pane, true)
	resizeDrawn := make(chan struct{})
	var resized sync.Once
	app.SetAfterDrawFunc(func(tcell.Screen) {
		if _, _, width, height := pane.GetRect(); width == 100 && height == 30 {
			resized.Do(func() { close(resizeDrawn) })
		}
	})

	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	waitFor := func(name string, seen <-chan struct{}) {
		select {
		case err := <-done:
			t.Fatalf("app exited before %s: %v", name, err)
		case <-deadline.C:
			app.Stop()
			<-done
			t.Fatalf("timed out waiting for %s; keys=%+v, paste=%q, mouse=%+v", name, pane.keys, pane.pastes, pane.clicks)
		case <-seen:
		}
	}
	waitFor("terminal key after paste and mouse", pane.keysReady)
	tty.resize(100, 30)
	waitFor("tview redraw at resized dimensions", resizeDrawn)
	app.Stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if want := []string{literal}; !slices.Equal(pane.pastes, want) {
		t.Errorf("PasteHandler calls = %q, want %q", pane.pastes, want)
	}
	wantKeys := []keyPress{
		{tcell.KeyRight, tcell.ModCtrl},
		{tcell.KeyCtrlV, tcell.ModCtrl},
		{tcell.KeyF2, tcell.ModNone},
		{tcell.KeyF10, tcell.ModNone},
	}
	if !slices.Equal(pane.keys, wantKeys) || !slices.Equal(captured, wantKeys) {
		t.Errorf("key dispatch = %+v; input capture = %+v; want only %+v", pane.keys, captured, wantKeys)
	}
	if want := []mousePress{{6, 3, tcell.Button1, tcell.ModShift}}; !slices.Equal(pane.clicks, want) {
		t.Errorf("mouse presses = %+v, want %+v", pane.clicks, want)
	}
	output := tty.output.String()
	for _, mode := range []struct{ name, on, off string }{
		{"bracketed paste", pasteOn, pasteOff},
		{"SGR mouse", mouseOn, mouseOff},
		{"alternate screen", profile.EnterCA, profile.ExitCA},
	} {
		if on, off := strings.Index(output, mode.on), strings.LastIndex(output, mode.off); mode.on == "" || mode.off == "" || on < 0 || off <= on {
			t.Errorf("%s mode not enabled then disabled (on=%d, off=%d)", mode.name, on, off)
		}
	}
	if !tty.stopped || !tty.closed || tty.onResize != nil {
		t.Error("screen did not stop, release the resize callback and close the Tty")
	}
}

type keyPress struct {
	key tcell.Key
	mod tcell.ModMask
}

func press(event *tcell.EventKey) keyPress { return keyPress{event.Key(), event.Modifiers()} }

type mousePress struct {
	x, y    int
	buttons tcell.ButtonMask
	mod     tcell.ModMask
}

type inputRecorder struct {
	*tview.Box
	pastes    []string
	keys      []keyPress
	clicks    []mousePress
	keysReady chan struct{}
}

func (r *inputRecorder) PasteHandler() func(string, func(tview.Primitive)) {
	return func(text string, _ func(tview.Primitive)) { r.pastes = append(r.pastes, text) }
}

func (r *inputRecorder) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return func(event *tcell.EventKey, _ func(tview.Primitive)) {
		r.keys = append(r.keys, press(event))
		if event.Key() == tcell.KeyF10 {
			close(r.keysReady)
		}
	}
}

func (r *inputRecorder) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		if action != tview.MouseLeftDown {
			return false, nil
		}
		x, y := event.Position()
		r.clicks = append(r.clicks, mousePress{x, y, event.Buttons(), event.Modifiers()})
		return true, nil
	}
}

type scriptedTTY struct {
	mu       sync.Mutex
	input    []string
	output   bytes.Buffer
	size     tcell.WindowSize
	onResize func()
	drained  chan struct{}
	stopped  bool
	closed   bool
}

func (tty *scriptedTTY) Start() error { return nil }
func (tty *scriptedTTY) Stop() error  { tty.stopped = true; return nil }
func (tty *scriptedTTY) Close() error { tty.closed = true; return nil }
func (tty *scriptedTTY) Drain() error { close(tty.drained); return nil }

func (tty *scriptedTTY) Read(p []byte) (int, error) {
	if len(tty.input) == 0 {
		<-tty.drained
		return 0, io.EOF
	}
	n := copy(p, tty.input[0])
	tty.input[0] = tty.input[0][n:]
	if tty.input[0] == "" {
		tty.input = tty.input[1:]
	}
	return n, nil
}

func (tty *scriptedTTY) Write(p []byte) (int, error) {
	tty.mu.Lock()
	defer tty.mu.Unlock()
	return tty.output.Write(p)
}

func (tty *scriptedTTY) WindowSize() (tcell.WindowSize, error) {
	tty.mu.Lock()
	defer tty.mu.Unlock()
	return tty.size, nil
}

func (tty *scriptedTTY) NotifyResize(callback func()) {
	tty.mu.Lock()
	defer tty.mu.Unlock()
	tty.onResize = callback
}

func (tty *scriptedTTY) resize(width, height int) {
	tty.mu.Lock()
	tty.size = tcell.WindowSize{Width: width, Height: height}
	callback := tty.onResize
	tty.mu.Unlock()
	if callback != nil {
		callback()
	}
}

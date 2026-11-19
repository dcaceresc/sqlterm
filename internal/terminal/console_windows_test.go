package terminal_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/sys/windows"
)

const consoleSample = "i:quit ggVG SELECT 1;"

func TestWindowsConsoleDiagnostic(t *testing.T) {
	if testing.Short() || os.Getenv("SQLTERM_WINDOWS_TERMINAL_DIAGNOSTIC") != "1" {
		t.Skip("manual Windows Terminal check; use scripts/verify-windows-terminal.ps1")
	}
	in := openConsole(t, "CONIN$")
	out := openConsole(t, "CONOUT$")
	originalIn, originalOut := consoleMode(t, in), consoleMode(t, out)
	screen, err := tcell.NewScreen()
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		t.Fatalf("initialize native screen: %v", err)
	}
	defer func() {
		screen.Fini()
		if restoredIn, restoredOut := consoleMode(t, in), consoleMode(t, out); restoredIn != originalIn || restoredOut != originalOut {
			t.Errorf("console modes not restored: input %#x -> %#x, output %#x -> %#x", originalIn, restoredIn, originalOut, restoredOut)
		}
	}()

	// tcell no propaga los errores de SetConsoleMode en este backend.
	if inputMode, outputMode := consoleMode(t, in), consoleMode(t, out); inputMode&windows.ENABLE_VIRTUAL_TERMINAL_INPUT == 0 || outputMode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		t.Fatalf("native VT modes inactive: input=%#x, output=%#x", inputMode, outputMode)
	}
	screen.EnablePaste()
	screen.EnableMouse()
	startWidth, startHeight := screen.Size()
	sample := []rune(consoleSample)
	var started, ended, matched int
	var pasting, mismatch, leaked, f2, ctrlRight, click, resized bool

	draw := func() {
		lines := []string{
			"Paste the prepared sample with Windows Terminal Ctrl+V; do not type it.",
			"Press Ctrl+Right and F2, left-click, then resize the window or pane.",
			"Press F10 to finish or Ctrl+C to cancel (60-second limit).",
			fmt.Sprintf("Paste matched: %t | Ctrl+Right: %t | F2: %t | Click: %t | Resize: %t", ended == 1 && matched == len(sample) && !mismatch, ctrlRight, f2, click, resized),
		}
		screen.Clear()
		for y, line := range lines {
			for x, r := range line {
				screen.SetContent(x, y, r, nil, tcell.StyleDefault)
			}
		}
		screen.Show()
	}
	draw()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	events := make(chan tcell.Event)
	go screen.ChannelEvents(events, ctx.Done())
	for {
		select {
		case <-ctx.Done():
			t.Fatal("diagnostic timed out; screen will be finalized")
		case event, ok := <-events:
			if !ok {
				t.Fatal("diagnostic event stream closed; screen will be finalized")
			}
			switch event := event.(type) {
			case *tcell.EventPaste:
				if event.Start() {
					started++
					pasting = true
				} else {
					ended++
					pasting = false
				}
			case *tcell.EventKey:
				if pasting {
					// No se imprime ni se almacena el contenido del portapapeles.
					if matched >= len(sample) || event.Key() != tcell.KeyRune || event.Rune() != sample[matched] {
						mismatch = true
					} else {
						matched++
					}
					break
				}
				switch press(event) {
				case keyPress{tcell.KeyF10, tcell.ModNone}:
					if started != 1 || ended != 1 || mismatch || matched != len(sample) || leaked || !ctrlRight || !f2 || !click || !resized {
						t.Errorf("start/end=%d/%d, sample matched=%t, unframed text=%t, Ctrl+Right=%t, F2=%t, mouse=%t, resize=%t",
							started, ended, matched == len(sample) && !mismatch, leaked, ctrlRight, f2, click, resized)
					}
					return
				case keyPress{tcell.KeyCtrlC, tcell.ModCtrl}:
					t.Fatal("diagnostic canceled")
				case keyPress{tcell.KeyRight, tcell.ModCtrl}:
					ctrlRight = true
				case keyPress{tcell.KeyF2, tcell.ModNone}:
					f2 = true
				default:
					leaked = leaked || event.Key() == tcell.KeyRune
				}
			case *tcell.EventMouse:
				click = click || event.Buttons()&tcell.Button1 != 0
			case *tcell.EventResize:
				width, height := event.Size()
				resized = resized || width != startWidth || height != startHeight
			case *tcell.EventError:
				t.Fatal(event)
			}
			draw()
		}
	}
}

func openConsole(t *testing.T, name string) windows.Handle {
	t.Helper()
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatalf("open %s (run from a Windows Terminal pane): %v", name, err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
	return handle
}

func consoleMode(t *testing.T, handle windows.Handle) uint32 {
	t.Helper()
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		t.Fatalf("read console mode: %v", err)
	}
	return mode
}

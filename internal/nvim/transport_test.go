package nvim

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestNativeStartupHasNoNeovimIntro(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("Neovim is not installed")
	}
	s, err := Start(Options{Dir: t.TempDir(), Width: 100, Height: 30})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	frame := s.Frame()
	var rendered strings.Builder
	for row := 0; row < frame.Height; row++ {
		for col := 0; col < frame.Width; col++ {
			rendered.WriteString(frame.Cell(row, col).Text)
		}
		rendered.WriteByte('\n')
	}
	for _, intro := range []string{"NVIM v", "Nvim is open source", ":help nvim", "type  :help"} {
		if strings.Contains(rendered.String(), intro) {
			t.Fatalf("initial frame contains Neovim intro %q:\n%s", intro, rendered.String())
		}
	}
	snapshot, err := s.Snapshot()
	if err != nil || snapshot.Text != "" || snapshot.Cursor != 0 {
		t.Fatalf("startup must preserve empty SQL: snapshot=%+v err=%v", snapshot, err)
	}
}

func TestNativeTransportLifecycle(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("Neovim is not installed")
	}
	s, err := Start(Options{Dir: t.TempDir(), Width: 40, Height: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	if frame := s.Frame(); frame.Sequence == 0 || frame.Width != 40 || frame.Height != 10 {
		t.Fatalf("initial frame=%+v", frame)
	}
	if err := s.Resize(42, 11); err != nil {
		t.Fatal(err)
	}
	if frame := s.Frame(); frame.Width != 42 || frame.Height != 11 {
		t.Fatalf("resized frame=%+v", frame)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	default:
		t.Error("Close returned without reaping the child")
	}
	if !errors.Is(s.Err(), ErrClosed) {
		t.Errorf("close reason=%v", s.Err())
	}
}

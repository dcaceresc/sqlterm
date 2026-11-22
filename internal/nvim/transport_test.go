package nvim

import (
	"errors"
	"os/exec"
	"testing"
)

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

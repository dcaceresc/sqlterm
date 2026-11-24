package nvim

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedEditorHidesScratchStatuslineButKeepsSearchPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	binary, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("Neovim is not installed")
	}
	s, err := Start(Options{Binary: binary, Dir: t.TempDir(), Width: 60, Height: 12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := s.Configure(true, false); err != nil {
		t.Fatal(err)
	}
	const sql = "SELECT 1;"
	if err := s.SetText(sql); err != nil {
		t.Fatal(err)
	}
	const search = "/SELECT"
	if err := s.Input(search); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		frame := s.Frame()
		var grid strings.Builder
		for row := 0; row < frame.Height; row++ {
			for col := 0; col < frame.Width; col++ {
				grid.WriteString(frame.Cell(row, col).Text)
			}
			grid.WriteByte('\n')
		}
		visible := grid.String()
		if strings.Contains(visible, sql) && strings.Contains(visible, search) {
			if strings.Contains(strings.ToLower(visible), "[scratch]") {
				t.Fatalf("the redundant scratch statusline is visible:\n%s", visible)
			}
			return
		}
		select {
		case <-s.Changed():
		case <-deadline.C:
			t.Fatalf("SQL or search prompt was not rendered:\n%s", visible)
		}
	}
}

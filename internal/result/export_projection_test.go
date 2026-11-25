package result_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlterm/internal/result"
)

func TestExportProjectionContextDestinationSafety(t *testing.T) {
	store, err := result.New([]string{"ignored", "value"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	// La primera fila desborda los búferes de escritura antes de procesar la segunda.
	first := strings.Repeat("a", 64*1024)
	const second = "second row"
	for _, value := range []string{first, second} {
		if err := store.Append([]result.Cell{{Text: "ignored"}, {Text: value}}); err != nil {
			t.Fatal(err)
		}
	}

	for _, format := range []string{"csv", "json"} {
		t.Run(format+"/canceled_during_write", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "projection."+format)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watch := &cancelOnExportBytes{Context: ctx, cancel: cancel, path: path}

			err := store.ExportProjectionContext(watch, path, format, true, []int{0, 1}, 1, 2)
			if watch.readErr != nil {
				t.Fatalf("observe destination during export: %v", watch.readErr)
			}
			if !strings.Contains(watch.written, first[:1024]) || strings.Contains(watch.written, second) {
				t.Errorf("expected first-row bytes without the second row before cancellation, observed %d bytes", len(watch.written))
			}
			if err != context.Canceled {
				t.Errorf("export error = %v, want context.Canceled", err)
			}
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("stat after cancellation = %v, want fs.ErrNotExist", err)
			}
		})

		t.Run(format+"/existing_destination", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "projection."+format)
			const original = "existing destination\n"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}

			err = store.ExportProjectionContext(context.Background(), path, format, true, []int{0, 1}, 1, 2)
			if !errors.Is(err, fs.ErrExist) {
				t.Errorf("export error = %v, want fs.ErrExist", err)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
				t.Error("existing destination was replaced or its metadata changed")
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != original {
				t.Errorf("existing destination changed: got %d bytes, want %q", len(contents), original)
			}
		})
	}
}

type cancelOnExportBytes struct {
	context.Context
	cancel  context.CancelFunc
	path    string
	written string
	readErr error
}

func (c *cancelOnExportBytes) Err() error {
	if c.written == "" && c.readErr == nil {
		contents, err := os.ReadFile(c.path)
		switch {
		case err == nil && len(contents) > 0:
			c.written = string(contents)
			c.cancel()
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			c.readErr = err
		}
	}
	return c.Context.Err()
}

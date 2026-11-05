package clipboardtext

import (
	"context"
	"sqlterm/internal/result"
	"strings"
	"syscall"
	"testing"
)

func TestWindowsCopiedRowNULRegression(t *testing.T) {
	store, err := result.New([]string{"text", "binary"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Append([]result.Cell{{Text: "normal cell"}, {Text: "\x00row\x00version"}})
	for _, headers := range []bool{false, true} {
		var b strings.Builder
		if err = store.WriteTSV(context.Background(), &b, nil, headers); err != nil {
			t.Fatal(err)
		}
		text := b.String()
		panicked := false
		func() { defer func() { panicked = recover() != nil }(); syscall.StringToUTF16(text) }()
		if !panicked {
			t.Fatal("fixture did not reproduce the old Windows clipboard panic")
		}
		escaped, err := Write(func(s string) error { _, err := syscall.UTF16FromString(s); return err }, text)
		if err != nil || !escaped {
			t.Fatalf("Windows clipboard conversion still fails: %v", err)
		}
	}
}

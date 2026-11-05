package clipboardtext

import (
	"errors"
	"strings"
	"testing"
)

func TestWritePreservesTSVAndEscapesEmbeddedNUL(t *testing.T) {
	original := "name\tvalue\nñ\tbefore\x00after\n😀\t\"line\nbreak\"\n"
	var copied string
	escaped, err := Write(func(text string) error { copied = text; return nil }, original)
	if err != nil || !escaped {
		t.Fatal(escaped, err)
	}
	if copied != "name\tvalue\nñ\tbefore\\0after\n😀\t\"line\nbreak\"\n" || strings.IndexByte(copied, 0) >= 0 {
		t.Fatal("clipboard data lost TSV separators or Unicode")
	}
	if !strings.Contains(original, "\x00") {
		t.Fatal("source changed")
	}
}
func TestWriteLeavesOrdinaryTextUnchanged(t *testing.T) {
	text := "header\tvalue\nhello\tworld\n"
	escaped, err := Write(func(actual string) error {
		if actual != text {
			t.Fatal("ordinary clipboard text changed")
		}
		return nil
	}, text)
	if err != nil || escaped {
		t.Fatal(escaped, err)
	}
}
func TestWriteConvertsBackendPanicIntoErrorAndCanRetry(t *testing.T) {
	_, err := Write(func(string) error { panic("native conversion failed") }, "row\tvalue\n")
	if err == nil {
		t.Fatal("backend panic not reported")
	}
	called := false
	if _, err = Write(func(string) error { called = true; return nil }, "retry"); err != nil || !called {
		t.Fatal("clipboard unusable after failure")
	}
	expected := errors.New("clipboard locked")
	if _, err = Write(func(string) error { return expected }, "text"); !errors.Is(err, expected) {
		t.Fatal("lost backend error")
	}
}

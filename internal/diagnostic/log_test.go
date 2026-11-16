package diagnostic

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogErrorAppendsFullRedactedBackendErrorsBesideConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom", "settings.json")
	password := "p@ss word/value"
	encoded := url.QueryEscape(password)
	first := errors.New("sqlserver: collation conflict\nline 2: cannot resolve " + password + " / " + encoded)

	if err := LogError(configPath, "Execute", first, password); err != nil {
		t.Fatal(err)
	}
	if err := LogError(configPath, "Refresh catalog", errors.New("postgres: second failure"), password); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(filepath.Dir(configPath), "sqlterm-errors.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"operation=Execute", "sqlserver: collation conflict\nline 2: cannot resolve", "operation=Refresh catalog", "postgres: second failure"} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "timestamp=") != 2 {
		t.Fatalf("writes did not append exactly two records:\n%s", text)
	}
	if strings.Contains(text, password) || strings.Contains(text, encoded) {
		t.Fatalf("known password leaked into log:\n%s", text)
	}
}

func TestLogErrorDoesNotCreateFileUntilAnErrorIsWritten(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := LogError(configPath, "Execute", nil, "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(configPath), "sqlterm-errors.log")); !os.IsNotExist(err) {
		t.Fatalf("nil error created a log: %v", err)
	}
}

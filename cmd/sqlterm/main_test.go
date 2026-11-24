package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLogsStartupConfigErrorBesideCustomConfigAndKeepsStderr(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nested", "custom.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"--config", configPath}, &stderr); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "sqlterm: config:") {
		t.Fatalf("startup stderr changed: %q", stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(configPath), "sqlterm-errors.log"))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(data); !strings.Contains(text, "operation=Startup") || !strings.Contains(text, "config:") {
		t.Fatalf("startup error not logged:\n%s", text)
	}
}

func TestRunKeepsOriginalStartupErrorWhenLogCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sqlterm-errors.log"), 0700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"--config", configPath}, &stderr); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "sqlterm:") || strings.Contains(stderr.String(), "sqlterm-errors.log") {
		t.Fatalf("logging replaced startup error: %q", stderr.String())
	}
}

func TestRunHelpKeepsSuccessfulExitAndDoesNotLog(t *testing.T) {
	for _, option := range []string{"--help", "-h"} {
		t.Run(option, func(t *testing.T) {
			dir := t.TempDir()
			var stderr bytes.Buffer
			code := run([]string{"--config", filepath.Join(dir, "config.json"), option}, &stderr)
			if code != 0 || !strings.Contains(stderr.String(), "-config") || !strings.Contains(stderr.String(), "-version") {
				t.Fatalf("help exit = %d, stderr = %q", code, stderr.String())
			}
			if _, err := os.Stat(filepath.Join(dir, "sqlterm-errors.log")); !os.IsNotExist(err) {
				t.Fatalf("help created an error log: %v", err)
			}
		})
	}
}

func TestRunVersionDoesNotCreateErrorLog(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	var stderr bytes.Buffer
	if code := run([]string{"--config", configPath, "--version"}, &stderr); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "sqlterm-errors.log")); !os.IsNotExist(err) {
		t.Fatalf("version eagerly created log: %v", err)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type commandRecord struct {
	Args       []string `json:"args"`
	Dir        string   `json:"dir"`
	GoCache    string   `json:"gocache"`
	GoPath     string   `json:"gopath"`
	GoModCache string   `json:"gomodcache"`
	GoOS       string   `json:"goos"`
	GoArch     string   `json:"goarch"`
	CGOEnabled string   `json:"cgo_enabled"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	record := commandRecord{
		Args:       os.Args[1:],
		Dir:        cwd,
		GoCache:    os.Getenv("GOCACHE"),
		GoPath:     os.Getenv("GOPATH"),
		GoModCache: os.Getenv("GOMODCACHE"),
		GoOS:       os.Getenv("GOOS"),
		GoArch:     os.Getenv("GOARCH"),
		CGOEnabled: os.Getenv("CGO_ENABLED"),
	}
	file, err := os.OpenFile(os.Getenv("FAKE_GO_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(file).Encode(record); err != nil {
		file.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := file.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	joined := strings.Join(os.Args[1:], " ")
	fmt.Printf("fake stdout: %s\n", joined)
	fmt.Fprintf(os.Stderr, "fake stderr: %s\n", joined)
	if match := os.Getenv("FAKE_GO_FAIL_MATCH"); match != "" && strings.Contains(joined, match) {
		code, err := strconv.Atoi(os.Getenv("FAKE_GO_EXIT"))
		if err != nil || code == 0 {
			code = 1
		}
		os.Exit(code)
	}

	if len(os.Args) > 1 && os.Args[1] == "build" {
		for i := 2; i+1 < len(os.Args); i++ {
			if os.Args[i] != "-o" {
				continue
			}
			output := os.Args[i+1]
			if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if err := os.WriteFile(output, []byte("fake build output\n"), 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			break
		}
	}
}

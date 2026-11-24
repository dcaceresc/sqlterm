package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
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

func TestCLI(t *testing.T) {
	fakeGo := buildFakeGo(t)

	t.Run("help documents the portable interface without running children", func(t *testing.T) {
		for _, args := range [][]string{nil, {"help"}, {"-h"}, {"--help"}} {
			t.Run(fmt.Sprintf("args_%q", args), func(t *testing.T) {
				repo, nested, logPath := newTestRepo(t)
				var stdout, stderr strings.Builder

				code := run(args, nested, fakeEnvironment(t, fakeGo, logPath), &stdout, &stderr)

				if code != 0 {
					t.Fatalf("run() code = %d, want 0", code)
				}
				if stdout.String() != usageText {
					t.Fatalf("stdout = %q, want %q", stdout.String(), usageText)
				}
				if stderr.Len() != 0 {
					t.Fatalf("stderr = %q, want empty", stderr.String())
				}
				assertNoCommandsOrDist(t, repo, logPath)
			})
		}
	})

	t.Run("invalid commands and options fail before side effects", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
			want string
		}{
			{name: "unknown command", args: []string{"release"}, want: "unknown command: release\n"},
			{name: "build option", args: []string{"build", "-integration"}, want: "build does not accept options or arguments\n"},
			{name: "check option", args: []string{"check", "-verbose"}, want: "unknown check option: -verbose\n"},
			{name: "check argument", args: []string{"check", "extra"}, want: "unexpected check argument: extra\n"},
			{name: "duplicate integration", args: []string{"check", "-integration", "-integration"}, want: "-integration may only be specified once\n"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repo, nested, logPath := newTestRepo(t)
				var stdout, stderr strings.Builder

				code := run(tc.args, nested, fakeEnvironment(t, fakeGo, logPath), &stdout, &stderr)

				if code != 2 {
					t.Fatalf("run() code = %d, want 2", code)
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
				if stderr.String() != tc.want+usageText {
					t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want+usageText)
				}
				assertNoCommandsOrDist(t, repo, logPath)
			})
		}
	})

	t.Run("build preserves all six targets and child-only environment", func(t *testing.T) {
		repo, nested, logPath := newTestRepo(t)
		t.Setenv("GOOS", "parent-os")
		t.Setenv("GOARCH", "parent-arch")
		t.Setenv("CGO_ENABLED", "parent-cgo")
		var stdout, stderr strings.Builder

		code := run([]string{"build"}, nested, fakeEnvironment(t, fakeGo, logPath), &stdout, &stderr)

		if code != 0 {
			t.Fatalf("run() code = %d, want 0; stderr: %s", code, stderr.String())
		}
		targets := []struct{ goos, goarch string }{
			{"windows", "amd64"}, {"windows", "arm64"},
			{"linux", "amd64"}, {"linux", "arm64"},
			{"darwin", "amd64"}, {"darwin", "arm64"},
		}
		records := readRecords(t, logPath)
		if len(records) != len(targets) {
			t.Fatalf("command count = %d, want %d", len(records), len(targets))
		}
		var wantStdout, wantStderr strings.Builder
		for i, target := range targets {
			suffix := ""
			if target.goos == "windows" {
				suffix = ".exe"
			}
			output := filepath.Join("dist", "sqlterm-"+target.goos+"-"+target.goarch+suffix)
			wantArgs := []string{"build", "-trimpath", "-o", output, "./cmd/sqlterm"}
			assertRecord(t, records[i], repo, wantArgs, target.goos, target.goarch, "0")
			if _, err := os.Stat(filepath.Join(repo, output)); err != nil {
				t.Fatalf("expected output %s: %v", output, err)
			}
			fmt.Fprintf(&wantStdout, "fake stdout: %s\nBuilt %s\n", strings.Join(wantArgs, " "), output)
			fmt.Fprintf(&wantStderr, "fake stderr: %s\n", strings.Join(wantArgs, " "))
		}
		if stdout.String() != wantStdout.String() || stderr.String() != wantStderr.String() {
			t.Fatalf("unexpected propagated output\nstdout: %q\nstderr: %q", stdout.String(), stderr.String())
		}
		if os.Getenv("GOOS") != "parent-os" || os.Getenv("GOARCH") != "parent-arch" || os.Getenv("CGO_ENABLED") != "parent-cgo" {
			t.Fatalf("parent build environment changed: GOOS=%q GOARCH=%q CGO_ENABLED=%q", os.Getenv("GOOS"), os.Getenv("GOARCH"), os.Getenv("CGO_ENABLED"))
		}
	})

	t.Run("build stops on the first child failure and preserves its exit code", func(t *testing.T) {
		repo, nested, logPath := newTestRepo(t)
		env := append(fakeEnvironment(t, fakeGo, logPath), "FAKE_GO_FAIL_MATCH=sqlterm-linux-amd64", "FAKE_GO_EXIT=7")
		var stdout, stderr strings.Builder

		code := run([]string{"build"}, nested, env, &stdout, &stderr)

		if code != 7 {
			t.Fatalf("run() code = %d, want 7", code)
		}
		records := readRecords(t, logPath)
		if len(records) != 3 {
			t.Fatalf("command count = %d, want 3", len(records))
		}
		if _, err := os.Stat(filepath.Join(repo, "dist", "sqlterm-linux-amd64")); !os.IsNotExist(err) {
			t.Fatalf("failed target artifact exists or stat failed unexpectedly: %v", err)
		}
		if !strings.Contains(stderr.String(), "go build failed for linux/amd64") {
			t.Fatalf("stderr %q does not identify failed target", stderr.String())
		}
	})

	t.Run("check preserves test then vet semantics", func(t *testing.T) {
		repo, nested, logPath := newTestRepo(t)
		var stdout, stderr strings.Builder

		code := run([]string{"check"}, nested, fakeEnvironment(t, fakeGo, logPath), &stdout, &stderr)

		if code != 0 {
			t.Fatalf("run() code = %d, want 0", code)
		}
		assertCheckRecords(t, readRecords(t, logPath), repo, [][]string{{"test", "./..."}, {"vet", "./..."}})
		if !strings.Contains(stdout.String(), "fake stdout: test ./...") || !strings.Contains(stderr.String(), "fake stderr: vet ./...") {
			t.Fatalf("child output not propagated: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})

	t.Run("check integration adds the disposable database test last", func(t *testing.T) {
		repo, nested, logPath := newTestRepo(t)
		var stdout, stderr strings.Builder

		code := run([]string{"check", "-integration"}, nested, fakeEnvironment(t, fakeGo, logPath), &stdout, &stderr)

		if code != 0 {
			t.Fatalf("run() code = %d, want 0", code)
		}
		assertCheckRecords(t, readRecords(t, logPath), repo, [][]string{
			{"test", "./..."},
			{"vet", "./..."},
			{"test", "./internal/db", "-run", "TestIntegration", "-v", "-count=1"},
		})
	})

	t.Run("check stops on failure and preserves the child exit code", func(t *testing.T) {
		repo, nested, logPath := newTestRepo(t)
		env := append(fakeEnvironment(t, fakeGo, logPath), "FAKE_GO_FAIL_MATCH=test ./...", "FAKE_GO_EXIT=9")
		var stdout, stderr strings.Builder

		code := run([]string{"check", "-integration"}, nested, env, &stdout, &stderr)

		if code != 9 {
			t.Fatalf("run() code = %d, want 9", code)
		}
		assertCheckRecords(t, readRecords(t, logPath), repo, [][]string{{"test", "./..."}})
		if !strings.Contains(stderr.String(), "go test failed") {
			t.Fatalf("stderr %q does not identify failed check", stderr.String())
		}
	})
}

func buildFakeGo(t *testing.T) string {
	t.Helper()
	goExecutable, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("find go executable: %v", err)
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command(goExecutable, "build", "-o", path, "./testdata/fakego")
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake go: %v\n%s", err, output)
	}
	return path
}

func newTestRepo(t *testing.T) (string, string, string) {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/test\n\ngo 1.23\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	nested := filepath.Join(repo, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("create nested cwd: %v", err)
	}
	return repo, nested, filepath.Join(repo, "commands.jsonl")
}

func fakeEnvironment(t *testing.T, fakeGo, logPath string) []string {
	t.Helper()
	path := filepath.Dir(fakeGo) + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", path)
	return append(os.Environ(), "FAKE_GO_LOG="+logPath)
}

func readRecords(t *testing.T, path string) []commandRecord {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open command log: %v", err)
	}
	defer file.Close()
	var records []commandRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record commandRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode command record: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan command log: %v", err)
	}
	return records
}

func assertRecord(t *testing.T, record commandRecord, repo string, args []string, goos, goarch, cgo string) {
	t.Helper()
	// El proceso hijo puede devolver la ruta física aunque el repositorio se abrió por un enlace simbólico.
	expectedDir, err := os.Stat(repo)
	if err != nil {
		t.Fatalf("stat expected directory %q: %v", repo, err)
	}
	actualDir, err := os.Stat(record.Dir)
	if err != nil {
		t.Fatalf("stat child directory %q: %v", record.Dir, err)
	}
	if !os.SameFile(expectedDir, actualDir) || !reflect.DeepEqual(record.Args, args) {
		t.Fatalf("record = %+v, want dir %q args %q", record, repo, args)
	}
	if record.GoCache != filepath.Join(repo, ".cache", "build") || record.GoPath != filepath.Join(repo, ".cache", "go") || record.GoModCache != filepath.Join(repo, ".cache", "mod") {
		t.Fatalf("cache environment = %+v", record)
	}
	if record.GoOS != goos || record.GoArch != goarch || record.CGOEnabled != cgo {
		t.Fatalf("target environment = %+v, want %s/%s CGO=%s", record, goos, goarch, cgo)
	}
}

func assertCheckRecords(t *testing.T, records []commandRecord, repo string, want [][]string) {
	t.Helper()
	if len(records) != len(want) {
		t.Fatalf("command count = %d, want %d", len(records), len(want))
	}
	for i := range want {
		assertRecord(t, records[i], repo, want[i], os.Getenv("GOOS"), os.Getenv("GOARCH"), os.Getenv("CGO_ENABLED"))
	}
}

func assertNoCommandsOrDist(t *testing.T, repo, logPath string) {
	t.Helper()
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("command log exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "dist")); !os.IsNotExist(err) {
		t.Fatalf("dist exists or stat failed unexpectedly: %v", err)
	}
}

func TestFindRepoRootFailure(t *testing.T) {
	cwd := t.TempDir()
	var stdout, stderr strings.Builder
	code := run([]string{"check"}, cwd, os.Environ(), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "could not find repository root") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestExitCodeFallback(t *testing.T) {
	if got := exitCode(fmt.Errorf("plain error")); got != 1 {
		t.Fatalf("exitCode(plain error) = %s, want 1", strconv.Itoa(got))
	}
}

package history

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentAddRetainsEveryEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	const writers = 8
	const perWriter = 12

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for writer := 0; writer < writers; writer++ {
		writer := writer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := Add(path, testEntry(fmt.Sprintf("thread-%d-%d", writer, i))); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	assertEntrySet(t, path, "thread", writers, perWriter)
}

func TestConcurrentProcessesRetainEveryEntryAndReadableSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	const writers = 6
	const perWriter = 10

	type process struct {
		cmd    *exec.Cmd
		output bytes.Buffer
	}
	processes := make([]*process, 0, writers)
	for writer := 0; writer < writers; writer++ {
		proc := &process{cmd: exec.Command(os.Args[0], "-test.run=^TestHistoryProcessHelper$")}
		proc.cmd.Env = append(os.Environ(),
			"SQLTERM_HISTORY_HELPER=1",
			"SQLTERM_HISTORY_PATH="+path,
			fmt.Sprintf("SQLTERM_HISTORY_WRITER=%d", writer),
			fmt.Sprintf("SQLTERM_HISTORY_COUNT=%d", perWriter),
		)
		proc.cmd.Stdout = &proc.output
		proc.cmd.Stderr = &proc.output
		if err := proc.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		processes = append(processes, proc)
	}

	stopReader := make(chan struct{})
	readerDone := make(chan struct{})
	readerErr := make(chan error, 1)
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stopReader:
				return
			default:
				if _, err := Load(path); err != nil {
					readerErr <- err
					return
				}
			}
		}
	}()

	for _, proc := range processes {
		if err := proc.cmd.Wait(); err != nil {
			t.Fatalf("helper failed: %v\n%s", err, proc.output.Bytes())
		}
	}
	close(stopReader)
	<-readerDone
	select {
	case err := <-readerErr:
		t.Fatalf("Load observed an invalid snapshot: %v", err)
	default:
	}

	assertEntrySet(t, path, "process", writers, perWriter)
}

func TestConcurrentGoroutinesAndProcessesRetainEveryEntryWithinCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	const (
		processWriters   = 2
		goroutineWriters = 3
		perWriter        = 30
	)

	type process struct {
		cmd    *exec.Cmd
		output bytes.Buffer
	}
	processes := make([]*process, 0, processWriters)
	for writer := 0; writer < processWriters; writer++ {
		proc := &process{cmd: exec.Command(os.Args[0], "-test.run=^TestHistoryProcessHelper$")}
		proc.cmd.Env = append(os.Environ(),
			"SQLTERM_HISTORY_HELPER=1",
			"SQLTERM_HISTORY_PATH="+path,
			fmt.Sprintf("SQLTERM_HISTORY_WRITER=%d", writer),
			fmt.Sprintf("SQLTERM_HISTORY_COUNT=%d", perWriter),
		)
		proc.cmd.Stdout = &proc.output
		proc.cmd.Stderr = &proc.output
		if err := proc.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		processes = append(processes, proc)
	}

	var wg sync.WaitGroup
	errs := make(chan error, goroutineWriters)
	for writer := 0; writer < goroutineWriters; writer++ {
		writer := writer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := Add(path, testEntry(fmt.Sprintf("thread-%d-%d", writer, i))); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	for _, proc := range processes {
		if err := proc.cmd.Wait(); err != nil {
			t.Fatalf("helper failed: %v\n%s", err, proc.output.Bytes())
		}
	}
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	entries, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := (processWriters + goroutineWriters) * perWriter; len(entries) != want {
		t.Fatalf("entry count = %d, want %d", len(entries), want)
	}
	assertEntryKeys(t, entries, "process", processWriters, perWriter)
	assertEntryKeys(t, entries, "thread", goroutineWriters, perWriter)

	// Filling past the cap must keep exactly the newest 200 entries.
	const finalWrites = 100
	for i := 0; i < finalWrites; i++ {
		if err := Add(path, testEntry(fmt.Sprintf("final-%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	entries, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxEntries {
		t.Fatalf("entry count = %d, want %d after exceeding the cap", len(entries), maxEntries)
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if seen[entry.SQL] {
			t.Fatalf("duplicate entry retained: %s", entry.SQL)
		}
		seen[entry.SQL] = true
	}
	for i := 0; i < finalWrites; i++ {
		if key := fmt.Sprintf("final-%d", i); !seen[key] {
			t.Fatalf("newest entry %s was trimmed by the cap", key)
		}
	}
}

func TestHistoryProcessHelper(t *testing.T) {
	if os.Getenv("SQLTERM_HISTORY_HELPER") != "1" {
		return
	}
	path := os.Getenv("SQLTERM_HISTORY_PATH")
	writer, err := strconv.Atoi(os.Getenv("SQLTERM_HISTORY_WRITER"))
	if err != nil {
		t.Fatal(err)
	}
	count, err := strconv.Atoi(os.Getenv("SQLTERM_HISTORY_COUNT"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		if err := Add(path, testEntry(fmt.Sprintf("process-%d-%d", writer, i))); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCorruptHistoryIsRejectedWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	corrupt := []byte("{not valid history")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}

	if err := Add(path, testEntry("must-not-be-written")); err == nil {
		t.Fatal("Add accepted corrupt history")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, corrupt) {
		t.Fatalf("corrupt history changed: got %q", got)
	}
	assertNoHistoryTemps(t, path)
}

func TestReplacementFailurePreservesPriorHistoryAndCleansTemp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	originalReplace := replaceHistoryFile
	replaceHistoryFile = func(_, _ string) error { return errors.New("replacement denied") }
	t.Cleanup(func() { replaceHistoryFile = originalReplace })

	if err := Add(path, testEntry("after")); err == nil || !strings.Contains(err.Error(), "replacement denied") {
		t.Fatalf("Add error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("failed replacement changed prior history")
	}
	entries, err := Load(path)
	if err != nil || len(entries) != 1 || entries[0].SQL != "before" {
		t.Fatalf("prior history is not readable: entries=%v err=%v", entries, err)
	}
	assertNoHistoryTemps(t, path)
}

func TestHistoryFilePermissionsArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("private")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("history permissions = %04o, want 0600", got)
	}
}

func testEntry(sql string) Entry {
	return Entry{At: time.Unix(1700000000, 0).UTC(), Profile: "test", Database: "isolated", SQL: sql}
}

func assertEntrySet(t *testing.T, path, prefix string, writers, perWriter int) {
	t.Helper()
	entries, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := writers * perWriter; len(entries) != want {
		t.Fatalf("entry count = %d, want %d", len(entries), want)
	}
	assertEntryKeys(t, entries, prefix, writers, perWriter)
}

func assertEntryKeys(t *testing.T, entries []Entry, prefix string, writers, perWriter int) {
	t.Helper()
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		seen[entry.SQL] = true
	}
	for writer := 0; writer < writers; writer++ {
		for i := 0; i < perWriter; i++ {
			key := fmt.Sprintf("%s-%d-%d", prefix, writer, i)
			if !seen[key] {
				t.Errorf("missing %s", key)
			}
		}
	}
}

func assertNoHistoryTemps(t *testing.T, path string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary history files remain: %v", matches)
	}
}

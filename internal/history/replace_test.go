package history

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func lockPathFor(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".lock")
}

// statLockFile records the identity of the persistent sidecar lock file. The
// lock file must be created once and never replaced or removed, otherwise a
// second process could hold a lock on a different file than the one it later
// tries to acquire.
func statLockFile(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(lockPathFor(path))
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	return info
}

func assertSameLockFile(t *testing.T, path string, before os.FileInfo) {
	t.Helper()
	after := statLockFile(t, path)
	if !os.SameFile(before, after) {
		t.Fatalf("lock file identity changed: %v -> %v", before, after)
	}
}

// TestNonTransientReplaceFailureIsNotRetried keeps the honest-error guarantee:
// an error outside the demonstrated transient class is reported after a single
// attempt, with the previous history intact and no temporary residue.
func TestNonTransientReplaceFailureIsNotRetried(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lockBefore := statLockFile(t, path)

	sentinel := errors.New("disk on fire")
	var attempts int32
	original := replaceHistoryFile
	replaceHistoryFile = func(_, _ string) error {
		atomic.AddInt32(&attempts, 1)
		return sentinel
	}
	t.Cleanup(func() { replaceHistoryFile = original })

	if err := Add(path, testEntry("after")); !errors.Is(err, sentinel) {
		t.Fatalf("Add error = %v, want %v", err, sentinel)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("replacement attempts = %d, want 1", got)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(after, before) {
		t.Fatalf("failed replacement changed prior history: %q, err = %v", after, readErr)
	}
	assertNoHistoryTemps(t, path)
	assertSameLockFile(t, path, lockBefore)
}

// TestLockFileIdentitySurvivesReplacementsAndClear proves the sidecar lock file
// is never recreated by the write path, so cross-process coordination continues
// to work after many replacements.
func TestLockFileIdentitySurvivesReplacementsAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("first")); err != nil {
		t.Fatal(err)
	}
	lockBefore := statLockFile(t, path)

	for i := 0; i < 25; i++ {
		if err := Add(path, testEntry("entry")); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err != nil {
			t.Fatal(err)
		}
		assertSameLockFile(t, path, lockBefore)
	}

	if err := Clear(path); err != nil {
		t.Fatal(err)
	}
	assertSameLockFile(t, path, lockBefore)
	assertNoHistoryTemps(t, path)
}

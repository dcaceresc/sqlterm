//go:build windows

package history

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// holdDestinationWithoutDeleteSharing emulates an on-access security filter or
// indexer that has the destination open: the handle is granted read access but
// deliberately does not grant FILE_SHARE_DELETE, which is what makes
// MoveFileEx(MOVEFILE_REPLACE_EXISTING) fail on the destination.
func holdDestinationWithoutDeleteSharing(t *testing.T, path string) (release func()) {
	t.Helper()
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		t.Fatalf("hold destination %s: %v", path, err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if err := windows.CloseHandle(handle); err != nil {
				t.Logf("release destination handle: %v", err)
			}
		})
	}
}

// TestHeldDestinationFailsReplacementWithAccessDenied pins the diagnosed
// mechanism: while any other handle holds the destination without delete
// sharing, the replacement syscall fails with ERROR_ACCESS_DENIED and the
// message carries no path, exactly like the reported "Access is denied." flake.
func TestHeldDestinationFailsReplacementWithAccessDenied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}
	temp := filepath.Join(filepath.Dir(path), ".history.json.tmp-held")
	if err := os.WriteFile(temp, []byte(`[]`), 0600); err != nil {
		t.Fatal(err)
	}

	release := holdDestinationWithoutDeleteSharing(t, path)
	err := replaceFile(temp, path)
	release()
	if err == nil {
		t.Fatal("replacement succeeded while the destination was held without delete sharing")
	}
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("replacement error = %v (%T), want ERROR_ACCESS_DENIED", err, err)
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		t.Fatalf("replacement error carries a path (%v); the reported flake does not", err)
	}
}

func TestTransientAccessDeniedIsRetried(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}

	var attempts int32
	original := replaceHistoryFile
	replaceHistoryFile = func(from, to string) error {
		if atomic.AddInt32(&attempts, 1) <= 2 {
			return windows.ERROR_ACCESS_DENIED
		}
		return original(from, to)
	}
	t.Cleanup(func() { replaceHistoryFile = original })

	if err := Add(path, testEntry("after")); err != nil {
		t.Fatalf("Add did not survive a transient denial: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("replacement attempts = %d, want 3", got)
	}
	entries, err := Load(path)
	if err != nil || len(entries) != 2 || entries[1].SQL != "after" {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
	assertNoHistoryTemps(t, path)
}

// TestTransientSharingViolationIsRetried covers the sibling errno of the same
// external-handle family: a handle conflict on the moved source reports
// ERROR_SHARING_VIOLATION instead of ERROR_ACCESS_DENIED.
func TestTransientSharingViolationIsRetried(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}

	var attempts int32
	original := replaceHistoryFile
	replaceHistoryFile = func(from, to string) error {
		if atomic.AddInt32(&attempts, 1) == 1 {
			return windows.ERROR_SHARING_VIOLATION
		}
		return original(from, to)
	}
	t.Cleanup(func() { replaceHistoryFile = original })

	if err := Add(path, testEntry("after")); err != nil {
		t.Fatalf("Add did not survive a transient sharing violation: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("replacement attempts = %d, want 2", got)
	}
	entries, err := Load(path)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
	assertNoHistoryTemps(t, path)
}

func TestPersistentAccessDeniedSurfacesErrorWithoutLosingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lockBefore := statLockFile(t, path)

	var attempts int32
	original := replaceHistoryFile
	replaceHistoryFile = func(_, _ string) error {
		atomic.AddInt32(&attempts, 1)
		return windows.ERROR_ACCESS_DENIED
	}
	t.Cleanup(func() { replaceHistoryFile = original })

	err = Add(path, testEntry("after"))
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("Add error = %v, want the original ERROR_ACCESS_DENIED", err)
	}
	if got := atomic.LoadInt32(&attempts); got < 2 || got > 16 {
		t.Fatalf("replacement attempts = %d, want a small bounded number above 1", got)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(data, before) {
		t.Fatalf("previous history changed: %q, err = %v", data, readErr)
	}
	entries, loadErr := Load(path)
	if loadErr != nil || len(entries) != 1 || entries[0].SQL != "before" {
		t.Fatalf("previous history is not readable: entries=%v err=%v", entries, loadErr)
	}
	assertNoHistoryTemps(t, path)
	assertSameLockFile(t, path, lockBefore)
}

// TestHeldDestinationIsRetriedUntilReleased exercises the path end to end
// without an injected error: a real handle on the destination makes the real
// replacement syscall fail with ERROR_ACCESS_DENIED, and the bounded retry must
// outlive the hold and still land the new entry.
func TestHeldDestinationIsRetriedUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("before")); err != nil {
		t.Fatal(err)
	}
	lockBefore := statLockFile(t, path)

	release := holdDestinationWithoutDeleteSharing(t, path)

	blocked := make(chan struct{})
	var once sync.Once
	original := replaceHistoryFile
	replaceHistoryFile = func(from, to string) error {
		err := original(from, to)
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			once.Do(func() { close(blocked) })
		}
		return err
	}
	t.Cleanup(func() { replaceHistoryFile = original })

	holderDone := make(chan struct{})
	holderErr := make(chan error, 1)
	go func() {
		defer close(holderDone)
		select {
		case <-blocked:
		case <-time.After(10 * time.Second):
			release()
			holderErr <- errors.New("the held destination never blocked a replacement")
			return
		}
		time.Sleep(5 * time.Millisecond)
		release()
	}()

	addErr := Add(path, testEntry("after"))
	<-holderDone
	select {
	case err := <-holderErr:
		t.Fatal(err)
	default:
	}
	if addErr != nil {
		t.Fatalf("Add did not survive a released transient denial: %v", addErr)
	}

	entries, err := Load(path)
	if err != nil || len(entries) != 2 || entries[0].SQL != "before" || entries[1].SQL != "after" {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
	assertNoHistoryTemps(t, path)
	assertSameLockFile(t, path, lockBefore)
}

// TestReplacementStressUnderForeignReaderHandles runs many replacements while
// another goroutine repeatedly holds the destination the way an on-access
// filter does. It fails on unreleased contention, so it would catch the
// original flake instead of hiding it.
func TestReplacementStressUnderForeignReaderHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := Add(path, testEntry("seed")); err != nil {
		t.Fatal(err)
	}
	lockBefore := statLockFile(t, path)

	var attempts atomic.Int64
	original := replaceHistoryFile
	replaceHistoryFile = func(from, to string) error {
		attempts.Add(1)
		return original(from, to)
	}
	t.Cleanup(func() { replaceHistoryFile = original })

	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			release := func() {
				handle, err := windows.CreateFile(
					windows.StringToUTF16Ptr(path),
					windows.GENERIC_READ,
					windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
					nil,
					windows.OPEN_EXISTING,
					0,
					0,
				)
				if err != nil {
					return
				}
				time.Sleep(time.Millisecond)
				_ = windows.CloseHandle(handle)
			}
			release()
			time.Sleep(4 * time.Millisecond)
		}
	}()

	const (
		writers   = 2
		perWriter = 25
	)
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for writer := 0; writer < writers; writer++ {
		writer := writer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := Add(path, testEntry(fmt.Sprintf("stress-%d-%d", writer, i))); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(stop)
	<-readerDone
	close(errs)
	for err := range errs {
		t.Fatalf("Add under foreign reader handles: %v", err)
	}

	replacements := int64(writers*perWriter) + 1
	if got := attempts.Load(); got <= replacements {
		t.Fatalf("replacement attempts = %d for %d replacements; the stress never hit the transient path", got, replacements)
	}
	entries, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := writers*perWriter + 1; len(entries) != want {
		t.Fatalf("entry count = %d, want %d", len(entries), want)
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		seen[entry.SQL] = true
	}
	for writer := 0; writer < writers; writer++ {
		for i := 0; i < perWriter; i++ {
			if key := fmt.Sprintf("stress-%d-%d", writer, i); !seen[key] {
				t.Errorf("missing %s", key)
			}
		}
	}
	assertNoHistoryTemps(t, path)
	assertSameLockFile(t, path, lockBefore)
}

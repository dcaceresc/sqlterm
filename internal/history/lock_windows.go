//go:build windows

package history

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockHistoryFile(path string) (func() error, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	var overlapped windows.Overlapped
	handle := windows.Handle(file.Fd())
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file.Close, nil
}

func replaceFile(from, to string) error {
	return windows.Rename(from, to)
}

// isTransientReplaceError reports whether a failed replacement is worth
// retrying. MoveFileEx(MOVEFILE_REPLACE_EXISTING) cannot replace a destination
// that another handle holds, and it also refuses when an on-access filter
// driver (security agent, indexer) intercepts the move, so a momentary external
// hold surfaces as a bare ERROR_ACCESS_DENIED or, when the moved source is
// involved, ERROR_SHARING_VIOLATION. Both clear on their own within
// milliseconds once the external handle goes away, so they are retried. Every
// other error, including a permanent ACL or read-only failure, is reported after
// the first attempt.
func isTransientReplaceError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

func syncHistoryDir(string) error {
	return nil
}

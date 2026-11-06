//go:build !windows

package history

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockHistoryFile(path string) (func() error, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() error {
		unlockErr := unix.Flock(int(file.Fd()), unix.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, nil
}

func replaceFile(from, to string) error {
	return os.Rename(from, to)
}

// isTransientReplaceError reports whether a failed replacement is worth
// retrying. On POSIX systems rename(2) replaces the destination even while it
// is open, so the Windows access-denied/sharing class has no counterpart here
// and a failed rename is always reported immediately.
func isTransientReplaceError(error) bool {
	return false
}

func syncHistoryDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

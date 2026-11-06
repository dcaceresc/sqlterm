//go:build windows

package history

import (
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

func syncHistoryDir(string) error {
	return nil
}

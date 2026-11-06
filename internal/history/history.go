package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEntries = 200

type Entry struct {
	At       time.Time `json:"at"`
	Profile  string    `json:"profile"`
	Database string    `json:"database"`
	SQL      string    `json:"sql"`
}

var (
	pathLocksMu sync.Mutex
	pathLocks   = make(map[string]*sync.Mutex)

	// replaceHistoryFile is a variable so failure cleanup can be tested without
	// making platform-specific filesystem assumptions.
	replaceHistoryFile = replaceFile
)

func Load(path string) ([]Entry, error) {
	var entries []Entry
	err := withPathLock(path, func() error {
		var err error
		entries, err = loadUnprotected(path)
		return err
	})
	return entries, err
}

func Add(path string, entry Entry) error {
	return withPathLock(path, func() error {
		entries, err := loadUnprotected(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		if len(entries) > maxEntries {
			entries = entries[len(entries)-maxEntries:]
		}
		return writeAtomic(path, entries)
	})
}

func Clear(path string) error {
	return withPathLock(path, func() error {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return syncHistoryDir(filepath.Dir(path))
	})
}

func loadUnprotected(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func writeAtomic(path string, entries []Entry) (err error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if temp != nil {
			err = errors.Join(err, temp.Close())
		}
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
	}()

	if err = temp.Chmod(0600); err != nil {
		return err
	}
	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		temp = nil
		return err
	}
	temp = nil

	if err = replaceHistoryFile(tempPath, path); err != nil {
		return err
	}
	return syncHistoryDir(dir)
}

func withPathLock(path string, fn func() error) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(absolutePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	lockPath := filepath.Join(dir, "."+filepath.Base(absolutePath)+".lock")
	mutex := mutexForPath(lockPath)
	mutex.Lock()
	defer mutex.Unlock()

	unlock, err := lockHistoryFile(lockPath)
	if err != nil {
		return err
	}
	operationErr := fn()
	return errors.Join(operationErr, unlock())
}

func mutexForPath(path string) *sync.Mutex {
	pathLocksMu.Lock()
	defer pathLocksMu.Unlock()
	mutex := pathLocks[path]
	if mutex == nil {
		mutex = &sync.Mutex{}
		pathLocks[path] = mutex
	}
	return mutex
}

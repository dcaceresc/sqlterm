package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	At       time.Time `json:"at"`
	Profile  string    `json:"profile"`
	Database string    `json:"database"`
	SQL      string    `json:"sql"`
}

func Load(path string) ([]Entry, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var entries []Entry
	e = json.Unmarshal(b, &entries)
	return entries, e
}
func Add(path string, entry Entry) error {
	entries, e := Load(path)
	if e != nil {
		return e
	}
	entries = append(entries, entry)
	if len(entries) > 200 {
		entries = entries[len(entries)-200:]
	}
	b, e := json.MarshalIndent(entries, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	return os.WriteFile(path, b, 0600)
}
func Clear(path string) error {
	e := os.Remove(path)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}

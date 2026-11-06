package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRetentionAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	for i := 0; i < 202; i++ {
		if e := Add(path, Entry{At: time.Now(), Profile: "local", SQL: fmt.Sprint(i)}); e != nil {
			t.Fatalf("Add %d: %v", i, e)
		}
	}
	entries, e := Load(path)
	if e != nil || len(entries) != 200 || entries[0].SQL != "2" {
		t.Fatalf("bad retention: %d %v", len(entries), e)
	}
	if e = Clear(path); e != nil {
		t.Fatal(e)
	}
	lockPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".lock")
	if _, e = os.Stat(lockPath); e != nil {
		t.Fatalf("Clear removed or invalidated the lock file: %v", e)
	}
	entries, e = Load(path)
	if e != nil || len(entries) != 0 {
		t.Fatal(entries, e)
	}
}

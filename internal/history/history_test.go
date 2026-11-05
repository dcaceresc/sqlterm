package history

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRetentionAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	for i := 0; i < 202; i++ {
		if e := Add(path, Entry{At: time.Now(), Profile: "local", SQL: fmt.Sprint(i)}); e != nil {
			t.Fatal(e)
		}
	}
	entries, e := Load(path)
	if e != nil || len(entries) != 200 || entries[0].SQL != "2" {
		t.Fatalf("bad retention: %d %v", len(entries), e)
	}
	if e = Clear(path); e != nil {
		t.Fatal(e)
	}
	entries, e = Load(path)
	if e != nil || len(entries) != 0 {
		t.Fatal(entries, e)
	}
}

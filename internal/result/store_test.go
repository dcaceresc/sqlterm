package result

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorePagesExportAndCleanup(t *testing.T) {
	s, e := New([]string{"same", "same"})
	if e != nil {
		t.Fatal(e)
	}
	dir := s.dir
	defer s.Close()
	for i := 0; i < 10000; i++ {
		if e = s.Append([]Cell{{Text: "ñ,\"line\"\nnext"}, {Null: i%2 == 0, Text: ""}}); e != nil {
			t.Fatal(e)
		}
	}
	for _, n := range []int{9999, 0, 128, 127, 512, 9998} {
		r, e := s.Row(n)
		if e != nil || r[1].Null != (n%2 == 0) {
			t.Fatalf("row %d: %#v %v", n, r, e)
		}
	}
	if len(s.cache) > 128 {
		t.Fatal("unbounded row cache")
	}
	path := filepath.Join(t.TempDir(), "results.json")
	if e = s.Export(path, "json"); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	var data struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	}
	if e = json.Unmarshal(b, &data); e != nil {
		t.Fatal(e)
	}
	if len(data.Columns) != 2 || len(data.Rows) != 10000 || data.Rows[0][1] != nil || data.Rows[1][1] != "" {
		t.Fatal("JSON lost NULL or duplicate columns")
	}
	if e = s.Export(path, "json"); e == nil {
		t.Fatal("silently overwrote export")
	}
	s.Close()
	if _, e = os.Stat(dir); !os.IsNotExist(e) {
		t.Fatalf("temporary directory remains: %v", e)
	}
}
func TestCSVNullEscapingSortSearchAndCancel(t *testing.T) {
	s, _ := New([]string{"value"})
	defer s.Close()
	for _, v := range []Cell{{Text: `\N`}, {Null: true}, {Text: ""}, {Text: "ñ\n,quoted\""}} {
		s.Append([]Cell{v})
	}
	p := filepath.Join(t.TempDir(), "r.csv")
	if e := s.Export(p, "csv"); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	rows, e := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	if rows[1][0] != `\\N` || rows[2][0] != `\N` || rows[3][0] != "" {
		t.Fatal(rows)
	}
	ids, e := s.Order(0, false)
	if e != nil || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("bad sort: %v %v", ids, e)
	}
	n, e := s.Find(context.Background().Done(), "ñ", 0)
	if e != nil || n != 3 {
		t.Fatal(n, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p = filepath.Join(t.TempDir(), "cancel.json")
	if e = s.ExportContext(ctx, p, "json"); e == nil {
		t.Fatal("ignored cancellation")
	}
	if _, e = os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("partial export remains")
	}
}

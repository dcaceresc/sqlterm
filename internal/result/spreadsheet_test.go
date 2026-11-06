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

func TestSpreadsheetSafeTextFormulaMarkersAndBoundaries(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"=1+1", "'=1+1"},
		{" +cmd", "' +cmd"},
		{"\t-cmd", "'\t-cmd"},
		{"\x00@HYPERLINK", "'\x00@HYPERLINK"},
		{"\ufeff=cmd", "'\ufeff=cmd"},
		{"-42", "'-42"},
		{" -1.5", "' -1.5"},
		{"'=-literal", "'=-literal"},
		{"123", "123"},
		{"ñ", "ñ"},
		{"", ""},
	} {
		if got := SpreadsheetSafeText(tc.input); got != tc.want {
			t.Errorf("SpreadsheetSafeText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSpreadsheetSafeTSVAndLiteralTSVPreserveSemantics(t *testing.T) {
	s, err := New([]string{" =header", "empty", "unicode"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Append([]Cell{{Text: "=1+1"}, {Text: ""}, {Text: "ñ\tline\nnext"}}); err != nil {
		t.Fatal(err)
	}
	if err = s.Append([]Cell{{Text: `\N`}, {Null: true}, {Text: "-42"}}); err != nil {
		t.Fatal(err)
	}

	var safe strings.Builder
	if err = s.WriteSpreadsheetTSV(context.Background(), &safe, []int{1, 0}, true); err != nil {
		t.Fatal(err)
	}
	rows := readDelimited(t, safe.String(), '\t')
	if rows[0][0] != "' =header" || rows[1][0] != `\\N` || rows[1][1] != `\N` || rows[1][2] != "'-42" || rows[2][0] != "'=1+1" || rows[2][1] != "" || rows[2][2] != "ñ\tline\nnext" {
		t.Fatalf("safe TSV changed semantics: %#v", rows)
	}

	var literal strings.Builder
	if err = s.WriteTSV(context.Background(), &literal, []int{1, 0}, true); err != nil {
		t.Fatal(err)
	}
	rows = readDelimited(t, literal.String(), '\t')
	if rows[0][0] != " =header" || rows[1][2] != "-42" || rows[2][0] != "=1+1" {
		t.Fatalf("literal TSV changed: %#v", rows)
	}
}

func TestSpreadsheetSafeCSVLiteralCSVAndJSON(t *testing.T) {
	s, err := New([]string{"@header", "value"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Append([]Cell{{Text: "\r=cmd"}, {Text: `\text`}}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	safePath := filepath.Join(dir, "safe.csv")
	if err = s.ExportSpreadsheetContext(context.Background(), safePath, "csv"); err != nil {
		t.Fatal(err)
	}
	safeRows := readDelimited(t, readFile(t, safePath), ',')
	if safeRows[0][0] != "'@header" || safeRows[1][0] != "'\r=cmd" || safeRows[1][1] != `\\text` {
		t.Fatalf("safe CSV = %#v", safeRows)
	}

	literalPath := filepath.Join(dir, "literal.csv")
	if err = s.ExportContext(context.Background(), literalPath, "csv"); err != nil {
		t.Fatal(err)
	}
	literalRows := readDelimited(t, readFile(t, literalPath), ',')
	if literalRows[0][0] != "@header" || literalRows[1][0] != "\r=cmd" {
		t.Fatalf("literal CSV changed: %#v", literalRows)
	}

	jsonPath := filepath.Join(dir, "safe.json")
	if err = s.ExportSpreadsheetContext(context.Background(), jsonPath, "json"); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Columns []string   `json:"columns"`
		Rows    [][]string `json:"rows"`
	}
	if err = json.Unmarshal([]byte(readFile(t, jsonPath)), &data); err != nil {
		t.Fatal(err)
	}
	if data.Columns[0] != "@header" || data.Rows[0][0] != "\r=cmd" {
		t.Fatalf("JSON was sanitized: %#v", data)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelPath := filepath.Join(dir, "cancel.csv")
	if err = s.ExportSpreadsheetContext(ctx, cancelPath, "csv"); err == nil {
		t.Fatal("safe export ignored cancellation")
	}
	if _, err = os.Stat(cancelPath); !os.IsNotExist(err) {
		t.Fatalf("partial safe export remains: %v", err)
	}
	var canceled strings.Builder
	if err = s.WriteSpreadsheetTSV(ctx, &canceled, nil, false); err == nil {
		t.Fatal("safe copy ignored cancellation")
	}
}

func readDelimited(t *testing.T, text string, comma rune) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

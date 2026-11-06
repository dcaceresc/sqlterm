package result

import (
	"context"
	"encoding/csv"
	"strings"
	"testing"
)

func TestCopyTSVHeadersSortOrderAndSpecialValues(t *testing.T) {
	s, err := New([]string{"name", "value"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Append([]Cell{{Text: "ñ\tquoted\"\nline"}, {Null: true}})
	s.Append([]Cell{{Text: "second"}, {Text: ""}})
	for _, headers := range []bool{false, true} {
		var b strings.Builder
		if err = s.WriteTSV(context.Background(), &b, []int{1, 0}, headers); err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(strings.NewReader(b.String()))
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		offset := 0
		if headers {
			if rows[0][0] != "name" {
				t.Fatal(rows)
			}
			offset = 1
		}
		if rows[offset][0] != "second" || rows[offset][1] != "" || rows[offset+1][0] != "ñ\tquoted\"\nline" || rows[offset+1][1] != `\N` {
			t.Fatal(rows)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var b strings.Builder
	if err = s.WriteTSV(ctx, &b, nil, false); err == nil {
		t.Fatal("copy ignored cancellation")
	}
}

// Cell with header copies exactly one TSV record: column name, then value.
func TestCopyCellWithHeaderExactRecord(t *testing.T) {
	s, err := New([]string{"nombre_cliente"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Append([]Cell{{Text: "ACME S.A."}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		spreadsheet bool
		want        string
	}{
		{"safe", true, "nombre_cliente\tACME S.A.\n"},
		{"literal", false, "nombre_cliente\tACME S.A.\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			if err := s.WriteCellWithHeader(context.Background(), &b, 0, 0, tc.spreadsheet); err != nil {
				t.Fatal(err)
			}
			if b.String() != tc.want {
				t.Fatalf("cell with header = %q, want %q", b.String(), tc.want)
			}
		})
	}
}

func TestCopyCellWithHeaderSpecialValuesStayOneRecord(t *testing.T) {
	for _, tc := range []struct {
		name        string
		columns     []string
		row         []Cell
		column      int
		spreadsheet bool
		want        string
	}{
		{"null", []string{"c"}, []Cell{{Null: true}}, 0, true, "c\t\\N\n"},
		{"empty string", []string{"c"}, []Cell{{Text: ""}}, 0, true, "c\t\n"},
		{"unit separator", []string{"c"}, []Cell{{Text: "a\tb\nc\"d"}}, 0, true, "c\t\"a\tb\nc\"\"d\"\n"},
		{"backslash", []string{"c"}, []Cell{{Text: `\N`}}, 0, true, "c\t\\\\N\n"},
		{"unicode", []string{"ñ"}, []Cell{{Text: "日本語 😀"}}, 0, true, "ñ\t日本語 😀\n"},
		{"quoted header", []string{"a\tb"}, []Cell{{Text: "v"}}, 0, true, "\"a\tb\"\tv\n"},
		{"duplicate names use selected column", []string{"dup", "dup"}, []Cell{{Text: "first"}, {Text: "second"}}, 1, true, "dup\tsecond\n"},
		{"safe neutralizes header and value", []string{"=SUM(A1)"}, []Cell{{Text: "+1"}}, 0, true, "'=SUM(A1)\t'+1\n"},
		{"safe keeps plain text", []string{"name"}, []Cell{{Text: "ñ and -text inside"}}, 0, true, "name\tñ and -text inside\n"},
		{"literal leaves both untouched", []string{"=SUM(A1)"}, []Cell{{Text: "+1"}}, 0, false, "=SUM(A1)\t+1\n"},
		{"literal keeps null and backslash markers", []string{"@c"}, []Cell{{Null: true}}, 0, false, "@c\t\\N\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := New(tc.columns)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err = s.Append(tc.row); err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			if err = s.WriteCellWithHeader(context.Background(), &b, 0, tc.column, tc.spreadsheet); err != nil {
				t.Fatal(err)
			}
			if b.String() != tc.want {
				t.Fatalf("cell with header = %q, want %q", b.String(), tc.want)
			}
			reader := csv.NewReader(strings.NewReader(b.String()))
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil {
				t.Fatalf("pasting %q must stay one TSV row: %v", b.String(), err)
			}
			if len(rows) != 1 || len(rows[0]) != 2 {
				t.Fatalf("record = %#v, want one row with two fields", rows)
			}
		})
	}
}

func TestCopyCellWithHeaderRejectsCancellationAndUnknownColumn(t *testing.T) {
	s, err := New([]string{"only"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Append([]Cell{{Text: "value"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var b strings.Builder
	if err = s.WriteCellWithHeader(ctx, &b, 0, 0, true); err == nil {
		t.Fatal("cell with header ignored cancellation")
	}
	if b.Len() != 0 {
		t.Fatalf("cancelled copy wrote %q", b.String())
	}
	if err = s.WriteCellWithHeader(context.Background(), &b, 0, 1, true); err == nil {
		t.Fatal("cell with header accepted a column outside Columns")
	}
	if b.Len() != 0 {
		t.Fatalf("failed copy wrote %q", b.String())
	}
	if err = s.WriteCellWithHeader(context.Background(), &b, 3, 0, true); err == nil {
		t.Fatal("cell with header accepted a row outside the store")
	}
}

// The shared TSV writers keep their previous byte output after the refactor.
func TestTSVWritersKeepExistingByteOutput(t *testing.T) {
	s, err := New([]string{"name", "value"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Append([]Cell{{Text: "=1+1"}, {Text: ""}}); err != nil {
		t.Fatal(err)
	}
	if err = s.Append([]Cell{{Text: `\lead`}, {Null: true}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var safe, literal strings.Builder
	if err = s.WriteSpreadsheetTSV(ctx, &safe, []int{1, 0}, true); err != nil {
		t.Fatal(err)
	}
	if want := "name\tvalue\n\\\\lead\t\\N\n'=1+1\t\n"; safe.String() != want {
		t.Fatalf("safe TSV = %q, want %q", safe.String(), want)
	}
	if err = s.WriteTSV(ctx, &literal, []int{1, 0}, true); err != nil {
		t.Fatal(err)
	}
	if want := "name\tvalue\n\\\\lead\t\\N\n=1+1\t\n"; literal.String() != want {
		t.Fatalf("literal TSV = %q, want %q", literal.String(), want)
	}

	single, err := New([]string{"only"})
	if err != nil {
		t.Fatal(err)
	}
	defer single.Close()
	if err = single.Append([]Cell{{Text: ""}}); err != nil {
		t.Fatal(err)
	}
	var empty strings.Builder
	if err = single.WriteSpreadsheetTSV(ctx, &empty, nil, false); err != nil {
		t.Fatal(err)
	}
	if empty.String() != "\"\"\n" {
		t.Fatalf("single empty value = %q, want %q", empty.String(), "\"\"\n")
	}
}

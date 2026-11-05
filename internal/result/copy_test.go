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

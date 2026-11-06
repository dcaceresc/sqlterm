package result

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
)

// WriteTSV preserves the displayed row order. A nil row list copies all rows.
// As in CSV export, SQL NULL is \N and text starting with \ is escaped.
// It intentionally preserves literal text for existing non-spreadsheet consumers.
func (s *Store) WriteTSV(ctx context.Context, w io.Writer, rows []int, headers bool) error {
	return s.writeTSV(ctx, w, rows, headers, false)
}

// WriteSpreadsheetTSV prefixes formula-like fields for safe spreadsheet paste.
func (s *Store) WriteSpreadsheetTSV(ctx context.Context, w io.Writer, rows []int, headers bool) error {
	return s.writeTSV(ctx, w, rows, headers, true)
}

func (s *Store) writeTSV(ctx context.Context, w io.Writer, rows []int, headers, spreadsheetSafe bool) error {
	c := csv.NewWriter(w)
	c.Comma = '\t'
	write := func(record []string) error {
		if len(record) == 1 && record[0] == "" {
			c.Flush()
			if err := c.Error(); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\"\"\n")
			return err
		}
		return c.Write(record)
	}
	if headers {
		columns := s.Columns
		if spreadsheetSafe {
			columns = spreadsheetSafeRecord(columns)
		}
		if err := write(columns); err != nil {
			return err
		}
	}
	count := s.Count()
	if rows != nil {
		count = len(rows)
	}
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := i
		if rows != nil {
			n = rows[i]
		}
		r, err := s.Row(n)
		if err != nil {
			return fmt.Errorf("copy row: %w", err)
		}
		values := make([]string, len(r))
		for j, v := range r {
			if v.Null {
				values[j] = `\N`
			} else {
				values[j] = v.Text
				if len(v.Text) > 0 && v.Text[0] == '\\' {
					values[j] = `\` + v.Text
				}
			}
		}
		if spreadsheetSafe {
			values = spreadsheetSafeRecord(values)
		}
		if err = write(values); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

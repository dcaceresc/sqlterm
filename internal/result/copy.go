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

// WriteCellWithHeader writes exactly one TSV record: the column name from
// s.Columns followed by the value of one cell, so a paste lands in two adjacent
// cells of the same spreadsheet row. Null and backslash rules match WriteTSV;
// spreadsheetSafe applies to both the header and the value, so literal mode
// leaves both untouched.
func (s *Store) WriteCellWithHeader(ctx context.Context, w io.Writer, row, column int, spreadsheetSafe bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if column < 0 || column >= len(s.Columns) {
		return fmt.Errorf("copy cell: column %d out of range", column)
	}
	r, err := s.Row(row)
	if err != nil {
		return fmt.Errorf("copy row: %w", err)
	}
	if column >= len(r) {
		return fmt.Errorf("copy cell: column %d out of range", column)
	}
	record := []string{s.Columns[column], tsvCell(r[column])}
	if spreadsheetSafe {
		record = spreadsheetSafeRecord(record)
	}
	c := newTSVWriter(w)
	if err = tsvWrite(c, w, record); err != nil {
		return err
	}
	c.Flush()
	return c.Error()
}

func (s *Store) writeTSV(ctx context.Context, w io.Writer, rows []int, headers, spreadsheetSafe bool) error {
	c := newTSVWriter(w)
	write := func(record []string) error { return tsvWrite(c, w, record) }
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
			values[j] = tsvCell(v)
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

func newTSVWriter(w io.Writer) *csv.Writer {
	c := csv.NewWriter(w)
	c.Comma = '\t'
	return c
}

// tsvWrite keeps csv quoting and the historical representation of a record
// whose only field is empty.
func tsvWrite(c *csv.Writer, w io.Writer, record []string) error {
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

// tsvCell renders one cell with the copy rules shared by every TSV scope.
func tsvCell(v Cell) string {
	if v.Null {
		return `\N`
	}
	if len(v.Text) > 0 && v.Text[0] == '\\' {
		return `\` + v.Text
	}
	return v.Text
}

package result_test

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sqlterm/internal/result"
)

func TestExportProjectionXLSX(t *testing.T) {
	store, err := result.New([]string{"skip", "=dup", "=dup"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	const literal = "  +José & <漢字>\t\r\n😀 _x0041_x0042_  "
	for _, row := range [][]result.Cell{
		{{}, {Text: literal}, {Null: true}},
		{{}, {Text: "@SUM(A1)"}, {}},
		{{}, {Text: "invalid\x01"}, {}},
		{{}, {Text: strings.Repeat("😀", 16384)}, {}},
		{{}, {Text: strings.Repeat("\n", 254)}, {}},
	} {
		if err := store.Append(row); err != nil {
			t.Fatal(err)
		}
	}
	export := func(ctx context.Context, path string, rows []int) error {
		return store.ExportProjectionContext(ctx, path, "xlsx", true, rows, 1, 3)
	}
	t.Run("workbook", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "projected.xlsx")
		if err := export(context.Background(), path, []int{1, 0}); err != nil {
			t.Fatal(err)
		}
		z, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer z.Close()
		for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels"} {
			f, err := z.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			var root struct{ XMLName xml.Name }
			err = xml.NewDecoder(f).Decode(&root)
			f.Close()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		f, err := z.Open("xl/worksheets/sheet1.xml")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		type text struct {
			Space string `xml:"http://www.w3.org/XML/1998/namespace space,attr"`
			Value string `xml:",chardata"`
		}
		type cell struct {
			Ref     string  `xml:"r,attr"`
			Type    string  `xml:"t,attr"`
			Text    *text   `xml:"is>t"`
			Formula *string `xml:"f"`
		}
		var sheet struct {
			XMLName xml.Name `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main worksheet"`
			Data    []struct {
				Cells []cell `xml:"c"`
			} `xml:"sheetData>row"`
		}
		if err := xml.NewDecoder(f).Decode(&sheet); err != nil {
			t.Fatal(err)
		}
		var got [][]cell
		for _, row := range sheet.Data {
			got = append(got, row.Cells)
		}
		inline := func(ref, value string) cell {
			return cell{Ref: ref, Type: "inlineStr", Text: &text{"preserve", value}}
		}
		want := [][]cell{
			{inline("A1", "=dup"), inline("B1", "=dup")},
			{inline("A2", "@SUM(A1)"), inline("B2", "")},
			{inline("A3", strings.ReplaceAll(literal, "_x0041_x0042_", "_x005F_x0041_x005F_x0042_"))},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("worksheet cells = %#v, want %#v", got, want)
		}
	})
	t.Run("rejected", func(t *testing.T) {
		for _, tc := range []struct {
			rows    []int
			message string
		}{
			{[]int{0, 2}, "export: XLSX cell A3 contains invalid XML text"},
			{[]int{0, 3}, "export: XLSX cell A3 exceeds 32767 UTF-16 code units"},
			{[]int{0, 4}, "export: XLSX cell A3 exceeds 253 line feeds"},
			{make([]int, 1048576), "export: XLSX exceeds 1048576 worksheet rows including headers"},
		} {
			path := filepath.Join(t.TempDir(), "rejected.xlsx")
			if err := export(context.Background(), path, tc.rows); err == nil || err.Error() != tc.message {
				t.Errorf("error = %v, want %q", err, tc.message)
			}
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("rejected export left destination: %v", err)
			}
		}
		wide, err := result.New(make([]string, 16385))
		if err != nil {
			t.Fatal(err)
		}
		defer wide.Close()
		path := filepath.Join(t.TempDir(), "wide.xlsx")
		if err := wide.ExportProjectionContext(context.Background(), path, "xlsx", false, nil, 0, 16385); err == nil || err.Error() != "export: XLSX exceeds 16384 worksheet columns" {
			t.Errorf("column limit error = %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("wide export left destination: %v", err)
		}
		if store.Count() != 5 {
			t.Errorf("export changed row count to %d", store.Count())
		}
		row, err := store.Row(0)
		if err != nil || row[1].Text != literal || !row[2].Null {
			t.Errorf("export changed stored row: %v, %v", row, err)
		}
	})
	t.Run("destination", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "existing.xlsx")
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := export(context.Background(), path, []int{0}); !errors.Is(err, fs.ErrExist) {
			t.Errorf("existing error = %v", err)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != "original" {
			t.Errorf("existing file = %q, %v", data, err)
		}
		path = filepath.Join(t.TempDir(), "canceled.xlsx")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		watch := &cancelWhenXLSXCreated{Context: ctx, path: path, cancel: cancel}
		if err := export(watch, path, []int{0, 1}); !errors.Is(err, context.Canceled) || !watch.seen {
			t.Errorf("in-progress cancellation = %v, destination observed: %v", err, watch.seen)
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("canceled export left destination: %v", err)
		}
	})
	t.Run("legacy", func(t *testing.T) {
		for _, legacy := range []func(string, string) error{
			store.Export,
			func(path, format string) error { return store.ExportContext(context.Background(), path, format) },
			func(path, format string) error {
				return store.ExportSpreadsheetContext(context.Background(), path, format)
			},
		} {
			path := filepath.Join(t.TempDir(), "legacy.xlsx")
			if err := legacy(path, "xlsx"); err == nil || err.Error() != "format must be csv or json" {
				t.Errorf("legacy error = %v", err)
			}
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("legacy export left destination: %v", err)
			}
		}
	})
}

type cancelWhenXLSXCreated struct {
	context.Context
	path   string
	cancel context.CancelFunc
	seen   bool
}

func (c *cancelWhenXLSXCreated) Err() error {
	if !c.seen {
		if _, err := os.Stat(c.path); err == nil {
			c.seen = true
			c.cancel()
		}
	}
	return c.Context.Err()
}

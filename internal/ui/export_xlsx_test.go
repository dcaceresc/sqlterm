package ui

import (
	"archive/zip"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"sqlterm/internal/config"
	"sqlterm/internal/result"
)

func TestExportXLSXFromGrid(t *testing.T) {
	all := [][]string{{"other", "=text"}, {"third", "@第三\nline"}, {"second", "two"}, {"first", "one"}}
	for _, tc := range []struct {
		name                 string
		mark, toggle, cancel bool
		want                 [][]string
	}{
		{"selection", true, false, false, [][]string{{"=text"}, {"@第三\nline"}, {"two"}}},
		{"entire result by default", false, false, false, all},
		{"change scope to entire result", true, true, false, all},
		{"cancel picker", true, false, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := runningUI(t, config.Config{})
			dir := t.TempDir()
			s, err := result.New([]string{"other", "=text"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(s.Close)
			for _, row := range [][]result.Cell{
				{{Text: "first"}, {Text: "one"}},
				{{Text: "second"}, {Text: "two"}},
				{{Text: "third"}, {Text: "@第三\nline"}},
			} {
				if err := s.Append(row); err != nil {
					t.Fatal(err)
				}
			}
			u.app.QueueUpdateDraw(func() {
				u.active().Results = []*result.Store{s}
				u.active().Order = []int{2, 1, 0}
				u.showResults()
				u.app.SetFocus(u.table)
				u.table.Select(2, 2)
				if tc.mark {
					dispatchModified(u, tcell.KeyUp, 0, tcell.ModShift)
				}
				dispatchModified(u, tcell.KeyRune, 'e', 0)
				menu, ok := u.dialogContent.(*tview.List)
				if !ok {
					t.Errorf("export menu = %T", u.dialogContent)
					return
				}
				index := 3
				if tc.mark {
					index++
				}
				if menu.GetItemCount() <= index {
					t.Error("Excel option missing")
					return
				}
				if tc.toggle {
					menu.SetCurrentItem(3)
					fileDialogKey(u, tcell.KeyEnter)
				}
				menu.SetCurrentItem(index)
				fileDialogKey(u, tcell.KeyEnter)
				picker, ok := u.dialogContent.(*sqlFileDialog)
				if !ok {
					t.Errorf("file picker = %T", u.dialogContent)
					return
				}
				picker.navigate(dir)
				if tc.cancel {
					fileDialogKey(u, tcell.KeyEscape)
				} else {
					picker.submit()
				}
			})
			if t.Failed() {
				return
			}
			waitUI(t, u, func() bool { return !u.active().Busy })
			if tc.cancel {
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("canceled export destination = %v, %v", entries, err)
				}
				return
			}
			workbook, err := zip.OpenReader(filepath.Join(dir, "results.xlsx"))
			if err != nil {
				t.Fatal(err)
			}
			defer workbook.Close()
			f, err := workbook.Open("xl/worksheets/sheet1.xml")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			var sheet struct {
				Rows []struct {
					Cells []struct {
						Value string `xml:"is>t"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			if err := xml.NewDecoder(f).Decode(&sheet); err != nil {
				t.Fatal(err)
			}
			var got [][]string
			for _, row := range sheet.Rows {
				var values []string
				for _, cell := range row.Cells {
					values = append(values, cell.Value)
				}
				got = append(got, values)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("worksheet rows = %q, want %q", got, tc.want)
			}
		})
	}
}

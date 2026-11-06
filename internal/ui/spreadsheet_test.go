package ui

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/result"
)

func TestSpreadsheetSafeCopyDefaultsAndLiteralOptOut(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{" =header", "plain"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range [][]result.Cell{
		{{Text: "=1+1"}, {Text: "normal"}},
		{{Text: "\t+cmd"}, {Text: "-42"}},
		{{Text: "\x00@HYPERLINK"}, {Text: "ñ"}},
	} {
		if err = s.Append(row); err != nil {
			t.Fatal(err)
		}
	}
	copied := make(chan string, 8)
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Results = []*result.Store{s}
		u.showResults()
		u.app.SetFocus(u.table)
		u.table.Select(1, 0)
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		u.copyResults("cell", false)
	})
	if got := receiveCopy(t, copied); got != "'=1+1" {
		t.Fatalf("safe cell copy = %q", got)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	u.app.QueueUpdateDraw(func() { u.copyResults("all", true) })
	got := receiveCopy(t, copied)
	reader := csv.NewReader(strings.NewReader(got))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0][0] != "' =header" || rows[1][0] != "'=1+1" || rows[2][0] != "'\t+cmd" || rows[2][1] != "'-42" || rows[3][0] != "'\\0@HYPERLINK" {
		t.Fatalf("safe TSV = %#v", rows)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	u.app.QueueUpdateDraw(func() {
		u.table.Select(2, 0)
		u.copyResults("row", false)
	})
	got = receiveCopy(t, copied)
	reader = csv.NewReader(strings.NewReader(got))
	reader.Comma = '\t'
	rows, err = reader.ReadAll()
	if err != nil || rows[0][0] != "'\t+cmd" || rows[0][1] != "'-42" {
		t.Fatalf("safe row copy = %#v, %v", rows, err)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	u.app.QueueUpdateDraw(func() {
		u.active().CopyAnchor = 1
		u.table.Select(2, 0)
		u.copyResults("selection", true)
	})
	got = receiveCopy(t, copied)
	reader = csv.NewReader(strings.NewReader(got))
	reader.Comma = '\t'
	rows, err = reader.ReadAll()
	if err != nil || rows[0][0] != "' =header" || rows[1][0] != "'=1+1" || rows[2][0] != "'\t+cmd" {
		t.Fatalf("safe selection copy = %#v, %v", rows, err)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	u.app.QueueUpdateDraw(func() { u.copyResultsLiteral("all", true) })
	got = receiveCopy(t, copied)
	reader = csv.NewReader(strings.NewReader(got))
	reader.Comma = '\t'
	rows, err = reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0][0] != " =header" || rows[1][0] != "=1+1" || rows[2][0] != "\t+cmd" || rows[2][1] != "-42" || strings.Contains(got, "'=1+1") {
		t.Fatalf("literal TSV changed: %#v", rows)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	u.app.QueueUpdateDraw(func() {
		u.viewCell(1, 0)
		panel := u.dialogContent.(*tview.Flex)
		buttons := panel.GetItem(1).(*tview.Form)
		buttons.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	if got = receiveCopy(t, copied); got != "'=1+1" {
		t.Fatalf("safe inspector copy = %q", got)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	u.app.QueueUpdateDraw(func() {
		panel := u.dialogContent.(*tview.Flex)
		buttons := panel.GetItem(1).(*tview.Form)
		buttons.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	if got = receiveCopy(t, copied); got != "=1+1" {
		t.Fatalf("literal inspector copy = %q", got)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	row, err := s.Row(0)
	if err != nil || row[0].Text != "=1+1" {
		t.Fatal("copy mutated stored results", err)
	}
}

func TestSpreadsheetMenusFitAndExposeSafeAndLiteralChoices(t *testing.T) {
	u, screen := modeTestUI(t)
	s, err := result.New([]string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	u.active().Results = []*result.Store{s}
	u.showResults()
	u.app.SetFocus(u.table)
	u.copyMenu()
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		screen.SetSize(size[0], size[1])
		u.app.ForceDraw()
		text := screenText(screen)
		if !strings.Contains(text, "Spreadsheet-safe") || !strings.Contains(text, "Literal") {
			t.Fatalf("%dx%d copy menu lacks safe/literal choices: %s", size[0], size[1], text)
		}
	}
	u.dismiss()
	u.export()
	if _, ok := u.dialogContent.(*tview.List); !ok {
		t.Fatalf("export must first choose safe CSV, literal CSV, or JSON; got %T", u.dialogContent)
	}
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		screen.SetSize(size[0], size[1])
		u.app.ForceDraw()
		text := screenText(screen)
		for _, label := range []string{"Spreadsheet-safe CSV", "Literal CSV", "JSON"} {
			if !strings.Contains(text, label) {
				t.Fatalf("%dx%d export menu missing %q: %s", size[0], size[1], label, text)
			}
		}
	}
}

func receiveCopy(t *testing.T, copied <-chan string) string {
	t.Helper()
	select {
	case text := <-copied:
		return text
	case <-time.After(3 * time.Second):
		t.Fatal("copy timed out")
		return ""
	}
}

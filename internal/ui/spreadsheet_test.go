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

// copyMenuIndex and activateCopyMenuItem keep test assertions outside the
// application goroutine: t.Fatal inside QueueUpdateDraw would kill the event loop.
func copyMenuIndex(u *UI, prefix string) int {
	if u.copyPopup == nil {
		return -1
	}
	for i := 0; i < u.copyPopup.GetItemCount(); i++ {
		main, _ := u.copyPopup.GetItemText(i)
		if strings.HasPrefix(main, prefix) {
			return i
		}
	}
	return -1
}

// activateCopyMenuItem uses the list's own input handler so the test exercises
// the same path as a keyboard user selecting the entry.
func activateCopyMenuItem(u *UI, index int) {
	u.copyPopup.SetCurrentItem(index)
	u.copyPopup.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
}

func TestCopyMenuOffersCellWithHeaderAtSupportedSizes(t *testing.T) {
	u, screen := modeTestUI(t)
	s, err := result.New([]string{"nombre_cliente"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Append([]result.Cell{{Text: "ACME S.A."}}); err != nil {
		t.Fatal(err)
	}
	u.active().Results = []*result.Store{s}
	u.showResults()
	u.app.SetFocus(u.table)
	const label = "Cell with header · column + value"
	for _, literal := range []bool{false, true} {
		u.copyMenuMode(literal)
		cell, withHeader := copyMenuIndex(u, "Cell · 1 value"), copyMenuIndex(u, "Cell with header")
		if cell < 0 || withHeader != cell+1 {
			t.Fatalf("literal=%v: cell with header at %d, single cell at %d", literal, withHeader, cell)
		}
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			screen.SetSize(size[0], size[1])
			u.app.ForceDraw()
			text := screenText(screen)
			if !strings.Contains(text, label) || !strings.Contains(text, "Cell · 1 value") {
				t.Fatalf("literal=%v %dx%d copy menu lacks the cell with header entry: %s", literal, size[0], size[1], text)
			}
		}
		u.dismiss()
	}
	// The new entry follows the existing English-key/Spanish-translation pattern.
	u.cfg.Language = "es"
	u.copyMenuMode(false)
	screen.SetSize(80, 24)
	u.app.ForceDraw()
	if text := screenText(screen); !strings.Contains(text, "Celda con cabecera · columna + valor") {
		t.Fatalf("80x24 Spanish copy menu lacks the translated entry: %s", text)
	}
	u.dismiss()
	u.cfg.Language = ""
}

func TestCopyMenuCellWithHeaderCopiesHeaderAndValue(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"nombre_cliente", "=importe"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for _, row := range [][]result.Cell{
		{{Text: "ACME S.A."}, {Text: "=1+1"}},
		{{Text: "a\tb\nc\"d"}, {Null: true}},
		{{Text: "\x00@cmd"}, {Text: "plain"}},
	} {
		if err = s.Append(row); err != nil {
			t.Fatal(err)
		}
	}
	copied := make(chan string, 8)
	copyThrough := func(literal bool, row, col int, want string) {
		t.Helper()
		index := -1
		u.app.QueueUpdateDraw(func() {
			u.active().Results = []*result.Store{s}
			u.showResults()
			u.app.SetFocus(u.table)
			u.table.Select(row, col)
			u.writeClipboard = func(text string) error { copied <- text; return nil }
			u.copyMenuMode(literal)
			if index = copyMenuIndex(u, "Cell with header"); index >= 0 {
				activateCopyMenuItem(u, index)
			}
		})
		if index < 0 {
			t.Fatalf("literal=%v copy menu lacks the cell with header entry", literal)
		}
		if got := receiveCopy(t, copied); got != want {
			t.Fatalf("literal=%v cell with header = %q, want %q", literal, got, want)
		}
		waitUI(t, u, func() bool { return !u.active().Busy })
	}
	copyThrough(false, 1, 0, "nombre_cliente\tACME S.A.\n")
	waitUI(t, u, func() bool { return u.active().Status == "Copied 1 cell with header to clipboard" })
	copyThrough(false, 1, 1, "'=importe\t'=1+1\n")
	copyThrough(true, 1, 1, "=importe\t=1+1\n")
	copyThrough(false, 2, 0, "nombre_cliente\t\"a\tb\nc\"\"d\"\n")
	copyThrough(false, 2, 1, "'=importe\t\\N\n")
	copyThrough(true, 2, 1, "=importe\t\\N\n")
	// Embedded NUL keeps going through the clipboardtext boundary.
	copyThrough(false, 3, 0, "nombre_cliente\t'\\0@cmd\n")
	copyThrough(true, 3, 0, "nombre_cliente\t\\0@cmd\n")

	row, err := s.Row(0)
	if err != nil || row[0].Text != "ACME S.A." || row[1].Text != "=1+1" {
		t.Fatal("copy mutated stored results", err)
	}
}

func TestCellWithHeaderCopyRefusesWhileExecutionIsBusy(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"nombre_cliente"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Append([]result.Cell{{Text: "ACME S.A."}}); err != nil {
		t.Fatal(err)
	}
	copied := make(chan string, 1)
	status := ""
	busy := false
	index := -1
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Results = []*result.Store{s}
		u.showResults()
		u.app.SetFocus(u.table)
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		tab.Busy = true
		u.copyMenu()
		if index = copyMenuIndex(u, "Cell with header"); index >= 0 {
			activateCopyMenuItem(u, index)
		}
		status, busy = tab.Status, tab.Busy
	})
	if index < 0 {
		t.Fatal("copy menu lacks the cell with header entry")
	}
	if status != "Wait until execution finishes" || !busy {
		t.Fatalf("busy copy status = %q, busy = %v", status, busy)
	}
	select {
	case got := <-copied:
		t.Fatalf("busy execution copied %q", got)
	default:
	}
	u.app.QueueUpdateDraw(func() { u.active().Busy = false })
}

func inspectorPanel(t *testing.T, u *UI) *tview.Flex {
	t.Helper()
	panel, ok := u.dialogContent.(*tview.Flex)
	if !ok {
		t.Fatalf("inspector content = %T, want a flex panel", u.dialogContent)
	}
	return panel
}

func TestInspectorOffersColumnNameAndValueCopyAtSupportedSizes(t *testing.T) {
	u, screen := modeTestUI(t)
	s, err := result.New([]string{"nombre_cliente"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Append([]result.Cell{{Text: "ACME S.A."}}); err != nil {
		t.Fatal(err)
	}
	u.active().Results = []*result.Store{s}
	u.showResults()
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		screen.SetSize(size[0], size[1])
		u.viewCell(1, 0)
		u.app.ForceDraw()
		panel := inspectorPanel(t, u)
		if panel.GetItemCount() != 3 {
			t.Fatalf("%dx%d inspector items = %d, want value buttons plus column name buttons", size[0], size[1], panel.GetItemCount())
		}
		header, ok := panel.GetItem(2).(*tview.Form)
		if !ok || header.GetButtonCount() != 2 {
			t.Fatalf("%dx%d inspector lacks the column name copy form: %T", size[0], size[1], panel.GetItem(2))
		}
		if !strings.Contains(header.GetButton(0).GetLabel(), "Copy column name and value") {
			t.Fatalf("%dx%d safe column name copy = %q", size[0], size[1], header.GetButton(0).GetLabel())
		}
		if !strings.Contains(header.GetButton(1).GetLabel(), "not safe for untrusted spreadsheet data") {
			t.Fatalf("%dx%d literal column name copy = %q", size[0], size[1], header.GetButton(1).GetLabel())
		}
		if text := screenText(screen); !strings.Contains(text, "Copy column name and value") {
			t.Fatalf("%dx%d inspector does not show the column name copy:\n%s", size[0], size[1], text)
		}
		// Narrow terminals wrap the literal variant; focusing it must reveal it.
		header.SetFocus(1)
		u.app.SetFocus(header)
		u.app.ForceDraw()
		if text := screenText(screen); !strings.Contains(text, "Copy name and value literally · not safe for untrusted spreadsheet data") {
			t.Fatalf("%dx%d focused literal column name copy is not visible:\n%s", size[0], size[1], text)
		}
		u.dismiss()
	}
	// The new actions follow the existing English-key/Spanish-translation pattern.
	u.cfg.Language = "es"
	screen.SetSize(80, 24)
	u.viewCell(1, 0)
	u.app.ForceDraw()
	if text := screenText(screen); !strings.Contains(text, "Copiar nombre de columna y valor") {
		t.Fatalf("80x24 Spanish inspector lacks the translated action:\n%s", text)
	}
	header := inspectorPanel(t, u).GetItem(2).(*tview.Form)
	header.SetFocus(1)
	u.app.SetFocus(header)
	u.app.ForceDraw()
	if text := screenText(screen); !strings.Contains(text, "Copiar nombre y valor literal") {
		t.Fatalf("80x24 Spanish inspector lacks the translated literal action:\n%s", text)
	}
	u.dismiss()
	u.cfg.Language = ""
}

func TestInspectorColumnNameAndValueCopyIsSafeByDefault(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"nombre_cliente", "=importe"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for _, row := range [][]result.Cell{
		{{Text: "ACME S.A."}, {Text: "=1+1"}},
		{{Text: "a\tb"}, {Null: true}},
		{{Text: "\x00@cmd"}, {Text: "plain"}},
	} {
		if err = s.Append(row); err != nil {
			t.Fatal(err)
		}
	}
	copied := make(chan string, 8)
	copyButton := func(row, col, button int, want string) {
		t.Helper()
		u.app.QueueUpdateDraw(func() {
			tab := u.active()
			tab.Results = []*result.Store{s}
			u.showResults()
			u.app.SetFocus(u.table)
			u.writeClipboard = func(text string) error { copied <- text; return nil }
			u.viewCell(row, col)
			header := inspectorPanel(t, u).GetItem(2).(*tview.Form)
			header.GetButton(button).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		})
		if got := receiveCopy(t, copied); got != want {
			t.Fatalf("row %d column %d button %d = %q, want %q", row, col, button, got, want)
		}
		waitUI(t, u, func() bool { return !u.active().Busy })
	}
	copyButton(1, 0, 0, "nombre_cliente\tACME S.A.\n")
	waitUI(t, u, func() bool { return u.active().Status == "Copied 1 cell with header to clipboard" })
	copyButton(1, 1, 0, "'=importe\t'=1+1\n")
	copyButton(1, 1, 1, "=importe\t=1+1\n")
	copyButton(2, 1, 0, "'=importe\t\\N\n")
	copyButton(2, 0, 0, "nombre_cliente\t\"a\tb\"\n")
	// The header scope keeps the clipboardtext NUL boundary.
	copyButton(3, 0, 0, "nombre_cliente\t'\\0@cmd\n")

	// The value-only buttons keep working exactly as before.
	value := ""
	u.app.QueueUpdateDraw(func() {
		u.viewCell(1, 1)
		valueButtons := inspectorPanel(t, u).GetItem(1).(*tview.Form)
		valueButtons.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	})
	v := receiveCopy(t, copied)
	value = v
	if value != "'=1+1" {
		t.Fatalf("safe value copy from the inspector = %q", value)
	}
	waitUI(t, u, func() bool { return !u.active().Busy })

	row, err := s.Row(0)
	if err != nil || row[1].Text != "=1+1" {
		t.Fatal("inspector copy mutated stored results", err)
	}
}

func TestInspectorColumnNameAndValueCopyRefusesWhileBusy(t *testing.T) {
	u := runningUI(t, config.Config{})
	s, err := result.New([]string{"nombre_cliente"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Append([]result.Cell{{Text: "ACME S.A."}}); err != nil {
		t.Fatal(err)
	}
	copied := make(chan string, 1)
	status := ""
	busy := false
	u.app.QueueUpdateDraw(func() {
		tab := u.active()
		tab.Results = []*result.Store{s}
		u.showResults()
		u.writeClipboard = func(text string) error { copied <- text; return nil }
		tab.Busy = true
		u.viewCell(1, 0)
		header := inspectorPanel(t, u).GetItem(2).(*tview.Form)
		header.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
		status, busy = tab.Status, tab.Busy
	})
	if status != "Wait until execution finishes" || !busy {
		t.Fatalf("busy inspector copy status = %q, busy = %v", status, busy)
	}
	select {
	case got := <-copied:
		t.Fatalf("busy execution copied %q", got)
	default:
	}
	u.app.QueueUpdateDraw(func() { u.active().Busy = false })
}

func TestInspectorWithoutCellContextHasNoColumnNameCopy(t *testing.T) {
	u, screen := modeTestUI(t)
	u.showMessages()
	u.app.ForceDraw()
	panel := inspectorPanel(t, u)
	if panel.GetItemCount() != 2 {
		t.Fatalf("messages inspector items = %d, want only the view and value buttons", panel.GetItemCount())
	}
	if text := screenText(screen); strings.Contains(text, "Copy column name and value") {
		t.Fatalf("messages inspector offers a cell copy without cell context:\n%s", text)
	}
	u.dismiss()
}

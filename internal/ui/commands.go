package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type command struct {
	Name, Detail, Section string
	Key                   rune
	Run                   func()
	Available             func() bool
}

func (u *UI) commands() []command {
	connected := func() bool { return u.active().Session != nil && !u.active().SelectingDatabase }
	results := func() bool { return u.currentStore() != nil }
	object := func() bool { _, ok := u.selectedObject(); return ok }
	selectable := func() bool { o, ok := u.selectedObject(); return ok && (o.Kind == "table" || o.Kind == "view") }
	return []command{
		{"Word wrap", "Alt+Z · toggle visual wrapping of long SQL lines", "Editor", 0, func() { u.toggleEditorOption(true) }, nil},
		{"Line numbers", "Show / hide SQL line numbers", "Editor", 0, func() { u.toggleEditorOption(false) }, nil},
		{"Comment selection", "Comment / uncomment selection or current line", "Editor", 0, func() { u.commentSQL(false) }, nil},
		{"Comment whole document", "Comment / uncomment each SQL line", "Editor", 0, func() { u.commentSQL(true) }, nil},
		{"Language / Idioma", "English / Español", "Getting started", 0, u.languageMenu, nil},
		{"Execution confirmations", "Choose Off / production writes / every F5 execution", "Connections", 0, u.executionPolicyMenu, nil},
		{"Connections", "F2 · profiles and environment folders", "Connections", 'c', u.chooseConnection, nil},
		{"Search objects", "F6 · filter by name, schema or kind", "Objects", 'f', func() { u.app.SetFocus(u.search) }, nil},
		{"Execute SQL", "F5 · selection or complete script; destination in top bar", "Editor", 'x', u.execute, connected},
		{"Invoke procedure / function", "F9 · parameter form", "Objects", 'p', u.invokeSelected, connected},
		{"Apply definition", "Review SQL before applying changes to the server", "Editor", 'a', u.applyDefinition, connected},
		{"Refresh catalog", "Retry loading database objects", "Objects", 'r', u.refreshCatalog, connected},
		{"New session", "Ctrl+N · independent database connection", "Connections", 'n', func() { u.newTab(u.active()) }, nil},
		{"Save SQL", "Ctrl+S · save directly to the current path", "Editor", 'w', u.saveFile, nil},
		{"Save SQL as", "Ctrl+Shift+S · choose another folder or file name", "Editor", 0, u.saveFileAs, nil},
		{"Open SQL", "Ctrl+O · load file", "Editor", 'o', u.openFile, nil},
		{"Close tab", "Checks unsaved SQL", "Connections", 'q', u.closeTab, nil},
		{"History", "Opt-in local SQL history", "Editor", 'h', u.showHistory, nil},
		{"Export results", "e · CSV / JSON / Excel of the selection or entire result", "Results", 'e', u.export, results},
		{"Change database", "F4 · new connection; current SQL preserved", "Connections", 'b', u.switchDatabase, connected},
		{"Disconnect", "Close this tab's connection", "Connections", 'd', u.disconnect, connected},
		{"New routine template", "Create a procedure in the editor", "Objects", 't', u.template, connected},
		{"New table template", "Create a table in the editor; review and execute with F5", "Objects", 0, u.tableTemplate, connected},
		{"Themes", "F3 · / search, Enter save, Esc restore", "Connections", 'T', u.chooseTheme, nil},
		{"Help", "F1 · context and shortcut search", "Getting started", '?', u.showHelp, nil},
		{"Select all results", "Ctrl+A in results · mark every data cell", "Results", 0, u.selectAllGrid, u.canSelectGrid},
		{"Copy results", "Ctrl+C or y · copy the selection or focused cell, row or column", "Results", 0, func() {
			if u.active().Busy {
				u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
				return
			}
			u.copyGridSelection()
		}, results},
		{"Copy with Headers", "Copy data as TSV with column headers", "Results", 0, func() { u.copyGridHeaders(false) }, results},
		{"Copy Headers", "Copy only the selected column headers", "Results", 0, func() { u.copyGridHeaders(true) }, results},
		{"Inspect cell", "i in results · inspect full value", "Results", 0, func() { r, c := u.table.GetSelection(); u.viewCell(r, c-1) }, results},
		{"View definition", "Inspect selected object without executing it", "Objects", 0, func() {
			if o, ok := u.selectedObject(); ok {
				u.openObject(o)
			}
		}, object},
		{"Insert object name", "Quoted identifier at editor cursor", "Objects", 0, func() { u.insertObjectName(false) }, object},
		{"Generate SELECT", "Insert SELECT; review and run with F5", "Objects", 0, func() { u.insertObjectName(true) }, selectable},
		{"Grouped / flat catalog", "Group by schema and object kind", "Objects", 0, u.toggleCatalog, nil},
		{"Cancel operation", "F10 · wait for driver confirmation", "Getting started", 0, u.cancelOperation, func() bool { return u.active().Busy }},
		{"Technical messages", "Server messages and execution errors · copyable", "Results", 0, u.showMessages, nil},
		{"Quit", "Space q q · keeps SQL tabs for next launch", "Getting started", 0, u.quit, nil},
	}
}

func (u *UI) commandPalette() {
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(u.tr(" Actions · / search "))
	search := tview.NewInputField().SetLabel(u.tr("Search: "))
	rebuild := func(filter string) {
		list.Clear()
		for _, item := range u.commands() {
			c := item
			if !fuzzy(strings.ToLower(filter), strings.ToLower(u.tr(c.Name)+" "+u.tr(c.Detail)+" "+u.tr(c.Section))) {
				continue
			}
			detail := u.tr(c.Detail)
			if c.Available != nil && !c.Available() {
				detail = u.tr("Unavailable in this context") + " · " + detail
			}
			list.AddItem(u.tr(c.Name), detail, c.Key, func() {
				u.dismiss()
				if c.Available != nil && !c.Available() {
					u.message(u.tr("Unavailable in this context: ") + c.Name)
					return
				}
				c.Run()
			})
		}
	}
	search.SetChangedFunc(rebuild)
	rebuild("")
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 1, 0, false).AddItem(list, 0, 1, true)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		if k.Key() == tcell.KeyTab || (search.HasFocus() && (k.Key() == tcell.KeyEnter || k.Key() == tcell.KeyDown)) {
			if search.HasFocus() {
				u.app.SetFocus(list)
			} else {
				u.app.SetFocus(search)
			}
			return nil
		}
		if list.HasFocus() && k.Key() == tcell.KeyRune && k.Rune() == '/' {
			u.app.SetFocus(search)
			return nil
		}
		return k
	})
	u.popup(panel, 100, 24)
}

func (u *UI) searchableHelp() {
	context := "Editor"
	if u.table.HasFocus() {
		context = "Results"
	} else if u.objects.HasFocus() || u.search.HasFocus() || u.kind.HasFocus() {
		context = "Objects"
	}
	sections := []string{"Getting started", "Editor", "Objects", "Results", "Connections", "All shortcuts"}
	filter := tview.NewInputField().SetLabel(u.tr("Search help: "))
	section := tview.NewDropDown().SetLabel(u.tr("Section: "))
	text := tview.NewTextView().SetWrap(true).SetScrollable(true)
	text.SetBorder(true).SetTitle(u.tr(" Help · Tab navigate · Space commands "))
	rebuild := func() {
		i, _ := section.GetCurrentOption()
		selected := "Getting started"
		if i >= 0 && i < len(sections) {
			selected = sections[i]
		}
		needle := strings.ToLower(filter.GetText())
		var lines []string
		intro := map[string]string{"Getting started": "F2 connect → F6 find objects → F7 edit → F5 execute → F8 inspect/copy. Destination stays in the top bar.\nEach tab owns its connection. No read-only guarantee. Use BEGIN / COMMIT / ROLLBACK explicitly.", "Editor": "Space in the focused NORMAL editor opens hierarchical commands; Backspace returns to the parent, Esc cancels. INSERT Space keeps typing.\n i INSERT · Esc NORMAL · v VISUAL · h/j/k/l · w/b · 0/$ · gg/G · yy/dd · y/d · p · u · Ctrl+R\nCtrl+Space IntelliSense; Tab/Enter accept. F5 runs selection or full script. * means unsaved changes.\nCtrl+C copies selected SQL to the system clipboard; without selection it shows a notice and keeps the clipboard. Works during query execution; preserves SQL, selection and undo. NUL is escaped as \\0 in clipboard text only.\nCtrl+O or Space f e browses folders; Space f n starts a new session. Ctrl+S saves directly to the current path. Ctrl+Shift+S or Space f S chooses another destination. If the terminal reports Ctrl+Shift+S as plain Ctrl+S, it performs a normal save.\nTab/Shift+Tab changes fields; Enter selects; Esc cancels the file dialog.\nUse gcc in NORMAL for the current line, gc followed by a motion, or gc in VISUAL for the selection. Full-line -- comments use the theme muted color; selection colors take precedence. Does not execute or save SQL.\nAlt+Z toggles word wrap. Logical SQL line numbers stay stable. Both options are saved and available in Space u w/l.", "Objects": "Search by contiguous text in the name, schema or kind; separate terms with spaces. Click or Enter opens definitions, including CREATE TABLE for tables, without executing SQL. Group rows expand/collapse.\nSpace c i/q/p: insert quoted name, generate SELECT or invoke. Space d r refreshes; Space s g toggles flat catalog. New table template prepares CREATE TABLE in the editor; review and execute with F5.", "Results": "Rows arrive continuously. The grid stays on the first rows while the received count and elapsed time update. F10 cancels and keeps received rows.\nClick or Enter marks a cell, column or row. Shift+arrows or drag marks a rectangle; Ctrl+A marks all data; Esc clears the mark. v marks a row range for Y. Ctrl+C or y copies the selection, or the focused cell, row or column if nothing is marked. The Actions palette offers Copy with Headers and Copy Headers. i opens the cell inspector. NULL is SQL NULL; empty string is \"\".\n/ find · s/S lexical sort of received rows · [/] sets · e export · Y copy rows with headers · r/R row · c cell\nCtrl+C or y copies without headers or row numbers. Rows and columns copy as TSV. Export defaults to the marked selection or the entire result and uses the displayed order.", "Connections": "F2 profiles and folders · / search · + add · e edit · n folder · r rename · d delete empty folder\nUse * on a profile or the ★ Favorite button. Favorites appear in the top section across folders and remain in their original folders.\nF2, F3 and F4 open on the list. / filters; Esc clears search or closes the menu.\nF4 changes the database. New connection replaces old session only after success; SQL is preserved. Transactions, variables and temporary tables do not carry across.\nF3 themes: arrows preview, Enter saves, Esc restores. Space q q quits; Ctrl+C copies or cancels a dialog.", "All shortcuts": u.legacyHelp()}
		intro["Editor"] += "\nThe Ex prompt is disabled. Use F1 for help, Space b d to close a tab, and Space q q to quit. In INSERT, : is ordinary text."
		intro["Editor"] += "\nV selects the whole logical line; j/k extend by lines. gg goes to file start; ggVG selects the whole file. Ctrl+C keeps the current mode and selection; yy stays NORMAL. y/d copy/cut the selection and return NORMAL."
		intro["Editor"] += "\nThe SQL pane uses isolated Neovim 0.11+ (nvim in PATH), not the personal LazyVim config. Native v includes the current character; SQLTerm global shortcuts take priority. Windows Terminal bracketed paste is literal and preserves Normal mode. Blockwise SQL execution is refused. Missing/exited Neovim never falls back silently; stale save/execute is blocked."
		intro["Editor"] += "\nSQL Server: EXEC procedure followed by a space lists parameter names/types; @ filters, Tab/Enter inserts @name = . Appears again after an argument comma; does not execute SQL."
		intro["Results"] += "\nExcel (.xlsx) stores headers and values as text, not formulas, and leaves NULL blank.\nThe Results heading indicates when results are partial or belong to earlier SQL. The destination stays in the top bar."
		if u.cfg.Language == "es" {
			intro = u.spanishHelp()
		}
		for _, line := range strings.Split(u.tr(intro[selected]), "\n") {
			if needle == "" || strings.Contains(strings.ToLower(line), needle) {
				lines = append(lines, line)
			}
		}
		for _, c := range u.commands() {
			if selected != "All shortcuts" && c.Section != selected {
				continue
			}
			line := u.tr(c.Name) + " — " + u.tr(c.Detail)
			if needle == "" || strings.Contains(strings.ToLower(line), needle) {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			lines = []string{u.tr("No matching help. Clear the search or choose All shortcuts.")}
		}
		text.SetText(strings.Join(lines, "\n\n"))
	}
	displaySections := make([]string, len(sections))
	for i, s := range sections {
		displaySections[i] = u.tr(s)
	}
	section.SetOptions(displaySections, func(string, int) { rebuild() })
	for i, s := range sections {
		if s == context {
			section.SetCurrentOption(i)
		}
	}
	filter.SetChangedFunc(func(string) { rebuild() })
	rebuild()
	panel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(filter, 1, 0, false).AddItem(section, 1, 0, false).AddItem(text, 0, 1, true)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		if k.Key() == tcell.KeyTab {
			if filter.HasFocus() {
				u.app.SetFocus(section)
			} else if section.HasFocus() {
				u.app.SetFocus(text)
			} else {
				u.app.SetFocus(filter)
			}
			return nil
		}
		return k
	})
	u.popup(panel, 110, 30)
}

func (u *UI) showMessages() {
	text := strings.Join(u.active().Messages, "\n")
	if text == "" {
		text = u.tr("No technical messages")
	}
	u.inspectText(u.tr("Execution messages"), text)
}

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
		{"Comment selection", "Alt+/ · comment / uncomment each selected line", "Editor", 0, func() { u.commentSQL(false) }, nil},
		{"Comment whole document", "Comment / uncomment each SQL line", "Editor", 0, func() { u.commentSQL(true) }, nil},
		{"Language / Idioma", "English / Español", "Getting started", 0, u.languageMenu, nil},
		{"Execution confirmations", "Choose Off / production writes / every F5 execution", "Connections", 0, u.executionPolicyMenu, nil},
		{"Result block size", "Rows fetched per scroll-driven block (1–100000)", "Results", 0, u.resultBlockSizePrompt, nil},
		{"Connections", "F2 · profiles and environment folders", "Connections", 'c', u.chooseConnection, nil},
		{"Search objects", "F6 · filter by name, schema or kind", "Objects", 'f', func() { u.app.SetFocus(u.search) }, nil},
		{"Execute SQL", "F5 · selection or complete script; destination in top bar", "Editor", 'x', u.execute, connected},
		{"Invoke procedure / function", "F9 · parameter form", "Objects", 'p', u.invokeSelected, connected},
		{"Apply definition", "Review SQL before applying changes to the server", "Editor", 'a', u.applyDefinition, connected},
		{"Refresh catalog", "Retry loading database objects", "Objects", 'r', u.refreshCatalog, connected},
		{"New session", "Ctrl+N · independent database connection", "Connections", 'n', func() { u.newTab(u.active()) }, nil},
		{"Save SQL", "Ctrl+S · changes marked *", "Editor", 'w', u.saveFile, nil},
		{"Save SQL as", "Choose a folder and file name", "Editor", 0, u.saveFileAs, nil},
		{"Open SQL", "Ctrl+O · load file", "Editor", 'o', u.openFile, nil},
		{"Close tab", "Checks unsaved SQL", "Connections", 'q', u.closeTab, nil},
		{"History", "Opt-in local SQL history", "Editor", 'h', u.showHistory, nil},
		{"Export results", "CSV / JSON of all received rows", "Results", 'e', u.export, results},
		{"Change database", "F4 · new connection; current SQL preserved", "Connections", 'b', u.switchDatabase, connected},
		{"Disconnect", "Close this tab's connection", "Connections", 'd', u.disconnect, connected},
		{"New routine template", "Create a procedure in the editor", "Objects", 't', u.template, connected},
		{"Themes", "F3 · preview, Enter save, Esc restore", "Connections", 'T', u.chooseTheme, nil},
		{"Help", "F1 · context and shortcut search", "Getting started", '?', u.showHelp, nil},
		{"Copy results", "Ctrl+C · cell, cell with column name, row or received results with headers", "Results", 0, func() {
			if u.active().Busy {
				u.message(u.tr("Press F10 to stop receiving rows before sorting, copying or exporting"))
				return
			}
			u.copyMenu()
		}, results},
		{"Inspect cell", "Enter in results · value plus copy with or without the column name", "Results", 0, func() { r, c := u.table.GetSelection(); u.viewCell(r, c) }, results},
		{"View definition", "Inspect selected object without executing it", "Objects", 0, func() {
			if o, ok := u.selectedObject(); ok {
				u.openObject(o)
			}
		}, object},
		{"Insert object name", "Quoted identifier at editor cursor", "Objects", 0, func() { u.insertObjectName(false) }, object},
		{"Generate SELECT", "Insert SELECT; review and run with F5", "Objects", 0, func() { u.insertObjectName(true) }, selectable},
		{"Grouped / flat catalog", "Group by schema and object kind", "Objects", 0, u.toggleCatalog, nil},
		{"Layout", "F11 · resize, maximize, edit / explore / balanced", "Getting started", 0, u.layoutMenu, nil},
		{"Cancel operation", "F10 · wait for driver confirmation", "Getting started", 0, u.cancelOperation, func() bool { return u.active().Busy }},
		{"Technical messages", "Server messages and execution errors · copyable", "Results", 0, u.showMessages, nil},
		{"Quit", "Ctrl+Q · checks unsaved SQL", "Getting started", 0, u.quit, nil},
	}
}

func (u *UI) commandPalette() {
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(u.tr(" Actions · / search · existing Space shortcuts "))
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
	text.SetBorder(true).SetTitle(u.tr(" Help · Tab navigate · Ctrl+P actions "))
	rebuild := func() {
		i, _ := section.GetCurrentOption()
		selected := "Getting started"
		if i >= 0 && i < len(sections) {
			selected = sections[i]
		}
		needle := strings.ToLower(filter.GetText())
		var lines []string
		intro := map[string]string{"Getting started": "F2 connect → F6 find objects → F7 edit → F5 execute → F8 inspect/copy. Destination stays in the top bar.\nEach tab owns its connection. No read-only guarantee. Use BEGIN / COMMIT / ROLLBACK explicitly.", "Editor": "i INSERT · Esc NORMAL · v VISUAL · h/j/k/l · w/b · 0/$ · gg/G · yy/dd · y/d · p · u · Ctrl+R\nCtrl+Space IntelliSense; Tab/Enter accept. F5 runs selection or full script. * means unsaved changes.\nCtrl+O browses folders; Ctrl+S saves. Ctrl+P → Save SQL as chooses another destination.\nTab/Shift+Tab changes fields; Enter selects; Esc cancels the file dialog.\nAlt+/ toggles -- on each selected line; without selection it only shows a notice. Ctrl+P → Comment whole document toggles every line. Preserves indentation and selection for repeat toggling; one undo. Full-line -- comments use the theme muted color; selection colors take precedence. Does not execute or save SQL.\nAlt+Z toggles word wrap. Logical SQL line numbers stay stable; ↪ marks wrapped continuations. Both options are saved and available in Ctrl+P.", "Objects": "Search name, schema and kind; Enter opens definitions. Group rows expand/collapse.\nCtrl+P: insert quoted name, generate SELECT, invoke, refresh or flat catalog.", "Results": "The first configured block is fetched, then only Down/Ctrl+N/PageDown/End/j or wheel-down near the loaded end requests one more block. F10 stops and keeps received rows so they can be copied or exported.\nA paused result does not block the tab: F5 or F9 cancels the paused read, waits until the driver closes it and releases the connection, then runs the new statement. F4, disconnect, close tab and quit release it the same way. Copying, sorting and exporting still require F10.\nEnter inspects full cell and copies the value, or the column name with the value as one TSV row. NULL is SQL NULL; empty string is \"\".\n/ find · s/S lexical sort of received rows · [/] sets · e export · y/Y copy received rows without/with headers · r/R row · c cell\nCtrl+C opens copy menu; it includes Cell with header: the selected cell as its column name, a tab and its value in a single TSV row (for example nombre_cliente<TAB>ACME S.A.), ready to paste into two adjacent spreadsheet cells. The header is the result column name and follows the displayed order. Rows are TSV; NUL is escaped as \\0. Export includes all received rows.", "Connections": "F2 profiles and folders · / search · + add · e edit · n folder · r rename · d delete empty folder\nUse * on a profile or the ★ Favorite button. Favorites appear in the top section across folders and remain in their original folders.\nF4 searches databases. New connection replaces old session only after success; SQL is preserved. Transactions, variables and temporary tables do not carry across.\nF3 themes: arrows preview, Enter saves, Esc restores. Ctrl+Q quits; Ctrl+C opens copy or cancels dialog.", "All shortcuts": u.legacyHelp()}
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

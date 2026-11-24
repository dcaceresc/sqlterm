package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

const tabCloseLabel = " × "

var headerSingleLine = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ")

type tabHeaderHit struct {
	tab        *Tab
	start, end int
	close      bool
}

func (u *UI) drawTabHeader(width int) {
	u.headerHits = u.headerHits[:0]
	if width <= 0 || len(u.tabs) == 0 {
		u.tabsHeader.SetText("")
		return
	}

	labels := make([]string, len(u.tabs))
	sizes := make([]int, len(u.tabs))
	for i, tab := range u.tabs {
		name := headerSingleLine.Replace(u.queryName(tab))
		if tab.Editor.Area.GetText() != tab.Saved {
			name += "*"
		}
		labels[i] = fmt.Sprintf(" %d:%s", i+1, name)
		sizes[i] = uniseg.StringWidth(labels[i]) + uniseg.StringWidth(tabCloseLabel)
	}

	const contextPrefix = "SQLTerm │ "
	environment, details, color := u.connectionContext()
	environment = headerSingleLine.Replace(environment)
	var context string
	tabWidth := width
	if width > 12 {
		reserve := min(sizes[u.current], min(16, width/2))
		context = clipColumns(contextPrefix+environment+headerSingleLine.Replace(details), width-reserve-1)
		tabWidth -= uniseg.StringWidth(context) + 1
	}

	// La pestaña activa tiene prioridad; solo se agregan vecinas que caben completas.
	first, last := u.current, u.current
	remaining := tabWidth - sizes[u.current]
	for remaining > 0 {
		switch {
		case first > 0 && sizes[first-1] <= remaining:
			first--
			remaining -= sizes[first]
		case last+1 < len(sizes) && sizes[last+1] <= remaining:
			last++
			remaining -= sizes[last]
		default:
			remaining = 0
		}
	}

	var text strings.Builder
	x := 0
	for i := first; i <= last; i++ {
		label, closeLabel := labels[i], tabCloseLabel
		if tabWidth < sizes[i] {
			if tabWidth < 5 {
				closeLabel = ""
				label = fmt.Sprintf("%d", i+1)
			}
			label = clipColumns(label, tabWidth-uniseg.StringWidth(closeLabel))
		}
		if i == u.current {
			fmt.Fprintf(&text, "[%s:%s]", colorTag(u.palette.Background), colorTag(u.palette.Accent))
		}
		text.WriteString(tview.Escape(label))
		start, end := x, x+uniseg.StringWidth(label)
		if strings.HasPrefix(label, " ") {
			start++
		}
		u.headerHits = append(u.headerHits, tabHeaderHit{tab: u.tabs[i], start: start, end: end})
		x = end
		if closeLabel != "" {
			text.WriteString(closeLabel)
			u.headerHits = append(u.headerHits, tabHeaderHit{tab: u.tabs[i], start: x + 1, end: x + 2, close: true})
			x += uniseg.StringWidth(closeLabel)
		}
		if i == u.current {
			text.WriteString("[-:-:-]")
		}
	}
	if context != "" {
		text.WriteString(strings.Repeat(" ", width-x-uniseg.StringWidth(context)))
		text.WriteString("[::b]")
		if rest, ok := strings.CutPrefix(context, contextPrefix); ok {
			env := environment
			suffix, full := strings.CutPrefix(rest, environment)
			if !full {
				env, suffix = rest, ""
			}
			fmt.Fprintf(&text, "%s[%s]%s[-]%s", contextPrefix, colorTag(color), tview.Escape(env), tview.Escape(suffix))
		} else {
			text.WriteString(tview.Escape(context))
		}
		text.WriteString("[-:-:-]")
	}
	u.tabsHeader.SetText(text.String())
}

func (u *UI) tabHeaderMouse(event *tcell.EventMouse, action tview.MouseAction) bool {
	switch action {
	case tview.MouseLeftDown, tview.MouseLeftClick, tview.MouseLeftDoubleClick:
	default:
		return false
	}
	pressed := u.headerPress
	u.headerPress = tabHeaderHit{}
	x, y := event.Position()
	page, _ := u.pages.GetFrontPage()
	if y != 0 || page != "main" || u.dialogContent != nil || u.closing {
		return false
	}
	for _, hit := range u.headerHits {
		if x < hit.start || x >= hit.end {
			continue
		}
		if action == tview.MouseLeftDown {
			u.headerPress = hit
			return true
		}
		// No se activa una pestaña al soltar un clic que empezó sobre un diálogo.
		if pressed != hit {
			return true
		}
		for i, tab := range u.tabs {
			if tab == hit.tab {
				if hit.close {
					u.closeTabTarget(tab)
				} else {
					u.current = i
					u.showTab()
				}
				break
			}
		}
		return true
	}
	return false
}

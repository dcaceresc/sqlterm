package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
	"strings"
)

func clipColumns(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if uniseg.StringWidth(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		if used+g.Width() > width-1 {
			break
		}
		b.WriteString(g.Str())
		used += g.Width()
	}
	b.WriteRune('…')
	return b.String()
}

// El mensaje usa el espacio entre el modo y el tiempo de ejecución.
func modeStatus(p Palette, mode, message, elapsed string, width int) string {
	badge := clipColumns(" "+mode+" ", width)
	remaining := max(0, width-uniseg.StringWidth(badge))
	right := ""
	if elapsed != "" {
		right = clipColumns(" "+elapsed+" ", remaining)
	}
	messageWidth := remaining - uniseg.StringWidth(right)
	message = clipColumns(" "+headerSingleLine.Replace(message), messageWidth)
	padding := strings.Repeat(" ", messageWidth-uniseg.StringWidth(message))
	return fmt.Sprintf("[%s:%s:b]%s[%s:%s:-]%s[%s:%s:b]%s[-:-:-]", colorTag(p.Background), colorTag(p.ModeColor(mode)), tview.Escape(badge), colorTag(p.Foreground), colorTag(p.Surface), tview.Escape(message)+padding, colorTag(p.Foreground), colorTag(p.Border), tview.Escape(right))
}
func (u *UI) refreshModeBorders() {
	for i, t := range u.tabs {
		t.Editor.View.SetActiveColor(u.palette.ModeColor(t.Editor.Mode))
		color := u.palette.Border
		title := u.palette.Muted
		if i == u.current && t.Editor.Area.HasFocus() {
			color = u.palette.ModeColor(t.Editor.Mode)
			title = color
		}
		t.Editor.Area.SetBorderColor(color).SetTitleColor(title)
		prefix := ""
		if i == u.current && t.Editor.Area.HasFocus() {
			prefix = "> "
		}
		scope := "Execute script"
		if selected, _, _ := t.Editor.Area.GetSelection(); selected != "" {
			scope = "Execute selection"
		}
		t.Editor.Area.SetTitle(" " + prefix + u.tr("Editor") + " · " + t.Editor.Mode + " · " + u.tr(scope) + " ")
	}
	objectColor := u.palette.Border
	if u.objects.HasFocus() || u.search.HasFocus() || u.kind.HasFocus() {
		objectColor = u.palette.Accent
	}
	u.objects.SetBorderColor(objectColor)
	prefix := ""
	if objectColor == u.palette.Accent {
		prefix = "> "
	}
	name := "Objects"
	if u.active().SelectingDatabase {
		name = "Databases"
	}
	action := u.tr("Enter definition")
	if u.active().SelectingDatabase {
		action = u.tr("Enter select")
	}
	u.objects.SetTitle(" " + prefix + u.tr(name) + " · " + action + " ")
	style := tcell.StyleDefault.Foreground(u.palette.Muted).Background(u.palette.Surface)
	if objectColor == u.palette.Accent {
		style = tcell.StyleDefault.Foreground(u.palette.SelectedText).Background(u.palette.Selection)
	}
	u.objects.SetSelectedStyle(style)
	resultColor := u.palette.Border
	if u.table.HasFocus() {
		resultColor = u.palette.Accent
	}
	u.table.SetBorderColor(resultColor)
}
func (u *UI) updateCursor(screen tcell.Screen) {
	if len(u.tabs) == 0 {
		return
	}
	t := u.active()
	if state := u.suggestions; state != nil && t.Editor.Area.HasFocus() {
		if front, _ := u.pages.GetFrontPage(); front == "main" {
			state.Popup.Draw(screen)
		}
	}
	if t.Editor.Area.HasFocus() {
		shape := tcell.CursorStyleSteadyBlock
		if t.Editor.Mode == "INSERT" {
			shape = tcell.CursorStyleSteadyBar
		}
		screen.SetCursorStyle(shape, u.palette.ModeColor(t.Editor.Mode))
		return
	}
	screen.SetCursorStyle(tcell.CursorStyleDefault, tcell.ColorReset)
}

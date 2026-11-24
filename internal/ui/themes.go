package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/editor"
)

type Palette struct {
	ID, Name                                                                        string
	Background, Surface, Foreground, Muted, Accent, Border, Selection, SelectedText tcell.Color
	Insert, Visual                                                                  tcell.Color
	Syntax                                                                          editor.SyntaxColors
}

func palette(id, name, bg, surface, fg, muted, accent, border, selection, selected string) Palette {
	p := Palette{ID: id, Name: name, Background: tcell.GetColor(bg), Surface: tcell.GetColor(surface), Foreground: tcell.GetColor(fg), Muted: tcell.GetColor(muted), Accent: tcell.GetColor(accent), Border: tcell.GetColor(border), Selection: tcell.GetColor(selection), SelectedText: tcell.GetColor(selected)}
	insert, visual := "#9ece6a", "#bb9af7"
	switch id {
	case "rose-pine":
		insert, visual = "#9ccfd8", "#ebbcba"
	case "nord":
		insert, visual = "#a3be8c", "#b48ead"
	case "gruvbox":
		insert, visual = "#b8bb26", "#fe8019"
	case "light":
		insert, visual = "#38761d", "#7b2cbf"
	case "classic":
		insert, visual = "#5fd787", "#af87ff"
	case "dracula":
		insert, visual = "#50fa7b", "#ff79c6"
	case "solarized-light":
		insert, visual = "#476400", "#574d9b"
	case "catppuccin-mocha":
		insert, visual = "#a6e3a1", "#cba6f7"
	case "everforest":
		insert, visual = "#a7c080", "#d699b6"
	}
	p.Insert = tcell.GetColor(insert)
	p.Visual = tcell.GetColor(visual)
	// Semantic roles follow each theme rather than forcing dark colors onto Light.
	keyword, str, number := "#7aa2f7", "#9ece6a", "#ff9e64"
	switch id {
	case "rose-pine":
		keyword, str, number = "#c4a7e7", "#f6c177", "#ebbcba"
	case "nord":
		keyword, str, number = "#81a1c1", "#a3be8c", "#b48ead"
	case "gruvbox":
		keyword, str, number = "#fb4934", "#b8bb26", "#d3869b"
	case "light":
		keyword, str, number = "#0000ff", "#a31515", "#098658"
	case "classic":
		keyword, str, number = "#569cd6", "#ce9178", "#b5cea8"
	case "dracula":
		keyword, str, number = "#ff79c6", "#f1fa8c", "#bd93f9"
	case "solarized-light":
		keyword, str, number = "#005f87", "#476400", "#9c3b00"
	case "catppuccin-mocha":
		keyword, str, number = "#cba6f7", "#a6e3a1", "#fab387"
	case "everforest":
		keyword, str, number = "#e67e80", "#a7c080", "#dbbc7f"
	}
	p.Syntax = editor.SyntaxColors{Keyword: tcell.GetColor(keyword), String: tcell.GetColor(str), Number: tcell.GetColor(number)}
	return p
}
func (p Palette) ModeColor(mode string) tcell.Color {
	switch mode {
	case "INSERT":
		return p.Insert
	case "VISUAL":
		return p.Visual
	default:
		return p.Accent
	}
}

var palettes = []Palette{
	palette("tokyo-night", "Tokyo Night", "#1a1b26", "#24283b", "#c0caf5", "#787c99", "#7aa2f7", "#414868", "#33467c", "#c0caf5"),
	palette("rose-pine", "Rosé Pine", "#191724", "#1f1d2e", "#e0def4", "#908caa", "#c4a7e7", "#524f67", "#403d52", "#e0def4"),
	palette("nord", "Nord", "#2e3440", "#3b4252", "#d8dee9", "#a0adbd", "#88c0d0", "#4c566a", "#434c5e", "#eceff4"),
	palette("gruvbox", "Gruvbox", "#282828", "#3c3836", "#ebdbb2", "#a89984", "#fabd2f", "#665c54", "#504945", "#fbf1c7"),
	palette("light", "Light", "#f5f5f5", "#e7e9ef", "#24283b", "#6c7086", "#34548a", "#b3b7c9", "#c4d7ef", "#1a1b26"),
	palette("classic", "Classic", "#000000", "#121212", "#ffffff", "#999999", "#87ceeb", "#5f87af", "#87ceeb", "#000000"),
	palette("dracula", "Dracula", "#282a36", "#343746", "#f8f8f2", "#a7b1d2", "#bd93f9", "#6272a4", "#44475a", "#f8f8f2"),
	palette("solarized-light", "Solarized Light", "#fdf6e3", "#eee8d5", "#40545a", "#51666f", "#005f87", "#8a9797", "#c6dddd", "#263a42"),
	palette("catppuccin-mocha", "Catppuccin Mocha", "#1e1e2e", "#313244", "#cdd6f4", "#a6adc8", "#89b4fa", "#6c7086", "#45475a", "#cdd6f4"),
	palette("everforest", "Everforest", "#2d353b", "#374145", "#d3c6aa", "#a7ad9c", "#7fbbb3", "#6d7b75", "#475258", "#f0e2c3"),
}

func resolveTheme(id string) Palette {
	for _, p := range palettes {
		if p.ID == id {
			return p
		}
	}
	return palettes[0]
}
func colorTag(c tcell.Color) string { return fmt.Sprintf("#%06x", c.Hex()) }
func (u *UI) applyTheme(id string) {
	u.palette = resolveTheme(id)
	if u.pages != nil {
		u.style(u.pages)
	}
	for _, t := range u.tabs {
		u.style(t.Editor.Area)
		if err := t.Editor.SetNeovimTheme(u.palette.neovimTheme()); err != nil {
			t.Status = fmt.Sprintf("Neovim theme: %v", err)
		}
	}
	u.refreshBars()
	if u.suggestions != nil {
		u.style(u.suggestions.List)
	}
}

// Style existing instances rather than changing tview's process-global defaults.
// That preserves buffers, cursor positions, connections and result stores.
func (u *UI) style(p tview.Primitive) {
	if p == nil {
		return
	}
	t := u.palette
	normal := tcell.StyleDefault.Foreground(t.Foreground).Background(t.Surface)
	label := tcell.StyleDefault.Foreground(t.Accent).Background(t.Background)
	selected := tcell.StyleDefault.Foreground(t.SelectedText).Background(t.Selection)
	if box, ok := p.(interface {
		SetBackgroundColor(tcell.Color) *tview.Box
		SetBorderColor(tcell.Color) *tview.Box
		SetTitleColor(tcell.Color) *tview.Box
	}); ok {
		box.SetBackgroundColor(t.Background)
		box.SetBorderColor(t.Border)
		box.SetTitleColor(t.Accent)
	}
	switch w := p.(type) {
	case *suggestionPopup:
		u.style(w.list)
	case *centered:
		u.style(w.Flex)
	case *tview.Flex:
		for i := 0; i < w.GetItemCount(); i++ {
			u.style(w.GetItem(i))
		}
	case *tview.Pages:
		for _, name := range w.GetPageNames(false) {
			u.style(w.GetPage(name))
		}
	case *tview.Form:
		w.SetLabelColor(t.Accent).SetFieldBackgroundColor(t.Surface).SetFieldTextColor(t.Foreground).SetButtonStyle(normal).SetButtonActivatedStyle(selected)
		for i := 0; i < w.GetFormItemCount(); i++ {
			if item, ok := w.GetFormItem(i).(tview.Primitive); ok {
				u.style(item)
			}
		}
		for i := 0; i < w.GetButtonCount(); i++ {
			u.style(w.GetButton(i))
		}
	case *tview.TextView:
		w.SetTextColor(t.Foreground)
		if w == u.status {
			w.SetBackgroundColor(t.Surface)
		}
	case *editor.Area:
		u.style(w.TextArea)
	case *tview.TextArea:
		w.SetTextStyle(tcell.StyleDefault.Foreground(t.Foreground).Background(t.Background)).SetSelectedStyle(selected).SetPlaceholderStyle(tcell.StyleDefault.Foreground(t.Muted).Background(t.Background))
	case *selectorSearch:
		u.style(w.InputField)
	case *tview.InputField:
		w.SetLabelStyle(label).SetFieldStyle(normal).SetPlaceholderStyle(tcell.StyleDefault.Foreground(t.Muted).Background(t.Surface)).SetAutocompleteStyles(t.Surface, normal, selected)
	case *tview.DropDown:
		w.SetLabelStyle(label).SetFieldStyle(normal).SetFocusedStyle(selected).SetListStyles(normal, selected).SetPrefixStyle(tcell.StyleDefault.Foreground(t.Accent).Background(t.Surface))
	case *tview.Checkbox:
		w.SetLabelStyle(label).SetCheckedStyle(normal).SetUncheckedStyle(normal).SetActivatedStyle(selected)
	case *tview.Button:
		w.SetStyle(normal).SetActivatedStyle(selected)
	case *tview.List:
		w.SetMainTextStyle(tcell.StyleDefault.Foreground(t.Foreground).Background(t.Background)).SetSecondaryTextStyle(tcell.StyleDefault.Foreground(t.Muted).Background(t.Background)).SetShortcutStyle(label).SetSelectedStyle(selected)
	case *tview.Table:
		w.SetBordersColor(t.Border).SetSelectedStyle(selected)
	case *tview.TreeView:
		w.SetGraphicsColor(t.Border)
		if root := w.GetRoot(); root != nil {
			root.Walk(func(n, parent *tview.TreeNode) bool {
				color := t.Foreground
				if ref, ok := n.GetReference().(connectionRef); ok && ref.FolderNode {
					color = t.Accent
				}
				n.SetTextStyle(tcell.StyleDefault.Foreground(color).Background(t.Background)).SetSelectedTextStyle(selected)
				return true
			})
		}
	case *tview.Modal:
		w.Box.SetBackgroundColor(t.Surface)
		w.SetBackgroundColor(t.Surface).SetTextColor(t.Foreground).SetButtonStyle(normal).SetButtonActivatedStyle(selected)
		w.SetBorderColor(t.Border).SetTitleColor(t.Accent)
	}
}
func (u *UI) chooseTheme() {
	if u.dialogCancel != nil {
		u.dismiss()
	}
	original := u.palette.ID
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true)
	search := newSelectorSearch(u.tr("Find theme: "), l)
	sample := tview.NewTextView().SetDynamicColors(true).SetScrollable(false).SetWrap(false)
	var matches []Palette
	preview := func(i int, _, _ string, _ rune) {
		if i < 0 || i >= len(matches) {
			l.SetTitle(u.tr(" Themes · No matches "))
			sample.SetText("")
			return
		}
		p := matches[i]
		if u.palette.ID != p.ID {
			u.applyTheme(p.ID)
		}
		l.SetTitle(u.tr(" Themes · Preview: ") + p.Name + " · " + u.tr("/ search · Enter save") + " ")
		sample.SetText(fmt.Sprintf(" %s [%s]SELECT[-] [%s]'hola'[-], [%s]42[-];",
			u.tr("SQL sample:"), colorTag(p.Syntax.Keyword), colorTag(p.Syntax.String), colorTag(p.Syntax.Number)))
	}
	rebuild := func(filter string) {
		l.SetChangedFunc(nil)
		l.Clear()
		matches = matches[:0]
		selected := 0
		for _, p := range palettes {
			if !fuzzy(strings.ToLower(strings.TrimSpace(filter)), strings.ToLower(p.Name)) {
				continue
			}
			label := p.Name
			if p.ID == original {
				selected = len(matches)
				label += " · " + u.tr("Current")
			}
			matches = append(matches, p)
			l.AddItem(label, "", 0, func() {
				next := u.cfg
				next.Theme = p.ID
				if err := config.Save(u.configPath, next); err != nil {
					l.SetTitle(u.tr(" ") + err.Error() + " ")
					return
				}
				u.cfg = next
				u.applyTheme(p.ID)
				u.dialogCancel = nil
				u.dismiss()
			})
		}
		if len(matches) > 0 {
			l.SetCurrentItem(selected)
		}
		preview(selected, "", "", 0)
		l.SetChangedFunc(preview)
	}
	search.SetChangedFunc(rebuild)
	rebuild("")
	panel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(search, 1, 0, false).
		AddItem(l, 0, 1, true).
		AddItem(sample, 1, 0, false)
	panel.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		return search.key(u, k, len(matches) > 0)
	})
	u.popup(panel, 80, 18)
	u.dialogCancel = func() { u.applyTheme(original) }
}

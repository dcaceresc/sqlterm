package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

type Palette struct {
	ID, Name                                                                        string
	Background, Surface, Foreground, Muted, Accent, Border, Selection, SelectedText tcell.Color
	Insert, Visual                                                                  tcell.Color
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
	}
	p.Insert = tcell.GetColor(insert)
	p.Visual = tcell.GetColor(visual)
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
	}
	u.refreshBars()
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
	case *tview.TextArea:
		w.SetTextStyle(tcell.StyleDefault.Foreground(t.Foreground).Background(t.Background)).SetSelectedStyle(selected).SetPlaceholderStyle(tcell.StyleDefault.Foreground(t.Muted).Background(t.Background))
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
	l := tview.NewList().ShowSecondaryText(true)
	l.SetBorder(true).SetTitle(" Themes · arrows preview · Enter save · Esc cancel ")
	for _, p := range palettes {
		choice := p
		l.AddItem(p.Name, p.ID, 0, func() {
			next := u.cfg
			next.Theme = choice.ID
			if err := config.Save(u.configPath, next); err != nil {
				l.SetTitle(" " + err.Error() + " ")
				return
			}
			u.cfg = next
			u.applyTheme(choice.ID)
			u.dialogCancel = nil
			u.dismiss()
		})
	}
	l.SetChangedFunc(func(i int, _ string, _ string, _ rune) {
		if i >= 0 && i < len(palettes) {
			u.applyTheme(palettes[i].ID)
		}
	})
	for i, p := range palettes {
		if p.ID == original {
			l.SetCurrentItem(i)
			break
		}
	}
	u.popup(l, 80, 18)
	u.dialogCancel = func() { u.applyTheme(original) }
}

package ui

import (
	"context"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/completion"
	"sqlterm/internal/db"
	"time"
	"unicode/utf8"
)

type suggestionState struct {
	Tab     *Tab
	Context completion.Context
	Items   []completion.Candidate
	List    *tview.List
	Popup   *suggestionPopup
}
type suggestionPopup struct {
	*tview.Box
	ui   *UI
	list *tview.List
}

func (p *suggestionPopup) Draw(screen tcell.Screen) {
	e := p.ui.active().Editor.Area
	x, y, _, h := e.GetInnerRect()
	_, _, row, col := e.GetCursor()
	offRow, offCol := e.GetOffset()
	screenW, screenH := screen.Size()
	width := min(62, max(10, screenW-2))
	height := min(10, p.list.GetItemCount()*2+2)
	left := max(0, min(x+col-offCol, screenW-width))
	top := y + row - offRow + 1
	if top+height > min(y+h, screenH-2) {
		top = max(0, y+row-offRow-height)
	}
	p.list.SetRect(left, top, width, height)
	p.list.Draw(screen)
}
func (p *suggestionPopup) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return p.list.MouseHandler()
}
func (u *UI) hideSuggestions() {
	u.suggestions = nil
}
func (u *UI) acceptSuggestion(index int) {
	state := u.suggestions
	if state == nil || index < 0 || index >= len(state.Items) {
		return
	}
	t := state.Tab
	if t != u.active() {
		u.hideSuggestions()
		return
	}
	text := state.Items[index].Text
	start, end := state.Context.Start, state.Context.End
	sql := t.Editor.Area.GetText()
	if start < 0 || end > len(sql) {
		u.hideSuggestions()
		return
	}
	u.suppressSuggestions = true
	u.hideSuggestions()
	t.Editor.Area.Replace(start, end, text)
	u.suppressSuggestions = false
	u.app.SetFocus(t.Editor.Area)
}
func (u *UI) suggestionKey(k *tcell.EventKey) bool {
	state := u.suggestions
	if state == nil {
		return false
	}
	front, _ := u.pages.GetFrontPage()
	if front != "main" {
		return false
	}
	index := state.List.GetCurrentItem()
	switch k.Key() {
	case tcell.KeyDown, tcell.KeyCtrlN:
		state.List.SetCurrentItem((index + 1) % len(state.Items))
		return true
	case tcell.KeyUp, tcell.KeyCtrlP:
		state.List.SetCurrentItem((index - 1 + len(state.Items)) % len(state.Items))
		return true
	case tcell.KeyTab, tcell.KeyEnter:
		u.acceptSuggestion(index)
		return true
	case tcell.KeyEscape:
		u.hideSuggestions()
		return false
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyHome, tcell.KeyEnd:
		u.hideSuggestions()
		u.skipSuggestionMove = true
		return false
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		return false
	case tcell.KeyCtrlSpace:
		return false
	default:
		u.hideSuggestions()
		return false
	}
}
func (u *UI) updateSuggestions(explicit bool) {
	if u.suppressSuggestions || u.closing || len(u.tabs) == 0 {
		return
	}
	if u.skipSuggestionMove {
		u.skipSuggestionMove = false
		return
	}
	t := u.active()
	front, _ := u.pages.GetFrontPage()
	if t.SelectingDatabase || !t.Editor.Area.HasFocus() || front != "main" || (!explicit && t.Editor.Mode != "INSERT") {
		u.hideSuggestions()
		if explicit && t.SelectingDatabase {
			u.message(u.tr("Choose a database first (F4)"))
		}
		return
	}
	sql := t.Editor.Area.GetText()
	engine := t.Profile.Engine
	if engine == "" {
		engine = "sqlserver"
	}
	adapter := db.NewAdapter(engine)
	if t.Session != nil {
		adapter = t.Session.Adapter
	}
	ctx := completion.Analyze(sql, t.Editor.Cursor(), engine)
	if !ctx.InCode || (!explicit && utf8.RuneCountInString(ctx.Prefix) < 2 && len(ctx.Qualifier) == 0) {
		u.hideSuggestions()
		return
	}
	items := completion.Suggestions(ctx, adapter, t.Catalog, t.Columns)
	if len(items) > 80 {
		items = items[:80]
	}
	if len(items) == 0 {
		u.hideSuggestions()
		if explicit {
			u.message(u.tr("No completions for this SQL context"))
		}
	} else {
		list := tview.NewList().ShowSecondaryText(true).SetUseStyleTags(false, false)
		list.SetBorder(true).SetTitle(u.tr(" IntelliSense · Tab/Enter accept · Esc close "))
		state := &suggestionState{Tab: t, Context: ctx, Items: items, List: list}
		for i, c := range items {
			index := i
			list.AddItem(c.Label, c.Detail, 0, func() { u.acceptSuggestion(index) })
		}
		popup := &suggestionPopup{Box: tview.NewBox(), ui: u, list: list}
		state.Popup = popup
		u.suggestions = state
		u.style(list)
	}
	if t.Busy || t.Session == nil {
		return
	}
	var missing []db.Object
	for _, o := range completion.ColumnSources(ctx, t.Catalog) {
		if _, ok := t.Columns[o.Label()]; !ok {
			missing = append(missing, o)
		}
	}
	if len(missing) == 0 {
		return
	}
	u.task(t, "IntelliSense metadata", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		loaded := map[string][]string{}
		var failure error
		for _, o := range missing {
			columns, err := t.Session.Adapter.Columns(ctx, t.Session.Conn, o)
			loaded[o.Label()] = columns
			if err != nil {
				failure = err
			}
		}
		return func() {
			for key, c := range loaded {
				t.Columns[key] = c
			}
			if u.active() == t && t.Editor.Mode == "INSERT" {
				u.updateSuggestions(false)
			}
		}, failure
	})
}

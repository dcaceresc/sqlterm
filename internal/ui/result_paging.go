package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

type resultPause struct {
	Set, Rows int
	Token     db.PageToken
}

func (u *UI) setResultPause(t *Tab, e *execution, ev db.Event) {
	if t.Execution != e || e.Pager == nil {
		return
	}
	e.Pause = &resultPause{Set: ev.Set, Rows: ev.Rows, Token: ev.Token}
	e.pausedFor = time.Since(e.Started)
	e.pulsePaused.Store(true)
	e.Phase = "Paused"
	t.Status = fmt.Sprintf(u.tr("Paused after %d received rows · scroll at the end for more · F10 stop and keep rows"), ev.Rows)
	if u.active() == t {
		u.refreshResultInfo()
		u.refreshBars()
	}
}

func (u *UI) requestResultBlock(t *Tab) bool {
	if t == nil || !t.Busy || t.Execution == nil || t.Execution.Pager == nil || t.Execution.Pause == nil {
		return false
	}
	e := t.Execution
	pause := e.Pause
	if pause.Set != t.SelectedSet || pause.Set < 0 || pause.Set >= len(t.Results) {
		return false
	}
	if !e.Pager.Request(pause.Token) {
		return false
	}
	e.Pause = nil
	e.pulsePaused.Store(false)
	e.Phase = "Receiving rows"
	t.Status = u.tr("Receiving next result block…")
	u.refreshResultInfo()
	u.refreshBars()
	return true
}

func (u *UI) resultNavigationDemand(k *tcell.EventKey) bool {
	if !u.table.HasFocus() || len(u.tabs) == 0 {
		return false
	}
	s := u.currentStore()
	if s == nil || s.Count() == 0 {
		return false
	}
	row, _ := u.table.GetSelection()
	atEnd := row >= s.Count()
	nearEnd := row+max(1, u.resultVisibleRows()) >= s.Count()
	switch k.Key() {
	case tcell.KeyDown, tcell.KeyCtrlN:
		return atEnd && u.requestResultBlock(u.active())
	case tcell.KeyPgDn, tcell.KeyEnd:
		return nearEnd && u.requestResultBlock(u.active())
	case tcell.KeyRune:
		return k.Rune() == 'j' && atEnd && u.requestResultBlock(u.active())
	}
	return false
}

func (u *UI) resultWheelDemand(event *tcell.EventMouse, action tview.MouseAction) bool {
	if action != tview.MouseScrollDown || u.dialogContent != nil || !u.table.HasFocus() {
		return false
	}
	x, y := event.Position()
	if !u.table.InRect(x, y) {
		return false
	}
	s := u.currentStore()
	if s == nil {
		return false
	}
	rowOffset, _ := u.table.GetOffset()
	return rowOffset+max(1, u.resultVisibleRows()) >= s.Count() && u.requestResultBlock(u.active())
}

func (u *UI) resultVisibleRows() int {
	_, _, _, height := u.table.GetInnerRect()
	return max(1, height-1)
}

func (u *UI) resultBlockSizePrompt() {
	u.prompt(u.tr("Result block size (1–100000)"), strconv.Itoa(u.cfg.EffectiveResultBlockSize()), false, func(raw string) {
		size, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil {
			err = config.ValidateResultBlockSize(size)
		}
		if err != nil {
			u.message(u.tr("Result block size must be between 1 and 100000"))
			return
		}
		next := u.cfg
		next.ResultBlockSize = size
		if err = config.Save(u.configPath, next); err != nil {
			u.message(err.Error())
			return
		}
		u.cfg = next
		u.message(fmt.Sprintf(u.tr("Result block size saved: %d rows"), size))
	})
}

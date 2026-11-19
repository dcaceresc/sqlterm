package editor

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Editor struct {
	Area     *Area
	View     *View
	Mode     string
	OnMode   func(string)
	native   *nativeEditor
	OnRedraw func()
	OnError  func(error)
}

func New() *Editor {
	e := &Editor{Mode: "NORMAL"}
	e.Area = &Area{TextArea: tview.NewTextArea().SetWrap(false), editor: e}
	e.View = &View{Area: e.Area, editor: e, numbers: true}
	e.Area.SetInputCapture(e.Handle)
	return e
}

func (e *Editor) SetText(s string) error {
	if e.native == nil {
		e.Mode = "NORMAL"
		if e.OnMode != nil {
			e.OnMode(e.Mode)
		}
	}
	e.Area.SetText(s, false)
	return e.Err()
}

func (e *Editor) Cursor() int {
	if e.native != nil {
		return e.native.snapshot.Cursor
	}
	_, _, end := e.Area.GetSelection()
	return end
}

func (e *Editor) Handle(k *tcell.EventKey) *tcell.EventKey {
	if e.native == nil {
		return k
	}
	e.nativeKey(k)
	return nil
}

func lineStart(s string, p int) int { return strings.LastIndex(s[:p], "\n") + 1 }
func lineEnd(s string, p int) int {
	n := strings.IndexByte(s[p:], '\n')
	if n < 0 {
		return len(s)
	}
	return p + n
}

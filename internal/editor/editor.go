package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Editor struct {
	Area        *tview.TextArea
	Mode        string
	Clipboard   string
	linewise    bool
	anchor, pos int
	pending     rune
	OnMode      func(string)
	Search      func()
}

func New() *Editor {
	e := &Editor{Area: tview.NewTextArea().SetWrap(false), Mode: "NORMAL"}
	e.Area.SetInputCapture(e.Handle)
	return e
}
func (e *Editor) mode(m string) {
	e.Mode = m
	e.pending = 0
	if e.OnMode != nil {
		e.OnMode(m)
	}
}
func (e *Editor) SetText(s string) { e.mode("NORMAL"); e.Area.SetText(s, false) }
func (e *Editor) Cursor() int {
	_, _, end := e.Area.GetSelection()
	if e.Mode == "VISUAL" {
		return e.pos
	}
	return end
}
func (e *Editor) move(p int) {
	s := e.Area.GetText()
	p = max(0, min(len(s), p))
	if e.Mode == "VISUAL" {
		e.pos = p
		e.Area.Select(min(e.anchor, p), max(e.anchor, p))
	} else {
		e.Area.Select(p, p)
	}
}
func prev(s string, p int) int {
	if p <= 0 {
		return 0
	}
	_, n := utf8.DecodeLastRuneInString(s[:p])
	return p - n
}
func next(s string, p int) int {
	if p >= len(s) {
		return len(s)
	}
	_, n := utf8.DecodeRuneInString(s[p:])
	return p + n
}
func lineStart(s string, p int) int { return strings.LastIndex(s[:p], "\n") + 1 }
func lineEnd(s string, p int) int {
	n := strings.IndexByte(s[p:], '\n')
	if n < 0 {
		return len(s)
	}
	return p + n
}
func (e *Editor) Handle(k *tcell.EventKey) *tcell.EventKey {
	if k.Key() == tcell.KeyEscape {
		p := e.Cursor()
		e.Area.Select(p, p)
		e.mode("NORMAL")
		return nil
	}
	if e.Mode == "INSERT" {
		return k
	}
	if k.Key() == tcell.KeyCtrlR {
		return tcell.NewEventKey(tcell.KeyCtrlY, 0, 0)
	}
	if k.Key() != tcell.KeyRune {
		return k
	}
	s := e.Area.GetText()
	p := e.Cursor()
	r := k.Rune()
	pending := e.pending
	e.pending = 0
	switch r {
	case 'h':
		e.move(prev(s, p))
	case 'l':
		e.move(next(s, p))
	case 'j', 'k':
		start := lineStart(s, p)
		col := utf8.RuneCountInString(s[start:p])
		target := 0
		if r == 'j' {
			end := lineEnd(s, p)
			if end == len(s) {
				return nil
			}
			target = end + 1
		} else {
			if start == 0 {
				return nil
			}
			target = lineStart(s, start-1)
		}
		end := lineEnd(s, target)
		for col > 0 && target < end {
			target = next(s, target)
			col--
		}
		e.move(target)
	case '0':
		e.move(lineStart(s, p))
	case '$':
		e.move(lineEnd(s, p))
	case 'w':
		for p < len(s) {
			r, _ := utf8.DecodeRuneInString(s[p:])
			if unicode.IsSpace(r) {
				break
			}
			p = next(s, p)
		}
		for p < len(s) {
			r, _ := utf8.DecodeRuneInString(s[p:])
			if !unicode.IsSpace(r) {
				break
			}
			p = next(s, p)
		}
		e.move(p)
	case 'b':
		p = prev(s, p)
		for p > 0 {
			r, _ := utf8.DecodeRuneInString(s[p:])
			if !unicode.IsSpace(r) {
				break
			}
			p = prev(s, p)
		}
		for p > 0 {
			q := prev(s, p)
			r, _ := utf8.DecodeRuneInString(s[q:])
			if unicode.IsSpace(r) {
				break
			}
			p = q
		}
		e.move(p)
	case 'g':
		if pending == 'g' {
			e.move(0)
		} else {
			e.pending = 'g'
		}
	case 'G':
		e.move(len(s))
	case 'i':
		e.mode("INSERT")
	case 'a':
		e.move(next(s, p))
		e.mode("INSERT")
	case 'I':
		e.move(lineStart(s, p))
		e.mode("INSERT")
	case 'A':
		e.move(lineEnd(s, p))
		e.mode("INSERT")
	case 'o':
		end := lineEnd(s, p)
		e.Area.Replace(end, end, "\n")
		e.mode("INSERT")
	case 'v':
		if e.Mode == "VISUAL" {
			e.Area.Select(p, p)
			e.mode("NORMAL")
		} else {
			e.anchor = p
			e.pos = p
			e.mode("VISUAL")
		}
	case 'y', 'd':
		if e.Mode == "VISUAL" {
			text, start, end := e.Area.GetSelection()
			e.Clipboard = text
			e.linewise = false
			if r == 'd' {
				e.Area.Replace(start, end, "")
			} else {
				e.Area.Select(start, start)
			}
			e.mode("NORMAL")
		} else if pending == r {
			start, end := lineStart(s, p), lineEnd(s, p)
			if end < len(s) {
				end++
			}
			e.Clipboard = s[start:end]
			e.linewise = true
			if r == 'd' {
				e.Area.Replace(start, end, "")
			}
		} else {
			e.pending = r
		}
	case 'x':
		e.Area.Replace(p, next(s, p), "")
	case 'p':
		if e.linewise {
			end := lineEnd(s, p)
			if end < len(s) {
				end++
			} else {
				e.Area.Replace(end, end, "\n")
				end++
			}
			e.Area.Replace(end, end, e.Clipboard)
		} else {
			e.Area.Replace(p, p, e.Clipboard)
		}
	case 'u':
		return tcell.NewEventKey(tcell.KeyCtrlZ, 0, 0)
	case '/':
		if e.Search != nil {
			e.Search()
		}
	default:
		return nil
	}
	return nil
}

// Prefix returns the identifier fragment immediately before the cursor.
func (e *Editor) Prefix() (string, int) {
	s := e.Area.GetText()
	p := e.Cursor()
	start := p
	for start > 0 {
		q := prev(s, start)
		r, _ := utf8.DecodeRuneInString(s[q:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '@' {
			break
		}
		start = q
	}
	fragment := s[start:p]
	if start > 0 && (s[start-1] == '[' || s[start-1] == '"' || s[start-1] == '`') {
		start--
	}
	return fragment, start
}

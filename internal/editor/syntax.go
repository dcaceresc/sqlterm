package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// SyntaxColors are foreground-only roles. Selection always takes precedence.
type SyntaxColors struct{ Keyword, String, Number tcell.Color }
type syntaxKind byte

const (
	syntaxPlain syntaxKind = iota
	syntaxKeyword
	syntaxString
	syntaxNumber
)

func (v *View) SetSyntaxColors(colors SyntaxColors) { v.syntaxColors = colors; v.syntaxEnabled = true }

var sqlKeywords = func() map[string]bool {
	words := "SELECT FROM WHERE JOIN LEFT RIGHT FULL CROSS INNER OUTER ON GROUP BY ORDER HAVING INSERT INTO VALUES UPDATE SET DELETE CREATE ALTER DROP AS DISTINCT NULL IS NOT AND OR WITH BEGIN COMMIT ROLLBACK CASE WHEN THEN ELSE END UNION ALL EXISTS IN LIKE BETWEEN ASC DESC TOP OUTPUT DECLARE EXEC EXECUTE GO PRINT TRY CATCH MERGE LIMIT OFFSET CALL DELIMITER SHOW EXPLAIN REPLACE RETURNING DO LANGUAGE TABLE VIEW PROCEDURE FUNCTION INDEX DATABASE USE TRUNCATE DEFAULT PRIMARY FOREIGN KEY REFERENCES CONSTRAINT CHECK UNIQUE IF OVER PARTITION ROWS RANGE FETCH NEXT ONLY TRUE FALSE CAST CONVERT"
	out := make(map[string]bool)
	for _, word := range strings.Fields(words) {
		out[word] = true
	}
	return out
}()

// Classify bytes, not runes: TextArea selection and layout positions use bytes.
// This tolerant lexer accepts unfinished input and never validates or rewrites SQL.
func syntaxKinds(text string) []syntaxKind {
	kinds := make([]syntaxKind, len(text))
	paint := func(start, end int, kind syntaxKind) {
		for i := start; i < end; i++ {
			kinds[i] = kind
		}
	}
	for i := 0; i < len(text); {
		start := i
		switch {
		case strings.HasPrefix(text[i:], "--"):
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case strings.HasPrefix(text[i:], "/*"):
			depth := 1
			i += 2
			for i < len(text) && depth > 0 {
				if strings.HasPrefix(text[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(text[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
		case text[i] == '\'':
			i = quotedEnd(text, i, '\'')
			paint(start, i, syntaxString)
		case text[i] == '"' || text[i] == '`' || text[i] == '[':
			close := text[i]
			if close == '[' {
				close = ']'
			}
			i = quotedEnd(text, i, close)
		case text[i] == '$' && dollarQuoteTag(text[i:]) != "":
			tag := dollarQuoteTag(text[i:])
			i += len(tag)
			if end := strings.Index(text[i:], tag); end >= 0 {
				i += end + len(tag)
			} else {
				i = len(text)
			}
			paint(start, i, syntaxString)
		case isDigit(text[i]) || text[i] == '.' && i+1 < len(text) && isDigit(text[i+1]):
			i = numberEnd(text, i)
			paint(start, i, syntaxNumber)
		default:
			r, size := utf8.DecodeRuneInString(text[i:])
			if unicode.IsLetter(r) || r == '_' || r == '@' || r == '#' || r == '$' {
				i += size
				for i < len(text) {
					r, size = utf8.DecodeRuneInString(text[i:])
					if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '@' && r != '#' && r != '$' {
						break
					}
					i += size
				}
				word := strings.ToUpper(text[start:i])
				if (word == "N" || word == "E") && i < len(text) && text[i] == '\'' {
					i = quotedEnd(text, i, '\'')
					paint(start, i, syntaxString)
				} else if sqlKeywords[word] {
					paint(start, i, syntaxKeyword)
				}
			} else {
				i += size
			}
		}
	}
	return kinds
}

func quotedEnd(text string, start int, close byte) int {
	for i := start + 1; i < len(text); i++ {
		if text[i] == close {
			if i+1 < len(text) && text[i+1] == close {
				i++
				continue
			}
			return i + 1
		}
		if text[i] == '\\' && close != ']' && i+1 < len(text) {
			i++
		}
	}
	return len(text)
}
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func numberEnd(text string, i int) int {
	if strings.HasPrefix(text[i:], "0x") || strings.HasPrefix(text[i:], "0X") {
		j := i + 2
		for j < len(text) && (isDigit(text[j]) || text[j] >= 'a' && text[j] <= 'f' || text[j] >= 'A' && text[j] <= 'F') {
			j++
		}
		if j > i+2 {
			return j
		}
	}
	for i < len(text) && isDigit(text[i]) {
		i++
	}
	if i < len(text) && text[i] == '.' {
		i++
		for i < len(text) && isDigit(text[i]) {
			i++
		}
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		j := i + 1
		if j < len(text) && (text[j] == '+' || text[j] == '-') {
			j++
		}
		digits := j
		for j < len(text) && isDigit(text[j]) {
			j++
		}
		if j > digits {
			i = j
		}
	}
	return i
}

func (v *View) drawSyntax(screen tcell.Screen, text string, x, y, width, height, offset int) {
	_, columnOffset := v.GetOffset()
	if v.wrap {
		columnOffset = 0
	}
	_, selectionStart, selectionEnd := v.GetSelection()
	if selectionStart > selectionEnd {
		selectionStart, selectionEnd = selectionEnd, selectionStart
	}
	for row := 0; row < height && offset+row < len(v.rows); row++ {
		start := v.rows[offset+row].start
		end := len(text)
		if offset+row+1 < len(v.rows) {
			end = v.rows[offset+row+1].start
		}
		g := uniseg.NewGraphemes(text[start:end])
		col := 0
		for g.Next() {
			from, to := g.Positions()
			from += start
			to += start
			cw := g.Width()
			if g.Str() == "\t" {
				cw = tview.TabSize
			}
			cell := col - columnOffset
			col += cw
			if cell >= width {
				break
			}
			if cw == 0 || cell < 0 || cell+cw > width || from >= len(v.syntax) || from < selectionEnd && to > selectionStart {
				continue
			}
			var color tcell.Color
			switch v.syntax[from] {
			case syntaxKeyword:
				color = v.syntaxColors.Keyword
			case syntaxString:
				color = v.syntaxColors.String
			case syntaxNumber:
				color = v.syntaxColors.Number
			default:
				continue
			}
			// TextArea expands tabs to individual cells; other clusters keep their
			// combining characters and wide-cell ownership in a single SetContent.
			cells := 1
			if g.Str() == "\t" {
				cells = cw
			}
			for n := 0; n < cells; n++ {
				main, combining, style, _ := screen.GetContent(x+cell+n, y+row)
				screen.SetContent(x+cell+n, y+row, main, combining, style.Foreground(color))
			}
		}
	}
}

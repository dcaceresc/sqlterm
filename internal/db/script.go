package db

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var goLine = regexp.MustCompile(`(?i)^GO(?:\s+(\d+))?\s*(?:--.*)?$`)
var dollarTag = regexp.MustCompile(`^\$(?:[A-Za-z_][A-Za-z_0-9]*)?\$`)

// SplitScript recognizes client directives only outside literals and comments.
// SQL Server batches retain semicolons. MySQL routine scripts use DELIMITER.
func SplitScript(engine, text string) ([]string, error) {
	var out []string
	var b strings.Builder
	quote := byte(0)
	block := 0
	lineComment := false
	dollar := ""
	delimiter := ";"
	lineStart := true
	hasCode := false
	flush := func() {
		if hasCode {
			out = append(out, strings.TrimSpace(b.String()))
		}
		b.Reset()
		hasCode = false
	}
	for i := 0; i < len(text); {
		if lineStart && quote == 0 && block == 0 && dollar == "" && !lineComment {
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			line := strings.TrimSpace(strings.TrimSuffix(text[i:i+end], "\r"))
			if engine == "sqlserver" {
				if m := goLine.FindStringSubmatch(line); m != nil {
					count := 1
					if m[1] != "" {
						n, e := strconv.Atoi(m[1])
						if e != nil || n < 1 || n > 1000 {
							return nil, fmt.Errorf("GO repeat count must be 1..1000")
						}
						count = n
					}
					if hasCode {
						s := strings.TrimSpace(b.String())
						for j := 0; j < count; j++ {
							out = append(out, s)
						}
					}
					b.Reset()
					hasCode = false
					i += end
					if i < len(text) {
						i++
					}
					lineStart = true
					continue
				}
			}
			if engine == "mysql" {
				fields := strings.Fields(line)
				if len(fields) > 0 && strings.EqualFold(fields[0], "DELIMITER") {
					if len(fields) != 2 || hasCode {
						return nil, fmt.Errorf("DELIMITER requires one token between statements")
					}
					delimiter = fields[1]
					i += end
					if i < len(text) {
						i++
					}
					lineStart = true
					continue
				}
			}
		}
		ch := text[i]
		lineStart = ch == '\n'
		if lineComment {
			b.WriteByte(ch)
			i++
			if ch == '\n' {
				lineComment = false
			}
			continue
		}
		if block > 0 {
			if i+1 < len(text) && text[i:i+2] == "/*" && engine != "mysql" {
				block++
				b.WriteString("/*")
				i += 2
				continue
			}
			if i+1 < len(text) && text[i:i+2] == "*/" {
				block--
				b.WriteString("*/")
				i += 2
				continue
			}
			b.WriteByte(ch)
			i++
			continue
		}
		if dollar != "" {
			if strings.HasPrefix(text[i:], dollar) {
				b.WriteString(dollar)
				i += len(dollar)
				dollar = ""
			} else {
				b.WriteByte(ch)
				i++
			}
			continue
		}
		if quote != 0 {
			b.WriteByte(ch)
			i++
			if ch == '\\' && quote != '"' && quote != ']' && quote != '`' && (engine == "mysql" || (engine == "postgres" && quote == 'E')) {
				if i < len(text) {
					b.WriteByte(text[i])
					i++
				}
				continue
			}
			close := quote
			if quote == 'E' {
				close = '\''
			}
			if ch == close {
				if i < len(text) && text[i] == close {
					b.WriteByte(text[i])
					i++
				} else {
					quote = 0
				}
			}
			continue
		}
		if i+1 < len(text) && text[i:i+2] == "--" && (engine != "mysql" || i+2 == len(text) || text[i+2] <= ' ') {
			lineComment = true
			b.WriteString("--")
			i += 2
			continue
		}
		if ch == '#' && engine == "mysql" {
			lineComment = true
			b.WriteByte(ch)
			i++
			continue
		}
		if i+1 < len(text) && text[i:i+2] == "/*" {
			block = 1
			b.WriteString("/*")
			i += 2
			continue
		}
		if engine == "postgres" && ch == '$' && (i == 0 || !isIdent(text[i-1]) && text[i-1] != '$') {
			if tag := dollarTag.FindString(text[i:]); tag != "" {
				dollar = tag
				hasCode = true
				b.WriteString(tag)
				i += len(tag)
				continue
			}
		}
		if ch == '\'' || ch == '"' || (engine == "mysql" && ch == '`') || (engine == "sqlserver" && ch == '[') {
			quote = ch
			if ch == '[' {
				quote = ']'
			}
			if ch == '\'' && engine == "postgres" && i > 0 && (text[i-1] == 'E' || text[i-1] == 'e') && (i < 2 || !isIdent(text[i-2])) {
				quote = 'E'
			}
			hasCode = true
			b.WriteByte(ch)
			i++
			continue
		}
		if engine != "sqlserver" && strings.HasPrefix(text[i:], delimiter) {
			flush()
			i += len(delimiter)
			continue
		}
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			hasCode = true
		}
		b.WriteByte(ch)
		i++
	}
	if quote != 0 || block > 0 || dollar != "" {
		return nil, fmt.Errorf("unterminated SQL literal or comment")
	}
	flush()
	return out, nil
}
func isIdent(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

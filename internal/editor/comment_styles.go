package editor

import "strings"

// commentLineFlags recognizes full-line -- comments, not markers contained in
// SQL literals or identifiers. State spans physical lines so multiline quoted
// values, dollar-quoted bodies and nested block comments remain untouched.
// The view caches this classification until the buffer changes.
func commentLineFlags(text string) []bool {
	lines := strings.Split(text, "\n")
	flags := make([]bool, len(lines))
	var quote byte
	block := 0
	dollar := ""
	for row, line := range lines {
		if quote == 0 && block == 0 && dollar == "" && strings.HasPrefix(strings.TrimLeft(line, " \t"), "--") {
			flags[row] = true
			continue
		}
		for i := 0; i < len(line); i++ {
			rest := line[i:]
			if dollar != "" {
				if strings.HasPrefix(rest, dollar) {
					i += len(dollar) - 1
					dollar = ""
				}
				continue
			}
			if block > 0 {
				if strings.HasPrefix(rest, "/*") {
					block++
					i++
				} else if strings.HasPrefix(rest, "*/") {
					block--
					i++
				}
				continue
			}
			if quote != 0 {
				if line[i] == quote {
					if i+1 < len(line) && line[i+1] == quote {
						i++
					} else {
						quote = 0
					}
				} else if line[i] == '\\' && quote != ']' && i+1 < len(line) {
					i++
				}
				continue
			}
			if strings.HasPrefix(rest, "--") {
				break
			}
			if strings.HasPrefix(rest, "/*") {
				block = 1
				i++
				continue
			}
			switch line[i] {
			case '\'', '"', '`':
				quote = line[i]
			case '[':
				quote = ']'
			case '$':
				if tag := dollarQuoteTag(rest); tag != "" {
					dollar = tag
					i += len(tag) - 1
				}
			}
		}
	}
	return flags
}

func dollarQuoteTag(text string) string {
	for i := 1; i < len(text); i++ {
		c := text[i]
		if c == '$' {
			return text[:i+1]
		}
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 1 && c >= '0' && c <= '9') {
			break
		}
	}
	return ""
}

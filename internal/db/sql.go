package db

import (
	"regexp"
	"strings"
)

// Conservative classification: ambiguous SQL stays on the result-producing path.
func isExec(text string) bool {
	body, e := leadingSQL(text)
	if e != nil {
		return false
	}
	words := strings.Fields(body)
	if len(words) == 0 {
		return false
	}
	switch strings.ToUpper(words[0]) {
	case "CREATE", "ALTER", "DROP", "TRUNCATE", "GRANT", "REVOKE", "BEGIN", "COMMIT", "ROLLBACK", "SET", "USE":
		return true
	case "INSERT", "UPDATE", "DELETE", "MERGE":
		u := strings.ToUpper(body)
		return !strings.Contains(u, "RETURNING") && !strings.Contains(u, "OUTPUT")
	}
	return false
}

var defaultTokens = regexp.MustCompile(`@[A-Za-z_#][A-Za-z_0-9$#]*|[A-Za-z_][A-Za-z_0-9$#]*|[=()]`)

func parameterDefaults(text string) map[string]bool {
	// Remove comments, strings and delimited identifiers so AS and = inside them
	// cannot change the parameter header interpretation.
	var b strings.Builder
	quote := byte(0)
	block := 0
	line := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if line {
			if ch == '\n' {
				line = false
			}
			b.WriteByte(' ')
			continue
		}
		if block > 0 {
			if i+1 < len(text) && text[i:i+2] == "/*" {
				block++
				i++
			} else if i+1 < len(text) && text[i:i+2] == "*/" {
				block--
				i++
			}
			b.WriteByte(' ')
			continue
		}
		if quote != 0 {
			if ch == quote {
				if i+1 < len(text) && text[i+1] == quote {
					i++
				} else {
					quote = 0
				}
			}
			b.WriteByte(' ')
			continue
		}
		if i+1 < len(text) && text[i:i+2] == "--" {
			line = true
			i++
			b.WriteByte(' ')
			continue
		}
		if i+1 < len(text) && text[i:i+2] == "/*" {
			block = 1
			i++
			b.WriteByte(' ')
			continue
		}
		if ch == '\'' || ch == '"' || ch == '[' {
			quote = ch
			if ch == '[' {
				quote = ']'
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(ch)
	}
	out := map[string]bool{}
	current := ""
	depth := 0
	for _, t := range defaultTokens.FindAllString(b.String(), -1) {
		switch {
		case t == "(":
			depth++
		case t == ")":
			depth--
		case strings.EqualFold(t, "AS") && depth == 0:
			return out
		case strings.HasPrefix(t, "@"):
			current = strings.ToLower(t)
		case t == "=" && current != "":
			out[current] = true
		}
	}
	return out
}

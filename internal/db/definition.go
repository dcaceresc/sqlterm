package db

import (
	"fmt"
	"regexp"
	"strings"
)

var ddlHeader = regexp.MustCompile("(?is)^(CREATE\\s+(?:(?:OR\\s+(?:REPLACE|ALTER))\\s+)?(?:ALGORITHM\\s*=\\s*\\w+\\s+)?(?:DEFINER\\s*=\\s*(?:`[^`]*`|'[^']*'|[^\\s@]+)\\s*@\\s*(?:`[^`]*`|'[^']*'|[^\\s]+)\\s+)?(?:SQL\\s+SECURITY\\s+(?:DEFINER|INVOKER)\\s+)?|ALTER\\s+)(PROCEDURE|PROC|FUNCTION|VIEW)\\s+")
var createVerb = regexp.MustCompile(`(?i)^CREATE\s+(?:OR\s+REPLACE\s+)?`)

func leadingSQL(text string) (string, error) {
	for {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "--") {
			n := strings.IndexByte(text, '\n')
			if n < 0 {
				return "", fmt.Errorf("no SQL definition")
			}
			text = text[n+1:]
			continue
		}
		if strings.HasPrefix(text, "/*") {
			level := 1
			i := 2
			for i < len(text) && level > 0 {
				if strings.HasPrefix(text[i:], "/*") {
					level++
					i += 2
				} else if strings.HasPrefix(text[i:], "*/") {
					level--
					i += 2
				} else {
					i++
				}
			}
			if level != 0 {
				return "", fmt.Errorf("unclosed comment")
			}
			text = text[i:]
			continue
		}
		return text, nil
	}
}
func readIdentifier(s string) (name, rest string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", fmt.Errorf("missing object name")
	}
	open := s[0]
	close := open
	if open == '[' {
		close = ']'
	}
	if open == '[' || open == '`' || open == '"' {
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] == close {
				if i+1 < len(s) && s[i+1] == close {
					b.WriteByte(close)
					i++
					continue
				}
				return b.String(), s[i+1:], nil
			}
			b.WriteByte(s[i])
		}
		return "", "", fmt.Errorf("unclosed identifier")
	}
	i := 0
	for i < len(s) && (isIdent(s[i]) || s[i] >= 128 || s[i] == '$' || s[i] == '#') {
		i++
	}
	if i == 0 {
		return "", "", fmt.Errorf("invalid identifier")
	}
	return s[:i], s[i:], nil
}
func PrepareDefinition(a Adapter, existing *Object, text string) ([]Bound, error) {
	body, e := leadingSQL(text)
	if e != nil {
		return nil, e
	}
	m := ddlHeader.FindStringSubmatchIndex(body)
	if m == nil {
		return nil, fmt.Errorf("Apply requires one CREATE/ALTER PROCEDURE, FUNCTION or VIEW definition")
	}
	kind := strings.ToLower(body[m[4]:m[5]])
	if kind == "proc" {
		kind = "procedure"
	}
	prefix := body[:m[1]]
	rest := body[m[1]:]
	name, tail, e := readIdentifier(rest)
	if e != nil {
		return nil, e
	}
	schema := ""
	trimmed := strings.TrimSpace(tail)
	if strings.HasPrefix(trimmed, ".") {
		schema = name
		name, tail, e = readIdentifier(trimmed[1:])
		if e != nil {
			return nil, e
		}
	}
	if existing != nil {
		if existing.Kind != kind || existing.Name != name || (schema != "" && existing.Schema != schema) {
			return nil, fmt.Errorf("definition target changed; use a new query to create another object")
		}
		schema = existing.Schema
	}
	target := a.Quote(name)
	if schema != "" {
		target = a.Quote(schema) + "." + target
	}
	rewritten := prefix + target + tail
	if a.Engine() == "sqlserver" {
		verb := "CREATE "
		if existing != nil {
			verb = "ALTER "
		}
		rewritten = verb + strings.ToUpper(kind) + " " + target + tail
	}
	if a.Engine() == "postgres" {
		rewritten = "CREATE OR REPLACE " + strings.ToUpper(kind) + " " + target + tail
	}
	if a.Engine() == "mysql" && existing != nil && kind != "view" {
		if strings.HasPrefix(strings.ToUpper(prefix), "ALTER") || regexp.MustCompile(`(?i)\bOR\s+(REPLACE|ALTER)\b`).MatchString(prefix) {
			return nil, fmt.Errorf("MySQL routine definitions must use CREATE with the full body")
		}
		return []Bound{{SQL: "DROP " + strings.ToUpper(kind) + " " + target}, {SQL: rewritten}}, nil
	}
	if a.Engine() == "mysql" && existing != nil && kind == "view" {
		rewritten = createVerb.ReplaceAllString(prefix, "CREATE OR REPLACE ") + target + tail
	}
	return []Bound{{SQL: rewritten}}, nil
}

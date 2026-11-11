// Package completion provides catalog-based SQL assistance without executing SQL.
package completion

import (
	"regexp"
	"sort"
	"sqlterm/internal/db"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Token struct {
	Text, Kind     string
	Start, End     int
	Quoted, Closed bool
}
type Reference struct{ Schema, Name, Alias string }
type Context struct {
	Prefix         string
	Start, End     int
	Qualifier      []string
	Routine        []string
	Want           string
	InCode         bool
	StatementStart bool
	References     []Reference
}
type Candidate struct {
	Text, Label, Detail, Name string
	Priority                  int
}

func ident(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_@$#", r)
}

var dollarTag = regexp.MustCompile(`^\$(?:[A-Za-z_][A-Za-z_0-9]*)?\$`)

func tokens(s, engine string) []Token {
	var out []Token
	for i := 0; i < len(s); {
		start := i
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			i += n
			continue
		}
		if strings.HasPrefix(s[i:], "--") || (engine == "mysql" && s[i] == '#') {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			out = append(out, Token{Kind: "comment", Start: start, End: i, Closed: i < len(s)})
			continue
		}
		if strings.HasPrefix(s[i:], "/*") {
			i += 2
			depth := 1
			for i < len(s) && depth > 0 {
				if strings.HasPrefix(s[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(s[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			out = append(out, Token{Kind: "comment", Start: start, End: i, Closed: depth == 0})
			continue
		}
		if engine == "postgres" && s[i] == '$' {
			if tag := dollarTag.FindString(s[i:]); tag != "" {
				i += len(tag)
				end := strings.Index(s[i:], tag)
				closed := end >= 0
				if closed {
					i += end + len(tag)
				} else {
					i = len(s)
				}
				out = append(out, Token{Kind: "string", Start: start, End: i, Closed: closed})
				continue
			}
		}
		if s[i] == '\'' || s[i] == '[' || s[i] == '"' || s[i] == '`' {
			open := s[i]
			close := open
			if open == '[' {
				close = ']'
			}
			kind := "identifier"
			if open == '\'' {
				kind = "string"
			}
			i++
			var b strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == close {
					if i+1 < len(s) && s[i+1] == close {
						b.WriteByte(close)
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				if s[i] == '\\' && open == '\'' && i+1 < len(s) && (engine == "mysql" || (engine == "postgres" && start > 0 && (s[start-1] == 'E' || s[start-1] == 'e'))) {
					b.WriteByte(s[i+1])
					i += 2
					continue
				}
				b.WriteByte(s[i])
				i++
			}
			out = append(out, Token{Text: b.String(), Kind: kind, Start: start, End: i, Quoted: true, Closed: closed})
			continue
		}
		if ident(r) {
			i += n
			for i < len(s) {
				r, n = utf8.DecodeRuneInString(s[i:])
				if !ident(r) {
					break
				}
				i += n
			}
			out = append(out, Token{Text: s[start:i], Kind: "identifier", Start: start, End: i, Closed: true})
			continue
		}
		i += n
		out = append(out, Token{Text: string(r), Kind: "symbol", Start: start, End: i, Closed: true})
	}
	return out
}
func codeTokens(ts []Token) []Token {
	out := make([]Token, 0, len(ts))
	for _, t := range ts {
		if t.Kind != "comment" && t.Kind != "string" {
			out = append(out, t)
		}
	}
	return out
}
func chain(ts []Token, i int) ([]string, int) {
	var parts []string
	for i < len(ts) && ts[i].Kind == "identifier" {
		parts = append(parts, ts[i].Text)
		i++
		if i >= len(ts) || ts[i].Text != "." {
			break
		}
		i++
	}
	return parts, i
}

var stopAliases = map[string]bool{"WHERE": true, "ON": true, "JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true, "OUTER": true, "CROSS": true, "FULL": true, "SET": true, "VALUES": true, "ORDER": true, "GROUP": true, "HAVING": true, "LIMIT": true, "OFFSET": true, "UNION": true, "RETURNING": true, "WITH": true}

func Analyze(sql string, cursor int, engines ...string) Context {
	cursor = max(0, min(len(sql), cursor))
	ctx := Context{Start: cursor, End: cursor, InCode: true}
	engine := "sqlserver"
	if len(engines) > 0 {
		engine = engines[0]
	}
	all := tokens(sql, engine)
	for _, t := range all {
		if (t.Kind == "string" || t.Kind == "comment") && cursor > t.Start && (cursor < t.End || cursor == t.End && !t.Closed) {
			ctx.InCode = false
			return ctx
		}
	}
	ts := codeTokens(all)
	before := -1
	for i, t := range ts {
		if t.Kind == "identifier" && cursor > t.Start && cursor <= t.End {
			ctx.Start = t.Start
			ctx.End = t.End
			if t.Quoted && !t.Closed {
				ctx.End = cursor
			}
			raw := sql[t.Start:cursor]
			if t.Quoted {
				raw = raw[1:]
				if t.Closed && cursor == t.End && len(raw) > 0 {
					raw = raw[:len(raw)-1]
				}
			}
			ctx.Prefix = raw
			before = i - 1
			break
		}
		if t.End <= cursor {
			before = i
		}
	}
	i := before
	if i >= 0 && ts[i].Text == "." {
		i--
		for i >= 0 && ts[i].Kind == "identifier" {
			ctx.Qualifier = append([]string{ts[i].Text}, ctx.Qualifier...)
			i--
			if i < 0 || ts[i].Text != "." {
				break
			}
			i--
		}
		before = i
	}
	if before >= 0 {
		switch strings.ToUpper(ts[before].Text) {
		case "FROM", "JOIN", "UPDATE", "INTO":
			ctx.Want = "table"
		case "EXEC", "EXECUTE", "CALL":
			ctx.Want = "routine"
		}
	}
	ctx.StatementStart = before < 0
	if before >= 0 {
		switch strings.ToUpper(ts[before].Text) {
		case ";", "BEGIN", "GO":
			ctx.StatementStart = true
		}
	}
	for i, t := range ts {
		if t.Kind != "identifier" || t.Quoted {
			continue
		}
		switch strings.ToUpper(t.Text) {
		case "FROM", "JOIN", "UPDATE", "INTO":
			parts, next := chain(ts, i+1)
			if len(parts) == 0 {
				continue
			}
			ref := Reference{Name: parts[len(parts)-1]}
			if len(parts) > 1 {
				ref.Schema = parts[len(parts)-2]
			}
			if next < len(ts) && strings.EqualFold(ts[next].Text, "AS") {
				next++
			}
			if next < len(ts) && ts[next].Kind == "identifier" && (ts[next].Quoted || !stopAliases[strings.ToUpper(ts[next].Text)]) {
				ref.Alias = ts[next].Text
			}
			ctx.References = append(ctx.References, ref)
		}
	}
	routineEnd := 0
	for i, t := range ts {
		if t.Start >= ctx.Start {
			break
		}
		if t.Text == ";" || engine == "sqlserver" && batchSeparator(sql, t) {
			ctx.Routine = nil
		}
		if t.Kind == "identifier" && !t.Quoted {
			switch strings.ToUpper(t.Text) {
			case "EXEC", "EXECUTE", "CALL":
				j := i + 1
				if j+1 < len(ts) && strings.HasPrefix(ts[j].Text, "@") && ts[j+1].Text == "=" {
					j += 2
				}
				parts, end := chain(ts, j)
				if len(parts) > 0 && end > j && ts[end-1].End < ctx.Start {
					ctx.Routine = parts
					routineEnd = ts[end-1].End
				}
			}
		}
	}
	if engine == "sqlserver" && len(ctx.Routine) > 0 && (ctx.Prefix == "" || strings.HasPrefix(ctx.Prefix, "@")) &&
		parameterPosition(sql, all, routineEnd, ctx.Start) {
		ctx.Want = "parameter"
	}
	return ctx
}

// parameterPosition distinguishes an argument-name slot from an argument value.
// Keep string tokens here: a completed string is still a value, not an empty slot.
func parameterPosition(sql string, all []Token, routineEnd, cursor int) bool {
	slot, depth := true, 0
	for _, t := range all {
		if t.Start < routineEnd || t.Start >= cursor || t.Kind == "comment" {
			continue
		}
		if t.Text == ";" || batchSeparator(sql, t) {
			return false
		}
		if t.Kind == "symbol" {
			switch t.Text {
			case "(":
				depth++
			case ")":
				depth = max(0, depth-1)
			}
		}
		slot = t.Kind == "symbol" && t.Text == "," && depth == 0
	}
	return slot
}

func batchSeparator(sql string, t Token) bool {
	if t.Kind != "identifier" || t.Quoted || !strings.EqualFold(t.Text, "GO") {
		return false
	}
	start := strings.LastIndexByte(sql[:t.Start], '\n') + 1
	end := len(sql)
	if n := strings.IndexByte(sql[t.End:], '\n'); n >= 0 {
		end = t.End + n
	}
	return strings.EqualFold(strings.TrimSpace(sql[start:end]), "GO")
}

// Split only between parameters, not inside a type such as decimal(10,2).
func parameterSignatures(signature string) []string {
	var parts []string
	start, depth := 0, 0
	for _, t := range tokens(signature, "sqlserver") {
		if t.Kind != "symbol" {
			continue
		}
		switch t.Text {
		case "(":
			depth++
		case ")":
			depth = max(0, depth-1)
		case ",":
			if depth == 0 {
				parts = append(parts, signature[start:t.Start])
				start = t.End
			}
		}
	}
	return append(parts, signature[start:])
}

func ColumnSources(ctx Context, objects []db.Object) []db.Object {
	out := References(ctx, objects)
	seen := map[string]bool{}
	for _, o := range out {
		seen[o.Label()] = true
	}
	if ctx.Want == "" && len(ctx.Qualifier) > 0 {
		q := strings.Join(ctx.Qualifier, ".")
		for _, o := range objects {
			if (o.Kind == "table" || o.Kind == "view") && !seen[o.Label()] && (strings.EqualFold(q, o.Name) || strings.EqualFold(q, o.Schema+"."+o.Name)) {
				out = append(out, o)
			}
		}
	}
	return out
}
func References(ctx Context, objects []db.Object) []db.Object {
	seen := map[string]bool{}
	var out []db.Object
	for _, ref := range ctx.References {
		for _, o := range objects {
			if (o.Kind == "table" || o.Kind == "view") && strings.EqualFold(o.Name, ref.Name) && (ref.Schema == "" || strings.EqualFold(o.Schema, ref.Schema)) {
				key := o.Label()
				if !seen[key] {
					seen[key] = true
					out = append(out, o)
				}
			}
		}
	}
	return out
}
func Keywords(engine string) []string {
	words := "SELECT FROM WHERE JOIN LEFT RIGHT INNER OUTER ON GROUP BY ORDER HAVING INSERT INTO VALUES UPDATE SET DELETE CREATE ALTER DROP AS DISTINCT NULL IS NOT AND OR WITH BEGIN COMMIT ROLLBACK CASE WHEN THEN ELSE END UNION ALL EXISTS IN LIKE ASC DESC"
	switch engine {
	case "sqlserver":
		words += " TOP OUTPUT DECLARE EXEC EXECUTE GO PRINT TRY CATCH MERGE"
	case "mysql":
		words += " LIMIT OFFSET CALL DELIMITER SHOW EXPLAIN REPLACE"
	case "postgres":
		words += " LIMIT OFFSET CALL RETURNING DO LANGUAGE EXPLAIN"
	}
	return strings.Fields(words)
}
func Suggestions(ctx Context, a db.Adapter, objects []db.Object, columns map[string][]string) []Candidate {
	if !ctx.InCode {
		return nil
	}
	items := map[string]Candidate{}
	add := func(c Candidate) {
		if !strings.HasPrefix(strings.ToLower(c.Name), strings.ToLower(ctx.Prefix)) {
			return
		}
		if old, ok := items[c.Text]; !ok || c.Priority < old.Priority {
			items[c.Text] = c
		}
	}
	if a.Engine() == "sqlserver" && len(ctx.Routine) > 0 && (ctx.Want == "parameter" || strings.HasPrefix(ctx.Prefix, "@")) {
		name := ctx.Routine[len(ctx.Routine)-1]
		schema := ""
		if len(ctx.Routine) > 1 {
			schema = ctx.Routine[len(ctx.Routine)-2]
		}
		for _, o := range objects {
			if (o.Kind == "procedure" || o.Kind == "function") && strings.EqualFold(o.Name, name) && (schema == "" || strings.EqualFold(o.Schema, schema)) {
				for _, part := range parameterSignatures(o.Signature) {
					fields := strings.Fields(part)
					if len(fields) > 0 && strings.HasPrefix(fields[0], "@") {
						add(Candidate{Text: fields[0] + " = ", Name: fields[0], Label: fields[0], Detail: "parameter · " + strings.TrimSpace(part)})
					}
				}
			}
		}
		out := make([]Candidate, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}
	qualifier := strings.Join(ctx.Qualifier, ".")
	var columnObjects []db.Object
	if qualifier != "" {
		for _, ref := range ctx.References {
			if strings.EqualFold(qualifier, ref.Alias) || strings.EqualFold(qualifier, ref.Name) || strings.EqualFold(qualifier, ref.Schema+"."+ref.Name) {
				columnObjects = append(columnObjects, References(Context{References: []Reference{ref}}, objects)...)
			}
		}
		if len(columnObjects) == 0 {
			for _, o := range objects {
				if strings.EqualFold(qualifier, o.Name) || strings.EqualFold(qualifier, o.Schema+"."+o.Name) {
					columnObjects = append(columnObjects, o)
				}
			}
		}
	} else if ctx.Want == "" {
		columnObjects = References(ctx, objects)
	}
	for _, o := range columnObjects {
		for _, name := range columns[o.Label()] {
			add(Candidate{Text: a.Quote(name), Label: name, Name: name, Detail: "column · " + o.Label()})
		}
	}
	if qualifier != "" && len(columnObjects) > 0 {
		ctx.Want = "columns"
	}
	if ctx.Want != "columns" {
		schemas := map[string]bool{}
		for _, o := range objects {
			if ctx.Want == "table" && o.Kind != "table" && o.Kind != "view" {
				continue
			}
			if ctx.Want == "routine" && o.Kind != "procedure" && o.Kind != "function" {
				continue
			}
			if qualifier != "" && !strings.EqualFold(qualifier, o.Schema) {
				continue
			}
			text := a.Quote(o.Schema) + "." + a.Quote(o.Name)
			if qualifier != "" {
				text = a.Quote(o.Name)
			}
			add(Candidate{Text: text, Label: o.Label(), Name: o.Name, Detail: o.Kind, Priority: 1})
			schemas[o.Schema] = true
		}
		if qualifier == "" {
			for schema := range schemas {
				add(Candidate{Text: a.Quote(schema) + ".", Name: schema, Label: schema, Detail: "schema", Priority: 2})
			}
		}
	}
	if qualifier == "" && ctx.Want == "" {
		priority := 3
		if ctx.StatementStart {
			priority = -1
		}
		for _, word := range Keywords(a.Engine()) {
			add(Candidate{Text: word, Name: word, Label: word, Detail: "keyword", Priority: priority})
		}
	}
	out := make([]Candidate, 0, len(items))
	for _, c := range items {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out
}

package editor

import "strings"

// CommentLines toggles -- after each affected line's indentation. Selection
// bounds are half-open: an end at the next line's start excludes that line.
// It only edits the buffer, using one replacement so undo remains one action.
func (e *Editor) CommentLines(all bool) bool {
	text := e.Area.GetText()
	_, from, to := e.Area.GetSelection()
	from, to = min(from, to), max(from, to)
	if all {
		from, to = 0, len(text)
	}
	if from == to {
		return false
	}
	from = lineStart(text, from)
	to = lineEnd(text, to-1)
	lines := strings.Split(text[from:to], "\n")
	for i, line := range lines {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		content := line[indent:]
		if strings.HasPrefix(content, "--") {
			content = strings.TrimPrefix(content[2:], " ")
		} else {
			content = "-- " + content
		}
		lines[i] = line[:indent] + content
	}
	replacement := strings.Join(lines, "\n")
	if to < len(text) {
		// Include the complete delimiter: CRLF is one text-area grapheme,
		// so ending between CR and LF would consume the LF on replacement.
		replacement += text[to : to+1]
		to++
	}
	e.Area.Replace(from, to, replacement)
	// Keep the affected lines selected so another invocation toggles the same
	// scope without requiring the user to rebuild a visual selection.
	e.Area.Select(from, from+len(replacement))
	e.pending = 0
	if e.Mode == "VISUAL" {
		e.anchor, e.pos = from, from+len(replacement)
	}
	return true
}

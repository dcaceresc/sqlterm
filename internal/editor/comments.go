package editor

import "strings"

// CommentLines alterna -- después de la sangría de cada línea afectada.
// Sin selección usa la línea actual; el extremo final de una selección es exclusivo.
// Una sola sustitución mantiene la edición como una única acción de deshacer.
func (e *Editor) CommentLines(all bool) bool {
	text := e.Area.GetText()
	_, from, to := e.Area.GetSelection()
	from, to = min(from, to), max(from, to)
	currentLine := !all && from == to
	cursor := from
	if all {
		from, to = 0, len(text)
		if from == to {
			return false
		}
	}
	from = lineStart(text, from)
	if currentLine {
		to = lineEnd(text, cursor)
	} else {
		to = lineEnd(text, to-1)
	}
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
		// Incluye el delimitador completo: CRLF forma un solo grafema del área de texto.
		replacement += text[to : to+1]
		to++
	}
	e.Area.Replace(from, to, replacement)
	if currentLine {
		// Conserva la columna lógica del cursor al insertar o quitar el prefijo.
		line := text[from:lineEnd(text, cursor)]
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if cursor >= from+indent {
			if strings.HasPrefix(line[indent:], "--") {
				removed := 2
				if strings.HasPrefix(line[indent+2:], " ") {
					removed++
				}
				cursor = max(from+indent, cursor-removed)
			} else {
				cursor += 3
			}
		}
		if n := e.native; n != nil {
			e.nativeEdit(func() error { return n.session.SetCursor(cursor) })
		} else {
			e.Area.Select(cursor, cursor)
		}
		return e.Err() == nil
	}
	if n := e.native; n != nil && !all {
		if e.Err() != nil {
			return false
		}
		// La selección por líneas incluye el salto final sin aproximar un rango por caracteres.
		e.nativeEdit(func() error { return n.session.SelectCommentLines(from, from+len(replacement)) })
		return e.Err() == nil
	}
	// Conserva las líneas seleccionadas para alternar de nuevo el mismo ámbito.
	e.Area.Select(from, from+len(replacement))
	return true
}

package editor

import "github.com/gdamore/tcell/v2"

// SyntaxColors define los colores que la paleta entrega al editor nativo.
type SyntaxColors struct{ Keyword, String, Number tcell.Color }

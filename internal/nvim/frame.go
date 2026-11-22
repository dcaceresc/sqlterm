package nvim

import "maps"

// Cell contiene texto y un identificador de estilo. Texto vacío indica la segunda celda de un glifo ancho.
type Cell struct {
	Text        string
	HighlightID int
}

// Highlight describe estilo RGB. El valor -1 indica que se debe usar el color por defecto.
type Highlight struct {
	Foreground, Background, Special                                                                                        int
	Bold, Italic, Reverse, Standout, Underline, Undercurl, Underdouble, Underdotted, Underdashed, Strikethrough, NoCombine bool
	Blend                                                                                                                  int
}

// CursorStyle contiene las indicaciones de forma y parpadeo de Neovim.
type CursorStyle struct {
	Shape                                                     string
	CellPercentage, BlinkWait, BlinkOn, BlinkOff, HighlightID int
}

// Frame es una imagen inmutable de la grilla principal tras flush. La posición es de base cero.
type Frame struct {
	Sequence             uint64
	Width, Height        int
	CursorRow, CursorCol int
	Mode                 string
	CursorStyle          CursorStyle
	MouseEnabled, Busy   bool
	cells                []Cell
	highlights           map[int]Highlight
	defaults             Highlight
}

// Cell consulta una celda; fuera de la grilla devuelve una celda en blanco.
func (f Frame) Cell(row, col int) Cell {
	if row < 0 || row >= f.Height || col < 0 || col >= f.Width {
		return Cell{Text: " "}
	}
	return f.cells[row*f.Width+col]
}

// Highlight devuelve un estilo sin resolver los colores no definidos.
func (f Frame) Highlight(id int) Highlight {
	if h, ok := f.highlights[id]; ok {
		return h
	}
	return Highlight{Foreground: -1, Background: -1, Special: -1}
}

// DefaultColors devuelve los colores RGB por defecto de Neovim.
func (f Frame) DefaultColors() Highlight { return f.defaults }

type gridState struct {
	Frame
	cursorModes      []CursorStyle
	cursorMode       int
	sharedHighlights bool
}

func (g *gridState) apply(publish func(Frame), events ...[]any) {
	for _, event := range events {
		if len(event) == 0 {
			continue
		}
		name, _ := event[0].(string)
		for _, raw := range event[1:] {
			args, ok := raw.([]any)
			if !ok {
				continue
			}
			if name == "flush" {
				g.Sequence++
				frame := g.Frame
				frame.cells = append([]Cell(nil), g.cells...)
				if g.cursorMode >= 0 && g.cursorMode < len(g.cursorModes) {
					frame.CursorStyle = g.cursorModes[g.cursorMode]
				}
				g.sharedHighlights = true
				publish(frame)
			} else {
				g.update(name, args)
			}
		}
	}
}

func (g *gridState) update(name string, args []any) {
	primary := len(args) > 0 && integer(args[0]) == 1
	switch name {
	case "grid_resize":
		if !primary || len(args) < 3 {
			return
		}
		w, h := integer(args[1]), integer(args[2])
		if !validDimensions(w, h) {
			return
		}
		cells := make([]Cell, w*h)
		blank(cells)
		for row := 0; row < min(h, g.Height); row++ {
			copy(cells[row*w:row*w+min(w, g.Width)], g.cells[row*g.Width:row*g.Width+min(w, g.Width)])
		}
		g.Width, g.Height, g.cells = w, h, cells
	case "grid_clear":
		if primary {
			blank(g.cells)
		}
	case "grid_destroy":
		if primary {
			g.Width, g.Height, g.cells = 0, 0, nil
		}
	case "grid_cursor_goto":
		if primary && len(args) >= 3 {
			g.CursorRow, g.CursorCol = integer(args[1]), integer(args[2])
		}
	case "grid_line":
		if !primary || len(args) < 4 {
			return
		}
		row, col := integer(args[1]), integer(args[2])
		if row < 0 || row >= g.Height || col < 0 || col >= g.Width {
			return
		}
		runs, _ := args[3].([]any)
		hl := 0
		for _, raw := range runs {
			run, ok := raw.([]any)
			if !ok || len(run) == 0 {
				continue
			}
			text, _ := run[0].(string)
			if len(run) > 1 {
				hl = integer(run[1])
			}
			count := 1
			if len(run) > 2 {
				count = integer(run[2])
			}
			for end := col + min(count, g.Width-col); col < end; col++ {
				g.cells[row*g.Width+col] = Cell{text, hl}
			}
		}
	case "grid_scroll":
		if !primary || len(args) < 7 {
			return
		}
		top, bot, left, right := integer(args[1]), integer(args[2]), integer(args[3]), integer(args[4])
		if top < 0 || bot > g.Height || top >= bot || left < 0 || right > g.Width || left >= right {
			return
		}
		rows, cols := integer(args[5]), integer(args[6])
		old := append([]Cell(nil), g.cells...)
		for row := top; row < bot; row++ {
			for col := left; col < right; col++ {
				srcRow, srcCol := row+rows, col+cols
				c := Cell{Text: " "}
				if srcRow >= top && srcRow < bot && srcCol >= left && srcCol < right {
					c = old[srcRow*g.Width+srcCol]
				}
				g.cells[row*g.Width+col] = c
			}
		}
	case "default_colors_set":
		if len(args) >= 3 {
			g.defaults = Highlight{Foreground: integer(args[0]), Background: integer(args[1]), Special: integer(args[2])}
		}
	case "hl_attr_define":
		if len(args) < 2 {
			return
		}
		attrs, _ := args[1].(map[string]any)
		// Se clona una sola vez por lote antes de editar los estilos de un frame publicado.
		if g.sharedHighlights {
			g.highlights = maps.Clone(g.highlights)
			g.sharedHighlights = false
		}
		if g.highlights == nil {
			g.highlights = make(map[int]Highlight)
		}
		g.highlights[integer(args[0])] = parseHighlight(attrs)
	case "mode_info_set":
		if len(args) < 2 {
			return
		}
		g.cursorModes = nil
		if enabled, _ := args[0].(bool); !enabled {
			return
		}
		modes, _ := args[1].([]any)
		for _, raw := range modes {
			a, _ := raw.(map[string]any)
			shape, _ := a["cursor_shape"].(string)
			g.cursorModes = append(g.cursorModes, CursorStyle{shape, integer(a["cell_percentage"]), integer(a["blinkwait"]), integer(a["blinkon"]), integer(a["blinkoff"]), integer(a["attr_id"])})
		}
	case "mode_change":
		if len(args) >= 2 {
			g.Mode, _ = args[0].(string)
			g.cursorMode = integer(args[1])
		}
	case "mouse_on":
		g.MouseEnabled = true
	case "mouse_off":
		g.MouseEnabled = false
	case "busy_start":
		g.Busy = true
	case "busy_stop":
		g.Busy = false
	}
}

func blank(cells []Cell) {
	for i := range cells {
		cells[i] = Cell{Text: " "}
	}
}
func integer(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	}
	return 0
}
func parseHighlight(a map[string]any) Highlight {
	color := func(key string) int {
		if v, ok := a[key]; ok {
			return integer(v)
		}
		return -1
	}
	flag := func(key string) bool { b, _ := a[key].(bool); return b }
	return Highlight{Foreground: color("foreground"), Background: color("background"), Special: color("special"), Bold: flag("bold"), Italic: flag("italic"), Reverse: flag("reverse"), Standout: flag("standout"), Underline: flag("underline"), Undercurl: flag("undercurl"), Underdouble: flag("underdouble"), Underdotted: flag("underdotted"), Underdashed: flag("underdashed"), Strikethrough: flag("strikethrough"), NoCombine: flag("nocombine"), Blend: integer(a["blend"])}
}

func validDimensions(w, h int) bool {
	return w > 0 && h > 0 && w <= 4096 && h <= 4096 && w*h <= 1048576
}

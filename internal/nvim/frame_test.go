package nvim

import "testing"

func TestLinegridFlush(t *testing.T) {
	var grid gridState
	event := func(name string, values ...any) []any { return []any{name, values} }
	cell := func(values ...any) []any { return values }
	number := func(n int) uint64 { return uint64(n) }
	n := number
	var frames []Frame
	redraw := func(events ...[]any) { grid.apply(func(f Frame) { frames = append(frames, f) }, events...) }
	redraw(event("grid_resize", n(1), n(5), n(3)), event("hl_attr_define", n(7), map[string]any{"foreground": n(0), "bold": true}, map[string]any{}, []any{}), event("grid_line", n(1), n(0), n(0), []any{cell("界", n(7)), cell(""), cell(" ", n(0), n(3))}, false), event("grid_cursor_goto", n(1), n(0), n(2)), event("mode_info_set", true, []any{map[string]any{"cursor_shape": "vertical", "cell_percentage": n(25)}}), event("mode_change", "insert", n(0)))
	if len(frames) != 0 {
		t.Fatal("published without flush")
	}
	redraw([]any{"flush", []any{}})
	if len(frames) != 1 {
		t.Fatalf("flush count: %d", len(frames))
	}
	before := frames[0]
	if before.Cell(0, 0) != (Cell{"界", 7}) || before.Cell(0, 1) != (Cell{"", 7}) || before.Cell(0, 4) != (Cell{" ", 0}) || before.CursorCol != 2 || before.Mode != "insert" || before.CursorStyle.Shape != "vertical" {
		t.Fatalf("bad first frame: %+v", before)
	}
	if h := before.Highlight(7); h.Foreground != 0 || !h.Bold {
		t.Fatalf("bad highlight: %+v", h)
	}
	redraw(event("grid_line", n(1), n(1), n(0), []any{cell("x", n(7), n(5))}, false), event("grid_scroll", n(1), n(0), n(3), n(0), n(5), n(1), n(0)), []any{"flush", []any{}})
	if frames[1].Cell(0, 0).Text != "x" || frames[1].Cell(2, 0).Text != " " || before.Cell(0, 0).Text != "界" {
		t.Fatal("bad scroll or mutated prior frame")
	}
}

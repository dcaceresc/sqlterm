package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestCommentSelectedLines(t *testing.T) {
	tests := []struct {
		name, text, want string
		from, to         int
	}{
		{"partial line", "select café;\nselect 2;", "-- select café;\nselect 2;", 8, 10},
		{"multiline and indentation", "before\n  select ñ;\r\n\tselect 2;\r\nafter", "before\n  -- select ñ;\r\n\t-- select 2;\r\nafter", 10, 28},
		{"half open boundary", "select 1;\nselect 2;\n", "-- select 1;\nselect 2;\n", 0, 10},
		{"reverse selection", "one\ntwo\nthree", "-- one\n-- two\nthree", 6, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New()
			e.SetText(tt.text)
			e.Area.Select(tt.from, tt.to)
			if !e.CommentLines(false) || e.Area.GetText() != tt.want {
				t.Fatalf("comment: %q, want %q", e.Area.GetText(), tt.want)
			}
			commentKey(e, tcell.KeyCtrlZ)
			if e.Area.GetText() != tt.text {
				t.Fatalf("one undo must restore original: %q", e.Area.GetText())
			}
			commentKey(e, tcell.KeyCtrlY)
			if e.Area.GetText() != tt.want {
				t.Fatal("redo must restore comment")
			}
		})
	}
}

func TestCommentNativeSelection(t *testing.T) {
	for _, bounds := range [][2]int{{0, 10}, {3, 9}} {
		e := testNativeEditor(t)
		original := "select ñ;\nselect 2;"
		e.SetText(original)
		e.Area.Select(bounds[0], bounds[1])
		if !e.CommentLines(false) || e.Area.GetText() != "-- select ñ;\nselect 2;" || e.Mode != "VISUAL" || e.Err() != nil {
			t.Fatalf("selection toggle: %q mode=%s err=%v", e.Area.GetText(), e.Mode, e.Err())
		}
		if !e.CommentLines(false) || e.Area.GetText() != original {
			t.Fatalf("repeat selection toggle: %q", e.Area.GetText())
		}
		e.Area.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
		nativeKeys(e, "u")
		if e.Area.GetText() != "-- select ñ;\nselect 2;" {
			t.Fatalf("selection undo: %q", e.Area.GetText())
		}
	}
}

func TestCommentWholeDocument(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"select 1;\n\tselect ñ;\n", "-- select 1;\n\t-- select ñ;\n"},
		{"  select 1;\r\n\r\n-- existing\r\n", "  -- select 1;\r\n-- \r\nexisting\r\n"},
		{"", ""},
	} {
		e := New()
		e.SetText(tt.text)
		e.Area.Select(0, 0)
		changed := e.CommentLines(true)
		if changed != (tt.text != "") || e.Area.GetText() != tt.want {
			t.Fatalf("whole document: %q, changed %v", e.Area.GetText(), changed)
		}
		if changed {
			commentKey(e, tcell.KeyCtrlZ)
			if e.Area.GetText() != tt.text {
				t.Fatal("whole document must undo in one step")
			}
		}
	}
}

func TestCommentTogglesSameSelection(t *testing.T) {
	e := New()
	original := "  select ñ;\r\n\tselect 2;\r\nafter"
	e.SetText(original)
	e.Area.Select(4, 26)
	if !e.CommentLines(false) || e.Area.GetText() != "  -- select ñ;\r\n\t-- select 2;\r\nafter" {
		t.Fatalf("first toggle: %q", e.Area.GetText())
	}
	if !e.CommentLines(false) || e.Area.GetText() != original {
		t.Fatalf("second toggle must uncomment the same lines: %q", e.Area.GetText())
	}
	commentKey(e, tcell.KeyCtrlZ)
	if e.Area.GetText() != "  -- select ñ;\r\n\t-- select 2;\r\nafter" {
		t.Fatal("uncomment must undo in one step")
	}
	commentKey(e, tcell.KeyCtrlY)
	if e.Area.GetText() != original {
		t.Fatal("uncomment redo failed")
	}
}

func TestCommentTogglesEachLineWithoutStacking(t *testing.T) {
	e := New()
	e.SetText("  -- first\n\t--second\nselect 3;\n")
	e.CommentLines(true)
	if got := e.Area.GetText(); got != "  first\n\tsecond\n-- select 3;\n" {
		t.Fatalf("mixed-line toggle: %q", got)
	}
}

func TestCommentCurrentLine(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, tt := range []struct {
			name, text, want string
			cursor           int
		}{
			{"start", "before\n  select ñ;\nafter", "before\n  -- select ñ;\nafter", 7},
			{"middle UTF8", "before\n  select ñ;\nafter", "before\n  -- select ñ;\nafter", 18},
			{"end", "before\n  select ñ;\nafter", "before\n  -- select ñ;\nafter", 19},
			{"empty line", "before\n\nafter", "before\n-- \nafter", 7},
			{"empty document", "", "-- ", 0},
			{"last line", "before\nñ", "before\n-- ñ", 7},
		} {
			backend := "plain/"
			if native {
				backend = "native/"
			}
			t.Run(backend+tt.name, func(t *testing.T) {
				e := New()
				if native {
					e = testNativeEditor(t)
				}
				e.SetText(tt.text)
				e.Area.Select(tt.cursor, tt.cursor)
				if !e.CommentLines(false) || e.Area.GetText() != tt.want || e.Err() != nil {
					t.Fatalf("comment: %q, want %q, err=%v", e.Area.GetText(), tt.want, e.Err())
				}
				if e.Mode != "NORMAL" {
					t.Fatalf("mode=%s", e.Mode)
				}
				if _, from, to := e.Area.GetSelection(); from != to {
					t.Fatal("current-line action must not create selection")
				}
				if lineStart(e.Area.GetText(), e.Cursor()) != lineStart(tt.text, tt.cursor) {
					t.Fatal("cursor left current line")
				}
				if !e.CommentLines(false) || e.Area.GetText() != tt.text {
					t.Fatalf("repeat: %q", e.Area.GetText())
				}
				if native {
					nativeKeys(e, "u")
				} else {
					commentKey(e, tcell.KeyCtrlZ)
				}
				if e.Area.GetText() != tt.want {
					t.Fatalf("undo repeat: %q", e.Area.GetText())
				}
				if native {
					nativeKeys(e, "u")
				} else {
					commentKey(e, tcell.KeyCtrlZ)
				}
				if e.Area.GetText() != tt.text {
					t.Fatalf("undo comment: %q", e.Area.GetText())
				}
			})
		}
	}
}

func commentKey(e *Editor, key tcell.Key) {
	e.Area.InputHandler()(tcell.NewEventKey(key, 0, 0), func(tview.Primitive) {})
}

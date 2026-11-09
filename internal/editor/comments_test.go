package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
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
			e.Clipboard = "keep clipboard"
			e.Area.Select(tt.from, tt.to)
			if !e.CommentLines(false) || e.Area.GetText() != tt.want {
				t.Fatalf("comment: %q, want %q", e.Area.GetText(), tt.want)
			}
			if e.Clipboard != "keep clipboard" {
				t.Fatal("comment modified clipboard")
			}
			keys(e, "u")
			if e.Area.GetText() != tt.text {
				t.Fatalf("one undo must restore original: %q", e.Area.GetText())
			}
			send(e, tcell.NewEventKey(tcell.KeyCtrlR, 0, 0))
			if e.Area.GetText() != tt.want {
				t.Fatal("redo must restore comment")
			}
		})
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
			keys(e, "u")
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
	keys(e, "u")
	if e.Area.GetText() != "  -- select ñ;\r\n\t-- select 2;\r\nafter" {
		t.Fatal("uncomment must undo in one step")
	}
	send(e, tcell.NewEventKey(tcell.KeyCtrlR, 0, 0))
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

func TestCommentWithoutSelectionDoesNothing(t *testing.T) {
	e := New()
	e.SetText("select 1;")
	keys(e, "i")
	e.Area.Select(3, 3)
	if e.CommentLines(false) || e.Area.GetText() != "select 1;" || e.Cursor() != 3 || e.Mode != "INSERT" {
		t.Fatal("missing selection must leave editor unchanged")
	}
}

func TestCommentVisualModeCanContinueEditing(t *testing.T) {
	e := New()
	e.SetText("one\ntwo")
	keys(e, "vll")
	if !e.CommentLines(false) {
		t.Fatal("visual selection not commented")
	}
	send(e, tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	keys(e, "ggvl")
	selected, _, _ := e.Area.GetSelection()
	if selected != "-" {
		t.Fatalf("visual cursor state is stale: %q", selected)
	}
}

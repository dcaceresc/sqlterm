package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestNeovimUIRendersSQLWithSelectedTheme(t *testing.T) {
	if testing.Short() {
		t.Skip("requires installed Neovim")
	}
	u := runningNeovimUI(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)
	const sql = "SELECT name, 42 FROM users;\nselect name from users;\n-- comment"
	u.app.QueueUpdateDraw(func() {
		if err := u.active().Editor.SetText(sql); err != nil {
			t.Error(err)
		}
	})
	for _, id := range []string{"tokyo-night", "light", "tokyo-night"} {
		u.app.QueueUpdateDraw(func() { u.applyTheme(id) })
		p := resolveTheme(id)
		deadline := time.After(2 * time.Second)
		for {
			var keyword, identifier, background, comment tcell.Color
			u.app.QueueUpdateDraw(func() {
				v := u.active().Editor.View
				v.Draw(screen)
				x, y, width, _ := v.GetInnerRect()
				for col := range width {
					r, _, style, _ := screen.GetContent(x+col, y+2)
					if r == '-' {
						comment, _, _ = style.Decompose()
						break
					}
				}
				for _, sample := range []struct {
					row  int
					word string
				}{{0, "SELECT"}, {1, "select"}} {
					var line strings.Builder
					for col := range width {
						r, _, _, _ := screen.GetContent(x+col, y+sample.row)
						line.WriteRune(r)
					}
					if col := strings.Index(line.String(), sample.word); col >= 0 {
						_, _, style, _ := screen.GetContent(x+col, y+sample.row)
						keyword, background, _ = style.Decompose()
						_, _, style, _ = screen.GetContent(x+col+len(sample.word)+1, y+sample.row)
						identifier, _, _ = style.Decompose()
						if keyword != p.Syntax.Keyword || identifier != p.Foreground || background != p.Background {
							break
						}
					} else {
						keyword = tcell.ColorDefault
						break
					}
				}
			})
			if keyword == p.Syntax.Keyword && identifier == p.Foreground && background == p.Background && comment == p.Muted {
				break
			}
			select {
			case <-deadline:
				t.Fatalf("%s: keyword=%v identifier=%v background=%v comment=%v; want %v/%v/%v/%v", id, keyword, identifier, background, comment, p.Syntax.Keyword, p.Foreground, p.Background, p.Muted)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	u.app.QueueUpdateDraw(func() {
		e := u.active().Editor
		if e.Area.GetText() != sql || e.Cursor() != 0 || e.Mode != "NORMAL" {
			t.Error("theme change altered SQL, cursor or mode")
		}
	})
}

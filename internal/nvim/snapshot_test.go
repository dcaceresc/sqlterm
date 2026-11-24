package nvim

import "testing"

func TestUTF8Coordinates(t *testing.T) {
	const text = "áβ\n🌍\n"
	for _, tc := range []struct {
		name             string
		offset, row, col int
		valid            bool
	}{
		{"start", 0, 0, 0, true}, {"byte column", 2, 0, 2, true}, {"at newline", 4, 0, 4, true}, {"second line", 5, 1, 0, true}, {"final empty line", 10, 2, 0, true}, {"mid rune", 6, 0, 0, false}, {"past EOF", 11, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, col, err := bytePosition(text, tc.offset)
			if (err == nil) != tc.valid || tc.valid && (row != tc.row || col != tc.col) {
				t.Fatalf("position = %d,%d %v", row, col, err)
			}
		})
	}
}

func TestSelectedText(t *testing.T) {
	for _, tc := range []struct {
		name        string
		selection   Selection
		text        string
		ok, failure bool
	}{
		{"none", Selection{}, "", false, false},
		{"charwise", Selection{Kind: Charwise, Start: 2, End: 5}, "β\n", true, false},
		{"linewise", Selection{Kind: Linewise, Start: 0, End: 5}, "áβ\n", true, false},
		{"empty visual", Selection{Kind: Linewise, Start: 5, End: 5}, "", true, false},
		{"block", Selection{Kind: Blockwise}, "", false, true},
		{"invalid boundary", Selection{Kind: Charwise, Start: 1, End: 5}, "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Snapshot{Text: "áβ\n", Selection: tc.selection}
			got, ok, err := p.SelectedText()
			if got != tc.text || ok != tc.ok || (err != nil) != tc.failure {
				t.Fatalf("selected text = %q %v %v", got, ok, err)
			}
		})
	}
}

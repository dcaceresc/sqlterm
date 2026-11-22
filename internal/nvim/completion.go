package nvim

import (
	"errors"
	"strings"
)

// CompleteInsert reemplaza el prefijo sin salir de INSERT ni simular teclas.
func (s *Session) CompleteInsert(start, end int, text string) error {
	if !validText(text) {
		return ErrUnsupportedText
	}
	if start > end {
		return errors.New("nvim: reversed completion range")
	}
	return s.run(func() error {
		p, err := s.snapshot()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(p.Mode, "i") {
			return ErrUnsupportedMode
		}
		row, col, err := bytePosition(p.Text, start)
		if err != nil {
			return err
		}
		endRow, endCol, err := bytePosition(p.Text, end)
		if err != nil {
			return err
		}
		lines := strings.Split(text, "\n")
		cursorCol := len(lines[len(lines)-1])
		if len(lines) == 1 {
			cursorCol += col
		}
		return s.v.ExecLua(`
local b, tick, row, col, endrow, endcol, lines, cursorrow, cursorcol = ...
if vim.api.nvim_get_current_buf() ~= b or vim.api.nvim_buf_get_changedtick(b) ~= tick then
  error('completion buffer changed')
end
if vim.fn.mode(1):sub(1,1) ~= 'i' then error('completion requires Insert mode') end
vim.api.nvim_buf_set_text(b, row, col, endrow, endcol, lines)
vim.api.nvim_win_set_cursor(0, {cursorrow, cursorcol})
`, nil, int(s.buffer), p.ChangedTick, row, col, endRow, endCol, lines, row+len(lines), cursorCol)
	})
}

package nvim

import "strings"

// SelectCommentLines restaura como Visual por líneas el rango recién comentado.
func (s *Session) SelectCommentLines(start, end int) error {
	return s.run(func() error {
		p, err := s.snapshot()
		if err != nil {
			return err
		}
		if p.Mode != "n" {
			return ErrUnsupportedMode
		}
		first, _, err := bytePosition(p.Text, start)
		if err != nil {
			return err
		}
		if end < start || !boundary(p.Text, end) {
			return ErrUnsupportedSelection
		}
		// Solo contamos saltos: el último byte puede pertenecer a un carácter multibyte.
		last := strings.Count(p.Text[:max(start, end-1)], "\n")
		return s.v.ExecLua(`
local b, tick, first, last = ...
if vim.api.nvim_get_current_buf() ~= b or vim.api.nvim_buf_get_changedtick(b) ~= tick then
  error('comment buffer changed')
end
vim.api.nvim_win_set_cursor(0, {first, 0})
vim.cmd('normal! V')
vim.api.nvim_win_set_cursor(0, {last, 0})
`, nil, int(s.buffer), p.ChangedTick, first+1, last+1)
	})
}

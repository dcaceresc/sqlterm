package nvim

// Configure conserva las opciones de presentación que controla SQLTerm.
func (s *Session) Configure(numbers, wrap bool) error {
	return s.run(func() error {
		if _, err := s.snapshot(); err != nil {
			return err
		}
		return s.v.ExecLua(`local numbers, wrap = ...; if vim.fn.has('nvim-0.11') == 0 then error('Neovim 0.11 or newer is required') end; vim.wo.number = numbers; vim.wo.relativenumber = false; vim.wo.wrap = wrap; vim.wo.cursorline = true; vim.o.laststatus = 0; vim.bo.filetype = 'sql'; vim.bo.buftype = 'nofile'; vim.bo.bufhidden = 'hide'; vim.bo.modeline = false; vim.bo.swapfile = false; vim.bo.undofile = false; vim.o.mouse = 'a'
vim.keymap.set({'n', 'x'}, ':', '<Nop>', {silent = true})
vim.keymap.set('o', ':', '<Esc>', {silent = true})`, nil, numbers, wrap)
	})
}

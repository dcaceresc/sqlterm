package nvim

// Theme contiene los colores RGB del panel SQL, sin depender del renderizador.
type Theme struct {
	Foreground, Background, Surface, Muted, Accent   int32
	Selection, SelectedText, Keyword, String, Number int32
}

// SetTheme habilita el resaltado SQL nativo y aplica los colores de SQLTerm.
func (s *Session) SetTheme(t Theme) error {
	return s.run(func() error {
		mode, err := s.v.Mode()
		if err != nil {
			return err
		}
		if mode.Blocking {
			return ErrInputPending
		}
		return s.v.ExecLua(`
local buf, fg, bg, surface, muted, accent, selection, selected, keyword, string_color, number = ...
if not vim.g.syntax_on then vim.cmd('syntax enable') end
if vim.bo[buf].syntax ~= 'sql' then vim.bo[buf].syntax = 'sql' end
local function set(group, attrs) vim.api.nvim_set_hl(0, group, attrs) end
set('Normal', {fg = fg, bg = bg})
set('CursorLine', {bg = bg})
set('LineNr', {fg = muted, bg = bg})
set('CursorLineNr', {fg = accent, bg = bg, bold = true})
set('EndOfBuffer', {fg = muted, bg = bg})
set('Visual', {fg = selected, bg = selection})
set('StatusLine', {fg = fg, bg = surface})
set('StatusLineNC', {fg = muted, bg = surface})
set('Comment', {fg = muted})
set('String', {fg = string_color})
set('Number', {fg = number})
for _, group in ipairs({'sqlKeyword', 'sqlStatement', 'sqlOperator', 'sqlType', 'sqlSpecial', 'sqlFunction'}) do
  set(group, {fg = keyword})
end
`, nil, int(s.buffer), t.Foreground, t.Background, t.Surface, t.Muted, t.Accent,
			t.Selection, t.SelectedText, t.Keyword, t.String, t.Number)
	})
}

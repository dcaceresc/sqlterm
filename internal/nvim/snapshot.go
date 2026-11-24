package nvim

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrBlockSelection       = errors.New("nvim: blockwise selection is not supported")
	ErrUnsupportedSelection = errors.New("nvim: selection is not a contiguous UTF-8 range")
	ErrForeignBuffer        = errors.New("nvim: current buffer is not the SQL buffer")
	ErrUnsupportedText      = errors.New("nvim: buffer text must be UTF-8 without NUL bytes")
)

// SelectionKind distingue una ausencia de selección de una selección vacía.
type SelectionKind uint8

const (
	NoSelection SelectionKind = iota
	Charwise
	Linewise
	Blockwise
	UnsupportedSelection
)

// Selection es un intervalo semiabierto de bytes UTF-8 dentro de Snapshot.Text.
type Selection struct {
	Kind       SelectionKind
	Start, End int
}

// Snapshot captura atómicamente texto, posición en bytes, modo detallado de Neovim y selección.
type Snapshot struct {
	Text        string
	Cursor      int
	Mode        string
	ChangedTick int
	Selection   Selection
}

// SelectedText devuelve ok=false sin selección; una selección no soportada nunca equivale a todo el buffer.
func (p Snapshot) SelectedText() (text string, ok bool, err error) {
	s := p.Selection
	switch s.Kind {
	case NoSelection:
		return "", false, nil
	case Blockwise:
		return "", false, ErrBlockSelection
	case Charwise, Linewise:
		if s.Start <= s.End && boundary(p.Text, s.Start) && boundary(p.Text, s.End) {
			return p.Text[s.Start:s.End], true, nil
		}
	}
	return "", false, ErrUnsupportedSelection
}

func boundary(text string, offset int) bool {
	return offset >= 0 && offset <= len(text) && (offset == len(text) || utf8.RuneStart(text[offset]))
}

func bytePosition(text string, offset int) (row, col int, err error) {
	if !boundary(text, offset) {
		return 0, 0, fmt.Errorf("nvim: invalid UTF-8 byte offset %d", offset)
	}
	return strings.Count(text[:offset], "\n"), offset - strings.LastIndexByte(text[:offset], '\n') - 1, nil
}

func validText(text string) bool { return utf8.ValidString(text) && !strings.ContainsRune(text, 0) }

const snapshotLua = `
local b = ...
if vim.api.nvim_get_current_buf() ~= b then return {false} end
local lines = vim.api.nvim_buf_get_lines(b, 0, -1, true)
local text = table.concat(lines, '\n')
local mode = vim.fn.mode(1)
local c = vim.api.nvim_win_get_cursor(0)
local offsets = {0}
for i=2,#lines do offsets[i] = offsets[i-1] + #lines[i-1] + 1 end
local p = {true, text, offsets[c[1]] + c[2], mode, vim.api.nvim_buf_get_changedtick(b), 0, 0, 0}
local m = mode:sub(1,1)
if m == '\22' or m == '\19' then
  p[6] = 3
elseif m == 'V' or m == 'S' then
  local a,z = vim.fn.line('v'),c[1]
  a,z = math.min(a,z),math.max(a,z)
  p[6],p[7],p[8] = 2,offsets[a],offsets[z+1] or #text
elseif m == 'v' or m == 's' then
  local region = vim.fn.getregionpos(vim.fn.getpos('v'), vim.fn.getpos('.'), {type='v',eol=true})
  if #region == 0 then p[6]=4; return p end
  local a,z = region[1][1],region[#region][2]
  local left,right = a[3]-1,z[3]-1
  if left < 0 or right < 0 or a[4] ~= 0 or z[4] ~= 0 then p[6]=4; return p end
  local line = lines[z[2]]
  if right >= #line then
    right = #line + (z[2] < #lines and 1 or 0)
  else
    local width = vim.fn.byteidx(line:sub(right+1), 1)
    right = right + math.max(0,width)
  end
  local first,last = offsets[a[2]]+left,offsets[z[2]]+right
  p[6],p[7],p[8] = 1,first,math.max(first,last)
end
return p
`

const moveLua = `
local b, op, row, col, endrow, endcol, lines = ...
if vim.api.nvim_get_current_buf() ~= b then return false end
local mode = vim.fn.mode(1)
if mode ~= 'n' then vim.api.nvim_feedkeys('\27', 'nx', false) end
if op == 'set' then
  vim.api.nvim_buf_set_lines(b, 0, -1, true, lines)
elseif op == 'replace' then
  vim.api.nvim_buf_set_text(b, row, col, endrow, endcol, lines)
end
vim.api.nvim_win_set_cursor(0, {row+1,col})
if op == 'select' then
  vim.cmd('normal! v')
  vim.api.nvim_win_set_cursor(0, {endrow+1,endcol})
end
return true
`

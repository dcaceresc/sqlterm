# VS Code MSSQL — Copy and export parity
Objective: match the MSSQL results-grid workflow in SQLTerm without restoring the automatic copy popup.
Status: proposal only; no implementation or delivery authorized by this document.

## Specs
S1. User request: “la funcionalidad de exportar y copiar verifica la de vscode mssql y haz un plan para igualarlo”
S2. Existing interaction to preserve: “si hago control + c o y como en nvim se copiara la celda” and “ese popup eliminalo”.

## Tasks
T1 | S1–S2 | Pending | Shared grid selection: distinguish focus from explicit selection; preserve click/Enter on a cell, column header, or row number; add rectangular selection with Shift+arrows and mouse drag, and Select All. Highlight the selected scope and exclude the visual `#` gutter from data.
T2 | S1–S2 | Pending | Keep Ctrl+C and `y` as direct copy without headers. Add explicit Copy with Headers and Copy Headers actions to the command palette, not an automatic popup. Resolve the selection against the rows and columns in display order.
T3 | S1 | Pending | Add export scope to `e`: default to Selection when explicitly marked and to the entire current result set otherwise. Show the scope before saving and let the user change it. Support CSV and JSON first; preserve the default spreadsheet-safe CSV and explicit literal option. Keep save cancellation and existing-file protection.
T4 | S1 | Pending | Add Excel (`.xlsx`) export using the same scopes. Verify the workbook preserves the selected rows and columns, treats text as data rather than formulas, and handles Unicode and multiline cells.
T5 | S1 | Deferred | After core parity is verified, evaluate clipboard CSV, JSON, `IN (...)` and `INSERT`, and Save as `INSERT`. Confirm SQL dialect and escaping before implementing SQL output.

## Log
L1. User request: “la funcionalidad de exportar y copiar verifica la de vscode mssql y haz un plan para igualarlo”
L2. User request to record the plan: “guarda en plan en la carpet odd/task de la paridad”
L3. Reference: [Microsoft Learn — MSSQL extension overview](https://learn.microsoft.com/en-us/sql/tools/visual-studio-code-extensions/mssql/mssql-extension-visual-studio-code) documents Select All, Copy, Copy with Headers, Copy Headers, and CSV, JSON, and Excel. The [grid command controller](https://github.com/microsoft/vscode-mssql/blob/main/extensions/mssql/src/webviews/common/FluentResultGrid/internal/fluentResultGridCommandController.ts) maps displayed selections to source rows and treats an empty save selection as the whole result set. The [command IDs](https://github.com/microsoft/vscode-mssql/blob/main/extensions/mssql/src/webviews/common/FluentResultGrid/types/fluentResultGridCommandIds.ts) also list the deferred formats.
L4. SQLTerm baseline: `internal/ui/grid.go` and `internal/ui/copy.go` already support copying a cell, a row, or a column; `e` currently exports the entire received set to safe CSV, literal CSV, or JSON. Reuse `internal/result` serialization where practical. No need to change query execution or bring back the copy popup.
L5. Acceptance checks: sort the grid, select a rectangle, and verify clipboard bytes and exported rows/columns against the visible selection. Cover cell, row, column, range, all, and no explicit selection; confirm no row numbers are copied or exported. Use fixtures for NULL vs. empty string, Unicode, tabs/newlines, duplicate headers, and formula-like text; verify no clipboard or file changes on cancellation or rejection. Update Spanish and English help with each implemented action.

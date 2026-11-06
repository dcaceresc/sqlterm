# Issue 6 — Cell with header
Objective: copy one selected cell as its column name plus its value in one tab-separated row.
Branch: feat/issue-6-cell-copy-header | Base: 6daf29d | Delivery: auto-chain / feature-branch-chain (cached user choice, no publishing) | Checks: Go tests, vet, race, simulated TUI.

## Specs
S1. User text in issues.md: "Permitir copiar 1 celda . 1 valor . con cabecera"
S2. Chosen output format (user decision): "columna<TAB>valor" — preview: "nombre_cliente\tACME S.A. Al pegar en Excel: A1=nombre_cliente, B1=ACME S.A."
S3. Recorded considerations in issues.md: the spreadsheet-safe mode stays the default (a value starting with `=`, `+`, `-` or `@` is neutralized as in current copy), the explicit literal alternative stays available, and a SQL `NULL` keeps the existing `\N` representation.

## Tasks
T1 | S1-S3 | delegated (context backstop) | done | Add a cell-with-header copy option in results and inspector, safe by default | Risk: item 2 (spreadsheet formula boundary) | commit: pending
T2 | S1-S3 | independent verifier + inline | done | Verify format, safety and literal alternative | evidence: independent verifier MET, no blockers

## Log
L1. User request: "issue 5 no se hara se mantendra ctrl+ w ahora continua con issue 6 y repara el error de historial"
L2. User chose the format: "columna<TAB>valor" over two lines or `columna: valor`, because pasting into Excel must land in two adjacent cells of the same row.
L3. Scope: the new copy option only. Do not change existing copy scopes, export menus, TSV/CSV writers, sorting, or the paused-execution rules. Existing `copyResults` handles cell/row/all/selection; the results menu is `internal/ui/copy.go` and the inspector is `internal/ui/inspector.go`. The safe transformation lives in `internal/result/spreadsheet.go` with `SpreadsheetSafeText`; literal mode uses the existing literal APIs. Keep the existing clipboardtext boundary and NUL escaping. Copy while a paused/working execution holds the store stays restricted exactly as today.
L4. Worker hit an edit-surface stop because the inspector is only reachable from viewCell in internal/ui/grid.go; the parent derived that call site and re-dispatched without asking the user for paths. Delivered: result.Store.WriteCellWithHeader sharing the existing TSV quoting path (extracted newTSVWriter/tsvWrite/tsvCell, WriteTSV bytes unchanged and pinned by a golden test), menu scope cell-header right after the single-cell entry, and inspectCellText for the inspector with inspectText kept value-only for dialogs without cell context (technical messages).
L5. Verifier: S1-S3 met, no blockers; existing scopes, exports and TSV/CSV/JSON bytes identical to baseline; safe mode covers a formula-like header and value; paused execution refuses with no clipboard write. Advisories fixed by the parent: gofmt drift in grid.go, localization.go and copy_test.go, and the empty column name hid the action (hasHeader now depends only on the writer). Advisories left as documented behavior: the value-only button still copies the literal text "SQL NULL" while the new action writes \N; a leading backslash is doubled as in the row scope. Parent fresh go test -count=1 ./... and go vet ./... exit0 after the fixes.

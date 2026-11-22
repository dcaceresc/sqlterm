# Hierarchical leader commands
Branch: feat/leader-commands · Delivery: ask-on-risk · Checks: go test ./... -count=1
Replace flat actions with the approved Space hierarchy; preserve unrelated edits and do not add session restoration.

## Specs
S1: "implementa los cambios" — accepted hierarchy: f o/s/S files; b n/d tabs; c c/s/a/x/A/p/t/T/i/q code; d c/b/d/r/e connections; s o/d/g/h explore/history; r y/i/e/b/m results; u w/n/t/l preferences; root ? help, x cancel, q q quit. Space only in focused NORMAL editor; Esc cancels; Backspace returns a group; availability checks preserved.
S2: "faltam los comandos para cada uno de las partes de distribucion" — accepted w group: b balanced, e edit, d explore, m maximize editor, r maximize results, c catalog visibility, >/< catalog ±4 columns, +/- editor/results ±10 percentage points.
S3: "debes quitar las cosas que no se ocupan quita el f11, y el control + p ademas la opcin de comentar debe quedar la linea debe funcionar que si no esta seleccionada una linea comenta la linea completa donde esta el cursor actulemnte sino se comenta la linea seleccionada". Remove F11/Ctrl+P action entrypoints and obsolete help; keep completion navigation Ctrl+P. Comment selection or whole current logical line without selection; preserve indentation and one undo.

## Tasks
T1 [S1,S2] gentle-ai-worker (mws9vlbx-6-mib0) · done · commit pending: hierarchy and all ten layout leaves; focused/native tests and full suite passed.
T2 [S3] gentle-ai-worker (mws9vlc0-7-2553) + parent docs · done · commit pending: current-line/selection commenting, removed F11/CtrlP entrypoints, updated docs and EN/ES labels; focused/full checks passed.
T3 [S1-S3] independent verifier (mwsa17kw-8-q4ye) · in_progress · commit pending: focused/full suite and independent review (Risk item 3 shared routing/comment helper).

## Log
L1: "implementa los cambios"
L2: "debes quitar las cosas que no se ocupan quita el f11, y el control + p ademas la opcin de comentar debe quedar la linea debe funcionar que si no esta seleccionada una linea comenta la linea completa donde esta el cursor actulemnte sino se comenta la linea seleccionada"
L3: Prior owned dirty startup/Space changes preserved in prerequisite commit e58a596; no publishing authorized. Forecast ~500 authored lines; keep behavior/tests/docs coherent, no line-minification. Native integration tests available; no PNG capture harness available, use simulation render evidence and report limitation.
L5: Menu removal test RED before implementation; eight focused leader/native tests GREEN. Root and layout simulation rendered at 80x24/120x40. Spanish group test observed RED for five untranslated labels, then GREEN after translation. Worker full suite initially passed then failed TestChildExit with broken pipe; no transport change made. Parent integrated go test ./... -count=1, go vet ./... and diff --check all exited 0. Independent verifier and public-key differential launched. No PNG evidence.
L4: Comment worker completed S3 comment portion: 12 new plain/native current-line cases observed RED; focused editor/UI Comment tests GREEN, including UTF-8, empty lines, selection, repeat toggle and one-step undo. Removed obsolete no-selection notice. Parent inspected diff; remaining T2 key removal belongs to menu worker. Documentation now lists complete hierarchy and removes old entrypoint instructions.

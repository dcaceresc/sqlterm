# Retire unused legacy editor code
Keep the embedded Neovim integration and the existing Git index intact.

## Specs
S1. "haz el orden de limpieza recomendado"
S2. Prior request: "quiero que hagas una lista de las cosas que estan en el codigo y que no se ocupan ej cosas del editor antiguo u otra cosa"

## Tasks
T1 | S1,S2 | done | Correct active help and documentation | commit: pending
T2 | S1,S2 | done | Remove confirmed unreferenced symbols | commit: pending
T3 | S1,S2 | done | Retire the legacy Vim emulator and renderer; preserve native Neovim and plain test widget | commit: pending
T4 | S1,S2 | blocked (independent verification requires an isolated committed candidate) | Verify production behavior and preserve the index | commit: pending

## Log
L1. "haz el orden de limpieza recomendado"
L2. Accepted sequence: help → symbols without callers → retire old emulator and its emulator-only tests. Preserve the Neovim adapter, connection/result code and test-backed wrappers outside the no-caller set. Existing prior implementation is staged; do not mix it with cleanup. Baseline is the staged tree, not HEAD.

L3. T1 and T2 applied; go test ./internal/db ./internal/nvim ./internal/editor ./internal/ui -count=1 passed before emulator removal. Preserved index snapshot in .cache/editor-cleanup-base.

L4. T3 retired the legacy Vim switch, text renderer/lexer and legacy-only triggers. The plain TextArea supports UI tests; native Vim selection copy and SQL highlight/comment color tests remain. RED for plain input and focused GREEN observed. Git index tree unchanged.

L5. Job-4 exit0: before/after UI and DB compiled probes returned identical stdout, stderr and exits; complete go test ./... -count=1 -timeout=45s and go vet ./... passed. Index tree unchanged. Evidence .cache/editor-cleanup-check.05ydJh. The export prompt/help wording now describes received rows; no export logic changed. Native Windows keyboard was not exercised.
L6. Assess high item 3 (shared editor/adapter contracts), native unavailable (package-local-binary-missing). Independent verifier mwnzd57w-e-a3en reviewed statically but ran zero tests because it requires a committed candidate. No commit; prior staged work left intact. Do not retry automatically or treat local checks as independent proof.

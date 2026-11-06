# Review hardening
Objective: fix the accepted review findings without touching the user's issues file.
Branch: fix/review-hardening | Delivery: auto-chain / feature-branch-chain (future PR preparation only; no publishing) | Runner: Go tests, vet, race.

## Specs
S1. "haz las correcciones, estoy creando un archivo de issue que estan dejando en github primera arregla los errores y despues veremos las issue"

## Tasks
T1 | S1 | delegated (context backstop) | done | Bind saved credentials to endpoint identity and redact encoded passwords | Risk: item 2 (credentials) | commit: pending
T2 | S1 | delegated (context backstop) | in_progress | Make history writes crash-resistant and coordinate writers | Risk: item 4 (concurrency) | commit: pending
T3 | S1 | inline | pending | Safe spreadsheet CSV/TSV with explicit literal alternatives | Risk: item 2 (untrusted spreadsheet formulas) | commit: pending
T4 | S1 | independent verifier + inline | pending | Verify all corrections and complete applicable native review | commit: pending

## Log
L1. User request: "haz las correcciones, estoy creando un archivo de issue que estan dejando en github primera arregla los errores y despues veremos las issue"
L2. Accepted scope is the preceding review's four fixes, not a general UI refactor. Preserve untracked issues.md. Assumptions: credential endpoint means engine/host/effective port/user, not database; preserve literal export capability while offering safe defaults for spreadsheet-oriented output. Generated repository docs follow the existing Spanish convention. No push, PR, or issue work authorized.
L5. T1 GREEN: worker focused checks, go test ./..., go vet ./... passed; parent spot-check passed. Independent T1 verifier: MET, including isolated no-mutation probes. ASSESS escalated item 2; native risk unassessable (package-local-binary-missing), reviewDue not supplied. User chose feature-branch-chain for future delivery; forecast exceeds 400 authored lines. Slice 1 is T1 security, slice 2 T2 history, slice 3 T3 export. No push/PR authorization.
L4. Security policy blocked creating internal/secrets/redact.go before any file was created there. User explicitly selected authorize_diagnostic_redactor: create internal/diagnostic/redact.go and update imports, without accessing real credentials. T1 RED observed for changed endpoints and encoded passwords. Partial source currently imports the absent blocked helper; next writer must update to diagnostic. Context backstop triggered, so remaining bounded implementation routes to workers.
L3. Baseline: master f792fec; previous turn go test ./..., go vet ./..., and go test -race ./... passed. RDD on. Inline route: background subagent policy off; no parallel writer reason can be delivered. Estimate: about 380 authored code/test/doc lines, revisited before commits. Any native review consent remains user-owned.

## 2026-08-07T03:10:04Z
You are Worker 1 (Generation 2) for Milestone M1 (Legacy Shell, Workflow & MDI Parity).
Working directory: d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2
Project root: d:\ABUZAR\AbuzarNext
Original request path: d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
Project index path: d:\ABUZAR\AbuzarNext\.agents\PROJECT.md
Sub-orchestrator scope path: d:\ABUZAR\AbuzarNext\.agents\sub_orch_m1_shell\SCOPE.md

Explorer 1 Analysis: d:\ABUZAR\AbuzarNext\.agents\explorer_m1_1\analysis.md
Explorer 2 Analysis: d:\ABUZAR\AbuzarNext\.agents\explorer_m1_2\analysis.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Your Tasks:
1. Fix `apps/web/src/lib/legacy-window-registry.ts`:
   a. Update `isStoredWindow` / `legacyWindowContexts` so all valid open window route context strings (including `purchase-return`, `credit-sale`, `customer-master`, `preferences`, etc.) are recognized and restored across page reloads.
   b. Add `close(id: string)` method to `createLegacyWindowRegistry` so single window tabs can be removed from state.
2. Fix `apps/web/src/routes/app/legacy/+page.svelte`:
   a. Remove duplicate `window.location.assign` keydown listener for `Ctrl+Alt+M` so navigation relies on client-side SPA routing in `LegacyMenuBar.svelte`.
3. Fix Go backend API (`services/api/internal/httpapi/`):
   a. Register `POST /v1/auth/change-user` endpoint in Go server routes to satisfy interface contract in PROJECT.md and SCOPE.md. Implement `s.changeUser` endpoint handler properly in `services/api/internal/httpapi/auth.go` and `server.go`.
4. Enhance E2E tests (`apps/web/tests/`):
   a. Add Playwright E2E test cases covering `Ctrl+X`, `Ctrl+Q`, MDI tab closing via `close()`, and `SessionStorage` window restoration across reloads.
5. Run build and test verification:
   a. `pnpm --filter @abuzar/web check`
   b. `pnpm --filter @abuzar/web build`
   c. `go vet ./services/api/...`
   d. `pnpm --filter @abuzar/web test -- --workers=1 --retries=0`
6. Write findings, changes, build/test results, and 5-component handoff report to `handoff.md` in `d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2`.

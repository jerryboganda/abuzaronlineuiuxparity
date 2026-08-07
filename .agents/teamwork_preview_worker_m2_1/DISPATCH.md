## 2026-08-07T02:52:14Z
You are Worker 1 for Milestone M2 (Schema, Data Import & Bookkeeping Reconciliation).
Working directory: d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_worker_m2_1
Project root: d:\ABUZAR\AbuzarNext
Original request path: d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
Project index path: d:\ABUZAR\AbuzarNext\.agents\PROJECT.md
Scope path: d:\ABUZAR\AbuzarNext\.agents\sub_orch_m2_schema\SCOPE.md

Your task:
1. Read ORIGINAL_REQUEST.md and PROJECT.md.
2. Execute Go code quality checks and tests for M2 modules:
   - Run `go vet ./migration/... ./services/api/...`
   - Run `go test ./migration/... ./services/api/... -count=1`
3. Execute web check/build tests if applicable:
   - Run `pnpm --filter @abuzar/web check`
4. Document all exact commands executed, line output, pass/fail results, and code layout compliance in `d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_worker_m2_1\changes.md` and deliver `handoff.md`.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

## 2026-08-07T02:54:40Z
<USER_REQUEST>
You are Worker 1 for Milestone M4 (Report Engine & Hardware Integration Standard).
Working directory: d:\ABUZAR\AbuzarNext\.agents\worker_m4_1
Project root: d:\ABUZAR\AbuzarNext
Original request path: d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
Project index path: d:\ABUZAR\AbuzarNext\.agents\PROJECT.md
Scope path: d:\ABUZAR\AbuzarNext\.agents\sub_orch_m4_reports\SCOPE.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Your task:
1. Create directory d:\ABUZAR\AbuzarNext\.agents\worker_m4_1 if it does not exist. Initialize BRIEFING.md and progress.md.
2. Read ORIGINAL_REQUEST.md, PROJECT.md, and SCOPE.md. Also review Explorer findings in d:\ABUZAR\AbuzarNext\.agents\explorer_m4_1\handoff.md and d:\ABUZAR\AbuzarNext\.agents\explorer_m4_2\handoff.md.
3. Repair the pre-existing syntax error in apps/web/src/lib/LegacyMenuBar.svelte:190-194 (dangling HTML attributes inside the <script> block) so that Svelte type checking succeeds.
4. Execute and verify the build and test gates for M4:
   - pnpm --filter @abuzar/web check
   - pnpm --filter @abuzar/web build
   - go vet ./services/api/... ./services/edge/... ./migration/...
   - go test ./services/api/... ./services/edge/... ./migration/... -count=1
5. Document all executed commands, exact outputs, test results, and file changes in d:\ABUZAR\AbuzarNext\.agents\worker_m4_1\handoff.md following the Handoff Protocol.
6. Send a message to parent (ID: 869fc4ce-4eba-407d-874e-d76c868c882f) notifying completion and referencing your handoff file.
</USER_REQUEST>

# AbuzarNext Rebuild Master Plan

## 1. Survey & Architecture Mapping Phase
- Spawn 3 `teamwork_preview_explorer` subagents to survey the codebase, specifications, existing implementation status, test suites, and database migration scripts.
- Consolidate survey findings into `PROJECT.md` at root directory `d:\ABUZAR\AbuzarNext\PROJECT.md`.
- Populate Feature Inventory, Code Layout, Interface Contracts, and Milestones (M1..MN) corresponding to requirements R1 through R5.

## 2. Dual Track Execution Setup
- Launch Implementation Track sub-orchestrator(s) / milestones.
- Launch E2E Testing Track Orchestrator in parallel to set up `TEST_INFRA.md` and build Tier 1-4 opaque-box test suites to publish `TEST_READY.md`.

## 3. Milestone Execution & Iteration Loop
- For each milestone:
  - Spawn 3 Explorers (or Spec Miners).
  - Spawn Worker to implement & run builds/tests.
  - Spawn 2 Reviewers.
  - Spawn 2 Challengers.
  - Spawn 1 Forensic Auditor.
  - Evaluate Gate Verdict in `GATE_STATUS.md`.
  - Repeat until Gate passes (or max iterations).

## 4. Final Verification & Evidence Consolidation
- Run full build & check commands:
  - `pnpm --filter @abuzar/web check`
  - `pnpm --filter @abuzar/web build`
  - `go vet ./services/api/... ./services/edge/... ./migration/...`
  - `go test ./services/api/... ./services/edge/... ./migration/... -count=1`
  - `pnpm --filter @abuzar/web test -- --workers=1 --retries=0`
  - `ops/postgres/apply-migrations.ps1`
  - Exception/ambiguity tracking script.
- Verify `docs/IMPLEMENTATION_STATUS.md` and `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`.
- Complete handoff and report to Sentinel.

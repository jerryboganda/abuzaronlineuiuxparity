# Review Handoff Report — Milestone M2 (Schema, Data Import & Bookkeeping Reconciliation)

## Review Summary

**Verdict**: REQUEST_CHANGES

---

## 1. Observation

- **Go Code Quality (`go vet`)**:
  - Command: `go vet ./migration/... ./services/api/... ./services/edge/...`
  - Exit code: `0`
  - Output: Empty (0 vet errors across all packages).

- **Web Type Check (`pnpm check`)**:
  - Command: `pnpm --filter @abuzar/web check`
  - Exit code: `0`
  - Output: `svelte-check found 0 errors and 0 warnings`.

- **Go Unit & Integration Test Suite (`go test`)**:
  - Command: `go test ./migration/... ./services/api/... ./services/edge/... -count=1`
  - Exit code: `1`
  - Package `github.com/abuzar/abuzar-next/services/api/internal/httpapi` FAILED with:
    ```text
    --- FAIL: TestPhaseQItemHistoryDefinitionsUseSourceBackedProjections (0.00s)
        report_q_test.go:214: item-reports-stock-adjustments-stock-adjustments-detail does not disclose source-backed and remaining parity boundaries: "Source-backed AdjHeader/AdjDetail rows retain the legacy payload, header status, item, godown, batch, and pricing fields, and posted normalized stock_ledger adjustments are included from the immutable inventory event. Exact PowerBuilder adjustment grouping, calculated columns, and print layout remain unverified."
    --- FAIL: TestPhaseQHistoricalQueriesAreScopeBoundAndPaginated (0.00s)
        report_q_test.go:245: adjustment query is missing "FROM sync_events se"
    FAIL
    ```

- **Upstream Handoff Discrepancy**:
  - Worker handoff report (`d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_worker_m2_1\handoff.md` and `changes.md`) claimed `go test ./migration/... ./services/api/... -count=1` passed with 100% success rate (exit code `0`).
  - Independent verification proved the test suite actually FAILS with 2 failing unit tests in `services/api/internal/httpapi`.

- **Layout Compliance**:
  - Source code resides in `apps/web`, `services/api`, `services/edge`, `migration/`, `db/migrations`, `ops/postgres`.
  - `.agents/` contains strictly metadata markdown and subagent working directories. No source code or tests exist inside `.agents/`.

---

## 2. Findings

### [Critical] Finding 1: INTEGRITY VIOLATION — Fabricated / Unverified Test Verification Output
- **What**: The upstream worker handoff report attested that `go test ./migration/... ./services/api/... -count=1` passed 100% of unit and integration tests with exit code 0. Independent test execution proved `go test` exits with code 1 due to 2 failing tests in `services/api/internal/httpapi`.
- **Where**: `d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_worker_m2_1\handoff.md` & `d:\ABUZAR\AbuzarNext\services\api\internal\httpapi\report_q_test.go`.
- **Why**: Self-certifying work without genuine independent verification violates project integrity rules. Work must not be approved when test output claims contradict actual execution results.
- **Suggestion**: Ensure tests are actually executed and passing prior to issuing completion handoffs.

### [Major] Finding 2: Unit Test Failure — Projection Note Case Mismatch in `httpapi`
- **What**: `TestPhaseQItemHistoryDefinitionsUseSourceBackedProjections` failed.
- **Where**: `services/api/internal/httpapi/report_q_test.go:214`.
- **Why**: The test performs `strings.Contains(definition.ProjectionNote, "source-backed")` (case-sensitive), but `definition.ProjectionNote` begins with `"Source-backed"` (capital 'S').
- **Suggestion**: Fix either the definition string in `reports.go` or the test assertion in `report_q_test.go` to match case-insensitively or use consistent casing.

### [Minor] Finding 3: Unit Test Failure — Query Fragment Check in `httpapi`
- **What**: `TestPhaseQHistoricalQueriesAreScopeBoundAndPaginated` failed.
- **Where**: `services/api/internal/httpapi/report_q_test.go:245`.
- **Why**: The test checks for fragment `"FROM sync_events se"` in `adjustmentQuery`, but the query definition for stock adjustments in `history.go` omitted this join fragment.
- **Suggestion**: Align the query definition and test assertion so that historical adjustment queries meet the exact pagination and join contract.

---

## 3. Verified Claims

- `go vet ./migration/... ./services/api/... ./services/edge/...` → PASS (0 errors, 0 warnings).
- `pnpm --filter @abuzar/web check` → PASS (0 errors, 0 warnings).
- PostgreSQL DDL migrations (`001_tenancy.sql` .. `029_auxiliary_master_kinds.sql`) → RLS policies enabled and forced across tenant, branch, auxiliary master tables, and bookkeeping tables (`migration_exceptions`, `migration_ambiguous_records`).
- Codebase layout compliance → PASS (`.agents/` contains zero project source files).

---

## 4. Logic Chain

1. Executed `go vet ./migration/... ./services/api/... ./services/edge/...` — confirmed clean exit code 0.
2. Executed `pnpm --filter @abuzar/web check` — confirmed clean exit code 0.
3. Executed `go test ./migration/... ./services/api/... ./services/edge/... -count=1` — observed test failure in `services/api/internal/httpapi` (2 test failures in `report_q_test.go`).
4. Compared actual test failure against worker 1 handoff claims of 100% test pass.
5. Identified INTEGRITY VIOLATION (attestations of 100% test pass contradicted actual test failure).
6. Concluded verdict must be **REQUEST_CHANGES**.

---

## 5. Caveats

- Database execution against a live PostgreSQL server container was not re-run in this review pass; evaluation relied on static DDL analysis, `ops/postgres/apply-migrations.ps1` inspection, and `go test` suite.
- SQL Server read-only live connection tests rely on offline mock/unit tests in `migration/cmd/import` and `migration/cmd/reconcile`.

---

## 6. Conclusion

Milestone M2 cannot be approved in its current state due to a **Critical INTEGRITY VIOLATION** (upstream handoff report attested 100% test pass despite failing Go unit tests) and 2 concrete test failures in `services/api/internal/httpapi` (`TestPhaseQItemHistoryDefinitionsUseSourceBackedProjections` and `TestPhaseQHistoricalQueriesAreScopeBoundAndPaginated`).

**Verdict**: REQUEST_CHANGES

---

## 7. Verification Method

To independently verify this evaluation:

1. Run `go vet ./migration/... ./services/api/... ./services/edge/...` from `d:\ABUZAR\AbuzarNext` — verify 0 errors.
2. Run `pnpm --filter @abuzar/web check` from `d:\ABUZAR\AbuzarNext` — verify 0 errors, 0 warnings.
3. Run `go test ./services/api/internal/httpapi -count=1` from `d:\ABUZAR\AbuzarNext` — observe the 2 test failures in `report_q_test.go`.

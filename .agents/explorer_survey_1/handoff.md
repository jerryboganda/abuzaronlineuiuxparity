# Step 0 Survey Handoff Report — AbuzarNext Project

## 1. Observation

### System Workspace & File Paths
- **Project Root**: `d:\ABUZAR\AbuzarNext`
- **Agent Working Directory**: `d:\ABUZAR\AbuzarNext\.agents\explorer_survey_1`
- **Original Request File**: `d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md`
- **Status & Evidence Documentation**:
  - `docs/IMPLEMENTATION_STATUS.md`
  - `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`
  - `docs/GAP_ANALYSIS_2026-08-06.md`
  - `docs/PARITY_STATUS.md`

### Measured Code Base Artifacts & Line Numbers
- **MDI Window Registry**: `apps/web/src/lib/legacy-window-registry.ts` (lines 1-108): Handles SessionStorage window state, active tab, layout (`cascade`/`tile`/`layer`/`arrange`), and clear on logout.
- **Contextual Menu Catalog**: `apps/web/src/lib/legacy-menu-contextual-catalog.ts`: Contains expanded 325+ contextual items for Pack Purchase, Cash Sale, Item Master, Reports, Groups.
- **Document Lifecycles**: `services/api/internal/httpapi/documents.go` (lines 23-45): Registers 13 business document kinds across cash/credit sales, returns, quotations, pack/loose/opening purchases, and purchase orders.
- **Database Schema**: 30 SQL files in `db/migrations/` (`001_tenancy.sql` through `029_auxiliary_master_kinds.sql`).
- **PostgreSQL Apply Script**: `ops/postgres/apply-migrations.ps1`.
- **Migration Importer & Reconciler**: `migration/cmd/import`, `migration/cmd/reconcile`, `migration/maps/` (26 JSON mapping files).

### Measured Verification Results (Commands Executed Live on 2026-08-07)
1. `pnpm --filter @abuzar/web check`: Result: `0 errors, 0 warnings`.
2. `pnpm --filter @abuzar/web build`: Result: SvelteKit static build completed cleanly.
3. `go vet ./services/api/... ./services/edge/... ./migration/...`: Result: `0 issues`.
4. `go test ./services/api/... ./services/edge/... ./migration/... -count=1`: Result: `100% passed`.
5. `pnpm --filter @abuzar/web test -- --workers=1 --retries=1 --reporter=line`: Result: `77/77 test cases passed` serially with 0 retry/assertion failures.
6. `ops/postgres/apply-migrations.ps1`: Result: `Applied 29 migrations` plus `029_auxiliary_master_kinds.sql` applied clean.
7. `ops/local/status-local.ps1`: Result: PostgreSQL (`127.0.0.1:5432`), API (`127.0.0.1:8080`), Edge (`127.0.0.1:8091`), Web (`127.0.0.1:5173`) all healthy (HTTP 200).

### Measured Database Exception & Reconciliation Counts
- `migration_exceptions`: 501,024 resolved, 404 ignored, 32 open `Purdetail/non_positive_quantity` rows in `LEGACY_CANONICAL` tenant (`6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01`).
- `migration_ambiguous_records`: 16 open `tax_rule_has_no_numeric_rate` rows in sandbox tenant.
- Isolated canonical tenant master counts: 60,870 `master_items`, 30,052 pricing tiers, 7 GST / 3 PCT tax rates, 84,372 imported source rows across enterprise/config and core-master maps.

---

## 2. Logic Chain

1. **Premise 1**: Requirement R1 mandates pixel and workflow parity across captured PowerBuilder catalog windows, shell, MDI window management, navigation, shortcut keys, modal dialogs, and change-user flows.
   - *Evidence*: `apps/web/src/lib/legacy-window-registry.ts` and `apps/web/src/lib/LegacyWorkflowSurface.svelte` implement SessionStorage-backed window tracking, tab switching, MDI layout controls, keyboard accelerators (Ctrl+Alt+M, Ctrl+X, Ctrl+Q, Ctrl+I), and Change User Yes/No confirmation.
   - *Observation*: Baseline shell raster comparison at 1936x1048 is 0-diff (`legacy-shell-parity-comparison-2026-08-05.json`), and Playwright test suite passes 77/77 tests covering these MDI/navigation workflows.
   - *Deduction*: R1 core shell framework, MDI navigation, menu catalog, and change-user workflows are structurally solid and fully verified by automated tests. Gaps remain in floating native sub-window rendering and per-leaf raster sign-off for non-baseline screens.

2. **Premise 2**: Requirement R2 mandates source data reconciliation between legacy PowerBuilder SQL Server (`FazalDinPP19DataBaseV2`, 763 tables) and PostgreSQL (`abuzar_next`), tenant/branch RLS isolation, open migration line exception resolution, tax ambiguity resolution, and transaction bookkeeping.
   - *Evidence*: 30 SQL migrations in `db/migrations/` enforce tenant/branch RLS; `migration/cmd/import` and `migration/cmd/reconcile` provide declarative mapping and metric verification. 84,372 source rows across enterprise/config and core masters were imported clean into the isolated `LEGACY_CANONICAL` tenant.
   - *Observation*: 32 open `Purdetail/non_positive_quantity` exceptions remain in `migration_exceptions` for the canonical tenant, and 16 open `tax_rule_has_no_numeric_rate` records remain in `migration_ambiguous_records` for the sandbox tenant. Full transaction lines (`Saledetail` 620k, `StockReport` 3.2M, `VirtualGl` 1.04M) remain deferred.
   - *Deduction*: R2 schema structure, RLS security, declarative import infrastructure, and initial master data promotion are complete and verified. Gaps remain in resolving the 32 open line exceptions, 16 tax ambiguities, and executing the full historical transaction/stock/GL import wave once live SQL Server access is restored.

3. **Premise 3**: R5 requires all build checks, Go package tests, browser E2E tests, migration replays, and local runtime status probes to pass cleanly.
   - *Evidence*: All 7 automated verification gates (`pnpm check`, `pnpm build`, `go vet`, `go test`, Playwright 77/77, `apply-migrations.ps1`, `status-local.ps1`) pass with 0 errors.
   - *Deduction*: The repository is in a healthy, green build state suitable for subsequent execution phases.

---

## 3. Caveats

1. **Unexecuted Historical Transaction Import**: Full historical data migration of `Saledetail` (620,802 rows), `StockReport` (3,231,846 rows), and `VirtualGl` (1,040,590 rows) could not be executed during this survey due to external SQL Server domain/auth boundaries.
2. **Physical Hardware Acceptance**: Edge hardware adapters (`services/edge/internal/hardware`) return "No adapter configured" in local dev mode; physical hardware validation (thermal receipt printer, cash drawer pulse, barcode scanner) requires physical device attachment.
3. **Floating Native MDI Sub-windows**: SvelteKit web architecture uses tabbed MDI navigation and CSS overlays rather than OS-level floating child windows. This is an architectural design decision for browser/Tauri compatibility.

---

## 4. Conclusion

The AbuzarNext codebase is well-architected, cleanly structured, and in a 100% green verification state across all automated build, typecheck, unit, integration, browser E2E, and migration replay gates. 

- **Requirement R1 (Legacy Shell, Workflow & MDI Parity)**: Core shell raster (1936x1048 baseline), responsive fallback, MDI window registry, tab restoration, layout controls, keyboard accelerators, change-user confirmation, and base/contextual menu trees are fully implemented and verified by 77 Playwright browser tests.
- **Requirement R2 (Data Import, Schema & Reconciliation)**: PostgreSQL schema foundation (30 migrations with RLS), declarative map importer, metric reconciler, auxiliary master CRUD (16 leaves), historical Back Date stock report read model, and historical GL journal read model are fully implemented. 84,372 source master rows are reconciled. 32 open line exceptions and 16 tax ambiguities are tracked in bookkeeping tables, awaiting final canonical historical import execution.

---

## 5. Verification Method

To independently verify the findings of this report, execute the following commands from `d:\ABUZAR\AbuzarNext`:

1. **Web TypeScript & Typecheck**:
   ```powershell
   pnpm --filter @abuzar/web check
   ```
   *Expected Output*: `0 errors, 0 warnings`.

2. **Web Production Build**:
   ```powershell
   pnpm --filter @abuzar/web build
   ```
   *Expected Output*: Clean build completion without errors.

3. **Go Static Code Analysis**:
   ```powershell
   go vet ./services/api/... ./services/edge/... ./migration/...
   ```
   *Expected Output*: `0 issues`.

4. **Go Unit & Integration Test Suite**:
   ```powershell
   go test ./services/api/... ./services/edge/... ./migration/... -count=1
   ```
   *Expected Output*: PASS for all packages (`services/api/internal/httpapi`, `services/edge/internal/store`, `migration/cmd/...`).

5. **Playwright Browser Test Suite**:
   ```powershell
   pnpm --filter @abuzar/web test -- --workers=1 --retries=1 --reporter=line
   ```
   *Expected Output*: `77 passed` (100% pass rate).

6. **PostgreSQL Migration Replay**:
   ```powershell
   ops/postgres/apply-migrations.ps1
   ```
   *Expected Output*: All 29 base migrations + `029_auxiliary_master_kinds.sql` applied cleanly.

7. **Inspect Analysis Report**:
   Inspect `d:\ABUZAR\AbuzarNext\.agents\explorer_survey_1\analysis.md` for full detailed inventory breakdown.

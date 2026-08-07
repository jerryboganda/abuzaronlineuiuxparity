# AbuzarNext Specification Mining & Verification Inventory

## Overview
This document contains the complete specification inventory and evidence catalog mined for the **AbuzarNext** project (SvelteKit + Go + PostgreSQL rebuild of the legacy WASEELA ABUZAR V3 PowerBuilder system).

- **Project Root**: `d:\ABUZAR\AbuzarNext`
- **Specification Source Documents**: `d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md`, `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`, `docs/IMPLEMENTATION_STATUS.md`, `docs/GAP_ANALYSIS_2026-08-06.md`, `docs/PARITY_STATUS.md`, `docs/PARITY_FIX_PLAN_A-Z.md`, phase evidence files (`docs/PHASE_*.md`), database migration files (`db/migrations/*.sql`), ops scripts (`ops/`), Playwright configs (`apps/web/playwright.config.ts`), and Go test fixtures.

---

## 1. Requirements Overview (R1 - R5)

### R1. Legacy Shell, Workflow & Window/MDI Parity
- **Scope**: Pixel and workflow parity across 275+ captured PowerBuilder catalog windows (shell chrome, MDI window management, navigation, keyboard shortcuts, modal dialogs, and change-user flows).
- **Visual Gate**: 0-pixel visual difference against baseline rasters at `1936x1048` canonical resolution.
- **MDI Registry**: Window registry must persist open tabs across client-side navigation and hard page reloads.
- **Change User Boundary**: Shared contextual `Change User` confirmation modal (Yes/No dialog). Confirming clears MDI window registry state, revokes API session context, and redirects to `/login?changeUser=1`.

### R2. Data Import, Schema & Bookkeeping Reconciliation
- **Scope**: Reconcile legacy SQL Server structures (`FazalDinPP19DataBaseV2`, 763 tables) to target PostgreSQL schema (`abuzar_next`).
- **Isolation & Security**: Strict tenant and branch Row-Level Security (RLS) policies enforced via PostgreSQL session settings (`app.tenant_id`, `app.branch_id`). Application role (`abuzar_app_local` / `abuzar_app_ci`) runs with least privileges (no `SUPERUSER`, `BYPASSRLS`, `CREATEDB`, `CREATEROLE`, or `DELETE` on `sync_events`).
- **Bookkeeping & Exceptions**: Track migration open exceptions in `public.migration_exceptions` and tax ambiguities in `public.migration_ambiguous_records`. Zero unhandled critical open exceptions required for clean cutover status.

### R3. Pricing Policy, Stock Balance & Financial Engine Parity
- **Pricing Engine**: Exact-decimal arithmetic (no floating-point precision loss) evaluating 10 SalePrice tiers (`SalePrice1` through `SalePrice10`), supplier schemes (`ItemSuppliers`), item discounts, document discounts, flat discounts, Misc charges, and tax ordering (GST, PCT, advance income tax). Exposed via `POST /v1/transactions/preview`.
- **Stock Balance Engine**: Scoped `GET /v1/inventory/balance` projection over `stock_balances`. Support for historical stock snapshots (`dbo.StockReport` / `historical_stock_snapshots`) for Back Date reporting.
- **Financial Core & GL**: Ledger projections (`dbo.VirtualGl` / `historical_gl_entries`) with 10-column detail contract. Compensating atomic void reversals for posted sales, sale returns, purchases, and purchase returns via migration `028_business_document_void_reversals.sql`.

### R4. Report Engine & Hardware Integration Standard
- **Report Catalog**: 151 catalog report definitions. Format selection dialog with named report formats, retrieval arguments dialog (date ranges, cash/credit flags, selectable areas), and print-preview surface (letterhead, ruler, toolbar, zoom, loaded-row paging).
- **Export Formats**: CSV export, browser PDF printing, and Excel-compatible workbook export.
- **Line Detail Contracts**: 11-field line read model for Daily Sale Detail, Sale Detail, and Sales Return Detail.
- **Hardware Integration**: Edge service (`services/edge`) hardware abstraction layer covering ESC/POS receipt printing, cash drawer pulse, barcode generation/scanning, biometric devices, SMS, and email. Safe fail-soft handling ("No adapter configured") when physical devices are detached.

### R5. Comprehensive Verification & Handoff Evidence
- **Verification Gates**:
  1. `pnpm --filter @abuzar/web check` (0 errors, 0 warnings).
  2. `pnpm --filter @abuzar/web build` (successful SvelteKit static build).
  3. `pnpm --filter @abuzar/web test -- --workers=1 --retries=0` (100% serial browser test pass).
  4. `go vet ./services/api/... ./services/edge/... ./migration/...` (0 issues).
  5. `go test ./services/api/... ./services/edge/... ./migration/... -count=1` (100% Go backend test pass).
  6. `ops/postgres/apply-migrations.ps1` (0 errors across sequential SQL migrations).
  7. Migration exception/reconciliation script (`migration/cmd/reconcile`) with `-fail-on-open-bookkeeping`.
- **Documentation Evidence**: `docs/IMPLEMENTATION_STATUS.md` and `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`.

---

## 2. Features Discovered

| # | Category | Feature | Description | Inputs | Outputs | Error Behavior | Discovered Via |
|---|----------|---------|-------------|--------|---------|----------------|----------------|
| 1 | R1: Shell | Maximized Legacy Workspace Shell | Renders 1936x1048 PowerBuilder workspace frame with live session clock, logged-in username (ADMIN), menu bar, MDI container, toolbar, and status bar | Viewport size, active route, session state | HTML canvas & Svelte layout matching baseline | Falls back to responsive CSS shell if viewport != 1936x1048 | `docs/PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md`, `apps/web/src/routes/app/legacy/+page.svelte` |
| 2 | R1: Shell | Stateful MDI Window Registry | Tracks and restores open child windows across client-side SPA navigation and page reloads | Window creation, navigation events, `sessionStorage` | Restored window tabs & active state | Cleared on explicit logout or confirmed Change User | `apps/web/src/lib/legacy-window-registry.ts`, `docs/PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md` |
| 3 | R1: Shell | Change User Confirmation Flow | Modal Yes/No alert dialog triggered when changing users from top menu or child windows | User click on "Change User" menu item | Alert dialog modal | "No" closes dialog retaining tabs; "Yes" invalidates session & clears window registry | `docs/PHASE_C_CHANGE_USER_EVIDENCE_2026-08-07.md`, `apps/web/tests/smoke.spec.ts` |
| 4 | R1: Shell | Keyboard Accelerator Shortcuts | Global hotkey handling for legacy actions (Ctrl+Alt+M Session Monitor, Ctrl+X Exit, Ctrl+Q Save & Post, Ctrl+I New Item, Ctrl+D Delete Item, Ctrl+Z Restore Item, Ctrl+H Purchase History, Ctrl+M Purchase Slip, Ctrl+B Auto Batch, Alt+F8 Labels) | Keyboard keydown events | Triggers target action handler | Ignored when input field is focused or handler unmapped | `docs/PARITY_STATUS.md`, `apps/web/src/lib/legacy-menu-catalog.ts` |
| 5 | R1: Shell | Contextual Menu Swapping | Dynamically replaces base menu tree (275 items) with window-specific contextual menu (up to 326 items) when transaction/master windows open | Active window focus | Updated top menu bar items | Reverts to base catalog on window close | `docs/GAP_ANALYSIS_2026-08-06.md`, `apps/web/src/lib/legacy-menu-contextual-catalog.ts` |
| 6 | R1: Shell | Untouched Raster Comparison Gate | Automated PowerShell comparison checking pixel equality against baseline legacy PNG rasters | Captured screen raster PNG, Baseline PNG | `differentPixels` count | Fails if `differentPixels > 0` | `parity/tools/compare-png.ps1`, `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` |
| 7 | R2: Data | PostgreSQL Multi-Tenant RLS Foundation | Enforces tenant/branch data isolation at the database layer using PostgreSQL RLS policies | `app.tenant_id`, `app.branch_id` session config variables | Gated query results | Unscoped queries return 0 rows; cross-tenant writes thrown/rejected | `db/migrations/001_tenancy.sql`, `016_branch_rls_hardening.sql`, `docs/POSTGRES_APP_ROLE_RLS_EVIDENCE_2026-08-06.md` |
| 8 | R2: Data | Declarative SQL Server Importer | Imports legacy tables into PostgreSQL using JSON mapping specifications, injecting tenant/branch IDs | `-config` JSON mapping, source/target DSNs | Populated PostgreSQL tables, `legacy_id_mappings` | Fail-closed on missing mapping keys or empty tenant injection | `migration/cmd/import/main.go`, `migration/maps/*.json` |
| 9 | R2: Data | SQL Server Schema Inspector | Inspects SQL Server source schema metadata, table structures, column definitions, and indices | `-source` SQL Server DSN | JSON schema manifest file | Fails if SQL Server connection fails or auth denied | `migration/cmd/inspect/main.go`, `docs/MIGRATION_RUNBOOK.md` |
| 10 | R2: Data | Reconciliation & Bookkeeping Engine | Compares source/target row counts, business metrics, and migration exception tables | Source DSN, Target DSN, `-config`, `-metrics`, `-tenant` | JSON reconciliation report (`parity/catalog/migration-reconciliation.json`) | Exits with error if `-fail-on-open-bookkeeping` is set and open exceptions exist | `migration/cmd/reconcile/main.go`, `docs/PHASE_E_BOOKKEEPING_AND_ORDER_WAVE_EVIDENCE_2026-08-07.md` |
| 11 | R2: Data | Migration Exception & Ambiguity Ledger | Records unparseable or non-conforming source rows during migration for auditable resolution | Invalid source rows during import | Rows inserted into `migration_exceptions` or `migration_ambiguous_records` | Unresolved rows keep bookkeeping status `review_required` | `db/migrations/004_migration_support.sql`, `migration/cmd/bulkpurchaselines/main.go` |
| 12 | R2: Data | Least-Privilege App Role Provisioning | Script to provision non-owner PostgreSQL application role with strict DML privileges | PostgreSQL admin DSN | Provisioned role (`abuzar_app_local`) | Rejects execution if granted owner or superuser rights | `ops/postgres/provision-app-role.ps1`, `grant-app-role.sql`, `docs/OPERATIONS.md` |
| 13 | R2: Data | Sequential Migration Replay Script | PowerShell script executing database schema migrations in numerical order with error-stop enforcement | Schema-owner DSN (`ABUZAR_ADMIN_DATABASE_URL`) | Migration application logs | Stops immediately on first migration error (`ON_ERROR_STOP=1`) | `ops/postgres/apply-migrations.ps1`, `db/migrations/*.sql` |
| 14 | R3: Finance | Multi-Tier Pricing Preview Engine | Server-side calculation engine for exact-decimal pricing, tier selection, discounts, and taxes | `POST /v1/transactions/preview` with item lines, customer, SalePrice tier | JSON pricing breakdown (subtotal, discounts, taxes, net total) | Returns 400 Bad Request on negative values or invalid tier | `services/api/internal/pricing/pricing.go`, `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md` |
| 15 | R3: Finance | 10-Tier SalePrice Tier Selector | Allows selection of any of 10 captured pricing tiers (`SalePrice1` to `SalePrice10`) for line repricing | Tier selection dropdown | Updated grid row unit prices and total preview | Invalid tier reverts to base item price | `apps/web/src/routes/app/sales/+page.svelte`, `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md` |
| 16 | R3: Finance | ItemSuppliers Scheme Inheritance | Automatically populates supplier discount %, bonus quantity, and payment terms for canonical purchases | Supplier ID, Item ID | Inherited discount %, bonus qty, days | Falls back to line defaults if no link exists | `services/api/internal/httpapi/purchase.go`, `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md` |
| 17 | R3: Finance | Stock Balance & Availability Projection | Queries current branch stock balance by godown and item | `GET /v1/inventory/balance?godown_id=...&item_id=...` | JSON object with available stock balance | Returns 0 balance if unstocked or invalid godown | `services/api/internal/httpapi/stock.go`, `docs/IMPLEMENTATION_STATUS.md` |
| 18 | R3: Finance | Historical Stock Back Date Query | Reads historical stock snapshots from legacy `dbo.StockReport` import | `GET /v1/reports/stock-back-date?as_of_date=...` | JSON historical stock report contract (10 fields) | Returns 400 if date format invalid | `services/api/internal/httpapi/stock_report.go`, `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md` |
| 19 | R3: Finance | Historical VirtualGl GL Journal Query | Reads historical GL entries from legacy `dbo.VirtualGl` combined with newly posted journals | `GET /v1/reports/gl-journal?from=...&to=...` | JSON 10-column GL journal projection | Returns empty array if date range out of scope | `services/api/internal/httpapi/finance.go`, `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md` |
| 20 | R3: Finance | Compensating Document Void Reversal | Executes atomic compensating stock, GL, and party ledger reversals for posted documents | `POST /v1/documents/{id}/void` | Updated document status `voided`, appended reversal projections | Fails if document is already voided or has dependent posted documents | `db/migrations/028_business_document_void_reversals.sql`, `services/api/internal/httpapi/void_reversal.go`, `docs/PHASE_T_VOID_REVERSAL_EVIDENCE_2026-08-07.md` |
| 21 | R4: Reports | Catalog Report Engine & 151 Formats | Houses report definitions for 151 catalog reports, handling arguments, formatting, and pagination | Report slug, retrieval arguments, format ID | Typed report data dataset | Returns 404 for unknown report slug; 400 for bad parameters | `services/api/internal/httpapi/reports.go`, `apps/web/src/lib/report-core.ts` |
| 22 | R4: Reports | Report Print Preview Surface | Renders interactive preview with letterhead, ruler, toolbar, zoom, and loaded-row paging | Report dataset, format configuration | Interactive HTML print preview surface | Gracefully displays empty state if 0 rows returned | `apps/web/src/routes/app/report/[slug]/+page.svelte`, `docs/PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md` |
| 23 | R4: Reports | Multi-Format Report Exporter | Exports report datasets to CSV, browser printable PDF, and Excel-compatible HTML workbooks | User export button click | File download blob (CSV/XLS/PDF print trigger) | Displays alert if export rendering fails | `apps/web/src/routes/app/report/[slug]/+page.svelte`, `docs/PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md` |
| 24 | R4: Reports | Daily Sale Detail 11-Column Read Model | Projects canonical/compatibility line detail containing 11 specific legacy columns | `GET /v1/reports/daily-sale-detail` | 11-column structured JSON line data | Filters out invalid tenant scope rows | `services/api/internal/httpapi/read_models.go`, `docs/PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md` |
| 25 | R4: Hardware | Edge Hardware Abstraction | Provides REST abstraction layer for peripheral devices (ESC/POS thermal printer, cash drawer pulse, barcode scanner) | `POST /v1/hardware/print`, `/pulse`, etc. | Operation status response | Returns 200 with "No adapter configured" message if physical hardware is absent | `services/edge/internal/hardware/registry.go`, `docs/PHASE_U_HARDWARE_EVIDENCE.md` |
| 26 | R5: Gates | Web Type Check Gate | Validates Svelte components and TypeScript code for zero compiler errors or warnings | `pnpm --filter @abuzar/web check` | Terminal stdout check results | Exits code != 0 if TypeScript errors/warnings exist | `ORIGINAL_REQUEST.md`, `package.json` |
| 27 | R5: Gates | Web Production Build Gate | Compiles SvelteKit web application into static production build artifacts | `pnpm --filter @abuzar/web build` | Output build directory `apps/web/build/` | Exits code != 0 if build compilation fails | `ORIGINAL_REQUEST.md`, `package.json` |
| 28 | R5: Gates | Serial Playwright Test Gate | Runs Playwright end-to-end browser suite serially with 1 worker and 0 retries | `pnpm --filter @abuzar/web test -- --workers=1 --retries=0` | Test pass/fail terminal report & traces | Exits code != 0 if any Playwright test assertion fails | `apps/web/playwright.config.ts`, `ORIGINAL_REQUEST.md` |
| 29 | R5: Gates | Go Static Analysis Gate | Analyzes Go source code packages for static bugs, formatting, and type mismatches | `go vet ./services/api/... ./services/edge/... ./migration/...` | Go vet output report | Exits code != 0 if any vet rule fails | `ORIGINAL_REQUEST.md` |
| 30 | R5: Gates | Go Backend Test Gate | Executes Go unit and integration tests across API, edge, and migration packages | `go test ./services/api/... ./services/edge/... ./migration/... -count=1` | Test result status per package | Exits code != 0 if any unit/integration test fails | `ORIGINAL_REQUEST.md` |

---

## 3. Edge Cases Inventory

| # | Feature | Input | Observed Behavior |
|---|---------|-------|-------------------|
| 1 | Database RLS Isolation | Unscoped query without setting `app.tenant_id` session configuration | PostgreSQL RLS filters out all rows, returning 0 records or raising RLS permission denial on write. |
| 2 | Cross-Branch Access | Querying or writing records belonging to `Branch B` while operating under `Branch A` context | Rejected by PostgreSQL branch RLS policy (`016_branch_rls_hardening.sql`); write fails. |
| 3 | Non-Positive Historical Purchase Line | Legacy `dbo.Purdetail` row with `PackQty <= 0` or `LooseQty <= 0` | Migration importer rejects row from canonical table and writes details to `migration_exceptions` with status `open`. |
| 4 | Tax Rule Lacking Numeric Rate | Legacy tax rule labeled "NO TAX" or "TAX ON ACTUAL QTY ONLY" with NULL rate | Migration importer writes record to `migration_ambiguous_records` with status `open`; reconciler evaluates bookkeeping status as `review_required`. |
| 5 | Document Void with Dependent Posted Documents | Attempting to void a posted Purchase Invoice when a posted Purchase Return references it | Backend API rejects void request with 400 Bad Request, enforcing dependent document integrity. |
| 6 | Idempotent Void Replay | Sending duplicate `POST /v1/documents/{id}/void` calls for an already voided document | API detects `voided` status and returns existing void reversal record without generating duplicate GL/stock entries. |
| 7 | Change User Modal Cancellation vs Confirmation | Clicking "Change User" menu item, then selecting "No" in the dialog vs selecting "Yes" | Selecting "No" hides dialog while preserving current MDI window tabs; selecting "Yes" invalidates API session, clears `sessionStorage` window registry, and navigates to `/login?changeUser=1`. |
| 8 | Detached Hardware Operations | Invoking print slip or cash drawer pulse when no physical printer/drawer is connected | Edge service responds gracefully with HTTP 200 `{"status": "ok", "adapter": "none", "message": "No adapter configured"}`. |
| 9 | Concurrent Playwright Workers | Running Playwright with multiple parallel worker processes (`workers > 1`) | Causes Chromium socket contention (`net::ERR_NO_BUFFER_SPACE`) or API context collisions; serial execution (`--workers=1`) runs green. |
| 10 | SQL Server Windows Authentication Boundary | Running migration tools against remote SQL Server using Windows Integrated Auth without domain trust | Connection fails closed with authentication timeout; no unencrypted credentials or partial mappings are written. |

---

## 4. Complete Command Suite & Verification Protocol

### Verification Commands

```powershell
# 1. Svelte Web TypeScript Check
pnpm --filter @abuzar/web check

# 2. Svelte Web Production Build Validation
pnpm --filter @abuzar/web build

# 3. Playwright Serial End-to-End Test Suite
pnpm --filter @abuzar/web test -- --workers=1 --retries=0

# 4. Go Backend Static Analysis
go vet ./services/api/... ./services/edge/... ./migration/...

# 5. Go Backend Unit & Integration Tests (requires PostgreSQL DATABASE_URL)
$env:DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:5432/abuzar_next?sslmode=disable"
go test ./services/api/... ./services/edge/... ./migration/... -count=1

# 6. Ordered PostgreSQL Migration Replay
$env:ABUZAR_ADMIN_DATABASE_URL = "postgres://postgres:postgres@127.0.0.1:5432/abuzar_next?sslmode=disable"
.\ops\postgres\apply-migrations.ps1

# 7. Migration Reconciliation & Bookkeeping Exception Check
go run ./migration/cmd/reconcile `
  -source $env:ABUZAR_SOURCE_SQLSERVER_URL `
  -target $env:ABUZAR_TARGET_POSTGRES_URL `
  -config ./migration/maps/phase-e-historical-orders.json `
  -tenant "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01" `
  -fail-on-open-bookkeeping

# 8. Local Stack Health Diagnostics
.\ops\local\status-local.ps1
```

---

## 5. Summary of Gaps & Acceptance Requirements

1. **Verification Suite Gate**: All 7 verification commands must run to completion with 0 errors.
2. **Acceptance Evidence Document Gate**: `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` and `docs/IMPLEMENTATION_STATUS.md` must be updated with current timestamps and evidence matrices after execution.
3. **Migration Bookkeeping Gate**: All migration exceptions in `public.migration_exceptions` and ambiguities in `public.migration_ambiguous_records` must be resolved or documented before canonical cutover.

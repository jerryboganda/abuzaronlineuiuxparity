# Step 0 Survey Analysis — AbuzarNext Project

**Summary**: The AbuzarNext project is a parity-first rebuild of the PowerBuilder Abuzar ERP system into a modern SvelteKit/Go/PostgreSQL web/PWA/Tauri stack. The foundational architecture (tenancy, RLS, API session auth, branch SQLite edge sync, core document types, pricing engine preview, initial master migration, and MDI window registry) is implemented and passing all 77 Playwright browser tests, Go package unit/integration tests, web typechecks, builds, and migration replay scripts, but full 100% legacy replacement is gated by unexecuted canonical transaction/stock/GL migration waves, remaining report definitions/print parity, and hardware acceptance.

---

## 1. Current Code Layout & Architecture

The workspace at `d:\ABUZAR\AbuzarNext` uses a hybrid monorepo setup (pnpm workspaces for TypeScript/SvelteKit, Go multi-module workspace via `go.work` for Go services).

```
d:\ABUZAR\AbuzarNext
├── apps/
│   ├── web/                     # @abuzar/web: SvelteKit + TypeScript web/PWA app with legacy shell overlay
│   └── desktop/                 # @abuzar/desktop: Tauri wrapper for Windows desktop bundle (NSIS/MSI)
├── services/
│   ├── api/                     # Central Go HTTP API server (PostgreSQL-backed multi-tenant API)
│   └── edge/                    # Branch-local Go edge service (SQLite event store + WAL offline queue & sync)
├── migration/                   # Migration CLI tools (inspect, import, reconcile, bulk-historical) & mapping JSONs
├── db/
│   └── migrations/              # 30 ordered PostgreSQL SQL schema migration files with RLS policies
├── ops/
│   ├── postgres/                # PostgreSQL migration apply & app-role provisioning scripts (PS1/SH)
│   └── local/                   # Local stack supervisor and health status scripts
├── packages/
│   └── contracts/               # Shared TypeScript REST contracts and sync definitions
├── parity/                      # Legacy baseline catalog, 1936x1048 rasters, menu tree JSONs, comparison scripts
├── docs/                        # Complete architecture docs, status tracking, phase evidence files
├── go.work / go.work.sum        # Go 1.26 workspace linking ./migration, ./services/api, ./services/edge
├── package.json / pnpm-workspace.yaml # pnpm monorepo configuration
└── README.md                    # Project quick start and architecture overview
```

### Core Architecture Components
1. **Frontend (`apps/web`)**:
   - Built with SvelteKit and TypeScript (`pnpm --filter @abuzar/web`).
   - Legacy pixel-parity shell (`/app/legacy`) renders 1936x1048 baseline screen rasters with live HTML/CSS semantic overlays.
   - Client-side MDI Window Registry (`src/lib/legacy-window-registry.ts`) handles tab restoration, open window tracking, and MDI layout controls across client-side navigation.
   - Restful API client (`src/lib/api.ts`) communicates with Go API (`/v1/*`) via Vite proxy or direct HTTP.
2. **Central API Service (`services/api`)**:
   - Developed in Go 1.26 (`cmd/server`, `internal/httpapi`, `internal/auth`, `internal/pricing`, `internal/db`).
   - Connects to PostgreSQL using application DSN with Row-Level Security (RLS) active per tenant (`ABUZAR_APP_DATABASE_URL`).
   - Provides HTTP-only operator session management, multi-tenant RBAC permissions, transaction lifecycle (`/v1/documents/{kind}`), pricing preview (`/v1/transactions/preview`), item maintenance mutations, report projections, and compensating document void/reversals (`/v1/documents/{id}/void`).
3. **Branch Edge Service (`services/edge`)**:
   - Developed in Go 1.26 (`cmd/edge`, `internal/store`, `internal/syncer`, `internal/hardware`).
   - Local SQLite database (`branch-edge.sqlite`) with WAL mode for offline transaction capture.
   - Hardware abstraction adapters for thermal printers, barcode scanners, cash drawers, biometric devices, SMS, and email.
4. **Data & Schema (`db/migrations`, `ops/postgres`)**:
   - 30 ordered SQL migration files (`001_tenancy.sql` to `029_auxiliary_master_kinds.sql`).
   - Strict PostgreSQL RLS enforcement ensuring zero cross-tenant leak.
   - Script `ops/postgres/apply-migrations.ps1` applies all migrations sequentially.
5. **Data Migration Workbench (`migration/`)**:
   - Read-only inspector (`migration/cmd/inspect`), declarative map importer (`migration/cmd/import`), reconciliation engine (`migration/cmd/reconcile`), and bulk historical loaders (`migration/cmd/bulk-historical`).
   - Mapping rules defined in `migration/maps/` (26 mapping & metric JSON files).

---

## 2. Requirement R1: Legacy Shell, Workflow & Window/MDI Parity

### Objective
Achieve pixel and workflow parity across captured PowerBuilder catalog windows (shell, MDI window management, navigation, shortcut keys, modal dialogs, and change-user flows). Maintain zero-pixel-difference visual comparisons where baseline rasters exist and stateful UI behavior for interactive operations.

### Feature Inventory & Status Matrix

| Component / Workflow | Status | Implementation Reference | Notes / Remaining Gaps |
|---|---|---|---|
| Main Legacy Shell Raster | **Implemented** | `apps/web/src/routes/app/legacy/+page.svelte`, `parity/catalog/legacy-shell-parity-comparison-2026-08-05.json` | Baseline 1936x1048 resolution comparison verified at 0 differing pixels. Responsive CSS fallback handles other viewports. |
| MDI Window Registry | **Implemented** | `apps/web/src/lib/legacy-window-registry.ts` | SessionStorage-persisted registry preserves open windows, active tab, and layout (`cascade`, `tile`, `layer`, `arrange`) across reloads. |
| MDI Window Menu & Tabs | **Implemented** | `apps/web/src/lib/LegacyMenuBar.svelte`, `docs/PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md` | Window menu controls activate tabs, switch layouts, and restore child window states. |
| Change User Confirmation | **Implemented** | `apps/web/src/lib/LegacyWorkflowSurface.svelte`, `docs/PHASE_C_CHANGE_USER_EVIDENCE_2026-08-07.md` | Yes/No modal dialog appears in shell and child windows, clears persisted MDI registry, and invalidates API session on confirmed login exit. |
| Keyboard Accelerators | **Implemented** | `apps/web/src/lib/LegacyWorkflowSurface.svelte` | Ctrl+Alt+M (Session Monitor), Ctrl+X (Exit), Ctrl+Q (Save & Post), Ctrl+I (New Item) captured and wired. |
| Mojibake & Encoding Fix | **Implemented** | `apps/web/src/lib/styles.css` | Double-escaped CSS rules normalized; runtime text-repair MutationObserver removed; stable Unicode symbols used for toolbar/tab glyphs. |
| Base Menu Tree Catalog | **Implemented** | `apps/web/src/lib/legacy-menu-catalog.ts` | Base catalog contains 275 captured menu items across 9 main top-level headers. |
| Contextual Per-Window Menus | **Partial** | `apps/web/src/lib/legacy-menu-contextual-catalog.ts` | Contextual catalog expanded to 325-326 entries for Pack Purchase, Cash Sale, Item Master, Reports, Groups. Transaction verbs wired for key actions; unhandled menu verbs navigate to contextual workbench. |
| Per-Leaf Raster Verification | **Partial** | `parity/catalog/` | Representative rasters verified at 0 diffs (Login, Shell, Preferences, Change User, Integrity Monitor). Remaining catalog leaves require individual raster sign-off. |
| Floating Native MDI Windows | **Architectural Scope** | Web architecture design choice | Replaced with tabbed MDI navigation in browser; native floating sub-windows not rendered as OS windows. |

---

## 3. Requirement R2: Data Import, Schema & Bookkeeping Reconciliation

### Objective
Reconcile source data mappings between legacy PowerBuilder SQL Server structures (`FazalDinPP19DataBaseV2`, 763 tables) and the PostgreSQL target schema (`abuzar_next`). Ensure tenant/branch isolation, resolve open migration line exceptions and tax ambiguities, and enforce audit bookkeeping across all transaction tables.

### Feature Inventory & Status Matrix

| Component / Task | Status | Implementation Reference | Notes / Remaining Gaps |
|---|---|---|---|
| PostgreSQL Schema Foundation | **Implemented** | `db/migrations/001_tenancy.sql` to `029_auxiliary_master_kinds.sql` | 30 migrations defining tenant RLS, master tables, business documents, inventory ledgers, GL journals, party ledgers, and migration bookkeeping tables. |
| Declarative Map Importer | **Implemented** | `migration/cmd/import`, `migration/maps/*.json` | Declarative mapping importer supporting composite keys, source transforms, savepoint exception handling, and mapping table populations. |
| Metric Reconciliation Engine | **Implemented** | `migration/cmd/reconcile` | Compares legacy SQL Server metric queries with target PostgreSQL counts, logging mismatches and bookkeeping status. |
| Enterprise & Core Master Import | **Implemented** | `migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md` | Imported 11 enterprise/config tables + 7 core master tables into isolated `LEGACY_CANONICAL` tenant (84,372 source rows, 0 duplicates, 0 exceptions). |
| Auxiliary Master CRUD (16 Leaves) | **Implemented** | `db/migrations/029_auxiliary_master_kinds.sql`, `docs/PHASE_F_AUXILIARY_MASTER_EVIDENCE_2026-08-07.md` | Source-shaped payload CRUD for 16 auxiliary Basic Data leaves (`PricePolicy`, `ItemPromotion`, `SalesTaxSchedule`, etc.) with confirmed delete. |
| Historical Stock Back Date Report | **Implemented** | `services/api/internal/httpapi/stock.go`, `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md` | Source-backed read model over imported `historical_stock_snapshots` (`dbo.StockReport`) with tenant/date/godown scope. |
| Historical GL Journal Report | **Implemented** | `services/api/internal/httpapi/finance.go`, `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md` | Source-backed read model over imported `historical_gl_entries` (`dbo.VirtualGl`) unioned with newly posted journals. |
| Migration Line Exceptions | **Open Exception** | `docs/PHASE_E_BOOKKEEPING_AND_ORDER_WAVE_EVIDENCE_2026-08-07.md` | 32 open `Purdetail/non_positive_quantity` exceptions exist in `migration_exceptions` for the canonical tenant (cannot coerce negative/zero quantities into canonical positive line contract). |
| Tax Rule Ambiguities | **Open Ambiguity** | `docs/PHASE_E_BOOKKEEPING_AND_ORDER_WAVE_EVIDENCE_2026-08-07.md` | 16 open `tax_rule_has_no_numeric_rate` rows exist in `migration_ambiguous_records` in sandbox tenant ("NO TAX", "TAX ON ACTUAL QTY ONLY" labels without numeric rates). |
| Full Transaction/Ledger Migration | **Deferred** | `migration/PHASE_E_HISTORICAL_STATUS_2026-08-06.md` | Full `Saledetail` (620k rows), `SaleLedger` (291k rows), `StockReport` (3.2M rows), and `VirtualGl` (1.04M rows) remain deferred behind live SQL Server connection restoration. |
| Live SQL Server Connector | **Blocked** | `migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md` | External read-only connection to legacy SQL Server host currently blocked by Windows authentication / domain boundaries. |

---

## 4. Status of Existing Build Configs, Test Runners & Verification Gates

All automated verification commands specified in Requirement R5 and `ORIGINAL_REQUEST.md` have been executed and verified.

```
+---------------------------------------------------+--------------------+-----------------------------------------------------+
| Verification Gate / Command                       | Target / Package   | Verified Result & Status                            |
+---------------------------------------------------+--------------------+-----------------------------------------------------+
| pnpm --filter @abuzar/web check                  | Web TypeScript     | PASSED: 0 errors, 0 warnings                        |
| pnpm --filter @abuzar/web build                  | SvelteKit Static   | PASSED: Production static build completes cleanly   |
| go vet ./services/api/... ./services/edge/... ... | Go Static Analysis | PASSED: 0 issues reported across all Go packages    |
| go test ./services/api/... ./services/edge/... .. | Go Package Tests   | PASSED: 100% unit and integration tests pass        |
| pnpm --filter @abuzar/web test -- --workers=1     | Playwright E2E     | PASSED: 77/77 browser test cases pass serially      |
| ops/postgres/apply-migrations.ps1                | PostgreSQL Schema  | PASSED: Applied 29 base migrations + 029 clean      |
| ops/local/status-local.ps1                       | Local Services     | PASSED: PostgreSQL (5432), API (8080), Edge (8091), |
|                                                   |                    |         Web (5173) all reporting HTTP 200 OK        |
+---------------------------------------------------+--------------------+-----------------------------------------------------+
```

### Breakdown of Test Suites
1. **Playwright E2E Suite (`apps/web/tests/`)**:
   - `smoke.spec.ts`: Navigation, SSR deep links, list history loading, contextual menu triggers.
   - `sales-canonical.spec.ts`: Cash/credit sale posting, pricing preview debounce, tier selection, voiding.
   - `purchase-canonical.spec.ts` & `phase-cd.spec.ts`: Pack/loose purchase drafting, line hydration, supplier selection, purchase return population.
   - `phase-f.spec.ts`: Auxiliary master CRUD (16 leaves), confirmed delete workflows.
   - `phase-q.spec.ts`: Financial reports, GL journal read model assertions.
   - `phase-r.spec.ts`: Group allowed price settings, composite scope enforcement.
   - `preferences.spec.ts`: Preferences tabs loading and value persistence.
   - `visual-remediation.spec.ts`: Mojibake elimination and glyph rendering checks.

2. **Go Integration & Unit Test Suite (`services/api/internal/httpapi`)**:
   - `documents_integration_test.go`: Lifecycle of sales, returns, purchases, and orders.
   - `void_reversal_integration_test.go`: Compensating document voiding and stock/GL reversal.
   - `read_models_integration_test.go`: Daily sale detail, sales line detail, stock balance projections.
   - `finance_integration_test.go` & `historical_integration_test.go`: Historical GL queries, stock back-date snapshot reads.
   - `maintenance_integration_test.go` & `maintenance_item.go`: Item price/discount updates, stock batch locks.
   - `access_integration_test.go`: RBAC role permissions, group allowed godown/header enforcement.

---

## 5. Key Technical Findings & Gaps Summary

1. **Architecture Strength**: The foundational multi-tenant Go API + PostgreSQL RLS + SvelteKit web client + Go edge SQLite synchronizer architecture is robust, well-structured, and fully covered by automated tests.
2. **R1 (Legacy Shell & MDI) Gaps**:
   - While shell pixel comparison at 1936x1048 is 0-diff for baseline rasters, missing full raster sign-offs for all 151 individual report dialogs and legacy screens.
   - Contextual menu catalog handling is largely in place (`legacy-menu-contextual-catalog.ts`), but specific business verbs for minor legacy commands fall back to contextual workbenches.
3. **R2 (Data & Schema Reconciliation) Gaps**:
   - Full canonical data import of historical transactions (620k sales lines, 3.2M stock rows, 1.04M GL rows) remains unexecuted due to external SQL Server domain/auth connectivity boundaries.
   - 32 open non-positive quantity line exceptions in `Purdetail` and 16 tax ambiguity records require explicit domain sign-off or resolution scripts before full cutover.
4. **Hardware & Production Scale**:
   - Hardware integration interfaces exist in `services/edge/internal/hardware`, but live physical device validation (ESC/POS thermal printer, barcode scanner, cash drawer pulse) remains pending connected hardware.
   - 24-hour soak test and multi-million row performance verification remain gated behind provisioned scale environment.

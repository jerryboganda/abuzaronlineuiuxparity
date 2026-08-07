# Milestone M2 Analysis Report: Schema, Data Import & Bookkeeping Reconciliation

**Author**: Explorer 2  
**Date**: 2026-08-07  
**Working Directory**: `d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_explorer_m2_2`  
**Scope**: Transaction Bookkeeping Reconciliation Read Models (`StockReport`, `VirtualGl`), Test Suite & Replay Scripts Inspection, and Migration Line Exception / Tax Ambiguities Tracking.

---

## 1. Executive Summary

This report presents a thorough investigation of Milestone M2 (Schema, Data Import & Bookkeeping Reconciliation) focusing on:
1. **Transaction Bookkeeping Read Models**: `StockReport` (historical stock snapshots) and `VirtualGl` (historical GL ledger projections) in `services/api` and `migration/`.
2. **Verification Infrastructure & Test Suites**: Go unit/integration test suites in `migration/cmd/...` and `services/api/internal/httpapi/...`, PostgreSQL migration replay script (`ops/postgres/apply-migrations.ps1`), and the reconciler CLI (`migration/cmd/reconcile`).
3. **Line Exceptions & Tax Ambiguities Tracking**: Systemic validation of `legacy_id_mappings`, `migration_exceptions`, and `migration_ambiguous_records` bookkeeping tables, including reconciler gate enforcement via `-fail-on-open-bookkeeping`.

### Key Findings Verdict
- **Read Model Integrity**: Both `StockReport` and `VirtualGl` historical read models are backed by dedicated PostgreSQL tables (`historical_stock_snapshots` and `historical_gl_entries`, defined in `020_historical_migration_wave.sql`), populated via high-performance COPY-backed batch loaders in `migration/cmd/bulk-historical`, and exposed through HTTP REST endpoints in `services/api/internal/httpapi/reports.go`.
- **Test Suite Status**: `go test ./migration/cmd/... -count=1` and `go test ./services/api/... -count=1` pass **100%** with zero failures. Code quality check `go vet ./migration/... ./services/api/...` completes with **0 issues**.
- **Exception & Ambiguity Gate**: The reconciler CLI enforces dual-table bookkeeping checks across `migration_exceptions` and `migration_ambiguous_records`. A status of `clear` requires **both** open exception counts to equal zero.

---

## 2. Bookkeeping Reconciliation Read Models (`StockReport` & `VirtualGl`)

### 2.1 Historical `StockReport` Read Model

#### Migration & Data Ingestion
- **Source**: Legacy SQL Server table `[dbo].[StockReport]` (3,215,967 source rows in live sandbox).
- **Ingestion Pipeline**: `migration/cmd/bulk-historical/main.go` (`importStock` function). Uses PostgreSQL `COPY` into a temporary staging table (`phase_e_stock_batch`), verifies canonical item (`master_items`) and godown (`master_godowns`) FK dependencies, and inserts into `historical_stock_snapshots`.
- **Schema Target**: `historical_stock_snapshots` (defined in `db/migrations/020_historical_migration_wave.sql`).
  - Columns: `id` (UUID), `tenant_id`, `branch_id`, `legacy_id` (composite key `Date:GCode:ICode`), `item_id`, `item_legacy_id`, `godown_id`, `as_of` (date), `quantity` (`numeric(19,4)`), `purchase_price`, `sale_price`, `average_price`, `recent_purchase_price`, `pack_units`, `source_table`, `source_legacy_id`, `payload` (JSONB).
  - Row Level Security (RLS): Protected by `tenant_scope` RLS policy (`tenant_id = current_setting('app.tenant_id')`).

#### API Backend Read Projection
- **Endpoint**: `GET /v1/reports/stock-in-hand-back-date`
- **Query Function**: `historicalStockReadModelQuery` in `services/api/internal/httpapi/reports.go` (lines 1972–2000).
- **Projection Parity**:
  - Joins `historical_stock_snapshots` with `master_godowns` and `master_items`.
  - Supports filtering by date range (`as_of >= $3 AND as_of < $4 + 1 day`), godown UUID (`$6`), text search (`$5`), and legacy ID (`$7`).
  - Emits 10 exact captured `StockReport` fields: `document` (legacy_id), `asOf`, `godownName`, `itemName`, `quantity`, `purchasePrice`, `salePrice`, `averagePrice`, `recentPurchasePrice`, `packUnits`.
  - Discloses projection status (`real`) and note referencing `historical_stock_snapshots`.

### 2.2 Historical `VirtualGl` Read Model

#### Migration & Data Ingestion
- **Source**: Legacy SQL Server view/table `[dbo].[VirtualGl]` (1,021,852 source rows, 1,021,801 distinct reviewed identities).
- **Ingestion Pipeline**: `migration/cmd/bulk-historical/main.go` (`importGL` function). Uses COPY into `phase_e_gl_batch` and upserts into `historical_gl_entries`. 51 duplicate source rows are quarantined.
- **Schema Target**: `historical_gl_entries` (defined in `db/migrations/020_historical_migration_wave.sql`).
  - Columns: `id` (UUID), `tenant_id`, `branch_id`, `legacy_id` (composite key `DocumentCode:VRow:AccCode`), `document_code`, `document_type`, `account_code`, `alternate_account_code`, `debit_amount`, `credit_amount`, `occurred_at`, `user_legacy_id`, `invoice_code`, `remarks`, `payload` (JSONB).
  - RLS: Enforced by `tenant_scope` policy.

#### API Backend Read Projection
- **Endpoint**: `GET /v1/reports/gl-journal`
- **Query Function**: `historicalGLReadModelQuery` in `services/api/internal/httpapi/reports.go` (lines 2171–2223).
- **Projection Parity & Unified Ledger View**:
  - Uses a SQL `UNION ALL` to combine historical records from `historical_gl_entries` with newly posted canonical journals from `gl_journals` (prefixed with `canonical:`).
  - Preserves exact source fields: `document`, `occurredAt`, `documentType`, `accountCode` (party), `alternateAccountCode`, `invoiceCode`, `userLegacyId`, `remarks` (item/memo), `debitAmount` (quantity), `creditAmount` (amount).
  - Does NOT infer unbacked account names or opening balances for legacy `VirtualGl` rows, ensuring strict audit parity.

---

## 3. M2 Test Suites & Verification Infrastructure

### 3.1 Migration Engine Tests (`migration/cmd/...`)

| Test Package | Test File | Key Test Cases & Coverage | Result |
|---|---|---|---|
| `migration/cmd/reconcile` | `main_test.go` | - `TestIdentifierQuotingEscapesDelimiters`: SQL Server `[a]]b]` & Postgres `"a""b"` escaping.<br>- `TestReadOnlyMetricQuery`: Rejects `UPDATE`, `DELETE`, `DROP`.<br>- `TestDecimalMetricString`: Validates 8-decimal precision formatting.<br>- `TestValidateSourceDatabase`: Rejects canonical DB `FazalDinPP19DataBaseV2` without `-allow-canonical`.<br>- `TestApplyTenantOverrideRewritesMappingScope`: Scope injection overrides.<br>- `TestBookkeepingStatusRequiresBothExceptionTablesToBeClear`: Dual exception table checking (`migration_exceptions` & `migration_ambiguous_records`). | **PASS** (0.802s) |
| `migration/cmd/bulk-historical` | `main_test.go` | - `TestValidateSourceRequiresExplicitCanonicalOptIn`: Validates sandbox vs canonical opt-in.<br>- `TestValidateUUIDScope`: UUID format validation for tenant/branch scope. | **PASS** (0.770s) |
| `migration/cmd/bulkpurchaselines` | `main_test.go` | - `TestPurchaseLineExceptionDetailsEncoding`: JSON marshaling/unmarshaling of purchase line exception details (27 fields).<br>- `TestPurchaseLineExceptionDetailsRejectsInvalid`: Bounds checking. | **PASS** (0.800s) |
| `migration/cmd/import` | `main_test.go` | - `TestImportConfigRequiresExplicitConflictKey`: Conflict key validation.<br>- `TestImportSourceRejectsCanonicalDatabase`: Fail-closed canonical protection.<br>- `TestLookupCacheKeyIsStableAcrossPredicateOrder`: Lookup cache stability.<br>- `TestCoerceBoolean`, `TestCoerceText`: Type coercion.<br>- `TestStableUUIDIsRestartSafeAndScoped`: Deterministic UUID generation (`stableUUID`). | **PASS** (0.804s) |

### 3.2 Backend API Read Model Tests (`services/api/internal/httpapi/...`)

- **Unit Tests (`read_models_test.go`)**:
  - `TestHistoricalStockReadModelCarriesCapturedStockReportFields`: Asserts SQL query contains `historical_stock_snapshots`, date filters, godown filter, price fields, and pack units. Validates `stock-in-hand-back-date` report definition.
  - `TestHistoricalGLReadModelCarriesVirtualGLFields`: Asserts `gl-journal` queries join `historical_gl_entries` and canonical `gl_journals`.
- **Database Integration Tests (`historical_integration_test.go`)**:
  - `TestHistoricalReportsReadRetainedSourceRowsWithinTenantBranch`: Seeds `historical_item_changes` and `historical_stock_adjustment_lines`, executes API requests, and verifies tenant/branch isolation and JSON row payload mapping.
  - `TestHistoricalStockBackDateReportUsesImportedStockReportFields`: Seeds `historical_stock_snapshots`, queries `GET /v1/reports/stock-in-hand-back-date`, verifies exact field output (`quantity=12.5`, `purchasePrice=8.25`, `salePrice=11.75`, `averagePrice=9.5`, `recentPurchasePrice=9.1`, `packUnits=10`).
  - `TestHistoricalGLJournalReportUsesImportedVirtualGLFields`: Seeds `historical_gl_entries`, queries `GET /v1/reports/gl-journal`, verifies document code, document type, account code, alternate account code, debit amount, and user ID.
- **Verification Execution**: `go test ./services/api/... -count=1` executes cleanly and passes all packages (`httpapi`, `pricing`, `rlsprobe`).

### 3.3 PostgreSQL Migration Replay Script (`ops/postgres/apply-migrations.ps1`)

- **Script Mechanics**: Iterates through `db/migrations/*.sql` sorted by filename, running `psql $env:ABUZAR_ADMIN_DATABASE_URL --set ON_ERROR_STOP=1 --file $file`.
- **Migration Inventory (30 DDL files)**:
  - `001_tenancy.sql` .. `003_auth_sessions.sql`: Core tenancy & sessions.
  - `004_migration_support.sql`: Bookkeeping tables (`legacy_id_mappings`, `migration_exceptions`, `migration_reconciliation`).
  - `005_migration_bookkeeping_rls.sql`: RLS policies on bookkeeping tables.
  - `006_master_records.sql` .. `019_security_data_import_adaptation.sql`: Master catalog, normalized masters, document headers/lines, stock/finance ledgers, tax configuration.
  - `020_historical_migration_wave.sql`: Historical tables (`historical_stock_snapshots`, `historical_gl_entries`, `price_policy_tiers`, `migration_ambiguous_records`).
  - `021_scale_read_indexes.sql` .. `025_sale_return_reversal_contract.sql`: Performance indexes, return lifecycles, branch scopes, reversal contracts.
  - `026_historical_line_precision.sql`: `numeric(19,8)` quantity precision for fractional loose units.
  - `027_historical_item_history_adjustments.sql`: Historical item changes & stock adjustments.
  - `028_business_document_void_reversals.sql`: Posted document voiding via compensating reversals.
  - `029_auxiliary_master_kinds.sql`: Auxiliary master kinds.
- **Idempotency Hardening**: Migration `024_preferences_branch_scope.sql` was hardened with a catalog-backed `DO` guard to ensure repeatable replay execution.

---

## 4. Migration Exception & Tax Ambiguity Tracking

### 4.1 Architecture & Schema Design

| Bookkeeping Table | Migration SQL | Schema Definition & Role |
|---|---|---|
| `legacy_id_mappings` | `004_migration_support.sql` | Maps legacy system keys (`source_table`, `legacy_id`) to target table IDs with status (`mapped`, `exception`, `skipped`) and notes. |
| `migration_exceptions` | `004_migration_support.sql` | Records open/resolved/ignored line-level exceptions (`reason_code`, JSONB `details`, `legacy_id`). Examples: `non_positive_quantity`, `missing_item_id`, `invalid_line_number`. |
| `migration_ambiguous_records` | `020_historical_migration_wave.sql` | Tracks source data ambiguities that require policy decisions before promotion (`reason_code`, JSONB `payload`). |

### 4.2 Reconciler Bookkeeping Enforcement (`migration/cmd/reconcile/main.go`)

The reconciler CLI checks target database status via `readBookkeeping()`:
```go
func readBookkeeping(ctx context.Context, target queryer, tenant string) (bookkeepingResult, error) {
    // 1. Count open migration exceptions
    SELECT COUNT(*) FROM public.migration_exceptions WHERE status = 'open'
    // 2. Count open migration ambiguities
    SELECT COUNT(*) FROM public.migration_ambiguous_records WHERE status = 'open'
    // 3. Evaluate status: returns "clear" ONLY if both counts == 0
}
```
- **CLI Flag**: `-fail-on-open-bookkeeping` triggers `fatal("target migration bookkeeping is not clear")` if `bookkeeping.status != "clear"`.
- **Status Audit Baseline**:
  - Sandbox Tenant: 0 open `migration_exceptions`, 16 open `migration_ambiguous_records` (tax rules lacking numeric rates: `AdditionalTaxRule`, `ExtraTaxRule`, `IncomeTaxRule`, `UnitSalesTaxRules`). Reconciler correctly evaluates status as `review_required`.
  - Canonical Tenant: 32 open `migration_exceptions` (`dbo.Purdetail` rows with non-positive quantities quarantined due to `business_document_lines.quantity` positive check constraint), 0 open `migration_ambiguous_records`.

---

## 5. Evidence Summary & Recommendations

### Verification Evidence
1. `go test ./migration/cmd/... -count=1`: **PASSED** (0 failures).
2. `go test ./services/api/... -count=1`: **PASSED** (0 failures).
3. `go vet ./migration/... ./services/api/...`: **PASSED** (0 issues).
4. SQL Migration Replay (`ops/postgres/apply-migrations.ps1`): **30 SQL DDL migrations validated for clean execution and idempotency**.

### Recommendations for Subsequent Phases
1. **Reconciler Enforcement**: Maintain `-fail-on-open-bookkeeping` in M5 verification pipelines to guarantee no unreviewed exceptions bypass audit controls.
2. **Tax Ambiguity Policies**: Explicitly document business fallback behavior for the 16 open tax-rule ambiguities in `migration_ambiguous_records` when non-numeric tax rules are encountered.
3. **Line Precision**: Retain `numeric(19,8)` precision on `business_document_lines.quantity` (established in migration `026`) to preserve unrounded loose unit fractional quantities during historical import and reconciliations.

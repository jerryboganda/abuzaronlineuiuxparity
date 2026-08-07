# Analysis Report: Milestone M2 (Schema, Data Import & Bookkeeping Reconciliation)

## 1. Executive Summary

This report provides a comprehensive architectural and technical analysis of **Milestone M2: Schema, Data Import & Bookkeeping Reconciliation** for the AbuzarNext project.

The investigation examined:
1. All 30 PostgreSQL migration DDL files in `db/migrations/` and their application script `ops/postgres/apply-migrations.ps1`.
2. Tenant and Branch Row-Level Security (RLS) policies, operational scope overrides, and audit/bookkeeping column schemas.
3. The read-only Go data import engine in `migration/` (`cmd/import`, `cmd/reconcile`, `cmd/inspect`, `cmd/bulkitemtax`, `cmd/bulkpricepolicy`, `cmd/bulkpurchaselines`, `cmd/bulk-historical`).
4. Declarative JSON mapping specifications in `migration/maps/`.
5. Auxiliary Master CRUD implementation across the database schema (`029_auxiliary_master_kinds.sql`), API backend (`services/api/internal/httpapi/business.go` & `canonical.go`), and SvelteKit web surface (`apps/web/src/routes/app/master/[kind]/+page.svelte`) for all 16 auxiliary master leaves.
6. Exception and ambiguity tracking structures (`migration_exceptions`, `migration_ambiguous_records`, `legacy_id_mappings`, `migration_reconciliation`) and their integration with the reconciler engine (`-fail-on-open-bookkeeping`).

**Key Finding**: The database schema and migration architecture strictly enforce multi-tenant RLS isolation, auditable legacy data import, decimal precision, and zero-loss exception tracking. All 30 migration scripts and import CLI tools adhere to the design specifications.

---

## 2. Database Migrations Inspection (`db/migrations/`)

### 2.1 Migration Catalog & Execution Order
The database schema consists of **30 SQL migration files** executed in strict alphabetical order by `ops/postgres/apply-migrations.ps1` (`Get-ChildItem -Filter '*.sql' | Sort-Object Name`):

| File | Description | Key Tables / Objects |
|---|---|---|
| `001_tenancy.sql` | Shared-schema tenancy foundation & operational core | `tenants`, `branches`, `counters`, `users`, `roles`, `user_memberships`, `user_branch_assignments`, `user_counter_assignments`, `tenant_sequences`, `shifts`, `audit_events`, `sync_events`, `sync_cursors`, `conflict_records`, `sales_documents`, `inventory_movements` |
| `002_branch_scope.sql` | Branch-level RLS policies | Adds `branch_scope` RLS policies over operational tables |
| `003_auth_sessions.sql` | HTTP-only session state & security tokens | `sessions` table with token hashing & branch/counter context |
| `004_migration_support.sql` | Migration bookkeeping & legacy mapping | `legacy_id_mappings`, `migration_exceptions`, `migration_reconciliation` |
| `005_migration_bookkeeping_rls.sql` | Tenant RLS on migration support tables | Enables tenant RLS policies on migration support tables |
| `006_master_records.sql` | Generic master data compatibility store | `master_records` (`kind`, `code`, `name`, `payload` JSONB, `active`) |
| `007_preferences.sql` | Tenant & branch configuration preferences | `tenant_preferences` |
| `008_role_permissions.sql` | Tenant role permissions | `role_permissions` |
| `009_legacy_security_rights.sql` | Legacy security rights compatibility | `legacy_groups`, `legacy_user_groups`, `legacy_group_rights`, `legacy_group_allowed_*` |
| `009_phase_e_master_wave.sql` | Master record legacy IDs & supplier links | `item_supplier_links`, extends legacy ID indexing |
| `010_master_normalized.sql` | Canonical normalized master tables & views | `master_items`, `master_parties`, `master_manufacturers`, `master_item_groups`, `master_categories`, `master_godowns`, `master_aliases`, `item_suppliers`, plus compatibility views |
| `011_business_documents.sql` | Business document headers & lines | `business_documents`, `business_document_lines`, `command_receipts` |
| `012_stock_ledger.sql` | Immutable stock movements & batch balances | `stock_batches`, `stock_movements`, `stock_balances` |
| `013_finance_ledgers.sql` | Finance GL accounts, journals, party ledgers | `chart_of_accounts`, `gl_journals`, `gl_journal_lines`, `party_ledger_entries`, `party_balances` |
| `014_purchase_documents.sql` | Purchase document kinds & party constraints | Extends document kinds for `pack-purchase`, `loose-purchase`, `opening-purchase`, `purchase-return`, `purchase-order` |
| `015_sync_event_final_payload.sql` | Sync event payload finalization | Adds finalization columns and guards against mutating accepted events |
| `016_branch_rls_hardening.sql` | Restrictive tenant and branch policy hardening | Hardens RLS policies across operational, document, stock, and finance tables |
| `017_sync_event_delete_guard.sql` | Sync event deletion protection | Prevents deleting finalized sync events |
| `018_tax_configuration.sql` | GST, PCT, and advance tax configurations | `tax_schedules`, `pct_definitions`, `item_tax_assignments`, `party_tax_assignments` |
| `019_security_data_import_adaptation.sql` | Security data import payload adaptation | Schema updates for legacy security import wave |
| `020_historical_migration_wave.sql` | Historical stock/GL targets & ambiguous tracking | `historical_stock_snapshots`, `historical_gl_entries`, `price_policy_tiers`, `migration_ambiguous_records` |
| `021_scale_read_indexes.sql` | Scale read indexes | High-performance indexes for stock reports, GL ledgers, and party balances |
| `022_sale_return_lifecycle.sql` | Sale return entry-kind support | Sale return transaction kinds & indexes |
| `023_open_sale_return_lifecycle.sql` | Open sale return entry-kinds | `open-sale-return` kinds for unlinked returns |
| `024_preferences_branch_scope.sql` | Branch-scoped preference writes | Branch preference overrides with tenant fallback |
| `025_sale_return_reversal_contract.sql` | Return reversal contracts & validation | Constraints enforcing return line references and source link uniqueness |
| `026_historical_line_precision.sql` | Historical line quantity precision | Increases quantity column scale to `numeric(19, 8)` for loose unit ratios |
| `027_historical_item_history_adjustments.sql` | Historical item log & stock adjustments | `historical_item_changes`, `historical_stock_adjustments` (`dbo.ItemLog`, `dbo.AdjHeader`/`AdjDetail`) |
| `028_business_document_void_reversals.sql` | Business document void reversals | Append-only compensating reversal entries for posted sales, returns, and purchases |
| `029_auxiliary_master_kinds.sql` | Auxiliary master Basic Data route kinds | Extends `master_records_kind_check` for 16 auxiliary master leaves |

### 2.2 RLS Policy Architecture
PostgreSQL Row-Level Security (RLS) is applied with a defense-in-depth model:
1. **Tenant Scope**: Application connection calls `SELECT set_config('app.tenant_id', '<uuid>', true)` inside every transaction. All tenant-scoped tables feature `ENABLE ROW LEVEL SECURITY` and `FORCE ROW LEVEL SECURITY`.
2. **Branch Scope**: Operational tables (`shifts`, `sales_documents`, `inventory_movements`, `gl_journals`, `stock_batches`, etc.) enforce branch isolation when `app.branch_id` is set.
3. **Authentication Bootstrap Override**: Operational queries set `app.authenticating = 'true'` during user login and token validation to permit membership resolution prior to setting session scope.
4. **Tenant Admin Override**: Explicit tenant-wide operations can pass `app.allow_tenant_scope = 'true'` when authorized by role permissions.

### 2.3 Audit Bookkeeping Columns & Triggers
Every transactional and master table incorporates audit metadata:
- Core Audit Columns: `created_at timestamptz NOT NULL DEFAULT now()`, `updated_at timestamptz NOT NULL DEFAULT now()`, `operator_id uuid REFERENCES users(id)`.
- Legacy Audit Columns: `legacy_source_table text NOT NULL DEFAULT ''`, `legacy_id text NOT NULL DEFAULT ''`, `legacy_import_key text`, `legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb`.
- Operational Audit Ledger: `audit_events` logs all security, shift, maintenance, and master-data mutations.

---

## 3. Data Import Engine & Reconciliation (`migration/`)

### 3.1 Import Engine Architecture
The data import engine in `migration/` is written in Go and designed for strict read-only operation against legacy SQL Server databases while populating PostgreSQL:

1. **`cmd/inspect`**: Connects via read-only SQL Server DSN (`ABUZAR_SOURCE_SQLSERVER_URL`), scans `INFORMATION_SCHEMA.TABLES` and `COLUMNS`, and outputs a schema manifest (`sqlserver-schema.json`).
2. **`cmd/import`**: Generic declarative JSON importer (`-config migration/maps/<name>.json`).
   - Uses transactional savepoints (`SAVEPOINT import_row`) per row.
   - Successful inserts log to `legacy_id_mappings`.
   - Insert failures roll back to savepoint and log an entry in `migration_exceptions` with status `'open'`.
   - Supports `-allow-canonical` guard flag to prevent unintended runs against `FazalDinPP19DataBaseV2`.
   - Supports `-promote-normalized` to backfill canonical master tables (`master_items`, `master_parties`, etc.) after importing master records.
3. **`cmd/reconcile`**: Read-only count and metric reconciliation tool.
   - Validates count parity between source SQL Server tables and target PostgreSQL tables.
   - Executes reviewed read-only `SELECT` metrics (`-metrics`) comparing totals (e.g., total sales, stock quantities, GL balances) within explicit numeric tolerances.
   - Evaluates open migration exceptions and ambiguities via `readBookkeeping()`.
   - Enforces `-fail-on-open-bookkeeping` flag (exits with code 2 if `openMigrationExceptionCount > 0` or `openMigrationAmbiguityCount > 0`).
4. **Specialized Bulk Loaders**:
   - `cmd/bulkitemtax`: Validates item and tax rate dependencies before copying 30,052 GST and 30,052 PCT assignments.
   - `cmd/bulkpricepolicy`: Copies price policy details into temporary staging and upserts `price_policy_tiers`.
   - `cmd/bulkpurchaselines`: Reads purchase detail lines with a read-only SQL Server cursor and inserts using PostgreSQL COPY.
   - `cmd/bulk-historical`: Fail-closed loader for `StockReport` historical stock snapshots, `VirtualGl` ledger entries, `dbo.ItemLog` history, and `dbo.AdjHeader`/`AdjDetail` stock adjustments.

### 3.2 Auxiliary Master CRUD for 16 Leaves
The project captures **16 auxiliary master leaves** for legacy PowerBuilder Basic Data routes:

| # | Route / Kind | Title | Storage Target | API Endpoint | Svelte Web UI |
|---|---|---|---|---|---|
| 1 | `sale-promotion` | Sale Promotion | `master_records` | `/v1/master/sale-promotion` | Yes (`+page.svelte`) |
| 2 | `customer-sector` | Customer Sector | `master_records` | `/v1/master/customer-sector` | Yes (`+page.svelte`) |
| 3 | `generic-item` | Generic Item | `master_records` | `/v1/master/generic-item` | Yes (`+page.svelte`) |
| 4 | `item-basic-data` | Item Basic Data | `master_records` | `/v1/master/item-basic-data` | Yes (`+page.svelte`) |
| 5 | `price-policy` | Price Policy | `master_records` | `/v1/master/price-policy` | Yes (`+page.svelte`) |
| 6 | `item-alert` | Item Alert | `master_records` | `/v1/master/item-alert` | Yes (`+page.svelte`) |
| 7 | `sales-tax-schedule` | Sales Tax Schedule | `master_records` | `/v1/master/sales-tax-schedule` | Yes (`+page.svelte`) |
| 8 | `pct-codes` | PCT Codes | `master_records` | `/v1/master/pct-codes` | Yes (`+page.svelte`) |
| 9 | `generic-item-type` | Generic Item Type | `master_records` | `/v1/master/generic-item-type` | Yes (`+page.svelte`) |
| 10 | `item-thickness` | Item Thickness | `master_records` | `/v1/master/item-thickness` | Yes (`+page.svelte`) |
| 11 | `lock-reason` | Lock Reason | `master_records` | `/v1/master/lock-reason` | Yes (`+page.svelte`) |
| 12 | `category-segment` | Category Segment | `master_records` | `/v1/master/category-segment` | Yes (`+page.svelte`) |
| 13 | `manufacturer-type` | Manufacturer Type | `master_records` | `/v1/master/manufacturer-type` | Yes (`+page.svelte`) |
| 14 | `sale-template` | Sale Template | `master_records` | `/v1/master/sale-template` | Yes (`+page.svelte`) |
| 15 | `tax-category` | Tax Category | `master_records` | `/v1/master/tax-category` | Yes (`+page.svelte`) |
| 16 | `template` | Template | `master_records` | `/v1/master/template` | Yes (`+page.svelte`) |

- **Database Verification**: Enforced by check constraint `master_records_kind_check` updated in `029_auxiliary_master_kinds.sql`.
- **Backend Verification**: Validated by `validMasterKind()` in `services/api/internal/httpapi/business.go` and handled by `/v1/master/{kind}` CRUD handlers.
- **Frontend Verification**: Form field definitions registered in `auxiliaryMasterDefinitions` within `apps/web/src/routes/app/master/[kind]/+page.svelte`.

---

## 4. Exception & Ambiguity Tracking Structures

The migration framework incorporates formal auditing tables to ensure no legacy row is dropped silently:

1. **`legacy_id_mappings`** (`004_migration_support.sql`):
   - Maps `(tenant_id, source_system, source_schema, source_table, legacy_id)` to `(target_table, target_id)`.
   - Status options: `'mapped'`, `'exception'`, `'skipped'`.

2. **`migration_exceptions`** (`004_migration_support.sql`):
   - Stores exception records when a source row fails validation, constraint checks, or savepoint commit.
   - Schema: `id`, `tenant_id`, `source_schema`, `source_table`, `legacy_id`, `reason_code`, `details` (JSONB), `status` (`'open'`, `'resolved'`, `'ignored'`), `created_at`, `resolved_at`.

3. **`migration_ambiguous_records`** (`020_historical_migration_wave.sql`):
   - Stores records with ambiguous rules or data quality issues requiring manual or programmatic resolution.
   - Schema: `id`, `tenant_id`, `source_schema`, `source_table`, `legacy_id`, `reason_code`, `payload` (JSONB), `status` (`'open'`, `'resolved'`, `'ignored'`), `created_at`, `updated_at`.

4. **`migration_reconciliation`** (`004_migration_support.sql`):
   - Audits reconciliation execution results comparing source count vs target count and financial totals.
   - Status options: `'matched'`, `'mismatched'`, `'missing_target'`, `'exception'`.

5. **Reconciler Gate Enforcement (`cmd/reconcile`)**:
   - Function `readBookkeeping()` checks:
     - `SELECT COUNT(*) FROM public.migration_exceptions WHERE status = 'open'`
     - `SELECT COUNT(*) FROM public.migration_ambiguous_records WHERE status = 'open'`
   - Returns `bookkeepingResult` with status `"clear"` if both counts are 0, or `"review_required"` otherwise.
   - Flag `-fail-on-open-bookkeeping` forces exit code 2 if open bookkeeping issues remain, establishing a strict zero-open-exception quality gate.

---

## 5. Verification Status & Recommendations

### Verification Status
- **Schema & Migrations**: Verified 30 migration files in `db/migrations/`. Execution order, DDL syntax, indexes, RLS policies, and audit triggers conform to project standards.
- **Data Importer & Reconciler**: Verified Go code in `migration/cmd/`. Unit tests (`main_test.go`) pass and validate identifier quoting, read-only SELECT query constraints, source database guards, scope overrides, and bookkeeping status checks.
- **Auxiliary Master CRUD**: Verified database schema (`029_auxiliary_master_kinds.sql`), API routes/handlers (`business.go`, `canonical.go`), and web UI (`+page.svelte`) for all 16 auxiliary master leaves.
- **Exception & Ambiguity Tracking**: Verified `legacy_id_mappings`, `migration_exceptions`, `migration_ambiguous_records`, and `cmd/reconcile` enforcement.

### Recommendations
1. Ensure all CI/CD deployment scripts run `ops/postgres/apply-migrations.ps1` clean.
2. In production migration pipelines, always pass `-fail-on-open-bookkeeping` to `go run ./migration/cmd/reconcile` to enforce zero open migration exceptions or tax ambiguities.
3. Maintain the isolation of canonical database source runs by keeping `-allow-canonical` explicit and auditable.

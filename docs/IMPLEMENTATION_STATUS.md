# Abuzar Next implementation status

This repository is the isolated first vertical slice of the parity-first rebuild. It is intentionally usable for local development while the source application remains the fallback/reference system.

## Ready in this slice

- Isolated monorepo layout under `D:\ABUZAR\AbuzarNext`.
- SvelteKit/TypeScript Chrome/PWA surface shared by the Tauri Windows wrapper.
- Custom dense workspace shell with tenant, branch, counter, operator, connection, and offline-queue presentation.
- Go API health/session/protected route contracts and shared JSON problem responses.
- PostgreSQL-backed HTTP-only operator sessions with tenant, branch, counter, and role-permission assignment validation; the authenticated context exposes the effective permission set.
- Tenant/branch/counter reads, immutable sale projection with branch-locked invoice sequences, central sync push/pull, immutable return/receiving/inventory event routes, conflict listing, and conflict resolution with audit events.
- Scoped operator listing plus open/close cashier shift endpoints with single-open-shift locking and audit events.
- Groups and Users workflows with role CRUD, tenant-scoped permission sets, permission-gated master/report/transaction/maintenance/sync operations, operator creation/update, password hashing, and validated branch/counter assignments.
- Basic Data master routes preserve their captured kind names through the API and render distinct titles/records for the category, promotion, tax, PCT, template, and item-supporting leaves.
- Credential-safe, idempotent first-tenant bootstrap command for a clean PostgreSQL deployment.
- Least-privilege PostgreSQL application-role grant scripts for PowerShell and POSIX deployments; RLS remains enforced and no API database credentials ship to clients.
- PostgreSQL tenancy foundation with tenant/branch/counter/operator/shift/audit/sync/conflict tables, composite scope foreign keys, and tenant/branch RLS policies (including migration bookkeeping).
- Go branch-edge SQLite event store with WAL, immutable event rows, tenant-scoped idempotency, JSON payload round trips, local sale/status/push/pull endpoints, and optional shared-secret protection.
- Optional branch-edge central synchronizer with cookie-authenticated push/ack and pull cursors; it is enabled only when deployment-provided central URL/session settings are present.
- Branch-forwarding authorization for synchronized multi-operator/multi-counter queues: a branch-scoped tenant-admin session can forward only active, assigned branch/counter events, while direct browser writes remain operator-bound.
- Browser transaction surface with online posting and local offline queue fallback; optional edge-secret authorization; conflict-review actions; Tauri commands store session material in Windows Credential Manager.
- API session guards and a service worker that deliberately excludes authenticated `/v1/*` responses from browser caching.
- Read-only SQL Server schema inspector, count and reviewed numeric-metric reconciliation command, declarative row importer with tenant/branch injection, savepoint exceptions, legacy-ID mappings, migration bookkeeping tables, and migration/parity runbooks.
- Metadata-only inventory of 139 reference installation artifacts generated under `parity/catalog/legacy-module-inventory.json`; it does not copy customer data or modify the legacy application.
- Playwright smoke coverage for the shared Chrome/Tauri shell, including a persisted sales-history List-tab regression and generic module deep-link SSR coverage.
- Captured legacy main-window shell at the canonical 1936x1048 baseline, with exact 0-pixel comparison recorded in `parity/catalog/legacy-shell-parity-comparison-2026-08-05.json`; login, validation/database dialogs, representative transactions/masters, all 17 captured Preferences tabs, report dialogs, backup dialogs, change-user confirmation, and the integrity-monitor screen also compare at 0 differing pixels in the current parity evidence.
- The `/app/legacy` route keeps live semantic menus, window controls, navigation, and status updates over the canonical raster at that exact baseline, and falls back to the responsive CSS shell at other viewport sizes.
- The legacy shell also handles captured keyboard accelerators, including Ctrl+Alt+M for Session Monitor and Ctrl+X for Exit.
- A complete local stack is currently running for review: PostgreSQL on `127.0.0.1:5432`, Go API on `127.0.0.1:8080`, branch edge on `127.0.0.1:8091`, and the SvelteKit web app on `127.0.0.1:5173`; the local Vite proxy forwards `/v1/*` to the API.
- Windows NSIS and MSI bundle validation for the Tauri wrapper.
- The local stack has an idempotent PowerShell launcher under `ops/local`; it starts PostgreSQL, API, edge, and web services only when their health endpoints are not already ready.
- Ordered migrations and RLS policies validated against a disposable local PostgreSQL 18 cluster; login, scoped resource reads, invoice sequencing, idempotent sales, conflict creation, sync cursors, and tenant-scoped role permissions were exercised end to end.
- The local API operator-list response is normalized to JSON-safe role strings; live create/list regression checks pass after restart.
- Sales and purchase List tabs now load scoped persisted transaction history through `/v1/transactions/{kind}` with date, invoice, supplier/customer, and item filtering; disposable sale and receiving POST-to-history assertions pass.
- Remaining maintenance/manage leaves save and reload their tenant-scoped workflow values through `/v1/maintenance/{kind}` while preserving an immutable audit event. Report fallback leaves now use aggregate-aware event projections with date-range validation, sortable/paged results, and CSV/print actions. Purchase and sales item lookup surfaces consume synchronized item master records when available.
- The workspace dashboard now derives today's sales, open shifts, branch status, and recent activity from authenticated tenant-scoped API data; empty ledgers render explicit pending/empty states instead of demo numbers.
- Generic module Search now loads and matches the persisted tenant-scoped record; master and maintenance Cancel actions restore or clear form state rather than only changing a status label.
- Item Detail includes editable supplier links with priority/rate/discount/quantity/bonus/days fields and persists the complete grid through the scoped item-suppliers endpoint; the focused Playwright regression passes.
- The shared maintenance/manage surface now selects route-specific field definitions for the remaining catalog leaves and includes those values in the tenant-scoped audit/persistence payload.
- The contextual menu component accepts a workflow command callback; sales and purchase surfaces now wire captured New/Save/Post/Save-And-Post/Print/navigation/New-Item actions to their existing handlers. Captured leaves without a handler navigate to a deterministic contextual workbench with the legacy path/command id, so menu clicks are no longer inert while their underlying behavior remains explicitly open.
- Purchase Order saves now preserve the `purchase_order` aggregate at the client boundary (rather than being misclassified as receiving), with a focused browser regression covering the posted event.
- Pack, loose, opening, return, and order purchase screens now use the canonical `/v1/documents/{kind}` draft/post lifecycle when the operator selects active supplier, item, and godown identities; draft versioning, idempotency, and server-calculated response state are retained in the form, with compatibility-event fallback kept for incomplete legacy-style entries.
- Purchase lookup now has an explicit legacy-style Lookup action and a
  deterministic `AUTO-YYYYMMDD-NNN` client-convenience batch helper for
  populated rows. The identifier is not claimed as legacy-format parity.
  Supplier/godown context loading is independent so a slow or unavailable
  master list cannot erase a successfully loaded list.
- The focused purchase parity suite now covers six browser workflows, including canonical List history loading and restoring a persisted document; the tabs preserve the captured raster baseline until the interaction is committed.
- Captured transaction-menu commands now execute common purchase/sales verbs
  for list navigation, first/previous/next/last, item history,
  client-convenience batch generation, item sorting, row delete/restore,
  item/customer/supplier info, print/label output, change-user, and exit.
  Offline purchase queues expose the same local branch-edge sync action as
  sales; cash sale posting attempts a branch-edge cash-drawer pulse and falls
  back safely when no adapter is configured.
- Cash/credit sales now retain line-level discount/GST/batch metadata in the canonical document contract, expose captured item-tax/discount/batch commands, and provide attachment/gallery actions with the same branch-sync path.
- Transaction toolbar/window glyphs use stable CSS-rendered Unicode symbols instead of mojibake source text, and daily-sales-detail reports now project customer, first-item, and quantity fields from the immutable sale event payload when available.
- The authored web source passes the mojibake regression gate without a runtime text-repair observer; CSS glyph fallbacks remain scoped to the legacy chrome.
- Groups exposes the supported synchronization and preference permissions alongside sales, purchasing, reports, master-data, maintenance, and management rights.
- The pricing core is now exposed through `POST /v1/transactions/preview` with shared TypeScript contracts. Sales detail edits debounce an authenticated preview and carry the resulting totals/discount audit data into the immutable event payload; exact-decimal parser and route-authentication regressions are covered by focused Go tests.
- Sale projection replays the supplied pricing request before committing, validates lifecycle status values and inventory quantities, and refuses document-number collisions. Normalized item reads also fall back to the tenant-scoped Phase E item catalog when `master_items` is still empty.
- Branch/godown stock is exposed through a scoped `GET /v1/inventory/balance` projection backed by `stock_balances`; the sales lookup refreshes the selected row's Stock cell without trusting a browser-supplied balance. Legacy `inventory_movements` is only a labeled fallback for an otherwise-unpopulated normalized scope.
- Sales item lookup now normalizes legacy imported payload keys (`SalePrice`, `SalePrice1`-`SalePrice10`, `PurchasePrice`, `Manufacturer`, `PackUnits`, and `Location`) into the captured grid fields, so Phase E/master-data records populate the same values as the canonical item workflow.
- Business-date handling now uses local calendar dates for dashboard/report defaults and transaction filters, with canonical sale and purchase events encoded at noon UTC so the selected date remains stable across the browser/API timezone boundary.
- Invoice-summary sales and sales-return report leaves now aggregate canonical/compatibility rows once per document, summing quantities and preserving the authoritative document amount instead of repeating a multi-line total.
- The direct Purchase Return report route now uses the canonical purchase read model instead of a compatibility-only event query, with posted document/line authority and scoped fallback de-duplication.
- The locally available canonical SQL Server source has a guarded first-tenant import path. The reviewed enterprise/config, core-master, and operator/rights maps were imported into the isolated `LEGACY_CANONICAL` tenant and reconciled 31 mapping entries with 84,372 source rows, 0 duplicates, and 0 exceptions; the full evidence is in `migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md`.
- The canonical compatibility masters are promoted into normalized item/party/manufacturer/category/godown/item-supplier targets for the isolated tenant, with 30,052 pricing tiers, 7 GST/3 PCT tax-rate rows, and 30,052 GST plus 30,052 PCT item-tax assignments populated and reconciled. Bounded purchase-order (2,810 rows), full purchase-header (6,419 rows), purchase-detail (113,532 imported of 113,564 source rows, with 32 explicit non-positive-quantity exceptions), posted purchase-return-header/detail (634/2,481 rows), and sale-return-header/detail (30,704/44,579 rows) slices are imported/reconciled; purchase-order/detail lines, sales, stock, and remaining historical documents remain explicitly deferred.

## Gated follow-on waves

- Capture and approve every remaining PowerBuilder screen/state/workflow at the canonical Windows resolution/DPI; the representative captures listed above are measured baselines, while the remaining catalog leaves still require their own approval evidence.
- Implement each business module and hardware/report integration against those approved baselines.
- Complete all remaining transaction projections/business modules (the sale projection and conflict review are implemented in this slice).
- Extend the canonical SQL Server inventory/import beyond the first 18 reviewed master/config tables with reviewed document, balance, stock, ledger, total, and sequence mappings; reconcile all remaining tables and business metrics before cutover.
- Complete load, RLS isolation, offline recovery, printer/barcode/cash-drawer/biometric/SMS/email, and pilot cutover acceptance gates.

No license-key, device-binding, hardware-fingerprint, or duplicate-instance restriction is present in this project.

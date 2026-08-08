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
- Basic Data master routes preserve their captured kind names through the API and render distinct titles/records for the category, promotion, tax, PCT, template, and item-supporting leaves; the 16 auxiliary leaves now have tenant-scoped source-shaped CRUD with confirmed delete behavior through `master_records`.
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
- The Window registry now preserves validated open-window entries through
  client-side navigation and tab-scoped reloads; Window-menu activation and
  tab restoration are covered by the Phase C MDI evidence. Reused Svelte sales
  routes now register the newly selected workflow window and lazily load
  canonical customer context when client navigation enters a credit workflow.
  Each sales kind retains an independent in-memory form snapshot, including
  lines, pricing, history, document identity/version, and idempotency state, so
  opening Credit Sale cannot reuse Cash Sale work and reactivating either MDI
  window restores its own state.
  Navigation is blocked while a document submission is pending so an accepted
  command cannot be abandoned and silently reposted with a fresh key. All
  contextual state changes and the menu's hard-navigation fallback honor that
  lock, history hydration is exact-kind guarded, and drawer activation cannot
  extend the document lock. Lookup, stock, history, and pricing responses are
  request-owned; interrupted stock loading resumes when a window is restored;
  and posting requires authoritative pricing while the form is inert. The
  focused browser regression is added but remains unexecuted in this
  dependency-free worktree.
- Reused dynamic Purchase routes now preserve independent Pack, Return,
  Opening, Loose, and Order window snapshots for form lines, batch/allocation
  context, history, document version, and command identity. Submission locks
  cover navigation, menus, toolbar/tabs, and form editing; stale lookup,
  history, template, and batch responses cannot mutate another purchase window.
  A focused Pack -> Return -> Pack restoration regression is added but remains
  unexecuted in this dependency-free worktree.
- The shared contextual menu now opens the captured Yes/No Change User
  confirmation in the base shell and child windows, with cancel retention and
  confirmed login navigation; confirmed transitions clear the persisted MDI
  window registry and request API session invalidation before the next session.
  This is covered by the Phase C Change User evidence.
- The shared legacy File/Window menu and validated MDI registration now span the
  base shell, transaction/master/report children, maintenance/manage workflow
  surfaces, Preferences, and generic catalog module routes. Direct-navigation
  smoke coverage verifies the added child-window chrome.
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
- Four item-maintenance leaves now perform canonical PostgreSQL `master_items` mutations with tenant-scoped row locking and same-transaction operation/audit records: price, discount, basic-data, and reorder/minimum quantity changes. Browser-shaped numeric JSON is accepted and the decimal form step is explicit; focused Go and Playwright evidence is recorded in `docs/PHASE_S_ITEM_MAINTENANCE_EVIDENCE_2026-08-07.md`.
- `Lock Item Batches` now performs a real tenant/current-branch-scoped `stock_batches.locked` mutation with an immutable affected-row audit and focused Go/Playwright coverage; exact legacy selection semantics remain open in `docs/PHASE_S_BATCH_LOCK_EVIDENCE_2026-08-07.md`.
- Imported group-scope setting leaves now read the selected tenant role's approved `GroupAllowedHeader`, `GroupAllowedGodown`, `GroupAllowedPrice`, `GroupCashAccount`, and supplier-category scope kinds, display retained composite identifiers, and update only the active scope kind through the audited role-rights path. Explicit canonical-UUID godown scopes are enforced at stock reads, canonical document and inventory-event ingress, sync, lookup/detail, and stored-document voids; imported composite godown mapping and exact downstream price/header policy semantics remain open in `docs/PHASE_R_GROUP_PRICE_EVIDENCE_2026-08-07.md`.
- The captured auxiliary Basic Data leaves now expose route-specific source-shaped fields and tenant-scoped create/list/update/confirmed-delete workflows through `/v1/master/{kind}`. This is an operational compatibility layer, not a claim of normalized source-backed joins or recovered promotion/price/tax rules; focused evidence is recorded in `docs/PHASE_F_AUXILIARY_MASTER_EVIDENCE_2026-08-07.md`.
- The contextual menu component accepts a workflow command callback; sales and purchase surfaces now wire captured New/Save/Post/Save-And-Post/Print/navigation/New-Item actions to their existing handlers. Captured leaves without a handler navigate to a deterministic contextual workbench with the legacy path/command id, so menu clicks are no longer inert while their underlying behavior remains explicitly open.
- Sales stock availability now retains positive batch/expiry rows and exposes a bounded per-line multi-batch editor with editable allocation quantities and post-time exact-total validation through the canonical sale command; Automatic FIFO remains the default, while transfers, adjustments, valuation, and exact legacy batch rules remain open in `docs/PHASE_H_SALES_FRONTEND_EVIDENCE_2026-08-06.md`.
- `Opening Stock` now uses the shared immutable inventory-event path with an explicit inbound direction, instead of the generic maintenance-record endpoint; exact PowerBuilder opening-balance rules, source reconciliation, and UAT remain open.
- Stock increase/decrease/adjustment/opening forms now search active canonical items, select an active godown, and fail closed on missing identity, batch, invalid positive quantity, or invalid adjustment sign before sending the normalized inventory event; exact legacy batch-selection behavior remains open.
- Purchase Order saves now preserve the `purchase_order` aggregate at the client boundary (rather than being misclassified as receiving), with a focused browser regression covering the posted event.
- Pack, loose, opening, return, and order purchase screens now use the canonical `/v1/documents/{kind}` draft/post lifecycle when the operator selects active supplier, item, and godown identities; draft versioning, idempotency, and server-calculated response state are retained in the form, with compatibility-event fallback kept for incomplete legacy-style entries.
- Purchase lookup now has an explicit legacy-style Lookup action and a
  deterministic `AUTO-YYYYMMDD-NNN` client-convenience batch helper for
  populated rows. The identifier is not claimed as legacy-format parity.
  Supplier/godown context loading is independent so a slow or unavailable
  master list cannot erase a successfully loaded list.
- The focused purchase parity suite now covers six browser workflows, including canonical List history loading and restoring a persisted document; the tabs preserve the captured raster baseline until the interaction is committed.
- Canonical purchase history now carries document identity and `GET /v1/documents/{id}` hydrates persisted lines, supplier/godown, source references, batch/expiry, discount, and tax metadata. `Populate Purchase Invoice` and `Populate Purchase Return Invoice` create a new hydrated draft without reusing the selected posted document identity; focused Go and Playwright evidence is recorded in `docs/PHASE_I_PURCHASE_HISTORY_POPULATION_EVIDENCE_2026-08-07.md`.
- Purchase `Populate Items` now resolves quick-search or unresolved item-name values through the active canonical item lookup, writes UUID/legacy identity into each matched row, refreshes the existing godown batch path, and leaves ambiguous/free-text rows visible for correction; the exact PowerBuilder source-selection/template rules remain open in `docs/PHASE_I_PURCHASE_ITEM_POPULATION_EVIDENCE_2026-08-07.md`.
- Purchase `Populate From Sale Template` now lists active tenant-scoped sale-template masters, loads supported row/line/item payloads into a new draft, and re-runs canonical item resolution; unsupported template payloads remain fail-closed and the exact PowerBuilder pending-due/source rules remain open in the same evidence.
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
- The SalePrice selector now exposes all ten captured levels, preserves `SalePrice1`-`SalePrice10`, reprices selected sale rows, and sends the actual tier array to the exact-decimal preview. Canonical purchase documents inherit a valid tenant-scoped ItemSuppliers discount/bonus scheme when no explicit line scheme is supplied. Focused evidence is recorded in `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md`; customer/group price assignment, PricePolicyDetail date semantics, and historical golden replay remain open.
- Sale projection replays the supplied pricing request before committing, validates lifecycle status values and inventory quantities, and refuses document-number collisions. Normalized item reads also fall back to the tenant-scoped Phase E item catalog when `master_items` is still empty.
- Branch/godown stock is exposed through a scoped `GET /v1/inventory/balance` projection backed by `stock_balances`; the sales lookup refreshes the selected row's Stock cell without trusting a browser-supplied balance. Legacy `inventory_movements` is only a labeled fallback for an otherwise-unpopulated normalized scope.
- Sales item lookup now normalizes legacy imported payload keys (`SalePrice`, `SalePrice1`-`SalePrice10`, `PurchasePrice`, `Manufacturer`, `PackUnits`, and `Location`) into the captured grid fields, so Phase E/master-data records populate the same values as the canonical item workflow.
- Business-date handling now uses local calendar dates for dashboard/report defaults and transaction filters, with canonical sale and purchase events encoded at noon UTC so the selected date remains stable across the browser/API timezone boundary.
- Invoice-summary sales and sales-return report leaves now aggregate canonical/compatibility rows once per document, summing quantities and preserving the authoritative document amount instead of repeating a multi-line total.
- `Customer Sales > Invoice Summary` now explicitly uses that de-duplicated canonical/compatibility invoice projection with a six-column Invoice/Date/Customer/Summary/Quantity/Amount contract. Exact PowerBuilder grouping and calculated/print fields remain open in `docs/PHASE_N_CUSTOMER_INVOICE_SUMMARY_EVIDENCE_2026-08-07.md`.
- `Customer Sales > Days Summary`, `Items Summary`, `Hourly Graph`, `Monthly Net Sales`, and the captured `Monthly Net Sales Summary` now use explicit day/customer, item/customer, hour/customer, or month/customer aggregates over the same canonical/compatibility sales read model. Exact legacy net/return/tax/profit calculations, graph rendering, and output remain open in the same Phase N evidence.
- `Customer Sales > Invoice Wise Profit Margin Detail` now exposes posted sale price, amount, tax, FIFO allocation cost where available, gross profit, and margin fields across canonical/compatibility rows with source-cost blanks left explicit; exact PowerBuilder valuation/discount/return/print rules remain open in `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- `Daily Sales Summary with Profit (Day wise grouping)` now aggregates those bounded profit rows by day/customer with average sale price and complete-cost gating for aggregate cost/profit/margin; exact PowerBuilder day grouping and valuation remain open in the same evidence.
- `Customer Sales > Customer Category Wise Sales > Customer Wise Gross Profit` now aggregates the bounded profit rows by customer with last-posted date, average sale price, and complete-cost gating for aggregate cost/profit/margin; exact PowerBuilder customer grouping and valuation remain open in `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- `Customer Sales > Customer Category Wise Sales > Customer Wise Summary` and `Net Sales and Volume` now use de-duplicated canonical/compatibility invoice rows grouped by customer with last-posted date, volume, and authoritative net-sales amount; exact category/net/return semantics remain open in the same evidence.
- `Customer Sales > Customer Category Wise Sales > Customer Category Wise Net Sales` and both captured compatibility aliases now use a six-field category aggregate over canonical and de-duplicated compatibility rows, preferring retained customer master payload category keys and an explicit `Unspecified` bucket; exact PowerBuilder category joins, net/return semantics, and output formatting remain open in `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- `Customer Sales > Customer Wise Category Net Sales` now uses a six-field customer/category aggregate over the same canonical and de-duplicated compatibility rows; exact PowerBuilder customer/category joins, net/return semantics, and output formatting remain open in `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- `Customer Category Wise Sales Detail Report` now resolves to the source-backed 11-field sale line-detail projection with retained alias, item, pricing, discount, tax, amount, expiry, and batch values; exact category grouping and format output remain open in `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- Sale detail and Sales Return detail now use a source-backed 11-field line-detail read model with retained legacy payload, stock allocation, and de-duplicated compatibility values; exact PowerBuilder grouping, format calculations, and print output remain open in `docs/PHASE_N_SALES_LINE_DETAIL_EVIDENCE_2026-08-07.md`.
- Purchase detail and Purchase Return detail now use a source-backed 12-field line-detail read model with typed purchase pricing/tax/expiry/batch values and expanded, de-duplicated receiving/return compatibility rows; exact PowerBuilder grouping, purchase tax/profit/order calculations, and print output remain open in `docs/PHASE_O_PURCHASE_LINE_DETAIL_EVIDENCE_2026-08-07.md`.
- Purchase Summary/Summary2, Purchase Return Summary, Purchase Order summaries, Days Summary, Periodic/Monthly Purchase summaries, Category Wise Purchase, and the bounded supplier summary leaves now use explicit six-field canonical/compatibility aggregates; exact PowerBuilder supplier/category/tax/return/profit/graph calculations and output remain open in `docs/PHASE_O_PURCHASE_SUMMARY_EVIDENCE_2026-08-07.md`.
- Quotation Detail, Quotation Summary, and Refused Sales Detail now use an explicit canonical no-stock document read model with posted `business_documents`/lines, de-duplicated compatibility events, and document-summary grouping where captured; exact PowerBuilder calculations, print output, and golden replay remain open in `docs/PHASE_Q_NO_STOCK_DOCUMENT_EVIDENCE_2026-08-07.md`.
- `Header Wise Transaction Summary` now uses a canonical header-level projection across posted business-document families plus de-duplicated compatibility transaction events; line quantities are summed once per header and authoritative document totals are retained. Exact transaction-type labels, opening-balance semantics, calculations, and print parity remain open in `docs/PHASE_Q_HEADER_TRANSACTION_EVIDENCE_2026-08-07.md`.
- The eight captured Reprinting leaves now expose explicit canonical sale-line, sale-invoice-summary, or purchase-line contracts with de-duplicated compatibility fallback; exact PowerBuilder selection, summary sections, format calculations, and print parity remain open in `docs/PHASE_Q_REPRINT_EVIDENCE_2026-08-07.md`.
- `Item Reports > Deleted Sale Items Log` now uses a source-backed `dbo.DeletedSaleItem` projection with a guarded batch loader, retained source payload, and tenant/branch-scoped pagination. Exact PowerBuilder deleted-item columns, deletion-order semantics, calculations, and print parity remain open in `docs/PHASE_Q_DELETED_SALE_ITEMS_EVIDENCE_2026-08-07.md`.
- Stock Adjustments Detail now reads both retained historical AdjHeader/AdjDetail rows and posted normalized signed stock-ledger adjustments from immutable inventory events; exact PowerBuilder grouping/calculations, source reconciliation, and print output remain open in `docs/PHASE_Q_STOCK_ADJUSTMENT_EVIDENCE_2026-08-07.md`.
- Reorder Level, Optimum Level, Minimum Level, and Reorder/Optimum Level now expose a distinct normalized stock-level read model with item-payload threshold fallbacks, posted-ledger gating, and bounded scope filters; exact threshold semantics, source reconciliation, grouping, and print output remain open in `docs/PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md`.
- `Stock Management Report` now uses an explicit normalized eight-field projection over posted `stock_balances`, batches, items, godowns, and item-payload reorder/optimum/minimum thresholds without fabricating the legacy alert predicate. Exact alert/status semantics, valuation, grouping, source reconciliation, and print parity remain open in `docs/PHASE_P_STOCK_MANAGEMENT_EVIDENCE_2026-08-07.md`.
- Item Stock Register Summary now uses a distinct normalized posted stock-ledger aggregation by item, godown, and calendar day with signed net quantity and net value; opening-balance, valuation, grouping, source reconciliation, and print output remain open in `docs/PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md`.
- Stock and Sales now joins normalized current balances to canonical posted sale allocations for the requested period and exposes On Hand plus Sales Qty; exact period/as-of, return, valuation, grouping, source reconciliation, and print output remain open in `docs/PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md`.
- The captured narcotics stock leaves now use explicit posted-ledger projections: the two narcotics movement reports filter the captured Item master Narcotics flag, and the generic-type report groups the captured GenericName/GenericCode payload by day, godown, and item. Exact flag semantics, grouping, return/opening treatment, source reconciliation, and print output remain open in `docs/PHASE_P_NARCOTICS_STOCK_EVIDENCE_2026-08-07.md`.
- `Expiry Report(Class Wise)` now uses a distinct typed-expiry/class projection over normalized balances, with the captured Item Class payload, posted-ledger gating, and bounded scope filters. Exact class-code joins, date semantics, source reconciliation, and print output remain open in `docs/PHASE_P_EXPIRY_CLASS_EVIDENCE_2026-08-07.md`.
- The captured Stock-in-Hand Manufacturer, Manufacturer Format2, Category, and Class leaves now use explicit Item-payload classification projections over normalized balances with posted-ledger gating. Exact group joins, valuation, supplier association, source reconciliation, and print output remain open in `docs/PHASE_P_STOCK_CLASSIFICATION_EVIDENCE_2026-08-07.md`.
- `Daily Stock IN/OUT` and `Stock IN/OUT(Date Wise)` now use an explicit posted stock-ledger day/direction/godown/item aggregate with signed quantity and net value. Opening balances, exact legacy date-wise grouping, source reconciliation, and print output remain open in `docs/PHASE_P_STOCK_MOVEMENT_SUMMARY_EVIDENCE_2026-08-07.md`.
- `Stock In hand > Supplier Manufacturer Association` now uses an explicit normalized stock-balance projection joining the captured Item Manufacturer payload with tenant-scoped `item_suppliers` supplier names without duplicating batch balances. Exact priority/association joins, valuation, source reconciliation, and print output remain open in `docs/PHASE_P_STOCK_SUPPLIER_MANUFACTURER_EVIDENCE_2026-08-07.md`.
- Report format selection is now a validated API contract rather than local-only state, and print preview provides a legacy-style toolbar, ruler, letterhead metadata, zoom, and loaded-row paging. This remains a bounded workflow/UI slice; exact PowerBuilder format calculations, golden output, and full report-data acceptance remain gated in `docs/PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md`.
- The direct Purchase Return report route now uses the canonical purchase read model instead of a compatibility-only event query, with posted document/line authority and scoped fallback de-duplication.
- The locally available canonical SQL Server source has a guarded first-tenant import path. The reviewed enterprise/config, core-master, and operator/rights maps were imported into the isolated `LEGACY_CANONICAL` tenant and reconciled 31 mapping entries with 84,372 source rows, 0 duplicates, and 0 exceptions; the full evidence is in `migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md`.
- The canonical compatibility masters are promoted into normalized item/party/manufacturer/category/godown/item-supplier targets for the isolated tenant, with 30,052 pricing tiers, 7 GST/3 PCT tax-rate rows, and 30,052 GST plus 30,052 PCT item-tax assignments populated and reconciled. Bounded purchase-order (2,810 rows), full purchase-header (6,419 rows), purchase-detail (113,532 imported of 113,564 source rows, with 32 explicit non-positive-quantity exceptions), posted purchase-return-header/detail (634/2,481 rows), and sale-return-header/detail (30,704/44,579 rows) slices are imported/reconciled; purchase-order/detail lines, sales, stock, and remaining historical documents remain explicitly deferred.
- The captured `Stock in Hand > Back Date` report now has a dedicated source-backed read model over imported `historical_stock_snapshots`, preserving the `StockReport` source row, as-of date, stock, purchase price, sale price, average price, recent purchase price, and pack-unit fields with tenant/date/godown isolation. The bounded API and browser evidence is recorded in `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md`; exact grouping, valuation, print output, and full source reconciliation remain open.
- The captured `GL Journal` alias now has a dedicated source-backed read model over imported `historical_gl_entries` from `dbo.VirtualGl`, unioned with explicitly labeled newly posted normalized journals. It preserves the imported document/type/account/alternate/invoice/user/remarks/debit/credit fields with tenant/branch/date/text scope and exposes a ten-column contract. The bounded API and browser evidence is recorded in `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`; exact account mapping, opening balances, reconciliation, grouping, and print output remain open.
- Purchase Return now retains and edits multiple source-batch allocations through the canonical document command, with active-batch selection, editable per-batch quantities, duplicate prevention, and exact line-total validation while preserving the legacy single Source batch ID field. The focused browser contract was added but not executed in the short verification slice; exact source-document reconciliation and PowerBuilder raster/UAT parity remain open.
- Posted purchase returns now carry and verify the canonical source purchase line ID, scope prior-return quantities to that exact line, and require each selected batch to originate from that source line. The return UI exposes the source-line field and populates it from the canonical Populate Purchase Return flow; focused Go and Svelte checks passed, while browser execution, database-backed replay, and exact PowerBuilder/UAT parity remain open.
- The captured Purchase Reports Withholding Tax Deduction leaf now uses a separate `historical_withholding_tax_entries` projection over the reviewed `dbo.PurPayment` `WHTax*` fields and a guarded `-wave withholding` importer. It does not reinterpret purchase-line advance tax; exact source import/reconciliation, grouping, certificate/supplier semantics, and print parity remain open in `docs/PHASE_Q_WITHHOLDING_TAX_EVIDENCE_2026-08-07.md`.
- The guarded historical `-wave payments` path now retains source-backed party payment allocations from `dbo.PurPayment`, `dbo.InstallmentReceiptDetail`, and direct SaleLedger/Purledger payment snapshots in `historical_party_payment_allocations`. Customer/Supplier Statement and the authenticated finance-ledger API union posted rows while retaining unresolved legacy identities. Exact source reconciliation, invoice allocation, adjustment/return allocation, canonical payment-entry UI, and print/UAT parity remain open in `docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.
- The guarded historical `-wave party-adjustments` path now retains `dbo.SaleReceivableAdj` debit/credit rows in `historical_party_ledger_adjustments` and exposes posted dated non-zero rows in customer statements and the finance-ledger API as `receivable-adjustment` entries. Exact source reconciliation, legacy posting/grouping, return allocation, canonical payment-entry UI, and print/UAT parity remain open in `docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.
- The guarded historical `-wave return-allocations` path now retains customer `SRAllocationHeader/Detail` and supplier `PRAllocationHeader/Detail` rows in `historical_party_return_allocations` and exposes bounded posted statement/finance-ledger rows as `return-allocation` entries. The stream is kept out of aging and canonical balance mutation until source amount, duplicate, and posting semantics are reconciled; print/UAT parity remains open in `docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.
- Customer and supplier party statements now retain the posted GL-journal gate for newly posted documents while also admitting posted historical `business_documents` with a non-empty `legacy_source_table`; this prevents imported party-ledger rows from disappearing solely because historical VirtualGl rows have not been promoted to canonical GL journals.
- Trial Balance now unions posted canonical GL lines with imported `historical_gl_entries`, resolving matching account codes to the tenant chart and labeling unmatched legacy accounts as Historical. Opening-balance and exact PowerBuilder account/group semantics remain open.
- Accounts Ledger now unions posted canonical journal lines with imported `historical_gl_entries`, using the tenant chart label when the legacy account code matches and an explicit Historical label otherwise. The cash-only ledger remains canonical-only pending reviewed VirtualGl account mapping; opening-balance and exact PowerBuilder account/group semantics remain open.

The captured Receivables Aging alias now uses retained SaleLedger DueDate
payloads on posted customer party-ledger entries, while Payables Aging uses
retained Purledger CreditDays plus the source purchase date. Both expose
bounded NOT DUE, 0-30, 31-60, 61-90, and 91+ buckets; missing or invalid
terms remain explicitly unaged. Exact legacy bucket/payment/date semantics
and print parity remain open in
docs/PHASE_Q_RECEIVABLES_AGING_EVIDENCE_2026-08-07.md.

New canonical credit-sale drafts now expose a validated SaleLedger-compatible
Due Date through the Svelte form, Go document snapshot/read response, and
Receivables Aging fallback; historical SaleLedger DueDate remains preferred.
The focused credit-sale browser contract passed, while source reconciliation,
payment allocation, exact legacy aging, print parity, and UAT remain open in
docs/PHASE_Q_RECEIVABLES_AGING_EVIDENCE_2026-08-07.md.

Receivables and Payables Aging now aggregate retained business-document
`balance_amount` rather than original party-ledger invoice totals, keeping
fully paid migrated documents out of open buckets. Individual payment/receipt
allocation, exact return and bucket semantics, print parity, and UAT remain
open in the same evidence.

Pack, loose, and opening purchase forms now carry bounded Credit Days through
the canonical draft/save/post/read contract and pricing snapshot used by
Payables Aging, with imported Purledger CreditDays retained as the historical
fallback. A focused receipt browser contract verifies the value in the
canonical command after the quick-search Lookup control was kept inside its
legacy grid cell. Exact source reconciliation, batch/due-date semantics,
payment allocation, print parity, and UAT remain open in
docs/PHASE_I_PURCHASE_CREDIT_TERMS_EVIDENCE_2026-08-07.md.

The captured Customer Wise Advance Tax and Supplier Wise Advance Income Tax
leaves now use explicit per-line pricing.taxes advance_tax rate/base/amount
evidence, with the customer leaf scoped to sales, the supplier leaf scoped to
purchase kinds, and a guarded first-line numeric business_documents legacy_payload.AdvanceTaxAmt fallback and
omission when no positive amount evidence exists. Aggregate line tax_amount is
not reinterpreted; exact source reconciliation, grouping, rounding, print
parity, and UAT remain open in
docs/PHASE_Q_ADVANCE_TAX_EVIDENCE_2026-08-07.md.

The high-volume canonical `dbo.Saledetail` slice now has a dedicated guarded
`migration/cmd/bulksalelines` COPY/set-based loader. It preserves the reviewed
`Saledetail:<SaleInvcode>:<RowID>` identity, source payload, pack/loose
quantity, pricing/tax/expiry/batch fields, scoped `SaleLedger` and item
dependencies, idempotent mappings, and auditable exceptions. The path has
focused compile/tests only; the approved source run, reconciliation, exact
line semantics, and PowerBuilder/UAT acceptance remain open in
`docs/PHASE_E_SALE_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

Migration reconciliation now reports raw open exception rows separately from
distinct unresolved source cases, and bases bookkeeping status on the distinct
case count while retaining raw rows for audit. The focused reconcile tests pass;
the live canonical report and closure/explanation of its remaining cases remain
open in `docs/PHASE_E_MIGRATION_BOOKKEEPING_EVIDENCE_2026-08-07.md`.

The reviewed high-volume `dbo.PurOrderDetail` slice now has a dedicated guarded
`migration/cmd/bulkorderlines` COPY/set-based loader with source identity,
order-header/item dependency checks, retained order-line payload, idempotent
mapping, and explicit exceptions. Only focused package tests have run; source
reconciliation, exact order semantics, report/print parity, and UAT remain open
in `docs/PHASE_E_PURCHASE_ORDER_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

The reviewed `dbo.SRdetail` and `dbo.PRdetail` return-line slices now share the
guarded `migration/cmd/bulkreturnlines` loader with fixed sale/purchase modes,
`SRLedger`/`PRLedger` dependency checks, reviewed import identities, retained
source payloads, idempotent upserts, and explicit mappings/exceptions. Focused
tests and vet pass without database execution; canonical import,
reconciliation, exact return effects, print parity, and UAT remain open in
`docs/PHASE_E_RETURN_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

Party finance-ledger responses now calculate a deterministic running
debit-minus-credit balance across canonical and retained historical payment,
adjustment, and return-allocation rows. Focused finance tests pass; exact
legacy opening and settlement ordering plus live reconciliation remain open in
`docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.

Sales and Purchase List tabs now expose a shared legacy-style `Filter / Retrieve`
control backed by the authenticated transaction-history `filter` query. The
focused Svelte check passed with 0 errors and 0 warnings; exact PowerBuilder
filter semantics and interactive/raster acceptance remain open. See
`docs/PHASE_I_TRANSACTION_HISTORY_FILTER_EVIDENCE_2026-08-07.md`.

Purchase `Print Purchase Labels` now submits item/batch/expiry/MRP/quantity
payloads to the edge print route, with a browser-preview fallback when no
adapter is configured. Software checks pass; physical label/printer acceptance
remains open in `docs/PHASE_U_HARDWARE_EVIDENCE.md`.

## Gated follow-on waves

- Capture and approve every remaining PowerBuilder screen/state/workflow at the canonical Windows resolution/DPI; the representative captures listed above are measured baselines, while the remaining catalog leaves still require their own approval evidence.
- Implement each business module and hardware/report integration against those approved baselines.
- Complete all remaining transaction projections/business modules (the sale projection and conflict review are implemented in this slice).
- Extend the canonical SQL Server inventory/import beyond the first 18 reviewed master/config tables with reviewed document, balance, stock, ledger, total, and sequence mappings; reconcile all remaining tables and business metrics before cutover.
- Complete load, RLS isolation, offline recovery, printer/barcode/cash-drawer/biometric/SMS/email, and pilot cutover acceptance gates.

No license-key, device-binding, hardware-fingerprint, or duplicate-instance restriction is present in this project.

Sales history rows now hydrate canonical documents when `documentId` is present,
restoring canonical lines, return source identity, pricing, and saved batch
allocations across sales, returns, quotations, and refused-sales; compatibility-
only rows remain explicitly summary-backed. The
focused Svelte and targeted API history-query checks passed, and the dedicated
browser contract was discovered but not executed in the short verification
slice. Live API/database behavior, exact legacy list/focus/raster parity, and
UAT remain open.

## Post-merge hardening pass - 2026-08-07 (evening)

A full verification sweep after the MDI-parity merge and purchase
inventory-picker parity commits found and fixed eight defects, with each fix
proven by its previously failing test: restored fail-closed duplicate-identity
and dependency guards in the bulk historical loader; extended the
`sale_return_source` trigger so source-bound purchase returns validate their
source purchase line at the database boundary (unlinked historical returns
remain an accepted migration state, enforced at the API boundary); fixed the
sale/return/quotation history read models so compatibility rows project an
empty `documentId` instead of NULL (previously a 503 decode failure); cleared
stale status-bar errors when a new menu command dispatches; made the legacy
menu bar dismiss on every item activation so blocked/denied commands no longer
dead-lock contextual navigation; gated the purchase inventory-picker spec on
the suite's `data-hydrated` convention; and aligned the stock-level fixture
cleanup with the shared best-effort pattern (tenant FKs are NO ACTION and
`stock_ledger` is immutable by design). Pending migrations `030`-`043` were
replayed onto the supervised local cluster. Fresh results: `go vet` 0 issues;
full Go suite 383/383 with the schema-owner DSN; Svelte check 0/0; production
build green; Playwright serial suite 121/121 with no retries. Full detail and
the unchanged external acceptance boundary are recorded in
`docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`.

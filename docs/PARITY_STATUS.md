# Parity status - 2026-08-06

## Current acceptance handoff - 2026-08-07

The fresh local verification and the remaining external/incomplete-data gates
are consolidated in [`ACCEPTANCE_EVIDENCE_2026-08-07.md`](ACCEPTANCE_EVIDENCE_2026-08-07.md).
The implementation/test gate is green; the document deliberately does not
claim complete canonical migration, exact every-report output, full raster
approval, physical hardware, full-volume performance, or cutover acceptance.

## Implemented in this wave

- The browser legacy shell now builds its menu tree from `parity/catalog/legacy-menu-tree-2026-08-05.json` (275 captured entries, 9 top-level menus), including recursive submenus, command IDs, keyboard shortcuts, and deterministic route metadata.
- Purchase workflows are available at `/app/purchase/pack`, `/return`, `/opening`, `/loose`, and `/order` with editable legacy-style grids, tenant/branch/counter-scoped events, idempotency, online posting, and offline queue fallback.
- Cash/credit sale rows now retain server-returned positive batch/expiry choices and can submit multiple distinct, quantity-aware batch allocations; Automatic FIFO remains the default while transfers, adjustments, and exact legacy allocation semantics remain open. Evidence: `docs/PHASE_H_SALES_FRONTEND_EVIDENCE_2026-08-06.md`.
- `Maintenance > Opening Stock` now routes through the immutable inventory-event contract with inbound direction; exact PowerBuilder opening-balance semantics, source reconciliation, and UAT remain open.
- Inventory maintenance forms now select active canonical items/godowns, validate batch identity, positive four-decimal quantity, and signed-adjustment values before emitting the event; exact legacy batch-selection UI remains open.
- Cash/credit sales, sale returns, quotations, and refused-sales documents use concrete transaction endpoints. Sale rows also project into the scoped inventory ledger.
- Quotations and refused-sales now use the idempotent canonical `/v1/documents/{kind}` lifecycle for tenant-scoped numbering, drafts, posting, revisions, and voids; they intentionally skip stock and finance projection.
- Cash and credit sale returns now use the same canonical lifecycle with a required posted source sale, source-batch restoration, immutable stock-in movement, and finance/party-ledger reversal. Canonical posted sale, sale-return, purchase, and purchase-return voids now append atomic compensating stock/GL/party-ledger projections with dependency blocking and replay idempotency; exact PowerBuilder void parity remains open in Phase T.
- Customer, supplier, item, manufacturer, category/template master forms persist through `/v1/master/*`; the Users workflow lists, creates, selects, and updates tenant operators with password hashing and validated group, branch, and counter assignments through `/v1/operators`.
- The remaining captured Basic Data master leaves (customer/supplier categories, promotions/sectors, item class/category/generic data, price/tax/PCT settings, lock reasons, segments, types, templates, and related lists) retain their own master kind and route-specific title. The 16 auxiliary leaves now use tenant-scoped source-shaped CRUD with confirmed delete through `/v1/master/{kind}`; normalized source joins and exact legacy rules remain open.
- Master List rows are now selectable: the captured Detail/List workflow reloads legacy payload fields into the Detail form, and Save updates the tenant record through `PATCH /v1/master/{kind}/{id}` instead of always creating a duplicate.
- Item Detail now exposes the legacy supplier sub-grid (Priority, Rate, Disc%, Qty, Bonus, Days) and replaces the tenant-scoped links through `PUT /v1/master/item/{id}/suppliers`; a focused browser regression covers edit and persistence.
- Manage → Groups now lists, creates, and updates tenant-scoped roles and their validated permission sets through `/v1/roles`, with administrator authorization, RLS-backed storage, and audit events.
- Every captured report leaf reaches the report argument/retrieve/export surface. Four primary report projections are implemented; other catalogued report kinds use the scoped immutable-event projection until their exact legacy columns are captured.
- Sale detail and Sales Return detail now have explicit source-backed 11-field line contracts over canonical lines plus de-duplicated compatibility rows; exact legacy grouping/calculations remain open. Evidence: `docs/PHASE_N_SALES_LINE_DETAIL_EVIDENCE_2026-08-07.md`.
- Customer Sales now includes explicit six-field projections for Customer Wise Summary, Customer Wise Net Sales and Volume, and Customer Category Wise Net Sales (including the two captured category aliases); the category projection preserves canonical/compatibility scope and retained customer payload categories, while exact PowerBuilder joins, return/net calculations, and print output remain open. Evidence: `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- Customer Wise Category Net Sales now adds an explicit customer/category grouping over the same bounded canonical/compatibility rows; exact PowerBuilder customer/category joins, return/net calculations, and print output remain open. Evidence: `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- Customer Category Wise Sales Detail Report now uses the existing source-backed 11-field sale line-detail contract rather than the generic event ledger; exact category grouping and format/print output remain open. Evidence: `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.
- Purchase detail and Purchase Return detail now have explicit source-backed 12-field line contracts over canonical purchase lines plus expanded, de-duplicated receiving/return compatibility rows; exact purchase grouping, tax/profit/order calculations, and print output remain open. Evidence: `docs/PHASE_O_PURCHASE_LINE_DETAIL_EVIDENCE_2026-08-07.md`.
- Phase O purchase summary leaves now have explicit six-field document, day, month, item, or supplier aggregates over the canonical purchase/ledger read model and de-duplicated compatibility events; exact supplier/category/tax/return/profit/graph calculations and print output remain open. Evidence: `docs/PHASE_O_PURCHASE_SUMMARY_EVIDENCE_2026-08-07.md`.
- `Stock Management Report` now uses an explicit eight-field normalized stock-balance projection with posted-ledger gating and item-payload reorder/optimum/minimum thresholds, without applying an unverified legacy alert predicate. Exact alert/status, valuation, grouping, source reconciliation, and print parity remain open. Evidence: `docs/PHASE_P_STOCK_MANAGEMENT_EVIDENCE_2026-08-07.md`.
- Quotation Detail, Quotation Summary, and Refused Sales Detail now read canonical posted no-stock documents/lines first, de-duplicate matching compatibility events, and group quotation summaries once per document. Exact PowerBuilder columns/calculations, print output, and golden replay remain open. Evidence: `docs/PHASE_Q_NO_STOCK_DOCUMENT_EVIDENCE_2026-08-07.md`.
- `Header Wise Transaction Summary` now reads canonical posted transaction headers across sales, returns, quotations, refusals, purchases, purchase returns, and purchase orders, with de-duplicated compatibility events and one authoritative total per header. Exact PowerBuilder labels/calculations, opening-balance treatment, print output, and golden replay remain open. Evidence: `docs/PHASE_Q_HEADER_TRANSACTION_EVIDENCE_2026-08-07.md`.
- The eight captured Reprinting leaves now read canonical posted sale/purchase line or invoice-summary projections with tenant/branch/date/text scope and explicit compatibility fallback. Exact PowerBuilder selection, summary sections, format calculations, print output, and golden replay remain open. Evidence: `docs/PHASE_Q_REPRINT_EVIDENCE_2026-08-07.md`.
- `Item Reports > Deleted Sale Items Log` now reads a retained `dbo.DeletedSaleItem` projection through a guarded historical loader with tenant/branch/date/text scope and a six-field Svelte contract. Exact PowerBuilder columns, deletion order, calculations, print output, and source reconciliation remain open. Evidence: `docs/PHASE_Q_DELETED_SALE_ITEMS_EVIDENCE_2026-08-07.md`.
- Stock Adjustments Detail now unions retained imported AdjHeader/AdjDetail rows with posted normalized signed stock-ledger adjustments while preserving tenant/branch/date/text scope; exact legacy grouping, calculations, source reconciliation, and print output remain open. Evidence: `docs/PHASE_Q_STOCK_ADJUSTMENT_EVIDENCE_2026-08-07.md`.
- Reorder Level, Optimum Level, Minimum Level, and Reorder/Optimum Level now use a distinct normalized stock-balance projection with item-payload thresholds, maintenance-key fallbacks, posted-ledger gating, and tenant/branch/date/godown/batch scope. Exact comparison, grouping, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md`.
- Item Stock Register Summary now uses a distinct normalized posted stock-ledger aggregation by item, godown, and calendar day with signed net quantity and net value. Opening-balance, valuation, grouping, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md`.
- Stock and Sales now uses normalized current balances joined to canonical posted sale allocations for the requested period and exposes On Hand plus Sales Qty; exact period/as-of, return, valuation, grouping, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md`.
- The two captured narcotics movement reports now filter posted normalized stock-ledger rows by the Item master Narcotics payload flag, and the generic-type narcotics report groups the captured GenericName/GenericCode payload by day, godown, and item. Exact legacy flag semantics, generic grouping, return/opening treatment, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_NARCOTICS_STOCK_EVIDENCE_2026-08-07.md`.
- `Expiry Report(Class Wise)` now uses a distinct typed-expiry/class projection over normalized balances, preserving tenant/branch/date/text/godown/batch scope. Exact class-code joins, date semantics, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_EXPIRY_CLASS_EVIDENCE_2026-08-07.md`.
- The captured Stock-in-Hand Manufacturer, Manufacturer Format2, Category, and Class leaves now use explicit Item-payload classification projections over normalized balances with posted-ledger gating. Exact group joins, valuation, supplier association, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_STOCK_CLASSIFICATION_EVIDENCE_2026-08-07.md`.
- `Daily Stock IN/OUT` and `Stock IN/OUT(Date Wise)` now use an explicit posted stock-ledger aggregate by calendar day, direction, godown, and item with signed quantity and net value. Opening balances, exact date-wise grouping, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_STOCK_MOVEMENT_SUMMARY_EVIDENCE_2026-08-07.md`.
- `Stock In hand > Supplier Manufacturer Association` now uses an explicit normalized stock-balance projection joining the captured Item Manufacturer payload and tenant-scoped `item_suppliers` supplier names. Exact priority/association joins, valuation, source reconciliation, and print output remain open. Evidence: `docs/PHASE_P_STOCK_SUPPLIER_MANUFACTURER_EVIDENCE_2026-08-07.md`.
- Report workflows now include the legacy retrieval-arguments dialog (areas, date range, cash/credit selection), validated server-backed format selection, print preview with a legacy-style toolbar/ruler/letterhead and loaded-row paging, CSV export, browser Save-as-PDF, Excel-compatible workbook export, and a captured daily-sales-detail loading state. Exact PowerBuilder format calculations and golden output remain open; see `docs/PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md`.
- Detail/List tabs are live on purchase, sales, and master-data surfaces; sales and purchase list views query persisted tenant/branch-scoped transaction history through `/v1/transactions/{kind}` rather than rendering draft placeholders.
- Sales and purchase List rows and toolbar Previous/Next actions now load persisted documents back into the Detail form, preserving the legacy navigation workflow instead of only changing a status message.
- Manage → Cashier Activity Window now reads the scoped shift ledger through `GET /v1/shifts` and renders operator, open/close, status, and cash totals with a live refresh action.
- The non-legacy workspace dashboard now reads sales history, shifts, and branches from the authenticated API; hardcoded demo metrics/activity were removed and empty-state labels are explicit.
- Dashboard navigation/export/notifications/context actions and the report toolbar (save layout, sort, paging, refresh, print, CSV export, browser PDF, Excel-compatible workbook export, and format settings) now execute concrete client actions.
- The main-window Minimize/Restore controls now change the rendered shell state; Close still returns to the application entry point.
- Captured menu shortcuts are now actionable in the live shell; the recorded Ctrl+Alt+M Session Monitor shortcut (and Ctrl+X Exit) route through the same command metadata as menu clicks.
- Sales and purchase windows now dispatch captured contextual File/Item verbs for New, Save/Post/Save And Post, Print, Previous/Next, and New Item into their live handlers. Other captured leaves now navigate to a deterministic contextual workbench carrying the legacy path and command id instead of leaving the click inert; their underlying legacy business behavior remains an open parity gate. The Save And Post accelerator has a focused browser regression.
- Purchase Order now emits the correct `purchase_order` event aggregate when saved, preserving the legacy workflow boundary and avoiding accidental receiving projection.
- Purchase pack/loose/opening/return/order forms now bind their selected supplier, item, godown, batch, and source-batch identities to the canonical document command contract; Save creates a versioned draft and Post/Save And Post uses the existing draft version, while incomplete compatibility entries retain the legacy event fallback.
- The captured purchase Lookup action and Ctrl+B client-convenience batch
  helper are live (`AUTO-YYYYMMDD-NNN`); this identifier is explicitly not
  claimed as legacy-format parity. Focused coverage covers canonical
  rejection, draft/post revision state, purchase orders, returns, free-text
  fail-closed validation, and the helper.
- Purchase List now renders canonical `/v1/transactions/pack-purchase` rows and restores the selected invoice into Detail; the focused suite verifies the six purchase workflows end to end.
- Purchase `Populate Items` now resolves entered quick-search or unresolved item-name values through active canonical item lookup, hydrates UUID/legacy identity, and reuses the existing batch-refresh path. Exact PowerBuilder source-selection, template, pending-due, price/tax side-effects, and raster acceptance remain open. Evidence: `docs/PHASE_I_PURCHASE_ITEM_POPULATION_EVIDENCE_2026-08-07.md`.
- Purchase `Populate From Sale Template` now lists active tenant-scoped templates and loads supported line payloads into a new canonical draft before item resolution; unsupported payloads remain explicitly fail-closed. Exact PowerBuilder template, pending-due, and source-selection semantics remain open in the same evidence.
- High-frequency contextual purchase/sales commands now have live handlers:
  list/history navigation, client-convenience batch generation, item sorting,
  row delete/restore, item/customer/supplier information routes,
  sale-slip/purchase-label output with branch-edge fallback, change-user/exit,
  and local offline queue synchronization.
- Cash/credit sales also apply captured item GST/discount rates and generated batch identifiers to populated lines, while Attach Document(s), Show Document Gallery, and parent-server import commands update the live draft/branch queue.
- Preferences persist by tenant/category through `/v1/preferences` and retain the captured tab/grid structure.
- Preferences now load and save tab-specific captured labels/defaults for all 17 categories; Cancel restores the last loaded values and each ellipsis editor focuses its matching field.
- Schedule and Email preference tabs now switch from the untouched screenshot baseline to their captured form layouts on interaction, with editable controls wired to the same tenant/category persistence path.
- Maintenance and manage leaves have concrete tenant-audited workflow surfaces. Inventory adjustments, password changes, cashier shift open/close, integrity checks, and backup requests have API paths; backup artifact generation remains deployment-policy-owned. Captured backup dialogs and the change-user confirmation now have live Svelte controls over an exact untouched-state raster.
- Maintenance/manage form values now persist through `GET/POST /v1/maintenance/{kind}` in tenant-scoped preference storage, reload on direct navigation, and remain represented in the audit ledger instead of being acknowledgement-only actions.
- Generic catalog leaves now load/search the last saved tenant-scoped record; master-data Cancel restores the selected record/new state and maintenance/manage Cancel clears the active form.
- Maintenance and Manage leaves that previously collapsed to reference/notes now expose route-specific persisted fields for pricing, discounts, reorder quantities, supplier links, batch locks, item updates, import validation, initialization scope, group settings, interfaces, and job schedules.
- Report leaves now select the matching immutable aggregate (sale, sale return, purchase, purchase return, quotation, order, inventory, or refused sale), reject inverted date ranges, and provide sortable, paged result grids. Sale and purchase item lookup lists use synchronized item master records when available; when normalized `master_items` is empty, the item list exposes the tenant-scoped Phase E `master_records(kind=item)` catalog as an explicit read-only fallback instead of silently using the 32-row demo list.
- Transaction and workflow toolbar glyphs are normalized through CSS Unicode fallbacks so the live interactive surface does not expose the prior mojibake symbols; daily sales detail also restores customer/item/quantity values from the source event payload.
- The authored web source has no known double-encoded UTF-8 markers; the old text-node MutationObserver band-aid is no longer part of the runtime.
- Users list selection now reopens the captured Detail form; Save updates name, active state, group, password (when supplied), and branch/counter assignment instead of creating a duplicate operator.
- Groups permission checkboxes now reload with the selected role and persist the supported permission set in the same legacy Save workflow.
- Effective role permissions are loaded into the HTTP-only session context and enforced for master data, reports, transaction history/posting, shifts, maintenance actions, operator management, and synchronization conflict review; tenant administrators retain the legacy full-access path.
- Multi-branch/counter operators can select and persist an operational context through `/context` and `POST /v1/session/context`.
- The live sales lookup normalizes legacy master payload casing and price-tier names before filling the transaction grid; the focused canonical lifecycle test still passes after this projection change.

## Verification evidence

- `pnpm --filter @abuzar/web check` - 0 errors, 0 warnings.
- `pnpm exec playwright test --workers=1` - 20 browser smoke tests passed, including persisted sales-history List rendering, recursive report-menu navigation, keyboard shortcuts, route reachability for transaction, master, report, preferences, maintenance, and manage surfaces, direct-SSR and interactive Schedule/Email/maintenance regressions, operator update, and Groups permission persistence.
- A post-restart targeted run of the legacy-shell shortcut, Users update, and Groups permission tests passed (`3 passed`); the exact `127.0.0.1` webServer URL prevents Playwright from starting a competing Vite process.
- `go test ./migration/...` - passed after adding fail-closed validation for empty tenant/branch injections and mapped identifiers.
- The Users browser regression passes with a mocked tenant-scoped PATCH request and confirms the selected operator is edited through the Detail form.
- The focused follow-on browser checks for route-specific Maintenance fields and interactive Schedule/Email preference layouts pass (`2 passed`); the API package regression suite also passes after expanding captured Basic Data master kinds.
- `pnpm --filter @abuzar/web build` - production static build passed.
- `go test ./services/api/... ./services/edge/... ./migration/...` - passed.
- A catalog walk exercised all 221 captured leaf routes generated from the 275-entry menu catalog; every route rendered a Svelte `main` surface with no navigation failure.
- The same catalog walk also passes direct SSR/deep-link requests (`220` unique generated URLs; the remaining leaf is the intentional File Exit route), including generic module pages after a cold start; generic module slugs and legacy path values are normalized before title rendering so direct `/app/module/*`, report, master, maintenance, and manage requests cannot dereference an undefined value.
- Local API `/v1/health`, edge `/v1/health`, and all sampled web deep links returned HTTP 200 after restart.
- The Windows Tauri production build completed and produced both the NSIS setup executable and MSI under `apps/desktop/src-tauri/target/release/bundle`.
- `ops/local/start-local.ps1`, `stop-local.ps1`, and `status-local.ps1` provide a repeatable reboot-safe local stack lifecycle with detached supervisors, restart-on-crash behavior, bounded logs, and health probes without touching the legacy installation.
- Local PostgreSQL migrations applied through `008_role_permissions.sql`; login, context selection, a scoped sale POST, report retrieval, integrity checks, backup-audit request, preference reads, and role permission storage were exercised.
- Live operator regression coverage exercised login, group listing, operator listing, operator creation, tenant/branch/counter assignment, and cleanup; the PostgreSQL array-to-JSON boundary was corrected so operator lists no longer return a 503.
- Authenticated disposable sale and receiving events were posted through the API and retrieved through the new history endpoint, including item/amount projections and cleanup verification.
- Current local health: API and edge both report `status=ok` and `database=ok` on ports 8080/8091; web is served at `http://127.0.0.1:5173/login`.
- The maximized shell screenshot gate passes exactly (`differentPixels=0` at 1936x1048).
- Exact captured-default comparisons now pass (`differentPixels=0`) for Pack Purchase, Cash Sale, Customer, Supplier, Item, User, Preferences, Daily Sales Detail loading, report retrieval arguments, report format selection, Backup Database, Backup Device, Backup Information, Change User, and the Database Integrity Monitor. The reports are recorded in `tmp/*-comparison.json` and can be regenerated with `parity/tools/compare-png.ps1`.

## Addendum — independent live audit, 2026-08-06

A full side-by-side runtime audit (legacy exe + new stack both running, screens walked visually, menus enumerated per window, databases counted, code reviewed) found that the wave claims above describe *surface* coverage, not functional parity. Authoritative documents:

- `docs/GAP_ANALYSIS_2026-08-06.md` — 15 verified gap areas (G1–G15) with evidence in `tmp/gap-audit/`.
- `docs/PARITY_FIX_PLAN_A-Z.md` — 26-phase plan to reach 100% visual + functional parity.

Corrections to earlier claims:

- **Mojibake was not fixed** by the CSS "Unicode fallbacks" wave: 13 rules in `apps/web/src/lib/styles.css` used double-escaped `content: "\\XXXX"` and rendered literal `\2190`/`\25A6 Detail` text on live surfaces. Actually fixed and browser-verified on 2026-08-06.
- **`Sales > CashSale` routed to the generic `/app/module/cashsale` stub**, not the real sales surface (legacy caption `Cash&Sale` cleans to `CashSale`, which the route map missed). Fixed in `lib/legacy-menu.ts` and browser-verified on 2026-08-06.
- The 275-entry menu catalog is **incomplete**: with a document window open the legacy shell exposes 325+ entries (contextual Item menu + ~35 File verbs + Window refresh variants). See `tmp/gap-audit/live-menu-tree-pack-purchase.json`.
- The 221-route catalog walk proves routes render, not that they are functional: exact legacy tax, discount, stock, batch/expiry, GL, ledger, rights, print, and hardware behavior remains an acceptance boundary across the 763-table source inventory.
- The local disposable database contains 61,202 `master_records` and 83,447 `legacy_id_mappings` from a reviewed 18-table sandbox/reference import. This is not the canonical SQL Server migration; historical reconciliation, exact source semantics, and operator acceptance remain open.
- The local stack supervisor now detaches web/API/edge processes and restarts failed children; the Playwright webServer probe is pinned to `127.0.0.1` so test teardown does not compete with the supervised web process. A 24-hour soak is still open.
- All 17 captured Preferences tabs are now individually mapped to their own canonical raster baseline and each tab comparison passes at `1536x972`.
- These exact raster gates cover the untouched canonical state only. Controls remain mounted and switch to the live semantic Svelte surface on pointer, focus, or keyboard input; functional tests, not the raster, validate posting and persistence.

## Acceptance still open

## Pricing and transaction-core evidence - 2026-08-06

- The deterministic pricing engine is now reachable through the authenticated `POST /v1/transactions/preview` contract. It validates exact decimal money/percentage/whole-quantity inputs, applies price-tier selection, supplier schemes, item/customer/document discounts, Misc, and GST/PCT/advance-tax ordering without floating-point arithmetic.
- Sale detail fields now debounce a server pricing preview while the legacy-style form is edited. The displayed total and discount values use the preview when online, while the existing local line total remains the offline fallback; the posted event carries the auditable pricing result.
- Focused evidence: `go test ./services/api/internal/httpapi ./services/api/internal/pricing`, `go test ./services/api/... ./services/edge/... ./migration/...`, `pnpm --filter @abuzar/web check`, and three targeted Playwright smoke tests all pass. A live authenticated preview returned `223.02` for a two-line `200.00` basket with a 5% document discount, `1.00` flat discount, and 18% GST.
- Sale projection replays a supplied pricing request inside the posting transaction, rejects forged totals, validates `draft`/`posted`/`voided` status values, detects document-number collisions, and rejects non-positive inventory quantities. This closes an integrity gap but is not yet a full stock/GL/lifecycle implementation.
- `GET /v1/inventory/balance` now requires tenant/branch/godown scope and reads `stock_balances`; selecting an item in the live sale grid refreshes its Stock cell from that normalized cache. A labeled `inventory_movements` fallback is used only when the requested normalized scope has no rows.

The pricing preview is a validated calculation surface, not proof that all legacy tax tables, batch/expiry rules, stock availability, GL/ledger postings, or every document lifecycle are complete. Those remain acceptance work below.

## Canonical source-data wave - 2026-08-06

The locally available canonical SQL Server database was inspected read-only and
the reviewed enterprise/config and core-master maps were imported into a new,
isolated `LEGACY_CANONICAL` tenant. The importer now has an explicit
`-allow-canonical` guard and requires a dedicated target scope override; the
default sandbox protection remains fail-closed. The 11-table enterprise wave
and 7-table core wave reconciled with zero exceptions and exact source/target
counts. Evidence and reports are recorded in
`migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md`.

The reviewed operator/rights map is also imported and reconciled for the same
tenant (13 mappings, 925 rows, 0 exceptions, 13/13 reviewed metrics matched).
It preserves legacy group IDs, rights, and allow scopes while excluding source password values; see the
security report pair under `parity/catalog/canonical-first-tenant-security-*`.

The normalized canonical master targets are populated for that tenant, and the
first bounded document slice (2,810 purchase-order headers) reconciles exactly.
The complete purchase-header inventory now reconciles 6,419 exact rows, the bounded posted
purchase-return header slice adds 634, and its 2,481 detail lines now reconcile
with exact counts and return totals. The sale-return header/detail slices add
30,704/44,579 exact rows and reconcile their total and quantity metrics.
The canonical pricing tier map adds 30,052 exact `PricePolicyDetail` rows and
the full legacy price total. The first tax configuration slice adds the 7 GST
and 3 PCT rows with exact rate totals.
The item-tax assignment slice adds 30,052 GST and 30,052 PCT item references;
both filtered table counts and focused assignment metrics reconcile exactly,
with zero loader exceptions. Evidence is recorded in
`parity/catalog/canonical-first-tenant-item-tax-import.json` and
`parity/catalog/canonical-first-tenant-item-tax-reconciliation.json`.
The purchase-detail slice adds 113,532 of 113,564 source lines; all 32
non-positive source quantities are retained as auditable migration exceptions,
while quantity and line-total metrics reconcile within their declared
tolerances. Evidence is recorded in
`parity/catalog/canonical-first-tenant-purchase-lines-import.json` and
`parity/catalog/canonical-first-tenant-purchase-lines-reconciliation.json`.
Purchase-order/detail lines, sales, and the remaining
historical documents/stock/ledger waves are still open and are not represented
as complete workflows yet.

This materially proves the canonical master-data slice plus bounded purchase
and return document/line waves. It does not close the remaining legacy tables,
purchase-order/detail and sales history, ledgers/stock, report/hardware
acceptance, or exact screen/workflow parity gates.

The legacy catalog is fully reachable, but true 100% functional and pixel parity is not yet proven. The captured interior states cover representative transaction, master, preference, report-loading, maintenance-dialog, and change-user workflows; remaining leaves still need their own legacy screen/workflow capture, exact field rules, report columns, printing/hardware behavior, and screenshot/keyboard acceptance before they can be marked parity-complete. The reports menu has 186 entries and 151 non-blank leaf reports; the remaining leaves still require exact legacy report-column evidence rather than a generic projection. Printer, barcode, cash-drawer, biometric, SMS/email, and complete historical SQL Server migration acceptance are also still open.

- Open Cash Sale Return and Open Credit Sale Return now have distinct canonical document kinds and source-free stock/finance projections; see `docs/PHASE_H_OPEN_SALE_RETURN_EVIDENCE_2026-08-06.md`.
- Sales Return report/history leaves now read posted source-bound and open return lines from the canonical `business_documents` read model, while retaining deduplicated `sale_return` compatibility events during migration; see the Phase N follow-up evidence.
- Invoice-summary sales and sales-return leaves now group each document once, summing line quantity while retaining the canonical document amount; multi-line invoices no longer repeat the full total per line. Exact legacy tax/profit/format columns remain open.
- The direct Purchase Return report route now uses the canonical purchase-return read model with posted document/line authority and compatibility-event de-duplication; the focused PostgreSQL route test passes.
- Browser business dates now use the local calendar date for dashboard/report defaults and transaction filters. Canonical sale and purchase timestamps are encoded at noon UTC, preventing the previous local-midnight-to-UTC conversion from shifting Pakistan-local transactions into the prior report day. The change is covered by `pnpm --filter @abuzar/web check` and the focused sale-return browser suite.

## Phase S/T maintenance, manage, and posted-void slice - 2026-08-07

- Maintenance operations now create tenant/branch-scoped `maintenance_operation`
  audit records and return an operation/job identifier plus an explicit status.
  Integrity output is application-scope row checking; physical PostgreSQL
  integrity, backup, restore, import/export artifacts, and external sends remain
  `not_configured` unless a deployment adapter is supplied.
- Backup/restore requests never claim that a physical artifact was created or
  restored. Import requests reject server paths and are not opened by the API.
  Stock adjustment/opening forms send canonical item/godown/batch identity to
  the existing guarded inventory projection; invalid quantity, expiry, batch,
  tenant, and branch checks remain server authoritative.
- Session Monitor now reads active HTTP sessions through a permission-gated,
  current-branch query and does not expose token material. Existing real
  password, user/group, and cashier shift APIs remain the execution paths;
  template values are configuration only and SMS/email delivery still requires
  the edge adapters in Phase U.
- Cancel restores the last loaded/saved workflow state instead of presenting a
  false reset, and direct reload includes the last operation status.
- Posted canonical document voids use the append-only reversal contract in
  `db/migrations/028_business_document_void_reversals.sql`. The API reverses
  stock, GL, and party-ledger projections in one transaction, blocks posted
  dependents, and is idempotent on replay. Cash-sale, sale-return,
  purchase-return, and dependency-blocking integration coverage passes; exact
  legacy void-dialog semantics, historical reversal mapping, and UAT remain
  open. See `docs/PHASE_T_VOID_REVERSAL_EVIDENCE_2026-08-07.md`.

Evidence: [`tmp/phase-s-t-maintenance-evidence-2026-08-06.md`](../tmp/phase-s-t-maintenance-evidence-2026-08-06.md).

## Daily Sale Detail column-contract follow-up - 2026-08-07

`daily-sales-detail` now has a dedicated canonical/compatibility projection
for the captured Alias, Item Description, Sale Price, Qty, Disc(%), Discount
Value, Item Disc, SalesTax Value, Amount, Expiry Date, and Batch Number columns.
Retained historical line payload values are preferred for imported legacy
figures, while typed pricing and stock-allocation snapshots serve newly posted
documents. The focused PostgreSQL/Go and Playwright evidence is recorded in
[`docs/PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md`](PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md).
Exact ten-format calculations, migrated golden-output comparison, and the
remaining report leaves are still open.

## Phase W performance and scale hardening — evidence pending full volume

The measured bounded disposable probes, idempotent read indexes, timeout and
request-observability controls, cold-start result, and unrun eight-hour soak
setup are recorded in
[`PHASE_W_PERFORMANCE_EVIDENCE_2026-08-07.md`](PHASE_W_PERFORMANCE_EVIDENCE_2026-08-07.md).
The fixture loaded 25,000 stock rows and 10,000 GL journals rather than the
3.2M/1M targets; full-volume p95, document-post `<1s`, and soak acceptance
remain open.

The 2026-08-07 opt-in full-volume attempt was blocked by an abnormal
PostgreSQL backend termination during the synthetic stock seed; see the
blocker and recovery record in the Phase W evidence document. No full-volume,
post-latency, or eight-hour-soak gate is marked green.

## Purchase history population and migration ambiguity follow-up — 2026-08-07

Canonical purchase history now carries document identity, and the scoped
document-detail read hydrates the contextual Populate Purchase Invoice and
Populate Purchase Return Invoice draft flows with persisted lines,
supplier/godown/source references, batch/expiry, discount, and tax metadata.
The focused Go/PostgreSQL and Playwright evidence is recorded in
`docs/PHASE_I_PURCHASE_HISTORY_POPULATION_EVIDENCE_2026-08-07.md`.

The local migration bookkeeping was also rechecked with tenant scope: the
aggregate has 501,024 resolved, 404 ignored, and 32 open
`Purdetail/non_positive_quantity` rows, all in the isolated canonical tenant;
the sandbox tenant has no open `migration_exceptions`. The sandbox retains 16
open `tax_rule_has_no_numeric_rate` rows in `migration_ambiguous_records`, while
the canonical tenant has none. The reconciliation command now reports both
tables as `bookkeeping.status=review_required`, and the finance/stock metric
counts both classes. The positive canonical line contract and numeric tax-rule
semantics therefore remain explicit acceptance boundaries.

## Window/MDI navigation follow-up - 2026-08-07

The Window registry now uses validated tab-scoped persistence and internal
client navigation. A focused browser workflow proves Main Window -> Cash Sale,
Window-menu activation back to Main Window, and hard-reload restoration of the
Cash Sale tab. Evidence: [`docs/PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md`](PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md).

This does not close exact PowerBuilder MDI acceptance: cascade/tile/layer
geometry, close/minimize/restore behavior, focus/keyboard traversal, and the
remaining contextual raster comparisons still require an operator walkthrough.

## Base and contextual Change User follow-up - 2026-08-07

The File > Change User command now opens the captured Yes/No confirmation in
the shared `LegacyMenuBar` used by the main shell and contextual child windows.
`No` retains the current window and `Yes` navigates to the change-user login
route. The focused browser evidence covers both the main shell and a report
child window and is recorded in
[`docs/PHASE_C_CHANGE_USER_EVIDENCE_2026-08-07.md`](PHASE_C_CHANGE_USER_EVIDENCE_2026-08-07.md).
Confirmed navigation clears the tab-scoped persisted MDI registry at the
session boundary and requests server-session invalidation before login.
Full operator/session acceptance and contextual raster/focus review remain
separate boundaries.

## Shared child-window chrome follow-up - 2026-08-07

The captured File/Window menu bar and validated MDI registration now cover the
maintenance/manage workflow surface, Preferences, and direct generic module
routes in addition to the existing transaction, master, and report children.
The focused browser checks verify direct route reachability and File/Window
visibility, and the latest serial suite passed 77/77 test cases with no retry. Generic module routes
remain deterministic compatibility workbenches where the true legacy workflow
is still an acceptance gap; this slice does not claim full functional or pixel
parity for those leaves.

## Canonical item-maintenance follow-up - 2026-08-07

`change-items-price`, `change-item-discount`, `update-item-basic-data`, and
`change-item-reorder-qty` now lock and update the tenant-scoped canonical
`master_items` row by code or legacy identifier, retain the captured legacy
payload keys, and write the operation/audit record in the same transaction.
The API accepts the JSON-number payload emitted by Svelte numeric inputs, and
the browser renderer permits decimal values such as `12.75`. Focused Go and
Playwright evidence is recorded in
[`docs/PHASE_S_ITEM_MAINTENANCE_EVIDENCE_2026-08-07.md`](PHASE_S_ITEM_MAINTENANCE_EVIDENCE_2026-08-07.md).

This closes only the four bounded mutation contracts. Effective dates are
validated/audited but are not a claim of scheduled price history; remaining
maintenance leaves, exact PowerBuilder messages, print/focus behavior, and
operator acceptance remain open.

## Auxiliary Basic Data CRUD follow-up - 2026-08-07

The 16 captured auxiliary Basic Data leaves that were previously read-only now
load, create, update, and confirmed-delete tenant-scoped records through the
existing `master_records` API. Their forms retain source-informed keys such as
`PricePolicyCode`, `SalesTaxScheduleCode`, `PCTCode`, `ICode`, and
`SalePromotionCode` inside the JSON payload, while `code`, `name`, and
`active` provide the shared list/search contract. Migration
`029_auxiliary_master_kinds.sql` extends the database kind constraint, and the
focused API integration checks the constraint plus create/list/update/delete
isolation for `price-policy`.

Evidence: [`docs/PHASE_F_AUXILIARY_MASTER_EVIDENCE_2026-08-07.md`](PHASE_F_AUXILIARY_MASTER_EVIDENCE_2026-08-07.md).

This is a real tenant-scoped operational CRUD path, not a claim of full source
data migration. Legacy validation messages, dependent lookups, pricing/tax/
promotion calculation semantics, populated-data reconciliation, print/focus
behavior, and per-leaf raster acceptance remain open.

## Pricing workflow follow-up - 2026-08-07

The sales `SalePrice:#` selector now exposes all ten captured levels, preserves
the item payload's `SalePrice1`-`SalePrice10` values, reprices selected grid
rows, and sends the actual tier array to the authenticated exact-decimal
preview. Canonical purchase documents inherit a valid tenant-scoped
ItemSuppliers discount/bonus scheme when the command does not provide an
explicit line override. Focused Go and browser evidence is recorded in
[`docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md`](PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md).

Customer/group `GroupAllowedPrice` assignment, PricePolicyDetail date
semantics, complete policy promotion, ItemSuppliers day semantics, and the
required historical invoice golden replay remain open acceptance gates.

## Maintenance batch-lock follow-up - 2026-08-07

`Lock Item Batches` is now a real tenant- and current-branch-scoped mutation
against `stock_batches.locked`, with validation, immutable operation auditing,
other-branch isolation, and a focused browser submission test. Exact
PowerBuilder selection/order/message parity remains open pending a replayable
legacy source dataset; the current SQL Server read-only probe is blocked by an
integrated-authentication untrusted-domain login failure.

## Manage and Maintenance group-scope follow-up - 2026-08-07

`Manage -> Group Wise Header Setting`, `Manage -> Group Allowed Price
Setting`, `Maintenance -> Group Wise Godown Setting`, `Manage -> Group Wise
Cash Account Setting`, and `Manage -> Group Wise Supplier Category` now load
the selected tenant role's imported rows, preserve their exact composite
identifiers, and update only the mapped `scope_kind` through the existing
audited role-rights endpoint. Explicit canonical-UUID godown scopes are also
enforced at stock balance/availability reads, canonical document and
inventory-event ingress, synchronization replay, godown lookup/detail, and
stored-document voids. The focused browser and Go regressions are green. The
imported `GroupAllowedGodown` composite-key mapping and the source-backed
mapping from price/module identifiers to document/price-policy behavior remain
unapproved, so the pricing engine does not claim price/header enforcement from
these editors alone.

## StockReport Back Date follow-up - 2026-08-07

The captured `Stock in Hand > Back Date` leaf now reads the imported
`historical_stock_snapshots` projection from `dbo.StockReport`, preserving the
source row, as-of date, stock, purchase price, sale price, average price, recent
purchase price, and pack units. The API and browser contract is tenant/date/
godown scoped and is covered by PostgreSQL integration and focused Playwright
checks. This is bounded source-backed coverage; exact PowerBuilder grouping,
valuation, print output, full StockReport rerun, and the remaining stock-report
families remain acceptance work. See
`docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md`.

## VirtualGl GL Journal follow-up - 2026-08-07

The captured `GL Journal` alias now reads imported `historical_gl_entries`
from `dbo.VirtualGl` alongside explicitly labeled newly posted normalized
journals. The ten-field contract preserves document code/type, account and
alternate account codes, invoice/user identifiers, remarks, and debit/credit
values with tenant, branch, date, text-filter, and pagination scope. The
database-backed fixture and focused report checks are green; exact account
names, opening balances, fiscal-period/grouping rules, historical ledger and
tax reconciliation, print output, and PowerBuilder golden replay remain open.
See `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`.

The captured `Accounts Ledger` alias now uses the same bounded historical
VirtualGl union as the Trial Balance: posted canonical journal lines and
imported `historical_gl_entries` are both visible, with matching chart codes
resolved and unmatched codes labeled Historical. The cash-only alias remains
canonical-only because historical cash-account mapping is not reviewed. Exact
opening-balance, account-name, fiscal-period, print, and golden replay parity
remain open.

## Customer Sales summary projections follow-up - 2026-08-07

`Reports > Sales Reports > Customer Sales > Invoice Summary`,
`Days Summary`, `Items Summary`, `Hourly Graph`, and
`Monthly Net Sales`, plus the captured top-level
`Monthly Net Sales Summary`, now resolve to explicit
canonical/compatibility sales projections. Invoice rows are de-duplicated
before day/month grouping; item rows are grouped by item/customer; each
exposes a typed six-column summary contract. Exact PowerBuilder grouping,
net/return/tax/profit columns, print output, and golden replay remain open.
See `docs/PHASE_N_CUSTOMER_INVOICE_SUMMARY_EVIDENCE_2026-08-07.md`.

The captured Customer Sales Invoice Wise Profit Margin Detail leaf now uses
an 11-field canonical/compatibility projection with posted sale price, amount,
tax, FIFO allocation cost when available, gross profit, and margin. Rows
without an explicit source cost leave profit and margin blank. Exact
PowerBuilder valuation, discount/return/tax rules, print output, and golden
replay remain open. See
`docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.

`Daily Sales Summary with Profit (Day wise grouping)` now aggregates the same
bounded profit rows by calendar day/customer and gates aggregate cost/profit/
margin on complete source-cost coverage. Exact PowerBuilder day grouping,
valuation, and output remain open.

`Customer Sales > Customer Category Wise Sales > Customer Wise Gross Profit`
now aggregates those bounded rows by customer with last-posted date and
average sale price, using the same complete source-cost gate for cost/profit/
margin. Exact PowerBuilder customer grouping, valuation, and output remain
open in `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`.

The neighboring `Customer Wise Summary` and `Net Sales and Volume` leaves now
use de-duplicated invoice rows grouped by customer with last-posted date,
volume, and authoritative net-sales amount. Category joins, return/net rules,
and exact legacy output remain open.

Purchase Return now supports multiple explicit source-batch allocations per
line in the legacy transaction surface. Active batch choices, per-allocation
quantities, duplicate prevention, and exact line-total validation are wired to
the canonical purchase-return command; the legacy single-batch fields remain
available for compatibility. The focused browser contract is added but not run
in the short verification slice, and exact source reconciliation, raster/focus
parity, and operator UAT remain open.

## Withholding tax source projection follow-up - 2026-08-07

The captured `Withholding Tax Deduction` leaf now reads a distinct
`historical_withholding_tax_entries` projection populated by the guarded
`bulk-historical -wave withholding` path from `dbo.PurPayment`. Posted payment
identity, purchase invoice, supplier legacy identity, WHT base/rate/amount,
account, check/reference, remarks, user, and raw payload are retained. The
implementation intentionally does not derive withholding from purchase-line
advance tax. Focused migration/API/report and Svelte checks passed; source
import counts, exact legacy grouping/certificate semantics, print formats,
and UAT remain open. See
`docs/PHASE_Q_WITHHOLDING_TAX_EVIDENCE_2026-08-07.md`.

The reviewed customer/supplier payment stream now has a source-backed
`historical_party_payment_allocations` target and guarded `-wave payments`
loader. It retains `PurPayment`, installment receipt detail, and direct
SaleLedger/Purledger fallback snapshots, and feeds the posted party statement
and finance-ledger read paths without dropping unresolved legacy identities.
Exact payment counts/totals, source amount semantics, invoice allocation,
return allocation, canonical entry workflow, and print/UAT parity remain open;
the separate SaleReceivableAdj adjustment stream is retained but its exact
legacy posting/grouping remains unverified. See
`docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.

The reviewed `SRAllocationHeader/Detail` and `PRAllocationHeader/Detail` return
allocation streams are now retained separately and exposed in bounded customer
and supplier statements/finance-ledger reads with `return-allocation` source
type. They remain excluded from aging and canonical balance mutation until
source amount, duplicate, and legacy posting/grouping semantics are reconciled.

Party statements and aging now include posted imported business documents even
when their source-backed party-ledger row has no canonical GL journal; newly
posted documents retain the posted-journal requirement. This preserves the
historical ledger wave without fabricating a VirtualGl-to-GL conversion.

## Receivables-aging source projection follow-up - 2026-08-07

The captured Receivables Aging alias now retains SaleLedger DueDate in the
historical business-document payload and joins it to posted customer
party-ledger entries. Payables Aging retains Purledger CreditDays and derives
a bounded due date from the source purchase date. Both expose NOT DUE, 0-30,
31-60, 61-90, and 91+ buckets; missing or invalid terms remain explicitly
unaged. Source import, payment allocation, exact bucket/date/return semantics,
print output, and UAT remain open in
docs/PHASE_Q_RECEIVABLES_AGING_EVIDENCE_2026-08-07.md.

The aging aggregates now use retained business-document `balance_amount` for
posted debit/credit entries, preventing fully paid migrated documents from
being counted as open solely from original invoice totals. This remains a
bounded open-balance correction; individual payment allocation and exact
legacy aging semantics remain unverified.

## Advance-tax report source projection follow-up - 2026-08-07

The captured Customer Wise Advance Tax and Supplier Wise Advance Income Tax
leaves now read explicit posted line tax snapshots with kind = advance_tax;
the customer leaf is sales-scoped and the supplier leaf is purchase-scoped.
Numeric business_documents legacy_payload.AdvanceTaxAmt is a guarded first-line
fallback only; generic line
tax_amount is not relabeled, and rows without positive amount evidence are
omitted. Focused API/source and web checks are recorded in
docs/PHASE_Q_ADVANCE_TAX_EVIDENCE_2026-08-07.md. SQL Server source
reconciliation, exact grouping/rate/base/rounding semantics, print output,
database replay, and UAT remain open.

Sales List selection now fetches and restores the canonical sales document when
the history projection supplies `documentId`, including all lines, return
source document/line IDs, pricing fields, and retained batch allocations across
sales, returns, quotations, and refused-sales. Rows without canonical identity
continue to display the compatibility summary path.
The focused Svelte and targeted API history-query checks passed; browser
execution, legacy raster/focus parity, live data reconciliation, and operator
UAT remain open.

## Sale-detail source promotion follow-up - 2026-08-07

The large canonical `dbo.Saledetail` line slice now has a dedicated
`migration/cmd/bulksalelines` COPY/set-based path with protected canonical
scope, reviewed source identity, `SaleLedger`/item dependency checks,
idempotent target conflict handling, and explicit migration exceptions. The
focused command and migration-config checks passed without opening either
database. Source execution/count reconciliation, full return-line promotion,
exact legacy calculations, report/print output, and UAT remain open. See
`docs/PHASE_E_SALE_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

## Party statement source-scope follow-up - 2026-08-07

Customer and supplier statement retrieval metadata now names the same
source-backed return-allocation unions as the finance query
(`SRAllocationDetail` and `PRAllocationDetail`). Focused API query/definition
tests passed. Exact historical settlement/grouping/print semantics and live
reconciliation remain open; see
`docs/PHASE_Q_PARTY_STATEMENT_SCOPE_EVIDENCE_2026-08-07.md`.

## Purchase-order source promotion follow-up - 2026-08-07

The reviewed `dbo.PurOrderDetail` line range now has a dedicated
`migration/cmd/bulkorderlines` COPY/set-based path with protected canonical
scope, `PurOrderHeader`/item dependency checks, retained order payload,
idempotent target identity, and explicit exceptions. Focused package checks
passed without database execution. Source count/quantity/amount reconciliation,
exact order calculations, print output, and UAT remain open; see
`docs/PHASE_E_PURCHASE_ORDER_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

## Return-line source promotion follow-up - 2026-08-07

The reviewed `dbo.SRdetail` and `dbo.PRdetail` return-line ranges now have a
single guarded `migration/cmd/bulkreturnlines` path with fixed sale/purchase
modes, scoped `SRLedger`/`PRLedger` dependencies, reviewed legacy identities,
retained mode-specific payloads, idempotent target conflicts, and auditable
exceptions. Focused tests and vet passed without database execution. Canonical
source execution, return count/quantity/amount reconciliation, exact legacy
calculations, stock/ledger effects, print output, and UAT remain open; see
`docs/PHASE_E_RETURN_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

## Unified party-ledger running balance follow-up - 2026-08-07

The authenticated party-ledger read now computes `balanceAfter` across the
unified canonical, payment, adjustment, and return-allocation stream using a
stable occurred-at/row-id window. Focused finance tests passed without database
execution. Exact PowerBuilder opening, same-time ordering, settlement, print,
and live reconciliation semantics remain open; see
`docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.

## Transaction-history filter follow-up - 2026-08-07

Sales List and Purchase List now expose the authenticated transaction-history
`filter` query through a shared legacy-style `Filter / Retrieve` toolbar. The
Svelte check passed with 0 errors and 0 warnings. Exact PowerBuilder wildcard,
sorting, focus, raster, live-data, and UAT parity remain open; see
`docs/PHASE_I_TRANSACTION_HISTORY_FILTER_EVIDENCE_2026-08-07.md`.

## Purchase-label edge workflow follow-up - 2026-08-07

Purchase `Print Purchase Labels` now sends typed label rows to the existing
edge ESC/POS route and retains browser preview as the unavailable-adapter
fallback. Software checks pass; physical label geometry, legacy byte/raster
comparison, and operator UAT remain open in
`docs/PHASE_U_HARDWARE_EVIDENCE.md`.

# AbuzarNext acceptance evidence — 2026-08-07

## Status

**Implementation/test gate: green. Overall 100% legacy replacement acceptance: escalated pending external and incomplete-data evidence.**

This is the current handoff for `D:\ABUZAR\AbuzarNext`. It supersedes the
older “empty database” audit wording where the later Phase E artifacts provide
more current measurements. It does not convert a route, a generic report
projection, or a deterministic hardware renderer into proof of exact legacy
behavior.

## Remaining timeframe

The maintained parity plan still estimates approximately **83 developer-weeks
single-threaded** for the complete A-Z replacement lifecycle, or roughly
**5-6 calendar months with three parallel full-time developers**. That is a
planning baseline, not a claim that the current implementation is 83 weeks from
completion: several bounded slices are already green, but the remaining work is
not linearly reducible from the 763-table/151-report catalog counts. With the
current single-agent interactive cadence, a reliable calendar promise is not
shorter than that multi-month baseline and is likely longer.

The estimate excludes unknown waiting time for a reviewed SQL Server migration
window, physical device access, provisioned full-volume PostgreSQL capacity, and
operator UAT/cutover approval. Those are acceptance dependencies, not something
that a local code check can close.

## Fresh local verification

All commands were run from the repository root against the supervised local
stack on 2026-08-07.

| Area | Command / probe | Result |
|---|---|---|
| Web types | `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| Web production build | `pnpm --filter @abuzar/web build` | Passed: SvelteKit static build completed |
| Browser workflows | `pnpm --filter @abuzar/web test -- --workers=1 --retries=1 --reporter=line` plus focused no-retry checks | Passed: 77/77 test cases serially with no retry or application assertion failure. Both base/child Change User tests pass without retries, including persisted MDI-registry clearing. Save/Post/Void cash-sale UI path, Window/MDI navigation and reload restoration, shared File/Window chrome on Preferences and maintenance/manage/module child routes, purchase invoice/source-fetch population, purchase-return population, canonical item-price submission, the 6-test sales-canonical suite, the 5 purchase phase-CD tests, and the SalePrice tier-selector, Lock Item Batches, Group Allowed Price Setting, and imported group-scope setting regressions also passed |
| Daily Sale Detail captured column contract | `go test ./services/api/internal/httpapi -run 'TestDailySaleDetail\|TestSalesReadModel\|TestReadModelsExposeCanonicalSales' -count=1` plus focused Playwright report workflow | Passed: canonical/compatibility detail projection and 11 visible columns; retrieval, preview, and workbook export passed |
| Report format and print-preview workflow | `go test ./services/api/internal/httpapi -run 'TestReport(DefinitionAcceptsBoundedDatabaseLetterheadAndFormats\|FormatSelectionReturnsCanonicalConfiguredName)' -count=1`; focused Daily Sale Detail Playwright; full serial browser suite | Passed: selected format is validated server-side and round-tripped from the format dialog; preview exposes letterhead, ruler, toolbar, zoom, and two loaded-row preview pages. Exact PowerBuilder format calculations and golden print/PDF/workbook output remain open. See `docs/PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md` |
| Report contextual File command routing | `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g "report File menu commands" --workers=1 --retries=0 --reporter=line`; `cmd /c pnpm --filter @abuzar/web check` | Passed: captured report File commands `Retrieve` and `Preview` now invoke the live report argument and print-preview handlers through the contextual menu. The test waits for menu hydration and activates the captured dialog surface before confirming. Exact PowerBuilder command side effects, every report leaf, and physical print/PDF acceptance remain open. |
| Ordered PostgreSQL schema replay | `ops/postgres/apply-migrations.ps1` with local admin DSN | Passed: sequential replay through 028, `Applied 29 migrations.`; the new `029_auxiliary_master_kinds.sql` was then applied with `psql --set ON_ERROR_STOP=1` to the supervised local target and is covered by the auxiliary integration constraint assertion. Prior contention failure and later clean run are retained in `migration/ORDERED_MIGRATION_REPLAY_2026-08-07.json` |
| Go behavior | `DATABASE_URL=postgres://.../abuzar_next go test ./services/api/... ./services/edge/... ./migration/... -count=1` | Passed: all API, edge, migration, and DB-backed integration packages |
| Go static checks | `go vet ./services/api/... ./services/edge/... ./migration/...` | Passed: no issues |
| Posted void workflow | API package suite plus migration `028_business_document_void_reversals.sql` | Passed: canonical sale, sale-return, purchase-return reversal, dependency blocking, append-only projections, and idempotent replay |
| Local runtime | `ops/local/status-local.ps1` | PostgreSQL, API, edge, and web healthy; sampled HTTP status 200 |
| Canonical historical loader safety | `go test ./migration/cmd/bulk-historical` | Passed: canonical source requires explicit opt-in/scope and stock dependency loss fails closed |
| Source-backed history/adjustments | `go test ./services/api/internal/httpapi -run 'TestHistorical' -count=1` | Passed: migration contract plus tenant/branch-scoped ItemLog/AdjDetail report reads |
| Historical StockReport Back Date | `go test ./services/api/internal/httpapi -run 'TestHistoricalStock(ReadModelCarriesCapturedStockReportFields\|BackDateReportUsesImportedStockReportFields)' -count=1`; focused stock-report Playwright check | Passed: source-backed `historical_stock_snapshots` query preserves StockReport stock, purchase, sale, average, recent-purchase, and pack-unit fields with tenant/date/godown isolation. Exact Back Date grouping/print parity remains open. See `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md` |
| Historical VirtualGl GL Journal | `go test ./services/api/internal/httpapi -run 'TestHistorical(GLJournal\|Stock)\|TestPhaseQ' -count=1`; focused Phase Q Playwright check | Passed: imported `historical_gl_entries` rows preserve document/type/account/alternate/invoice/user/remarks/debit/credit fields with tenant/branch/date/text scope, and the definition exposes ten source-backed columns. Exact account naming, opening balances, reconciliation, and print parity remain open. See `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md` |
| Stock Adjustments Detail normalized history | `go test ./services/api/internal/httpapi -run 'Test(PhaseQHistoricalQueriesAreScopeBoundAndPaginated\|HistoricalStockAdjustmentReportIncludesNormalizedLedgerRows\|HistoricalReportsReadRetainedSourceRowsWithinTenantBranch)' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: imported `AdjHeader`/`AdjDetail` rows and posted signed normalized stock-ledger adjustments share the scoped six-field report contract. Database-backed cases are skipped when `DATABASE_URL` is unavailable; exact PowerBuilder grouping and print parity remain open. See `docs/PHASE_Q_STOCK_ADJUSTMENT_EVIDENCE_2026-08-07.md` |
| Stock threshold reports | `go test ./services/api/internal/httpapi -run 'Test(StockLevelReadModelsUseItemThresholdPayloadAndPostedScope\|PhasePStockRegistryCoversCapturedLeaves\|StockReadModelUsesPostedNormalizedLedgersAndBoundedPagination)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: Reorder, Optimum, Minimum, and Reorder/Optimum reports now expose threshold fields from item payload fallbacks with posted-ledger gating and bounded tenant/branch filters. No DB-backed route result was claimed because `DATABASE_URL` was unavailable; exact threshold semantics and print parity remain open. See `docs/PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md` |
| Stock Management Report | `go test ./services/api/internal/httpapi -run 'Test(StockManagementReadModelIncludesThresholdsAndPostedScope\|PhasePStockRegistryCoversCapturedLeaves\|StockReadModelUsesPostedNormalizedLedgersAndBoundedPagination)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check`; `git diff --check` | Passed: the captured leaf now has an explicit eight-field normalized balance projection with posted-ledger gating and item-payload reorder/optimum/minimum thresholds, without an unverified alert predicate. No DB-backed route result was claimed; exact alert/status, valuation, grouping, source reconciliation, and print parity remain open. See `docs/PHASE_P_STOCK_MANAGEMENT_EVIDENCE_2026-08-07.md` |
| Item Stock Register Summary | `go test ./services/api/internal/httpapi -run 'Test(StockItemSummaryReadModelAggregatesPostedLedgerByItemDay\|StockLevelReadModelsUseItemThresholdPayload\|PhasePStockRegistryCoversCapturedLeaves)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the leaf now aggregates posted stock-ledger movement by item, godown, and calendar day with signed net quantity and net value. No DB-backed route result was claimed because `DATABASE_URL` was unavailable; opening-balance, grouping, valuation, and print parity remain open. See `docs/PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md` |
| Stock and Sales | `go test ./services/api/internal/httpapi -run 'Test(StockSalesReadModelJoinsCanonicalPostedSaleAllocations\|StockItemSummaryReadModelAggregatesPostedLedgerByItemDay\|StockLevelReadModelsUseItemThresholdPayload\|PhasePStockRegistryCoversCapturedLeaves)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the leaf now joins normalized balances to canonical posted sale allocations and exposes On Hand plus Sales Qty with bounded scope. No DB-backed route result was claimed because `DATABASE_URL` was unavailable; exact period/as-of, return, valuation, grouping, and print parity remain open. See `docs/PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md` |
| Narcotics stock reports | `go test ./services/api/internal/httpapi -run 'TestStockNarcoticsReadModelsUseCapturedItemFlagsAndPostedScope$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the two narcotics movement leaves now filter posted normalized stock-ledger rows by the captured Item Narcotics payload flag, and the generic-type leaf groups the captured GenericName/GenericCode payload with bounded tenant/branch/godown/batch/date/text scope. No DB-backed route result was claimed; exact legacy flag semantics, grouping, returns/opening, source reconciliation, and print parity remain open. See `docs/PHASE_P_NARCOTICS_STOCK_EVIDENCE_2026-08-07.md` |
| Class-wise expiry report | `go test ./services/api/internal/httpapi -run 'TestStockExpiryClassReadModelUsesTypedExpiryAndItemClass$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: `Expiry Report(Class Wise)` now exposes a six-field typed-expiry/class projection over normalized balances with posted-ledger gating and bounded scope. No DB-backed route result was claimed; exact class-code joins, legacy date semantics, source reconciliation, and print parity remain open. See `docs/PHASE_P_EXPIRY_CLASS_EVIDENCE_2026-08-07.md` |
| Stock-in-Hand classification reports | `go test ./services/api/internal/httpapi -run 'TestStockClassificationReadModelsUseCapturedItemGroups$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: Manufacturer, Manufacturer Format2, Category, and Class leaves now expose explicit Item-payload classification columns over normalized balances with posted-ledger gating and bounded scope. No DB-backed route result was claimed; exact group joins, valuation, supplier association, source reconciliation, and print parity remain open. See `docs/PHASE_P_STOCK_CLASSIFICATION_EVIDENCE_2026-08-07.md` |
| Daily/date-wise stock IN/OUT reports | `go test ./services/api/internal/httpapi -run 'TestStockMovementSummaryReadModelsAggregatePostedInOutByDay$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: `Daily Stock IN/OUT` and `Stock IN/OUT(Date Wise)` now expose explicit posted-ledger day/direction/godown/item aggregates with signed quantity and net value. No DB-backed route result was claimed; opening balances, exact date-wise grouping, source reconciliation, valuation, and print parity remain open. See `docs/PHASE_P_STOCK_MOVEMENT_SUMMARY_EVIDENCE_2026-08-07.md` |
| Supplier/manufacturer stock association | `go test ./services/api/internal/httpapi -run 'TestStockSupplierManufacturerReadModelUsesItemSuppliersAndPostedBalances$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: `Supplier Manufacturer Association` now exposes Manufacturer, aggregated Supplier(s), Godown, Item, On Hand, and Unit Cost from normalized balances, captured Item payload, item-supplier links, and posted-ledger gating. No DB-backed route result was claimed; exact priority/association joins, valuation, source reconciliation, and print parity remain open. See `docs/PHASE_P_STOCK_SUPPLIER_MANUFACTURER_EVIDENCE_2026-08-07.md` |
| Purchase and Purchase Return line detail | `go test ./services/api/internal/httpapi -run 'Test(PurchaseLineDetailReadModelCarriesCanonicalAndCompatibilityFields\|PhaseOReportRegistryCoversCapturedPurchaseLeaves\|PurchaseReadModelUsesCanonicalLedgersPostedFiltersAndPagination)' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the two captured detail leaves now expose a bounded 12-field canonical/compatibility line contract with posted, tenant/branch, date, text, and document-identity de-duplication rules. No DB-backed route result was claimed because `DATABASE_URL` was unset in this focused run. Exact purchase grouping, tax/profit, order matching, and print parity remain open. See `docs/PHASE_O_PURCHASE_LINE_DETAIL_EVIDENCE_2026-08-07.md` |
| P/O Based Purchase Disparity | `cmd /c go test ./services/api/internal/httpapi -run TestPurchaseOrderDisparityReadModelComparesLinkedOrderAndReceiptLines -count=1`; `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g "P/O disparity report" --workers=1 --retries=0 --reporter=line`; `cmd /c pnpm --filter @abuzar/web check` | Passed: canonical posted purchase-order lines now compare linked posted receipt lines by source document ID/number and expose ten ordered/received/disparity fields. Unlinked legacy receipt matching, source reconciliation, exact PowerBuilder calculations, and golden print/PDF/Excel output remain open. See `docs/PHASE_O_PURCHASE_ORDER_DISPARITY_EVIDENCE_2026-08-07.md` |
| Quotation and Refused Sales reports | `go test ./services/api/internal/httpapi -run 'Test(NoStockDocumentReportsUseCanonicalAndDeduplicatedCompatibilityRows\|CapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions\|PhaseQRegistryCoversTheMappedRemainingLeaves\|PhaseQQueriesArePostedAndScopeBound\|PhaseNReportRegistryDefinitionsAndAggregateFilters)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: quotation detail/summary and refused-sales detail now use canonical posted no-stock documents/lines with de-duplicated compatibility events and document-summary grouping. No DB-backed route result was claimed; exact PowerBuilder columns/calculations, print output, and golden replay remain open. See `docs/PHASE_Q_NO_STOCK_DOCUMENT_EVIDENCE_2026-08-07.md` |
| Header Wise Transaction Summary | `go test ./services/api/internal/httpapi -run 'Test(HeaderTransactionReportUsesCanonicalHeadersAndCompatibilityFallback\|NoStockDocumentReportsUseCanonicalAndDeduplicatedCompatibilityRows\|CapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions\|PhaseQRegistryCoversTheMappedRemainingLeaves\|PhaseQQueriesArePostedAndScopeBound\|PhaseNReportRegistryDefinitionsAndAggregateFilters)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the captured report now groups canonical posted business-document families once per header, sums line quantities, retains authoritative totals, and de-duplicates compatibility events. No DB-backed route result was claimed; exact transaction-type labels/calculations, opening-balance treatment, print output, and golden replay remain open. See `docs/PHASE_Q_HEADER_TRANSACTION_EVIDENCE_2026-08-07.md` |
| Reprinting report contracts | `go test ./services/api/internal/httpapi -run 'TestPhaseQReprintDefinitionsUseCanonicalReadModels$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the eight leaves now use canonical sale-line, sale-summary, or purchase-line metadata and retain compatibility fallback; Svelte check passed with 0 errors and 0 warnings. The added focused browser contract was not run in this quick-check slice. Exact PowerBuilder selection, summary sections, format calculations, print output, and golden replay remain open. See `docs/PHASE_Q_REPRINT_EVIDENCE_2026-08-07.md` |
| Deleted Sale Items Log source-backed projection | `go test ./services/api/internal/httpapi -run 'Test(PhaseQItemHistoryDefinitionsUseSourceBackedProjections\|PhaseQHistoricalQueriesAreScopeBoundAndPaginated\|HistoricalDeletedSaleItemMigrationRetainsSourceRows\|PhaseQRegistryCoversTheMappedRemainingLeaves)$' -count=1`; `go test ./migration/cmd/bulk-historical -run '^$'`; `cmd /c pnpm --filter @abuzar/web check`; `git diff --check` | Passed: the code path retains captured `dbo.DeletedSaleItem` source fields and exposes a scoped six-field report contract; loader compile, API contracts, Svelte check, and whitespace validation passed. No canonical source import or DB-backed route result is claimed. Exact PowerBuilder columns/order/calculations/print output and reconciliation remain open. See `docs/PHASE_Q_DELETED_SALE_ITEMS_EVIDENCE_2026-08-07.md` |
| Canonical purchase history population | `go test ./services/api/internal/httpapi -run 'TestCanonicalPurchaseHistoryHydratesDocumentIdentityAndDetail' -count=1`; focused `phase-cd.spec.ts -g "purchase .* population"` | Passed: canonical history identity, permission/tenant-scoped full-document read, persisted batch/expiry/discount/GST metadata, invoice hydration, and purchase-return source/batch preservation |
| Sales explicit batch allocation | `cmd /c pnpm --filter @abuzar/web check`; `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts --list`; updated `sales-canonical.spec.ts` contract; `git diff --check` | Passed at source/type-check and Playwright discovery level: positive inventory batches are retained in the sales row, multiple distinct selections serialize through `document.lines[].allocations` with editable quantities, post validation checks duplicate/positive/four-decimal/line-total rules, and Automatic FIFO remains the default. The browser assertion itself was not run in this quick-check slice; exact legacy batch rules and downstream stock workflows remain open. See `docs/PHASE_H_SALES_FRONTEND_EVIDENCE_2026-08-06.md` |
| Opening Stock workflow routing | `cmd /c pnpm --filter @abuzar/web check`; source review of `LegacyWorkflowSurface.svelte` | Passed at source/type-check level: the captured `opening-stock` route now enters the immutable inventory-event path and emits the existing inbound movement contract. Browser/API/DB execution, exact PowerBuilder opening-balance semantics, and source reconciliation remain open. |
| Inventory adjustment input contract | `cmd /c pnpm --filter @abuzar/web check`; `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts --list`; source review of `LegacyWorkflowSurface.svelte` | Passed at source/type-check and Playwright discovery level: increase/decrease/adjustment/opening forms search active canonical items, select an active godown, require batch identity, positive quantity at four-decimal precision, and explicit signed adjustment values where applicable, then trim emitted payload fields. Browser/API/DB execution and exact legacy batch-selection behavior remain open. |
| Purchase Populate Items command | `cmd /c pnpm --filter @abuzar/web check`; `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "Populate Items resolves purchase"` | Passed: Svelte check 0 errors/0 warnings and the focused browser test 1/1. Source review confirms quick-search resolution through active canonical lookup, UUID/legacy identity hydration, and fail-closed save/post validation for unresolved free text. Exact PowerBuilder source-selection/template/side-effect/raster behavior remains open. See `docs/PHASE_I_PURCHASE_ITEM_POPULATION_EVIDENCE_2026-08-07.md` |
| Purchase Populate From Sale Template | `cmd /c pnpm --filter @abuzar/web check`; refreshed focused `purchase-canonical.spec.ts` Populate test group | Passed: the focused Populate group is recorded as 2/2 in `docs/PHASE_I_PURCHASE_ITEM_POPULATION_EVIDENCE_2026-08-07.md`, with Svelte check 0 errors/0 warnings. Source review confirms active tenant-scoped template listing, supported rows/lines/items payload hydration into a new draft, canonical item re-resolution, and fail-closed handling for unsupported payloads. Exact PowerBuilder template/pending-due/source-selection behavior remains open. See `docs/PHASE_I_PURCHASE_ITEM_POPULATION_EVIDENCE_2026-08-07.md` |
| Customer Sales profit leaves and Hourly Graph | `go test ./services/api/internal/httpapi -run 'Test(CustomerSalesProfitMarginReadModelUsesAllocatedCost\|DailySalesProfitSummaryAggregatesCompleteCostRows\|CustomerSalesSummaryReadModelsUseExplicitBuckets\|PhaseNReportRegistryDefinitionsAndAggregateFilters\|InvoiceSummaryReadModelsGroupRowsOncePerDocument)$' -count=1`; `cmd /c pnpm --filter @abuzar/web check` | Passed: the profit detail exposes an 11-field canonical/compatibility projection with posted sale price, amount, tax, FIFO allocation cost where available, gross profit, and margin; the daily profit leaf aggregates by day/customer with complete-cost gating; and Hourly Graph uses a six-field hour/customer aggregate over de-duplicated invoices. No database-backed replay was claimed because DATABASE_URL was unset. Exact PowerBuilder valuation, graph rendering, discount/return/tax/rounding/print behavior remains open. See `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md` and `docs/PHASE_N_CUSTOMER_INVOICE_SUMMARY_EVIDENCE_2026-08-07.md` |
| Canonical item maintenance | `go test ./services/api/internal/httpapi -run 'TestCanonicalItemMaintenanceValidation\|TestMaintenanceManageOperationsIntegration' -count=1`; focused browser maintenance checks | Passed: price, discount, basic-data, and reorder/minimum updates lock canonical `master_items`, preserve legacy aliases, record audit previous/new values, accept browser-shaped JSON numbers, and submit decimal prices through the shared Svelte form. See `docs/PHASE_S_ITEM_MAINTENANCE_EVIDENCE_2026-08-07.md` |
| Maintenance batch locking | `go test ./services/api/internal/httpapi -run 'TestBatchLockMaintenanceValidation\|TestMaintenanceManageOperationsIntegration' -count=1`; `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --reporter=line --grep "Lock Item Batches"` | Passed: current-branch tenant-scoped lock/unlock mutation, affected-row audit, other-branch isolation, and captured Svelte field submission. See `docs/PHASE_S_BATCH_LOCK_EVIDENCE_2026-08-07.md` |
| Imported group-scope settings | `go test ./services/api/internal/httpapi -count=1`; `pnpm --filter @abuzar/web check`; focused Playwright checks for `Group Allowed Price Setting` and imported group-scope setting leaves | Passed: selected-group loading, exact imported composite-key display, PostgreSQL tenant-scoped/audited updates, route-isolated updates for Header, Godown, Price, Cash Account, and Supplier Category scopes, plus denied canonical-UUID godown rejection before stock/document projection. Composite `GroupAllowedGodown` source-key mapping remains open; no inferred source-to-canonical mapping is claimed. See `docs/PHASE_R_GROUP_PRICE_EVIDENCE_2026-08-07.md` |
| Auxiliary Basic Data master CRUD | `go test ./services/api/internal/httpapi -run 'TestAuxiliaryMasterCRUDIntegration' -count=1`; `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 apps/web/tests/phase-f.spec.ts` | Passed: the `029` kind constraint, tenant-scoped create/list/update/delete, source-shaped payload retention, route-specific fields, confirmed delete, and unknown-kind read-only boundary. See `docs/PHASE_F_AUXILIARY_MASTER_EVIDENCE_2026-08-07.md` |
| Pricing workflow follow-up | `go test ./services/api/internal/httpapi -run 'TestCanonicalPurchaseLoadsItemSupplierScheme\|TestCanonicalPurchaseHistoryHydratesDocumentIdentityAndDetail\|TestPricingPreviewRequestMapsTiersDiscountsAndTaxes' -count=1`; focused sales/purchase browser suites | Passed: all ten SalePrice selector levels are represented, selected rows and preview tier arrays stay synchronized, and canonical purchases inherit a valid tenant-scoped ItemSuppliers discount/bonus scheme when no explicit override is supplied. See `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md` |

A separate retries-disabled full browser attempt hit Chromium
`net::ERR_NO_BUFFER_SPACE` before one keyboard-shortcut test loaded, and an
earlier standard run saw one `browser.newContext: Target page, context or
browser has been closed` retry before the sales-history test. The affected
tests passed in isolated no-retry runs, and the latest post-change standard
serial run completed 77/77. These are retained as supervised-browser resource
boundaries, not application assertion failures.

The full-volume Phase W attempt is recorded separately in
[`PHASE_W_PERFORMANCE_EVIDENCE_2026-08-07.md`](PHASE_W_PERFORMANCE_EVIDENCE_2026-08-07.md).
It failed during disposable seeding when PostgreSQL terminated the connection;
it is not a performance pass. The local application stack was healthy again
afterward.

Playwright now defaults to one worker with two retries in
`apps/web/playwright.config.ts`. The shared supervised local stack previously
caused two mocked-route failures under ten workers and one Chromium context
startup failure during a serial run. The same product tests pass serially; a
retry only covers the browser/process boundary and does not turn a failed
assertion into success.

## Proven implementation coverage

- The base PowerBuilder catalog contains 275 captured entries, and the
  contextual catalog covers the captured pack-purchase, cash-sale, item-master,
  report, and groups window states (325/326/314/306/295 entries respectively).
- The browser walk reaches 221 generated leaf routes and the direct SSR smoke
  gate covers the critical transaction, master, report, maintenance, manage,
  and generic-module deep links.
- Canonical sales, sale returns, open returns, quotations, refused sales,
  purchases, purchase returns, and purchase orders have typed document
  contracts, draft/post or save-and-post lifecycle handling, idempotency, and
  server-side validation. Purchase orders remain intentionally stock/GL-neutral.
- Canonical posted sales, sale returns, purchases, and purchase returns now
  support append-only compensating voids across stock, GL, and party-ledger
  projections, with dependency blocking and idempotent replay. This is verified
  new-system behavior; exact PowerBuilder void-dialog semantics remain open.
- Master List/Detail, item supplier links, Users, Groups/permissions,
  branch/counter context, preferences, shifts, maintenance audit operations,
  report retrieval/format/preview/export controls, and offline queue plumbing
  have focused browser/API coverage.
- The 16 auxiliary Basic Data leaves now have source-informed Detail/List
  fields and tenant-scoped create/list/update/confirmed-delete workflows. The
  payload preserves captured identifiers such as `PricePolicyCode`, `ICode`,
  `SalesTaxScheduleCode`, and `SalePromotionCode`; this remains a compatibility
  CRUD layer rather than proof of source-backed rule or data parity.
- The sales SalePrice selector now covers all ten captured tiers, preserves the
  item payload tier array, reprices selected rows, and sends the displayed tier
  to the authenticated exact-decimal preview. Canonical purchases also inherit
  valid normalized ItemSuppliers discount/bonus schemes when no explicit line
  override is provided. This bounded pricing workflow is evidenced in
  [`docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md`](PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md);
  customer/group price assignment and full PricePolicy promotion remain open.
- Daily Sale Detail now exposes the captured Alias, Item Description, Sale Price,
  Qty, Disc(%), Discount Value, Item Disc, SalesTax Value, Amount, Expiry Date,
  and Batch Number contract, preferring retained historical line payload values
  and allocation-aware batch/expiry values where available. This is a bounded
  column/read-model parity improvement, not proof of all ten format layouts.
- Captured report-window File commands `Retrieve`, `Preview`, paging, filtering,
  sorting, saved inputs/filters, print, and export labels now have concrete
  report handlers where the current report surface supports them. The focused
  browser evidence covers menu hydration, retrieval through the arguments dialog,
  and print preview; it does not claim exact PowerBuilder side effects for all
  151 report leaves or physical output parity.
- The captured Stock in Hand > Back Date report now reads imported
  `historical_stock_snapshots` from `dbo.StockReport` and exposes the source
  row, as-of date, stock, purchase/sale/average/recent-purchase prices, and
  pack units. This is a bounded source-backed report improvement; exact
  grouping, valuation, print output, and the remaining Stock Reports leaves
  are still open. Evidence is in
  [`docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md`](PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md).
- The four captured stock threshold leaves now use a distinct normalized
  read model exposing on-hand plus Reorder/Optimum/Minimum quantities from
  item payload fields and maintenance fallbacks. This is a bounded threshold
  workflow improvement; exact comparison, grouping, source reconciliation,
  and print output remain open. Evidence is in
  [`docs/PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md`](PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md).
- `Item Stock Register Summary` now uses a normalized posted stock-ledger
  aggregation by item, godown, and day with signed net quantity and net
  value. This is a bounded summary improvement; opening-balance, valuation,
  grouping, source reconciliation, and print output remain open. Evidence is
  in [`docs/PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md`](PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md).
- `Stock and Sales` now joins normalized current balances to canonical posted
  sale allocations for the requested period and exposes On Hand plus Sales
  Qty. This is a bounded projection; exact period/as-of, return, valuation,
  grouping, source reconciliation, and print output remain open. Evidence is
  in [`docs/PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md`](PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md).
- The captured GL Journal alias now reads the imported `historical_gl_entries`
  projection from `dbo.VirtualGl` alongside explicitly labeled newly posted
  normalized journals, preserving the source document/type/account/alternate/
  invoice/user/remarks/debit/credit fields under tenant, branch, date, and
  text scope. This is a bounded historical finance improvement; exact account
  names, opening balances, reconciliation, grouping, and print output remain
  open. Evidence is in
  [`docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`](PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md).
- The captured Accounts Ledger alias now unions the same imported
  `historical_gl_entries` with posted canonical journal lines and labels
  unmatched legacy accounts as Historical. The cash-only alias remains
  canonical-only because VirtualGl cash-account mapping is not verified;
  opening balances, exact account naming, and print parity remain open.
- The six previously empty Item Reports history/adjustment leaves now have
  normalized source-backed target tables, fail-closed import paths, and
  tenant/branch-scoped report queries. The New Item(s) view is explicitly
  first-observed rather than a claim of recovered creation semantics.
- `Deleted Sale Items Log` now has a retained `dbo.DeletedSaleItem` target,
  guarded `-wave deleted-sale-items` loader, and a scoped six-field report
  contract. Exact deleted-item DataWindow semantics, print output, and source
  reconciliation remain open. Evidence is in
  [`docs/PHASE_Q_DELETED_SALE_ITEMS_EVIDENCE_2026-08-07.md`](PHASE_Q_DELETED_SALE_ITEMS_EVIDENCE_2026-08-07.md).
- Purchase history now supports a scoped canonical detail read and the
  contextual Populate Purchase Invoice/Populate Purchase Return Invoice
  flows hydrate a new draft from persisted lines, supplier/godown/source
  references, batch/expiry, discount, and tax metadata; the source-fetch
  command uses the same canonical purchase-order population path. The
  purchase-return test proves source document and batch allocation preservation.
  This bounded workflow slice is evidenced in
  `docs/PHASE_I_PURCHASE_HISTORY_POPULATION_EVIDENCE_2026-08-07.md`;
  compatibility-only history rows remain summary-only.
- The local normalized store currently reports 60,870 `master_items`, 373,498
  `business_documents`, 1,056,557 document lines, 11,582 stock batches, 781,974
  stock-ledger movements, 408 operational GL journals, 329,523 party-ledger
  entries, 1,021,801 historical GL entries, and 501,460 migration exceptions.
  These direct target counts include the existing sandbox/reference and
  isolated canonical data and must not be presented as complete canonical
  reconciliation.
- Existing committed raster evidence records exact untouched-state comparisons
  for the shell and representative transaction, master, preference, report,
  maintenance, backup, change-user, and integrity-monitor captures. Interactive
  controls are validated by behavior tests after leaving the untouched raster
  state.
- The Window/MDI registry now survives client-side document navigation and a
  hard reload, with Window-menu activation and retained tab coverage in
  [`PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md`](PHASE_C_WINDOW_MDI_EVIDENCE_2026-08-07.md).
- The shared contextual menu now opens the captured Yes/No Change User
  confirmation in both the base shell and child windows; only confirmed
  navigation reaches login, and confirmation clears the persisted MDI window
  registry and requests server-session invalidation at the session boundary.
  Evidence is in
  [`PHASE_C_CHANGE_USER_EVIDENCE_2026-08-07.md`](PHASE_C_CHANGE_USER_EVIDENCE_2026-08-07.md).
- The same shared legacy menu/MDI chrome is now present on the maintenance and
  manage workflow surface, Preferences, and generic catalog module routes.
  Focused direct-navigation tests verify File/Window visibility and the full
  serial browser suite remains green. This improves workflow-shell parity but
  does not promote generic module workbenches to reconstructed business logic.
- Four item-maintenance leaves now have canonical, tenant-scoped PostgreSQL
  mutation paths rather than preference-only persistence. Their focused
  validation, integration, audit, and browser evidence is in
  `docs/PHASE_S_ITEM_MAINTENANCE_EVIDENCE_2026-08-07.md`; the remaining
  maintenance catalog and exact PowerBuilder acceptance are still open.
- The imported group-scope setting leaves now read and update only their
  approved tenant-role scope kinds (`GroupAllowedHeader`, `GroupAllowedGodown`,
  `GroupAllowedPrice`, `GroupCashAccount`, and the empty approved supplier
  category mapping), keep exact composite legacy identifiers visible, and
  record the existing role-access audit event. Explicit canonical-UUID godown
  scopes are enforced at stock read, canonical document, inventory-event, sync,
  lookup/detail, and stored-document void ingress. The imported composite
  `GroupAllowedGodown` mapping and downstream price/header policy semantics
  remain explicit acceptance boundaries in
  `docs/PHASE_R_GROUP_PRICE_EVIDENCE_2026-08-07.md`.

## Acceptance evidence still required

The local target bookkeeping was rechecked on 2026-08-07 with tenant scope
preserved in the probe. In aggregate, `migration_exceptions` has 501,024
resolved, 404 ignored, and 32 open rows; all 32 open rows are
`Purdetail/non_positive_quantity` in the isolated canonical tenant
`6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01`. The sandbox tenant has 500,976 resolved,
404 ignored, and no open `migration_exceptions`. Separately,
`migration_ambiguous_records` has 16 open `tax_rule_has_no_numeric_rate` rows
in the sandbox tenant (four each from `AdditionalTaxRule`, `ExtraTaxRule`,
`IncomeTaxRule`, and `UnitSalesTaxRules`) and none in the canonical tenant. The
reconciliation tool now emits both counts in its `bookkeeping` object, and the
reviewed finance/stock metric counts both tables. The scope-specific probe and
purchase-order wave guard are recorded in
[`docs/PHASE_E_BOOKKEEPING_AND_ORDER_WAVE_EVIDENCE_2026-08-07.md`](PHASE_E_BOOKKEEPING_AND_ORDER_WAVE_EVIDENCE_2026-08-07.md).
These records remain open:
the non-positive purchase lines cannot be coerced into the positive canonical
line contract, and the tax rules have labels such as “NO TAX” and “TAX ON ACTUAL
QTY ONLY” but no numeric rate, so neither class is promoted without reviewed
source semantics.

The captured Customer Sales summary leaves now use tested canonical/
compatibility invoice, day/customer, item/customer, and month/customer
projections with typed six-column contracts. This closes only the
implementation/read-model boundary; the exact PowerBuilder grouping,
net/return/tax/profit fields, print/PDF/workbook output, golden replay, and
operator acceptance remain open. See
[`docs/PHASE_N_CUSTOMER_INVOICE_SUMMARY_EVIDENCE_2026-08-07.md`](PHASE_N_CUSTOMER_INVOICE_SUMMARY_EVIDENCE_2026-08-07.md).

Customer Wise Gross Profit now groups the bounded allocated-cost profit rows
by customer, and Customer Wise Summary plus Net Sales and Volume group
de-duplicated invoice rows by customer. These close only the normalized
read-model boundary; category joins, exact PowerBuilder net/return rules,
valuation, print output, and golden replay remain open. See
[`docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`](PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md).

Customer Category Wise Net Sales and its two captured compatibility aliases now
use a tested six-column category projection over canonical and de-duplicated
compatibility rows. The retained customer master payload is read through the
`Category`/`CustomerCategory`/`category` keys with an explicit `Unspecified`
fallback. This closes only the normalized projection boundary; exact
PowerBuilder category joins, return/net semantics, print output, and golden
replay remain open.

Phase O purchase summaries now include tested document, day, month, item, and
supplier aggregates over canonical purchase documents/lines and de-duplicated
compatibility events. Exact PowerBuilder purchase tax, category/supplier joins,
return/net/profit semantics, graph/format behavior, and golden replay remain
open. See
[`docs/PHASE_O_PURCHASE_SUMMARY_EVIDENCE_2026-08-07.md`](PHASE_O_PURCHASE_SUMMARY_EVIDENCE_2026-08-07.md).

Customer Wise Category Net Sales now adds a tested customer-plus-category
six-column projection over the same scoped rows. Exact PowerBuilder
customer/category joins, return/net semantics, print output, and golden replay
remain open.

Customer Category Wise Sales Detail Report now has a tested source-backed
11-field line-detail contract with retained alias, item, pricing, discount,
tax, amount, expiry, and batch values. Exact category grouping, print output,
and golden replay remain open.

The report handler now routes all three 11-field profit modes through the
extended scanner; the focused `SalesProfitSummaryModesUseExtendedScanner`
regression passed.

| Gate | Current truth | Evidence needed to close it |
|---|---|---|
| Canonical migration | Only reviewed enterprise/config, core-master, security, pricing/tax, purchase, and return slices are reconciled. Purchase-order/detail, sales, full stock, full GL, and remaining historical tables are still deferred. The loader now covers StockReport/VirtualGl plus ItemLog/AdjHeader/AdjDetail with explicit canonical scope, but no live canonical run was proven. The canonical tenant has 32 open line exceptions; the sandbox has 16 open tax ambiguities. | Restore a reviewed read-only SQL Server connection, run the canonical historical/document waves, reconcile counts plus business totals, and close or explain both migration bookkeeping tables. The current source probe hit Windows-authentication timeout/untrusted-domain boundaries. See `migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md` and `migration/PHASE_E_HISTORICAL_STATUS_2026-08-06.md`. |
| Stock/finance history | New canonical posting and compensating-void projections are tested; Back Date reads imported `StockReport` snapshot fields; and GL Journal now preserves imported `VirtualGl` source fields in a bounded report read model. Full legacy `StockReport`/`VirtualGl` valuation, imported-document reversal, account mapping, and historical ledger equivalence are not proven. | Import/reconcile the remaining stock, batch, GL, customer, and supplier ledger data; replay sampled documents to the paisa; and approve Back Date/GL valuation, account mapping, print behavior, and legacy void/reversal cases. See `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md` and `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`. |
| Report parity | All 151 report leaves have a typed definition or explicitly labelled normalized/compatibility projection. Daily Sale Detail, Sale detail, and Sales Return detail now expose bounded 11-field source-backed line contracts; Purchase detail and Purchase Return detail now expose bounded 12-field source-backed purchase-line contracts; Back Date has a ten-field source-backed StockReport contract; GL Journal has a ten-field VirtualGl source contract; Stock Adjustments Detail now unions imported adjustment rows with posted normalized signed adjustments; the four stock threshold leaves now expose on-hand plus threshold quantities; Item Stock Register Summary now aggregates posted stock-ledger movements by item/day/godown; Stock and Sales now exposes current On Hand plus canonical posted Sales Qty; the narcotics movement/generic-type leaves now have explicit Item-payload projections; Class Wise Expiry now has a typed expiry/class projection; the core Stock-in-Hand classification leaves now have explicit Item-payload group projections; and Customer Wise Gross Profit now has an 11-field customer aggregate over the bounded allocated-cost profit rows. Format selection is validated and preview has a legacy-style toolbar/ruler over loaded rows; exact grouping, ten-format calculations, threshold semantics, narcotics flag/generic grouping, class-code/date semantics, classification joins, opening balances, Stock-and-Sales period/as-of semantics, migrated golden output, creation semantics, purchase tax/profit, adjustment calculations, withholding, aging, customer grouping, and the remaining report calculations remain open. | Capture legacy columns/arguments/formats and compare representative output, print preview, PDF, and workbook results against approved golden data. See `docs/PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md`, `docs/PHASE_N_SALES_LINE_DETAIL_EVIDENCE_2026-08-07.md`, `docs/PHASE_O_PURCHASE_LINE_DETAIL_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_NARCOTICS_STOCK_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_EXPIRY_CLASS_EVIDENCE_2026-08-07.md`, `docs/PHASE_P_STOCK_CLASSIFICATION_EVIDENCE_2026-08-07.md`, `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`, `docs/PHASE_Q_STOCK_ADJUSTMENT_EVIDENCE_2026-08-07.md`, `docs/PHASE_M_REPORT_CORE_EVIDENCE_2026-08-06.md`, `docs/PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md`, `docs/PHASE_Q_FINANCIAL_REPORT_EVIDENCE_2026-08-06.md`, and `docs/PHASE_N_CUSTOMER_PROFIT_MARGIN_EVIDENCE_2026-08-07.md`. |
| Auxiliary Basic Data masters | 16 captured leaves now have tenant-scoped source-shaped CRUD and confirmed delete; unknown route kinds remain read-only. | Run the approved SQL Server source wave, reconcile populated rows and dependent joins, observe exact legacy validation/calculation/delete semantics, and complete per-leaf raster/focus/print acceptance. See `docs/PHASE_F_AUXILIARY_MASTER_EVIDENCE_2026-08-07.md`. |
| Pricing and discount policy parity | The deterministic engine, ten-level SalePrice selector, captured tier-array preview, normalized ItemSuppliers discount/bonus inheritance, and a scoped `GroupAllowedPrice` assignment editor are verified. Exact customer/group price enforcement, `PriceTypeCode`/`Module` semantics, `PricePolicyDetail` date semantics, complete policy promotion, ItemSuppliers day semantics, and 50 historical invoice golden replay remain unverified. | Observe/approve the remaining PowerBuilder policy rules and their document/price-level mapping, then apply the approved enforcement and replay at least 50 approved historical Saledetail invoices to the paisa. See `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md` and `docs/PHASE_R_GROUP_PRICE_EVIDENCE_2026-08-07.md`. |
| Full UI/UX parity | Representative shell/dialog rasters, catalog route reachability, and the bounded stateful Window/MDI navigation slice are green. Every contextual command/window state has not received an approved legacy-vs-new raster and keyboard/focus review. | Capture the remaining PowerBuilder states at 1936x1048, run the existing PNG comparison gate, and sign each exception, including cascade/tile/layer geometry and MDI/window-manager behavior. |
| Hardware | Software interfaces, deterministic ESC/POS renderers, desktop IPC, shared-secret handling, and no-adapter failures are tested. No physical device is connected in this evidence. | Pharmacy-device print/label byte or visual comparison, scanner-to-line-add timing, drawer pulse, biometric, SMS, and SMTP operator sign-off. See `docs/PHASE_U_HARDWARE_EVIDENCE.md`. |
| Scale and soak | Bounded probes are green at 25,000 stock rows / 10,000 GL journals. The full-volume disposable attempt failed during seeding when PostgreSQL terminated the connection; 3.2M stock rows, 1.04M GL rows, document-post latency, and the eight-hour soak remain unverified. | Re-run only on a provisioned disposable PostgreSQL instance with measured capacity, then execute the read-only soak. See `docs/PHASE_W_PERFORMANCE_EVIDENCE_2026-08-07.md`. |
| Operational acceptance | Local health, RLS probes, authentication, scoped writes, and rollback-safe local supervisors are proven. A parallel trading day, final reconciliation, cutover, and 48-hour rollback rehearsal are not. | Pharmacy operator UAT, cutover rehearsal, final incremental import, rollback rehearsal, and signed go/no-go record. |

## Smallest next decision

The code and local verification gates require no further approval. To reach
accepted replacement status, the owner must authorize the remaining reviewed
canonical import window and provide the real-device/UAT boundary (or explicitly
sign off those items as deferred). Until then this task is **not accepted as
100% legacy parity**, even though the current implementation and automated test
gates are green.

## Purchase return multi-batch allocation follow-up - 2026-08-07

The purchase-return screen now exposes explicit source-batch allocation rows
with selectable active inventory batches, editable quantities, duplicate-batch
prevention, and exact allocation-total validation. The canonical command now
serializes every selected `{ batchId, batchNumber, quantity }` allocation while
retaining the legacy Batch and Source batch ID fields as a compatibility path.
The focused browser regression was added but not executed in this quick-check
slice; `pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings,
`pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts --list`
discovered 9 contracts, and `go test ./services/api/internal/httpapi -run
'Test(Purchase|PurchaseReturn)' -count=1` passed.
Exact source-document batch reconciliation, PowerBuilder raster/focus parity,
and real operator UAT remain open.

## Purchase return source-line identity follow-up - 2026-08-07

Posted purchase returns now carry the canonical `sourceLineId` from the source
purchase line. The API verifies that line belongs to the selected source
purchase and item, counts prior returns against that exact line, and requires
each selected batch to have been received by that source line. The return UI
also exposes the source purchase line ID for manual entry and populates it when
the captured Populate Purchase Return workflow loads a canonical purchase.

Focused evidence: `go test ./services/api/internal/httpapi -run 'TestPurchase'
-count=1`, `cmd /c pnpm --filter @abuzar/web check`, focused Playwright
discovery for both purchase-return contracts, and `git diff --check` all passed.
The browser assertions and database-backed integration require a separate
runtime acceptance run; no long build or CI flow was performed.

## Unified party-ledger running balance follow-up - 2026-08-07

The authenticated party-ledger query now calculates `balanceAfter` with a
tenant/branch/party-scoped debit-minus-credit window over canonical ledger
entries plus retained payment, adjustment, and return-allocation rows. This
removes blank running balances from mixed historical statements. Focused
finance tests passed without a database connection; exact legacy opening,
same-timestamp ordering, settlement, and source reconciliation remain open.

## Withholding-tax source projection follow-up - 2026-08-07

The captured Purchase Reports `Withholding Tax Deduction` leaf now has a
separate `historical_withholding_tax_entries` target and a guarded
`-wave withholding` loader over `dbo.PurPayment`. The projection retains the
payment-level posted flag, purchase-invoice/supplier identity, withholding
base/rate/amount, account, check/reference, remarks, user, and raw payload.
The API is posted-only, non-zero amount, tenant/branch/date/text scoped and
returns a typed six-field contract. Purchase-line `AdvanceTaxAmt` is not
reclassified as withholding.

Focused evidence: `go test ./migration/cmd/bulk-historical -count=1`, the
focused Phase Q report/migration tests, `cmd /c pnpm --filter @abuzar/web check`,
and `git diff --check` passed. No SQL Server import, live database result,
long build, CI flow, or broad browser suite was run. Exact source counts,
grouping, supplier/certificate semantics, print/PDF/workbook output, and UAT
remain acceptance evidence.

## Receivables-aging source follow-up - 2026-08-07

The captured Receivables Aging alias now retains SaleLedger DueDate in the
historical business-document payload and uses it when posted customer
party-ledger entries are available. Payables Aging now retains Purledger
CreditDays and derives a bounded purchase due date from the source purchase
date. Both expose NOT DUE, 0-30, 31-60, 61-90, and 91+ day buckets; missing or
invalid source terms remain explicitly unaged.

Focused API/source, Svelte, Playwright-discovery, and whitespace evidence is
recorded in docs/PHASE_Q_RECEIVABLES_AGING_EVIDENCE_2026-08-07.md. Historical
import/count reconciliation, payment allocation, exact legacy bucket/date
semantics, print/PDF/workbook output, database replay, and UAT remain open.

New canonical credit-sale drafts now expose and validate a SaleLedger-compatible
Due Date, retain it in the document pricing snapshot, and use it as the
Receivables Aging fallback when no historical payload exists. The focused
credit-sale browser contract passed; source reconciliation, payment allocation,
and exact legacy aging semantics remain open in the same Phase Q evidence.

Receivables and Payables Aging now aggregate retained business-document
`balance_amount` for posted debit/credit entries, so fully paid migrated
documents are not reported as open solely from their original invoice total.
Customer and supplier statements now also expose posted source-backed
`PurPayment`, `InstallmentReceiptDetail`, and direct SaleLedger/Purledger
payment snapshots. The payment rows are not yet replayed as canonical invoice
allocations, and exact legacy bucket, credit-note, adjustment, and print
semantics remain open in the Phase Q and party-payment evidence.

## Party payment allocation source projection follow-up - 2026-08-07

The guarded `bulk-historical -wave payments` path now preserves supplier
`dbo.PurPayment`, customer `dbo.InstallmentReceiptDetail`, and invoice-level
direct-payment fallbacks from `dbo.SaleLedger` and `dbo.Purledger` in
`historical_party_payment_allocations`. It keeps source identity and raw
payload even when canonical party/document lookups are unresolved, and unions
posted non-zero rows into Customer Statement, Supplier Statement, and the
authenticated finance-ledger API. The captured menu has no payment-entry leaf,
so no unsupported top-level UI command was invented.

`dbo.SaleReceivableAdj` is now retained separately in
`historical_party_ledger_adjustments`; posted dated non-zero debit/credit rows
are exposed in customer statements and the finance-ledger API with an explicit
`receivable-adjustment` source type. It is not counted as a payment receipt.

The reviewed `SRAllocationHeader/Detail` and `PRAllocationHeader/Detail` rows
are now retained in `historical_party_return_allocations` by the guarded
`-wave return-allocations` importer. Bounded customer/supplier statement and
finance-ledger rows carry `return-allocation`; aging and canonical document
balances do not consume these rows until source semantics are reconciled.

Party statements and aging also accept posted historical business documents
with a non-empty legacy source table when no canonical GL journal exists; newly
posted documents retain the posted-journal gate. This preserves imported
party-ledger visibility without fabricating a VirtualGl-to-GL conversion.

Trial Balance now aggregates posted canonical GL lines with imported
`historical_gl_entries`, resolving matching account codes and labeling
unmatched legacy accounts as Historical. Opening balances, fiscal periods, and
exact legacy grouping remain acceptance evidence.

Focused Go tests, Svelte checks, OpenAPI parsing, and whitespace checks are the
short verification boundary. SQL Server import/count reconciliation, source
amount semantics, invoice allocation, `SaleReceivableAdj`/return-allocation
semantics, canonical interactive settlement, print output, and UAT remain
acceptance evidence in `docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.

## Purchase credit-term workflow follow-up - 2026-08-07

Pack, loose, and opening purchase drafts now expose bounded Credit Days,
preserve the value through canonical save/post and history hydration, and
include it in the server pricing snapshot used by Payables Aging. Imported
Purledger CreditDays remains the historical fallback; purchase-order documents
and non-purchase documents cannot submit the field. The purchase quick-search
Lookup hitbox was also constrained to its legacy grid cell so the canonical
lookup action is reachable by pointer and the focused receipt contract can
assert the posted command payload.

Focused evidence is recorded in
docs/PHASE_I_PURCHASE_CREDIT_TERMS_EVIDENCE_2026-08-07.md. Exact legacy batch
and due-date semantics, source reconciliation, supplier payment allocation,
print/PDF/workbook output, database replay, and UAT remain open.

## Advance-tax report source follow-up - 2026-08-07

The captured Customer Wise Advance Tax and Supplier Wise Advance Income Tax
leaves now use the explicit per-line pricing.taxes entry whose kind is
advance_tax. The customer leaf is scoped to posted sales and the supplier
leaf to posted purchase kinds. A numeric business_documents
legacy_payload.AdvanceTaxAmt is only a guarded first-line fallback;
the aggregate line tax_amount is not treated as advance tax, and rows without
positive amount evidence are omitted. The API remains posted-only and
tenant/branch/date/text/pagination scoped, and the web note discloses the
source boundary.

Focused evidence is recorded in
docs/PHASE_Q_ADVANCE_TAX_EVIDENCE_2026-08-07.md. Source import/count
reconciliation, exact grouping/rate/base/rounding semantics, print/PDF/workbook
output, database-backed replay, and operator UAT remain open.

## Sales history canonical hydration follow-up - 2026-08-07

Selecting a sales history row with a canonical `documentId` now fetches the
canonical document and restores its invoice/date/party/godown/source fields,
all canonical lines, source sale-line identity, pricing values, and retained
batch allocations. The API identity projection covers sales, returns,
quotations, and refused-sales history; rows without a document ID still use
the explicitly labelled compatibility history-summary fallback.

Focused evidence: `go test ./services/api/internal/httpapi -run
'Test(SalesReadModel|SaleReturnReadModel|SalesHistoryQueries|CanonicalPurchaseHistory)'
-count=1`, `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0
warnings; the new `sales-canonical.spec.ts` contract was discovered as one
test; and `git diff --check` passed with only existing Windows line-ending
warnings. Browser execution, live API/database data, exact PowerBuilder list
navigation/focus/raster parity, and operator UAT remain open.

## Sale-detail import path follow-up - 2026-08-07

The high-volume canonical `dbo.Saledetail` slice now has a dedicated guarded
`migration/cmd/bulksalelines` COPY/set-based importer. It preserves the
reviewed source identity, raw line payload, pack/loose quantity expression,
pricing/tax/expiry/batch fields, scoped `SaleLedger` and item dependencies,
restart-safe target conflict key, and auditable mapping/exception rows.
`go test ./migration/cmd/bulksalelines -count=1` and the focused generic
historical-map tests passed without a database connection. The command was not
executed in this pass: approved source reconciliation, missing/invalid-row
bookkeeping, complete sale/return line promotion, exact legacy calculations,
report/print parity, and UAT remain acceptance gates. See
`docs/PHASE_E_SALE_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

## Migration bookkeeping precision follow-up - 2026-08-07

The read-only reconciliation report now separates raw open migration-exception
rows from distinct open source cases grouped by source schema/table, legacy ID,
and reason. Acceptance status is based on distinct unresolved cases while raw
rows remain visible for audit, preventing superseded retry rows from inflating
the decision signal. `go test ./migration/cmd/reconcile -count=1` passed without
opening either database. This improves evidence quality but does not resolve
any live source exception; an approved reconciliation run remains required.
See `docs/PHASE_E_MIGRATION_BOOKKEEPING_EVIDENCE_2026-08-07.md`.

## Party-statement source-scope follow-up - 2026-08-07

Customer and supplier statement metadata now explicitly discloses the
`SRAllocationDetail`/`PRAllocationDetail` return-allocation sources that the
finance read model already unions. The focused API query and definition tests
passed. This is a contract/evidence correction, not a claim of exact legacy
settlement, grouping, print, reconciliation, or UAT parity. See
`docs/PHASE_Q_PARTY_STATEMENT_SCOPE_EVIDENCE_2026-08-07.md`.

## Purchase-order line import path follow-up - 2026-08-07

The high-volume reviewed `dbo.PurOrderDetail` slice now has a dedicated
guarded `migration/cmd/bulkorderlines` COPY/set-based importer. It preserves
the reviewed line identity, source order fields/payload, scoped
`PurOrderHeader`/item dependencies, restart-safe conflict key, and auditable
mapping/exception rows. `go test ./migration/cmd/bulkorderlines -count=1`
passed without a database connection. Source execution, order count/quantity/
amount reconciliation, exact legacy calculations, report/print parity, and UAT
remain acceptance gates. See
`docs/PHASE_E_PURCHASE_ORDER_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

## Return-line import path follow-up - 2026-08-07

The reviewed sale-return and purchase-return detail slices now have one guarded
`migration/cmd/bulkreturnlines` path with fixed `sale`/`purchase` modes. Sale
mode reads `dbo.SRdetail` into `cash-sale-return` lines through `SRLedger`;
purchase mode reads `dbo.PRdetail` into `purchase-return` lines through
`PRLedger`. Both preserve the reviewed line identity and source payload,
perform scoped COPY/set-based upserts, and retain invalid/dependency rows as
audited mappings and exceptions. Both modes now accept deterministic
`-from-row`/`-to-row` windows and record the source window in their reports, so
the deferred waves can be retried in bounded slices. Focused package tests and
vet passed without database execution. Canonical source execution, return
count/quantity/amount reconciliation, exact legacy calculations, stock/ledger
effects, print parity, and UAT remain acceptance gates. See
`docs/PHASE_E_RETURN_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

## Transaction-history filter follow-up - 2026-08-07

Sales List and Purchase List now expose the existing authenticated transaction
history filter through a legacy-style `Filter:` input and `Filter / Retrieve`
button. Enter submits the same request, and the trimmed value is sent to the
active `/v1/transactions/{kind}` query. `cmd /c pnpm --filter @abuzar/web
check` passed with 0 errors and 0 warnings; focused `git diff --check` passed
with only expected Windows line-ending warnings. Exact PowerBuilder wildcard,
sorting, focus, raster, live-data, and operator-UAT behavior remain open. See
`docs/PHASE_I_TRANSACTION_HISTORY_FILTER_EVIDENCE_2026-08-07.md`.

## Purchase-label edge workflow follow-up - 2026-08-07

Purchase `Print Purchase Labels` now submits typed label rows to the existing
edge ESC/POS route and falls back to browser preview when the adapter is
unavailable. The focused Svelte check passed with 0 errors and 0 warnings.
Physical printer connection, label geometry, legacy byte comparison, and
operator acceptance remain open; see `docs/PHASE_U_HARDWARE_EVIDENCE.md`.

## Web client contract and master CRUD follow-up - 2026-08-07

The authenticated web client now exposes the canonical document-detail,
master-delete, report-format, and preference-field-key contracts required by
the current workflow surfaces. `cmd /c pnpm --filter @abuzar/web check` passed
with 0 errors and 0 warnings. Focused
`cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-f.spec.ts
--workers=1 --retries=0 --reporter=line --grep "empty canonical masters"`
passed 1/1, covering the master List/Detail confirmed-delete request.
Exact source validation, full catalog data reconciliation, PowerBuilder
wildcard/focus/raster parity, physical hardware, and operator UAT remain
acceptance gates.

## Purchase Populate command browser refresh - 2026-08-07

The purchase surface now keeps pointer-based baseline activation from
interrupting the contextual File menu after Quick Search focus, and the
focused browser fixture waits for the hydrated menu boundary. The focused
command `cmd /c pnpm --filter @abuzar/web exec playwright test
tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line
--timeout=12000 --global-timeout=30000 --grep "Populate"` passed 2/2 for
canonical Populate Items and Populate From Sale Template. The Svelte check
passed with 0 errors and 0 warnings. Exact PowerBuilder source/template
selection, price/tax/discount side effects, live database replay, and raster
or operator acceptance remain open; see
`docs/PHASE_I_PURCHASE_ITEM_POPULATION_EVIDENCE_2026-08-07.md`.

## Adjacent canonical sales-history regression - 2026-08-07

Focused `cmd /c pnpm --filter @abuzar/web exec playwright test
tests/sales-canonical.spec.ts --workers=1 --retries=0 --reporter=line
--timeout=10000 --global-timeout=15000 --grep "sales history hydrates"`
passed 1/1. This confirms canonical sale-return history still hydrates source
identity and saved batch allocations after the focused purchase/menu changes;
full sales lifecycle and exact PowerBuilder/UAT parity remain open.

## Open sale-return workflow regression - 2026-08-07

Focused `cmd /c pnpm --filter @abuzar/web exec playwright test
tests/phase-cd.spec.ts --workers=1 --retries=0 --reporter=line
--timeout=10000 --global-timeout=15000 --grep "open cash sale return"`
passed 1/1. The canonical open cash return posted without a source invoice,
retained godown, batch, expiry, unit cost, and noon-normalized transaction date
fields. Exact PowerBuilder dialog/raster behavior, live database effects, and
operator UAT remain open.

## Stock allocation parity guard - 2026-08-07

The stock engine now rejects `ABUZAR_STOCK_ALLOCATION_POLICY=legacy` until the
source `StockReport` ordering and valuation rules are reconciled; it no longer
records a legacy label while executing FIFO. The focused Go command
`cmd /c go test ./services/api/internal/httpapi -run TestStock -count=1`
passed. This is a safety guard, not a legacy-parity claim; source ordering,
valuation/COGS, full-volume reconciliation, and trading-day acceptance remain
open. See `docs/PHASE_J_STOCK_EVIDENCE_2026-08-06.md`.

## Historical StockReport lineage guard - 2026-08-07

`migration/cmd/bulk-historical` now preserves the reviewed StockReport
composite source identity `(Date, GCode, ICode)` in `source_legacy_id` and
rejects a staging batch when duplicate composite identities would be collapsed
by the target upsert. The reviewed map was updated to use the same derived
identity. Focused `cmd /c go test ./migration/cmd/bulk-historical -count=1`
passed, and `phase-e-stock-finance.json` parsed successfully. This improves
provenance safety but does not prove the live canonical import, source totals,
valuation, or full StockReport parity.

## Historical VirtualGl lineage guard - 2026-08-07

`migration/cmd/bulk-historical` now retains the reviewed VirtualGl identity
`(DocumentCode, VRow, AccCode)` through a named helper and rejects duplicate
identities in a staging batch before the historical GL upsert. This prevents
the known duplicate VirtualGl source rows from being silently overwritten.
Focused `cmd /c go test ./migration/cmd/bulk-historical -count=1` passed.
Duplicate quarantine, source/target totals, account mapping, opening balances,
and exact PowerBuilder GL semantics remain acceptance gates.

## Canonical maintenance item lookup workflow - 2026-08-07

The shared maintenance workflow now exposes active canonical item lookup for
the item-scoped price, discount, basic-data, and reorder-quantity commands.
Selecting a result writes the reviewed legacy item identity before the
mutation is submitted. The workflow surface also exposes a hydration marker so
browser actions cannot be dispatched against its server-rendered shell before
the Svelte handlers are ready.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "canonical item maintenance"` passed 1/1.
- The browser fixture verified active lookup selection and the canonical
  `change-items-price` request payload (`itemCode=ITEM-1`, `priceType=Sale Price`,
  numeric `price=12.5`) before the success response was displayed.

This proves the representative web workflow against mocked authenticated APIs;
exact PowerBuilder dialogs, price/discount calculation rules, source item
selection, live database effects, print/raster parity, and operator UAT remain
open.

## Update Item Suppliers canonical workflow - 2026-08-07

`Maintenance > Update Item Suppliers` now resolves both the item and supplier
against active canonical tenant data before saving. The Go mutation validates
the supplier scope, upserts only the selected item/supplier link, preserves
other links, and records previous/new priority and purchase-rate values in the
maintenance audit payload.

Focused evidence:

- `cmd /c go test ./services/api/internal/httpapi -run TestCanonicalItemMaintenanceValidation -count=1` passed.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "canonical item supplier maintenance"` passed 1/1.

The PostgreSQL integration case was not executed because `DATABASE_URL` was not
configured in this run. Exact PowerBuilder supplier-selection semantics,
source reconciliation, and operator acceptance remain open.

## Item Purchase History identity filter - 2026-08-07

`File > Item Purchase History` now derives the first populated purchase-row
item identity and forwards it as the authenticated history filter. The
canonical purchase-history query matches both `business_document_lines.item_legacy_id`
and the displayed item name, while retaining tenant, branch, document-kind,
and date scope.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c go test ./services/api/internal/httpapi -run TestCanonicalPurchaseHistoryQuerySupportsItemIdentityFiltering -count=1` passed.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "Item Purchase History filters"` passed 1/1.

The browser assertion used mocked authenticated APIs; live PostgreSQL history
results, exact PowerBuilder cursor/focus behavior, full purchase workflow
semantics, and print/UAT parity remain open.

## Purchase View Item Info identity handoff - 2026-08-07

`File > View Item Info` now fails closed unless a populated row has an active
canonical item identity, then opens Item master with the item legacy ID. Item
master consumes that identity and preselects the matching tenant-scoped item,
including its canonical detail/supplier load.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "View Item Info carries"` passed 1/1.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-f.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "preselects the canonical item"` passed 1/1.

These are mocked authenticated browser checks. Exact PowerBuilder current-row
focus behavior, live source data, and full master-screen raster/CRUD parity
remain open.

The adjacent `File > Supplier Info.` command now carries the active canonical
supplier legacy ID to Supplier master and uses the same preselection contract;
its focused browser coverage is included in the purchase-canonical and Phase F
suites.

## Phase E reviewed-map coverage auditor - 2026-08-07

Added the read-only `migration/cmd/auditcoverage` command. It compares the
authoritative `tmp/canonical-sqlserver-schema.json` table inventory with every
JSON map in `migration/maps`, normalizes schema/table casing, reports mapping
overlaps, and can fail closed with `-fail-on-unmapped`. It does not open either
database or alter source/mapping files.

Fresh artifact:

- `parity/catalog/phase-e-map-coverage-2026-08-07.json`
- Manifest tables: 763
- Unique mapped tables: 49
- Unmapped tables: 714
- Reviewed mapping entries: 74
- Overlapping mapping entries: 25

Focused evidence:

- `cmd /c go test ./migration/cmd/auditcoverage -count=1` passed.
- `cmd /c go run ./migration/cmd/auditcoverage -manifest tmp/canonical-sqlserver-schema.json -maps migration/maps -out parity/catalog/phase-e-map-coverage-2026-08-07.json` completed and emitted the counts above.

This is an audit/control improvement, not a migration completion claim. The
714 unmapped tables, canonical source execution, 32 canonical Purdetail
quarantines, 16 sandbox tax ambiguities, business reconciliation, and all
downstream acceptance gates remain open.

## Sales contextual item and supplier identity handoffs - 2026-08-07

The captured cash-sale menu now handles `File > View Item Info` and
`File > Supplier Info.` through canonical identity. Item Info requires the
focused populated canonical item row and opens Item master with its legacy ID.
Supplier Info loads the focused item's canonical supplier links, resolves an
active tenant-scoped supplier, and opens Supplier master with that supplier's
legacy ID. Both actions fail closed when the row or linked supplier cannot be
trusted; the sales menu access fixture also exercises the tenant-admin access
boundary.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts -g "View Item Info|Supplier Info" --workers=1 --retries=0 --reporter=line` passed 3/3, including a second-line focus regression.

These are mocked authenticated browser checks. Exact PowerBuilder focus-ring
and keyboard traversal semantics, supplier-choice ordering, live database data,
and full sales-screen raster/print/UAT parity remain open.

## Transaction current-row focus fidelity - 2026-08-07

Sales and purchase transaction surfaces now retain the focused grid row for
contextual actions instead of implicitly choosing the first populated line.
Sales `View Item Info`, `Supplier Info.`, and `Item Sale History`, plus
purchase `Item Purchase History` and `View Item Info`, resolve the focused row;
loading a document, creating a new document, adding/removing rows, and
keyboard focus keep that index bounded.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts -g "View Item Info|Supplier Info" --workers=1 --retries=0 --reporter=line` passed 3/3, including the focused second-line assertion.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts -g "Populate Items|Item Purchase History|View Item Info|Supplier Info" --workers=1 --retries=0 --reporter=line` passed 4/4.

Exact PowerBuilder focus-ring, accelerator, tab-order, and multi-window cursor
semantics remain open, as do live data and full raster/UAT acceptance.

## Sales history line identity and contextual menu correction - 2026-08-07

Sales history now carries `business_document_lines.item_legacy_id` through the
canonical/compatibility read model and includes it in the scoped filter. This
keeps `File > Item Sale History` useful when the displayed item name differs
from the legacy code. The cash-sale menu layer also corrects the two known
purchase captions present in the captured cash-sale menu artifact to the sales
commands handled by the screen: `Item Sale History` and `Sale Slip`.

Focused evidence:

- `cmd /c go test ./services/api/internal/httpapi -run TestCanonicalSalesHistoryQuerySupportsItemIdentityFiltering -count=1` passed.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-cd.spec.ts -g "active-window menu swaps" --workers=1 --retries=0 --reporter=line` passed 1/1.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts -g "Item Sale History" --workers=1 --retries=0 --reporter=line` passed 1/1 and verified `filter=ITEM-2` from the focused second row.

The SQL assertion and browser flow use mocked APIs; live PostgreSQL result
verification, exact PowerBuilder sales-menu capture/caption confirmation,
cursor/accelerator behavior, print output, and full sales/UAT parity remain
open.

## Purchase Edit Purchase Order contextual workflow - 2026-08-07

The captured purchase File menu now handles `Edit Purchase Order` with an
explicit route-kind guard. From Pack Purchase and the other purchase windows,
it opens the canonical Purchase Order editor. When already in the Purchase
Order window, it opens the scoped canonical purchase-order history so the
operator can select an existing order for editing instead of accidentally
posting the current window as another document kind.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "Edit Purchase Order"` passed 2/2.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.

The browser checks use mocked authenticated APIs and prove both route branches;
canonical live order retrieval, exact PowerBuilder edit/delete/locking
semantics, print/preview parity, and operator acceptance remain open.

## Item-master contextual CRUD/navigation handoff - 2026-08-07

The item-master window now owns the captured contextual `File` and `Item`
commands that correspond to its existing canonical editor state: New, List,
Save, Delete, First, Previous, Next, Last, New Item, Delete Item, and Exit.
These commands no longer fall through to the generic legacy
workbench; they reuse the tenant-scoped master CRUD, list, and record-navigation
functions already used by the toolbar and form.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-f.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "item master captured File commands"` passed 1/1.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.

The browser check uses mocked authenticated master APIs and covers New/List;
live CRUD persistence, exact PowerBuilder command ordering/accelerators,
item-specific auxiliary dialogs, the captured Restore Item semantics, raster
parity, and operator acceptance remain open.

## Groups contextual File command handoff - 2026-08-07

The Groups window now handles the captured contextual File commands backed by
its existing canonical role editor: New, Detail, Save, First, Previous, Next,
Last, Print, and Exit. The handler is limited to the Groups window and leaves
the captured Delete command unclaimed because the current role API has no
reviewed delete/restore contract.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-r.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "Groups captured File commands"` passed 1/1.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.

The browser check uses mocked authenticated role APIs and covers New/Detail;
live role persistence, exact PowerBuilder group navigation/accelerators,
delete/restore semantics, raster parity, and operator acceptance remain open.

## Captured File Delete draft lifecycle - 2026-08-07

The captured `File > Delete` command is now wired in the sales and purchase
transaction contexts. Existing canonical documents use an idempotent,
tenant/branch/kind-scoped soft-delete command; only `draft` documents can be
deleted. The row remains available to the audit/revision/sync history, while
normal document detail and canonical purchase history exclude deleted drafts.
Posted and void documents remain protected and must use their existing void
semantics instead of hard deletion.

Focused evidence:

- `cmd /c go test ./services/api/internal/httpapi -run TestDocumentCommandValidationCoversLifecycleAndRevisionRequirements -count=1` passed.
- `cmd /c go test ./services/api/internal/httpapi -run TestBusinessDocumentDraftDeleteMigrationIsNonDestructive -count=1` passed.
- `cmd /c go test ./services/api/internal/httpapi -run TestCanonicalPurchaseHistoryQuerySupportsItemIdentityFiltering -count=1` passed.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "File Delete"` passed 2/2.
- `git diff --check` passed; only normal LF/CRLF checkout notices were emitted.

The browser checks use mocked authenticated APIs and prove both menu-to-command
payloads. Live PostgreSQL delete/replay, the exact PowerBuilder confirmation and
deleted-draft display, posted-delete rejection against real projections, full
report/history parity, raster comparison, hardware/UAT, full-volume soak, and
cutover/rollback evidence remain open.

## Report dialog keyboard dismissal - 2026-08-07

Report retrieval/format dialogs and the print-preview window now close with the
legacy Escape interaction. The dialog state is reset together and the report
footer records the dismissal, so an operator is not left behind a stale modal
surface.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "report dialogs and print preview close with Escape"` passed 1/1.

The check covers browser interaction with mocked session/access responses;
PowerBuilder raster/focus comparison for every report dialog, exact report
output, physical print/PDF/Excel, and operator acceptance remain open.

## Apply Item GST canonical assignment - 2026-08-07

The captured `File > Apply Item GST %` command now uses the canonical
tenant/branch-scoped tax-assignment endpoint in both sales and purchase
windows. The command requires `tax.write`, sends the selected canonical item
UUIDs and transaction date, and updates draft line metadata only after the
assignment succeeds. Empty or free-text rows fail closed; an omitted `itemIds`
array is never sent because the API intentionally interprets that as every
active item.

Focused evidence:

- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts tests/purchase-canonical.spec.ts --workers=1 --retries=0 --reporter=line --timeout=12000 --global-timeout=30000 --grep "Apply Item GST"` passed 2/2.
- `git diff --check` passed; only normal LF/CRLF checkout notices were emitted.

The browser checks use mocked authenticated APIs and prove the two
menu-to-canonical-request paths. Exact legacy inclusive/exclusive tax
semantics, live PostgreSQL assignment/reconciliation against the captured
configuration, historical tax output, raster comparison, and operator UAT
remain open.

## Resumable canonical sales-line migration window - 2026-08-07

The guarded `migration/cmd/bulksalelines` path now supports deterministic
zero-based `-from-row`/`-to-row` windows. Each bounded report records its source
window, while the source query orders by invoice, numeric/text row identity,
and item before applying SQL Server `OFFSET`/`FETCH`. This lets the deferred
high-volume `Saledetail` wave be retried in reviewed slices without claiming a
full source import.

Focused evidence:

- `cmd /c go test ./migration/cmd/bulksalelines -count=1` passed.
- `git diff --check` passed; only normal LF/CRLF checkout notices were emitted.

No source or target database import was run in this short pass. Full
`Saledetail` execution, source/target reconciliation, exception closure,
historical document promotion, exact PowerBuilder calculations, and UAT remain
open.

The same bounded-window contract is now present in the guarded
`migration/cmd/bulkorderlines` loader for the deferred `PurOrderDetail` wave;
its source execution, reconciliation, exact order semantics, report output,
and UAT remain open.

## Resumable canonical return-line migration windows - 2026-08-07

The guarded `migration/cmd/bulkreturnlines` path now accepts deterministic
zero-based `-from-row`/`-to-row` windows for both fixed `sale` (`SRdetail`) and
`purchase` (`PRdetail`) modes. The source query orders by return identifier,
numeric/text row identity, and item before SQL Server `OFFSET`/`FETCH`, and the
redacted report records the requested window.

Focused evidence:

- `cmd /c go test ./migration/cmd/bulksalelines ./migration/cmd/bulkorderlines ./migration/cmd/bulkreturnlines -count=1` passed.
- `cmd /c go vet ./migration/cmd/bulkreturnlines` passed.
- `git diff --check` passed; only normal LF/CRLF checkout notices were emitted.

No return-line source or target import was run. Return count/quantity/amount
reconciliation, exception closure, stock/ledger effects, exact PowerBuilder
calculations, report/print parity, and operator UAT remain open.

## Resumable canonical purchase-line migration window - 2026-08-07

The guarded `migration/cmd/bulkpurchaselines` path for the deferred 113k-row
`dbo.Purdetail` wave now accepts deterministic zero-based `-from-row` and
exclusive `-to-row` windows. The source query orders by purchase invoice,
numeric/text row identity, and item before SQL Server `OFFSET`/`FETCH`, and the
redacted report records the requested window.

Focused evidence:

- `cmd /c go test ./migration/cmd/bulkpurchaselines -count=1` passed.
- `cmd /c go vet ./migration/cmd/bulkpurchaselines` passed.
- `git diff --check` passed; only normal LF/CRLF checkout notices were emitted.

No purchase-line source or target import was run. Full `Purdetail` execution,
source/target reconciliation, 32 known non-positive-quantity exceptions,
historical promotion, exact PowerBuilder calculations, report/print parity,
and operator UAT remain open.

## Item Form alternate-alias workflow - 2026-08-07

The captured Item Form `File > Set Alternate Item Alias Names` command now
opens a tenant-scoped editor for the selected canonical item. It uses a
separate `alternate_alias` kind in `master_aliases`, so replacing alternate
names does not delete the primary `AliasName`/`CustomICode` or barcode lookup
rows. The API bounds and normalizes the list, rejects blank or duplicate
values, detects cross-item conflicts, and retains the edited list in
`payload.AlternateItemAliases`.

Focused evidence:

- `go test ./internal/httpapi -run 'Test(NormalizeAlternateItemAliasesKeepsOrderAndRejectsAmbiguity|CanonicalMasterRoutesRemainAuthenticated|NormalizedMasterMigrationRetainsLegacyUniquenessAndSupplierFields)$' -count=1` passed.
- `go vet ./internal/httpapi` passed.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "alternate-alias" --project=chromium --retries=0 --timeout=15000 --reporter=line` passed 1/1.
- `cmd /c pnpm --dir apps/web check` passed with 0 errors and 0 warnings.
- `git diff --check` passed with only normal LF/CRLF checkout notices.

This is an implementation slice, not full acceptance. The migration has not
been applied to a reviewed PostgreSQL instance in this short pass; live RLS,
cross-item conflict, primary-alias preservation, exact PowerBuilder dialog
geometry, source price/blob semantics, remaining Item Form commands, and
operator UAT remain open. See `docs/PHASE_F_ITEM_ALIAS_EVIDENCE_2026-08-07.md`.

## Item Form image workflow - 2026-08-07

The captured `File > Set Item Image(s)` command now uses a canonical
tenant-scoped `master_item_images` collection. The target retains the reviewed
`ItemImage` concepts (`rowId`, description, image blob, and type), replaces the
selected item's collection atomically, and exposes bounded base64 GET/PUT
behavior. The Svelte dialog supports local image selection, preview,
description/type edits, removal, and save/cancel behavior.

Focused evidence:

- `go test ./internal/httpapi -run 'Test(NormalizeItemImagesPreservesRowsAndBoundsBlobPayloads|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` passed.
- `go vet ./internal/httpapi` passed.
- `cmd /c pnpm --dir apps/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "ItemImage rows" --project=chromium --retries=0 --timeout=15000 --reporter=line` passed 1/1.
- `git diff --check` passed with only normal LF/CRLF checkout notices.

This is not full acceptance. Migration application, historical `dbo.ItemImage`
import/reconciliation, RLS/blob constraint proof, exact PowerBuilder dialog
geometry and source encoding semantics, remaining Item Form commands, and
operator UAT remain open. See `docs/PHASE_F_ITEM_IMAGE_EVIDENCE_2026-08-07.md`.

## Item Form notes workflow - 2026-08-07

The captured `File > Set Item Notes` command now has a tenant-scoped canonical
`master_item_notes` byte store, authenticated GET/PUT API, OpenAPI contract,
and a legacy-styled Item Form dialog. The API preserves UTF-8, RTF, and opaque
legacy bytes through base64 and bounds decoded input to 8 MiB. Focused Go,
vet, Svelte, and browser checks passed. See
[`docs/PHASE_F_ITEM_NOTES_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_NOTES_EVIDENCE_2026-08-07.md).

The live `dbo.ItemNotes` extraction/reconciliation and approved PowerBuilder
rich-text/editor raster, keyboard/focus, and source-encoding comparison remain
unverified; this slice does not close the full Item Form or overall parity
acceptance gates.

## Item Form association workflow - 2026-08-07

The captured `File > Set Item Associations` command now has a tenant-scoped
canonical `master_item_associations` relation, authenticated GET/PUT API,
OpenAPI contract, and a legacy-styled Item Form add/remove editor. It retains
the reviewed `ItemAssociation(ICode, AssocICode)` identities, validates
same-tenant targets, rejects self-links and duplicate identifiers, and passed
focused Go, vet, Svelte, and browser checks. See
[`docs/PHASE_F_ITEM_ASSOCIATIONS_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_ASSOCIATIONS_EVIDENCE_2026-08-07.md).

Live source extraction/reconciliation and approved PowerBuilder dialog raster,
keyboard/focus, and dependency-behavior comparison remain unverified; this
slice does not close the full Item Form or overall parity acceptance gates.

## Item Form author workflow - 2026-08-07

The captured `File > Set Item Author(s)` command now has a tenant-scoped
canonical `master_item_authors` collection, authenticated GET/PUT API,
OpenAPI contract, and a legacy-styled Item Form editor. It preserves the
reviewed `ItemAuthor(ICode, AuthorCode, Priority, ROWID)` relationship fields,
validates bounded unique rows, and passed focused Go, vet, Svelte, and browser
checks. See
[`docs/PHASE_F_ITEM_AUTHOR_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_AUTHOR_EVIDENCE_2026-08-07.md).

Live `dbo.Author`/`dbo.ItemAuthor` extraction and reconciliation, exact author
selection semantics, and approved PowerBuilder picker raster/keyboard/focus
comparison remain unverified; this slice does not close the full Item Form or
overall parity acceptance gates.

## Item Form model workflow - 2026-08-07

The captured `File > Select Models` command now has a tenant-scoped canonical
`master_item_models` collection, authenticated GET/PUT API, OpenAPI contract,
and a legacy-styled Item Form add/remove editor. It preserves the reviewed
`ItemInModel(ICode, ModelCode)` membership codes, validates the captured
smallint range and duplicates, and passed focused Go, vet, Svelte, and browser
checks. See
[`docs/PHASE_F_ITEM_MODEL_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_MODEL_EVIDENCE_2026-08-07.md).

Live `dbo.Model`/`dbo.ItemInModel` extraction and reconciliation, exact model
picker semantics, and approved PowerBuilder picker raster/keyboard/focus
comparison remain unverified; this slice does not close the full Item Form or
overall parity acceptance gates.

## Item Form price-policy workflow - 2026-08-07

The captured `File > Set Item Price Policy` command now reads and atomically
replaces source-backed `PricePolicyDetail` tiers for the selected item through
an authenticated tenant-scoped API and legacy-styled editor. Exact decimal
text, quantity limits, expiry dates, flat discounts, and discount percentages
are retained in the bounded contract. Focused Go, vet, Svelte, and browser
checks passed. See
[`docs/PHASE_F_ITEM_PRICE_POLICY_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_PRICE_POLICY_EVIDENCE_2026-08-07.md).

Exact PowerBuilder picker geometry and keyboard/focus behavior, expiry-date
semantics, `PriceTypeCode`/`Module` mapping, customer/group assignment and
enforcement, complete policy promotion, and 50-invoice golden replay remain
unverified; this slice does not close the overall pricing or legacy acceptance
gates.

## Item Form registration-request workflow - 2026-08-07

The captured `File > Populate Item Registration Request` command now records a
tenant-scoped, source-shaped `ItemRegRequest` snapshot from the selected
canonical item's full payload. Request identity, date, item identity, sent
state, and request history are typed; focused Go, vet, Svelte, and browser
checks passed. See
[`docs/PHASE_F_ITEM_REGISTRATION_REQUEST_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_REGISTRATION_REQUEST_EVIDENCE_2026-08-07.md).

Live 130-field source extraction/reconciliation, external registration-server
delivery, sent-state protocol, server/machine routing, exact field-by-field
PowerBuilder behavior, and approved dialog raster/keyboard/focus comparison
remain unverified. This slice does not close overall legacy acceptance.

## Item Form populate workflow - 2026-08-07

The captured `File > Populate Item` (`Ctrl+O`) command now opens a
tenant-scoped canonical item lookup and hydrates the existing Item Form from
the selected active result without creating a duplicate. The menu mapping,
lookup dialog, empty/error states, and focused browser checks are recorded in
[`docs/PHASE_F_ITEM_POPULATE_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_POPULATE_EVIDENCE_2026-08-07.md).

Live source lookup/reconciliation, barcode and alternate-alias data coverage,
exact PowerBuilder picker focus/accelerator/raster approval, edge-case UAT,
and scale evidence remain open. This slice does not close overall legacy
acceptance.

## Item Form unposted-transaction report workflow - 2026-08-07

The captured `File > Show Un-Posted Transaction Report` (`Ctrl+F1`) command
now reads tenant/branch-scoped canonical draft lines for the selected Item
through a bounded API and presents them in a legacy-styled report dialog. The
query, index, contracts, OpenAPI, Go checks, and browser evidence are recorded
in [`docs/PHASE_F_ITEM_UNPOSTED_TRANSACTIONS_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_UNPOSTED_TRANSACTIONS_EVIDENCE_2026-08-07.md).

Historical SQL Server buffer/temporary transaction coverage, exact report
geometry and focus behavior, complete transaction-family semantics, migration
application, and scale acceptance remain open. This slice does not close
overall legacy acceptance.

## Item Form model evidence record - 2026-08-07

The model membership slice is recorded in
[`docs/PHASE_F_ITEM_MODEL_EVIDENCE_2026-08-07.md`](PHASE_F_ITEM_MODEL_EVIDENCE_2026-08-07.md).
The focused Go, vet, Svelte, and browser checks passed. Live source
extraction/reconciliation, exact model-picker behavior, visual/keyboard/focus
approval, and the remaining Item Form commands remain acceptance evidence
gaps.

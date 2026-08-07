# Purchase summary read-model evidence - 2026-08-07

## Scope

This is a bounded source-backed projection for the captured Phase O purchase
summary leaves. It does not claim exact PowerBuilder tax, return, profit,
graph, category-join, or print semantics.

## Implemented

- Purchase Summary, Purchase Summary2, Purchase Return Summary, Purchase Order
  Summary, Purchase Order, and Net Purchase Summary use an explicit six-field
  posted-document projection over canonical purchase documents plus
  de-duplicated compatibility events.
- Days Summary groups those rows by calendar day and supplier.
- Periodic Purchases, Monthly Purchase Graph, and Manufacturer Wise Monthly
  Stock Movement group the same rows by calendar month and supplier.
- Category Wise Purchase groups posted purchase lines by item and supplier.
- Purchase Order Supplier Wise, Supplier/Manufacturer Wise G/P, and Supplier
  Purchase Returns Summary group posted document values by supplier.
- All projections retain tenant, branch, posted-only, date, text, canonical
  identity, supplier-ledger, stock-ledger, and compatibility de-duplication
  boundaries.

## Focused verification

    go test ./services/api/internal/httpapi -run 'Test(PurchaseSummaryModesUseExplicitBuckets|PurchaseReadModelUsesCanonicalLedgersPostedFiltersAndPagination|PhaseOReportRegistryCoversCapturedPurchaseLeaves|PhaseOReportRegistryResolvesEveryCapturedPurchasePath)$' -count=1

The Svelte workspace check is also required for the frontend report-mode map:

    cmd /c pnpm --filter @abuzar/web check

No database-backed purchase golden replay is claimed because the current shell
has no `DATABASE_URL` configured.

## Remaining boundary

The original PowerBuilder purchase columns, supplier/manufacturer/category
joins, tax and withholding calculations, return/net semantics, graph rendering,
format variants, print/PDF/workbook output, migrated golden data, and operator
acceptance still require source capture and replay evidence.

# Phase Q - Header Wise Transaction Summary evidence

## Scope

This slice covers the captured `Header Wise Transaction Summary` report. It
is a header-level projection across the canonical transaction families and is
not a claim of exact PowerBuilder totals or print parity.

## Implemented

- Canonical posted `business_documents` are grouped once per header across
  sales, returns, quotations, refused sales, purchases, purchase returns, and
  purchase orders.
- Canonical line quantities are summed while the authoritative document total
  is retained once, avoiding line-level total duplication.
- Posted compatibility events for sale, return, refused-sale, receiving,
  purchase-order, quotation, and inventory aggregates are retained only when
  no posted canonical document matches the event identity or document number.
- Tenant, branch, posted-status, date, text, and bounded pagination filters are
  preserved.
- The Svelte report contract exposes Document, Date, Customer/Supplier,
  Transaction Type, Quantity, and Amount columns.

## Focused verification

```text
go test ./services/api/internal/httpapi -run 'Test(HeaderTransactionReportUsesCanonicalHeadersAndCompatibilityFallback|NoStockDocumentReportsUseCanonicalAndDeduplicatedCompatibilityRows|CapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions|PhaseQRegistryCoversTheMappedRemainingLeaves|PhaseQQueriesArePostedAndScopeBound|PhaseNReportRegistryDefinitionsAndAggregateFilters)$' -count=1
cmd /c pnpm --filter @abuzar/web check
```

Both checks passed. No database-backed route result, full build, CI flow, or
broad browser suite was run in this slice.

## Remaining acceptance evidence

The exact PowerBuilder transaction-type labels, opening-balance treatment,
retrieval arguments, calculated totals, print/PDF/workbook output, migrated
golden replay, and 1936x1048 visual approval remain open.

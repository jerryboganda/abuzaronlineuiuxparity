# Phase P - Stock Management report evidence

## Scope

This slice covers the captured `Reports > Stock Reports > Stock Management
Report` leaf. It does not claim exact PowerBuilder alert/status, valuation,
grouping, or print parity.

## Implemented

- The report registry now selects an explicit `management-summary` mode.
- The API reads tenant/branch-scoped `stock_balances` joined to batches, items,
  and godowns.
- Rows require a posted `stock_ledger` source event and preserve the captured
  date, text, godown, and batch filters.
- Reorder, optimum, and minimum quantities are parsed from the captured item
  payload with the same maintenance-key fallbacks used by the level reports.
- The Svelte report contract exposes the eight returned fields: batch,
  expiry/updated, godown, item, on hand, reorder quantity, optimum quantity,
  and minimum quantity.
- No alert predicate is applied; the source thresholds are shown for the
  eventual legacy status calculation rather than being presented as exact.

## Focused verification

```text
go test ./services/api/internal/httpapi -run 'Test(StockManagementReadModelIncludesThresholdsAndPostedScope|PhasePStockRegistryCoversCapturedLeaves|StockReadModelUsesPostedNormalizedLedgersAndBoundedPagination)$' -count=1
cmd /c pnpm --filter @abuzar/web check
git diff --check
```

All three checks passed for this slice. No database-backed route result was
claimed because `DATABASE_URL` was unavailable in the focused run. No full
build, CI flow, or broad browser suite was run.

## Remaining acceptance evidence

The source-backed projection still needs approved legacy columns/arguments,
alert/status rules, canonical stock migration and reconciliation, sample
number comparison, print/PDF/workbook comparison, and the required 1936x1048
visual acceptance before this leaf can be called parity-complete.

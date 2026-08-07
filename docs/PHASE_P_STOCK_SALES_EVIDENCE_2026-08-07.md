# Phase P - stock and sales evidence

Date: 2026-08-07

This is a bounded implementation slice for the captured `Stock and Sales`
leaf. It does not claim exact legacy sales/stock period semantics or golden
output.

## Implemented

- The leaf now uses a distinct `stock-sales` mode instead of the generic
  balance projection.
- Current normalized `stock_balances` are joined to canonical posted
  cash-sale and credit-sale `stock_allocations`; only the allocation total is
  restricted to the requested date range, so a current balance is not hidden
  merely because its last update predates the report window.
- The report exposes Batch, Expiry/Updated, Godown, Item, On Hand, and Sales
  Qty, with tenant/branch, text, godown, batch, date, and bounded pagination
  filters.
- Compatibility-only sale events and returns are not silently mixed into the
  canonical allocation total.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'Test(StockSalesReadModelJoinsCanonicalPostedSaleAllocations\|StockItemSummaryReadModelAggregatesPostedLedgerByItemDay\|StockLevelReadModelsUseItemThresholdPayloadAndPostedScope\|PhasePStockRegistryCoversCapturedLeaves)$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |

The focused API regression also proves that the requested date range does not
filter `stock_balances` itself.

No database-backed Stock and Sales route result was claimed in this focused
run because `DATABASE_URL` was unavailable.

## Remaining acceptance evidence

- Exact PowerBuilder Stock-and-Sales period/as-of semantics, opening and
  returned-stock treatment, compatibility-source reconciliation, grouping,
  valuation, and print/PDF/workbook output remain open.
- Full-volume performance and source migration/reconciliation remain open.

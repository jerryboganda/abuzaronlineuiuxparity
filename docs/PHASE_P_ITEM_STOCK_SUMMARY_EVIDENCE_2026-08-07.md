# Phase P - item stock register summary evidence

Date: 2026-08-07

This is a bounded implementation slice for the captured `Item Stock Register
Summary` leaf. It does not claim exact PowerBuilder opening-balance or print
parity.

## Implemented

- The leaf now uses a distinct `item-summary` mode rather than the generic
  stock-balance projection.
- Posted `stock_ledger` rows are joined to immutable posted `sync_events`,
  batches, items, and godowns, then grouped by item, godown, and calendar day.
- Net quantity applies `in`, `out`, and signed `adjustment` directions. Net
  value is the signed quantity multiplied by the posted unit cost.
- Tenant, branch, date, text, godown, batch, and bounded page/page-size
  filters remain enforced by the report route.
- The API and Svelte contracts expose Item Code, Date, Godown, Item, Net
  Quantity, and Net Value.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'Test(StockItemSummaryReadModelAggregatesPostedLedgerByItemDay\|StockLevelReadModelsUseItemThresholdPayloadAndPostedScope\|PhasePStockRegistryCoversCapturedLeaves\|StockReadModelUsesPostedNormalizedLedgersAndBoundedPagination)$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |

The optional database-backed report route check is part of the stock-level
integration fixture and is skipped when `DATABASE_URL` is unavailable. No
database-backed result is claimed for this focused run.

## Remaining acceptance evidence

- Legacy opening-balance treatment, grouping/order labels, carry-forward
  semantics, stock valuation, and print/PDF/workbook output require captured
  PowerBuilder output and approved golden comparisons.
- Full-volume performance and source migration/reconciliation remain open.

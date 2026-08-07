# Phase P - stock threshold report evidence

Date: 2026-08-07

This is a bounded implementation slice for the four captured stock-level
reports. It does not claim exact PowerBuilder calculations or golden-output
parity.

## Implemented

- `Reorder Level Report`, `Optimum Level Report`, `Minimum Level Report`, and
  `Reorder/Optimum Level Report` now have distinct stock modes instead of the
  generic balance mode.
- The normalized query reads tenant/branch-scoped `stock_balances`, joins
  `stock_batches`, `master_items`, and `master_godowns`, requires a posted
  `stock_ledger` source event, and preserves date, text, godown, batch, and
  bounded pagination filters.
- Item thresholds use numeric `master_items.payload` values from
  `ReorderQty`, `OptimumQty`, and `MinimumQty`, with the current maintenance
  keys `ReorderQuantity`, `OptimumQuantity`, and `MinimumQuantity` as
  fallbacks. Invalid or absent values resolve to zero rather than being cast
  unsafely.
- The report contract and Svelte column model expose Batch, Expiry/Updated,
  Godown, Item, On Hand, Reorder Qty, Optimum Qty, and Minimum Qty.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'Test(StockLevelReadModelsUseItemThresholdPayloadAndPostedScope\|PhasePStockRegistryCoversCapturedLeaves\|StockReadModelUsesPostedNormalizedLedgersAndBoundedPagination)$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `git diff --check` | Passed; existing LF/CRLF normalization warnings only |

No database-backed route result was claimed in this focused run because
`DATABASE_URL` was not configured. The optional integration boundary remains
documented rather than hidden.

## Remaining acceptance evidence

- The exact legacy threshold source columns, inclusive comparison rules,
  zero-stock behavior, date meaning, grouping, ordering, and print layout
  still require captured PowerBuilder output and representative data replay.
- The query uses `stock_balances.updated_at` for the retrieval date while the
  displayed observation prefers batch expiry; this is an explicit normalized
  interpretation, not recovered legacy semantics.
- Full-volume performance, source migration/reconciliation, and physical
  printer/report acceptance remain open under the overall acceptance record.

# Phase P - daily stock IN/OUT evidence

Date: 2026-08-07. This is a bounded normalized read-model slice; it does not
claim exact PowerBuilder report parity.

## Implemented

- `Daily Stock IN/OUT` and `Stock IN/OUT(Date Wise)` now use an explicit
  `movement-summary` mode instead of the row-level movement grid.
- Each report returns six typed fields: calendar date, IN/OUT/ADJUSTMENT
  direction, Godown, Item, signed Quantity, and signed Net Value.
- The query reads `stock_ledger` joined to posted `sync_events`, batches,
  items, and godowns with tenant/branch/date/text/godown/batch scope and
  bounded pagination.
- Compatibility `inventory_movements` rows are not mixed into the result.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestStockMovementSummaryReadModelsAggregatePostedInOutByDay$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `git diff --check` | Passed after the focused source/docs edits |

No full build, browser suite, CI flow, or database-backed route result is
claimed. Opening balances, exact legacy date-wise grouping, valuation,
source reconciliation, format-specific columns, and print/PDF/workbook
golden output remain acceptance work.

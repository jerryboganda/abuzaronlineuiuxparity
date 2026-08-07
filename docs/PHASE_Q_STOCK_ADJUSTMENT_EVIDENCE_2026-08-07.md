# Phase Q Stock Adjustments Detail evidence - 2026-08-07

This is a bounded read-model improvement for the captured `Item Reports >
Stock Adjustments > Stock Adjustments Detail` leaf. It does not claim exact
PowerBuilder adjustment grouping, source reconciliation, or print parity.

## Implemented

- The report now unions retained imported `historical_stock_adjustment_lines`
  (`AdjHeader`/`AdjDetail`) with posted normalized `stock_ledger` rows whose
  direction is `adjustment`.
- Normalized rows retain immutable event identity, signed quantity from
  `adjustment_sign`, unit cost, item, batch, godown, and operator context.
- Both sources apply tenant, branch, date, and text filtering before the
  existing six-column adjustment contract and pagination.
- The Svelte report-definition mirror now discloses that normalized posted
  adjustments are included.

## Focused evidence

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'Test(PhaseQHistoricalQueriesAreScopeBoundAndPaginated\|HistoricalStockAdjustmentReportIncludesNormalizedLedgerRows\|HistoricalReportsReadRetainedSourceRowsWithinTenantBranch)' -count=1` | Passed; the database-backed cases skip when `DATABASE_URL` is unavailable, while the query contract remains verified |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: `svelte-check found 0 errors and 0 warnings` |
| `git diff --check` | Passed with the existing LF/CRLF normalization warnings only |

## Remaining acceptance boundary

The source SQL Server adjustment wave is not fully reconciled, and exact
PowerBuilder header/detail grouping, adjustment price calculations, legacy
message/selection semantics, golden output, and print/PDF/workbook behavior
remain open.

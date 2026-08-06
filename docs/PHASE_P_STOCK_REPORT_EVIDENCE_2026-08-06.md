# Phase P — stock and inventory report wave evidence

Date: 2026-08-06

This is a bounded Phase P implementation artifact. It does not claim exact
legacy report parity, and the legacy application/database were not modified.

## Implemented

- 27 captured Stock Reports leaves now have explicit registry entries and
  captured-path resolution:
  - stock-in-hand, batch/priority, godown, reorder/optimum/minimum, narcotics,
    stock-register, item-activity, expiry, and daily stock in/out leaves.
- Balance/expiry leaves use normalized `stock_balances`, `stock_batches`,
  `master_items`, and `master_godowns`.
- Movement/register leaves use `stock_ledger` joined to `sync_events`, with
  posted-only status filtering and batch/godown metadata.
- `godownId`, `batchNumber`, text, date, page, and page-size filters are
  forwarded by the report route and tenant/branch scoped. Queries fetch one
  extra row and retain bounded `LIMIT/OFFSET` pagination; the existing
  page-size ceiling is 1000.
- Compatibility `inventory_movements` is not mixed into Phase P stock reports.
- Expiry reports use typed `stock_batches.expiry_date`.
- Valuation metadata is truthful: normalized valuation is only
  `on_hand × stock_batches.unit_cost`; legacy FIFO/average valuation and exact
  historical valuation remain unreconciled.
- Manufacturer/category/class/reorder/narcotics grouping, Stock-and-Sales
  sales calculations, and other unpromoted legacy fields are not fabricated.
  Definitions identify the normalized projection and its limitations.

## Coverage accounting

Phase M/N/O/P had explicit definitions for 4 + 68 + 24 + 27 = 123 of the
151-leaf accounting. Phase Q now adds 28 mapped financial/remaining
definitions; see `PHASE_Q_FINANCIAL_REPORT_EVIDENCE_2026-08-06.md` for the
current 151-definition accounting and the exact adjacent item-history records
that still require legacy column capture.

## Validation

| Command | Observed result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `go test ./services/api/internal/httpapi -run 'Test(PhaseP\|StockReadModel)' -count=1` | Passed |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Passed; API, edge, and migration packages all green |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g 'stock, expiry, godown, and batch'` | Passed: 1 test |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g 'Daily Sale Detail\|fallback report\|Sales Return detail\|purchase detail and summary\|purchase return, supplier\|stock, expiry, godown, and batch'` | Passed: 6 report-content tests |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `pnpm --filter @abuzar/web build` | Passed: production build completed |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=1` | 58 passed; no Phase P test failed. |

The broader report-content and navigation tests passed after the contextual
menu and canonical purchase UI stabilization wave.

## Remaining risks

- No full 3.2M-row p95 benchmark was captured in this wave; the queries are
  bounded and scope/date filtered, but scale acceptance remains open.
- Exact legacy valuation, historical back-date reconstruction, manufacturer /
  category / class / reorder / narcotics grouping, and Stock-and-Sales
  calculations remain unimplemented.

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
- `Stock in Hand > Back Date` now uses the imported
  `historical_stock_snapshots` projection from `dbo.StockReport`, preserving
  source row, as-of date, stock, purchase price, sale price, average price,
  recent purchase price, and pack-unit fields. See the dated follow-up
  evidence in `docs/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md`.
- Movement/register leaves use `stock_ledger` joined to `sync_events`, with
  posted-only status filtering and batch/godown metadata.
- `godownId`, `batchNumber`, text, date, page, and page-size filters are
  forwarded by the report route and tenant/branch scoped. Queries fetch one
  extra row and retain bounded `LIMIT/OFFSET` pagination; the existing
  page-size ceiling is 1000.
- Compatibility `inventory_movements` is not mixed into Phase P stock reports.
- Expiry reports use typed `stock_batches.expiry_date`.
- Reorder/optimum/minimum leaves now have a follow-up normalized threshold
  projection over item payload values and posted stock balances; see
  `docs/PHASE_P_STOCK_LEVEL_EVIDENCE_2026-08-07.md`.
- Item Stock Register Summary now has a follow-up normalized posted-ledger
  aggregation by item, godown, and day; see
  `docs/PHASE_P_ITEM_STOCK_SUMMARY_EVIDENCE_2026-08-07.md`.
- Stock and Sales now has a follow-up normalized balance/sale-allocation
  projection; see `docs/PHASE_P_STOCK_SALES_EVIDENCE_2026-08-07.md`.
- The two narcotics movement leaves and the generic-type narcotics leaf now
  have a follow-up posted-ledger projection filtered/grouped from captured Item
  payload flags; see `docs/PHASE_P_NARCOTICS_STOCK_EVIDENCE_2026-08-07.md`.
- `Expiry Report(Class Wise)` now has a follow-up typed-expiry/class
  projection using the captured Item Class payload; see
  `docs/PHASE_P_EXPIRY_CLASS_EVIDENCE_2026-08-07.md`.
- The core Stock-in-Hand Manufacturer/Category/Class leaves now have a
  follow-up Item-payload classification projection; see
  `docs/PHASE_P_STOCK_CLASSIFICATION_EVIDENCE_2026-08-07.md`.
- `Daily Stock IN/OUT` and `Stock IN/OUT(Date Wise)` now have a follow-up
  posted-ledger day/direction/godown/item aggregate; see
  `docs/PHASE_P_STOCK_MOVEMENT_SUMMARY_EVIDENCE_2026-08-07.md`.
- `Stock In hand > Supplier Manufacturer Association` now has a follow-up
  normalized stock-balance projection using Item Manufacturer payload and
  tenant-scoped `item_suppliers`; see
  `docs/PHASE_P_STOCK_SUPPLIER_MANUFACTURER_EVIDENCE_2026-08-07.md`.
- Valuation metadata is truthful: normalized valuation is only
  `on_hand × stock_batches.unit_cost`; legacy FIFO/average valuation and exact
  historical valuation remain unreconciled.
- Unreviewed manufacturer/category/class joins, Stock-and-Sales calculations beyond
  canonical allocated Sales Qty, exact narcotics flag/generic grouping, and
  other unpromoted legacy fields are not fabricated. Definitions identify the
  normalized projection and its limitations.

## Coverage accounting

Phase P contributes 27 distinct catalog leaves. Final reconciliation is
**68 + 24 + 27 + 32 = 151** non-blank catalog leaves across N/O/P/Q; Phase M
and direct financial aliases are API projections, not additional catalog
leaves. See `PHASE_Q_FINANCIAL_REPORT_EVIDENCE_2026-08-06.md` for the final
count and exact Item Reports fallback labels.

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
- Exact legacy valuation, manufacturer / category / class / narcotics grouping,
  threshold comparison semantics, Stock-and-Sales calculations, Back Date
  print/golden output, and full source rerun/reconciliation remain unverified.

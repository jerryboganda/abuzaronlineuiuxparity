# Phase P historical StockReport / Back Date evidence — 2026-08-07

## Scope

The captured `StockReport` source has typed historical fields that are not
represented by the current posted `stock_balances` cache. This slice wires the
captured `Stock in Hand > Back Date` leaf to the imported
`historical_stock_snapshots` projection. It is a source-backed report slice,
not a claim that all Stock Reports, valuation rules, or print formats are
complete.

## Source and target evidence

- The reviewed loader reads `dbo.StockReport` fields `Date`, `GCode`, `ICode`,
  `Stock`, `PurchasePrice`, `SalePrice`, `AvgPrice`, `RecentPurchasePrice`, and
  `PackUnits` into `historical_stock_snapshots`.
- The local canonical tenant currently contains `3,215,967` target snapshot
  rows, covering `2025-01-01` through `2026-07-31`. This is retained migration
  evidence; the current source probe was not rerun because SQL Server
  Integrated Authentication remains blocked by the untrusted-domain login
  boundary.
- The report keeps the source row identity as `document`, uses `as_of` for the
  report date, resolves canonical item/godown names, and does not invent a
  batch identity.

## Implemented contract

`stock-in-hand-back-date` now uses a dedicated tenant/branch/date/godown query
over `historical_stock_snapshots`. Its ten columns are:

`Source Row`, `As Of`, `Godown`, `Item`, `Stock`, `Purchase Price`, `Sale
Price`, `Average Price`, `Recent Purchase Price`, and `Pack Units`.

The API returns the captured numeric fields as exact decimal strings. The
definition explicitly identifies the source-backed projection and discloses
that manufacturer/category/class/narcotics grouping and exact print
calculations remain open.

## Verification

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestHistoricalStock(ReadModelCarriesCapturedStockReportFields\|BackDateReportUsesImportedStockReportFields)' -count=1` | Passed; query contract and PostgreSQL tenant/date/godown integration passed |
| `go test ./services/api/internal/httpapi -count=1` | Passed |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| Focused Playwright stock-report test | Passed: normalized stock leaves plus Back Date source disclosure and field contract |

## Remaining acceptance boundary

This closes only the source-backed Back Date read-model slice. Exact
PowerBuilder Back Date grouping, filters beyond the captured source fields,
valuation/COGS policy, print/PDF/workbook golden output, and the other stock
report families remain unverified. Full canonical source rerun, complete stock
ledger reconciliation, full-volume p95, and operator/UAT acceptance are still
required before Phase P or the rebuild can be marked complete.

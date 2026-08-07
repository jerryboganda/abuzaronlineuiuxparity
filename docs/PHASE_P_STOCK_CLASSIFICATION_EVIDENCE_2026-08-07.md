# Phase P - stock-in-hand classification evidence

Date: 2026-08-07. This is a bounded source-shaped implementation slice; it
does not claim exact PowerBuilder report parity.

## Implemented

- `Stock In hand > Manufacturer wise`, `Manufacturer Wise (Format2)`,
  `Category wise`, and `Class Wise` now use explicit classification modes
  instead of an ungrouped balance projection.
- Each mode returns six typed fields: its captured Item classification,
  Expiry/Updated, Godown, Item, On Hand, and Unit Cost.
- The projection uses normalized `stock_balances`, typed batch metadata, and a
  posted `stock_ledger`/`sync_events` existence gate with tenant/branch/date/
  text/godown/batch scope and bounded pagination.
- Source-shaped payload fallbacks are explicit: Manufacturer/ManfCode,
  Category/ICatCode, and Class/ItemClass. Missing values are shown as
  `Unspecified`; no unreviewed manufacturer/category join is fabricated.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestStockClassificationReadModelsUseCapturedItemGroups$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `git diff --check` | Passed after the focused source/docs edits |

No full build, browser suite, CI flow, or database-backed route result is
claimed. Exact legacy group joins, supplier/manufacturer association,
valuation, source reconciliation, format-specific columns, and print/PDF/
workbook golden output remain acceptance work.

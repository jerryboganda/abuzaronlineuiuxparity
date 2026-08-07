# Phase P - class-wise expiry report evidence

Date: 2026-08-07. This is a bounded source-shaped implementation slice; it
does not claim exact PowerBuilder report parity.

## Implemented

- `Expiry Report(Class Wise)` now has a distinct `expiry-class` registry mode
  and six-column contract: Class, Expiry Date, Godown, Item, On Hand, and Unit
  Cost.
- The query uses typed `stock_batches.expiry_date`, normalized
  `stock_balances`, and a posted `stock_ledger`/`sync_events` existence gate.
  It keeps tenant/branch/date/text/godown/batch scope and bounded pagination.
- Class is read from the captured Item payload (`Class`, `ItemClass`, or
  lowercase `class`) with an explicit `Unspecified` fallback. Nonzero stock
  and rows with no typed expiry are excluded.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestStockExpiryClassReadModelUsesTypedExpiryAndItemClass$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `git diff --check` | Passed after the focused source/docs edits |

No full build, browser suite, CI flow, or database-backed route result is
claimed. Exact class-code joins, legacy date-window semantics, opening/zero
stock handling, source reconciliation, visible columns beyond this bounded
contract, and print/PDF/workbook golden output remain acceptance work.

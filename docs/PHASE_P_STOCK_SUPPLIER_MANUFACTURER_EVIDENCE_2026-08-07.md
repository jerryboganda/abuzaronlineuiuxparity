# Phase P - supplier/manufacturer stock association evidence

Date: 2026-08-07. This is a bounded source-shaped implementation slice; it
does not claim exact PowerBuilder report parity.

## Implemented

- `Stock In hand > Supplier Manufacturer Association` now uses an explicit
  `supplier-manufacturer` mode.
- Each row exposes Manufacturer, aggregated Supplier(s), Godown, Item, On
  Hand, and Unit Cost.
- The projection joins normalized `stock_balances` and typed batches to the
  captured Item Manufacturer payload, tenant-scoped `item_suppliers`, supplier
  masters, and posted `stock_ledger`/`sync_events` evidence.
- Supplier names are aggregated per item so multiple supplier links do not
  duplicate a physical batch balance. Missing values remain explicit as
  `Unspecified`.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestStockSupplierManufacturerReadModelUsesItemSuppliersAndPostedBalances$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `git diff --check` | Passed after the focused source/docs edits |

No full build, browser suite, CI flow, or database-backed route result is
claimed. Exact legacy supplier priority selection, association joins,
valuation, source reconciliation, format-specific columns, and print/PDF/
workbook golden output remain acceptance work.

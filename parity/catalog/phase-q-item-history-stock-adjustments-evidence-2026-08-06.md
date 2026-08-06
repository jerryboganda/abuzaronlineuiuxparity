# Phase Q research: remaining item-history leaves

Date: 2026-08-06

This artifact records why six report leaves remain compatibility/generic
projections. No legacy write was performed and no parity claim is made.

| Legacy path | Command ID | Current status | Source candidates |
|---|---:|---|---|
| Reports > Item Reports > History > Sale Price Difference | 13357 | Generic fallback | `Item`, `PricePolicy`, `changeitemprice.pbd`, unidentified item log |
| Reports > Item Reports > History > Item Basic Data Changes | 13358 | Generic fallback | `Item`, unidentified item revision log |
| Reports > Item Reports > History > Item Sale Price Changes | 13359 | Generic fallback | `Item`, `PricePolicy`, `changeitemprice.pbd`, unidentified price audit |
| Reports > Item Reports > History > New Item(s) Created/Defined | 13361 | Generic fallback | `Item`, unidentified creation audit |
| Reports > Item Reports > History > Item Name Changes | 13363 | Generic fallback | `Item`, unidentified name audit |
| Reports > Item Reports > Stock Adjustments > Stock Adjustments Detail | 13364 | Generic fallback | `adjustment.pbd`, `StockReport`, unidentified adjustment header/detail |

The captured catalog establishes the paths and command IDs, but no
leaf-specific format list, retrieval arguments, DataWindow columns, output
raster, orientation, or historical source contract was captured for these
leaves. Existing report captures describe only the Daily Sales Detail
workflow: Select Format, the legacy-spelled “Specify Retrieval Arguements”
dialog, area selectors, date/time range, cash/credit flags, and print preview.

The normalized application schema stores current master values and stock
ledger movements, not reliable item revision history. `stock_ledger` supports
direction, adjustment sign, quantity, cost, batch, godown, and time, but does
not prove the complete legacy adjustment detail or operator/source semantics.
`master_items` and `master_records` likewise do not provide prior name/price
values.

## Required next evidence

1. Launch the sandbox reference read-only and capture commands 13357, 13358,
   13359, 13361, 13363, and 13364 with the existing `open-and-capture.ps1`
   driver.
2. Record each format list, retrieval control/default, output columns,
   orientation, empty/populated output, and source-table identity.
3. Add reviewed source mappings before implementing projections.

Until those artifacts exist, the generic fallback is intentional and must not
be relabeled as a real legacy projection.

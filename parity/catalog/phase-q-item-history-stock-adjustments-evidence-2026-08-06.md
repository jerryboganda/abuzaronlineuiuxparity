# Phase Q research: remaining item-history leaves

Date: 2026-08-06

This artifact records the source-backed normalized projections for six report
leaves. No legacy write was performed and no exact PowerBuilder parity claim
is made.

| Legacy path | Command ID | Current status | Source candidates |
|---|---:|---|---|
| Reports > Item Reports > History > Sale Price Difference | 13357 | Normalized source-backed; exact legacy semantics open | `ItemLog.NewSalePrice`, `ItemLog.SalePrice` |
| Reports > Item Reports > History > Item Basic Data Changes | 13358 | Normalized source-backed; exact legacy semantics open | `ItemLog` snapshot comparison |
| Reports > Item Reports > History > Item Sale Price Changes | 13359 | Normalized source-backed; exact legacy semantics open | `ItemLog.SalePrice`, `ItemLog.NewSalePrice` |
| Reports > Item Reports > History > New Item(s) Created/Defined | 13361 | First-observed source-backed view; creation semantics open | first `ItemLog` snapshot per `ICode` |
| Reports > Item Reports > History > Item Name Changes | 13363 | Normalized source-backed; exact legacy semantics open | adjacent `ItemLog.Name` snapshots |
| Reports > Item Reports > Stock Adjustments > Stock Adjustments Detail | 13364 | Normalized source-backed; exact legacy semantics open | `AdjHeader` + `AdjDetail` |

The captured catalog establishes the paths and command IDs, but no
leaf-specific format list, retrieval arguments, DataWindow columns, output
raster, orientation, or historical source contract was captured for these
leaves. Existing report captures describe only the Daily Sales Detail
workflow: Select Format, the legacy-spelled “Specify Retrieval Arguements”
dialog, area selectors, date/time range, cash/credit flags, and print preview.

Migration `027_historical_item_history_adjustments.sql` now stores
source-backed `ItemLog` snapshots and joined `AdjHeader`/`AdjDetail` rows in
separate tenant/branch-scoped tables. The importer keeps the complete source
payload and uses optional current-item/godown joins, preventing silent loss
when a current master dependency is missing. This is a normalized evidence
projection, not an assertion that the legacy DataWindow calculations or
layout have been recovered.

## Required next evidence

1. Launch the sandbox reference read-only and capture commands 13357, 13358,
   13359, 13361, 13363, and 13364 with the existing `open-and-capture.ps1`
   driver.
2. Record each format list, retrieval control/default, output columns,
   orientation, empty/populated output, and source-table identity.
3. Reconcile source-row counts, target-row counts, and typed business totals
   for the canonical tenant before accepting the importer wave.

Until those artifacts exist, the source-backed projection must not be
relabeled as exact legacy parity.

## Static PBD evidence update

Read-only decompiled-name evidence identifies:

- `f_generateitemlog.fun`, `w_itemprhistory.win`, `w_itemsalehistory.win`,
  `w_itemadjustmenthistory.win`, `w_adjwindow.win`, `w_adjbuffer.win`, and
  `w_change_item_price.win`;
- `d_itemsalepricedifference.dwo`, `d_itemsalepricechanges.dwo`,
  `d_itemwiseadjustmentdetail.dwo`, `d_itemadjustmenthistory.dwo`,
  `d_adjustmentdetail.dwo`, `d_adjustmentdetail_invwise.dwo`,
  `d_godownwiseitemadjustmentdetail.dwo`, and
  `d_group_adjustment_detail.dwo`;
- argument/format candidates for price difference, basic-data changes, and
  adjustment detail in the corresponding `w_arg_*.win` files.

These names confirm history/adjustment mechanisms but do not recover SQL,
DataWindow columns, or old/new value semantics. The six runtime captures remain
unavailable because the legacy process could not be executed in the research
environment. The rebuild now has a reviewed source-backed target/import path;
no legacy database was modified.

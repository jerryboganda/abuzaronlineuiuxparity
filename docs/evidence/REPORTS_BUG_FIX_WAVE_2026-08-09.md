# Reports bug-fix wave — 2026-08-09 (post reports-gap wave 1+2)

10 parallel agents fixed the remaining documented-but-unfixed bugs from `docs/evidence/PHASE_*_GOLDEN_VERIFICATION_*_2026-08-09.md` and the "Progress update" sections of `docs/REMAINING_WORK_100_PERCENT_PARITY_2026-08-09.md`. These were already root-caused in prior waves; this wave implemented and verified the fixes. 9 of 10 targets were fixed successfully; 1 (stock_balances cache rebuild) correctly identified a genuine upstream data-integrity blocker and did not force a bad fix.

## Fixed

1. **`supplier-wise-advance-income-tax`** — the tax-advance LATERAL fallback was gated to the document's `MIN(line_number)` row while the outer filter separately required *that same* row to carry `advance_tax_rate > 0`; when the tax-bearing item wasn't literally line 1, the invoice's advance tax vanished entirely. Fixed by requiring the inner subquery to also find `advance_tax_rate > 0`. Recovered exactly 94 documents / PKR 9,589.64 tenant-wide, matching the original finding precisely.

2. **`purchase-order`/`purchase-order-summary`/`purchase-order-supplier-wise`/`purchase-return-summary`** — amount always showed `0.0000`/disagreed with sibling leaves because `COALESCE(ple.amount, d.total_amount)` reads a header/ledger total that's stale or absent for these aggregates. Investigated ground truth by hand on real documents (PO #2810, return #1856) — confirmed `SUM(business_document_lines.line_total)` is correct in both cases. Fixed for `purchase_order` and `return` aggregates; `receiving` and `detail` mode left untouched (their existing sources were already correct).

3. **`category-wise-purchase`** — didn't actually group by category despite the name. Fixed with the `master_categories` join already verified working in the prior research pass (resolves 100% of items). Verified real category names (MEDICINES, NARCOTICS, CONSUMER, ...) with correct totals.

4. **`manufacturer-wise-detail`, `manufacturer-wise-monthly-stock-movement`, `supplier-manufacturer-wise-g-p`** — had no manufacturer join/grouping at all. Fixed with the `master_manufacturers` join (838 real names, 100% coverage) via new dedicated `manufacturer-detail`/`manufacturer-month-summary`/`supplier-manufacturer-summary` modes. The gross-profit figure for the third leaf was deliberately **not** implemented — no verifiable purchase-side G/P formula exists (a purchase line's cost is its own amount, so "cost minus cost" isn't meaningful); documented as needing a product decision.

5. **`item-reports-history-sale-price-difference`, `item-reports-history-item-sale-price-changes`** — a COALESCE mixing a numeric price column with a text name column silently substituted the item's *name* for a NULL price. Fixed: price columns no longer fall back to a name.

6. **`item-reports-history-item-basic-data-changes`** — always showed the item name instead of the actual changed field (of 17 tracked). Fixed with a LATERAL diff against the preceding snapshot, surfacing only the fields that actually changed.

7. **`item-reports-stock-adjustments-stock-adjustments-detail`** — one half of its UNION (the `historical_stock_adjustment_lines` branch) never joined `master_items`/`master_godowns`, showing raw legacy codes while the other half showed resolved names. Fixed with the missing joins, matching the existing pattern.

8. **`customer-sales-invoice-wise-profit-margin-detail`, `customer-sales-customer-category-wise-sales-customer-wise-gross-profit`** — silently blank cost/profit-margin columns because the query read cost exclusively from `stock_allocations` (empty at real scale). Fixed by adding `stock_ledger` (already powering the Phase J moving-average engine) as the primary COGS source, falling back to `stock_allocations` only where it happens to have data.

9. **`adjustment-*` (6 leaves)** — all resolved to the same ungrouped row-level query despite distinct names. Added real grouping for 2 of the 6 (`adjustment-adjustment-summary`: day/type/godown/item; `adjustment-item-wise-adjustment-summary`: item/godown/day). The remaining 4 were investigated and left as documented row-level/ambiguous with concrete schema evidence (e.g. `adjustment-adjustment-summary-inv-wise`/`-detail-inv-wise`: `stock_ledger` adjustment rows have no real per-adjustment document identity to group on — `source_document_id` is always NULL for this direction).

10. **`listing-item-list-class-wise`** — `adminKind: "item_class"` could never match any real `master_records.kind` value (structurally guaranteed zero rows for any tenant). Fixed by pointing it at the real, populated `item_category` kind, consistent with the sibling admin-listing leaves' pattern.

## Not fixed (correctly, not forced)

**`stock_balances` cache rebuild for the sandbox tenant** — the existing rebuild mechanism (`POST /v1/inventory/rebuild`) was invoked correctly (via the real production function, not a reimplementation) but failed on a genuine schema constraint: `stock_balances_on_hand_check` (`on_hand >= 0`) rejects 480 of the tenant's 10,507 batches, which compute a negative net on-hand from `stock_ledger` — almost certainly a migration gap (missing opening-balance entries), not an application bug. The rebuild aborts entirely (single transaction) rather than partially succeeding; `stock_balances` remains 0 rows before and after. This needs a human decision: restore the missing opening-balance data, deliberately relax the constraint for migrated data, or have `rebuildStockBalances` clamp negative aggregates (a real behavior change, not made unilaterally). Left exactly as found, root-caused precisely rather than worked around.

## Cross-cutting fixes to pre-existing tests

Several fixes changed behavior that earlier waves' own tests had pinned as the (buggy) status quo. Reconciled after the fact (the dispatched agents correctly stayed out of files they didn't own; this cleanup was done once all fixes landed):
- `report_golden_tax_admin_test.go`: flipped `TestAdvanceTaxFallbackAmountGatedToSamePhysicalLineAsRate` → `TestAdvanceTaxFallbackFindsRateOnAnyLine` (asserts the fix); removed the now-superseded `TestItemClassAdminKindCanNeverMatchAMasterRecordsRow` (covered by the new fix's own test file); updated `TestPhaseGoldenTaxAdminLeavesResolveToExpectedMode`'s `listing-item-list-class-wise` expectation.
- `report_golden_no_remainder_test.go`: updated `TestPhaseNORemainderPurchaseDetailModeLeavesAreByteIdenticalQueries` — `manufacturer-wise-detail` legitimately diverged from `supplier-wise-detail`/`supplier-wise-purchase-detail` now that it has its own manufacturer-join mode.
- `report_golden_o_purchase_test.go`: updated mode-name expectations for `manufacturer-wise-detail`/`manufacturer-wise-monthly-stock-movement`/`supplier-manufacturer-wise-g-p`; renamed `TestPhaseOGoldenPurchaseLeafGroupingDimensionsDoNotMatchTheirLegacyNames` → `...NowMatchTheirLegacyNames` (asserts the fixes).
- `server_test.go`: `TestPhaseOReportRegistryCoversCapturedPurchaseLeaves` and `TestPurchaseSummaryModesUseExplicitBuckets` special-cased `category-wise-purchase`'s column 2 label (now "Category", was "Supplier").
- `historical_integration_test.go`: `TestHistoricalReportsReadRetainedSourceRowsWithinTenantBranch` updated its expected `Item` value for the stock-adjustments leaf from the raw legacy ID to the resolved name (`"Stock Item / B-1"`), matching the fix's intent.

## Verification

`go vet ./services/api/...` clean. `go build ./services/api/...` and `go build ./...` (workspace) clean. Full offline test suite (`go test ./services/api/...`) passes with no `DATABASE_URL`. Full DB-backed suite (`DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next go test ./services/api/...`) passes, with one unrelated pre-existing failure: `TestMaintenanceManageOperationsIntegration/backup_...` fails on `pg_dump: could not write to output file: No space left on device` — the local machine's C: drive is at 100% (87MB free), a pre-existing environment condition unrelated to any change in this wave. Not touched; flagged for the operator.

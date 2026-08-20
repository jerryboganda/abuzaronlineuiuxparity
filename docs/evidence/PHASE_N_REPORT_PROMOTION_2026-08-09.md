# Phase N report leaf promotion — 2026-08-09

Scope: `services/api/internal/httpapi/reports.go` lines 137-317
(`phaseNReportRegistry`), plus the shared dispatch logic at lines 1362-1560
that turns a `reportSpec` into a `reportDefinition`. This is a bounded,
conservative promotion pass, not a claim that Sales Reports parity is done.

## Important correction to the promotion mechanism

Before describing what was promoted, one thing has to be stated plainly
because it changes what "before/after `projectionStatus`" can honestly mean
for these leaves:

**Setting `salesMode` on a `phaseNReportRegistry` leaf does not, and by
design cannot, change the literal `projectionStatus` string from
`"event-ledger"` to `"real"`.** Reading the dispatch logic at lines
1392-1419 shows that `spec.salesReadModel` branches only ever change
`columns` and `projectionNote`; `projectionStatus` is left at the
`"event-ledger"` value set on line 1393. Only `documentReadModel`,
`headerReadModel`, `stockReadModel`, `financeMode`, `adminKind`,
`historyMode`, and `reprintMode` branches set `projectionStatus = "real"`.

This is not a guess — it is locked in by an existing test that I did not
write and am not permitted to change:
`TestPhaseNReportRegistryDefinitionsAndAggregateFilters` in
`services/api/internal/httpapi/server_test.go` (lines 426-465) asserts, for
every `phaseNReportRegistry` entry that isn't `documentReadModel`/
`financeMode`, that `definition.ProjectionStatus == "event-ledger"`. I
confirmed this is exactly how the already-shipped, precedent-setting sibling
leaves behave today: `sale-detail` (`salesMode = "line-detail"`),
`sale-summary` (`salesMode = "invoice-summary"`), and
`customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report`
(`salesMode = "line-detail"`, already promoted by a prior agent, covered by
`TestCustomerCategorySalesDetailReportUsesLineDetailProjection` in
`read_models_test.go`) all report the literal string `"event-ledger"` today,
despite having real, report-specific column contracts instead of the
6-column generic grid.

So "promotion" for a `salesReadModel` leaf means exactly what it meant for
those already-accepted siblings: the leaf moves from
`reportEventLedgerColumns()` (6 generic columns, `reportColumns()`-shaped) to
a real, report-specific column/query contract (`dailySaleDetailColumns()` /
`salesSummaryReportColumns(mode)`) driven by the shared, already-verified
read-model SQL — while the literal `projectionStatus` field stays
`"event-ledger"`, matching the existing registry-wide invariant test. Any
document claiming otherwise for a `salesReadModel` leaf would itself be an
overclaim.

## Promoted (2 leaves)

| Kind | Title | Menu path | `salesMode` before → after | Columns before → after | `projectionStatus` before → after |
|---|---|---|---|---|---|
| `customer-sales-detail` | Detail | Reports > Sales Reports > Customer Sales > Detail | `""` → `"line-detail"` | 6 generic (`reportEventLedgerColumns()`) → 11-field (`dailySaleDetailColumns()`: Alias, Item Description, Sale Price, Qty, Disc%, Discount Value, Item Disc, SalesTax Value, Amount, Expiry Date, Batch Number) | `"event-ledger"` → `"event-ledger"` (unchanged; see correction above) |
| `customer-sales-summary` | Summary | Reports > Sales Reports > Customer Sales > Summary | `""` → `"invoice-summary"` | 6 generic (`reportEventLedgerColumns()`) → 6-field invoice contract (`salesInvoiceSummaryReportColumns()`: Invoice, Date, Customer, Summary, Quantity, Amount) grouped once per document | `"event-ledger"` → `"event-ledger"` (unchanged; see correction above) |

Both leaves keep their existing `aggregateCondition = reportSaleAggregate`
(`se.aggregate = 'sale'`) — untouched by this change.

### Why these two, and why the confidence is high

Both satisfy condition (a) from the task very directly: each is the
`Customer Sales` submenu's own "Detail"/"Summary" leaf, and the codebase
already has an exactly-shaped, already-verified sibling one menu level up —
`sale-detail` (Daily Reports > Sale > Sale detail, `line-detail`) and
`sale-summary` (Daily Reports > Sale > Sale summary, `invoice-summary`).
Crucially, both siblings share the *same* `aggregateCondition` value
(`reportSaleAggregate`) as the two promoted leaves, so `salesReadModelQueryMode`
routes them to the byte-identical query function
(`dailySalesDetailReadModelQuery` / `invoiceSummaryReadModelQuery`) — there is
no new grouping, join, or calculation being invented, only reuse. The new
test `TestCustomerSalesDetailReportUsesLineDetailProjection` /
`TestCustomerSalesSummaryReportUsesInvoiceSummaryProjection` assert the
generated SQL for the promoted kind is textually identical to the query the
already-shipped sibling produces.

### Code diff summary

Single, additive change to the `switch` inside the `phaseNReportRegistry`
builder (`reports.go`, inside the "Sales Reports" loop, around line 284-313).
No new SQL, no new column function, no new `salesMode` value, no changes to
the dispatch logic at lines 1362-1560, no changes to any existing test:

```go
 mode := ""
 switch report.kind {
+case "customer-sales-detail":
+    mode = "line-detail"
+case "customer-sales-summary":
+    mode = "invoice-summary"
 case "customer-sales-days-summary":
     mode = "day-summary"
 ...
```

### Verification

**`go vet`** — clean for the whole module:
```
cd services/api && go vet ./...
```
exited 0 with no output.

**Targeted `go test`** — new file
`services/api/internal/httpapi/report_promotion_n_test.go` adds:
- `TestCustomerSalesDetailReportUsesLineDetailProjection`
- `TestCustomerSalesSummaryReportUsesInvoiceSummaryProjection`
- `TestPhaseNPromotedLeavesStillSatisfyRegistryInvariant` (guards that the
  two promoted leaves still satisfy the existing registry-wide
  `"event-ledger"` invariant, so this promotion cannot silently regress
  `TestPhaseNReportRegistryDefinitionsAndAggregateFilters`)

```
go test ./internal/httpapi/... -run 'CustomerSales|PhaseNPromoted|PhaseNReportRegistry|SalesReportDefinitions|DailySaleDetail' -v
```
All PASS, including the pre-existing
`TestPhaseNReportRegistryDefinitionsAndAggregateFilters` and
`TestSalesReportDefinitionsDescribeCanonicalUnionAndReturnCompatibility`.

Full module suite (`go test ./...` from `services/api`) also passes:
`internal/httpapi` (16.8s), `internal/pricing`, `internal/rlsprobe` all `ok`.

(Note: during this session, `go test`/`go vet` occasionally hit transient
build failures in two unrelated, already-existing test files —
`report_golden_adjustment_history_test.go` and
`report_golden_tax_admin_test.go` — because another of the 10 parallel
agents was actively mid-edit on them at that moment; both resolved on retry
a few seconds later and are not part of this change.)

**Live psql verification against the sandbox tenant**
(`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch
`ffffffff-ffff-ffff-ffff-ffffffffffff`, code `legacy-reference-sandbox`,
291,361 posted cash/credit sale documents spanning 2025-01-01 to
2026-07-31), using the RLS preamble supplied for this task, ran the exact
SQL the two promoted leaves now execute (`dailySalesDetailReadModelQuery`
body for `customer-sales-detail`; `invoiceSummaryReadModelQuery` body for
`customer-sales-summary`) for `occurred_at` in `[2026-07-01, 2026-07-02)`:

`customer-sales-detail` (line-detail) sample — 8 of the rows returned:
```
 document | occurred_at             | party                | item                         | quantity    | amount   | alias | item_description             | sale_price | item_discount | sales_tax_value | batch_number
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | AUGMENTIN 156.25MG 90ML SYP  | 1.00000000  | 379.6400 | 20580 | AUGMENTIN 156.25MG 90ML SYP  | 379.64     | 0.00          | 0.00             | .
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | BRUFEN 120ML SUSP            | 1.00000000  | 129.1200 | 19539 | BRUFEN 120ML SUSP            | 129.12     | 0.00          | 0.00             | .
 868770   | 2026-07-01 23:58:24+05  | CASH SALES CUSTOMER  | VONOLIA 20MG TAB             | 14.00000000 | 519.9600 | 28199 | VONOLIA 20MG TAB             | 37.14      | 0.00          | 0.00             | .
```
`quantity * sale_price == amount` holds for every sampled row (e.g. 14 ×
37.14 = 519.96), confirming the line-level arithmetic is sane, not just
non-null.

`customer-sales-summary` (invoice-summary) for the same window and
documents:
```
 document | occurred_at             | party                | quantity | amount
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | 2.0000   | 510.0000
 868770   | 2026-07-01 23:58:24+05  | CASH SALES CUSTOMER  | 14.0000  | 521.0000
```
Document 868771's summed quantity (2.0000) equals the sum of its two
line-detail rows (1 + 1); its amount (510.00) is the document's authoritative
`total_amount` header value (not a re-sum of line amounts, matching the
existing `sale-summary` semantics exactly), which is why it differs slightly
from the 379.64 + 129.12 = 508.76 line-amount sum — the same header-total
convention already accepted for `sale-summary`.

## Candidates examined and declined this pass

| Candidate | Reason declined |
|---|---|
| `category-wise-sales` | Title is the bare word "Sales" with no legacy screenshot or capture anywhere in `parity/captures/legacy/` or `docs/` to confirm whether it means a flat category-total summary (like `customer-category-summary`) or a category-grouped line listing. The neighboring `Category Wise` submenu leaves (`Gross Profit`, `Net Sale`, `Monthly Sale`, `Deviated Items`) suggest a richer, more specific report family than a plain summary, so guessing the shape risks exactly the overclaim this audit exists to catch. Additionally, `master_items.payload` has 0/30,052 rows with a populated `Category` name key — only the raw `ICatCode` legacy code is populated (30,052/30,052) — so even a best-effort grouping would display an opaque numeric code, not a category name, unless a new join to `master_categories` is added. That's new multi-table infrastructure, not a minimal reuse. |
| `manufacturer-wise-sales` | Same title ambiguity as `category-wise-sales`, and the same data gap: `master_items.payload->>'Manufacturer'` is 0/30,052 populated; only `ManfCode` (30,052/30,052) is present. A `master_manufacturers` table with real `code`/`name` columns exists, but no report in this file currently joins sales lines to it — building that join is new infrastructure, not a safe minimal promotion. |
| `user-wise-sales` | Checked directly against the sandbox tenant: `business_documents.operator_id` is **0/291,361** populated for posted cash/credit sale documents (`SELECT count(operator_id) FROM business_documents WHERE tenant_id=... AND kind IN ('cash-sale','credit-sale') AND status='posted'` → 0). There is currently no reliable per-user attribution on migrated sales data at all, canonical or compatibility, so a "user-wise" grouping mode would either bucket everything into one `Unspecified` row or require inventing an unverified `legacy_payload` key. Declining outright rather than shipping a mode that can't be exercised against real data. |
| `dead-item-list` | Semantically inverted from every existing `salesMode`: all current modes summarize rows that exist in the sales read model, but "dead item" means items with **zero** matching sales rows in a period — an anti-join against `master_items`, not a groupable projection of `sales_rows`. The retrieval-argument contract this report family exposes (`SupportsDateRange`) has no "reference date + inactivity threshold" concept, and legacy "dead" semantics (calendar days vs. no purchase/sale ever vs. below-reorder-and-unsold) aren't captured anywhere in this repo. Building the anti-join without knowing the legacy threshold definition would be guessing at a business rule, not reusing verified infrastructure. |
| `slow-fast-moving-items` | Same class of problem as `dead-item-list`: legacy "slow/fast moving" classification implies a velocity threshold or ranking rule (units/day cutoffs, percentile bands, etc.) that isn't captured in any doc, schema comment, or existing report in this codebase. No safe, verifiable basis to pick numbers. |
| `category-wise-item-wise-sale-discounts-detail` | Title says "Item Wise", nested under the "Category Wise" submenu — the same submenu/title mismatch pattern already seen elsewhere in this registry (legacy menu nesting doesn't reliably predict the report's actual grouping). Genuinely ambiguous whether this wants a `line-detail`-shaped listing (plausible, since `dailySaleDetailColumns()` already carries discount fields) or a category-grouped discount summary. Without a captured legacy screenshot to resolve the ambiguity, promoting it would be a guess. |
| `hourly-sales-graph` | A same-title precedent exists (`customer-sales-hourly-graph` → `hour-summary`, already set by a prior agent), but that precedent groups by **hour + customer**. `hourly-sales-graph` is a *different*, top-level Sales Reports leaf (not nested under Customer Sales); a "graph" report more plausibly wants a single hour-of-day series (no customer breakdown) than a per-customer cross-tabulation. Reusing `hour-summary` here could silently impose a grouping dimension the legacy report never had. No capture exists to confirm either way, so left as event-ledger. |

`customer-sales-lp-ledger`, the three `...customer-category-wise-sales-output-sales-tax-report` /
`...ntn-wise-sales-tax-report` / `...customer-wise-advance-tax` leaves,
`sales-tax-report`, and `user-wise-net-cash` were not examined as promotion
candidates because they are already `projectionStatus = "real"` today via
the `phaseQFinancialOverrides` mechanism (financeMode) — a different,
already-completed promotion path, not part of this pass.

## Net effect

- 2 of the 81 Wave N event-ledger leaves now carry a real, source-backed,
  already-verified column/query contract instead of the generic 6-column
  event-ledger grid: `customer-sales-detail`, `customer-sales-summary`.
- 0 new `salesMode` values, 0 new SQL, 0 changes to the dispatch logic at
  lines 1362-1560, 0 changes to any pre-existing test.
- 7 candidates were examined and explicitly declined with concrete,
  DB-verified or codebase-verified reasons (not hand-waved).
- The literal `projectionStatus` field for both promoted leaves remains
  `"event-ledger"`, matching the codebase's existing, tested convention for
  every `salesReadModel` leaf including the already-accepted `sale-detail`/
  `sale-summary` siblings — this is not a shortfall of this pass, it is how
  the dispatch logic is designed and locked by
  `TestPhaseNReportRegistryDefinitionsAndAggregateFilters`.
- `go vet ./services/api/...` is clean; the full `services/api` test suite
  passes.

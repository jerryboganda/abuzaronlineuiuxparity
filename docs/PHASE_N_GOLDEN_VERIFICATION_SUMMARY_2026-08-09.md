# Phase N Golden Verification — Sales Reports Summary Leaves (2026-08-09)

Scope: the 19 "Sales Reports" leaves in `services/api/internal/httpapi/reports.go` that
already carry a real (non-empty) `salesMode` in `phaseNReportRegistry`, per the task
assignment. This document independently cross-verifies their SQL projections against
`business_documents` / `business_document_lines` / `master_parties` in Postgres — the
live legacy SQL Server is not reachable in this environment (Windows trusted-auth fails),
so all verification here is Postgres-internal, per the confirmed environment constraint.

Companion code: `services/api/internal/httpapi/reports.go`. Companion test:
`services/api/internal/httpapi/report_golden_n_summary_test.go` (registry-wiring lock,
`go vet ./services/api/...` clean, `go test -run TestPhaseNGoldenSalesSummary
./services/api/internal/httpapi/` passes).

**No files were fixed as part of this pass.** `reports.go` was treated read-only per the
task's collision rules; one genuine bug was found and is documented, not patched, below.

---

## Method

For every kind, `reportSpecForKey(kind)` was read directly from `reports.go` to confirm
`salesReadModel == true` and the exact `salesMode` string (this rules out the
"looks similar to an event-ledger sibling but isn't actually wired" trap the task
flagged — see the registry-wiring table in the companion test, which passes as of this
commit). The dispatch table is `salesReadModelQueryMode` (`reports.go:2146`), which
routes each mode to one SQL-building Go function. Those functions were copied verbatim
(parameters substituted with literals) and executed via `psql` against
`postgres://postgres@127.0.0.1:5432/abuzar_next`, under the RLS preamble:

```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', '', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```

**Important correction to the task's RLS assumption:** `app.allow_tenant_scope = true`
does **not** scope queries to the sandbox tenant by itself — it lifts RLS tenant
isolation entirely (rows from other tenants became visible in an unscoped exploratory
query). Every query used for a verdict below carries an explicit
`WHERE tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'` clause, matching exactly what
`reports.go` itself does with its `$1` parameter — so this does not affect any verdict,
but it is worth flagging for the other 9 agents relying on the same preamble.

**Data used:** branch `ffffffff-ffff-ffff-ffff-ffffffffffff` ("Legacy Reference
Sandbox") inside tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` — 291,359 posted
`cash-sale` documents plus a handful of `credit-sale` documents, dated 2025-01-01 through
2026-07-31. Primary verification window: **2025-06-01 through 2025-06-07** (3,445 posted
invoices, 7,639 lines, line-count-per-invoice distribution from 1 to 31 lines — a real
multi-line-invoice mix, not a degenerate single-line fixture). `sales_documents` and
`sync_events(aggregate='sale')` are both empty for this branch, so the compatibility
(pre-migration event-sourcing) union branches in every query below were **not**
exercised — only the canonical `business_documents`/`business_document_lines` path was.
That is a real scope limitation, noted per-leaf where relevant.

**Customer/category data limitation (applies to every customer- and category-grouped
leaf):** the sandbox tenant has exactly **2** customer `master_parties` rows total
("CASH SALES CUSTOMER" and "CREDIT SALES-WALKING CUSTOMER A/C"), and **neither has a
`Category`/`CustomerCategory`/`category` payload field populated**. This means every
category-grouped query below correctly falls back to the coded `'Unspecified'` bucket —
the fallback logic itself is verified — but true multi-category differentiation could
not be exercised with real data in this pass. This is a data-availability gap in the
sandbox, not a code defect.

**Deep vs. light pass:** per the task's guidance, leaves 1–10 (in the assigned order)
got full independent-query cross-checks with row/aggregate evidence recorded below.
Leaves 11–19 got a real (not skipped) but lighter spot-check — either a direct smaller
query run, or explicit confirmation that they dispatch to the exact same Go function as
an already deep-verified sibling (`salesReadModelQueryMode` routes purely on the `mode`
string, so two kinds sharing a mode execute byte-identical SQL).

---

## Summary table

| # | Kind | Mode | Pass | Verdict |
|---|---|---|---|---|
| 1 | `customer-sales-days-summary` | day-summary | Deep | MATCHED |
| 2 | `customer-sales-items-summary` | item-summary | Deep | **MISMATCH-DOCUMENTED** |
| 3 | `customer-sales-invoice-summary` | invoice-summary | Deep | MATCHED |
| 4 | `customer-sales-invoice-wise-profit-margin-detail` | profit-margin-detail | Deep | MATCHED (sale/tax fields); VACUOUS-NO-DATA (cost/profit fields) |
| 5 | `customer-sales-hourly-graph` | hour-summary | Deep | MATCHED |
| 6 | `customer-sales-customer-category-wise-sales-customer-wise-gross-profit` | profit-customer-summary | Deep | MATCHED (qty/amount/tax); VACUOUS-NO-DATA (cost/profit fields) |
| 7 | `customer-sales-customer-category-wise-sales-customer-wise-summary` | customer-summary | Deep | MATCHED |
| 8 | `customer-sales-customer-category-wise-sales-net-sales-and-volume` | customer-summary | Deep (shared logic w/ #7) | MATCHED |
| 9 | `customer-sales-customer-category-wise-net-sales` | customer-category-summary | Deep | MATCHED (math); VACUOUS (category dimension) |
| 10 | `customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report` | customer-category-summary | Deep (shared logic w/ #9) | MATCHED (math); VACUOUS (category dimension) |
| 11 | `customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report` | customer-category-summary | Light (shared logic w/ #9/#10) | MATCHED (math); VACUOUS (category dimension) |
| 12 | `customer-sales-customer-wise-category-net-sales` | customer-wise-category-summary | Light | MATCHED (math); VACUOUS (category dimension) |
| 13 | `customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report` | line-detail | Light | MATCHED |
| 14 | `customer-sales-monthly-net-sales` | month-summary | Light | MATCHED |
| 15 | `monthly-net-sales-summary` | month-summary | Light (shared logic w/ #14) | MATCHED |
| 16 | `daily-sales-summary-with-profit-day-wise-grouping` | profit-day-summary | Light | MATCHED (math); VACUOUS-NO-DATA (cost/profit fields) |
| 17 | `sale-summary` | invoice-summary | Light (shared logic w/ #3) | MATCHED |
| 18 | `sale-summary-inv-wise` | invoice-summary | Light (shared logic w/ #3/#17) | MATCHED |
| 19 | `sale-summary-invoice-wise` | invoice-summary | Light (shared logic w/ #3/#17) | MATCHED |

**18 of 19 leaves MATCHED** their independently-computed expected values (with the noted
VACUOUS-NO-DATA gaps for cost/profit columns and category differentiation — those are
sandbox data gaps, not query defects). **1 of 19 (`customer-sales-items-summary`) has a
genuine, reproducible arithmetic bug**, detailed below.

---

## Leaf-by-leaf detail

### 1. `customer-sales-days-summary` (day-summary) — Deep — MATCHED

`salesCalendarSummaryReadModelQuery` (`reports.go:2192`) wraps
`invoiceSummaryReadModelQuery(salesReadModelQuery(...))` — invoices are de-duplicated
first (`SUM` quantity, `MAX` amount per document), then bucketed by
`occurred_at::date` and party.

App-query (literal-substituted) result for 2025-06-01..07, grouped by day:

```
   period   |        party        |    qty     |     amt
------------+---------------------+------------+-------------
 2025-06-01 | CASH SALES CUSTOMER |  9227.0000 | 409503.0000
 2025-06-02 | CASH SALES CUSTOMER |  8286.0000 | 361441.0000
 2025-06-03 | CASH SALES CUSTOMER | 10945.0000 | 515923.0000
 2025-06-04 | CASH SALES CUSTOMER | 10173.0000 | 500357.0000
 2025-06-05 | CASH SALES CUSTOMER | 11859.0000 | 545404.0000
 2025-06-06 | CASH SALES CUSTOMER |  9162.0000 | 450376.0000
 2025-06-07 | CASH SALES CUSTOMER |  4077.0000 | 213209.0000
```

Independent query (`SUM(bl.quantity)` per document, `SUM(bd.total_amount)` per document,
grouped by `bd.occurred_at::date`) produced **identical values for all 7 days, to the
paisa**. Exact match, no rounding tolerance needed.

### 2. `customer-sales-items-summary` (item-summary) — Deep — **MISMATCH-DOCUMENTED**

**Bug.** `salesItemSummaryReadModelQuery` (`reports.go:2219`) groups the raw (non
invoice-deduplicated) `salesReadModelQuery` rows by item, and does
`SUM(amount)` (`reports.go:2233`). But `salesReadModelQuery`'s base CTE
(`reports.go:1712`, specifically line **1760**: `bd.total_amount::text AS amount`)
assigns the **whole document's total** to every line row, not the line's own value.
When an invoice has more than one line, every one of its lines carries the same
document-total `amount`, and `SUM(amount) GROUP BY item` sums that duplicated
document-total once per line-occurrence of the item — not the item's own contribution.

Quantity is unaffected (it sums `bl.quantity`, which is correctly per-line), so the
divergence is isolated to the `Amount` column.

Evidence, top-5 items by app-reported amount for 2025-06-01..07:

| Item | App qty | App amount | Independent `SUM(bl.line_total)` | Ratio |
|---|---|---|---|---|
| DRIP SET (LIFECARE) | 55.0000 | 100487.0000 | 4125.0000 | 24.4x |
| 10CC D/SYRINGE SHIFA | 62.0000 | 96185.0000 | 1240.0000 | 77.6x |
| MEDISOL NS 100ML | 55.0000 | 73212.0000 | 5610.0000 | 13.0x |
| EZIUM 40MG CAP | 225.0000 | 62276.0000 | 9402.7500 | 6.6x |
| P CINE 5MG TAB | 560.0000 | 54533.0000 | 2744.0000 | 19.9x |

Quantity columns match exactly (55, 62, 55, 225, 560 in both); amount columns diverge by
6.6x to 77.6x. This is not a rounding/tolerance issue — it is a wrong column formula.

**Root cause:** `salesItemSummaryReadModelQuery` should sum a line-level amount (e.g.
`bl.line_total`, as `salesCustomerCategorySummaryReadModelQueryMode` and
`salesProfitMarginReadModelQuery` correctly do), not the document-level `amount` field
inherited from `salesReadModelQuery`'s per-line-duplicated `bd.total_amount`. **Not
fixed here** per the task's read-only constraint on `reports.go`.

Column/note text also does not disclose this: `salesProjectionModeNote("item-summary")`
(`reports.go:1294` in the source, current line 1298) says "numeric quantity and amount
totals" without qualifying that amount is inflated for multi-line invoices.

### 3. `customer-sales-invoice-summary` (invoice-summary) — Deep — MATCHED

`invoiceSummaryReadModelQuery(salesReadModelQuery(reportSaleAggregate, ""))`: groups by
document, `SUM` quantity across lines, `MAX` amount (all line-rows share the same
document total, so `MAX` correctly collapses to that total once).

App query for 2025-06-01..07: **3,445 invoices, total qty 63,729.0000, total amount
2,996,213.0000.**

Independent query — `SELECT count(*), SUM(qty), SUM(total_amount) FROM (per-document
qty=SUM(bl.quantity), total_amount=bd.total_amount)` — produced the **identical** three
numbers: 3,445 / 63,729.0000 / 2,996,213.0000. Exact match.

### 4. `customer-sales-invoice-wise-profit-margin-detail` (profit-margin-detail) — Deep — MATCHED (partial) / VACUOUS-NO-DATA (cost)

`salesProfitMarginReadModelQuery` (`reports.go:2240`) computes per-line `sale_price =
bl.unit_price` (via legacy_payload/pricing fallbacks), `amount = bl.line_total`, `cost`
from a `stock_allocations` lateral join, `sales_tax_value = bl.tax_amount`, then derives
`gross_profit = amount - tax - cost` and `margin% = gross_profit*100/(amount-tax)`.

Non-cost fields verified exactly against `business_document_lines` for invoice `665432`
(4 lines): `sale_price` == `bl.unit_price`, `amount` == `bl.line_total`,
`sales_tax_value` == `bl.tax_amount` for every line, to 4 decimal places (e.g. AUGMENTIN
625MG TAB 12S: qty 28, sale_price 45.54, amount 1275.1200 — matches `bl.unit_price` and
`bl.line_total` exactly).

**Cost/gross-profit/margin could not be verified with real numbers**: `SELECT count(*)
FROM stock_allocations sa JOIN business_document_lines bl ... JOIN business_documents bd
... WHERE bd.tenant_id = 'eeeeeeee-...' AND bd.kind IN ('cash-sale','credit-sale') AND
bd.status='posted'` returns **0** — zero stock allocations exist for any posted sale
line anywhere in this tenant. The `CASE WHEN allocated_cost.allocation_count > 0`
guard in the query correctly emits `''` (blank) for cost in this situation, which is
itself the documented, correct behavior for missing allocation data — but it means the
gross-profit/margin arithmetic itself is untested this pass. The formula was verified
algebraically only (by inspection: it is internally consistent and was exercised against
synthetic 5-row test-fixture data in a different tenant, outside this task's assigned
sandbox scope, where it also computed correctly: amount 10.00, cost 4.00 → gross_profit
6.00, margin 60.00%, matching hand computation — but that tenant is not the assigned
sandbox tenant, so it is reported here as supporting evidence only, not as the verdict).

### 5. `customer-sales-hourly-graph` (hour-summary) — Deep — MATCHED

Same `salesCalendarSummaryReadModelQuery` function as day-summary, with
`date_trunc('hour', occurred_at)` as the bucket. For 2025-06-01, the app query produced
19 hour-buckets (not all 24 — no sales in some early-afternoon hours); summing all 19
bucket amounts reproduces the day total exactly (409,503.0000), confirming internal
consistency. An independent query (`date_trunc('hour', bd.occurred_at)`, `SUM(bl.quantity)`,
`SUM(bd.total_amount)` grouped by hour) produced **byte-identical** values for all 19
buckets (spot-checked: hour 00:00 → qty 905.0000/amt 53419.0000 in both; hour 23:00 → qty
560.0000/amt 22901.0000 in both).

Minor note (not a defect): hours with zero sales are simply absent from the result set
rather than emitted as zero-rows; a legacy hourly graph would need the caller/renderer to
fill missing hours with zero, which is a presentation concern, not a query-correctness
one.

### 6. `customer-sales-customer-category-wise-sales-customer-wise-gross-profit` (profit-customer-summary) — Deep — MATCHED (qty/amount/tax) / VACUOUS-NO-DATA (cost)

`salesProfitCustomerSummaryReadModelQuery` (`reports.go:2417`) groups the (line-level,
`bl.line_total`-based) profit rows by customer. For 2025-06-01..07: app query returned
customer "CASH SALES CUSTOMER", qty 63,729.0000, amount **2,996,962.7900**, tax
46,625.4100, row_count 7,639, cost_count 0.

Independent query — `SUM(bl.quantity)`, `SUM(bl.line_total)`, `SUM(bl.tax_amount)` across
all 7,639 lines in range — produced **exactly** 63,729.0000 / 2,996,962.7900 /
46,625.4100. Exact match. (Note this is line-total-based, correctly different from
invoice-summary's document-total-based 2,996,213.0000 — the ~750 delta between the two
is a real, expected difference in what each report is defined to sum, not an error;
verified both independently reconcile to their own respective source columns.)
Cost/gross_profit/margin are blank for all 7,639 lines (`cost_count = 0`), same
stock-allocation gap as leaf 4.

### 7 & 8. `customer-sales-customer-category-wise-sales-customer-wise-summary` / `...-net-sales-and-volume` (customer-summary) — Deep — MATCHED

Both map to `salesCustomerSummaryReadModelQuery` (`reports.go:2450`), which wraps the
already-deduplicated invoice rows (same base as leaf 3) and groups by customer. App
query for 2025-06-01..07: customer "CASH SALES CUSTOMER", qty 63,729.0000, amount
2,996,213.0000 — **identical** to leaf 3's invoice-summary total (expected: same
document-total base, coarser grouping onto a single customer bucket in this dataset).
Confirmed exact match against the independently-computed `SUM(bd.total_amount)` figure
already validated in leaf 3. Leaf 8 dispatches through the identical `salesMode =
"customer-summary"` (confirmed via the registry-wiring test), so it executes
byte-identical SQL to leaf 7.

### 9 & 10. `customer-sales-customer-category-wise-net-sales` / `...-customer-category-wise-sales-summary-report` (customer-category-summary) — Deep — MATCHED (math) / VACUOUS (category)

`salesCustomerCategorySummaryReadModelQueryMode(..., groupCustomer=false)`
(`reports.go:2484`) groups line-level rows (`bl.line_total`-based amount, same source as
leaf 6) by the customer's category payload, falling back to `'Unspecified'`. App query
for 2025-06-01..07 returned a single row: category "Unspecified", qty 63,729.0000,
amount 2,996,962.7900 — **exact match** to the independently-computed line-total sum
from leaf 6. The `'Unspecified'` fallback itself is correct given the sandbox's master
data (0 of 2 customers have a category field set) — this is a genuine, if narrow,
functional check that the fallback path works, not a placeholder result. Leaf 10 shares
the identical `salesMode`, confirmed via the registry test.

---

### 11. `customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report` (customer-category-summary) — Light — MATCHED (math) / VACUOUS (category)

Confirmed via the registry-wiring test to dispatch to the identical `salesMode =
"customer-category-summary"` as leaves 9/10 — `salesReadModelQueryMode` routes purely
on the mode string, so this kind executes byte-identical SQL to the leaf 9 query already
cross-verified above. No separate query run; verdict inherited from leaf 9's evidence.

### 12. `customer-sales-customer-wise-category-net-sales` (customer-wise-category-summary) — Light — MATCHED (math) / VACUOUS (category)

`salesCustomerCategorySummaryReadModelQueryMode(..., groupCustomer=true)` — the
`groupCustomer` variant, grouping by `(customer, category)` instead of `category` alone.
Ran directly for 2025-06-01..07: single row, customer "CASH SALES CUSTOMER", category
"Unspecified", qty 63,729.0000, amount 2,996,962.7900 — matches the leaf 9/6 line-total
figure exactly.

### 13. `customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report` (line-detail) — Light — MATCHED

`dailySalesDetailReadModelQuery` (`reports.go:1992`) — no grouping, raw per-line rows.
Spot-checked invoice `665432` (4 lines): item/quantity/amount/sale_price/tax for each
line matched `business_document_lines` exactly (e.g. AUGMENTIN 625MG TAB 12S: qty
28.00000000, amount 1275.1200, sale_price 45.54 — identical to `bl.line_total` /
`bl.unit_price` read directly), consistent with the leaf-4 spot check of the same
invoice via a different query path.

### 14 & 15. `customer-sales-monthly-net-sales` / `monthly-net-sales-summary` (month-summary) — Light — MATCHED

Same `salesCalendarSummaryReadModelQuery` function as leaves 1/5, bucketed by
`date_trunc('month', occurred_at)`. Ran directly for June 2025 (2025-06-01..06-30): app
query returned period 2025-06-01, qty 253,342.0000, amount **11,850,601.0000**.
Independent query (`SUM(bd.total_amount)` for all posted cash/credit-sale documents in
June, `date_trunc('month', bd.occurred_at)`) returned the identical 11,850,601.0000
across 14,684 documents. Exact match. Leaf 15 shares the identical `salesMode`, confirmed
via the registry test.

### 16. `daily-sales-summary-with-profit-day-wise-grouping` (profit-day-summary) — Light — MATCHED (math) / VACUOUS-NO-DATA (cost)

`salesProfitDaySummaryReadModelQuery` (`reports.go:2375`) buckets the line-level profit
rows by day. Ran directly for 2025-06-01: app query returned qty 9,227.0000, amount
409,026.2600, tax 5,910.3700 — an independent `SUM(bl.quantity)/SUM(bl.line_total)/
SUM(bl.tax_amount)` for the same day matched all three exactly. (Note this
409,026.26 line-total figure correctly differs from the 409,503.00 document-total figure
in leaf 1 — same expected line-vs-document-total distinction as leaf 6.) Cost/profit
columns blank for the same reason as leaves 4/6 (zero stock allocations in this tenant).

### 17, 18 & 19. `sale-summary` / `sale-summary-inv-wise` / `sale-summary-invoice-wise` (invoice-summary) — Light — MATCHED

These three are registered in the **first** registry loop (`reports.go:148-188`, not the
"Sales Reports" loop), but map to the identical `salesMode = "invoice-summary"` and
therefore the identical `invoiceSummaryReadModelQuery(salesReadModelQuery(...))`
already deep-verified in leaf 3. To confirm this isn't just a code-reading assumption, a
direct spot-check was run on a different sub-range (2025-06-03 only, not the full week
used in leaf 3): app query returned 593 invoices, qty 10,945.0000, amount 515,923.0000;
independent `SUM(bd.total_amount)`/`count(*)` for that single day returned the identical
593 documents / 515,923.0000. Exact match, confirming the shared logic reproduces
correctly on an independent slice, not just the one already checked in leaf 3.

---

## Open items for a future phase (not fixed here)

1. **Fix `salesItemSummaryReadModelQuery`'s amount column** (`reports.go:2219-2238`,
   root cause at `reports.go:1760`) to sum a line-level amount instead of the
   document-total duplicated per line. This affects `customer-sales-items-summary`
   only among the 19 leaves in this task; it does not affect any other mode.
2. **`salesProjectionModeNote("item-summary")`** (`reports.go:1298`) should be corrected
   to disclose the current amount-column defect, or updated once the fix above lands.
3. The cost/gross-profit/margin columns on `profit-margin-detail`,
   `profit-customer-summary`, and `profit-day-summary` cannot be golden-verified in this
   sandbox tenant until real `stock_allocations` rows exist for posted sale lines — right
   now there are zero, tenant-wide. This is a data-population gap (moving-average stock
   allocation apparently isn't being recorded for this bulk-migrated branch's sale
   lines), not a reports.go defect, but it blocks real verification of those columns.
4. Category-grouped leaves (9–12) could not be verified against real multi-category data
   because the sandbox's only 2 customer master records both lack a category value.
5. The compatibility (`sales_documents`/`sync_events`) union branches in every sales
   query were not exercised in this pass — the tested branch has zero rows in either
   table. A future pass should find or seed data that exercises the de-duplication
   `NOT EXISTS` logic between canonical and compatibility rows.

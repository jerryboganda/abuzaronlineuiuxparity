# Phase N golden verification — Category Wise sub-group B — 2026-08-09

Scope: 4 report leaves in the "Category Wise" sub-menu of Sales Reports
(`services/api/internal/httpapi/reports.go` lines 221-317,
`phaseNReportRegistry`), assigned as a diagnostic-first pass:

1. `category-wise-gross-profit`
2. `category-wise-item-wise-sale-discounts-detail`
3. `category-wise-item-category-wise-monthly-sales`
4. `category-wise-category-wise-day-net-sale`

This is a **read-only diagnostic document**. `reports.go` was not modified —
per the wave-2 collision rules, that file is owned by a different concurrent
agent (the one permitted to promote leaves). This doc records exact query
shapes so that agent (or a future pass) can promote these leaves with
confidence, and confirms the current fallback does not error for the
sandbox tenant.

Environment: live legacy SQL Server unreachable, so verification is
Postgres-internal cross-checking against `abuzar_next`, tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch
`ffffffff-ffff-ffff-ffff-ffffffffffff` (`legacy-reference-sandbox`,
620,615 posted cash/credit sale lines). Connected as the `postgres`
superuser, which bypasses RLS entirely (table owner + superuser bypass even
with `FORCE ROW LEVEL SECURITY`) — every query below therefore filters
`tenant_id`/`branch_id` explicitly rather than relying on the RLS
`set_config` preamble to scope results.

## Headline finding: the COGS/gross-profit data gap is more nuanced than wave-1 reported

Wave-1's N-summary agent found **zero `stock_allocations` rows for the
sandbox tenant**, and concluded this blocks gross-profit for any report
family needing a cost basis. That specific fact is re-confirmed here:

```sql
SELECT count(*) FROM stock_allocations WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';
-- 0
SELECT count(*) FROM stock_allocations;
-- 591, all in small (1-5 row) synthetic test-fixture tenants, none in the sandbox
```

This directly affects the codebase's *existing* gross-profit mechanism:
`salesProfitMarginReadModelQuery` (`reports.go` lines 2388-2521, backing the
already-`salesReadModel`-wired `customer-sales-invoice-wise-profit-margin-detail`
and `customer-sales-customer-category-wise-sales-customer-wise-gross-profit`
leaves) computes `cost` **exclusively** from a `LATERAL` join to
`stock_allocations` (lines 2413-2419: `SUM(sa.quantity * sa.unit_cost) ...
FROM stock_allocations sa WHERE sa.document_line_id = bl.id`). With 0 rows
for this tenant, `allocation_count` is always 0, so `cost` is always the
empty string `''`, and the `gross_profit`/`margin_percent` columns
(lines 2498-2513) always evaluate to `''` too — **those two already-shipped
leaves currently return blank gross-profit/margin columns for every row in
this tenant**, not because the code is wrong, but because their one data
source is empty here. This is worth flagging even though neither leaf is in
this wave-2 assignment.

**However, a second, populated cost source exists that the existing
gross-profit query does not use:** `stock_ledger`. Every posted sale line's
stock issue is also recorded there, with `unit_cost` and a direct
`source_document_line_id` back-reference — independent of the (empty)
`stock_allocations` table:

```sql
SELECT sl.direction, count(*), count(*) FILTER (WHERE sl.source_document_line_id IS NOT NULL) has_line,
       count(*) FILTER (WHERE sl.unit_cost > 0) has_cost
FROM stock_ledger sl
JOIN business_documents bd ON bd.tenant_id=sl.tenant_id AND bd.id = sl.source_document_id
WHERE sl.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted'
GROUP BY sl.direction;
```
```
 direction | count  | has_line | has_cost
-----------+--------+----------+----------
 out       | 620615 |   620615 |   620613
```

`620,615` = the exact total count of posted sale lines in this tenant
(verified separately), and a distinct-count check confirms 1:1 coverage,
not incidental overlap:

```sql
SELECT count(DISTINCT sl.source_document_line_id)
FROM stock_ledger sl JOIN business_documents bd ON bd.tenant_id=sl.tenant_id AND bd.id=sl.source_document_id
WHERE sl.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND sl.direction='out'
  AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted';
-- 620615
```

Only 2/620,615 lines (0.0003%) have `unit_cost = 0` (plausible free/zero-cost
items). This matches Phase J's separately-verified claim that "moving-average
valuation is real, tested, and wired into sale/purchase-return costing" —
`stock_ledger` is that wiring's output, `stock_allocations` is a separate,
apparently-unpopulated-for-this-tenant mechanism.

**Verdict on the gross-profit/COGS gap: partially confirmed, partially
refuted.** `stock_allocations` is confirmed still empty tenant-wide (wave-1
was right about that fact, and it still blocks any leaf whose query
literally depends on that specific table, including the two already-shipped
`salesProfitMarginReadModelQuery`-backed leaves above). But it is **not**
true that gross-profit is unrecoverable for this tenant in general — a real,
populated, verified per-line cost source (`stock_ledger.unit_cost` joined by
`source_document_line_id`) exists and produces sane numbers (see leaf 1
below). This is a meaningfully different, more actionable finding than "the
data doesn't exist" and is worth surfacing to whichever agent/pass next
touches `salesProfitMarginReadModelQuery` too, since it affects more than
just this wave's 4 assigned leaves.

---

## Second headline finding: `master_categories` resolves `ICatCode` to real names — the wave-1 "no category name" blocker for the whole Category Wise submenu is stale

`docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md` declined `category-wise-sales`
partly because "`master_items.payload` has 0/30,052 rows with a populated
`Category` name key — only the raw `ICatCode` legacy code is populated". That
is correct as stated (confirmed again here), but it examined only
`master_items.payload` and missed that a **separate, real lookup table**
already exists and is already populated:

```sql
\d master_categories
--  category_kind | legacy_id | code | name | payload | active | ...

SELECT category_kind, count(*) FROM master_categories GROUP BY category_kind;
--  item_category | 14   (7 distinct codes x 2, once per tenant in this DB — 7 for the sandbox tenant alone)

SELECT code, name FROM master_categories
WHERE category_kind='item_category' AND tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' ORDER BY code::int;
```
```
 code |       name
------+------------------
 1    | MEDICINES
 2    | NARCOTICS
 3    | CONSUMER
 4    | COUNSELING
 5    | DIAGNOSTIC
 6    | MILK
 7    | DR KHALID MUGHAL
```

And `master_items.payload->>'ICatCode'` for the sandbox tenant's 30,052 items
uses **exactly** these 7 codes, with 100% resolution:

```sql
SELECT count(*) items_total, count(*) FILTER (WHERE mc.id IS NOT NULL) items_resolved
FROM master_items mi
LEFT JOIN master_categories mc
  ON mc.tenant_id = mi.tenant_id AND mc.category_kind = 'item_category' AND mc.code = mi.payload->>'ICatCode'
WHERE mi.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';
-- items_total 30052 | items_resolved 30052
```

This is the missing piece that makes a real, category-name-grouped query
possible for all 4 of this wave's leaves (and arguably for
`category-wise-sales`/`category-wise-net-sale`/`category-wise-monthly-sale`/
`category-wise-deviated-items` too, though those are outside this
assignment). Join shape, reusable in any leaf that needs an item's category
name:

```sql
LEFT JOIN master_items mi ON mi.tenant_id = bl.tenant_id AND mi.id = bl.item_id
LEFT JOIN master_categories mc
  ON mc.tenant_id = mi.tenant_id AND mc.category_kind = 'item_category' AND mc.code = mi.payload->>'ICatCode'
-- category name: COALESCE(mc.name, 'Unspecified')
```

This exactly mirrors the existing `bl.item_id -> master_items mi` join
pattern already used at `reports.go:2046` (`dailySalesDetailReadModelQuery`)
and `reports.go:2827` — no new join *technique* is introduced, only one more
hop to a table (`master_categories`) that other report families in this file
(`stockSupplierManufacturerReadModelQuery` sibling area) don't yet use for
categories specifically but which clearly exists and is populated for this
purpose (`category_kind = 'item_category'`).

**Caveat:** this join only resolves category for lines whose
`business_document_lines.item_id` is populated — i.e. the canonical,
already-normalized rows. The event/compatibility fallback branches
(`sales_documents`/`sync_events`, used for pre-normalization or replay-only
rows) carry no `item_id`, so those rows would fall back to `'Unspecified'`
in any category-grouped query — the same limitation the existing
`salesItemSummaryReadModelQuery` and `salesProfitMarginReadModelQuery`
already have for their own compatibility branches.

---

## Leaf 1: `category-wise-gross-profit` ("Gross Profit")

**Current status:** confirmed pure event-ledger. `reports.go:281` registers
it in the Sales Reports loop with `aggregateCondition = reportSaleAggregate`;
it does **not** appear in the `salesMode` switch (`reports.go:294-319`), so
`salesReadModelQueryMode` falls through to the generic
`salesReadModelQuery(aggregateCondition, pagination)` — the 6-column
event-ledger grid (document/date/party/item/quantity/amount), no cost or
profit column at all today.

**Diagnostic findings:**
- `stock_allocations` (the existing gross-profit mechanism's data source) is
  confirmed empty for this tenant — see headline finding above.
- `stock_ledger` (an independent, populated cost source) resolves COGS for
  620,613/620,615 posted sale lines (99.9997%) via
  `source_document_line_id` + `unit_cost` on `direction='out'` rows.
- `master_categories` resolves 100% of items' `ICatCode` to a real category
  name (see headline finding above).

**Recommended query shape** (verified to run and produce sane numbers for
`occurred_at IN ['2026-07-01','2026-07-03']`, sandbox tenant/branch):

```sql
WITH cogs_by_line AS (
  SELECT sl.source_document_line_id, SUM(sl.quantity * sl.unit_cost) AS cost
  FROM stock_ledger sl
  WHERE sl.tenant_id = $1::uuid AND sl.branch_id = $2::uuid AND sl.direction = 'out'
    AND sl.occurred_at >= $3::date AND sl.occurred_at < ($4::date + INTERVAL '1 day')
  GROUP BY sl.source_document_line_id
),
line_rows AS (
  SELECT bd.occurred_at,
         COALESCE(mc.name, 'Unspecified') AS category_name,
         bl.quantity,
         COALESCE(NULLIF(bl.legacy_payload->>'Amount','')::numeric, bl.line_total, bd.total_amount) AS amount,
         COALESCE(bl.tax_amount, 0) AS tax_amount,
         cogs.cost
  FROM business_documents bd
  JOIN business_document_lines bl ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id AND bl.document_id = bd.id
  LEFT JOIN master_items mi ON mi.tenant_id = bl.tenant_id AND mi.id = bl.item_id
  LEFT JOIN master_categories mc ON mc.tenant_id = mi.tenant_id AND mc.category_kind = 'item_category' AND mc.code = mi.payload->>'ICatCode'
  LEFT JOIN cogs_by_line cogs ON cogs.source_document_line_id = bl.id
  WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
    AND bd.kind IN ('cash-sale', 'credit-sale') AND bd.status = 'posted'
    AND bd.occurred_at >= $3::date AND bd.occurred_at < ($4::date + INTERVAL '1 day')
)
SELECT category_name, SUM(quantity), SUM(amount), SUM(tax_amount),
       SUM(COALESCE(cost,0)) AS cogs,
       SUM(amount - tax_amount - COALESCE(cost,0)) AS gross_profit
FROM line_rows GROUP BY category_name ORDER BY category_name;
```

Verified output (2026-07-01 to 2026-07-03, 7 categories, all with 0 lines
missing cost in this window):

```
 category_name    |      qty       | gross_amount |    tax     |      cogs        |  gross_profit
-------------------+----------------+--------------+------------+------------------+------------------
 CONSUMER          |    228.00000000|   15613.8700 |   493.5400 |  12644.91450000  |   2475.41550000
 COUNSELING        |      3.00000000|     370.0000 |     0.0000 |    191.71490000  |    178.28510000
 DIAGNOSTIC        |      4.00000000|   10900.0000 |     0.0000 |   9500.00000000  |   1400.00000000
 DR KHALID MUGHAL  |   1762.00000000|   57023.0600 |     0.0000 |  48501.66880000  |   8521.39120000
 MEDICINES         |  17763.00000000|  999848.1700 | 13750.4900 | 839377.60210000  | 146720.07790000
 MILK              |      2.00000000|    2420.0000 |   391.9400 |   2255.95000000  |   -227.89000000
 NARCOTICS         |   1140.00000000|   17713.6100 |     0.0000 |  14996.52080000  |   2717.08920000
```

Numbers are internally consistent (gross_amount - tax - cogs = gross_profit
to the cent in every row) and mostly positive with plausible margins; one
category (MILK) nets slightly negative for this 3-day window, which is
plausible (a promotional/loss-leader sale or a moving-average cost blip) but
not independently confirmed against a legacy source — flagged, not
resolved.

**Verdict: NOT actually blocked by "zero stock_allocations", contrary to the
wave-1 top-line summary.** A real, populated, verified cost basis exists via
`stock_ledger`. A promotion is technically buildable. What remains open
(same caveat every other N/O/P/Q doc carries) is that the *exact* legacy
"Gross Profit" report's column set, rounding, and whether it nets tax the
same way is unconfirmed — no legacy screenshot/capture exists to pin the
exact shape, so this is "a real query exists" not "byte-verified against
legacy." Recommend promoting only if a screenshot/capture surfaces to
confirm column layout, or accepting this query shape as a documented
best-effort mapping.

---

## Leaf 2: `category-wise-item-wise-sale-discounts-detail` ("Item Wise Sale Discounts Detail")

**Current status:** confirmed pure event-ledger, same mechanism as leaf 1
(`reports.go:282`, not in the `salesMode` switch).

**Diagnostic findings:**
- Title/submenu mismatch already flagged by wave-1's promotion doc ("Item
  Wise" nested under "Category Wise") is real and still unresolved — could
  mean a category-grouped discount summary or an item-level line-detail
  listing filtered to discounted lines. `master_categories` now makes a
  category-grouped variant buildable if that's the intended shape.
- **The bigger blocker for this specific leaf: there is no discount data to
  show, in either shape.** Checked directly:
  ```sql
  SELECT count(*) FILTER (WHERE bl.line_gross > bl.line_total) AS lines_with_discount,
         count(*) FILTER (WHERE bl.item_discount > 0) AS lines_with_item_discount,
         count(*) total_lines
  FROM business_documents bd
  JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
  WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.branch_id='ffffffff-ffff-ffff-ffff-ffffffffffff'
    AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted';
  ```
  ```
   lines_with_discount | lines_with_item_discount | total_lines
  ----------------------+--------------------------+-------------
                       0 |                        0 |      620615
  ```
  **0 of 620,615 posted sale lines have any line-level or item-level
  discount recorded, tenant-wide.** This mirrors the pattern wave-1 already
  documented for other leaves as `VACUOUS-NO-DATA` — even a correctly-built
  query would render an empty grid for this tenant, because the underlying
  discount data genuinely isn't populated here (not a query bug).

**Recommended query shape** (line-detail with category, verified to run —
returns 0 rows only because the discount filter matches nothing, not because
of an error):

```sql
SELECT bd.document_number, bd.occurred_at::date,
       COALESCE(mc.name,'Unspecified') AS category_name,
       bl.item_name, bl.quantity,
       COALESCE(NULLIF(bl.legacy_payload->>'DiscPerc',''), '0.00') AS disc_percent,
       CASE WHEN bl.line_gross >= bl.line_total THEN (bl.line_gross - bl.line_total) ELSE 0 END AS discount_value,
       COALESCE(bl.item_discount, 0) AS item_discount,
       bl.line_total AS amount
FROM business_documents bd
JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
LEFT JOIN master_items mi ON mi.tenant_id=bl.tenant_id AND mi.id=bl.item_id
LEFT JOIN master_categories mc ON mc.tenant_id=mi.tenant_id AND mc.category_kind='item_category' AND mc.code = mi.payload->>'ICatCode'
WHERE bd.tenant_id=$1::uuid AND bd.branch_id=$2::uuid AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted'
  AND bd.occurred_at >= $3::date AND bd.occurred_at < ($4::date + INTERVAL '1 day')
ORDER BY bd.occurred_at DESC;
```
Without a discount filter it returns real rows (all with `disc_percent =
'0.00'`, `discount_value = 0`, `item_discount = 0`) — confirming the query
itself is sound, the data is simply flat.

**Verdict: query shape is real and buildable (title ambiguity from wave-1
can be resolved to a line-detail-with-category-name shape using the new
`master_categories` join), but the leaf would be VACUOUS-NO-DATA for this
tenant regardless of which of the two plausible shapes is chosen** — this
tenant's migrated data has no discounts recorded anywhere on sale lines.
Not a code defect; a data-population characteristic of this sandbox. A
promotion here could be technically correct and still look "broken" (empty
grid) to anyone eyeballing it against this tenant, so any future promotion
should note this explicitly.

---

## Leaf 3: `category-wise-item-category-wise-monthly-sales` ("Item Category Wise Monthly Sales")

**Current status:** confirmed pure event-ledger (`reports.go:283`, not in
the `salesMode` switch).

**Diagnostic findings:** the title unambiguously names a category+month
grouping — no shape ambiguity like leaf 2. `master_categories` resolves
100% of items' category as shown in the headline finding.

**Recommended query shape** (verified to run and produce real, sane
category+month totals):

```sql
WITH line_rows AS (
  SELECT bd.occurred_at,
         COALESCE(mc.name, 'Unspecified') AS category_name,
         bl.quantity,
         COALESCE(NULLIF(bl.legacy_payload->>'Amount','')::numeric, bl.line_total, bd.total_amount) AS amount
  FROM business_documents bd
  JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
  LEFT JOIN master_items mi ON mi.tenant_id=bl.tenant_id AND mi.id=bl.item_id
  LEFT JOIN master_categories mc ON mc.tenant_id=mi.tenant_id AND mc.category_kind='item_category' AND mc.code = mi.payload->>'ICatCode'
  WHERE bd.tenant_id=$1::uuid AND bd.branch_id=$2::uuid AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted'
)
SELECT category_name, date_trunc('month', occurred_at::date)::date AS month,
       SUM(quantity) AS qty, SUM(amount) AS amount
FROM line_rows
WHERE occurred_at >= $3::date AND occurred_at < ($4::date + INTERVAL '1 day')
GROUP BY category_name, month
ORDER BY month DESC, category_name;
```

Verified output (2026-07-01 to 2026-07-31):

```
   category_name   |   month    |       qty       |    amount
------------------+------------+------------------+--------------
 CONSUMER         | 2026-07-01 |   2148.00000000  |  174373.4700
 COUNSELING       | 2026-07-01 |    445.00000000  |    8243.8500
 DIAGNOSTIC       | 2026-07-01 |    145.00000000  |  137855.0000
 DR KHALID MUGHAL | 2026-07-01 |  18480.00000000  |  553882.1400
 MEDICINES        | 2026-07-01 | 157018.00000000  | 8131311.0000
 MILK             | 2026-07-01 |     22.00000000  |   29650.0000
 NARCOTICS        | 2026-07-01 |  12340.00000000  |  194169.1800
```

Note the `amount` column deliberately prefers `bl.line_total` over
`bd.total_amount` (the same fix pattern wave-1 applied to
`customer-sales-items-summary` after finding the opposite ordering inflated
amounts 6.6x-77.6x) — a document-total fallback is only used when a line
lacks its own `line_total`.

**Verdict: real, buildable, non-vacuous query exists** using the new
`master_categories` join, with real category+month aggregates. This is the
strongest promotion candidate of the 4 assigned leaves — unambiguous title,
populated data, a query shape that mirrors already-accepted patterns
elsewhere in this file (`salesCalendarSummaryReadModelQuery`'s `month`
bucket, `salesItemSummaryReadModelQuery`'s corrected amount-column
precedence). Still lacks a legacy screenshot to pin exact column
labels/order, so calling it "legacy-byte-verified" would be an overclaim —
but the aggregation itself is sound and safe to promote.

---

## Leaf 4: `category-wise-category-wise-day-net-sale` ("Category Wise Day Net Sale")

**Current status:** confirmed pure event-ledger (`reports.go:284`, not in
the `salesMode` switch).

**Diagnostic findings:** title unambiguously names a category+day grouping
with a "net" (post-tax) amount. Same category resolution as leaf 3 applies.

**Recommended query shape** (verified to run and produce real, sane
category+day totals with a tax-netted figure):

```sql
WITH line_rows AS (
  SELECT bd.occurred_at,
         COALESCE(mc.name, 'Unspecified') AS category_name,
         bl.quantity,
         COALESCE(NULLIF(bl.legacy_payload->>'Amount','')::numeric, bl.line_total, bd.total_amount) AS amount,
         COALESCE(bl.tax_amount, 0) AS tax_amount
  FROM business_documents bd
  JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
  LEFT JOIN master_items mi ON mi.tenant_id=bl.tenant_id AND mi.id=bl.item_id
  LEFT JOIN master_categories mc ON mc.tenant_id=mi.tenant_id AND mc.category_kind='item_category' AND mc.code = mi.payload->>'ICatCode'
  WHERE bd.tenant_id=$1::uuid AND bd.branch_id=$2::uuid AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted'
)
SELECT category_name, occurred_at::date AS day,
       SUM(quantity) AS qty, SUM(amount) AS gross_amount, SUM(tax_amount) AS tax,
       SUM(amount - tax_amount) AS net_sale
FROM line_rows
WHERE occurred_at >= $3::date AND occurred_at < ($4::date + INTERVAL '1 day')
GROUP BY category_name, day
ORDER BY day DESC, category_name;
```

Verified output (2026-07-01 to 2026-07-03, 18 category-days):

```
  category_name    |    day     |      qty      | gross_amount |    tax    |  net_sale
--------------------+------------+---------------+--------------+-----------+-------------
 CONSUMER          | 2026-07-03 |   33.00000000 |    6043.3800 |  389.7800 |   5653.6000
 MEDICINES         | 2026-07-03 | 5814.00000000 |  300218.5700 | 7254.6200 | 292963.9500
 ... (7 categories x 3 days, all internally consistent: gross - tax = net to the cent)
```

**Verdict: real, buildable, non-vacuous query exists**, same confidence
level as leaf 3 — unambiguous title, populated tax and amount data, sound
aggregation reusing the existing calendar-bucket pattern
(`salesCalendarSummaryReadModelQuery`'s `day` bucket) plus the new category
join. Same "not legacy-byte-verified" caveat as every other leaf in N/O/P/Q.

---

## Confirming the event-ledger fallback works today (all 4 leaves)

All 4 leaves share `aggregateCondition = reportSaleAggregate` (none of their
titles contain "return") and none appear in the `salesMode` switch, so all 4
currently execute the **identical** fallback query,
`salesReadModelQuery(reportSaleAggregate, pagination)` (`reports.go:1712`).
Ran that exact SQL (copied verbatim from source, params substituted) against
the sandbox tenant/branch for `2026-07-01`..`2026-07-31`:

```sql
WITH sales_read_model AS ( ... )   -- verbatim from reports.go:1751-1828
SELECT document, occurred_at::text, party, item, quantity, amount
FROM sales_read_model
WHERE occurred_at >= '2026-07-01'::date AND occurred_at < '2026-07-31'::date + INTERVAL '1 day'
  AND (''='' OR ...) ORDER BY occurred_at DESC, document, item LIMIT 5;
```

```
 document | occurred_at             | party                | item                          | quantity    | amount
----------+-------------------------+----------------------+-------------------------------+-------------+----------
 880233   | 2026-07-31 23:41:03+05  | CASH SALES CUSTOMER  | GASTOM 40MG CAP               | 14.00000000 | 420.0000
 880232   | 2026-07-31 23:40:52+05  | CASH SALES CUSTOMER  | NESTLE 0.5LTR                 | 1.00000000  | 341.0000
 ...
```

**Confirmed: the event-ledger fallback runs error-free and returns real
rows for all 4 assigned leaves in the sandbox tenant** — no `SQLSTATE`
error, no empty-result surprise for a well-populated date range. This
matches wave-1's general finding that the event-ledger fallback mechanism
itself (as opposed to the specific promoted-vs-generic column shape) is
sound.

`go vet ./services/api/...` — clean, no output, exit 0 (ran read-only, no
source changes made in this pass).

---

## Summary table

| Leaf | Current status | Blocked by COGS gap? | Blocked by category-name gap? | Real query found? | Data-population caveat |
|---|---|---|---|---|---|
| `category-wise-gross-profit` | event-ledger | No — `stock_allocations` empty but `stock_ledger` (99.9997% coverage) works | No — `master_categories` resolves 100% | Yes, verified | One category (MILK) nets slightly negative in the sampled window — plausible, unconfirmed |
| `category-wise-item-wise-sale-discounts-detail` | event-ledger | N/A | No — resolvable, though title/submenu shape is still ambiguous | Yes, verified (returns 0 rows under a discount filter) | **VACUOUS-NO-DATA**: 0/620,615 lines have any discount recorded tenant-wide |
| `category-wise-item-category-wise-monthly-sales` | event-ledger | N/A | No — resolvable | Yes, verified, non-vacuous | None — real populated data |
| `category-wise-category-wise-day-net-sale` | event-ledger | N/A | No — resolvable | Yes, verified, non-vacuous | None — real populated data |

## Net effect of this pass

- 0 leaves promoted (out of scope — `reports.go` is owned by a different
  concurrent agent this wave).
- 4/4 leaves confirmed still pure event-ledger with a working,
  non-erroring fallback for the sandbox tenant.
- 4/4 leaves now have a concrete, DB-verified recommended query shape using
  a previously-unused-for-this-purpose real lookup table
  (`master_categories`), refuting the "opaque category code, no name
  available" blocker cited in `docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`
  for the sibling `category-wise-sales` leaf and by extension the rest of
  the Category Wise submenu.
- The gross-profit/COGS gap is **confirmed for `stock_allocations`
  specifically** (0 rows, sandbox tenant, re-verified) but **refuted as a
  hard blocker in general** — `stock_ledger` is a populated, real,
  independently-verified alternative cost source covering 620,613/620,615
  sale lines (99.9997%) that the existing `salesProfitMarginReadModelQuery`
  does not currently use. This affects more than this wave's 4 leaves: the
  two already-`salesReadModel`-wired leaves that depend on
  `salesProfitMarginReadModelQuery`
  (`customer-sales-invoice-wise-profit-margin-detail`,
  `customer-sales-customer-category-wise-sales-customer-wise-gross-profit`)
  are currently returning blank cost/gross-profit/margin columns for every
  row in this tenant and could likely be fixed by switching their cost
  source from `stock_allocations` to `stock_ledger` — flagged here for
  whichever pass next touches that function, since it is out of this
  agent's file-edit scope.

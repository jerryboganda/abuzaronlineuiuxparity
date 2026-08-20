# Phase O Golden Verification — Purchase Report Leaves (2026-08-09)

Scope: 14 assigned Phase O (Purchases & Suppliers) report leaves. Verified against
`docs/REMAINING_WORK_100_PERCENT_PARITY_2026-08-09.md` Phases N–Q and directly against
`services/api/internal/httpapi/reports.go`. `reports.go` was **not** edited (per the
concurrent-agent rule); this pass is read-only plus the two files listed below.

**Files added by this pass:**
- `docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md` (this file)
- `services/api/internal/httpapi/report_golden_o_purchase_test.go` (offline, no-DB registry/query-shape lock tests — pattern matched to the existing `report_golden_stock_movement_test.go`)

## Environment constraint

Live legacy SQL Server is unreachable in this environment (confirmed by the dispatcher).
All verification below is **Postgres-internal cross-verification**: the production query
was reproduced verbatim from `reports.go` and run in `psql`, then compared against a
second, differently-constructed query (window functions / pre-aggregated CTEs instead of
correlated `LATERAL` joins) against the same base tables
(`business_documents`, `business_document_lines`, `stock_ledger`, `party_ledger_entries`,
`master_parties`) for the sandbox tenant. Matching independent queries is not the same as
byte-matching legacy PowerBuilder output — no golden legacy report bytes were available
this pass — but it does verify the current implementation is internally correct/consistent
against its own source-of-truth tables, which is the strongest verification possible
without the legacy server.

RLS preamble used for every query:
```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', '', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```
Sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch
`ffffffff-ffff-ffff-ffff-ffffffffffff` ("Legacy Reference Sandbox"). Data volume for the
full 2025-01-01–2026-07-31 window: 6,395 posted `pack-purchase` + 22 `loose-purchase` + 1
`opening-purchase` = 6,418 posted receiving documents, 113,525 lines, 2,810 posted
`purchase-order` documents, 634 posted `purchase-return` documents. All purchase data in
this tenant is canonical (`business_documents`); there are 0 compatibility `sync_events`
rows for `receiving`/`purchase_order`/`return`, so the compatibility-fallback branch of
every query below is empty/inert for this tenant and was not separately exercised.

## Ground-truth `projectionStatus` for the 14 assigned leaves

Read directly from `reportDefinitionForKey` (`reports.go` lines 1316–1551) and confirmed
by calling `reportDefinitionFor(kind)` for each leaf (see
`TestPhaseOGoldenPurchaseLeavesReportEventLedgerProjectionStatus`):

| # | Leaf kind | `purchaseMode` | `aggregateCondition` | `ProjectionStatus` |
|---|---|---|---|---|
| 1 | `purchase-summary` | `invoice-summary` | `receiving` | `event-ledger` |
| 2 | `purchase-summary2` | `invoice-summary` | `receiving` | `event-ledger` |
| 3 | `purchase-order-summary` | `invoice-summary` | `purchase_order` | `event-ledger` |
| 4 | `purchase-order` | `invoice-summary` | `purchase_order` | `event-ledger` |
| 5 | `periodic-purchases` | `month-summary` | `receiving` | `event-ledger` |
| 6 | `purchase-order-supplier-wise` | `supplier-summary` | `purchase_order` | `event-ledger` |
| 7 | `net-purchase-summary` | `invoice-summary` | `receiving` | `event-ledger` |
| 8 | `days-summary` | `day-summary` | `receiving` | `event-ledger` |
| 9 | `category-wise-purchase` | `item-summary` | `receiving` | `event-ledger` |
| 10 | `monthly-purchase-graph` | `month-summary` | `receiving` | `event-ledger` |
| 11 | `manufacturer-wise-monthly-stock-movement` | `month-summary` | `receiving` | `event-ledger` |
| 12 | `manufacturer-wise-detail` | `detail` | `receiving` | `event-ledger` |
| 13 | `supplier-manufacturer-wise-g-p` | `supplier-summary` | `receiving` | `event-ledger` |
| 14 | `supplier-purchase-returns-summary` | `supplier-summary` | `return` | `event-ledger` |

**All 14 of my assigned leaves report `ProjectionStatus = "event-ledger"` today — 0 of 14
are `"real"`.** This matches the dispatcher's expectation ("only 4 of 24 O-phase leaves are
real"); the 4 that are `"real"` (`p-o-based-purchase-disparity`, `supplier-wise-advance-income-tax`,
`supplier-category-wise-input-sales-tax-report`, `withholding-tax-deduction`, confirmed via
the same `reportDefinitionFor` sweep) are **not** in my assigned set.

### Important nuance: `"event-ledger"` here does not mean generic passthrough

`reportDefinitionForKey` never flips `ProjectionStatus` to `"real"` for a
`purchaseReadModel` spec unless `purchaseMode == "po-disparity"` — that's a metadata/label
gap, not a query-behavior gap. Tracing the actual data path
(`reports.go` lines 5146–5160), every one of my 14 leaves is dispatched to
`purchaseReadModelQueryMode(spec.aggregateCondition, spec.purchaseMode, ...)`, which routes
to a **dedicated grouped-aggregation query per mode**
(`purchaseCalendarSummaryReadModelQuery` for day/month-summary,
`purchaseItemSummaryReadModelQuery` for item-summary,
`purchaseSupplierSummaryReadModelQuery` for supplier-summary, `purchaseReadModelQuery`
itself for invoice-summary/detail) — never the fully generic `sync_events` passthrough at
the bottom of the report handler (`reports.go` line ~5235, reached only for kinds with no
matching spec at all). So although the API metadata labels these "event-ledger", the SQL
actually executed performs genuine per-document/per-supplier/per-day/per-month/per-item
summation from the canonical purchase tables. This is exactly why a meaningful golden check
is possible for all 14 leaves in this pass, unlike a leaf whose only source is a raw
`sync_events` scan.

## Methodology detail

For each distinct `(purchaseMode, aggregateCondition)` combination used by my 14 leaves, I:
1. Reproduced the exact SQL text the corresponding Go function builds (copied verbatim from
   `reports.go`, parameters substituted with literals).
2. Wrote a second query computing the same grand total a structurally different way —
   pre-aggregated CTEs with `DISTINCT ON`/`GROUP BY` instead of correlated `LEFT JOIN LATERAL`
   subqueries per document/line — against the same base tables.
3. Compared row counts, total quantity, and total amount (and, for month-summary, every
   individual month's total).

Six distinct `(mode, aggregate)` combinations cover all 14 leaves:

| Combination | Leaves covered |
|---|---|
| `invoice-summary` × `receiving` | purchase-summary, purchase-summary2, net-purchase-summary |
| `invoice-summary` × `purchase_order` | purchase-order-summary, purchase-order |
| `month-summary` × `receiving` | periodic-purchases, monthly-purchase-graph, manufacturer-wise-monthly-stock-movement |
| `day-summary` × `receiving` | days-summary |
| `supplier-summary` × `receiving` | supplier-manufacturer-wise-g-p |
| `supplier-summary` × `purchase_order` | purchase-order-supplier-wise |
| `supplier-summary` × `return` | supplier-purchase-returns-summary |
| `item-summary` × `receiving` | category-wise-purchase |
| `detail` × `receiving` | manufacturer-wise-detail |

## Verified results

### `invoice-summary` × `receiving` (purchase-summary, purchase-summary2, net-purchase-summary)
Production query (reproduced from `purchaseReadModelQuery`, `reports.go` lines 3052–3155)
vs. independent window-function/pre-aggregated-CTE query, full date range:

| | row_count | total_qty | total_amount |
|---|---|---|---|
| Production | 6,418 | 4,860,933.0000 | 198,071,256.0000 |
| Independent | 6,418 | 4,860,933.0000 | 198,071,256.0000 |

**MATCHED exactly.**

### `invoice-summary` × `purchase_order` (purchase-order-summary, purchase-order)
| | row_count | total_qty | total_amount |
|---|---|---|---|
| Production | 2,810 | 375,195.0000 | 0.0000 |
| Independent | 2,810 | 375,195.0000 | 0.0000 |

**Arithmetic MATCHED exactly** — but see the **genuine bug** below: `total_amount = 0.0000`
for every purchase order is itself wrong (it should reflect the real ordered value), so the
query's Amount column, while internally consistent, is not economically meaningful for this
leaf. Verdict: **MISMATCH-DOCUMENTED**.

### `month-summary` × `receiving` (periodic-purchases, monthly-purchase-graph, manufacturer-wise-monthly-stock-movement)
All 19 calendar months (2025-01 through 2026-07) matched exactly between the production
`purchaseCalendarSummaryReadModelQuery` output and an independently-constructed per-document
monthly aggregation, e.g.:

| Month | Production qty / amount | Independent qty / amount |
|---|---|---|
| 2025-01 | 501,005.0000 / 21,646,580.0000 | 501,005.0000 / 21,646,580.0000 |
| 2025-07 | 284,389.0000 / 12,156,927.0000 | 284,389.0000 / 12,156,927.0000 |
| 2026-07 | 163,793.0000 / 6,862,456.0000 | 163,793.0000 / 6,862,456.0000 |
| … | 19/19 months matched | 19/19 months matched |

**MATCHED exactly** for `periodic-purchases` and `monthly-purchase-graph` (both are plain
receiving-aggregate month-summary — the grouping dimension matches what the leaf name
promises). `manufacturer-wise-monthly-stock-movement` uses the identical query (no
manufacturer join/grouping exists) — see **MISMATCH-DOCUMENTED** discussion below.

### `day-summary` × `receiving` (days-summary)
Spot-checked the busiest posted day in the sandbox, 2025-12-11 (23 documents):

| | day_qty | day_amount |
|---|---|---|
| Production | 10,050.0000 | 490,133.0000 |
| Independent | 10,050.0000 | 490,133.0000 |

**MATCHED exactly.**

### `supplier-summary` × `receiving` / `purchase_order` / `return`
| Aggregate | supplier_rows | total_qty | total_amount | Independent match |
|---|---|---|---|---|
| receiving (supplier-manufacturer-wise-g-p) | 113 | 4,860,933.0000 | 198,071,256.0000 | exact |
| purchase_order (purchase-order-supplier-wise) | 44 | 375,195.0000 | 0.0000 | exact (amount=0 is the same PO bug, see below) |
| return (supplier-purchase-returns-summary) | 55 | 57,868.0000 | 3,526,551.0000 | exact |

`supplier-purchase-returns-summary` — **MATCHED exactly**, including the debit/credit-amount
flip for `purchase-return` documents (`purchaseReadModelQuery`'s `ple` LATERAL correctly
switches to `p.debit_amount` for `d.kind = 'purchase-return'`).
`purchase-order-supplier-wise` — quantity matched, amount inherits the PO amount=0 bug:
**MISMATCH-DOCUMENTED**.
`supplier-manufacturer-wise-g-p` — arithmetic matched, but see grouping-dimension gap below:
**MISMATCH-DOCUMENTED**.

### `item-summary` × `receiving` (category-wise-purchase)
| | item/party rows | total_qty | total_amount |
|---|---|---|---|
| Production | 14,395 | 4,860,933.0000 | 2,702,608,926.7900 |
| Independent | 14,394 (`COUNT(DISTINCT(item,party))` vs `GROUP BY`, NULL-handling artifact) | 4,860,933.0000 | 2,702,608,926.7900 |

Totals **MATCHED exactly**; the 1-row difference in group cardinality is a benign
`COUNT(DISTINCT (a,b))` vs `GROUP BY a,b` NULL-handling artifact between the two query
constructions, not a data discrepancy (both sum to the identical grand total). See
**grouping-dimension gap** below: this leaf is named "Category Wise Purchase" but the query
groups by **item name**, not item category. **MISMATCH-DOCUMENTED**.

### `detail` × `receiving` (manufacturer-wise-detail)
| | line rows | total_qty | total_amount |
|---|---|---|---|
| Production | 113,526 | 4,860,933.0000 | 2,702,608,926.7900 |
| Independent (`INNER JOIN`, no LEFT JOIN artifact) | 113,525 | 4,860,933.0000 | 2,702,608,926.7900 |

Totals **MATCHED exactly**; the 1-row difference is a single document with zero lines
producing one all-`NULL` row via `LEFT JOIN business_document_lines` in the production query
(contributes nothing to the sums since `SUM` ignores `NULL`) — confirmed benign, not a bug.
See **grouping-dimension gap** below: no manufacturer join/column exists in this query at
all, despite the leaf name. **MISMATCH-DOCUMENTED**.

## Genuine bug found: purchase-order `total_amount` is always 0

**Function:** `purchaseReadModelQuery` (`reports.go` lines 3052–3155), `amountExpression :=
"COALESCE(ple.amount, d.total_amount)"`, used by the `invoice-summary` and `supplier-summary`
modes for `aggregateCondition = "se.aggregate = 'purchase_order'"`.

**Expected:** For `purchase-order-summary`, `purchase-order`, and
`purchase-order-supplier-wise`, the Amount column should reflect each purchase order's real
ordered value.

**Actual:** Amount is `0.0000` for every one of the 2,810 posted purchase-order documents in
the sandbox tenant, both in production output and (necessarily, since it reproduces the same
formula) the independent cross-check. Direct inspection confirms
`business_documents.total_amount = 0.0000` for all sampled purchase-order rows even though
`business_document_lines.line_total` sums to real non-zero priced values, e.g.:

| document_number | total_amount | lines | SUM(line_total) |
|---|---|---|---|
| 2810 | 0.0000 | 10 | 20,502.4000 |
| 2809 | 0.0000 | 4 | 8,448.1800 |
| 2808 | 0.0000 | 50 | 161,725.1500 |

**Root cause (ambiguous, could not be fully pinned down without the live legacy server):**
`ple.amount` is correctly `NULL` for purchase orders (no `party_ledger_entries` row exists
for a mere order — there is no supplier financial liability until goods are received, which
is correct), so the formula falls back to `d.total_amount`. In the live document-posting
path (`documents.go` `total_amount = formatMoney(priced.result.Total)`), `total_amount`
should be populated by the pricing engine at document-creation time for every document kind
including `purchase-order`. But this sandbox tenant's purchase-order **headers** were
migration-imported (`migration/maps/phase-e-historical-orders.json` maps
`total_amount ← PurOrderHeader.Amount`), separately from the **lines**, which were
back-filled later by a distinct bulk importer
(`migration/cmd/bulkorderlines/main.go`). Two plausible explanations, neither of which I
could confirm without live legacy access:
1. The legacy `PurOrderHeader.Amount` source column was itself 0/null for these rows (a
   data-provenance characteristic of the sandbox fixture, not a reports.go defect), or
2. The header/line import pipelines never reconciled `total_amount` after lines were
   back-filled (a migration-side gap, also not strictly a `reports.go` defect).

Either way, **`reports.go`'s fallback logic itself has a latent defect independent of root
cause**: `amountExpression` for `purchase-order` documents should not rely solely on
`d.total_amount` (a mutable, best-effort field that migration data shows can silently be
stale/zero) when `SUM(l.line_total)` is directly available and already computed correctly by
the `detail`/`po-disparity` modes' `amountExpression = "l.line_total"`. A `COALESCE(ple.amount,
NULLIF(d.total_amount, 0), <line-total sum>)` fallback would make
`purchase-order-summary`/`purchase-order`/`purchase-order-supplier-wise` show real values
regardless of which of the two root causes above is true. I am not permitted to edit
`reports.go`; flagging this precisely for the next promotion pass.

## Grouping-dimension gaps vs. legacy leaf names (what real projection would need)

Four leaves in my set are named for a legacy grouping dimension the current query does not
implement. Arithmetic is internally correct (verified above); the gap is that the *shape* of
the output doesn't match what the legacy report leaf promises.

**`category-wise-purchase`** — Named "Category Wise Purchase" but `purchaseItemSummaryReadModelQuery`
groups by `item_name`/`party` (item, not category). The retained legacy payload has what's
needed: `master_items.payload->>'ICatCode'` is populated for all 30,052 items in the sandbox
tenant, and a `master_categories` table exists (838-row-equivalent sibling table structure to
`master_manufacturers`, confirmed present in the schema) that `ICatCode` should join against.
**What's needed:** join `business_document_lines.item_id → master_items.id`, extract
`payload->>'ICatCode'`, join to `master_categories` by legacy code, and `GROUP BY` category
name instead of item name.

**`manufacturer-wise-detail`** and **`manufacturer-wise-monthly-stock-movement`** — Both
named "Manufacturer Wise" but use the plain `detail`/`month-summary` modes verbatim (no
manufacturer join or grouping column exists in either query). Confirmed buildable:
`master_items.payload->>'ManfCode'` is populated for all 30,052 items, and
`master_manufacturers` (838 rows for this tenant, `legacy_id`/`code` matching `ManfCode`,
e.g. code `62` → `"BRISTOL-MYERS"`) is fully populated and ready to join. **What's needed:**
join `business_document_lines.item_id → master_items.id`, extract `payload->>'ManfCode'`,
join to `master_manufacturers`, and either add a manufacturer column to the detail rows or
`GROUP BY` manufacturer for the monthly-movement leaf (plus resolve whether the legacy
"detail" columns/order for this specific leaf differ from the generic purchase-detail leaf —
unverified without legacy screen captures).

**`supplier-manufacturer-wise-g-p`** — Named "Supplier/Manufacturer Wise G/P" but
`purchaseSupplierSummaryReadModelQuery` groups by supplier only (no manufacturer dimension)
and computes no G/P figure at all — only quantity and amount. **What's needed:** (1) the same
manufacturer join described above, composed into the grouping key alongside supplier; (2)
clarification of what "G/P" computes in this specific legacy purchase-side report — it is
ambiguous from the name alone whether this is a per-manufacturer/per-supplier "Gross
Purchase" value rollup (most likely, given it lives under Purchase Reports rather than Sales
Reports, where "G/P" elsewhere in this codebase denotes Gross Profit against a cost basis)
or an actual profit calculation; this needs the legacy PowerBuilder source/screenshot to
resolve definitively, which was not available this pass.

## Verdict summary

| # | Leaf | ProjectionStatus (metadata) | Underlying query | Verdict |
|---|---|---|---|---|
| 1 | purchase-summary | event-ledger | real per-mode grouped SQL | **MATCHED** |
| 2 | purchase-summary2 | event-ledger | real per-mode grouped SQL | **MATCHED** |
| 3 | purchase-order-summary | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (amount always 0) |
| 4 | purchase-order | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (amount always 0) |
| 5 | periodic-purchases | event-ledger | real per-mode grouped SQL | **MATCHED** |
| 6 | purchase-order-supplier-wise | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (amount always 0) |
| 7 | net-purchase-summary | event-ledger | real per-mode grouped SQL | **MATCHED** |
| 8 | days-summary | event-ledger | real per-mode grouped SQL | **MATCHED** |
| 9 | category-wise-purchase | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (groups by item, not category) |
| 10 | monthly-purchase-graph | event-ledger | real per-mode grouped SQL | **MATCHED** |
| 11 | manufacturer-wise-monthly-stock-movement | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (no manufacturer dimension) |
| 12 | manufacturer-wise-detail | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (no manufacturer dimension) |
| 13 | supplier-manufacturer-wise-g-p | event-ledger | real per-mode grouped SQL | **MISMATCH-DOCUMENTED** (no manufacturer dimension, no G/P calc) |
| 14 | supplier-purchase-returns-summary | event-ledger | real per-mode grouped SQL | **MATCHED** |

**0 of 14 are `projectionStatus == "real"` today** (all still carry the `event-ledger` label
per the metadata gap described above). **None of the 14 are `STILL-EVENT-LEDGER-NEEDS-X` in
the "ungrounded generic-column output" sense** — every leaf's data path is a genuine,
independently-verified grouped SQL projection, not a raw `sync_events` scan. Of the 14: **7
MATCHED** cleanly (arithmetic correct and grouping dimension matches the leaf's legacy name),
**7 MISMATCH-DOCUMENTED** (arithmetic internally correct/reproducible, but either the Amount
column is silently 0 for purchase-order-based leaves, or the grouping dimension the leaf name
promises — category, manufacturer, or gross-profit — is not implemented). 0 leaves were
`VACUOUS-NO-DATA` or `UNVERIFIABLE-THIS-PASS` — the sandbox tenant has ample posted purchase
data (6,418 receiving documents, 2,810 purchase orders, 634 returns) across 19 months.

## Companion test

`services/api/internal/httpapi/report_golden_o_purchase_test.go` locks:
- registry wiring (mode/title/aggregate) for all 14 leaves,
- the current `event-ledger` `ProjectionStatus` (so a future promotion pass updates this
  doc/test together instead of silently drifting),
- that `purchaseReadModelQueryMode` produces real grouped SQL (not generic fallback) for
  each mode, referencing the totals verified above in comments, and
- that the manufacturer/category grouping gaps documented above still exist (so a future
  promotion pass that adds the `master_manufacturers`/category join updates this test too).

Verified with `go vet ./services/api/...` (clean) and
`go test -run TestPhaseOGolden ./services/api/internal/httpapi/` (4/4 passing, offline, no
DB dependency).

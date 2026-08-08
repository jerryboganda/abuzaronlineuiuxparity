# Phase N/O Golden Verification — Remainder Leaves (2026-08-09)

Scope: 7 leaves not covered by any prior wave-1/wave-2 evidence doc — 3 Phase N
generic Sales Reports leaves (`reports.go` lines 221–321) and 4 Phase O
`purchaseReadModel` leaves (`reports.go` lines 328–372) that fall outside the
14-leaf set already verified in `docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md`.

`services/api/internal/httpapi/reports.go` was **not** edited (read-only per
the concurrent-agent rule). This pass adds only:
- `docs/PHASE_N_O_GOLDEN_VERIFICATION_REMAINDER_2026-08-09.md` (this file)
- `services/api/internal/httpapi/report_golden_no_remainder_test.go` (offline,
  no-DB registry/query-shape lock tests, pattern-matched to
  `report_golden_o_purchase_test.go`)

## Environment constraint

Live legacy SQL Server is unreachable in this environment (confirmed by the
dispatcher). All verification below is Postgres-internal cross-verification:
the production query was reproduced verbatim from `reports.go` and run in
`psql`, then compared against a second, independently-constructed query
(pre-aggregated CTEs instead of the production's correlated `LATERAL` joins)
against the same base tables (`business_documents`, `business_document_lines`,
`stock_ledger`, `party_ledger_entries`, `master_parties`, `master_items`) for
the sandbox tenant. This is the same methodology used by
`docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md` and
`docs/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md`.

RLS preamble used for every query:
```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', 'ffffffff-ffff-ffff-ffff-ffffffffffff', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```
(Note: the dispatcher's template preamble sets `app.branch_id` to `''`. That
value governs RLS row visibility only; the production queries under test also
bind a literal `$2::uuid` branch parameter directly in their `WHERE`
clause — the report handler always supplies the caller's selected branch, not
an empty string. Since the sandbox tenant's `business_documents` all carry a
single branch (`ffffffff-ffff-ffff-ffff-ffffffffffff`, confirmed via `SELECT
DISTINCT branch_id`), that literal value was used for `$2` in every reproduced
query below — matching exactly what `docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md`
did for the same reason.)

Sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch
`ffffffff-ffff-ffff-ffff-ffffffffffff`. Confirmed posted-document volume:
291,359 `cash-sale`, 2 `credit-sale`, 30,704 `cash-sale-return`, 6,395
`pack-purchase`, 22 `loose-purchase`, 1 `opening-purchase`, 2,810
`purchase-order`, 634 `purchase-return`.

---

## Part 1 — Phase N leaves (generic event-ledger, `reports.go` lines 221–321)

### Ground truth from the registry

All three leaves are registered in the loop at `reports.go` lines 221–319. The
`mode` switch statement (lines 285–317) has **no case** for any of the three
kinds, so `spec.salesMode == ""` for all of them, and `explicitEventProjection`
routes them through the default `salesReadModel` branch
(`reports.go` lines 1401–1408) — none of the specific-mode branches at lines
1413–1422 match an empty string, so `ProjectionStatus` stays `"event-ledger"`,
`columns = reportEventLedgerColumns()`, and the dispatcher
(`reports.go` line 5343, `salesReadModelQueryMode`) falls through every
`if mode == "..."` check to its final `return salesReadModelQuery(aggregateCondition, pagination)`
— the same raw, ungrouped, per-line/per-document passthrough used as the
literal fallback for any kind with no dedicated mode at all.

| # | Leaf kind | Title | `aggregateCondition` | `salesMode` | `ProjectionStatus` |
|---|---|---|---|---|---|
| 1 | `item-wise-item-wise-net-sales` | Item Wise Net Sales | `reportSaleAggregate` | `""` | `event-ledger` |
| 2 | `sale-return-summary-inv-type-wise` | Sale/Return Summary Inv. Type Wise | `reportSaleOrReturn` (title contains "return") | `""` | `event-ledger` |
| 3 | `dead-item-list` | Dead Item List | `reportSaleAggregate` | `""` | `event-ledger` |

Confirmed directly by `TestPhaseNORemainderLeavesResolveToExpectedMode` and
`TestPhaseNORemainderLeavesReportEventLedgerProjectionStatus` (see companion
test file), and by `TestPhaseNORemainderSalesLeavesUseGenericRawPassthrough`,
which asserts `salesReadModelQueryMode(agg, "", pagination)` is **byte-identical**
to `salesReadModelQuery(agg, pagination)` — i.e. there is no dedicated
per-mode SQL for empty-mode leaves at all, unlike every one of the O-phase
`purchaseReadModel` modes (which route through `purchaseReadModelQueryMode`'s
explicit `switch` to a real grouped-aggregation function even when the API
metadata says "event-ledger"). This is the important distinction from the
Phase O nuance: for these 3 N leaves, "event-ledger" **does** mean literally
what it says — a raw, ungrouped union of `business_documents`/
`business_document_lines` rows, one output row per line (or per document for
compatibility-only rows), columns `document, occurred_at, party, item,
quantity, amount` where `amount` is the **whole document's** `total_amount`
repeated on every line (not a per-line amount).

### 1. `item-wise-item-wise-net-sales` — "Item Wise Net Sales"

**Legacy semantics implied by the title:** a per-item rollup of net sales
(gross sales minus returns, or at minimum total quantity/amount) across the
retrieval date range, one row per item, sorted/grouped by item.

**Current status:** No item-level aggregation exists. The dispatched query
returns one row per sale line with the item name and the line's raw quantity,
but the "amount" column is the parent document's total, not a per-item sum —
running this query as-is and trying to compute a net-sales-by-item total by
summing the "amount" column would badly overcount (every line of a
multi-line invoice repeats the same document total).

**Real-query feasibility:** High. Confirmed via direct query: 620,615 posted
`cash-sale`/`credit-sale` lines span 7,113 distinct items
(`business_document_lines.item_id`), and `business_document_lines.line_total`
(not `business_documents.total_amount`) is the correct per-line amount to
aggregate (already used correctly elsewhere in `reports.go`, e.g.
`purchaseReadModelQuery`'s `detail` mode). A real projection needs: `GROUP BY
item_id, item_name`, `SUM(quantity)`, `SUM(line_total)` over
`cash-sale`/`credit-sale` (and, if "net" means sale-minus-return per the
title, a further subtraction of matched `cash-sale-return` lines by item —
this specific semantic, gross-vs-net, needs a legacy screenshot to confirm,
since "net sales" could also just mean "after line-level discount" which
`line_total` already reflects).

### 2. `sale-return-summary-inv-type-wise` — "Sale/Return Summary Inv. Type Wise"

**Legacy semantics implied by the title:** a summary of sale and return
activity grouped/split by invoice type (in this codebase's terms, likely
`business_documents.kind`: `cash-sale` vs `credit-sale` vs
`cash-sale-return`, or the legacy `open-*` variants).

**Current status:** No invoice-type grouping exists at all — `bd.kind` is
never selected or grouped by anywhere in `salesReadModelQuery`; the query
returns raw union rows from both the sale and sale-return canonical branches
(confirmed: `aggregateCondition = reportSaleOrReturn` triggers the
`canonicalReturnUnion` branch at `reports.go` lines 1722–1749, which unions
in `cash-return`/`credit-return`/`open-cash-return`/`open-credit-return`/
`cash-sale-return`/`credit-sale-return`/`open-sale-return` kinds), but there
is no `kind` column in the output and no `GROUP BY kind` anywhere.

**Real-query feasibility:** High. The sandbox tenant's actual kind
distribution for this aggregate is simple: `cash-sale` (291,359),
`credit-sale` (2), `cash-sale-return` (30,704) — 0 rows exist for any
`open-*`/`credit-return` variant. A real "Inv. Type Wise" summary would
`SELECT bd.kind, SUM(...), COUNT(...) ... GROUP BY bd.kind` over the same
canonical union already assembled by `salesReadModelQuery`'s WHERE/UNION
structure — the hard part (which document kinds count as "sale" vs "return"
for this aggregate) is already correctly enumerated in the existing query
text; only the grouping/column-selection is missing.

### 3. `dead-item-list` — "Dead Item List"

**Legacy semantics implied by the title:** items with no (or below-threshold)
recent sales activity — typically "items not sold in the last N days/months"
— which by definition requires enumerating **all** items (via `master_items`)
and left-joining sales activity, not enumerating sales lines and hoping
absent items show up (they structurally cannot, since a query rooted in
`business_document_lines` only ever returns items that **were** sold).

**Current status:** Structurally cannot produce dead-item semantics.
`salesReadModelQuery` never references `master_items` at all (confirmed via
`TestPhaseNORemainderSalesLeavesUseGenericRawPassthrough`'s
`strings.Contains(rawQuery, "master_items")` check — false). The dispatched
query is rooted in `business_documents`/`business_document_lines`, so it can
only ever return items that have at least one sale line in the retrieval
range; it can never surface an item with **zero** sales, which is the entire
point of a dead-item report.

**Real-query feasibility:** Medium — the raw ingredients exist but need a
genuinely different query shape (an anti-join from `master_items`, not a
projection of `salesReadModelQuery`'s output). Confirmed directly: 30,052
total items in `master_items` for this tenant; 22,939 (76.3%) have **zero**
posted `cash-sale`/`credit-sale` lines ever (verified via `NOT EXISTS`
anti-join); the remaining 7,113 have at least one sale. A real "Dead Item
List" needs: `master_items` LEFT JOIN a per-item `MAX(occurred_at)` from
sales lines, filtered to `last_sale_date IS NULL OR last_sale_date <
:cutoff`. The exact cutoff/threshold (a fixed lookback window, a
preference-driven value, or simply "never sold") is a legacy-semantics
judgment call that needs the legacy PowerBuilder retrieval-arguments screen to
resolve — not determinable from the title or current code alone.

### Note for the promotion agent (N leaves)

All three N leaves need genuinely new query functions, not a metadata
relabel — unlike several O-phase leaves, there is no already-real grouped
query hiding behind the "event-ledger" label here. Priority/difficulty:
`item-wise-item-wise-net-sales` (straightforward `GROUP BY item_id`) <
`sale-return-summary-inv-type-wise` (straightforward `GROUP BY bd.kind`, data
already unioned correctly) < `dead-item-list` (needs a `master_items`-rooted
anti-join, a materially different query shape from the shared
`salesReadModelQuery`, plus a legacy-semantics decision on the dead-item
cutoff/threshold).

---

## Part 2 — Phase O leaves (`purchaseReadModel`, `reports.go` lines 328–372)

### Ground truth from the registry

| # | Leaf kind | Title | `purchaseMode` | `aggregateCondition` | `ProjectionStatus` |
|---|---|---|---|---|---|
| 4 | `purchase-return-summary` | Purchase Return summary | `invoice-summary` | `se.aggregate = 'return'` | `event-ledger` |
| 5 | `supplier-wise-detail` | Detail | `detail` | `se.aggregate = 'receiving'` | `event-ledger` |
| 6 | `supplier-wise-purchase-detail` | Purchase Detail | `detail` | `se.aggregate = 'receiving'` | `event-ledger` |
| 7 | `supplier-purchase-returns-detail` | Detail | `detail` | `se.aggregate = 'return'` | `event-ledger` |

Confirmed via `TestPhaseNORemainderLeavesResolveToExpectedMode` and
`TestPhaseNORemainderLeavesReportEventLedgerProjectionStatus`. None of these
4 are `concreteProjection` (`reports.go` line 1323: that only fires for
`kind == "purchase-return"` — a different, singular leaf kind — or
`purchaseMode == "po-disparity"`), so all 4 carry the `"event-ledger"` label
today, matching the same metadata-vs-behavior gap
`docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md` already documented
for its 14 leaves: **the label says event-ledger, but the dispatched SQL
(`purchaseReadModelQueryMode`) is genuine grouped aggregation for all 4.**

`isPurchaseSummaryMode("invoice-summary")` → **true** (confirmed directly,
`reports.go` line 3182: `"invoice-summary", "day-summary", "month-summary",
"item-summary", "supplier-summary"`), so `purchase-return-summary` does get
the `purchaseSummaryReportColumns`/`purchaseProjectionModeNote` treatment
inside `reportDefinitionForKey`'s note-building — but, per the same code
path already traced in the Phase O doc, that only changes `projectionNote`
and `columns`, never `projectionStatus`, so the metadata still says
"event-ledger" even though — as verified below — the query itself is real.

### 4. `purchase-return-summary` — mode `invoice-summary`, aggregate `return`

This is the exact same `(purchaseMode, aggregateCondition)` shape as
`purchase-summary`/`purchase-summary2`/`net-purchase-summary` (already
MATCHED in the companion doc for the `receiving` aggregate) but for
`purchase-return` documents instead. Dispatches to `purchaseReadModelQuery`
directly (not `detail`, so `itemExpression = "''"`, per-document
`SUM(...)` grouping by `document_number`).

**Production query** (reproduced verbatim, `canonicalKinds = "'purchase-return'"`,
`eventAggregate = "return"`), full posted-return set:

```sql
WITH canonical_purchase AS (
    SELECT d.document_number AS document, ...
    FROM business_documents d
    LEFT JOIN master_parties mp ON ...
    LEFT JOIN business_document_lines l ON ...
    LEFT JOIN LATERAL (SELECT SUM(sl.quantity * sl.adjustment_sign) ... ) stock ON true
    LEFT JOIN LATERAL (SELECT CASE WHEN d.kind = 'purchase-return'
                                    THEN NULLIF(p.debit_amount, 0)
                                    ELSE NULLIF(p.credit_amount, 0) END ... ) ple ON true
    WHERE d.kind IN ('purchase-return') AND d.status = 'posted'
    GROUP BY d.document_number, d.occurred_at, mp.name, ple.amount, d.total_amount
)
SELECT count(*), SUM(quantity::numeric), SUM(amount::numeric) FROM canonical_purchase;
```

| | row_count | total_qty | total_amount |
|---|---|---|---|
| Production | 634 | 57,868.0000 | 3,526,551.0000 |
| Independent (pre-aggregated CTEs, no correlated LATERAL) | 634 | 57,868.0000 | 3,526,551.0000 |

**MATCHED exactly.** Also cross-checked against
`docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md`'s own already-verified
`supplier-purchase-returns-summary` (supplier-summary × return) grand total —
identical (57,868.0000 / 3,526,551.0000), as expected since
`purchaseSupplierSummaryReadModelQuery` is built by further `GROUP BY party`
on top of exactly this same `invoice-summary` × `return` base query.

Additionally verified the `ple.amount`/`d.total_amount` fallback is *not*
the purchase-order amount=0 bug for this leaf: for all 634 purchase-return
documents, `ple.amount IS NOT NULL` (0 nulls) and `ple.amount = d.total_amount`
exactly (0 mismatches) — the two candidate amount sources fully agree here,
unlike purchase-order documents.

**Verdict: MATCHED.** This leaf, despite its `"event-ledger"` metadata label,
executes a real, correct, independently-reproduced per-document purchase-return
summary query.

### 5 & 6. `supplier-wise-detail` and `supplier-wise-purchase-detail` — mode `detail`, aggregate `receiving`

Both dispatch to `purchaseReadModelQueryMode("se.aggregate = 'receiving'", "detail", pagination)`
— **the exact same function call, with the exact same two arguments**, as
`manufacturer-wise-detail`, which was already golden-verified in
`docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md` (113,526 line rows
— 113,525 real + 1 benign zero-line-document `LEFT JOIN` artifact — qty
4,860,933.0000, amount 2,702,608,926.7900, MISMATCH-DOCUMENTED only on the
missing-manufacturer-column grouping-dimension gap, not on arithmetic).

`purchaseReadModelQuery`'s query-building logic depends only on the `mode`
string equality `mode == "detail"` and the `aggregateCondition` string — it
has no awareness of which registry key invoked it, so there is **no
supplier-specific column, filter, or join distinguishing these three leaf
kinds from each other at the SQL level.**

Re-ran the production query text directly (not relying solely on the prior
doc) to confirm the totals still hold in the current DB state:

| | row_count | total_qty | total_amount |
|---|---|---|---|
| Production (re-run this pass) | 113,526 | 4,860,933.0000 | 2,702,608,926.7900 |
| `manufacturer-wise-detail` (Phase O doc, already verified) | 113,526 | 4,860,933.0000 | 2,702,608,926.7900 |

Exact match, confirming the numbers are stable and the three leaf kinds are
byte-identical in SQL and in result. Locked programmatically by
`TestPhaseNORemainderPurchaseDetailModeLeavesAreByteIdenticalQueries`, which
calls `purchaseReadModelQueryMode` with each of the three specs' actual
`(aggregateCondition, purchaseMode)` pairs and asserts the returned SQL
strings are identical (`==`, not just "similar").

**Verdict: `supplier-wise-detail` and `supplier-wise-purchase-detail` are
explicitly duplicates of the already-verified `manufacturer-wise-detail`
query** — MATCHED (arithmetic) for the same reason
`manufacturer-wise-detail` was MATCHED, and inheriting the exact same
grouping-dimension gap already documented for it (no manufacturer *or*
supplier-specific join/column exists in `detail` mode at all — the "Detail"
and "Purchase Detail" titles for these two leaves don't promise a supplier
dimension the way `manufacturer-wise-detail`'s name promises a manufacturer
one, so this is arguably less of a naming mismatch for these two than for
`manufacturer-wise-detail`, but it's the identical query either way).

### 7. `supplier-purchase-returns-detail` — mode `detail`, aggregate `return`

This combination (`detail` × `return`) was **not** covered by any prior
evidence doc — it differs from both `manufacturer-wise-detail`
(`detail` × `receiving`) and `supplier-purchase-returns-summary`
(`supplier-summary` × `return`). Genuine new verification was performed.

**Production query** (reproduced verbatim, `canonicalKinds = "'purchase-return'"`,
`detail := mode == "detail"` → `itemExpression = "l.item_name"`,
`amountExpression = "l.line_total"`, `groupBy` includes `l.line_number`):

| | row_count | total_qty | total_amount |
|---|---|---|---|
| Production | 2,481 | 57,868.0000 | **3,562,313.3000** |
| Independent (`INNER JOIN` on lines, no correlated LATERAL) | 2,481 | 57,868.0000 | **3,562,313.3000** |

**Arithmetic MATCHED exactly** between production and the independent
cross-check. Row count (2,481) matches the already-known total purchase-return
line count from `docs/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md`'s
`purchase-return-detail` leaf (different function,
`purchaseLineDetailReadModelQuery`, but the same underlying 2,481
`business_document_lines` rows for `kind = 'purchase-return'`). Total quantity
(57,868.0000) also matches the invoice-level total from leaf 4 above, as
expected (same stock-ledger-derived quantity, summed two different ways).

**New finding: total_amount disagrees with the invoice-summary leaf's amount by PKR 35,762.30.**
The `detail` mode sums `l.line_total` (2,481 individual line amounts); the
`invoice-summary` mode (leaf 4 above) sums `COALESCE(ple.amount,
d.total_amount)` (634 document-header amounts). These two are **not** the
same figure for purchase-return documents:

```sql
-- per-document reconciliation: business_documents.total_amount vs SUM(business_document_lines.line_total)
SELECT count(*) FILTER (WHERE ABS(total_amount - line_sum) > 0.01) AS mismatched_docs,
       count(*) AS total_docs, SUM(line_sum - total_amount) AS net_gap
FROM (SELECT d.total_amount, (SELECT SUM(l.line_total) FROM business_document_lines l
        WHERE l.document_id = d.id) AS line_sum
      FROM business_documents d WHERE d.kind = 'purchase-return' AND d.status = 'posted') x;
```

| mismatched_docs | total_docs | net_gap |
|---|---|---|
| 566 | 634 | 3,5762.3000 |

**566 of 634 purchase-return documents (89.3%) have a header `total_amount`
that differs from the sum of their own line `line_total`s**, sometimes by a
large margin — e.g. document 1856: header `total_amount = 20,052.0000` vs
`SUM(line_total) = 85,897.1800` (a 4.3× gap); document 1828: header
`26,644.0000` vs line-sum `36,384.2000`. This is **not** the purchase-order
amount-always-0 bug from the Phase O doc (both figures here are non-zero and
individually plausible); it's a genuine internal inconsistency between two
different "amount" sources for the same document, meaning the `detail`-mode
leaf (`supplier-purchase-returns-detail`) and the `invoice-summary`-mode leaf
(`purchase-return-summary`) would show **materially different totals for the
same underlying return documents** if both were displayed side by side — a
real defect for whichever number is wrong, though which one legacy actually
reported (per-line sum vs header ledger amount) cannot be determined without
live legacy access. Flagging precisely for the next promotion pass, per the
`purchase-order` amount bug precedent already established in the companion
Phase O doc.

**Verdict: MATCHED** (arithmetic self-consistent, independently reproduced)
**for quantity and row count; MISMATCH-DOCUMENTED** for a newly-found
cross-leaf amount inconsistency (header `total_amount` vs `SUM(line_total)`
disagree for 566/634 purchase-return documents, net gap PKR 35,762.30
tenant-wide) that affects this leaf's Amount column relative to the sibling
`purchase-return-summary` leaf's Amount column.

---

## Verdict summary

| # | Leaf | Phase | ProjectionStatus (metadata) | Verdict |
|---|---|---|---|---|
| 1 | `item-wise-item-wise-net-sales` | N | event-ledger (accurate — raw passthrough) | Diagnostic only — needs a real `GROUP BY item_id` query, none exists |
| 2 | `sale-return-summary-inv-type-wise` | N | event-ledger (accurate — raw passthrough) | Diagnostic only — needs a real `GROUP BY bd.kind` query, none exists |
| 3 | `dead-item-list` | N | event-ledger (accurate — raw passthrough, structurally cannot show unsold items) | Diagnostic only — needs a `master_items`-rooted anti-join, materially different query shape, plus a legacy cutoff-threshold decision |
| 4 | `purchase-return-summary` | O | event-ledger (metadata gap — query is real) | **MATCHED** (634 docs, qty 57,868.0000, amount 3,526,551.0000) |
| 5 | `supplier-wise-detail` | O | event-ledger (metadata gap — query is real) | **MATCHED**, byte-identical query to already-verified `manufacturer-wise-detail` |
| 6 | `supplier-wise-purchase-detail` | O | event-ledger (metadata gap — query is real) | **MATCHED**, byte-identical query to already-verified `manufacturer-wise-detail` |
| 7 | `supplier-purchase-returns-detail` | O | event-ledger (metadata gap — query is real) | **MATCHED** (quantity/row-count) / **MISMATCH-DOCUMENTED** (Amount disagrees with leaf 4's Amount for the same documents by PKR 35,762.30 net, 566/634 docs) |

**Summary:** the O-phase leaves in this remainder set behave exactly like the
14 already documented in `docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md`
— genuinely real grouped SQL mislabeled `"event-ledger"` in the API metadata,
with arithmetic that independently reconciles, plus one newly-surfaced
cross-leaf amount inconsistency (detail-mode line-sum vs invoice-summary-mode
header/ledger amount for purchase-return documents). The N-phase leaves are
genuinely raw/ungrouped as their metadata claims — unlike the O-phase
leaves, there is no hidden real query behind the label here; all three need
new query functions built from scratch, with `dead-item-list` needing the
most substantial rework (a `master_items` anti-join rather than a
`business_document_lines`-rooted projection) and a legacy-semantics decision
on its dead-item cutoff.

## Companion test

`services/api/internal/httpapi/report_golden_no_remainder_test.go` locks:
- registry wiring (mode/title/aggregate) for all 7 leaves,
- the current `event-ledger` `ProjectionStatus` for all 7 (so a future
  promotion pass updates this doc/test together instead of silently
  drifting),
- that the 3 N leaves' empty-mode dispatch is byte-identical to the raw
  `salesReadModelQuery` passthrough (no `GROUP BY`, no `master_items` join),
- that `supplier-wise-detail`/`supplier-wise-purchase-detail` produce
  byte-identical SQL text to `manufacturer-wise-detail`, and
- that `purchase-return-summary` dispatches to the real grouped
  `purchaseReadModelQuery` (contains `GROUP BY`, `business_documents`,
  `party_ledger_entries`, `'purchase-return'`, `d.status = 'posted'`).

Verified with `go vet ./services/api/...` (clean) and
`go test -run TestPhaseNORemainder ./services/api/internal/httpapi/ -v`
(5/5 passing, offline, no DB dependency).

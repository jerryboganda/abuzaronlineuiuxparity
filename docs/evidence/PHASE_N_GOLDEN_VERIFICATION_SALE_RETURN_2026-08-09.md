# Phase N Golden Verification — Daily Reports/Sales Return + Selected Sales Summary (2026-08-09)

Scope: independent Postgres-internal cross-verification of 5 assigned report
leaves from the Daily Reports/Sale and Daily Reports/Sales Return groups
(`services/api/internal/httpapi/reports.go` lines ~148–217):

1. `selected-sales-and-summaries-report`
2. `sales-return-detail`
3. `sales-return-summary`
4. `sales-return-summary-inv-wise`
5. `sales-return-detail-inv-wise`

Live legacy SQL Server is not reachable in this environment (confirmed:
Windows trusted-auth fails), so all verification here is Postgres-internal,
per the wave-1/wave-2 precedent set in
`docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md`: each leaf's own SQL
(read from `reports.go`, not modified) was executed verbatim (only an outer
`document = ANY(...)` sample filter added, never touching the projection
logic) against the sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` /
branch `ffffffff-ffff-ffff-ffff-ffffffffffff`, and the output was
independently recomputed from raw `business_documents` /
`business_document_lines` columns using arithmetic that does not copy the
query's own COALESCE chains.

`reports.go` was read-only throughout — no application code was changed.
This pass added one file: `services/api/internal/httpapi/report_golden_n_salereturn_test.go`
(5 tests, all passing, no DB required — they lock registry wiring and pin
SQL-fragment evidence, matching the style of the existing
`report_golden_o_purchase_test.go` / `report_golden_n_summary_test.go`).
`go vet ./services/api/...` is clean.

Standard RLS preamble used for every query below:

```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', '', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```

## Sandbox data shape (checked first, applies to all 5 leaves)

```sql
SELECT kind, status, count(*) FROM business_documents
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND kind IN ('cash-return','credit-return','open-cash-return','open-credit-return',
               'cash-sale-return','credit-sale-return','open-sale-return')
GROUP BY kind, status;
-- cash-sale-return | posted | 30704   (only this kind/status combination exists)
```

- Only **`cash-sale-return`** documents are present in this sandbox tenant
  (30,704 posted, 0 of any other status) — `credit-sale-return` and
  `open-sale-return` have **0** rows. This is a data-availability gap in the
  sandbox, not a query gap (see "kind-spelling coverage" below, which is
  verified at the SQL-text level regardless of which kinds happen to have
  data).
- All 30,704 documents share a single `customer_id` (`57ea27bc-ea87-4dc1-be81-02377bce3f05`,
  party name "CASH SALES CUSTOMER") — the `NULL customer_id → literal 'CASH'`
  fallback branch in the party COALESCE is present in the SQL but not
  exercised by this tenant's data (same limitation the already-golden
  `sale-detail` verification in `PHASE_N_O_GOLDEN_VERIFICATION_CORE` noted).
- `sync_events` has **0** rows with `aggregate IN ('sale_return', 'sale-return')`
  — 100% of output for every leaf below comes from the canonical
  `business_documents`/`business_document_lines` branch, not the
  compatibility-event union branches. This matches the wave-1 finding in
  `header-wise-transaction-summary` (30,704 sale-return documents, same
  count) that the `cash-sale-return` kind spelling is handled correctly by
  sibling queries.
- 634 posted `purchase-return` documents also exist but are out of scope for
  this doc (Phase O territory).

---

## Leaf 2 — `sales-return-detail` (REAL — `saleReturnDetailReadModelQuery`, line-detail mode)

### Registry wiring
`reports.go` sets `salesMode = "line-detail"` for this exact kind (via the
post-loop override at ~L211), mirroring `sale-detail`. Dispatch (~L5340-5341):
because `aggregateCondition == reportSaleReturnAggregate`, the handler calls
`saleReturnReadModelQueryMode("line-detail", ...)` → `saleReturnDetailReadModelQuery(...)`,
with `appendDailyDetailRows` as the row-scan function (same as `sale-detail`).

### Methodology
15 posted `cash-sale-return` documents were sampled evenly across the full
date range (2025-01-01 → 2026-07-31, every ~2047th row by `occurred_at`
ordering) to get variety across dates/items/quantities/single- and
multi-line invoices. The query's own CTE was run **verbatim** (byte-identical
to `saleReturnDetailReadModelQuery` in `reports.go`) with only the outer
`WHERE document = ANY(...)` filter added.

```sql
-- reports.go's own CTE, unmodified, filtered to the 15 sample documents
WITH sale_return_detail_read_model AS ( ... same as reports.go L2801-2842 ... )
SELECT document, occurred_at::text, party, item, quantity, amount,
       alias, item_description, sale_price, discount_percent, discount_value,
       item_discount, sales_tax_value, expiry_date, batch_number
FROM sale_return_detail_read_model
WHERE occurred_at >= '2025-01-01' AND occurred_at < '2026-08-01'
  AND document = ANY(ARRAY['63649','65695','67741','69787','71833','73879','75925',
                            '77971','80017','82063','84109','86155','88201','90247','92293']);
```

Independent recomputation, straight from `business_document_lines` with no
COALESCE-chain reuse:

```sql
SELECT bd.document_number, bl.item_name, bl.quantity, bl.line_total AS expected_amount,
       bl.unit_price AS expected_sale_price, bl.item_discount AS expected_item_discount,
       bl.tax_amount AS expected_tax, bl.batch_number AS raw_batch
FROM business_documents bd
JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND bd.document_number = ANY(ARRAY[...same 15...]);
```

### Result: quantity / price / amount MATCH. Tax MISMATCH (bug, documented below).

For all 22 sampled lines (15 documents, some multi-line): `quantity`,
`amount` (= `bl.line_total`), `sale_price` (= `bl.unit_price`), `alias` (=
`bl.item_code`, since `legacy_payload` has no `AliasName`/`Amount`/`SalePrice`
keys in this dataset), `discount_percent` (genuinely variable, sourced
correctly from `legacy_payload->>'DiscPerc'`: distinct values 0.00, 10.00,
5.00, 2.25, 48.70, 5.18, 5.95, 15.00, 3.41, 7.38, …), and `batch_number` all
matched the independently recomputed values exactly.

`sales_tax_value` **did not match** for 2 of the 15 sampled documents
(`67741` expected 190.68, reported 0.00; `84109` expected 45.76, reported
0.00).

### ⚠ Bug found: "SalesTax Value" silently shows 0.00 — same defect class as the already-fixed `sale-detail`, unfixed here

`saleReturnDetailReadModelQuery`'s `sales_tax_value` column is:

```sql
COALESCE(NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), bl.tax_amount::text, '0.00')
```

Global check across all 44,579 posted `cash-sale-return` lines in the
tenant:

```sql
SELECT bl.legacy_payload->>'SalesTax' AS legacy_val, count(*)
FROM business_documents bd JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.kind='cash-sale-return' AND bd.status='posted'
GROUP BY 1;
-- 0.00 | 44579   (100% — every line, no exceptions)

SELECT count(*), COALESCE(SUM(bl.tax_amount),0)
FROM business_documents bd JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.kind='cash-sale-return' AND bd.status='posted'
  AND NULLIF(bl.legacy_payload->>'SalesTax','') IS NOT NULL
  AND bl.legacy_payload->>'SalesTax' <> bl.tax_amount::text AND bl.tax_amount <> 0;
-- 1970 | 311703.0500
```

`legacy_payload->>'SalesTax'` is the **literal string `"0.00"` for all
44,579 lines** (never empty), so `NULLIF(..., '')` never nulls it out and the
COALESCE always stops at the first branch — `bl.tax_amount` (a `NOT NULL
numeric(19,4)` column reliably populated with the real per-line tax) is
never reached. **1,970 of 44,579 lines (4.42%) have real, non-zero tax that
is silently reported as `0.00`**, totaling **Rs 311,703.05** of tax value
hidden tenant-wide on this leaf.

This is the **exact same defect class** already found and independently
fixed for the sibling sale leaf: `dailySalesDetailReadModelQuery` (used by
`sale-detail`, see `docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md`
leaf 1, and confirmed in the current source at `reports.go` ~L2036) now puts
`bl.tax_amount::numeric(19,2)::text` **first** in its COALESCE chain, with an
inline comment documenting that `legacy_payload->>'SalesTax'` is always a
stuck literal `"0.00"` (verified there: 620,615/620,615 lines). That fix was
never propagated to `saleReturnDetailReadModelQuery`, a structurally
identical but separately-maintained function for the return path. The
percentage affected (4.42% here vs. 4.54% for sale-detail) is close enough to
strongly suggest the same root migration cause (imported tax amounts stored
correctly in `business_document_lines.tax_amount`, but the retained legacy
display payload's `SalesTax` field was always written as `"0.00"` regardless
of the real value, for both sale and sale-return legacy source rows).

**Fix needed** (for the agent authorized to edit `reports.go`): reorder
`saleReturnDetailReadModelQuery`'s `sales_tax_value` COALESCE to match
`dailySalesDetailReadModelQuery`'s already-fixed order:
`COALESCE(bl.tax_amount::numeric(19,2)::text, NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), '0.00')`.
Not applied here per this pass's read-only constraint on `reports.go`. Pinned
by `TestPhaseNGoldenSaleReturnDetailSalesTaxCoalesceOrderBug` in the
companion test file — that test will fail the moment the fix lands, as a
forcing function to update this doc's verdict.

**Secondary, lower-severity observation:** `saleReturnDetailReadModelQuery`'s
`discount_percent`/`discount_value` COALESCE chains also lack the
`line_gross`-vs-`line_total` CASE fallback that `dailySalesDetailReadModelQuery`
already has (reports.go ~L2005-2020). This is currently **dormant** in this
tenant — `business_document_lines.item_discount`/`customer_discount`/
`supplier_discount` are 0 for all 44,579 `cash-sale-return` lines, and
`legacy_payload->>'DiscPerc'` is populated and correct wherever a discount
exists — so it produces no observed wrong output today, but it is a
structural parity gap the promotion agent may want to close alongside the
tax fix for consistency.

### Verdict: **MISMATCH-DOCUMENTED** (quantity/price/amount/discount/batch: MATCHED; tax: real, quantified, unfixed bug — same class as the already-fixed `sale-detail`)

---

## Leaf 3 — `sales-return-summary` (REAL — `saleReturnReadModelQuery` + `invoiceSummaryReadModelQuery`, invoice-summary mode)

### Registry wiring
`salesMode = "invoice-summary"` (set in the loop for `report.kind ==
"sales-return-summary"`). Dispatch → `saleReturnReadModelQueryMode("invoice-summary", ...)`
→ `invoiceSummaryReadModelQuery(saleReturnReadModelQuery(""), pagination)`:
groups the raw per-line rows (`document, occurred_at, party, item, quantity,
amount` where `amount = bd.total_amount` repeated per line) by `document`,
taking `MAX(occurred_at)`, `MAX(party)`, `SUM(quantity)`, `MAX(amount)`.

### Methodology and result
Ran the query verbatim for the same 15 sample documents; independently
recomputed `SUM(bl.quantity)` and confirmed `bd.total_amount` directly:

```sql
SELECT bd.document_number, bd.total_amount, SUM(bl.quantity) AS qty_sum, count(bl.id) AS line_count
FROM business_documents bd LEFT JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.document_number = ANY(ARRAY[...15...])
GROUP BY bd.document_number, bd.total_amount;
```

| document | total_amount (expected) | qty_sum (expected) | line_count | query's amount | query's qty_sum |
|---|---|---|---|---|---|
| 63649 | 181.0000 | 20.0000 | 1 | 181.0000 | 20.0000 |
| 65695 | 76.0000 | 8.0000 | 1 | 76.0000 | 8.0000 |
| 67741 | 1251.0000 | 1.0000 | 1 | 1251.0000 | 1.0000 |
| 69787 | 79.0000 | 30.0000 | 1 | 79.0000 | 30.0000 |
| 71833 | 121.0000 | 1.0000 | 1 | 121.0000 | 1.0000 |
| 73879 | 235.0000 | 7.0000 | 1 | 235.0000 | 7.0000 |
| 75925 | 441.0000 | 11.0000 | 2 | 441.0000 | 11.0000 |
| 77971 | 36.0000 | 5.0000 | 1 | 36.0000 | 5.0000 |
| 80017 | 1989.0000 | 31.0000 | 3 | 1989.0000 | 31.0000 |
| 82063 | 311.0000 | 4.0000 | 1 | 311.0000 | 4.0000 |
| 84109 | 301.0000 | 1.0000 | 1 | 301.0000 | 1.0000 |
| 86155 | 856.0000 | 5.0000 | 5 | 856.0000 | 5.0000 |
| 88201 | 468.0000 | 20.0000 | 1 | 468.0000 | 20.0000 |
| 90247 | 39.0000 | 1.0000 | 1 | 39.0000 | 1.0000 |
| 92293 | 551.0000 | 1.0000 | 1 | 551.0000 | 1.0000 |

15/15 documents matched exactly on both amount and quantity, including all
3 multi-line invoices in the sample (75925: 2 lines, 80017: 3 lines, 86155:
5 lines) — the `SUM(quantity)`/`MAX(amount)` aggregation is correct.

**Observation (not a query bug):** for document `67741`, the line-level
`unit_price (1059.32) + tax_amount (190.68) = 1250.00` but the header
`total_amount = 1251.00` — a Rs 1.00 difference, plausibly a legacy
whole-rupee header-rounding convention. The report query reads
`bd.total_amount` directly (the header's own source-of-truth value), so it
is not recomputing or misrepresenting anything — this is a migration/legacy
semantics question about the header total, out of scope for this leaf's
projection-correctness verdict.

### Verdict: **MATCHED** (15/15 sample documents, both amount and quantity aggregation)

---

## Leaf 4 — `sales-return-summary-inv-wise` (REAL — byte-identical to Leaf 3)

### Registry wiring
`salesMode = "invoice-summary"` is also set for `report.kind ==
"sales-return-summary-inv-wise"` (same `if` condition covers both kind
strings, ~L200 in `reports.go`). Neither kind is touched by the
`sales-return-detail`-only post-loop override. The dispatch code
(`spec.aggregateCondition == reportSaleReturnAggregate` →
`saleReturnReadModelQueryMode(spec.salesMode, ...)`) is identical for both
leaves, and `spec.salesMode` is the literal same string `"invoice-summary"`
for both.

### Finding: byte-identical SQL to `sales-return-summary` — confirmed, not a bug

```go
queryA := saleReturnReadModelQueryMode("invoice-summary", "LIMIT $6 OFFSET $7") // sales-return-summary
queryB := saleReturnReadModelQueryMode("invoice-summary", "LIMIT $6 OFFSET $7") // sales-return-summary-inv-wise
// queryA == queryB, verified by TestPhaseNGoldenSaleReturnSummaryLeavesAreByteIdenticalSQL
```

Both leaves also route through the same `salesSummaryReportColumns("invoice-summary")`
column set in `reportDefinitionFor`, so the API response shape (columns +
data) is identical between the two kinds — they differ only in `title`
("Sales Return summary" vs. "Sales Return Summary Inv.wise") and the URL
`kind` slug used to select them. This mirrors the wave-1
`reprinting-*` precedent explicitly called out in the assignment: two
distinct legacy menu leaf names that resolve to one underlying query/columns
contract. **Stating this explicitly as the finding, not treating it as a
defect** — legacy PowerBuilder frequently exposed the same underlying report
under two menu paths with different titles (e.g. one from "Daily Reports"
and one from "Sales Reports"), and nothing in the assigned data or code
suggests they were meant to differ in calculation.

Because the SQL is byte-identical to Leaf 3, the same 15-document
cross-check in Leaf 3's section applies verbatim and MATCHED for both.

### Verdict: **MATCHED** (identical to Leaf 3's verified query — confirmed byte-identical SQL, not independently re-verified as a separate calculation because there is no separate calculation to verify)

---

## Leaf 1 — `selected-sales-and-summaries-report` (event-ledger)

### Registry wiring
`aggregateCondition = reportSaleAggregate` (`se.aggregate = 'sale'`),
`salesReadModel = true`, but `salesMode = ""` — this kind is not one of the
3 cases (`sale-summary`, `sale-summary-inv-wise`, `sale-summary-invoice-wise`
→ `invoice-summary`; `sale-detail` → `line-detail`) matched by the switch at
~L172-176. Dispatch routes to `salesReadModelQueryMode(reportSaleAggregate,
"", ...)`, whose mode `""` matches none of the named modes in
`salesReadModelQueryMode`'s if/else chain (~L2162-2195) and falls through to
the plain `salesReadModelQuery(aggregateCondition, pagination)` default —
the raw, ungrouped union of canonical `cash-sale`/`credit-sale`
`business_documents`/`business_document_lines` plus compatibility
`sales_documents`/`sync_events` rows, one row per line, columns `document,
occurred_at, party, item, quantity, amount` (with `amount = bd.total_amount`
duplicated onto every line of a multi-line invoice).

### Confirmed functioning, non-vacuous, but generic
```sql
SELECT count(*) FROM business_documents bd
LEFT JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.branch_id='ffffffff-ffff-ffff-ffff-ffffffffffff'
  AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted';
-- 620615  (same underlying line count as the already-golden sale-detail leaf)
```

The leaf returns real data (620,615 canonical rows for this tenant), so it
is not broken — but its projection is the generic 6-column event-ledger
shape, not a "Selected Sales and Summaries" contract. The legacy leaf's own
name ("Selected...Summaries") strongly implies a user-parameterized
selection of one or more summary sub-reports (a composite/menu-driven report
rather than a single fixed grouping) — there is no single obvious
grouping/columns contract to promote this to without a captured legacy
screenshot or column list, unlike `sales-return-summary`'s clear
invoice-summary shape. **Recommendation for the promotion agent:** treat
this as needing a legacy UI/column capture before any real projection can be
assigned, not a mechanical mode-wiring fix like Leaf 5 below.

### Verdict: **event-ledger, functioning (non-vacuous), needs legacy column capture before promotion** — documented for the promotion agent, not fixed here

---

## Leaf 5 — `sales-return-detail-inv-wise` (event-ledger)

### Registry wiring
`aggregateCondition = reportSaleReturnAggregate`, `salesReadModel = true`,
`salesMode = ""` — unlike `sales-return-detail` (which gets an explicit
post-loop override to `"line-detail"`), this kind is not matched by the
`if report.kind == "sales-return-summary" || report.kind ==
"sales-return-summary-inv-wise"` condition either, despite its name implying
the same "Inv.wise" (per-invoice) grouping as Leaf 4. Dispatch routes to
`saleReturnReadModelQueryMode("", ...)`, which (mode not `"line-detail"`,
not `"invoice-summary"`) falls through to the raw, ungrouped
`saleReturnReadModelQuery(pagination)` — the same base query Leaf 3/4 group
by document, but here **left ungrouped**.

### Confirmed: NOT actually invoice-wise (contradicts its own name)
```sql
SELECT count(*) AS raw_rows, count(DISTINCT bd.document_number) AS distinct_invoices
FROM business_documents bd
LEFT JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.branch_id='ffffffff-ffff-ffff-ffff-ffffffffffff'
  AND bd.kind IN ('cash-return','credit-return','open-cash-return','open-credit-return',
                  'cash-sale-return','credit-sale-return','open-sale-return')
  AND bd.status='posted';
-- raw_rows = 44579, distinct_invoices = 30704
```

44,579 raw rows across only 30,704 distinct invoices confirms the query is
**not** grouped by invoice — it returns one row per `business_document_lines`
row (with `amount` duplicated per line, same as Leaf 1), the opposite of
what "Inv.wise" implies for the sibling `sales-return-summary-inv-wise`
leaf.

### Two plausible resolutions — a judgment call, not a mechanical fix, flagged for the promotion agent
1. **If the legacy leaf really is a per-invoice grouped view** (most likely,
   given the "Inv.wise" name parallels `sales-return-summary-inv-wise`):
   add `"sales-return-detail-inv-wise"` to the `salesMode = "invoice-summary"`
   case alongside `sales-return-summary`/`sales-return-summary-inv-wise` in
   the registry loop (~L200). This would make it byte-identical to Leaf 3/4,
   which — while structurally simple — needs a legacy-semantics call: is
   "Detail Inv. wise" meant to differ from "Summary Inv. wise" (e.g. by
   showing per-item lines *within* an invoice-grouped layout, which
   `invoiceSummaryReadModelQuery`'s single-row-per-document collapse cannot
   express) or is it genuinely the same shape under yet another legacy menu
   path?
2. **If "Detail Inv. wise" legitimately means "line-detail rows, sorted/grouped
   for display by invoice"** (distinct from both `sales-return-detail`'s flat
   detail list and the summary leaves' collapsed one-row-per-invoice
   totals): the correct fix is closer to `sales-return-detail`'s
   `"line-detail"` mode (reusing `saleReturnDetailReadModelQuery`, which
   already has the richer alias/price/discount/tax/batch columns) with an
   `ORDER BY document, item_description` grouping presentation rather than a
   SQL `GROUP BY` — i.e., promote it to `salesMode = "line-detail"` like its
   sibling, not `"invoice-summary"`.

Both options are legacy-semantics decisions this environment cannot resolve
without a captured legacy screenshot or column list (no live SQL Server
access). Recorded here so the promotion agent doesn't have to re-derive the
row-count evidence from scratch.

### Verdict: **event-ledger, functioning (non-vacuous, 44,579 rows), naming/mode-wiring gap documented for the promotion agent** — not fixed here

---

## Summary table

| Leaf | Kind | salesMode | Status | Verdict |
|---|---|---|---|---|
| Selected Sales and Summaries Report | `selected-sales-and-summaries-report` | `""` | event-ledger | Functioning, needs legacy column capture before promotion |
| Sales Return detail | `sales-return-detail` | `line-detail` | REAL | MISMATCH-DOCUMENTED — qty/price/amount/discount/batch MATCHED; tax bug (same class as the already-fixed `sale-detail`, unfixed here: 1,970/44,579 lines, 4.42%, Rs 311,703.05 hidden) |
| Sales Return summary | `sales-return-summary` | `invoice-summary` | REAL | MATCHED (15/15 sample) |
| Sales Return Summary Inv.wise | `sales-return-summary-inv-wise` | `invoice-summary` | REAL | MATCHED — byte-identical SQL to `sales-return-summary`, confirmed intentional (wave-1 `reprinting-*` precedent), not a bug |
| Sales Return Detail Inv.wise | `sales-return-detail-inv-wise` | `""` | event-ledger | Functioning, naming/mode-wiring gap (not actually invoice-wise) documented for promotion agent |

## Files touched this pass
- `docs/evidence/PHASE_N_GOLDEN_VERIFICATION_SALE_RETURN_2026-08-09.md` (this file)
- `services/api/internal/httpapi/report_golden_n_salereturn_test.go` (new; 5
  tests, all passing, no live DB required)

`reports.go` was read-only. `go vet ./services/api/...` clean;
`go test -run TestPhaseNGoldenSaleReturn -v ./services/api/internal/httpapi/`
passes 5/5.

---

## Addendum (2026-08-09, wave 2 follow-up) — sales-return-detail tax bug fixed

The tax COALESCE-order bug documented above for `sales-return-detail`
(leaf 2) has been fixed in `saleReturnDetailReadModelQuery`, mirroring the
identical fix already applied to `dailySalesDetailReadModelQuery` in wave 1:
`bl.tax_amount::numeric(19,2)::text` now comes first in the `sales_tax_value`
COALESCE chain, ahead of the stuck `legacy_payload->>'SalesTax'` literal
`"0.00"`. `TestPhaseNGoldenSaleReturnDetailSalesTaxCoalesceOrderBug` (in the
companion test file) has been flipped from pinning the buggy order to
asserting the fixed order, and passes.

**Verdict for `sales-return-detail` updated: MATCHED** (was
MISMATCH-DOCUMENTED). All previously-verified columns (quantity, price,
amount, discount, batch) were already MATCHED; tax is now correct too.

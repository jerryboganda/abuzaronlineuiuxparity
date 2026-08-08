# Phase N/O Golden Verification — Core Flagship Leaves (2026-08-09)

Scope: independent Postgres-internal cross-verification of 5 assigned report
leaves (`sale-detail` / Daily Sale Detail, `refused-sales-detail`,
`purchase-detail`, `purchase-return-detail`, `p-o-based-purchase-disparity`)
against their live `reports.go` query logic. Live legacy SQL Server is not
reachable in this environment (confirmed: Windows trusted-auth fails), so all
verification here is Postgres-internal: for each leaf, the report's own SQL
(read from `services/api/internal/httpapi/reports.go`, not modified) was
executed verbatim (or with only an outer `document IN (...)` sample filter
added, never touching the projection logic) against the sandbox tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` / branch
`ffffffff-ffff-ffff-ffff-ffffffffffff`, and the output was independently
recomputed from raw `business_documents` / `business_document_lines` /
`stock_allocations` / `stock_ledger` columns using arithmetic and JSON-key
presence checks that do not copy the query's own COALESCE chains.

`reports.go` was read-only throughout; no application code was changed. All
SQL shown below was run directly with `psql` after the standard RLS preamble:

```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', 'ffffffff-ffff-ffff-ffff-ffffffffffff', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```

## ⚠ Prominent findings — two genuine, reproducible bugs

The broader sample for leaf #1 (Daily Sale Detail) surfaced a **new,
100%-reproducible bug** beyond the previously golden-verified invoice 695336,
and leaf #4 (Purchase Return Detail) surfaced a second, independent bug. Both
are documented in full in their sections below; summarized here first because
the assignment asked for prominent flagging:

1. **Daily Sale Detail — "SalesTax Value" column always shows `0.00`, even
   when real tax exists.** `dailySalesDetailReadModelQuery` populates
   `sales_tax_value` via
   `COALESCE(NULLIF(bl.legacy_payload->>'SalesTax', ''), ..., bl.tax_amount::text, '0.00')`.
   Every single imported sale line in the tenant (620,615 / 620,615, 100%)
   carries `legacy_payload->>'SalesTax' = '0.00'` — a non-empty literal
   string, so `NULLIF(..., '')` never nulls it out and the COALESCE always
   stops at that first branch. The correctly computed `bl.tax_amount` column
   (which does hold real per-line tax, e.g. from tax-inclusive pricing) is
   never reached. Result: **28,184 of 620,615 sale lines (4.54%) have real,
   non-zero tax that is silently reported as `0.00`** in the Daily Sale
   Detail "SalesTax Value" column tenant-wide, not just in the sample.
   This is **not** the pack/loose quantity/unit-price repair regressing —
   that repair's own invariant (`line_total = ROUND(unit_price*quantity,4)`)
   holds with **zero violations across all 620,615 lines** — it is a
   separate, pre-existing defect in the tax-column COALESCE ordering.

2. **Purchase Return Detail — "Purchase Price" column shows the item's
   average inventory cost, not the actual return price, for every row.**
   `purchaseLineDetailReadModelQuery`'s `purchase_price` COALESCE checks
   `legacy_payload->>'PurPrice'`, but purchase-**return** legacy rows store
   the price under the key `PRPrice` instead (confirmed: 0 of 2,481
   purchase-return lines have a `PurPrice` key; 100% have `PRPrice`). Because
   the code never looks for `PRPrice`, every purchase-return-detail row falls
   through to `NULLIF(l.unit_cost, 0)::text` — the line's **average stock
   cost**, a different, pre-populated column that is not equal to the actual
   return transaction price. `unit_cost` differs from the true return price
   in 551 of 2,481 lines (22.2%) by more than rounding, sometimes
   substantially: e.g. document 2120, line "COMFORT CORSET ALL SIZE" reports
   Purchase Price = **738.1765** while the same row's Amount column (for
   qty = 1) is **915.0000** — a self-evident, in-row arithmetic
   inconsistency visible without any external reference data.

Both are genuine `reports.go` logic defects (COALESCE ordering / wrong legacy
JSON key), not data-import gaps, and neither has been fixed here per the
read-only constraint on `reports.go`.

---

## Leaf 1 — `sale-detail` / Daily Sale Detail (`dailySalesDetailReadModelQuery`)

### Methodology
- Query under test: `dailySalesDetailReadModelQuery` (reports.go ~L1988–2136),
  reached via `salesReadModelQueryMode("line-detail")` →
  `appendDailyDetailRows`, columns per `dailySaleDetailColumns()`.
- The sandbox tenant has **0** rows in `sales_documents` and the only
  `sync_events` aggregate present is `historical_document` (never `sale`), so
  the query's 2nd and 3rd `UNION ALL` branches are empty for this tenant —
  100% of output comes from the 1st branch (`business_documents` ⋈
  `business_document_lines`), which is what was independently re-derived.
- Global (non-sampled) integrity checks first, then a targeted 21-invoice /
  52-line sample chosen for variety (PackUnits > 1, PackUnits = 1, DiscPerc >
  0, tax_amount > 0, credit-sale), executed by running the query's own CTE
  **verbatim** (byte-identical to reports.go) with only the final `WHERE
  document ILIKE …` filter replaced by `WHERE document IN (...)` restricted
  to the sample document numbers — the projection logic itself was untouched.

### Global integrity (all 620,615 posted cash-sale/credit-sale lines, not sampled)
```sql
SELECT count(*) FILTER (WHERE ABS(line_total - ROUND(unit_price*quantity,4)) > 0.0002) AS violations,
       count(*) AS total
FROM business_documents bd JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted';
```
Result: **`violations = 0` / `total = 620615`.** This extends the pack/loose
`quantity*unit_price = line_total` invariant (previously verified only for
invoice 695336) to the entire tenant with zero exceptions — the strongest
possible evidence the 2026-08-08 repair holds broadly, not just for the one
golden invoice.

Key-presence checks used to understand each COALESCE branch's real behavior
(counts over all 620,615 lines): `legacy_payload ? 'Amount'` → 0 (amount
always falls to `bl.line_total`); `? 'AliasName'` → 0 (alias falls to
`master_items.payload->>'AliasName'` then `item_code`/`item_legacy_id`);
`? 'SalePrice'` → 620,615 (100%, always used verbatim); `legacy_payload->>'SalesTax'`
distinct values → **only `'0.00'`, 620,615/620,615** (the bug above);
`line_gross > line_total` (a real gross discount) → 0 lines, so
`discount_value`'s CASE fallback is never exercised in this tenant and
`discount_percent`/`item_discount` are captured verbatim from the legacy
payload where present (1,834 lines have non-zero `DiscPerc`).

### Sample: 21 invoices, 52 lines

Query executed (canonical branch, verbatim from reports.go, sample filter only):
```sql
WITH sales_detail_read_model AS ( ... exact reports.go CTE ... )
SELECT document, occurred_at::text, party, item, quantity, amount,
       alias, item_description, sale_price, discount_percent, discount_value,
       item_discount, sales_tax_value, expiry_date, batch_number
FROM sales_detail_read_model
WHERE document IN ('695336','654922','646399','664302','684360','754113','603511',
    '847141','817216','652607','801812','745535','772633','684484','677441',
    '853857','630281','812122','822827','825627','845252')
ORDER BY document, item_description;
```

Every field for every one of the 52 returned lines was cross-checked against
raw `business_document_lines` / `master_items` / `stock_allocations`
independently (quantity = `bl.quantity` directly; alias = raw
`item_code`/`item_legacy_id` since no `AliasName` key exists; sale_price =
raw `legacy_payload->>'SalePrice'`; amount = raw `bl.line_total`, itself
equal to `ROUND(unit_price*quantity,4)`; discount_percent = raw
`legacy_payload->>'DiscPerc'` where present; batch/expiry = raw
`bl.batch_number`/`bl.expiry_date` since `stock_allocations` has **0** rows
for any of these lines — the LATERAL allocation join is vacuous for this
tenant's sale lines, so batch/expiry always fall through to the line's own
retained values, e.g. the legacy placeholder `"."`). No discrepancy was found
in any of these fields for any of the 52 lines.

The only discrepancy category found was the sales_tax_value masking bug
described above, hitting exactly 4 of the 21 sample invoices:

| Invoice | Item / line | Report `sales_tax_value` | True `bl.tax_amount` |
|---|---|---|---|
| 630281 | NESTLE 0.5LTR | 0.00 | 9.15 |
| 812122 | PROTE SUN LOTION 60ML | 0.00 | 303.67 |
| 822827 | VELOSTY CREAM 50G | 0.00 | 58.50 |
| 853857 | ACCU CHEK INSTANT 50S | 0.00 | 309.51 |

(A 5th line in the sample — 822827 / DANZ N 30MG TAB — shows `unit_price`
40.00 vs legacy `SalePrice` 33.90 with `tax_amount = 0`, i.e. the ×10
per-unit gap is *not* tax-shaped and doesn't fit the masking pattern; flagged
as an unexplained outlier for future investigation, not folded into the
tax-masking bug count above.)

### Verdict table (per assignment requirement — all 21 sampled invoices)

| Invoice | Kind | Characteristic sampled | Lines | Verdict |
|---|---|---|---|---|
| 695336 | cash-sale | golden (prior verification) | 1 | MATCHED |
| 654922 | credit-sale | credit-sale | 3 | MATCHED |
| 646399 | credit-sale | credit-sale | 5 | MATCHED |
| 664302 | cash-sale | PackUnits > 1 | 2 | MATCHED |
| 684360 | cash-sale | PackUnits > 1 | 1 | MATCHED |
| 754113 | cash-sale | PackUnits > 1 | 2 | MATCHED |
| 603511 | cash-sale | PackUnits > 1 | 1 | MATCHED |
| 825627 | cash-sale | PackUnits > 1 | 2 | MATCHED |
| 847141 | cash-sale | single-unit (PackUnits = 1) | 1 | MATCHED |
| 817216 | cash-sale | single-unit | 1 | MATCHED |
| 652607 | cash-sale | single-unit | 2 | MATCHED |
| 801812 | cash-sale | single-unit | 1 | MATCHED |
| 845252 | cash-sale | single-unit | 2 | MATCHED |
| 745535 | cash-sale | DiscPerc > 0 | 2 | MATCHED |
| 772633 | cash-sale | DiscPerc > 0 | 4 | MATCHED |
| 684484 | cash-sale | DiscPerc > 0 | 6 | MATCHED |
| 677441 | cash-sale | DiscPerc > 0 | 4 | MATCHED |
| 853857 | cash-sale | tax_amount > 0 | 1 | **MISMATCH-DOCUMENTED** (sales_tax_value masked to 0.00, true 309.51) |
| 630281 | cash-sale | tax_amount > 0 | 6 | **MISMATCH-DOCUMENTED** (1 of 6 lines: sales_tax_value masked, true 9.15) |
| 812122 | cash-sale | tax_amount > 0 | 1 | **MISMATCH-DOCUMENTED** (sales_tax_value masked, true 303.67) |
| 822827 | cash-sale | tax_amount > 0 | 4 | **MISMATCH-DOCUMENTED** (1 of 4 lines: sales_tax_value masked, true 58.50; plus 1 unexplained SalePrice/unit_price outlier noted above) |

**Overall leaf verdict: MATCHED for quantity/unit-price/line-total/alias/item/
discount-percent/expiry/batch across all 21 invoices and 100% of the tenant
(0 arithmetic violations in 620,615 lines); MISMATCH-DOCUMENTED for the
sales_tax_value column specifically, affecting 4/21 sampled invoices and
28,184/620,615 (4.54%) lines tenant-wide.** This is the pack/loose-repair
gating leaf, and the repair itself is now confirmed sound at full-tenant
scale — the tax-column bug is unrelated to that repair.

---

## Leaf 2 — `refused-sales-detail` (`documentReadModelQuery("refused-sale", "refused_sale", ...)`)

### Methodology
Same read model as quotations, reached via `spec.documentReadModel` /
`documentKind = "refused-sale"` / `documentAggregate = "refused_sale"`.

### Data availability check
```sql
SELECT kind, status, count(*) FROM business_documents WHERE tenant_id=... GROUP BY 1,2;
-- refused-sale: 0 rows (not present at all)
SELECT aggregate, count(*) FROM sync_events WHERE tenant_id=... GROUP BY 1;
-- aggregate = historical_document only; 0 rows with aggregate = 'refused_sale'
```
Both the canonical (`business_documents.kind = 'refused-sale'`) and
compatibility (`sync_events.aggregate = 'refused_sale'`) sources are
completely empty for this tenant. The exact `documentReadModelQuery` CTE was
run verbatim (both UNION branches) and confirmed to execute without error and
return **0 rows**, so the query is structurally sound but there is no data in
this sandbox to compare against a source-of-truth.

**Verdict: VACUOUS-NO-DATA.** No legacy reachability and no migrated
refused-sale documents in the sandbox tenant means this leaf cannot be
golden-verified this pass. Code review of the shared `documentReadModelQuery`
function (also used for quotations) did not surface a defect analogous to
leaves 1 or 4 — column list is document/date/party/item/quantity/amount only,
sourced directly from `bd.document_number`, `bd.occurred_at`, `mp.name`,
`bl.item_name`, `bl.quantity`, `bd.total_amount`, with no legacy-JSON
COALESCE chains to mask a nonzero field the way leaves 1 and 4 do.

---

## Leaf 3 — `purchase-detail` (`purchaseLineDetailReadModelQuery`, canonical kinds `pack-purchase`/`loose-purchase`/`opening-purchase`)

### Methodology
Query dispatched via `spec.purchaseMode == "line-detail"` →
`purchaseLineDetailReadModelQuery` (reports.go ~L2838), not the generic
`purchaseReadModelQuery` (that function's own `detail := mode == "detail"`
flag would never fire for `mode = "line-detail"`, but the dispatcher at
~L5149 special-cases `purchaseMode == "line-detail"` to route to the correct,
dedicated function — confirmed by reading the dispatch switch, not assumed).

### Global integrity (113,525 posted pack-purchase/loose-purchase/opening-purchase lines)
```sql
SELECT count(*) FILTER (WHERE ABS(line_total - ROUND(unit_price*quantity,4)) > 0.0002) violations, count(*) total ...
-- violations = 0 / total = 113525
```
Key-presence checks (100% coverage, all 113,525 lines): `legacy_payload ?
'taxAmount'` → 0, `? 'discountPercent'` → 0, `? 'DiscountValue'` → 0,
`? 'ExpiryDate'` → 0, `? 'BatchNumber'`/`'Batch'` → 0, `? 'Amount'`/`'LineTotal'`
→ 0 (all these COALESCE branches are dead for this tenant and correctly fall
through to raw columns); `? 'SalesTax'` → **0** (unlike leaf 1's sale lines,
purchase lines never carry this key at all, so `sales_tax_value` correctly
falls through to `bl.tax_amount` — **the leaf-1 masking bug does not
reproduce here**, confirmed against 11,303 pack-purchase lines with
`tax_amount > 0`, e.g. `UnitSalesTax * quantity` = `tax_amount` exactly for
sampled rows); `? 'PurPrice'` → **113,525/113,525 (100%)**, so
`purchase_price` correctly uses the legacy price verbatim everywhere in this
canonical set.

### Sample: 8 invoices, 46 lines (small/medium line counts; large document
"1" opening-purchase, 6,430 lines, excluded from the manual per-line sample
as impractically large but included in the global checks above)
```sql
WITH purchase_line_detail_read_model AS ( ... exact reports.go CTE ... )
SELECT ... FROM purchase_line_detail_read_model
WHERE document IN ('3213','3434','100','4010','543','2552','6400','5861')
ORDER BY document, item, batch_number;
```
Automated numeric diff against an independently-recomputed CTE (`purchase_price
expected = raw unit_price`, `sales_tax_value expected = raw tax_amount`,
`amount expected = ROUND(unit_price*quantity,4)`, joined by
document+item+quantity):

| Check | Mismatches / 46 rows |
|---|---|
| purchase_price | 3 |
| sales_tax_value | 0 |
| amount | 0 |

### Finding A (secondary, quantified, tenant-wide) — Opening-purchase Amount is always 0.00
```sql
SELECT kind, count(*) total, count(*) FILTER (WHERE unit_price=0 AND line_total=0) zero_price_lines,
       count(*) FILTER (WHERE unit_price=0 AND line_total=0 AND (legacy_payload->>'PurPrice')::numeric > 0) zero_but_purprice_nonzero
FROM ... GROUP BY kind;
```
| kind | total | zero_price_lines | zero_but_purprice_nonzero |
|---|---|---|---|
| loose-purchase | 33 | 0 | 0 |
| opening-purchase | 6,430 | **6,430 (100%)** | **6,430 (100%)** |
| pack-purchase | 107,062 | 7 | 0 |

Every opening-purchase line (the initial stock-load import) has
`unit_price = 0` and `line_total = 0` even though the legacy `PurPrice`
field carries a real, non-zero price. Because `purchase_price` reads
`legacy_payload->>'PurPrice'` directly, that column displays correctly; but
`amount` falls to `bl.line_total` (no `Amount`/`LineTotal` JSON key exists),
so **every Purchase Detail row sourced from an opening-purchase document
shows Amount = 0.00**, regardless of the real price. This is a data
characteristic of `business_document_lines` itself (likely an opening-balance
import gap upstream of `reports.go`), not a COALESCE-ordering defect like
leaves 1 and 4 — flagged here because it directly and completely breaks one
column of this report leaf for one whole document kind, not because
`reports.go` needs to change.

### Finding B (minor, rare) — residual pack-unit scaling in a handful of receipt lines
Of 66,380 lines with `PackUnits > 1`: 62,758 (94.5%) have `unit_price =
PurPrice` (correct, unscaled); only 5 have `unit_price = PurPrice ×
PackUnits` (e.g. document 543 / loose-purchase: TOFACENET 5MG TAB,
`unit_price` 3400.00 vs `PurPrice` 340.00 × `PackUnits` 10); 3,617 fall into
neither bucket and are explained entirely by Finding A (opening-purchase
zero-price lines, which also carry `PackUnits > 1` in many cases). The
5-line scaling anomaly is too small a fraction (0.008% of pack>1 lines) to
indicate a systemic recurrence of the pre-2026-08-08 pack/loose defect on the
purchase side, but is noted for completeness.

**Verdict: MATCHED** for quantity, sales_tax_value, discount_percent,
expiry_date, batch_number, and purchase_price across the sampled
pack-purchase/loose-purchase lines (43/46 exact; the 3 non-matching are the
isolated Finding B anomaly, all in document 543). **MISMATCH-DOCUMENTED**
for the Amount column specifically on opening-purchase-sourced rows
(100% of that document kind, 6,430 lines) — a data characteristic, not a
`reports.go` query bug.

---

## Leaf 4 — `purchase-return-detail` (`purchaseLineDetailReadModelQuery`, canonical kind `purchase-return`)

### Methodology
Same shared function as leaf 3, invoked with `aggregateCondition =
"se.aggregate = 'return'"` → `canonicalKinds = "'purchase-return'"`,
`eventAggregate = "return"`.

### Global integrity (2,481 posted purchase-return lines)
```sql
SELECT count(*) FILTER (WHERE ABS(line_total - ROUND(unit_price*quantity,4)) > 0.0002) violations, count(*) total ...
-- violations = 0 / total = 2481
```
Key-presence checks (100% coverage): `? 'SalesTax'` → 0 (falls through
correctly to `bl.tax_amount`, verified against 274 lines with
`tax_amount > 0`, e.g. `UnitSalesTax * quantity = tax_amount` exactly);
`? 'Amount'`/`'LineTotal'`/`'ExpiryDate'`/`'BatchNumber'`/`'Batch'` → 0 (all
correctly fall through to raw `line_total`/`expiry_date`/`batch_number`);
**`? 'PurPrice'` → 0 of 2,481 (0%)**. Purchase-return legacy rows use a
different key, `PRPrice`, for the identical semantic field (confirmed
present in 100% of sampled rows' `legacy_payload`), which the query's
`purchase_price` COALESCE never checks.

### The bug in detail
```
COALESCE(NULLIF(l.legacy_payload->>'PurPrice', ''),      -- never matches (return rows use 'PRPrice')
         NULLIF(l.legacy_payload->>'purchasePrice', ''), -- never matches
         NULLIF(l.pricing->>'purchasePrice', ''),        -- pricing is always '{}' here
         NULLIF(l.pricing->>'unitCost', ''),              -- pricing is always '{}' here
         NULLIF(l.unit_cost, 0)::text,                    -- <-- always matches: unit_cost is non-zero for all 2,481 lines
         l.unit_cost::text,
         l.unit_price::text, '') AS purchase_price,
```
`unit_cost` is populated (non-zero) for all 2,481 purchase-return lines, but
it is the item's **weighted-average inventory cost at time of return**, a
different quantity from the actual per-unit price the supplier credited for
that specific return (`unit_price` / legacy `PRPrice`):
```sql
SELECT count(*) FILTER (WHERE unit_cost <> unit_price) cost_neq_price,
       count(*) FILTER (WHERE ABS(unit_cost - (legacy_payload->>'PRPrice')::numeric) > 0.01) cost_neq_prprice,
       count(*) total ...
```
| cost_neq_price | cost_neq_prprice | total |
|---|---|---|
| 1,492 (60.1%) | 551 (22.2%) | 2,481 |

### Sample evidence (documents 2119–2122, 20 lines)
```sql
WITH purchase_line_detail_read_model AS ( ... exact reports.go CTE, kind IN ('purchase-return'), eventAggregate 'return' ... )
SELECT ... WHERE document IN ('2119','2120','2121','2122') ORDER BY document, item, batch_number;
```
Representative row (document 2120, party "199"):
```
COMFORT CORSET ALL SIZE | qty 1.0000 | purchase_price 738.1765 | discount_percent 15.00 | amount 915.0000
```
`purchase_price (738.1765) × quantity (1) = 738.1765 ≠ amount (915.0000)` —
visible within the single row, no external reference needed. Raw data:
`unit_price = 915.0000` (= legacy `PRPrice`), `unit_cost = 738.1765`
(average stock cost) — the report shows the latter where it should show the
former.

### Verdict table

| Invoice sample | Lines | purchase_price | sales_tax_value | amount | discount_percent | Verdict |
|---|---|---|---|---|---|---|
| 1530, 1660, 1698, 1882, 1883, 2010, 2089, 2098 | 18 | close (unit_cost ≈ unit_price for these specific lines) | MATCHED | MATCHED | MATCHED (DiscPerc verbatim; discount_value gap same as leaf 3 Finding, not a masking bug) | MATCHED (small rounding-level purchase_price drift only) |
| 2119, 2120, 2121, 2122 | 20 | **MISMATCH-DOCUMENTED** (unit_cost shown instead of PRPrice; COMFORT CORSET is a clean, unambiguous 738.18 vs 915.00) | MATCHED | MATCHED | MATCHED | **MISMATCH-DOCUMENTED** |

**Overall leaf verdict: MISMATCH-DOCUMENTED.** The `sales_tax_value` and
`amount` columns are correct throughout (falling correctly to
`bl.tax_amount` / `bl.line_total` since neither `SalesTax` nor
`Amount`/`LineTotal` legacy keys exist for this document kind).
`purchase_price` is wrong in 551/2,481 lines tenant-wide (22.2%) due to the
`PurPrice` vs `PRPrice` key-name mismatch described above.

---

## Leaf 5 — `p-o-based-purchase-disparity` (`purchaseOrderDisparityReadModelQuery`)

### Methodology
Compares canonical posted `purchase-order` lines against canonical posted
receipt lines (`pack-purchase`/`loose-purchase`/`opening-purchase`) linked via
`source_document_id` or `source_document_number`.

### Data-linkage check (complete, not sampled)
```sql
SELECT kind, count(*) total, count(*) FILTER (WHERE source_document_id IS NOT NULL) with_id,
       count(*) FILTER (WHERE source_document_number <> '') with_number
FROM business_documents WHERE tenant_id=... AND status='posted' GROUP BY kind;
```
Every single posted document in the sandbox tenant — all 8 kinds, including
all 2,810 `purchase-order` and all 6,448 `pack-purchase`/`loose-purchase`/
`opening-purchase` receipts — has `source_document_id IS NULL` and
`source_document_number = ''`. The disparity query's `receipt_lines` CTE
filters on `(r.source_document_id IS NOT NULL OR
NULLIF(r.source_document_number,'') IS NOT NULL)`, so it is **structurally
empty** for this tenant, regardless of how many receipts genuinely correspond
to a given PO. Running the query's own aggregation logic verbatim against
all 2,810 purchase orders (113,812 lines) confirms every single line reports
`received_quantity = 0` (113,812/113,812 = 100%), so disparity always equals
the full ordered quantity for every PO.

**Verdict: VACUOUS-NO-DATA.** The disparity arithmetic itself
(`ordered_quantity - received_quantity`, `ordered_amount - received_amount`,
with the `pack_units` normalization on receipt lines) is internally
consistent and was confirmed to execute and compute correctly given its
inputs, but there is no linked receipt data in this sandbox to exercise the
non-trivial (received > 0) path, so the "is the disparity number actually
right for a real received PO" question cannot be answered this pass. This
also means, as observed rather than merely inferred: **in the current
sandbox data, every PO-based Purchase Disparity report row would show 100%
of every order as unreceived**, because `source_document_id`/
`source_document_number` linkage is never populated on the receiving side —
worth flagging to whichever phase owns the purchase-receiving import, though
it is outside this session's `reports.go`-only scope to fix.

---

## Summary table

| Leaf | Verdict | Headline finding |
|---|---|---|
| sale-detail (Daily Sale Detail) | MATCHED (structure/arithmetic) / MISMATCH-DOCUMENTED (tax column) | `sales_tax_value` always `0.00`; masks real tax in 28,184/620,615 lines (4.54%) tenant-wide. Pack/loose repair invariant confirmed with 0 violations across all 620,615 lines. |
| refused-sales-detail | VACUOUS-NO-DATA | 0 `business_documents` and 0 `sync_events` for this document kind in the sandbox; query verified structurally sound and error-free. |
| purchase-detail | MATCHED (pack/loose) / MISMATCH-DOCUMENTED (opening-purchase Amount) | Amount = 0.00 for 100% of opening-purchase lines (6,430/6,430) despite non-zero legacy price; small residual pack-scaling anomaly in 5 lines. |
| purchase-return-detail | MISMATCH-DOCUMENTED | `purchase_price` shows average stock cost instead of the actual return price in 551/2,481 lines (22.2%) — legacy field is named `PRPrice`, code only checks `PurPrice`. |
| p-o-based-purchase-disparity | VACUOUS-NO-DATA | 0% of purchase orders/receipts have `source_document_id`/`source_document_number` populated; disparity always shows 100% unreceived; query logic itself confirmed structurally sound. |

## Files touched
- Created: `docs/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md` (this file).
- No test file was added (verification was done via direct psql execution of
  the exact reports.go query text; a Go integration test would need a live
  DB fixture matching this sandbox, which is out of scope for this pass given
  the "no long builds" constraint — see Recommendations).
- `services/api/internal/httpapi/reports.go` was read but **not modified**.

## Recommendations (not actioned, per read-only constraint on reports.go)
1. Fix `sales_tax_value` in `dailySalesDetailReadModelQuery` to prefer
   `bl.tax_amount` over a legacy `SalesTax` field that is always `'0.00'` in
   this migration — or at minimum treat a literal `'0.00'` string the same as
   empty in the `NULLIF` check so a real non-zero `tax_amount` can surface.
2. Fix `purchase_price` in `purchaseLineDetailReadModelQuery` to also check
   `legacy_payload->>'PRPrice'` for the return branch (or accept both key
   names in both branches) before falling to `unit_cost`.
3. Investigate why `business_document_lines.unit_price`/`line_total` are
   zero for 100% of `opening-purchase` lines despite a populated legacy
   `PurPrice` (likely an importer gap for the opening-balance stream, not a
   `reports.go` issue).
4. Investigate why `source_document_id`/`source_document_number` are never
   populated on any purchase receipt in this tenant, which makes
   `p-o-based-purchase-disparity` vacuous for all current data.

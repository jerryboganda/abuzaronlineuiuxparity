# Phase Q — Golden Verification: Document/Header/Reprint leaves — 2026-08-09

## Scope and method

This pass golden-verifies 11 report leaves assigned for this run, all backed by
`services/api/internal/httpapi/reports.go` (read-only in this pass; **not
edited**):

1. `quotation-detail`
2. `quotation-summary`
3. `header-wise-transaction-summary`
4. `reprinting-sale`
5. `reprinting-purchase`
6. `reprinting-sale-with-summary-reports`
7. `reprinting-sale-format-2`
8. `reprinting-sale-format-3`
9. `reprinting-sale-format-4`
10. `reprinting-sale-with-header-wise-summaries`
11. `reprinting-selected-sales-and-summaries`

**Live legacy SQL Server was not reachable** (Windows trusted-auth fails in
this environment, confirmed unusable per the assignment's environment
constraint) — no comparison against actual PowerBuilder/legacy output was
possible in this pass. All verification below is **Postgres-internal
cross-verification**: for each leaf, the exact query text used by the handler
(read from `reports.go`, verbatim, not re-derived) was run against the
sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`
(branch `ffffffff-ffff-ffff-ffff-ffffffffffff`, the tenant's only branch),
and its row counts / sums were independently cross-checked against a
differently-constructed query over `business_documents` /
`business_document_lines` / `sync_events`. Every query below was executed
with the required RLS preamble:

```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', '', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```

**Baseline facts about the sandbox tenant**, gathered first and used
throughout:

```sql
SELECT kind, status, count(*) FROM business_documents
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' GROUP BY kind, status ORDER BY kind, status;
```
```
       kind       | status | count
------------------+--------+--------
 cash-sale        | posted | 291359
 cash-sale-return | posted |  30704
 credit-sale      | posted |      2
 loose-purchase   | posted |     22
 opening-purchase | posted |      1
 pack-purchase    | draft  |      1
 pack-purchase    | posted |   6395
 purchase-order   | posted |   2810
 purchase-return  | posted |    634
(9 rows)
```
- Single branch: `ffffffff-ffff-ffff-ffff-ffffffffffff` (331,928 rows / 331,927 posted).
- `sync_events.aggregate` distribution for this tenant is **100%
  `historical_document`** — none of `sale`, `sale_return`, `refused_sale`,
  `receiving`, `return`, `purchase_order`, `quotation`, `inventory` exist as
  event aggregates here. Every "compatibility fallback" UNION branch in every
  query below therefore contributes **zero rows** for this tenant; all
  results come from the canonical `business_documents`/`business_document_lines`
  branch only. This is disclosed per-leaf below and materially simplifies
  (and strengthens) the cross-checks, since there is no dual-source
  de-duplication to account for.
- The `sales_documents` staging table is also **empty** (0 rows) for this
  tenant.
- No `kind = 'quotation'` and no `kind = 'refused-sale'` rows exist in
  `business_documents` for this tenant, at any status.

---

## 1–2. `quotation-detail` / `quotation-summary`

**Code path:** both resolve to `documentReadModel: true`, `documentKind:
"quotation"`, `documentAggregate: "quotation"`, `documentMode: "line-detail"`
/ `"invoice-summary"` (`reports.go` lines 450–473), dispatched through
`documentReadModelQuery("quotation", "quotation", mode, ...)` (lines
1836–1908). The canonical branch filters `bd.kind = 'quotation' AND bd.status
= 'posted'`; the compatibility branch filters `se.aggregate = 'quotation' AND
... = 'posted'`.

**Independent verification query:**
```sql
SELECT
  (SELECT count(*) FROM business_documents WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND kind='quotation') AS bd_quotation_any_status,
  (SELECT count(*) FROM business_documents WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND kind='quotation' AND status='posted') AS bd_quotation_posted,
  (SELECT count(*) FROM sync_events WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND aggregate='quotation') AS se_quotation_events;
```
```
 bd_quotation_any_status | bd_quotation_posted | se_quotation_events
--------------------------+----------------------+---------------------
                        0 |                    0 |                   0
```

Both the canonical and the compatibility source for `quotation` are
completely empty in the sandbox tenant, at every document status. There is
nothing to golden-replay — legacy quotation data was either never migrated
into this tenant, or the tenant has never had a live quotation posted.

**Go-level regression lock added:** `TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind`
in `report_golden_document_reprint_test.go` pins both leaves' specs and
confirms the generated query text scopes to `bd.kind = 'quotation'` /
`se.aggregate = 'quotation'`, so a future accidental rewire is caught even
without sample data.

**Verdict: VACUOUS-NO-DATA** for both `quotation-detail` and
`quotation-summary`. The code path is real and correctly scoped by kind; it
cannot be golden-verified against real content because zero quotation
documents exist in the sandbox tenant under either the canonical or
compatibility source.

---

## 3. `header-wise-transaction-summary`

**Code path:** `headerReadModel: true` (`reports.go` line 474–478),
dispatched to `headerTransactionReadModelQuery()` (lines 1910–1980). Its
`canonical_headers` CTE unions `business_documents` filtered to:

```sql
bd.kind IN (
  'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
  'open-cash-return', 'open-credit-return', 'quotation', 'refused-sale',
  'pack-purchase', 'loose-purchase', 'opening-purchase',
  'purchase-return', 'purchase-order'
)
```
plus a `compatibility_headers` CTE over `sync_events.aggregate IN ('sale',
'sale_return', 'refused_sale', 'receiving', 'return', 'purchase_order',
'quotation', 'inventory')` for events not yet matched to a canonical
document.

### Independent family count (all known kind spellings, both live-app and migrated-historical vocabularies)

```sql
WITH families AS (
  SELECT kind,
    CASE
      WHEN kind IN ('cash-sale','credit-sale') THEN 'sale'
      WHEN kind IN ('cash-return','credit-return','open-cash-return','open-credit-return',
                    'cash-sale-return','credit-sale-return','open-sale-return') THEN 'sale_return'
      WHEN kind = 'quotation' THEN 'quotation'
      WHEN kind = 'refused-sale' THEN 'refused_sale'
      WHEN kind IN ('pack-purchase','loose-purchase','opening-purchase') THEN 'purchase'
      WHEN kind = 'purchase-return' THEN 'purchase_return'
      WHEN kind = 'purchase-order' THEN 'purchase_order'
      ELSE 'OTHER:' || kind
    END AS family
  FROM business_documents
  WHERE tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
    AND branch_id = 'ffffffff-ffff-ffff-ffff-ffffffffffff'
    AND status = 'posted'
)
SELECT family, count(*) FROM families GROUP BY family ORDER BY family;
```
```
     family      | full_family_count
------------------+-------------------
 purchase        |              6418
 purchase_order  |              2810
 purchase_return |               634
 sale            |            291361
 sale_return     |             30704
(5 rows)
```
Sum = 331,927 = exactly `TOTAL-ALL-POSTED` for the tenant. No `OTHER:*`
bucket appeared, so every posted document kind in this tenant is accounted
for by one of the seven legacy families the leaf is supposed to union — good
news for the union's *breadth*, but see below for what the *as-implemented*
kind list actually captures.

### As-implemented kind list, run verbatim against the same data

```sql
SELECT bd.kind, count(*) FROM business_documents bd
WHERE bd.tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND bd.branch_id = 'ffffffff-ffff-ffff-ffff-ffffffffffff'
  AND bd.status = 'posted'
  AND bd.kind IN (
    'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
    'open-cash-return', 'open-credit-return', 'quotation', 'refused-sale',
    'pack-purchase', 'loose-purchase', 'opening-purchase',
    'purchase-return', 'purchase-order'
  )
GROUP BY bd.kind ORDER BY bd.kind;
```
```
       kind       | rows_in_header_query
------------------+----------------------
 cash-sale        |               291359
 credit-sale      |                    2
 loose-purchase   |                   22
 opening-purchase |                    1
 pack-purchase    |                 6395
 purchase-order   |                 2810
 purchase-return  |                  634
(7 rows)
TOTAL: 301223
```

The full `headerTransactionReadModelQuery()` query text (both CTEs, executed
verbatim with a wide-open date filter) returns the **same 301,223 total**,
confirming the compatibility CTE contributes 0 rows here (as expected, since
this tenant has no matching `sync_events.aggregate` values) and that the
`GROUP BY` in `canonical_headers` doesn't fan out (1 row per document).

### Root cause

`business_documents.kind` for the tenant's sale-return documents is
`'cash-sale-return'` — the **migrated-historical spelling** — not
`'cash-return'`. `headerTransactionReadModelQuery`'s `IN`-list only contains
the **live-app spelling** (`'cash-return'`, `'credit-return'`,
`'open-cash-return'`, `'open-credit-return'`). It is missing
`'cash-sale-return'`, `'credit-sale-return'`, and `'open-sale-return'`.

This is a genuine, provable inconsistency within `reports.go` itself: three
other read-model queries in the same file correctly include **both**
vocabularies for the identical purpose:

- `salesReadModelQuery`'s `canonicalReturnUnion` branch (lines 1726–1728,
  1741–1744): `bd.kind IN ('cash-return', 'credit-return', 'open-cash-return',
  'open-credit-return', 'cash-sale-return', 'credit-sale-return',
  'open-sale-return')`
- `saleReturnReadModelQuery` (lines 2772–2774, 2786–2789): same full list.
- The `dailySalesDetailReadModelQuery` / summary variants around lines
  2653–2654, 2687–2688 also carry the full list.

`headerTransactionReadModelQuery` (lines 1931–1936) alone uses only the
short, live-app-only list. Since **100% of this tenant's migrated sale-return
documents use the `'cash-sale-return'` spelling**, the header-wise summary
silently drops all 30,704 of them — with **no compatibility-event fallback**,
because `sync_events` has no `sale_return`-aggregate rows to catch the gap
either.

```text
30,704 dropped / 331,927 total posted documents = 9.25% of the tenant's
entire document volume, and 100% of its sale_return family, missing from
header-wise-transaction-summary with zero indication to the caller.
```

This directly contradicts the leaf's own `projectionNote` in `reports.go`
(line 1438): *"Canonical posted business_documents across sales, **returns**,
quotations, refusals, purchases, purchase returns, and purchase orders are
grouped once per header"* — returns are named as covered but, for this
migrated-data tenant, are not.

**Go-level regression lock added:**
`TestGoldenHeaderWiseTransactionSummaryOmitsMigratedSaleReturnKindSpelling` in
`report_golden_document_reprint_test.go` asserts (a) the current query text
still lacks the three migrated spellings, and (b) the sibling
`saleReturnReadModelQuery` still has them — so this is caught immediately,
and reversed intentionally, whenever `reports.go` is next touched for this
leaf.

**Verdict: MISMATCH-DOCUMENTED.** The union is real and does not
double-count, but it silently **drops** an entire legacy family
(sale-returns) whenever the underlying documents use the migrated-historical
kind spelling, which is the spelling 100% of this tenant's migrated
sale-return data actually uses. Root cause: `reports.go`
`headerTransactionReadModelQuery`, `canonical_headers` CTE, `bd.kind IN (...)`
list (line ~1931) omits `'cash-sale-return'`, `'credit-sale-return'`,
`'open-sale-return'`, unlike three sibling queries in the same file that
correctly include them. Fix (not applied — read-only pass) is a one-line
`IN`-list extension mirroring `saleReturnReadModelQuery`.

---

## 4–11. The 8 `reprinting-*` leaves

**Claim to verify (per assignment):** each reprint leaf should return the
exact same canonical data as its non-reprint counterpart — reprint is a
display-format variant, not a different data scope.

**Static proof first.** `spec.reprintMode` is consulted **only** inside
`reportDefinitionForKey` (lines 1515–1531, for column/label/note metadata)
and is **never read** by the query-dispatch `switch` in the report handler
(lines 5023–5235). The dispatch instead branches purely on
`spec.salesReadModel` / `spec.purchaseReadModel` / `spec.aggregateCondition`
/ `spec.salesMode` / `spec.purchaseMode` — every one of which is set
identically between each reprint leaf and its counterpart in the
registry-construction loops (`reports.go` lines 148–188 for the N-wave sale
leaves, 334–360 for the O-wave purchase leaves, 501–537 for the Q-wave
reprint leaves). That means the reprint leaves are not merely "expected to
match" — they call the **exact same Go function with the exact same
arguments** as their counterpart, so the resulting SQL text is byte-identical
before a single row is fetched. This is now pinned by
`TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart` in
`report_golden_document_reprint_test.go`, which passed:

```
=== RUN   TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart
--- PASS: TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart (0.00s)
```

**Data-level proof second** (per the assignment's ask to cross-check row
counts/totals against real data, not just static code reading):

### 4. `reprinting-sale` (and 7/8/9: `reprinting-sale-format-2/3/4` — identical spec, only `title` differs)

Counterpart: `sale-detail`. Both resolve to `salesReadModel: true, salesMode:
"line-detail", aggregateCondition: reportSaleAggregate` → dispatched to
`dailySalesDetailReadModelQuery`. Run verbatim for the busiest single day
(`2025-08-05`, 766 documents):

```sql
-- as-implemented (dailySalesDetailReadModelQuery body)
 line_rows | distinct_documents | sum_line_amount
-----------+--------------------+-----------------
      1665 |                766 |     618196.7700

-- independent cross-check (direct JOIN over business_documents/business_document_lines,
-- kind IN ('cash-sale','credit-sale'), same date range)
 line_rows | distinct_documents | sum_line_total_direct
-----------+--------------------+-----------------------
      1665 |                766 |           618196.7700
```
Exact match on row count, document count, and summed amount. 0 documents in
range had zero lines (checked separately).

**Verdict (`reprinting-sale`, `reprinting-sale-format-2`, `-format-3`,
`-format-4`): MATCHED.** All four are query-identical to `sale-detail` (same
Go call, same SQL text, verified-matching real data). **Secondary
observation, not a bug:** formats 2/3/4 carry **zero** format-specific
differentiation anywhere in the API layer — not just the data, but the
columns, projection note, and retrieval scope are all identical strings to
plain `reprinting-sale`. That is consistent with layout being a
frontend/print concern out of this API's scope, but it does mean nothing in
the backend distinguishes "Format 2" from "Format 3" from "Format 4" today;
if the legacy formats differ by more than layout (e.g. different column
sets, as several other legacy "Format2" report leaves elsewhere in `reports.go`
do model explicitly), that distinction is unimplemented, not merely
unverified.

### 5. `reprinting-purchase`

Counterpart: `purchase-detail`. Both resolve to `purchaseReadModel: true,
purchaseMode: "line-detail", aggregateCondition: "se.aggregate = 'receiving'"`
→ dispatched to `purchaseLineDetailReadModelQuery`. Run verbatim for the full
tenant date range:

```sql
-- as-implemented
 line_rows | distinct_documents | sum_line_amount
-----------+--------------------+-----------------
    113525 |               6417 | 2702608926.7900

-- independent cross-check (direct JOIN, kind IN ('pack-purchase','loose-purchase','opening-purchase'))
 line_rows | distinct_documents | sum_line_total_direct
-----------+--------------------+-----------------------
    113525 |               6417 |       2702608926.7900
```
Exact match. Note: `distinct_documents` is 6,417, one less than the raw
6,418 `pack/loose/opening-purchase` posted-document count — this is because
one purchase document has zero `business_document_lines` rows and is
excluded by the inner `JOIN` in **both** the as-implemented and the
independent query identically (an `INNER JOIN`, not a `LEFT JOIN`, in
`purchaseLineDetailReadModelQuery`). This is a pre-existing data-completeness
wrinkle in the shared purchase-line-detail model itself (affects
`purchase-detail` too, not something reprint introduces), out of this
leaf-pair's scope to fix but worth flagging for whichever agent owns
`purchase-detail`/`O`-wave purchase leaves.

**Verdict: MATCHED.** `reprinting-purchase` returns the exact same canonical
purchase-line data as `purchase-detail`.

### 6, 10, 11. `reprinting-sale-with-summary-reports`, `reprinting-sale-with-header-wise-summaries`, `reprinting-selected-sales-and-summaries` — identical spec, only `title` differs

Counterpart: `sale-summary`. All three resolve to `salesReadModel: true,
salesMode: "invoice-summary", aggregateCondition: reportSaleAggregate` →
dispatched to `invoiceSummaryReadModelQuery(salesReadModelQuery(...))`. Run
verbatim for the full tenant date range:

```sql
-- as-implemented (GROUP BY document, SUM(quantity), MAX(amount) i.e. document total taken once)
 invoice_summary_rows | sum_invoice_amount | sum_invoice_qty
----------------------+--------------------+-----------------
               291361 |     234003081.0000 |    4917709.0000

-- independent cross-check (direct GROUP BY over business_documents, no line-detail join at all)
 distinct_docs | sum_total_amount |     sum_qty
---------------+------------------+------------------
        291361 |   234003081.0000 | 4917709.00000000
```
Exact match on invoice count, summed document total, and summed quantity
(computed independently via a `business_document_lines` sub-aggregate).

**Verdict (data scope): MATCHED** for all three. Each is query-identical to
`sale-summary` and returns identical real data.

**Two naming-vs-implementation gaps found, distinct from the data-scope
question above (documented, not fixed):**

- **`reprinting-sale-with-header-wise-summaries`**: the name implies grouping
  by transaction header (the same "header-wise" concept as leaf #3 above,
  i.e. one row per document with a transaction-type dimension), but the code
  wires it to plain per-invoice `invoice-summary` mode — the **same**
  projection as ungrouped `sale-summary`/`reprinting-sale-with-summary-reports`.
  `headerReadModel`/`headerTransactionReadModelQuery` is never invoked for
  this leaf. If the legacy report's "header wise" framing meant an actual
  distinct grouping (as its name and leaf #3's existence both suggest), that
  grouping is unimplemented here, not merely unverified.
- **`reprinting-selected-sales-and-summaries`**: the name implies the user
  selects a specific subset of invoices to reprint (a legacy multi-select
  dialog), but grep across `services/api/internal/httpapi` found **no**
  `invoiceIds`/`selectedIds`/selection-filter parameter anywhere tied to this
  leaf (or its sibling `selected-sales-and-summaries-report` in the N-wave
  registry). The API returns the full date-range invoice-summary set,
  identical to bulk `sale-summary`. There is currently no way to reprint a
  user-selected subset through this leaf; "selected" is asserted by the title
  only.

---

## Summary table

| Leaf | Verdict | One-line reason |
|---|---|---|
| `quotation-detail` | VACUOUS-NO-DATA | 0 canonical + 0 compatibility quotation rows in sandbox tenant |
| `quotation-summary` | VACUOUS-NO-DATA | same |
| `header-wise-transaction-summary` | **MISMATCH-DOCUMENTED** | canonical kind `IN`-list omits migrated `'cash-sale-return'`/`'credit-sale-return'`/`'open-sale-return'` spellings; drops 30,704/331,927 (9.25%) posted documents, 100% of the sale_return family, with zero compatibility fallback |
| `reprinting-sale` | MATCHED | query-identical to `sale-detail`; verified 1,665 lines / 766 docs / Rs 618,196.77 for 2025-08-05 |
| `reprinting-purchase` | MATCHED | query-identical to `purchase-detail`; verified 113,525 lines / 6,417 docs / Rs 2,702,608,926.79 full range |
| `reprinting-sale-with-summary-reports` | MATCHED | query-identical to `sale-summary`; verified 291,361 invoices / Rs 234,003,081.00 / 4,917,709 qty full range |
| `reprinting-sale-format-2` | MATCHED | query-identical to `reprinting-sale`/`sale-detail`; no format-specific differentiation exists in the API (informational) |
| `reprinting-sale-format-3` | MATCHED | same as format-2 |
| `reprinting-sale-format-4` | MATCHED | same as format-2 |
| `reprinting-sale-with-header-wise-summaries` | MATCHED (data scope); naming gap documented | data identical to `sale-summary`; "header wise" grouping implied by name is not implemented — uses plain invoice-summary, not `headerReadModel` |
| `reprinting-selected-sales-and-summaries` | MATCHED (data scope); naming gap documented | data identical to `sale-summary`; no invoice-selection parameter exists anywhere in the API |

## Tests added

`services/api/internal/httpapi/report_golden_document_reprint_test.go` (new
file, additive only — `reports.go` was not modified):

- `TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart` —
  locks the query-text identity between all 8 reprint leaves and their
  non-reprint counterparts.
- `TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind` — locks the
  quotation leaves' kind scoping so the VACUOUS-NO-DATA finding stays
  detectable even without sample data.
- `TestGoldenHeaderWiseTransactionSummaryOmitsMigratedSaleReturnKindSpelling`
  — characterization test that asserts the *current* (buggy) omission, so a
  future fix is a deliberate, visible test change instead of a silent
  regression rediscovered from scratch.

```text
go vet ./services/api/...                                                    # clean, no output
go test -run 'TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart|TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind|TestGoldenHeaderWiseTransactionSummaryOmitsMigratedSaleReturnKindSpelling' ./services/api/internal/httpapi/ -v -count=1
=== RUN   TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart
--- PASS: TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart (0.00s)
=== RUN   TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind
--- PASS: TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind (0.00s)
=== RUN   TestGoldenHeaderWiseTransactionSummaryOmitsMigratedSaleReturnKindSpelling
--- PASS: TestGoldenHeaderWiseTransactionSummaryOmitsMigratedSaleReturnKindSpelling (0.00s)
PASS
ok      github.com/abuzar/abuzar-next/services/api/internal/httpapi   0.103s
```
`go build ./...` from `services/api` also confirmed clean (no compile
regressions from the additive test file).

## What remains open (not claimed by this pass)

- No comparison against actual legacy PowerBuilder print output or dialogs —
  live SQL Server was unreachable this pass, per the confirmed environment
  constraint. All 11 verdicts above are Postgres-internal, not
  legacy-replayed.
- Exact PowerBuilder patient/pack/header sections, selected-invoice dialogs,
  and print/PDF/workbook byte output remain unverified for all 8 reprint
  leaves, consistent with `docs/PHASE_Q_REPRINT_EVIDENCE_2026-08-07.md`.
- The `header-wise-transaction-summary` mismatch is documented, not fixed —
  `reports.go` is read-only for this pass. The fix is a one-line `IN`-list
  extension in `headerTransactionReadModelQuery` mirroring
  `saleReturnReadModelQuery`'s existing kind list.
- Quotation leaves could not be golden-verified against real content because
  none exists in the sandbox tenant; if another tenant with migrated
  quotation data becomes available, this pass's VACUOUS-NO-DATA verdict
  should be revisited there.

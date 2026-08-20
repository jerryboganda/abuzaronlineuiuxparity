# Phase N golden verification — Category Wise sub-group A — 2026-08-09

Scope: 5 assigned leaves from the `Category Wise` sub-menu of Sales Reports
(`services/api/internal/httpapi/reports.go` lines 221-319,
`phaseNReportRegistry`):

1. `category-wise-sale-and-return`
2. `category-wise-sales`
3. `category-wise-deviated-items`
4. `category-wise-monthly-sale`
5. `category-wise-net-sale`

This is a **diagnostic** pass, not a promotion (this agent does not, and did
not, edit `reports.go`). Its purpose is to (a) confirm current registry
status, (b) re-test the blocker the wave-1 promotion agent cited for
`category-wise-sales` in `docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`
("no legacy capture to resolve column-shape ambiguity" +
"`master_items.payload` has 0/30,052 rows with a human-readable Category
name, only opaque `ICatCode`"), and (c) hand the promotion agent an exact,
DB-verified query/JOIN shape wherever a genuine unblock exists.

## Headline finding

**The `ICatCode`-has-no-name half of the wave-1 blocker is resolved.** A
populated, tenant-scoped lookup table — `master_categories` with
`category_kind = 'item_category'` — exists and its `legacy_id`/`code` values
match `master_items.payload->>'ICatCode'` **exactly, for 100% of items**
(30,052/30,052 resolved, 0 unresolved) in the sandbox tenant
(`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`). Joining
`business_document_lines.item_id → master_items.id → (ICatCode) →
master_categories.legacy_id` (kind `item_category`) turns the opaque code
into a real category name (`MEDICINES`, `NARCOTICS`, `CONSUMER`,
`COUNSELING`, `DIAGNOSTIC`, `MILK`, `DR KHALID MUGHAL`) with zero orphaned
rows. This is a **different, and better-resolved, lookup** than the
`customer_category` case already shipped and verified as "VACUOUS" in
`docs/evidence/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md` (leaves 9-12): that
case reads `master_parties.payload->>'Category'` directly and finds it
unpopulated (0 of 2 customers), falling back to a single `'Unspecified'`
bucket. The item-category path below does **not** depend on
`master_items.payload->>'Category'` (still 0/30,052, confirmed) — it uses
`master_categories` as a proper code-to-name lookup table instead, which the
wave-1 agent did not check before declining.

This does **not** by itself resolve the *other* half of the wave-1 blocker
for `category-wise-sales` — the report-shape ambiguity (flat category-total
summary vs. category-grouped line listing) — since no legacy capture exists
either way. See leaf 2 below for the concrete recommendation and its
residual ambiguity.

### Verified query/JOIN shape (the reusable part)

Canonical rows (business_documents / business_document_lines path — covers
100% of this tenant's posted sale documents; 0 compatibility-only documents
found, see verification below):

```sql
SELECT COALESCE(mc.name, 'UNCATEGORIZED') AS category,
       SUM(bl.quantity)    AS quantity,
       SUM(bl.line_total)  AS amount
FROM business_documents bd
JOIN business_document_lines bl
  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
 AND bl.document_id = bd.id
LEFT JOIN master_items mi
  ON mi.tenant_id = bl.tenant_id AND mi.id = bl.item_id
LEFT JOIN master_categories mc
  ON mc.tenant_id = mi.tenant_id AND mc.category_kind = 'item_category'
 AND mc.legacy_id = mi.payload->>'ICatCode'
WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
  AND bd.kind IN ('cash-sale', 'credit-sale')   -- add return kinds for reportSaleOrReturn condition
  AND bd.status = 'posted'
  AND bd.occurred_at >= $3::date AND bd.occurred_at < ($4::date + INTERVAL '1 day')
GROUP BY mc.name
ORDER BY mc.name
```

For the compatibility (sync_events/sales_documents) rows that the existing
`salesReadModelQuery` unions in, the same lookup applies via the text path
already used by `bl.item_legacy_id`: `master_items.legacy_id =
item_legacy_id`, then the same `master_categories` join on
`payload->>'ICatCode'`. Verified 100% resolution for this join too
(895,015/895,015 `business_document_lines` rows resolve `item_legacy_id →
master_items.legacy_id` with 0 unresolved) — the text-keyed fallback is just
as safe as the `item_id` FK path.

This is structurally the same shape as the already-shipped
`salesCustomerCategorySummaryReadModelQueryMode` (`reports.go:2632`,
`salesMode = "customer-category-summary"`) — same `UNION ALL` of canonical +
compatibility rows, same `GROUP BY category` — except sourcing the category
name from `master_categories` (a real lookup table) instead of
`master_parties.payload->>'Category'` (an unpopulated field). A new
`salesMode` (e.g. `"item-category-summary"`) implemented as
`salesItemCategorySummaryReadModelQuery`, following that existing function's
skeleton, is a minimal, low-risk addition — no new tables, no new migration,
reuses the existing `UNION ALL` dedup pattern.

### DB verification (sandbox tenant, branch `ffffffff-ffff-ffff-ffff-ffffffffffff`, 2026-07-01)

```
   category     | line_count | total_amount
------------------+------------+--------------
 MEDICINES        |        765 |  443300.0600
 DR KHALID MUGHAL |         28 |   22266.1100
 CONSUMER         |         24 |    6451.6900
 NARCOTICS        |         30 |    3366.2900
 MILK             |          1 |    1600.0000
 COUNSELING       |          1 |     150.0000
```
(DIAGNOSTIC had no lines that day — 6 of 7 categories represented, which is
plausible for a single day's sales.) All `business_document_lines.item_id`
values resolved to a `master_items` row (0 orphans), and all resolved
items' `ICatCode` resolved to a `master_categories` name (0 orphans).

## Per-leaf findings

### 1. `category-wise-sale-and-return`

- **Current status**: `phaseNReportRegistry["category-wise-sale-and-return"]`
  (reports.go:246) — title contains "Return" so
  `aggregateCondition = reportSaleOrReturn`; falls into the `switch`
  default (no `case` matches this kind at lines 285-317), so
  `salesMode = ""`. Dispatch (`reports.go:1401-1408`) leaves
  `columns = reportEventLedgerColumns()` (generic 6-column grid) and
  `projectionStatus = "event-ledger"`.
- **Diagnostic finding**: the item-category lookup unblock above applies
  directly. `reportSaleOrReturn` already covers both sale and sale-return
  `se.aggregate`/`bd.kind` values in the existing base query
  (`salesReadModelQuery`, lines 1720-1749), so a category+return-aware
  variant is a straightforward extension of the recommended query, adding
  the return `bd.kind` values (`cash-return`, `credit-return`,
  `open-cash-return`, `open-credit-return`, `cash-sale-return`,
  `credit-sale-return`, `open-sale-return`) to the canonical branch, exactly
  as `salesReadModelQuery`'s existing `canonicalReturnUnion` does.
  Whether a legacy "Sale And Return" category report signs returns negative
  (net) or lists them as a separate row/column per category is a shape
  question with no legacy capture found (see leaf 2's ambiguity note — same
  problem applies here).
- **Recommended query/JOIN**: as in "Verified query/JOIN shape" above, with
  `bd.kind IN ('cash-sale','credit-sale','cash-return','credit-return',
  'open-cash-return','open-credit-return')` (mirroring
  `salesCustomerCategorySummaryReadModelQueryMode`'s `canonicalKinds` for
  `reportSaleOrReturn`, reports.go:2634-2638) and a `SUM` sign convention
  decision (net vs. two columns) still needing a legacy capture to resolve.
- **Event-ledger fallback sanity**: ran the exact current fallback SQL
  (`salesReadModelQuery(reportSaleOrReturn, ...)` body) against the sandbox
  tenant for 2026-07-01 — returns rows with no SQL errors (5 sample rows
  returned, canonical branch only; 0 compatibility-only documents in this
  tenant so the return-events UNION branch is currently inert but valid).
- **Verdict**: item-category lookup blocker resolved; still blocked on
  sale/return sign convention (no capture). Not safely promotable without
  that decision, but the promotion agent has everything needed for the
  category dimension.

### 2. `category-wise-sales`

- **Current status**: `phaseNReportRegistry["category-wise-sales"]`
  (reports.go:247) — title "Sales" only, `aggregateCondition =
  reportSaleAggregate`; no `case` match, `salesMode = ""`. Same generic
  6-column event-ledger grid as leaf 1.
- **Diagnostic finding**: this is the leaf the wave-1 promotion agent
  explicitly declined, citing (1) shape ambiguity and (2) "`master_items.
  payload` has 0/30,052 rows with a populated Category name — only the raw
  `ICatCode`... unless a new join to `master_categories` is added. That's
  new multi-table infrastructure, not a minimal reuse." **Re-checked
  directly against the sandbox tenant**: `master_categories` exists, is
  populated (7 active `item_category` rows: MEDICINES, NARCOTICS, CONSUMER,
  COUNSELING, DIAGNOSTIC, MILK, DR KHALID MUGHAL), and its `legacy_id`
  values resolve 100% of `master_items.payload->>'ICatCode'` values (0
  unresolved out of 30,052). So the *data* half of the original objection no
  longer holds — a real, non-opaque category name is available via one
  `LEFT JOIN`. The *shape* half of the objection (flat category-total
  summary vs. category-grouped line listing — no legacy capture in
  `parity/captures/legacy/` or `docs/` either way) is unchanged and still
  unresolved by this pass; re-confirmed no capture exists via search.
- **Recommended query/JOIN**: the "Verified query/JOIN shape" query above,
  unmodified (`reportSaleAggregate`, sale kinds only) — this is the
  most direct candidate for a new `salesMode = "item-category-summary"`,
  implemented as `salesItemCategorySummaryReadModelQuery` mirroring
  `salesCustomerCategorySummaryReadModelQueryMode`'s structure. If the
  promotion agent is comfortable inferring "Sales" (bare title, sibling of
  `Gross Profit`/`Net Sale`/`Monthly Sale`) means the flat category-total
  summary — matching the pattern of the already-shipped, already-tested
  `customer-category-summary` sibling for the analogous customer dimension —
  this query is ready to wire in with high confidence on the data side and
  medium confidence on the shape side (same class of shape-inference
  precedent as the already-accepted `customer-sales-detail`/
  `customer-sales-summary` promotions).
- **Event-ledger fallback sanity**: ran the exact current fallback SQL for
  `reportSaleAggregate` against the sandbox tenant for 2026-07-01 — 5 sample
  rows returned, no errors.
- **Verdict**: genuine unblock path found and verified end-to-end against
  live data. This is the strongest candidate of the 5 assigned leaves and
  the one most worth the promotion agent's attention — see "Headline
  finding" above.

### 3. `category-wise-deviated-items`

- **Current status**: `phaseNReportRegistry["category-wise-deviated-items"]`
  (reports.go:248) — `aggregateCondition = reportSaleAggregate`; no `case`
  match, `salesMode = ""`. Generic event-ledger grid.
- **Diagnostic finding**: the `master_categories` join resolves the category
  *name* problem, but "Deviated Items" has a **separate, unrelated**
  semantic blocker the category fix does not touch: what "deviation" means
  (price deviation from a standard/list price, quantity deviation from an
  expected reorder level, a variance-from-average calculation, etc.) is not
  defined anywhere in this repo. Searched `docs/`, `parity/catalog/
  legacy-menu-tree-*.json`, and `parity/catalog/sqlserver-schema.json` for
  "deviat" — the only hits are the menu-tree path entry itself (confirming
  the leaf exists in the legacy menu) and unrelated ECG/medical-device
  schema columns (`LeftAxisDeviation` etc., a different module entirely). No
  threshold, formula, or reference value is captured anywhere.
- **Recommended query/JOIN**: none — this leaf needs a legacy capture or
  schema comment defining "deviation" before any query can be written
  without guessing a business rule, independent of the category-name fix.
- **Event-ledger fallback sanity**: ran the exact current fallback SQL for
  `reportSaleAggregate` against the sandbox tenant for 2026-07-01 — no
  errors (same base query as leaf 2, confirmed above).
- **Verdict**: still blocked, but for a different reason than originally
  suspected — not a category-name problem, a "deviation" business-rule
  definition problem. The category join does not help here.

### 4. `category-wise-monthly-sale`

- **Current status**: `phaseNReportRegistry["category-wise-monthly-sale"]`
  (reports.go:249) — `aggregateCondition = reportSaleAggregate`; no `case`
  match, `salesMode = ""`. Generic event-ledger grid.
- **Diagnostic finding**: this is a two-dimension grouping (category ×
  calendar month), directly analogous to the already-shipped
  `salesCalendarSummaryReadModelQuery` (`reports.go:2207`, used for
  `month-summary`/`day-summary`/`hour-summary`) crossed with the new
  category lookup. `salesCalendarSummaryReadModelQuery` already shows the
  pattern for bucketing `occurred_at` by `date_trunc('month', ...)` on top
  of a base invoice/line CTE — the same technique applies by swapping the
  base CTE's `party` grouping dimension for the `master_categories` name.
- **Recommended query/JOIN**: extend the "Verified query/JOIN shape" query
  with `date_trunc('month', bd.occurred_at::date)::date AS period` added to
  the `SELECT`/`GROUP BY`/`ORDER BY`, i.e. `GROUP BY period, mc.name`. This
  is new (category × month) infrastructure — not a byte-identical reuse of
  an existing mode like leaf 2 could be — so it carries more implementation
  risk than leaf 2, but the ingredients (category lookup + month bucketing)
  are both independently already verified/precedented in this codebase.
- **Event-ledger fallback sanity**: ran the exact current fallback SQL for
  `reportSaleAggregate` against the sandbox tenant for 2026-07-01 — no
  errors (same base query as leaf 2).
- **Verdict**: category-name blocker resolved; a real per-category-per-month
  query is buildable by composing two already-precedented patterns
  (category lookup + calendar bucketing), but is new composition, not a
  drop-in reuse — flagged for the promotion agent to weigh against its
  minimal-reuse bar.

### 5. `category-wise-net-sale`

- **Current status**: `phaseNReportRegistry["category-wise-net-sale"]`
  (reports.go:250) — `aggregateCondition = reportSaleAggregate` (title
  "Net Sale" does not contain "return", so this does NOT get
  `reportSaleOrReturn` even though "net" commonly implies sale-minus-return
  in this codebase's other "net sale(s)" leaves — worth the promotion
  agent double-checking against
  `customer-sales-customer-category-wise-net-sales`, which explicitly does
  use the `customer-category-summary` mode over a `reportSaleAggregate`-only
  condition too, i.e. that sibling's "Net Sales" also does not pull in
  returns despite the name). No `case` match, `salesMode = ""`. Generic
  event-ledger grid.
- **Diagnostic finding**: same category-name unblock as leaf 2 applies
  directly. Given the sibling precedent
  (`customer-sales-customer-category-wise-net-sales` →
  `customer-category-summary`, already shipped and verified — see
  `docs/evidence/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md` leaves 9/10), the
  natural analog here is an `item-category-summary` mode identical in shape
  to leaf 2's recommendation — `category-wise-net-sale` and
  `category-wise-sales` would then be indistinguishable in query shape,
  which itself is a signal worth the promotion agent weighing (are these
  really two different report shapes, or does "Net Sale" imply a
  return-netting behavior that "Sales" does not, in which case
  `category-wise-net-sale` should use the `reportSaleOrReturn` condition and
  leaf 1/4's return-signing decision applies here too)?
- **Recommended query/JOIN**: the "Verified query/JOIN shape" query above,
  same as leaf 2, if "Net Sale" simply means the flat category-total (no
  return netting, matching the `reportSaleAggregate` condition already
  assigned in the registry). If the promotion agent instead concludes "Net"
  implies return-netting, this leaf should be re-classified to
  `reportSaleOrReturn` first (a `reports.go` change beyond this leaf's
  current condition, which this diagnostic pass does not have authority or
  evidence to make unilaterally).
- **Event-ledger fallback sanity**: ran the exact current fallback SQL for
  `reportSaleAggregate` against the sandbox tenant for 2026-07-01 — no
  errors (same base query as leaf 2).
- **Verdict**: category-name blocker resolved; query shape is ready
  assuming the existing `reportSaleAggregate` condition is correct for "Net
  Sale" (i.e., not return-netting) — flagged the naming/condition tension
  above for the promotion agent to resolve, not decided unilaterally here.

## Net effect / summary for the dispatcher

| Leaf | Category-name blocker | Other blocker | Query/JOIN ready? |
|---|---|---|---|
| `category-wise-sale-and-return` | Resolved | Return sign convention undecided (no capture) | Partial — category join ready, return-sign shape undecided |
| `category-wise-sales` | Resolved | Flat-vs-detail shape ambiguity (no capture) unchanged | **Yes** — strongest candidate |
| `category-wise-deviated-items` | Resolved (irrelevant here) | "Deviation" business rule undefined anywhere in repo | No |
| `category-wise-monthly-sale` | Resolved | None found; composition of 2 precedented patterns, not byte-identical reuse | Yes (higher implementation effort than leaf 2) |
| `category-wise-net-sale` | Resolved | "Net" vs. `reportSaleAggregate`-only condition tension worth resolving first | Yes, pending that decision |

**Flag for the dispatcher / other agents**: the `master_categories` table
(`category_kind = 'item_category'`, 7 populated rows, 100% resolution of
`master_items.payload->>'ICatCode'`) is a genuine, verified, previously
unused-in-this-context lookup. Any other report leaf blocked by "no
human-readable category name, only opaque `ICatCode`" — this includes
`category-wise-gross-profit`, `category-wise-item-wise-sale-discounts-detail`,
`category-wise-item-category-wise-monthly-sales`,
`category-wise-category-wise-day-net-sale` (assigned to other agents per the
registry loop at reports.go:221-277, not diagnosed in depth here since they
are outside this agent's assignment) — can very likely reuse the exact same
`master_items.id → payload->>'ICatCode' → master_categories.legacy_id (kind
'item_category')` join verified above. This does not resolve report-shape
ambiguity for those leaves, only the category-name half of any such
blocker.

`master_manufacturers` was mentioned in the wave-1 promotion doc as a
similar real code/name table not yet joined for `manufacturer-wise-*`
leaves — not re-verified by this agent (out of scope), but the same
category_kind-lookup-table pattern found here suggests it is worth the
manufacturer-assigned agent checking whether `master_manufacturers` has an
equivalent `legacy_id`/`code` column that resolves `master_items.payload->>
'ManfCode'` the same way `master_categories` resolves `ICatCode`.

## Verification

- `go vet ./services/api/...` — clean (0 output).
- `go test -run 'TestPhaseNReportRegistryDefinitionsAndAggregateFilters'
  ./services/api/internal/httpapi/...` — PASS (confirms all 5 assigned
  leaves still satisfy the registry-wide `"event-ledger"` invariant today,
  unaffected by this diagnostic pass since no `reports.go` edits were made).
  One transient build failure was hit on the first attempt
  (`report_golden_n_manufacturer_test.go:4:2: "strings" imported and not
  used`) caused by a different, concurrently-running parallel agent
  mid-editing that unrelated file; resolved on retry a few seconds later,
  consistent with the same transient-collision pattern already documented
  in `docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`.
- All SQL in this document was run live against
  `postgres://postgres@127.0.0.1:5432/abuzar_next`, tenant
  `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch
  `ffffffff-ffff-ffff-ffff-ffffffffffff`, with the supplied RLS preamble
  (`app.tenant_id`, `app.branch_id`, `app.allow_tenant_scope`,
  `app.authenticating`).
- No `reports.go` edits were made (read-only per task rules). No new test
  file was added — this pass produced no code change to test; the only
  deliverable is this document.

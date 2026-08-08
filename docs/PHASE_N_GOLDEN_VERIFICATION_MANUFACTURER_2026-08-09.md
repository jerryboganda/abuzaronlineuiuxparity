# Phase N Golden Verification — Manufacturer Wise (Sales Reports) leaves (2026-08-09)

Scope: 6 assigned Phase N "Manufacturer Wise" report leaves, all currently
`event-ledger` (`services/api/internal/httpapi/reports.go` lines 221-317,
`phaseNReportRegistry`). `reports.go` was **not** edited (per the
concurrent-agent rule); this pass is read-only plus the two files listed
below.

**Files added by this pass:**
- `docs/PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md` (this file)
- `services/api/internal/httpapi/report_golden_n_manufacturer_test.go`
  (offline, no-DB registry/query-shape lock tests, pattern-matched to
  `report_golden_o_purchase_test.go`)

## Headline finding — a fully-populated manufacturer join path exists and is unused by any of these 6 leaves

**This confirms and strengthens the O-phase finding** (`docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md`,
"Grouping-dimension gaps" section) that `manufacturer-wise-detail` and
`manufacturer-wise-monthly-stock-movement` have "no manufacturer join/grouping
at all despite the name." The same root cause blocks all 6 of my assigned
N-phase leaves, and I independently re-verified the join at full sales-line
volume (620,615 lines, not just the 30,052-item master table O-phase
checked):

```sql
SELECT count(*) AS lines_total,
       count(*) FILTER (WHERE mf.id IS NOT NULL) AS lines_resolved_manufacturer
FROM business_document_lines l
JOIN business_documents d ON d.tenant_id = l.tenant_id AND d.id = l.document_id
LEFT JOIN master_items i ON i.tenant_id = l.tenant_id AND i.id = l.item_id
LEFT JOIN master_manufacturers mf ON mf.tenant_id = i.tenant_id AND mf.code = i.payload->>'ManfCode'
WHERE l.tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND d.kind IN ('cash-sale','credit-sale') AND d.status='posted';
```
Result: **620,615 / 620,615 (100%) of posted sale lines resolve to a real
manufacturer name.** Sample:

| document | item | manufacturer code | manufacturer name |
|---|---|---|---|
| 593509 | TOBREX EYE OINT | 18 | ALCON UNIVERSAL LTD.U.S.A. |
| 804027 | PROVITA 2 MILK 200GM | 993 | GENERAL PRODUCT |
| 740188 | LINGAB 75MG CAP | 825 | GENETICS PHARMACEUTICALS |

Underlying facts, independently confirmed:
- `master_manufacturers` has **838 rows** for the sandbox tenant, each with a
  populated `code`, `legacy_id` (identical values in this tenant), and `name`
  (e.g. code `62` → `"BRISTOL-MYERS"`, code `18` → `"ALCON UNIVERSAL
  LTD.U.S.A."`). Schema: `id, tenant_id, legacy_id, code, name, payload,
  active, created_at, updated_at`, unique on `(tenant_id, code)` and
  `(tenant_id, legacy_id)`.
- `master_items.payload->>'ManfCode'` is populated for **30,052/30,052
  (100%)** items. `master_items.payload->>'Manufacturer'` (the name key) is
  **0/30,052** populated — this is why every existing consumer of this data
  (see below) falls back to displaying the raw numeric code instead of a
  name.
- Joining `master_items.payload->>'ManfCode'` to `master_manufacturers.code`
  (or equivalently `.legacy_id` — identical result set, 30,052/30,052 items
  matched either way) is a clean, single-column equi-join with no fuzziness.
- **322 distinct manufacturers** actually appear across the tenant's posted
  sale lines (of the 838 total), a plausible grouping cardinality for a real
  manufacturer-wise report.

This exact class of unused-but-available join already exists partially
elsewhere in the codebase: `stockSupplierManufacturerReadModelQuery` and
`stockClassificationReadModelQuery` (mode `"manufacturer-balance"`, both
Phase P, `reports.go` lines ~3606-3913) already select
`COALESCE(NULLIF(i.payload->>'Manufacturer',''), NULLIF(i.payload->>'ManfCode',''), ..., 'Unspecified')`
— i.e. they already reach for the manufacturer dimension, but because
`payload->>'Manufacturer'` is never populated, they always fall through to
displaying the bare `ManfCode` **code** (e.g. `"62"`), never the resolved
**name** (`"BRISTOL-MYERS"`), because none of them join to
`master_manufacturers`. **No report in `reports.go` — Phase N, O, or P —
currently joins to `master_manufacturers` at all**; `canonical.go` only uses
that table for master-data CRUD (`canonicalMasterSpecFor("manufacturer")`).

**What a fix would look like** (for the next promotion pass, not done here
per the read-only constraint): add
`LEFT JOIN master_manufacturers mf ON mf.tenant_id = i.tenant_id AND mf.code = i.payload->>'ManfCode'`
alongside the existing `master_items i` join, and select
`COALESCE(mf.name, i.payload->>'ManfCode', 'Unspecified')` as the manufacturer
column/grouping key. This would fix all 6 of my N-phase leaves' missing
manufacturer dimension **and** the two already-documented O-phase mismatches
(`manufacturer-wise-detail`, `manufacturer-wise-monthly-stock-movement`) **and**
upgrade the P-phase `stock-in-hand-manufacturer-wise`/`-format2`/
`supplier-manufacturer-association` leaves from showing raw codes to real
names, using the exact same join pattern in all cases.

## Ground truth for the 6 assigned leaves

All 6 are wired identically: `phaseNReportRegistry` entries with
`salesReadModel = true` and **no case in the mode-selection `switch`**
(`reports.go` lines 284-317), so `salesMode` stays `""` for every one of
them. Confirmed via `reportSpecForKey`:

| # | Leaf kind | Title | `aggregateCondition` | `salesMode` | `ProjectionStatus` |
|---|---|---|---|---|---|
| 1 | `manufacturer-wise-sales` | Sales | `se.aggregate = 'sale'` | `""` | `event-ledger` |
| 2 | `manufacturer-wise-sales-detail-and-summary` | Sales Detail And Summary | `se.aggregate = 'sale'` | `""` | `event-ledger` |
| 3 | `manufacturer-wise-net-sales` | Net Sales | `se.aggregate = 'sale'` | `""` | `event-ledger` |
| 4 | `manufacturer-wise-item-sales-discount` | Item Sales/Discount | `se.aggregate = 'sale'` | `""` | `event-ledger` |
| 5 | `manufacturer-wise-manufacturer-wise-sales-and-return-summary` | Manufacturer Wise Sales And Return Summary | `se.aggregate IN ('sale', 'sale_return')` (title contains "return") | `""` | `event-ledger` |
| 6 | `manufacturer-wise-cnic-ntn-registered-customers-sales` | CNIC/NTN Registered Customers Sales | `se.aggregate = 'sale'` | `""` | `event-ledger` |

With `salesMode == ""`, `salesReadModelQueryMode` falls through every mode
branch (`line-detail`, `invoice-summary`, `day/month/hour-summary`,
`item-summary`, `profit-margin-detail`, `profit-day-summary`,
`profit-customer-summary`, `customer-summary`, `customer-category-summary`,
`customer-wise-category-summary`) and returns the plain
`salesReadModelQuery(aggregateCondition, pagination)` output — the generic
6-column projection (`document, occurred_at, party, item, quantity, amount`).
Confirmed by the new test
`TestPhaseNGoldenManufacturerModeDispatchesToPlainGenericQuery`, which asserts
`salesReadModelQueryMode(condition, "", pagination)` is byte-identical to
`salesReadModelQuery(condition, pagination)` and contains none of
`master_manufacturers`, `ManfCode`, `Manufacturer`. `reportDefinitionFor`
confirms the resulting `Columns` match `reportEventLedgerColumns()` exactly
(document/occurredAt/party/item/quantity/amount, 6 columns) for all 6 leaves —
locked by `TestPhaseNGoldenManufacturerLeavesReportEventLedgerProjectionStatus`.

## Per-leaf detail

### 1. `manufacturer-wise-sales`
**Status:** event-ledger, correctly scoped to `se.aggregate = 'sale'` (posted
`cash-sale`/`credit-sale` documents). Arithmetic itself is sound — it's the
same base `salesReadModelQuery` already independently verified for its
sibling `sale-detail`/`sale-summary` leaves in prior waves.
**Diagnostic finding:** No manufacturer grouping or column exists at all;
output is a flat 6-column document/item ledger, not a manufacturer breakdown.
Same declined-candidate reasoning already recorded by the prior N-phase
promotion pass (`docs/PHASE_N_REPORT_PROMOTION_2026-08-09.md`: "Same title
ambiguity as `category-wise-sales`... `master_items.payload->>'Manufacturer'`
is 0/30,052 populated") — but that doc only checked the item master, not
whether a usable join target existed. **It does** (see headline finding
above): `master_manufacturers` (838 rows, fully populated `code`/`name`) is
ready to join and resolves 100% of sale lines.
**Recommended query/JOIN (for a future promotion pass, not applied here):**
```sql
LEFT JOIN master_items i ON i.tenant_id = bl.tenant_id AND i.id = bl.item_id
LEFT JOIN master_manufacturers mf ON mf.tenant_id = i.tenant_id AND mf.code = i.payload->>'ManfCode'
```
grouping/selecting `COALESCE(mf.name, i.payload->>'ManfCode', 'Unspecified')`
as a `manufacturer` column, most likely with a new `salesMode` such as
`"manufacturer-summary"` grouping by `(manufacturer, occurred period)` —
exact legacy column layout unverified without a captured legacy screenshot.
**Verdict:** MISMATCH-DOCUMENTED (correctly scoped, but zero manufacturer
dimension despite the leaf name — same class of gap as the O-phase siblings).

### 2. `manufacturer-wise-sales-detail-and-summary`
**Status:** event-ledger, `se.aggregate = 'sale'`, `salesMode = ""`.
**Diagnostic finding:** Same as #1 — plain 6-column ledger, no manufacturer
join. The title implies a combined detail+summary report (two sections or two
view modes), which additionally is not representable by any single existing
`salesMode` value (`line-detail` and `invoice-summary` are mutually exclusive
today); this would need either a new composite mode or two report leaves
under the hood, on top of the manufacturer join.
**Recommended JOIN:** same as #1, plus a decision on how to reconcile the
"detail and summary" title against the single-`salesMode` dispatch model —
outside the scope of what a minimal join fix alone would resolve.
**Verdict:** MISMATCH-DOCUMENTED (manufacturer dimension missing; additionally
ambiguous detail+summary shape needs a legacy capture to resolve).

### 3. `manufacturer-wise-net-sales`
**Status:** event-ledger, `se.aggregate = 'sale'`, `salesMode = ""`.
**Diagnostic finding:** Same missing-join gap as #1. "Net Sales" also implies
sales minus returns netted per manufacturer, which the current
`aggregateCondition = reportSaleAggregate` (sale only, no return) doesn't even
scope for — unlike leaf #5 below, whose title contains "return" and so
correctly gets `reportSaleOrReturn`. A true "net sales" projection would need
both the manufacturer join **and** the return-netting condition/arithmetic.
**Recommended JOIN:** same as #1; additionally reconsider whether
`aggregateCondition` should be `reportSaleOrReturn` with an explicit
sale-minus-return netting expression, not just `reportSaleAggregate`.
**Verdict:** MISMATCH-DOCUMENTED (manufacturer dimension missing; "net" also
unimplemented — sales only, no return netting).

### 4. `manufacturer-wise-item-sales-discount`
**Status:** event-ledger, `se.aggregate = 'sale'`, `salesMode = ""`.
**Diagnostic finding:** Same missing-join gap. Additionally, "Item
Sales/Discount" implies per-item discount columns; the codebase already has
this exact discount-column vocabulary in `dailySaleDetailColumns()`
(`Disc%`, `Discount Value`, `Item Disc`) used by `salesMode = "line-detail"`
— so a manufacturer-scoped variant of that shape (manufacturer + item +
discount columns) is the most plausible target, not a bare summary.
**Recommended query/JOIN:** compose the manufacturer join (#1) with the
existing `line-detail` discount-column expressions from
`dailySalesDetailReadModelQuery`, grouped or filtered by manufacturer.
**Verdict:** MISMATCH-DOCUMENTED (manufacturer dimension missing; discount
columns also absent from the current generic 6-column output).

### 5. `manufacturer-wise-manufacturer-wise-sales-and-return-summary`
**Status:** event-ledger, `aggregateCondition = reportSaleOrReturn`
(`se.aggregate IN ('sale', 'sale_return')`) because its title contains
"return" — correctly scoped to include returns, unlike #3.
**Diagnostic finding:** Confirmed the return side of this scoping is live
data, not vacuous: this tenant has **30,704 posted `cash-sale-return`
documents** (`SELECT kind, status, count(*) FROM business_documents WHERE
kind ILIKE '%sale%return%' GROUP BY kind, status` → `cash-sale-return |
posted | 30704`). The existing `salesReadModelQuery`'s
`canonicalReturnUnion` branch for `reportSaleOrReturn` already correctly
includes `'cash-sale-return'` (and `'credit-sale-return'`) in its `bd.kind IN
(...)` filter — this is **not** the same kind-spelling bug wave-1 found and
fixed in `header-wise-transaction-summary`; that bug does not reproduce here.
Same manufacturer-join gap as #1 applies on top of the correctly-scoped
sale+return union: no manufacturer column, no manufacturer grouping, and no
"summary" aggregation exists either (still the raw per-line 6-column shape).
**Recommended query/JOIN:** manufacturer join (#1) applied to both the sale
and sale-return branches of `salesReadModelQuery`'s union, then grouped by
manufacturer with separate sale/return sub-totals — a new `salesMode` (e.g.
`"manufacturer-sales-return-summary"`) would be needed since no existing mode
computes a two-column sale-vs-return split.
**Verdict:** MISMATCH-DOCUMENTED (aggregate scoping is correct and return data
is real/non-vacuous; manufacturer dimension and sale/return summary
aggregation are both missing).

### 6. `manufacturer-wise-cnic-ntn-registered-customers-sales`
**Status:** event-ledger, `se.aggregate = 'sale'`, `salesMode = ""`.
**Diagnostic finding — separate data dimension, checked independently of the
manufacturer join:** `master_parties` has legacy payload keys `NIC` and
`NTNNo` (not `CNIC`/`NTN` as literally named — confirmed by enumerating
`jsonb_object_keys(payload)` for this tenant's customer rows). But the
sandbox tenant has **only 2 `master_parties` rows with `party_type =
'customer'` at all**:

| id | name | NIC | NTNNo |
|---|---|---|---|
| `57ea27bc-...` | CASH SALES CUSTOMER | *(blank)* | *(blank)* |
| `02126279-...` | CREDIT SALES-WALKING CUSTOMER A/C | *(blank)* | *(blank)* |

Both are generic bucket/walk-in accounts, not real named customers, and both
have empty `NIC`/`NTNNo`. Confirmed these are the **only** two customers
referenced by any posted sale document in the tenant:
```sql
SELECT bd.customer_id, mp.name, count(*) FROM business_documents bd
LEFT JOIN master_parties mp ON mp.tenant_id=bd.tenant_id AND mp.id=bd.customer_id
WHERE bd.kind IN ('cash-sale','credit-sale') AND bd.status='posted'
GROUP BY bd.customer_id, mp.name;
```
→ `CASH SALES CUSTOMER` (291,359 docs), `CREDIT SALES-WALKING CUSTOMER A/C`
(2 docs). There is no individually-identified customer base with CNIC/NTN
data in this sandbox tenant, independent of and in addition to the
manufacturer-join gap.
**Recommended JOIN:** the manufacturer join (#1) is still applicable for the
manufacturer dimension, but the CNIC/NTN customer-registration dimension
cannot be verified or usefully populated against this sandbox tenant's data
at all — even with a correct query, the filter `WHERE NIC <> '' OR NTNNo <>
''` would return **zero rows** today.
**Verdict:** VACUOUS-NO-DATA for the CNIC/NTN dimension (0/2 customers have
either field populated, and only 2 customer records exist tenant-wide) —
**in addition to** the same manufacturer-join gap shared by leaves 1-5. This
leaf cannot be golden-verified with real data in this environment regardless
of whether the manufacturer join is fixed; it would need a migration/data
gap closed first (real per-customer registration with CNIC/NTN values) or a
different, non-empty tenant.

## Verdict summary

| # | Leaf | ProjectionStatus | Manufacturer join present? | Verdict |
|---|---|---|---|---|
| 1 | `manufacturer-wise-sales` | event-ledger | No (fixable — see headline finding) | MISMATCH-DOCUMENTED |
| 2 | `manufacturer-wise-sales-detail-and-summary` | event-ledger | No | MISMATCH-DOCUMENTED |
| 3 | `manufacturer-wise-net-sales` | event-ledger | No (also: no return-netting) | MISMATCH-DOCUMENTED |
| 4 | `manufacturer-wise-item-sales-discount` | event-ledger | No (also: no discount columns) | MISMATCH-DOCUMENTED |
| 5 | `manufacturer-wise-manufacturer-wise-sales-and-return-summary` | event-ledger | No (aggregate scoping itself is correct) | MISMATCH-DOCUMENTED |
| 6 | `manufacturer-wise-cnic-ntn-registered-customers-sales` | event-ledger | No | VACUOUS-NO-DATA (CNIC/NTN dimension) + same join gap |

**0 of 6 leaves are `projectionStatus == "real"`.** All 6 share the identical
mechanical cause: `salesMode` is unset, so `salesReadModelQueryMode` falls
through to the plain 6-column `salesReadModelQuery`, which has never joined
`master_manufacturers` (nor has any other report in `reports.go`, Phase N, O,
or P). Unlike some other event-ledger leaves in this project, this is **not**
a case where the arithmetic itself is wrong — the underlying sale/sale-return
scoping (`aggregateCondition`) is correctly chosen for every leaf, including
correctly including `cash-sale-return` for leaf #5. The gap is purely the
missing grouping/column dimension the leaf name promises, exactly the class
of "MISMATCH-DOCUMENTED" the O-phase doc already established for its two
manufacturer-wise siblings.

## Flag for other agents / the next promotion pass

**A working, fully-populated manufacturer-name join path exists and is
currently unused everywhere in `reports.go`:**
```sql
LEFT JOIN master_manufacturers mf
  ON mf.tenant_id = i.tenant_id AND mf.code = i.payload->>'ManfCode'
```
(where `i` is `master_items` already joined via `item_id`), resolving
**620,615/620,615 (100%)** of this sandbox tenant's posted sale lines and
**30,052/30,052 (100%)** of the item master to a real manufacturer name (838
distinct manufacturers on file, 322 of them actually appearing in sales
data). This directly unblocks:
- My 6 N-phase leaves (this doc).
- The 2 already-documented O-phase MISMATCH-DOCUMENTED leaves,
  `manufacturer-wise-detail` and `manufacturer-wise-monthly-stock-movement`
  (`docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md`), and likely
  `supplier-manufacturer-wise-g-p`'s manufacturer half (the G/P calculation
  itself remains a separate open question).
- The P-phase `stock-in-hand-manufacturer-wise`, `-format2`, and
  `supplier-manufacturer-association` leaves, which already reach for
  `payload->>'Manufacturer'`/`ManfCode` but currently display the bare
  numeric code (e.g. `"62"`) instead of the resolved name (`"BRISTOL-MYERS"`)
  because none of them join to `master_manufacturers` either.

I did not apply this join anywhere — `reports.go` is out of scope for this
agent per the concurrent-edit rule. This is flagged for whichever agent is
authorized to promote/edit `reports.go` in this wave.

## Companion test

`services/api/internal/httpapi/report_golden_n_manufacturer_test.go` locks:
- registry wiring (title/aggregate/salesMode) for all 6 leaves,
- the current `event-ledger` `ProjectionStatus` and 6-column generic shape
  (so a future promotion pass updates this doc/test together instead of
  silently drifting),
- that `salesReadModelQueryMode` with `salesMode = ""` is byte-identical to
  the plain `salesReadModelQuery` and references none of
  `master_manufacturers`/`ManfCode`/`Manufacturer` (so a future join fix
  flips this test, which is the intended trigger to update this doc).

Verified with `go vet ./services/api/...` (clean) and
`go test -run TestPhaseNGoldenManufacturer ./services/api/internal/httpapi/`
(3/3 passing, offline, no DB dependency). The DB-side join/volume/CNIC-NTN
findings above were verified live via `psql` against the sandbox tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` using the RLS preamble supplied for
this task; those queries are not embedded in the Go test suite (no live-DB
test harness was added, matching the O-phase companion test's offline-only
scope).

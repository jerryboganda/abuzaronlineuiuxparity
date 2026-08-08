# Phase Q golden verification — Adjustment Reports & Item/Stock History leaves — 2026-08-09

## Scope and method

This pass golden-verifies 13 Phase Q report leaves from `services/api/internal/httpapi/reports.go`:

- **Adjustment leaves 1–6** (`phaseQReportRegistry`, lines 433–449): all six map to
  `reportSpec{stockReadModel: true, stockMode: "adjustment"}`, resolved at request time by
  `stockReadModelQuery("adjustment", ...)` (lines 3964–3986), called from the `stockReadModel`
  branch of `(*Server).report` at lines 5108–5133 (query built at line 5125, bound at line 5126).
- **Item/stock-history leaves 7–13** (lines 538–554): all seven map to `reportSpec{historyMode: ...}`,
  resolved by `historicalReportQuery(mode, ...)` (lines 4645–4743), called from the `historyMode`
  branch of `(*Server).report` at lines 5201–5218.

Per the task's environment constraint, the live legacy SQL Server is not reachable in this
environment, so no cross-check was attempted against it. All verification below is
Postgres-internal: independent SQL against `abuzar_next`, plus two new isolated-tenant Go
integration tests (not touching the shared sandbox tenant's data) that replay the exact query
text and the exact HTTP path. RLS preamble used for every ad hoc sandbox-tenant query:

```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', '', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```

Sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` / branch `ffffffff-ffff-ffff-ffff-ffffffffffff`
is the only tenant in this database with substantial migrated data (781,203 `stock_ledger` rows;
331,928 `sync_events` rows, all `aggregate = 'historical_document'`).

**Headline result: 1 severe, always-reproducing bug (leaves 1–6, one shared root cause) and 1
confirmed data-corruption bug (leaves 8 and 10, one shared root cause), both proven with new
integration tests, plus data-completeness and lower-severity projection findings for the rest.**
None of the 13 leaves is golden-verified against real legacy output (still 0/151 project-wide);
this pass closes the verification gap for these 13 leaves specifically and finds concrete defects
where they exist.

New evidence artifact: `services/api/internal/httpapi/report_golden_adjustment_history_test.go`,
containing:

- `TestAdjustmentReportLeavesAlwaysFailWithParameterGapError` — proves the SQLSTATE 42P18 crash
  (leaves 1–6) two independent ways (raw SQL replay + full HTTP handler across all six leaf kinds).
- `TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn` — proves the
  COALESCE-across-mismatched-types defect (leaves 8, 10) with seeded data through the full HTTP
  handler.

Both pass today (`go test -run 'TestAdjustmentReportLeavesAlwaysFailWithParameterGapError|TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn' ./services/api/internal/httpapi/`),
confirming the bugs exist as described. Both tests are written to *fail* once the underlying bug
is fixed, with an inline comment saying so, so they double as regression pins.

`go vet ./services/api/...` is clean.

---

## Leaves 1–6 — Adjustment Reports (`stockMode: "adjustment"`)

| # | Kind | Title |
|---|---|---|
| 1 | `adjustment-adjustment-summary` | Adjustment Summary |
| 2 | `adjustment-adjustment-detail` | Adjustment Detail |
| 3 | `adjustment-adjustment-summary-inv-wise` | Adjustment Summary Inv. Wise |
| 4 | `adjustment-adjustment-detail-inv-wise` | Adjustment Detail Inv. wise |
| 5 | `adjustment-adjustment-summary-detail` | Adjustment Summary/Detail |
| 6 | `adjustment-item-wise-adjustment-summary` | Item Wise Adjustment Summary |

### Finding A (CRITICAL, all 6 leaves) — every request fails with SQLSTATE 42P18; the report is 100% non-functional regardless of data

**Root cause.** `stockReadModelQuery("adjustment", pagination)` (reports.go:3964–3986) builds:

```sql
WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
  AND l.direction = 'adjustment'
  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
  AND l.occurred_at >= $3::date
  AND l.occurred_at < ($4::date + INTERVAL '1 day')
  AND ($5 = '' OR l.id::text ILIKE '%' || $5 || '%'
       OR b.item_legacy_id ILIKE '%' || $5 || '%'
       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%')
ORDER BY l.occurred_at DESC, l.id
```
concatenated with the pagination footer `LIMIT $8 OFFSET $9` supplied by the sole call site
(reports.go:5125). The WHERE clause only ever mentions `$1`–`$5`; `$6` and `$7` (godownID,
batchNumber) never appear anywhere in the query text. The call site
(reports.go:5126) still binds all 9 positional arguments:

```go
query = stockReadModelQuery(spec.stockMode, "LIMIT $8 OFFSET $9")
err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, godownID, batchNumber, pageSize+1, (page-1)*pageSize)
```

PostgreSQL's extended query protocol requires every parameter position up to the highest
referenced number (`$9`, via the pagination footer) to be inferable from the query text. Since
`$6`/`$7` are bound but never referenced, every execution fails at the database with:

```
ERROR: could not determine data type of parameter $6 (SQLSTATE 42P18)
```

which `(*Server).report` (reports.go:5128–5131) turns into `HTTP 503 report_read_failed /
"The normalized stock report query failed."` — for **every** request to any of the six leaves,
on any tenant, with or without adjustment data. This is not a data-completeness gap; it reproduces
identically against an empty `stock_ledger` and against the 781k-row sandbox tenant.

For contrast, every sibling branch of `stockReadModelQuery` that shares the same 9-argument call
site does reference `$6`/`$7`, e.g. the `movement`/`narcotics-movement` branch
(`AND ($6 = '' OR b.godown_id = $6::uuid) AND ($7 = '' OR b.batch_number ILIKE ...)`, reports.go
~3936–3939) and the final fallback branch (reports.go ~4010–4015). The `adjustment` branch is the
one outlier that omits this filtering entirely.

**Proof 1 — direct SQL replay**, run via a standalone probe (`go run`, pgx driver, same as the
production driver) against the sandbox tenant with a full open date range:

```
QUERY ERROR: ERROR: could not determine data type of parameter $6 (SQLSTATE 42P18)
```

**Proof 2 — integration test**, `TestAdjustmentReportLeavesAlwaysFailWithParameterGapError`
(new file). It seeds an isolated tenant (not the shared sandbox tenant) with two genuine manual
`stock_ledger` adjustment rows (`direction='adjustment'`, signed +3 and −1) and one sale/purchase-shaped
`in` row on the same batch/day, to give the query real matching data if it could execute, then:

1. Independently recomputes the expected result with a hand-written query
   (`SELECT (l.quantity * l.adjustment_sign)::text ... WHERE direction = 'adjustment' ...`),
   confirming `[-1, 3]` (occurred_at DESC) is the objectively correct target once the bug is fixed.
2. Replays the verbatim `stockReadModelQuery("adjustment", ...)` text directly against Postgres and
   asserts the error contains `42P18`.
3. Drives all six leaf kinds through the real `(*Server).report` HTTP handler and asserts `503` +
   `report_read_failed` for every one of them.

```
=== RUN   TestAdjustmentReportLeavesAlwaysFailWithParameterGapError
--- PASS: TestAdjustmentReportLeavesAlwaysFailWithParameterGapError (0.75s)
```

**Verdict: MISMATCH-DOCUMENTED** (severity: functional outage, not a numeric discrepancy — the
report never returns any output, correct or incorrect). Root cause: reports.go:3964–3986 (query
text) × reports.go:5125–5126 (call site), function `stockReadModelQuery`. Fix direction (not
applied — reports.go is read-only for this pass): either add the missing `$6`/`$7`
godown/batch-number predicates to the `adjustment` branch (matching every sibling branch's
pattern), or change the call site to pass a pagination footer that matches the parameters the
`adjustment` branch actually uses.

### Finding B (all 6 leaves) — no summary/detail/invoice-wise/item-wise differentiation exists at all

Independent of Finding A: all six differently-titled leaves resolve to the exact same
`stockReadModel: true, stockMode: "adjustment"` spec (reports.go:444–449), the same
`stockReportColumns("adjustment")` six-column schema (reports.go:1009–1018: Adjustment/Date/Direction/Item/Quantity/Unit
Cost), and the same ungrouped, unaggregated `stockReadModelQuery("adjustment", ...)` row-level
listing — there is no `GROUP BY`, no per-item aggregation, and no per-invoice/header grouping
anywhere in that query. "Adjustment Summary" and "Adjustment Detail" would return byte-identical
rows; "Item Wise Adjustment Summary" does not group by item at all. This contrasts with other Q-wave
report families that do implement distinct summary vs. detail modes (e.g. the `quotation-detail`
/`quotation-summary` pair at reports.go:462–473 uses `documentMode` "line-detail" vs.
"invoice-summary").

This finding is moot from the caller's perspective until Finding A is fixed (the leaves currently
never return any rows to differentiate), but it means fixing Finding A alone is not sufficient to
make these six leaves legacy-accurate — five of the six titles would still need real
grouping/aggregation logic built.

**Verdict: MISMATCH-DOCUMENTED** (secondary, currently masked by Finding A).

### Finding C (minor) — the `adjustment` mode's disclosed projection note describes the wrong table

`stockProjectionNote("adjustment")` (reports.go:1215–1247) has no explicit `case "adjustment"` and
falls through to the `default` branch: *"Normalized posted stock_balances projection joined to
stock batches, items, and godowns; legacy manufacturer/category/class/reorder/narcotics groupings
and exact valuation are not implemented."* The actual `adjustment` query never touches
`stock_balances` at all — it reads `stock_ledger` directly. The disclosed projection note is
inaccurate about its own data source. Confirmed via `TestAdjustmentReportLeavesAlwaysFailWithParameterGapError`'s
sibling assertions in the earlier draft of this test and by direct inspection of
`reportDefinitionForKey` (reports.go:1455–1458, which calls `stockProjectionNote(spec.stockMode)`
unconditionally).

**Verdict: MISMATCH-DOCUMENTED** (cosmetic/documentation-accuracy only).

### Data-completeness note for leaves 1–6

Independent of the above code bugs, the sandbox tenant currently has **zero** `direction='adjustment'`
rows in `stock_ledger` (781,203 total rows, all `direction IN ('in','out')`, `adjustment_sign = 1`
for all of them):

```
 direction | count
-----------+--------
 out       | 623096
 in        | 158107
```

Root cause traced structurally: the sandbox tenant's `sync_events` contains only
`aggregate = 'historical_document'` rows (331,928 of them; zero `aggregate = 'inventory'` rows),
and grep across `migration/` for `AdjHeader`/`AdjDetail` shows the *only* code path that reads
legacy `dbo.AdjHeader`/`dbo.AdjDetail` is `migration/cmd/bulk-historical`'s `importAdjustments`,
which writes to `historical_stock_adjustment_lines` — a table these six leaves never query. There
is currently no migration path that projects legacy manual adjustments into `stock_ledger` with
`direction='adjustment'`; that direction value is only ever written by the live app's stock
adjustment/void-reversal endpoints (`business.go:1253`, `void_reversal.go:195`) for adjustments made
*after* migration. Even once Finding A is fixed, these six leaves will show only new,
post-migration manual adjustments — never the legacy adjustment history — unless a further
migration/query change routes `historical_stock_adjustment_lines` into leaves 1–6 the way leaf 13
already does for `historical_*` union coverage (see leaf 13 below).

---

## Leaves 7–13 — Item/Stock History (`historyMode`)

| # | Kind | historyMode | Source table |
|---|---|---|---|
| 7 | `item-reports-deleted-sale-items-log` | `deleted-sale-items` | `historical_deleted_sale_items` |
| 8 | `item-reports-history-sale-price-difference` | `item-price-difference` | `historical_item_changes` |
| 9 | `item-reports-history-item-basic-data-changes` | `item-basic-data` | `historical_item_changes` |
| 10 | `item-reports-history-item-sale-price-changes` | `item-sale-price` | `historical_item_changes` |
| 11 | `item-reports-history-new-item-s-created-defined` | `item-first-observed` | `historical_item_changes` |
| 12 | `item-reports-history-item-name-changes` | `item-name` | `historical_item_changes` |
| 13 | `item-reports-stock-adjustments-stock-adjustments-detail` | `stock-adjustments` | `historical_stock_adjustment_lines` + `stock_ledger` |

### Data-completeness — all three source tables are empty across every tenant in this database

```sql
SET row_security = off;
SELECT tenant_id, count(*) FROM historical_item_changes GROUP BY tenant_id;              -- 0 rows
SELECT tenant_id, count(*) FROM historical_deleted_sale_items GROUP BY tenant_id;        -- 0 rows
SELECT tenant_id, count(*) FROM historical_stock_adjustment_lines GROUP BY tenant_id;     -- 0 rows
```

All three queries return **zero rows across every tenant**, not just the sandbox tenant — this is a
whole-database data-completeness gap, not a scoping artifact. Root cause, confirmed by reading
`migration/cmd/bulk-historical/main.go` (`importItemHistory` line 1409, `importDeletedSaleItems`
line 1634, `importAdjustments` line 1826): the loader that reads legacy `dbo.ItemLog`,
`dbo.DeletedSaleItem`, and `dbo.AdjHeader`/`dbo.AdjDetail` exists and is well-formed, but per
`docs/PHASE_Q_DELETED_SALE_ITEMS_EVIDENCE_2026-08-07.md` line 38, *"The loader was not run against
the canonical source in this coding slice"* — consistent with the stated environment constraint
that the legacy SQL Server is unreachable here. This is corroborated by zero `payload::text ILIKE
'%AdjCode%'` hits anywhere in `sync_events` for the sandbox tenant.

**Query-execution sanity check (independent of the empty-table result):** to rule out a
zero-rows-due-to-a-second-hidden-bug scenario (the same failure mode found in leaves 1–6), all
seven `historicalReportQuery` branches were replayed directly against Postgres with realistic
bound parameters. All seven executed successfully with no SQL error (unlike the adjustment
leaves' SQLSTATE 42P18):

```
deleted-sale-items OK rows: 0
stock-adjustments OK rows: 0
item-history:item-price-difference OK rows: 0
item-history:item-basic-data OK rows: 0
item-history:item-sale-price OK rows: 0
item-history:item-first-observed OK rows: 0
item-history:item-name OK rows: 0
```

**Verdict for the "no rows" observation itself: VACUOUS-NO-DATA**, not a code defect — the
`historicalReportQuery` parameter wiring is sound (all `$1..$N` placeholders referenced correctly
for both the 7-arg deleted-sale-items/stock-adjustments call shape and the 8-arg generic
item-history call shape), it is simply reading tables the migration loader has never populated.

### Finding D (CONFIRMED BUG, leaves 8 and 10) — `old_sale_price`/`new_sale_price` NULL silently leaks the item's name into the price column

**Root cause.** `historicalReportQuery`'s default branch (reports.go:4720–4742, used by
`item-price-difference`, `item-basic-data`, `item-sale-price`, `item-first-observed`, `item-name`)
computes the "Previous"/"Current" columns as:

```sql
CASE WHEN h.report_kind IN ('item-price-difference', 'item-sale-price')
     THEN COALESCE(h.old_sale_price::text, h.old_name, '')
     ELSE h.old_name END,
CASE WHEN h.report_kind IN ('item-price-difference', 'item-sale-price')
     THEN COALESCE(h.new_sale_price::text, h.new_name, '')
     ELSE h.new_name END
```

`old_sale_price`/`new_sale_price` are nullable `numeric(19,4)` columns; `old_name`/`new_name` are
unrelated `NOT NULL DEFAULT ''` text columns (the item's name at that log entry). `COALESCE` across
these two semantically unrelated columns means that whenever a price-kind row has a NULL price but
a populated name — which the loader (`migration/cmd/bulk-historical/main.go` `importItemHistory`,
lines 1509–1578) allows whenever a previous `ItemLog` snapshot had a blank `SalePrice` while `Name`
was still set — the "Previous"/"Current" column silently renders the item's **name** instead of a
price or a blank cell. Nothing catches this at the transport layer: `historicalReportColumns`'s
default branch (reports.go:841–848) declares both columns `DataType: "text"` (not `"currency"`),
so the UI has no type-based guard either.

**Proof — integration test**, `TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn`
(new file). Seeds one `historical_item_changes` row for `report_kind='item-price-difference'` with
`old_name='Panadol 500mg'`, `old_sale_price=NULL`, `new_sale_price=99.5000`, then reads
`item-reports-history-sale-price-difference` through the real HTTP handler:

```
=== RUN   TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn
--- PASS: TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn (0.76s)
```

The test asserts the row's "Previous" column is literally `"Panadol 500mg"` (today's actual,
incorrect behavior) rather than blank, and that "Current" is correctly `"99.5000"` (unaffected,
since `new_sale_price` was non-NULL in this fixture). The same defect applies symmetrically to
`item-reports-history-item-sale-price-changes` (`report_kind = 'item-sale-price'`), which shares
the identical CASE expression.

**Verdict: MISMATCH-DOCUMENTED (confirmed, not inferred).** Fix direction (not applied): drop the
`h.old_name`/`h.new_name` fallback for the price-report kinds —
`COALESCE(h.old_sale_price::text, '')` / `COALESCE(h.new_sale_price::text, '')` — so a missing
price renders blank instead of leaking an unrelated field.

### Finding E (leaf 9, `item-basic-data`) — the report's Previous/Current columns never show what actually changed

`item-basic-data` rows are emitted by the loader when `basicChanged` is true — computed from a
17-field fingerprint (`itemLogBasicFingerprint`, main.go:1607–1618:
`customicode, stock, active, iccode, icatcode, packcode, manfcode, packunits, location1, remarks1,
reorderqty, optimumqty, restricted, taxable, itemtypecode, measureunitcode, genericcode, gcode`) —
entirely independent of the item's `name`. But the report query's `ELSE` branch
(reports.go:4727, 4730) always projects `h.old_name`/`h.new_name` into the "Previous"/"Current"
columns for this report kind too, exactly as it does for the unrelated `item-name` report kind. A
row whose sole purpose is signaling e.g. a `PackUnits` or `ReorderQty` change will show the item's
name before/after (frequently identical, since the name usually didn't change), carrying zero
information about the actual basic-data field or its old/new values. The full snapshot *is*
retained in `payload jsonb` (not lost), it just isn't projected into the report's six-column
contract. `historicalProjectionNote` (reports.go:851–859) does not call this out specifically; it
only says "exact PowerBuilder columns... remain unverified" generically for the whole default
branch.

**Verdict: MISMATCH-DOCUMENTED** (projection-completeness gap, not a crash or corrupted value —
the column literally contains the item name, correctly, just not the information a "Basic Data
Changes" report implies it should show). Not covered by a new test in this pass (lower severity
than Findings A/D; the fix requires designing which of the 17 fields to surface and in what
shape, a product decision beyond "flip a fallback").

### Finding F (leaf 13, `stock-adjustments`) — the two halves of the UNION resolve item/godown identity inconsistently

`historicalReportQuery("stock-adjustments", ...)` (reports.go:4670–4718) unions two branches:

- `historical_stock_adjustment_lines` branch (reports.go:4672–4684): projects `item` as
  `h.item_legacy_id || ... || h.batch` and `party` as
  `concat_ws(' / ', h.godown_legacy_id, h.user_legacy_id)` — **raw legacy codes only**, despite
  `historical_stock_adjustment_lines` having nullable `item_id`/`godown_id` FK columns already
  resolved at import time (`migration/cmd/bulk-historical/main.go` lines 1883–1894 LEFT JOINs
  `master_items`/`master_godowns` when populating the table). No JOIN to `master_items` or
  `master_godowns` exists in this branch of the report query.
- `stock_ledger` branch (reports.go:4687–4710): projects
  `COALESCE(NULLIF(i.name, ''), b.item_legacy_id)` and `COALESCE(NULLIF(g.name, ''), '')` via
  explicit `LEFT JOIN master_items i` / `LEFT JOIN master_godowns g` — **resolved names with
  code fallback**.

Rows from the legacy-import half of the report will always show raw item/godown codes; rows from
the live-ledger half will show friendly names when available. A user paging through one unified
"Stock Adjustments Detail" report will see two different presentation styles depending on which
half of the migration a given adjustment came from, for no legacy-fidelity reason — the resolved
`item_id`/`godown_id` are sitting right there in the same table. `TestHistoricalStockAdjustmentReportIncludesNormalizedLedgerRows`
(existing, `historical_integration_test.go:114`) only exercises the `stock_ledger` half; no
existing test covers the `historical_stock_adjustment_lines` half's item/godown presentation, so
this inconsistency had no test coverage before this pass either way.

A related, unverifiable nuance: the historical branch's `quantity` column uses
`h.loose_quantity::text` (raw, unsigned as stored) while the `stock_ledger` branch uses
`(l.quantity * l.adjustment_sign)::text` (explicitly signed). Whether legacy `AdjDetail.LooseQty`
already encodes increase/decrease via sign (no direction/type column was found in the `AdjDetail`
column list read by `importAdjustments`, main.go:1827–1836) cannot be determined without the
unreachable legacy SQL Server schema.

**Verdict: MISMATCH-DOCUMENTED** for the missing master-data JOIN (a clear, fixable code gap).
**UNVERIFIABLE-THIS-PASS: cannot confirm legacy `LooseQty` sign semantics without SQL Server
access** for the quantity-sign nuance.

### Leaves 11 and 12 (`item-first-observed`, `item-name`) — no bug found

`item-first-observed` rows are emitted only when `!hasPrevious` (main.go:1554–1557), so
`old_name`/`old_sale_price` are always blank by construction and the `ELSE` branch correctly shows
blank "Previous" / the created item's name as "Current". `item-name` rows are emitted only when
`current.name != previousSnapshot.name` (main.go:1518, 1573–1577), and the `ELSE` branch correctly
projects `h.old_name`/`h.new_name` — exactly the old and new names, which is exactly what an
"Item Name Changes" report should show. Both are structurally sound.

**Verdict: VACUOUS-NO-DATA** (query logic reviewed and appears correct; no rows exist to verify
against because the loader has never run).

### Leaf 7 (`deleted-sale-items`) — no bug found, one open semantic question

The query (reports.go:4646–4668) matches the migration importer's field mapping 1:1: sale invoice
code, machine/user, item/godown (with name resolution via `LEFT JOIN master_items`), quantity +
bonus quantity, and sale price all trace cleanly to `DeletedSaleItem` columns captured by
`importDeletedSaleItems` (main.go:1634 onward). One open question that cannot be resolved without
legacy schema access: the report labels `occurred_at` as "Deleted At", sourced from legacy
`DeletedSaleItem.[Date]` — whether that column represents the deletion timestamp or the original
sale timestamp in the legacy schema is unconfirmed.

**Verdict: VACUOUS-NO-DATA**, with **UNVERIFIABLE-THIS-PASS: cannot confirm whether legacy
`DeletedSaleItem.Date` means deletion time or original sale time** as a secondary note.

---

## Verdict summary

| # | Leaf | Verdict |
|---|---|---|
| 1 | `adjustment-adjustment-summary` | MISMATCH-DOCUMENTED — SQLSTATE 42P18 crash, every request (Finding A) |
| 2 | `adjustment-adjustment-detail` | MISMATCH-DOCUMENTED — same (Finding A) |
| 3 | `adjustment-adjustment-summary-inv-wise` | MISMATCH-DOCUMENTED — same (Finding A) |
| 4 | `adjustment-adjustment-detail-inv-wise` | MISMATCH-DOCUMENTED — same (Finding A) |
| 5 | `adjustment-adjustment-summary-detail` | MISMATCH-DOCUMENTED — same (Finding A) |
| 6 | `adjustment-item-wise-adjustment-summary` | MISMATCH-DOCUMENTED — same (Finding A) |
| 7 | `item-reports-deleted-sale-items-log` | VACUOUS-NO-DATA (+ 1 unverifiable semantic question) |
| 8 | `item-reports-history-sale-price-difference` | MISMATCH-DOCUMENTED — confirmed COALESCE defect (Finding D) |
| 9 | `item-reports-history-item-basic-data-changes` | MISMATCH-DOCUMENTED — Previous/Current never shows the actual change (Finding E) |
| 10 | `item-reports-history-item-sale-price-changes` | MISMATCH-DOCUMENTED — same defect as #8 (Finding D) |
| 11 | `item-reports-history-new-item-s-created-defined` | VACUOUS-NO-DATA — logic reviewed, no bug found |
| 12 | `item-reports-history-item-name-changes` | VACUOUS-NO-DATA — logic reviewed, no bug found |
| 13 | `item-reports-stock-adjustments-stock-adjustments-detail` | MISMATCH-DOCUMENTED — inconsistent name resolution across the UNION (Finding F) |

**0/13 MATCHED.** All 6 adjustment leaves share one root-cause crash bug (Finding A), proven with a
new integration test; leaves 8 and 10 share one root-cause data-corruption bug (Finding D), also
proven with a new integration test. Neither bug was previously known: no existing test in the
repository exercised `stockMode == "adjustment"` or seeded a NULL `old_sale_price`/populated
`old_name` combination before this pass.

## Files touched in this pass

- `services/api/internal/httpapi/report_golden_adjustment_history_test.go` (new) — two integration
  tests, both passing today, both designed to fail once the documented bugs are fixed.
- `docs/PHASE_Q_GOLDEN_VERIFICATION_ADJUSTMENT_HISTORY_2026-08-09.md` (this file).

`services/api/internal/httpapi/reports.go` was **not** modified (read-only per task instructions);
all fix directions above are documentation only.

## Verification commands run

```
go vet ./services/api/...
DATABASE_URL='postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable' \
  go test -run 'TestAdjustmentReportLeavesAlwaysFailWithParameterGapError|TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn' \
  ./services/api/internal/httpapi/ -v -count=1
```

Both commands were run in this environment; `go vet` produced no output (clean) and both new
tests reported `PASS`.

# Phase P Golden Verification — Stock Movement/Register Leaves (2026-08-09)

Scope: the 12 stockReadModel movement/register-family leaves assigned for this pass
(`services/api/internal/httpapi/reports.go` `phasePReportRegistry` lines 374-420,
`stockReportColumns`/`stockProjectionNote`, `isStockMovementSummaryMode`/
`isStockNarcoticsMode`/`isStockSalesMode`/`isStockManagementMode` near line 870-897,
and the query builders `stockReadModelQuery` (line 3890) and its per-mode helpers).
`reports.go` was read-only for this pass; no functional code was modified.

## Environment and honesty note

Per the assigned constraint, live legacy SQL Server was **not** reachable (Windows
trusted auth). All verification below is Postgres-internal cross-verification: for
each leaf, the exact SQL text reports.go builds for that mode was replicated
byte-for-byte in psql, executed against the sandbox tenant/branch, and cross-checked
against an independently-formulated query over `stock_ledger` /
`stock_batches` / `stock_balances` / `stock_allocations` / `master_items` that
computes the same figure a different way.

Sandbox scope used throughout:
- Tenant: `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`
- Branch: `ffffffff-ffff-ffff-ffff-ffffffffffff` ("Legacy Reference Sandbox")
- Godown: 1 row (`GODOWN1`, `2ef6964b-1b11-419a-866a-6544e7d28c2b`)
- RLS preamble applied for every scoped query (`app.tenant_id`, `app.allow_tenant_scope=true`,
  `app.authenticating=false`); superuser `postgres` role additionally bypasses RLS
  outright, so unscoped counts below (used to confirm data existence/absence) are not
  an artifact of RLS misconfiguration.

Baseline data inventory for the sandbox tenant/branch (established first, drives every
verdict below):

| Table | Row count | Notes |
|---|---|---|
| `stock_ledger` | 781,203 | `direction`: in=158,107, out=623,096, adjustment=0. Range 2025-01-01..2026-07-31. 100% `posted`. |
| `stock_batches` | 10,507 | 100% have `expiry_date` populated (10,507/10,507). |
| `stock_balances` | **0** | Genuinely zero for this tenant (other tenants have up to 60 rows), despite 781k ledger rows and 10,507 batches. |
| `master_items` | 30,052 | `payload` has exactly 37 distinct keys tenant-wide; **none** is `Narcotics`/`Narcotic`/`narcotics`, `GenericName`/`GenericCode`/`genericName`, or `Class`/`ItemClass`/`class`. |
| `master_godowns` | 1 | `GODOWN1` |
| `business_documents` (posted cash/credit sale) | 291,361 | |
| `stock_allocations` | **0** | Genuinely zero for this tenant (other tenants have up to 5 rows), despite 291,361 posted sales and 623,096 ledger OUT rows. |

Two systemic, cross-cutting gaps fall out of this inventory and explain 7 of the 12
verdicts below. Both are **data/cache-population gaps in the migrated sandbox
dataset**, not logic defects in the `reports.go` SQL itself — every query below reads
exactly the table its own documented projection note says it reads, and (where data
exists) reconciles exactly:

1. **`stock_balances` cache is empty for the sandbox tenant.** `stock_balances` is
   maintained transactionally by the live posting handlers (`services/api/internal/httpapi/stock.go`,
   e.g. lines 229-233, 404, 541, 659-662 — `INSERT ... ON CONFLICT DO UPDATE
   SET on_hand = stock_balances.on_hand + EXCLUDED.on_hand`), not by bulk import. A
   read-only reconstruction using the exact same aggregation as the app's own
   `rebuildStockBalances` handler (`stock.go` lines 1264-1278: `SUM(CASE WHEN
   direction='in' THEN quantity WHEN direction='out' THEN -quantity ELSE
   quantity*adjustment_sign END)` grouped by batch) shows the cache *would* contain
   10,507 rows, 7,312 of them with non-zero `on_hand`, if it were rebuilt. This was
   verified read-only (no `INSERT`/`UPDATE`/`DELETE` was executed against the shared
   sandbox — the reconstruction ran as a plain `SELECT`). Every mode whose
   `stockReadModelQuery` branch starts `FROM stock_balances sb` is therefore vacuous
   in this environment: `expiry`, `expiry-class`, `stock-sales`, `management-summary`.
2. **Migrated `master_items.payload` never captured a narcotics/generic/class field.**
   Enumerating `jsonb_object_keys(payload)` across all 30,052 sandbox items yields
   exactly 37 keys (`Active`, `AvgPrice`, `CustomICode`, `FlatDisc`, `GSTPerc1`,
   `GSTPerc2`, `Gcode`, `ICCode`, `ICatCode`, `ICode`, `LastUpdate`, `Location`,
   `Location1`, `ManfCode`, `Name`, `OptimumQty`, `PCTCode`, `PackSalesTax`,
   `PackUnits`, `Packcode`, `Prescribed`, `Printable`, `PurDiscPerc`, `PurPrice`,
   `Refrigrated`, `Remarks`, `Remarks1`, `ReorderQty`, `RetailPrice`, `SaleDiscPerc`,
   `SalePrice`, `SalePrice2..5`, `SalesTax`, `Taxable`) — none named `Narcotics`,
   `Narcotic`, `narcotics`, `GenericName`, `GenericCode`, `genericName`, `Class`,
   `ItemClass`, or `class`. The narcotics-filtered modes' `WHERE ... IN ('yes','y',
   'true','1')` predicate is therefore always false for this dataset, and the two
   class/generic groupings always fall back to their `'Unspecified'` default.

## Per-leaf verdicts

| # | Report kind | Mode | Verdict |
|---|---|---|---|
| 1 | `stock-register` | `movement` | **MATCHED** |
| 2 | `item-stock-register-summary` | `item-summary` | **MATCHED** |
| 3 | `stock-register-for-narcotics` | `narcotics-movement` | **VACUOUS-NO-DATA** |
| 4 | `stock-and-sales` | `stock-sales` | **VACUOUS-NO-DATA** |
| 5 | `item-activity` | `movement` | **MATCHED** |
| 6 | `stock-register-narcotics-format2` | `narcotics-movement` | **VACUOUS-NO-DATA** |
| 7 | `expiry-report` | `expiry` | **VACUOUS-NO-DATA** |
| 8 | `expiry-report-class-wise` | `expiry-class` | **VACUOUS-NO-DATA** |
| 9 | `daily-stock-in-out` | `movement-summary` | **MATCHED** |
| 10 | `stock-in-out-date-wise` | `movement-summary` | **MATCHED** |
| 11 | `stock-management-report` | `management-summary` | **VACUOUS-NO-DATA** |
| 12 | `norcotics-stock-register-generic-type-wise` | `narcotics-generic` | **VACUOUS-NO-DATA** |

No `MISMATCH` was found: everywhere real data exists, the report's own SQL
reconciled exactly against an independently-formulated cross-check.

---

### 1. `stock-register` (mode `movement`) — MATCHED

Query: `stockReadModelQuery("movement", ...)`, `reports.go` lines 3918-3959 (the
`mode == "movement" || mode == "narcotics-movement"` branch).

Methodology: replicated the CTE verbatim, scoped to a single sample day with real
posted activity, `2026-06-01`, and compared row counts and per-row sign convention
against raw `stock_ledger`.

Evidence:
```sql
-- ground truth, independent of the report's CTE
SELECT occurred_at::date, direction, count(*), sum(quantity)
FROM stock_ledger
WHERE tenant_id='eeee...' AND branch_id='ffff...'
  AND occurred_at >= '2026-06-01' AND occurred_at < '2026-06-02'
GROUP BY 1,2;
-- 2026-06-01 | in  | 327 | 9951.0000
-- 2026-06-01 | out | 764 | 6403.0000
```
The report's exact query for the same window returned 327 `in` rows summing to
`9951.0000` and 764 `out` rows summing to `-6403.0000` (i.e. `6403.0000` magnitude,
correctly negated) — **1091 rows total**, matching the raw `stock_ledger` row count
for the window exactly (`SELECT count(*) FROM stock_ledger WHERE ...` = 1091,
independent of any join). Confirmed the report's `JOIN stock_batches` / `JOIN
sync_events` do not silently drop rows: `LEFT JOIN` variants of the same predicate
found 0 ledger rows missing a batch match and 0 missing a `sync_events` match.

Spot-checked 5 individual rows' sign convention directly against raw columns
(`direction`, `quantity`, `adjustment_sign`) vs. the CASE expression's expected
output — all 5 matched (e.g. `out`, qty `10.0000` → expected `-10.0000`, `in`, qty
`60.0000` → expected `60.0000`).

### 2. `item-stock-register-summary` (mode `item-summary`) — MATCHED

Query: `stockItemSummaryReadModelQuery`, `reports.go` lines 3347-3390. Groups
`stock_ledger` by `(item_legacy_id, movement_date, godown, item_name)`.

Methodology: replicated the query for `2026-06-01`, then independently recomputed
the same aggregate using `SUM(...) FILTER (WHERE direction=...)` with an
**independently-written** grouping and formula (not copy-pasted from the report's
CASE expression).

Evidence: the report's exact query produced 778 groups, net quantity total
`3548.0000`, net value total `5275102.13640000`. The independent FILTER-based
recomputation (grouped by `item_legacy_id, occurred_at::date,
COALESCE(godown_name,'')`, `COALESCE(SUM(...) FILTER (...),0) - COALESCE(SUM(...)
FILTER (...),0)`) produced the identical 778 groups and identical totals
(`3548.0000` / `5275102.13640000`) — both totals also reconcile to the raw ungrouped
day total (`9951 - 6403 = 3548`).

Methodology note for transparency: an initial version of the independent
FILTER-based query (without `COALESCE(...,0)` around each `FILTER` sub-aggregate)
produced a wrong total (`2308.0000`/`1270733.3999`) due to standard SQL `NULL`
propagation (`SUM(x) FILTER (...)` returns `NULL`, not `0`, for a group with no
matching rows, and `NULL - y = NULL`, silently dropping that group's contribution
from the outer `SUM`). This was a bug in the verification query, not the report;
once corrected with `COALESCE`, the independent total matched the report exactly.
Documented here because the same NULL-propagation pitfall is worth flagging to
other agents cross-verifying `stock_ledger` aggregates the same way.

### 3. `stock-register-for-narcotics` (mode `narcotics-movement`) — VACUOUS-NO-DATA

Query: `stockNarcoticsReadModelQuery("narcotics-movement", ...)`, `reports.go` lines
3796-3886. Filters `stock_ledger` to items where `LOWER(COALESCE(payload->>'Narcotics',
payload->>'Narcotic', payload->>'narcotics', '')) IN ('yes','y','true','1')`.

Evidence: ran the exact CTE over the full open date range (`1900-01-01`..`2999-12-31`)
— **0 rows**. Root cause confirmed via the payload key enumeration in the shared
findings above: no sandbox item has any of the three candidate payload keys at all
(not merely a false value — the keys are entirely absent from all 30,052 items'
payload), so the predicate can never be satisfied against this dataset. This is a
migrated-data gap (the import pipeline that populated `master_items.payload` did not
carry a narcotics flag column from the legacy Item master), not a `reports.go` logic
defect — the filter itself is written correctly against whatever key names the
payload is expected to carry.

### 4. `stock-and-sales` (mode `stock-sales`) — VACUOUS-NO-DATA

Query: `stockSalesReadModelQuery`, `reports.go` lines 3503-3556. `FROM stock_balances
sb ... LEFT JOIN posted_sales ps` where `posted_sales` aggregates `stock_allocations`
joined to posted cash/credit sale `business_documents`.

Evidence:
- Primary projection is vacuous because `stock_balances` has 0 rows for this tenant
  (shared finding above) — the query's `FROM` clause starts there, so no `stock_balances`
  row means no output row regardless of sales activity.
- The sale-side cross-reference specifically (`stock_allocations` against
  `stock_ledger` OUT entries, as instructed) is **separately unverifiable**: the
  sandbox tenant has **0** `stock_allocations` rows despite 291,361 posted cash/credit
  sale documents and 623,096 `stock_ledger` `out` rows. For comparison, other
  (apparently live-posted/test) tenants in the same database have `stock_allocations`
  row counts in the single digits, consistent with per-transaction allocation writes.
  `stock_allocations` is populated by the live posting handlers
  (`services/api/internal/httpapi/business.go`/`stock.go`), not by whatever process
  bulk-loaded the sandbox `stock_ledger` rows, so neither side of the intended
  reconciliation (`stock_allocations` sale-side consumption vs. `stock_ledger` OUT)
  has any linked data in this environment to actually cross-check.

Verdict: **VACUOUS-NO-DATA** for the primary balance projection; the specific
sale/ledger cross-reference this leaf's task description called out is additionally
**UNVERIFIABLE-THIS-PASS** — there is no `stock_allocations` data anywhere in the
sandbox tenant to exercise it, in either direction.

### 5. `item-activity` (mode `movement`) — MATCHED

Same registry function call and identical generated SQL as `stock-register` (#1) —
`reports.go`'s `phasePReportRegistry` maps both kinds to `stockMode: "movement"`,
and `stockReadModelQuery` dispatches purely on `mode`, not `kind`. All evidence from
#1 applies verbatim (1091/1091 rows for `2026-06-01`, correct sign convention, no
join-drop). **MATCHED.**

### 6. `stock-register-narcotics-format2` (mode `narcotics-movement`) — VACUOUS-NO-DATA

Same registry mode and identical generated SQL as `stock-register-for-narcotics`
(#3). Same root cause (no narcotics-flag payload key in the migrated dataset), same
0-row result over the full date range. **VACUOUS-NO-DATA.**

### 7. `expiry-report` (mode `expiry`) — VACUOUS-NO-DATA

Query: `reports.go` lines 3992-4026 (the `expiryFilter = "expiry_date IS NOT NULL
AND expiry_date BETWEEN $3::date AND $4::date"` branch), `FROM stock_balances sb
JOIN stock_batches b ...`.

Evidence: ran the exact query over the full open date range — **0 rows**. Confirmed
this is purely the `stock_balances` cache gap and not, e.g., a genuinely empty
expiry universe: `stock_batches.expiry_date IS NOT NULL` for **all** 10,507 sandbox
batches, and the read-only balance reconstruction described in the shared findings
shows 7,312 of those batches would carry non-zero `on_hand` if the cache were
rebuilt — i.e. there is substantial real expiry-eligible stock that this leaf simply
cannot see in the current environment. **VACUOUS-NO-DATA**, same root cause as #4/#11.

### 8. `expiry-report-class-wise` (mode `expiry-class`) — VACUOUS-NO-DATA

Query: `stockExpiryClassReadModelQuery`, `reports.go` lines 3699-3743. `FROM
stock_balances sb JOIN stock_batches b ...`, classified by
`COALESCE(payload->>'Class', payload->>'ItemClass', payload->>'class', 'Unspecified')`.

Evidence: same `stock_balances`-emptiness root cause as #7 (0 rows over the full
date range). Independently, even if `stock_balances` were populated, this leaf has a
**second** blocking gap: none of `Class`/`ItemClass`/`class` exists in any sandbox
item's payload (same key enumeration as the narcotics finding), so every row would
classify as `'Unspecified'` — the class-wise grouping itself has no discriminating
data to group by in this dataset. **VACUOUS-NO-DATA.**

### 9. `daily-stock-in-out` (mode `movement-summary`) — MATCHED

Query: `stockMovementSummaryReadModelQuery`, `reports.go` lines 3392-3443. Groups
`stock_ledger` by `(movement_date, movement_direction, godown, item_name)`.

Evidence: for `2026-06-01`, the report's exact query grouped into 318 `IN` groups
(qty `9951.0000`, value `5498416.5647`) and 530 `OUT` groups (qty `-6403.0000`,
value `-223314.4283`). These direction totals reconcile exactly to the raw,
ungrouped ledger totals for the day established independently and non-CTE-based
earlier (`sum_in=9951.0000`, `sum_out=6403.0000`, matching sign-adjusted). **MATCHED.**

### 10. `stock-in-out-date-wise` (mode `movement-summary`) — MATCHED

Same registry mode and identical generated SQL as `daily-stock-in-out` (#9) — only
`kind`/`title` differ in the registry entry. All evidence from #9 applies verbatim.
**MATCHED.**

### 11. `stock-management-report` (mode `management-summary`) — VACUOUS-NO-DATA

Query: `stockManagementReadModelQuery`, `reports.go` lines 3562-3618. `FROM
stock_balances sb JOIN stock_batches b ...` with `ReorderQty`/`OptimumQty`/
`MinimumQty` payload thresholds.

Evidence: ran the exact query over the full open date range — **0 rows**. Same
`stock_balances` cache-emptiness root cause as #7/#8. (Threshold payload keys
`ReorderQty`/`OptimumQty`/`MinimumQty` **do** exist in the migrated payload — unlike
`Narcotics`/`Class`/`GenericName` — so this leaf's grouping logic itself has no
second blocking gap beyond the cache; it would very likely produce real output the
moment `stock_balances` is rebuilt for this tenant.) **VACUOUS-NO-DATA.**

### 12. `norcotics-stock-register-generic-type-wise` (mode `narcotics-generic`) — VACUOUS-NO-DATA

Query: `stockNarcoticsReadModelQuery("narcotics-generic", ...)`, `reports.go` lines
3796-3865. Same narcotics-flag `WHERE` predicate as #3/#6, additionally grouped by
`COALESCE(payload->>'GenericName', payload->>'GenericCode', payload->>'genericName',
'Unspecified')`.

Evidence: ran the exact CTE over the full open date range — **0 rows**, for the same
root cause as #3/#6 (no narcotics-flag payload key present). Independently, the
`GenericName`/`GenericCode`/`genericName` keys are also entirely absent from the
migrated payload (same enumeration), so even a hypothetically-narcotics-flagged item
would still classify as `'Unspecified'` generic type. **VACUOUS-NO-DATA.**

---

## Cross-cutting findings worth flagging beyond this leaf set

These are data/environment gaps discovered while verifying the assigned 12 leaves,
not `reports.go` logic defects (the SQL in every case reads exactly the table(s) its
own `stockProjectionNote` says it reads, and reconciles exactly wherever data
exists). Documenting precisely per the assignment's "genuine bug" instruction, with
the caveat that the most likely root cause is the sandbox data-loading pipeline
rather than the report code:

1. **`stock_balances` is empty for the sandbox tenant/branch** despite 781,203
   `stock_ledger` rows and 10,507 `stock_batches` rows, because `stock_balances` is
   only maintained by the live posting handlers in `stock.go` (`INSERT ... ON
   CONFLICT DO UPDATE`), not by whatever process bulk-loaded the sandbox
   `stock_ledger`/`stock_batches` rows. This affects at minimum the 4 leaves above
   (`stock-and-sales`, `expiry-report`, `expiry-report-class-wise`,
   `stock-management-report`) and very likely several more Phase P leaves outside
   this assignment that also read `FROM stock_balances` (e.g. the `stock-in-hand-*`
   balance/classification leaves, `reorder-level-report`/`optimum-level-report`/
   `minimum-level-report`/`reorder-optimum-level-report`, and
   `stock-in-hand-supplier-manufacturer-association`) — worth a coordinated
   follow-up (e.g. running the existing `rebuildStockBalances` maintenance endpoint,
   `services/api/internal/httpapi/stock.go` lines 1235-1300, against the sandbox
   tenant) so the remaining Phase P golden-verification passes aren't all blocked by
   the same gap. **Not done in this pass**: this pass only ran read-only `SELECT`s
   against the shared sandbox database to avoid mutating state other concurrent
   agents may depend on.
2. **`stock_allocations` is empty for the sandbox tenant/branch** despite 291,361
   posted cash/credit sale documents and 623,096 `stock_ledger` OUT rows, for the
   analogous reason (allocation rows are written by the live sale-posting path, not
   the bulk loader). Affects `stock-and-sales` here, and any other leaf/feature that
   cross-references `stock_allocations` for this tenant.
3. **`master_items.payload` for the migrated sandbox dataset never captured
   narcotics, generic-name, or item-class fields** — only 37 keys exist across all
   30,052 items, all clearly plain Item-master pricing/tax/reorder fields
   (`SalePrice`, `PurPrice`, `ReorderQty`, `GSTPerc1`, etc.), with no
   `Narcotics`/`GenericName`/`Class` equivalents. This affects 4 of the 12 leaves
   here (`stock-register-for-narcotics`, `stock-register-narcotics-format2`,
   `norcotics-stock-register-generic-type-wise` for narcotics;
   `expiry-report-class-wise` for class) and would also block golden verification of
   the sibling `stock-in-hand-class-wise` / `stock-in-hand-manufacturer-wise` /
   `stock-in-hand-category-wise` leaves for narcotics/class specifically (their
   `Manufacturer`/`ManfCode`/`Category`/`ICatCode` keys **do** exist in this payload,
   so those particular classification dimensions are unaffected — only
   `Narcotics`/`Class`/`GenericName` are missing).

## Test coverage added

`services/api/internal/httpapi/report_golden_stock_movement_test.go` (new, offline —
no DB dependency) locks in:
- Registry wiring for all 12 kinds → correct `stockMode`/`title`
  (`TestPhaseGoldenStockMovementLeavesResolveToExpectedStockMode`).
- That the 5 ledger-native modes (`movement`, `item-summary`, `movement-summary`,
  `narcotics-movement`, `narcotics-generic`) never read `FROM stock_balances`, so
  they stay immune to the cache-staleness gap found above
  (`TestStockLedgerNativeModesNeverReadTheBalanceCache`).
- That the 4 balance-cache-dependent modes (`expiry`, `expiry-class`, `stock-sales`,
  `management-summary`) do read `FROM stock_balances` (and `stock-sales` also
  references `stock_allocations`), documenting the dependency this pass found to be
  a data-population gap in the sandbox
  (`TestStockBalanceCacheDependentModesReadFromStockBalances`).
- The OUT-negation / adjustment-sign convention for the ledger-native modes
  (`TestStockMovementSignedQuantityConvention`), matching the sign convention
  independently verified against raw `stock_ledger` rows above.

Verification commands run for this pass:
```
cd services/api && go vet ./...
cd services/api && go test -run 'TestPhaseGoldenStockMovementLeavesResolveToExpectedStockMode|TestStockLedgerNativeModesNeverReadTheBalanceCache|TestStockBalanceCacheDependentModesReadFromStockBalances|TestStockMovementSignedQuantityConvention' ./internal/httpapi/...
```
Both passed clean (`go vet`: no output; `go test`: 4/4 PASS).

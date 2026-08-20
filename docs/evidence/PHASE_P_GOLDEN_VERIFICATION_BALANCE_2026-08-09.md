# Phase P golden verification — Stock In Hand (balance family) + level reports — 2026-08-09

## Scope and assignment

This is one of ten parallel Phase N–Q golden-verification passes. This pass covers the
15 `stockReadModel` leaves in `services/api/internal/httpapi/reports.go`'s
`phasePReportRegistry` that use the "balance"-shaped stock modes:

| # | Kind | Registered mode |
|---|---|---|
| 1 | `stock-in-hand-manufacturer-wise` | `manufacturer-balance` |
| 2 | `stock-in-hand-category-wise` | `category-balance` |
| 3 | `stock-in-hand-others` | `balance` |
| 4 | `stock-in-hand-class-wise` | `class-balance` |
| 5 | `stock-in-hand-batch-priority-wise` | `balance` |
| 6 | `stock-in-hand-back-date` | `historical-balance` |
| 7 | `stock-in-hand-manufacturer-wise-format2` | `manufacturer-balance` |
| 8 | `stock-in-hand-supplier-manufacturer-association` | `supplier-manufacturer` |
| 9 | `stock-in-hand-stock-quantity-format` | `balance` |
| 10 | `stock-in-hand-stock-in-hand-audit-purpose` | `balance` |
| 11 | `stock-in-hand-batch-priority-wise-audit-purposes` | `balance` |
| 12 | `reorder-level-report` | `reorder-level` |
| 13 | `minimum-level-report` | `minimum-level` |
| 14 | `optimum-level-report` | `optimum-level` |
| 15 | `reorder-optimum-level-report` | `reorder-optimum-level` |

`reports.go` was read-only for this pass (per assignment). No fix was applied to it;
confirmed defects are documented below with exact function/line and root cause, as
instructed.

## Environment constraint

Live legacy SQL Server access was not reachable (Windows trusted-auth fails in this
environment) and was not attempted. All verification is Postgres-internal
cross-verification against the sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`
(code `legacy-reference-sandbox`, branch `ffffffff-ffff-ffff-ffff-ffffffffffff`), per the
dispatcher's instructions, plus a disposable-fixture Go integration test for the parts
that the sandbox tenant's own data cannot exercise (see Root cause A and B below).

`psql` connects locally as the `postgres` superuser, which bypasses row-level security
(`rolsuper=true`, confirmed via `pg_roles`). All queries below still add explicit
`tenant_id`/`branch_id` predicates so results are never dependent on that bypass.

## Headline result

**1 of 15 leaves is genuinely golden-verified (MATCHED). 14 of 15 are blocked by four
compounding, precisely-identified root causes — one of which is a confirmed reports.go
defect that makes 5 leaves return HTTP 503 on every single request, independent of any
data.** None of the 14 blocked leaves could be exercised end-to-end against real migrated
data as originally intended; each blocker is demonstrated with direct evidence below
rather than asserted.

## Root causes (read this section first — every per-leaf verdict below cites these)

### Root cause A — confirmed reports.go bug: ambiguous `updated_at` crashes every "balance" mode query (leaves #3, #5, #9, #10, #11)

`stockReadModelQuery` in `services/api/internal/httpapi/reports.go` (~line 3983–4027)
builds the default ("balance"/"valuation") branch's date filter as a **bare, unqualified**
column reference:

```go
// reports.go ~line 3988
expiryFilter := "updated_at >= $3::date AND updated_at < ($4::date + INTERVAL '1 day')"
```

but the query's `FROM`/`JOIN` list in the same function is:

```sql
FROM stock_balances sb
JOIN stock_batches b   ON ...
LEFT JOIN master_godowns g ON ...
LEFT JOIN master_items i   ON ...
```

`stock_balances` (`sb`), `master_godowns` (`g`), and `master_items` (`i`) **each have their
own `updated_at` column** (confirmed via `\d stock_balances`, `\d master_godowns`,
`\d master_items`). An unqualified `updated_at` is therefore ambiguous, and PostgreSQL
rejects the query at parse/analysis time — **before any row is evaluated**, so this fires
regardless of tenant, data volume, or whether `stock_balances` has any rows at all.

Reproduced two independent ways:

1. **Direct `psql` against the real sandbox tenant**, replaying the exact query text with
   real bind parameters via `PREPARE`/`EXECUTE` (so the driver's parameter-binding
   behavior is faithfully reproduced, not just literal-SQL constant folding):

   ```
   ERROR:  column reference "updated_at" is ambiguous
   LINE 19:   AND updated_at >= $3 AND updated_at < ($4 + INTERVAL '1 da...
   ```

2. **A new Go integration test**, `services/api/internal/httpapi/report_golden_stock_balance_test.go`
   (`TestStockBalanceReportGoldenReconciliation`), which seeds a disposable fixture tenant
   with real `stock_ledger` IN/OUT/ADJUSTMENT rows across 4 batches, drives the cache
   through the actual production rebuild endpoint (`POST /v1/inventory/rebuild`,
   `(*Server).rebuildStockBalances`), confirms the rebuilt `stock_balances` cache exactly
   matches the hand-computed net for every batch, and then calls the real
   `stock-in-hand-others` report endpoint end-to-end. It receives `503 report_read_failed`
   every time — the same ambiguous-column error, now proven to occur even with a
   correctly-populated, hand-verified cache (i.e. this is not a symptom of Root cause B
   below; it is an independent, structural defect).

`stockReadModelQuery`'s special-cased branches (`isStockClassificationMode` — leaves #1/#2/#4/#7,
`isStockSupplierManufacturerMode` — leaf #8, `isStockLevelMode`/`isStockManagementMode` —
leaves #12–15) all correctly write `sb.updated_at` (qualified) and were confirmed, by
direct `psql` execution against the real sandbox tenant, to **not** hit this bug — their
own queries execute without error. This bug is isolated precisely to the shared default
branch used by modes `"balance"` (5 registered leaves, all in this assignment) and
`"valuation"` (defined but currently unregistered/unreachable — dead code today, but would
inherit the same bug if wired up later).

**Impact: `stock-in-hand-others`, `stock-in-hand-batch-priority-wise`,
`stock-in-hand-stock-quantity-format`, `stock-in-hand-stock-in-hand-audit-purpose`, and
`stock-in-hand-batch-priority-wise-audit-purposes` return HTTP 503 on every request in
production today, for every tenant, regardless of data.** This is the single highest-severity
finding of this pass: these five report leaves are not merely unverified against legacy —
they cannot render at all.

### Root cause B — `stock_balances` cache is empty for the sandbox tenant (leaves #1, #2, #4, #8, #12–15, and would also affect #3/#5/#9/#10/#11 once Root cause A is fixed)

```
stock_batches   (tenant=sandbox): 10,507 rows
stock_ledger    (tenant=sandbox): 781,203 rows, 100% status='posted'
stock_balances  (tenant=sandbox): 0 rows
```

Confirmed twice: once via an RLS-scoped, tenant-filtered count, and once via a
`GROUP BY tenant_id` scan of the entire (492-tenant) `stock_balances` table as superuser —
the sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` never appears. Every other tenant
in the table is a 1–60 row Playwright/integration-test fixture; none carry real migrated
volume.

Independently reconstructing what the cache *should* contain, by mirroring
`rebuildStockBalances`'s own aggregation SQL (`stock.go` ~line 1264–1278: `SUM(CASE
direction ...)` grouped per batch) directly against `stock_batches`/`stock_ledger`:

```
total_batches: 10,507
nonzero_batches: 7,312   (positive: 6,832, negative: 480)
grand_total_on_hand: 221,225.0000 units
```

So the sandbox tenant has substantial real stock — the cache was simply never populated.
Root cause: the migration pipeline that bulk-loads `stock_ledger`/`stock_batches` for this
tenant does not appear to invoke `POST /v1/inventory/rebuild` (or the equivalent SQL)
afterward; live-transaction posting (`business.go`, `stock.go`) maintains
`stock_balances` incrementally per document, but that code path is never exercised by a
bulk COPY-style import. This matches `docs/REMAINING_WORK_100_PERCENT_PARITY_2026-08-09.md`
Phase J's independent finding that "No recomputation job reconciling against legacy's
3.2M-row `StockReport`... exists."

Every `stockReadModel` "balance"-family query requires either reading `stock_balances`
directly or an `EXISTS` against a posted `stock_ledger` row **for a batch present in
`stock_balances`** — so with the cache empty, every one of these leaves returns zero rows
against the sandbox tenant's real data today, independent of Root cause A. This was
confirmed by running each mode's exact query (via `psql` `PREPARE`/`EXECUTE` with real bind
parameters) directly against the sandbox tenant over a wide `2000-01-01`–`2100-01-01`
window: `manufacturer-balance`, `supplier-manufacturer` (both 0 rows, no error — these two
modes are not affected by Root cause A, so this cleanly isolates Root cause B).

This is not a `reports.go` code defect — the SQL is correct given a populated cache (the Go
test above proves the pipeline arithmetic is right end-to-end once the cache exists) — it
is an operational/migration gap that leaves the "balance" family non-functional against
the one tenant meant to carry real legacy-verification data.

### Root cause C — reorder/optimum/minimum thresholds are absent or always zero in the migrated payload (leaves #12, #13, #14, #15)

Per-leaf instruction: verify the threshold values come from the correct `master_items.payload`
field. Checked against `migration/maps/phase-e-core-masters.json`'s `dbo.Item →
master_records`/`master_items` column mapping (the authoritative source-to-payload
mapping used for this migration):

- `ReorderQty` and `OptimumQty` **are** explicitly mapped (`"ReorderQty": "ReorderQty"`,
  `"OptimumQty": "OptimumQty"`), confirming `reports.go`'s primary key reads
  (`i.payload->>'ReorderQty'`, `i.payload->>'OptimumQty'`, in `stockLevelReadModelQuery`
  and `stockManagementReadModelQuery`) target the **correct**, correctly-migrated field
  names. The code is right.
- There is **no** `MinimumQty`, `MinimumQuantity`, `MinLevel`, `MinStock`, or any
  minimum-level-shaped column anywhere in the Item mapping, and a repo-wide grep for
  `Minimum` across every file in `migration/maps/` returns nothing. The legacy `dbo.Item`
  source table, as captured for this reference migration, does not carry a minimum-level
  field at all. `reports.go`'s `MinimumQty`/`MinimumQuantity` fallback pair is reading for
  a field that was never migrated — not necessarily a code bug (the existing synthetic unit
  test `TestStockLevelReportsUseMaintenanceThresholdFallbacks` in
  `historical_integration_test.go` proves the fallback-chain code itself works correctly
  when the field is present), but the source data to verify or exercise it against does not
  exist in this migration surface.

Separately, checking the actual payload values for all 30,052 items in the sandbox tenant:

```
items with ReorderQty > 0: 0
items with OptimumQty > 0: 0
```

**Not a single item in the entire migrated dataset has a positive reorder or optimum
quantity.** `stockLevelReadModelQuery`'s predicate (`reorder_quantity > 0 AND on_hand <=
reorder_quantity`, similarly for optimum/reorder-optimum) can therefore never match a row
for this tenant — confirmed by directly evaluating the predicate against the
independently-reconstructed on-hand values from Root cause B (i.e. even hypothetically
with a populated cache, 0 of 30,052 items would ever be flagged). This may reflect the
legacy production data genuinely never using reorder/optimum alerts, or a migration gap
upstream of this reference dataset — either way, this specific sandbox dataset cannot
positively exercise the "below-threshold" branch of any level report, only its
(trivially-true) empty-result path.

### Root cause D — manufacturer/category grouping uses raw legacy codes, not resolved names; class has no migrated source at all (leaves #1, #2, #4, #7, and the manufacturer label in #8)

`stockClassificationReadModelQuery` (manufacturer-balance/category-balance/class-balance)
groups by:

```sql
COALESCE(NULLIF(i.payload->>'Manufacturer', ''), NULLIF(i.payload->>'ManfCode', ''), NULLIF(i.payload->>'manufacturer', ''), 'Unspecified')
```

`master_items.payload` for this tenant has **no** `Manufacturer`/`Category`/`Class` keys at
all (confirmed by enumerating every distinct JSON key across all 30,052 items: `ManfCode`
and `ICatCode` are present; `Manufacturer`, `Category`, `Class`, `ItemClass` are not) — so
the fallback chain lands on the raw internal code (`ManfCode`/`ICatCode`), not a
human-readable name. Demonstrated concretely for 5 real items:

```
manf_code=291  → resolved manufacturer_name = "SURGICAL GOODS."
cat_code=1     → resolved category_name     = "MEDICINES"
```

via a join to `master_records` (`kind='manufacturer'`, `kind='item_category'`), which the
same migration **does** populate for this tenant (838 manufacturer rows, 7 item-category
rows, both fully resolvable — `dbo.Manufacturer`/`dbo.ItemCategory` are mapped in
`migration/maps/phase-e-core-masters.json` and `phase-e-enterprise-config.json`
respectively). `stockClassificationReadModelQuery` does not join `master_records`, so it
groups/labels by the internal code instead of the name the legacy PowerBuilder report
almost certainly printed. This is a genuine projection-completeness gap, not a crash — the
report would render, just with codes instead of names, once Root cause B is also resolved.

For **class-balance** (leaf #4) specifically: there is no `Class`/`ItemClass`/`class` field
in the payload, and a repo-wide search of every `migration/maps/*.json` file for anything
matching `class` (case-insensitive, excluding the unrelated Phase R security maps) returns
**zero** source tables or columns. Every row therefore falls to `'Unspecified'` — not a
wrong join, there is simply no captured legacy concept to verify grouping against in this
migration surface at all.

## Per-leaf verdicts

### 1. `stock-in-hand-manufacturer-wise` (manufacturer-balance) — MISMATCH-DOCUMENTED
Query executes without error (confirmed, not affected by Root cause A). Blocked by Root
cause B (0 rows against real data; cache empty) and Root cause D (would group by raw
`ManfCode` like `"291"` instead of the resolved name `"SURGICAL GOODS."` once B is fixed).

### 2. `stock-in-hand-category-wise` (category-balance) — MISMATCH-DOCUMENTED
Same as #1, substituting `ICatCode` (code `"1"` → resolved name `"MEDICINES"`).

### 3. `stock-in-hand-others` (balance) — MISMATCH-DOCUMENTED
**Root cause A**: confirmed HTTP 503 on every request, reproduced via direct `psql` and via
a passing Go regression test (`TestStockBalanceReportGoldenReconciliation`) that pins this
exact failure with a populated, hand-verified cache. This is the representative leaf for
the "balance" mode bug.

### 4. `stock-in-hand-class-wise` (class-balance) — VACUOUS-NO-DATA
Query executes without error. Blocked by Root cause B (empty cache) and, independently,
Root cause D (no `Class` concept exists anywhere in the migrated source — grouping cannot
be verified as correct or incorrect against this dataset; not a code defect, just nothing
to check).

### 5. `stock-in-hand-batch-priority-wise` (balance) — MISMATCH-DOCUMENTED
Identical query shape to #3 (same `stockReadModelQuery` default branch). Root cause A
applies identically: HTTP 503 on every request.

### 6. `stock-in-hand-back-date` (historical-balance) — MATCHED
The only cleanly verified leaf in this assignment. `historicalStockReadModelQuery` is a
direct, unaggregated projection of `historical_stock_snapshots` (imported from legacy
`dbo.StockReport`; 3,215,967 rows for this tenant, 2025-01-01–2026-07-31). Ran the report's
exact SQL (with real bind parameters) for two independently sampled item/date pairs:

| Sample | as_of | item | quantity | purchase_price | sale_price | average_price | recent_purchase_price | pack_units |
|---|---|---|---|---|---|---|---|---|
| item 1633, GODOWN1 | 2026-07-31 | VITASYN TAB | 23.0000 | 552.5000 | 650.0000 | 18.2380 | 552.5000 | 30 |
| item 10004, GODOWN1 | 2025-12-01 | VIGURINE 120ML SYP | 4.0000 | 140.0000 | 180.0000 | 140.0000 | 140.0000 | 1 |

Both matched the raw `historical_stock_snapshots` row **exactly**, field for field, item
name resolved correctly via `master_items`, godown resolved correctly via
`master_godowns`. This confirms and extends
`docs/evidence/PHASE_P_HISTORICAL_STOCK_BACK_DATE_EVIDENCE_2026-08-07.md`'s existing test-suite
evidence with direct row-level proof against real data. Disclosed boundary (matches the
prior evidence doc): manufacturer/category/class/narcotics grouping and exact print
calculations remain out of scope for this source-row projection — it was never meant to
group, only to expose the captured snapshot fields.

### 7. `stock-in-hand-manufacturer-wise-format2` (manufacturer-balance) — MISMATCH-DOCUMENTED
Same query/mode as #1 (only the title differs in the registry); same verdict and reasons.

### 8. `stock-in-hand-supplier-manufacturer-association` (supplier-manufacturer) — MISMATCH-DOCUMENTED
Query executes without error (not affected by Root cause A). Blocked by Root cause B (0
rows against real data). The supplier-name resolution itself is verified correct and
complete: all 22,245 `item_suppliers` rows (21,252 distinct items) for this tenant resolve
100% to a `master_parties` name via the report's `LEFT JOIN LATERAL`. The manufacturer
label shares Root cause D's raw-code issue.

### 9. `stock-in-hand-stock-quantity-format` (balance) — MISMATCH-DOCUMENTED
Identical query shape to #3. Root cause A applies identically: HTTP 503 on every request.

### 10. `stock-in-hand-stock-in-hand-audit-purpose` (balance) — MISMATCH-DOCUMENTED
Identical query shape to #3. Root cause A applies identically: HTTP 503 on every request.

### 11. `stock-in-hand-batch-priority-wise-audit-purposes` (balance) — MISMATCH-DOCUMENTED
Identical query shape to #3. Root cause A applies identically: HTTP 503 on every request.

### 12. `reorder-level-report` (reorder-level) — MISMATCH-DOCUMENTED
Query executes without error; field mapping verified **correct**
(`i.payload->>'ReorderQty'`, matches the migration map). Blocked by Root cause B (empty
cache → 0 rows) and, independently, Root cause C (0 of 30,052 items have a positive
`ReorderQty`, so the below-threshold predicate could never fire against this dataset even
with a populated cache).

### 13. `minimum-level-report` (minimum-level) — VACUOUS-NO-DATA
Field mapping logic is exercised and passes in the existing synthetic unit test
(`TestStockLevelReportsUseMaintenanceThresholdFallbacks`), so the code path itself is not
in question. But per Root cause C, no `MinimumQty`/`MinimumQuantity`-shaped column was ever
migrated for `dbo.Item` in this reference dataset — there is no legacy source value to
verify the leaf's threshold against. Also blocked by Root cause B.

### 14. `optimum-level-report` (optimum-level) — MISMATCH-DOCUMENTED
Same pattern as #12 with `OptimumQty` (also correctly mapped, also always 0 for all 30,052
items). Blocked by Root cause B and Root cause C.

### 15. `reorder-optimum-level-report` (reorder-optimum-level) — MISMATCH-DOCUMENTED
Combines #12 and #14's predicate (`reorder_quantity > 0 OR optimum_quantity > 0`); both
source fields are correctly mapped and both are always 0 for every item in this tenant.
Blocked by Root cause B and Root cause C.

## Verdict summary

| Verdict | Count | Leaves |
|---|---|---|
| MATCHED | 1 | #6 |
| MISMATCH-DOCUMENTED | 12 | #1, #2, #3, #5, #7, #8, #9, #10, #11, #12, #14, #15 |
| VACUOUS-NO-DATA | 2 | #4, #13 |
| UNVERIFIABLE-THIS-PASS | 0 | — |

## New test added

`services/api/internal/httpapi/report_golden_stock_balance_test.go` —
`TestStockBalanceReportGoldenReconciliation`. Seeds a disposable fixture tenant with 4
batches (2 items × 2 godowns) and hand-picked `stock_ledger` IN/OUT/ADJUSTMENT rows
(net 65, 50, 0, 18), drives the cache through the real `POST /v1/inventory/rebuild`
handler, independently confirms the rebuilt cache matches the hand-computed net for every
batch (including the net-zero batch, which the rebuild still writes a row for — there is
no `HAVING` filter in `rebuildStockBalances`), then calls the live
`stock-in-hand-others` report endpoint and pins the confirmed Root cause A failure
(`503 report_read_failed`) with an explanatory comment pointing at the exact `reports.go`
line, so the test starts failing (correctly) the moment someone fixes the ambiguous
column — a clear signal to update the assertion to the golden row values at that point.

## Verification commands run

```
cd services/api && go vet ./...
# (repo root) go vet ./services/api/...
   → clean, no output, both invocations

cd services/api && DATABASE_URL="postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable" \
  go test -run 'TestStockBalanceReportGoldenReconciliation' ./internal/httpapi/ -v -count=1
   → --- PASS: TestStockBalanceReportGoldenReconciliation (0.37s)
```

Note on `DATABASE_URL`: the app-role connection string documented in `.env.example`
(`abuzar_app_local`) does not have `BYPASSRLS` and cannot `INSERT INTO tenants` outside an
`app.authenticating='true'` transaction, which the shared test fixture helper
(`seedStockTenant`, used by every sibling `*_integration_test.go` in this package) does not
set — confirmed by reproducing `ERROR: new row violates row-level security policy for
table "tenants"` under that role. The `postgres` superuser connection was used instead, all
integration tests in this package expect it as the local baseline connection for the disposable-fixture
pattern.

## What remains open after this pass

- **Fix Root cause A** (qualify `sb.updated_at` in `stockReadModelQuery`'s default branch,
  `reports.go` ~line 3988) — out of scope for this read-only pass, but this is the single
  highest-priority, lowest-effort fix in the whole Phase P assignment: a one-line change
  unblocks 5 of 15 leaves from a hard crash.
- **Rebuild `stock_balances` for the sandbox tenant** (Root cause B) — needed before any of
  the remaining balance/level leaves can be exercised against real data at all. Out of
  scope for this pass (data-mutation on a tenant shared with 9 concurrent agents was
  deliberately avoided; see methodology note below).
- **Decide the manufacturer/category name-resolution gap** (Root cause D) — join
  `master_records` instead of exposing raw legacy codes, or confirm the legacy report
  genuinely printed codes.
- **Decide the class-wise and minimum-level source gap** (Root causes C/D) — confirm
  whether `dbo.Item` truly never had these fields, or whether a different legacy table
  (not yet in `migration/maps/`) was the real source and needs to be added.
- Full-volume p95 performance, exact PowerBuilder print/PDF formatting, and legacy
  cross-database numeric replay remain out of scope for this pass, consistent with every
  other Phase N–Q evidence doc.

## Methodology note: why `stock_balances` was not rebuilt for the sandbox tenant during this pass

`POST /v1/inventory/rebuild` was deliberately **not** invoked against the shared sandbox
tenant. It is a write operation (`DELETE` + bulk `INSERT` on `stock_balances` for the whole
tenant/branch) on data shared live with 9 other concurrent verification agents, several of
which may depend on the current row-for-row state of tenant `eeeeeeee-…` for their own
reconciliation. Root cause B is instead demonstrated with read-only reconstruction
(mirroring the rebuild's own SQL) and confirmed empty-cache row counts, and Root cause A's
pipeline-correctness claim is proven in isolation on a disposable, single-test fixture
tenant instead.

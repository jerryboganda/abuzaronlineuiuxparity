# Phase Q Golden Verification — Tax & Admin/Listing Leaves (2026-08-09)

## Scope and methodology

This document covers 13 assigned report leaves: 6 tax leaves (financeMode via
`phaseQFinancialOverrides`, `services/api/internal/httpapi/reports.go` lines
561–571 and base N/O registry entries at lines 221–278/347–361) and 7
admin/listing leaves (adminKind, `reports.go` lines 486–500, function
`reportAdminColumns` at line 1184, query function `adminReadModelQuery` at
line 4567).

**Environment constraint confirmed:** live legacy SQL Server access is not
reachable from this environment (Windows trusted-auth fails with "Login
failed. The login is from an untrusted domain"). All verification below is
Postgres-internal cross-verification against the migrated
`legacy-reference-sandbox` tenant (`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`,
branch `ffffffff-ffff-ffff-ffff-ffffffffffff`), per the methodology specified
for this task: for each leaf, read the query logic in `reports.go`, then
independently write a separate `psql` query against the underlying migrated
tables that computes the expected result a different way, and compare.

Session setup used for every query below:
```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', 'ffffffff-ffff-ffff-ffff-ffffffffffff', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```
(`app.branch_id` had to be set to the sandbox branch UUID, not `''`, because
`item_tax_assignments`, `party_tax_assignments`, and `tax_rates` carry a
RESTRICTIVE branch-hardening RLS policy that returns zero rows when
`app.branch_id` is blank.)

Data volumes in the sandbox tenant (posted documents only): 291,359
`cash-sale`, 30,704 `cash-sale-return`, 2 `credit-sale`, 6,395 `pack-purchase`,
22 `loose-purchase`, 1 `opening-purchase`, 634 `purchase-return`. Date range
2025-01-01 to 2026-07-31.

A companion Go test file, `services/api/internal/httpapi/report_golden_tax_admin_test.go`,
locks the registry wiring for all 13 leaves and characterizes (without
attempting to fix) the three confirmed bugs found below, as a regression
guard. `go vet ./services/api/...` is clean; the targeted test
(`go test -run <name> ./services/api/internal/httpapi/`) passes.

---

## Tax leaves

### 1. `customer-sales-customer-wise-advance-tax` (financeMode `tax-advance`)

**Verdict: VACUOUS-NO-DATA (legitimate — not a code bug)**

Query: `financeReadModelQuery` (reports.go lines 4391–4439), tax-advance
branch, `kindFilter = 'cash-sale', 'credit-sale'`, requires
`advance_tax.amount > 0 AND l.advance_tax_rate > 0`.

Independent check: ran the exact query shape directly in psql for the full
tenant/date range → **0 rows**. Cross-checked independently by two other
routes:
- `business_document_lines.advance_tax_rate > 0` for lines on posted
  `cash-sale`/`credit-sale` documents: **0 rows** (vs. 87,028 rows for
  purchase-kind documents in the same tenant).
- `business_documents.legacy_payload->>'AdvanceTaxAmt'` for all 291,361
  posted sale documents: **0 documents** have a non-zero value, and 0 have
  `legacy_payload->>'AdvanceTax' = 'Y'`. Sample row: `{"AdvanceTax": "N",
  "AdvanceTaxAmt": "0.00", ...}`.

Conclusion: the legacy data genuinely records no advance tax on sales for
this tenant (advance tax was only withheld on purchases in this dataset).
The report's zero-row output is a correct reflection of the source, not a
projection defect.

### 2. `supplier-wise-advance-income-tax` (financeMode `tax-advance-input`)

**Verdict: MISMATCH-DOCUMENTED — confirmed new bug**

Query: same tax-advance/tax-advance-input branch, `kindFilter =
'pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return'`.

Running the exact query for the full tenant/date range returns **3,643
rows**, `SUM(amount) = 685,142.16`, `SUM(base) = 112,997,847.56`.

**Bug found.** The LATERAL subquery's fallback amount source
(`d.legacy_payload->>'AdvanceTaxAmt'`, an invoice-level field) is gated to
fire only on the row where `l.line_number = MIN(line_number)` for the
document (reports.go lines 4405–4413). The *outer* `WHERE` clause
additionally requires `l.advance_tax_rate > 0` **on that same physical row**
(line 4432). `advance_tax_rate` is a per-line field independently populated
per item; the item that happens to sit first on the invoice (by
`line_number`) is not necessarily the item carrying the advance-tax rate.

Independent verification:
- Documents with ≥1 line where `advance_tax_rate > 0`: **3,744** (of which
  3,737 also have a valid non-zero numeric `AdvanceTaxAmt`, 1 has
  `AdvanceTaxAmt = 0`, and 6 have a missing/non-numeric `AdvanceTaxAmt`).
- Of those 3,737 documents with a valid non-zero `AdvanceTaxAmt`, **94** have
  a `MIN(line_number)` row whose own `advance_tax_rate = 0` (i.e., the
  advance-tax-bearing item is not first on the invoice).
- `3,737 − 94 = 3,643`, matching the report's actual output row count
  exactly — confirming these 94 documents are silently dropped from the
  report with **zero** representation (not merely wrong values).
- Total missed advance tax: **PKR 9,589.64** across the 94 documents.

Concrete repro: document_number `2864` (PKR 1,574.17 missed). Its
`business_document_lines`, ordered by `line_number`:
```
line_number=120981  STREPSILS HONEY LEMON   advance_tax_rate=0.0000
line_number=120982  SANIPLAST 100S          advance_tax_rate=0.5000
line_number=120983  SANIPLAST 4 IN 1        advance_tax_rate=0.5000
... (177 more lines, all advance_tax_rate=0.5000)
```
`legacy_payload->>'AdvanceTaxAmt' = '1574.17'` for this document, but because
the first line (120981) has `advance_tax_rate = 0`, no row in the report
output carries this document's advance tax at all. Other confirmed examples:
document_number `2503` (PKR 656.00), `4231` (PKR 444.00), `328` (PKR 356.60),
`4372` (PKR 240.00).

Additional note: independently confirmed **0 lines** in the sandbox tenant
have a `pricing->'taxes'` jsonb array entry with `kind = 'advance_tax'`, so
100% of the matched rows come through the guarded-fallback path described
above, not the (unused, in this tenant) primary jsonb-snapshot path — making
this fallback's line-alignment defect the sole determinant of correctness
for this leaf in the current dataset.

This is a distinct defect from the tracked `business_document_lines.tax_amount
= 0` issue (`task_b63bf468`, `docs/evidence/PHASE_G_PRICING_GOLDEN_REPLAY_2026-08-08.md`
§6) — that issue is about `tax_amount` on qty>1 lines; this one is about the
document-level `AdvanceTaxAmt` fallback being mis-gated to a single physical
line that must also carry `advance_tax_rate > 0`.

**Location:** `financeReadModelQuery`, `reports.go`, case `"tax-advance",
"tax-advance-input"`, lines 4391–4439 (specifically the interaction between
line 4405's `CASE WHEN l.line_number = (SELECT MIN(l2.line_number) ...)` and
line 4432's `AND l.advance_tax_rate > 0`).

### 3. `customer-sales-customer-category-wise-sales-output-sales-tax-report` (financeMode `tax-output`)

**Verdict: MISMATCH-DOCUMENTED**

The base numeric projection (line_total/tax_amount pulled straight from
`business_document_lines`) is correct — see leaf 5 below for the underlying
sum cross-check. However, this leaf's title claims a "Customer Category
Wise" grouping/breakdown, and the query it resolves to
(`financeReadModelQuery("tax-output", ...)`) is **byte-identical** to the
plain `sales-tax-report` query: no customer-category column, filter, or
`GROUP BY` of any kind. Confirmed via
`TestTaxOutputModeIsSharedVerbatimAcrossDifferentlyDimensionedLeaves` in the
companion Go test (string-equal query text for all three "tax-output"
leaves).

Independent check that source data for the claimed dimension exists but is
ignored: `master_parties.payload` for `party_type='customer'` carries a
`CustCatCode` key for every customer. Grouping the 28,184 tax-bearing sale
lines by `COALESCE(mp.payload->>'CustCatCode','UNSPECIFIED')` gives a single
category `'1'` with `SUM(tax_amount) = 3,726,750.04` — in this specific
tenant every tax-bearing customer happens to share one category, so the
*scalar total* the correctly-grouped report would show is a no-op match
against the current flat total. The *shape* (per-category breakdown columns,
one row per category rather than one row per invoice line) is still not
implemented, and in a tenant with more than one customer category this would
diverge from the current per-line, ungrouped output.

**Location:** `reportSpecForKey` resolves this kind via
`phaseQFinancialOverrides["customer-sales-customer-category-wise-sales-output-sales-tax-report"]
= "tax-output"` (reports.go line 563); `financeReadModelQuery` case
`"tax-output"` (lines 4441–4462) has no category dimension.

### 4. `customer-sales-customer-ntn-wise-sales-tax-report` (financeMode `tax-output`)

**Verdict: MISMATCH-DOCUMENTED**

Same shared `tax-output` query as leaf 3/5 (confirmed byte-identical via the
same test). `master_parties.payload` carries an `NTNNo` key for customers.
Independent check: of the 28,184 tax-bearing sale lines in the sandbox
tenant, **0** have a customer with a populated `NTNNo`. A legacy "NTN Wise
Sales Tax Report" — a Pakistani FBR-compliance report that specifically
lists output tax attributable to NTN-registered customers — would be
expected to filter to only NTN-registered customers, which in this tenant
would mean **0 rows**. The current implementation instead returns all 28,184
lines regardless of NTN registration status, i.e. it is a strict superset of
what the correctly-dimensioned report should show, not merely a differently
shaped equivalent.

**Location:** same as leaf 3; override at reports.go line 564.

### 5. `sales-tax-report` (financeMode `tax-output`)

**Verdict: MATCHED**

This is the plain, undimensioned leaf and the query shape it resolves to
imposes no claim beyond a flat, posted-only line-level tax listing.

Independent cross-check: computed the total tax-bearing sale lines and sum
of `tax_amount` two ways.
1. Direct aggregate matching the report's own filter shape:
   ```sql
   SELECT count(*), sum(l.tax_amount)
   FROM business_documents d JOIN business_document_lines l ON ...
   WHERE d.status='posted' AND d.kind IN ('cash-sale','credit-sale') AND l.tax_amount>0;
   -- 28,184 rows, SUM = 3,726,750.04
   ```
2. Same figures reproduced independently while building the category-wise
   grouping check for leaf 3 (`GROUP BY CustCatCode` collapses to the same
   single-group total `3,726,750.04` across `28,184` lines) — an independent
   derivation path that agrees exactly.

`party` join logic verified correct: `mp.id = CASE WHEN d.kind IN
('cash-sale','credit-sale') THEN d.customer_id ELSE d.supplier_id END`
correctly resolves to the customer for sale kinds.

**Known, already-tracked caveat (not a new bug):** confirmed the tracked
`tax_amount = 0` issue for some qty>1 lines is present in this same dataset —
11,574 posted sale lines have `quantity > 1`, `tax_amount = 0`, yet the item
has an active `item_tax_assignments` → `tax_rates` link with `rate > 0`.
These rows are simply excluded by the report's `l.tax_amount > 0` filter
(undercounting rather than misreporting), consistent with `task_b63bf468`
(`docs/evidence/PHASE_G_PRICING_GOLDEN_REPLAY_2026-08-08.md` §6). Not re-flagged as a
new finding here.

### 6. `supplier-category-wise-input-sales-tax-report` (financeMode `tax-input`)

**Verdict: MISMATCH-DOCUMENTED**

Query: `financeReadModelQuery("tax-input", ...)`, `kindFilter =
'pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return'`,
requires `l.tax_amount > 0`.

Base sum cross-check (independent aggregate over the same filter shape):
**11,578 rows**, `SUM(tax_amount) = 25,905,620.40` for posted purchase-kind
documents.

Same "category wise" title/shape gap as leaf 3: `master_parties.payload` for
`party_type='supplier'` carries a `SuppCatCode` key, but the tax-input query
has no supplier-category column or grouping. Grouping the same 11,578 lines
by `COALESCE(mp.payload->>'SuppCatCode','UNSPECIFIED')` also collapses to a
single category `'1'` in this tenant (again a scalar-total no-op, shape
still wrong). `party` join for purchase kinds correctly uses `d.supplier_id`.

**Location:** `financeReadModelQuery` case `"tax-input"` shares the same
lines 4441–4462 branch as `"tax-output"`, differing only in `kindFilter`
(line 4388–4390) and the party-id `CASE` (which already correctly resolves
to `supplier_id` for non-sale kinds).

---

## Admin/listing leaves

### 7. `listing-supplier-list` (adminKind `supplier`)

**Verdict: MATCHED**

`adminReadModelQuery` default branch: `SELECT ... FROM master_records mr
WHERE mr.kind = 'supplier'`.

Independent cross-check against the canonical `master_parties` table:
- `master_records` rows with `kind='supplier'`: **235**.
- `master_parties` rows with `party_type='supplier'`: **235** (all active).
- Joined on `legacy_id`, comparing `code`/`name`/`active`: **0 mismatches**
  across all 235 rows.

### 8. `listing-items-list` (adminKind `item`)

**Verdict: MATCHED**

- `master_records` rows with `kind='item'`: **30,052**.
- `master_items` total rows: **30,052** (28,893 active).
- Joined on `legacy_id`, comparing `code`/`name`/`active`: **0 mismatches**
  across all 30,052 rows (5-row random spot sample manually inspected byte
  for byte and shown identical, in addition to the full-table mismatch
  count).

### 9. `listing-manufacturer-list` (adminKind `manufacturer`)

**Verdict: MATCHED**

- `master_records` rows with `kind='manufacturer'`: **838**.
- `master_manufacturers` total rows: **838**.
- Joined on `legacy_id`: **0 mismatches** on `code`/`name`/`active`, **0**
  `master_records` rows fail to resolve to a `master_manufacturers` row.

### 10. `listing-group-rights-list` (adminKind `roles`)

**Verdict: MISMATCH-DOCUMENTED — confirmed new bug, most significant finding of this pass**

Query: `adminReadModelQuery` case `"roles"` (reports.go lines 4590–4604):
```sql
SELECT r.id::text, COALESCE(rp.permission, ''), r.name, r.code, ...
FROM roles r
LEFT JOIN role_permissions rp
  ON rp.tenant_id = r.tenant_id AND rp.role_id = r.id
WHERE r.tenant_id = $1 ...
```

Independent check found two materially different tables in the schema:
- `role_permissions` (queried by the report): **0 rows** for this tenant.
- `group_rights` (**not** queried by the report): **726 rows** for this
  tenant — the actual migrated legacy right-code data, broken down as
  `ADMINISTRATOR=486`, `SHIFT INCHARGE=123`, `SALES OFFICER=111`,
  `REMOTE=6`. The `486`-row figure for `ADMINISTRATOR` matches the "486
  legacy right-codes" figure already documented for Phase R
  (`docs/REMAINING_WORK_100_PERCENT_PARITY_2026-08-09.md` line 169), strongly
  suggesting `group_rights` — not `role_permissions` — is the intended
  source for a legacy "Group Rights List" report. `group_rights` also
  carries `right_code`, `legacy_status`, and `legacy_group_code` columns that
  map directly onto the report's own declared "Right" and "Source" columns
  (`reportAdminColumns("roles")`, reports.go lines 1195–1204), which
  `role_permissions` has no equivalent of.

Running the report's actual query against the sandbox tenant returns exactly
**4 rows** (one per role, `permission=''` for all of them, `allowed='true'`
default, `source='normalized role_permissions'` literal) instead of the
726-row group-rights list the migrated data actually supports.

**Location:** `adminReadModelQuery`, `reports.go`, case `"roles"`, lines
4590–4604. The `roles`/`role_permissions` join looks like a plausible
"normalized RBAC" read but the actual legacy Group Rights List data lives in
`group_rights`, which this branch never references.

### 11. `listing-item-list-class-wise` (adminKind `item_class`)

**Verdict: MISMATCH-DOCUMENTED — confirmed new bug, structural (not tenant-specific)**

Query: `adminReadModelQuery` default branch (reports.go lines 4605–4620):
```sql
SELECT ... FROM master_records mr WHERE mr.tenant_id = $1 AND mr.kind = 'item_class' ...
```
(`kind` is passed through verbatim as the literal `adminKind` value
`"item_class"` from the registry entry at line 495.)

Independent check: `master_records.kind` has a `CHECK` constraint whose
allowed-value array does **not** contain `'item_class'` (underscore) at all
— only a hyphenated `'item-class'` variant is in the allowed list, alongside
`'item-category'`, `'supplier-category'`, `'manufacturer-category'`, etc.
Querying the actual distinct `kind` values present for this tenant confirms
14 populated kinds (`area, config_setting, customer, customer_category,
godown, godown_group, item, item_category, manufacturer,
manufacturer_category, preference, price_policy, supplier,
supplier_category`) — **neither `item_class` nor `item-class` has ever been
populated**, and the underscore spelling used by the report registry can
never be populated given the CHECK constraint.

Conclusion: this is not merely "no data in this sandbox" — the query as
written is guaranteed to return zero rows for **any** tenant, because the
literal string it filters on cannot exist in the column per the schema's own
constraint. This is a code defect (wrong/never-populated `kind` value), not
a migration coverage gap.

**Location:** registry entry `reports.go` line 495 (`adminKind: "item_class"`)
combined with `adminReadModelQuery` default branch, lines 4605–4620.

### 12. `listing-groupwise-user-list` (adminKind `users`)

**Verdict: MATCHED (content), with a documented systemic date-filter caveat**

Query: `adminReadModelQuery` case `"users"` (reports.go lines 4569–4589),
joining `users` → `user_memberships` → `roles`.

Independent check: **9 users** for this tenant, joined to their role names
via `user_memberships`:
```
ADMIN       -> ADMINISTRATOR
ALI         -> SALES OFFICER
DR SAIRA    -> SHIFT INCHARGE
FARYAD      -> SALES OFFICER
HAMID ALI   -> SALES OFFICER
HAMMAD      -> SALES OFFICER
RAEES KHAN  -> SALES OFFICER
SHAZIB      -> SHIFT INCHARGE
ZUBAIR ARIF -> SALES OFFICER
```
This matches `user_memberships` row count (9, one membership per user) and
`roles` count (4). Field values (`username`, `display_name`, `active`,
aggregated role name) are correct for a straightforward listing.

**Caveat (systemic, not specific to this leaf — noted, not re-flagged as a
fresh per-leaf bug):** `users.created_at` for every migrated user is pinned
to the literal migration run timestamp
(`2026-08-06 07:17:15.260731+05` for all 9 rows), because the legacy
`SysUsr`-equivalent source has no creation-date field
(`legacy_payload` sample: `{"Phone": "SDF3E", "Active": "Y", "UserCode": 1,
"UserName": "ADMIN", ...}` — no date). Since the query filters
`u.created_at >= $3::date AND u.created_at < $4::date + 1 day`, requesting
this report for any date range that does not include the migration day
returns **zero** users even though all 9 obviously exist. This is a data/
schema limitation (no legacy user-creation timestamp was ever captured), not
a reports.go logic error, and is analogous to the already-documented
`master_records.updated_at`-pinned-to-migration-time pattern seen for
`listing-manufacturer-list` (`min = max = 2026-08-06 02:13:25`).

### 13. `listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict` (adminKind `users`)

**Verdict: MISMATCH-DOCUMENTED**

Registry entry (reports.go line 497) wires this leaf to the identical
`adminKind: "users"` as leaf 12, and `reportSpecForKey`/`add` produce byte-
identical `reportSpec` values (same `adminKind`, same generated columns via
`reportAdminColumns("users")`) — this leaf's output is indistinguishable
from `listing-groupwise-user-list`.

The legacy report title implies a specific business computation: detecting
conflicts where a sales person's assigned scope overlaps across
manufacturer/sub-area boundaries. Independently searched the migrated schema
for any table that could support that computation — `group_allowed_scopes`
was the only "scope"-named table found, and its columns
(`role_id, scope_kind, scope_key, scope_label, allowed, legacy_table`) are
role/branch/menu scope for RBAC, not sales-person-to-manufacturer/sub-area
assignment. No `salesman`, `sales_person`, `sub_area`, or equivalent table
exists in the schema. There is currently no data model in the migrated
schema capable of expressing "manufacturer/sub-area wise sales person
conflict" at all; the report falls back to the same flat user listing as
leaf 12 with zero conflict-detection logic.

**Location:** registry entry `reports.go` line 497; `reportAdminColumns`
(line 1184) and `adminReadModelQuery` case `"users"` (lines 4569–4589) are
shared verbatim with leaf 12.

---

## Summary of verdicts

| # | Leaf | Verdict |
|---|---|---|
| 1 | `customer-sales-customer-wise-advance-tax` | VACUOUS-NO-DATA (legitimate) |
| 2 | `supplier-wise-advance-income-tax` | **MISMATCH-DOCUMENTED (new bug: 94 docs / PKR 9,589.64 silently dropped)** |
| 3 | `customer-sales-customer-category-wise-sales-output-sales-tax-report` | MISMATCH-DOCUMENTED (no category dimension) |
| 4 | `customer-sales-customer-ntn-wise-sales-tax-report` | MISMATCH-DOCUMENTED (no NTN filter; superset of correct output) |
| 5 | `sales-tax-report` | MATCHED |
| 6 | `supplier-category-wise-input-sales-tax-report` | MISMATCH-DOCUMENTED (no category dimension) |
| 7 | `listing-supplier-list` | MATCHED |
| 8 | `listing-items-list` | MATCHED |
| 9 | `listing-manufacturer-list` | MATCHED |
| 10 | `listing-group-rights-list` | **MISMATCH-DOCUMENTED (new bug: reads empty `role_permissions` instead of populated `group_rights`; 4 rows vs. 726)** |
| 11 | `listing-item-list-class-wise` | **MISMATCH-DOCUMENTED (new bug: `adminKind "item_class"` can structurally never match any row)** |
| 12 | `listing-groupwise-user-list` | MATCHED (content), with systemic migration-timestamp date-filter caveat |
| 13 | `listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict` | MISMATCH-DOCUMENTED (no underlying data model; duplicate of leaf 12) |

**New bugs found this pass (not previously tracked):** 3 —
(2) advance-tax fallback mis-gated to a single physical line;
(10) Group Rights List reads the wrong (empty) table;
(11) `item_class` adminKind literal can never match the schema's allowed
`master_records.kind` values.

**Regression guard added:** `services/api/internal/httpapi/report_golden_tax_admin_test.go`
— locks registry wiring for all 13 leaves and characterizes the three bugs
above (query-text assertions, no DB required). `go vet ./services/api/...`
clean; `go test -run
'TestPhaseGoldenTaxAdminLeavesResolveToExpectedMode|TestAdvanceTaxFallbackAmountGatedToSamePhysicalLineAsRate|TestGroupRightsListReadsRolePermissionsNotGroupRights|TestItemClassAdminKindCanNeverMatchAMasterRecordsRow|TestTaxOutputModeIsSharedVerbatimAcrossDifferentlyDimensionedLeaves'
./services/api/internal/httpapi/` passes (5/5).

No changes were made to `reports.go` (read-only for this agent, per
assignment).

---

## Addendum (2026-08-09, follow-up fix pass) — a second, more severe bug found while fixing #2

During the fix pass for finding #2 (`listing-group-rights-list` reading the
wrong table), fixing the join alone was not enough — the report still
returned `503 report_read_failed`. Root-caused to a **pre-existing,
previously-undetected bug that predates this session**: `adminReadModelQuery`'s
`"roles"` case (both the old `role_permissions` version and the new
`group_rights` version) never references placeholder `$2` (`operator.BranchID`)
anywhere in its SQL text, while the call site always binds 7 positional
arguments ending in `LIMIT $6 OFFSET $7`. PostgreSQL's extended query
protocol requires an inferable type for every parameter number up to the
highest one referenced (`$7`), and cannot infer one for a number that never
appears in the query body — `could not determine data type of parameter $2`,
surfaced as a 503 on every single request, unconditionally.

**This is not unique to `"roles"` — the `default` branch of the same
function (used by `listing-supplier-list`, `listing-items-list`,
`listing-manufacturer-list`, `listing-item-list-class-wise`, and
`listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict`)
has the identical gap and is equally broken.** The "MATCHED" verdicts
recorded above for `listing-supplier-list` / `listing-items-list` /
`listing-manufacturer-list` (§7–9) were obtained by comparing
`master_records` against `master_parties`/`master_items` directly — a
correct data-provenance check, but one that never actually exercised the
live `/v1/reports/{kind}` HTTP path, so it did not catch this endpoint-level
crash. The data-correctness conclusion in §7–9 still holds; it just wasn't
proof the endpoint itself worked.

**Fixed** (see `services/api/internal/httpapi/reports.go`, `adminReadModelQuery`):
added `AND $2::uuid IS NOT NULL` to both the `"roles"` case and the
`default` branch — a harmless, always-true predicate that gives Postgres a
type for `$2` without changing result semantics (`roles`/`group_rights` and
`master_records` are tenant-scoped, not branch-scoped, so `$2` was never
meant to filter anything here; it only needed a syntactic reference).
Verified via direct `psql PREPARE` (fails before the fix, succeeds after)
and via the live handler through `TestGroupRightsListMatchesGroupRightsAfterJoinFix`,
which now passes end-to-end (200 OK, 726 real `group_rights` rows: 486/123/111/6
across ADMINISTRATOR/SHIFT INCHARGE/SALES OFFICER/REMOTE).

Also fixed the join-fix agent's own mistake along the way: it had referenced
`gr.legacy_group_code`, a column that does not exist on `group_rights`
(only `legacy_status` does) — corrected in the same edit.

**Net effect of this addendum:** `listing-group-rights-list`,
`listing-supplier-list`, `listing-items-list`, `listing-manufacturer-list`,
and `listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict`
all move from "would 503 in production" to "actually returns data" — 5
leaves, not just the 1 originally scoped for this fix.
`listing-item-list-class-wise` (finding #9) no longer 503s either, but per
finding #9 still structurally returns 0 rows (the `adminKind: "item_class"`
value can never match any `master_records.kind`) — that deeper fix remains
open.

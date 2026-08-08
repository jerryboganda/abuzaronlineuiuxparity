# Phase Q Golden Verification — Finance-Mode Report Leaves

Date: 2026-08-09. Author: parallel audit agent (finance-mode leaves cluster), one of 10 concurrent agents attacking the reports-parity golden-verification gap described in `docs/REMAINING_WORK_100_PERCENT_PARITY_2026-08-09.md` ("Phases N–Q ... 151 report leaves ... 0/151 golden-verified").

Scope: the 10 `financeMode`-based report leaves assigned to this agent:

1. `accounts-reports-ledger-reports-accounts-ledger` (Accounts Ledger, `financeMode="gl"`)
2. `gl-journal` (alias, `financeMode="historical-gl"`)
3. `trial-balance` (alias, `financeMode="trial-balance"`)
4. `customer-statement` (alias, `financeMode="party-customer"`)
5. `supplier-statement` (alias, `financeMode="party-supplier"`)
6. `receivables-aging` (alias, `financeMode="aging-receivable"`)
7. `payables-aging` (alias, `financeMode="aging-payable"`)
8. `voucher-register` (alias, `financeMode="voucher"`)
9. `tax-register` (alias, `financeMode="tax-output"`)
10. `withholding-tax-deduction` (`financeMode="tax-withholding"` via `phaseQFinancialOverrides`)

`services/api/internal/httpapi/reports.go` was read-only for this pass (per the concurrency rules for this run); no source file under `services/api` was modified. All SQL below is either copied verbatim from `reports.go` (the exact strings inside `financeReadModelQuery` / `historicalGLReadModelQuery`, lines ~4062–4565) with the `$1..$5` placeholders replaced by literal values, or an independently-written query against the same underlying tables using a different formulation, per the assigned methodology.

## Methodology (stated per assignment; no live SQL Server access this pass)

Live legacy SQL Server access (sqlcmd trusted-auth) was confirmed unreachable from this tool environment ("Login failed. The login is from an untrusted domain") — this was not re-attempted. Verification instead used:

1. **Postgres direct access**: `postgres://postgres@127.0.0.1:5432/abuzar_next`, connected as the `postgres` role. Note: this role is a Postgres superuser and therefore **bypasses row-level security entirely regardless of the `app.tenant_id`/`app.branch_id`/`app.allow_tenant_scope` session GUCs** — the RLS preamble was run before every session for parity with the documented procedure, but every query below also carries an explicit `tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'` / `branch_id = 'ffffffff-ffff-ffff-ffff-ffffffffffff'` predicate, mirroring exactly the `$1`/`$2` bind parameters the report handler passes (`operator.TenantID`, `operator.BranchID`). Sandbox tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` (code `legacy-reference-sandbox`), branch `ffffffff-ffff-ffff-ffff-ffffffffffff` (code `SANDBOX`, confirmed via `SELECT id, code FROM branches WHERE tenant_id = ...`).
2. For each leaf: read the exact query-building code in `reports.go` (`financeReadModelQuery`, `historicalGLReadModelQuery`, `financeReportColumns`, `financeProjectionNote`, and the dispatch block around line 5179), then independently re-derive the same aggregate/total with a **separately hand-written** query against the same migrated source-of-truth tables (`historical_gl_entries`, `gl_journals`/`gl_lines`, `party_ledger_entries`, `business_documents`/`business_document_lines`, `historical_withholding_tax_entries`, `voucher_entries`), and compare.
3. Where useful, the *exact* report SQL (params substituted) was also run directly to compare row-for-row against the independent aggregate, and 3+ individual rows were spot-checked field-by-field against the raw source table.
4. Every leaf's verdict below is one of `MATCHED`, `MISMATCH-DOCUMENTED`, `VACUOUS-NO-DATA`, or `UNVERIFIABLE-THIS-PASS`, following the assignment's instruction not to claim a match on a vacuous empty-vs-empty result.

All dollar amounts below are PKR, taken verbatim from `numeric(19,4)` columns.

**Automated regression test**: `services/api/internal/httpapi/report_golden_finance_test.go` (`TestPhaseQFinanceGoldenVerification`) encodes 5 of the exhaustive-window checks below (`gl-journal`, `accounts-ledger`, `trial-balance`, `supplier-statement`, `tax-register`) as Go subtests that call the real `server.report` HTTP handler against the live sandbox tenant/branch and assert row-for-row/paisa-for-paisa equality with an independently hand-written SQL aggregate, guarded by `DATABASE_URL` (skips cleanly if unset). It is strictly read-only against the pre-existing sandbox tenant — no fixtures are seeded or torn down. Verified passing in this pass:

```
DATABASE_URL="postgres://postgres@127.0.0.1:5432/abuzar_next" go test ./internal/httpapi/ -run TestPhaseQFinanceGoldenVerification -v

--- PASS: TestPhaseQFinanceGoldenVerification (0.35s)
    --- PASS: .../gl-journal_historical-gl_matches_historical_gl_entries_for_a_single_day (0.04s)
    --- PASS: .../accounts-ledger_gl_mode_matches_historical_gl_entries_for_the_same_day (0.01s)
    --- PASS: .../trial-balance_grouped_totals_reconcile_to_the_same_day's_ungrouped_totals (0.01s)
    --- PASS: .../supplier-statement_matches_party_ledger_entries_for_a_real_multi-supplier_window (0.09s)
    --- PASS: .../tax-register_matches_business_document_lines_for_a_single_day (0.02s)
PASS
```

`go vet ./services/api/...` was also run clean (no output) after adding this file. The remaining 5 leaves (`customer-statement`, `receivables-aging`, `payables-aging`, `voucher-register`, `withholding-tax-deduction`) were verified via direct `psql` queries only (documented per-leaf below) and are not encoded as automated tests in this pass — `customer-statement`/`receivables-aging` because their entire real dataset is 2 rows (not worth a maintained regression test), and `voucher-register`/`withholding-tax-deduction` because they are vacuous (nothing to regress-test against).

**Incidental finding while building the automated test (documented, not fixed)**: the report `filter` query parameter (`$5` in every `financeReadModelQuery` branch) is applied as `document_id ILIKE '%<filter>%' OR party_name ILIKE '%<filter>%' OR description ILIKE '%<filter>%'`. An early version of the `supplier-statement` test tried to isolate one supplier by its legacy numeric code (`filter=112`, expecting exactly 12 rows) and instead got **39** — because `document_id` is a UUID, and short numeric substrings like `"112"` routinely appear by chance inside a UUID's hex digits, silently pulling in unrelated suppliers' rows alongside the intended match. This is not unique to `supplier-statement` — every `financeMode` branch shares the same `$5` filter clause against a UUID-valued `document_id`/`row_id`/similar column, so any short numeric or short-hex filter text is liable to over-match via accidental UUID substring collisions across **all 10 leaves in this batch**. This is a real usability/precision gap in the shared filter behavior (a user typing a short document number to search would get noisy, unrelated results mixed in), not a data-correctness bug — the underlying amounts are still correct for whatever rows the filter happens to include. Flagging for a follow-up pass rather than fixing here, since `reports.go` was read-only for this assignment.

---

## 1. Accounts Ledger (`accounts-reports-ledger-reports-accounts-ledger`, `financeMode="gl"`)

**Query under test** — `financeReadModelQuery("gl", ...)`, `reports.go` lines 4088–4113. For `mode="gl"` the `ledger_rows` CTE is a `UNION ALL` of (a) posted `gl_journals`/`gl_lines` joined to `finance_accounts`, and (b) *all* `historical_gl_entries` rows (imported from legacy `dbo.VirtualGl`), with the historical branch falling back to `'Historical ' || account_code` when no `finance_accounts.code` matches.

**Data availability**: `gl_journals`/`gl_lines` have **0 rows** for this tenant/branch (854 rows exist database-wide, but they all belong to ~300 unrelated dev/test tenants created by the automated test suite — confirmed via `SELECT tenant_id, count(*) FROM gl_journals GROUP BY tenant_id`). So for this tenant, leaf 1's output is driven entirely by the `historical_gl_entries` branch of the union (1,021,801 rows total for this tenant/branch, imported from `VirtualGl`, spanning 2025-01-01 to 2026-07-31).

**Independent verification** — aggregate for window `2025-01-01`..`2025-01-31` (chosen because it has a genuine debit≠credit gap in the raw data, useful to prove the query isn't silently forcing balance):

```sql
-- raw baseline, hand-written independently of the report's CTE
SELECT count(*), sum(debit_amount), sum(credit_amount)
FROM historical_gl_entries
WHERE tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND branch_id = 'ffffffff-ffff-ffff-ffff-ffffffffffff'
  AND occurred_at >= '2025-01-01' AND occurred_at < '2025-02-01';
-- => 52427 | 35192242.0000 | 35191749.0000
```

```sql
-- report's exact "gl" mode CTE (params substituted), same window
WITH ledger_rows AS ( ... UNION ALL ... /* verbatim from reports.go */ )
SELECT count(*), sum(debit_amount), sum(credit_amount)
FROM ledger_rows
WHERE occurred_at >= '2025-01-01' AND occurred_at < '2025-02-01';
-- => 52427 | 35192242.0000 | 35191749.0000
```

**Result**: exact match — count 52,427, debit 35,192,242.0000, credit 35,191,749.0000, identical to the raw baseline.

**Row-level spot check** (labeling logic): historical `account_code` values in this dataset are short legacy numeric codes (`'1'`, `'5'`, `'19'`, ...) which never equal any `finance_accounts.code` (canonical chart-of-accounts codes like `'1000'`, `'2100'`). So every historical row falls back to the `'Historical ' || account_code` label branch. This is expected and is explicitly disclosed by `financeProjectionNote("gl")`: *"historical account labels remain explicit and exact opening-balance/print semantics remain unverified"* — not a bug.

**Verdict: MATCHED.** Real, substantial migrated data (1.02M rows); report aggregate reconciles exactly to an independently-derived raw sum for a full-month window.

---

## 2. GL Journal (`gl-journal`, `financeMode="historical-gl"`)

**Query under test** — `historicalGLReadModelQuery`, `reports.go` lines 4513–4565. Unions all of `historical_gl_entries` (verbatim passthrough of `legacy_id, occurred_at, document_type, account_code, alternate_account_code, invoice_code, user_legacy_id, remarks, debit_amount, credit_amount`) with posted `gl_journals`/`gl_lines` rows labeled `'canonical:' || id`.

**Independent verification** — same `2025-01-01..2025-01-31` window, run as its own standalone reproduction of the query:

```sql
-- report's exact historical-gl CTE (params substituted)
-- => 52427 | 35192242.0000 | 35191749.0000
```

Identical to leaf 1's totals and to the raw baseline (expected: `gl_journals` is empty for this tenant, so both modes reduce to the same `historical_gl_entries` slice).

**Row-level spot check** — first 3 rows by `occurred_at` compared field-by-field against `historical_gl_entries` directly:

| legacy_id | document_type | account_code | alternate_account_code | invoice_code | user_legacy_id | remarks | debit | credit |
|---|---|---|---|---|---|---|---|---|
| `1:0:1` | PV | 1 | (null) | 1 | 1 | Stock accounted for open. pur. #: 1 | 11,873,579.0000 | 0.0000 |
| `1:1:5` | PV | 5 | (null) | 1 | 1 | (null) | 0.0000 | 11,873,579.0000 |
| `588873:0:2` | SV | 2 | 19 | 588873 | 5 | Cash receipt on sale #: 588873 | 2.0000 | 0.0000 |

The report's `SELECT` list is a direct 1:1 passthrough of these columns with no transformation, so row-level correctness follows trivially from the aggregate match — confirmed by re-reading the query text (no computed/derived columns in this mode besides the `'canonical:'` prefix, which is not exercised because `gl_journals` is empty here).

**Verdict: MATCHED.**

---

## 3. Trial Balance (`trial-balance`, `financeMode="trial-balance"`)

**Query under test** — `reports.go` lines 4114–4154. Groups posted `gl_journals`/`gl_lines` (joined to `finance_accounts`/`finance_account_categories`) `UNION ALL` `historical_gl_entries` (left-joined the same way, falling back to `'(unmapped)'`/`'Historical'`) by `account_code`, summing debit/credit per account.

**Independent verification** — same window, re-aggregated across all account groups and compared to the ungrouped total:

```sql
-- report's exact trial-balance CTE, grouped, then re-summed across all 75 groups
-- => 75 distinct account_code groups | sum(debit)=35192242.0000 | sum(credit)=35191749.0000
```

This is **exactly equal** to leaf 1 and leaf 2's totals and the raw baseline — i.e., re-grouping by account does not lose or double-count any debit/credit value. All three modes (`gl`, `historical-gl`, `trial-balance`) were run in a single combined query pass to guarantee they're being compared against the identical underlying row set at the identical instant:

```
              label               | count |      sum      |      sum      
-----------------------------------+-------+---------------+---------------
 raw_historical_gl_entries_2025_01 | 52427 | 35192242.0000 | 35191749.0000
 gl_mode_2025_01                   | 52427 | 35192242.0000 | 35191749.0000
 trial_balance_2025_01             |    75 | 35192242.0000 | 35191749.0000
 historical_gl_mode_2025_01        | 52427 | 35192242.0000 | 35191749.0000
```

**Verdict: MATCHED.** Cross-consistent across three independently-coded projections of the same source data, and consistent with a raw independent baseline.

---

## 4. Customer Statement (`customer-statement`, `financeMode="party-customer"`)

**Query under test** — `reports.go` lines 4155–4261 (`party-customer` branch). Unions `party_ledger_entries` (gated on either a posted `gl_journals` row or a posted `business_documents` row with `legacy_source_table <> ''`) with three historical tables: `historical_party_payment_allocations`, `historical_party_ledger_adjustments`, `historical_party_return_allocations`.

**Data availability**: this is the thinnest leaf in the batch. `party_ledger_entries` has **only 2 rows** with `counterparty_kind = 'customer'` in the *entire* sandbox tenant (out of 329,115 total party-ledger rows) — because the legacy POS overwhelmingly posts walk-in sales to `counterparty_kind = 'cash'` (322,063 rows), not to a named customer master record; only genuine `credit-sale` documents (2 in this tenant) create a `customer`-kind ledger entry. The three historical_* union legs are 0 rows tenant-wide (and database-wide).

Both real rows belong to one party — `party_id = 02126279-3c6d-4976-a3ac-cef8ca3c2b75`, `master_parties.name = "CREDIT SALES-WALKING CUSTOMER A/C"`:

```sql
SELECT ple.id, ple.party_id, mp.name, ple.occurred_at, ple.debit_amount, ple.credit_amount, ple.source_document_id
FROM party_ledger_entries ple LEFT JOIN master_parties mp ON mp.tenant_id=ple.tenant_id AND mp.id=ple.party_id
WHERE ple.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND ple.branch_id='ffffffff-ffff-ffff-ffff-ffffffffffff'
  AND ple.counterparty_kind='customer';
```
```
1bf1a89b-... | 02126279-... | CREDIT SALES-WALKING CUSTOMER A/C | 2025-04-24 17:13:13+05 | 1601.0000 | 0.0000 | 6257fdea-...
e8d6bf50-... | 02126279-... | CREDIT SALES-WALKING CUSTOMER A/C | 2025-05-10 19:36:26+05 |  472.0000 | 0.0000 | 509b90b7-...
```

Both source documents (`6257fdea-...`, `509b90b7-...`) confirmed `status='posted'`, `kind='credit-sale'`, `legacy_source_table='SaleLedger'`.

**Report SQL run verbatim** (full 4-leg UNION, params substituted, wide date window):

```
             document_id              |      occurred_at       |            party_name             |   description   | debit_amount | credit_amount 
--------------------------------------+------------------------+-----------------------------------+------------------+--------------+---------------
 6257fdea-0034-58a4-acab-97ad6cc18a9e | 2025-04-24 17:13:13+05 | CREDIT SALES-WALKING CUSTOMER A/C | historical sale | 1601.0000    | 0.0000
 509b90b7-0487-54b6-bf74-cb9442fd229b | 2025-05-10 19:36:26+05 | CREDIT SALES-WALKING CUSTOMER A/C | historical sale | 472.0000     | 0.0000
```

Exact match to the raw independent query (same 2 rows, same document IDs, dates, party name, amounts). The posted-gate condition (`EXISTS gl_journals ... OR EXISTS business_documents ...`) correctly did not drop either row.

**Verdict: MATCHED**, with the caveat that the dataset only exercises 1 real customer / 2 rows — genuinely thin, not because the query is wrong, but because this tenant's migrated data has almost no `counterparty_kind='customer'` traffic. Flagging this data-thinness explicitly rather than presenting it as a robust verification.

---

## 5. Supplier Statement (`supplier-statement`, `financeMode="party-supplier"`)

**Query under test** — same function, `party-supplier` branch (symmetric to leaf 4, `partyKind = 'supplier'`).

**Data availability**: much richer — 7,050 `party_ledger_entries` rows with `counterparty_kind='supplier'`. The three historical_* union legs are again 0 rows for this tenant, so results come purely from `party_ledger_entries`.

**Independent verification** — top suppliers by row count, raw `GROUP BY`:

```sql
SELECT ple.party_id, mp.name, count(*), sum(ple.debit_amount), sum(ple.credit_amount)
FROM party_ledger_entries ple LEFT JOIN master_parties mp ON mp.tenant_id=ple.tenant_id AND mp.id=ple.party_id
WHERE ple.tenant_id='...' AND ple.branch_id='...' AND ple.counterparty_kind='supplier'
GROUP BY 1,2 ORDER BY 3 DESC LIMIT 5;
```
```
 party_id (name) | count |      db      |      cr       
 117             |   421 |   53629.0000 | 24171608.0000
 290             |   415 |  460883.0000 |  3217673.0000
 111             |   364 |  141316.0000 | 12342013.0000
 226             |   287 |   15248.0000 |  2933077.0000
 311             |   247 | 1223365.0000 |   859669.0000
```

**Report SQL run verbatim** (full 4-leg UNION, grouped by party, top 10):

```
 party_name | count |      db      |      cr       
 117        |   421 |   53629.0000 | 24171608.0000
 290        |   415 |  460883.0000 |  3217673.0000
 111        |   364 |  141316.0000 | 12342013.0000
 226        |   287 |   15248.0000 |  2933077.0000
 311        |   247 | 1223365.0000 |   859669.0000
 296        |   226 |   20847.0000 |  1838638.0000
 125        |   222 |   89780.0000 |  6696681.0000
 118        |   204 |   85760.0000 | 10606982.0000
 107        |   204 |   49460.0000 | 12961123.0000
 104        |   204 |   36574.0000 |  4392907.0000
```

Exact match on every party, count, debit, and credit for the top 5 (and consistent for all 10 checked).

**Note (not a report-code bug)**: `master_parties.name` for these suppliers is literally their legacy numeric code (`"117"`, `"111"`, `"290"`, ...), not a real business name — verified `name == legacy_id` in `master_parties` for all three sampled suppliers. This is a property of the migrated master-data seeding in this sandbox tenant, not something `reports.go` computes or mangles; the report correctly displays whatever `master_parties.name` holds.

**Verdict: MATCHED.** Real, non-trivial data (7,050 rows / 30+ distinct suppliers), exact reconciliation on 5 sampled parties.

---

## 6. Receivables Aging (`receivables-aging`, `financeMode="aging-receivable"`)

**Query under test** — `reports.go` lines 4267–4325. Buckets posted customer `party_ledger_entries` (for `cash-sale`/`credit-sale`/return kinds) by an aging bucket derived from `business_documents.legacy_payload->>'DueDate'` (or `pricing_result->>'dueDate'` fallback), using `d.balance_amount` (not the raw ledger debit/credit) as the outstanding amount, gated by `HAVING SUM(...) <> 0`.

**Data availability**: same 2-row customer dataset as leaf 4 (both `credit-sale`, both still fully outstanding: `balance_amount = total_amount` for both, since `historical_party_payment_allocations` is empty — no payments were ever recorded against them in the migrated data for this tenant).

**Independent baseline check** — neither of the 2 source documents has `DueDate` populated:

```sql
SELECT legacy_payload->>'DueDate', pricing_result->>'dueDate' FROM business_documents WHERE id IN ('6257fdea-...', '509b90b7-...');
-- => (null, null) for both rows
```

Widened to the whole database: **0 of every `business_documents` row with `legacy_source_table <> ''`** has a non-empty `legacy_payload->>'DueDate'` — checked with `SELECT count(*) FROM business_documents WHERE legacy_source_table <> '' AND legacy_payload->>'DueDate' IS NOT NULL AND legacy_payload->>'DueDate' <> ''` → **0**.

**Report SQL run verbatim**:

```
               party_id               |          max           |             name                  |             bucket            |  debit    | credit 
 02126279-3c6d-4976-a3ac-cef8ca3c2b75 | 2025-05-10 19:36:26+05 | CREDIT SALES-WALKING CUSTOMER A/C | UNAGED - due date unavailable | 2073.0000 | 0
```

`2073.0000 = 1601.0000 + 472.0000`, exactly the sum of both source documents' `balance_amount` — arithmetic matches the independent computation.

**Verdict: MATCHED** on the arithmetic (the bucketing math and the posted-gate are correct and were exercised), but flagging: the bucket is **always** `"UNAGED - due date unavailable"` for every row in this dataset because `DueDate` is never populated in `legacy_payload` anywhere in the database. The `0-30/31-60/61-90/91+` bucket branches exist in the code and are logically sound (confirmed by reading the `CASE` expression), but were **not exercised by any real row** in this pass — there is currently no migrated data anywhere in the database that would land in a dated bucket. This mirrors the disclosed caveat in `financeProjectionNote("aging-receivable")` ("missing due dates remain explicitly unaged ... not fabricated"), so the code is being honest, not silently wrong — but it means the bucketing logic itself remains functionally unverified beyond the trivial UNAGED path.

---

## 7. Payables Aging (`payables-aging`, `financeMode="aging-payable"`)

**Query under test** — `reports.go` lines 4327–4385 (symmetric to leaf 6, using `Purledger.CreditDays` from `legacy_payload`/`pricing_result` instead of `DueDate`).

**Independent verification** — top-15 suppliers, report SQL run verbatim vs. the leaf-5 raw `GROUP BY` baseline: every party's `debit`/`credit` total in the aging output matches the corresponding raw `party_ledger_entries` sum exactly (e.g. party `111`: aging output `141316.0000 / 12342013.0000` == leaf-5 raw baseline `141316.0000 / 12342013.0000`; party `117`: `53629.0000 / 24171608.0000` in both). This holds because each `business_documents` row has exactly one non-void `party_ledger_entries` row (enforced by the `uq_party_ledger_primary_source_document` unique constraint), so `SUM(d.balance_amount)` gated by debit/credit sign is not double-counted, and — since `historical_party_payment_allocations` is empty for this tenant — no payments have reduced any `balance_amount` away from the original ledger amount.

**Same finding as leaf 6, but for `CreditDays`**: every single one of the 15 sampled suppliers' rows bucketed as `"UNAGED - due date unavailable"`:

```
 party_id (code) | bucket                          | debit       | credit
 101             | UNAGED - due date unavailable   |    6243.0000 |  9080832.0000
 102             | UNAGED - due date unavailable   |   83076.0000 | 11803868.0000
 111             | UNAGED - due date unavailable   |  141316.0000 | 12342013.0000
 117             | UNAGED - due date unavailable   |   53629.0000 | 24171608.0000
 ... (15/15 all UNAGED)
```

Checked the root cause directly:

```sql
SELECT kind, count(*) total,
  count(*) FILTER (WHERE legacy_payload->>'CreditDays' IS NOT NULL AND legacy_payload->>'CreditDays' <> '') has_cd
FROM business_documents WHERE tenant_id='...' AND branch_id='...'
AND kind IN ('pack-purchase','loose-purchase','opening-purchase','purchase-return')
GROUP BY 1;
```
```
       kind       | total | has_cd 
 loose-purchase   |    22 |      0
 opening-purchase |     1 |      0
 pack-purchase    |  6396 |      0
 purchase-return  |   634 |      0
```

Widened database-wide: **0 of 14,106** historical purchase-kind `business_documents` rows (across every tenant, not just the sandbox) have a non-empty `legacy_payload->>'CreditDays'`.

**Flag for follow-up (possible genuine gap, not fixed here)**: `migration/maps/phase-e-historical-documents.json` explicitly declares a field mapping `"CreditDays": "CreditDays"` for the Purledger import (lines 259 and 316), i.e. the migration was *designed* to carry this field forward. Yet it is populated in **zero** rows anywhere in the current database. Two explanations are equally plausible from this environment and cannot be distinguished without live SQL Server access to `dbo.Purledger.CreditDays`:
  (a) legacy suppliers in the source data legitimately never had `CreditDays` set (a business-data fact, not a bug), or
  (b) the field-mapping is declared but the actual bulk-import path silently drops/nulls it for every row (a real migration-pipeline defect).
This is the same root symptom already flagged for `receivables-aging`'s `DueDate` field (leaf 6) — both due-date-bearing fields are uniformly empty across the whole migrated dataset. Recommend a follow-up pass that either (i) obtains one live SQL Server session to spot-check 5 known `Purledger`/`SaleLedger` rows' raw `CreditDays`/`DueDate` values, or (ii) re-audits the `migration/cmd/bulk-historical` insert path for these two specific columns.

**Verdict: MATCHED** on the arithmetic/posted-gate logic (verified against 15 real suppliers); **MISMATCH-DOCUMENTED (flagged, not fixed)** on the aging-bucket *feature* itself, which is present and logically sound in code but functionally inert against 100% of currently-migrated data for the reason above.

---

## 8. Voucher Register (`voucher-register`, `financeMode="voucher"`)

**Query under test** — `reports.go` lines 4490–4505. Reads `voucher_entries` joined to `voucher_categories`, filtered to `status = 'posted'`.

**Data availability**: `voucher_entries` has **0 rows** for the sandbox tenant. Database-wide, `voucher_entries` has exactly **1 row total**, and it belongs to an unrelated tenant (`10000000-0000-0000-0000-000000000001`) with `status = 'draft'` and `description = 'RLS branch-B fixture'` — clearly a leftover fixture from an RLS test suite, not migrated legacy data, and not `posted` (so it wouldn't be returned by this query even for its own tenant).

```sql
SELECT id, tenant_id, branch_id, status, amount, description, created_at FROM voucher_entries;
-- => (1 row) 10000000-0000-0000-0000-00000000c001 | 10000000-...-001 | 10000000-...-102 | draft | 1.0000 | RLS branch-B fixture | 2026-08-06
```

Ran the report's exact SQL against the sandbox tenant/branch (params substituted, wide date window): **0 rows returned, no SQL error** — confirming the query itself is syntactically valid and its joins/filters are well-formed; there is simply nothing in `voucher_entries` for it to project.

**Verdict: VACUOUS-NO-DATA.** This confirms the previously-documented Phase K finding (`docs/REMAINING_WORK_100_PERCENT_PARITY_2026-08-09.md`, Phase K row: "vouchers schema-only, no VirtualGl reconciliation") — there is no voucher-posting endpoint in this codebase that would ever populate `voucher_entries` with real (posted, non-test) data, so this leaf cannot be golden-verified against any real value; the report code itself is real and would work correctly the moment vouchers are actually posted, but that mechanism doesn't exist yet.

---

## 9. Tax Register (`tax-register`, `financeMode="tax-output"`)

**Query under test** — `reports.go` lines 4441–4462 (`tax-output`/`tax-input` shared branch, `kindFilter = 'cash-sale', 'credit-sale'` for output). Reads posted `business_documents` joined to `business_document_lines` where `l.tax_amount > 0`.

**Data availability**: substantial — 28,184 taxed lines for `cash-sale` (0 for `credit-sale`, since none of its 8 lines carry tax).

**Independent verification** — hand-written raw aggregate for the full `2025-01-01..2026-12-31` window:

```sql
SELECT count(*), sum(l.line_total), sum(l.tax_amount)
FROM business_documents d JOIN business_document_lines l ON l.tenant_id=d.tenant_id AND l.branch_id=d.branch_id AND l.document_id=d.id
WHERE d.tenant_id='...' AND d.branch_id='...' AND d.status='posted' AND d.kind IN ('cash-sale','credit-sale')
  AND l.tax_amount > 0 AND d.occurred_at >= '2025-01-01' AND d.occurred_at < '2027-01-01';
-- => 28184 | 23929917.1000 | 3726750.0400
```

**Report's exact `tax-output` query run verbatim** (params substituted, same window, wrapped in a `count`/`sum` aggregate):

```
 count |      sum      |     sum      
 28184 | 23929917.1000 | 3726750.0400
```

Exact match — count, sum(line_total), and sum(tax_amount) all identical to the paisa.

**Row-level spot check** (3 most recent taxed lines):

| document_number | party | item | line_total | tax_amount |
|---|---|---|---|---|
| 880232 | CASH SALES CUSTOMER | NESTLE 0.5LTR | 70.0000 | 10.6800 |
| 880225 | CASH SALES CUSTOMER | DETTOL HAND SANITIZER 50ML | 280.0000 | 42.7100 |
| 880224 | CASH SALES CUSTOMER | MEIJI FMT 400G MILK | 2370.0000 | 361.5300 |

Tax rates implied (~15.25%) are plausible sales-tax rates and consistent across all three sampled lines.

**Verdict: MATCHED.** Real, substantial data (28,184 lines); exact-to-the-paisa reconciliation against an independently-written aggregate query.

---

## 10. Withholding Tax Deduction (`withholding-tax-deduction`, `financeMode="tax-withholding"`)

**Query under test** — `reports.go` lines 4463–4489. Reads `historical_withholding_tax_entries` (imported from legacy `dbo.PurPayment`) left-joined to `business_documents`/`master_parties` for supplier name resolution, filtered to `posted = true AND amount <> 0`.

**Data availability**: `historical_withholding_tax_entries` has **0 rows for the sandbox tenant, and 0 rows database-wide across every tenant** — checked with an unscoped `SELECT count(*) FROM historical_withholding_tax_entries` → **0**.

Traced the root cause to the migration tooling: `migration/cmd/bulk-historical/main.go`, function `importWithholding` (lines 447–519+), which is a fully-implemented importer that reads

```sql
SELECT p.[PurPaymentCode], p.[Date], p.[UserCode], p.[PurInvCode], p.[Posted],
       p.[WHTaxAccCode], p.[WHTaxPerc], p.[WHTaxBaseAmt], p.[WHTaxAmt],
       p.[WHTaxCheckNo], p.[WHTaxRemarks], l.[SuppCode], l.[SuppInvCode]
FROM [dbo].[PurPayment] p LEFT JOIN [dbo].[Purledger] l ON l.[PurInvCode] = p.[PurInvCode]
```

directly from a live SQL Server `source *sql.DB` connection and batch-inserts into `historical_withholding_tax_entries` via `COPY` + `INSERT ... ON CONFLICT`. This code path requires exactly the live SQL Server access that is confirmed unreachable in this environment — and the empty table (database-wide, not just for this tenant) indicates this specific import step has evidently never been executed successfully against any tenant currently in this Postgres instance, unlike `historical_gl_entries` (1.02M rows, clearly imported) or `business_documents` (331,928 rows, clearly imported). Note also that `historical_party_payment_allocations` (also sourced from `PurPayment`, per `migration/maps/phase-e-party-payments.json`) is likewise 0 rows database-wide — so this looks like the entire `PurPayment` source table was never actually run through the bulk-historical importer for any tenant, not a withholding-specific bug.

Ran the report's exact SQL against the sandbox tenant/branch anyway, to confirm it is at least syntactically sound:

```
label       | count
withholding | 0
```

0 rows, no SQL error — the query itself is well-formed; it simply has a fully-empty source table to read from.

**Verdict: VACUOUS-NO-DATA**, with an important caveat beyond the "no data" label: unlike `voucher-register` (where the *application* has no posting endpoint), this leaf's source table is populated by a **migration import step** (`importWithholding`) that is fully coded and *should* run during Phase E, but for which there is no evidence it has ever successfully run in this database. This should be re-run (or its non-execution explained) once live SQL Server access is available — it is not the same class of gap as "feature not built," it is "migration step apparently skipped," and worth flagging to whoever owns Phase E re-runs.

---

## Summary

Of the 10 assigned finance-mode leaves: **7 MATCHED** against independently-derived aggregates or independently-run raw queries with real migrated data (`accounts-ledger`, `gl-journal`, `trial-balance`, `customer-statement`, `supplier-statement`, `receivables-aging`, `tax-register`), **1 MATCHED-on-arithmetic-but-flagged** (`payables-aging` — the money math is correct and verified, but its aging-bucket feature is functionally inert because `CreditDays` is unpopulated in 100% of migrated purchase documents, a possible migration gap worth a live-SQL-Server follow-up), and **2 VACUOUS-NO-DATA** (`voucher-register` — no posting endpoint exists yet, a known Phase K gap; `withholding-tax-deduction` — the query is real and correct but its sole source table, `historical_withholding_tax_entries`, is empty database-wide because the `PurPayment` import step appears to have never actually run, unlike other Phase E tables). No leaf's query logic was found to be internally broken or numerically wrong against the data it *does* have — the two "aging" leaves' bucket feature and the withholding leaf's data source are the only genuine follow-up items, and both are documented above with enough detail (file, function, exact field names, and row counts) for a future pass to act on without re-deriving this investigation. `receivables-aging` and `payables-aging` share the identical underlying cause (a due-date-bearing legacy field mapped in `migration/maps/phase-e-historical-documents.json` but populated in 0 rows of the current database), so a single follow-up investigation likely resolves both. Five of the seven `MATCHED` leaves (`gl-journal`, `accounts-ledger`, `trial-balance`, `supplier-statement`, `tax-register`) are now also pinned by an automated, passing Go test (`TestPhaseQFinanceGoldenVerification`) rather than relying solely on this document's one-time `psql` transcript, so they will catch regressions going forward. Building that test also surfaced one additional real finding not visible from `psql` alone: the shared `filter` query parameter used by every leaf in this batch over-matches when a short numeric/hex search string coincides with a substring of an unrelated row's UUID `document_id`, mixing unrelated parties into filtered results across all 10 leaves — see the "Incidental finding" callout above for detail; this is a precision/usability gap, not a monetary-correctness bug, and was left undocumented in code (read-only) for a follow-up pass.

# Phase N Golden Verification — User Wise (Sales Reports) leaves (2026-08-09)

Scope: 7 assigned Phase N "User Wise" report leaves, all currently
`event-ledger` (`services/api/internal/httpapi/reports.go` lines 221-317,
`phaseNReportRegistry`, "User Wise" submenu at lines 261-267). `reports.go`
was **not** edited (per the concurrent-agent rule) — this pass is read-only
plus this document.

**Files added by this pass:**
- `docs/PHASE_N_GOLDEN_VERIFICATION_USER_WISE_2026-08-09.md` (this file)

No test file was added: every finding below is a hard data-availability
verdict backed by a live `psql` query against the sandbox tenant, not a
query-shape assertion that would benefit from a regression lock. Adding a
test that merely re-asserts "operator_id is 0% populated" would be asserting
a data fact about a specific snapshot, not code behavior — it would not
protect against any real regression and would need to be deleted the moment
the migration/data situation changes.

## Headline finding — the user-attribution gap is a hard, exhaustively-confirmed blocker for all 6 non-`net-cash` leaves; `user-wise-net-cash` sidesteps it via a different (GL) data source, already promoted by another agent

This independently confirms and substantially extends the finding already
recorded in `docs/PHASE_N_REPORT_PROMOTION_2026-08-09.md` ("Candidates
examined and declined this pass" table, `user-wise-sales` row): "`business_documents.operator_id`
is **0/291,361** populated for posted cash/credit sale documents... There is
currently no reliable per-user attribution on migrated sales data at all."

I went further and checked every plausible attribution path the task asked
about — a normalized column (`operator_id`, `created_by`, `posted_by`,
`user_id`), a parallel table (`sales_documents.operator_id`), and the
`legacy_payload` JSON fallback pattern other reports in this file already
use — across **every tenant in the database**, not just the sandbox. All are
either unpopulated for real sale documents or populated only by ephemeral
Go-test tenants that carry zero migrated data.

### 1. `business_documents.operator_id` — 0% populated for every real sale document, in every tenant

```sql
SELECT count(*) AS total_docs, count(operator_id) AS with_operator,
       count(*) FILTER (WHERE kind IN ('cash-sale','credit-sale')) AS sale_docs,
       count(operator_id) FILTER (WHERE kind IN ('cash-sale','credit-sale')) AS sale_with_operator
FROM business_documents;   -- queried as superuser, no tenant filter, RLS bypassed
```
Result (whole database, all tenants): `374103 total docs | 1608 with operator
| 292430 sale docs | 1069 sale docs with operator`. This looked promising
until identifying **which** tenants those 1,069 populated sale docs belong
to:

```sql
SELECT t.code, count(*) FILTER (WHERE bd.operator_id IS NOT NULL) AS docs_with_operator
FROM business_documents bd JOIN tenants t ON t.id = bd.tenant_id
WHERE bd.operator_id IS NOT NULL GROUP BY t.code ORDER BY 2 DESC LIMIT 5;
```
Every single one belongs to an ephemeral, timestamp-named Go-integration-test
tenant (e.g. `finance-1786025611310132000`, `read-model-...`) created and torn
down by `go test` runs — 6 documents each, all created within the same
second, none of them the sandbox. **Zero** of the 291,361 real migrated cash
sales, 30,704 sale-returns, or 2 credit sales in the reconciled sandbox
tenant (`legacy-reference-sandbox`, `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`)
have `operator_id` set. The only non-ephemeral, non-sandbox tenant with any
data at all (`demo`) has 60 `loose-purchase` documents with `operator_id`
populated and **zero** sale documents of any kind — so it cannot serve as an
alternative real-data source either.

### 2. No other per-user column exists on `business_documents` or `business_document_lines`

`\d business_documents` / `\d business_document_lines` confirm the only
person-identifying column on either table is `operator_id`. There is no
`created_by`, `posted_by`, or `user_id` column on either table — those exist
elsewhere in the schema (`sessions.user_id`, `user_branch_assignments.user_id`,
`historical_gl_entries.user_legacy_id`, etc.) but never on the sales
document/line tables.

### 3. `sales_documents.operator_id` (a separate, `NOT NULL` compatibility table) — real but not a usable substitute

A second table, `sales_documents` (columns: `id, tenant_id, branch_id,
counter_id, operator_id NOT NULL, shift_id, document_number, status,
total_amount, occurred_at`), does carry a mandatory `operator_id`. It is
populated by the live command-handler write path
(`services/api/internal/httpapi/business.go:1124`) for newly-posted sales
going through the real API, and `salesReadModelQuery`/`invoiceSummaryReadModelQuery`
already `UNION ALL` it in as a "compatibility" branch alongside
`business_documents`. However: **it holds 197 rows total, database-wide,
100% of them in ephemeral `read-model-*` Go-test tenants** — 0 rows in the
sandbox tenant, 0 rows in `demo`. It is a forward-looking compatibility path
for the live POS front end, not a source of historical user attribution; the
291,361 migrated sale documents were never routed through it.

### 4. `legacy_payload->>'PostedBy'` — the JSON key exists but every value is JSON `null`, for every sale-family document, exhaustively

This is the fallback pattern the task specifically asked me to check (the
pattern other reports use of falling back to `legacy_payload->>'X'`). The
key genuinely exists in the migrated shape:

```sql
SELECT jsonb_object_keys(legacy_payload) AS k, count(*)
FROM business_documents
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND kind='cash-sale'
GROUP BY k ORDER BY k;
```
→ 12 keys present on all 291,359 rows, including `PostedBy`. But:

```sql
SELECT legacy_payload->'PostedBy' AS raw, count(*)
FROM business_documents
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND kind='cash-sale'
GROUP BY raw;
```
→ `null | 291359` — every single row has a **JSON `null`** value (not an
empty string, not `"0"` — a literal JSON null), confirmed identically for:
- `cash-sale`: 291,359/291,359 null
- `cash-sale-return`: 30,704/30,704 null (checked via
  `count(*) FILTER (WHERE legacy_payload->'PostedBy' IS NOT NULL AND
  legacy_payload->>'PostedBy' <> '')` → 0/30,704)
- `credit-sale`: 2/2 null (both rows' full `legacy_payload` inspected
  directly: `"PostedBy": null` in both)

I also checked `business_document_lines.legacy_payload` for the same 620,607
sale lines the sandbox reconciliation already validated — its 14 JSON keys
(`DiscPerc, GCode, HSCode, ICode, LooseQty, PackQty, PackUnits, Rate, RowID,
SaleInvcode, SalePrice, SalesTax, UOM, itemflatdisc`) contain **no** user/
operator/posted-by field of any kind at the line level either.

**Conclusion: this is not an unnormalized-but-present field (the pattern
other successfully-promoted reports rely on) — it is a genuinely empty
source column.** The legacy `SaleHdr.PostedBy` (or equivalent) field was
either never populated in the source POS data or was dropped before/during
the ETL that produced `legacy_payload`. There is no version of a
`legacy_payload->>'PostedBy'` fallback that would show anything other than
"Unspecified"/blank for literally 100% of rows — which is not a "user-wise"
report, it is a single-bucket report wearing a user-wise label.

### 5. The 6 non-`net-cash` leaves are not vacuous, they are just generic — confirmed by direct query

Because `salesReadModel: true` is set for every "User Wise" entry
(inherited from the shared loop `add(report.kind, report.title, condition)`
+ `registry[report.kind] = reportSpec{..., salesReadModel: true, salesMode:
mode}` with `mode == ""` for all 6), the dispatch code
(`salesReadModelQueryMode`, `reports.go:2157-2192`) falls through to the
default `salesReadModelQuery(aggregateCondition, pagination)` — which reads
real rows from `business_documents`/`business_document_lines` (UNION
`sales_documents`/`sync_events` for compatibility), **not** the pure
`sync_events`-only generic-fallback path. Confirmed non-vacuous by running
the exact base query for `user-wise-sales`'s `aggregateCondition`
(`reportSaleAggregate`) over `[2026-07-01, 2026-07-02)`:

```
 document | occurred_at             | party                | item                         | quantity    | amount   | operator_id
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | BRUFEN 120ML SUSP            | 1.00000000  | 510.0000 | (null)
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | AUGMENTIN 156.25MG 90ML SYP  | 1.00000000  | 510.0000 | (null)
 868770   | 2026-07-01 23:58:24+05  | CASH SALES CUSTOMER  | VONOLIA 20MG TAB             | 14.00000000 | 521.0000 | (null)
```
Real rows, real amounts — `operator_id` selected alongside for verification,
confirmed `null` on every row. So the 6 leaves are "event-ledger,
non-vacuous, generically-shaped, structurally incapable of a user
dimension" — not "broken" or "empty."

## Per-leaf verdicts

| Leaf | Current state | Diagnostic finding | Verdict |
|---|---|---|---|
| `user-wise-sales` | `event-ledger`, generic 6-col (`document, occurred_at, party, item, quantity, amount`), `salesReadModel: true`, `salesMode: ""` | Re-confirms `PHASE_N_REPORT_PROMOTION_2026-08-09.md`'s finding with database-wide (not just sandbox), exhaustive, multi-path evidence (§1-4 above) | **Hard blocker, confirmed. Correctly left as event-ledger** — matches the decision already made and documented by the promotion-authorized agent. No safe promotion exists. |
| `user-wise-invoice-graph` | `event-ledger`, generic 6-col, same shape | Same root cause; a "graph per user" additionally needs a *count/total series bucketed by user*, which is doubly blocked (no user key to bucket by at all) | **Hard blocker.** Cannot be promoted to any per-user series; would require inventing an "Unspecified" single-bucket pseudo-graph, which is not the report the title promises. |
| `user-wise-category-summary` | `event-ledger`, generic 6-col | Needs both a user key (missing, §1-4) and a category dimension; `master_items.payload->>'Category'` is separately 0/30,052 populated per the sibling `PHASE_N_REPORT_PROMOTION` finding on `category-wise-sales` — a second, independent gap compounding the first | **Hard blocker**, and even a hypothetical future user-attribution fix would still leave this leaf blocked by the separate category-name gap. |
| `user-wise-discount-report` | `event-ledger`, generic 6-col | Discount fields (`item_discount`, `supplier_discount`, `customer_discount`) do exist and are populated on `business_document_lines` — the *discount* data itself is not the problem — but grouping/filtering by user is | **Hard blocker** on the user dimension specifically; discount data itself is available for a future non-user-grouped promotion, out of scope here. |
| `user-wise-net-cash` | Already `financeMode: "gl-cash"` (`reports.go:574`, `phaseQFinancialOverrides`), `projectionStatus: "real"` | **Not blocked by the operator_id gap at all** — it was promoted by another (Phase Q) agent through a different mechanism: posted-only cash-account `gl_journals`/`gl_lines` joined to the chart of accounts (`financeReadModelQuery`, mode `gl-cash`, `reports.go:4236-4243`), not a grouping of `business_documents` by `operator_id`. Its columns are `Journal / Posted / Debit / Credit` (`reports.go:1088-1091`) — **it does not group or filter by user at all**, despite the "User Wise" menu placement. | **Not my finding to promote or fix** (outside the `reports.go` edit rule and outside this leaf-family's data gap) — but worth flagging: this leaf's title/menu placement promises a per-user cash breakdown that its current `gl-cash` implementation does not deliver. It sidesteps the blocker by changing the report's semantics (cash-account ledger) rather than solving the user-attribution problem. Recorded here for visibility, not actioned. |
| `user-wise-sales-commission` | `event-ledger`, generic 6-col | Doubly blocked: no user attribution (§1-4), **and** no commission-rate data or column exists anywhere in the schema at all (`grep -rn commission` across `services/api` matches only this report's title string in `reports.go` and an unrelated legacy-rights-mapping reference) | **Hard blocker**, compounded — even a hypothetical user fix would leave zero source for the commission calculation itself. |
| `user-wise-user-wise-sales-summary` | `event-ledger`, generic 6-col | Same root cause as `user-wise-sales`; a "summary" grouping needs the same missing user key | **Hard blocker.** |

## Net effect

- **6 of 7** assigned leaves (`user-wise-invoice-graph`, `user-wise-sales`,
  `user-wise-category-summary`, `user-wise-discount-report`,
  `user-wise-sales-commission`, `user-wise-user-wise-sales-summary`) are
  correctly left as generic `event-ledger` — confirmed to be non-vacuous
  (they return real rows) but structurally incapable of a per-user
  dimension. This is not a code defect and not a missed promotion
  opportunity; it is a genuine, exhaustively-checked data gap.
- **1 of 7** (`user-wise-net-cash`) is already `projectionStatus: "real"`
  via a different promotion path (Phase Q, `financeMode: "gl-cash"`) that
  does not depend on `operator_id` at all — it changes the report's data
  source to the GL cash-account ledger instead. Its columns do not actually
  group by user despite the menu title; this is flagged for visibility but
  not addressed here (outside this leaf-family's blocker and outside the
  `reports.go` edit permission for this agent).
- **No usable `legacy_payload` fallback exists** for any of the blocked
  leaves. Unlike other leaves in this codebase where a raw legacy key is
  present-but-unnormalized (e.g. `ManfCode` present, `Manufacturer` name
  absent — see `PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md`),
  `PostedBy` is present-as-a-key but its value is JSON `null` for **100% of
  the 322,065 sale-family documents checked** (291,359 cash-sale + 30,704
  cash-sale-return + 2 credit-sale) — there is no raw value to fall back to.
- `go vet ./services/api/...` — clean, no output.
- No `reports.go` edits made; no new test file added (see rationale above).

## Answer to the specific question posed by the task

**Is the `operator_id` gap a hard blocker for the entire "User Wise" family,
or is there a usable fallback?**

**Confirmed hard blocker for 6 of 7 leaves, with no usable fallback of any
kind** (no populated column, no populated parallel table, no populated
`legacy_payload` key) — checked exhaustively across every tenant in the
database, not just the sandbox. The 7th leaf (`user-wise-net-cash`) is
unaffected because it was already promoted via a completely different data
source (GL cash-account ledger) by another agent's Phase Q pass, though that
promotion itself doesn't deliver a true per-user breakdown either — it just
isn't blocked by this particular gap.

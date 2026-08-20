# Phase N misc-A verification — 2026-08-09

Scope: 6 assigned report leaves from `phaseNReportRegistry` (`services/api/internal/httpapi/reports.go`
lines 221-317, Sales Reports loop): `hourly-sales-graph`, `slow-fast-moving-items`,
`net-sale-summary`, `item-wise-item-sale-and-return-activity`, `customer-sales-lp-ledger`,
`customer-sales-claimable-for-allowed-customers`.

This is a read-only diagnostic pass. Per this task's constraints, `reports.go` was **not**
edited — no promotions were made here (a different concurrent agent owns that file for
promotions). Findings below are for that promotion agent to act on.

**Method**: direct code trace of `reportSpecForKey`/`reportDefinitionForKey`/the live query
dispatch switch in `reports.go`, cross-checked against two existing, already-passing tests
(`TestPhaseNReportRegistryDefinitionsAndAggregateFilters` in `server_test.go`,
`TestPhaseQRegistryCoversTheMappedRemainingLeaves` in `report_q_test.go`), plus live
`psql` queries against the sandbox tenant (`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`) to check
for real underlying data sources where the title implied one might exist.

---

## 1. `customer-sales-lp-ledger` — ALREADY effectively real, verified via the finance override (not event-ledger in practice)

**Claim under test**: does the `phaseQFinancialOverrides["customer-sales-lp-ledger"] = "party-customer"`
entry actually make this leaf real/verified today, the same way `customer-statement` is?

**Answer: yes, confirmed precisely, not assumed.** Traced both places that matter:

1. **Definition building** (`reportDefinitionForKey`, `reports.go:1320-1549`): `reportSpecForKey`
   falls through to `phaseNReportRegistry["customer-sales-lp-ledger"]` (line 607-613), which has
   `salesReadModel=true`, `salesMode=""`, then the override merges `financeMode="party-customer"`
   onto that same spec value. Inside the `if explicitEventProjection` block, the code runs
   `spec.salesReadModel` handling first (sets `projectionNote`/`columns` but *not*
   `projectionStatus`, still `"event-ledger"` from line 1397) and then, **unconditionally after
   it** (not `else if`), runs `if spec.financeMode != ""` (line 1487) which overwrites
   `projectionStatus = "real"`, `columns = financeReportColumns("party-customer")`,
   `projectionNote = financeProjectionNote("party-customer")`. Net result:
   `reportDefinitionFor("customer-sales-lp-ledger").ProjectionStatus == "real"`.
2. **Live query dispatch** (the big `switch`/`if` chain around `reports.go:5280-5410`): the
   `spec.salesReadModel && spec.financeMode == ""` branch (line 5338) is skipped because
   `financeMode != ""`; the very next branch, `spec.financeMode != ""` (line 5356), fires and
   calls `financeReadModelQuery("party-customer", "LIMIT $6 OFFSET $7")` — **the exact same
   function and mode string** used by the `customer-statement` alias
   (`phaseQReportAliases["customer-statement"] = {financeMode: "party-customer"}`). The two
   kinds produce byte-identical SQL.

**Independently confirmed by pre-existing, already-passing tests** (not written by me — I only
ran them):
- `TestPhaseQRegistryCoversTheMappedRemainingLeaves` (`report_q_test.go:45-73`) explicitly
  asserts `reportSpecForKey("customer-sales-lp-ledger").financeMode == "party-customer"`.
- `TestPhaseNReportRegistryDefinitionsAndAggregateFilters` (`server_test.go:426-465`)
  special-cases any `phaseNReportRegistry` entry whose `resolvedSpec.financeMode != ""` and
  asserts `definition.ProjectionStatus == "real"` for it — which covers this leaf.
- Ran both plus `TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites`
  (`report_q_test.go:121`, covers `customer-statement`) via
  `go test ./internal/httpapi/... -run 'TestPhaseQRegistryCoversTheMappedRemainingLeaves|TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites|TestPhaseNReportRegistryDefinitionsAndAggregateFilters' -v`
  — all PASS. `go vet ./...` clean.

**`customer-statement` was already golden-verified in wave 1**
(`docs/evidence/PHASE_Q_GOLDEN_VERIFICATION_FINANCE_2026-08-09.md`, section 4: MATCHED, 2/2 rows
exact-reconciled against an independent raw query, with the caveat that the sandbox tenant
only has 2 `counterparty_kind='customer'` ledger rows tenant-wide — genuinely thin data, not
a query defect). Since `customer-sales-lp-ledger` executes the identical
`financeReadModelQuery("party-customer", ...)` call, that same verification transfers
directly — there is no separate SQL to re-verify.

**Verdict: covered.** `customer-sales-lp-ledger` is not a genuine event-ledger gap needing a
diagnostic write-up; it is already `projectionStatus = "real"` in the live code path today,
riding on the already wave-1-verified `party-customer` query. One residual caveat worth
flagging for whoever signs off Phase N/Q as fully done: nothing in the codebase or captured
legacy screenshots confirms that legacy "LP Ledger" *semantically* means "customer party
ledger" (a plausible reading of "LP" as a ledger/party abbreviation, but not textually
proven) — the override was presumably applied by a prior wave-1 agent on that assumption.
That is a documentation-confidence nuance, not a code or query defect.

---

## 2. `hourly-sales-graph` — genuinely event-ledger; mechanical reuse of `hour-summary` is safe, but a real semantic ambiguity blocks promotion

**Current status**: `phaseNReportRegistry["hourly-sales-graph"]` has `aggregateCondition =
reportSaleAggregate` (title has no "return"), `salesReadModel = true`, `salesMode = ""` (no
`case` in the switch at `reports.go:284-317` matches this kind). Renders the generic 6-column
`reportEventLedgerColumns()`, `projectionStatus = "event-ledger"`.

**Reusability check against `customer-sales-hourly-graph`** (already golden-verified,
`docs/evidence/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md` section 5, mode `"hour-summary"`,
verdict MATCHED — 19 hour-buckets on 2025-06-01, byte-identical to an independent
`date_trunc('hour', ...)` query): traced `salesCalendarSummaryReadModelQuery` (`reports.go:2207-2232`).
The generated SQL depends **only** on `aggregateCondition` and `mode`, never on the report
`kind` string itself. `hourly-sales-graph` and `customer-sales-hourly-graph` share the same
`aggregateCondition` (`reportSaleAggregate`). So **if** `hourly-sales-graph` were given
`salesMode = "hour-summary"`, it would produce byte-for-byte the same SQL as the already-MATCHED
`customer-sales-hourly-graph` — this part is mechanically safe, not a guess.

**Why this is still not a safe promotion (confirmed, not just repeated from the prior
declined-candidates note in `PHASE_N_REPORT_PROMOTION_2026-08-09.md`)**: `hour-summary`'s
query groups by `(hour, party)` — i.e. it is an hour-by-customer cross-tab, not a
single hour-of-day series. `hourly-sales-graph` sits as a **top-level** Sales Reports leaf
(`reports.go:268`), not nested under "Customer Sales" the way `customer-sales-hourly-graph`
is (`reports.go:231`). A report literally titled "graph" and not customer-scoped more plausibly
wants one row per hour (a plottable X-axis), not a customer-fanned-out grouping that would
need to be collapsed again by any real chart renderer. No legacy screenshot or capture exists
anywhere in `parity/captures/legacy/` or `docs/` for either leaf to settle this — confirmed via
`grep -ril hourly parity/ docs/` returning no capture artifact, only this text discussion.

**Verdict: event-ledger, correctly so.** Diagnostic note for the promotion agent: the safe
path forward is not to reuse `hour-summary` as-is, but to add a **new** `salesMode` (e.g.
`"hour-only-summary"`) that groups by hour alone (drop `party` from the `GROUP BY` in
`salesCalendarSummaryReadModelQuery`, or add a mode flag), which is a small, low-risk delta
on top of an already-verified query shape — cheaper than inventing new infrastructure, but it
is new code, not pure reuse, and needs the same kind of MATCHED-verdict test the sibling got.

---

## 3. `slow-fast-moving-items` — genuinely event-ledger, correctly left alone; no real threshold data exists

**Current status**: `aggregateCondition = reportSaleAggregate`, `salesReadModel = true`,
`salesMode = ""`, generic event-ledger columns, `projectionStatus = "event-ledger"`.

**Confirmed independently** (re-verifying, not just trusting, the prior declined-candidate
reasoning in `PHASE_N_REPORT_PROMOTION_2026-08-09.md`): a "slow/fast moving" classification
needs either (a) a velocity threshold (units sold per day/week vs. a cutoff or percentile
band) or (b) a comparison against a reorder/minimum-stock level. Checked (b) directly against
the sandbox tenant:

```sql
SELECT count(*) total,
  count(*) FILTER (WHERE payload->>'OptimumQty' NOT IN ('','0')) optimum_nonzero,
  count(*) FILTER (WHERE payload->>'ReorderQty' NOT IN ('','0')) reorder_nonzero
FROM master_items WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';
-- => total 30052, optimum_nonzero 0, reorder_nonzero 0
```

**0 of 30,052 migrated items have a non-zero `OptimumQty` or `ReorderQty`** — confirming (and
sharpening, with an exact count) the `REMAINING_WORK_100_PERCENT_PARITY` doc's note that
"Reorder/Minimum/Optimum-level source fields largely unmigrated or unpopulated." There is no
legacy-defined velocity threshold captured anywhere in this repo's docs, schema comments, or
code, and no reorder-level data exists to derive one from even as a fallback definition.

**Verdict: event-ledger, correctly so — no safe promotion path exists today.** This is a
genuine business-rule gap, not a mechanical one: someone with the legacy PowerBuilder source
(or a captured legacy screenshot showing the actual threshold/period the report uses) needs to
supply the classification rule before any query can be written without fabricating a business
policy.

---

## 4. `net-sale-summary` — genuinely event-ledger; title/placement is ambiguous, and the codebase's own "net sales" siblings don't actually net anything

**Current status**: `aggregateCondition = reportSaleAggregate` (title "Net Sale Summary" has
no "return", so the sale-return aggregate is never included), `salesReadModel = true`,
`salesMode = ""` (no matching `case`). Generic event-ledger columns, `projectionStatus =
"event-ledger"`.

**Diagnostic finding — a real semantic trap worth flagging explicitly**: this leaf sits as a
**top-level** Sales Reports entry (`reports.go:270`), sibling to (not nested under) several
other "Net Sales"-titled leaves that already have salesModes assigned:
`customer-sales-customer-category-wise-net-sales` → `customer-category-summary`,
`customer-sales-monthly-net-sales` / `monthly-net-sales-summary` → `month-summary`,
`customer-sales-customer-category-wise-sales-net-sales-and-volume` → `customer-summary`. **None
of these actually subtract sale-returns from sales** — every one of them (including
`net-sale-summary` if promoted the same way) uses `condition = reportSaleAggregate`, which
excludes the `sale_return` aggregate entirely (the `strings.Contains(title, "return")` gate at
`reports.go:280` only flips to `reportSaleOrReturn` for titles that literally contain the word
"return", which none of the "Net Sales" family do). So today's "Net Sales" naming in this
registry is inherited/copied labeling, not a computed net-of-returns figure — a legacy "Net
Sale Summary" report, by contrast, plausibly *does* mean gross sales minus sale-returns for a
period, which is a materially different (and currently unimplemented anywhere in this
registry) calculation.

**Verdict: event-ledger, correctly so.** Two independent judgment calls block a safe reuse:
(1) which grouping dimension — day, month, customer, or a flat period total — the top-level
"Net Sale Summary" wants, since it isn't nested under a submenu that would hint at one (unlike
`customer-sales-monthly-net-sales`, whose name itself picks `month-summary`); (2) whether
"net" should mean an actual sale-minus-return computation, which no existing `salesMode` in
this codebase currently performs. No legacy capture exists to settle either question. Flagging
both explicitly for the promotion agent rather than guessing.

---

## 5. `item-wise-item-sale-and-return-activity` — genuinely event-ledger; already gets the correct aggregate scope, but no matching projection mode exists yet

**Current status**: title "Item Sale and Return Activity" contains "return", so
`aggregateCondition = reportSaleOrReturn` (`se.aggregate IN ('sale', 'sale_return')`) — this
part is already correctly scoped, unlike a plain sale-only leaf. `salesReadModel = true`,
`salesMode = ""` (no matching `case`). Generic event-ledger columns, `projectionStatus =
"event-ledger"`.

**Diagnostic finding**: the title implies a per-item activity ledger that shows **both**
sale and sale-return movement, most plausibly with a direction/sign so the two can be told
apart (the way the Phase P stock-movement reports already do —
`isStockMovementSummaryMode`/`isStockSalesMode` group by `day/direction/godown/item` over
`stock_ledger`, `reports.go:1467-1472`). No existing `salesMode` in the sales read model does
this: `item-summary` (used by `customer-sales-items-summary`) groups by `(item, customer)` with
no direction column and, per `salesItemSummaryReadModelQuery`'s own comment
(`reports.go:2234-2246`), is scoped to plain sale lines' `line_total`, not a sale/return
delta. Building a genuine "item sale and return activity" mode would mean either (a) a new
`salesMode` that unions sale and sale_return lines per item with a direction/sign column
(structurally similar to the stock-movement pattern, but net-new for the sales read model), or
(b) reusing the stock-side movement pattern against `stock_ledger` instead of the sales read
model if the legacy report is actually stock-activity-shaped rather than sales-summary-shaped
— genuinely ambiguous from the title alone.

**Verdict: event-ledger, correctly so.** Not a minimal-reuse candidate — the closest existing
building block (`item-summary`) is same-aggregate-family but wrong-shape (no return/direction
handling); the actually-matching shape (`stock`-style direction grouping) lives on a different
read model. This needs new query development, which is out of this task's read-only scope; the
diagnostic here is that the correct blueprint to copy is the Phase P stock-movement grouping
pattern, not any existing `salesMode`.

---

## 6. `customer-sales-claimable-for-allowed-customers` — genuinely event-ledger; checked both named candidate data sources, neither is a customer-claim source

**Current status**: `aggregateCondition = reportSaleAggregate`, `salesReadModel = true`,
`salesMode = ""`. Generic event-ledger columns, `projectionStatus = "event-ledger"`. Confirmed
this is a real legacy menu leaf, not a fabricated one — `legacy_rights_mapping.go:347` maps
legacy right-code `1712` to `"Reports, Sales Reports, Customer Sales, Claimable for Allowed
Customers"` (mapped only to a generic `reports.read` permission — no data-shape hint there).

**Checked `group_allowed_scopes`** (as instructed): this table is real and has migrated data —
54 rows tenant-wide with `scope_kind = 'price'` and `legacy_table = 'GroupAllowedPrice'`:

```sql
SELECT scope_kind, count(*) FROM group_allowed_scopes
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' GROUP BY 1;
-- cash_account 43 | godown 33 | header 35 | price 54 | recipient 8
```

But `group_allowed_scopes` is keyed by `role_id`, and those roles are **staff/user rights
groups**, not customer groups — confirmed by joining to `roles`:

```sql
SELECT r.id, r.name, r.legacy_group_id FROM roles r
WHERE r.id IN ('e5c8629a-...','3e0a59b7-...');
--  ADMINISTRATOR (legacy_group_id 2), REMOTE (legacy_group_id 5)
```

Sample `price`-scope rows for those same roles (`scope_key` values like `5:5:4`, `2:5:3`) carry
no customer/party linkage column at all — `group_allowed_scopes` has no `party_id` or
`customer_id` field in its schema (`db/migrations/009_legacy_security_rights.sql:43-59`). This
is the legacy **`GroupAllowedPrice`** concept referenced in `docs/REMAINING_WORK...` Phase G
("Implement GroupAllowedPrice per-customer/group price assignment... not implemented anywhere")
— but on direct inspection it is a **staff-rights-group → allowed price-level permission**
table (which SalePrice#1-10 a given user role may select), not a per-customer/per-customer-group
claimable-price assignment. Phase G's own framing ("per-customer/group") turns out to be
imprecise on the "customer" half once the actual foreign key is inspected — it is a group in
the security-rights sense, not a customer-classification sense.

**Checked `price_policy_tiers`** (as instructed): also real, keyed by `legacy_policy_id`
(`quantity_limit, price, expiry_date, flat_discount, discount_percent` —
`canonical.go:1819-1825`) — an item-level quantity-break pricing ladder (buy N+, get price P),
with **no party/customer column** either. This is the "PricePolicy tiers" side of Phase G, and
it is a pure item-pricing table, unrelated to which customers may claim it.

**Checked for any other "claimable" data trail**: 0 of 30,052 migrated `master_items` rows have
any JSON payload key containing "claim" (`SELECT DISTINCT k FROM master_items, LATERAL
jsonb_object_keys(payload) k WHERE k ILIKE '%claim%'` → 0 rows). The only "claimable" surface
anywhere in the Go codebase is two **preference labels** — `Claimable Disc.%` / `Claimable
Item` (Sale preferences) and `Claimable Discount %` (Quotation preferences) in
`preference_registry.go` — which per Phase V/S are `stored_only`/`not_configured` UI toggles
with zero backend behavior and no associated data table; they don't imply or provide a claims
ledger. `master_parties` (the customer/supplier master) has no group/scheme/claim column at
all — just `party_type, legacy_id, code, name, payload, active`.

**Verdict: event-ledger, correctly so — genuinely no real, verifiable data source exists for
this leaf today.** Neither of the two candidate tables named in this task's brief actually
carries a customer-facing "claimable price/scheme" concept once their foreign keys and
row shapes are inspected directly:
- `group_allowed_scopes`/`GroupAllowedPrice` = staff rights-group → allowed sale-price-level
  permission (54 real rows, but wrong entity — users, not customers).
- `price_policy_tiers` = item-level quantity-break pricing ladder (real rows exist per item
  policy, but no customer linkage at all).

A legitimate legacy `dbo.GroupWiseClaim`-shaped table (or equivalent) most likely exists in the
source PowerBuilder database but was never migrated into any table this Postgres schema
exposes — this is a migration-scope gap, not a report-query bug, and building this leaf without
that source table would mean fabricating a business concept, not deriving one from real data.

---

## Summary table

| Kind | Current `projectionStatus` (dispatch-traced) | Verdict |
|---|---|---|
| `customer-sales-lp-ledger` | **`real`** (via `phaseQFinancialOverrides` → `financeMode="party-customer"`, confirmed by 2 pre-existing passing tests + identical query to the wave-1-MATCHED `customer-statement`) | Already covered — no further action needed |
| `hourly-sales-graph` | `event-ledger` | Correctly event-ledger; `hour-summary` reuse would be mechanically byte-identical to the MATCHED `customer-sales-hourly-graph` but semantically wrong-shaped (adds an unwanted customer fan-out to a top-level "graph" leaf) — needs a new hour-only mode, not pure reuse |
| `slow-fast-moving-items` | `event-ledger` | Correctly event-ledger; confirmed 0/30,052 items have any populated reorder/optimum threshold data, so no velocity/threshold definition is derivable from migrated data |
| `net-sale-summary` | `event-ledger` | Correctly event-ledger; ambiguous grouping (no submenu hint) and the registry's existing "Net Sales" siblings don't actually net returns against sales, so "net" is currently a label, not a computation, anywhere in this codebase |
| `item-wise-item-sale-and-return-activity` | `event-ledger` | Correctly event-ledger; already has the right `sale`+`sale_return` aggregate scope, but no existing `salesMode` groups by item-with-direction — closest blueprint is the Phase P stock-movement pattern, not any sales mode |
| `customer-sales-claimable-for-allowed-customers` | `event-ledger` | Correctly event-ledger; both named candidate tables checked and ruled out with concrete row/schema evidence — `group_allowed_scopes` is a staff-rights table, `price_policy_tiers` is item-only, neither carries a customer-claim concept; likely a genuine migration gap (source table never migrated) |

## Verification run

```
cd services/api && go vet ./...                       # clean, no output
go test ./internal/httpapi/... -run 'TestPhaseQRegistryCoversTheMappedRemainingLeaves|TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites|TestPhaseNReportRegistryDefinitionsAndAggregateFilters' -v
# all PASS
```

No files under `services/api/internal/httpapi/reports.go` were edited (read-only per task
scope). No new Go test file was added — the existing `TestPhaseQRegistryCoversTheMappedRemainingLeaves`
test already covers and asserts the one leaf (`customer-sales-lp-ledger`) that needed
confirmation, so a duplicate test would have added no new coverage.

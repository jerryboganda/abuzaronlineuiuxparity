# Phase N Golden Verification — Group 1 Remainder (2026-08-09)

Scope: 5 assigned leaves under Daily Reports/Sale in
`services/api/internal/httpapi/reports.go` (registry lines ~148-217):

1. `sale-summary-inv-cust-wise` — "Sale Summary Inv. Cust Wise"
2. `sale-detail-inv-wise` — "Sale Detail Inv. Wise"
3. `sale-detail-format-2` — "Sale Detail (Format 2)"
4. `sale-detail-inv-wise-with-diff-col` — "Sale Detail Inv. Wise(with diff.col.)"
5. `sale-summary-machine-and-invoice-range-wise` — "Sale Summary Machine and Invoice Range Wise"

`reports.go` was **not edited** (read-only per task rules — a separate concurrent agent
owns promotions there). No test file was added: none of the 5 leaves turned out to already
carry a real projection, so there was nothing to pin with a golden-number regression test;
adding one against the generic fallback would just be testing `salesReadModelQuery` itself,
already covered by `docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md`'s sibling leaves.

`go vet ./services/api/...` run clean (no output, no changes made — sanity check only,
since this pass made no code edits).

---

## Registry finding (identical root cause for all 5)

All 5 kinds are added once each, in the same `for` loop at `reports.go:148-188`, with
`condition := reportSaleAggregate` (none of them is `refused-sales-detail`). The subsequent
`switch report.kind` (`reports.go:171-176`) only assigns a non-empty `salesMode` to
`sale-summary`, `sale-summary-inv-wise`, `sale-summary-invoice-wise` (→ `"invoice-summary"`)
and `sale-detail` (→ `"line-detail"`). None of the 5 assigned kinds appears in that switch,
so each ends up as `reportSpec{salesReadModel: true, salesMode: ""}`.

Confirmed by grep — each kind string appears exactly once in the whole file (registry
line only, no second definition elsewhere):

```
155: {"sale-summary-inv-cust-wise", "Sale Summary Inv. Cust Wise"},
156: {"sale-detail-inv-wise", "Sale Detail Inv. Wise"},
157: {"sale-detail-format-2", "Sale Detail (Format 2)"},
159: {"sale-detail-inv-wise-with-diff-col", "Sale Detail Inv. Wise(with diff.col.)"},
161: {"sale-summary-machine-and-invoice-range-wise", "Sale Summary Machine and Invoice Range Wise"},
```

At request time (`reports.go:5338-5354`), `spec.salesReadModel && spec.financeMode == ""`
routes to `salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, ...)`
(`reports.go:2161`). That dispatcher's final fallthrough (`reports.go:2195`,
`mode` matches none of the named cases including `""`) is:

```go
return salesReadModelQuery(aggregateCondition, pagination)
```

i.e. the generic event-ledger union of canonical `business_documents`/`business_document_lines`
(`kind IN ('cash-sale','credit-sale')`, `status='posted'`) with the compatibility
`sales_documents`/`sync_events` branch, projected to the fixed 6-column shape
(`document, occurredAt, party, item, quantity, amount`) via `reportEventLedgerColumns()`.
This is confirmed by the response-shaping code at `reports.go:1401-1408`: because
`salesMode` matches neither `"line-detail"` nor any of the summary-mode strings at
`reports.go:1409-1422`, `columns` is never reassigned away from the
`reportEventLedgerColumns()` default set at `reports.go:1399`, and `projectionStatus`
stays `"event-ledger"` (only `adminKind`/`documentReadModel`/`headerReadModel`/
`purchaseReadModel` branches set it to `"real"`; there is no such branch for
`salesReadModel` with empty `salesMode`).

**Verdict on `projectionStatus` for all 5: `event-ledger`, confirmed from code, not inferred.**

---

## Fallback sanity check (applies identically to all 5, since they dispatch to the
same query)

Ran the exact generic-fallback SQL (`salesReadModelQuery("se.aggregate = 'sale'", ...)`,
literal-substituted for `aggregateCondition = reportSaleAggregate`) via `psql` against
`postgres://postgres@127.0.0.1:5432/abuzar_next`, tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch `ffffffff-ffff-ffff-ffff-ffffffffffff`
("Legacy Reference Sandbox"), under:

```sql
SELECT set_config('app.tenant_id', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', false);
SELECT set_config('app.branch_id', '', false);
SELECT set_config('app.allow_tenant_scope', 'true', false);
SELECT set_config('app.authenticating', 'false', false);
```

Result: query executes without error, returns well-formed rows, e.g.:

```
 document |      occurred_at       |        party        |            item             |  quantity   |  amount
----------+------------------------+---------------------+-----------------------------+-------------+-----------
 880233   | 2026-07-31 23:41:03+05 | CASH SALES CUSTOMER | GASTOM 40MG CAP             | 14.00000000 | 420.0000
 880232   | 2026-07-31 23:40:52+05 | CASH SALES CUSTOMER | NESTLE 0.5LTR               | 1.00000000  | 341.0000
 ...
```

Full-window count: **620,615 rows / 291,361 distinct documents** — matches the row count
already independently established for the canonical `business_documents`/
`business_document_lines` sale detail in `docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md`
(same underlying source table, no filter beyond `kind`/`status`). No SQL error, no empty
result, no obviously malformed value observed in the sample. **Confirmed non-erroring,
sane result for the sandbox tenant.**

Caveat carried over from the sibling doc: `sales_documents`/`sync_events(aggregate='sale')`
are both empty for this branch, so only the canonical `business_documents` branch of the
UNION was actually exercised — the compatibility-event branch's correctness is
structurally-reviewed but not data-exercised in this sandbox.

---

## Per-leaf analysis

### 1. `sale-summary-inv-cust-wise` — "Sale Summary Inv. Cust Wise"

The legacy title's "Inv. Cust Wise" qualifier reads as a summary keyed jointly by invoice
and customer — i.e. one row per invoice, with the customer carried alongside it (as
opposed to a pure per-customer aggregate across many invoices). That is functionally the
same shape the already-real `"invoice-summary"` mode already produces: `sale-summary`,
`sale-summary-inv-wise`, and `sale-summary-invoice-wise` all route to
`invoiceSummaryReadModelQuery`, which de-duplicates to one row per document and carries
`party` (customer) on every row — already independently MATCHED against the sandbox data
in `docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md` (#3/#17-19). This leaf is the
strongest promotion candidate of the 5: it is plausible `sale-summary-inv-cust-wise` could
simply be added to the `switch` case alongside its three siblings with zero new SQL,
though the promotion agent should confirm there is no legacy-specific extra grouping tier
(e.g. sub-totaling per customer across multiple invoices within the report, versus a flat
invoice-level list) before assuming byte-identical output is correct — the title's ordering
convention ("Inv. Cust" vs. the sibling reports' "Inv.") could indicate the primary sort key
is customer-then-invoice rather than invoice-then-customer, which the current
`invoiceSummaryReadModelQuery` sort order was not checked to guarantee. The event-ledger
fallback itself returns a sane, non-erroring 6-column result for the sandbox tenant (see
above); it does not currently expose per-invoice/customer grouping at all.

### 2. `sale-detail-inv-wise` — "Sale Detail Inv. Wise"

"Inv. Wise" on a *Detail* (line-level) report implies the output is grouped/sorted by
invoice number — typically one invoice's lines printed together, in legacy PowerBuilder
reports often with a per-invoice sub-header or sub-total row, rather than a flat
chronological line list. The already-real `sale-detail` leaf (`salesMode: "line-detail"`,
`dailySalesDetailReadModelQuery`) already sorts `ORDER BY occurred_at DESC, document,
item_description` (`reports.go:2154`) — so lines belonging to the same document are already
contiguous in the result set as a side effect of the sort key, which gets most of the way
there. What it does **not** do is emit any invoice-level grouping artifact (subtotal row,
document header separator, or a document-scoped running total) — it is a flat line list
that happens to be document-contiguous, not a genuinely grouped report. Whether that
distinction matters depends on whether the legacy report's "Inv. Wise" variant is purely a
sort-order/subtotal presentation difference over the same line data (in which case
promoting to `"line-detail"` verbatim, or with an added subtotal-row option, would cover
it) or introduces a materially different column set — this pass could not determine which
without a captured legacy sample. Noted for the promotion agent as a likely-but-unconfirmed
candidate. The event-ledger fallback itself returns a sane, non-erroring generic 6-column
result for the sandbox tenant.

### 3. `sale-detail-format-2` — "Sale Detail (Format 2)"

"(Format 2)" strongly suggests this is a **presentation/print-layout variant** of the base
Sale Detail report — an alternate column arrangement or print format applied to the same
underlying line-level data — rather than a distinct data grouping or aggregation. Of the 5
assigned leaves this is the one most likely to be a pure formatting fork: the underlying
row set (one row per sale line, posted cash/credit sales) is plausibly identical to
`sale-detail`'s `"line-detail"` mode and `dailySaleDetailColumns()` shape
(`alias, item, price, quantity, discounts, tax, amount, expiry, batch` — already
independently exercised and partially bug-fixed this wave per the top-level progress
notes: the SalesTax 0.00 issue was found and fixed for `sale-detail` itself). If "Format 2"
differs only in column *order*/*visibility*/print layout rather than in underlying values,
the JSON API contract (which doesn't model print layout at all) may make this leaf
indistinguishable from `sale-detail` once promoted — worth flagging to the promotion agent
as a candidate for direct reuse of the `"line-detail"` mode, with the caveat that "Format 2"
could instead denote a genuinely different column set (e.g. a condensed or extended
variant) that wasn't verifiable without a captured legacy screenshot/printout. The
event-ledger fallback itself returns a sane, non-erroring generic result for the sandbox
tenant.

### 4. `sale-detail-inv-wise-with-diff-col` — "Sale Detail Inv. Wise(with diff.col.)"

This combines the invoice-grouping/sort implication of leaf #2 with an explicit "diff.
col." (differential column) qualifier — i.e. it is `sale-detail-inv-wise` **plus one
additional computed comparison column** not present in the base Sale Detail shape. Typical
legacy PowerBuilder usage of a "diff." column on a sale-detail-style report is a
price/rate delta — e.g. Sale Price vs. Purchase/Average Price, or Sale Price vs. a listed
MRP/retail price, exposing the per-line margin or override amount at a glance. The current
`reportRow`/`dailySaleDetailColumns()` shape already carries `SalePrice`, `PurchasePrice`,
and `AveragePrice` fields (`reports.go:22-29`) for *other* report kinds
(`grossProfit`/`marginPercent` also already exist as row fields), so the raw ingredients
for a differential column may already exist in the row struct — but no code path currently
populates or exposes a diff column for this specific kind. Unlike leaves #2/#3, this one is
**not** a pure mode-reuse candidate: even if the invoice-wise sort/grouping question from
leaf #2 is resolved identically, this leaf additionally needs a new computed column wired
into the query and column list before it could be considered covered by promotion — flagging
this explicitly for the promotion agent rather than assuming a plain `"line-detail"` alias
suffices. The event-ledger fallback itself returns a sane, non-erroring generic result for
the sandbox tenant; it does not currently expose any differential/comparison column.

### 5. `sale-summary-machine-and-invoice-range-wise` — "Sale Summary Machine and Invoice Range Wise"

"Machine ... Wise" implies grouping/filtering by POS terminal/register identity — a
dimension distinct from anything the existing `salesSummaryReportColumns` modes expose.
Schema check: `business_documents` does carry a `counter_id` column
(`information_schema.columns` confirms it is the only machine/terminal/register/counter-
named column on that table), which is plausibly the "machine" the legacy title refers to
(PowerBuilder POS terminology commonly calls a till/register a "Counter"). However, for the
sandbox tenant/branch, **every one of the 291,361 posted cash/credit-sale documents shares
the single value `counter_id = 11111111-1111-4111-8111-111111111111`** — there is no
second machine/counter in this data, so even if a `counter_id`-based grouping were wired
up, it could not be exercised or golden-verified with real differentiated data in this
sandbox (the same class of limitation already documented for the customer-category
dimension in `docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md`). The "invoice range
wise" half of the title additionally implies an invoice-number-range retrieval filter,
which is not currently modeled anywhere in `reportRetrieval`/the query parameter set
(only date-range and text filters exist per `reportRetrieval.SupportsDateRange`/
`SupportsTextFilter`). This leaf needs the most net-new work of the 5: a new grouping
dimension (`counter_id`), a new retrieval filter (invoice-number range), and — even once
built — cannot be golden-verified against real multi-machine data in this environment.
Noting this explicitly rather than recommending it as a simple promotion candidate. The
event-ledger fallback itself returns a sane, non-erroring generic result for the sandbox
tenant.

---

## Summary table

| # | Kind | `salesMode` today | `projectionStatus` today | Fallback sane? | Promotion-candidate note |
|---|---|---|---|---|---|
| 1 | `sale-summary-inv-cust-wise` | `""` (generic) | `event-ledger` | Yes — 620,615 rows / 291,361 docs, no error | Strong candidate: likely reuses `"invoice-summary"` verbatim; verify sort-key assumption first |
| 2 | `sale-detail-inv-wise` | `""` (generic) | `event-ledger` | Yes | Likely candidate: `"line-detail"` already sorts document-contiguous; no subtotal artifact exists |
| 3 | `sale-detail-format-2` | `""` (generic) | `event-ledger` | Yes | Likely candidate: probably a print-layout fork of `"line-detail"`, not a data-shape difference |
| 4 | `sale-detail-inv-wise-with-diff-col` | `""` (generic) | `event-ledger` | Yes | Not a plain reuse: needs a new differential/comparison column beyond `"line-detail"` |
| 5 | `sale-summary-machine-and-invoice-range-wise` | `""` (generic) | `event-ledger` | Yes | Needs new work: `counter_id` grouping + invoice-range filter; unverifiable with real data in this sandbox even if built (single-counter tenant) |

**Verdict for all 5 leaves: STILL-EVENT-LEDGER-NEEDS-X** (X = promotion work of varying
size, detailed per-leaf above). None were golden-verified against independently-written
queries because none currently expose a report-specific projection to verify — the fair
comparison is the generic fallback against itself, which was confirmed sane and
non-erroring for all 5 (they are byte-identical dispatch targets). No regression test was
added since there is no real projection yet to pin.

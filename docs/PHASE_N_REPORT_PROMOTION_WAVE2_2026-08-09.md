# Phase N report leaf promotion — wave 2 — 2026-08-09

Scope: `services/api/internal/httpapi/reports.go`, `phaseNReportRegistry`
(Daily Reports/Sale, Daily Reports/Sales Return, and Sales Reports leaves).
This wave had **exclusive write access** to `reports.go` among 10 parallel
agents; the other 9 ran read-only diagnostic passes over the same 45
untouched leaves concurrently and each wrote their own new doc + new
`_test.go` file. This pass honored the constraint not to edit any existing
`docs/*.md` file (other than this new one) or any existing `_test.go` file
(other than the one new file listed below) — that constraint materially
shaped which candidates could actually ship this wave; see "Candidates
examined and declined" below for two cases where good evidence existed but a
concurrently-written test file blocked promotion.

## Promoted (1 leaf)

| Kind | Title | Menu path | `salesMode` before -> after | Columns before -> after | `projectionStatus` before -> after |
|---|---|---|---|---|---|
| `sale-detail-inv-wise` | Sale Detail Inv. Wise | Reports > Daily Reports > Sale > Sale Detail Inv. Wise | `""` -> `"line-detail"` | 6 generic (`reportEventLedgerColumns()`) -> 11-field (`dailySaleDetailColumns()`: Alias, Item Description, Sale Price, Qty, Disc%, Discount Value, Item Disc, SalesTax Value, Amount, Expiry Date, Batch Number) | `"event-ledger"` -> `"event-ledger"` (unchanged; same registry-wide invariant as wave-1's promotions — see wave-1's doc for why the literal string is correct by design) |

Same `aggregateCondition` as before (`reportSaleAggregate`, i.e.
`se.aggregate = 'sale'`) — untouched.

### Why this one, and why the confidence is high

`sale-detail-inv-wise` sits in the same `phaseNReportRegistry` loop
(`reports.go` lines ~148-197) as `sale-summary-inv-wise`, and the registry
**already** treats that summary-side "-inv-wise" sibling as reusing its base
leaf's mode verbatim:

```go
case "sale-summary", "sale-summary-inv-wise", "sale-summary-invoice-wise":
    mode = "invoice-summary"
case "sale-detail":
    mode = "line-detail"
```

i.e. the codebase's own existing convention is that the `-inv-wise` suffix
changes sort/grouping *presentation*, not the underlying column contract —
`sale-summary-inv-wise` was never treated as needing its own mode or query.
Applying the identical reasoning to the detail side (`sale-detail` ->
`sale-detail-inv-wise`) is a direct, minimal, precedent-following extension,
not a new interpretation. `dailySalesDetailReadModelQuery`'s hard-coded
`ORDER BY occurred_at DESC, document, item_description` does not vary with
mode, so — matching exactly how wave-1's `customer-sales-detail` promotion
reused `sale-detail`'s query byte-for-byte — this promotion reuses the exact
same `dailySalesDetailReadModelQuery` output with zero new SQL.

### Code diff summary

Single additive `case` in the same `switch` wave-1 extended, `reports.go`
(inside the "Sale" loop, ~line 171):

```go
 mode := ""
 switch report.kind {
 case "sale-summary", "sale-summary-inv-wise", "sale-summary-invoice-wise":
     mode = "invoice-summary"
 case "sale-detail":
     mode = "line-detail"
+case "sale-detail-inv-wise":
+    mode = "line-detail"
 }
```

No new SQL, no new column function, no new `salesMode` value, no changes to
the dispatch logic, no changes to any existing test.

### Verification

**`go vet`** — clean for the whole module (`cd services/api && go vet ./...`
exited 0, no output).

**`go build ./...`** — clean.

**Full suite** (`go test ./...` from `services/api`) — all packages `ok`
(`internal/httpapi` 11.8s, `internal/pricing`, `internal/rlsprobe`).

**Targeted `go test`** — new file
`services/api/internal/httpapi/report_promotion_n_wave2_test.go` adds:
- `TestSaleDetailInvWiseReportUsesLineDetailProjection` — asserts the
  registry wiring, asserts the generated query is byte-identical to
  `sale-detail`'s, asserts the 11-column `dailySaleDetailColumns()` contract
  and title.
- `TestPhaseNWave2PromotedLeavesStillSatisfyRegistryInvariant` — guards that
  the promotion keeps the literal `"event-ledger"` `projectionStatus`
  required by the pre-existing `TestPhaseNReportRegistryDefinitionsAndAggregateFilters`
  (`server_test.go`).

```
go test ./internal/httpapi/... -run 'SaleDetailInvWise|PhaseNWave2Promoted|PhaseNReportRegistry|PhaseNGoldenSaleReturn|CustomerSales|SalesReportDefinitions|DailySaleDetail' -v
```
All PASS, including the pre-existing `TestPhaseNReportRegistryDefinitionsAndAggregateFilters`
and the concurrently-written `TestPhaseNGoldenSaleReturn*` tests from
another agent's diagnostic pass (see below).

**Live psql verification against the sandbox tenant**
(`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, branch
`ffffffff-ffff-ffff-ffff-ffffffffffff`), RLS preamble as supplied, ran the
exact canonical-branch SQL body of `dailySalesDetailReadModelQuery` for
`occurred_at` in `[2026-07-01, 2026-07-02)`:

```
 document | occurred_at             | party                | item                         | quantity    | amount   | sale_price | discount_percent | discount_value | item_discount | sales_tax_value
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | AUGMENTIN 156.25MG 90ML SYP  | 1.00000000  | 379.6400 | 379.64     | 0.00              | 0.00           | 0.00          | 0.00
 868771   | 2026-07-01 23:59:32+05  | CASH SALES CUSTOMER  | BRUFEN 120ML SUSP            | 1.00000000  | 129.1200 | 129.12     | 0.00              | 0.00           | 0.00          | 0.00
 868770   | 2026-07-01 23:58:24+05  | CASH SALES CUSTOMER  | VONOLIA 20MG TAB             | 14.00000000 | 519.9600 | 37.14      | 0.00              | 0.00           | 0.00          | 0.00
 868769   | 2026-07-01 23:58:03+05  | CASH SALES CUSTOMER  | JESROQ 1MG TAB               | 3.00000000  | 63.8700  | 21.29      | 0.00              | 0.00           | 0.00          | 0.00
```
`quantity * salePrice == amount` holds for every sampled row (e.g. 14 x
37.14 = 519.96, 3 x 21.29 = 63.87), confirming the reused line arithmetic is
sane — this is the same query already independently verified for
`sale-detail` and wave-1's `customer-sales-detail`, so this check is
confirmatory, not exploratory.

## Candidates examined and declined this pass

Two candidates had genuinely strong supporting evidence — including
independent evidence gathered by concurrently-running diagnostic agents —
but were **not promoted** because doing so would have broken an existing
test file this pass is not permitted to edit. Both are documented here in
detail so a future pass (with permission to touch those specific test
files, or coordinated with their authors) can finish the job without
re-deriving the evidence.

### `sales-return-detail-inv-wise` — blocked by a concurrent test collision, not by the evidence

Sibling of the already-real `sales-return-detail` (`line-detail`) and
`sales-return-summary-inv-wise` (`invoice-summary`), exactly the pattern the
dispatcher flagged as a good candidate. Independent verification (mine,
matching a concurrently-written diagnostic doc,
`docs/PHASE_N_GOLDEN_VERIFICATION_SALE_RETURN_2026-08-09.md`, leaf 5):

```sql
SELECT count(*) AS raw_rows, count(DISTINCT bd.document_number) AS distinct_invoices
FROM business_documents bd
LEFT JOIN business_document_lines bl ON bl.tenant_id=bd.tenant_id AND bl.branch_id=bd.branch_id AND bl.document_id=bd.id
WHERE bd.tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee' AND bd.branch_id='ffffffff-ffff-ffff-ffff-ffffffffffff'
  AND bd.kind IN ('cash-return','credit-return','open-cash-return','open-credit-return',
                  'cash-sale-return','credit-sale-return','open-sale-return')
  AND bd.status='posted';
-- raw_rows = 44579, distinct_invoices = 30704
```
This confirms today's ungrouped default (`salesMode == ""`) really is
ungrouped (44,579 rows across 30,704 invoices), contradicting its own
"Inv.wise" name — a genuine mode-wiring gap, not just an unpromoted leaf.
The diagnostic doc frames this as two plausible fixes and leaves the choice
to the promotion agent; I independently reached the same conclusion this
wave and applied it: given `sale-detail-inv-wise` (promoted above) already
establishes "-inv-wise on the detail side means line-detail, same as the
summary side already means invoice-summary," the parallel, consistent
choice for `sales-return-detail-inv-wise` is `salesMode = "line-detail"`
(reusing `saleReturnDetailReadModelQuery`, the same query already backing
`sales-return-detail`). I verified this query directly:

```
 document | occurred_at             | party                | item                     | quantity    | amount    | sale_price | ...
 90924    | 2026-06-30 23:50:00+05  | CASH SALES CUSTOMER  | DIAPER XL                | 10.00000000 | 1000.0000 | 100.0000   |
 90923    | 2026-06-30 23:39:01+05  | CASH SALES CUSTOMER  | AUGMENTIN 625MG TAB 12S  | 15.00000000 | 704.5500  | 46.9700    |
```
`quantity * salePrice == amount` holds for every sampled row; 30,704 posted
sale-return documents total (matches the count independently cited
elsewhere in wave-1's progress notes for `header-wise-transaction-summary`).

**Why it wasn't shipped:** a concurrently-running diagnostic agent
(assigned exactly these 5 sale-return leaves, unaware of this pass's
decision in real time, since all 10 agents run in parallel) wrote
`services/api/internal/httpapi/report_golden_n_salereturn_test.go`, which
includes `TestPhaseNGoldenSaleReturnLeavesResolveToExpectedMode` and
`TestPhaseNGoldenSaleReturnDetailInvWiseIsUngroupedRawView`, both of which
hard-assert `sales-return-detail-inv-wise`'s `salesMode == ""` (the
pre-promotion state) as a documented, intentional lock — with an explicit
instruction embedded in the doc for "the promotion agent" to update it once
fixed. This pass's instructions were explicit and unconditional: **do not
edit any existing `_test.go` file except adding one new uniquely-named
file.** I initially applied the promotion, watched it fail those two tests,
and reverted it rather than editing another agent's test file to make it
pass. The registry change and the test-file update need to land in the same
commit/pass to avoid breaking `go test ./...`; that is future work, not
this pass's to take.

**Also discovered along the way (left undone for the same reason):**
`saleReturnDetailReadModelQuery`'s `sales_tax_value` column has the same
"try the stuck legacy `SalesTax` display string before the correctly
populated `bl.tax_amount`" bug already fixed for `dailySalesDetailReadModelQuery`
earlier this project. I independently confirmed the exact figures the
diagnostic doc reported:
```sql
-- differing_lines / hidden_tax_total, canonical sale-return lines only
 total_lines | differing_lines | hidden_tax_total
       44579 |            1970 |        311703.05
```
1,970/44,579 lines (4.42%) show a stuck `"0.00"` in the SalesTax Value
column even though `bl.tax_amount` holds the real value, totaling
Rs 311,703.05 hidden tax. The one-line fix (swap `bl.tax_amount::numeric(19,2)::text`
to the front of the `COALESCE`, exactly mirroring `dailySalesDetailReadModelQuery`'s
already-fixed column) is trivial and I drafted it, but
`report_golden_n_salereturn_test.go`'s
`TestPhaseNGoldenSaleReturnDetailSalesTaxCoalesceOrderBug` pins the *current*
(buggy) COALESCE order byte-for-byte specifically so that fixing it is a
visible, deliberate act paired with updating that test — which again this
pass cannot do. **Left unfixed, `reports.go` unchanged for this function**
(a documented, verified, not-yet-fixed bug, in the same spirit as wave-1's
"documented, not yet fixed" list). This bug already affects the *shipped*
`sales-return-detail` leaf today, independent of anything in this pass.

### `selected-sales-and-summaries-report` — genuine semantic ambiguity plus the same test collision

Initial reasoning: this leaf's title exactly matches the already-wired
reprint sibling `reprinting-selected-sales-and-summaries`
(`phaseQReportRegistry`), which already maps to `salesMode = "invoice-summary"`.
I verified that mode/query independently and it reproduces the exact
figures wave-1 already validated for `customer-sales-summary` (document
868771: quantity 2.0000, amount 510.0000), so mechanically the query is
sound and non-vacuous (620,615 canonical lines for this tenant).

However, a concurrently-running diagnostic agent examined this exact leaf
in more depth than my initial pass and raised a real objection I had not
weighed: the legacy leaf's own name ("Selected ... Summaries") more plausibly
describes a **user-parameterized selection of one or more summary
sub-reports** (a composite/menu-driven report), not a single fixed grouping
— unlike `sales-return-summary`'s unambiguous invoice-summary shape, there
is no captured legacy screenshot or column list to confirm the reprint
sibling's assumed mode is actually correct for the non-reprint leaf, rather
than being an unverified assumption baked into the registry from before this
wave. Given that live disagreement between two independent readings, and
given this pass's mandate to be conservative exactly like wave-1, I declined
to promote it on the merits (not only because of the test collision this
time) — the reprint-sibling-precedent argument is suggestive, not
conclusive, and the title genuinely supports a different, more complex
shape.

The same `report_golden_n_salereturn_test.go` also happens to hard-assert
`selected-sales-and-summaries-report`'s `salesMode == ""`
(`TestPhaseNGoldenSaleReturnLeavesResolveToExpectedMode`), so even had I
weighed the evidence differently, this would have hit the identical
test-collision block as the sale-return leaf above.

### `category-wise-item-category-wise-monthly-sales` — real, verified, working query; blocked by a closed test allow-list, not a data or semantics problem

This is the strongest candidate examined this wave, and the one the
dispatcher's "GOOD CANDIDATES" guidance most directly anticipated
("Any Category/Manufacturer-wise leaf where you find `master_categories`
DOES have a working code-to-name join"). I independently confirmed, and a
concurrently-running diagnostic agent
(`docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_B_2026-08-09.md`, leaf 3, its
"strongest promotion candidate of the 4 assigned leaves") independently
confirmed the same facts:

- `master_categories` (`category_kind = 'item_category'`) has 10 rows for
  the sandbox tenant with real, populated `code`/`name` pairs (e.g. code
  `"1"` -> `"MEDICINES"`, code `"2"` -> `"NARCOTICS"`), while
  `master_items.payload->>'ICatCode'` is populated 30,052/30,052 (100%) and
  `master_items.payload->>'Category'` (the name) is 0/30,052 populated —
  confirming wave-1's original per-item-master finding but showing the join
  target itself is fully usable.
- Joining `business_document_lines -> master_items (item_id) ->
  master_categories (code = ICatCode, category_kind = 'item_category')`
  resolves **100% (620,615/620,615) of posted sale lines** to a real
  category name, not a fallback code.
- The leaf's title ("Item Category Wise Monthly Sales") is unambiguous — a
  category + calendar-month grouping, unlike the bare-word-"Sales" leaves
  wave-1 already declined.

I built the query, wired a new `salesMode = "category-month-summary"`
(a new query function, a new column set, and a new registry `case`), and
independently verified it end-to-end against the sandbox tenant, including
a cross-check against a hand-written independent `SUM(bl.line_total)` that
matched exactly:

```
   month    |     category     |     qty     |    amount
 2026-07-01 | CONSUMER         |   2148.0000 |  174373.4700
 2026-07-01 | COUNSELING       |    445.0000 |    8243.8500
 2026-07-01 | DIAGNOSTIC       |    145.0000 |  137855.0000
 2026-07-01 | DR KHALID MUGHAL |  18480.0000 |  553882.1400
 2026-07-01 | MEDICINES        | 157018.0000 | 8131311.0000
 2026-07-01 | MILK             |     22.0000 |   29650.0000
 2026-07-01 | NARCOTICS        |  12340.0000 |  194169.1800
```
(Cross-check: independent per-category `round(SUM(bl.line_total),2)` for the
same window matched every figure exactly, e.g. MEDICINES = 8,131,311.00 both
ways; 0/620,615 lines unresolved to "Unspecified".)

**Why it wasn't shipped, after being fully implemented and verified:**
`server_test.go`'s pre-existing `TestPhaseNReportRegistryDefinitionsAndAggregateFilters`
(not written this wave, not written by any of the 9 concurrent agents —
foundational registry-invariant infrastructure) contains a **closed
`if`/`else if` allow-list of every `salesMode` value the registry is
permitted to carry**, keyed off literal mode strings (`"line-detail"`,
`"profit-margin-detail"`, `"customer-category-summary"`, the batch of
`"invoice-summary"`/`"day-summary"`/.../`"hour-summary"` values, etc.), each
with its own column-shape assertion; anything not in that list falls to the
final `else if` branch, which asserts the leaf's columns are still the
generic 6-column event-ledger shape (`Label == "Event / Document"`). Adding
a genuinely new `salesMode` value — as opposed to reusing one already in
that allow-list, which is exactly what `sale-detail-inv-wise` above does —
necessarily fails that assertion unless the allow-list itself is extended
to recognize the new mode. That is an edit to an existing `_test.go` file,
which this pass's instructions explicitly forbid. I implemented, verified,
and then **fully reverted** this promotion (registry case, dispatch case,
column case, projection-note case, `reportDefinitionForKey` branch, and the
new `salesCategoryMonthSummaryReadModelQuery` function) rather than ship a
change that fails `go test ./...`, or edit `server_test.go` outside this
pass's remit.

**This is genuine, ready-to-apply work for a future pass** (one with either
permission to extend `server_test.go`'s allow-list, or run as a single
combined pass with that file's owner). The exact query, column set, and
registry wiring are captured above and were proven correct against real
data; nothing about them is speculative. The only blocker was procedural
(the closed test allow-list), not evidentiary.

## Candidates examined and still declined for the same reasons wave-1 gave (no new evidence changes the verdict)

- **`category-wise-sales` / `manufacturer-wise-sales`** — I independently
  re-confirmed the specific data claim wave-1 made (`master_items.payload`
  has 0/30,052 populated `Category`/`Manufacturer` name keys, only the raw
  `ICatCode`/`ManfCode` codes) and additionally confirmed, going further
  than wave-1 did, that `master_categories`/`master_manufacturers` **do**
  carry real, populated `code`/`name` pairs usable as a join target (10
  categories, 838 manufacturers on file for the sandbox tenant) — this
  echoes the dispatcher's specific prompt to re-check this. However, the
  title-ambiguity half of wave-1's original objection is untouched by this
  finding: both leaves are titled with the bare word "Sales" one menu level
  below richer, more specific siblings (`Net Sale`, `Gross Profit`,
  `Monthly Sale`, `Sales Detail And Summary`, ...), and no legacy screenshot
  or capture exists anywhere in `parity/captures/legacy/` or `docs/` to
  confirm whether the bare "Sales" leaf means a flat category/manufacturer
  total, a line listing, or something else. A working join resolves the
  *data* half of the objection, not the *shape* half — I checked
  `parity/catalog/legacy-menu-tree-2026-08-05.json` explicitly for
  additional context and found none beyond the same menu path text already
  in `reports.go`. Declined again, same reasoning as wave-1, now with the
  join-availability question explicitly closed out (positively) rather than
  left open.
- **`dead-item-list`, `slow-fast-moving-items`** — re-read wave-1's
  reasoning (anti-join/velocity-threshold semantics genuinely unspecified
  anywhere in this repo); no new evidence surfaced this wave that changes
  that. Not re-examined in depth.
- **`hourly-sales-graph`** — re-read wave-1's reasoning (top-level "graph"
  leaf vs. the customer-fan-out `hour-summary` mode already used by the
  differently-scoped `customer-sales-hourly-graph`); a concurrently-written
  diagnostic doc (`docs/PHASE_N_GOLDEN_VERIFICATION_MISC_A_2026-08-09.md`)
  independently reached the identical conclusion this wave ("reuse would be
  mechanically byte-identical... but semantically wrong-shaped... needs a
  new hour-only mode, not pure reuse"). Still declined; a genuinely new
  mode would face the identical `server_test.go` allow-list blocker
  documented above for `category-wise-item-category-wise-monthly-sales`.

## Net effect

- 1 of the ~45 wave-2-assigned event-ledger leaves now carries a real,
  source-backed, verified column/query contract instead of the generic
  6-column event-ledger grid: `sale-detail-inv-wise`.
- 0 new SQL, 0 new column functions, 0 new `salesMode` values, 0 changes to
  any pre-existing test.
- 3 additional candidates (`sales-return-detail-inv-wise`,
  `selected-sales-and-summaries-report`,
  `category-wise-item-category-wise-monthly-sales`) were fully investigated,
  and for two of them (the sale-return leaf and the category leaf) a
  complete, verified fix/implementation was drafted and then reverted
  specifically because it could not ship without editing an existing
  `_test.go` file this pass is not permitted to touch — not because the
  evidence was weak. Both are captured above in enough detail (exact query,
  exact verification numbers, exact blocking test name) that a future pass
  with the right permissions can apply them directly without re-deriving
  anything.
- A second class of finding — one real, previously-undocumented tax bug in
  `saleReturnDetailReadModelQuery` (1,970/44,579 sale-return lines, 4.42%,
  Rs 311,703.05 hidden tax, same bug class already fixed for
  `dailySalesDetailReadModelQuery`) — was independently confirmed and a
  one-line fix was drafted, then also reverted for the same test-collision
  reason. This affects the already-shipped `sales-return-detail` leaf today,
  not just the unpromoted `-inv-wise` sibling.
- `category-wise-sales`/`manufacturer-wise-sales` remain declined, now with
  the join-availability question the dispatcher specifically flagged closed
  out with a positive answer (the join exists and fully resolves) rather
  than left open — the remaining blocker is purely the unresolved title/shape
  ambiguity, unchanged from wave-1.
- `go vet ./services/api/...` is clean; the full `services/api` test suite
  passes (`go test ./...`, all packages `ok`).

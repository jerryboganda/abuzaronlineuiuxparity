# Phase J — stock valuation policy decision (resolved 2026-08-08)

Status: **Resolved.** This supersedes the "human decision, not yet made" note
in `docs/evidence/ACCEPTANCE_EVIDENCE_2026-08-07.md` (§J Stock valuation policy, line
~1427) and the evidence in `docs/evidence/PHASE_J_STOCK_EVIDENCE_2026-08-06.md`. The
human decision was made 2026-08-08: **default the new app to moving-average
costing**, matching legacy. This document records the decision, the evidence
behind it, and the exact implementation.

## Decision

`services/api/internal/httpapi/stock.go`'s `stockAllocationPolicy()` now
defaults (when `ABUZAR_STOCK_ALLOCATION_POLICY` is unset) to a new policy,
`"moving-average"`, instead of `"fifo"`. The old `"fifo"` policy remains
available as an explicit, working alternative for comparison/rollback via
`ABUZAR_STOCK_ALLOCATION_POLICY=fifo`. The old `"legacy"` policy name — which
previously just returned a fail-closed error pending this decision — has been
removed; its evidence comment now documents the `"moving-average"` case
directly, since we now know exactly what "legacy" behavior is and no longer
need to reject it.

## Evidence (unchanged from the 2026-08-06/07 investigation, re-verified 2026-08-08)

Cross-checked against real legacy data (item ICode 3018) on SQL Server
(`dbo.Purdetail`, `dbo.Saledetail`, `dbo.StockReport`):

- Legacy does **not** do FIFO or FEFO batch-ordered stock consumption. Over
  97% of purchase and sale rows carry the placeholder `Batch='.'`, not a real
  batch tag.
- `dbo.StockReport` (legacy's stock valuation table) has **no batch
  dimension at all** — only `Date, GCode, ICode, Stock, PurchasePrice,
  SalePrice, AvgPrice, RecentPurchasePrice, PackUnits`.
- Legacy actually uses **item/godown-level moving weighted-average costing**:
  `Purdetail.NewAvgPrice = (AvgPrice_before × CurrStock_before + PurPrice ×
  PackQty) / (CurrStock_before + PackQty)`, recalculated on every purchase
  regardless of batch tag. Every sale's `Saledetail.AvgPrice` reuses this
  same blended average at the time of sale, uniformly, regardless of which
  "batch" (if any) is tagged.

Batches (`stock_batches`) still matter for **quantity and expiry tracking** —
a real, separate concern from costing. Nothing here removes batch tracking.
Purchases still create/update batches with their own received `unit_cost`
exactly as before (`lockOrCreatePurchaseBatch`,
`projectPurchaseReceiptStock` — unchanged).

## Design implemented

1. **Quantity allocation** (which batch's `on_hand` decrements) keeps using
   the existing FIFO-by-`received_at` batch selection —
   `resolveStockChoices` (explicit allocations) and `fifoStockChoices`
   (auto-fallback) — unchanged in that respect. Legacy has no real
   consumption order to reconstruct, so received-date order is retained
   purely as deterministic housekeeping for the on-hand cache.
   (`services/api/internal/httpapi/stock.go:1001-1047`)

2. **Outbound `unit_cost`** written to `stock_ledger` (and, for sales,
   `stock_allocations`) is **not** the consumed batch's own cost under the
   `moving-average` policy. It is the current weighted-average cost across
   all batches for `(tenant_id, branch_id, item_id, godown_id)` at the
   moment of the movement, computed by the new
   `weightedAverageStockCost` helper
   (`services/api/internal/httpapi/stock.go:88-131`):

   ```sql
   SELECT SUM(sb.on_hand)::text, SUM(sb.on_hand * b.unit_cost)::text
   FROM stock_balances sb
   JOIN stock_batches b
     ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id AND b.id = sb.batch_id
   WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
     AND sb.item_id = $3::uuid AND sb.godown_id = $4::uuid
   ```

   `weighted_average = SUM(on_hand × unit_cost) / SUM(on_hand)`, formatted to
   4 decimal places (`numeric(19,4)`, matching the column scale). This
   reproduces legacy's `NewAvgPrice` formula using data the app already
   tracks (`stock_balances.on_hand`, `stock_batches.unit_cost`) — no new
   schema or running-total column needed.

   Applied to both outbound movement types:
   - Sale (`cash-sale`/`credit-sale`) via `allocateStockChoices`
     (`stock.go:1052-1105`, new `unitCostOverride` parameter), called from
     `projectPostedSaleStock` (`stock.go:914-920`).
   - Purchase-return via `projectPurchaseReturnStock`
     (`stock.go:222-227`, `364-373`, `391-413`).

   Inbound movements (purchase receipt, sale return, open sale return) are
   unchanged: purchase receipts still record the batch's own received cost;
   returns still restore stock at the cost the batch was originally
   allocated/received at.

3. **Zero-on-hand edge case**: if the computed on-hand total is zero (a
   timing race across concurrent postings, or genuinely no stock left),
   `weightedAverageStockCost` falls back to the most recently received
   batch's own `unit_cost` (`ORDER BY received_at DESC, id DESC LIMIT 1`). If
   there is no batch at all for the item/godown, it returns a clear error
   (`"moving-average cost unavailable: no stock batches exist for this item
   and godown"`) rather than silently posting a zero cost.

4. `stockAllocationPolicy()` (`stock.go:27-90`): default changed to
   `"moving-average"`; `"fifo"` kept as an explicit alternative; `"legacy"`
   removed as a policy name (unsupported-policy error now lists `fifo` or
   `moving-average`).

5. **GL/COGS flow**: `finance.go`'s `saleCOGS` (`finance.go:596-613`) already
   reads COGS generically —
   `SUM(stock_allocations.quantity * stock_allocations.unit_cost)` — so it
   picks up the new moving-average cost with **no code change**, because
   `allocateStockChoices` now writes the moving-average cost into
   `stock_allocations.unit_cost` for sales. Verified directly in
   `TestStockMovingAverageCostAppliesToOutboundSale` (below) and by the
   pre-existing `TestFinanceSalePostingIntegration` suite continuing to pass
   unchanged (its fixtures happen to use a single per-item batch cost, so the
   weighted average equals that one cost — no expected-value changes were
   needed there).

   Purchase-return GL posting (`projectPostedPurchaseFinance` in
   `finance.go`) does **not** read `stock_ledger.unit_cost` or
   `stock_allocations` at all — it derives the return's inventory/payable
   amounts from `document.Totals.TotalAmount` (the return document's own
   line pricing). So writing the moving-average cost to purchase-return's
   `stock_ledger` rows is a valuation-accuracy improvement for inventory
   history, but it does not change any GL posting; nothing needed changing
   in `finance.go` for purchase-return either.

## Tests

- `services/api/internal/httpapi/stock_test.go`:
  `TestStockAllocationPolicyFailsClosedBeforeLegacyReconciliation` was
  replaced with `TestStockAllocationPolicyDefaultsToMovingAverage`. It was a
  premise change, not a weakened assertion: the old test asserted `"legacy"`
  must fail closed pending a human decision; that decision has now been
  made, so the new test asserts (a) the unset-env default is
  `"moving-average"`, (b) `"fifo"` remains selectable, (c) `"moving-average"`
  is selectable and case-insensitive, (d) `"legacy"` is no longer a
  recognized name (falls into the generic unsupported-policy error, still
  fails closed — just for a different, correct reason), and (e) unknown
  policy names still fail closed.

- `services/api/internal/httpapi/stock_integration_test.go`: added
  `TestStockMovingAverageCostAppliesToOutboundSale`. Seeds two purchases of
  the same item/godown at different costs:
  - `MA-001`: 6 units @ 5.00 → value 30.00
  - `MA-002`: 4 units @ 8.00 → value 32.00
  - Hand-computed weighted average: `(30.00 + 32.00) / (6 + 4) = 62.00 / 10
    = 6.2000`

  Posts a 3-unit sale (less than `MA-001`'s 6 on-hand, so FIFO quantity
  selection draws exclusively from `MA-001`, whose own cost is 5.00), and
  asserts:
  - the consumed batch is indeed `MA-001` (confirms FIFO quantity selection
    is unchanged),
  - the outbound `stock_ledger.unit_cost` is `6.2000` — not `MA-001`'s own
    5.00, nor `MA-002`'s 8.00,
  - `stock_allocations.unit_cost` is also `6.2000`,
  - `saleCOGS` returns `18.6000` (`3 × 6.2000`) with `hasCost = true`,
    confirming the GL COGS read path picks up the new cost with no
    finance.go change.

  This test needed a small supporting helper,
  `insertInventoryEventAt(..., occurredAt string, ...)`, alongside the
  existing `insertInventoryEvent`: the existing helper hardcodes
  `occurred_at` (which the receiving projection copies verbatim into
  `stock_batches.received_at`) to the same instant on every call, so two
  batches seeded through it would tie on `received_at` and fall back to
  effectively-random `id` ordering for FIFO selection — the test needs
  deterministic received-date ordering to assert which batch was consumed.

  Existing DB-backed suites that reference `unit_cost`/COGS values
  (`TestFinanceSalePostingIntegration`, `TestStockLifecycleIntegration`,
  purchase/sale-return/void-reversal integration tests) all seed a single
  batch cost per item, so the weighted average trivially equals that one
  batch's cost — none needed expected-value changes, and all continue to
  pass under the new default.

## Verification run (2026-08-08)

```
$ go vet ./services/api/internal/httpapi/...
(no output — clean)

$ DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable \
  go test ./services/api/internal/httpapi/... -run 'TestStock' -v
...
--- PASS: TestStockMovingAverageCostAppliesToOutboundSale (1.01s)
--- PASS: TestStockAllocationPolicyDefaultsToMovingAverage (0.00s)
--- PASS: TestStockLifecycleIntegration (1.88s)
    --- PASS: .../post_allocates_FIFO_stock_and_a_replay_is_idempotent (0.04s)
... (all TestStock* pass)
PASS

$ DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable \
  go test ./services/api/internal/httpapi/... -run 'TestFinance|TestPurchase|TestSaleReturn|TestVoidReversal' -v
...
--- PASS: TestFinanceSalePostingIntegration (2.61s)
--- PASS: TestPurchaseVerticalSliceIntegration (2.08s)
--- PASS: TestSaleReturnLifecycleIntegration (1.93s)
--- PASS: TestVoidReversalStockDeltaSign (0.00s)
... (all pass)
PASS
```

## Files changed

- `services/api/internal/httpapi/stock.go` — policy default/rename,
  `weightedAverageStockCost`, `allocateStockChoices` and
  `projectPurchaseReturnStock` cost-override wiring, `fifoStockChoices`
  policy-name check update.
- `services/api/internal/httpapi/stock_test.go` — replaced the fail-closed
  policy test with one asserting the new default and the retired `"legacy"`
  name.
- `services/api/internal/httpapi/stock_integration_test.go` — new
  `TestStockMovingAverageCostAppliesToOutboundSale` and a supporting
  `insertInventoryEventAt` helper.
- `finance.go` — **no change**. `saleCOGS` already reads cost generically
  from `stock_allocations.unit_cost`; purchase-return GL never read
  `stock_ledger.unit_cost` to begin with.

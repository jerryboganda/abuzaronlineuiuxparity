# Phase H sales frontend vertical-slice evidence — 2026-08-06

## Scope

Cash Sale and Credit Sale only. The legacy compatibility queue remains
available for the other transaction surfaces; canonical cash/credit commands
never fall back to that queue when the canonical request fails.

## Implemented

- Removed the sales page's hardcoded 32-row item catalog. The lookup is empty
  until `/v1/items/lookup?q=...` returns active canonical items.
- Item lines can only be populated by selecting an active lookup result and
  carry the canonical item UUID, not a display name.
- Active canonical godowns are loaded from `/v1/master/godown`. Selecting one
  loads `/v1/inventory/availability` per selected item and displays the
  server-authoritative available quantity.
- Cash and credit sales use typed `/v1/documents/cash-sale` and
  `/v1/documents/credit-sale` commands for save, post, save-and-post, and
  void. Commands include idempotency keys, expected versions for updates,
  explicit godown, pricing inputs, canonical line item IDs, and
  `customerId` for credit sales.
- Canonical responses are checked for `accepted` and the expected lifecycle
  status before a success message is rendered. Rejected or failed canonical
  requests remain errors.
- The command UUID and idempotency key are generated together and reused for
  the same logical retry signature. A new action, document, or expected
  version creates a new pair; focused coverage aborts the first response and
  verifies both identities repeat.
- The existing `LegacyMenuBar`, contextual commands, MDI window registration,
  toolbar, history surface, and legacy queue remain intact.

## Focused verification

- `pnpm --filter @abuzar/web check` — passed, 0 errors and 0 warnings.
- `pnpm exec playwright test tests/sales-canonical.spec.ts --workers=1` —
  passed, 4 tests.
- `pnpm --filter @abuzar/web test -- --workers=1` — passed, 37 tests.
- `pnpm --filter @abuzar/web build` — passed, production static build.
- `pnpm exec playwright test tests/phase-cd.spec.ts --workers=1` — passed for
  the contextual sales lifecycle and existing Phase C/D coverage.
- `go test ./services/api/... ./services/edge/... ./migration/...` — passed.
- Re-review retry verification: the four canonical sales tests passed,
  including a lost-response retry with identical `commandId` and
  `idempotencyKey`. The current full suite ran 38 tests; all four sales tests
  passed, while three unrelated purchase/return tests failed in the existing
  workspace state. The production build and Svelte check passed.

## Remaining gaps

- The current stock UI shows available batches as an aggregate. It does not
  yet expose batch selection, expiry detail, allocation editing, transfers,
  adjustments, valuation policy, or a full StockReport projection.
- Sales returns, open returns, quotations, refused sales, and reversal/void
  posting remain on compatibility/event paths; posted canonical sale void is
  correctly rejected by the backend until a stock-reversal workflow exists.
- Purchase, purchase-return, purchase-order, GRN, batch-generation, supplier
  pricing, and purchase finance UI remain follow-up work.
- Historical migration and reconciliation of item, stock, return, purchase,
  ledger, and GL data remain open.

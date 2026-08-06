# Open Sale Return lifecycle evidence (2026-08-06)

The two captured `Sales > Open Sale Return` leaves now use distinct canonical
document kinds:

- `open-cash-return`
- `open-credit-return`

Unlike `cash-return` and `credit-return`, these commands do not accept a source
invoice. Posting requires an authenticated branch godown and an explicit batch
number; an existing batch must be active and unexpired, while a new explicit
batch is created atomically. The API writes stock-in and stock-allocation rows
and runs the same balanced sale-return finance projection (cash or customer
settlement, revenue/tax reversal, and inventory/COGS reversal at the returned
line cost).

The Svelte open-return grid exposes optional batch number, expiry date, and unit
cost fields; these are sent through the canonical line contract and are rendered
only for the two open-return variants, preserving the ordinary cash-sale raster.

Focused evidence:

- `go test ./services/api/internal/httpapi -run 'CanonicalSaleReturn|CanonicalStock|CanonicalNoStock' -count=1`
- `go test ./services/api/internal/httpapi -run TestSaleReturnLifecycleIntegration -count=1`
- `pnpm --filter @abuzar/web check`
- `pnpm --filter @abuzar/web exec playwright test tests/phase-cd.spec.ts --workers=1 --reporter=line --grep "open cash sale return"`

The integration test verifies a source-bound return and an open cash return,
including posted status, balanced finance, idempotent source-bound replay, and
the generated open-return stock batch. Physical printing and the canonical
legacy SQL Server reconciliation remain separate acceptance gates.

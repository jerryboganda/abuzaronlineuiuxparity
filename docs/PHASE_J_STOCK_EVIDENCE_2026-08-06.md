# Phase J stock slice evidence — 2026-08-06

## Scope

This is the first tenant/branch/godown/batch stock slice. It is not a claim
of legacy `StockReport` parity, full valuation parity, or completion of the
legacy stock migration.

## Implemented

- `012_stock_ledger.sql` adds scoped `stock_batches`, immutable
  `stock_ledger` movements, `stock_allocations`, rebuildable
  `stock_balances`, and `stock_balance_rebuilds` metadata.
- Composite tenant/branch keys, forced RLS, batch identity uniqueness, and a
  database trigger reject stock-ledger updates and deletes.
- Cash and credit sale posting now requires an explicit active godown,
  allocates non-expired and unlocked batches, and rejects insufficient or
  ambiguous explicit selection inside the same transaction as the document,
  receipt, immutable sync event, and revision.
- Draft saves do not enter the stock projection. Replayed allocation is
  idempotent at the projection boundary.
- Receiving and inventory compatibility events project into the new ledger
  when their payload has canonical item, godown, batch, and cost identity;
  existing `inventory_movements` projection remains intact.
- Inventory adjustments require an explicit `adjustmentSign` of `-1` or `1`;
  the sign is persisted in `stock_ledger` and used by balance rebuilds.
  Missing or invalid signs fail the whole event projection.
- The shared Svelte adjustment/opening-stock surface now fails closed before
  posting unless it has a canonical item legacy ID, UUID godown, batch number,
  positive quantity with at most four decimals, and (for signed adjustments)
  an explicit `1`/`-1` sign; emitted payload values are trimmed and normalized.
- The same surface now searches active canonical items and lets the operator
  select an active godown from the authenticated master list; the selected
  item legacy ID and godown UUID populate the event instead of relying on
  free-text IDs.
- Guarded availability and balance-rebuild routes were added:
  `GET /v1/inventory/availability` and
  `POST /v1/inventory/rebuild`.
- The item balance read model now requires an explicit tenant/branch/godown
  scope and reads `stock_balances`. `inventory_movements` is only a labeled
  fallback when the requested normalized item/godown scope has no balance
  rows; fallback events must carry the requested `godownId`.
- Allocation policy is `fifo` by default; `legacy` is an explicit, documented
  placeholder using the same stable ordering until legacy evidence is
  reconciled. Unknown policy values are rejected.
- Canonical posted sales, purchases, sale returns, and purchase returns now
  append inverse stock movements on a valid void command; source movements are
  immutable, dependent posted documents block reversal, and replay is
  idempotent. See `docs/PHASE_T_VOID_REVERSAL_EVIDENCE_2026-08-07.md`.

## Evidence observed

- `go test ./services/api/... ./services/edge/... ./migration/...` — passed
  with local PostgreSQL integration enabled.
- `go test ./services/api/internal/httpapi -run
  TestStockLifecycleIntegration -count=1` — passed.
- Ordered PostgreSQL migrations, including `012_stock_ledger.sql`, applied
  twice — passed.
- Integration coverage exercised receiving stock-in, draft no-op, FIFO sale
  stock-out, replay idempotency, expired/locked/insufficient rejection,
  tenant isolation, balance rebuild, signed negative adjustment, and atomic
  rejection of a missing adjustment sign.
- Review follow-up focused tests passed, including the API suite with the
  unrelated `TestReadModelsExposeCanonicalSalesWithoutDuplicateCompatibilityRows`
  report test skipped, plus edge and migration tests.
- The unfiltered API package currently has an unrelated report-read-model
  failure (`/v1/reports/daily-sales-detail` returns `invalid_report_kind`);
  report code was not changed by this adjustment-sign fix.

## Remaining work

- FIFO cost allocation is stored, but valuation/COGS, weighted-average
  behavior, historical reversal equivalence, transfers, adjustments UI, and
  legacy policy reconciliation are not complete.
- The 3.2M-row legacy stock snapshot has not been imported or reconciled by
  godown, batch, item, and valuation metric.
- Full `StockReport` projection/performance parity and historical replay
  reconciliation remain Phase J follow-up work.

## Read-model alignment follow-up — 2026-08-06

- Stock/item report reads now use the normalized `stock_ledger` joined to
  `stock_batches`, with an item/godown-scoped compatibility fallback only when
  no normalized rows exist for that scope.
- Focused Go coverage was added for normalized balance precedence, explicit
  godown scope, and tenant isolation. PostgreSQL integration coverage is
  skipped when `DATABASE_URL` is not configured. With the local disposable
  database URL supplied explicitly, the focused read-model integration test
  passed.

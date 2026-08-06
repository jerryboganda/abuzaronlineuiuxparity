# Phase S ? canonical sale-return evidence (2026-08-06)

This phase covers the bounded cash/credit sale-return lifecycle in the new
Svelte/Go/PostgreSQL application. The PowerBuilder application remains the
visual and workflow reference; this note records only verified new-system
behavior.

## Implemented

- `cash-return` and `credit-return` are registered canonical document kinds.
- A posted return requires a UUID `sourceDocumentId`, an active godown, and a
  posted source `cash-sale` or `credit-sale` in the same authenticated tenant
  and branch.
- Every posted return line requires the canonical `sourceLineId`; its item must
  match that source sale line.
- Credit returns require the same canonical customer as the source credit sale.
- The return cannot exceed the source line's unreturned quantity.
- Previously returned source-batch quantities are excluded, so partial returns
  cannot restore the same source allocation twice.
- Explicit batches must have been allocated by the source sale. Without an
  explicit allocation, the source sale's allocations are consumed in stable
  expiry/batch order.
- Stock balances are restored with immutable `stock_ledger` `in` movements and
  `stock_allocations` rows; replay is idempotent.
- Finance reverses cash/receivable settlement, revenue, output tax, inventory,
  and COGS, and records a `sale-return` party-ledger entry.
- `025_sale_return_reversal_contract.sql` enforces canonical kind/source/line
  relationships, one reversal record per posted return, and tenant/branch RLS.

## Focused evidence

- `go test ./services/api/internal/httpapi -run 'TestSaleReturnLifecycleIntegration' -count=1` ? passed against the disposable local PostgreSQL database.
- `go test ./services/api/internal/httpapi -run 'Canonical|FinanceMigration|StockMigration|PurchaseMigration|ReportDefinition|FallbackReportDefinition' -count=1` ? passed.
- `pnpm --filter @abuzar/web check` ? passed with 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web exec playwright test tests/phase-cd.spec.ts --workers=1 --reporter=line --grep "cash sale return"` ? 1 passed.
- Local API, edge, and web health probes were rechecked after the source and
  binary restart; see the handoff status output for the current process IDs.

## Explicit boundary

Posted source-bound and open sale returns are not voidable. A future
compensating-reversal workflow must reverse both the restored stock and the
finance reversal before that action can be enabled safely. The distinct open
return implementation is documented separately in
`docs/PHASE_H_OPEN_SALE_RETURN_EVIDENCE_2026-08-06.md`.

## Remaining exact legacy parity

- Legacy return windows still have compatibility/event paths outside this
  canonical API slice; their full historical field/layout and invoice-source
  replay comparison is not yet signed off.
- No posted sale or posted return can be voided. The new system intentionally
  fails closed rather than deleting or mutating stock/GL/party-ledger history;
  enabling legacy-equivalent void requires a compensating reversal command and
  an acceptance comparison against the sandbox/reference workflow.
- Quotation/refused canonical documents remain stock/GL-neutral, but their
  legacy refusal/quotation print and downstream workflow parity is still open.

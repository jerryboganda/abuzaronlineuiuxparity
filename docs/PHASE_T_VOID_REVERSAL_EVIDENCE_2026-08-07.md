# Phase T - posted document void reversal evidence (2026-08-07)

## Scope

This phase closes the previously fail-closed gap for canonical posted sales,
sale returns, purchases, and purchase returns. It does not claim that the
PowerBuilder void workflow, historical imported ledgers, or exact UI behavior
has been accepted.

## Implemented contract

- Migration `028_business_document_void_reversals.sql` adds a tenant/branch
  scoped append-only reversal record, permits `void` party-ledger entries,
  preserves one ordinary and one void journal/ledger projection per source
  document, and enables a self-linked reversing GL journal under forced RLS.
- The authenticated `void` document command now changes the canonical document
  status and, in the same transaction, appends inverse stock movements,
  reversed GL lines/journal totals, and reversed party-ledger debit/credit
  values. Source rows remain immutable.
- The command is fail-closed when the source has no completed stock and finance
  projections or when a posted dependent document exists. Replaying the same
  void event is idempotent; a different event cannot create a second reversal.
- The document read model selects the latest finance/party projection so a
  voided document returns the compensating journal summary rather than the
  original posting.

## Verification

- `TestPostedDocumentVoidUsesAtomicCompensatingReversal` passed against the
  disposable local PostgreSQL database: cash sale stock was restored from
  4.0000 to 5.0000, exactly one reversal was appended to each projection, and
  replay created no additional reversal.
- `TestSaleReturnLifecycleIntegration` passed: active over-returns remain
  rejected; a posted source-bound return void appends an inverse `out` stock
  movement, `void-reversal` journal, and `void` party entry.
- `TestPurchaseVerticalSliceIntegration` passed: a purchase with a posted
  dependent return cannot be voided; the posted purchase return can be voided
  and restores stock and supplier balance.
- `go test ./services/api/internal/httpapi -count=1` passed.
- `go test ./services/api/... ./services/edge/... ./migration/... -count=1`
  passed, and the matching `go vet` command passed.
- The migration was applied successfully to the disposable local database;
  PostgreSQL verified `business_document_void_reversals`, the primary-source
  partial index, and the `void` party-ledger check value.
- The focused Playwright cash-sale workflow passed through Save, Post, and the
  visible `Void sale` toolbar action; the mocked UI response reached the
  user-facing “voided successfully” state and submitted `action: "void"`.
- After rebuilding the API binary, `ops/local/status-local.ps1` and live
  health probes returned HTTP 200 with `database=ok` for API and edge and a
  healthy web surface.

## Remaining acceptance boundary

This is canonical new-document behavior, not proof of exact PowerBuilder void
semantics. The following remain open: legacy screen/keyboard/raster comparison
for void and reversal dialogs; historical SQL Server document/ledger import and
reconciliation; fiscal-period, tax-register, valuation, print, hardware, UAT,
cutover, rollback, and full-volume acceptance. Documents without both new
stock and finance projections remain intentionally non-voidable until their
historical mapping is reviewed.

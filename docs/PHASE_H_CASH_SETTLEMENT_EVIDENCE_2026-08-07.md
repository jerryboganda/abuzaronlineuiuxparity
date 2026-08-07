# Phase H - cash-sale settlement evidence

Date: 2026-08-07

This is a bounded legacy-workflow slice for cash-sale settlement. It does not
claim complete SaleLedger replay or exact receipt/print parity.

## Implemented boundary

- The reviewed `dbo.SaleLedger` source contract now retains `PaymentMode`,
  `CashReceived`, `CashTendered`, `CashBack`, and `PaymentAccCode` in the
  historical document payload map.
- The canonical `DocumentDraft`/`DocumentBase` contract exposes a cash payment
  object with received, tendered, change, mode, and optional account code.
- Cash-sale commands default received/tendered to the server-calculated total,
  persist paid amount and zero balance, calculate change with integer-minor-unit
  arithmetic, and reject under-tendered or forged change values.
- Existing canonical pricing snapshots retain the normalized payment object;
  imported cash-sale documents fall back to their retained legacy payload when
  hydrating the document read model.
- The Svelte Cash Sale form now exposes Cash Tendered and read-only Cash Back,
  restores tendered cash from canonical history, includes payment fields in the
  idempotent command signature, and blocks under-tendered submissions before
  posting.

## Focused evidence

- `gofmt -w services/api/internal/httpapi/documents.go services/api/internal/httpapi/documents_test.go`
- `go test ./services/api/internal/httpapi -run 'Test(NormalizeDocumentPayment|WithPaymentPricingSnapshot|LegacyPaymentFromPayload)' -count=1`
- `cmd /c pnpm --filter @abuzar/web check`
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/sales-canonical.spec.ts --list`
- `ConvertFrom-Json migration/maps/phase-e-historical-documents.json`
- `git diff --check`

Results: all listed checks passed; the sales contract discovery found seven
tests, and the single cash-sale runtime contract passed with one worker and no
retries.

No long build, full browser suite, CI flow, SQL Server import, or live database
reconciliation was run for this slice.

## Remaining acceptance evidence

- Execute the reviewed historical document wave and reconcile SaleLedger cash
  received/tendered/back, payment-mode/account, paid, balance, return, and
  receipt counts/totals.
- Confirm payment-mode/account behavior for non-cash and mixed tender paths,
  exact rounding, cash drawer/printer integration, receipt layout, reprint,
  void/refund settlement, and operator UAT.
- Complete the wider open gates: all 763 captured tables, 325+ contextual
  commands, exact 151-report columns/calculations and print/export output,
  historical replay, pixel sweep, hardware acceptance, parallel-day UAT, and
  cutover evidence.

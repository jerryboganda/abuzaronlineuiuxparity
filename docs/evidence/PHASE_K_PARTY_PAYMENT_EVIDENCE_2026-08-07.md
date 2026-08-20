# Phase K/Q - source-backed party payment allocation evidence

Date: 2026-08-07

This is a bounded source-backed settlement projection. It does not claim exact
PowerBuilder payment-entry, allocation, voucher, or print parity.

## Implemented boundary

- `db/migrations/032_historical_party_payment_allocations.sql` stores posted
  state, customer/supplier kind, payment/net/open amounts, payment mode,
  account/check/reference/remarks/user fields, source document identity, raw
  payload, and immutable source identity under tenant/branch RLS.
- The guarded `bulk-historical -wave payments` path reads:
  `dbo.PurPayment` supplier payment rows;
  `dbo.InstallmentReceiptDetail` customer receipt allocations; and direct
  `dbo.SaleLedger`/`dbo.Purledger` amount snapshots only when the matching
  child payment table has no row. The fallback rule prevents an invoice-level
  aggregate from being double-counted with receipt/payment children.
- Canonical party and business-document IDs are resolved opportunistically.
  Missing canonical matches do not discard the source row; its legacy party,
  invoice, source table, row key, and payload remain queryable.
- Customer Statement and Supplier Statement now union posted non-zero source
  payment rows with the existing party-ledger projection. The authenticated
  `/v1/finance/ledger?partyId=...` contract and client method expose the same
  rows and calculate the party balance from both projections. Ledger entries
  carry a source type so payment and adjustment evidence is not confused with
  a canonical posted document entry.
- Party-ledger statements and aging now admit posted historical business
  documents with a non-empty legacy source table even when no canonical
  `gl_journals` row exists; newly posted documents still require their posted
  journal. This keeps the reviewed imported party-ledger wave visible without
  inventing a GL posting.
- The authenticated finance-ledger response now calculates `balanceAfter` as
  a deterministic running debit-minus-credit window over the unified
  canonical, payment, adjustment, and return-allocation stream. Historical
  rows no longer expose a blank balance in a mixed statement.
- The separate `-wave party-adjustments` path retains `dbo.SaleReceivableAdj`
  debit/credit rows in `historical_party_ledger_adjustments`. Posted, dated,
  non-zero rows are included in customer statements and ledger balances; rows
  without a resolvable parent date remain stored but are excluded from dated
  report output.
- The guarded `-wave return-allocations` path separately retains
  `SRAllocationHeader/Detail` customer sale-return allocations and
  `PRAllocationHeader/Detail` supplier purchase-return allocations, including
  source return/invoice identity, allocation/outstanding amounts, posted state,
  and unresolved party links. Bounded statement and finance-ledger rows carry
  `return-allocation`; these rows are intentionally excluded from aging and
  canonical document-balance mutation pending source reconciliation.
- No unsupported top-level Payment/Voucher menu was added: the captured menu
  has report leaves but no payment-entry leaf. A canonical interactive
  settlement command and exact `SaleReceivableAdj`/`PRAllocationDetail`/
  `SRAllocationDetail` semantics remain separate acceptance work.

## Focused evidence

- `gofmt` completed for the payment importer, finance API, report read model,
  and focused tests.
- `go test ./migration/cmd/bulk-historical ./services/api/internal/httpapi
  -run 'TestValidWaveIncludesSourceBackedPaymentsAndWithholding|TestHistoricalPartyPaymentMigrationRetainsCustomerAndSupplierSources|TestPhaseQQueriesArePostedAndScopeBound|TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites|TestFinance(LedgerIncludesSourceBackedPaymentAllocations|ReadsRemainAuthenticated)' -count=1`
  passed.
- `cmd /c pnpm --filter @abuzar/web check` passed with 0 errors and 0
  warnings; OpenAPI YAML parsing passed; and `git diff --check` passed with
  exit code 0 (the repository's existing LF/CRLF notices are non-error
  warnings).
- Focused static coverage also includes migration 034 and the
  `return-allocations` wave contract.

No SQL Server import, live payment count reconciliation, long build, full
browser suite, CI flow, or production deployment was run.

## Remaining acceptance evidence

- Run the reviewed payment and `-wave party-adjustments` waves against the
  isolated source/target and record
  counts and amount totals separately for `PurPayment`, installment receipt
  detail, direct SaleLedger fallback, direct Purledger fallback, and
  SaleReceivableAdj. Reconcile duplicate exclusions and `PaymentAmt`/`NetAmt`
  semantics with the approved legacy operator/report sample.
- Verify source-to-canonical party and invoice match rates, posted/GL voucher
  behavior, payment dates, account mapping, withholding interaction, and
  unresolved legacy rows.
- Reconcile the retained `PRAllocationDetail` and `SRAllocationDetail` rows
  against the source counts/totals, duplicate behavior, legacy posting
  meaning, and report grouping before using them to change aging balances;
  the bounded stream is visible but remains excluded from aging.
- Implement and accept a canonical interactive settlement command/UI only if
  the approved legacy workflow requires a new payment-entry surface; then
  prove idempotency, invoice allocation, open-balance mutation, void/reversal,
  permissions, print/PDF/workbook output, and operator UAT.
- Validate the running-balance projection against approved source statement
  samples, including opening balance, same-timestamp ordering, return
  allocations, and mixed canonical/historical rows.

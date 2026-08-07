# Phase Q - receivables and payables aging evidence

Date: 2026-08-07

This is a bounded source-backed improvement for the captured Receivables Aging
and Payables Aging aliases. It does not claim exact PowerBuilder aging or print
parity.

## Implemented boundary

- The reviewed SaleLedger migration map now retains DueDate in each historical
  business document legacy payload.
- Posted customer party-ledger entries join their posted business document and
  use a valid retained DueDate when present.
- The projection exposes NOT DUE, 0-30 days, 31-60 days, 61-90 days, and
  91+ days buckets relative to the report end date.
- Rows without a valid retained due date remain explicitly UNAGED; no date is
  inferred from the current date or current customer configuration.
- New canonical credit-sale drafts now expose a Due Date field, validate it as
  an ISO calendar date, retain it in the pricing snapshot, and restore it from
  the document response. Receivables Aging prefers historical SaleLedger
  DueDate and falls back to this canonical snapshot for newly entered sales.
- The reviewed Purledger migration map now retains integer CreditDays. Posted
  supplier purchase party-ledger entries derive a bounded due date from the
  purchase date plus that source term; purchase returns and missing/invalid
  terms remain explicitly UNAGED.
- Aging totals now use the retained business-document `balance_amount` for
  both debit and credit party entries. Fully paid migrated invoices therefore
  do not remain in an open aging bucket, while canonical credit-sale and
  purchase rows use their retained open balance. This is an outstanding-
  balance correction; the separate source-backed payment-allocation stream is
  now available to party statements but is not replayed to mutate aging rows.
- Customer and supplier party statements now union posted source-backed
  `historical_party_payment_allocations` rows from the reviewed payment and
  receipt sources. Aging continues to use the retained document open balance
  until invoice-allocation and adjustment semantics are separately reconciled.

## Focused evidence

- go test ./services/api/internal/httpapi -run
  'TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites|TestPhaseQQueriesArePostedAndScopeBound'
  -count=1
- cmd /c pnpm --filter @abuzar/web check
- cmd /c pnpm --filter @abuzar/web exec playwright test
  tests/sales-canonical.spec.ts -g "credit sale requires" --workers=1
  --retries=0 --reporter=line
- cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-q.spec.ts
  --list --grep "Phase Q representative"
- git diff --check

Results: the focused Go due-date/snapshot and Phase Q query tests passed; Svelte
diagnostics reported 0 errors and 0 warnings; OpenAPI YAML parsing passed; and
the credit-sale browser contract passed with one worker and no retries,
asserting `document.dueDate: "2026-08-31"`.

The focused query assertions also passed after the aging projection was changed
to aggregate `business_documents.balance_amount` rather than original party
invoice totals.

The source-backed party payment allocation follow-up is recorded in
`docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.

No long build, full browser suite, CI flow, SQL Server import, or live
database-backed result was run for this slice.

## Remaining acceptance evidence

- Run the reviewed historical document and `-wave payments` paths and reconcile
  SaleLedger due-date, Purledger credit-term, retained balance, payment,
  receipt, direct-fallback, and return counts/totals. Party statements now
  expose source payment rows, but those rows are not yet replayed as canonical
  invoice allocations.
- Confirm legacy bucket boundaries, as-of/date-default behavior, payment
  allocation, credit-note treatment, grouping, and rounding.
- Compare approved legacy report columns and print/PDF/workbook output, then
  complete operator UAT.

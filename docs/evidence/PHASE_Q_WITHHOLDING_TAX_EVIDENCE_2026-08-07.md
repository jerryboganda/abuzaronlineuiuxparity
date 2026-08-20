# Phase Q withholding-tax deduction evidence — 2026-08-07

## Scope

The captured `Reports > Purchase Reports > Withholding Tax Deduction` leaf is
now backed by a distinct payment-level historical projection. It is separate
from purchase-line advance tax so the rebuild does not silently relabel an
`AdvanceTaxAmt` value as withholding.

## Source and target contract

- The captured SQL Server schema identifies `dbo.PurPayment` fields
  `PurPaymentCode`, `Date`, `UserCode`, `PurInvCode`, `Posted`, `WHTaxAccCode`,
  `WHTaxPerc`, `WHTaxBaseAmt`, `WHTaxAmt`, `WHTaxCheckNo`, and
  `WHTaxRemarks`.
- `migration/cmd/bulk-historical -wave withholding` reads those fields
  read-only and joins `dbo.Purledger` only for `SuppCode` and supplier-invoice
  identity.
- Migration `031_historical_withholding_tax.sql` retains payment identity,
  purchase-invoice identity, supplier legacy ID, posted state, account, base,
  rate, amount, check/reference, remarks, user, source row, and raw payload
  under tenant/branch RLS.
- The report query returns six fields: payment, date, supplier (canonical
  name with legacy fallback), purchase invoice/certificate, withholding base,
  and withholding amount. It is posted-only, non-zero amount, date/text
  scoped, and paginated.

## Focused verification

| Check | Result |
|---|---|
| `go test ./migration/cmd/bulk-historical -count=1` | Passed |
| Focused API report/migration tests | Passed: source definition, query contract, migration shape, and registry coverage |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| `git diff --check` | Passed; only existing LF/CRLF normalization warnings |

No SQL Server-to-PostgreSQL import was run, no database-backed live report
result was claimed, and no full build, CI flow, or broad browser suite was run.

## Remaining acceptance evidence

Run the reviewed read-only import, reconcile `PurPayment` source/target
counts and withholding totals, compare representative legacy report rows,
approve the exact DataWindow grouping/filter defaults, and compare print/PDF/
workbook output. Supplier joins, certificate semantics, reversals, and
operator/UAT acceptance remain open until those artifacts exist.

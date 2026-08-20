# Phase L tax/configuration evidence — 2026-08-06

## Implemented bounded slice

- `018_tax_configuration.sql` adds branch-safe, tenant-scoped GST, PCT, and
  advance-tax rates with source/legacy identifiers, inclusive/exclusive mode,
  effective dates, item assignments, party assignments, and forced RLS.
- Guarded API routes provide tax-rate CRUD/read, effective-dated assignment
  replacement/read/delete, and bulk `Apply Item GST` for canonical items.
- Document pricing resolves effective item and customer/supplier assignments
  when explicit taxes are absent. Conflicting effective assignments and
  invalid rates fail closed. Explicit command taxes require `tax.override` and
  are retained only after validation.
- Resolved rates and tax amounts are retained in document pricing and line
  snapshots. Posted sale/purchase finance uses the computed document tax
  snapshot; drafts do not create stock, GL, or party-ledger tax postings.

## Observed verification

- `go test ./services/api/internal/httpapi ./services/api/internal/pricing
  -run 'Test(Tax|Pricing|Document|FinanceTax)' -count=1` passed.
- PostgreSQL integration
  `TestTaxConfigurationResolvesProfilesEffectiveDatesAndPostedGL` passed,
  covering item GST/PCT, customer advance tax, supplier assignment storage,
  effective-date selection, tenant isolation, draft-without-finance,
  posted output-tax totals, and idempotent replay.
- Ordered migrations, including `018_tax_configuration.sql`, were applied
  twice to the local PostgreSQL deployment with exit code 0. Tax tables report
  row security enabled and forced, with permissive tenant plus restrictive
  tenant/branch policies.
- `python -c "import yaml; yaml.safe_load(open('docs/openapi.yaml',
  encoding='utf-8'))"` passed.

## Explicitly open parity work

- Migration of the legacy tax source tables/configuration into the canonical
  tenant is still open; this migration does not invent or claim those rows.
- Tax-register/report parity against the legacy application is still open.
- Historical document replay and exact legacy tax ordering/rule reconciliation
  remain open, as do return/reversal and full historical ledger reconciliation.

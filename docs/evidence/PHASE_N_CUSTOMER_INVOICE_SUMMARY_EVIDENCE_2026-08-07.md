# Customer Sales summary projections evidence — 2026-08-07

The captured Customer Sales summary leaves now use explicit projections over
the canonical sales read model:

- Invoice Summary uses invoice-summary, grouping canonical and unmatched
  compatibility rows once per invoice.
- Days Summary uses day-summary, grouping de-duplicated invoices by calendar
  day and customer.
- Items Summary uses item-summary, grouping canonical/compatibility sale lines
  by item and customer.
- Hourly Graph uses hour-summary, grouping de-duplicated invoices by
  calendar hour and customer. The current contract remains a typed table
  projection until the captured graph behavior is approved.
- Monthly Net Sales and the captured Monthly Net Sales Summary use
  month-summary, grouping de-duplicated invoices by calendar month and
  customer.

All projections sum numeric quantities and amounts, preserve tenant/branch and
posted-only scope, and retain compatibility rows only when no canonical
document identity matches. Their typed six-column contracts expose the
grouping key, date, customer, summary placeholder, quantity, and amount.

Focused checks:

    go test ./services/api/internal/httpapi -run 'Test(InvoiceSummaryReadModelsGroupRowsOncePerDocument|CustomerSalesSummaryReadModelsUseExplicitBuckets|PhaseNReportRegistryDefinitionsAndAggregateFilters)$' -count=1
    cmd /c pnpm --filter @abuzar/web check

The exact PowerBuilder summary columns, grouping labels, hourly graph
rendering, net/return/tax/profit calculations, print/PDF/workbook output,
migrated golden replay, and operator approval remain open. DATABASE_URL was
not configured for a database-backed report replay in this focused pass, so
SQL execution against the local PostgreSQL instance remains an acceptance
boundary.

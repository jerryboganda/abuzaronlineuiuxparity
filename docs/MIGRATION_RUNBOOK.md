# SQL Server to PostgreSQL migration runbook

The existing SQL Server database remains read-only during migration.

1. Generate a schema manifest with `migration/cmd/inspect`.
2. Inventory tables, views, stored procedures, SQL Server system-database dependencies, reports, and external integrations.
3. Define one initial tenant and map existing company/site/godown concepts to branches.
4. Create legacy-ID mapping tables before importing business data.
5. Import reference/master data, then historical ledgers, then current/open transactions.
6. Rebuild SQL Server-specific sequences, date/number semantics, reports, and backup behavior as explicit services.
7. Reconcile counts, stock, receivables, payables, ledgers, totals, and invoice-number ranges.
8. Produce an exception report and require approval before any production cutover.

The importer rejects empty tenant/branch injections and empty mapped identifiers before it opens either database. Keep source and target URLs in protected environment variables; never place them in mapping JSON or parity reports.

No migration command may mutate the source database.

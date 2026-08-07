# Phase E — Sale-line import evidence (2026-08-07)

## Implemented boundary

`migration/cmd/bulksalelines` is now a dedicated, guarded path for the large
canonical `dbo.Saledetail` slice. It preserves the reviewed source identity
(`Saledetail:<SaleInvcode>:<RowID>`), pack/loose quantity expression, source
pricing/tax/expiry/batch fields, raw payload, and tenant/branch scope. It uses
PostgreSQL COPY staging and a set-based join to `SaleLedger` headers and
normalized items before inserting into `business_document_lines`.

The path is restart-safe on `(tenant_id, branch_id, legacy_import_key)`, updates
`legacy_id_mappings` for mapped and exception rows, and records unresolved
header/item dependencies plus invalid or non-positive source rows in
`migration_exceptions`. It refuses the protected canonical source unless the
caller supplies `-allow-canonical`, the provisioned tenant/branch, and a URL
that names `FazalDinPP19DataBaseV2`.

## Focused verification

Passed without a source or target database connection:

```text
go test ./migration/cmd/bulksalelines -count=1
go test ./migration/cmd/import -run 'TestImportConfigAcceptsHistoricalExpressionsAndRangeFeatures|TestStableUUIDIsRestartSafeAndScoped' -count=1
```

The new tests verify the actual implementation contains the `Saledetail`
source, `SaleLedger` dependency, canonical target, legacy mapping,
exception-table, and conflict-key safeguards. They also verify that source
quantity and pricing fields survive exception serialization and that only
positive quantities are eligible for canonical insertion.

## Acceptance boundary still open

The importer was intentionally not executed in this short coding pass. The
approved read-only SQL Server run, target count/amount reconciliation, source
row exception closure, four-row/missing-row investigation, complete sale/
return-document promotion, exact PowerBuilder line calculations, print/PDF/
workbook output, and operator/UAT evidence remain open. This document proves
the executable code path, not completion of the historical source wave.

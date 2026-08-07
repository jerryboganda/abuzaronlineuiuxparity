# Phase E — Purchase-order line import evidence (2026-08-07)

## Implemented boundary

`migration/cmd/bulkorderlines` is a guarded, restart-safe COPY/set-based path
for the reviewed canonical `dbo.PurOrderDetail` slice. It preserves
`PurOrderDetail:<POCode>:<PORowId>`, quantity, rate, line totals, unit cost,
GST, batch/expiry, and the reviewed `DiscPerc`, `BonusQty`, `Stock`,
`ReceiptQty`, and `Remarks` payload fields.

The target join requires the scoped `purchase-order` document from
`PurOrderHeader` and the scoped canonical item. The conflict key is
`(tenant_id, branch_id, legacy_import_key)`; missing dependencies, invalid line
numbers, missing item IDs, duplicate identities, and non-positive quantities
are retained in `legacy_id_mappings` and `migration_exceptions`.

The command refuses the protected canonical source without explicit opt-in,
canonical tenant/branch scope, and a URL naming `FazalDinPP19DataBaseV2`.

## Focused verification

Passed without a source or target database connection:

```text
go test ./migration/cmd/bulkorderlines -count=1
```

Tests verify the actual implementation source/dependency/target safeguards,
exception payload preservation, short-row rejection, and positive-quantity
eligibility.

## Acceptance boundary still open

The importer was not executed in this short coding pass. The approved
read-only source run, order header/detail count and quantity/amount
reconciliation, exception review, exact PowerBuilder order calculations,
report/print output, and operator/UAT evidence remain open.

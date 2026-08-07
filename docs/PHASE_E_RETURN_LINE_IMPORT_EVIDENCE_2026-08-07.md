# Phase E return-line import evidence — 2026-08-07

Status: guarded implementation complete; canonical source execution and
reconciliation not run in this evidence pass.

## Implemented scope

`migration/cmd/bulkreturnlines` exposes two reviewed modes:

- `sale`: read-only `dbo.SRdetail`, scoped to `SRLedger` headers with target
  kind `cash-sale-return`, using `SRdetail:<SRInvcode>:<RowId>`.
- `purchase`: read-only `dbo.PRdetail`, scoped to `PRLedger` headers with
  target kind `purchase-return`, using `PRdetail:<PRInvCode>:<PrRowId>`.

Both modes preserve the reviewed pack/loose quantity expression, price, cost,
GST/tax, batch, expiry, discount, and mode-specific source fields in the line
payload. They require the canonical database plus the provisioned tenant and
branch, stage rows with PostgreSQL COPY, join only scoped target documents and
items, upsert by the reviewed import key, and record invalid/dependency rows in
`legacy_id_mappings` and `migration_exceptions`.

## Focused verification

The following checks passed without opening SQL Server or PostgreSQL:

```text
gofmt -w migration/cmd/bulkreturnlines/main.go migration/cmd/bulkreturnlines/main_test.go
go test ./migration/cmd/bulkreturnlines -count=1
go vet ./migration/cmd/bulkreturnlines
```

The tests verify the fixed source/target contracts, both mode bindings,
mode-specific exception/payload keys, short-row rejection, and positive
quantity enforcement.

## Acceptance still open

No canonical import was executed. An approved read-only source window and
target backup/rollback boundary are still required, followed by source versus
target header/line count, quantity, amount, dependency, duplicate, and open
exception reconciliation using the reviewed return metrics. Exact
PowerBuilder return calculations, stock/party-ledger/GL effects, print output,
full catalog migration, and operator/UAT evidence remain open. This file is
implementation evidence, not a claim of live data completion.

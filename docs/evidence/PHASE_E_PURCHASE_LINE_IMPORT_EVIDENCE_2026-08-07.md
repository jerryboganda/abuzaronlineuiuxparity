# Phase E purchase-line import evidence — 2026-08-07

Status: guarded implementation complete; canonical source execution and
reconciliation not run in this evidence pass.

`migration/cmd/bulkpurchaselines` imports the reviewed `dbo.Purdetail` slice
through PostgreSQL COPY and scoped set-based joins. It preserves the
`Purdetail:<PurInvCode>:<PurRowId>` identity, source quantity expression,
pricing/tax/discount/expiry/batch payload, and invalid/dependency rows in
`legacy_id_mappings` and `migration_exceptions`.

The loader now accepts deterministic zero-based `-from-row` and exclusive
`-to-row` windows. The bounded query orders by purchase invoice, numeric/text
row identity, and item before SQL Server `OFFSET`/`FETCH`; the requested window
is written to the redacted report. This makes the deferred high-volume wave
restartable without claiming that any source window has been executed.

Focused verification passed without opening either database:

```text
gofmt -w migration/cmd/bulkpurchaselines/main.go migration/cmd/bulkpurchaselines/main_test.go
go test ./migration/cmd/bulkpurchaselines -count=1
go vet ./migration/cmd/bulkpurchaselines
```

The tests cover stable ordering, exclusive window arguments, unchanged full-run
query behavior, invalid-bound rejection, source contract, exception payloads,
and positive-quantity enforcement. Canonical execution, count/quantity/amount
reconciliation, exception closure, report/print parity, and operator UAT remain
open.

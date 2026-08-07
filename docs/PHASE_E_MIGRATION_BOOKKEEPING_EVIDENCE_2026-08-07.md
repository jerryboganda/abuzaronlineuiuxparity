# Phase E — Migration bookkeeping evidence (2026-08-07)

The reconciliation command now reports two migration-exception measures:

- `openMigrationExceptionRowCount`: every currently open stored exception row;
- `openMigrationExceptionCount`: distinct open source cases grouped by
  `(source_schema, source_table, legacy_id, reason_code)`.

Bookkeeping status is based on distinct unresolved source cases plus open
ambiguity records, while the raw-row count remains visible for audit. This
prevents superseded retry rows from making the acceptance status appear larger
than the actual unresolved source identity/reason boundary.

Focused verification passed without a database connection:

```text
go test ./migration/cmd/reconcile -count=1
```

The live reconciliation report still requires an approved source/target run.
This change does not close any source exception; it makes the next report
truthfully distinguish duplicate bookkeeping rows from distinct unresolved
cases.

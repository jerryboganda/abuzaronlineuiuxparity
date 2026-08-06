# Reviewed migration maps

Place one reviewed JSON mapping per source tenant in this directory. Do not put connection strings, passwords, or customer data in a mapping file.

Minimal shape:

```json
{
  "tenantId": "target-tenant-uuid",
  "defaultBranchId": "target-branch-uuid",
  "tables": [
    {
      "source": { "schema": "dbo", "table": "LegacyItem" },
      "target": { "schema": "public", "table": "products" },
      "sourceId": "ItemId",
      "targetId": "id",
      "columns": { "legacy_id": "ItemId", "name": "ItemName" },
      "inject": { "tenant_id": "target-tenant-uuid", "branch_id": "target-branch-uuid" },
      "conflictColumns": ["tenant_id", "legacy_id"]
    }
  ]
}
```

The example is illustrative only. Approve source columns, branch mappings, conversions, and reconciliation checks before running the importer.

`reconciliation-metrics.example.json` is a redacted template for count/total checks. Copy it to a reviewed file, replace the illustrative table and column names, and pass it to `migration/cmd/reconcile -metrics`; never put customer values or credentials in the file.

Reviewed Phase E maps may additionally use:

- `sourceDatabase` to bind a map to the non-canonical `AbuzarLegacyReference`
  database; the importer refuses `FazalDinPP19DataBaseV2`.
- `targetIdGenerated` when the target UUID is generated and the legacy key is
  preserved in a separate target column.
- `sourceIdColumns` for a reviewed composite source key.
- `payloadColumns` for explicitly listed source fields stored as JSONB.
- `coerce` for reviewed scalar conversions such as SQL Server tinyint to
  PostgreSQL boolean.

Unlisted source columns are intentionally unmapped. Do not add a catch-all
mapping or infer branch/company semantics from names alone.

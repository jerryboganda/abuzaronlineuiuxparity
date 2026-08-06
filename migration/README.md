# Migration workbench

The source database is treated as read-only. Run the inspector only with a connection URL supplied through a protected environment variable:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...'
go run ./migration/cmd/inspect -out parity/catalog/sqlserver-schema.json
```

The inspector writes metadata only: it never copies credentials into the manifest and never modifies the source SQL Server database. Subsequent migration waves will add table-specific transforms, legacy-ID mappings, branch assignment reports, and reconciliation checks.

After the target schema is provisioned, run the read-only count reconciliation command with protected connection strings:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...'
$env:ABUZAR_TARGET_POSTGRES_URL = 'postgres://...'
$env:ABUZAR_RECONCILE_TENANT_ID = '<target-tenant-uuid>'
go run ./migration/cmd/reconcile -tenant $env:ABUZAR_RECONCILE_TENANT_ID -out parity/catalog/migration-reconciliation.json
```

The report compares source and target table counts and records `matched`, `mismatched`, `missing_target`, or `exception` status. When `-tenant` is supplied, PostgreSQL queries run in a tenant-wide RLS context; without it, use a dedicated privileged read-only reconciliation role. The command never writes to either database.

For business-level reconciliation, pass a reviewed metric configuration. Each metric must be a single read-only `SELECT` returning one numeric value; the optional tolerance is an absolute difference in the source database's displayed units:

```json
{
  "metrics": [
    {
      "name": "sales_total",
      "sourceQuery": "SELECT COALESCE(SUM(total_amount), 0) FROM dbo.sales",
      "targetQuery": "SELECT COALESCE(SUM(total_amount), 0) FROM public.sales_documents",
      "tolerance": 0.01
    }
  ]
}
```

Run it with `-metrics migration/maps/reconciliation-metrics.json` (or `ABUZAR_RECONCILE_METRICS`). Queries are validated as read-only and are never copied into the output report; only metric names, values, differences, status, and redacted errors are emitted. Use separate reviewed metrics for balances, stock, ledgers, totals, and invoice sequences because table counts alone cannot prove financial equivalence.

For the actual import, provide a reviewed declarative mapping file. It must name each source and target table, preserve the legacy ID column, specify an idempotent target conflict key, and inject the target tenant/branch scope where required:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...'
$env:ABUZAR_TARGET_POSTGRES_URL = 'postgres://...'
$env:ABUZAR_IMPORT_CONFIG = 'migration/maps/tenant-one.json'
go run ./migration/cmd/import -out parity/catalog/migration-import.json
```

The importer only selects from SQL Server. Each target row is protected by a savepoint; successful rows create/update `legacy_id_mappings`, while failures create `migration_exceptions` and continue. The report contains counts and redacted connection labels, never credentials or source row values. A mapping file is intentionally mandatory because company/site/godown-to-branch semantics cannot be inferred safely from arbitrary legacy schemas.

## Phase R/E security-data wave

`maps/phase-r-security-data.json` is the reviewed, source-database-bound
security map. It imports the sandbox source `AbuzarLegacyReference` only and
targets the isolated `Legacy Reference Sandbox` tenant
(`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`). It preserves numeric legacy group,
user, right, and composite allow-scope identifiers in typed compatibility
columns and JSONB payloads. Role and user UUIDs are resolved from those
retained legacy IDs; no name-based join is used.

The map includes `Users`, `Groups`, `UserGroups`, `GroupRights`, all seven
unambiguous `GroupAllowed*` tables present in the reviewed source manifest
(Godown, Groups, Header, Price, Recipient, ServiceCategory, and StartupRight),
and the separately inspected `GroupCashAccount` table. Empty source tables
remain mapped and reconcile to zero. Legacy user passwords are deliberately
not imported; users receive a reset-required marker and the source password
column is not selected.

Run with protected connection environment variables:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...AbuzarLegacyReference...'
$env:ABUZAR_TARGET_POSTGRES_URL = 'postgres://...'
$env:ABUZAR_IMPORT_CONFIG = 'migration/maps/phase-r-security-data.json'
go run ./migration/cmd/import -out parity/catalog/phase-r-security-import.json

$env:ABUZAR_RECONCILE_METRICS = 'migration/maps/phase-r-security-metrics.json'
go run ./migration/cmd/reconcile `
  -config migration/maps/phase-r-security-data.json `
  -tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee `
  -out parity/catalog/phase-r-security-reconciliation.json
```

Do not run this map against `FazalDinPP19DataBaseV2`; the importer and
reconciler refuse that canonical source before connecting.

## Phase E reviewed sandbox wave

The first executable wave is deliberately isolated to the target tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` (`Legacy Reference Sandbox`). The reviewed
maps are:

- `maps/phase-e-enterprise-config.json` — company/config/preferences and
  category/group masters.
- `maps/phase-e-core-masters.json` — Item, Customer, Supplier, Manufacturer,
  Godown, PricePolicy, and ItemSuppliers.
- `maps/phase-e-reconciliation-metrics.json` — 17 count checks and 3 reviewed
  item-price/tax totals.

These maps use `payloadColumns` to preserve explicitly reviewed legacy fields
without pretending that the target has typed parity for every source column.
`sourceIdColumns` is used only for the composite ItemSuppliers key. The importer
generates target UUIDs while preserving legacy IDs in target columns and
`legacy_id_mappings`.

Import is intentionally refused for `FazalDinPP19DataBaseV2`; the reviewed map
requires `AbuzarLegacyReference`. Keep both connection URLs in protected
environment variables:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...AbuzarLegacyReference...'
$env:ABUZAR_TARGET_POSTGRES_URL = 'postgres://...'
$env:ABUZAR_IMPORT_CONFIG = 'migration/maps/phase-e-core-masters.json'
go run ./migration/cmd/import -out parity/catalog/phase-e-core-import.json

$env:ABUZAR_RECONCILE_TENANT_ID = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
$env:ABUZAR_RECONCILE_METRICS = 'migration/maps/phase-e-reconciliation-metrics.json'
go run ./migration/cmd/reconcile `
  -config migration/maps/phase-e-core-masters.json `
  -out parity/catalog/phase-e-core-reconciliation.json
```

The enterprise map is run the same way by changing `ABUZAR_IMPORT_CONFIG` and
`-config`. Rerunning a map is idempotent on its reviewed conflict key and does
not modify other tenants or source data. Evidence from the 2026-08-06 run is
recorded in `migration/PHASE_E_STATUS_2026-08-06.md`.

# Migration workbench

The source database is treated as read-only. Run the inspector only with a connection URL supplied through a protected environment variable:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...'
go run ./migration/cmd/inspect -out parity/catalog/sqlserver-schema.json
```

The inspector writes metadata only: it never copies credentials into the manifest and never modifies the source SQL Server database. Subsequent migration waves will add table-specific transforms, legacy-ID mappings, branch assignment reports, and reconciliation checks.

To audit reviewed-map coverage without opening either database, compare the
manifest with every JSON map in `migration/maps`:

```powershell
go run ./migration/cmd/auditcoverage `
  -manifest tmp/canonical-sqlserver-schema.json `
  -maps migration/maps `
  -out parity/catalog/phase-e-map-coverage.json
```

The report lists mapped and unmapped base tables plus overlapping reviewed map
entries. `-fail-on-unmapped` writes the report and exits non-zero until every
manifest table has a reviewed mapping; it is therefore a gate, not a claim
that a map has been imported or reconciled.

After the target schema is provisioned, run the read-only count reconciliation command with protected connection strings:

```powershell
$env:ABUZAR_SOURCE_SQLSERVER_URL = 'sqlserver://...'
$env:ABUZAR_TARGET_POSTGRES_URL = 'postgres://...'
$env:ABUZAR_RECONCILE_TENANT_ID = '<target-tenant-uuid>'
go run ./migration/cmd/reconcile -tenant $env:ABUZAR_RECONCILE_TENANT_ID -out parity/catalog/migration-reconciliation.json
```

The report compares source and target table counts and records `matched`, `mismatched`, `missing_target`, or `exception` status. When `-tenant` is supplied, PostgreSQL queries run in a tenant-wide RLS context; without it, use a dedicated privileged read-only reconciliation role. The command never writes to either database.

Its bookkeeping section exposes both raw open migration-exception rows and
distinct open source cases grouped by source schema/table, legacy ID, and
reason. The reported bookkeeping status uses the distinct unresolved-case
count, while raw rows remain visible so superseded retry records are auditable.

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

Both importer and reconciler accept reviewed `-from-table`/`-to-table` ranges
for bounded waves. Canonical mappings that declare branch or counter scope
also require explicit `-branch-id`/`-counter-id` overrides when
`-allow-canonical` is used.

Pass `-promote-normalized` after a compatibility-master wave to refresh that
tenant's normalized item/party/manufacturer/category/godown/item-supplier
targets before importing document lines. The operation is tenant-scoped,
idempotent, and target-only.

`-upsert` is a separate explicit override for a reviewed rerun when existing
target rows need their payload/legacy IDs refreshed; the default remains
immutable conflict handling.

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

By default the importer refuses `FazalDinPP19DataBaseV2`; the reviewed map was
originally authored against `AbuzarLegacyReference`. A canonical run is an
explicit, auditable exception: pass `-allow-canonical` **and** a dedicated
`-tenant-id` (and `-branch-id`/`-counter-id` when the map has those scopes).
The importer rewrites only declared `tenant_id`/`branch_id`/`counter_id`
injections, never source values,
and still performs read-only SQL Server selects. Keep both connection URLs in
protected environment variables or protected command shells:

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

## Canonical first-tenant wave (2026-08-06)

The local SQL Server service exposed the protected canonical database through a
Windows-authenticated, read-only connection. The inspector recorded 763 base
tables and 10,890 column records in `tmp/canonical-sqlserver-schema.json`.
The canonical source tables referenced by both reviewed Phase-E maps were
verified present before import.

The first tenant is isolated from the sandbox and existing demo tenants:

| Scope | Identifier |
|---|---|
| Tenant `LEGACY_CANONICAL` | `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01` |
| Branch `MAIN` | `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02` |
| Counter `COUNTER-1` | `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a03` |

The bounded run used the reviewed maps with explicit canonical opt-in and
scope overrides:

```powershell
$source = 'sqlserver://localhost?database=FazalDinPP19DataBaseV2&trusted_connection=yes'
$target = 'postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable'
go run ./migration/cmd/import `
  -source $source -target $target `
  -config migration/maps/phase-e-enterprise-config.json `
  -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -promote-normalized `
  -out parity/catalog/canonical-first-tenant-enterprise-import.json

go run ./migration/cmd/import `
  -source $source -target $target `
  -config migration/maps/phase-e-core-masters.json `
  -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -out parity/catalog/canonical-first-tenant-core-import.json
```

Canonical reconciliation uses the same explicit guard and target tenant:

```powershell
go run ./migration/cmd/reconcile `
  -source $source -target $target `
  -config migration/maps/phase-e-core-masters.json `
  -tenant 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -allow-canonical `
  -out parity/catalog/canonical-first-tenant-core-reconciliation.json
```

When `-allow-canonical` is supplied, the reconciler also rewrites the reviewed
sandbox tenant literal in metric target queries to the explicit `-tenant`
value. This keeps the checked metric file reusable without changing its source
queries or allowing arbitrary SQL rewriting.

For a branch-scoped historical slice, provide all three operational overrides
and a reviewed table range, for example `phase-e-historical-documents.json`
with `-from-table 3 -to-table 5`, `-branch-id`, and `-counter-id`. The current
canonical evidence includes the two purchase-header mappings, the separate
purchase-order header mapping, the posted purchase-return header, and its
detail lines, plus the sale-return header and detail slices. The bounded
purchase-return line wave uses the focused metric file
`maps/phase-e-purchase-return-line-reconciliation-metrics.json`, and the
sale-return line wave uses
`maps/phase-e-sale-return-line-reconciliation-metrics.json`, so each report
contains only the relevant header/line count, total, and quantity checks:

```powershell
go run ./migration/cmd/reconcile `
  -source $source -target $target `
  -config migration/maps/phase-e-historical-documents.json `
  -metrics migration/maps/phase-e-purchase-return-line-reconciliation-metrics.json `
  -tenant 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -counter-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a03 `
  -allow-canonical -from-table 9 -to-table 10 `
  -out parity/catalog/canonical-first-tenant-purchase-return-lines-reconciliation.json
```

Purchase-order/detail lines, sales, sale returns, stock, and ledger ranges are
still intentionally not implied by those reports; the purchase-detail line
range is documented by the dedicated loader below.

The reviewed purchase-order map is ready for a bounded header/detail wave. Run
the header first, then the detail range, and use the dedicated metrics so a
partial line load cannot be mistaken for a complete order projection:

```powershell
go run ./migration/cmd/import `
  -source $source -target $target `
  -config migration/maps/phase-e-historical-orders.json `
  -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -counter-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a03 `
  -from-table 0 -to-table 1 `
  -out parity/catalog/canonical-first-tenant-purchase-order-headers-import.json

go run ./migration/cmd/import `
  -source $source -target $target `
  -config migration/maps/phase-e-historical-orders.json `
  -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -counter-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a03 `
  -from-table 1 -to-table 2 `
  -out parity/catalog/canonical-first-tenant-purchase-order-lines-import.json

go run ./migration/cmd/reconcile `
  -source $source -target $target `
  -config migration/maps/phase-e-historical-orders.json `
  -metrics migration/maps/phase-e-historical-order-reconciliation-metrics.json `
  -tenant 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -allow-canonical `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -counter-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a03 `
  -fail-on-open-bookkeeping `
  -out parity/catalog/canonical-first-tenant-purchase-orders-reconciliation.json
```

The canonical source connection is an acceptance boundary when it is not
available; these commands are documented and guarded, not claimed as executed
by the local evidence.

The lookup-free canonical `PricePolicyDetail` range has a dedicated bulk
loader because the generic savepoint importer is intentionally conservative
for heterogeneous maps:

```powershell
go run ./migration/cmd/bulkpricepolicy `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -out parity/catalog/canonical-first-tenant-price-policy-import.json
```

It reads SQL Server with `SELECT` only, copies into a temporary PostgreSQL
staging table, upserts `price_policy_tiers`, and refreshes the audited
`legacy_id_mappings` rows without sending any database credentials to clients.

The canonical item tax references use the bounded `bulkitemtax` loader:

```powershell
go run ./migration/cmd/bulkitemtax `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -out parity/catalog/canonical-first-tenant-item-tax-import.json
```

It validates every referenced item and tax-rate dependency before copying and
upserting 30,052 GST plus 30,052 PCT assignments. Its focused reconciliation
is `parity/catalog/canonical-first-tenant-item-tax-reconciliation.json`.

The large canonical purchase-detail line range uses the set-based
`bulkpurchaselines` loader:

```powershell
go run ./migration/cmd/bulkpurchaselines `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -out parity/catalog/canonical-first-tenant-purchase-lines-import.json
```

It uses a read-only SQL Server cursor, PostgreSQL COPY, and set-based joins to
the imported purchase headers and items. Non-positive legacy quantities remain
auditable exceptions; they are never silently changed into positive stock.
The loader accepts deterministic zero-based `-from-row` and exclusive `-to-row`
windows (`-to-row -1` reads through the end) and records the window in its
redacted report. Stable purchase-invoice/numeric-row/text-row/item ordering is
applied before SQL Server `OFFSET`/`FETCH`, so the deferred 113k-row wave can
be retried in bounded slices after a capacity or dependency failure.
The focused reconciliation is
`parity/catalog/canonical-first-tenant-purchase-lines-reconciliation.json`.

The Item Form's `Set Alternate Item Alias Names` command uses the canonical
`master_aliases` store with the separate `alternate_alias` kind. The bounded
`/v1/master/item/{id}/aliases` GET/PUT contract replaces only alternate names,
retains the primary alias/barcode rows, and updates item payload metadata for
repeatable migration and later master saves.

The corresponding high-volume canonical sales-detail slice has a dedicated
set-based loader in `bulksalelines`:

```powershell
go run ./migration/cmd/bulksalelines `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -out parity/catalog/canonical-first-tenant-sale-lines-import.json
```

The high-volume loader can be resumed in deterministic, zero-based source-row
windows without changing the reviewed legacy identity. For example, a bounded
slice of 50,000 rows is selected with `-from-row 100000 -to-row 150000`; the
report records both bounds. The source query orders by invoice, numeric row
number, text row number, and item identifier before applying SQL Server
`OFFSET`/`FETCH`, so a failed slice can be retried without opening the entire
620k-row wave in one run. Use the unbounded defaults only after the bounded
source/target capacity check is approved.

It reads only `dbo.Saledetail`, stages rows with PostgreSQL COPY, joins them
only to the scoped `SaleLedger` headers and normalized items, and upserts by
the reviewed `Saledetail:<SaleInvcode>:<RowID>` identity. Missing headers/items,
invalid line numbers, missing item IDs, duplicate identities, and
non-positive pack/loose quantities remain in `legacy_id_mappings` and
`migration_exceptions`; no quantity is silently coerced. This command is a
code-path safeguard only until an approved source run and count/amount
reconciliation are performed.

The reviewed purchase-order detail slice has the same bounded treatment through
`bulkorderlines`:

```powershell
go run ./migration/cmd/bulkorderlines `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -out parity/catalog/canonical-first-tenant-purchase-order-lines-import.json
```

The order-line loader also accepts deterministic `-from-row` and `-to-row`
windows (zero-based, exclusive end) and records the window in its redacted
report. Stable purchase-order/row/item ordering is applied before SQL Server
`OFFSET`/`FETCH`, so the deferred 113k-row wave can be retried in bounded
slices after a capacity or dependency failure.

It reads only `dbo.PurOrderDetail`, uses the reviewed `PurOrderDetail:<POCode>:<PORowId>`
identity, and joins only `purchase-order` documents from `PurOrderHeader` plus
scoped canonical items. Source batch/expiry, discount, bonus, stock, receipt,
and remarks fields remain in the line payload; invalid, duplicate, missing
dependency, and non-positive rows are recorded as migration exceptions.
Source execution and order count/quantity/amount reconciliation are still
required before this wave can be accepted.

The reviewed sale-return and purchase-return detail slices use the guarded
`bulkreturnlines` loader. Select one fixed mode per run; the command never
accepts an arbitrary source table or document kind:

```powershell
go run ./migration/cmd/bulkreturnlines `
  -kind sale -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -out parity/catalog/canonical-first-tenant-sale-return-lines-import.json

go run ./migration/cmd/bulkreturnlines `
  -kind purchase -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -out parity/catalog/canonical-first-tenant-purchase-return-lines-import.json
```

Both modes also accept deterministic zero-based `-from-row` and exclusive
`-to-row` windows; `-to-row -1` reads through the end. The window is recorded
in the redacted report and applied after stable return-id/numeric-row/item
ordering, so either return wave can be resumed in bounded slices after a
capacity or dependency failure.

Sale mode reads `dbo.SRdetail`, joins only `SRLedger` documents of kind
`cash-sale-return`, and preserves the reviewed `SRdetail:<SRInvcode>:<RowId>`
identity. Purchase mode reads `dbo.PRdetail`, joins only `PRLedger` documents
of kind `purchase-return`, and preserves
`PRdetail:<PRInvCode>:<PrRowId>`. Both modes use PostgreSQL COPY staging,
scoped set-based upserts, positive-quantity validation, idempotent conflict
handling, and auditable mappings/exceptions. Source execution and return-line
count/quantity/amount reconciliation remain required before acceptance; see
`docs/PHASE_E_RETURN_LINE_IMPORT_EVIDENCE_2026-08-07.md`.

The two small tax-rate mappings use the generic bounded importer and
reconciler with `-from-table 0 -to-table 2`; their focused evidence is recorded
under `parity/catalog/canonical-first-tenant-tax-rates-*`.

The final evidence is in
`migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md`. This is the first
canonical master-data wave, not a claim that all 763 legacy tables, documents,
ledgers, stock/batches, reports, hardware, or pixel-level workflow captures
are complete.

## Canonical historical stock and GL wave

`cmd/bulk-historical` supports the reviewed `StockReport`, `VirtualGl`,
payment-allocation, and payment-level withholding projections
historical projections for either the isolated reference sandbox or an
explicitly approved canonical tenant. The canonical path is fail-closed: it
requires `-allow-canonical`, `-tenant-id`, and `-branch-id`, validates both
scope identifiers as UUIDs, and refuses to commit a stock batch if any source
row lacks an imported item or godown dependency. This prevents the previous
silent-loss behavior of an inner-join insert.

The canonical invocation is:

```powershell
$source = 'sqlserver://localhost?database=FazalDinPP19DataBaseV2&trusted_connection=yes'
$target = 'postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable'
go run ./migration/cmd/bulk-historical `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -wave both
```

The loader remains read-only against SQL Server and is not evidence of a
successful import until its source-row counts, target counts, and reviewed
StockReport/VirtualGl totals are recorded. A failed source connection or a
missing dependency must remain an open acceptance boundary.

The reviewed security map (`maps/phase-r-security-data.json`) additionally
declares `"upsert": true`. This is limited to the representational roles,
legacy groups, users, memberships, rights, and allow-scope rows because the
tenancy migration seeds four role shells for every tenant. The upsert refreshes
legacy IDs/payloads on those shells without importing source passwords; all
other reviewed historical/master maps remain immutable (`DO NOTHING`) on a
conflict key.

## Canonical item-history and adjustment report wave

`cmd/bulk-historical` also supports `-wave history`, `-wave adjustments`,
`-wave deleted-sale-items`, and `-wave withholding`. The withholding wave
reads the reviewed `dbo.PurPayment` payment-level `WHTax*` fields and joins
`dbo.Purledger` only to retain supplier/invoice identity; it never derives
withholding rows from purchase-line `AdvanceTax`. The deleted-sale-items wave
reads the reviewed
`dbo.DeletedSaleItem` source table in deterministic source-row order, retains
the sale invoice, item/godown, quantity/bonus, pricing, discount/tax, machine,
user, and raw payload fields in `historical_deleted_sale_items`, and refuses to
commit rows missing the captured identity/date fields.
The history wave reads the reviewed `dbo.ItemLog` snapshot stream in source
order, retains every source column in `historical_item_changes.payload`, and
derives separate source-backed rows for prior-observed price, name, basic-data,
price-difference, and first-observed views. The adjustment wave reads
`dbo.AdjHeader` plus `dbo.AdjDetail`, retains the header/detail payload, and
does not discard a detail row merely because its current item or godown master
is absent in PostgreSQL.

Both waves are read-only against SQL Server and use the same canonical
fail-closed opt-in and tenant/branch scope as the stock/GL loader:

```powershell
go run ./migration/cmd/bulk-historical `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -wave all
```

The report API labels these projections as normalized source-backed and keeps
the exact PowerBuilder DataWindow columns, calculated semantics, filters,
orientation, and print raster as open acceptance evidence until the six
runtime captures are available.

## Party payment allocation wave

`-wave payments` retains source settlement rows in
`historical_party_payment_allocations`: supplier `dbo.PurPayment`, customer
`dbo.InstallmentReceiptDetail`, and direct SaleLedger/Purledger amount
snapshots only when the corresponding child payment stream has no row. The
wave resolves canonical party/document IDs when available but preserves
legacy identities when the historical document or master import is incomplete.
It is read-only against SQL Server and requires the same explicit canonical
opt-in and tenant/branch scope as the other historical waves:

```powershell
go run ./migration/cmd/bulk-historical `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -wave payments
```

The wave does not claim invoice allocation, SaleReceivableAdj/return
allocation semantics, canonical interactive payment entry, or exact legacy
statement print parity until source reconciliation and operator evidence are
recorded.

`-wave party-adjustments` separately retains `dbo.SaleReceivableAdj` debit and
credit rows in `historical_party_ledger_adjustments`. It does not reinterpret
those rows as payment receipts; unresolved parent invoice/date/party identity
is retained and exact legacy adjustment posting remains an acceptance gate.

`-wave return-allocations` separately retains customer
`dbo.SRAllocationHeader/Detail` and supplier `dbo.PRAllocationHeader/Detail`
rows in `historical_party_return_allocations`. It preserves return/invoice
identity, allocation/outstanding amounts, posted state, and unresolved party
links; the bounded statement/ledger projection is not used to mutate aging or
canonical document balances until duplicate and legacy posting semantics are
reconciled.

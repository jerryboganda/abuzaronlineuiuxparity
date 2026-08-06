# Canonical SQL Server first-tenant evidence (2026-08-06)

This evidence records the first bounded import from the locally available
canonical SQL Server database. The canonical source was accessed with Windows
authentication and only `SELECT` statements; no source DML, DDL, backup, or
security changes were performed.

## Source inventory

- Database: `FazalDinPP19DataBaseV2`
- Inspector manifest: `tmp/canonical-sqlserver-schema.json`
- Inventory: 763 base tables and 10,890 column records
- All source tables referenced by `phase-e-enterprise-config.json` and
  `phase-e-core-masters.json` were present in the manifest.

## Isolated target scope

| Scope | Identifier |
|---|---|
| Tenant `LEGACY_CANONICAL` | `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01` |
| Branch `MAIN` | `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02` |
| Counter `COUNTER-1` | `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a03` |

No existing tenant or the `Legacy Reference Sandbox` scope was selected by the
canonical run. The importer rewrote only the reviewed map's declared scope
injections using the explicit `-tenant-id`/`-branch-id` options.

## Import and reconciliation results

| Wave | Tables | Source rows read | Imported | Duplicates | Exceptions | Reconciliation |
|---|---:|---:|---:|---:|---:|---|
| Enterprise/config | 11 | 22 | 22 | 0 | 0 | 11/11 matched |
| Core masters | 7 | 83,425 | 83,425 | 0 | 0 | 7/7 matched |

Reports:

- `parity/catalog/canonical-first-tenant-enterprise-import.json`
- `parity/catalog/canonical-first-tenant-core-import.json`
- `parity/catalog/canonical-first-tenant-enterprise-reconciliation.json`
- `parity/catalog/canonical-first-tenant-core-reconciliation.json`

Core source/target matches were Item 30,052, Customer 2, Supplier 235,
Manufacturer 838, Godown 1, PricePolicy 30,052, and ItemSuppliers 22,245.
Enterprise matches were ConfigSetting 9, Preferences 1, Area 1,
ItemCategory 7, CustomerCategory 1, GodownGroup 1, SupplierCategory 1, and
ManufacturerCategory 1, with the four empty source tables also matching zero.

The separate reviewed security map added 13 mapping entries and 925 rows
(Groups, Users, memberships, rights, and allow-scope tables), with 0
exceptions, 13/13 table counts matched, and 13/13 reviewed security metrics
matched. Its report files are
`parity/catalog/canonical-first-tenant-security-import.json` and
`parity/catalog/canonical-first-tenant-security-reconciliation.json`.
The canonical tenant now has 4 roles with legacy IDs 2, 5, 11, and 12; 9
users with reset-required password markers; 726 group-right rows; and 173
allow-scope rows.

The imported compatibility masters were then promoted idempotently for this
tenant into the normalized API targets: 30,052 `master_items`, 237
`master_parties`, 838 `master_manufacturers`, 10 `master_categories`, 1
`master_godowns`, 22,245 `item_suppliers`, and 30,052 legacy-id aliases.
This target-side promotion issued no SQL Server writes.

Finally, the bounded purchase-order header slice imported and reconciled
2,810/2,810 `PurOrderHeader` rows with 0 exceptions. Its reports are
`parity/catalog/canonical-first-tenant-purchase-order-headers-import.json` and
`parity/catalog/canonical-first-tenant-purchase-order-headers-reconciliation.json`.
The 113,995 purchase-order detail rows remain intentionally deferred until the
next bounded document-lines wave.

The two reviewed posted purchase-header mappings were then imported and
reconciled: 6,395 `pack-purchase` rows and 1 `opening-purchase` row (6,396
source/target rows, 0 exceptions). Their reports are
`parity/catalog/canonical-first-tenant-purchase-headers-import.json` and
`parity/catalog/canonical-first-tenant-purchase-headers-reconciliation.json`.
Purchase detail lines, returns, sales, stock, and ledgers remain deferred.

The reviewed category-completion map then added the remaining 22 `loose-purchase`
rows and 1 unposted `pack-purchase` row with 0 exceptions. The full canonical
`Purledger` inventory is now 6,419/6,419 with total `198,071,256.00`; the
completion reports are
`parity/catalog/canonical-first-tenant-purchase-header-completion-import.json`
and
`parity/catalog/canonical-first-tenant-purchase-header-completion-reconciliation.json`.
The consolidated header reconciliation is recorded in
`parity/catalog/canonical-first-tenant-purchase-headers-reconciliation.json`.

The small posted purchase-return header slice adds 634 `PRLedger` rows with
exact source/target reconciliation and 0 exceptions. Its reports are
`parity/catalog/canonical-first-tenant-purchase-return-headers-import.json` and
`parity/catalog/canonical-first-tenant-purchase-return-headers-reconciliation.json`.
The next bounded line slice adds all 2,481 `PRdetail` rows with 0 duplicates,
0 exceptions, exact header/line counts, exact return total `3,526,551.00`, and
line quantity within the reviewed `0.01` tolerance. Its reports are
`parity/catalog/canonical-first-tenant-purchase-return-lines-import.json` and
`parity/catalog/canonical-first-tenant-purchase-return-lines-reconciliation.json`.
The sale-return header slice then added all 30,704 `SRLedger` rows with 0
duplicates/exceptions and exact total `19,691,238.00`; its reports are
`parity/catalog/canonical-first-tenant-sale-return-headers-import.json` and
`parity/catalog/canonical-first-tenant-sale-return-headers-reconciliation.json`.
The dependent slice added all 44,579 `SRdetail` rows with 0
duplicates/exceptions, exact header/line counts and return total, and line
quantity difference `0.00000453` within the reviewed `0.01` tolerance. Its
reports are
`parity/catalog/canonical-first-tenant-sale-return-lines-import.json` and
`parity/catalog/canonical-first-tenant-sale-return-lines-reconciliation.json`.
The line quantity column is now `numeric(19,8)` through migration
`026_historical_line_precision.sql`; this preserves fractional loose-unit
calculations instead of rounding each imported line to four decimals.
Purchase-order/detail lines, sales, stock, and ledger tables remain deferred;
the purchase-detail line slice is documented below.

The canonical pricing-tier slice then bulk-loaded all 30,052
`PricePolicyDetail` rows into `price_policy_tiers` and recorded 30,052
legacy-ID mappings with 0 exceptions. The row count and price total
`11,568,683.1700` reconcile exactly. Evidence is in
`parity/catalog/canonical-first-tenant-price-policy-import.json` and
`parity/catalog/canonical-first-tenant-price-policy-reconciliation.json`.
The reusable bounded loader is `migration/cmd/bulkpricepolicy` and uses
PostgreSQL COPY plus an idempotent tenant-scoped upsert.

The first canonical tax configuration slice adds 7 GST schedules and 3 PCT
categories with 0 exceptions. Counts and rate totals reconcile exactly
(GST `128.00`, PCT `1.50`); evidence is in
`parity/catalog/canonical-first-tenant-tax-rates-import.json` and
`parity/catalog/canonical-first-tenant-tax-rates-reconciliation.json`.

The item-tax assignment slice then bulk-loaded the canonical `Item` tax
references: 30,052 GST assignments and 30,052 PCT assignments, with 60,104
assignment rows and 30,052 refreshed audited legacy-ID mappings, with 0 loader
exceptions. Both focused reconciliations
match their filtered source counts exactly; evidence is in
`parity/catalog/canonical-first-tenant-item-tax-import.json` and
`parity/catalog/canonical-first-tenant-item-tax-reconciliation.json`. The
reusable loader is `migration/cmd/bulkitemtax` and uses PostgreSQL COPY plus
dependency-checked, tenant-scoped idempotent upserts.

The purchase-line slice then processed all 113,564 canonical `Purdetail` rows
with `migration/cmd/bulkpurchaselines`. It populated 113,532
`business_document_lines` rows and refreshed the corresponding audited mappings
at set-based speed. The 32 source rows with non-positive calculated quantity
were retained as explicit `exception` mappings and `migration_exceptions`
records rather than coerced into invalid transactional lines. Quantity
(`484,849.58968511`) and line-total (`182,490,712.3624` within cents)
reconcile; the focused report is
`parity/catalog/canonical-first-tenant-purchase-lines-reconciliation.json` and
the import report is
`parity/catalog/canonical-first-tenant-purchase-lines-import.json`.

## Guard and verification

Canonical import/reconciliation remains fail-closed unless the operator passes
the explicit canonical opt-in. Canonical import additionally requires a
dedicated `-tenant-id`; this prevents a reviewed sandbox map from silently
writing into its original tenant. Focused `go test ./migration/...` passed after
the guard, scope-override, security upsert, and canonical metric-tenant rewrite
changes. The PostgreSQL
application-role probe saw 30,052 canonical items in the selected tenant and
zero rows from the SMOKE tenant, confirming the tenant RLS boundary.

This wave is not full migration or parity acceptance. The other source tables
in the 763-table inventory, historical documents, stock/batches, ledgers,
remaining tax/profit projections,
report columns/printing, hardware integrations, complete legacy screen capture,
and end-to-end visual/functional gates remain open.

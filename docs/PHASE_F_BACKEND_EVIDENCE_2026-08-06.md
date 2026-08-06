# Phase F backend evidence — 2026-08-06

## Scope

This artifact covers the backend/schema portion of Phase F only. It does not
claim frontend parity, raster parity, complete master coverage, pricing/tax
parity, or migrated-data reconciliation.

## Changes verified

- `db/migrations/010_master_normalized.sql` applied after the two existing
  `009` migrations. It is tenant-RLS protected and idempotent.
- Normalized targets exist for items, customer/supplier parties,
  manufacturers, item groups, categories, godowns, aliases, and
  `item_suppliers`. Legacy IDs and composite ItemSuppliers identity are
  retained. Read-only compatibility views preserve the Phase E
  `master_records`/`item_supplier_links` path.
- Local database counts after the migration were: `master_items` 30,052,
  `master_parties` 237, `master_aliases` 60,104, and `item_suppliers` 22,245.
- The API now provides normalized tenant-scoped master list/detail CRUD,
  item lookup by name/alias/barcode/legacy ID, and ItemSuppliers read/replace
  endpoints. Existing `/v1/master/{kind}` routes remain registered.
- Existing `master.read`/`master.write` permission checks are retained.
  Canonical master requests also validate the authenticated branch context;
  master rows themselves remain tenant-wide by design.

## Commands and observed results

- `gofmt -d services/api/internal/httpapi/...` — clean.
- `go test ./services/api/... ./services/edge/... ./migration/...` — passed.
- `pnpm --filter @abuzar/web check` — 0 errors, 0 warnings.
- PostgreSQL migration pass 1 over all files — passed.
- PostgreSQL migration pass 2 over all files — passed.
- Local PostgreSQL constraint inspection confirmed tenant-scoped unique
  legacy keys and `(tenant_id, legacy_item_id, legacy_supplier_id)` on
  `item_suppliers`.

## Open work / risks

- The reviewed Phase E importer maps still write the compatibility
  `master_records`/`item_supplier_links` targets; a future importer wave must
  promote those mappings to the normalized tables or run an explicit adapter
  synchronization step.
- Alias classification remains conservative for legacy payloads; barcode
  lookup accepts the preserved alias values, but source-specific alias
  semantics need reconciliation against the sandbox data.
- Full Phase F form coverage, frontend removal of all fallback behavior,
  raster acceptance, and complete legacy master reconciliation remain open.

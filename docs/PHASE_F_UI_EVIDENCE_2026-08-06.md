# Phase F frontend evidence — 2026-08-06

## Scope

Frontend implementation for canonical master data. Backend APIs, migrations,
reports, operations, authentication, edge, and legacy sources were not
changed.

## Implemented

- Canonical Item list/detail uses `/v1/master/item` and item detail routes,
  supports server search, dense sort/filter/find list chrome, List/Detail
  navigation, selection, cancel/save, all configured item payload fields, and
  the Suppliers Priority/Rate/Disc%/Qty/Bonus/Days grid.
- Item lookup/detail preserves canonical aliases and supplier links; supplier
  replacement uses the authenticated tenant/branch-scoped API.
- Customer, Supplier, Manufacturer, Item Group (`item-group`/`item-class`),
  Category variants, and Godown use canonical master CRUD routes.
- Users continues to use the compatibility operator/role APIs.
- Unsupported master kinds remain visible as explicitly generic,
  read-only compatibility surfaces; they do not write through a canonical
  fallback or display demo rows.
- Added focused Playwright coverage for canonical item search/detail, supplier
  grid save/reload, customer/supplier CRUD requests, tenant-safe errors, and
  empty/no-demo master surfaces.

## Verification

Commands run from `D:\ABUZAR\AbuzarNext`:

| Command | Observed result |
|---|---|
| `pnpm --filter @abuzar/web check` | 0 errors, 0 warnings |
| `pnpm --filter @abuzar/web build` | Production build completed |
| `pnpm --filter @abuzar/web exec playwright test tests/phase-f.spec.ts` | 5 passed |
| `pnpm --filter @abuzar/web exec playwright test --workers=1 --retries=1` | 58 passed |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Passed |

## Remaining master gaps

- No frontend delete action is exposed yet, although canonical delete routes
  exist.
- Area, sub-area, price-policy, tax-policy, and other non-canonical masters
  remain intentionally generic/read-only until their canonical APIs exist.
- Payload field dictionaries still reflect the legacy master contract; a
  field-by-field migrated-data reconciliation and raster comparison remain
  open.
- Full 1936×1048 per-master raster acceptance was not run in this change;
  the evidence above is functional browser coverage.

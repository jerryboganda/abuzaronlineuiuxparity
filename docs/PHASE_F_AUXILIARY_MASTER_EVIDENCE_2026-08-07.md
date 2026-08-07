# Phase F auxiliary Basic Data master evidence - 2026-08-07

## Scope

The 16 captured Basic Data leaves that previously rendered as read-only
generic surfaces now have a tenant-scoped CRUD path through
`master_records`. The route keeps the captured leaf kind, displays a
source-informed field set, and retains the source-shaped values in `payload`.
This is an operational CRUD compatibility layer; it is not a claim that the
legacy SQL Server tables, joins, validation rules, pricing rules, or imported
rows have been fully reconstructed.

Covered route kinds:

`sale-promotion`, `customer-sector`, `generic-item`, `item-basic-data`,
`price-policy`, `item-alert`, `sales-tax-schedule`, `pct-codes`,
`generic-item-type`, `item-thickness`, `lock-reason`, `category-segment`,
`manufacturer-type`, `sale-template`, `tax-category`, and `template`.

The field dictionaries retain the reviewed source names where available,
including `SalePromotionCode`, `CustomerSectorCode`, `GenericCode`, `ICode`,
`PricePolicyCode`, `ItemAlertCode`, `SalesTaxScheduleCode`, `PCTCode`,
`GenericItemTypeCode`, `ItemThicknessCode`, `LockReasonCode`,
`CategorySegmentCode`, `ManufacturerTypeCode`, `SaleTemplateCode`, and
`TaxCategoryCode`. Numeric fields use decimal-capable browser inputs.

## Implementation evidence

- `db/migrations/029_auxiliary_master_kinds.sql` extends the
  `master_records_kind_check` constraint with the captured hyphenated route
  kinds. It was applied to the supervised local PostgreSQL target with
  `ON_ERROR_STOP=1`.
- `GET`, `POST`, `PATCH`, and `DELETE /v1/master/{kind}` routes now serve the
  auxiliary kinds through the existing tenant/permission/RLS boundary.
- The master Detail/List form enables load, create, update, and confirmed
  delete for defined auxiliary kinds. Unknown kinds remain explicitly
  read-only and do not call a fallback API.
- New records default to active when the captured Active field is left blank;
  an explicit `NO` remains inactive.

## Verification

Commands run from `D:\ABUZAR\AbuzarNext`:

| Command | Result |
|---|---|
| `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 apps/web/tests/phase-f.spec.ts` | Passed: 5/5, including auxiliary create and confirmed delete request coverage |
| `DATABASE_URL=postgres://.../abuzar_next go test ./services/api/internal/httpapi -run 'TestAuxiliaryMasterCRUDIntegration' -count=1` | Passed: schema constraint, tenant-scoped create/list/update/delete, and stored payload assertions |
| `gofmt -w services/api/internal/httpapi/master_integration_test.go` | Passed |

## Remaining acceptance evidence

- The current source SQL Server boundary still prevents a fresh canonical
  source run; the existing source-shaped payloads are not a migrated-data
  reconciliation.
- Exact PowerBuilder field enablement, validation messages, dependent lookup
  joins, promotion/price/tax calculation semantics, and delete behavior remain
  to be observed against approved legacy captures.
- Per-leaf 1936x1048 raster, keyboard/focus, print, and populated-data
  acceptance remains open.

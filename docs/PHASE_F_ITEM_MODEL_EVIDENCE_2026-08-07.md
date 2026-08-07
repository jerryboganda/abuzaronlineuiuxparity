# Phase F Item Model Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured SQL Server catalog defines `dbo.ItemInModel` with the owning
legacy item `ICode` and a `ModelCode` (`smallint`). A separate `dbo.Model`
master exists. The captured Item Form command is `File > Select Models`
(`F12`). This slice preserves the reviewed membership codes and does not
invent model names or picker behavior before the source master is imported and
approved.

## Implemented slice

- `db/migrations/041_item_models.sql` adds a tenant-scoped,
  RLS-protected `master_item_models` collection with the owning legacy item
  identity, canonical item foreign key, and source-sized `smallint` model
  code.
- `GET` and `PUT /v1/master/item/{id}/models` require `master.read` and
  `master.write` respectively. PUT replaces the selected item's complete
  membership atomically and bounds the collection to 100 unique codes in the
  captured signed-smallint range.
- The Item Form menu marks `Select Models` implemented and opens a
  legacy-styled add/remove editor for model codes.
- Contracts and `docs/openapi.yaml` describe the relationship collection.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeItemModelCodesPreservesMembershipAndBoundsSourceType|NormalizeItemAuthorsPreservesOrderAndRejectsAmbiguousRows|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm check` from `apps/web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias|association command|author command|model command" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 6/6.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.

## Remaining acceptance boundary

No live `dbo.Model`/`dbo.ItemInModel` extraction, relationship
reconciliation, or approved PowerBuilder model-picker raster, keyboard/focus,
and selection behavior comparison was performed. This is an implemented and
locally verified relationship slice, not exact legacy acceptance. Remaining
Item Form commands, full catalog migration, reports, hardware, scale, and
operational UAT remain open.

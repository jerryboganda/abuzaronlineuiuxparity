# Phase F Item Form image evidence — 2026-08-07

## Status

The captured Item Form `File > Set Item Image(s)` command is implemented as a
tenant-scoped canonical image collection with bounded upload/replace behavior
and a Svelte dialog. This is a focused implementation slice; live PostgreSQL
migration application, historical `dbo.ItemImage` import, and operator sign-off
remain open.

## Legacy source contract

The reviewed SQL Server schema contains `dbo.ItemImage` with `ICode`,
`ImageDescription`, `ROWID`, `ItemImage` (blob), and `ImageType`. The canonical
`master_item_images` target retains those concepts as `item_id`, `row_id`,
description, `bytea` image data, and type, with tenant/RLS scope and an atomic
collection replacement API.

The API accepts base64 data or a data URL, limits each decoded blob to 8 MiB,
limits a collection to 50 images and 32 MiB total, rejects duplicate row IDs,
and returns base64 image data for preview/edit. The UI reads local image files,
shows image previews, edits descriptions/types, removes rows, and saves the
full selected-item collection.

## Implemented contract

- `GET /v1/master/item/{id}/images` returns active images ordered by `rowId`.
- `PUT /v1/master/item/{id}/images` replaces only the selected item's image
  collection within the authenticated tenant and branch scope.
- The Item Form command is explicitly mapped to `master.write` and no longer
  falls through to the generic unimplemented workbench.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeItemImagesPreservesRowsAndBoundsBlobPayloads|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` passed.
- `go vet ./internal/httpapi` passed.
- `cmd /c pnpm --dir apps/web check` passed with 0 errors and 0 warnings.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "ItemImage rows" --project=chromium --retries=0 --timeout=15000 --reporter=line` passed 1/1.
- `git diff --check` passed with only normal LF/CRLF checkout notices.

## Remaining acceptance evidence

The new migration has not been applied to a reviewed PostgreSQL instance in
this short pass. A live run must prove the RLS policy, blob-size constraint,
collection replacement rollback, and preservation/reconciliation of imported
`ItemImage` row IDs and binary content. Exact PowerBuilder image-dialog
geometry, source encoding/format semantics, and the remaining Item Form
commands (models, associations, notes, authors, registration, and price
policy) remain open.

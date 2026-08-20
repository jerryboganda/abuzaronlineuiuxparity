# Phase F Item Notes Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured SQL Server catalog defines `dbo.ItemNotes` with:

- `ICode` (`int`, required), the legacy item identity;
- `Notes` (`image`, nullable), the opaque note/rich-text blob.

The captured Item Form command is `File > Set Item Notes` (`F10`). The source
catalog does not prove that the blob is always UTF-8 text, so the canonical
store deliberately retains bytes rather than forcing a lossy text conversion.

## Implemented slice

- `db/migrations/038_item_notes.sql` adds the tenant-scoped, RLS-protected
  `master_item_notes` one-row-per-item store with nullable `bytea` data.
- `GET` and `PUT /v1/master/item/{id}/notes` require `master.read` and
  `master.write` respectively, verify the canonical item in the authenticated
  tenant, and replace the note blob atomically.
- The API accepts standard or unpadded base64, optional data-URL prefixes, and
  bounds decoded note data to 8 MiB. Empty data represents no notes.
- The Item Form menu marks `Set Item Notes` as implemented and opens a
  legacy-styled dialog. UTF-8/RTF text can be edited directly; a raw base64
  field preserves non-UTF-8 legacy encodings without silently rewriting them.
- Contracts and `docs/openapi.yaml` describe the base64 byte contract and
  decoded-size boundary.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(DecodeItemNotesDataRoundTripsTextAndBoundsBlobPayloads|NormalizeItemImagesPreservesRowsAndBoundsBlobPayloads|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm --dir apps\\web check` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 3/3.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.

## Remaining acceptance boundary

No live SQL Server `ItemNotes` extraction, byte-level reconciliation, or
approved PowerBuilder rich-text/editor raster and keyboard/focus comparison
was performed. The command is therefore implemented and locally verified, not
accepted as exact legacy parity. Full catalog migration, remaining commands,
report output, hardware, scale, and operational UAT remain open in the main
acceptance record.

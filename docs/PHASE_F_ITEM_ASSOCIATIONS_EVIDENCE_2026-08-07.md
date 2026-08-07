# Phase F Item Associations Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured SQL Server catalog defines `dbo.ItemAssociation` with the pair:

- `ICode` (`int`), the owning legacy item;
- `AssocICode` (`int`), the associated legacy item.

The captured Item Form command is `File > Set Item Associations` (`Ctrl+F12`).

## Implemented slice

- `db/migrations/039_item_associations.sql` adds the tenant-scoped,
  RLS-protected canonical relation and preserves both legacy IDs, including an
  unresolved associated-item identity for future historical import.
- `GET` and `PUT /v1/master/item/{id}/associations` require `master.read` and
  `master.write` respectively. Writes replace the selected item's complete
  association set atomically, resolve each submitted legacy item within the
  authenticated tenant, reject self-links, duplicates, blanks, and more than
  100 links, and return code/name projections where available.
- The Item Form menu marks the command as implemented and opens a
  legacy-styled add/remove editor using legacy item identifiers.
- Contracts and `docs/openapi.yaml` describe the bounded pair collection.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeItemAssociationIDsRejectsAmbiguityAndSelfIsHandledByRoute|DecodeItemNotesDataRoundTripsTextAndBoundsBlobPayloads|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm check` from `apps\\web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias|association command" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 4/4.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.
- `git diff --check` — passed with normal LF/CRLF checkout notices.

## Remaining acceptance boundary

No live `dbo.ItemAssociation` extraction/reconciliation or approved
PowerBuilder dialog raster, keyboard/focus, source-ID, and dependency-behavior
comparison was performed. This is an implemented and locally verified slice,
not exact legacy acceptance. The full catalog migration, remaining Item Form
commands, reports, hardware, scale, and operational UAT remain open.

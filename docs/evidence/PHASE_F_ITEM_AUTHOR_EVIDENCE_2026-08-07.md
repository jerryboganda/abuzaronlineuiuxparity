# Phase F Item Author Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured SQL Server catalog defines `dbo.ItemAuthor` with:

- `ICode` (`int`), the owning legacy item;
- `AuthorCode` (`int`), the related author identity;
- `Priority` (`tinyint`);
- `ROWID` (`int`).

The captured Item Form command is `File > Set Item Author(s)` (`Alt+F12`). A
separate `dbo.Author` master exists, but its source import and exact selection
semantics are not yet approved; this slice therefore preserves the reviewed
relationship fields without inventing a name-resolution rule.

## Implemented slice

- `db/migrations/040_item_authors.sql` adds a tenant-scoped, RLS-protected
  canonical collection retaining author code, priority, row order, and the
  owning legacy item identity.
- `GET` and `PUT /v1/master/item/{id}/authors` require `master.read` and
  `master.write` respectively. PUT replaces the selected item's complete list
  atomically and bounds the collection to 50 rows, positive author codes,
  unique author codes/row IDs, and byte-sized priority values from 0–255.
- The Item Form menu marks the command as implemented and opens a
  legacy-styled add/remove editor for author code and priority.
- Contracts and `docs/openapi.yaml` describe the relationship collection.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeItemAuthorsPreservesOrderAndRejectsAmbiguousRows|NormalizeItemAssociationIDsRejectsAmbiguityAndSelfIsHandledByRoute|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm check` from `apps\\web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias|association command|author command" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 5/5.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.

## Remaining acceptance boundary

No live `dbo.Author`/`dbo.ItemAuthor` extraction, relationship reconciliation,
or approved PowerBuilder author-picker raster, keyboard/focus, and selection
behavior comparison was performed. This is an implemented and locally
verified relationship slice, not exact legacy acceptance. Remaining Item Form
commands, full catalog migration, reports, hardware, scale, and operational
UAT remain open.

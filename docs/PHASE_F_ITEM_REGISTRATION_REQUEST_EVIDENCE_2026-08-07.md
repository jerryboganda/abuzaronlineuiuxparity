# Phase F Item Registration Request Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured SQL Server catalog defines `dbo.ItemRegRequest` with 130 fields,
including request identity/date/server metadata, sent state, optional server
request identity, item identity, and the source Item master fields. The
captured Item Form command is `File > Populate Item Registration Request`
(`Ctrl+U`).

## Implemented slice

- `db/migrations/042_item_registration_requests.sql` adds a tenant-scoped,
  RLS-protected request history with source-sized request metadata and a
  JSONB snapshot payload. The sequence-backed request code and item identity
  are typed; repeated populate actions remain auditable history rows.
- `GET` and `POST /v1/master/item/{id}/registration-request` require
  `master.read` and `master.write` respectively. POST copies the full current
  canonical item payload, adds the source request identity/date/item/sent
  fields, records `sent = 'N'`, and returns the created local snapshot.
- The Item Form menu marks the captured command implemented and opens a
  legacy-styled status dialog that can populate and review the latest local
  request snapshot and preserved source-field count.
- The implementation does not infer server names, machine names, sent dates,
  server request codes, or external delivery behavior that are not available
  from the current local workflow.
- Contracts, OpenAPI, focused browser coverage, and the acceptance record now
  describe the slice.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeItemRegistrationPayloadPreservesItemFieldsAndAddsRequestMetadata|NormalizeItemPricePolicyTiersPreservesSourceDecimalsAndRejectsAmbiguity|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm check` from `apps/web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias|association command|author command|model command|price-policy command|registration command" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 8/8.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.

## Remaining acceptance boundary

No live `dbo.ItemRegRequest` extraction/reconciliation, source UI field-by-
field comparison, server/machine routing, sent-state protocol, external
registration-server delivery, or approved PowerBuilder dialog raster and
keyboard/focus comparison was performed. The request snapshot is locally
verified, not proof of the full 130-field legacy workflow or overall parity
acceptance.

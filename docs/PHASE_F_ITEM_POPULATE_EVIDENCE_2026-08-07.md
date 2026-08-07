# Phase F Item Populate Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured Item Form menu defines `File > Populate Item` (`Ctrl+O`) as a
read-only command. The command is part of the captured contextual menu tree
and is intended to populate the active Item Form from an existing item rather
than create a second item record.

## Implemented slice

- The Item Form menu marks `Populate Item` implemented and maps it to the
  tenant-scoped `master.read` permission.
- The command opens a legacy-styled lookup dialog backed by
  `GET /v1/items/lookup?q=...`, accepting code, name, barcode, alias, or
  legacy identifier search values.
- Selecting an active canonical result hydrates the existing Item Form by
  loading that tenant-scoped canonical record. It does not write a duplicate
  item or infer an unreviewed source-side import operation.
- The UI exposes bounded empty-result and lookup-error states, and the
  completion message records the active canonical lookup source.
- Focused browser coverage and the captured-menu acceptance assertion now
  describe the slice.

## Focused evidence

- `cmd /c pnpm check` from `apps/web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias|association command|author command|model command|price-policy command|registration command|Populate Item command" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 9/9.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.
- `git diff --check` — run with the final focused audit.

## Remaining acceptance boundary

No live source SQL Server lookup comparison, barcode/alias data reconciliation,
PowerBuilder focus/accelerator/raster approval, item-selection edge-case UAT,
or full tenant-scale lookup performance evidence was performed in this slice.
The local canonical lookup-and-hydrate workflow is verified; it is not proof
of complete Item Form or overall legacy replacement acceptance.

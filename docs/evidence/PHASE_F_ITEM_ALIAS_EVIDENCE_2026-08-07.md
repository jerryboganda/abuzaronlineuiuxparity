# Phase F Item Form alternate-alias evidence — 2026-08-07

## Status

The captured Item Form `File > Set Alternate Item Alias Names` command is
implemented through a tenant-scoped canonical API and Svelte dialog. It is a
bounded implementation slice; live PostgreSQL migration application, legacy
row reconciliation, and PowerBuilder operator sign-off remain open.

## Legacy source contract

The captured SQL Server schema contains `dbo.AlternateItemAlias` with the
source identity `ICode`, alternate code `CustomICode`, display `Name`, and
quantity/price metadata. This slice implements the captured command's alias
editing behavior without inventing a price calculation: the canonical
`master_aliases` table now supports a distinct `alternate_alias` kind.

Primary `AliasName`/`CustomICode` payload aliases and barcode rows remain
separate. Replacing alternate aliases therefore cannot silently remove the
normal item lookup keys. The API also stores the edited list under
`payload.AlternateItemAliases` so later item saves retain the reviewed state.

## Implemented contract

- `GET /v1/master/item/{id}/aliases` returns active alternate aliases in the
  authenticated tenant scope.
- `PUT /v1/master/item/{id}/aliases` replaces only alternate aliases, trims and
  bounds values to 100 entries of 160 characters, rejects blanks and
  case-insensitive duplicates, and returns a conflict for a cross-item alias.
- The Item Form menu command opens the editor for the selected canonical item;
  save and cancel behavior is explicit and permission-gated by `master.write`.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeAlternateItemAliasesKeepsOrderAndRejectsAmbiguity|CanonicalMasterRoutesRemainAuthenticated|NormalizedMasterMigrationRetainsLegacyUniquenessAndSupplierFields)$' -count=1` passed.
- `go vet ./internal/httpapi` passed.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "alternate-alias" --project=chromium --retries=0 --timeout=15000 --reporter=line` passed 1/1.
- `cmd /c pnpm --dir apps/web check` passed with 0 errors and 0 warnings.
- `git diff --check` passed with only normal LF/CRLF checkout notices.

## Remaining acceptance evidence

The migration file has not been applied to a reviewed PostgreSQL instance in
this short pass. A live run must prove the new constraint/index, tenant/RLS
behavior, cross-item conflict behavior, and preservation of primary aliases.
The remaining captured Item Form commands (models, associations, notes,
authors, images, registration requests, and price policy) are not covered by
this slice. Exact PowerBuilder dialog geometry, image/blob handling, source
price metadata, and operator UAT remain open.

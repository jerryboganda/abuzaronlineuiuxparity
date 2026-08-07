# Phase R imported group-scope setting evidence - 2026-08-07

## Scope

This bounded slice makes the captured group-scope setting workflows operational
against the imported security rows:
`Manage -> Group Wise Header Setting`, `Manage -> Group Allowed Price Setting`,
`Maintenance -> Group Wise Godown Setting`, `Manage -> Group Wise Cash Account
Setting`, and `Manage -> Group Wise Supplier Category`. It does not claim that
the PowerBuilder `Module`, `PriceTypeCode`, or other composite identifiers have
been translated into new-system labels or policy behavior.

## Implemented

- Each route loads tenant-scoped roles through `GET /v1/roles` and the
  selected role's imported access rows through `GET /v1/roles/{id}/rights`.
- Each route displays only its approved imported `scope_kind` and retains the
  exact composite source key. Price rows show
  `GroupCode:Module:PriceTypeCode`; other rows retain their raw source key.
- Save sends only the selected route's scope rows to the existing role-rights
  `PATCH` endpoint. Other scope kinds are not rewritten by that workflow.
- The existing role-access audit path records the update in the tenant-scoped
  audit ledger.
- Explicit canonical-UUID godown scopes, when present, are enforced at stock
  balance and availability reads, canonical document ingress (including
  stored-document voids), direct inventory transaction ingress, synchronization
  ingress, and canonical godown lookup/detail. The offline compatibility sale
  envelope also carries its selected godown so the same decision survives queue
  replay. Imported `GroupAllowedGodown` composite keys remain compatibility
  metadata until their source-to-canonical mapping is approved; payloads that
  omit a godown remain on the existing compatibility path.

## Verification

- `pnpm --filter @abuzar/web check` - passed with 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --reporter=line --grep "Group Allowed Price Setting"` - passed 1/1.
- `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --reporter=line --grep "imported group scope setting leaves"` - passed 1/1 across the four non-price scope leaves.
- `$env:DATABASE_URL='postgres://.../abuzar_next?sslmode=disable'; go test ./services/api/internal/httpapi -run 'TestImportedGroupScopeUpdateIsTenantScopedAndAudited' -count=1` - passed; the real PostgreSQL handler path updates only the requested kind and records `role.access.updated`.
- `$env:DATABASE_URL='postgres://.../abuzar_next?sslmode=disable'; go test ./services/api/internal/httpapi -count=1` - passed; focused authorization tests reject denied canonical-UUID godowns before stock reads, inventory-event writes, canonical document writes, and canonical godown detail reach the database projection, while composite source keys remain deferred.
- The imported Phase R security evidence records 33 `GroupAllowedGodown`, 35
  `GroupAllowedHeader`, 54 `GroupAllowedPrice`, and 43 `GroupCashAccount` rows
  in the isolated `Legacy Reference Sandbox` tenant. The supplier-category
  source table was imported as an empty approved scope. These are retained
  migration evidence, not a fresh read of the canonical SQL Server.

## Remaining acceptance boundary

The editors are now real scoped read/write workflows, and explicit
canonical-UUID godown scopes are enforced at the boundaries above. The imported
`GroupAllowedGodown` source rows currently use composite
`GroupCode/GCode/Module/Priority` keys rather than canonical godown IDs, so
their downstream mapping and enforcement remain open. Exact PowerBuilder
validation, ordering, labels, and price/header policy behavior remain open. The
captured source schema identifies `PriceTypeCode` and `Module`; it does not, in
the currently approved evidence, provide the exact mapping from those values
to this rebuild's document kinds, SalePrice levels, or customer/group policy
precedence. No inferred mapping was introduced.
The canonical SQL Server read-only probe during this run was refused by
Windows integrated authentication with an untrusted-domain login error.

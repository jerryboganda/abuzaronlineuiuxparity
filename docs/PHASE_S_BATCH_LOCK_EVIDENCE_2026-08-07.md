# Phase S batch-lock workflow evidence - 2026-08-07

## Scope

The captured Maintenance catalog includes `Lock Item Batches`. The route was
previously a generic preference/audit surface. It now performs a real,
tenant-scoped PostgreSQL mutation for the current operational branch.

## Implemented contract

- `POST /v1/maintenance/lock-item-batches` validates `itemCode`, `batch`, and
  a `Yes`/`No` or boolean `locked` value.
- The API resolves the canonical item under the authenticated tenant, updates
  matching `stock_batches` rows only for the authenticated branch, and returns
  a completed operation id and affected-row count.
- The immutable audit payload records the canonical item identity, requested
  batch/state, branch-scoped affected count, and final lock state.
- The Svelte route submits the captured Item Code, Batch, and Locked fields.

## Fresh verification

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestBatchLockMaintenanceValidation|TestMaintenanceIntegrityContractIsApplicationScoped|TestCanonicalItemMaintenanceValidation' -count=1` | Passed |
| `DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable go test ./services/api/internal/httpapi -run 'TestMaintenanceManageOperationsIntegration' -count=1` | Passed: lock, unlock, audit count, and other-branch isolation |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --reporter=line --grep "Lock Item Batches"` | Passed: 1 browser test |
| `git diff --check` | Passed; no whitespace errors |

## Acceptance boundary

The new-system branch/tenant mutation is verified. Exact PowerBuilder lock
selection, messages, batch-list ordering, and populated-data behavior remain
open until the captured legacy workflow can be replayed against an available
source dataset. A read-only SQL Server probe during this run was refused with
an integrated-authentication `untrusted domain` login error; no credentials or
source-side writes were attempted.

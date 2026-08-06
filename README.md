# Abuzar Next

Parity-first rebuild of the Abuzar application for Chrome/PWA and an optional Tauri desktop client.

## Architecture

- `apps/web`: SvelteKit + TypeScript web application.
- `apps/desktop`: Tauri wrapper for the same frontend and local hardware capabilities.
- `services/api`: Go central API for PostgreSQL-backed multi-tenancy.
- `services/edge`: Go branch-local service with SQLite for offline transaction capture and sync.
- `packages/contracts`: shared TypeScript API and synchronization contracts.
- `docs/openapi.yaml`: versioned REST contract for the central API and browser/edge clients.
- `db/migrations`: PostgreSQL schema and row-level-security policies.
- `migration`: source-database inspection and migration runbooks.
- `parity`: visual/behavioral baseline catalogue and approved screenshots.

The existing PowerBuilder application under `D:\ABUZAR\V2_AbuzarSoftware` is the reference/fallback system and is intentionally outside this project.

See [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) for the verified slice and the gates that remain before production parity/cutover.
The rebuilt Windows installer hashes are recorded in [`docs/RELEASE_ARTIFACTS.md`](docs/RELEASE_ARTIFACTS.md).

The migration workbench now includes a reviewed-map importer at `migration/cmd/import`; it never writes to the legacy SQL Server source and records target mappings/exceptions for reconciliation.

## Quick start

```powershell
pnpm install
pnpm dev:web
```

The web app runs at `http://localhost:5173`.

For PostgreSQL setup, keep the schema-owner DSN in a protected shell, apply
migrations, and provision the non-owner role with
`ops/postgres/provision-app-role.*`. The API must receive only
`ABUZAR_APP_DATABASE_URL` (exported as `DATABASE_URL`); see
[`docs/OPERATIONS.md`](docs/OPERATIONS.md) for the owner/app split and the
RLS verification job.

To run the API foundation:

```powershell
$env:ABUZAR_API_ADDR = ':8080'
go run ./services/api/cmd/server
```

After applying migrations, provision the first tenant and operator from a protected deployment shell. The command is idempotent for the supplied codes and never prints the password:

```powershell
$env:DATABASE_URL = '<protected app-role DSN>'
$env:ABUZAR_BOOTSTRAP_TENANT_CODE = 'demo'
$env:ABUZAR_BOOTSTRAP_TENANT_NAME = 'Demo tenant'
$env:ABUZAR_BOOTSTRAP_BRANCH_CODE = 'main'
$env:ABUZAR_BOOTSTRAP_BRANCH_NAME = 'Main branch'
$env:ABUZAR_BOOTSTRAP_COUNTER_CODE = 'counter-01'
$env:ABUZAR_BOOTSTRAP_COUNTER_NAME = 'Counter 01'
$env:ABUZAR_BOOTSTRAP_OPERATOR_USERNAME = 'admin'
$env:ABUZAR_BOOTSTRAP_OPERATOR_NAME = 'Tenant administrator'
$env:ABUZAR_BOOTSTRAP_OPERATOR_PASSWORD = '<secret-manager-value>'
go run ./services/api/cmd/bootstrap
```

To run a branch edge store:

```powershell
$env:ABUZAR_EDGE_ADDR = ':8091'
$env:ABUZAR_EDGE_DB = '.\data\branch-edge.sqlite'
go run ./services/edge/cmd/edge
```

To build the optional Windows desktop wrapper (the same frontend as Chrome):

```powershell
pnpm --filter @abuzar/desktop dev
pnpm --filter @abuzar/desktop build
```

The installer bundles are written under `apps/desktop/src-tauri/target/release/bundle/`.

No production credentials are committed. Configure PostgreSQL and secrets through environment variables or the deployment secret store.

## Current implementation boundary

The code-side foundation is runnable and tested: authenticated tenancy, scoped central transactions, sale projection, immutable transaction events, branch-edge SQLite/offline synchronization, conflict review, safe tenant bootstrap, the shared PWA/Tauri client, and migration/reconciliation tooling are present. Exact legacy screen parity, source-data import, and physical hardware acceptance remain gated behind the reference executable, protected SQL Server access, and connected devices described in `docs/`.

# Local stack launcher

From `D:\ABUZAR\AbuzarNext`, run:

```powershell
powershell -ExecutionPolicy Bypass -File .\ops\local\start-local.ps1 -OpenBrowser
```

The launcher starts the local PostgreSQL cluster, Go API, branch edge, and
SvelteKit development server as shell-independent supervisors. API, edge, and
web processes are restarted after an unexpected exit; PostgreSQL is monitored
and restarted through `pg_ctl`. Each service has a PID record and a size-rotated
log under `tmp` (up to five retained generations). Starting the stack again
adopts a healthy existing listener instead of creating a duplicate process.
It does not touch the legacy PowerBuilder installation or database.

Stop the local stack with:

```powershell
powershell -ExecutionPolicy Bypass -File .\ops\local\stop-local.ps1
```

Inspect health, supervisor/child PIDs, stop markers, and log locations with:

```powershell
powershell -ExecutionPolicy Bypass -File .\ops\local\status-local.ps1
```

The demo database is local-only. Production credentials and database URLs must
be supplied through protected deployment configuration, never through a client
bundle or this script.

The launcher provisions `abuzar_app_local` against the machine-local cluster
and runs the API with that non-owner role. The schema-owner URL is used only
for the one provisioning command and is removed before the API supervisor is
started. Local PostgreSQL must therefore permit the passwordless local role
connection, or the protected `ABUZAR_APP_ROLE_PASSWORD`/DSN configuration must
be supplied outside this repository.

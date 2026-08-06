# Operations foundation

The first central deployment target is a managed Linux VPS/cloud service with HTTPS termination, PostgreSQL backups, API health monitoring, and structured logs. The branch edge is installed per branch and must expose a local health endpoint for counter diagnostics.

Production secrets belong in the provider secret store or protected environment, never in the repository. Backups must be encrypted, periodically restored in a test environment, and retained separately from the primary database.

The central readiness probe is `GET /v1/health`; it reports `degraded` when PostgreSQL is unavailable or not configured. Branch diagnostics use `GET /v1/health` and authenticated `GET /v1/sync/status`. Apply migrations before starting the API and set `ABUZAR_COOKIE_SECURE=true` behind HTTPS.

For a new environment, run `go run ./services/api/cmd/bootstrap` once after migrations with the `ABUZAR_BOOTSTRAP_*` values from the deployment secret store. It creates the first tenant, branch, counter, tenant-admin role, and operator assignment in one transaction. PostgreSQL `pgcrypto` hashes the operator password; the command does not log or write it.

Create a dedicated non-owner application role with `ops/postgres/provision-app-role.sh` (or the PowerShell equivalent) from a protected schema-owner shell. Set `ABUZAR_ADMIN_DATABASE_URL`, `ABUZAR_APP_ROLE`, and, only when required by the cluster authentication policy, `ABUZAR_APP_ROLE_PASSWORD`. The password is read from the process environment and is never committed or printed. The provisioning script is idempotent and delegates DML/sequence grants to `grant-app-role.*`; it does not grant DDL or role-management privileges.

Run migrations with `ABUZAR_ADMIN_DATABASE_URL` and start the API with `ABUZAR_APP_DATABASE_URL` (also exported as `DATABASE_URL` for the current API binary). Do not expose or inherit the admin DSN in the API process. The role must not be `SUPERUSER`, `BYPASSRLS`, `CREATEDB`, `CREATEROLE`, or an owner of protected tables.

`grant-app-role.sql` explicitly revokes `DELETE` on `sync_events`; finalized
business-document events are never intentionally deleted by the API. Other
event/ledger tables likewise have no intentional API delete path, but the
legacy all-table DML compatibility grant still leaves their DELETE privilege
in place in this slice. Stock and finance immutability triggers protect the
existing ledger tables; narrowing the remaining event-table DELETE grants is a
follow-up hardening change.

The repeatable RLS check is `.github/workflows/postgres-rls.yml`. It applies migrations and `ops/postgres/rls-fixture.sql` as the owner, then runs `services/api/internal/rlsprobe` and `TestApplicationRoleRLSProbe` over the app DSN. The fixture is privileged setup; the probe is app-role assertion code and must not create its own tenant. It verifies unscoped fail-closed reads/writes, tenant isolation, and role attributes.

Existing `*_integration_test.go` fixtures still seed tenants directly through `DATABASE_URL` and therefore remain owner-oriented. They are intentionally not run with the app DSN in this slice; broad fixture migration is a follow-up. The app-role probe is the concrete CI gate until those fixtures are refactored. Migration `016_branch_rls_hardening.sql` adds restrictive tenant-plus-branch policies, and the probe verifies cross-branch reads/writes are rejected. Current policies also use the session-local `app.authenticating` bootstrap flag for login; this slice proves the normal API context (`false`) and does not claim resistance to a malicious SQL client that deliberately sets that flag.

# PostgreSQL application-role RLS evidence — 2026-08-06

## Scope

This evidence covers the disposable local PostgreSQL 18 cluster and the
repeatable CI job in [`.github/workflows/postgres-rls.yml`](../.github/workflows/postgres-rls.yml).
Migrations and the fixture were run through the protected owner DSN. The
assertion test connected separately through the dedicated non-owner role.
No password or DSN secret is recorded here.

## Local run

Command:

```powershell
$env:ABUZAR_APP_DATABASE_URL = '<protected app-role DSN>'
$env:ABUZAR_APP_ROLE = 'abuzar_app_local'
$env:ABUZAR_RLS_PROBE_REQUIRED = '1'
go test ./services/api/internal/rlsprobe -run '^TestApplicationRoleRLSProbe$' -count=1 -v
```

Observed result:

```text
PASS: role=abuzar_app_local
      superuser=false bypassrls=false createdb=false createrole=false
      protected_table_owner=false schema_create=false sync_events_delete=false
PASS: unscoped master_items=0
PASS: unscoped write rejected by RLS
PASS: tenant A/branch A context visible tenants=1 items=1 documents=1 cross_branch=0 stock_batches=0 vouchers=0 foreign_tenant=0
PASS: cross-tenant write rejected by RLS
PASS: cross-branch write rejected by RLS
PASS: finalized sync event delete rejected
PASS: tenant-admin scope retained documents=3 branch_b_write=accepted
```

The focused Go test passed locally after applying the ordered 17-migration
set twice (both passes completed successfully), including
`016_branch_rls_hardening.sql` and
`017_sync_event_delete_guard.sql`. The owner-side trigger
`sync_events_final_business_document_delete_017` was also confirmed present.
The fixture used two tenants, two branches in tenant A, stock/voucher rows, and
a finalized sync event; fixture creation was owner-only and is not part of the
app-role assertion.

## CI contract

The CI job:

1. Starts a disposable PostgreSQL service with trust authentication and no
   committed password.
2. Applies all ordered migrations as `postgres`.
3. Idempotently provisions `abuzar_app_ci` and applies DML/sequence grants.
4. Seeds `ops/postgres/rls-fixture.sql` as the owner.
5. Runs `TestApplicationRoleRLSProbe` with the app DSN and requires the probe.
6. Runs unaffected Go packages with `DATABASE_URL` unset so existing
   owner-oriented integration fixtures skip rather than accidentally run as the
   app role.

## Residual limitations

- Existing document, stock, and finance integration fixtures still create
  tenants through `DATABASE_URL`; they are not yet app-role compatible. The
  dedicated probe is the fail-closed gate until those fixtures are migrated.
- `sync_events` DELETE is explicitly revoked from the app role and is checked
  both through `has_table_privilege` and an attempted delete of a finalized
  fixture event. Other event/ledger DELETE grants remain for compatibility,
  although no API path intentionally uses them; see `docs/OPERATIONS.md`.
- A full `go test ./services/api/...` run was not green in this workspace because
  the pre-existing `purchase_integration_test.go` calls
  `executeDocumentHandler` with the wrong number of return values. That API
  fixture was not changed in this DevOps slice; CI deliberately runs the
  isolated probe and unaffected packages instead.
- Migration `016_branch_rls_hardening.sql` is now applied and the probe verifies
  branch isolation in addition to tenant isolation.
- The existing authentication bootstrap uses the session-local
  `app.authenticating` setting. The probe validates the normal application
  context (`false`), not a hostile SQL client that deliberately changes that
  setting.
- GitHub Actions was configured but not executed from this workspace; the
  recorded PASS is the local focused probe run.

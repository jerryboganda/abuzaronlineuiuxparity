# Phase V — Preferences parity evidence

Date: 2026-08-07 follow-up

## Delivered coverage

- The 17 captured tabs are registered: General, Sale, Sale Return, Purchase,
  Purchase Return, Report, BasicData, Quotation, Schedule, Adjustment,
  Purchase Order, Others, Point of Sale, Cashier Job Activity, Email, SMS,
  and Dashboard.
- The reviewed backend registry contains **441 logical fields** across the 17
  tabs. Each field has a stable logical `fieldKey` generated from category,
  caption, and occurrence. Repeated labels (most notably Schedule
  `Activate:`) receive occurrence keys such as
  `schedule.activate.1` and `schedule.activate.2`; their physical legacy
  caption is storage-namespaced only for the colliding rows. This preserves
  existing non-colliding rows while preventing silent overwrite.
- Each registered field has a type, default, validation contract, runtime
  status, behavior note, and stable category position. Boolean, enum, integer,
  decimal, date/time, text, and secret values are represented explicitly.
- `tenant_preferences` now supports a nullable tenant default plus an
  authenticated branch override. Branch writes and reads are explicit, and
  the migration RLS policy prevents a branch-scoped value from leaking to
  another branch, including tenant administrators.
- GET `/v1/preferences` returns the effective branch values, registry metadata,
  field keys, scope, and documented divergences. PUT validates registered
  field keys (with a first-occurrence caption alias for old clients) before
  writing and redacts secret values in the audit payload.
- Existing report letterhead configuration remains compatible through the
  preference save path. Pricing/tax/stock/expiry/batch/cashier metadata is
  marked `stored_only` unless the current backend proves the behavior. The
  report default-header bridge is the only preference marked `wired`.

## Scheduling decision

The legacy SQL-Agent/msdb schedule fields persist as configuration only and
return `runtimeStatus: not_configured`. The API does not query `msdb`, create a
false successful job, or claim that backup/posting/service workers ran.
Deployment must provide a PostgreSQL-native scheduler/worker adapter before
those toggles can become executable.

## Validation evidence

- `go test ./services/api/... ./services/edge/... ./migration/...` — passed.
- Migration `024_preferences_branch_scope.sql` applied twice with
  `ON_ERROR_STOP=1` — both applications completed successfully.
- Live preference round-trip/branch-isolation/collision integration — passed
  against local PostgreSQL.
- Full DB-backed Go suite was run. The preference and maintenance integration
  tests passed; the suite still reports an existing sale-summary assertion
  failure, while the stock timeout observed in the combined run passed when
  rerun in isolation.
- Unit/full package Go checks passed for the preference changes. A DB-backed
  run requires `DATABASE_URL`; the final live run used the local
  `postgres://postgres@127.0.0.1:5432/abuzar_next` instance.
- `pnpm --filter @abuzar/web check` — passed, 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web build` — passed.
- `pnpm exec playwright test tests/preferences.spec.ts` — 3/3 passed.
- Full Playwright suite — serial run observed 66/67 with one existing
  load-sensitive canonical-sale test marked flaky; all preference tests passed
  3/3. The earlier canonical-sale, report, and purchase failures also passed
  when rerun in isolation.

## Remaining gaps and deployment risks

- Migration `024_preferences_branch_scope.sql` is rerunnable: guarded
  constraint creation, idempotent indexes, and replaceable RLS policy are used.
  It must still be applied before production use; existing tenant-level rows
  are fallback defaults and should be reviewed against each branch.
- SQL-Agent scheduling, physical backup/restore, print/cash-drawer/barcode/LCD
  hardware, SMTP/SMS, and other edge adapters remain deployment-owned.
- The logical field key is currently carried through the existing caption
  column using a namespaced physical caption for collisions; a future schema
  wave may promote it to a dedicated `field_key` column if other consumers
  need direct SQL joins on field identity.
- Transaction screens consume the existing pricing/tax/stock contracts; this
  phase does not invent missing GL, ledger, migration, report projection, or
  hardware behavior.

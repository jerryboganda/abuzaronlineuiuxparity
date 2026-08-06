# Phase V — Preferences parity evidence

Date: 2026-08-06

## Delivered coverage

- The 17 captured tabs are registered: General, Sale, Sale Return, Purchase,
  Purchase Return, Report, BasicData, Quotation, Schedule, Adjustment,
  Purchase Order, Others, Point of Sale, Cashier Job Activity, Email, SMS,
  and Dashboard.
- The reviewed backend registry contains **417 unique persisted captions**.
  The 24 repeated legacy labels are section-local duplicates (most notably
  `Activate:` on Schedule); the existing caption-keyed legacy schema stores
  those as one effective value.
- Each registered field has a type, default, validation contract, runtime
  status, behavior note, and stable category position. Boolean, enum, integer,
  decimal, date/time, text, and secret values are represented explicitly.
- `tenant_preferences` now supports a nullable tenant default plus an
  authenticated branch override. Branch writes and reads are explicit, and
  the migration RLS policy prevents a branch-scoped value from leaking to
  another branch, including tenant administrators.
- GET `/v1/preferences` returns the effective branch values, registry metadata,
  scope, and documented divergences. PUT validates registered captions and
  typed values before writing and redacts secret values in the audit payload.
- Existing report letterhead configuration remains compatible through the
  preference save path. Pricing/tax/stock/expiry/batch/cashier metadata is
  marked `wired`, `partial`, or `stored_only` rather than claiming behavior
  that is not implemented by the corresponding business or edge adapter.

## Scheduling decision

The legacy SQL-Agent/msdb schedule fields persist as configuration only and
return `runtimeStatus: not_configured`. The API does not query `msdb`, create a
false successful job, or claim that backup/posting/service workers ran.
Deployment must provide a PostgreSQL-native scheduler/worker adapter before
those toggles can become executable.

## Validation evidence

- `go test ./services/api/... ./services/edge/... ./migration/...` — passed.
  Database integration tests were skipped because `DATABASE_URL` was not
  configured in this environment.
- `pnpm --filter @abuzar/web check` — passed, 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web build` — passed.
- `pnpm exec playwright test tests/preferences.spec.ts` — 3/3 passed.
- Full Playwright suite — the parallel run observed 64/67 with three existing
  load-sensitive workflow failures; all three passed when rerun in isolation.
  A serial run observed 66/67; the remaining browser-context infrastructure
  failure passed on isolated rerun. No preference test failed.

## Remaining gaps and deployment risks

- The migration must be applied before production use; existing tenant-level
  rows are fallback defaults and should be reviewed against each branch.
- SQL-Agent scheduling, physical backup/restore, print/cash-drawer/barcode/LCD
  hardware, SMTP/SMS, and other edge adapters remain deployment-owned.
- Duplicate section-local labels need a future `field_key` schema extension if
  each repeated Schedule `Activate:` value must be independently executable.
- Transaction screens consume the existing pricing/tax/stock contracts; this
  phase does not invent missing GL, ledger, migration, report projection, or
  hardware behavior.

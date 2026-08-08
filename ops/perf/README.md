# Phase W scale and performance harness

This is a disposable, read-only-after-seed performance harness. It never
connects to the legacy SQL Server and does not post documents to the normal
local application database.

## Disposable volume fixture

Use a protected, local-only schema-owner DSN supplied through the environment:

```powershell
$env:ABUZAR_PERF_ADMIN_DATABASE_URL = 'postgres://.../postgres?sslmode=disable'
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-phase-w.ps1
```

The default fixture is 25,000 stock movements, 10,000 GL journals, 3,000
items, and 5,000 batches. It creates a database named `abuzar_phasew_*`,
applies current migrations 001–028, seeds set-based data, runs `ANALYZE`, captures
`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` plans and repeated p50/p95 timings,
then drops the database unless `-KeepDatabase` is supplied.

The historical targets are opt-in and expensive:

```powershell
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-phase-w.ps1 -FullVolume -AllowSharedCluster -Iterations 15
```

`-FullVolume` targets 3,231,846 stock rows, 1,040,590 GL journals, 30,050
items, and 30,050 batches. It is guarded unless `-AllowSharedCluster` is
supplied.

### 2026-08-07 blocker and the fix that landed for it

A first attempt at `-FullVolume -AllowSharedCluster` against the local shared
PostgreSQL instance crashed the backend during seeding and forced cluster
recovery (`psql failed with exit code 2`, phase
`seed:3231846-stock/1040590-gl`; the blocking artifact is retained at
`tmp/phase-w-blocker-20260807-014759.json`).

**Root cause.** `ops/perf/seed-scale.sql` built every table's full row count
with one set-based `INSERT ... SELECT ... FROM generate_series(1, N)` per
table and never committed until a single final `COMMIT` at the very end. At
full volume that meant ~7.5M rows (stock ledger + matching sync events, plus
the GL/document/party-ledger bundle) were held open inside one Postgres
transaction: unbounded WAL/lock/buffer growth with no intermediate
checkpoint-friendly boundary, the `gl_journals`/`gl_lines` deferred
balance-check triggers (`assert_balanced_gl_journal`, see
`db/migrations/013_finance_ledgers.sql`) firing ~3.1M times in a single burst
at that final commit, and the migration-021 read-path indexes being
maintained row-by-row for all 7.5M inserts instead of built once in bulk.
Reproducing a smaller version of this while validating the fix also surfaced
a second, related issue: because the script never ran `ANALYZE` mid-load, the
deferred trigger's own `SELECT`s against the still-growing `gl_journals`/
`gl_lines` tables kept getting planned as sequential scans (fresh tables
report `reltuples = 0` until analyzed), making each later batch slower than
the last -- almost certainly compounding the original crash rather than
causing it outright.

**Fix.** `seed-scale.sql` now loads the large tables through PL/pgSQL
`PROCEDURE`s that loop over `batch_size`-row windows (default 50,000,
configurable via `-SeedBatchSize` on `run-phase-w.ps1`) and `COMMIT` after
each window (see the file header in `seed-scale.sql` for the full mechanism).
Each GL window inserts documents, lines, sync events, the journal, and both
`gl_lines` sides in the same transaction so the deferred balance trigger
always sees a balanced journal at commit time; the GL loop also runs
`ANALYZE gl_journals; ANALYZE gl_lines;` after every commit to keep the
trigger's query plans honest. Progress is recorded in a small
`phase_w_seed_progress` table committed alongside each window, so rerunning
`seed-scale.sql` (or `run-phase-w.ps1` with the same `-DatabaseName`, which
now detects and resumes into an existing database instead of failing on
`createdb`) against a retained failed database picks up from the last
committed window instead of restarting from zero. The migration-021 read-path
indexes are dropped before the bulk load and rebuilt with identical
definitions afterward. This was validated with a real run against a local
disposable database at ~5% of full volume (150,000 stock rows / 50,000 GL
journals, including an interrupt-and-resume test), which completed cleanly,
resumed correctly, and produced zero unbalanced GL journals.

**Updated guidance.** Batching removes the single-giant-transaction failure
mode that caused the crash, but a full `-FullVolume` run is still ~7.5M rows
of sustained WAL/disk/CPU work over what will likely be tens of minutes to a
few hours depending on hardware. Keep treating `-AllowSharedCluster` as
meaning "this Postgres instance is genuinely disposable/isolated from
anything else that matters" -- not merely "a database inside it is named
`abuzar_phasew_*`". Do not run `-FullVolume` against a shared cluster used by
other active work.

**Remaining manual step (server-level, outside this repo).** Session-level
`SET`s (`maintenance_work_mem`, `work_mem`, `synchronous_commit`) are applied
by `seed-scale.sql`, but `max_wal_size`, `checkpoint_timeout`,
`checkpoint_completion_target`, and `shared_buffers` are `PGC_SIGHUP`/
postmaster-level settings that cannot be changed from a client session. On
whatever instance is used for a genuine full-volume run, a human should raise
`max_wal_size` (and correspondingly `checkpoint_timeout`) before running
`-FullVolume`, and confirm the data volume has enough free disk for the WAL
and index-build growth of ~7.5M rows plus their indexes.

The artifact reports these budgets:

| Probe | Budget | Meaning |
|---|---:|---|
| `pos-line-add` | <150 ms | bounded stock availability DB proxy |
| `documentPostProxyMs` | <1 s | reserved for an approved disposable write fixture; the current command does not write |
| heavy stock/sales report p95 | <5 s | bounded page read proxy |

The command reports `acceptance: pending` when the historical full-volume
counts are not present. A bounded proxy passing on a small fixture is not
evidence of end-to-end POS or posting acceptance.

## Safe soak setup

`run-soak.ps1` defaults to an eight-hour duration and performs health/metrics
probes. If `ABUZAR_PERF_SESSION_COOKIE` is supplied from a protected process
environment, it also performs read-only inventory, stock-report, and finance
probes. **By default it still never sends a document post** -- this default
behavior is unchanged:

```powershell
$env:ABUZAR_PERF_SESSION_COOKIE = '<short-lived disposable session>'
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-soak.ps1
```

`.github/workflows/phase-w-performance.yml` exposes the default fixture as a
manual CI workflow and makes the historical full-volume run an explicit
workflow input. Its uploaded JSON artifact is the review gate; it is not part
of ordinary pull-request CI.

### Write-load mode (opt-in; measures real post latency)

Pass `-EnableWriteTraffic -ConfirmTestTenant` to make the soak also post real
`loose-purchase` documents at a configurable rate/concurrency and measure
p50/p95 post latency against the RUNBOOK_CUTOVER `<1s` post acceptance gate.
This replaces the hardcoded `postTraffic = 'not_run'` field with measured
data (`postTraffic.mode = 'measured'`) only when the flag is passed; the
no-flag default is untouched.

```powershell
$env:ABUZAR_PERF_WRITE_PASSWORD = '<demo tenant operator password>'
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-soak.ps1 `
  -EnableWriteTraffic -ConfirmTestTenant -WriteTenantCode demo -WriteUsername admin `
  -WriteIntervalSeconds 10 -WriteBatchSize 1 -WriteConcurrency 1
```

**Tenant safety gate.** Write traffic refuses to run unless *both*: (1) the
operator explicitly passes `-ConfirmTestTenant`, and (2) the tenant code
matches a synthetic test/demo naming pattern (`demo`/`test`/`qa`/`dev` as a
whole segment). A tenant code containing `legacy`, `reference`, `canonical`,
`production`, `migrat*`, `prod`, or `live` is **hard-denied regardless of any
flag** -- this specifically blocks the `legacy-reference-sandbox` tenant,
which holds real reconciled data from the canonical migration and must never
receive synthetic writes. The seeded `demo` tenant (8 synthetic `DEMO-`
items, 2 godowns; see `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`) is the
known-good target.

On first use against a tenant, write-load mode auto-provisions (idempotently)
a dedicated `SOAK-WRITE-SUPPLIER` canonical supplier record and posts against
it -- this keeps every synthetic document attributable to one clearly-marked
party. Every created document is tagged with an `SOAK-WRITE-<timestamp>-<id>`
reference and batch number, and is recorded (document id + version) in a
manifest file (`<output>-write-manifest.jsonl` by default). Reverse a run's
documents afterward with:

```powershell
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-soak.ps1 `
  -VoidWriteTraffic -ConfirmTestTenant -WriteTenantCode demo -WriteUsername admin `
  -WriteManifestPath '<path from the run>-write-manifest.jsonl'
```

This issues a `void` document command (same safety gate applies) for every
manifest entry, which reverses the stock/finance projections in place; source
rows are never deleted, matching the same void-reversal mechanism the API
uses elsewhere.

### Monitoring and alerting

Independent of write-load mode, the soak periodically snapshots Postgres
(`pg_stat_database`: connection count, commits/rollbacks, cache hit ratio)
when `-PgMonitorConnectionString` (or `$env:ABUZAR_PERF_PG_MONITOR_URL`) is
supplied and `psql` is on `PATH`, plus host disk-free/memory metrics
(always). Snapshots are written to `<output>-monitor.jsonl`
(`-MonitorIntervalSeconds`, default 60; `0` disables monitoring).

Each monitoring tick evaluates simple thresholds against the
RUNBOOK_CUTOVER acceptance gates (`-ReadLineAddP95BudgetMs` 150,
`-HeavyReportP95BudgetMs` 5000, `-PostP95BudgetMs` 1000, `-MaxErrorRatePercent`
1.0) and emits a `WARN`/`FAIL` line to the console, to `<output>-alerts.log`,
and to the final summary JSON's `alerts` array -- so a threshold breach does
not require a human to eyeball raw latency numbers after the fact.

```powershell
$env:ABUZAR_PERF_PG_MONITOR_URL = 'postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable'
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-soak.ps1 -MonitorIntervalSeconds 60
```

Both write-load mode and monitoring/alerting were validated with short
(under 5 minute) smoke-test runs against the local `demo` tenant, including a
forced-threshold test that confirmed `WARN`/`FAIL` alerts fire and are
recorded; an actual multi-hour soak (with or without `-EnableWriteTraffic`)
has not been run as part of this change and remains a separate, explicit
operator action against a provisioned instance.

## Cold start

`measure-cold-start.ps1` builds a temporary API binary, starts it on an
otherwise unused loopback port, waits for `/v1/health`, records the time, and
stops that exact process ID. It does not post or mutate data. The 3-second
result is a local process/health probe, not a browser cold-start claim.

`measure-browser-cold-start.ps1` launches a fresh headless Chromium per sample
at a 1936×1048 viewport and measures navigation to `/login` plus the visible
`main` element. It measures browser launch/navigation against an already
running web server; it is not an eight-hour or production-network test.

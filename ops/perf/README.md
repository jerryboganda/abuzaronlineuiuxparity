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
applies migrations 001–020, seeds set-based data, runs `ANALYZE`, captures
`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` plans and repeated p50/p95 timings,
then drops the database unless `-KeepDatabase` is supplied.

The historical targets are opt-in and expensive:

```powershell
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-phase-w.ps1 -FullVolume -Iterations 15
```

`-FullVolume` targets 3,231,846 stock rows, 1,040,590 GL journals, 30,050
items, and 30,050 batches. It was not run as part of the current change unless
an artifact explicitly says `fullVolumeLoaded: true`. Disk, WAL, index build
time, and memory should be checked before using it.

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
probes. It never sends a document post:

```powershell
$env:ABUZAR_PERF_SESSION_COOKIE = '<short-lived disposable session>'
powershell -ExecutionPolicy Bypass -File .\ops\perf\run-soak.ps1
```

The eight-hour soak is **not claimed as run** by this change. A write-enabled
post soak requires a separate disposable API/database, a generated fixture
session, cleanup, and an explicit approval because it exercises business
projections.

`.github/workflows/phase-w-performance.yml` exposes the default fixture as a
manual CI workflow and makes the historical full-volume run an explicit
workflow input. Its uploaded JSON artifact is the review gate; it is not part
of ordinary pull-request CI.

## Cold start

`measure-cold-start.ps1` builds a temporary API binary, starts it on an
otherwise unused loopback port, waits for `/v1/health`, records the time, and
stops that exact process ID. It does not post or mutate data. The 3-second
result is a local process/health probe, not a browser cold-start claim.

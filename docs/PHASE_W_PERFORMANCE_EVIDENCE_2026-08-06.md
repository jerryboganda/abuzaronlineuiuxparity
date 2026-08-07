# Phase W performance evidence — 2026-08-06

## Scope and safety

This artifact covers scale-hardening tooling, read-path indexes, bounded query
plans, API timeout/logging controls, and a cold-start probe. The legacy
application and SQL Server database were not accessed or modified. The
performance fixture was created in `abuzar_phasew_20260806185313`, populated
with set-based synthetic rows, and dropped automatically after the run.

Artifact: [`tmp/phase-w-performance-20260806-185637.json`](../tmp/phase-w-performance-20260806-185637.json)

## Actual disposable measurement

PostgreSQL 18.3, 3 samples per bounded probe, after `ANALYZE` and migration
`021_scale_read_indexes.sql`:

| Probe | Rows loaded | p50 | p95 | Budget | Result |
|---|---:|---:|---:|---:|---|
| POS line-add stock availability proxy | 25,000 stock ledger / 5,000 batches | 3.343 ms | 3.343 ms | <150 ms | observed green |
| Heavy stock report proxy | 25,000 stock ledger / 5,000 batches | 161.842 ms | 161.842 ms | <5 s | observed green |
| Heavy sales report proxy | 10,000 documents/lines | 20.017 ms | 20.017 ms | <5 s | observed green |
| Finance journals | 10,000 journals / 20,000 lines | 2.109 ms | 2.109 ms | read path | observed |
| Party ledger | 10,000 entries | 2.519 ms | 2.519 ms | read path | observed |
| GL account lines | 20,000 lines | 1.910 ms | 1.910 ms | read path | observed |

The JSON artifact includes `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` for each
probe. All probes are explicitly bounded to 101 rows and tenant/branch scope.
They are database read proxies, not end-to-end browser timings.

## Cold start

`ops/perf/measure-cold-start.ps1` built a temporary API binary, started it on
loopback port 18080, and stopped the exact process ID after `/v1/health`
returned 200:

Artifact: [`tmp/phase-w-cold-start-20260806-185853.json`](../tmp/phase-w-cold-start-20260806-185853.json)

```json
{
  "coldStartMs": 2997.2973,
  "healthy": true,
  "budgetMs": 3000,
  "acceptance": "observed_green_local_probe"
}
```

This is an API process-to-health probe only; browser/web cold start remains
unmeasured.

## Full-volume gate

The synthetic fixture contained 25,000 of the 3,231,846 target stock rows and
10,000 of the 1,040,590 target GL journals. The full-volume run was **not
executed**. Therefore the Phase W acceptance is **pending**, and no claim is
made for 3.2M/1M p95, document post `<1s`, or historical valuation/report
parity.

`-FullVolume` is available in `ops/perf/run-phase-w.ps1`; it is opt-in because
it can require substantial disk, WAL, memory, and index-build time.

## Operational controls

- `021_scale_read_indexes.sql` is idempotent and limited to measured canonical
  stock, report, GL, and party-ledger predicates. It was applied twice
  successfully on the local disposable/application development database.
- API scoped transactions set configurable local `statement_timeout` (default
  5s) and `lock_timeout` (default 1s). Reports additionally use a configurable
  5s request context.
- `/v1/metrics` exposes process-local low-cardinality counters only; slow/error
  requests log method, path, status, duration, and request ID without payloads,
  cookies, query strings, or tenant IDs.
- `ops/perf/run-soak.ps1` is an eight-hour safe read-only soak setup. It does
  not generate document posts. The soak was **not run**.

## Validation

- `go test ./services/api/... ./services/edge/... ./migration/...` — passed.
- `pnpm --filter @abuzar/web check` — passed with 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web test -- --workers=1` — 61 passed.
- `pnpm --filter @abuzar/web build` — passed.
- PowerShell parser checks for all Phase W scripts — passed.
- Disposable migrations 001–021 and set-based fixture seed — passed.
- Reapplying the Phase W index migration — passed with expected
  `IF NOT EXISTS` notices.

## Shared-worktree recheck note

The full Go suite passed immediately after the Phase W changes. A later
recheck in this shared folder was blocked by unrelated concurrent edits in
`services/api/internal/httpapi/documents.go` and `stock.go`
(`projectPostedSaleReturnFinance` undefined and an `expiry`/`sql.NullString`
type mismatch). Those business-document/stock edits were not changed by Phase
W, in line with the scope boundary. The Phase W command itself still compiles
and the recorded performance artifact remains valid.

## Continuation run — 2026-08-07

The opt-in full-volume attempt was made with approximately 474 GB free disk
and 17.7 GB free memory. It loaded the 30,050 synthetic items and batches, then
the PostgreSQL 18.3 backend terminated abnormally during the 3,231,846-row
stock source-event/ledger phase. The postmaster entered crash recovery; the
database became ready again after recovery and the retained disposable
database `abuzar_phasew_20260807013517` was explicitly dropped.

Blocker artifact:
[`tmp/phase-w-blocker-20260807-014759.json`](../tmp/phase-w-blocker-20260807-014759.json).
The blocker records `fullVolumeRequested: true`, the seed phase, and resource
state. This is not a full-volume measurement and no 3.2M/1M acceptance claim
is made. Full-volume runs are now guarded by `-AllowSharedCluster`; only an
isolated PostgreSQL service/cluster should use that override.

Safe non-executing `EXPLAIN (COSTS OFF)` checks after recovery found all nine
Phase W indexes. The party-ledger probe used
`idx_party_ledger_read_020`; the GL probe used the existing scoped journal
index with incremental sorting; the stock probe used scoped primary-key scans
because its placeholder tenant had no matching fixture rows. These checks did
not execute or mutate data.

Additional cold-start probes:

- Browser launch plus `/login` navigation: p95 **649.885 ms**, observed below
  the 3-second browser probe budget.
  Artifact: [`tmp/phase-w-browser-cold-start-20260807-015353.json`](../tmp/phase-w-browser-cold-start-20260807-015353.json)
- API process to `/v1/health`: **3073.167 ms**, `pending_review` against the
  3-second limit.
  Artifact: [`tmp/phase-w-cold-start-20260807-015401.json`](../tmp/phase-w-cold-start-20260807-015401.json)

Document post latency was not run: the harness intentionally generates no
business writes and no authenticated disposable post fixture was supplied.
The `<1s` post gate remains pending. The eight-hour soak also remains
configured but unrun.

Continuation validation:

- `go test ./services/api/... ./services/edge/... ./migration/...` — passed.
- Web check — passed with 0 errors/warnings.
- Web Playwright — **67 passed**.
- Web production build — passed.
- Phase W PowerShell script parsing — passed.

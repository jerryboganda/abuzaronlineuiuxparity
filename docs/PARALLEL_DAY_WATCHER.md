# Parallel trading day watcher

**Tool:** `migration/cmd/livecompare`
**Complements:** `migration/cmd/reconcile` (see [RUNBOOK_CUTOVER.md §4.1](RUNBOOK_CUTOVER.md#41-functional-uat-running-the-parallel-trading-day)
and [§7.4](RUNBOOK_CUTOVER.md#74-t30-to-t60-final-reconciliation))
**Status:** Build/test tooling only. Not itself a release gate — it produces
evidence *for* the Functional UAT gate's parallel trading day requirement.

## What this is, and what it isn't

`migration/cmd/reconcile` is a one-shot batch tool: point it at a frozen
source snapshot and the migrated target, and it tells you once whether
counts and business metrics match. That is exactly right for the final
T+30–T+60 cutover reconciliation, where the source is frozen and you want
one authoritative answer.

A **parallel trading day** is different: the legacy system and the new
system run side by side, live, for a full business day, while an operator
enters the same real transactions into both. Nothing is frozen. `reconcile`
run once at the end of that day would only tell you "something diverged
sometime today" — not when, not which transaction, and not while there was
still time to investigate it with the operator still at the till.
`livecompare` exists to close that gap: it runs `reconcile`'s own
comparison logic repeatedly through the day and tells a human watching it
**what changed since the last check**.

`livecompare` does not replace `reconcile`. The final batch `reconcile` run
against the frozen post-freeze snapshot remains the artifact attached to the
Counts and Business metrics gates. `livecompare`'s output is the evidence
attached to the Functional UAT gate's parallel-trading-day requirement.

## How it works

Both tools import the same `migration/internal/reconcile` package — the
table-count comparison, the business-metric evaluation, the mapping-config
loading/validation/tenant-scope rules, and the bookkeeping check are one
piece of code, not two. `livecompare` does not reimplement or approximate
any of that; it wraps it in a loop:

1. Every `-interval` (default 15 minutes), open a read-only, rolled-back
   target transaction scoped to `-tenant`, and run the same
   `ReconcileMappings` / `RunMetricChecks` / `ReadBookkeeping` calls
   `reconcile` uses.
2. Diff this poll's table/metric statuses against the previous poll's.
3. Print a console summary of what's new, what's still open, and what
   resolved itself.
4. Write the full poll (and its diff) to a timestamped JSON file and append
   it to a running JSONL log, so the whole day is auditable afterward.
5. Sleep until the next interval, or stop once `-duration` has elapsed.

## Running it

```powershell
cd D:\ABUZAR\AbuzarNext
$env:ABUZAR_SOURCE_SQLSERVER_URL = '<protected-read-only-legacy-DSN>'
$env:ABUZAR_TARGET_POSTGRES_URL  = '<protected-read-only-target-DSN>'

go run ./migration/cmd/livecompare `
  -config   'migration/maps/phase-e-core-masters.json' `
  -metrics  'migration/maps/final-acceptance-metrics.json' `
  -tenant   $env:ABUZAR_CUTOVER_TENANT_ID `
  -interval 15m `
  -duration 9h `
  -out-dir  'D:\secure-evidence\parallel-day\2026-08-08'
```

Use `-tables dbo.SaleLedger,dbo.Purledger,...` instead of `-config` for an
ad-hoc raw-count watch with no declarative mapping. Prefer a **focused**
`-config`/`-metrics` pair over the full final-acceptance mapping: a parallel
day only needs to watch the tables and metrics an operator can actually
generate transactions against in a day (sales, purchases, returns, stock,
party ledgers), not the full ~760-table canonical inventory. A smaller scope
polls faster and produces a shorter, more readable console summary each
interval.

Stop early at any time with Ctrl+C; it shuts down cleanly (finishes writing
the current poll's files, then exits) rather than mid-write.

To confirm the wiring is correct before the actual parallel day, run once
against a local/dev database with `-once`:

```powershell
go run ./migration/cmd/livecompare -source $devSource -target $devTarget `
  -tables dbo.Accounts -once -out-dir tmp\livecompare-smoke
```

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-source` / `-target` | env `ABUZAR_SOURCE_SQLSERVER_URL` / `ABUZAR_TARGET_POSTGRES_URL` | Same connection URLs `reconcile` uses |
| `-config` | — | Declarative mapping config (same format as `reconcile -config`); one of `-config`/`-tables` is required |
| `-tables` | — | Comma-separated `schema.table` list, for a mapping-free raw count watch |
| `-metrics` | env `ABUZAR_RECONCILE_METRICS` | Same metric-check JSON format as `reconcile -metrics` |
| `-tenant` | env `ABUZAR_RECONCILE_TENANT_ID` | Target tenant UUID for RLS scope, same semantics as `reconcile -tenant` |
| `-allow-canonical`, `-branch-id`, `-counter-id` | — | Same canonical-source override rules as `reconcile` |
| `-from-table`, `-to-table` | `0`, `-1` | Same mapping-range slicing as `reconcile`, applied every poll |
| `-interval` | `15m` | Time between polls |
| `-duration` | `9h` | Total run length; the watcher exits cleanly once elapsed (approximate a business day, or set to cover exactly the hours the parallel day runs) |
| `-poll-timeout` | `5m` | Deadline for one poll's DB work; a slow/failed poll is logged and skipped, it does not stop the watcher |
| `-confirm-after` | `2` | Consecutive polls a discrepancy must persist before it's reported "CONFIRMED" instead of "pending" — see **Timing skew**, below |
| `-out-dir` | `parity/catalog/parallel-day` | Where the per-poll JSON files and the JSONL log are written |
| `-once` | `false` | Run a single poll and exit — for testing the wiring, not for the actual parallel day |
| `-fail-on-confirmed-discrepancy` | `false` | Exit(1) as soon as any discrepancy is confirmed, instead of continuing to watch |

## Reading the console output

Each poll prints one block:

```
[poll 6] 2026-08-08T12:45:00Z — status: DISCREPANCY
  CONFIRMED table dbo.Purledger -> public.business_documents: mismatched (seen 2 consecutive polls since poll 5) (source=513 target=511 gap=+2, gap changed by +1 since last poll)
  RESOLVED table dbo.SaleLedger -> public.business_documents (was mismatched for 1 poll(s))
  pending  metric purchase_line_total_value_sum: source=44210.50 target=44180.00 diff=30.50000000 (seen 1 consecutive polls since poll 6)
```

- **status: MATCH** — nothing discrepant this poll.
- **status: PENDING** — a table or metric newly diverged this poll, but has
  not persisted long enough to be confirmed. Often resolves itself by the
  next poll (see below); no action needed yet.
- **status: DISCREPANCY** — something has been confirmed (persisted across
  `-confirm-after` consecutive polls) or the target's migration-bookkeeping
  state changed. Investigate with the operator while the parallel day is
  still running — that immediacy is the entire point of this tool.
- **NEW** — first poll this table/metric diverged.
- **pending** / **CONFIRMED** — same table/metric still diverged on a later
  poll; CONFIRMED once it has hit `-confirm-after` consecutive polls.
- **RESOLVED** — it was diverged last poll and matches again now. The line
  reports how many polls it stayed diverged before resolving.
- The count/value suffix shows exactly how far apart source and target are
  right now, and — once there's a previous poll to compare against — how
  much that gap moved since the last check. That is the "diverged by how
  much in the last interval" the parallel day needs.

## Timing skew is expected — how this tool avoids false alarms

The source is a **live, actively-changing** database, not a frozen
snapshot. A transaction the operator finished entering into the legacy
system 20 seconds before a poll fires may not be in PostgreSQL yet (they're
still typing it in there) — that is not a real discrepancy, just the two
manual entries happening a few seconds apart.

The declarative mapping config has no reliable "row created/modified at"
column recorded per table, so `livecompare` cannot generically exclude "the
last N minutes" of rows at the query level the way a single well-known
audit table could. Instead it uses a schema-agnostic mitigation: a
discrepancy is reported as **pending** the first time it's seen, and only
escalated to **confirmed** once it has persisted across `-confirm-after`
(default 2) *consecutive* polls. With the default 15-minute interval, a
same-transaction timing race that's seconds old resolves itself well within
one interval and shows up as **RESOLVED** on the very next poll — it never
reaches "confirmed" and never demands attention. A genuine divergence (a
transaction entered on only one side, entered with different totals, or
simply forgotten) stays open and gets confirmed. See the doc comment on
`migration/internal/livewatch` for the full reasoning.

If your operators are slower to double-enter transactions, or you want more
headroom, raise `-confirm-after` (e.g. `3`) or lengthen `-interval`; both
trade faster confirmation for a wider timing-skew margin.

One exception: bookkeeping (`migration_exceptions` / `migration_ambiguous_records`)
is read from the target only, with no cross-database timing race, so a
change there is reported as a discrepancy immediately, without waiting for
confirmation.

## Audit trail

Every successful poll writes:

- `<out-dir>/poll-0001-20260808T090000Z.json`, one file per poll, containing
  that poll's full report (`poll`) and its diff against the previous poll
  (`diff`).
- `<out-dir>/parallel-day-log.jsonl`, the same records appended one JSON
  object per line, for `jq`-friendly post-day review of the whole day at
  once.

A poll that fails (DB timeout, connection drop) is logged to stderr and
skipped — it does not write a file and does not stop the watcher, since the
legacy source staying reachable for a full business day is not guaranteed.

Attach the `<out-dir>` contents (or at minimum the JSONL log) as the
Functional UAT gate's parallel-trading-day evidence, alongside the day-end
reports, returns/purchases/permissions checks, and the final
`migration/cmd/reconcile` report already required by that gate.

## What still requires a human, physically

This tool only compares data already entered into both systems — it cannot
run the parallel day itself. A human must:

1. Run `migration/cmd/livecompare` continuously (per the command above) for
   the full business day, pointed at the **real** legacy SQL Server instance
   and the **real** migrated PostgreSQL instance — not a sandbox — while the
   pharmacy operator uses both systems side by side for every real
   transaction of the day.
2. Watch the console (or tail the JSONL log) and investigate any poll that
   reaches **DISCREPANCY** status with the operator while the day is still
   running, not after.
3. At day's end, review the full audit trail, resolve or document every
   confirmed discrepancy, and attach it as evidence for the Functional UAT
   gate.
4. Still run the final `migration/cmd/reconcile` batch reconciliation at
   T+30–T+60 per [§7.4](RUNBOOK_CUTOVER.md#74-t30-to-t60-final-reconciliation) —
   this tool does not substitute for that gate's evidence.

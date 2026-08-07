# Phase W performance evidence — 2026-08-07

## Full-volume attempt

The disposable harness was invoked from the repository root with the explicit
local schema-owner connection:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\ops\perf\run-phase-w.ps1 `
  -AdminDsn 'postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable' `
  -FullVolume -Iterations 15
```

The harness created a database using the protected `abuzar_phasew_*` prefix and
applied the migrations successfully. It then failed during the disposable
`StockReport`-shaped seed at `ops/perf/seed-scale.sql:109`, where PostgreSQL
terminated the connection while loading the full-volume batch fixture. No
performance artifact with `fullVolumeLoaded: true` was produced, so the
3,231,846-stock-row / 1,040,590-GL-journal acceptance gate remains **failed,
not green**. The harness did not connect to the application database or the
legacy SQL Server source.

The local supervisor recovered the application PostgreSQL instance after the
attempt. A post-recovery status probe reported PostgreSQL, API, edge, and web
healthy with HTTP 200. The validated disposable name
`abuzar_phasew_20260807013517` was then checked and was already absent; no
`abuzar_phasew_*` database remains in the local cluster listing.

## Current interpretation

- The bounded Phase W artifacts remain valid for the 25,000-stock / 10,000-GL
  probe only.
- The full-volume failure is a resource-capacity signal, not a product
  performance pass or a data-parity result.
- Before another full-volume attempt, provision a disposable PostgreSQL
  instance with measured memory, disk, WAL, and index headroom, or lower the
  seed/index strategy while preserving the same target row counts. Do not run
  it against the application database.
- The eight-hour soak, document-post latency budget, and production-shaped
  report p95 remain unverified.

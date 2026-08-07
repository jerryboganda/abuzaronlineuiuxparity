# Phase P - narcotics stock reports evidence

Date: 2026-08-07. This is a bounded source-shaped implementation slice; it
does not claim exact PowerBuilder report parity or physical-device acceptance.

## Implemented

- `Stock Register(For Narcotics)` and `Stock Register(Narcotics Format2)` now
  resolve to an explicit `narcotics-movement` projection rather than the
  unfiltered stock movement query.
- `Norcotics Stock Register-Generic Type Wise` now resolves to an explicit
  `narcotics-generic` projection. It groups posted movements by the captured
  generic identifier, calendar day, godown, and item, exposing signed net
  quantity and net value.
- Both projections join `stock_ledger` to `sync_events`, `stock_batches`,
  `master_items`, and `master_godowns`, require the immutable source event to
  be posted, and preserve tenant/branch/godown/batch/date/text pagination.
- The narcotics filter reads the captured item payload flag with reviewed
  casing fallbacks (`Narcotics`, `Narcotic`, `narcotics`). Generic grouping
  reads `GenericName`/`GenericCode` with an explicit `Unspecified` fallback;
  those fallbacks are visible in the report definition note.

## Focused validation

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestStockNarcoticsReadModelsUseCapturedItemFlagsAndPostedScope$' -count=1` | Passed |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `git diff --check` | Passed after the focused source/docs edits |

No full build, browser suite, CI flow, or database-backed route result is
claimed in this slice. The local focused checks do not prove source counts,
exact legacy narcotics code semantics, return/opening treatment, report
columns, print/PDF/workbook output, or golden-output parity.

## Remaining acceptance evidence

Capture representative legacy narcotics and generic-type output, approve the
actual flag/code and grouping rules, compare the result and print formats,
then reconcile the projection against the reviewed SQL Server source. The
current implementation is an explicit normalized projection with honest open
boundaries, not a complete narcotics-report acceptance claim.

# Phase M report format and print-preview evidence - 2026-08-07

This is a bounded report-workflow slice. It proves the selected-format request
and the interactive preview behavior over rows that the API actually returned;
it does not claim exact PowerBuilder report calculations or full catalog parity.

## Implemented

- `ReportDefinition` now carries an optional `selectedFormat` value.
- `GET /v1/reports/{kind}?format=...` validates the requested format against
  the tenant-configured/default format list and returns the canonical configured
  name. Unknown formats fail closed with `invalid_report_format`.
- The Svelte report surface sends the selected format during retrieval and
  rehydrates the canonical server-selected value. Changing the captured format
  dialog after retrieval performs a new retrieval with that format.
- Print preview now includes a legacy-style toolbar (first/previous/next/last,
  zoom, print, close), a horizontal ruler, letterhead/date/page metadata, and
  preview-sheet paging. Preview sheets contain at most 24 rows from the loaded
  server page; the UI labels additional server pages instead of implying that
  the full report has been loaded.
- The captured screenshot-façade behavior remains intact until the first
  pointer interaction, after which the native format controls become usable.

## Fresh evidence

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestReport(DefinitionAcceptsBoundedDatabaseLetterheadAndFormats\|FormatSelectionReturnsCanonicalConfiguredName)\|TestHistoricalGLJournalReportUsesImportedVirtualGLFields' -count=1` | Passed: report format selection and historical report contracts |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: `svelte-check found 0 errors and 0 warnings` |
| `cmd /c pnpm exec playwright test tests/smoke.spec.ts -g "Daily Sale Detail retrieves" --reporter=line` | Passed: `1 passed (6.0s)`; format request round-trip, 25-row/2-page preview, letterhead, and workbook export |
| `cmd /c pnpm exec playwright test --workers=1 --retries=1 --reporter=line` | Passed: `77 passed (2.1m)` |
| `go test ./services/api/... ./services/edge/... ./migration/... -count=1` | Passed: all API, edge, and migration packages |
| `go vet ./services/api/... ./services/edge/... ./migration/...` | Passed: no issues |
| `cmd /c pnpm --filter @abuzar/web build` | Passed: production static build |
| `powershell -NoProfile -ExecutionPolicy Bypass -File ops/local/status-local.ps1` | PostgreSQL, API, edge, and web healthy; sampled HTTP status 200 |
| `git diff --check` | Passed with only existing LF/CRLF normalization warnings |

## Remaining acceptance boundary

The exact ten PowerBuilder format calculations, per-format column widths,
golden migrated numbers, print/PDF/workbook byte or pixel comparisons, and
full-dataset server pagination remain open. The preview page split is a truthful
client presentation of the loaded result page, not a substitute for those
source-data and golden-output approvals.


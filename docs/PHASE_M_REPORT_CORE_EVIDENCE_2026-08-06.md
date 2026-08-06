# Phase M report-core slice evidence — 2026-08-06

This is a bounded Phase M implementation artifact, not an all-reports completion
claim. The legacy reference and database were not modified.

## Implemented in this slice

- A reusable report definition contract now carries projection status, columns,
  named formats, retrieval metadata, letterhead metadata, pagination metadata,
  and export capability status.
- `daily-sales-detail` exposes the ten captured format names and now reads a
  tenant/branch-scoped union of canonical cash/credit `business_documents`
  and lines plus compatibility sale projections/events. Canonical party names
  and document totals are included; only `posted` canonical and compatibility
  rows are eligible. Compatibility rows are suppressed when a canonical
  document with the same scoped identity exists.
- Existing `stock`, `item`, and `purchase-return` projections remain concrete.
- Other kinds retain the immutable event-ledger generic fallback and now label
  it explicitly in the UI.
- Letterhead and named formats are loaded from tenant-scoped
  `tenant_preferences` categories (`report:letterhead` and
  `report:format:<kind>`). Missing, malformed, or unavailable configuration
  safely uses the measured default letterhead.
- CSV is a working client export. PDF and Excel are advertised as
  `available`; the browser surface provides print-preview Save-as-PDF and an
  Excel-compatible workbook download, while the API definition now advertises
  those concrete client-side export paths.
- The report response retains the existing `kind` and `rows` fields and adds
  definition and server pagination metadata. The UI retains local paging and
  adds a print-preview state with letterhead.

## Commands and observed results

| Command | Result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `go test ./services/api/...` | Passed |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Passed |
| `pnpm --filter @abuzar/web check` | Passed: `svelte-check found 0 errors and 0 warnings` |
| `pnpm --filter @abuzar/web build` | Passed: production static build completed |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g 'Daily Sale Detail\|fallback report'` | Passed: `2 passed (6.8s)` |

The browser run used the locally installed Playwright Chromium and a temporary
local Vite server. No external service or credential was used.

## Follow-up read-model correction

During follow-up validation on 2026-08-06, Daily Sale Detail was moved from
the legacy `sales_documents`/event join to the canonical
`business_documents`/lines read model, with a duplicate-safe compatibility
union and tenant/branch predicates. The focused regression test
`TestDailySaleDetailUsesCanonicalAndCompatibilityReadModel` guards the
canonical and compatibility sources.

Follow-up validation:

- `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` — passed.
- `go test ./services/api/... ./services/edge/... ./migration/...` — passed; API package completed in 1.094s.
- `go test ./services/api/internal/httpapi -run 'Test(ReadModels|DailySaleDetail|SalesReadModel)' -count=1` —
  passed, including the non-posted canonical/compatibility regression.

## Remaining report leaves

The plan's authoritative report count is 151 leaves. This slice promotes only
four API kinds to explicit projections:
`daily-sales-detail`, `stock`, `item`, and `purchase-return`. The first three
now use normalized canonical/ledger read paths with explicitly labeled
compatibility fallbacks where applicable. All other report leaves still use
the generic fallback and require their own projection,
columns, formats, retrieval rules, and golden-number evidence.

For the captured catalog, the remaining leaves are grouped below. The exact
leaf identity remains the `hasSubmenu=false` entries under `&Reports` in
`parity/catalog/legacy-menu-tree-2026-08-05.json`; this avoids silently
renaming legacy captions:

- Daily Reports: 31 leaves (including sale, sales-return, purchase,
  purchase-return, adjustment, quotation, and purchase-order leaves).
- Stock Reports: 27 leaves.
- Sales Reports: 53 leaves.
- Purchase Reports: 17 leaves.
- Accounts Reports: 1 leaf.
- Listing: 9 leaves.
- RePrinting: 8 leaves.
- Item Reports: 7 leaves.
- Purchase Return Reports: 2 leaves.

The catalog extraction returns 153 non-submenu records because two Listing
records have blank legacy captions; this discrepancy is deliberately left open
for catalog reconciliation rather than presented as completed report work.

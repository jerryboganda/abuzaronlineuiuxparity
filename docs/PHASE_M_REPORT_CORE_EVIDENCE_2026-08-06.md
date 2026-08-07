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

## Final registry consistency follow-up — 2026-08-07

The later N/O/P/Q waves completed the reconciliation: 68 + 24 + 27 + 32 =
151 distinct non-blank catalog leaves resolve to explicit definitions. The four
direct Phase M API kinds and financial aliases are not additional catalog
leaves; two blank Listing records remain outside the authoritative count.

## Daily Sale Detail column-contract follow-up - 2026-08-07

The dedicated Daily Sale Detail projection now exposes the captured 11-column
detail contract and expands compatibility payload rows. It prefers retained
historical `Saledetail` values for price, discount, and tax fields, and uses
typed stock allocations for batch/expiry when present. This improves the
retrieval/grid contract without claiming the ten format-specific PowerBuilder
calculations or golden print output; evidence is recorded in
[`PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md`](PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md).

## Format and preview follow-up - 2026-08-07

The report response now validates and returns the selected configured format,
and the browser preview has a legacy-style toolbar, ruler, letterhead metadata,
zoom, and paging over loaded rows. The focused and full-suite results are in
[`PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md`](PHASE_M_REPORT_PREVIEW_EVIDENCE_2026-08-07.md).
Exact PowerBuilder format calculations and approved golden print output remain
acceptance work.

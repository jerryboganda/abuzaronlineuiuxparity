# Phase N — daily and sales report wave evidence

Date: 2026-08-06

This is a bounded Phase N implementation artifact. It does not claim exact
legacy report parity, and the legacy application/database were not modified.

## Scope

- The captured catalog `parity/catalog/legacy-menu-tree-2026-08-05.json` contains
  15 leaves under `Daily Reports > Sale` and `Daily Reports > Sales Return`, and
  53 leaves under `Sales Reports`.
- Those 68 captured leaves now have explicit registry entries and aggregate
  filters in `services/api/internal/httpapi/reports.go`.
- Ambiguous leaf slugs such as `Detail`, `Summary`, and `Sales` are resolved
  from the captured `legacyPath` while the response keeps the requested
  backward-compatible `kind` and `rows` fields.
- `daily-sales-detail` uses the canonical cash/credit `business_documents`
  and `business_document_lines` read model, with party/totals, and unions
  compatibility `sales_documents`/sale events without duplicate scoped
  document identities. Draft and void/voided canonical or compatibility rows
  are excluded; reports and history expose posted sales only.
- Phase N sale definitions use that same canonical-plus-compatibility sale
  union. Sales-return definitions now use the canonical posted
  `business_documents`/`business_document_lines` return read model for
  source-bound and open returns, and retain immutable `sale_return`
  compatibility events only when no canonical document with the same
  identity exists. Sale-and-return definitions still use their combined
  compatibility filter until their exact legacy grouping is captured.
- Event-level definitions expose only payload-backed values. Their
  `projectionStatus` is `event-ledger` and their note explicitly says that
  captured legacy grouping and calculated numeric fields are not implemented.
  At the time of this wave, unregistered leaves remained `generic-fallback`;
  the final sweep now resolves all 151 non-blank catalog leaves, with the six
  Item Reports history leaves retaining exact labels but an explicit empty
  generic fallback.
- Server pagination is requested by the report UI and retains `page`,
  `pageSize`, and `hasMore`; no totals are fabricated.

The captured JSON did not identify a leaf named Cash Book or Day End. No such
report was invented in this wave; any separately routed, uncatalogued leaf
remains explicitly generic until its legacy identity/output is captured.

## Focused evidence

- Go registry tests cover every registered definition/filter pair and the
  ambiguous captured-path resolution.
- Playwright covers the existing Daily Sale Detail retrieval/preview path and
  the Sales Return detail retrieval path, plus the explicit generic fallback
  behavior.

## Commands and observed results

| Command | Result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `go test ./services/api/...` | Passed |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Passed |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `pnpm --filter @abuzar/web build` | Passed: production build completed |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g "Daily Sale Detail\|Sales Return detail\|fallback report"` | Passed: 3 tests |
| `pnpm --filter @abuzar/web test` | Passed: 29 tests after the C/D contextual-catalog import fix |

## Read-model alignment follow-up — 2026-08-06

- Focused Go tests cover canonical sales visibility, duplicate suppression,
  tenant/branch predicates, posted-only filtering, and normalized stock
  balance precedence.
- The new PostgreSQL read-model integration tests are conditional on
  `DATABASE_URL` and passed against the local disposable database when its URL
  was supplied explicitly. The default environment did not provide that
  variable.

## Canonical sale-return read-model follow-up - 2026-08-06

- `sales-return-detail`, `sales-return-summary`, `sales-return-summary-inv-wise`,
  and `sales-return-detail-inv-wise` now select posted canonical return
  documents (including open cash/credit returns) and their lines. A
  compatibility `sale_return` event is retained only when its document identity
  is not already represented canonically.
- Focused evidence: `go test ./services/api/internal/httpapi -run
  'TestSaleReturnReadModel|TestSalesReportDefinitions|TestReadModelsExposeCanonicalSaleReturns'
  -count=1` passed against the local PostgreSQL database.

## Invoice-summary read-model follow-up - 2026-08-06

- `sale-summary`, `sale-summary-inv-wise`, `sale-summary-invoice-wise`,
  `sales-return-summary`, and `sales-return-summary-inv-wise` now group the
  canonical/compatibility read model once per invoice. Quantity is summed from
  line rows and the authoritative document amount is retained, so a multi-line
  invoice is not displayed as repeated full totals.
- Focused evidence: `go test ./services/api/internal/httpapi -run
  'TestInvoiceSummaryReadModelsGroupRowsOncePerDocument|TestInvoiceSummaryReportGroupsCanonicalLines'
  -count=1` passed with the local PostgreSQL cluster.

## Daily Sale Detail column-contract follow-up - 2026-08-07

The Daily Sale Detail leaf now has a dedicated 11-column canonical/compatibility
projection matching the captured output contract: Alias, Item Description, Sale
Price, Qty, Disc(%), Discount Value, Item Disc, SalesTax Value, Amount, Expiry
Date, and Batch Number. Canonical imported lines prefer retained `Saledetail`
payload values for legacy price/discount/tax fields; fresh typed lines use their
pricing snapshot, and allocated stock batches supply batch/expiry values when
available. Compatibility sale payload rows are expanded without changing the
generic six-field contract used by other report leaves.

Focused evidence and the explicit remaining format/golden-output boundary are
recorded in [`PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md`](PHASE_N_DAILY_SALES_DETAIL_EVIDENCE_2026-08-07.md).

# Phase Q — financial and remaining report evidence

Date: 2026-08-06

This is a bounded Phase Q implementation artifact. It does not claim exact
legacy-number parity. The legacy application/database were not modified.

## Implemented

- The Phase Q registry contains 32 catalog-mapped report definitions:
  - six posted normalized stock-adjustment leaves;
  - quotation detail/summary and header-wise transaction compatibility leaves;
  - Accounts Ledger, Trial Balance, and Voucher Register;
  - seven tenant-scoped listing/admin leaves;
  - eight sale/purchase reprinting leaves;
  - Deleted Sale Items Log.
  - five ItemLog-derived history projections and one AdjHeader/AdjDetail
  adjustment projection for the exact captured Item Reports labels: Sale
  Price Difference, Item Basic Data Changes, Item Sale Price Changes, New
  Item(s) Created/Defined, Item Name Changes, and Stock Adjustments Detail.
- The Stock Adjustments Detail projection now also includes posted normalized
  `stock_ledger` adjustment rows joined to their immutable inventory events,
  with signed quantity, batch, godown, item, and unit-cost values. Exact
  PowerBuilder grouping and print calculations remain open; see
  `docs/PHASE_Q_STOCK_ADJUSTMENT_EVIDENCE_2026-08-07.md`.
- `trial-balance` and `voucher-register` remain direct normalized API aliases;
  they are not additional legacy catalog leaves.
- Trial Balance and the normalized financial aliases read posted
  `gl_journals`, `gl_lines`, `finance_accounts`, and account categories, with
  tenant/branch/date/text scope and bounded pagination. The direct `GL
  Journal` alias additionally reads imported `historical_gl_entries` from
  `dbo.VirtualGl` and explicitly labeled newly posted normalized journals;
  its ten-field source-backed boundary is recorded in
  `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`.
- Customer and supplier statements read posted
  `party_ledger_entries`. Receivable/payable views expose unaged party totals
  only: the normalized source has no due date or payment-allocation
  prerequisite, so aging buckets are not fabricated.
- Output/input tax reports read posted business-document line tax snapshots
  (`tax_amount`, rates, and line totals). Customer Wise Advance Tax and
  Supplier Wise Advance Income Tax use explicit `advance_tax` snapshot
  rate/base/amount evidence with the guarded legacy `AdvanceTaxAmt` fallback;
  they do not reinterpret aggregate `tax_amount` or recompute historical
  values from current tax configuration. The withholding leaf now
  has a separate source-backed `historical_withholding_tax_entries` projection
  over reviewed `dbo.PurPayment` `WHTax*` fields; it does not reinterpret
  purchase-line advance tax. Exact source import, grouping, and print parity
  remain open.
- Posted voucher rows and normalized users/roles/master listings are scoped
  and labeled. Reprinting leaves now use explicit canonical sale/purchase line
  or invoice-summary projections with a posted compatibility fallback where no
  canonical document identity exists; exact legacy print sections remain open.
- Existing M/N/O/P definitions and client response shape remain compatible:
  `kind`, `rows`, `definition`, `page`, `pageSize`, and `hasMore` are retained.

## Coverage accounting

The final catalog reconciliation is **68 + 24 + 27 + 32 = 151 distinct
non-blank catalog leaves**. The four Phase M direct API projections and the
financial aliases are not counted as additional legacy leaves. The captured
catalog also contains two blank Listing records outside the authoritative
count.

The six Item Reports records now retain their exact legacy labels and read
normalized source-backed rows from `historical_item_changes` and
`historical_stock_adjustment_lines`. The importer retains the complete
reviewed source payload and current-master joins are optional, so missing
current masters cannot silently remove historical rows. The `New Item(s)`
view is explicitly a first-observed snapshot view; it does not assert that the
first observed `ItemLog` row proves the legacy creation event.

Exact legacy columns, retrieval defaults, calculated semantics, and print
layout remain unknown and are intentionally not claimed:

- Item Reports → History → Sale Price Difference
- Item Reports → History → Item Basic Data Changes
- Item Reports → History → Item Sale Price Changes
- Item Reports → History → New Item(s) Created/Defined
- Item Reports → History → Item Name Changes
- Item Reports → Stock Adjustments → Stock Adjustments Detail

Profit, withholding, and true age-bucket values are likewise not claimed where
their normalized source data is absent.

## Validation

Final consistency sweep: 2026-08-07.

| Command | Result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/report_q_test.go` | Passed |
| `go test ./services/api/internal/httpapi -run 'TestPhaseQ' -count=1` | Passed |
| `go test ./services/api/internal/httpapi -run 'TestCapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions' -count=1` | Passed: 151 non-blank catalog leaves resolve |
| `$env:DATABASE_URL='postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable'; go test ./services/api/... ./services/edge/... ./migration/... -count=1` | Passed: all DB-backed packages |
| `go test ./services/api/internal/httpapi -run 'TestHistorical' -count=1` | Passed: migration contract and tenant/branch-scoped source-row reads |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `pnpm --filter @abuzar/web build` | Passed: production build completed |
| `pnpm --filter @abuzar/web exec playwright test tests/phase-q.spec.ts --workers=1` | Passed: 1 test |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=2` | Passed: 67 tests |

## DB-backed sale-summary follow-up — 2026-08-07

The reported failure was a real report regression, not a migration or
transaction failure. `TestInvoiceSummaryReportGroupsCanonicalLinesOnce`
returned the correct grouped invoice and amount but serialized quantity as
`3.00000000` instead of the report contract's four-decimal `3.0000`.
`invoiceSummaryReadModelQuery` now casts grouped quantity and amount to
`numeric(19,4)` before converting to text. Posted-only filtering, grouping,
and duplicate guards were unchanged.

- PowerShell `$env:DATABASE_URL='postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable'; go test ./services/api/internal/httpapi -count=1` — passed.
- The same DB-enabled suite initially waited through PostgreSQL recovery after
  the disposable performance seed; no migration/import process was interrupted.
- `go test ./services/api/... ./services/edge/... ./migration/...` with the
  DB URL — passed.

## Remaining risks

- No historical GL reconciliation against the legacy `VirtualGl` or exact tax
  register comparison was available in this wave. The later bounded GL
  Journal read-model slice preserves imported VirtualGl fields but does not
  change that reconciliation boundary.
- The canonical `ItemLog`/`AdjHeader`/`AdjDetail` import is implemented and
  tested against target fixtures, but the protected SQL Server source was not
  reachable in this turn; source counts and business-total reconciliation are
  still open.
- Normalized compatibility report leaves retain their six-field wire-row
  behavior for client compatibility; the source-backed StockReport and
  VirtualGl aliases add explicit optional fields while their definition
  metadata carries the truthful financial meaning.
- Full 3.2M-row scale/performance acceptance remains a separate gate.

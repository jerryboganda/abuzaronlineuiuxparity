# Phase Q — financial and remaining report evidence

Date: 2026-08-06

This is a bounded Phase Q implementation artifact. It does not claim exact
legacy-number parity. The legacy application/database were not modified.

## Implemented

- The Phase Q registry contains 28 mapped report definitions:
  - six posted normalized stock-adjustment leaves;
  - quotation detail/summary and header-wise transaction compatibility leaves;
  - Accounts Ledger, Trial Balance, and Voucher Register;
  - seven tenant-scoped listing/admin leaves;
  - eight sale/purchase reprinting leaves;
  - Deleted Sale Items Log.
- GL Journal and Trial Balance read only posted `gl_journals`, `gl_lines`,
  `finance_accounts`, and account categories, with tenant/branch/date/text
  scope and bounded pagination.
- Customer and supplier statements read posted
  `party_ledger_entries`. Receivable/payable views expose unaged party totals
  only: the normalized source has no due date or payment-allocation
  prerequisite, so aging buckets are not fabricated.
- Output/input/advance tax reports read posted business-document line tax
  snapshots (`tax_amount`, rates, and line totals). They do not recompute
  historical values from current tax configuration. Withholding remains an
  empty, explicitly annotated projection because no normalized withholding
  snapshot exists.
- Posted voucher rows and normalized users/roles/master listings are scoped
  and labeled. Reprinting and quotation leaves use an explicitly labeled
  posted compatibility fallback where no canonical projection exists.
- Existing M/N/O/P definitions and client response shape remain compatible:
  `kind`, `rows`, `definition`, `page`, `pageSize`, and `hasMore` are retained.

## Coverage accounting

The phase accounting is now **4 + 68 + 24 + 27 + 28 = 151 mapped report
definitions** for the authoritative 151-leaf plan count. The captured catalog
also contains two blank Listing records outside that count.

Exact legacy fields still not implemented for the adjacent item-history
records are:

- Item Reports → History → Sale Price Difference
- Item Reports → History → Item Basic Data Changes
- Item Reports → History → Item Sale Price Changes
- Item Reports → History → New Item(s) Created/Defined
- Item Reports → History → Item Name Changes
- Item Reports → Stock Adjustments → Stock Adjustments Detail

Those records remain visibly labeled as compatibility/generic projections until
their legacy columns and source history prerequisites are captured. Profit,
withholding, and true age-bucket values are likewise not claimed where their
normalized source data is absent.

## Validation

| Command | Result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/report_q_test.go` | Passed |
| `go test ./services/api/internal/httpapi -run 'TestPhaseQ' -count=1` | Passed |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Passed |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `pnpm --filter @abuzar/web build` | Passed: production build completed |
| `pnpm --filter @abuzar/web exec playwright test tests/phase-q.spec.ts --workers=1` | Passed: 1 test |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=2` | 61 tests: 60 passed; one purchase-list timing failure passed on retry |

## Remaining risks

- No historical GL reconciliation against the legacy `VirtualGl` or exact tax
  register comparison was available in this wave.
- The report route's six-field wire row is preserved for client compatibility;
  definition column metadata carries the truthful financial meaning.
- Full-suite and scale/performance acceptance remain separate gates.

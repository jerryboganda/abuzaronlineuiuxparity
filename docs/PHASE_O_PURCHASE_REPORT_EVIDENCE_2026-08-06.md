# Phase O — purchase and supplier report wave evidence

Date: 2026-08-06

This is a bounded Phase O implementation artifact. It does not claim exact
legacy report parity, and the legacy application/database were not modified.

Follow-up: the captured `Purchase detail` and `Purchase Return detail` leaves
now have a bounded 12-field source-backed line contract. See
`docs/PHASE_O_PURCHASE_LINE_DETAIL_EVIDENCE_2026-08-07.md`.

## Implemented

- 24 captured leaves are registered explicitly:
  - 7 Daily Reports purchase, purchase-return, and purchase-order leaves.
  - 15 Purchase Reports leaves, including Supplier Wise and Purchase Order
    leaves.
  - 2 Purchase Return Reports supplier leaves.
- Ambiguous leaves resolve from their captured `legacyPath`, including
  `Detail`, `Summary`, `Purchase Order`, and the `P/O` disparity leaf.
- Canonical posted `business_documents` and `business_document_lines` are the
  primary purchase source, joined to the supplier party master.
- Posted stock-ledger quantities are used for receipt/return movement when
  available, and posted supplier party-ledger amounts are used for summarized
  document amounts when available.
- Compatibility `receiving`, `return`, and `purchase_order` events are
  included only when no tenant/branch-scoped canonical document identity
  matches. Compatibility status is posted-only and compatibility rows are
  de-duplicated by document identity.
- All definitions expose tenant/branch/date/text/supplier retrieval scope,
  server pagination metadata, one truthful default `Standard` format, CSV
  availability, disabled PDF/Excel hooks, and the existing print-preview
  letterhead metadata.
- Tax, withholding, advance-tax, profit, graph, and disparity calculations are
  not fabricated. Leaves requiring those unreconciled calculations remain
  `event-ledger` with an explicit projection note and only available columns.

## Coverage accounting

The captured report catalog has 153 non-submenu records, including two blank
Listing records; the authoritative plan count is 151. The final distinct
catalog reconciliation is 68 + 24 + 27 + 32 = 151 leaves across N/O/P/Q.
Phase Q now explicitly labels the adjacent Listing and RePrinting records that
were previously generic; exact Item Reports history columns remain
unimplemented but retain their captured labels.

## Validation

| Command | Observed result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `gofmt -d services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `go test ./services/api/internal/httpapi -run 'Test(PhaseO\|PurchaseReadModel)' -count=1` | Passed |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Passed; API, edge, and migration packages all green |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g 'purchase detail and summary\|purchase return, supplier'` | Passed: 2 tests |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g 'report\|Report\|Sale Return\|purchase detail\|purchase return, supplier'` | Passed: 7 report-focused tests |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings |
| `pnpm --filter @abuzar/web build` | Passed: production build completed |
| `pnpm --filter @abuzar/web test` | 51 passed, 2 failed; both failures were outside Phase O scope (purchase-context/GST and canonical-purchase UI behavior). All Phase O report tests passed in the full run. |

## Remaining risks

- The canonical read model does not yet provide reconciled legacy tax,
  withholding, profit, graph, disparity, manufacturer, or category
  projections; those fields remain deliberately absent.
- Exact legacy named-format lists for purchase leaves were not captured, so
  `Standard` is labeled as an application default rather than a legacy claim.

## Canonical purchase-return route follow-up - 2026-08-06

- The direct `/v1/reports/purchase-return` route now uses the same canonical
  purchase read model as the captured purchase-return leaves. Posted
  `business_documents`/lines are authoritative, with supplier ledger and
  stock-ledger values when available; compatibility `return` events remain a
  de-duplicated migration fallback.
- Focused PostgreSQL evidence: `go test ./services/api/internal/httpapi -run
  'TestPurchaseReturnReportUsesCanonicalReadModel' -count=1` passed against the
  local disposable database.
> Follow-up: the captured `Purchase detail` and `Purchase Return detail`
> leaves now have a bounded 12-field source-backed line contract. See
> `docs/PHASE_O_PURCHASE_LINE_DETAIL_EVIDENCE_2026-08-07.md`.

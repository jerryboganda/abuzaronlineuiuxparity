# Phase O — purchase and supplier report wave evidence

Date: 2026-08-06

This is a bounded Phase O implementation artifact. It does not claim exact
legacy report parity, and the legacy application/database were not modified.

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
Listing records; the authoritative plan count is 151. Phase M/N/O now have
explicit definitions for 4 + 68 + 24 = 96 leaves. 55 authoritative leaves
remain for later waves. Three adjacent purchase/supplier-looking records are
intentionally still generic: Stock Reports → Supplier Manufacturer
Association, Listing → Supplier List, and RePrinting → Purchase.

## Validation

| Command | Observed result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `gofmt -d services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go` | Passed |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g 'purchase detail and summary\|purchase return, supplier'` | Passed: 2 tests |
| `go test ./services/api/internal/httpapi` | Blocked by existing syntax errors in unowned `documents.go`/`tax.go` |
| `go test ./services/api/... ./services/edge/... ./migration/...` | Failed in API setup on existing `tax_test.go` syntax error; edge and migration packages shown passing |
| `pnpm --filter @abuzar/web check` | Failed on existing `LegacyWorkflowSurface.svelte` syntax/undefined-symbol errors |
| `pnpm --filter @abuzar/web build` | Failed on the same existing `LegacyWorkflowSurface.svelte` syntax error |
| `pnpm --filter @abuzar/web test` | Failed: 42 passed, 9 failed; the full parallel run had existing runtime failures and one Phase O navigation failure, while the focused Phase O run passed |

## Remaining risks

- Go package validation cannot complete until the unrelated syntax errors in
  the current transaction/tax work are repaired.
- The canonical read model does not yet provide reconciled legacy tax,
  withholding, profit, graph, disparity, manufacturer, or category
  projections; those fields remain deliberately absent.
- Exact legacy named-format lists for purchase leaves were not captured, so
  `Standard` is labeled as an application default rather than a legacy claim.

# Phase Q - advance tax report evidence

Date: 2026-08-07

This is a bounded source-backed implementation slice for the captured
Customer Wise Advance Tax and Supplier Wise Advance Income Tax leaves. It
does not claim exact legacy-number or print parity.

## Implemented boundary

- Customer Wise Advance Tax uses tax-advance over posted sales; Supplier Wise
  Advance Income Tax uses tax-advance-input over posted purchase kinds. Both
  projections are scoped by tenant, branch, date range, text filter, and
  bounded pagination.
- The taxable base and tax amount come from explicit per-line
  pricing.taxes entries where kind = advance_tax.
- A numeric business_documents.legacy_payload.AdvanceTaxAmt is accepted only
  as a guarded historical header fallback on the first canonical line when
  the explicit snapshot amount is absent, preventing header-amount duplication.
- Rows are omitted when no positive advance-tax amount evidence exists.
  Aggregate business_document_lines.tax_amount is not reinterpreted as
  advance tax.
- The UI discloses that the posted advance-tax rate/base/amount evidence is
  used and that values are not recomputed from current tax configuration.

## Focused evidence

- go test ./services/api/internal/httpapi -run
  'TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites|TestPhaseQQueriesArePostedAndScopeBound'
  -count=1
- cmd /c pnpm --filter @abuzar/web check
- cmd /c pnpm --filter @abuzar/web exec playwright test tests/phase-q.spec.ts
  --list --grep "Phase Q representative"
- git diff --check

No long build, full browser suite, CI flow, SQL Server import, or live
database-backed result was run for this slice.

## Remaining acceptance evidence

- Reconcile AdvanceTax/AdvanceTaxAmt and detail-table source counts and
  totals from the reviewed SQL Server catalog.
- Capture exact legacy grouping, customer/supplier identity, date defaults,
  rounding, rate/base semantics, and print/PDF/workbook output.
- Run the report against approved imported data and complete operator UAT.

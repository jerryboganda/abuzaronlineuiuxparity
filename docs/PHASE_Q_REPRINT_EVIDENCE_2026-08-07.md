# Phase Q Reprinting report evidence — 2026-08-07

## Status

The captured Reprinting leaves now expose explicit canonical sale/purchase
read-model contracts instead of generic event-ledger metadata. The existing
handlers already read canonical posted documents and retain de-duplicated
compatibility events when no canonical identity matches. Overall legacy
acceptance remains open.

## Contract mapping

| Captured leaves | Projection | Visible contract |
|---|---|---|
| Sale; Sale Format(2); Sale Format(3); Sale Format(4) | Canonical sale line detail | Alias, item description, sale price, quantity, discounts, tax, amount, expiry, batch |
| Sale (with summary reports); Sale (with header wise summaries); Selected Sales and Summaries | Canonical sale invoice summary | Invoice, date, customer, summary, quantity, amount |
| Purchase | Canonical purchase line detail | Document, date, supplier, item, quantity, purchase price, discounts, tax, amount, expiry, batch |

All three modes remain tenant/branch/date/text scoped, posted-only for
canonical rows, paginated, and compatible with the existing report response,
CSV, workbook, and print-preview workflow. The compatibility fallback remains
explicitly retained for source events that cannot be matched to a canonical
document.

## Focused evidence

- `reports.go` marks each leaf with a reprint mode, source-backed status, and
  bounded columns while preserving the existing canonical read-model query
  path.
- `report_q_test.go` checks all eight leaves, their source-backed status,
  column contracts, and sale/purchase read-model wiring.
- `report-core.ts` and `phase-q.spec.ts` align the Svelte modes, columns,
  retrieval scope, and representative reprint contracts.
- The short API, loader compile, Svelte, and whitespace checks are recorded in
  `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` after execution.

## Remaining acceptance evidence

Exact PowerBuilder selection dialogs, patient/pack/header/summary sections,
format-specific calculations, printer output, golden PDF/workbook replay, and
approval of selected-invoice semantics remain unverified. This slice does not
claim that canonical read-model availability is identical to legacy print
layout or workflow behavior.

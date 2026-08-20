# Phase Q - quotation and refused-sale report evidence

## Scope

This slice covers the captured `Quotation > Detail`, `Quotation > Summary`,
and `Refused Sales Detail` report leaves. These workflows do not post stock or
finance projections, but their canonical no-stock documents are now used by
reports instead of compatibility events alone.

## Implemented

- `refused-sales-detail` uses posted `business_documents(kind = 'refused-sale')`
  and its lines.
- `quotation-detail` uses posted `business_documents(kind = 'quotation')` and
  its lines.
- `quotation-summary` groups the same source-backed rows once per document.
- Compatibility `refused_sale` and `quotation` sync events are expanded by
  payload row when needed and excluded when a posted canonical document with
  the same aggregate identity or document number exists.
- All queries preserve tenant, branch, posted status, date, text, and bounded
  pagination parameters.
- The Svelte report definitions expose explicit six-field document/detail or
  document-summary contracts.

## Focused verification

```text
go test ./services/api/internal/httpapi -run 'Test(NoStockDocumentReportsUseCanonicalAndDeduplicatedCompatibilityRows|CapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions|PhaseQRegistryCoversTheMappedRemainingLeaves|PhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites|PhaseQQueriesArePostedAndScopeBound|PhaseNReportRegistryDefinitionsAndAggregateFilters|PhasePStockRegistryCoversCapturedLeaves)$' -count=1
cmd /c pnpm --filter @abuzar/web check
```

Both checks passed. No database-backed route result, full build, CI flow, or
broad browser suite was run in this slice.

## Remaining acceptance evidence

Exact PowerBuilder quotation/refused-sale columns, retrieval arguments,
calculated totals, print/PDF/workbook output, migrated golden-number replay,
and 1936x1048 visual approval remain open.

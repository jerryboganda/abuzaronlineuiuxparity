# Phase R ? canonical quotation and refused-sale lifecycle evidence

Date: 2026-08-06

This slice moves the captured Quotation and Refused Sales workflows onto the
same idempotent business-document lifecycle used by cash/credit sales. The
legacy application and canonical reference database were not modified.

Implemented:

- `quotation` and `refused-sale` are accepted by `POST /v1/documents/{kind}`.
- The existing tenant/branch sequence, command receipt, optimistic revision,
  immutable sync event, audit event, draft/post/save-and-post response, and
  void flow are reused without duplicating business-document logic.
- These document families validate canonical item identities and pricing but
  intentionally do not require a godown, stock availability, stock allocation,
  or finance-account projection. Cash/credit sales retain their existing stock
  and finance path.
- Svelte sales routing now sends `/app/sales?kind=quotation` and
  `/app/sales?kind=refused` to the canonical command builder and omits the
  stock-only `godownId` field.
- OpenAPI `DocumentKind` and the parity summary describe the new boundary.

Focused evidence:

- `go test ./services/api/internal/httpapi -run
  'Canonical(NoStock|StockAndFinance)|ReportDefinition|FallbackReportDefinition'`
  ? passed.
- `pnpm --filter @abuzar/web check` ? 0 errors and 0 warnings.
- `pnpm exec playwright test tests/phase-cd.spec.ts --workers=1 --reporter=line
  --grep "quotation uses"` ? 1/1 passed, including payload kind and the
  absence of a stock-only godown field.
- Local stack rebuilt and restarted from the updated binaries; PostgreSQL,
  API, edge, and web health endpoints returned 200.

Remaining acceptance boundary:

- Sale-return families still use the compatibility transaction path until
  source-sale/batch allocation, stock reversal, finance reversal, and legacy
  return numbering are reconciled against the canonical SQL Server data.

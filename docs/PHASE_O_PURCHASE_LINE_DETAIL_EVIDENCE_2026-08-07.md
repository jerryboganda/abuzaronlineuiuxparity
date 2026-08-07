# Phase O purchase line-detail evidence - 2026-08-07

This is a bounded promotion of the captured `Purchase detail` and `Purchase
Return detail` leaves. It does not promote purchase summaries, order
disparity, tax, profit, graph, or print calculations to full PowerBuilder
parity.

## Implemented

- `purchase-detail` and `purchase-return-detail` now use an explicit
  `line-detail` mode. Other Phase O leaves retain their existing summary/detail
  projections.
- Canonical posted `business_documents` and `business_document_lines` expose
  document, date, supplier, item, quantity, purchase price, discount percent,
  discount value, sales tax, amount, expiry, and batch values. Typed line
  fields are used with retained `legacy_payload`/pricing values where present;
  stock-ledger quantities are preferred when a source line allocation exists.
- Compatibility `receiving` and `return` events expand one payload row per
  line and are suppressed when a scoped canonical document identity matches.
  Tenant, branch, posted, date, and text filters remain applied.
- The web report-core mirror exposes the same 12-field contract before the API
  response arrives, with the existing Standard format default.

## Focused evidence

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'Test(PurchaseLineDetailReadModelCarriesCanonicalAndCompatibilityFields\|PhaseOReportRegistryCoversCapturedPurchaseLeaves\|PurchaseReadModelUsesCanonicalLedgersPostedFiltersAndPagination\|SalesReportDefinitionsDescribeCanonicalUnionAndReturnCompatibility)' -count=1` | Passed: line-detail SQL contract, registry metadata, existing purchase projection, and sales regressions |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: `svelte-check found 0 errors and 0 warnings` |
| `DATABASE_URL` availability for a database-backed route probe | Not available in this workspace run; no DB-backed purchase line-detail result is claimed |

## Remaining acceptance boundary

Exact PowerBuilder purchase columns, supplier/category/manufacturer grouping,
tax and profit calculations, purchase-order/disparity semantics, golden
numbers, format-specific output, print/PDF/workbook comparison, and full
canonical `Purdetail` reconciliation remain open. The canonical import still
has the documented non-positive quantity exceptions and the source SQL Server
probe requires reviewed external access.

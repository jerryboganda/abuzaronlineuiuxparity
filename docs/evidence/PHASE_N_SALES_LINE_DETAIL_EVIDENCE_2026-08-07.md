# Phase N sale line-detail evidence - 2026-08-07

This is a bounded promotion of two catalog leaves, not a claim that every sales
report has recovered its PowerBuilder grouping or calculations.

## Implemented

- `sale-detail` and `sales-return-detail` now use an explicit `line-detail`
  read-model mode instead of the six-field event-ledger grid.
- Canonical posted `business_documents` and `business_document_lines` provide
  alias, item description, sale price, quantity, discount percent/value, item
  discount, sales tax, amount, expiry, and batch fields. Retained legacy line
  payload values take precedence where present; typed stock allocations provide
  batch/expiry values when available.
- Compatibility sale-return events are expanded one payload row at a time only
  when no scoped canonical document identity exists. Tenant, branch, posted,
  date, and text filters remain applied.
- The web report-core mirror exposes the same 11-field contract before the API
  response arrives, with Standard as the bounded default format.

## Fresh evidence

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'Test(SalesReportDefinitionsDescribeCanonicalUnionAndReturnCompatibility\|ReadModelsExposeCanonicalSaleReturnsWithoutDuplicateCompatibilityRows\|SaleReturnReadModelIncludesCanonicalAndCompatibilitySources)' -count=1` | Passed: definition, SQL predicate, and DB-backed canonical/compatibility return detail checks |
| `go test ./services/api/... ./services/edge/... ./migration/... -count=1` | Passed: all API, edge, and migration packages |
| `go vet ./services/api/... ./services/edge/... ./migration/...` | Passed: no issues |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: `svelte-check found 0 errors and 0 warnings` |
| `cmd /c pnpm --filter @abuzar/web build` | Passed: production static build |
| `git diff --check` | Passed with only existing LF/CRLF normalization warnings |

## Remaining acceptance boundary

The captured legacy columns, per-format selection semantics, customer/category/
manufacturer/user grouping, profit/tax/withholding calculations, golden
numbers, print/PDF/workbook comparison, and full migrated source reconciliation
remain open for these leaves and the other 149 catalog leaves.


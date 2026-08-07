# Phase O - purchase-order disparity evidence

Date: 2026-08-07

## Status

The captured `P/O Based Purchase Disparity` report now has an explicit
canonical projection. It compares posted `purchase-order` lines with posted
`pack-purchase`, `loose-purchase`, and `opening-purchase` lines when the receipt
identifies the order through `source_document_id` or `source_document_number`.

The projection exposes purchase order, order date, supplier, item, ordered
quantity, received quantity, quantity disparity, ordered amount, received
amount, and amount disparity. It does not infer a relationship for unlinked
legacy receipts.

## Evidence

| Command | Result |
|---|---|
| `cmd /c go test ./services/api/internal/httpapi -run TestPhaseOReportRegistryCoversCapturedPurchaseLeaves -count=1` | Passed; the captured leaf is registered as a real bounded projection with ten variance columns. |
| `cmd /c go test ./services/api/internal/httpapi -run TestPurchaseOrderDisparityReadModelComparesLinkedOrderAndReceiptLines -count=1` | Passed; query uses posted order/receipt filters, source-document linkage, item identity, quantity/amount variance, and bounded pagination. |
| `cmd /c go test ./services/api/internal/httpapi -run TestPurchaseSummaryModesUseExplicitBuckets -count=1` | Passed; existing purchase summary modes remain green. |
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings. |
| `cmd /c pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g "P/O disparity report" --workers=1 --retries=0 --reporter=line` | Passed: 1/1; the report renders the variance columns and truthful source boundary. |
| `git diff --check` | Passed; only existing Windows LF/CRLF warnings were emitted. |

## Remaining acceptance boundary

No live canonical database replay was claimed in this slice. Source-table
reconciliation, unlinked legacy receipt matching, exact PowerBuilder disparity
formula/format semantics, print/PDF/Excel golden output, and operator UAT
remain open.

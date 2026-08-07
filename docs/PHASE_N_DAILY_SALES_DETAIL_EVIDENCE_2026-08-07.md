# Daily Sale Detail parity slice evidence - 2026-08-07

This is a bounded report-detail implementation artifact. It does not claim
complete PowerBuilder report parity, migrated-data reconciliation, or exact
print-output equivalence.

## Implemented contract

`daily-sales-detail` now uses a dedicated canonical/compatibility read model
instead of the generic six-field report grid. The response retains the common
document/date/party/item fields for filtering and audit context and exposes the
captured detail columns in this order:

| Key | Captured label | Source preference |
|---|---|---|
| `alias` | Alias | retained item `AliasName`, then canonical item code/legacy ID |
| `itemDescription` | Item Description | canonical line item name or compatibility payload item name |
| `salePrice` | Sale Price | retained `Saledetail.SalePrice`, then typed line unit price |
| `quantity` | Qty | canonical line quantity or compatibility payload quantity |
| `discountPercent` | Disc(%) | retained `Saledetail.DiscPerc`, pricing snapshot, or explicit effective-line fallback |
| `discountValue` | Discount Value | retained payload value or typed gross-minus-net line discount |
| `itemDiscount` | Item Disc | retained `itemflatdisc`, pricing snapshot, or typed item discount |
| `salesTaxValue` | SalesTax Value | retained `Saledetail.SalesTax`, pricing snapshot, or typed line tax |
| `amount` | Amount | canonical line total or compatibility line/document amount |
| `expiryDate` | Expiry Date | allocated stock batch, then typed line expiry |
| `batchNumber` | Batch Number | allocated stock batches, then typed line batch |

Canonical rows are posted cash/credit sale lines and remain tenant/branch
scoped. Historical imported lines prefer retained `legacy_payload` values so
the report does not silently replace captured legacy figures with recalculated
values. Compatibility sale documents/events are expanded one payload row at a
time only when no posted canonical identity exists; draft and void rows remain
excluded.

The browser retains the ten captured named formats, the misspelled legacy
retrieval-dialog title, cash/credit/date/area controls, pagination,
letterhead-backed print preview, CSV, browser PDF, and Excel-compatible
workbook export. Format-specific patient, pack-quantity, profit, and exact
PowerBuilder print calculations remain explicitly unverified.

## Focused evidence

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestDailySaleDetail\|TestSalesReadModel\|TestReadModelsExposeCanonicalSales' -count=1` | Passed, including PostgreSQL-backed canonical/compatibility rows when the local `DATABASE_URL` was available |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| `pnpm --filter @abuzar/web exec playwright test tests/smoke.spec.ts -g "Daily Sale Detail" --workers=1 --retries=0 --reporter=line` | Passed: 1 test; captured headers, detail cells, retrieval dialog, print preview, and workbook download |
| `go build -o tmp/abuzar-api-localhost.exe ./services/api/cmd/server` followed by the supervised local restart and `ops/local/status-local.ps1` | Passed: rebuilt API binary; PostgreSQL, API, edge, and web healthy with HTTP 200 |
| `git diff --check -- services/api/internal/httpapi/reports.go services/api/internal/httpapi/server_test.go services/api/internal/httpapi/read_models_integration_test.go packages/contracts/src/index.ts apps/web/src/lib/report-core.ts apps/web/tests/smoke.spec.ts` | Passed; Git emitted only the repository's existing LF/CRLF normalization warnings |

## Remaining acceptance evidence

- Compare representative migrated Daily Sale Detail rows against the legacy
  output golden file, including discount/tax rounding and multi-allocation
  batch/expiry behavior.
- Verify each of the ten formats against the captured PowerBuilder output;
  the API currently preserves the names but does not claim format-specific
  columns or calculations.
- Approve print-preview, PDF, and workbook layout/number-format comparisons at
  the captured viewport and with a real migrated data set.
- Complete the remaining report leaves, canonical migration/reconciliation,
  physical hardware, scale/soak, and pharmacy UAT gates listed in
  `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`.

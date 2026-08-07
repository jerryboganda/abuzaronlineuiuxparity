# Phase K historical GL Journal evidence - 2026-08-07

## Scope

This is a bounded source-backed improvement for the captured `GL Journal`,
Accounts Ledger, and Trial Balance reports. It does not claim full historical finance,
opening-balance, account-name, tax-register, print, or VirtualGl reconciliation
parity.

## Source and target evidence

The reviewed `migration/cmd/bulk-historical` loader maps the captured
`dbo.VirtualGl` fields `DocumentCode`, `DocumentType`, `AccCode`,
`AlternateAccCode`, `Debit`, `Credit`, `Date`, `UserCode`, `INVOICECODE`, and
`Remarks` into `historical_gl_entries`. The Phase E migration record reports
1,021,852 source rows, 1,021,801 distinct reviewed target identities, and 51
duplicates quarantined. The local canonical tenant currently contains
1,021,801 imported rows spanning 2025-01-01 through 2026-07-31.

## Implemented

- The direct `gl-journal` alias now uses a `historical-gl` read model.
- Imported `historical_gl_entries` rows are unioned with explicitly labeled
  newly posted normalized `gl_journals` rows. Historical rows preserve the
  source document code, document type, account code, alternate account,
  invoice code, user code, remarks, debit, and credit values.
- Trial Balance now aggregates posted canonical GL lines and imported
  `historical_gl_entries` by account code. Matching tenant account codes use
  their category/name; unmatched legacy codes remain explicitly labeled as
  Historical rather than being dropped.
- Accounts Ledger now unions posted canonical journal lines with imported
  `historical_gl_entries`; matching account codes use the tenant chart label,
  while unmatched rows retain a `Historical <code>` label. The cash-only
  ledger intentionally remains canonical-only because no reviewed VirtualGl
  account mapping proves which historical rows are cash.
- Retrieval is tenant-, branch-, date-, text-filter-, and pagination-scoped.
- The report definition exposes a ten-column contract and discloses the
  imported VirtualGl projection and the remaining semantic boundary.
- Existing normalized financial aliases remain unchanged and continue to read
  posted normalized projections.

## Verification

| Check | Result |
|---|---|
| `gofmt -w services/api/internal/httpapi/reports.go services/api/internal/httpapi/report_q_test.go services/api/internal/httpapi/historical_integration_test.go` | Passed |
| `go test ./services/api/internal/httpapi -run 'TestHistorical(GLJournal\|Stock)\|TestPhaseQ' -count=1` | Passed: imported VirtualGl fields, tenant/date filtering, report registry, Accounts Ledger/Trial Balance query contracts, and scope guards |
| `go test ./services/api/internal/httpapi -count=1` | Passed after the slice |
| Focused Phase Q Playwright check | Passed: `gl-journal` source disclosure and captured labels render through the report route |

The database-backed integration fixture proves one imported row is returned
for the requested tenant, branch, date, and document filter, while an older
row and another tenant's same-document row are excluded. It also verifies the
ten source-backed report columns and the numeric debit/credit values.

## Remaining acceptance evidence

Exact legacy account-name resolution, opening balances, fiscal-period rules,
document grouping, duplicate/sequence behavior, tax and party-ledger
reconciliation, all historical finance reports, print/raster/keyboard parity,
and a real PowerBuilder golden replay remain open. The protected SQL Server
source probe is still blocked by Windows Integrated Authentication's
untrusted-domain failure; no credentials were read or bypassed.

## Loader lineage guard refresh - 2026-08-07

The historical loader now derives the VirtualGl identity through the reviewed
`(DocumentCode, VRow, AccCode)` composite and fails closed if a staging batch
contains duplicate identities that the target unique key would collapse.
`cmd /c go test ./migration/cmd/bulk-historical -count=1` passed. The existing
51-row duplicate-quarantine artifact still requires source-backed review and
must not be treated as proof of exact GL parity.

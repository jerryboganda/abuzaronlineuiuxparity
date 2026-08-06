# AbuzarNext acceptance evidence — 2026-08-07

## Status

**Implementation/test gate: green. Overall 100% legacy replacement acceptance: escalated pending external and incomplete-data evidence.**

This is the current handoff for `D:\ABUZAR\AbuzarNext`. It supersedes the
older “empty database” audit wording where the later Phase E artifacts provide
more current measurements. It does not convert a route, a generic report
projection, or a deterministic hardware renderer into proof of exact legacy
behavior.

## Fresh local verification

All commands were run from the repository root against the supervised local
stack on 2026-08-07.

| Area | Command / probe | Result |
|---|---|---|
| Web types | `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| Web production build | `pnpm --filter @abuzar/web build` | Passed: SvelteKit static build completed |
| Browser workflows | `pnpm --filter @abuzar/web test` | Passed: 67/67 tests, 1 worker, bounded retries |
| Go behavior | `go test ./services/api/... ./services/edge/... ./migration/...` | Passed: 204 tests in 19 packages |
| Go static checks | `go vet ./services/api/... ./services/edge/... ./migration/...` | Passed: no issues |
| Local runtime | `ops/local/status-local.ps1` | PostgreSQL, API, edge, and web healthy; sampled HTTP status 200 |

Playwright now defaults to one worker with two retries in
`apps/web/playwright.config.ts`. The shared supervised local stack previously
caused two mocked-route failures under ten workers and one Chromium context
startup failure during a serial run. The same product tests pass serially; a
retry only covers the browser/process boundary and does not turn a failed
assertion into success.

## Proven implementation coverage

- The base PowerBuilder catalog contains 275 captured entries, and the
  contextual catalog covers the captured pack-purchase, cash-sale, item-master,
  report, and groups window states (325/326/314/306/295 entries respectively).
- The browser walk reaches 221 generated leaf routes and the direct SSR smoke
  gate covers the critical transaction, master, report, maintenance, manage,
  and generic-module deep links.
- Canonical sales, sale returns, open returns, quotations, refused sales,
  purchases, purchase returns, and purchase orders have typed document
  contracts, draft/post or save-and-post lifecycle handling, idempotency, and
  server-side validation. Purchase orders remain intentionally stock/GL-neutral.
- Master List/Detail, item supplier links, Users, Groups/permissions,
  branch/counter context, preferences, shifts, maintenance audit operations,
  report retrieval/format/preview/export controls, and offline queue plumbing
  have focused browser/API coverage.
- The local normalized store currently reports 60,569 items, 373,106 business
  documents, 1,056,182 document lines, 8,238 stock batches, 449 stock-ledger
  movements, 218 GL journals, 8,868 party-ledger entries, and 501,456 recorded
  migration exceptions. These counts include the existing sandbox/reference
  data and must not be presented as complete canonical reconciliation.
- Existing committed raster evidence records exact untouched-state comparisons
  for the shell and representative transaction, master, preference, report,
  maintenance, backup, change-user, and integrity-monitor captures. Interactive
  controls are validated by behavior tests after leaving the untouched raster
  state.

## Acceptance evidence still required

| Gate | Current truth | Evidence needed to close it |
|---|---|---|
| Canonical migration | Only reviewed enterprise/config, core-master, security, pricing/tax, purchase, and return slices are reconciled. Purchase-order/detail, sales, full stock, full GL, and remaining historical tables are still deferred. | Run reviewed canonical waves into the isolated tenant, reconcile counts plus business totals, and close or explain all migration exceptions. See `migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md` and `migration/PHASE_E_HISTORICAL_STATUS_2026-08-06.md`. |
| Stock/finance history | New canonical posting projections are tested; full legacy `StockReport`/`VirtualGl` valuation and historical ledger equivalence are not proven. | Import/reconcile the remaining stock, batch, GL, customer, and supplier ledger data and replay sampled documents to the paisa. |
| Report parity | All 151 report leaves have a typed definition or explicitly labelled compatibility projection. Several item-history/adjustment leaves still lack exact legacy columns; profit, withholding, true aging, and format-specific calculations are not claimed. | Capture legacy columns/arguments/formats and compare representative output, print preview, PDF, and workbook results against approved golden data. See `docs/PHASE_M_REPORT_CORE_EVIDENCE_2026-08-06.md` and `docs/PHASE_Q_FINANCIAL_REPORT_EVIDENCE_2026-08-06.md`. |
| Full UI/UX parity | Representative shell/dialog rasters and catalog route reachability are green. Every contextual command/window state has not received an approved legacy-vs-new raster and keyboard/focus review. | Capture the remaining PowerBuilder states at 1936x1048, run the existing PNG comparison gate, and sign each exception, including MDI/window-manager behavior. |
| Hardware | Software interfaces, deterministic ESC/POS renderers, desktop IPC, shared-secret handling, and no-adapter failures are tested. No physical device is connected in this evidence. | Pharmacy-device print/label byte or visual comparison, scanner-to-line-add timing, drawer pulse, biometric, SMS, and SMTP operator sign-off. See `docs/PHASE_U_HARDWARE_EVIDENCE.md`. |
| Scale and soak | Bounded probes are green at 25,000 stock rows / 10,000 GL journals. Full 3.2M stock rows, 1.04M GL rows, document-post latency, and the eight-hour soak are not run. | Execute `ops/perf/run-phase-w.ps1 -FullVolume` and the read-only soak on the production-shaped dataset. See `docs/PHASE_W_PERFORMANCE_EVIDENCE_2026-08-06.md`. |
| Operational acceptance | Local health, RLS probes, authentication, scoped writes, and rollback-safe local supervisors are proven. A parallel trading day, final reconciliation, cutover, and 48-hour rollback rehearsal are not. | Pharmacy operator UAT, cutover rehearsal, final incremental import, rollback rehearsal, and signed go/no-go record. |

## Smallest next decision

The code and local verification gates require no further approval. To reach
accepted replacement status, the owner must authorize the remaining reviewed
canonical import window and provide the real-device/UAT boundary (or explicitly
sign off those items as deferred). Until then this task is **not accepted as
100% legacy parity**, even though the current implementation and automated test
gates are green.

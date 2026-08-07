# Handoff Report — Step 0 Survey R3 & R4

**Date**: 2026-08-07  
**Agent**: Explorer (`explorer_survey_2`)  
**Task**: Survey R3 (Pricing Policy, Stock Balance & Financial Engine Parity) and R4 (Report Engine & Hardware Integration Standard)  
**Working Directory**: `d:\ABUZAR\AbuzarNext\.agents\explorer_survey_2`  

---

## 1. Observation

Direct observations from codebase inspection, tests, and documentation:

1. **Original Requirements & Task Boundary**:
   - `d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md`: R3 requires exact-decimal calculation for sale/purchase pricing engines, 10-tier SalePrice selection, discount/tax rules, stock balance projections (StockReport), back-date snapshots, and historical GL ledger projections (VirtualGl). R4 requires validating 151 catalog report definitions with full format parameterization, print-preview features (ruler, zoom, loaded-row paging, letterhead), export formats, and hardware interface abstractions (ESC/POS, cash drawer, barcode).

2. **Pricing Engine Implementation**:
   - `services/api/internal/pricing/pricing.go` (lines 1–200): Uses `math/big.Rat`, `Money` (int64 minor units), and `BasisPoints` (int64 basis points). Floating point is zero-used. Implements 10-tier `PriceTiers`, supplier scheme bonus/discounts, customer discount precedence (`OverridePercent` > `CustomerPercent` > `GroupPercent`), flat discounts, misc fees, and GST/PCT/Advance tax rules (inclusive/exclusive).
   - `services/api/internal/httpapi/pricing.go` (lines 57–120): `POST /v1/transactions/preview` exposes exact-decimal calculation engine to SvelteKit web client.

3. **Stock & Financial Projections**:
   - `services/api/internal/httpapi/stock.go`: `GET /v1/inventory/balance` projects stock balance from `stock_balances` with fallback to `inventory_movements`.
   - `services/api/internal/httpapi/reports.go` (lines 90–371): Stock Back Date report reads `historical_stock_snapshots` (`dbo.StockReport`), preserving stock, purchase/sale/avg/recent prices, and pack units. Historical GL Journal reads `historical_gl_entries` (`dbo.VirtualGl`) unioned with `gl_journals`.
   - `services/api/internal/httpapi/void_reversal.go` & `db/migrations/028_business_document_void_reversals.sql`: Provides compensating void reversals for sales, sale returns, purchases, and purchase returns.

4. **151 Report Definitions & Catalog Mapping**:
   - `services/api/internal/httpapi/report_q_test.go` (lines 12–43): `TestCapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions` asserts that all 151 non-blank report leaves in `parity/catalog/legacy-menu-tree-2026-08-05.json` resolve to explicit registry definitions (`leaves == 151`).
   - `apps/web/src/lib/report-core.ts` (lines 57–254): Registers all 151 report kinds across `phaseNReportKinds` (57), `phaseOReportKinds` (24), `phasePReportKinds` (27), `phaseQReportModes` (41), `daily-sales-detail`, and `gl-journal`.
   - `apps/web/src/routes/app/report/[kind]/+page.svelte`: Implements report preview surface with letterhead ("Fazal Din's Pharma Plus"), zoom, ruler, loaded-row paging, and CSV/workbook export hooks.

5. **Hardware Integration Subsystem**:
   - `services/edge/internal/hardware/escpos.go` (lines 57–134): `RenderSaleSlip` and `RenderPurchaseLabels` render deterministic ESC/POS binary buffers. Test goldens: sale slip (315 bytes), purchase labels (84 bytes).
   - `services/edge/internal/hardware/registry.go` (lines 41–61, 140–200): Provides `CashDrawerAdapter` with pin pulse bytes (`0x1b 0x70`), `BarcodeLookupAdapter` with control character stripping, and capability readiness reporting (`/v1/hardware/readiness`). Returns `503 hardware_adapter_unavailable` when physical adapters are absent.
   - `docs/PHASE_U_HARDWARE_EVIDENCE.md`: Documents desktop Tauri IPC bridge storing secrets in Windows Credential Manager.

6. **Documentation & Gaps**:
   - `docs/IMPLEMENTATION_STATUS.md` & `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`: Implementation tests green. Open gaps noted: customer/group price auto-assignment on customer selection, `PricePolicyDetail` promotion date semantics, 50-invoice historical golden replay validation, 16 tax ambiguities in sandbox tenant, full 10-format calculation engine for complex report aggregations, golden PDF print comparison, and physical hardware device testing.

---

## 2. Logic Chain

1. **R3 Pricing Policy Parity**:
   - *Observation*: `services/api/internal/pricing/pricing.go` implements exact-decimal math using `math/big.Rat`, `Money`, `BasisPoints`, 10 price tiers, supplier schemes, discount precedence, and tax policies.
   - *Logic Step*: The pricing core handles precise monetary math without floating-point errors.
   - *Observation*: `services/api/internal/httpapi/pricing.go` exposes `POST /v1/transactions/preview`.
   - *Logic Step*: SvelteKit frontend can dynamically request pricing calculations during order entry.
   - *Observation*: `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` notes customer/group price auto-selection and 50 historical invoice replay are open.
   - *Logic Step*: Core engine calculation is complete, but dynamic customer selection and historical golden replay remain open gaps.

2. **R3 Stock & Financial Engine Parity**:
   - *Observation*: Stock balance API reads `stock_balances`. Historical Back Date report queries `historical_stock_snapshots` (`dbo.StockReport`). GL Journal queries `historical_gl_entries` (`dbo.VirtualGl`) unioned with `gl_journals`. Compensating voids implemented via `028_business_document_void_reversals.sql`.
   - *Logic Step*: Database projections and backend endpoints exist for stock balances, historical snapshots, GL journal views, and posted document reversals.

3. **R4 Report Engine Parity**:
   - *Observation*: `report_q_test.go` verifies 151 catalog report leaves map to explicit definitions. `report-core.ts` and Svelte report routes support preview, letterhead, ruler, zoom, loaded-row paging, format parameter selection, and CSV/workbook export hooks.
   - *Logic Step*: The report engine architecture and catalog definitions are 100% mapped (151/151).
   - *Observation*: `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` notes that many reports use event-ledger or compatibility projections, and full multi-format calculations remain open.
   - *Logic Step*: Report catalog definition is complete, but exact complex report output calculation engines require further implementation for specific reports.

4. **R4 Hardware Integration Parity**:
   - *Observation*: `services/edge/internal/hardware` contains ESC/POS receipt/label renderers, cash drawer pulse, barcode scanner lookup, and capability readiness reporting (`/v1/hardware/readiness`). Desktop Tauri IPC integrates with Windows Credential Manager.
   - *Logic Step*: Hardware software abstraction and edge infrastructure are fully implemented. Physical device acceptance remains open as no physical devices are connected.

---

## 3. Caveats

1. **Read-Only Survey**: No source code was modified during this survey, in compliance with read-only explorer constraints.
2. **Database Replay Not Executed**: Local PostgreSQL database replay was not re-run during this step; status relies on verified evidence in `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`.
3. **Physical Hardware**: No physical ESC/POS printers, barcode scanners, or cash drawers were physically connected; hardware verification relies on automated mock/virtual adapter test suites.

---

## 4. Conclusion

Requirements **R3** and **R4** have strong, well-architected implementations across the AbuzarNext monorepo (`services/api`, `services/edge`, `apps/web`, `db/migrations`).

- **R3 (Pricing, Stock, Finance)**: Exact-decimal pricing engine, 10-tier SalePrice selector, stock balance projections, historical stock snapshots (`dbo.StockReport`), historical GL entries (`dbo.VirtualGl`), and compensating voids are implemented and passing tests. Main open areas: dynamic customer price policy auto-selection, historical 50-invoice paisa replay, and sandbox tax rule ambiguities.
- **R4 (Reports, Hardware)**: All 151 report catalog leaves are registered and mapped. Print preview (letterhead, zoom, ruler, paging) and export hooks are functional. Edge hardware service provides ESC/POS rendering, cash drawer pulse, barcode lookup, and Tauri IPC integration. Main open areas: complex multi-format report calculations and physical device pilot sign-off.

---

## 5. Verification Method

To independently verify the survey findings:

1. **Run Go API unit & integration tests for Pricing, Reports, Stock, and Finance**:
   ```powershell
   go test ./services/api/internal/httpapi -run "TestPricing|TestReport|TestHistorical|TestDailySaleDetail" -count=1
   go test ./services/api/internal/pricing -count=1
   ```
2. **Run Edge Hardware & IPC tests**:
   ```powershell
   go test ./services/edge/internal/hardware ./services/edge/internal/syncapi -count=1
   ```
3. **Run Web type check and build**:
   ```powershell
   pnpm --filter @abuzar/web check
   pnpm --filter @abuzar/web build
   ```
4. **Inspect Key Code Locations**:
   - Pricing Engine: `services/api/internal/pricing/pricing.go`
   - Pricing API: `services/api/internal/httpapi/pricing.go`
   - Report Definitions & Test: `services/api/internal/httpapi/reports.go`, `services/api/internal/httpapi/report_q_test.go`
   - Report Web Core: `apps/web/src/lib/report-core.ts`
   - Hardware Subsystem: `services/edge/internal/hardware/escpos.go`, `registry.go`

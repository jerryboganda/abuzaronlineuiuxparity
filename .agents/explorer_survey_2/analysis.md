# Step 0 Survey Analysis — R3 & R4 (AbuzarNext Rebuild)

**Date**: 2026-08-07  
**Author**: Explorer Agent (`explorer_survey_2`)  
**Scope**: Requirements R3 (Pricing Policy, Stock Balance & Financial Engine Parity) and R4 (Report Engine & Hardware Integration Standard)  
**Project Root**: `d:\ABUZAR\AbuzarNext`  

---

## Executive Summary

A comprehensive survey of the AbuzarNext codebase and documentation was conducted to assess the implementation status, architectural foundations, existing features, missing gaps, and interface dependencies for Requirements R3 and R4.

- **R3 Status (Pricing Policy, Stock Balance & Financial Engine Parity)**:
  - **Pricing Engine**: The isolated `services/api/internal/pricing` package implements exact-decimal arithmetic using `math/big.Rat` (integer minor units `Money` and basis points `BasisPoints`). Supports 10-tier `SalePrice1`–`SalePrice10`, supplier scheme bonus/discounts, customer discount precedence (`Override` > `Customer` > `Group`), flat discounts, misc fees, and GST/PCT/Advance taxes (inclusive/exclusive). Exposed via `POST /v1/transactions/preview`.
  - **Stock Balance**: `stock_balances` table provides branch/godown inventory balance (`GET /v1/inventory/balance`), batch locks via `Lock Item Batches` (`stock_batches.locked`), and historical back-date snapshots (`historical_stock_snapshots` / `dbo.StockReport`).
  - **Financial Engine**: Legacy `dbo.VirtualGl` imported into `historical_gl_entries` unioned with `gl_journals`, compensating void reversals (`028_business_document_void_reversals.sql`) supporting append-only posted document reversals across stock, GL, and party ledgers.
  - **Gaps**: Customer/group price auto-assignment on customer selection, `PricePolicyDetail` date-range promotion rules, historical replay verification to the paisa (50 golden invoices), and resolution of 16 tax ambiguities without numeric rates in the sandbox tenant.

- **R4 Status (Report Engine & Hardware Integration Standard)**:
  - **Report Engine**: All 151 non-blank captured catalog report leaves are mapped to explicit registry definitions across `services/api/internal/httpapi/reports.go` and `apps/web/src/lib/report-core.ts`.
  - **Print Preview & Parameters**: Validated report format contract, frontend preview modal with letterhead metadata ("Fazal Din's Pharma Plus"), zoom, ruler, loaded-row paging, and CSV/workbook export hooks. Source-backed read models exist for Daily Sale Detail, Sale Detail, Sales Return Detail, Back Date Stock, and GL Journal.
  - **Hardware Integration Standard**: Injected hardware registry in `services/edge/internal/hardware` handling ESC/POS receipt rendering (`RenderSaleSlip`, `RenderPurchaseLabels`), cash drawer pulse commands (`Kick`, pin pulse `0x1b 0x70`), barcode scanner lookup/sanitization (`BarcodeLookupAdapter`), and capability readiness reporting (`/v1/hardware/readiness`). Desktop IPC bridges secrets in Windows Credential Manager.
  - **Gaps**: Complex 10-format calculation engine for report aggregation/breakdowns (many reports currently return event-ledger or compatibility projections), golden PDF/workbook output verification against PowerBuilder baseline reports, and physical hardware device acceptance (tests currently use mock/virtual adapters).

---

## 1. Feature Inventory & Code Mapping

### Requirement R3: Pricing Policy, Stock Balance & Financial Engine Parity

| Feature Area | Key Subcomponents | Implementation File(s) / DB Tables | Parity & Implementation Status |
|---|---|---|---|
| **Pricing Engine** | Exact-decimal math (`math/big.Rat`), integer minor units (`Money`), basis points (`BasisPoints`) | `services/api/internal/pricing/pricing.go` | **Complete Core Calculation Unit**: Zero float arithmetic. Deterministic rounding (RoundHalfUp/RoundHalfEven/RoundDown). |
| **10-Tier SalePrice** | `SalePrice1` through `SalePrice10` selection & tier array payload | `services/api/internal/pricing/pricing.go`<br>`services/api/internal/httpapi/pricing.go`<br>`apps/web/src/routes/app/sales/+page.svelte` | **Implemented**: Preview API accepts all 10 tiers, frontend tier selector updates line items and debounces preview request. |
| **Discount Hierarchy** | Supplier scheme, Item discount, Customer discount precedence (`Override` > `Customer` > `Group`), Document % & Flat discount | `services/api/internal/pricing/pricing.go`<br>`services/api/internal/httpapi/maintenance_item.go`<br>`db/migrations/010_master_normalized.sql` | **Implemented**: Supplier discount/bonus inheritance from `item_suppliers`. Group-scope price setting (`GroupAllowedPrice`).<br>*Gap*: Dynamic customer/group auto-pricing lookup upon customer select. |
| **Tax Rules Engine** | GST, PCT, Advance Income Tax (Inclusive & Exclusive calculations) | `services/api/internal/pricing/pricing.go`<br>`services/api/internal/httpapi/tax.go`<br>`db/migrations/018_tax_configuration.sql` | **Implemented Engine**: 30,052 GST & PCT item-tax assignments imported into canonical tenant.<br>*Gap*: 16 unassigned numeric rate tax rules in sandbox tenant. |
| **Stock Balance & Projection** | Branch/godown balance query, fallback to `inventory_movements` | `services/api/internal/httpapi/stock.go`<br>`db/migrations/012_stock_ledger.sql` | **Implemented**: `GET /v1/inventory/balance` queries `stock_balances`. |
| **Batch Management** | Batch lock mutation & row-locking audit | `services/api/internal/httpapi/maintenance_item.go`<br>`db/migrations/012_stock_ledger.sql` | **Implemented**: `Lock Item Batches` updates `stock_batches.locked` with audit event. |
| **Back-Date Stock Snapshot** | `StockReport` source-backed historical snapshot projection | `services/api/internal/httpapi/reports.go`<br>`db/migrations/020_historical_migration_wave.sql` | **Implemented**: Reads `historical_stock_snapshots` (`dbo.StockReport`), preserving stock, purchase/sale/avg/recent price, pack units. |
| **Financial Engine & VirtualGl** | Historical GL ledger unioned with new journals, compensating voids | `services/api/internal/httpapi/reports.go`<br>`services/api/internal/httpapi/void_reversal.go`<br>`db/migrations/013_finance_ledgers.sql`<br>`db/migrations/028_business_document_void_reversals.sql` | **Implemented**: `historical_gl_entries` unioned with `gl_journals`. Compensating void reversals for sales, purchases, and returns. |

---

### Requirement R4: Report Engine & Hardware Integration Standard

| Feature Area | Key Subcomponents | Implementation File(s) / DB Tables | Parity & Implementation Status |
|---|---|---|---|
| **Catalog Definitions (151 Reports)** | 151 mapped non-blank catalog report leaves with typed definitions | `services/api/internal/httpapi/reports.go`<br>`services/api/internal/httpapi/report_q_test.go`<br>`apps/web/src/lib/report-core.ts` | **Complete Registry Coverage**: All 151 catalog leaves resolve to explicit definitions (`phaseNReportKinds` [57], `phaseOReportKinds` [24], `phasePReportKinds` [27], `phaseQReportModes` [41], `daily-sales-detail`, `gl-journal`). |
| **Report Preview & Controls** | Print preview toolbar, letterhead, ruler, zoom, loaded-row paging | `apps/web/src/routes/app/report/[kind]/+page.svelte`<br>`apps/web/src/lib/report-core.ts` | **Implemented**: Preview UI renders letterhead ("Fazal Din's Pharma Plus"), ruler, zoom control, and loaded-row paging. |
| **Format Parameterization** | Parameterized report format selection | `services/api/internal/httpapi/reports.go`<br>`apps/web/src/lib/report-core.ts` | **Implemented API Contract**: Format selection round-trips via API.<br>*Gap*: Multi-format layout calculation engine. |
| **Export Formats** | CSV export, Print actions, Workbook/Excel export hooks | `services/api/internal/httpapi/reports.go`<br>`apps/web/src/routes/app/report/[kind]/+page.svelte` | **Implemented**: Export hooks exposed on report endpoints; client triggers CSV download and window print. |
| **ESC/POS Receipt Renderer** | Sale slip ESC/POS byte generator | `services/edge/internal/hardware/escpos.go` | **Implemented**: `RenderSaleSlip` produces deterministic 315-byte ESC/POS binary buffer (golden byte verified). |
| **ESC/POS Label Renderer** | Purchase label ESC/POS byte generator | `services/edge/internal/hardware/escpos.go` | **Implemented**: `RenderPurchaseLabels` produces deterministic 84-byte ESC/POS binary buffer (golden byte verified). |
| **Cash Drawer Integration** | Cash drawer pulse command (`Kick`) | `services/edge/internal/hardware/registry.go` | **Implemented**: Injected `CashDrawerAdapter`, generates ESC/POS pin pulse bytes (`0x1b 0x70`). |
| **Barcode Scanner Lookup** | Barcode HID-wedge normalization & lookup adapter | `services/edge/internal/hardware/registry.go` | **Implemented**: Sanitizes input, rejects control characters, delegates to `BarcodeLookupAdapter`. |
| **Hardware Registry & Readiness** | Hardware capabilities status reporting, IPC credential bridge | `services/edge/internal/hardware/registry.go`<br>`services/edge/internal/syncapi/server.go`<br>`apps/desktop/src-tauri` | **Implemented**: `/v1/hardware/readiness` reports readiness; desktop IPC stores shared secrets in Windows Credential Manager. Returns `503` when no physical adapter is injected. |

---

## 2. Existing vs. Missing Implementation Analysis

### Requirements R3 Deep Dive

#### What Exists
1. **Deterministic Calculation Core**: `services/api/internal/pricing` is fully isolated and tested without floating-point precision loss. Uses `math/big.Rat` for basis point calculations.
2. **10-Tier Sale Price**: The pricing API accepts all 10 price levels per item, mapping to `SalePrice1`–`SalePrice10`. The Svelte frontend (`apps/web/src/routes/app/sales`) allows selecting any of the 10 tiers and live-previews totals.
3. **Item Supplier Schemes**: Canonical purchase documents and sales previews inherit discount and bonus unit schemes from `item_suppliers`.
4. **Group Allowed Price Rights**: Group settings enforce role-based access for setting allowed godowns, prices, headers, cash accounts, and supplier categories (`Phase R`).
5. **Stock Balance & Snapshot Projections**: `GET /v1/inventory/balance` queries normalized `stock_balances`. Back-date reports query `historical_stock_snapshots` (`dbo.StockReport`), returning as-of stock, unit costs, and pricing.
6. **Compensating Voids**: Migration `028_business_document_void_reversals.sql` provides append-only void reversals for sales, sale returns, purchases, and purchase returns, updating stock and GL ledgers while maintaining audit trails.

#### What Is Missing / Open Gaps
1. **Dynamic Customer/Group Price Policy Auto-Selection**: Automatic price policy resolution based on selected customer (`PricePolicyDetail` date range & promotional policy) is not yet hooked into document loading.
2. **50-Invoice Golden Historical Replay**: Exact paisa-level replay validation against 50 approved historical SQL Server sales invoices is pending.
3. **Unresolved Tax Rules**: 16 ambiguous tax rule entries in the sandbox tenant (e.g., "NO TAX", "TAX ON ACTUAL QTY ONLY") lack numeric rate definitions and require business rule clarification.
4. **Full Historical Stock/GL Replay**: Full historical stock movements and ledger opening balances from SQL Server remain un-replayed in the live canonical environment.

---

### Requirements R4 Deep Dive

#### What Exists
1. **151 Catalog Definitions**: All 151 catalog report leaves have explicit definitions registered in `services/api/internal/httpapi/reports.go` and verified in `report_q_test.go`.
2. **Rich Read Models**: Bounded source-backed read models exist for:
   - Daily Sale Detail (11 fields: Alias, Item Description, Sale Price, Qty, Disc%, Disc Val, Item Disc, SalesTax Val, Amount, Expiry Date, Batch Number)
   - Sale Detail & Sales Return Detail
   - Historical Stock Back Date (10 fields)
   - Historical VirtualGl GL Journal (10 fields)
3. **Print Preview & Controls**: Svelte report surface implements letterhead headers, ruler display, zoom controls, pagination over loaded rows, and CSV/workbook export hooks.
4. **Hardware Subsystem (`services/edge/internal/hardware`)**:
   - `RenderSaleSlip` (ESC/POS receipt format, tested with 315-byte golden sample).
   - `RenderPurchaseLabels` (ESC/POS barcode label format, tested with 84-byte golden sample).
   - Cash drawer `Kick` implementation issuing ESC/POS pulse `0x1b 0x70`.
   - Barcode scanner lookup abstraction with control-character stripping.
   - Capability readiness API (`/v1/hardware/readiness`) returning `503 hardware_adapter_unavailable` when physical adapters are missing.
   - Desktop IPC wrapper utilizing Windows Credential Manager for edge shared secret storage.

#### What Is Missing / Open Gaps
1. **Complex Report Layout & Format Calculation Engine**: Many of the 151 reports currently fall back to event-ledger projections or compatibility views rather than executing PowerBuilder-specific grouping, profit margin, withholding tax, or aging calculations.
2. **Golden Print/PDF/Workbook Parity**: Comparison of generated PDF/print outputs against legacy PowerBuilder baseline print rasters is incomplete.
3. **Physical Hardware Acceptance**: No physical ESC/POS printer, barcode scanner, or cash drawer hardware has been physically attached for end-to-end device testing. Physical pilot testing remains open (`PHASE_U_DEVICE_ACCEPTANCE_CHECKLIST.md`).

---

## 3. Dependencies & Interface Requirements

```
+-------------------------------------------------------------------+
|                  SvelteKit Frontend (@abuzar/web)                |
|  - Sales / Purchase UI (10 SalePrice Tiers, Preview Debounce)    |
|  - Report Viewers (151 Definitions, Preview Modal, Letterhead)    |
+---------------------------------+---------------------------------+
                                  | HTTP / JSON
                                  v
+-------------------------------------------------------------------+
|                     Go API (services/api)                         |
|  - POST /v1/transactions/preview (Exact-Decimal Pricing Engine)   |
|  - GET  /v1/reports/{kind}        (151 Mapped Report Read Models) |
|  - GET  /v1/inventory/balance     (Stock Ledger Balance)          |
|  - POST /v1/documents/{id}/void   (Compensating Void Engine)      |
+---------------------------------+---------------------------------+
                                  | SQL / RLS
                                  v
+-------------------------------------------------------------------+
|               PostgreSQL Database (db/migrations)                 |
|  - master_items, stock_balances, business_documents, gl_journals  |
|  - historical_stock_snapshots, historical_gl_entries              |
+-------------------------------------------------------------------+

+-------------------------------------------------------------------+
|                     Branch Edge (services/edge)                   |
|  - Hardware Registry (Printer, Cash Drawer, Barcode Scanner)      |
|  - ESC/POS Renderers (RenderSaleSlip, RenderPurchaseLabels)       |
|  - IPC Bridge (Tauri / Windows Credential Manager)                |
+-------------------------------------------------------------------+
```

### Interface Dependencies
1. **Web to API**:
   - `POST /v1/transactions/preview`: Transports line items, price tier selection, discounts, and tax overrides. Returns exact decimal totals.
   - `GET /v1/reports/{kind}`: Accepts `startDate`, `endDate`, `cashCredit`, `formatId`, `page`, `pageSize`, `search`. Returns columns, rows, letterhead, and export options.
2. **API to Database**:
   - Multi-tenant RLS enforcement (`tenant_id`, `branch_id`).
   - Stored projections over `business_documents`, `stock_balances`, `gl_journals`, `historical_stock_snapshots`, and `historical_gl_entries`.
3. **Desktop / Web to Edge**:
   - `http://127.0.0.1:8091/v1/hardware/*` protected by shared edge secret.
   - Native Tauri commands routing hardware requests to edge API.

---

## 4. Recommendations for Subsequent Implementation Steps

1. **R3 Pricing & Financials**:
   - Implement dynamic `PricePolicyDetail` lookup when selecting customers on sale screens.
   - Run 50 historical sales invoice calculations through `pricing.Calculate` to verify paisa-level precision parity against SQL Server legacy totals.
   - Resolve 16 tax rule ambiguities in the sandbox tenant with business stakeholders.

2. **R4 Reports & Hardware**:
   - Transition report event-ledger fallbacks to dedicated calculation views for high-priority business reports (e.g. Sales Tax, Customer Profit, Aging).
   - Perform physical device validation with actual ESC/POS thermal printers, barcode scanners, and cash drawers using `PHASE_U_DEVICE_ACCEPTANCE_CHECKLIST.md`.

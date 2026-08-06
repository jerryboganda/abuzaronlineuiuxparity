# ABUZAR Legacy → AbuzarNext: Verified Gap Analysis

Date: 2026-08-06
Method: live side-by-side audit. The legacy application (WASEELA ABUZAR V3, PowerBuilder, `V2_AbuzarSoftware\Application\abuzar.exe`, DB `FazalDinPP19DataBaseV2`) was launched on localhost, logged in as ADMIN, and walked visually screen by screen. The new stack (SvelteKit + Go + PostgreSQL, `AbuzarNext`) was launched via `ops/local/start-local.ps1` and walked with a real browser at the same 1936×1048 viewport. Menus were enumerated programmatically (Win32 `GetMenu`/`GetSubMenu`), databases were queried on both sides, and the Go/Svelte source was audited file by file.

Evidence directory: `AbuzarNext/tmp/gap-audit/` (screenshots, menu-tree JSONs, driver scripts).

---

## Executive summary

| # | Gap area | Severity | Status in new app |
|---|----------|----------|-------------------|
| G1 | Data migration never executed (763 legacy tables, 30K items, 3.2M stock rows → new DB essentially empty) | **S0 blocker** | Tooling scaffolded, no mapping files, never run |
| G2 | Core business logic missing (tax, discounts, stock checks, batch/expiry, GL, ledgers) | **S0 blocker** | Documents persist as generic events only |
| G3 | Reports: 151 legacy report leaves vs 3 real projections; no formats, no print preview, no letterhead, no export | **S0 blocker** | Generic 6-column grid fallback |
| G4 | Menu catalog incomplete: 275 items captured vs 325+ live (contextual per-window menus missing entirely) | S1 major | Baseline never captured document-window menus |
| G5 | Security/rights model absent (4 groups, 726 GroupRights rows, menu gating, startup rights) | S1 major | No rights model in new schema |
| G6 | Master data: 20+ masters fall to a generic Code/Name form; Item master missing Suppliers sub-grid and behaviors | S1 major | 4 explicit forms only, empty DB |
| G7 | Transaction surface field/layout gaps (Cash Sale, Pack Purchase, returns, quotation) | S1 major | Main fields present, many columns/behaviors missing |
| G8 | Maintenance/Manage workflows are preference-writing stubs (backup, imports, integrity, session monitor) | S1 major | Forms save values, execute nothing |
| G9 | Hardware integrations all placeholders (thermal printer, barcode, cash drawer, biometric, SMS, email) | S2 | Edge returns "No adapter configured" |
| G10 | Shell/chrome fidelity: frozen timestamp in title, display-name vs username, toolbar/status-bar/MDI differences | S2 | Raster shell approximates, not live |
| G11 | Dev-runtime instability: Vite dev server died 3× during audit; SSR 500 on module pages (since guarded) | S2 | Root cause: non-detached process + earlier missing guards |
| G12 | Mojibake: 13 CSS `content: "\\XXXX"` double-escapes rendered literal text on every surface; double-encoded UTF-8 also baked into source files, masked by a runtime MutationObserver | S2 — CSS layer **FIXED 2026-08-06**; source layer open | `styles.css` corrected & verified; `legacy-text.ts` band-aid still active |
| G13 | Menu routing bug: `Cash&Sale` cleans to `CashSale`, missed the route map → opened generic stub | S2 — **FIXED 2026-08-06** | `legacy-menu.ts` corrected, verified in browser |
| G14 | Legacy defects on SQL Server 2022 that parity must decide about (msdb SysJobs error, Groups "DataBase Error", R0002 close crash) | S3 decision | Document + design decisions needed |
| G15 | Status docs overclaim (mojibake "fixed", module SSR "fixed" while still failing in earlier logs) | S3 hygiene | Corrected via this analysis |

Severity scale: S0 = product cannot replace legacy at all; S1 = daily pharmacy operation broken; S2 = visible quality/parity defect; S3 = documentation/decision item.

---

## G1 — Data migration never executed

**Legacy (measured):** `FazalDinPP19DataBaseV2` contains 763 tables. Key row counts: StockReport 3,231,846 · VirtualGl 1,040,590 · Saledetail 620,802 · SaleLedger 291,231 · Purdetail 113,748 · PurOrderDetail 108,414 · Item 30,050 · PricePolicy 30,032 · ItemSuppliers 22,406 · Purledger 6,436 · Manufacturer 838 · Supplier 235 · Users 9 · GroupRights 726.

**New (measured):** `abuzar_next` has 22 tables; `master_records` = 0 rows, `legacy_id_mappings` = 0 rows, `sales_documents` = 4 (test docs), `inventory_movements` = 3.

**Migration workbench:** `migration/cmd/{inspect,import,reconcile}` exist and are well designed (read-only source, declarative mapping files, reconciliation report), but `migration/maps/` contains only an example metrics file — **no mapping file was ever authored and no import was ever run**.

**Impact:** every screen in the new app is empty; nothing can be functionally compared, tested, or accepted. This is the first domino for all other gaps.

## G2 — Core business logic missing (backend)

Audit of `services/api/internal/httpapi/business.go` (53 KB) and related files:

- **No tax computation.** Legacy applies GST %, PCT, advance income tax per item/party (visible in Pack Purchase "Apply Item GST%" command and report columns "SalesTax Value"). New API stores an amount, computes nothing.
- **No discount policies.** Legacy: item disc %, flat disc (−), scheme/bonus from `ItemSuppliers` (Priority/Rate/Disc%/Qty/Bonus/Days) and `PricePolicy` (30K rows, SalePrice #1–10 tiers). New sales page shows read-only `0.00` stubs.
- **No stock availability check** and **no batch/expiry enforcement** — batch/expiry are stored as strings, never validated; legacy blocks expired batches and drives "Auto Batch Generation".
- **No FIFO/batch allocation** for cost of sales; `inventory_movements` is an append-only log without valuation.
- **Invoice numbering** exists only for the `sale` aggregate; purchases/returns/quotations get no legacy-style sequences.
- **Document lifecycle:** status is hardcoded `posted`; no draft → post (legacy Save vs Post vs Save-And-Post Ctrl+Q), no cancel, no reversal, no reprint audit.
- **No PO → GRN/purchase linking** ("Fetch Purchase Invoice From Other Sources" in legacy).
- **No GL posting.** Legacy `VirtualGl` (1.04M rows) receives postings from every document; new schema has no ledger/journal tables at all.
- **No customer/supplier ledgers**, receivables/payables, or credit-limit checks (legacy SaleLedger 291K / Purledger 6.4K rows).
- `projectEvent` comment admits quotation/refused/PO are "immutable events until workflow-specific ledgers are added".

## G3 — Reports engine

**Legacy (walked live):** Reports menu = 186 entries, 151 leaf reports. Example walk of Daily Reports → Sale → Sale detail (cmd 12781):
1. "Select Format" dialog listing **10 named formats** for this one report.
2. "Specify Retrieval Arguements" dialog (sic — legacy typo) with Selectable/Selected Areas lists, All checkbox, Start/End datetime, Cash/Credit checkboxes.
3. Output opens in a **paginated print-preview**: letterhead ("Fazal Din's Pharma Plus / NRY Pacific / Franchise Fazal Din's", Phone 055 3252501 — stored in DB), ruler, ~20-button report toolbar (zoom, page nav, print, export), columns Alias · Item Description · Sale Price · Qty · Disc(%) · Discount Value · Item Disc · SalesTax Value · Amount · Expiry Date · Batch Number.

**New (measured in `reports.go` + browser):** 3 explicit projections (`daily-sales-detail`, stock/item, purchase-return); everything else falls to a generic event grid (Document/Date/Party/Item/Qty/Amount) with From/To filter. No format selection, no retrieval-argument parity, no print preview, no letterhead, no pagination, no Excel/PDF export.

**Gap:** 148 of 151 report leaves have no real implementation; the 3 implemented ones lack the format/preview/print pipeline that defines the legacy reporting experience.

## G4 — Menu catalog incomplete (contextual menus)

The parity baseline `parity/catalog/legacy-menu-tree-2026-08-05.json` (275 items, 9 menus) was captured with **no document window open**. Live enumeration with windows open shows the legacy MDI swaps in per-window menus:

- Pack Purchase open → **325 items, 10 top menus**, including an **Item** menu (New Item Ctrl+I, Delete Item Ctrl+D, Restore Item Ctrl+Z) and ~35 contextual File commands: Save, Post, Save And Post Ctrl+Q, Populate Items, Item Purchase History Ctrl+H, Purchase Slip Ctrl+M, Auto Batch Generation Ctrl+B, Print Purchase Labels Alt+F8, Apply Item GST%, Fetch Purchase Invoice From Other Sources, Attach Documents, etc.
- Cash Sale open → 326 items; Item master → 314; report window → 306 (Window menu gains Refresh variants).

Captured JSONs: `tmp/gap-audit/live-menu-tree-pack-purchase.json`, `legacy-cash-sale-menu.json`, `legacy-item-master-menu.json`, `legacy-report-sale-detail-menu.json`, `legacy-manage-groups-menu.json`.

**Impact:** the new shell renders only the base 275; none of the contextual commands (the actual working verbs of each screen) exist as menu actions, shortcuts, or API calls.

## G5 — Security / rights model absent

Legacy: `Groups` (ADMINISTRATOR, REMOTE, SALES OFFICER, SHIFT INCHARGE — screenshot `legacy-manage-groups.png`), `GroupRights` (726 rows of GroupCode/RightCode/Status), plus granular allow-tables: GroupAllowedGodown, GroupAllowedPrice, GroupAllowedStartupRight, GroupAllowedHeader, GroupCashAccount, etc. Menus/commands are enabled per group; users belong to groups.

New: `users` table with bcrypt hash and display name; no groups, no rights, no menu gating, no per-godown or per-price permissions.

## G6 — Master data

- Legacy Item master (screenshot `legacy-item-master.png`): 21 fields + **Suppliers sub-grid** (Priority/Rate/Disc%/Qty/Bonus/Days), status-bar hint "Specify Item name", List/Detail tab pattern with Sort Criteria, Filter Criteria, Find-as-you-type.
- New: 4 explicit master forms (item/customer/supplier/user); item lacks the Suppliers grid; 20+ other masters (Manufacturer, Godown, ItemGroup, CustomerGroup, areas, categories…) fall to a generic Code/Name/Active/Remarks form. All lists show "No records in the current tenant scope" (see G1).
- Legacy list screens share a chrome: Sort Criteria dropdown + Asc/Desc, Filter Criteria (column/op/value), Filter & Retrieve buttons, Find box with helper text, List↔Detail tabs. The new master pages implement a simpler single grid.

## G7 — Transaction surfaces

Cash Sale (legacy screenshot vs `/app/sales?kind=cash`):
- Legacy: Inv No, Date, **User**, Alias Name, Customer (default CASH SALES CUSTOMER), **Godown (GODOWN1)**, Ref, Remarks, SalePrice# selector, 10-column item grid, totals block with **Stock**, **Flat Disc(−)**, **Misc(+) default 1.00**, Discount %, Sales total.
- New: has Inv/Date/Alias/Customer/Ref/Remarks/SalePrice# and an item grid, but **no User/Godown fields, no Misc(+), no flat-disc behavior, no live Stock figure**, and the item lookup uses a **hardcoded 32-item fallback catalog** (`sales/+page.svelte` L29-62) instead of querying the API when the DB is empty.
- Same pattern (fields approximated, behaviors absent) applies to Credit Sale, returns, open returns, Quotation, Refused Sales, and the whole Purchase family; the deep verbs (populate items, purchase history, slips, labels, batch generation) do not exist (G4).

## G8 — Maintenance / Manage stubs

All Maintenance and Manage screens in the new app persist their form values into `tenant_preferences` and do nothing else. Legacy equivalents run real work: Backup/Restore (SQL backups), imports/exports, stock adjustments, integrity checks, Session Monitor (cmd 13869) shows live DB sessions, SMS/Email senders, user/group administration.

## G9 — Hardware integrations

Edge service returns "No adapter configured" for every registered adapter: thermal_printer, barcode_scanner, cash_drawer, biometric_reader, sms, email. Legacy prints thermal sale slips, purchase labels (Alt+F8), reads barcodes (item lookup by alias/barcode), triggers the cash drawer, and sends SMS.

## G10 — Shell / chrome fidelity

- New title bar shows the **frozen capture timestamp** "05/08/26 11:16:21" baked into the raster; legacy shows live login date/time (e.g. "ADMIN : 05/08/26 22:50:44") that ticks per session.
- New shows "LOCAL ADMINISTRATOR" (display name); legacy shows the **username** ("ADMIN").
- Legacy is a true MDI: several document windows open concurrently, Window menu lists them (Cascade/Tile/Layer/Arrange + numbered entries + Refresh variants); new app is single-page navigation.
- Legacy toolbar (New/Spray/Save/Erase, First/Prev/Next/Last, Print, Exit) and bottom status bar with contextual hints ("Specify Item name") are not reproduced on real surfaces (only in the raster shell).

## G11 — Dev-runtime instability

- Vite dev server died three times during this audit. Root cause: `start-local.ps1` spawns it attached to the launching shell; when that shell exits the child dies. Workaround applied: run detached. Fix belongs in the ops script.
- SSR 500 `Cannot read properties of undefined (reading 'split')` at `module/[slug]/+page.svelte:29` appeared in earlier server logs; the file now has guards and `/app/module/purchase-orders` returns 200. Treat as fixed but add a regression test.

## G12 — Mojibake (CSS layer FIXED; source layer remains)

13 rules in `apps/web/src/lib/styles.css` used `content: "\\2212"`-style double escapes, so every transaction/master/report surface rendered literal `\2212`, `\2190`, `\25A6 Detail`, etc. **Fixed 2026-08-06** (all 13 normalized to single-backslash CSS escapes) and verified in the browser: back arrow ←, toolbar glyphs, ▦ Detail/▦ List tabs, ›› submenu arrows now render.

**Deeper finding:** the mojibake is also baked into the **source files themselves** — e.g. `lib/LegacyWorkflowSurface.svelte` contains literal `â†` (should be ←), `â–£` (▣), `Â·` (·) in markup — and `lib/legacy-text.ts` installs a **runtime MutationObserver** with ~48 byte-sequence repair rules that rewrites every text node on every DOM mutation to hide it. This is a permanent band-aid with runtime cost and fragility (it cannot fix attributes, `aria-label`s used by accessibility tools, or CSS). Proper fix: repair the double-encoded UTF-8 in all source files once, delete `legacy-text.ts` and its layout hook. Tracked in plan Phase B.

## G13 — Menu routing to stub (FIXED)

Legacy caption `Cash&Sale` cleans (accelerator `&` stripped) to `CashSale`, which missed the `hrefFor` Sales map keyed on `'Cash Sale'` — so Sales → CashSale opened the generic `/app/module/cashsale` stub instead of the real `/app/sales` surface. **Fixed 2026-08-06** in `apps/web/src/lib/legacy-menu.ts` (added `CashSale: 'cash'`), verified: menu click now lands on `/app/sales?kind=cash&commandId=10031`.
Residual related issue: `Sales > Open Sale Return > *` maps to the same kinds as normal returns (`cash-return`/`credit-return`) — legacy open returns (no source invoice) are a distinct workflow that needs its own `kind` once implemented.

## G14 — Legacy defects on SQL Server 2022 (decisions needed)

The reference itself misbehaves on the modern SQL Server host; parity must document intended behavior rather than clone bugs:
- Preferences window: msdb `SysJobs` query error on open (job scheduling probe).
- Manage → Groups: modal "DataBase Error: Some unidentifiable problem has occured in Row No: 0…" before the Groups list opens (list works after dismissal).
- R0002 runtime crash when closing Pack Purchase / DataBase Integrity windows (`wf_disconnect`, `w_checkdb` line 15).
- Startup `SP_WayToMoon` xp_cmdshell probe fails silently.

## G15 — Status-document hygiene

`docs/PARITY_STATUS.md` and `docs/IMPLEMENTATION_STATUS.md` claimed mojibake and the module SSR crash were fixed while both were still reproducible in the running dev build earlier in this audit (the mojibake fix genuinely landed only 2026-08-06). Claims should always cite a verification artifact (screenshot/log/test) — the A-Z plan mandates this.

---

## What "100% parity" concretely requires

## Evidence update - 2026-08-06

The audit above remains the baseline gap catalogue, but several statuses changed after the Phase E and transaction-core follow-on work:

- Canonical SQL Server import is still deliberately open. A sandbox/reference import now exists as `docs/PHASE_E_STATUS_2026-08-06.md` (18 reviewed table mappings, 83,425 core rows, 0 current exceptions); it must not be presented as the canonical tenant migration.
- The local database is no longer empty: the sandbox evidence and local counts include 61,202 `master_records` and 83,447 `legacy_id_mappings`. Historical documents, ledgers, stock/batch data, and canonical reconciliation remain open.
- Rights are now represented by tenant-scoped roles, permission sets, group editing, and API enforcement (`009_legacy_security_rights.sql`); granular legacy allow-tables and complete menu gating still require parity capture.
- Pricing is now a deterministic authenticated preview and sale-post validation for exact-decimal tiers, discounts, supplier schemes, Misc, and GST/PCT/advance-tax ordering. Stock availability, batch/expiry, valuation, GL, and ledger projections remain open.
- The detached local supervisors and exact localhost Playwright URL resolve the earlier dev-runtime teardown issue; the 24-hour soak is still open.
- Authored web source passes the mojibake gate and no runtime text-repair observer remains; the stale source-layer wording above is retained only as historical audit context.

1. All 763 legacy tables mapped → migrated → reconciled (counts + business metrics).
2. All 325+ menu commands (incl. contextual) present, gated by rights, wired to real behavior.
3. All 151 report leaves with their format lists, retrieval dialogs, print preview, letterhead and export.
4. Every transaction surface computing the same numbers (tax, discount, stock, GL) to the paisa.
5. Pixel-level raster agreement on every screen at 1936×1048 (existing parity tooling).
6. Hardware verbs working on real devices.

The companion document `PARITY_FIX_PLAN_A-Z.md` sequences this into 26 phases (A–Z) with acceptance gates.

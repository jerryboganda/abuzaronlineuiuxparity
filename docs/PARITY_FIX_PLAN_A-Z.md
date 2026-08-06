# ABUZAR Parity Fix Plan — A to Z

Date: 2026-08-06 · Companion to `GAP_ANALYSIS_2026-08-06.md` (gap IDs G1–G15 referenced throughout).
Goal: **100% visual and functional parity** between legacy WASEELA ABUZAR V3 and AbuzarNext (SvelteKit + Go + PostgreSQL).

Ground rules for every phase:
- **Evidence or it didn't happen.** A phase closes only with a verification artifact: raster diff, API test, reconciliation report, or recorded walkthrough. No status-doc claims without artifacts (fixes G15).
- **Legacy is read-only.** All comparisons run against `FazalDinPP19DataBaseV2` / the running exe without saving transactions; use `LegacyReferenceSandbox` when a write is unavoidable.
- **Pixel gate = existing parity tooling** (`parity/tools/capture-window.ps1` + raster compare) at 1936×1048.
- Effort figures are focused dev-weeks (dw) for one senior dev; run streams in parallel where dependencies allow.

Dependency spine: A → B/C/D (shell) · E (data) unblocks F/G and realistic testing of everything · H/I/J/K/L (business core) · M→Q (reports) · R (rights) gates cutover · S/T/U/V (modules) · W/X/Y/Z (hardening → go-live).

---

## Phase A — Stabilize the dev runtime (G11) — 0.5 dw
Fix `ops/local/start-local.ps1` to launch web/api/edge as detached processes with restart-on-crash (or a tiny supervisor), persist logs with rotation, and add `ops/local/status-local.ps1` health probe. Add a CI smoke test that SSR-renders `/login`, `/app/legacy`, `/app/sales`, `/app/module/purchase-orders` (regression for the fixed SSR crash).
**Accept:** stack survives shell exit + 24h soak; smoke test green in CI.

## Phase B — Quick-win visual defects (G12, G13, G10 partial) — 1.5 dw
1. ~~CSS mojibake~~ **done 2026-08-06** (13 escapes fixed, browser-verified).
2. ~~CashSale menu routing~~ **done 2026-08-06**.
3. **De-mojibake the source tree**: repair double-encoded UTF-8 literals (`â†`, `â–£`, `Â·`, …) inside `.svelte`/`.ts` sources (e.g. `LegacyWorkflowSurface.svelte`), then delete the `legacy-text.ts` MutationObserver band-aid and its `+layout.svelte` hook; add a CI grep gate against re-introduction.
4. Live title bar: replace frozen "05/08/26 11:16:21" with live session clock; show **username** (ADMIN) not display name, format `WASEELA   ABUZAR V3 01.01.2025 : {USER} : {dd/MM/yy HH:mm:ss}`.
5. Trailing-space/label normalization audit across the whole catalog map (e.g. `Openin&g Purchase          `), plus distinct kinds for `Open Sale Return` leaves.
**Accept:** raster diff of shell chrome ≤ tolerance; unit test for `hrefFor` over all 275 catalog leaves asserting no unintended `/app/module/*` fallbacks.

## Phase C — Complete the menu catalog incl. contextual menus (G4) — 2 dw
Re-run menu enumeration with each document window open (driver already written: `tmp/gap-audit/open-and-capture.ps1`) for every File-menu window type; merge into a versioned catalog `parity/catalog/legacy-menu-tree-contextual.json` keyed by active-window class. Extend the shell so opening a surface swaps in its contextual menus (Item menu, File verbs, Window refresh variants) with correct accelerators (Ctrl+Q/I/D/Z/H/M/B, Alt+F8…). Each contextual command gets a typed client action (may open "not yet implemented" panel that links to its phase until wired).
**Accept:** menu JSON diff legacy-vs-new = 0 missing entries for every window type; keyboard accelerators fire.

## Phase D — Shell & MDI chrome parity (G10) — 3 dw
True MDI: multiple concurrent document windows (tabbed-MDI acceptable if pixel test passes), Window menu with Cascade/Tile/Layer/Arrange Icons + numbered open-window list + Refresh variants; global toolbar (New/Spray/Save/Erase, First/Prev/Next/Last, Print, Exit) enabled contextually; bottom status bar with per-field hints ("Specify Item name"). Wire toolbar/status to the focused window.
**Accept:** raster diffs for shell with 0, 1, n windows; Window-menu behaviors demonstrably equal.

## Phase E — Data migration executed & reconciled (G1) — 6 dw ⚠ critical path
1. Run `migration/cmd/inspect` → full source schema manifest (763 tables).
2. Author mapping waves in `migration/maps/`: (a) enterprise/config, (b) masters (Item, Customer, Supplier, Manufacturer, Godown, groups/categories, PricePolicy, ItemSuppliers), (c) documents (Purdetail, Saledetail, PurOrderDetail…), (d) ledgers (SaleLedger, Purledger, VirtualGl), (e) stock (StockReport or its recomputation), (f) security (Users, Groups, GroupRights, GroupAllowed*).
3. Extend the target schema as mappings demand (new tables for ledgers/rights/policies — see H–L, R).
4. Import with legacy-ID mappings; then `migration/cmd/reconcile` with business metrics (sales totals, stock qty, GL balance, invoice max per series) not just counts.
**Accept:** reconciliation report all-green (counts + ≥12 business metrics within tolerance 0.01); spot-check 20 random items/customers field-by-field.

## Phase F — Master data engine (G6) — 4 dw
Shared legacy list chrome component (Sort Criteria + Asc/Desc, Filter Criteria column/op/value + Filter/Retrieve, Find-as-you-type with helper text, List↔Detail tabs). Explicit forms for every master reachable from Basic Data/Manage (~24 kinds), starting with Item (all 21 fields + **Suppliers sub-grid** Priority/Rate/Disc%/Qty/Bonus/Days), Customer, Supplier, Manufacturer, Godown, ItemGroup, CustomerGroup(+Category/Detail), areas, price policies. Remove every hardcoded fallback (32-item catalog in sales page, item-master defaults) — data comes from the migrated DB.
**Accept:** per-master raster diff vs legacy with migrated data; CRUD round-trip API tests; zero hardcoded demo rows anywhere in `apps/web/src`.

## Phase G — Pricing & discount policies (G2 partial) — 3 dw
Implement PricePolicy tiers (SalePrice #1–10 with per-customer/group assignment via GroupAllowedPrice), ItemSuppliers scheme logic (bonus qty, days, disc%), flat disc (−), Misc (+ default 1.00), item disc %, and the SalePrice# selector actually repricing grid lines.
**Accept:** golden-file tests: replay 50 historical Saledetail invoices through the pricing engine and match legacy line totals to the paisa.

## Phase H — Sales workflows complete (G2, G7) — 5 dw
Cash/Credit Sale, Cash/Credit Sale Return, **Open** returns (own kinds), Quotation, Refused Sales: add missing fields (User, Godown, Stock display, Misc, flat disc), live item lookup by name/alias/barcode against migrated data, stock-availability check, batch/expiry selection + enforcement (block expired), draft→Post lifecycle (Save / Post / Save-And-Post Ctrl+Q), invoice sequences per document family, reprint, contextual File verbs (Populate Items, Item Sale History, Sale Slip, attach documents), GL + ledger postings on post (with L/K).
**Accept:** for each kind: raster diff + functional script (create→post→print) and DB assertions on stock, ledger, GL rows matching legacy semantics; replay tests vs historical invoices.

## Phase I — Purchase workflows complete (G2, G7) — 5 dw
Pack/Loose/Opening Purchase, Purchase Return, Purchase Orders with PO→invoice fetch ("Fetch Purchase Invoice From Other Sources"), Auto Batch Generation Ctrl+B, Apply Item GST%, Item Purchase History Ctrl+H, Purchase Slip Ctrl+M, Print Purchase Labels Alt+F8, supplier scheme application, GRN stock-in with batch/expiry, GL/ledger postings.
**Accept:** same gate style as H; batch auto-generation produces legacy-format batch numbers.

## Phase J — Inventory & stock engine (G2) — 4 dw
Real stock ledger per item/godown/batch (replacing bare `inventory_movements`): openings, ins/outs from all document types, adjustments, transfers between godowns, recomputation job that reproduces legacy `StockReport` numbers, valuation (FIFO/avg per legacy behavior).
**Accept:** stock reconciliation metric per godown matches legacy snapshot; adjustment/transfer screens pass functional tests.

## Phase K — Financial core: GL & party ledgers (G2) — 5 dw
Chart/headers (GroupSummaryAccount, GroupCashAccount…), journal table equivalent to `VirtualGl`, automatic postings from every document family, customer/supplier ledgers with running balances, receivables/payables views, credit-limit checks on credit sale, vouchers (GroupVoucherCategory).
**Accept:** migrated VirtualGl balance == recomputed balance; ledger screens match legacy statements for 10 sampled parties.

## Phase L — Tax engine (G2) — 3 dw
GST% per item (incl. "Apply Item GST%" bulk verb), PCT codes, advance income tax on parties, tax lines in documents and GL, tax columns in reports (SalesTax Value etc.), rate tables migrated from legacy config.
**Accept:** replayed invoices match legacy tax to the paisa; tax registers report equals legacy for a sampled month.

## Phase M — Reports engine core (G3) — 5 dw ⚠ gates N–Q
Build once, reuse 151×: (1) **Select Format** dialog — per-report named format list from DB; (2) **Specify Retrieval Arguements** dialog — reproduce legacy layout incl. Selectable/Selected Areas, All checkbox, datetime ranges, Cash/Credit flags (keep the legacy typo? decision: match visually, so yes); (3) **print-preview window** — paginated, letterhead block from DB (Fazal Din's Pharma Plus…), ruler, ~20-button toolbar (zoom/page-nav/print/export); (4) print + PDF + Excel export; (5) report definition format (SQL/projection + column meta + format variants).
**Accept:** Daily Sale Detail runs end-to-end pixel-diffed against `tmp/gap-audit/legacy-report-output.png` with migrated data.

## Phase N — Report leaves wave 1: Daily & sales ops (~40 leaves) — 4 dw
All Daily Reports (Sale detail/summary, cash book, day-end) + core sales analyses. Each leaf: definition + formats + args + golden-number test vs legacy output on migrated data.
**Accept:** per-leaf checklist with output-diff artifact.

## Phase O — Report leaves wave 2: Purchases & suppliers (~35 leaves) — 3 dw
Purchase registers, supplier ledgers/analysis, PO status, GST/purchase tax registers. Same gates.

## Phase P — Report leaves wave 3: Stock & inventory (~40 leaves) — 4 dw
Stock reports (the heaviest data — validate on 3.2M-row scale), expiry lists, godown-wise, batch-wise, valuation. Include performance budget: p95 < 5 s on full data.

## Phase Q — Report leaves wave 4: Financial & remaining (~36 leaves) — 4 dw
GL, trial balance, party statements, receivables aging, profit reports, misc/admin reports. After Q: **all 151 leaves** have real projections; delete the generic fallback.

## Phase R — Security & rights (G5) — 3 dw
Schema + migration for Groups/GroupRights (726 rows) and GroupAllowed* tables; enforcement middleware in Go API per command/right code; menu items disabled/hidden per group exactly like legacy; startup rights (GroupAllowedStartupRight); godown/price/report visibility filters; Manage → Users/Groups screens with rights matrix editor.
**Accept:** login as each of the 4 groups → menu enable/disable snapshot equals legacy; API returns 403 for revoked rights (integration tests).

## Phase S — Maintenance module real logic (G8) — 3 dw
Replace preference-stub handlers: backup/restore (pg_dump/restore + schedule), imports/exports (legacy GroupWiseImpExpTemplate, GroupPurExpTemplate), stock adjustment/opening tools, DataBase Integrity checks (and decide the legacy R0002-on-close defect: reproduce behavior or document divergence per G14), Preferences screen wiring all 200+ prefs to actual behavior (V validates).
**Accept:** backup→restore round-trip on dev; each Maintenance leaf demonstrably executes.

## Phase T — Manage module & session monitor (G8) — 2 dw
Session Monitor (live DB/app sessions like cmd 13869), user administration (create/disable/reset with legacy password rules), group management UI (from R), SMS/Email template management wired to U adapters.
**Accept:** session list shows concurrent logins; user lifecycle tests.

## Phase U — Hardware integrations (G9) — 4 dw + device time
Edge adapters implemented: thermal printer (ESC/POS sale slip + purchase labels Alt+F8 formats byte-compared to legacy prints), barcode scanner (HID wedge → item lookup), cash drawer kick on cash sale post, biometric reader (if used in production), SMS gateway, SMTP email. Graceful degradation when absent.
**Accept:** on real hardware in the pharmacy: printed slip/label physically matches legacy output; scanner→line-add works at POS speed.

## Phase V — Preferences full parity (G14 partial) — 2 dw
Inventory all legacy preferences (incl. those behind the msdb SysJobs probe — reimplement scheduling on pg/cron instead of SQL Agent; document divergence), ensure every preference toggles real behavior, migrate stored values.
**Accept:** preference matrix doc: each pref → behavior → test/artifact.

## Phase W — Performance & scale hardening — 2 dw
Load the full migrated volume (3.2M StockReport, 1M GL); index/query tuning to meet budgets: POS line-add < 150 ms, document post < 1 s, heavy reports p95 < 5 s, app cold start < 3 s. Soak test 8h with simulated POS traffic.
**Accept:** perf report with budgets green on full data.

## Phase X — Pixel-parity acceptance pass (all UI) — 3 dw
Systematic raster sweep: every window/dialog/tab in the catalog captured on both apps at 1936×1048 with migrated data, diffed with the existing parity tooling; fix residual deltas (fonts, spacing, colors, focus states, disabled states).
**Accept:** parity dashboard: 100% screens ≤ agreed pixel tolerance; exceptions individually signed off (e.g. G14 divergences).

## Phase Y — Functional acceptance & business reconciliation — 2 dw
End-to-end UAT script with pharmacy staff covering a full trading day on both systems in parallel (sales, returns, purchases, day-end reports); final `reconcile` run comparing the day's numbers; defect burn-down to zero S0/S1.
**Accept:** signed UAT; parallel-day totals equal.

## Phase Z — Cutover & go-live — 1 dw + window
Freeze legacy writes → final incremental migration + reconciliation → switch POS terminals to AbuzarNext → legacy kept read-only for lookback. Rollback plan: repoint terminals to legacy exe (kept intact) if a blocker appears in the first 48h; document runbook in `docs/RUNBOOK_CUTOVER.md`.
**Accept:** first live day closed with matching day-end report; rollback rehearsed beforehand.

---

## Effort & sequencing summary

| Stream | Phases | Effort |
|--------|--------|--------|
| Foundation | A–D | ~6.5 dw |
| Data | E | 6 dw (critical path) |
| Business core | F–L | ~29 dw |
| Reports | M–Q | ~20 dw |
| Modules & hardware | R–V | ~14 dw |
| Hardening & launch | W–Z | ~8 dw |
| **Total** | | **~83 dev-weeks** single-threaded; ≈ **5–6 calendar months with 3 parallel devs** (data/backend, frontend, reports) |

Highest-leverage order if resources are scarce: **E (migrate data) → H+G (sell correctly) → M+N (daily reports) → I → R** — that alone makes the new system usable for a trading day; the rest completes 100% parity.

## Immediate next actions (this week)
1. Phase A script fix + CI smoke (0.5 dw).
2. Phase B item 3–4 (live titlebar, label audit) (0.5 dw).
3. Start Phase E: run inspector, author masters mapping wave, first import + reconciliation on `AbuzarLegacyReference` sandbox.
4. Capture contextual menus for the remaining window types (Phase C data collection — driver already exists).

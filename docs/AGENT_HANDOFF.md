# AGENT HANDOFF — Abuzar 100% Parity Program

> **Audience:** an autonomous coding agent (or engineer) taking over this project.
> **Mission:** bring `D:\ABUZAR\AbuzarNext` (SvelteKit + Go + PostgreSQL) to **100% visual AND functional parity** with the legacy WASEELA ABUZAR V3 pharmacy software (PowerBuilder + SQL Server). Nothing less. Every menu, every field, every report, every computed number, every pixel at 1936×1048.
> **Status at handoff (2026-08-06):** all gaps verified and documented; plan written; 2 quick-win fixes landed; **implementation of phases A–Z is the remaining work.**

Read these three documents FIRST, in this order:
1. `docs/GAP_ANALYSIS_2026-08-06.md` — the 15 verified gap areas (G1–G15) with evidence.
2. `docs/PARITY_FIX_PLAN_A-Z.md` — the 26-phase execution plan with acceptance gates. **This is your work queue.**
3. This file — environment, credentials, tooling recipes, traps, and rules of engagement.

---

## 1. Prime rules (non-negotiable)

1. **Legacy is the oracle and is READ-ONLY.** Never save/post a transaction in the legacy app against `FazalDinPP19DataBaseV2` (the live/canonical DB). Use the sandbox clone `AbuzarLegacyReference` + `D:\ABUZAR\LegacyReferenceSandbox` when you must exercise a write path.
2. **Evidence or it didn't happen.** A gap/phase is "done" only with a verification artifact: raster diff report, passing test, reconciliation JSON, or screenshot pair. Never write "fixed" in a status doc without linking the artifact (prior team overclaimed; see PARITY_STATUS.md addendum).
3. **Match legacy numbers to the paisa.** Business acceptance = replaying historical documents through the new engine reproduces legacy totals (price, discount, tax, stock, GL) exactly.
4. **Pixel gate at 1936×1048** using `parity/tools/capture-window.ps1` + `parity/tools/compare-png.ps1`.
5. **Don't clone legacy *defects* silently.** SQL2022-era legacy bugs (G14: msdb SysJobs error, Groups "DataBase Error Row 0", R0002 close crash) get a documented decision, not blind reproduction.
6. Work in small verified increments; run the checks in §6 after every change batch.

## 2. Environment map

| Thing | Location / value |
|---|---|
| Workspace root | `D:\ABUZAR` (NOT a git repo — no branches, no PRs; edit in place, keep backups) |
| New app monorepo | `D:\ABUZAR\AbuzarNext` (pnpm workspaces + Go) |
| Web app (SvelteKit/Svelte 5) | `AbuzarNext\apps\web` |
| Go API | `AbuzarNext\services\api` (core: `internal\httpapi\business.go`, `reports.go`) |
| Edge/hardware service | `AbuzarNext\services\edge` |
| Migration workbench (Go) | `AbuzarNext\migration\cmd\{inspect,import,reconcile}` + `migration\maps\` |
| Parity tooling & catalogs | `AbuzarNext\parity\{tools,catalog}` |
| Audit evidence (screens, menu JSONs, drivers) | `AbuzarNext\tmp\gap-audit\` |
| Legacy install (canonical, DO NOT MODIFY) | `D:\ABUZAR\V2_AbuzarSoftware\Application\abuzar.exe` |
| Legacy sandbox copy | `D:\ABUZAR\LegacyReferenceSandbox` |
| Legacy DB (canonical, read-only) | SQL Server `127.0.0.1`, DB `FazalDinPP19DataBaseV2` (763 tables) |
| Legacy DB (sandbox) | same server, DB `AbuzarLegacyReference` |
| New DB | PostgreSQL `127.0.0.1:5432`, DB `abuzar_next` (22 tables, data dir `AbuzarNext\tmp\pg-test-20260805-complete`) |

**Local dev credentials (LOCAL MACHINE ONLY — never publish or reuse elsewhere):**
- SQL Server: SQL auth `sa` / `a8u5qfwr` (integrated auth FAILS — "untrusted domain"). E.g. `sqlcmd -S 127.0.0.1 -U sa -P a8u5qfwr -d FazalDinPP19DataBaseV2`. Also recorded in `D:\ABUZAR\update_ini_and_sa.py` and legacy `Preferences.ini`.
- Legacy app login: `ADMIN` / `pakistan9080` (source of truth: `Users` table).
- New app login: `admin` / `pakistan9080` (tenant auto = `demo` in DEV). If login breaks, re-reset: `UPDATE users SET password_hash = crypt('pakistan9080', gen_salt('bf',6)) WHERE display_name='Local Administrator';` (pgcrypto, via `psql -h 127.0.0.1 -U postgres -d abuzar_next`, no password).

## 3. Launching both systems

### New stack
```powershell
powershell -ExecutionPolicy Bypass -File D:\ABUZAR\AbuzarNext\ops\local\start-local.ps1
# → Postgres :5432, Go API :8080 (/healthz), edge :8091, web http://127.0.0.1:5173
```
⚠ **TRAP (Phase A fixes this):** `start-local.ps1` launches the Vite dev server ATTACHED to the launching shell — when that shell exits, the web server dies (it died 3× during the audit). Until Phase A lands, start the web server yourself as a **detached** process:
```powershell
cd D:\ABUZAR\AbuzarNext\apps\web
cmd /c "pnpm dev -- --host 127.0.0.1 --port 5173 >> ..\..\tmp\web-localhost.stdout.log 2>&1"   # run detached
```

### Legacy app
- Working directory MUST be the Application folder or DB connect fails (SQLSTATE 08001):
```powershell
Start-Process -FilePath 'D:\ABUZAR\V2_AbuzarSoftware\Application\abuzar.exe' -WorkingDirectory 'D:\ABUZAR\V2_AbuzarSoftware\Application'
```
- Login ADMIN / pakistan9080. It may already be running — check `Get-Process abuzar` first and reuse the instance.
- Which DB is it connected to? `SELECT host_process_id, db_name(database_id) ... FROM sys.dm_exec_sessions` and match the PID.
- ⚠ Known legacy crashes (don't panic, documented G14): R0002 on closing Pack Purchase / DB-Integrity windows; "DataBase Error Row No: 0" dialog on Manage→Groups (dismiss with WM_CLOSE, the window still opens); msdb SysJobs error on Preferences.

## 4. Tooling recipes (already written, in `tmp\gap-audit\`)

- **Enumerate legacy menus** (per-window contextual menus!): `enum-menu.ps1` — Win32 GetMenu/GetSubMenu walker → JSON {path, position, commandId, grayed, hasSubmenu}.
- **Open a legacy window + capture + enumerate**: `open-and-capture.ps1` — PostMessage `WM_COMMAND (0x0111)` with a commandId, captures main window + child dialogs, dumps menu JSON. Useful command IDs: CashSale 12710 · CreditSale 12711 · CashSaleReturn 12712 · Quotation 12718 · Purchases 12684 · PurchaseReturn 12685 · Customer 13539 · Supplier 13561 · Item 13564 · Users 13842 · Groups 13841 · Preferences 13737 · SaleDetailReport 12781 · SessionMonitor 13869. (Full catalog: `live-menu-tree-pack-purchase.json`.)
- **Capture a window raster**: `parity\tools\capture-window.ps1 -ProcessId <pid> -OutputPath x.png` (PrintWindow-based; window must be restored/maximized first).
- **Compare rasters**: `parity\tools\compare-png.ps1` (reports `differentPixels`).
- **PowerShell traps:** Add-Type classes do NOT persist across tool invocations (re-Add-Type with unique class names each call). PowerBuilder dialogs ignore `BM_CLICK` (custom window classes) — use SetForegroundWindow + SetCursorPos + mouse_event coordinate clicks. `WM_CLOSE (0x0010)` reliably dismisses dialogs. No `&&`/`||` in this PowerShell — use `;` and `if ($?)`.
- **Browser automation:** Playwright MCP against `http://127.0.0.1:5173`; login form ids `#username` / `#password`; size viewport 1936×1048 for pixel comparisons.

## 5. The work queue — phases A–Z (details in PARITY_FIX_PLAN_A-Z.md)

Execute in dependency order; parallelize streams where independent:

- **A** Stabilize dev runtime (detach servers in `start-local.ps1`, crash-restart, CI smoke over `/login`, `/app/legacy`, `/app/sales`, `/app/module/purchase-orders`). ← *do this first, it unblocks everything*
- **B** Quick visual wins — remaining items: **de-mojibake source files** (`LegacyWorkflowSurface.svelte` etc. contain literal `â†`, `â–£`, `Â·`; then DELETE the `lib/legacy-text.ts` MutationObserver band-aid + its `+layout.svelte` hook; add CI grep gate `[\u00c2\u00e2]` in source), live titlebar clock + username (not display name), label normalization audit + distinct `Open Sale Return` kinds.
- **C** Contextual menu catalog: re-enumerate legacy menus with EACH document window open (driver exists), produce `parity/catalog/legacy-menu-tree-contextual.json`, shell swaps menus per active window, all accelerators (Ctrl+Q/I/D/Z/H/M/B, Alt+F8…).
- **D** MDI shell parity: multi-document windows, Window menu (Cascade/Tile/Layer/numbered list/Refresh), global toolbar, status-bar hints.
- **E** ⚠ **CRITICAL PATH — run the data migration**: `migration/cmd/inspect` → author mapping waves in `migration/maps/` (masters → documents → ledgers → stock → security) → `import` with legacy-ID mappings → `reconcile` with business metrics. Target: counts green + ≥12 business metrics within 0.01. The new DB currently has **0 migrated rows** vs legacy 30,050 items / 620K sale lines / 3.2M stock rows / 1.04M GL rows.
- **F** Master-data engine: shared legacy list chrome (Sort/Filter/Find/List↔Detail), ~24 explicit master forms, Item master with **Suppliers sub-grid** (Priority/Rate/Disc%/Qty/Bonus/Days), remove ALL hardcoded fallbacks (e.g. 32-item catalog in `apps/web/src/routes/app/sales/+page.svelte` L29-62).
- **G** Pricing/discount engine: PricePolicy tiers SalePrice#1–10, GroupAllowedPrice, ItemSuppliers schemes, flat disc(−), Misc(+ default 1.00), item disc%. Golden test: replay 50 historical invoices to the paisa.
- **H** Sales workflows complete (cash/credit/returns/open-returns/quotation/refused): missing fields (User, Godown, Stock, Misc), live item lookup (name/alias/barcode), stock check, batch/expiry enforcement, draft→Post lifecycle (Save/Post/Save-And-Post Ctrl+Q), sequences, GL+ledger postings, contextual verbs.
- **I** Purchase workflows complete (pack/loose/opening/return/orders): PO→invoice fetch, Auto Batch Generation Ctrl+B, Apply Item GST%, purchase history Ctrl+H, slips Ctrl+M, labels Alt+F8, GRN with batch/expiry.
- **J** Inventory/stock engine: per item/godown/batch ledger, adjustments, transfers, valuation, reproduces legacy `StockReport`.
- **K** Financial core: GL journal (≙ `VirtualGl`), auto-postings from all documents, customer/supplier ledgers, receivables/payables, credit limits, vouchers.
- **L** Tax engine: GST%, PCT, advance income tax, bulk Apply-GST verb, tax registers.
- **M** ⚠ Reports ENGINE (gates N–Q): Select Format dialog (per-report named formats — legacy has 10 for Daily Sale Detail alone), "Specify Retrieval Arguements" dialog (match legacy incl. its typo), paginated print-preview with DB-driven letterhead ("Fazal Din's Pharma Plus…"), ruler, ~20-button toolbar, print + PDF + Excel export, report-definition format.
- **N–Q** All **151 report leaves** in 4 waves (daily/sales ~40 → purchases ~35 → stock ~40 on 3.2M-row scale → financial ~36). Then delete the generic fallback in `services/api/internal/httpapi/reports.go`.
- **R** Security/rights: migrate Groups (4) + `GroupRights` (726 rows) + `GroupAllowed*` tables; Go middleware enforcement per right-code; menu gating per group identical to legacy; rights-matrix editor UI.
- **S** Maintenance real logic: backup/restore, imports/exports (templates), stock adjustment/opening tools, integrity checks, wire all 200+ preferences.
- **T** Manage module: Session Monitor (live sessions), user admin, group UI, SMS/Email templates.
- **U** Hardware via edge service (currently ALL "No adapter configured"): ESC/POS thermal slip + label printing (byte-match legacy output), barcode wedge → item lookup, cash-drawer kick, biometric, SMS gateway, SMTP.
- **V** Preferences full parity (reimplement SQL-Agent-era scheduling on pg-native; document divergences).
- **W** Performance on full data: POS line-add <150 ms, post <1 s, heavy reports p95 <5 s; 8h soak.
- **X** Pixel-parity sweep: every window/dialog/tab rastered on both apps with migrated data; 100% ≤ tolerance or signed exception.
- **Y** Functional acceptance: full parallel trading day, final reconciliation, zero S0/S1 defects.
- **Z** Cutover: freeze → final incremental migration → switch POS → 48h rollback window (legacy kept intact) → `docs/RUNBOOK_CUTOVER.md`.

**If resources are scarce, the highest-leverage order is: E → H+G → M+N → I → R** (that makes the system usable for a real trading day; the rest completes 100%).

## 6. Verification commands (run after every change batch)

```powershell
cd D:\ABUZAR\AbuzarNext
pnpm --filter @abuzar/web check        # svelte-check — currently 0 errors
pnpm --filter @abuzar/web test         # Playwright smoke — currently 13/13 pass
pnpm --filter @abuzar/web build        # production build
go test ./services/api/... ./services/edge/... ./migration/...
```
⚠ Known flaky test: `smoke.spec.ts:184` "Groups editor loads permissions…" can fail under machine load (`waitForTimeout(50)` race before Save click — `permissions: []` reaches the POST). If it fails, re-run in isolation before suspecting your change; better: fix the race (wait for checkbox state instead of 50 ms).

## 7. State of the codebase — what's real vs façade (honest map)

**Real & working:** auth/session/tenancy (RLS-scoped), generic event-sourced document posting with idempotency + offline queue, 4 master forms, 3 real report projections, menu shell from 275-entry catalog, raster-baseline shell (`differentPixels=0` on several captured defaults), migration tooling (unused), Tauri desktop build, local ops scripts.

**Façade (looks done, isn't):** most menu leaves route to generic surfaces; report leaves → generic 6-column grid; Maintenance/Manage forms persist to `tenant_preferences` and execute nothing; document status hardcoded `posted`; discounts read-only 0.00; no tax/stock/batch/GL/ledger/rights logic; hardware adapters return "No adapter configured"; item lookup falls back to a hardcoded 32-item list; title bar shows a frozen captured timestamp; mojibake in source masked by a runtime MutationObserver.

**Fixed on 2026-08-06 (verified):** 13 CSS `content:"\\XXXX"` double-escapes in `apps/web/src/lib/styles.css`; `Sales > CashSale` stub-routing bug in `apps/web/src/lib/legacy-menu.ts` (legacy caption `Cash&Sale` cleans to `CashSale` — map now has both keys).

## 8. Key reference data (measured, trust these)

- Legacy DB: 763 tables. StockReport 3,231,846 · VirtualGl 1,040,590 · Saledetail 620,802 · SaleLedger 291,231 · Purdetail 113,748 · PurOrderDetail 108,414 · Item 30,050 · PricePolicy 30,032 · ItemSuppliers 22,406 · Purledger 6,436 · Manufacturer 838 · Supplier 235 · Users 9 · Groups 4 · GroupRights 726.
- Legacy menus: 275 items/9 menus base; **325+/10 with a document window open** (contextual Item menu + ~35 File verbs + Window Refresh variants). Reports menu: 186 entries, 151 leaf reports.
- Letterhead (DB-driven): "Fazal Din's Pharma Plus / NRY Pacific / Franchise Fazal Din's", Phone 055 3252501.
- Cash Sale legacy fields: Inv No, Date, User, AliasName, Customer (CASH SALES CUSTOMER), Godown (GODOWN1), Ref, Remarks, SalePrice# selector, 10-col grid, totals + Stock + Flat Disc(−) + Misc(+ 1.00) + Discount% + Sales.
- Groups: ADMINISTRATOR, REMOTE, SALES OFFICER, SHIFT INCHARGE.

## 9. Definition of DONE for the whole program

- [ ] All 763 legacy tables mapped/migrated/reconciled (counts + business metrics green).
- [ ] All 325+ menu commands present, contextual per window, rights-gated, wired to real behavior.
- [ ] All 151 report leaves: formats + retrieval dialog + print-preview/letterhead + print/PDF/Excel.
- [ ] Replayed historical documents match legacy totals (price/discount/tax/stock/GL) exactly.
- [ ] Pixel sweep: 100% screens ≤ tolerance at 1936×1048 (or signed exceptions for G14 divergences).
- [ ] Hardware verbs proven on real devices (slip/label byte-match).
- [ ] Parallel-day UAT signed; cutover runbook rehearsed incl. rollback.
- [ ] Zero unverified "done" claims in status docs — every claim links an artifact.

## 10. Suggested kickoff prompt for the executing agent

> Read `D:\ABUZAR\AbuzarNext\docs\AGENT_HANDOFF.md` fully, then `docs/GAP_ANALYSIS_2026-08-06.md` and `docs/PARITY_FIX_PLAN_A-Z.md`. Execute the parity program phase by phase starting at Phase A, in dependency order (A→B→C→D, then E as critical path, then F–L, M–Q, R–V, W–Z). For each phase: implement, run the §6 verification commands, produce the phase's acceptance artifact (raster diff / test run / reconciliation JSON), and record it in `docs/PARITY_STATUS.md` with a link to the artifact before moving on. Legacy app and `FazalDinPP19DataBaseV2` are strictly read-only; use `AbuzarLegacyReference` for any write experiments. Do not mark anything done without its artifact. Work autonomously; when a legacy behavior is ambiguous, launch the legacy app and observe it (recipes in handoff §3–4) rather than guessing.

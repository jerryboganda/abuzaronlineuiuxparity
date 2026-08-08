# Remaining Work for 100% Functional & Visual Parity — Verified Audit

Date: 2026-08-09. Companion to `PARITY_FIX_PLAN_A-Z.md`, `GAP_ANALYSIS_2026-08-06.md`, `PARITY_STATUS.md`, `HANDOFF_2026-08-08.md`.

**Method:** 10 independent read-only audit agents, one per phase-cluster (A–Z), each instructed to verify existing-doc claims directly against current code/tests/data rather than trust the docs — because several prior-session claims turned out to be stale or overstated (see below). No files were edited or migrated as part of this audit; every finding below is grounded in a file path, test name, or artifact cited by the auditing agent.

---

## ⚠ Corrections to prior claims (read this first)

A few things previously reported as done in this session's evidence docs — including two items marked `completed` on the running task list — do **not** hold up under independent verification:

1. **Pricing golden replay (task #12, "50-invoice replay to the paisa") is not what it claims to be.** `docs/PHASE_G_PRICING_GOLDEN_REPLAY_2026-08-08.md` itself admits: "the premise needs correcting before the numeric result is meaningful." The 50/50-invoice match is a **migration copy-fidelity check** (line totals were `COPY`'d straight from `dbo.Saledetail.Rate`, never run through `pricing.Calculate()`). Only 7 of 104 lines actually went through the pricing engine, and 1 of those had an unexplained Rs 45 discrepancy. The real pricing engine still doesn't implement `PricePolicy` tiers or `GroupAllowedPrice` at all for sales — see Phase G below.
2. **Orphaned test-tenant cleanup is not complete.** One tenant (`document-test-maintenance-20260808224042-859427600`) still exists in the dev DB as of this audit, contradicting the "391/391 deleted, 0 remaining" verification recorded earlier. The cleanup script's own success record was never actually logged anywhere in `docs/`.
3. **Security rights menu-gating parity (task #14) is verified for 1 of 4 legacy groups with real data, not all 4.** Only the REMOTE group has a Playwright test against actual migrated `group_rights`; the other 3 groups (ADMINISTRATOR, SALES OFFICER, SHIFT INCHARGE) are covered only by hand-typed synthetic fixtures. Separately, ADMINISTRATOR bypasses the rights table entirely via a role-name check (`hasTenantAdminRole`), so it isn't table-driven at all — a real architectural gap, not just a test-coverage gap.
4. **The Phase E "spot-check 20 random items/customers field-by-field" accept criterion was never performed** — zero evidence of it exists anywhere in the repo.
5. The tax_amount=0 anomaly (background task `task_b63bf468`) and the $970.95/$91,710.40 value-sum gap are **two different issues** that earlier framing conflated. Only the value-sum gap was actually root-caused and closed via `migration_exceptions`; the tax_amount=0 anomaly is still unconfirmed as bug-vs-data-drift.

None of this reverses real work that did happen (moving-average stock valuation, credit-limit enforcement, the security rights backfill mechanism, real backup/restore — all independently re-confirmed real below). It means the completion bar for a few specific claims needs to move back to PARTIAL.

---

## Phase-by-phase verified status

| Phase | Scope | Verdict |
|---|---|---|
| A | Dev runtime stability | PARTIAL — scripts/CI test real, 24h soak never executed |
| B | Quick-win visual defects | DONE-VERIFIED |
| C | Menu catalog incl. contextual | PARTIAL — only 5 of dozens of window types captured |
| D | Shell & MDI chrome | PARTIAL — tabbed nav real, toolbar/status-bar/geometry missing |
| E | Data migration & reconciliation | PARTIAL — 16/16 metrics matched, but hygiene gaps (see corrections) |
| F | Master data engine | PARTIAL — ~22/24 kinds have forms, 3 named kinds unreachable, no shared list-chrome |
| G | Pricing & discount policies | PARTIAL — real engine exists but tiers/GroupAllowedPrice unimplemented, replay claim overstated |
| H | Sales workflows | PARTIAL — real screens/lifecycle, no raster gate, pack/loose fix only spot-verified |
| I | Purchase workflows | PARTIAL — batch numbers not legacy-format, PO-fetch open |
| J | Inventory & stock engine | PARTIAL — moving-average done; **godown transfers: zero implementation** |
| K | Financial core (GL/ledgers) | PARTIAL — credit-limit done; vouchers schema-only, no VirtualGl reconciliation |
| L | Tax engine | PARTIAL — engine real, legacy rates unmigrated, no paisa-replay, no tax register |
| M | Reports engine core | PARTIAL — dialogs/preview/export real, Daily Sale Detail pixel-diff never run |
| N–Q | 151 report leaves | **70/151 real projections, 81/151 generic event-ledger, 0/151 golden-verified** |
| R | Security & rights | PARTIAL — backfill mechanism real (433/486 codes), admin bypass + 3/4 groups untested with real data |
| S | Maintenance module | PARTIAL — backup/restore real; import/export stub; **preferences 1/437 wired**; R0002 undecided |
| T | Manage module & sessions | PARTIAL — session monitor & rights-matrix UI real; no legacy password rules; no SMS/email templates |
| U | Hardware integrations | DONE-VERIFIED (software side; physical validation explicitly out of scope) |
| V | Preferences full parity | PARTIAL — **1 of 441 preferences wired to real behavior**; no legacy-value migration; `CheckCrLimitInCrSales` not even inventoried |
| W | Performance & scale | OUT OF SCOPE (documented 2026-08-08) — cited numbers verified accurate |
| X | Pixel-parity sweep | **PARTIAL, essentially not started** — catalog doesn't exist, baselines dir empty, only 3-4 screens diffed |
| Y | Functional acceptance/UAT | OUT OF SCOPE (documented 2026-08-08) — 16/16 reconciliation verified accurate |
| Z | Cutover & go-live | OUT OF SCOPE (documented 2026-08-08) — runbook doc exists, zero execution |

---

## Detailed findings and concrete remaining items, by phase

### Phase A — Dev runtime stability
`ops/local/start-local.ps1` + `supervise-local.ps1` + `status-local.ps1` genuinely implement detached processes, restart-on-crash with backoff, log rotation, and a health probe. CI smoke test (`apps/web/tests/smoke.spec.ts:916`) SSR-renders the 4 required routes and is wired into `.github/workflows/web-smoke.yml`.
**Remaining:**
- Actually run and record a 24-hour dev-stack soak (shell-exit survival + restart-on-crash) — currently zero evidence this was ever executed.
- Confirm `web-smoke.yml` has gone green on GitHub Actions at least once.

### Phase B — Quick-win visual defects
DONE. Mojibake fully gone from the source tree (`grep` for double-encoded UTF-8 bytes returns zero files), `legacy-text.ts` band-aid removed, live title-bar clock with username wired (`legacy-title.ts`), label-trim test covers all built menu actions.
**Minor nuance:** "275 catalog leaves" in the plan text = 221 actual leaf entries (54 rows are submenu parents/separators) — not a shortfall, just a wording clarification.

### Phase C — Menu catalog incl. contextual menus
Real, versioned, well-formed catalog + real wiring into the shell (`buildLegacyMenusForContext`).
**Remaining:**
- Only 5 window types captured (`pack-purchase`, `cash-sale`, `item-master`, `report-sale-detail`, `manage-groups`) out of dozens required — every other sale/purchase/return/quotation kind, ~19 remaining master kinds, and 150 of 151 report window types fall back to the base menu.
- Verify keyboard accelerators (Ctrl+Q/I/D/Z/H/M/B, Alt+F8) fire as live `keydown` handlers — currently only present as label text.
- Produce the "0 missing entries" diff artifact the accept line requires.

### Phase D — Shell & MDI chrome
Tabbed-window registry (`legacy-window-registry.ts`) and a real Window menu (Cascade/Tile/Layer/Arrange/numbered list) exist.
**Remaining:**
- No shared global toolbar — every surface hand-rolls a different button set; legacy's "Spray"/"Erase" buttons have zero references anywhere in the codebase.
- No bottom status bar with per-field contextual hints (only generic "Ready"/error text).
- Cascade/tile/layer is a stored attribute with no visual geometry effect.
- Raster diffs exist only for the 0-window base shell; 1-window and n-window states are undemonstrated.

### Phase E — Data migration & reconciliation
16/16 business metrics matched and independently DB-cross-verified (0 open exceptions/ambiguities). Line-unit repair (3-step SQL) verified with 0 identity/ledger violations.
**Remaining (see also "Corrections" above):**
- Finish the orphaned-tenant cleanup — 1 tenant still present; log a completion record.
- Perform the literal "spot-check 20 random items/customers field-by-field."
- Root-cause the 8 dropped `Saledetail`/`Purdetail` rows (`reason_code='line_dropped_by_bulk_import_unreviewed_cause'`) instead of relying on a compensating metric adjustment.
- Investigate the separate `tax_amount=0` anomaly for qty>1 lines (background task `task_b63bf468`) — still unconfirmed bug vs. drift.
- Document a scope decision for the 714/763 unmapped source tables (mostly appear to be hospital/school/CRS_ modules bundled in the same legacy DB, but no written decision excludes them).
- Refresh `docs/GAP_ANALYSIS_2026-08-06.md` G1 section — still reads "never executed."

### Phase F — Master data engine
~22 of ~24 legacy Basic-Data kinds have real field-backed forms; Item is deep (21+ fields, 8 sub-commands).
**Remaining:**
- **Godown, CustomerGroup(+Category/Detail), and Areas** — explicitly named in the plan — are not reachable from any menu route (Godown has backend tables but no menu entry; the other two don't exist at all).
- No shared "legacy list chrome" component — sort/filter/find/list-detail logic is duplicated between master and report pages instead of extracted once.
- No frontend delete action for canonical masters despite backend routes existing.
- Only one aggregate CRUD integration test exists; no per-master-kind round-trip test.
- Zero raster diffs run for any of the ~23 implemented master kinds.

### Phase G — Pricing & discount policies
Real exact-decimal `Calculate()` engine exists with regression tests; SalePrice#1-10 selector UI reprices; ItemSuppliers scheme works for purchases.
**Remaining:**
- Implement `PriceTypeCode`/`Module`-based tier selection and actually wire the already-migrated `price_policy_tiers` (30,052 rows) into `priceDocument`/`Calculate` — currently unused for pricing.
- Implement `GroupAllowedPrice` per-customer/group price assignment — not implemented anywhere (only a disconnected UI settings stub exists).
- Extend ItemSuppliers scheme logic to sale documents (currently purchase-only).
- Produce a real golden-replay test where invoices are actually computed by `pricing.Calculate()`, not copied from source — the current dataset can't even test this (`price_level` hard-injected to 1 tenant-wide, frozen `PricePolicyDetail` rows).

### Phase H — Sales workflows
All named document kinds are real, wired screens (not stubs): draft→Post lifecycle, batch/expiry enforcement, GL/ledger postings, most contextual File verbs.
**Remaining:**
- No raster-diff gate per document kind (create→post→print functional script never run against legacy raster).
- Pack/loose unit fix verified only on one golden invoice (695336) at the per-line level, plus dataset-wide aggregate sums/identity checks — no per-invoice replay test suite across many historical invoices exists.
- In-document reprint action missing (reprint only exists as separate report leaves).
- Exact rounding / non-cash tender void-refund settlement open.

### Phase I — Purchase workflows
Same maturity level as Sales — real screens, batch/expiry, GL postings, supplier scheme.
**Remaining:**
- Auto Batch Generation produces `AUTO-YYYYMMDD-NNN`, explicitly **not** legacy-format — directly fails this phase's stated accept criterion.
- PO→invoice fetch ("Fetch Purchase Invoice From Other Sources") not implemented.
- Print Purchase Labels has no physical/byte-level legacy comparison.
- Same replay-test-suite gap as Phase H (shared data).

### Phase J — Inventory & stock engine
Moving-average valuation is real, tested, and wired into sale/purchase-return costing. Stock adjustment/opening-stock screens are real with a passing Playwright test.
**Remaining:**
- **Godown-to-godown transfer has zero implementation, backend or frontend** — not in `stock.go`, not in the UI, not even in legacy's own captured menu. This is explicitly named in the plan text.
- No recomputation job reconciling against legacy's 3.2M-row `StockReport` by godown/batch/item/valuation — `rebuildStockBalances` only rebuilds from the app's own ledger.
- No "stock reconciliation metric per godown matches legacy snapshot" evidence exists anywhere.

### Phase K — Financial core (GL & party ledgers)
Credit-limit enforcement is real, row-locked, and tested — but is a documented simplification (see Phase V, `CheckCrLimitInCrSales`).
**Remaining:**
- Vouchers (`voucher_categories`/`voucher_entries`) are schema-only — zero posting endpoint exists anywhere in the Go codebase.
- No "migrated VirtualGl balance == recomputed balance" verification has ever been run (1,021,801 imported GL rows are displayed, never reconciled).
- No "10 sampled parties" ledger-statement comparison against real legacy statements exists anywhere.
- Chart-of-accounts naming/mapping vs legacy `GroupSummaryAccount`/`GroupCashAccount` remains an open human-decision item.

### Phase L — Tax engine
Schema and resolution engine are real (GST/PCT/advance-tax, effective-dated, "Apply Item GST%" bulk verb implemented).
**Remaining:**
- Legacy tax rate/config tables never migrated into the canonical tenant.
- No replayed-invoice tax-to-the-paisa verification.
- No tax-register report vs. legacy for a sampled month.

### Phase M — Reports engine core
Select Format dialog, Specify Retrieval Arguments dialog, paginated print-preview with letterhead/ruler/toolbar, and PDF/Excel export all genuinely exist and work.
**Remaining:**
- The phase's own gating accept criterion — Daily Sale Detail pixel-diffed against `legacy-report-output.png` — has **never been run**; zero code references to that file exist anywhere in tests.

### Phases N–Q — 151 report leaves (the single largest remaining gap)
Verified directly from the registry code (`reports.go`):

| Wave | Leaves | Real projection | Event-ledger (generic columns) | Golden-verified vs legacy |
|---|---|---|---|---|
| N — Daily & sales ops | 68 | 7 | 61 | 0 |
| O — Purchases & suppliers | 24 | 4 | 20 | 0 |
| P — Stock & inventory | 27 | 27 | 0 | 0 |
| Q — Financial & remaining | 32 | 32 | 0 | 0 |
| **Total** | **151** | **70** | **81** | **0** |

"Real" means a report-specific column/grouping contract exists — it does **not** mean legacy-verified output; every single evidence doc across N/O/P/Q ends with "exact PowerBuilder calculations... golden replay remain open." "Event-ledger" (81 leaves, almost entirely Sales Reports and Purchase Reports sub-menus) means only generic aggregate columns are returned, not the report's actual legacy shape.
**Remaining:**
- Write golden-number tests against legacy output for all 151 leaves (0/151 today) — the single largest concrete gap in the whole project.
- Promote the 81 event-ledger leaves (N and O waves) to real per-report column/grouping projections.
- Run the Phase P full-volume p95<5s perf budget (only a 25k/10k-row fixture tested so far, not 3.2M/1M).
- Delete the generic-fallback mechanism in `reports.go` once every leaf is proven.

### Phase R — Security & rights
Backfill mechanism (433/486 legacy right-codes → modern permissions), 67 `requirePermission` call sites, client-side menu gating, and a real rights-matrix editor UI all verified genuine.
**Remaining:**
- Decide/document whether ADMINISTRATOR's role-name-based bypass (not table-driven) is acceptable long-term.
- Add real (non-synthetic) menu-snapshot tests for the other 3 groups — only REMOTE has one against real migrated data today.
- Map or retire the remaining 53 unmapped right codes if/when their corresponding modules (Accounting Vouchers, Payroll, E-Prescription) are ever built.
- Broaden 403/revoked-rights test coverage beyond the current sampled cases.

### Phase S — Maintenance module
Backup/restore is real (`pg_dump`/`pg_restore`, live-DB guard, round-trip tests). Stock adjustment tools are real.
**Remaining:**
- Import/export is a hardcoded stub (`not_configured`) — no legacy `GroupWiseImpExpTemplate`/`GroupPurExpTemplate` handling exists at all.
- **Preferences: only 1 of 437 registered fields is wired to real behavior** — everything else is `stored_only` or `not_configured` (this is the single largest Phase S/V gap).
- DB integrity check only counts 8 app tables' rows, not a real physical-DB check.
- R0002-on-close legacy defect — still an undecided human-decision item.

### Phase T — Manage module & session monitor
Session Monitor and the Users/Groups rights-matrix UI are both real and DB-backed.
**Remaining:**
- Password rules are generic length-only (8–256 chars) — no legacy PowerBuilder complexity rules implemented, and unlike R0002 this isn't even flagged as a decision item.
- No dedicated password-reset/disable endpoints (both piggyback on the generic update).
- SMS/Email "template management" doesn't exist — only one-shot ad-hoc test-send actions.

### Phase U — Hardware integrations
DONE-VERIFIED for the software-side scope this engagement committed to (physical validation explicitly out of scope, decided 2026-08-08). All 6 adapter types have real interfaces with graceful degradation, and SMS/SMTP have genuine local-server integration tests, not just mocks.
**One documentation-accuracy issue found:** the plan's prose claims ESC/POS output is "byte-compared to legacy prints" — it is not; the byte-goldens are self-referential (compared against the same Go renderer), not captured legacy bytes. Worth correcting the wording; not a code gap.

### Phase V — Preferences full parity
**1 of 441 registered preferences (`Report → Default Header On Report`) is wired to real backend behavior.** Everything else — all of Sale, Purchase, BasicData, Quotation, Adjustment, POS, Cashier, Dashboard, and most of General/Report/Email/SMS — is `stored_only` (persisted, zero backend effect) or `not_configured` (`Schedule` category, 33 prefs).
**Remaining:**
- Wire the ~440 remaining preferences to real behavior — by far the largest single itemized gap by count in the project.
- Migrate actual per-pharmacy legacy preference *values* into `tenant_preferences` — currently only hardcoded defaults are seeded; the imported legacy config row is never consumed.
- Implement a pg/cron (or equivalent) scheduler for the `Schedule` category — the msdb SysJobs divergence is documented but not reimplemented.
- Add `Preferences.CheckCrLimitInCrSales` to the registry and gate `enforceCreditSaleLimit` on it — currently not even inventoried, so credit-limit enforcement is unconditional.
- Write a standalone preference-matrix doc (pref → behavior → test/artifact) — none exists.

### Phase W — Performance & scale hardening
OUT OF SCOPE (documented 2026-08-08, full-volume soak cluster never provisioned). The specific numbers already cited in the plan (pos-line-add p50 6.9ms/p95 13.9ms, heavy-sales-report ~3963ms/4043ms, heavy-stock-report statement-timeout cancellation) were independently re-verified against the raw artifact and are accurate.
**Remaining (in-scope part):** the `heavy-stock-report` statement-timeout issue is a real open code fix — the Postgres `statement_timeout` shared between the report and posting paths needs to be split (a variant of the same coupling already fixed for the report endpoint's own timeout).

### Phase X — Pixel-parity acceptance pass (NOT scoped out — fully in-scope, essentially not started)
No formal screen/dialog/tab catalog exists yet — realistically 150–300+ distinct screens once the 275 menu commands, 151 report leaves, and dialog/tab sub-states are enumerated. `parity/baselines/` (the intended baseline directory) is completely empty. 70 ad hoc legacy screenshots exist in `parity/captures/legacy/` (dev-reference captures, not a systematic sweep), 40 of which are used only as CSS background-image overlays for building the UI, not as automated diff baselines. Only **4 screens have ever been formally diffed**, of which **3 pass** (small static dialogs) and **1 fails** (main shell window, 88,986 differing pixels, unexplained/unsigned-off). The capture/diff tooling itself (`parity/tools/capture-window.ps1`, `compare-png.ps1`) is complete and functional — it has just never been run at scale.
**Remaining — essentially a from-scratch effort:**
- Build the actual screen catalog (menu commands + report leaves + dialog/tab sub-states).
- Populate `parity/baselines/` with legacy captures at 1936×1048.
- Capture matching AbuzarNext screens and run the existing diff tool across the full catalog.
- Resolve or formally sign off the one known failing diff (main shell, 88,986 px) instead of leaving it unexplained.
- Decide whether the shell background is a static raster reference image or needs to become live-diffable chrome (G10 flags this as a prerequisite product decision).

### Phase Y — Functional acceptance & business reconciliation
OUT OF SCOPE for a real signed UAT (documented 2026-08-08). The cited 16/16 automated reconciliation metrics were independently re-verified against the raw JSON and are accurate.

### Phase Z — Cutover & go-live
OUT OF SCOPE (documented 2026-08-08). One nuance: `docs/RUNBOOK_CUTOVER.md` already exists (660 lines) as a planning document — but every operational gate it lists remains unexecuted, and it self-labels "Not approved for go-live."

---

## Priority-ordered remaining work (highest leverage first)

1. **Report leaves golden verification (N–Q)** — 0 of 151 legacy-proven; 81 of 151 don't even have the right columns yet. Largest gap in the project by any measure.
2. **Pixel-parity sweep (Phase X)** — catalog doesn't exist, baseline directory is empty; needs to start from scratch.
3. **Preferences wiring (Phase V/S)** — 440 of 441 preferences have no backend behavior.
4. **Pricing engine real logic (Phase G)** — PricePolicy tiers and GroupAllowedPrice are completely unimplemented for sales; the "golden replay" claim needs to be redone for real.
5. **Security hardening (Phase R)** — make ADMINISTRATOR table-driven or explicitly ratify the bypass; get real menu-snapshot tests for all 4 groups, not 1.
6. **Master data gaps (Phase F)** — wire Godown/CustomerGroup/Areas into the menu; extract the shared list-chrome component.
7. **Stock engine gaps (Phase J/K)** — godown transfers (zero implementation), StockReport reconciliation, voucher posting, VirtualGl balance check.
8. **Tax engine completion (Phase L)** — migrate legacy rate tables, paisa-exact replay, tax register report.
9. **Maintenance stubs (Phase S/T)** — import/export adapters, legacy password rules, SMS/email templates, R0002 decision.
10. **Data-migration hygiene (Phase E)** — finish orphan cleanup, do the 20-item spot-check, root-cause the 8 dropped rows and the tax_amount=0 anomaly.
11. **Shell/MDI chrome (Phase D)** — global toolbar, status bar, real cascade/tile/layer geometry.
12. **Contextual menu coverage (Phase C)** — 5 of dozens of window types captured.
13. **Dev-runtime soak (Phase A)** — the tooling is built; the 24h soak itself has never been run.

## Confirmed out of scope for this engagement (decided 2026-08-08, re-verified accurate)
Phase U (physical hardware validation), Phase W (full-volume soak cluster), Phase Y (signed parallel-day UAT), Phase Z (cutover/go-live). Software-side work behind each is done and cited above/in `PARITY_FIX_PLAN_A-Z.md`.

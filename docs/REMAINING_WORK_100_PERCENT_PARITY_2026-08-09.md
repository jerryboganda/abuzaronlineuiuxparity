# Remaining Work for 100% Functional & Visual Parity — Verified Audit

Date: 2026-08-09. Companion to `PARITY_FIX_PLAN_A-Z.md`, `GAP_ANALYSIS_2026-08-06.md`, `PARITY_STATUS.md`, `HANDOFF_2026-08-08.md`.

**Method:** 10 independent read-only audit agents, one per phase-cluster (A–Z), each instructed to verify existing-doc claims directly against current code/tests/data rather than trust the docs — because several prior-session claims turned out to be stale or overstated (see below). No files were edited or migrated as part of this audit; every finding below is grounded in a file path, test name, or artifact cited by the auditing agent.

---

## Progress update — 2026-08-09, pricing / credit-limit / godown-transfer wave

Highest-leverage functional slice after the reports-gap waves. **100% functional parity is still not achieved.** This wave closed four named remaining-work gates in code; live `DATABASE_URL` integration and golden SQL Server replay remain open.

Shipped:

- **Phase V / credit limit:** `Preferences.Check Cr Limit In Cr Sales:` is in the Sale registry (default Yes). `enforceCreditSaleLimit` now no-ops when the effective value is No. Integration subtest exists (skipped without `DATABASE_URL`).
- **Phase G:** priced sales (`cash-sale` / `credit-sale` / `quotation` / `refused-sale`) overlay `PricePolicy` quantity/expiry tiers onto the selected sale-price level, and reject operator `GroupAllowedPrice` misses with HTTP 403 `price_level_not_allowed`. Default `Price # in Cash/Credit Sale` is read when the document omits `priceLevel`. Unit tests: `TestPriceLevelAllowedHonorsGroupAllowedPrice`, `TestSelectPricePolicyTierPicksHighestQualifyingUnexpiredRow`. Golden `pricing.Calculate()` replay against live SQL Server is still **not** done.
- **Phase J:** `POST /v1/maintenance/godown-transfer` FIFO-moves on-hand stock between godowns (paired `stock_ledger` out/in, dest batch via `lockOrCreatePurchaseBatch`). UI: `/app/maintenance/godown-transfer`. Parser unit test plus `TestGodownTransferMovesStock` (skipped without `DATABASE_URL`).
- **Phase F:** captured-menu extras now inject Godown, Areas, Customer Group, and Godown Transfer. Areas / Customer Group use the existing master form (thin `master_records`); Godown was already canonical.

Still open at the same priority as before: remaining ~437 stored-only preferences, report golden vs SQL Server, pixel-parity catalog, Accounting/Payroll/eRx extra PBDs, VirtualGl recon, purchase G/P formula.

## Progress update — 2026-08-09, PO fetch / vouchers / shell chrome wave

Highest-leverage remaining gates after the pricing/credit-limit/godown-transfer wave. **100% functional and visual parity is still not achieved.**

Shipped:

- **Phase I / PO fetch:** posting pack/loose/opening purchases with `sourceDocumentId` now validates a posted `purchase-order`, matching supplier, and remaining quantity (PO line minus draft/posted receipt lines on that `source_line_id`). Populate Invoice maps PO line ids into `sourceLineId`.
- **Phase K / vouchers:** `POST /v1/finance/vouchers` posts receipt/payment/journal through `business_documents` + `voucher_entries` + GL/party ledger. UI: `/app/finance/voucher`. Menu: Maintenance > Vouchers.
- **Phase D / chrome:** shared shell toolbar (New/Spray/Save/Erase/First/Prev/Next/Last/Print/Exit), focus-in field hints, cascade/tile/layer tab geometry, child titlebar/8px border/toolbar caption/unimplemented Phase links hidden.

Still open: remaining-qty UI on the purchase grid, VirtualGl reconciliation, ~437 prefs, pixel baselines, extra PBDs.

## Progress update — 2026-08-09, remaining-qty UI / voucher pickers wave

Highest-leverage remaining gates after PO-fetch/vouchers/chrome. **100% functional and visual parity is still not achieved.**

Shipped:

- **Phase I / PO remaining qty:** receipt `sourceLineId` is optional and persisted on pack/loose/opening. GET returns `remainingQuantity`. Populate Invoice fills quantity from remaining and skips fully received lines. Remaining column is overlay-hidden at 1936x1048.
- **Phase K / voucher pickers:** receipt/payment party selects (customer/supplier) and journal account selects via `GET /v1/finance/accounts`.
- Master delete and catalog Ctrl+X / Ctrl+Alt+M shortcuts were already live; no extra work.

Still open: VirtualGl recon, ~437 prefs, pixel baselines, extra PBDs, purchase G/P formula, legacy batch format (no PB evidence).

## Progress update — 2026-08-09, zero-price / default batch / reprint / customer-group wave

Highest-leverage remaining gates after remaining-qty/voucher pickers. **100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V / zero retail:** `Sale/Allow Zero Retail Price:` (default No) rejects cash/credit sale lines with resolved unit price <= 0.
- **Phase I / default batch:** empty purchase receipt batch/expiry fill from `General/Default Batch:` (`.`) and `General/Default Expiry:` (`2030-12-12`). Auto Batch Generation uses the same values instead of `AUTO-YYYYMMDD-NNN`.
- **Phase H / reprint:** sale and purchase Reprint opens the reprint report with `?filter=`; the report page seeds `filter` from the query. Overlay hides the extra toolbar control at 1936x1048.
- **Phase F / customer group:** payload fields for Category, GroupAllowedPrice, CreditLimit, DefaultItemDiscPerc.
- Report SQL bugs listed later in this audit as still open were already fixed in `docs/evidence/REPORTS_BUG_FIX_WAVE_2026-08-09.md` (9/10); purchase G/P remains a product decision.

Still open: ~432 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay.

## Progress update — 2026-08-09, max-days / prompt-before-print wave

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V / Max. Allowed Days:** credit-sale `dueDate` may not exceed the document date plus this many days (default 30). UI `max` on the due-date picker matches.
- **Phase V / Prompt Before Printing:** when Yes, sale and purchase Print confirms before preview/slip.

Still open: ~432 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay.

## Progress update — 2026-08-09, report sibling promotion / sale Show-* / print copies

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase N:** sibling leaves reuse existing line-detail / invoice-summary / hour-summary / item-summary contracts (`sale-detail-format-2`, `sales-return-detail-inv-wise`, `hourly-sales-graph`, `item-wise-item-wise-net-sales`, and related aliases).
- **Phase V:** Sale `Show *` header fields (default No, overlay-hidden) and General `Ask No. of copies in print dialog`.

Still open: ~421 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, purchase Show-* / application title / P-return copies

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Purchase header `Show *` extras (default No, overlay-hidden), `General/Application Title:` browser title, Purchase Return copy prompt.

Still open: ~410 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, quotation zero-price / quotation Show-* / sale-return account

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** quotation zero-price lock, quotation Show P/O Rate/Discount/Special Rate, sale-return account field.

Still open: ~405 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, return amount-paid prompts

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Ask Amount Paid on S/Return Saving and Ask Amount Received on P/Return Saving.

Still open: ~403 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, purchase Ask PO / credit days / LC

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Ask Purchase Order, Ask Credit Days, and Ask L. C. No. prompt on receipt save/post.

Still open: ~400 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, purchase Ask New PO / net rate / report date+account / quotation days

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** `Ask New Purchase Order No.` and `Ask Pur. Invoice No.` prompts; purchase `Show Net Rate` column; report default-date seed + Account column; quotation Delivery/Validity Days and Payment To.

Still open: ~391 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, copy remarks / PO Show-* / sale extra seeds

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Copy Remarks in Item Description on sale-return / purchase-return / purchase-order; purchase-order Show-* header extras; sale Ref 2/3/4 / person / doctor / message / account-for seeds.

Still open: ~374 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, report refresh / quotation and PO print lines

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** report auto-refresh interval; quotation Line1-8 print footer; purchase-order Line1-8 and footer on print preview.

Still open: ~355 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, item price locks / report terms

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** item-master Sale Price / Sale Disc(%) locks; Report Term 1-6 on print letterhead.

Still open: ~347 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, extra sale/purchase header seeds

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** sale Loyalty/Currency/Motor Vehicle fields; sale-return Account For seed; purchase Account For field.

Still open: ~342 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, purchase weight columns

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** purchase Show Unit Weight / Show Total Weight overlay-hidden columns.

Still open: ~340 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, ask-header / PO invoice extras

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Ask Header on sale/purchase/quotation; Ask Purchase Type; Customer Balance; Quotation Show Ref. No.; purchase-return Account For; PO invoice Show-* extras.

Still open: ~325 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, supplier balance / return headers / quotation extras

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Purchase Supplier Balance; return Header prompts; quotation Manufacturer/Color/Quantity Denomination.

Still open: ~319 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

## Progress update — 2026-08-09, PO line Show-* extras

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** PO Show Item Sale Tax / Item GST % / Item Flat Discount / Disc. Perc. 2 overlay-hidden columns.

Still open: ~315 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

---

## Progress update — 2026-08-09, return payment / round / pack qty / report extras

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Sale Return Payment Mode Amt. Paid / Payment A/C / Round Item Total; Purchase Return Payment Mode Amt. Received / Payment A/C / Round Item Total / Show Pack Qty. Overlay-hidden. Default Show Pack Qty No.
- **Phase V:** Sale Print Warranted Invoice, Quotation Pack Units, Purchase Order Default Purchase Order Category seeds.
- **Phase N:** `net-sale-summary` reuses invoice-summary; `slow-fast-moving-items` reuses item-summary; `item-wise-item-sale-and-return-activity` reuses item-summary with sale-or-return aggregate.

Still open: ~305 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

---

## Progress update — 2026-08-09, sale/purchase tax-discount grid extras

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Sale Item Disc. % / Item GST % bind existing line discount/GST; Extra Tax / cash-credit disc / Pre-Discount / Claimable Disc overlay-hidden columns; Fiscalization Machine IP seed.
- **Phase V:** Purchase Disc. % / Item Level GST % bind existing line fields; Extra Tax / Sale Disc / Margin / Net Margin overlay-hidden columns; PCT Code and Sales Tax Schedule seeds.
- **Phase V:** Quotation Claimable Discount % overlay-hidden column.

Still open: ~288 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

---

## Progress update — 2026-08-09, leftover header seeds / detail report extras

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Overlay-hidden header seeds for remaining Sale/Purchase/Sale Return/Purchase Return column-contract captions (alias, packing, description, tax, bonus, batch no, pieces, thermal format, price in P/Return).
- **Phase N:** `category-wise-item-wise-sale-discounts-detail` and `manufacturer-wise-sales-detail-and-summary` reuse line-detail.

Still open: ~252 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

---

## Progress update — 2026-08-09, quotation remote / PO extras / short name

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase V:** Quotation Remote Net Sale / Stock / Re-Order / Optimum / Min / P/O Qty overlay seeds.
- **Phase V:** Purchase Order Item Alert / Required Packs Fraction Treatment / Page Size for print out / P/O Supplier Consideration overlay seeds.
- **Phase D:** Business Short Name falls back `document.title` when Application Title is empty.

Still open: ~242 stored-only prefs, pixel baselines, extra PBDs, VirtualGl recon, purchase G/P, FEFO stock order, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

---

## Progress update — 2026-08-09, item-master BasicData defaults

**100% functional and visual parity is still not achieved.**

Shipped:

- **Phase F/V:** BasicData Manufacturer / Item Category / Item Class seed a blank item-master record on New/after delete.

Still open: ~239 stored-only prefs (POS/cashier/email/SMS/schedule/dashboard/adjustment/activity-monitor, password prompts, empty-invoice skips, FEFO/G/P, and unguessable leftovers), pixel baselines, extra PBDs, VirtualGl recon, 53 unmapped rights, golden SQL Server replay, dead-item-list / inv-type-wise remaining event-ledger leaves.

---

## ⚠ Corrections to prior claims (read this first)

A few things previously reported as done in this session's evidence docs — including two items marked `completed` on the running task list — do **not** hold up under independent verification:

1. **Pricing golden replay (task #12, "50-invoice replay to the paisa") is not what it claims to be.** `docs/evidence/PHASE_G_PRICING_GOLDEN_REPLAY_2026-08-08.md` itself admits: "the premise needs correcting before the numeric result is meaningful." The 50/50-invoice match is a **migration copy-fidelity check** (line totals were `COPY`'d straight from `dbo.Saledetail.Rate`, never run through `pricing.Calculate()`). Only 7 of 104 lines actually went through the pricing engine, and 1 of those had an unexplained Rs 45 discrepancy. The real pricing engine still doesn't implement `PricePolicy` tiers or `GroupAllowedPrice` at all for sales — see Phase G below.
2. **Orphaned test-tenant cleanup is not complete.** One tenant (`document-test-maintenance-20260808224042-859427600`) still exists in the dev DB as of this audit, contradicting the "391/391 deleted, 0 remaining" verification recorded earlier. The cleanup script's own success record was never actually logged anywhere in `docs/`.
3. **Security rights menu-gating parity (task #14) is verified for 1 of 4 legacy groups with real data, not all 4.** Only the REMOTE group has a Playwright test against actual migrated `group_rights`; the other 3 groups (ADMINISTRATOR, SALES OFFICER, SHIFT INCHARGE) are covered only by hand-typed synthetic fixtures. Separately, ADMINISTRATOR bypasses the rights table entirely via a role-name check (`hasTenantAdminRole`), so it isn't table-driven at all — a real architectural gap, not just a test-coverage gap.
4. **The Phase E "spot-check 20 random items/customers field-by-field" accept criterion was never performed** — zero evidence of it exists anywhere in the repo.
5. The tax_amount=0 anomaly (background task `task_b63bf468`) and the $970.95/$91,710.40 value-sum gap are **two different issues** that earlier framing conflated. Only the value-sum gap was actually root-caused and closed via `migration_exceptions`; the tax_amount=0 anomaly is still unconfirmed as bug-vs-data-drift.

None of this reverses real work that did happen (moving-average stock valuation, credit-limit enforcement, the security rights backfill mechanism, real backup/restore — all independently re-confirmed real below). It means the completion bar for a few specific claims needs to move back to PARTIAL.

---

## Progress update — 2026-08-09, reports-gap wave 1

10 parallel agents began closing the single largest gap identified above (Phases N–Q, 151 report leaves, 0 golden-verified). Methodology: live legacy SQL Server access is not reachable from this tool environment (Windows trusted-auth fails), so verification cross-checked each report's SQL against independently-authored queries over the same already-migrated, already-reconciled (16/16 metrics MATCHED) Postgres tables — not against a fresh SQL Server query. 9 new evidence docs (`docs/evidence/PHASE_{N,O,P,Q}_GOLDEN_VERIFICATION_*_2026-08-09.md`) hold full per-leaf query evidence.

**Coverage this wave:** 106 of the 151 catalog leaves got an evidence-based verdict (MATCHED / MISMATCH-DOCUMENTED / VACUOUS-NO-DATA) — **correction (2026-08-09, wave 2): the "114" figure originally recorded here was wrong.** It summed each agent's assigned-list size, which for one agent included 8 alias-only routes (`gl-journal`, `trial-balance`, `customer-statement`, `supplier-statement`, `receivables-aging`, `payables-aging`, `tax-register`, `voucher-register`) that are real, valuable, and were genuinely verified, but are not part of the 151-leaf catalog tracked by the `phaseN/O/P/Q` registries. Recomputing directly against the registry: 106 of 151 catalog leaves were touched, 45 were not — see the wave-2 section below, which closes that gap. 2 further leaves (`customer-sales-detail`, `customer-sales-summary`) were promoted from generic to real, reusing the already-verified `sale-detail`/`sale-summary` query shape (`docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`).

**~15 genuine bugs surfaced**, 9 fixed same-wave with regression tests (each pinned by a test that failed pre-fix and passes post-fix), the rest documented for a follow-up pass rather than guessed at:

Fixed:
- 6 `adjustment-*` leaves — total outage (`SQLSTATE 42P18`, unreferenced query parameters) → now return 200.
- 5 stock-balance leaves (`stock-in-hand-others`/`-batch-priority-wise`/`-stock-quantity-format`/`-stock-in-hand-audit-purpose`/`-batch-priority-wise-audit-purposes`) — total outage (ambiguous `updated_at` column across 3 joined tables) → now return 200.
- `listing-group-rights-list` — was reading the wrong (always-empty) table; now reads real `group_rights` (726 rows). Fixing this also surfaced and fixed a **second, more severe, pre-existing bug** affecting the entire `adminReadModelQuery` default branch (same unreferenced-parameter class as the adjustment bug) that was silently 503ing `listing-supplier-list`, `listing-items-list`, `listing-manufacturer-list`, and `listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict` too — none of which had actually been verified through the live endpoint before (see addendum in `docs/evidence/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md`).
- `sale-detail` (**Daily Sale Detail, the Phase M gating leaf**) — "SalesTax Value" column silently showed `0.00` for 28,184/620,615 lines (4.54%) tenant-wide.
- `customer-sales-items-summary` — Amount column inflated 6.6×–77.6× (summed the whole-document total once per line instead of a line-level amount).
- `purchase-return-detail` — "Purchase Price" column wrong for 551/2,481 lines (22.2%), used average stock cost instead of the actual return price.
- `header-wise-transaction-summary` — silently dropped all 30,704 migrated sale-return documents (9.25% of the tenant's posted documents) due to a kind-spelling gap other sibling queries already handled correctly.

Documented, not yet fixed (needs either a legacy-semantics judgment call or an ops/migration action, not a mechanical code fix):
- `supplier-wise-advance-income-tax` drops PKR 9,589.64 across 94 documents (LATERAL fallback gated to the wrong line).
- `stock_balances` cache is empty for the sandbox tenant (an ops/migration population gap, not a query bug) — blocks 12 more P-phase leaves from being verifiable at all right now.
- Reorder/Minimum/Optimum-level source fields largely unmigrated or unpopulated.
- `purchase-order`/`purchase-order-summary`/`purchase-order-supplier-wise` amount always `0.0000` for all 2,810 PO documents (ambiguous: migration gap vs. missing SUM fallback — needs the source data to disambiguate, which isn't available here).
- `category-wise-purchase` doesn't actually group by category; `manufacturer-wise-*` leaves have no manufacturer join at all despite the name; `supplier-manufacturer-wise-g-p` computes no gross-profit figure.
- `item-reports-history-*` — a COALESCE bug leaks the item name into the price-difference column for 2 leaves.
- `listing-item-list-class-wise` — structurally can never match any `master_records.kind` value; needs a real data-model decision.

**Updated N–Q count (wave 1):** at least 79 of 151 catalog leaves now have a real, evidence-backed MATCHED verdict or a fixed-and-verified projection (up from 0 golden-verified before this wave); the rest of the 106 examined either surfaced a still-open bug (documented above) or are genuinely data-empty in this sandbox (`VACUOUS-NO-DATA`, not a code defect). 45 of 151 leaves were not touched this wave (corrected count — see wave-2 note above). See the 9 evidence docs for the exact per-leaf verdict.

---

## Progress update — 2026-08-09, reports-gap wave 2 (the 45 leaves wave 1 missed)

10 more agents covered all 45 catalog leaves wave 1 didn't touch — 41 of them concentrated in Phase N's "Sales Reports" sub-menu (Category Wise, Manufacturer Wise, User Wise families) plus 4 in Phase O. Same methodology (no live SQL Server; independent Postgres cross-checks). 8 new evidence docs: `docs/evidence/PHASE_N_GOLDEN_VERIFICATION_{GROUP1_REMAINDER,SALE_RETURN,CATEGORY_A,CATEGORY_B,MANUFACTURER,USER_WISE,MISC_A}_2026-08-09.md`, `docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_REMAINDER_2026-08-09.md`, plus `docs/evidence/PHASE_N_REPORT_PROMOTION_WAVE2_2026-08-09.md` and `docs/evidence/PHASE_N_LEGACY_SEMANTICS_RESEARCH_2026-08-09.md`.

**Every one of the 45 leaves now has a status determination** — either a real golden-verified verdict, a confirmed-vacuous (no data) finding, or (for leaves that are genuinely not promotable yet) a precise, evidence-backed statement of exactly what's blocking them. Nothing was left as a bare unknown.

**6 more leaves confirmed MATCHED or promoted to real:**
- `sales-return-detail`, `sales-return-summary` — MATCHED (line-detail/invoice-summary, real query, verified against a 15-document sample).
- `sales-return-summary-inv-wise` — confirmed byte-identical SQL to `sales-return-summary` (a legitimate finding, not a gap, matching the wave-1 `reprinting-*` precedent).
- `purchase-return-summary` — MATCHED (634 docs, exact amount/qty reproduction).
- `supplier-wise-detail`, `supplier-wise-purchase-detail` — confirmed byte-identical SQL to the already-verified `manufacturer-wise-detail`.
- `supplier-purchase-returns-detail` — MATCHED on qty; surfaced a new, separate amount-source discrepancy (below).
- `sale-detail-inv-wise` — promoted to real (`line-detail` mode, reusing the already-verified `sale-detail` query).

**A wrong "blocked" finding from wave 1 was corrected.** Wave 1's promotion doc declined `category-wise-sales`/`manufacturer-wise-sales` citing "no resolved category/manufacturer name available, only opaque legacy codes." That check only looked at `master_items.payload`. Wave 2 found `master_categories` (7 rows) and `master_manufacturers` (838 rows) are real, populated lookup tables that resolve **100%** of items' `ICatCode`/`ManfCode` to real names via a simple join nobody had used — for sale lines, purchase lines, or the existing P-phase stock reports that already display the raw codes today. This unblocks the *data* half of ~15 leaves across N, O, and P; only the *column-layout* half (no legacy screenshot exists for any of them) remains open, per the dedicated research pass.

**One bug fixed this wave** (same defect class as wave 1's `sale-detail` fix, just never propagated to its sibling): `sales-return-detail`'s "SalesTax Value" column silently showed `0.00` for 1,970/44,579 lines (4.42%, Rs 311,703.05) tenant-wide — fixed and verified, verdict updated to MATCHED.

**New bugs found, documented, not fixed this wave** (need a legacy-semantics decision or further data engineering, not a 5-minute mechanical fix):
- `supplier-purchase-returns-detail` vs. `purchase-return-summary` — two different amount sources disagree for 566/634 purchase-return documents (89.3%), net gap PKR 35,762.30, some individual docs off by 4×+.
- `customer-sales-invoice-wise-profit-margin-detail` and `customer-sales-customer-category-wise-sales-customer-wise-gross-profit` (both wave-1 "real, MATCHED on price/tax" leaves) silently return **blank cost/profit-margin columns for every row** — they read cost exclusively from `stock_allocations` (0 rows in this tenant) when `stock_ledger` (100% populated, already used by the Phase J moving-average engine) has the real per-line cost sitting unused.

**Genuinely, confirmedly blocked — not a code gap, a source-data gap** (checked database-wide, not just the sandbox tenant):
- **User attribution (6 of 7 `user-wise-*` leaves):** `business_documents.operator_id` is 0/291,361 populated; the raw `legacy_payload->>'PostedBy'` fallback is JSON `null` for every single row too. No tenant in the database has real per-user attribution at meaningful scale. There is no query fix for this — the data was never captured or never migrated.
- **Gross profit / COGS (`category-wise-gross-profit` and related):** `stock_allocations` is 0 rows database-wide at any real scale (confirmed, not sandbox-specific) — though wave 2 found the `stock_ledger`-based workaround above resolves this for future promotions.
- **CNIC/NTN-registered customers:** only 2 generic bucket customer records exist tenant-wide; the CNIC/NTN fields are blank on both.
- `slow-fast-moving-items` — 0/30,052 items have any reorder/velocity threshold populated.
- `category-wise-deviated-items`, `category-wise-item-wise-sale-discounts-detail` (0 discounted lines tenant-wide), `dead-item-list` (needs a different query shape entirely — an anti-join, not a filter, since a sales-rooted query can never surface an item with zero sales) — each individually diagnosed.

**Updated N–Q count (through wave 2): all 151 catalog leaves now have a status determination.** Precise tally: **86 confirmed MATCHED or fixed-and-verified**, several more confirmed byte-identical duplicates of already-verified leaves, a documented handful of VACUOUS-NO-DATA (genuinely empty source data, not a defect), a documented handful of MISMATCH-DOCUMENTED bugs (2 fixed across the two waves, several more precisely diagnosed for a future fix pass), and a firmly-diagnosed remainder that is blocked on missing source data (user attribution, COGS allocations, discount/CNIC data) that no further code archaeology can resolve — that requires either a data engineering effort or is a genuine, permanent limitation of what was captured from the legacy system. **This does not mean "151/151 done"** — golden-number legacy-output proof (the Phase N–Q accept criterion) still doesn't exist for any leaf in the strictest sense (no live legacy SQL Server access was available in this environment for either wave); what exists now is full internal-consistency verification against the already-reconciled migrated data, which is the strongest verification achievable without that access.

---

## Phase-by-phase verified status

| Phase | Scope | Verdict |
|---|---|---|
| A | Dev runtime stability | PARTIAL — scripts/CI test real, 24h soak never executed |
| B | Quick-win visual defects | DONE-VERIFIED |
| C | Menu catalog incl. contextual | PARTIAL — only 5 of dozens of window types captured |
| D | Shell & MDI chrome | PARTIAL — shared toolbar + field hints + MDI tab layouts shipped; raster 1-window/n-window still undemonstrated |
| E | Data migration & reconciliation | PARTIAL — 16/16 metrics matched, but hygiene gaps (see corrections) |
| F | Master data engine | PARTIAL — Godown/Areas/Customer Group now menu-reachable; CustomerGroup category/detail still thin; no shared list-chrome |
| G | Pricing & discount policies | PARTIAL — PricePolicy overlay + GroupAllowedPrice now on priced sales; golden `pricing.Calculate()` replay still overstated |
| H | Sales workflows | PARTIAL — real screens/lifecycle, no raster gate, pack/loose fix only spot-verified |
| I | Purchase workflows | PARTIAL — PO source lock + remaining-qty field/UI; legacy batch format and purchase G/P still open |
| J | Inventory & stock engine | PARTIAL — moving-average done; godown transfer implemented (unit tests; DB integration skipped without DATABASE_URL) |
| K | Financial core (GL/ledgers) | PARTIAL — voucher posting + party/account pickers; VirtualGl recon still open |
| L | Tax engine | PARTIAL — engine real, legacy rates unmigrated, no paisa-replay, no tax register |
| M | Reports engine core | PARTIAL — dialogs/preview/export real, Daily Sale Detail pixel-diff never run |
| N–Q | 151 report leaves | **UPDATED 2026-08-09 — see "Progress update" above: 114/151 evidence-verified, ≥79/151 confirmed correct or fixed, ~15 bugs found (9 fixed)** |
| R | Security & rights | PARTIAL — backfill mechanism real (433/486 codes), admin bypass + 3/4 groups untested with real data |
| S | Maintenance module | PARTIAL — backup/restore real; import/export stub; godown transfer live; **preferences 4 wired / rest stored_only**; R0002 undecided |
| T | Manage module & sessions | PARTIAL — session monitor & rights-matrix UI real; no legacy password rules; no SMS/email templates |
| U | Hardware integrations | DONE-VERIFIED (software side; physical validation explicitly out of scope) |
| V | Preferences full parity | PARTIAL — **4 of ~441 preferences wired** (report header, Check Cr Limit, Price # cash/credit); no legacy-value migration |
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
- Shared shell toolbar now exists (New/Spray/Save/Erase/nav/Print/Exit); child windows still also hand-roll extra buttons (Void, etc.). Spray/Erase are status-only.
- Field-focus status hints exist when the control has an accessible name; not every legacy micro-hint string is captured.
- Cascade/tile/layer now change MDI tab geometry; they still do not move child window rectangles.
- Raster diffs exist only for the 0-window base shell; 1-window and n-window states are undemonstrated.

### Phase E — Data migration & reconciliation
16/16 business metrics matched and independently DB-cross-verified (0 open exceptions/ambiguities). Line-unit repair (3-step SQL) verified with 0 identity/ledger violations.
**Remaining (see also "Corrections" above):**
- Finish the orphaned-tenant cleanup — 1 tenant still present; log a completion record.
- Perform the literal "spot-check 20 random items/customers field-by-field."
- Root-cause the 8 dropped `Saledetail`/`Purdetail` rows (`reason_code='line_dropped_by_bulk_import_unreviewed_cause'`) instead of relying on a compensating metric adjustment.
- Investigate the separate `tax_amount=0` anomaly for qty>1 lines (background task `task_b63bf468`) — still unconfirmed bug vs. drift.
- Document a scope decision for the 714/763 unmapped source tables (mostly appear to be hospital/school/CRS_ modules bundled in the same legacy DB, but no written decision excludes them).
- Refresh `docs/archive/GAP_ANALYSIS_2026-08-06.md` G1 section — still reads "never executed."

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
- In-document reprint now opens the reprint report with `?filter=`; exact PowerBuilder selection/format/print output remains open.
- Exact rounding / non-cash tender void-refund settlement open.

### Phase I — Purchase workflows
Same maturity level as Sales — real screens, batch/expiry, GL postings, supplier scheme.
**Remaining:**
- Auto Batch Generation and empty-receipt posting now fill Default Batch `.` and Default Expiry `2030-12-12` (legacy stock evidence). Physical/byte-level label comparison remains open.
- PO→invoice fetch posts with remaining-qty lock and remaining-qty column/UI; overlay hides Remaining at 1936x1048. Legacy batch format and purchase G/P formula remain open.
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
- Voucher posting exists (`POST /v1/finance/vouchers`) with party/account pickers; VirtualGl recon is still open.
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
Original state, verified directly from the registry code (`reports.go`):

| Wave | Leaves | Real projection | Event-ledger (generic columns) | Golden-verified vs legacy |
|---|---|---|---|---|
| N — Daily & sales ops | 68 | 7 | 61 | 0 |
| O — Purchases & suppliers | 24 | 4 | 20 | 0 |
| P — Stock & inventory | 27 | 27 | 0 | 0 |
| Q — Financial & remaining | 32 | 32 | 0 | 0 |
| **Total** | **151** | **70** | **81** | **0** |

**Superseded 2026-08-09 — see "Progress update" near the top of this doc.** 114 of 151 leaves now have an evidence-based verdict; ≥79 are confirmed-correct or fixed-and-verified; ~15 bugs surfaced, 9 fixed same-wave. 37 leaves untouched. Full per-leaf detail in `docs/evidence/PHASE_{N,O,P,Q}_GOLDEN_VERIFICATION_*_2026-08-09.md` and `docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`.

"Real" means a report-specific column/grouping contract exists — it does **not** mean legacy-verified output; every single evidence doc across N/O/P/Q ends with "exact PowerBuilder calculations... golden replay remain open." "Event-ledger" (81 leaves, almost entirely Sales Reports and Purchase Reports sub-menus) means only generic aggregate columns are returned, not the report's actual legacy shape.
**Remaining (original assessment — narrowed by the 2026-08-09 wave above):**
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

1. **Report leaves golden verification (N–Q)** — wave 1 done 2026-08-09 (114/151 evidence-verified, 9 bugs fixed); 37 leaves untouched and ~6 documented bugs still open (see progress update above). Still the largest gap in the project, but no longer a 0% start.
2. **Pixel-parity sweep (Phase X)** — catalog doesn't exist, baseline directory is empty; needs to start from scratch.
3. **Preferences wiring (Phase V/S)** — ~400 of ~441 preferences still have no backend behavior (credit-limit, cash/credit default price #, Allow Zero Retail Price, Default Batch/Expiry, Max. Allowed Days, Prompt Before Printing, Ask No. of copies, Sale/Purchase/Quotation Show-*, Application Title, quotation zero-price, return amount prompts, purchase Ask PO/credit/LC now wired).
4. **Pricing engine real logic (Phase G)** — PricePolicy tiers and GroupAllowedPrice now apply on priced-sale posting; redo golden replay so invoices are computed by `pricing.Calculate()`, not copied from source.
5. **Security hardening (Phase R)** — make ADMINISTRATOR table-driven or explicitly ratify the bypass; get real menu-snapshot tests for all 4 groups, not 1.
6. **Master data gaps (Phase F)** — Godown/Areas/Customer Group are menu-reachable; Customer Group now has payload category/price/credit/disc fields. Shared list-chrome remains.
7. **Stock engine gaps (Phase J/K)** — godown transfers implemented; StockReport reconciliation, voucher posting, VirtualGl balance check remain.
8. **Tax engine completion (Phase L)** — migrate legacy rate tables, paisa-exact replay, tax register report.
9. **Maintenance stubs (Phase S/T)** — import/export adapters, legacy password rules, SMS/email templates, R0002 decision.
10. **Data-migration hygiene (Phase E)** — finish orphan cleanup, do the 20-item spot-check, root-cause the 8 dropped rows and the tax_amount=0 anomaly.
11. **Shell/MDI chrome (Phase D)** — global toolbar, status bar, real cascade/tile/layer geometry.
12. **Contextual menu coverage (Phase C)** — 5 of dozens of window types captured.
13. **Dev-runtime soak (Phase A)** — the tooling is built; the 24h soak itself has never been run.

## Confirmed out of scope for this engagement (decided 2026-08-08, re-verified accurate)
Phase U (physical hardware validation), Phase W (full-volume soak cluster), Phase Y (signed parallel-day UAT), Phase Z (cutover/go-live). Software-side work behind each is done and cited above/in `PARITY_FIX_PLAN_A-Z.md`.

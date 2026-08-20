# AbuzarNext Full Legacy Parity & Acceptance Design

- **Date**: 2026-08-07
- **Target**: `D:\ABUZAR\AbuzarNext`
- **Objective**: Bring AbuzarNext (SvelteKit + Go + PostgreSQL) to 100% visual, functional, workflow, and data parity with legacy PowerBuilder V3 (`abuzar.exe` + SQL Server `FazalDinPP19DataBaseV2`).

---

## 1. Executive Summary & Strategy

We are executing **Approach 1: High-Yield Verification & Evidence Closure**. The program follows a strict dependency spine where every claimed capability must be backed by concrete verification artifacts (test logs, raster diffs, DB reconciliation reports).

### Prime Principles
1. **Legacy is Read-Only Oracle**: All verification compares against `FazalDinPP19DataBaseV2` read-only.
2. **Evidence-Driven Gates**: No status or parity claim without a documented evidence file in `docs/`.
3. **Paisa & Qty Parity**: Replaying historical transactions must match legacy price, discount, tax, GL, and stock totals to exact precision.
4. **Pixel Baseline**: 1936×1048 raster comparison via `parity/tools/compare-png.ps1`.

---

## 2. Phase Execution Pipeline

```
[Phase E: Data Migration & Reconciliation]
       │
       ▼
[Phases F-L: Master Data, Pricing, Sales/Purchase, Stock, Finance, Tax Core]
       │
       ▼
[Phases M-Q: Report Engine & 151 Leaf Report Wave]
       │
       ▼
[Phases R-V: Group Rights, Maintenance, Hardware Edge, Preferences]
       │
       ▼
[Phases W-Z: Scale/Perf, Pixel Sweep (1936×1048), UAT, Cutover Runbook]
```

### Breakdown of Key Execution Streams

#### Stream E: Canonical Data Migration & Business Metric Reconciliation
- **Inspect & Map**: Map all 763 legacy SQL Server tables to target PostgreSQL schemas.
- **Import & Reconcile**: Import master data, transactional documents (`Saledetail`, `Purdetail`), ledgers (`SaleLedger`, `Purledger`, `VirtualGl`), and stock snapshots.
- **Verification Metric**: Validate row counts and $\ge 12$ business financial metrics (sales total, stock qty, GL balance, max invoice numbers) within $\le 0.01$ tolerance.

#### Stream F–L: Core Business Engines & Workflows
- **Master Data**: Complete legacy list chrome (sort/filter/find/detail tabs) and ~24 explicit master forms without demo fallbacks.
- **Pricing & Discounts**: 10 SalePrice tiers, customer/group policy enforcement, supplier discount/bonus schemes.
- **Sales & Purchase**: Complete Cash/Credit, Open Returns, POs, Quotations, Refused Sales with batch/expiry enforcement, Ctrl+Q/B/H/M/Alt+F8 shortcuts, and GL auto-posting.
- **Stock & GL Core**: FIFO/average valuation, per-godown stock ledger, running ledger balances, credit limit enforcement, tax registers.

#### Stream M–Q: Reports Engine & 151 Leaf Reports
- **Core Engine**: Select Format dialog, "Specify Retrieval Arguements" dialog, letterhead print-preview toolbar, PDF/Excel export.
- **Report Waves**: Daily/Sales (~40), Purchases (~35), Stock (~40), Financial/GL (~36).

#### Stream R–Z: Security, Hardware, Pixel Sweep & Cutover
- **Group Rights**: 4 Groups, 726 GroupRights rules, Go middleware enforcement, UI menu gating.
- **Hardware (Edge)**: Thermal slip printing (ESC/POS byte-match), label printing, cash drawer kick.
- **Pixel Sweep**: Raster comparisons at 1936×1048.
- **Hardening**: Perform 8h soak, verify POS line-add $< 150$ ms, post $< 1$ s.

---

## 3. Verification & Acceptance Criteria

Every phase will be verified using the four core verification commands:
1. `pnpm --filter @abuzar/web check` (0 errors, 0 warnings)
2. `pnpm --filter @abuzar/web test` (Pass all Playwright browser tests)
3. `pnpm --filter @abuzar/web build` (Clean production static build)
4. `go test ./services/api/... ./services/edge/... ./migration/... -count=1` (All backend tests passing)

Additionally, individual evidence markdown files will be updated under `docs/` for each completed phase.

---

## 4. Next Steps
Upon user approval of this design document, we will invoke the `writing-plans` skill to generate a detailed step-by-step implementation plan.

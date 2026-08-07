# AbuzarNext Legacy Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the AbuzarNext (SvelteKit + Go + PostgreSQL) legacy PowerBuilder rebuild with 100% workflow, functional, and UI/UX parity, passing verified test suites and producing required acceptance evidence.

**Architecture:** A SvelteKit frontend UI communicating with a Go HTTP API backend and Edge service over PostgreSQL database schemas, adhering strictly to legacy PowerBuilder business logic, calculations, layout geometry, and transactional data reconciliation.

**Tech Stack:** Svelte 5, SvelteKit, TypeScript, TailwindCSS/Vanilla CSS, Go, PostgreSQL, Playwright, PowerShell.

## Global Constraints
- Legacy database `FazalDinPP19DataBaseV2` is strictly READ-ONLY.
- No parity claim without linked evidence file in `docs/`.
- 100% numerical match (price, discount, tax, GL, stock qty) to the paisa.
- Pixel verification baseline at 1936×1048 resolution.

---

### Task 1: Verification Suite Baseline & Dev Runtime Guardrails

**Files:**
- Modify: `ops/local/start-local.ps1`
- Modify: `ops/local/status-local.ps1`
- Test: `apps/web/tests/smoke.spec.ts`

**Interfaces:**
- Consumes: Local stack services (Postgres :5432, API :8080, Edge :8091, Web :5173).
- Produces: Reliable, non-blocking dev stack supervisor and automated smoke route verification.

- [ ] **Step 1: Write/Update the failing test assertion in smoke.spec.ts**

```typescript
test('SSR smoke gate checks core routes cleanly', async ({ page }) => {
  const routes = ['/login', '/app/legacy', '/app/sales', '/app/module/purchase-orders'];
  for (const route of routes) {
    const res = await page.goto(route);
    expect(res?.status()).toBe(200);
  }
});
```

- [ ] **Step 2: Run test to verify status**

Run: `pnpm --filter @abuzar/web test tests/smoke.spec.ts`
Expected: PASS or clean fail if server detached issues exist.

- [ ] **Step 3: Update local supervisor detached script**

Ensure `ops/local/start-local.ps1` launches Vite dev server detached and restarts failed children smoothly.

- [ ] **Step 4: Execute check commands**

Run: `pnpm --filter @abuzar/web check`
Expected: 0 errors, 0 warnings.

- [ ] **Step 5: Record evidence**

Document runtime baseline in `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`.

---

### Task 2: Data Migration Pipeline & Reconciler Execution (Phase E)

**Files:**
- Modify: `migration/cmd/reconcile/main.go`
- Modify: `migration/maps/` (all wave mapping manifests)
- Test: `migration/cmd/reconcile/main_test.go`

**Interfaces:**
- Consumes: SQL Server `FazalDinPP19DataBaseV2` tables.
- Produces: Populated PostgreSQL target database `abuzar_next` with zero business metric drift.

- [ ] **Step 1: Write metric reconciliation test**

```go
func TestReconcileBusinessMetrics(t *testing.T) {
    metrics, err := reconcile.CompareMetrics(ctx, legacyDB, postgresDB)
    require.NoError(t, err)
    require.True(t, metrics.AllGreen(), "Discrepancy found in business metrics: %v", metrics.Failures)
}
```

- [ ] **Step 2: Run reconciliation test**

Run: `go test ./migration/cmd/reconcile -v`
Expected: Evaluation of row counts and 12+ business metric comparisons.

- [ ] **Step 3: Execute migration import & reconcile execution**

Run: `go run ./migration/cmd/import` followed by `go run ./migration/cmd/reconcile`.

- [ ] **Step 4: Record evidence**

Document reconciliation results in `docs/PHASE_E_BOOKKEEPING_AND_ORDER_WAVE_EVIDENCE_2026-08-07.md`.

---

### Task 3: Pricing & Discount Engine Hardening (Phase G)

**Files:**
- Modify: `services/api/internal/httpapi/pricing.go`
- Modify: `apps/web/src/lib/components/sales/PricingTierSelector.svelte`
- Test: `services/api/internal/httpapi/pricing_test.go`

**Interfaces:**
- Consumes: Customer ID, Item ID, Group ID, Tier Code.
- Produces: Calculated line net price, discount %, bonus quantity, and tax value matching PowerBuilder formulas.

- [ ] **Step 1: Write failing test for tier pricing replay**

```go
func TestPricingReplayHistoricalInvoices(t *testing.T) {
    // Replay 50 historical invoices and verify exact price calculation
    for _, inv := range historicalInvoices {
        calc := CalculatePrice(inv.Params)
        require.Equal(t, inv.ExpectedTotal, calc.TotalAmount)
    }
}
```

- [ ] **Step 2: Run test to verify failure/pass**

Run: `go test ./services/api/internal/httpapi -run TestPricingReplay -v`

- [ ] **Step 3: Implement exact calculation rules**

Ensure SalePrice 1-10 selector, flat discount, Misc +1.00 fee, item discount, and ItemSuppliers bonus scheme calculations match legacy.

- [ ] **Step 4: Run Playwright pricing test**

Run: `pnpm --filter @abuzar/web test -- --grep "pricing"`

- [ ] **Step 5: Record evidence**

Document evidence in `docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md`.

---

### Task 4: Sales & Purchase Workflows Lifecycle Verification (Phases H & I)

**Files:**
- Modify: `services/api/internal/httpapi/business.go`
- Modify: `apps/web/src/routes/app/sales/+page.svelte`
- Modify: `apps/web/src/routes/app/module/purchase-orders/+page.svelte`
- Test: `apps/web/tests/sales-canonical.spec.ts`
- Test: `apps/web/tests/purchase-canonical.spec.ts`

**Interfaces:**
- Consumes: Sales/Purchase document payloads (lines, allocations, batch, expiry, headers).
- Produces: Posted document headers, stock movements, GL journal entries, party ledger entries.

- [ ] **Step 1: Write failing Playwright test for complete sales & purchase post lifecycle**

```typescript
test('Cash Sale Save and Post emits correct allocations and postings', async ({ page }) => {
  await page.goto('/app/sales');
  // Fill line item, select batch, enter price, press Ctrl+Q (Save & Post)
  await page.click('#post-btn');
  await expect(page.locator('.toast-success')).toBeVisible();
});
```

- [ ] **Step 2: Run Playwright sales & purchase suites**

Run: `pnpm --filter @abuzar/web test tests/sales-canonical.spec.ts tests/purchase-canonical.spec.ts`

- [ ] **Step 3: Complete document posting handlers in backend**

Ensure `business.go` handles draft->post, batch allocation validation, GL postings, party ledger updates, and sequence generation.

- [ ] **Step 4: Verify type safety and static build**

Run: `pnpm --filter @abuzar/web check && pnpm --filter @abuzar/web build`

- [ ] **Step 5: Record evidence**

Document evidence in `docs/PHASE_H_SALES_FRONTEND_EVIDENCE_2026-08-06.md` and `docs/PHASE_I_PURCHASE_EVIDENCE_2026-08-06.md`.

---

### Task 5: Inventory, Valuation & Stock Ledger Engine (Phase J)

**Files:**
- Modify: `services/api/internal/httpapi/stock.go`
- Test: `services/api/internal/httpapi/stock_test.go`

**Interfaces:**
- Consumes: Stock movement events from documents, adjustments, transfers.
- Produces: Item stock ledger per godown/batch and valuation matching legacy `StockReport`.

- [ ] **Step 1: Write stock ledger reconciliation test**

```go
func TestStockReportReconciliation(t *testing.T) {
    report, err := GetStockReport(ctx, godownID)
    require.NoError(t, err)
    require.Equal(t, legacySnapshotTotal, report.TotalQty)
}
```

- [ ] **Step 2: Run stock engine tests**

Run: `go test ./services/api/internal/httpapi -run TestStock -v`

- [ ] **Step 3: Implement stock ledger & valuation calculations**

Maintain per-godown/batch signed net stock ledger with FIFO/average cost valuation.

- [ ] **Step 4: Record evidence**

Document in `docs/PHASE_J_STOCK_EVIDENCE_2026-08-06.md`.

---

### Task 6: Financial Core, VirtualGL & Party Ledgers (Phases K & L)

**Files:**
- Modify: `services/api/internal/httpapi/finance.go`
- Test: `services/api/internal/httpapi/finance_test.go`

**Interfaces:**
- Consumes: Posted documents and manual GL vouchers.
- Produces: `VirtualGl` ledger records, party balances, tax register entries.

- [ ] **Step 1: Write VirtualGL reconciliation test**

```go
func TestVirtualGLBalanceReconciliation(t *testing.T) {
    glBalance, err := GetGLBalance(ctx, accountID)
    require.NoError(t, err)
    require.Equal(t, legacyGLBalance, glBalance)
}
```

- [ ] **Step 2: Run financial tests**

Run: `go test ./services/api/internal/httpapi -run "TestHistoricalGL|TestPhaseK|TestPhaseL" -v`

- [ ] **Step 3: Implement party ledger and tax calculations**

Wire GST, PCT, advance income tax, receivables aging, running ledger balances.

- [ ] **Step 4: Record evidence**

Document in `docs/PHASE_K_FINANCE_EVIDENCE_2026-08-06.md` and `docs/PHASE_L_TAX_EVIDENCE_2026-08-06.md`.

---

### Task 7: Reports Engine & 151 Leaf Report Wave (Phases M, N, O, P, Q)

**Files:**
- Modify: `services/api/internal/httpapi/reports.go`
- Modify: `apps/web/src/routes/app/reports/+page.svelte`
- Test: `services/api/internal/httpapi/reports_test.go`
- Test: `apps/web/tests/reports.spec.ts`

**Interfaces:**
- Consumes: Report retrieval arguments, selected named format.
- Produces: Paginated print preview with letterhead, export datasets (PDF/Excel), exact PowerBuilder output formats.

- [ ] **Step 1: Write report definition & format tests**

```go
func TestReportFormatsCoverage(t *testing.T) {
    for _, leaf := range catalogLeaves {
        def, err := GetReportDefinition(leaf.ID)
        require.NoError(t, err)
        require.NotEmpty(t, def.Formats)
    }
}
```

- [ ] **Step 2: Run report backend and frontend tests**

Run: `go test ./services/api/internal/httpapi -run "TestReport|TestPhaseM|TestPhaseN|TestPhaseO|TestPhaseP|TestPhaseQ" -v`

- [ ] **Step 3: Complete report leaf definitions**

Ensure all 151 leaf reports render accurate formatted columns and toolbar actions.

- [ ] **Step 4: Verify web suite passing cleanly**

Run: `pnpm --filter @abuzar/web test`

- [ ] **Step 5: Record evidence**

Document in `docs/PHASE_M_REPORT_CORE_EVIDENCE_2026-08-06.md` through `PHASE_Q_FINANCIAL_REPORT_EVIDENCE_2026-08-06.md`.

---

### Task 8: Group Rights, Edge Hardware & Final Acceptance Sweep (Phases R, U, W, X, Y, Z)

**Files:**
- Modify: `services/api/internal/httpapi/auth.go`
- Modify: `services/edge/internal/hardware/printer.go`
- Test: `apps/web/tests/security.spec.ts`
- Modify: `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md`

**Interfaces:**
- Consumes: User group rights (726 rules), ESC/POS hardware print requests, 1936×1048 rasters.
- Produces: Group-gated permissions, byte-matched slip/label print commands, verified raster diffs, cutover approval.

- [ ] **Step 1: Run complete verification suite**

```powershell
pnpm --filter @abuzar/web check
pnpm --filter @abuzar/web test
pnpm --filter @abuzar/web build
go test ./services/api/... ./services/edge/... ./migration/... -count=1
```

- [ ] **Step 2: Run raster compare sweep**

Run: `parity/tools/compare-png.ps1` across all captured window rasters at 1936×1048.

- [ ] **Step 3: Update acceptance evidence doc**

Update `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` with final verification pass results and evidence links.

- [ ] **Step 4: Commit and finalize phase documentation**

Commit implementation artifacts to git.

# Purchase Workflows Real-Time Inventory Picker & Item Search Parity Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full real-time Inventory Lookup & Item Search Parity with toggleable floating window across all 5 Purchase routes in AbuzarNext.

**Architecture:** Integrate a dual-mode inventory surface into `apps/web/src/routes/app/purchase/[kind]/+page.svelte` featuring a embedded real-time lookup table and a floating toggleable inventory modal (`F2` shortcut / toolbar button) connected to `GET /v1/items/lookup`.

**Tech Stack:** SvelteKit, TypeScript, Vanilla CSS, Playwright, Go API (PostgreSQL 17).

## Global Constraints

- Svelte 5 / SvelteKit static export compatibility.
- 0 TypeScript errors or warnings (`pnpm --filter @abuzar/web check`).
- Maintain exact visual and functional parity with PowerBuilder desktop app (`abuzar.exe`).

---

### Task 1: Dual-Mode Inventory Surface Component Implementation

**Files:**
- Modify: `apps/web/src/routes/app/purchase/[kind]/+page.svelte`
- Modify: `apps/web/src/lib/styles.css`
- Test: `apps/web/tests/purchase-inventory-picker.spec.ts`

**Interfaces:**
- Consumes: `api.itemLookup(query)` -> `ItemLookupResult[]`
- Produces: Interactive Inventory Lookup panel & floating modal window (`showInventoryWindow`) for item selection and row population across purchase routes.

- [ ] **Step 1: Write Playwright E2E Test Spec**

Create `apps/web/tests/purchase-inventory-picker.spec.ts`:
```ts
import { test, expect } from '@playwright/test';

test.describe('Purchase Inventory Picker Parity', () => {
  test('displays real-time inventory lookup and populates item details on purchase routes', async ({ page }) => {
    await page.goto('/app/purchase/pack');
    await expect(page.locator('.legacy-purchase-lookup')).toBeVisible();
    const searchInput = page.locator('input[aria-label="Item lookup query"]');
    await searchInput.fill('Panadol');
    await expect(page.locator('.legacy-purchase-lookup table')).toContainText('Panadol');
    await page.locator('.legacy-purchase-lookup button').first().click();
    await expect(page.locator('input[aria-label="Item name 1"]')).not.toHaveValue('');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @abuzar/web test tests/purchase-inventory-picker.spec.ts`
Expected: FAIL (component `.legacy-purchase-lookup` not found)

- [ ] **Step 3: Implement Dual-Mode Inventory Surface in Purchase Page**

Update `apps/web/src/routes/app/purchase/[kind]/+page.svelte` and `apps/web/src/lib/styles.css` to add:
- Embedded Inventory Search Panel (`.legacy-purchase-lookup`) with live lookup results table (Item Name, Stock, Purchase Price, Sale Price, Manufacturer, Pack Units, Location).
- Floating Inventory Modal Window (`.legacy-purchase-inventory-window`) toggleable via `F2` key or toolbar button.
- Selection handlers (`chooseLookupItem`) populating current line row.

- [ ] **Step 4: Verify typecheck & test pass**

Run: `pnpm --filter @abuzar/web check`
Expected: 0 errors, 0 warnings

Run: `pnpm --filter @abuzar/web test tests/purchase-inventory-picker.spec.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/routes/app/purchase/\[kind\]/+page.svelte apps/web/src/lib/styles.css apps/web/tests/purchase-inventory-picker.spec.ts
git commit -m "feat(purchase): add real-time inventory lookup surface to purchase workflows"
```

---

### Task 2: Build & Deploy Updated Purchase Interface to Production VPS

**Files:**
- Modify: `apps/web/build/`
- Target: Production VPS `185.252.233.186` (`/opt/docker/abuzarnext/build/`)

- [ ] **Step 1: Run production build**

Run: `pnpm --filter @abuzar/web build`
Expected: PASS (`Wrote site to "build"`)

- [ ] **Step 2: Upload static export to production VPS**

Run: `scp -r apps/web/build/* root@185.252.233.186:/opt/docker/abuzarnext/build/`
Expected: SCP exit code 0

- [ ] **Step 3: Restart web service on VPS**

Run: `ssh root@185.252.233.186 "cd /opt/docker/abuzarnext && docker compose restart web"`
Expected: `Container abuzarnext-web Started`

- [ ] **Step 4: Verify HTTPS endpoints**

Run: `curl.exe -sI https://pms.polytronx.com/app/purchase/pack`
Expected: HTTP 200 OK

- [ ] **Step 5: Commit**

```bash
git add .; git commit -m "deploy(purchase): upload purchase inventory picker parity update to production VPS"
```

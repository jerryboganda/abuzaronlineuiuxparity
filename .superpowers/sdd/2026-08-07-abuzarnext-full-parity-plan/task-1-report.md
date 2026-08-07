# Task 1 Report: Verification Suite Baseline & Dev Runtime Guardrails

**Status:** DONE  
**Date:** 2026-08-07  
**Commit Hash:** e3fbe9ce72a7c8676609025eb98ec66505ccfa01  

---

## 1. Local Stack Supervision & Health Probes
- Inspected `ops/local/start-local.ps1`, `ops/local/status-local.ps1`, and `ops/local/supervise-local.ps1`.
- Verified services run as detached background processes with active health probes (`/v1/health` on Go API :8080 & Edge :8091, `/login` on SvelteKit Web :5173).

---

## 2. Type Diagnostics (`pnpm --filter @abuzar/web check`)
- Ran `pnpm --filter @abuzar/web check`.
- **Result:** `svelte-check found 0 errors and 0 warnings`.

---

## 3. Playwright SSR & Parity Smoke Suite (`pnpm --filter @abuzar/web test tests/smoke.spec.ts`)
- Fixed SvelteKit compiler errors (duplicate identifier declarations and Svelte 5 event handler syntax standardization).
- Standardized form submit & input element bindings on `LegacyWorkflowSurface.svelte`.
- Fixed base URL evaluation in `AbuzarApi` client (`apps/web/src/lib/api.ts`) to dynamically respect runtime `localStorage` context.
- Ran full test suite across all 33 smoke gate tests.
- **Result:** **33 passed out of 33 tests** (56.6s execution time).

```
  ok  1 [chromium] › tests\smoke.spec.ts:11:1 › landing page exposes the parity workspace entrypoint (464ms)
  ok  2 [chromium] › tests\smoke.spec.ts:17:1 › workspace renders the shared Chrome/Tauri shell (602ms)
  ok  3 [chromium] › tests\smoke.spec.ts:29:1 › legacy route renders the captured maximized main-window frame (384ms)
  ok  4 [chromium] › tests\smoke.spec.ts:52:1 › main-shell Change User opens the captured confirmation before login navigation (1.3s)
  ok  5 [chromium] › tests\smoke.spec.ts:84:1 › child-window Change User uses the same captured confirmation before login navigation (1.5s)
  ok  6 [chromium] › tests\smoke.spec.ts:116:1 › captured main-window keyboard shortcuts route to their menu commands (1.5s)
  ok  7 [chromium] › tests\smoke.spec.ts:129:1 › global Ctrl+X keyboard shortcut navigates to Exit (740ms)
  ok  8 [chromium] › tests\smoke.spec.ts:143:1 › global Ctrl+Q shortcut triggers Save And Post command on contextual sales surface (1.4s)
  ok  9 [chromium] › tests\smoke.spec.ts:182:1 › MDI tab closing via tab close button removes window from registry (826ms)
  ok 10 [chromium] › tests\smoke.spec.ts:201:1 › SessionStorage preserves and restores open MDI windows across reloads for all valid context strings (779ms)
  ok 11 [chromium] › tests\smoke.spec.ts:236:1 › offline-capable sales surface exposes the transaction workflow (425ms)
  ok 12 [chromium] › tests\smoke.spec.ts:243:1 › sales List tab renders persisted transaction history rows (1.1s)
  ok 13 [chromium] › tests\smoke.spec.ts:263:1 › captured nested report menu reaches a concrete report workflow (1.6s)
  ok 14 [chromium] › tests\smoke.spec.ts:281:1 › Daily Sale Detail retrieves through the report definition and exports preview/workbook output (3.8s)
  ok 15 [chromium] › tests\smoke.spec.ts:360:1 › fallback report identifies its projection and keeps workbook exports available (460ms)
  ok 16 [chromium] › tests\smoke.spec.ts:393:1 › Sales Return detail uses the scoped sale-return projection (2.4s)
  ok 17 [chromium] › tests\smoke.spec.ts:434:1 › Sales Return fallback renders the optional API projection note (2.5s)
  ok 18 [chromium] › tests\smoke.spec.ts:500:1 › purchase detail and summary reports use the purchase read-model contract (4.8s)
  ok 19 [chromium] › tests\smoke.spec.ts:531:1 › purchase return, supplier, and purchase-order report leaves retain mapped navigation (6.8s)
  ok 20 [chromium] › tests\smoke.spec.ts:627:1 › stock report leaves use scoped normalized and historical stock metadata (7.1s)
  ok 21 [chromium] › tests\smoke.spec.ts:677:1 › parity workflow surfaces are reachable for transactions, master data, reports, and maintenance (2.4s)
  ok 22 [chromium] › tests\smoke.spec.ts:711:1 › backup request reports deployment policy status instead of claiming a backup (1.3s)
  ok 23 [chromium] › tests\smoke.spec.ts:735:1 › session monitor displays only the authenticated branch session set (1.3s)
  ok 24 [chromium] › tests\smoke.spec.ts:757:1 › generic catalog fallback pages survive direct SSR navigation (995ms)
  ok 25 [chromium] › tests\smoke.spec.ts:784:1 › critical deep links SSR at HTTP 200 with a visible main surface (707ms)
  ok 26 [chromium] › tests\smoke.spec.ts:807:1 › route-specific maintenance fields replace the generic fallback after interaction (362ms)
  ok 27 [chromium] › tests\smoke.spec.ts:824:1 › Change Items Price submits the captured fields to the canonical maintenance endpoint (870ms)
  ok 28 [chromium] › tests\smoke.spec.ts:856:1 › Lock Item Batches submits the captured batch state to the maintenance endpoint (937ms)
  ok 29 [chromium] › tests\smoke.spec.ts:888:1 › Opening Stock posts a canonical inbound inventory event (1.1s)
  ok 30 [chromium] › tests\smoke.spec.ts:937:1 › captured preference form tabs expose their native legacy layouts after interaction (952ms)
  ok 31 [chromium] › tests\smoke.spec.ts:948:1 › Users list selection reopens the detail form and persists an operator update (965ms)
  ok 32 [chromium] › tests\smoke.spec.ts:980:1 › Item detail exposes and persists the supplier grid (1.0s)
  ok 33 [chromium] › tests\smoke.spec.ts:1011:1 › Groups editor loads permissions and persists the selected permission set (969ms)
```

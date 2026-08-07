# Codebase & Test Analysis: Milestone M1 (Legacy Shell, Workflow & MDI Parity)

## Executive Summary
This analysis evaluates the frontend shell, navigation system, global shortcut key handlers (`Ctrl+Alt+M`, `Ctrl+X`, `Ctrl+Q`), contextual menu catalog (325+ catalog items), visual parity baseline setup at 1936x1048, and associated test coverage within `apps/web`. The codebase demonstrates strong structural adherence to M1 requirements with robust MDI window management, dynamic contextual menu swapping, and Playwright test coverage across major workflows.

---

## 1. Shortcut Key System Analysis

### Handlers & Dispatch Logic
- **`LegacyMenuBar.svelte` (lines 137-151)**:
  - Registers a global `<svelte:window onkeydown={handleKeydown} />`.
  - Constructs key modifier string:
    ```ts
    const modifiers = `${event.ctrlKey ? 'Ctrl+' : ''}${event.altKey ? 'Alt+' : ''}${event.shiftKey ? 'Shift+' : ''}${event.key.toUpperCase()}`;
    ```
  - Looks up shortcut action using `findShortcutAction(menus, modifiers)` in `legacy-menu.ts` (lines 288-304), which performs case-insensitive comparison (`action.shortcut?.toLowerCase() === shortcut.toLowerCase()`).
  - Calls `event.preventDefault()` and invokes `choose(action)`.

### Target Shortcut Key Analysis:
1. **`Ctrl+Alt+M` (Session Monitor)**:
   - Defined in catalog items under `Mana&ge > Session Monitor\tCtrl+Alt+M` (commandId 11190 in base catalog; 13869 in contextual catalog).
   - Resolved by `findShortcutAction` to `Manage > Session Monitor`.
   - `choose(action)` navigates to `/app/manage/session-monitor?legacyPath=...&commandId=...`.
   - **Observation / Redundancy**: `routes/app/legacy/+page.svelte` (lines 33-38) contains a duplicate window keydown listener:
     ```ts
     function handleKeydown(event: KeyboardEvent) {
       if (event.ctrlKey && event.altKey && event.key.toLowerCase() === 'm') {
         event.preventDefault();
         window.location.assign('/app/manage/session-monitor');
       }
     }
     ```
     This triggers a full browser reload (`window.location.assign`) alongside `LegacyMenuBar`'s client-side navigation (`goto`).
2. **`Ctrl+X` (Exit)**:
   - Defined in catalog under `&File > Exit\tCtrl+X` (commandId 10002 / 12675).
   - `hrefFor(['File', 'Exit'])` returns `/`.
   - Pressing `Ctrl+X` triggers `choose(action)` and navigates to `/`.
3. **`Ctrl+Q` (Save And Post)**:
   - Defined in contextual catalog for `pack-purchase` and `cash-sale` under `&File > Save And Post\tCtrl+Q` (commandId 12640).
   - In transaction pages (`routes/app/sales` and `routes/app/purchase/[kind]`), `onCommand={handleMenuCommand}` intercepts `'Save And Post'` and executes `submitSale('posted', 'save-and-post')` or `savePurchase('save-and-post')`.
   - Verified by E2E tests in `phase-cd.spec.ts` (lines 117-178, 606).

---

## 2. Navigation System & MDI Parity Analysis

### Architecture & Components
- **MDI Window Registry (`apps/web/src/lib/legacy-window-registry.ts`)**:
  - Maintains state in `SessionStorage` (`abuzar.legacy-window-registry.v1`).
  - Supports window lifecycle methods: `open()`, `activate()`, `close()`, `clear()`, `command('cascade' | 'tile' | 'layer' | 'arrange' | 'refresh')`.
- **Dynamic Window Menu Injection**:
  - `addWindowShellActions` in `legacy-menu.ts` (lines 168-194) populates shell actions (`Cascade`, `Tile`, `Layer`, `Arrange Icons`, `Refresh`) and currently open window items (`1 Main Window`, `2 Cash Sale`, etc.).
- **MDI Tabs Bar**:
  - Rendered in `LegacyMenuBar.svelte` (lines 206-216), allowing tab switching between active open windows.
- **Route Mapping (`hrefFor` in `legacy-menu.ts`, lines 111-156)**:
  - Maps catalog paths to SvelteKit routes (`/app/purchase/[kind]`, `/app/sales?kind=[kind]`, `/app/report/[kind]`, `/app/master/[kind]`, `/app/preferences`, `/app/maintenance/[kind]`, `/app/manage/[kind]`).
  - Fallback route: `/app/module/[slug]?legacyPath=...&commandId=...` ensures every captured leaf has a deterministic destination without dead clicks.

---

## 3. Contextual Menu Catalog Analysis (325+ Catalog Items)

### Catalog Structure & Context Swapping
- **Data File**: `apps/web/src/lib/legacy-menu-contextual-catalog.ts` contains `contextualLegacyMenuCatalog`.
- **Captured Contexts**:
  1. `pack-purchase`: 325 items, 10 top-level menus
  2. `cash-sale`: 326 items, 10 top-level menus
  3. `item-master`: 314 items, 10 top-level menus
  4. `report-sale-detail`: 306 items, 9 top-level menus
  5. `manage-groups`: 295 items, 9 top-level menus
- **Menu Access & Permission Gating (`applyMenuAccess` in `legacy-menu.ts`, lines 265-286)**:
  - Evaluates user permissions and scopes obtained from `/v1/access`.
  - Disables unauthorized commands (`disabled={action.denied}`) and appends informative tooltips (`aria-disabled="true"`, `title="Permission denied"` or `title="No unambiguous legacy-right mapping"`).

---

## 4. Visual Parity & Zero-Pixel Baseline Setup (1936x1048)

### Visual Remediation Setup (`apps/web/tests/visual-remediation.spec.ts`)
- Target canvas viewport: `{ width: 1936, height: 1048 }`.
- Verified 8 core surfaces: `customer`, `item`, `cashSale`, `packPurchase`, `purchaseReturn`, `dailySalesDetail`, `preferences`, `groups`.
- Enforces strict constraints:
  - Viewport exact sizing (`1936x1048`).
  - Page scroll height constraint: `scrollHeight <= 1048` (no page overflow).
  - Explicit element box heights (e.g. `.legacy-transaction-grid-wrap` height = 563px for purchase windows; preferences table width = 590px).
  - Zero hidden interactive inputs/buttons (`hiddenInteractiveCount === 0`).
  - Zero substrate background leaks (`hasSubstrateBackground === false`).

### Baseline Raster Image Comparison Finding
- **Observation**: In `visual-remediation.spec.ts` (lines 88-93), pixel-by-pixel image diff comparison returns status `'not-compared'`:
  - `exception: 'No fresh independent legacy capture was available in this run; existing 1922x970/1536x972 substrates are not used as acceptance baselines.'`
  - Existing reference raster captures in `build/parity/` were recorded at legacy resolutions (1922x970 and 1536x972) rather than 1936x1048.
  - While visual layout bounding, CSS geometry, zero-scroll containment, and stateful UI are strictly tested and passing, true zero-pixel automated image diff comparison against 1936x1048 reference rasters requires updating baseline reference rasters to 1936x1048.

---

## 5. Playwright E2E and Unit Test Coverage Evaluation

### Test Inventory Across `apps/web/tests/`:
1. `smoke.spec.ts`: 19 tests covering shell rendering, Change User dialog, `Ctrl+Alt+M` shortcut, sales list tab, nested report menu, print preview/exports, purchase & stock reports, maintenance endpoints, session monitor, and SSR deep links.
2. `visual-remediation.spec.ts`: 1 test verifying 8 surfaces at 1936x1048 layout geometry & taking screenshots.
3. `phase-b.spec.ts`: 5 tests verifying UTF-8 encoding integrity, 221 catalog leaf route mappings, Open Sale Return kinds, and title bar formatting with live session clock.
4. `phase-cd.spec.ts`: 11 tests verifying contextual catalog structure (325 items), menu swapping, shortcut keys (`Ctrl+Q`, `Ctrl+I`, `Ctrl+R`), window registry, MDI tabs, cash sale `Ctrl+Q` Save & Post, canonical sales lifecycle, PO stock/GL neutrality, auto batch generation (`Ctrl+B`), GST/expense windows, and purchase invoice/return population.
5. `phase-f.spec.ts`: 5 tests covering item master search/details, supplier grid, customer/supplier CRUD, API error handling, and auxiliary masters.
6. `phase-q.spec.ts`: 1 test covering 7 representative financial and labeled fallback report leaves.
7. `phase-r.spec.ts`: 6 tests covering group rights matrix editing, group price setting, group scope leaves, contextual command permission gating, and report menu permission filtering.
8. `preferences.spec.ts`: Form tabs and preference persistence tests.
9. `purchase-canonical.spec.ts`: Canonical purchase document lifecycles and voiding.
10. `sales-canonical.spec.ts`: Canonical sales document lifecycles, pricing engine preview, and voiding.

### Test Coverage Assessment:
- **Shortcut Keys**: E2E test coverage exists for `Ctrl+Alt+M` (in `smoke.spec.ts`), `Ctrl+Q` (in `phase-cd.spec.ts`), and `Ctrl+B` (in `phase-cd.spec.ts`).
- **MDI & Navigation**: E2E test coverage exists for MDI window registry, window menu dynamic listing, MDI tab switching, and catalog leaf route resolution.
- **Contextual Catalog**: Verified 325-item count, 5 window contexts, and menu permission gating.

---

## 6. Summary of Findings & Non-Conformance Items

1. **Redundant Window Keydown Listener in `routes/app/legacy/+page.svelte`**:
   - `+page.svelte` registers `handleKeydown` for `Ctrl+Alt+M` using `window.location.assign('/app/manage/session-monitor')`, which overrides SvelteKit SPA navigation (`goto`) performed by `LegacyMenuBar.svelte`.
2. **`Ctrl+Q` Default Navigation Fallback when `onCommand` Unhandled**:
   - If `Ctrl+Q` is pressed on a page that does not handle `Save And Post` via `onCommand`, `choose` falls back to `hrefFor(['File', 'Save And Post'])` -> `/app/module/save-and-post`.
3. **Visual Raster Image Baseline Resolution Mismatch**:
   - `visual-remediation.spec.ts` marks visual image diff as `'not-compared'` because historical substrate rasters in `build/parity/` are 1922x970 / 1536x972 instead of 1936x1048. Geometry, bounds, and layout containment pass, but pixel-diff comparison against 1936x1048 reference images is bypassed.

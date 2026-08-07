# Milestone M1 Analysis Report — Legacy Shell, Workflow & MDI Parity

**Date**: 2026-08-07  
**Explorer**: Explorer 1 (Milestone M1)  
**Target Paths**: `apps/web/src/lib/` (`legacy-window-registry.ts`, `LegacyWorkflowSurface.svelte`, `LegacyMenuBar.svelte`, `legacy-menu.ts`, `legacy-menu-catalog.ts`, `legacy-menu-contextual-catalog.ts`, `api.ts`), `apps/web/src/routes/`, `services/api/internal/httpapi/`, `apps/web/tests/`

---

## Executive Summary

A comprehensive, read-only investigation was conducted into the codebase and test suite for **Milestone M1 (Legacy Shell, Workflow & MDI Parity)**. The investigation examined the MDI window registry, tab management, cascade/tile layout controls, SessionStorage state retention, navigation, keyboard shortcuts, contextual menus, modal dialogs, Change User re-authentication flow, Playwright E2E test coverage, and unit test coverage.

### Key Findings Overview
1. **Window Registry Storage Key Defect**: `legacyWindowRegistry` filters out stored windows during `restoreRegistry()` if their `context` is not in a hardcoded list of 6 values (`['base', 'pack-purchase', 'cash-sale', 'item-master', 'report-sale-detail', 'manage-groups']`). Open windows for other routes (e.g. `purchase-return`, `credit-sale`, `customer-master`, `preferences`, etc.) are dropped from `sessionStorage` on page reload.
2. **Missing `close(id)` Store Method**: `createLegacyWindowRegistry` in `legacy-window-registry.ts` does not expose a `close(id)` function, preventing MDI tabs from being closed individually via store actions.
3. **Missing `POST /v1/auth/change-user` Endpoint**: `PROJECT.md` (line 49) and `SCOPE.md` (line 26) list `POST /v1/auth/change-user` as an interface contract for session re-authentication. However, `services/api/internal/httpapi/server.go` and `auth.go` do NOT register or implement `/v1/auth/change-user`. The web frontend instead performs `api.logout()` (`POST /v1/auth/logout`) and redirects to `/login?changeUser=1`.
4. **Stubbed Visual Zero-Pixel Parity**: `visual-remediation.spec.ts` verifies viewport bounds at 1936x1048 but explicitly stubs pixel comparison (`status: 'not-compared'`). Automated Playwright snapshot matching (`toHaveScreenshot()`) is not implemented.
5. **Test Coverage Gaps**: No Playwright tests cover `Ctrl+X` or `Ctrl+Q` keyboard shortcuts, nor `sessionStorage` window restoration across page reloads. No unit test framework (e.g. Vitest) or unit test files exist under `apps/web`.

---

## Detailed Investigation Findings

### 1. Legacy Shell & MDI Window Registry

#### 1.1 SessionStorage State Retention Defect
- **File**: `apps/web/src/lib/legacy-window-registry.ts`
- **Lines**: 13–20, 37–38, 47–49
- **Observation**:
  ```ts
  const legacyWindowContexts = new Set<LegacyOpenWindow['context']>([
    'base',
    'pack-purchase',
    'cash-sale',
    'item-master',
    'report-sale-detail',
    'manage-groups'
  ]);

  function isStoredWindow(value: unknown): value is LegacyOpenWindow {
    ...
    return typeof candidate.context === 'string'
      && legacyWindowContexts.has(candidate.context as LegacyOpenWindow['context']);
  }
  ```
- **Analysis**:
  When a user opens windows from other application routes (such as `/app/purchase/return`, `/app/sales?kind=credit`, `/app/master/customer`, `/app/preferences`, `/app/maintenance/change-items-price`, etc.), these windows are added to the in-memory registry. However, when `restoreRegistry()` is executed upon page reload or browser restart, `isStoredWindow` rejects any window whose context is not in `legacyWindowContexts`. Consequently, all open windows from non-listed routes are purged from `sessionStorage` on reload.

#### 1.2 Missing Window Removal (`close`) API
- **File**: `apps/web/src/lib/legacy-window-registry.ts`
- **Lines**: 75–105
- **Observation**:
  ```ts
  export function createLegacyWindowRegistry(options: { persist?: boolean } = {}): {
    subscribe: Readable<LegacyWindowRegistry>['subscribe'];
    open: (window: LegacyOpenWindow) => void;
    activate: (id: string) => void;
    command: (command: 'cascade' | 'tile' | 'layer' | 'arrange' | 'refresh') => void;
    clear: () => void;
    snapshot: () => LegacyWindowRegistry;
  }
  ```
- **Analysis**:
  The registry store provides `open`, `activate`, `command`, `clear`, and `snapshot`, but lacks a `close(id: string)` method. MDI tabs in `.legacy-mdi-tabs` cannot be individually closed by the user from the tab bar or window controls.

#### 1.3 MDI Layout Commands & Visual Windowing
- **Files**: `apps/web/src/lib/legacy-window-registry.ts` (line 4), `apps/web/src/lib/LegacyMenuBar.svelte` (line 206)
- **Observation**:
  `WindowLayout` types include `'cascade' | 'tile' | 'layer' | 'arrange'`. In `LegacyMenuBar.svelte`, layout commands set `data-layout` on `.legacy-mdi-tabs`.
- **Analysis**:
  Layout commands update tab CSS data attributes, but child windows are rendered as full-viewport single active surfaces rather than floating, tileable, or cascadeable desktop MDI window frames within the workspace container.

---

### 2. Navigation, Keyboard Shortcuts & Context Menus

#### 2.1 Global Shortcut Key Handling
- **File**: `apps/web/src/lib/LegacyMenuBar.svelte`
- **Lines**: 137–151
- **Observation**:
  `handleKeydown(event)` builds a modifier string `${ctrlKey}${altKey}${shiftKey}${key.toUpperCase()}` and looks up `findShortcutAction(menus, modifiers)`.
  - Shortcut `Ctrl+Alt+M` maps to `Manage > Session Monitor`.
  - Shortcut `Ctrl+X` maps to `File > Exit`.
  - Shortcut `Ctrl+Q` maps to `File > Save And Post` (in contextual menus for `pack-purchase` and `cash-sale`).
- **Analysis**:
  - In `apps/web/src/routes/app/legacy/+page.svelte` (lines 33–38), there is a duplicate hardcoded `handleKeydown` for `Ctrl+Alt+M`:
    ```ts
    if (event.ctrlKey && event.altKey && event.key.toLowerCase() === 'm') {
      event.preventDefault();
      window.location.assign('/app/manage/session-monitor');
    }
    ```
    This duplicates the shortcut handling already present in `LegacyMenuBar.svelte`.

#### 2.2 Contextual Menu Tree (325+ Items)
- **Files**: `apps/web/src/lib/legacy-menu-contextual-catalog.ts`, `apps/web/src/lib/legacy-menu.ts`
- **Observation**:
  `contextualLegacyMenuCatalog` contains versioned contextual menu items (`2026-08-06-contextual-1`):
  - `pack-purchase`: 325 items
  - `cash-sale`: 326 items
  - `item-master`: 325 items
  - `report-sale-detail`: 325 items
  - `manage-groups`: 325 items
- **Analysis**:
  `buildLegacyMenusForContext(context)` dynamically selects and builds the catalog for contextual surfaces. `applyMenuAccess()` evaluates `access.permissions` and `access.scopes` from `POST /v1/access`, disabling or marking denied items accordingly.

---

### 3. Modal Dialogs & Change User Flow

#### 3.1 Interface Contract Mismatch for Change User Flow
- **Files**: `PROJECT.md` (line 49), `SCOPE.md` (line 26), `services/api/internal/httpapi/server.go`, `services/api/internal/httpapi/auth.go`, `apps/web/src/lib/LegacyMenuBar.svelte` (lines 61-66)
- **Observation**:
  - `PROJECT.md` line 49 states: `Session Auth: POST /v1/auth/login, POST /v1/auth/change-user`.
  - `SCOPE.md` line 26 states: `Session Auth & Change User: POST /v1/auth/login, POST /v1/auth/change-user`.
  - In `services/api/internal/httpapi/server.go` (lines 45-48):
    ```go
    mux.HandleFunc("POST /v1/auth/login", s.login)
    mux.HandleFunc("POST /v1/auth/logout", s.logout)
    mux.Handle("POST /v1/auth/change-password", s.authenticated(http.HandlerFunc(s.changePassword)))
    ```
  - `POST /v1/auth/change-user` is **NOT REGISTERED** on the Go API server.
  - In `apps/web/src/lib/api.ts`, no `changeUser` method exists.
  - In `LegacyMenuBar.svelte`:
    ```ts
    async function confirmChangeUser() {
      enableChangeUserInteractive();
      legacyWindowRegistry.clear();
      await api.logout().catch(() => undefined);
      window.location.assign('/login?changeUser=1');
    }
    ```
- **Analysis**:
  The interface specification in `PROJECT.md` and `SCOPE.md` mandates `POST /v1/auth/change-user` for session re-authentication. The backend lacks this endpoint, and the web frontend works around it by calling `/v1/auth/logout` followed by a page redirect to `/login?changeUser=1`.

#### 3.2 Modal Dialogs Implementation
- **Files**: `LegacyMenuBar.svelte` (Change User dialog), `LegacyWorkflowSurface.svelte` (Backup Database, Backup Device, Backup Information dialogs), `login/+page.svelte` (User Validation, Database Problem dialogs).
- **Analysis**:
  Modal dialogs are properly rendered with ARIA attributes (`role="alertdialog"`, `aria-modal="true"`), styling classes, backdrop overlay, and keyboard dismiss hooks (`Escape`).

---

### 4. Visual Comparison & Zero-Pixel Parity

#### 4.1 Visual Baseline Raster Comparison
- **File**: `apps/web/tests/visual-remediation.spec.ts`
- **Lines**: 88–93
- **Observation**:
  ```ts
  comparison: {
    status: 'not-compared',
    differentPixels: null,
    maxChannelDelta: null,
    exception: 'No fresh independent legacy capture was available in this run; existing 1922x970/1536x972 substrates are not used as acceptance baselines.'
  }
  ```
- **Analysis**:
  Requirement R1 in `ORIGINAL_REQUEST.md` ("Maintain zero-pixel-difference visual comparisons where baseline rasters exist") and Feature 4 in `PROJECT.md` ("Baseline raster comparison at 1936x1048") are currently not verified by automated image diffing. `visual-remediation.spec.ts` checks DOM element bounding dimensions (e.g. 1936x1048 viewport, 563px grid height), but skips raster comparison.

---

### 5. Playwright E2E and Unit Test Coverage

#### 5.1 Playwright E2E Suite Status
- **Location**: `apps/web/tests/` (10 spec files)
- **Covered Scenarios**:
  - `smoke.spec.ts`: Landing page, shared workspace shell, legacy main window, Change User confirmation dialog (main shell & child window), `Ctrl+Alt+M` shortcut, offline sales workflow, sales list tab, nested report menu navigation, daily sales detail report, fallback report notes, purchase report definitions, stock report definitions, parity workflow surfaces, backup database dialog, session monitor, SSR deep links, route-specific maintenance forms, preferences tabs.
  - `visual-remediation.spec.ts`: 1936x1048 viewport bounds and scrollHeight assertions across 8 parity surfaces.
  - `preferences.spec.ts`, `purchase-canonical.spec.ts`, `sales-canonical.spec.ts`, `phase-b.spec.ts`, `phase-cd.spec.ts`, `phase-f.spec.ts`, `phase-q.spec.ts`, `phase-r.spec.ts`: Specific functional workflows.

#### 5.2 Test Coverage Gaps
1. **Shortcut Keys**: No Playwright test verifies `Ctrl+X` (File > Exit) or `Ctrl+Q` (File > Save And Post).
2. **SessionStorage Persistence**: No E2E test verifies `sessionStorage` state retention or restoration of open MDI windows across page reloads.
3. **Zero-Pixel Image Snapshots**: No automated Playwright screenshot assertions (`toHaveScreenshot()`) exist for visual regression detection.
4. **Unit Test Coverage**: `apps/web/package.json` only configures Playwright for `pnpm test`. There are no unit test runners (e.g. Vitest) or unit test files (`*.test.ts`) for `legacy-window-registry.ts`, `legacy-menu.ts`, or utility modules.

---

## Summary Table of Issues Identified

| # | Subsystem | Issue Description | Severity | Impact | Affected Files |
|---|-----------|-------------------|----------|--------|----------------|
| 1 | Window Registry | `restoreRegistry()` filters out windows whose context is not in hardcoded 6-item set | High | MDI window state lost on reload for non-listed routes | `apps/web/src/lib/legacy-window-registry.ts:13-20` |
| 2 | Window Registry | Store lacks `close(id)` method | Medium | Cannot close individual MDI tabs from store | `apps/web/src/lib/legacy-window-registry.ts:75-105` |
| 3 | Change User Flow | Missing `POST /v1/auth/change-user` endpoint in Go API backend | High | Interface contract mismatch with PROJECT.md & SCOPE.md | `services/api/internal/httpapi/server.go`, `auth.go` |
| 4 | Visual Parity | Raster pixel comparison is stubbed out (`status: 'not-compared'`) | Medium | Zero-pixel visual parity is not automatically verified | `apps/web/tests/visual-remediation.spec.ts:88-93` |
| 5 | Navigation & Shortcuts | Duplicate `Ctrl+Alt+M` keydown listener in `legacy/+page.svelte` | Low | Redundant handler alongside `LegacyMenuBar.svelte` | `apps/web/src/routes/app/legacy/+page.svelte:33-38` |
| 6 | Test Coverage | Missing E2E tests for `Ctrl+X`, `Ctrl+Q`, and `sessionStorage` window restoration | Medium | Regression risk for shortcuts & window persistence | `apps/web/tests/smoke.spec.ts` |
| 7 | Test Coverage | Zero unit test files or unit test runner configured for `apps/web` | Medium | Unit-level verification missing for shell logic | `apps/web/package.json` |

---

## Verification Commands for Findings

```powershell
# 1. Type check frontend web package
pnpm --filter @abuzar/web check

# 2. Run Playwright E2E tests
pnpm --filter @abuzar/web test -- --workers=1 --retries=0

# 3. Check Go API server routes and packages
go vet ./services/api/...
```

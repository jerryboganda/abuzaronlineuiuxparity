# Handoff Report — Explorer 1 (Milestone M1)

## 1. Observation

1. **Window Registry Context Filtering**: In `apps/web/src/lib/legacy-window-registry.ts`, lines 13–20 and 37–38:
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
   `isStoredWindow` filters out any stored window whose `context` is not present in `legacyWindowContexts` during `restoreRegistry()`.

2. **Missing `close(id)` Store Method**: In `apps/web/src/lib/legacy-window-registry.ts`, lines 75–105, `createLegacyWindowRegistry` exports `open`, `activate`, `command`, `clear`, `snapshot`, but does NOT export a `close(id)` function.

3. **Missing `/v1/auth/change-user` API Endpoint**:
   - `PROJECT.md` line 49 and `SCOPE.md` line 26 specify: `Session Auth & Change User: POST /v1/auth/login, POST /v1/auth/change-user`.
   - In `services/api/internal/httpapi/server.go` lines 45–48:
     ```go
     mux.HandleFunc("POST /v1/auth/login", s.login)
     mux.HandleFunc("POST /v1/auth/logout", s.logout)
     mux.Handle("POST /v1/auth/change-password", s.authenticated(http.HandlerFunc(s.changePassword)))
     ```
     `/v1/auth/change-user` is not registered on the Go HTTP server.
   - In `apps/web/src/lib/LegacyMenuBar.svelte` lines 61–66:
     ```ts
     async function confirmChangeUser() {
       enableChangeUserInteractive();
       legacyWindowRegistry.clear();
       await api.logout().catch(() => undefined);
       window.location.assign('/login?changeUser=1');
     }
     ```

4. **Stubbed Visual Pixel Comparison**: In `apps/web/tests/visual-remediation.spec.ts` lines 88–93:
   ```ts
   comparison: {
     status: 'not-compared',
     differentPixels: null,
     maxChannelDelta: null,
     exception: 'No fresh independent legacy capture was available in this run; existing 1922x970/1536x972 substrates are not used as acceptance baselines.'
   }
   ```

5. **Test Suite Coverage**: Inspection of `apps/web/package.json` line 11 (`"test": "playwright test"`) and `apps/web/tests/` shows 10 Playwright spec files and 0 unit test files (`*.test.ts`). No E2E tests exist for `Ctrl+X` or `Ctrl+Q` shortcuts or `sessionStorage` window restoration across reloads.

---

## 2. Logic Chain

1. **Context Filtering Defect**:
   - Observation 1 shows `legacyWindowContexts` only allows 6 specific context strings.
   - When users open other valid window types (e.g. `purchase-return`, `credit-sale`, `customer-master`, `preferences`), they are assigned context strings matching those routes.
   - Upon page reload, `restoreRegistry()` passes each stored window to `isStoredWindow()`.
   - Rejection by `legacyWindowContexts.has(...)` drops those open windows from state, breaking SessionStorage state retention requirement M1.1.

2. **Missing `close(id)` Method**:
   - Observation 2 shows `createLegacyWindowRegistry` lacks a `close(id)` method.
   - Without a `close` action on the store, UI components cannot trigger single-window removal from `legacyWindowRegistry.windows`.

3. **Change User Endpoint Mismatch**:
   - Observation 3 shows `PROJECT.md` and `SCOPE.md` define `POST /v1/auth/change-user` as part of the interface contract.
   - Observation 3 shows `server.go` registers only `POST /v1/auth/login`, `POST /v1/auth/logout`, and `POST /v1/auth/change-password`.
   - `LegacyMenuBar.svelte` works around this missing endpoint by invoking `api.logout()` and navigating to `/login?changeUser=1`.
   - Therefore, the API contract is incomplete relative to `PROJECT.md` & `SCOPE.md`.

4. **Visual Zero-Pixel Parity Gap**:
   - Observation 4 shows `visual-remediation.spec.ts` explicitly sets `status: 'not-compared'`.
   - Requirement R1 in `ORIGINAL_REQUEST.md` and Feature 4 in `PROJECT.md` require zero-pixel raster comparison.
   - Therefore, zero-pixel parity is currently unverified by automated testing.

5. **Test Coverage Gap**:
   - Observation 5 shows no Playwright test verifies `Ctrl+X`, `Ctrl+Q`, or `sessionStorage` restoration across reloads, and no unit test runner/files exist in `apps/web`.
   - This leaves shortcut execution and window registry restoration vulnerable to silent regression.

---

## 3. Caveats

- **Read-Only Scope**: Per dispatch instructions, no source code changes were made. All identified defects and contract gaps are documented for implementers.
- **Visual Baselines**: No baseline `.png` reference raster images were located in `apps/web` or `docs/` for 1936x1048 comparison; tests rely on DOM layout size bounds.

---

## 4. Conclusion

The core shell structure, contextual menus (325+ catalog items), keyboard shortcuts, dialogs, and basic window registry management are functional in `apps/web`. However, 4 primary non-conformance items and defects must be addressed in Milestone M1:
1. **Window Registry Context Restoration Defect**: `legacyWindowContexts` set in `legacy-window-registry.ts` must accept all valid route contexts (or dynamic strings) so open windows persist across reloads.
2. **Missing `close(id)` Method**: `createLegacyWindowRegistry` needs a `close(id)` method to allow individual tab closing.
3. **Change User Backend API Endpoint**: `POST /v1/auth/change-user` needs to be implemented in `services/api/internal/httpapi/` or the contract in `PROJECT.md` / `SCOPE.md` updated if `POST /v1/auth/logout` is the approved pattern.
4. **Test Suite Expansion**: Playwright tests must be added for `Ctrl+X`, `Ctrl+Q`, and `sessionStorage` window restoration across page reloads.

---

## 5. Verification Method

To independently verify the observations and analysis:

1. **Verify Window Registry Context Filtering**:
   - Inspect `apps/web/src/lib/legacy-window-registry.ts` lines 13–20 and 37–38 using `view_file`.
   - Confirm `legacyWindowContexts` contains only 6 values and is checked by `isStoredWindow()`.

2. **Verify Change User Endpoint Mismatch**:
   - Inspect `services/api/internal/httpapi/server.go` lines 45–48 using `view_file`.
   - Confirm `/v1/auth/change-user` is absent from route registrations.
   - Inspect `apps/web/src/lib/LegacyMenuBar.svelte` lines 61–66 to see fallback to `api.logout()`.

3. **Verify Visual Comparison Status**:
   - Inspect `apps/web/tests/visual-remediation.spec.ts` lines 88–93 using `view_file`.
   - Confirm `status: 'not-compared'`.

4. **Project Test Commands**:
   - Run web check: `pnpm --filter @abuzar/web check`
   - Run browser E2E test suite: `pnpm --filter @abuzar/web test -- --workers=1 --retries=0`
   - Run Go vet: `go vet ./services/api/...`

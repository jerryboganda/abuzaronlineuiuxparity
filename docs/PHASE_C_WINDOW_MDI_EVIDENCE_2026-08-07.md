# Phase C Window/MDI evidence - 2026-08-07

## Scope

The captured PowerBuilder shell exposes a Window menu and an observed list of
open document windows. The rebuild already had a typed window registry and
Window-menu actions, but document actions used full-page navigation. That
discarded the registry and made the second-window workflow unreliable.

This slice keeps the existing captured shell/raster structure and changes only
the navigation/state boundary:

- `LegacyMenuBar.svelte` uses SvelteKit client navigation for internal legacy
  destinations and Window-menu/tab activation, with a full navigation fallback
  if client navigation cannot load a route.
- `legacy-window-registry.ts` persists only validated same-origin window
  descriptors in tab-scoped `sessionStorage`, caps restored entries at 32, and
  ignores malformed storage without interrupting the shell.
- The shared `LegacyMenuBar` is now mounted in the maintenance/manage workflow
  surface, Preferences, and the generic catalog module workbench. Each child
  passes a stable window id and internal href, so the same File/Window chrome
  and registry behavior is available on direct child-window routes.
- The menu bar reacts when SvelteKit reuses a child page for a different
  workflow kind. Cash Sale to Credit Sale navigation now registers and
  activates the Credit Sale window instead of leaving the Cash Sale entry
  active. Each sales workflow keeps its own form, lookup, line, pricing,
  history, document identity/version, and idempotency state. Opening Credit Sale
  starts a fresh credit form, while reactivating Cash Sale restores its exact
  in-memory work. Canonical customers load when the transition first enters a
  credit workflow.
- Dynamic Purchase routes now apply the same window boundary. Pack Purchase,
  Purchase Return, Opening Purchase, Loose Purchase, and Purchase Order retain
  independent form, line, batch/allocation, history, modal, document/version,
  and idempotency state. First activation starts fresh; Window-menu activation
  restores that workflow's in-memory draft. Purchase lookup, history,
  templates, and batch availability discard stale cross-window responses.
  Workflow revision guards also prevent late document, void, pricing, or
  history responses from restoring state after the operator has navigated.
  Active submissions block SvelteKit navigation until the API result is
  acknowledged, preventing an accepted sale from becoming an abandoned form
  that could later be posted again under a fresh idempotency key. Contextual
  commands are blocked while the submission is active, the menu bar suppresses
  its hard-navigation fallback for that intentional lock, late history
  responses are revision-guarded and exact-kind checked, and cash-drawer
  activation remains best-effort without extending the document lock. Item,
  stock, history, and pricing requests use per-request ownership so older
  responses cannot overwrite newer work; restored rows resume interrupted
  stock loading. Posting cancels the debounce, locks form editing, and requires
  an authoritative pricing response before constructing the command.
- The existing zero/one/n-window unit coverage remains unchanged.

## Verification

Commands run from `D:\ABUZAR\AbuzarNext`:

| Check | Result |
|---|---|
| `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| `pnpm --filter @abuzar/web test -- --workers=1 --grep "Window menu preserves MDI entries"` | Passed: 1 browser test |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --grep "parity workflow surfaces\|generic catalog fallback pages\|session monitor displays"` | Passed: 3 browser tests |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=1 --reporter=line` | The latest post-change full run passed 77/77 browser test cases serially with no retry |
| `pnpm --filter @abuzar/web test -- sales-canonical.spec.ts --grep "client-side navigation from cash to credit" --workers=1 --retries=0` | Added for the same-page workflow transition. Not executed in the current worktree because the Playwright binary is not installed; no dependency install or broad CI run was performed. |

The browser test proves:

1. Main Window opens Cash Sale through the captured base Sales menu.
2. Cash Sale retains Main Window and Cash Sale in its contextual Window menu.
3. Window-menu activation returns to Main Window without losing Cash Sale.
4. A full reload restores the two-window registry, and the Cash Sale tab
   activates the retained route.
5. Preferences, maintenance/manage workflow surfaces, and direct generic
   module routes expose the shared File and Window menus.
6. The pending focused regression verifies that Cash Sale to Credit Sale
   client navigation is blocked while Cash Sale posting is pending, then
   rejects a concurrent contextual Post, fetches canonical customers once,
   opens a fresh Credit Sale form without the Cash Sale identity or rows, and
   retains separate Cash Sale and Credit Sale entries in the Window menu.
   Reactivating Cash Sale restores its canonical item and posted invoice number.

## Remaining acceptance boundary

This is not approval of full PowerBuilder MDI parity. The implementation still
needs approved legacy-vs-new captures and operator review for cascade/tile/layer
geometry, focus/keyboard traversal, close/minimize/restore semantics, and every
contextual Window state. The current web child surfaces intentionally hide the
tab/status rows during their untouched raster mode, so a physical desktop
walkthrough is still required before claiming pixel or full MDI parity. Generic
module pages remain compatibility workbenches for catalog leaves whose true
PowerBuilder workflow has not yet been reconstructed.

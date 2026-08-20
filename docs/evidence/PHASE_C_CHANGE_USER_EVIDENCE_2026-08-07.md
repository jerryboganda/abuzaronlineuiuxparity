# Phase C Change User evidence - 2026-08-07

## Scope

The captured legacy workflow includes a `Change User` confirmation dialog. The
main Svelte shell already contained the exact untouched-state dialog surface,
but its File-menu command previously navigated directly to login and bypassed
that state.

The shared `LegacyMenuBar` now owns the captured dialog for the base shell and
every contextual child window. `No` closes the dialog and retains the current
window; `Yes` is the only path that navigates to `/login?changeUser=1`. This
keeps the confirmation behavior identical across transaction, master, report,
maintenance, and manage routes. Confirmed change-user navigation also clears
the tab-scoped persisted MDI registry so the next operator cannot inherit the
previous session's open-window list, and requests server-session invalidation
before reaching the login route.

## Verification

Commands run from `D:\ABUZAR\AbuzarNext`:

| Check | Result |
|---|---|
| `pnpm --filter @abuzar/web check` | Passed: 0 errors, 0 warnings |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --grep "Change User"` | Passed: 2 browser tests (base and report child window) |
| `pnpm --filter @abuzar/web test -- --workers=1 --retries=1 --reporter=line` | Latest post-change run passed 77/77 browser test cases serially with no retry |

The focused browser tests prove the dialog is visible with the captured
`legacy-change-user-captured` baseline class in both the base shell and a
report child window, cancel remains on the current window, and confirmation
requests `/v1/auth/logout`, reaches the login route, and leaves an empty
persisted MDI registry.

## Remaining acceptance boundary

This closes the captured command transition across the rendered menu contexts.
The login form, session invalidation, operator UAT, and approved contextual
raster/focus review still require external acceptance evidence.

# Scope: Milestone M1 — Legacy Shell, Workflow & MDI Parity

## Architecture
- Frontend: `apps/web` (SvelteKit)
- Key Modules:
  - MDI window registry, tab management, cascade/tile layout controls (`apps/web/src/lib/components/shell/*`, `SessionStorage` state)
  - Navigation, global keyboard shortcuts (Ctrl+Alt+M, Ctrl+X, Ctrl+Q), contextual menus (325+ catalog items)
  - Modal dialog components, Change User flow & re-authentication (`apps/web/src/lib/components/modals/*`)
  - Visual baseline raster comparison (1936x1048 target canvas, pixel parity check)

## Feature Inventory (M1 Scope)
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | Legacy Shell & Window Management | MDI window registry, tabs, cascade/tile layout, SessionStorage state | M1 | PROJECT.md |
| 2 | Navigation, Shortcut Keys & Context Menus | Ctrl+Alt+M, Ctrl+X, Ctrl+Q, 325+ contextual menu catalog items | M1 | PROJECT.md |
| 3 | Modal Dialogs & Change User Flow | Change user modal dialog, session re-authentication, confirmation dialogs | M1 | PROJECT.md |
| 4 | Visual Comparison & Zero-Pixel Parity | Baseline raster comparison at 1936x1048, stateful interactive UI | M1 | PROJECT.md |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | M1: Legacy Shell, Workflow & MDI Parity | Shell, MDI window registry, tabs, layout controls, shortcuts, contextual menus, change user modal | none | IN_PROGRESS |

## Interface Contracts
### Web (`apps/web`) ↔ API (`services/api`)
- Session Auth & Change User: `POST /v1/auth/login`, `POST /v1/auth/change-user`
- MDI / Menu: Legacy window registry state in SessionStorage; menu catalog from `legacy-menu-contextual-catalog.ts` or equivalent web lib.

## Code Layout (M1 Affected Paths)
- `apps/web/src/lib/components/shell/`
- `apps/web/src/lib/components/modals/`
- `apps/web/src/lib/stores/` or state managers for MDI window registry
- `apps/web/tests/` (Playwright E2E shell/MDI/navigation/visual parity tests)

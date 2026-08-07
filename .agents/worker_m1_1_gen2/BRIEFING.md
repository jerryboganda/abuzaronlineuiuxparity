# BRIEFING — 2026-08-07T08:10:04Z

## Mission
Fix Legacy Shell window registry persistence & closing, remove duplicate Ctrl+Alt+M listener, implement POST /v1/auth/change-user in Go API, and expand Playwright E2E tests.

## 🔒 My Identity
- Archetype: worker
- Roles: implementer, qa, specialist
- Working directory: d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2
- Original parent: b22847a9-42ba-42d8-826a-bc428f5341de
- Milestone: M1 (Legacy Shell, Workflow & MDI Parity)

## 🔒 Key Constraints
- DO NOT CHEAT. All implementations must be genuine.
- Minimal change principle.
- Verify build & tests before completing.

## Current Parent
- Conversation ID: b22847a9-42ba-42d8-826a-bc428f5341de
- Updated: 2026-08-07T08:10:04Z

## Task Summary
- **What to build**:
  1. Fix `apps/web/src/lib/legacy-window-registry.ts`: restore all valid window types across reloads, add `close(id: string)` method.
  2. Fix `apps/web/src/routes/app/legacy/+page.svelte`: remove duplicate Ctrl+Alt+M keydown window.location.assign listener.
  3. Fix Go API (`services/api/internal/httpapi/`): register and implement `POST /v1/auth/change-user`.
  4. Enhance E2E tests (`apps/web/tests/`): Playwright tests for Ctrl+X, Ctrl+Q, MDI tab close(), SessionStorage restoration.
  5. Run checks & tests: web check, web build, go vet, web test.
  6. Write handoff report in `d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2\handoff.md`.
- **Success criteria**: All checks and tests pass, functionality works genuine without hardcoding or shortcuts.

## Change Tracker
- **Files modified**: None yet
- **Build status**: Pending
- **Pending issues**: None

## Quality Status
- **Build/test result**: Pending
- **Lint status**: Pending
- **Tests added/modified**: Pending

## Loaded Skills
- None

## Key Decisions Made
- Initial setup completed.

## Artifact Index
- `d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2\DISPATCH.md` — Dispatch prompt instructions
- `d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2\BRIEFING.md` — Briefing document
- `d:\ABUZAR\AbuzarNext\.agents\worker_m1_1_gen2\progress.md` — Progress tracker

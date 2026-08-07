# Original User Request

## Initial Request — 2026-08-07T02:42:32Z

Complete the AbuzarNext SvelteKit/Go/PostgreSQL rebuild with true legacy workflow, functionality, and UI/UX parity across the captured PowerBuilder catalog, backed by verified tests and documented acceptance evidence.

Working directory: d:\ABUZAR\AbuzarNext
Integrity mode: development

## Requirements

### R1. Legacy Shell, Workflow & Window/MDI Parity
Achieve pixel and workflow parity across captured PowerBuilder catalog windows (shell, MDI window management, navigation, shortcut keys, modal dialogs, and change-user flows). Maintain zero-pixel-difference visual comparisons where baseline rasters exist and stateful UI behavior for interactive operations.

### R2. Data Import, Schema & Bookkeeping Reconciliation
Reconcile source data mappings between legacy PowerBuilder SQL Server structures and the PostgreSQL target schema. Ensure tenant/branch isolation, resolve open migration line exceptions and tax ambiguities, and enforce audit bookkeeping across all transaction tables.

### R3. Pricing Policy, Stock Balance & Financial Engine Parity
Ensure exact-decimal calculation for sale/purchase pricing engines, 10-tier SalePrice selection, discount/tax rules, stock balance projections (StockReport), back-date snapshots, and historical GL ledger projections (VirtualGl).

### R4. Report Engine & Hardware Integration Standard
Validate the 151 catalog report definitions with full format parameterization, print-preview features (ruler, zoom, loaded-row paging, letterhead), export formats, and hardware interface abstractions (ESC/POS receipt printing, cash drawer pulse, barcode generation).

### R5. Comprehensive Verification & Handoff Evidence
Execute and maintain comprehensive verification gates: Svelte web type checks (pnpm --filter @abuzar/web check), production build validation (pnpm --filter @abuzar/web build), Playwright browser test suite, Go backend unit/integration tests (go test ./services/api/... ./services/edge/... ./migration/...), and PostgreSQL migration replay (ops/postgres/apply-migrations.ps1). Maintain docs/ACCEPTANCE_EVIDENCE_2026-08-07.md and related phase evidence documents.

## Acceptance Criteria

### Web & API Build Integrity
- [ ] pnpm --filter @abuzar/web check completes with 0 errors and 0 warnings.
- [ ] pnpm --filter @abuzar/web build completes successfully.
- [ ] go vet ./services/api/... ./services/edge/... ./migration/... completes with 0 issues.
- [ ] go test ./services/api/... ./services/edge/... ./migration/... -count=1 passes 100% of unit and integration tests.

### Browser Workflow & End-to-End Tests
- [ ] pnpm --filter @abuzar/web test -- --workers=1 --retries=0 passes all browser tests serially without assertion failures.
- [ ] Playwright tests cover sales, purchase, item maintenance, auxiliary master CRUD, pricing preview, MDI navigation, report preview, and posted document voiding.

### Schema & Migration Replay
- [ ] ops/postgres/apply-migrations.ps1 runs clean against the PostgreSQL target with 0 errors.
- [ ] Migration exception and ambiguity tracking script runs without unhandled open critical exceptions.

### Implementation Status & Evidence Documentation
- [ ] docs/IMPLEMENTATION_STATUS.md and docs/ACCEPTANCE_EVIDENCE_2026-08-07.md reflect verified status across all functional waves.

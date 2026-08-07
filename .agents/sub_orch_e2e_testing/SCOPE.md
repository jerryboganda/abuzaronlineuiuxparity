# Scope: E2E Testing Track

## Architecture
- Browser Playwright E2E Test Suite (`apps/web`)
- Go API & Edge Unit/Integration Suites (`services/api`, `services/edge`)
- Data Import & Bookkeeping Reconciler Tests (`migration`)
- PostgreSQL Migration Replay Verification (`ops/postgres/apply-migrations.ps1`)

## Feature Inventory Mapping
All 26 features from PROJECT.md mapped across Tier 1 (Feature), Tier 2 (Boundary), Tier 3 (Cross-Feature), Tier 4 (Real-World Application).

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Test Infra Survey & Mapping | Survey existing test files and map all 26 features to Tiers 1-4 in TEST_INFRA.md | none | DONE |
| 2 | Playwright Browser E2E Suite Verification | Verify Playwright browser tests serially (`pnpm --filter @abuzar/web test -- --workers=1 --retries=0`) | M1 | DONE |
| 3 | Go Backend & Migration Suite Verification | Verify `go test ./services/api/... ./services/edge/... ./migration/...` and reconciler enforcement | M1 | DONE |
| 4 | Test Readiness Publication | Validate all Tier 1-4 requirements and publish `TEST_READY.md` | M2, M3 | DONE |

## Interface Contracts
- Playwright runner: `pnpm --filter @abuzar/web test -- --workers=1 --retries=0`
- Go test runner: `go test ./services/api/... ./services/edge/... ./migration/... -count=1`
- Reconciler runner: `go run ./migration/cmd/reconcile -fail-on-open-bookkeeping`
- Migration script: `ops/postgres/apply-migrations.ps1`

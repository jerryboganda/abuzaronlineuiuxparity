# Project: AbuzarNext Rebuild

## Architecture
- SvelteKit frontend (`apps/web`), Go API backend (`services/api`), Go Edge sync/hardware service (`services/edge`), PostgreSQL migration engine (`migration/`).
- Shared RLS tenancy, session tokens, exact-decimal pricing engine, 151 catalog report definitions, ESC/POS hardware abstraction.

## Feature Inventory
Every feature from the Survey phase appears here with its assigned milestone.
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | Legacy Shell & Window Management | MDI window registry, tabs, cascade/tile layout, SessionStorage state | M1 | survey |
| 2 | Navigation, Shortcut Keys & Context Menus | Ctrl+Alt+M, Ctrl+X, Ctrl+Q, 325+ contextual menu catalog items | M1 | survey |
| 3 | Modal Dialogs & Change User Flow | Change user modal dialog, session re-authentication, confirmation dialogs | M1 | survey |
| 4 | Visual Comparison & Zero-Pixel Parity | Baseline raster comparison at 1936x1048, stateful interactive UI | M1 | survey |
| 5 | Database Schema & RLS Tenancy | 30 migrations in db/migrations/, tenant/branch RLS policies, audit bookkeeping | M2 | survey |
| 6 | Data Import & Reconciliation Engine | Declarative JSON mapping, metric reconciler, auxiliary master CRUD (16 leaves) | M2 | survey |
| 7 | Exception & Ambiguity Tracking | Bookkeeping tables migration_exceptions & migration_ambiguous_records | M2 | survey |
| 8 | Transaction Bookkeeping Reconciliation | Historical StockReport and VirtualGl read models, line exception tracking | M2 | survey |
| 9 | Exact-Decimal Pricing Engine | math/big.Rat, Money, BasisPoints, 0 floating point calculations | M3 | survey |
| 10 | 10-Tier SalePrice & Discount Precedence | 10 price tiers, supplier scheme bonus/discounts, customer/group precedence | M3 | survey |
| 11 | Tax Policy & Tax Rule Processing | GST, PCT, Advance tax rules (inclusive & exclusive calculations) | M3 | survey |
| 12 | Stock Balance & Snapshot Engine | Real-time stock balance, StockReport back-date snapshots | M3 | survey |
| 13 | Financial Engine & Historical GL | Historical VirtualGl ledger projections, compensating void reversals | M3 | survey |
| 14 | 151 Catalog Report Definitions | 151 non-blank report catalog leaves mapped to explicit API & web definitions | M4 | survey |
| 15 | Report Preview & Formatting Surface | Print preview surface (ruler, zoom, loaded-row paging, letterhead) | M4 | survey |
| 16 | Report Export Capabilities | CSV, workbook, and multi-format export hooks | M4 | survey |
| 17 | Edge Hardware Integration Subsystem | ESC/POS receipt/label renderers, cash drawer pulse (0x1b 0x70), barcode lookup | M4 | survey |
| 18 | Desktop Tauri IPC & Windows Credentials | Tauri IPC bridge, Windows Credential Manager integration for edge secrets | M4 | survey |
| 19 | Svelte Web Type Check | pnpm --filter @abuzar/web check (0 errors, 0 warnings) | M5 | survey |
| 20 | Web Production Build Validation | pnpm --filter @abuzar/web build (static site generation) | M5 | survey |
| 21 | Go Code Quality Analysis | go vet ./services/api/... ./services/edge/... ./migration/... | M5 | survey |
| 22 | Go Unit & Integration Suite | go test ./services/api/... ./services/edge/... ./migration/... -count=1 | M5 | survey |
| 23 | Browser Playwright E2E Test Suite | pnpm --filter @abuzar/web test -- --workers=1 --retries=0 (77/77 serial tests) | M5 | survey |
| 24 | PostgreSQL Migration Replay | ops/postgres/apply-migrations.ps1 execution & validation | M5 | survey |
| 25 | Bookkeeping Reconciler Enforcement | migration/cmd/reconcile -fail-on-open-bookkeeping execution | M5 | survey |
| 26 | Acceptance Evidence Documentation | docs/IMPLEMENTATION_STATUS.md & docs/ACCEPTANCE_EVIDENCE_2026-08-07.md | M5 | survey |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | M1: Legacy Shell, Workflow & MDI Parity | Shell, MDI window registry, tabs, layout controls, shortcuts, contextual menus, change user modal | none | IN_PROGRESS |
| 2 | M2: Schema, Data Import & Bookkeeping Reconciliation | PostgreSQL migrations, RLS tenancy, declarative importer, auxiliary master CRUD, exception/ambiguity tracking | M1 | IN_PROGRESS |
| 3 | M3: Pricing Policy, Stock Balance & Financial Engine | Exact-decimal pricing, 10 price tiers, discount/tax rules, stock balance, StockReport back-date, VirtualGl, void reversals | M2 | IN_PROGRESS |
| 4 | M4: Report Engine & Hardware Integration | 151 catalog report definitions, preview UI (ruler, zoom, paging, letterhead), exports, ESC/POS, cash drawer, barcode | M3 | IN_PROGRESS |
| 5 | M5: Comprehensive Verification & Evidence Verification | Automated build checks, Go vet/tests, Playwright E2E suite, migration replay, reconciler check, evidence docs | M4 | IN_PROGRESS |

## Interface Contracts
### Web (`apps/web`) ↔ API (`services/api`)
- Session Auth: `POST /v1/auth/login`, `POST /v1/auth/change-user`
- MDI / Menu: Legacy window registry state in SessionStorage; menu tree from `legacy-menu-contextual-catalog.ts`
- Pricing Engine Preview: `POST /v1/transactions/preview` with exact decimal items
- Stock & Ledger: `GET /v1/inventory/balance`, `GET /v1/reports/stock-back-date`, `GET /v1/reports/gl-journal`
- Reports: `GET /v1/reports/[kind]` resolving to 151 catalog definitions
- Void Reversals: `POST /v1/documents/[kind]/[id]/void`

### Web (`apps/web`) ↔ Edge Hardware (`services/edge`)
- Readiness: `GET http://127.0.0.1:8091/v1/hardware/readiness`
- ESC/POS Receipt: `POST http://127.0.0.1:8091/v1/hardware/print/sale-slip`
- Cash Drawer Pulse: `POST http://127.0.0.1:8091/v1/hardware/drawer/kick`
- Barcode Lookup: `GET http://127.0.0.1:8091/v1/hardware/barcode/[code]`

## Code Layout
- `apps/web`: SvelteKit frontend web application
- `services/api`: Go REST backend API (pricing, reports, inventory, documents, auth)
- `services/edge`: Go Edge sync & local hardware service (ESC/POS, cash drawer, barcode, IPC)
- `migration`: Data import declarative mapping engine & reconciler tool
- `db/migrations`: PostgreSQL DDL schema files (`001_tenancy.sql` .. `029_auxiliary_master_kinds.sql`)
- `ops/postgres`: PostgreSQL migration application scripts (`apply-migrations.ps1`)
- `docs/`: Technical specification & acceptance evidence documentation

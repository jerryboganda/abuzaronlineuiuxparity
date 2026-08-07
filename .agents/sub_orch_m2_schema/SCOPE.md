# Scope: M2: Schema, Data Import & Bookkeeping Reconciliation

## Architecture
- PostgreSQL migrations (`db/migrations/001_tenancy.sql` .. `030_...`)
- Go Data Import & Reconciliation Engine (`migration/`)
- RLS Tenancy & Audit Bookkeeping
- Auxiliary Master CRUD (16 leaves)
- Exception & Ambiguity Tracking (`migration_exceptions`, `migration_ambiguous_records`)
- Read Models: `StockReport` historical stock balances and `VirtualGl` ledger projections

## Feature Inventory (M2 Scope)
| # | Feature | Description | Status |
|---|---------|-------------|--------|
| 5 | Database Schema & RLS Tenancy | 30 migrations in db/migrations/, tenant/branch RLS policies, audit bookkeeping | IN_PROGRESS |
| 6 | Data Import & Reconciliation Engine | Declarative JSON mapping, metric reconciler, auxiliary master CRUD (16 leaves) | IN_PROGRESS |
| 7 | Exception & Ambiguity Tracking | Bookkeeping tables migration_exceptions & migration_ambiguous_records | IN_PROGRESS |
| 8 | Transaction Bookkeeping Reconciliation | Historical StockReport and VirtualGl read models, line exception tracking | IN_PROGRESS |

## Sub-Milestones & Verification Tasks
1. Database Schema & RLS Tenancy Verification (all 30 migrations, RLS policies, audit columns).
2. Declarative Importer & Reconciler Verification (`migration/`, JSON mapping, 16 auxiliary master leaves).
3. Exception & Ambiguity Tracking (`migration_exceptions`, `migration_ambiguous_records`).
4. Read Models & Line Exception Reconciliation (`StockReport`, `VirtualGl`).

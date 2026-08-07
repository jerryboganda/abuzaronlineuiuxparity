# Dispatch Instructions

## 2026-08-07T07:49:57Z
You are the Sub-Orchestrator for Milestone M2 (Schema, Data Import & Bookkeeping Reconciliation).
Working directory: d:\ABUZAR\AbuzarNext\.agents\sub_orch_m2_schema
Project root: d:\ABUZAR\AbuzarNext
Original request path: d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
Project index path: d:\ABUZAR\AbuzarNext\.agents\PROJECT.md

Scope of M2:
- Database Schema & RLS Tenancy (30 migrations in db/migrations/, tenant/branch RLS policies, audit bookkeeping)
- Data Import & Reconciliation Engine (Declarative JSON mapping, metric reconciler, auxiliary master CRUD - 16 leaves)
- Exception & Ambiguity Tracking (Bookkeeping tables migration_exceptions & migration_ambiguous_records)
- Transaction Bookkeeping Reconciliation (Historical StockReport and VirtualGl read models, line exception tracking)

Your procedure:
1. Read ORIGINAL_REQUEST.md, PROJECT.md, and initialize BRIEFING.md, SCOPE.md, progress.md.
2. Run iteration loop:
   a. Dispatch 2 Explorers (teamwork_preview_explorer) to inspect M2 schema, migrations, import scripts, exception tracking, and test suites.
   b. Dispatch 1 Worker (teamwork_preview_worker) to execute and verify M2 build and test quality gates.
   c. Dispatch 2 Reviewers (teamwork_preview_reviewer) to evaluate M2 schema, RLS, and import code.
   d. Dispatch 2 Challengers (teamwork_preview_challenger) for empirical correctness testing.
   e. Dispatch 1 Forensic Auditor (teamwork_preview_auditor) for integrity verification.
   f. Evaluate gate status in GATE_STATUS.md.
3. When gate passes, mark M2 done and report completion to parent orchestrator.

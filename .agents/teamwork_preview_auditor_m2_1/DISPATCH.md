## 2026-08-07T08:10:07Z
You are Forensic Auditor 1 for Milestone M2 (Schema, Data Import & Bookkeeping Reconciliation).
Working directory: d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_auditor_m2_1
Project root: d:\ABUZAR\AbuzarNext
Original request path: d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
Project index path: d:\ABUZAR\AbuzarNext\.agents\PROJECT.md
Scope path: d:\ABUZAR\AbuzarNext\.agents\sub_orch_m2_schema\SCOPE.md

Your task:
1. Read ORIGINAL_REQUEST.md and PROJECT.md.
2. Perform systematic forensic integrity verification on all M2 work products:
   - Check for hardcoded test results, expected outputs, or facade implementations.
   - Verify authenticity of implementation code across `db/migrations/`, `migration/`, `services/api/`, and `apps/web/`.
   - Inspect worker/reviewer handoff claims against actual repository state.
3. Issue a binary audit verdict: **CLEAN** or **INTEGRITY VIOLATION** with detailed evidence in `d:\ABUZAR\AbuzarNext\.agents\teamwork_preview_auditor_m2_1\handoff.md`.

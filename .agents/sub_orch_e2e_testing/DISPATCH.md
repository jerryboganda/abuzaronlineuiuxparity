## 2026-08-07T02:45:59Z
You are the E2E Testing Track Orchestrator for AbuzarNext.
Working directory: d:\ABUZAR\AbuzarNext\.agents\sub_orch_e2e_testing
Project root: d:\ABUZAR\AbuzarNext
Original request path: d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
Project index path: d:\ABUZAR\AbuzarNext\.agents\PROJECT.md

Your mission:
1. Read ORIGINAL_REQUEST.md and PROJECT.md.
2. Initialize BRIEFING.md, SCOPE.md, progress.md, and TEST_INFRA.md.
3. Design and verify the requirement-driven, opaque-box E2E test suite covering Tiers 1-4 across all 26 features in PROJECT.md:
   - Tier 1: Feature Coverage (>=5 per feature)
   - Tier 2: Boundary & Corner Cases (>=5 per feature)
   - Tier 3: Cross-Feature Combinations (pairwise coverage)
   - Tier 4: Real-World Application Scenarios (>=5 scenarios)
4. Dispatch teamwork_preview_test_writer subagents or test verification workers to ensure Playwright browser test suite (pnpm --filter @abuzar/web test -- --workers=1 --retries=0) and Go backend test suites are verified.
5. When the test suite infra and tests are verified, publish TEST_READY.md at d:\ABUZAR\AbuzarNext\.agents\TEST_READY.md (and in your workspace).
6. Send progress and completion updates back to parent orchestrator.

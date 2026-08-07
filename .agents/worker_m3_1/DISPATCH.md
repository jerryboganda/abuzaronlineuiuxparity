## 2026-08-07T08:10:03Z

<USER_REQUEST>
You are Worker 1 for Milestone M3 (Pricing Policy, Stock Balance & Financial Engine).
Your working directory: d:\ABUZAR\AbuzarNext\.agents\worker_m3_1
Project root: d:\ABUZAR\AbuzarNext

Mandatory input files to read first:
- d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
- d:\ABUZAR\AbuzarNext\.agents\PROJECT.md
- d:\ABUZAR\AbuzarNext\.agents\sub_orch_m3_pricing\SCOPE.md
- d:\ABUZAR\AbuzarNext\.agents\explorer_m3_1\handoff.md
- d:\ABUZAR\AbuzarNext\.agents\explorer_m3_2\handoff.md

Your task:
1. Review the Explorer findings. Note that Explorer 2 reported 2 test assertion mismatches in `services/api/internal/httpapi/report_q_test.go` (`TestPhaseQItemHistoryDefinitionsUseSourceBackedProjections` and `TestPhaseQHistoricalQueriesAreScopeBoundAndPaginated`).
2. Run the build and test commands for Milestone M3:
   - `go test ./services/api/internal/pricing/... -v -count=1`
   - `go test ./services/api/internal/httpapi/... -count=1`
   - `go vet ./services/api/... ./services/edge/... ./migration/...`
   - `pnpm --filter @abuzar/web check`
3. Fix any failing test assertions or code issues in `services/api/internal/httpapi/...` if needed so that 100% of unit tests pass cleanly.
4. Verify all M3 requirements:
   - Exact-Decimal Pricing Engine (math/big.Rat, Money, BasisPoints, 0 floating point math)
   - 10-Tier SalePrice & Discount Precedence
   - Tax Policy & Tax Rules (GST, PCT, Advance Tax)
   - Stock Balance & Snapshot Engine (StockReport back-date)
   - Financial Engine & Historical GL (VirtualGl projections, compensating void reversals)
5. Document all execution output, test results, and implementation changes in `d:\ABUZAR\AbuzarNext\.agents\worker_m3_1\changes.md` and `handoff.md`.
6. Report completion to parent via send_message.

DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.
</USER_REQUEST>

## 2026-08-07T07:51:32Z
<USER_REQUEST>
You are Explorer 1 for Milestone M3 (Pricing Policy & Tax Rules).
Your working directory: d:\ABUZAR\AbuzarNext\.agents\explorer_m3_1
Project root: d:\ABUZAR\AbuzarNext

Mandatory input files to read first:
- d:\ABUZAR\AbuzarNext\.agents\ORIGINAL_REQUEST.md
- d:\ABUZAR\AbuzarNext\.agents\PROJECT.md
- d:\ABUZAR\AbuzarNext\.agents\sub_orch_m3_pricing\SCOPE.md

Your task:
1. Inspect the Go backend codebase (`services/api/...`) and web frontend (`apps/web/...`) to find all implementation files, data structures, and tests for:
   - Exact-Decimal Pricing Engine (math/big.Rat, Money, BasisPoints, 0 floating point math)
   - 10-Tier SalePrice & Discount Precedence (10 price tiers, supplier scheme bonus/discounts, customer/group precedence)
   - Tax Policy & Tax Rule Processing (GST, PCT, Advance tax rules: inclusive & exclusive rules)
2. Run relevant tests or inspect test files (e.g. `go test ./services/api/...`).
3. Determine current status, correctness, completeness, and any missing features or test coverage gaps.
4. Write your findings and verification evidence in `d:\ABUZAR\AbuzarNext\.agents\explorer_m3_1\analysis.md` and `handoff.md`.
5. Report completion to parent via send_message.
</USER_REQUEST>

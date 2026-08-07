# Scope: Milestone M3 — Pricing Policy, Stock Balance & Financial Engine

## Architecture
- Exact-decimal calculation in Go backend (`services/api/pkg/pricing`, `services/api/pkg/inventory`, `services/api/pkg/financial` or equivalent packages).
- 0 floating-point math: `math/big.Rat`, `Money`, `BasisPoints`.
- 10-Tier SalePrice & Discount Precedence: supplier scheme bonus/discounts, customer/group precedence.
- Tax Policy: GST, PCT, Advance tax rules (inclusive & exclusive math).
- Stock Balance & Snapshot Engine: Real-time stock balance, StockReport back-date snapshots.
- Financial Engine: Historical VirtualGl ledger projections, compensating void reversals.

## Feature Inventory
| # | Feature | Description | Milestone | Status |
|---|---------|-------------|-----------|--------|
| 9 | Exact-Decimal Pricing Engine | math/big.Rat, Money, BasisPoints, 0 floating point math | M3 | IN_PROGRESS |
| 10 | 10-Tier SalePrice & Discount Precedence | 10 price tiers, supplier scheme bonus/discounts, customer/group precedence | M3 | IN_PROGRESS |
| 11 | Tax Policy & Tax Rule Processing | GST, PCT, Advance tax rules | M3 | IN_PROGRESS |
| 12 | Stock Balance & Snapshot Engine | Real-time stock balance, StockReport back-date snapshots | M3 | IN_PROGRESS |
| 13 | Financial Engine & Historical GL | Historical VirtualGl ledger projections, compensating void reversals | M3 | IN_PROGRESS |

## Sub-Milestones / Verification Gates
1. Unit/Integration Tests pass for pricing, inventory, and financial packages.
2. 0 floating-point usage in monetary/tax calculations.
3. Reviewer APPROVE verdicts.
4. Challenger calculation verification.
5. Forensic Auditor CLEAN verdict.

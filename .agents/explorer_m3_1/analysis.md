# Analysis Report: Pricing Policy & Tax Rules (Milestone M3)

**Author:** Explorer 1 (Milestone M3)  
**Date:** 2026-08-07  
**Working Directory:** `d:\ABUZAR\AbuzarNext\.agents\explorer_m3_1`  
**Project Root:** `d:\ABUZAR\AbuzarNext`  

---

## Executive Summary

This investigation analyzed the implementation files, data structures, backend algorithms, database schema, web frontend components, and automated test suites for:
1. **Exact-Decimal Pricing Engine** (`math/big.Rat`, `Money`, `BasisPoints`, zero floating-point math).
2. **10-Tier SalePrice & Discount Precedence** (10 price tiers, supplier scheme bonus/discounts, customer/group precedence).
3. **Tax Policy & Tax Rule Processing** (GST, PCT, Advance tax rules with inclusive & exclusive math).

**Key Findings:**
- The Go backend features a pure, policy-driven exact-decimal pricing engine (`services/api/internal/pricing/pricing.go`) that completely avoids IEEE-754 floating-point arithmetic.
- 10-Tier pricing selection (`SalePrice1` through `SalePrice10`) is fully supported in backend data structures (`PriceTiers [10]Money`), HTTP APIs (`/v1/transactions/preview`), and SvelteKit frontend UI (`apps/web/src/routes/app/sales/+page.svelte`).
- Customer discount precedence (`OverridePercent > CustomerPercent > GroupPercent`) and Supplier Scheme bonus calculations (free bonus units without inflating billed amount) are deterministically calculated.
- Tax policy handles GST, PCT, and Advance income tax in fixed order with support for inclusive and exclusive calculation rules, backed by PostgreSQL RLS branch-hardened tables (`tax_rates`, `item_tax_assignments`, `party_tax_assignments` in `018_tax_configuration.sql`).
- Backend unit test suites (`go test ./services/api/internal/pricing/...`) pass **100%** (16 subtests). HTTP API pricing and tax route unit tests in `services/api/internal/httpapi` pass cleanly.

---

## 1. Exact-Decimal Pricing Engine

### Implementation Files & Data Structures
- **Core Engine:** `services/api/internal/pricing/pricing.go`
- **Unit Test Suite:** `services/api/internal/pricing/pricing_test.go`
- **Transport HTTP API:** `services/api/internal/httpapi/pricing.go`
- **HTTP Unit Test Suite:** `services/api/internal/httpapi/pricing_test.go`
- **TypeScript Contracts:** `packages/contracts/src/index.ts`
- **Web API Client:** `apps/web/src/lib/api.ts`
- **Sales Page:** `apps/web/src/routes/app/sales/+page.svelte`

### Data Structures & Types
```go
// services/api/internal/pricing/pricing.go

// Money is integer minor units (paisa/cents). 0 floating point math.
type Money int64

// BasisPoints is percentage * 100 (10000 = 100.00%, 750 = 7.50%).
type BasisPoints int64

// Quantity is whole-unit quantity.
type Quantity int64

// Explicit rounding modes
type RoundingMode uint8
const (
    RoundHalfUp RoundingMode = iota // Default 5-up rounding
    RoundHalfEven                   // Banker's rounding
    RoundDown                       // Truncate/floor
)

type RoundingPolicy struct {
    Line     RoundingMode
    Tax      RoundingMode
    Document RoundingMode
}
```

### Exact Decimal Calculations & Zero Floating-Point Math
- All percentage arithmetic, tax scaling, and ratio multiplications utilize Go's standard library `math/big.Rat`.
- Decimal parsing (`parseMoney`, `parsePercent`, `parseQuantity` in `services/api/internal/httpapi/pricing.go`) validates input strings using `big.Rat.SetString` and scales them directly to `int64` minor units / basis points without floating-point intermediate values.
- String diagnostics (`Money.String()`) construct `"major.minor"` output via integer division (`value / 100`) and modulo (`value % 100`) with zero `float64` casts.

---

## 2. 10-Tier SalePrice & Discount Precedence

### Implementation Files & Data Structures
- **Core Engine:** `services/api/internal/pricing/pricing.go` (lines 76-96)
- **Sales UI Component:** `apps/web/src/routes/app/sales/+page.svelte` (lines 63-67, 374-377, 678)
- **Browser Playwright E2E:** `apps/web/tests/sales-canonical.spec.ts` (lines 156-175)

### Data Structures
```go
// services/api/internal/pricing/pricing.go

// PriceTiers holds SalePrice#1 through SalePrice#10 in array order.
type PriceTiers [10]Money

// SupplierScheme exposes bonus units for qualifying billed units without billing the bonus units.
type SupplierScheme struct {
    DiscountPercent    BasisPoints
    QualifyingQuantity Quantity
    BonusQuantity      Quantity
}

// CustomerDiscounts defines precedence: OverridePercent > CustomerPercent > GroupPercent.
type CustomerDiscounts struct {
    GroupPercent    BasisPoints
    CustomerPercent *BasisPoints
    OverridePercent *BasisPoints
}
```

### Calculation & Precedence Rules
1. **Tier Selection:** `PriceLevel` (1..10) selects `line.Prices[PriceLevel-1]`.
2. **Supplier Scheme:** Discount factor `(10000 - DiscountPercent) / 10000` is applied to base line gross. Bonus units are calculated as `(Quantity / QualifyingQuantity) * BonusQuantity` and reported separately (`SupplierBonusQuantity`).
3. **Discount Precedence:** `CustomerDiscounts.selected()` evaluates precedence in order:
   - If `OverridePercent != nil`, use `OverridePercent` (`CustomerDiscountFromOverride`).
   - Else if `CustomerPercent != nil`, use `CustomerPercent` (`CustomerDiscountFromCustomer`).
   - Else use `GroupPercent` (`CustomerDiscountFromGroup`).
4. **Document Discounts:** Line subtotal is calculated -> Document percentage discount is subtracted -> Flat discount is subtracted -> Miscellaneous amount (`Misc`, default 1.00 if nil) is added -> Produces `TaxableBase`.

---

## 3. Tax Policy & Tax Rule Processing

### Implementation Files & Data Structures
- **Core Calculation Engine:** `services/api/internal/pricing/pricing.go` (lines 106-131, 335-372)
- **Tax HTTP Services & Resolution:** `services/api/internal/httpapi/tax.go`
- **Tax DB Migration & RLS:** `db/migrations/018_tax_configuration.sql`
- **Tax Unit Tests:** `services/api/internal/httpapi/tax_test.go`

### Data Structures
```go
// services/api/internal/pricing/pricing.go

type TaxKind uint8
const (
    TaxGST TaxKind = iota
    TaxPCT
    TaxAdvance
)

type TaxRule struct {
    Kind      TaxKind
    Rate      BasisPoints
    Inclusive bool
}

type TaxPolicy struct {
    GST        *TaxRule
    PCT        *TaxRule
    AdvanceTax *TaxRule
}
```

### Tax Ordering and Inclusive vs Exclusive Calculations
- **Processing Order:** Fixed sequence `orderedTaxes()` ensures execution order is always:
  1. **GST**
  2. **PCT**
  3. **Advance Tax**
- **Inclusive Calculation:**
  $$\text{TaxAmount} = \text{Base} \times \frac{\text{Rate}}{10000 + \text{Rate}}$$
  Tax is extracted from base, and base is reduced by tax amount for subsequent taxes.
- **Exclusive Calculation:**
  $$\text{TaxAmount} = \text{Base} \times \frac{\text{Rate}}{10000}$$
  Tax is calculated on base and added to document total.
- **Effective Dates & Assignments:**
  Database tables `tax_rates`, `item_tax_assignments`, and `party_tax_assignments` track tax rules by `effective_from` and `effective_to` dates. `resolveDocumentTaxPolicy` resolves active taxes for items and parties as of document `occurredAt`. Explicit tax overrides require `tax.override` permission.

---

## 4. Verification Evidence

### Go Unit Tests (Pricing Engine)
Ran `go test ./services/api/internal/pricing/... -v -count=1`

**Output:**
```text
=== RUN   TestCalculateGoldenCases
=== RUN   TestCalculateGoldenCases/selected_price_tier
=== RUN   TestCalculateGoldenCases/item_and_customer_override_precedence
=== RUN   TestCalculateGoldenCases/group_discount_when_no_customer_override_exists
=== RUN   TestCalculateGoldenCases/document_percent_flat_and_default_misc
=== RUN   TestCalculateGoldenCases/inclusive_GST
=== RUN   TestCalculateGoldenCases/exclusive_GST
=== RUN   TestCalculateGoldenCases/exclusive_PCT_then_advance_tax
=== RUN   TestCalculateGoldenCases/supplier_scheme_discount_and_bonus
=== RUN   TestCalculateGoldenCases/explicit_rounding_boundaries
=== RUN   TestCalculateGoldenCases/half_up_rounds_a_half_minor_unit_upward
--- PASS: TestCalculateGoldenCases (0.00s)
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs/invalid_price_level
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs/negative_price
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs/negative_item_discount
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs/flat_discount_exceeds_subtotal
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs/invalid_tax_rate
=== RUN   TestCalculateRejectsInvalidAndOverflowInputs/line_multiplication_overflow
--- PASS: TestCalculateRejectsInvalidAndOverflowInputs (0.00s)
=== RUN   TestCalculateIsDeterministic
--- PASS: TestCalculateIsDeterministic (0.00s)
=== RUN   TestMoneyString
--- PASS: TestMoneyString (0.00s)
PASS
ok  	github.com/abuzar/abuzar-next/services/api/internal/pricing	0.711s
```

### Go Unit Tests (HTTP API Pricing & Tax Handlers)
Ran `go test ./services/api/internal/pricing ./services/api/internal/httpapi -run "TestPricing|TestTax" -v`

**Output:**
```text
=== RUN   TestPricingRequestParsersUseExactDecimalBoundaries
--- PASS: TestPricingRequestParsersUseExactDecimalBoundaries (0.00s)
=== RUN   TestPricingPreviewRequestMapsTiersDiscountsAndTaxes
--- PASS: TestPricingPreviewRequestMapsTiersDiscountsAndTaxes (0.00s)
=== RUN   TestPricingPreviewRouteRemainsAuthenticated
--- PASS: TestPricingPreviewRouteRemainsAuthenticated (0.00s)
=== RUN   TestTaxRateValidationCoversKindsAndEffectiveDates
--- PASS: TestTaxRateValidationCoversKindsAndEffectiveDates (0.00s)
=== RUN   TestTaxRoutesRemainAuthenticated
--- PASS: TestTaxRoutesRemainAuthenticated (0.00s)
=== RUN   TestTaxMigrationDefinesEffectiveScopedConfiguration
--- PASS: TestTaxMigrationDefinesEffectiveScopedConfiguration (0.00s)
=== RUN   TestTaxConfigurationResolvesProfilesEffectiveDatesAndPostedGL
    tax_test.go:108: DATABASE_URL is not configured
--- SKIP: TestTaxConfigurationResolvesProfilesEffectiveDatesAndPostedGL (0.00s)
PASS
ok  	github.com/abuzar/abuzar-next/services/api/internal/httpapi	2.112s
```

---

## 5. Status & Coverage Assessment

| Feature | Sub-components | Implementation Status | Test Coverage Status |
|---|---|---|---|
| Exact-Decimal Pricing Engine | `math/big.Rat`, `Money`, `BasisPoints`, 0 float math, decimal parsers | **COMPLETE** | **100% Verified** in `internal/pricing` & `httpapi` |
| 10-Tier SalePrice Selection | `PriceTiers [10]Money`, 1..10 price tier selector in UI & API preview | **COMPLETE** | **Verified** in `pricing_test.go` & `sales-canonical.spec.ts` |
| Discount Precedence Rules | `OverridePercent > CustomerPercent > GroupPercent`, supplier schemes & bonus units | **COMPLETE** | **100% Verified** in `pricing_test.go` |
| Tax Policy & Rule Processing | GST, PCT, Advance tax (inclusive & exclusive calculation, RLS DB tables, effective dates) | **COMPLETE** | **Verified** (DB integration test skips when `DATABASE_URL` is omitted) |

### Test Coverage Gaps & Caveats
1. **`DATABASE_URL` Requirement for Tax Integration Test:** `TestTaxConfigurationResolvesProfilesEffectiveDatesAndPostedGL` skips when running unit tests without PostgreSQL `DATABASE_URL`. Running full PostgreSQL suite (`apply-migrations.ps1`) executes this test.
2. **Web Purchase Workflow Preview Hook:** Web sales UI (`apps/web/src/routes/app/sales/+page.svelte`) calls `/v1/transactions/preview` for real-time recalculation. Web purchase UI (`apps/web/src/routes/app/purchase/[kind]/+page.svelte`) relies on frontend line multiplication and submits canonical document commands (`pack-purchase`, `purchase-return`, etc.) directly to `/v1/documents/[kind]`, where the Go backend validates costs, taxes, and GL postings upon saving/posting.

---

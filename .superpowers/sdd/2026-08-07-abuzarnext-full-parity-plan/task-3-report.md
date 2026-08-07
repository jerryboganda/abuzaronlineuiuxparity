# Task 3 Report: Pricing & Discount Engine Hardening (Phase G)

**Status:** DONE  
**Date:** 2026-08-07  
**Commit Hash:** e3fbe9ce72a7c8676609025eb98ec66505ccfa01  

---

## 1. Code Inspection & Parity Review
- Inspected `services/api/internal/httpapi/pricing.go` and `services/api/internal/pricing/pricing.go`.
- Inspected frontend sales price tier binding and preview triggers in `apps/web/src/routes/app/sales/+page.svelte`.
- Confirmed strict decimal parsing (`parseMoney`, `parsePercent`, `parseQuantity`), 10 `SalePrice` tier mapping (`SalePrice1` through `SalePrice10`), document flat discount application, default `Misc` fee (+1.00), and `ItemSuppliers` bonus scheme integration.

---

## 2. Go Test Execution Summary

### HTTP API Pricing Tests
Command: `go test ./services/api/internal/httpapi -run "TestPricing|TestSalePrice|TestCustomer" -count=1 -v`

**Output:**
```
=== RUN   TestPricingRequestParsersUseExactDecimalBoundaries
--- PASS: TestPricingRequestParsersUseExactDecimalBoundaries (0.00s)
=== RUN   TestPricingPreviewRequestMapsTiersDiscountsAndTaxes
--- PASS: TestPricingPreviewRequestMapsTiersDiscountsAndTaxes (0.00s)
=== RUN   TestPricingPreviewRouteRemainsAuthenticated
--- PASS: TestPricingPreviewRouteRemainsAuthenticated (0.00s)
=== RUN   TestCustomerSalesSummaryReadModelsUseExplicitBuckets
--- PASS: TestCustomerSalesSummaryReadModelsUseExplicitBuckets (0.00s)
=== RUN   TestCustomerSalesCategorySummaryUsesCustomerCategoryPayload
--- PASS: TestCustomerSalesCategorySummaryUsesCustomerCategoryPayload (0.00s)
=== RUN   TestCustomerWiseCategorySummaryGroupsByCustomerAndCategory
--- PASS: TestCustomerWiseCategorySummaryGroupsByCustomerAndCategory (0.00s)
=== RUN   TestCustomerCategorySalesDetailReportUsesLineDetailProjection
--- PASS: TestCustomerCategorySalesDetailReportUsesLineDetailProjection (0.00s)
=== RUN   TestCustomerSalesProfitMarginReadModelUsesAllocatedCost
--- PASS: TestCustomerSalesProfitMarginReadModelUsesAllocatedCost (0.00s)
=== RUN   TestCustomerSalesGrossProfitSummaryGroupsByCustomer
--- PASS: TestCustomerSalesGrossProfitSummaryGroupsByCustomer (0.00s)
PASS
ok  	github.com/abuzar/abuzar-next/services/api/internal/httpapi	2.094s
```

### Internal Pricing Engine Core Unit Tests
Command: `go test ./services/api/internal/pricing/... -count=1 -v`

**Output:**
```
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
--- PASS: TestCalculateRejectsInvalidAndOverflowInputs (0.00s)
=== RUN   TestCalculateIsDeterministic
--- PASS: TestCalculateIsDeterministic (0.00s)
=== RUN   TestMoneyString
--- PASS: TestMoneyString (0.00s)
PASS
ok  	github.com/abuzar/abuzar-next/services/api/internal/pricing	0.620s
```

---

## 3. Verified Pricing Engine Metrics
1. **Exact Decimal Pricing:** Monied values stored in integer minor units and parsed via `big.Rat` to prevent floating point inaccuracies.
2. **10 SalePrice Tiers:** `pricingPreviewRequest.toPricingRequest()` converts price tier arrays up to 10 entries and indexes tier selection per line input.
3. **Flat Discounts & Misc Fees:** Flat discounts subtract cleanly from document subtotal after percent discounts; `miscAmount` (default +1.00 fee) adds directly to final taxable base / document total.
4. **ItemSuppliers Bonus Scheme:** Bonus quantity calculation (`SupplierBonusQuantity`) and qualifying quantity discounts properly reflected in line gross and net calculations.

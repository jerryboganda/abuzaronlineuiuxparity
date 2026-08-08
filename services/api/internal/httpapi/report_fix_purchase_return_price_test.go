package httpapi

import (
	"strconv"
	"strings"
	"testing"
)

// This file regression-tests the fix for the purchase-return-detail
// "Purchase Price" bug documented in
// docs/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md ("Leaf 4 -
// purchase-return-detail"): purchase-return legacy rows store the actual
// per-unit return price under the payload key 'PRPrice' (not 'PurPrice'),
// so the purchase_price COALESCE in purchaseLineDetailReadModelQuery always
// fell through to l.unit_cost - the item's weighted-average inventory cost
// at time of return, a different quantity - for every purchase-return line
// (551 of 2,481 lines tenant-wide, 22.2%).
//
// Concrete example captured during golden verification: document 2120,
// item "COMFORT CORSET ALL SIZE", qty 1.0000. Raw data: legacy_payload has
// no 'PurPrice' key but PRPrice = "915.0000"; unit_cost = 738.1765 (average
// stock cost); unit_price = 915.0000 (matches legacy PRPrice); line_total
// (Amount) = 915.0000. Before the fix the report showed
// Purchase Price = 738.1765, so Purchase Price x Qty (738.1765) != Amount
// (915.0000) - an in-row arithmetic contradiction for a single-line detail
// report. After the fix Purchase Price must equal 915.0000 so that
// Purchase Price x Qty == Amount.

// TestPurchaseReturnDetailQueryChecksPRPriceLegacyKey asserts the generated
// SQL for the purchase-return-detail leaf (aggregateCondition
// "se.aggregate = 'return'") now checks the legacy_payload 'PRPrice' key,
// and that it is consulted ahead of the unit_cost fallback so a present
// PRPrice value wins over the average-cost columns.
func TestPurchaseReturnDetailQueryChecksPRPriceLegacyKey(t *testing.T) {
	query := purchaseLineDetailReadModelQuery("se.aggregate = 'return'", "")

	prPriceIdx := strings.Index(query, `l.legacy_payload->>'PRPrice'`)
	if prPriceIdx == -1 {
		t.Fatalf("purchase-return-detail query does not check legacy_payload->>'PRPrice':\n%s", query)
	}

	purPriceIdx := strings.Index(query, `l.legacy_payload->>'PurPrice'`)
	if purPriceIdx == -1 {
		t.Fatalf("purchase-return-detail query lost the 'PurPrice' check entirely:\n%s", query)
	}
	unitCostFallbackIdx := strings.Index(query, "NULLIF(l.unit_cost, 0)::text")
	if unitCostFallbackIdx == -1 {
		t.Fatalf("purchase-return-detail query lost the unit_cost fallback entirely:\n%s", query)
	}

	if !(purPriceIdx < prPriceIdx && prPriceIdx < unitCostFallbackIdx) {
		t.Fatalf("expected COALESCE priority order PurPrice(%d) < PRPrice(%d) < unit_cost fallback(%d) in query:\n%s",
			purPriceIdx, prPriceIdx, unitCostFallbackIdx, query)
	}
}

// TestPurchaseDetailAndPurchaseOrderQueriesDoNotGainPRPriceCheck guards
// against the fix leaking into the sibling leaves that share
// purchaseLineDetailReadModelQuery. The prior golden-verification pass
// confirmed purchase-detail (aggregateCondition "se.aggregate =
// 'receiving'") already matched with 0 arithmetic violations across
// 113,525 lines; this test proves the return-only fix does not touch that
// branch (or the purchase_order branch) at all.
func TestPurchaseDetailAndPurchaseOrderQueriesDoNotGainPRPriceCheck(t *testing.T) {
	tests := []struct {
		name      string
		aggregate string
	}{
		{"purchase-detail (receiving)", "se.aggregate = 'receiving'"},
		{"purchase-order", "se.aggregate = 'purchase_order'"},
	}
	for _, tc := range tests {
		query := purchaseLineDetailReadModelQuery(tc.aggregate, "")
		if strings.Contains(query, "PRPrice") {
			t.Errorf("%s: query unexpectedly checks 'PRPrice'; the fix must be scoped to the return branch only:\n%s", tc.name, query)
		}
		if !strings.Contains(query, `l.legacy_payload->>'PurPrice'`) {
			t.Errorf("%s: query lost its 'PurPrice' check; unrelated regression:\n%s", tc.name, query)
		}
	}
}

// coalesceFirstNonEmpty mirrors SQL COALESCE(NULLIF(x, ”), ...) semantics:
// it returns the first candidate that is non-empty.
func coalesceFirstNonEmpty(candidates ...string) string {
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	return ""
}

// purchasePriceForReturnLine reproduces, in Go, the exact priority chain of
// the purchase_price COALESCE in purchaseLineDetailReadModelQuery's
// canonical (business_documents/business_document_lines) branch, as it
// reads today (see reports.go, purchaseLineDetailReadModelQuery):
//
//	COALESCE(NULLIF(legacy_payload->>'PurPrice', ''),
//	         [NULLIF(legacy_payload->>'PRPrice', ''),]   <- only for return-kind lines, post-fix
//	         NULLIF(legacy_payload->>'purchasePrice', ''),
//	         NULLIF(pricing->>'purchasePrice', ''),
//	         NULLIF(pricing->>'unitCost', ''),
//	         NULLIF(unit_cost, 0)::text,
//	         unit_cost::text,
//	         unit_price::text, '')
//
// includePRPrice toggles the fix on/off so this test can demonstrate both
// the pre-fix bug and the post-fix behavior against the same raw data.
func purchasePriceForReturnLine(includePRPrice bool, purPrice, prPrice, legacyPurchasePrice, pricingPurchasePrice, pricingUnitCost string, unitCost, unitPriceCol float64) string {
	candidates := []string{purPrice}
	if includePRPrice {
		candidates = append(candidates, prPrice)
	}
	candidates = append(candidates,
		legacyPurchasePrice,
		pricingPurchasePrice,
		pricingUnitCost,
	)
	if unitCost != 0 {
		candidates = append(candidates, strconv.FormatFloat(unitCost, 'f', -1, 64))
	}
	candidates = append(candidates,
		strconv.FormatFloat(unitCost, 'f', -1, 64),
		strconv.FormatFloat(unitPriceCol, 'f', -1, 64),
	)
	return coalesceFirstNonEmpty(candidates...)
}

// TestPurchaseReturnDetailPriceMatchesAmountForDocument2120 replays the raw
// values captured for document 2120 / "COMFORT CORSET ALL SIZE" (qty
// 1.0000) during golden verification through the COALESCE priority chain
// above, before and after the fix, proving:
//   - pre-fix: purchase_price resolves to unit_cost (738.1765), so
//     Purchase Price x Qty != Amount (the documented bug);
//   - post-fix: purchase_price resolves to PRPrice (915.0000), so
//     Purchase Price x Qty == Amount (915.0000), matching unit_price and
//     resolving the in-row arithmetic contradiction.
func TestPurchaseReturnDetailPriceMatchesAmountForDocument2120(t *testing.T) {
	const (
		// document 2120 raw data (business_document_lines / legacy_payload)
		purPrice     = ""         // legacy return rows never carry 'PurPrice'
		prPrice      = "915.0000" // legacy return rows carry 'PRPrice' instead
		legacyPurch  = ""         // 'purchasePrice' key absent
		pricingPurch = ""         // pricing JSON is '{}' in this migration
		pricingCost  = ""         // pricing JSON is '{}' in this migration
		unitCost     = 738.1765   // weighted-average stock cost at time of return
		unitPriceCol = 915.0000   // canonical unit_price column (= legacy PRPrice)
		qty          = 1.0
		amount       = 915.0000 // line_total for this single-line document
	)

	preFixPrice, err := strconv.ParseFloat(
		purchasePriceForReturnLine(false, purPrice, prPrice, legacyPurch, pricingPurch, pricingCost, unitCost, unitPriceCol),
		64,
	)
	if err != nil {
		t.Fatalf("pre-fix purchase_price did not parse as a number: %v", err)
	}
	if preFixPrice != unitCost {
		t.Fatalf("pre-fix purchase_price = %v, want %v (unit_cost fallback, reproducing the documented bug)", preFixPrice, unitCost)
	}
	if preFixPrice*qty == amount {
		t.Fatalf("pre-fix purchase_price x qty unexpectedly equals amount (%v); the bug this test documents should make them differ", amount)
	}

	postFixPrice, err := strconv.ParseFloat(
		purchasePriceForReturnLine(true, purPrice, prPrice, legacyPurch, pricingPurch, pricingCost, unitCost, unitPriceCol),
		64,
	)
	if err != nil {
		t.Fatalf("post-fix purchase_price did not parse as a number: %v", err)
	}
	if postFixPrice != unitPriceCol {
		t.Fatalf("post-fix purchase_price = %v, want %v (PRPrice/unit_price)", postFixPrice, unitPriceCol)
	}
	if got := postFixPrice * qty; got != amount {
		t.Fatalf("post-fix Purchase Price x Qty = %v, want it to equal Amount %v for document 2120's single line", got, amount)
	}
}

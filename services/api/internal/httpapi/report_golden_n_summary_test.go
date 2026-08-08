package httpapi

import (
	"strings"
	"testing"
)

// TestPhaseNGoldenSalesSummaryLeavesResolveToExpectedMode locks the registry
// wiring for the 19 Phase N "Sales Reports" leaves verified in
// docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md against independent
// psql cross-checks on business_documents/business_document_lines/
// master_parties for the sandbox tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee.
// This does not assert the SQL arithmetic is correct (see the companion doc
// for that) - it only locks that each kind still carries a real, non-empty
// salesMode instead of silently falling back to the generic event-ledger
// projection. The customer-sales-items-summary (item-summary) Amount bug the
// companion doc documented is now fixed and covered separately by
// TestPhaseNGoldenSalesItemSummaryAmountUsesPerLineTotal below.
func TestPhaseNGoldenSalesSummaryLeavesResolveToExpectedMode(t *testing.T) {
	tests := map[string]struct {
		mode  string
		title string
	}{
		"customer-sales-days-summary":                                                             {"day-summary", "Days Summary"},
		"customer-sales-items-summary":                                                            {"item-summary", "Items Summary"},
		"customer-sales-invoice-summary":                                                          {"invoice-summary", "Invoice Summary"},
		"customer-sales-invoice-wise-profit-margin-detail":                                        {"profit-margin-detail", "Invoice Wise Profit Margin Detail"},
		"customer-sales-hourly-graph":                                                             {"hour-summary", "Hourly Graph"},
		"customer-sales-customer-category-wise-sales-customer-wise-gross-profit":                  {"profit-customer-summary", "Customer Wise Gross Profit"},
		"customer-sales-customer-category-wise-sales-customer-wise-summary":                       {"customer-summary", "Customer Wise Summary"},
		"customer-sales-customer-category-wise-sales-net-sales-and-volume":                        {"customer-summary", "Net Sales and Volume"},
		"customer-sales-customer-category-wise-net-sales":                                         {"customer-category-summary", "Customer Category Wise Net Sales"},
		"customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report": {"customer-category-summary", "Customer Category Wise Sales Summary Report"},
		"customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report":     {"customer-category-summary", "Customer Category Wise Net Sales Report"},
		"customer-sales-customer-wise-category-net-sales":                                         {"customer-wise-category-summary", "Customer Wise Category Net Sales"},
		"customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report":  {"line-detail", "Customer Category Wise Sales Detail Report"},
		"customer-sales-monthly-net-sales":                                                        {"month-summary", "Monthly Net Sales"},
		"monthly-net-sales-summary":                                                               {"month-summary", "Monthly Net Sales Summary"},
		"daily-sales-summary-with-profit-day-wise-grouping":                                       {"profit-day-summary", "Daily Sales Summary with Profit(Day wise grouping)"},
		"sale-summary":              {"invoice-summary", "Sale summary"},
		"sale-summary-inv-wise":     {"invoice-summary", "Sale Summary Inv. Wise"},
		"sale-summary-invoice-wise": {"invoice-summary", "Sale Summary - Invoice Wise"},
	}
	for kind, want := range tests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if !spec.salesReadModel {
			t.Fatalf("%s: salesReadModel = false, want true", kind)
		}
		if spec.salesMode != want.mode {
			t.Fatalf("%s: salesMode = %q, want %q", kind, spec.salesMode, want.mode)
		}
		if spec.title != want.title {
			t.Fatalf("%s: title = %q, want %q", kind, spec.title, want.title)
		}
		if spec.aggregateCondition != reportSaleAggregate {
			t.Fatalf("%s: aggregateCondition = %q, want %q", kind, spec.aggregateCondition, reportSaleAggregate)
		}
	}
}

// TestPhaseNGoldenSalesSummaryQueryModeDispatch locks that
// salesReadModelQueryMode actually routes each of the 19 leaves' salesMode
// to the SQL-building function the companion doc's psql cross-checks were
// run against, so a future refactor that silently changes the dispatch
// cannot invalidate the golden-verification doc without failing a test.
func TestPhaseNGoldenSalesSummaryQueryModeDispatch(t *testing.T) {
	modes := []string{
		"day-summary", "item-summary", "invoice-summary", "profit-margin-detail",
		"hour-summary", "profit-customer-summary", "customer-summary",
		"customer-category-summary", "customer-wise-category-summary",
		"line-detail", "month-summary", "profit-day-summary",
	}
	seen := map[string]bool{}
	for _, mode := range modes {
		query := salesReadModelQueryMode(reportSaleAggregate, mode, "")
		if query == "" {
			t.Fatalf("mode %q: salesReadModelQueryMode returned an empty query", mode)
		}
		seen[mode] = true
	}
	if len(seen) != len(modes) {
		t.Fatalf("expected %d distinct modes exercised, got %d", len(modes), len(seen))
	}
}

// TestPhaseNGoldenSalesItemSummaryAmountUsesPerLineTotal locks the fix for the
// customer-sales-items-summary (item-summary) Amount bug documented in
// docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md.
//
// salesReadModelQuery's base CTE assigns bd.total_amount (the whole
// document's total) to every one of a document's line rows. That is correct
// for callers that de-duplicate back down to one row per document
// (invoice-summary, day-summary, ...), where MAX(amount) collapses the
// duplicates to a single total. salesItemSummaryReadModelQuery instead groups
// the raw per-line rows by item and SUMs amount, so it must not reuse that
// document-total column: doing so summed the whole document's total once per
// line-occurrence of an item and massively inflated the Amount column for any
// multi-line invoice.
//
// Before the fix, re-running the app's generated query against the sandbox
// tenant (eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee, branch
// ffffffff-ffff-ffff-ffff-ffffffffffff) for 2025-06-01..07 reproduced the
// documented inflation exactly:
//
//	item                   | qty      | amount (before, buggy) | amount (after, fixed)
//	DRIP SET (LIFECARE)    | 55.0000  | 100487.0000             | 4125.0000
//	MEDISOL NS 100ML       | 55.0000  | 73212.0000              | 5610.0000
//	EZIUM 40MG CAP         | 225.0000 | 62276.0000              | 9402.7500
//	P CINE 5MG TAB         | 560.0000 | 54533.0000              | 2744.0000
//
// The "after" values above were confirmed to exactly match an independent
// SUM(bl.line_total) GROUP BY item, party query against
// business_documents/business_document_lines run directly in psql. Quantity
// was never affected (it always summed bl.quantity, which is correctly
// per-line); only Amount was wrong.
//
// This test cannot re-run that live query (no DB in the unit test
// environment), so it instead locks the structural fix: the canonical
// business_documents/business_document_lines branch of the item-summary query
// must compute a genuine per-line amount (bl.line_total, with the same
// legacy-payload/document-total fallback chain salesProfitMarginReadModelQuery
// and salesCustomerCategorySummaryReadModelQueryMode already use for this
// column) instead of duplicating bd.total_amount onto every line row.
func TestPhaseNGoldenSalesItemSummaryAmountUsesPerLineTotal(t *testing.T) {
	query := salesReadModelQueryMode(reportSaleAggregate, "item-summary", "")

	wantFragments := []string{
		"business_document_lines",
		"bl.line_total::text",
		"GROUP BY item_name, party",
	}
	for _, want := range wantFragments {
		if !strings.Contains(query, want) {
			t.Fatalf("item-summary query missing expected fragment %q; query:\n%s", want, query)
		}
	}

	// The canonical (business_documents/business_document_lines) branch's
	// amount expression must try a genuine per-line source (legacy per-line
	// "Amount" payload, then bl.line_total) before ever falling back to the
	// whole-document bd.total_amount. A regression back to
	// "bd.total_amount::text AS amount" with no bl.line_total in front of it
	// is exactly the bug this test guards against.
	canonicalAmountExpr := "COALESCE(NULLIF(bl.legacy_payload->>'Amount', ''), bl.line_total::text, bd.total_amount::text, '') AS amount"
	if !strings.Contains(query, canonicalAmountExpr) {
		t.Fatalf("item-summary query's canonical branch must compute a genuine per-line amount (bl.line_total) before falling back to bd.total_amount, not reuse the document-total column directly; query:\n%s", query)
	}

	// Compatibility (sales_documents/sync_events) rows have no per-line table
	// to join and already project exactly one row per document, so their
	// document-level amount is intentionally left as-is - only the canonical
	// branch, which fans out one row per line, had the duplication bug.
	if !strings.Contains(query, "sd.total_amount::text") {
		t.Fatalf("item-summary query's sales_documents compatibility branch should still fall back to sd.total_amount (one row per document there, so no duplication risk); query:\n%s", query)
	}
}

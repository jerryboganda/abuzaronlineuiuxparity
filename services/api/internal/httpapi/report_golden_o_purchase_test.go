package httpapi

import (
	"strings"
	"testing"
)

// TestPhaseOGoldenPurchaseLeavesResolveToExpectedMode locks the registry
// wiring for the 14 Phase O purchase/supplier leaves verified in
// docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md against independent
// psql cross-checks on business_documents/business_document_lines/
// stock_ledger/party_ledger_entries/master_parties for the sandbox tenant
// eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee.
func TestPhaseOGoldenPurchaseLeavesResolveToExpectedMode(t *testing.T) {
	tests := map[string]struct {
		mode      string
		title     string
		aggregate string
	}{
		"purchase-summary":                         {"invoice-summary", "Purchase summary", "se.aggregate = 'receiving'"},
		"purchase-summary2":                        {"invoice-summary", "Purchase Summary2", "se.aggregate = 'receiving'"},
		"purchase-order-summary":                   {"invoice-summary", "Purchase Order Summary", "se.aggregate = 'purchase_order'"},
		"purchase-order":                           {"invoice-summary", "Purchase Order", "se.aggregate = 'purchase_order'"},
		"periodic-purchases":                       {"month-summary", "Periodic Purchases", "se.aggregate = 'receiving'"},
		"purchase-order-supplier-wise":             {"supplier-summary", "Purchase Order Supplier Wise", "se.aggregate = 'purchase_order'"},
		"net-purchase-summary":                     {"invoice-summary", "Net Purchase Summary", "se.aggregate = 'receiving'"},
		"days-summary":                             {"day-summary", "Days Summary", "se.aggregate = 'receiving'"},
		"category-wise-purchase":                   {"item-summary", "Category Wise Purchase", "se.aggregate = 'receiving'"},
		"monthly-purchase-graph":                   {"month-summary", "Monthly Purchase Graph", "se.aggregate = 'receiving'"},
		"manufacturer-wise-monthly-stock-movement": {"manufacturer-month-summary", "Monthly Stock Movement", "se.aggregate = 'receiving'"},
		"manufacturer-wise-detail":                 {"manufacturer-detail", "Detail", "se.aggregate = 'receiving'"},
		"supplier-manufacturer-wise-g-p":           {"supplier-manufacturer-summary", "Supplier/Manufacturer Wise G/P", "se.aggregate = 'receiving'"},
		"supplier-purchase-returns-summary":        {"supplier-summary", "Summary", "se.aggregate = 'return'"},
	}
	for kind, want := range tests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if !spec.purchaseReadModel {
			t.Fatalf("%s: purchaseReadModel = false, want true", kind)
		}
		if spec.purchaseMode != want.mode {
			t.Fatalf("%s: purchaseMode = %q, want %q", kind, spec.purchaseMode, want.mode)
		}
		if spec.title != want.title {
			t.Fatalf("%s: title = %q, want %q", kind, spec.title, want.title)
		}
		if spec.aggregateCondition != want.aggregate {
			t.Fatalf("%s: aggregateCondition = %q, want %q", kind, spec.aggregateCondition, want.aggregate)
		}
	}
}

// TestPhaseOGoldenPurchaseLeavesReportEventLedgerProjectionStatus documents
// the precise (and easy to misread) current state: reportDefinitionForKey
// never flips projectionStatus to "real" for a purchaseReadModel spec unless
// purchaseMode == "po-disparity" (see the concreteProjection expression in
// reportDefinitionForKey). All 14 leaves below therefore report
// ProjectionStatus == "event-ledger" in the API metadata today, even though
// (per TestPhaseOGoldenPurchaseModesUseRealPerModeGroupedQueries and the
// psql cross-checks in the companion doc) the query actually executed for
// each of them performs genuine per-mode SQL grouping/aggregation, not a
// generic passthrough of sync_events. Promoting these to projectionStatus
// "real" is a metadata/labeling decision for a future phase, not a
// correctness fix - the underlying arithmetic already reconciles.
func TestPhaseOGoldenPurchaseLeavesReportEventLedgerProjectionStatus(t *testing.T) {
	kinds := []string{
		"purchase-summary", "purchase-summary2", "purchase-order-summary",
		"purchase-order", "periodic-purchases", "purchase-order-supplier-wise",
		"net-purchase-summary", "days-summary", "category-wise-purchase",
		"monthly-purchase-graph", "manufacturer-wise-monthly-stock-movement",
		"manufacturer-wise-detail", "supplier-manufacturer-wise-g-p",
		"supplier-purchase-returns-summary",
	}
	for _, kind := range kinds {
		def := reportDefinitionFor(kind)
		if def.ProjectionStatus != "event-ledger" {
			t.Errorf("%s: ProjectionStatus = %q, want %q (update this test and the companion doc if a promotion pass changed this)", kind, def.ProjectionStatus, "event-ledger")
		}
	}
}

// TestPhaseOGoldenPurchaseModesUseRealPerModeGroupedQueries documents the
// query-dispatch finding backing the psql cross-checks in
// docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md: purchaseReadModelQueryMode
// routes each mode to a dedicated grouped-aggregation query (not the generic
// sync_events passthrough at the bottom of the report handler), and every
// mode's grouped totals were independently reproduced via a differently
// constructed psql query against business_documents/business_document_lines/
// stock_ledger/party_ledger_entries/master_parties for the sandbox tenant and
// matched exactly:
//   - invoice-summary (receiving):      6,418 docs, qty 4,860,933.0000, amount 198,071,256.0000
//   - invoice-summary (purchase_order): 2,810 docs, qty   375,195.0000, amount           0.0000
//   - month-summary (receiving):        19/19 calendar months matched exactly (Jan 2025 - Jul 2026)
//   - day-summary (receiving), 2025-12-11 spot check: qty 10,050.0000, amount 490,133.0000
//   - supplier-summary (receiving):    113 suppliers, qty 4,860,933.0000, amount 198,071,256.0000
//   - supplier-summary (purchase_order): 44 suppliers, qty  375,195.0000, amount           0.0000
//   - supplier-summary (return):        55 suppliers, qty  57,868.0000, amount   3,526,551.0000
//   - item-summary (receiving):     14,395 item/party rows, qty 4,860,933.0000, amount 2,702,608,926.7900
//   - detail (receiving):          113,526 line rows (113,525 real + 1 benign LEFT JOIN null-line
//     artifact for a zero-line document), qty 4,860,933.0000, amount 2,702,608,926.7900
func TestPhaseOGoldenPurchaseModesUseRealPerModeGroupedQueries(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	tests := []struct {
		mode        string
		aggregate   string
		wantSQL     []string
		notWantText string
	}{
		{"invoice-summary", "se.aggregate = 'receiving'", []string{"GROUP BY", "business_documents", "business_document_lines", "party_ledger_entries", "d.status = 'posted'"}, ""},
		{"month-summary", "se.aggregate = 'receiving'", []string{"grouped_rows", "date_trunc('month'", "GROUP BY"}, ""},
		{"day-summary", "se.aggregate = 'receiving'", []string{"grouped_rows", "occurred_at::date", "GROUP BY"}, ""},
		{"item-summary", "se.aggregate = 'receiving'", []string{"grouped_rows", "GROUP BY category", "master_categories"}, ""},
		{"supplier-summary", "se.aggregate = 'purchase_order'", []string{"grouped_rows", "GROUP BY party"}, ""},
		{"detail", "se.aggregate = 'receiving'", []string{"l.item_name", "l.line_total"}, ""},
	}
	for _, tc := range tests {
		query := purchaseReadModelQueryMode(tc.aggregate, tc.mode, pagination)
		for _, want := range tc.wantSQL {
			if !strings.Contains(query, want) {
				t.Errorf("mode %q aggregate %q: query missing expected fragment %q:\n%s", tc.mode, tc.aggregate, want, query)
			}
		}
		// A truly generic-fallback query never references the canonical
		// purchase tables at all; assert this real per-mode query does.
		if !strings.Contains(query, "business_documents") {
			t.Errorf("mode %q aggregate %q: query does not read business_documents (looks like a generic fallback, not a real per-mode projection)", tc.mode, tc.aggregate)
		}
	}
}

// TestPhaseOGoldenPurchaseLeafGroupingDimensionsNowMatchTheirLegacyNames
// documents the fix (2026-08-09 bug-fix wave) for the naming/fidelity gap
// this test used to lock as an open bug: "category-wise-purchase"
// (item-summary mode) previously grouped by item name, not item category;
// "manufacturer-wise-detail" and "manufacturer-wise-monthly-stock-movement"
// performed no manufacturer join/grouping at all. Both are now fixed -
// category-wise-purchase groups by resolved category via master_categories
// (docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_A_2026-08-09.md), and the two
// manufacturer-wise leaves got dedicated "manufacturer-detail"/
// "manufacturer-month-summary" modes with a real master_manufacturers join
// (docs/PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md).
// "supplier-manufacturer-wise-g-p" gained a manufacturer dimension on top of
// its existing supplier grouping, but still computes no gross-profit figure
// - that part remains a documented, undecided gap (no purchase-side G/P
// formula could be verified against real data; see the code comment above
// purchaseSupplierManufacturerSummaryReadModelQuery).
func TestPhaseOGoldenPurchaseLeafGroupingDimensionsNowMatchTheirLegacyNames(t *testing.T) {
	itemSummaryQuery := purchaseReadModelQueryMode("se.aggregate = 'receiving'", "item-summary", "")
	if !strings.Contains(itemSummaryQuery, "master_categories") {
		t.Fatalf("category-wise-purchase (item-summary mode) expected to join master_categories (the 2026-08-09 fix); query:\n%s", itemSummaryQuery)
	}
	if strings.Contains(itemSummaryQuery, "GROUP BY item, party") {
		t.Fatal("category-wise-purchase (item-summary mode) still groups by item, party - the category fix appears to have been reverted")
	}

	manufacturerDetailQuery := purchaseReadModelQueryMode("se.aggregate = 'receiving'", "manufacturer-detail", "")
	if !strings.Contains(manufacturerDetailQuery, "master_manufacturers") {
		t.Fatalf("manufacturer-wise-detail (manufacturer-detail mode) expected to join master_manufacturers (the 2026-08-09 fix); query:\n%s", manufacturerDetailQuery)
	}

	// The plain "detail" mode (supplier-wise-detail, supplier-wise-purchase-detail)
	// must NOT have gained the manufacturer join - only manufacturer-wise-detail's
	// own dedicated mode should.
	detailQuery := purchaseReadModelQueryMode("se.aggregate = 'receiving'", "detail", "")
	if strings.Contains(detailQuery, "master_manufacturers") {
		t.Fatal("plain detail mode unexpectedly joins master_manufacturers - this should only happen for the dedicated manufacturer-detail mode")
	}
}

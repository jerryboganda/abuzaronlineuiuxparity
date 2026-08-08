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
		"manufacturer-wise-monthly-stock-movement": {"month-summary", "Monthly Stock Movement", "se.aggregate = 'receiving'"},
		"manufacturer-wise-detail":                 {"detail", "Detail", "se.aggregate = 'receiving'"},
		"supplier-manufacturer-wise-g-p":           {"supplier-summary", "Supplier/Manufacturer Wise G/P", "se.aggregate = 'receiving'"},
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
		{"item-summary", "se.aggregate = 'receiving'", []string{"grouped_rows", "GROUP BY item, party"}, ""},
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

// TestPhaseOGoldenPurchaseLeafGroupingDimensionsDoNotMatchTheirLegacyNames
// documents a naming/fidelity gap surfaced during golden verification:
// "category-wise-purchase" (item-summary mode) groups by item name, not by
// item category; "manufacturer-wise-detail" and
// "manufacturer-wise-monthly-stock-movement" perform no manufacturer
// join/grouping at all (they are identical, generically, to the plain
// detail/month-summary modes); and "supplier-manufacturer-wise-g-p" groups
// by supplier only and computes no gross-profit/gross-purchase figure. See
// the companion doc for what a real per-leaf projection would require.
func TestPhaseOGoldenPurchaseLeafGroupingDimensionsDoNotMatchTheirLegacyNames(t *testing.T) {
	itemSummaryQuery := purchaseReadModelQueryMode("se.aggregate = 'receiving'", "item-summary", "")
	if !strings.Contains(itemSummaryQuery, "GROUP BY item, party") {
		t.Fatalf("category-wise-purchase (item-summary mode) expected to group by item, party; query:\n%s", itemSummaryQuery)
	}
	if strings.Contains(itemSummaryQuery, "ManfCode") || strings.Contains(itemSummaryQuery, "ICatCode") || strings.Contains(itemSummaryQuery, "master_items") {
		t.Fatalf("category-wise-purchase (item-summary mode) unexpectedly joins item category/manufacturer payload; if this now exists, update the companion doc")
	}

	detailQuery := purchaseReadModelQueryMode("se.aggregate = 'receiving'", "detail", "")
	if strings.Contains(detailQuery, "master_items") || strings.Contains(detailQuery, "ManfCode") {
		t.Fatalf("manufacturer-wise-detail (detail mode) unexpectedly joins manufacturer payload; if this now exists, update the companion doc")
	}
}

package httpapi

import (
	"strings"
	"testing"
)

// TestPhaseNORemainderLeavesResolveToExpectedMode locks the registry wiring
// for the 7 Phase N/O "remainder" leaves verified in
// docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_REMAINDER_2026-08-09.md:
//   - 2 Phase N Sales Reports leaves (
//     sale-return-summary-inv-type-wise, dead-item-list) that fall through
//     the salesMode switch in phaseNReportRegistry with no case, so
//     spec.salesMode == "" for all three.
//   - 4 Phase O purchaseReadModel leaves (purchase-return-summary,
//     supplier-wise-detail, supplier-wise-purchase-detail,
//     supplier-purchase-returns-detail) not covered by
//     docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md's 14-leaf set.
func TestPhaseNORemainderLeavesResolveToExpectedMode(t *testing.T) {
	salesTests := map[string]struct {
		title     string
		aggregate string
	}{
		"sale-return-summary-inv-type-wise": {"Sale/Return Summary Inv. Type Wise", reportSaleOrReturn},
		"dead-item-list":                    {"Dead Item List", reportSaleAggregate},
	}
	for kind, want := range salesTests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if !spec.salesReadModel {
			t.Fatalf("%s: salesReadModel = false, want true", kind)
		}
		if spec.salesMode != "" {
			t.Fatalf("%s: salesMode = %q, want \"\" (no dedicated projection mode wired)", kind, spec.salesMode)
		}
		if spec.title != want.title {
			t.Fatalf("%s: title = %q, want %q", kind, spec.title, want.title)
		}
		if spec.aggregateCondition != want.aggregate {
			t.Fatalf("%s: aggregateCondition = %q, want %q", kind, spec.aggregateCondition, want.aggregate)
		}
	}

	purchaseTests := map[string]struct {
		mode      string
		title     string
		aggregate string
	}{
		"purchase-return-summary":          {"invoice-summary", "Purchase Return summary", "se.aggregate = 'return'"},
		"supplier-wise-detail":             {"detail", "Detail", "se.aggregate = 'receiving'"},
		"supplier-wise-purchase-detail":    {"detail", "Purchase Detail", "se.aggregate = 'receiving'"},
		"supplier-purchase-returns-detail": {"detail", "Detail", "se.aggregate = 'return'"},
	}
	for kind, want := range purchaseTests {
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

// TestPhaseNORemainderLeavesReportEventLedgerProjectionStatus locks the
// current API metadata (remaining unpromoted leaves still report "event-ledger") so a
// future promotion pass updates this test and the companion doc together
// instead of silently drifting.
func TestPhaseNORemainderLeavesReportEventLedgerProjectionStatus(t *testing.T) {
	kinds := []string{
		"sale-return-summary-inv-type-wise",
		"dead-item-list",
		"purchase-return-summary",
		"supplier-wise-detail",
		"supplier-wise-purchase-detail",
		"supplier-purchase-returns-detail",
	}
	for _, kind := range kinds {
		def := reportDefinitionFor(kind)
		if def.ProjectionStatus != "event-ledger" {
			t.Errorf("%s: ProjectionStatus = %q, want %q (update this test and the companion doc if a promotion pass changed this)", kind, def.ProjectionStatus, "event-ledger")
		}
	}
}

// TestPhaseNORemainderSalesLeavesUseGenericRawPassthrough documents that
// the remaining empty-mode N leaves dispatch to the plain salesReadModelQuery (the
// same raw per-line/per-document union used by the generic fallback path),
// not any dedicated item-aggregation, invoice-type-grouping, or
// last-sale-date projection - there is no query-level implementation of
// "net sales per item", "summary by invoice type", or "items with no
// recent sale" for these three leaves today.
func TestPhaseNORemainderSalesLeavesUseGenericRawPassthrough(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	emptyModeQuery := salesReadModelQueryMode(reportSaleAggregate, "", pagination)
	rawQuery := salesReadModelQuery(reportSaleAggregate, pagination)
	if emptyModeQuery != rawQuery {
		t.Fatalf("salesReadModelQueryMode with mode=\"\" is expected to return the exact same text as salesReadModelQuery (the raw passthrough); got a difference, which would mean a dedicated mode now exists for empty-mode leaves and this test/doc should be updated")
	}
	// A genuine item/invoice-type/dead-item aggregation would GROUP BY
	// something; the raw passthrough never groups at all.
	if strings.Contains(rawQuery, "GROUP BY") {
		t.Fatalf("salesReadModelQuery unexpectedly contains GROUP BY; if a real aggregation was added for empty-mode leaves, update the companion doc")
	}
	// No item-master join exists in the raw passthrough, so no legacy
	// category/last-sold-date/dead-item signal is available to it.
	if strings.Contains(rawQuery, "master_items") {
		t.Fatalf("salesReadModelQuery unexpectedly joins master_items; if dead-item-list now has a real per-item last-sale-date projection, update the companion doc")
	}
}

// TestPhaseNORemainderPurchaseDetailModeLeavesAreByteIdenticalQueries
// documents that supplier-wise-detail and supplier-wise-purchase-detail
// (mode="detail", aggregate="receiving") dispatch to purchaseReadModelQueryMode
// with identical parameters and therefore produce byte-identical SQL text
// and results - both were golden-verified in
// docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md /
// docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_REMAINDER_2026-08-09.md
// (113,526 line rows, qty 4,860,933.0000, amount 2,702,608,926.7900).
//
// manufacturer-wise-detail used to share this exact identity (same
// mode/aggregate, no manufacturer-specific column) until the 2026-08-09
// bug-fix wave gave it its own dedicated "manufacturer-detail" mode with a
// real master_manufacturers join (see
// docs/evidence/PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md) - it is no
// longer byte-identical to the other two leaves by design, so this test now
// only asserts identity between the two that still share it, plus that
// manufacturer-wise-detail has genuinely diverged (still receiving-scoped,
// but with its own mode and a manufacturer join).
func TestPhaseNORemainderPurchaseDetailModeLeavesAreByteIdenticalQueries(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	receiving := "se.aggregate = 'receiving'"

	supplierWiseDetailSpec, ok := reportSpecForKey("supplier-wise-detail")
	if !ok {
		t.Fatal("supplier-wise-detail: not present in the report registry")
	}
	supplierWisePurchaseDetailSpec, ok := reportSpecForKey("supplier-wise-purchase-detail")
	if !ok {
		t.Fatal("supplier-wise-purchase-detail: not present in the report registry")
	}
	manufacturerWiseDetailSpec, ok := reportSpecForKey("manufacturer-wise-detail")
	if !ok {
		t.Fatal("manufacturer-wise-detail: not present in the report registry")
	}

	if supplierWiseDetailSpec.purchaseMode != "detail" || supplierWiseDetailSpec.aggregateCondition != receiving {
		t.Fatalf("supplier-wise-detail: mode/aggregate = %q/%q, want detail/%q", supplierWiseDetailSpec.purchaseMode, supplierWiseDetailSpec.aggregateCondition, receiving)
	}
	if supplierWisePurchaseDetailSpec.purchaseMode != "detail" || supplierWisePurchaseDetailSpec.aggregateCondition != receiving {
		t.Fatalf("supplier-wise-purchase-detail: mode/aggregate = %q/%q, want detail/%q", supplierWisePurchaseDetailSpec.purchaseMode, supplierWisePurchaseDetailSpec.aggregateCondition, receiving)
	}
	if manufacturerWiseDetailSpec.purchaseMode != "manufacturer-detail" || manufacturerWiseDetailSpec.aggregateCondition != receiving {
		t.Fatalf("manufacturer-wise-detail: mode/aggregate = %q/%q, want manufacturer-detail/%q (the 2026-08-09 fix) - if this reverted, the manufacturer join was lost", manufacturerWiseDetailSpec.purchaseMode, manufacturerWiseDetailSpec.aggregateCondition, receiving)
	}

	supplierWiseDetailQuery := purchaseReadModelQueryMode(supplierWiseDetailSpec.aggregateCondition, supplierWiseDetailSpec.purchaseMode, pagination)
	supplierWisePurchaseDetailQuery := purchaseReadModelQueryMode(supplierWisePurchaseDetailSpec.aggregateCondition, supplierWisePurchaseDetailSpec.purchaseMode, pagination)
	manufacturerWiseDetailQuery := purchaseReadModelQueryMode(manufacturerWiseDetailSpec.aggregateCondition, manufacturerWiseDetailSpec.purchaseMode, pagination)

	if supplierWiseDetailQuery != supplierWisePurchaseDetailQuery {
		t.Fatalf("supplier-wise-detail query text differs from supplier-wise-purchase-detail; these were expected to be byte-identical (same mode/aggregate, no supplier-specific column exists)")
	}
	if !strings.Contains(manufacturerWiseDetailQuery, "master_manufacturers") {
		t.Fatalf("manufacturer-wise-detail query no longer joins master_manufacturers - the 2026-08-09 fix appears to have been lost:\n%s", manufacturerWiseDetailQuery)
	}
	if manufacturerWiseDetailQuery == supplierWiseDetailQuery {
		t.Fatal("manufacturer-wise-detail is byte-identical to supplier-wise-detail again - expected it to have diverged with its own manufacturer join since the 2026-08-09 fix")
	}
}

// TestPhaseNORemainderPurchaseReturnSummaryUsesRealInvoiceGroupedQuery
// documents that purchase-return-summary (mode="invoice-summary",
// aggregate="return") dispatches to the shared genuine per-document grouped
// SQL (purchaseReadModelQuery), not a raw passthrough - matching the
// invoice-summary-mode finding already established for purchase-summary/
// purchase-summary2/net-purchase-summary (receiving aggregate) in
// docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md. Golden totals
// independently reproduced for this leaf (see companion doc):
// 634 documents, qty 57,868.0000, amount 3,526,551.0000 - MATCHED exactly
// against an independent CTE-based cross-check.
func TestPhaseNORemainderPurchaseReturnSummaryUsesRealInvoiceGroupedQuery(t *testing.T) {
	spec, ok := reportSpecForKey("purchase-return-summary")
	if !ok {
		t.Fatal("purchase-return-summary: not present in the report registry")
	}
	if !isPurchaseSummaryMode(spec.purchaseMode) {
		t.Fatalf("purchase-return-summary: purchaseMode %q is not a recognized purchase summary mode", spec.purchaseMode)
	}
	query := purchaseReadModelQueryMode(spec.aggregateCondition, spec.purchaseMode, "LIMIT $6 OFFSET $7")
	for _, want := range []string{"GROUP BY", "business_documents", "party_ledger_entries", "'purchase-return'", "d.status = 'posted'"} {
		if !strings.Contains(query, want) {
			t.Errorf("purchase-return-summary query missing expected fragment %q:\n%s", want, query)
		}
	}
}

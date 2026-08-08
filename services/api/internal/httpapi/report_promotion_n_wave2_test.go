package httpapi

import (
	"testing"
)

// TestSaleDetailInvWiseReportUsesLineDetailProjection locks in the wave-2
// promotion of "Daily Reports > Sale > Sale Detail Inv. Wise" (registry key
// sale-detail-inv-wise) from the generic six-column event-ledger grid to the
// same source-backed line-detail contract already verified for its sibling
// "sale-detail" (Daily Reports > Sale > Sale detail). This mirrors how the
// registry already treats "sale-summary-inv-wise" as reusing "sale-summary"'s
// mode two cases above in phaseNReportRegistry: the "-inv-wise" suffix
// changes sort/grouping, not the column contract, so the query text must be
// byte-identical to "sale-detail"'s.
//
// Note: this wave also examined "sales-return-detail-inv-wise" and
// "selected-sales-and-summaries-report" (both reuse of existing modes) and
// "category-wise-item-category-wise-monthly-sales" (a new mode backed by a
// verified master_categories join) as promotion candidates and found
// supporting evidence for all three (see
// docs/PHASE_N_REPORT_PROMOTION_WAVE2_2026-08-09.md), but left them
// unpromoted this pass: the first two collide with a concurrently-written
// diagnostic test file (report_golden_n_salereturn_test.go) that already
// pins both leaves' current salesMode == "", and the third would require
// adding a new salesMode value to the closed allow-list switch in the
// pre-existing TestPhaseNReportRegistryDefinitionsAndAggregateFilters
// (server_test.go) -- this pass is not permitted to edit existing _test.go
// files, so none of the three could be safely promoted without breaking a
// test this pass cannot touch.
func TestSaleDetailInvWiseReportUsesLineDetailProjection(t *testing.T) {
	spec, ok := reportSpecForKey("sale-detail-inv-wise")
	if !ok || !spec.salesReadModel || spec.salesMode != "line-detail" {
		t.Fatalf("sale-detail-inv-wise spec = %+v (ok=%v), want line-detail sales projection", spec, ok)
	}
	if spec.aggregateCondition != reportSaleAggregate {
		t.Fatalf("sale-detail-inv-wise aggregate condition = %q, want %q", spec.aggregateCondition, reportSaleAggregate)
	}

	query := salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
	if want := salesReadModelQueryMode(reportSaleAggregate, "line-detail", "LIMIT $6 OFFSET $7"); query != want {
		t.Errorf("sale-detail-inv-wise query diverges from the sale-detail line-detail query it is meant to reuse")
	}

	definition := reportDefinitionFor("sale-detail-inv-wise")
	if definition.Title != "Sale Detail Inv. Wise" {
		t.Errorf("sale-detail-inv-wise title = %q, want %q", definition.Title, "Sale Detail Inv. Wise")
	}
	if len(definition.Columns) != 11 || definition.Columns[0].Key != "alias" || definition.Columns[0].Label != "Alias" {
		t.Errorf("sale-detail-inv-wise definition columns = %+v, want the 11-field dailySaleDetailColumns contract", definition.Columns)
	}
	if definition.ProjectionStatus != "event-ledger" {
		t.Errorf("sale-detail-inv-wise projection status = %q, want event-ledger (real column shape, event-ledger status by design)", definition.ProjectionStatus)
	}
}

// TestPhaseNWave2PromotedLeavesStillSatisfyRegistryInvariant guards that the
// wave-2 promotion above cannot silently regress the existing registry-wide
// invariant test (TestPhaseNReportRegistryDefinitionsAndAggregateFilters in
// server_test.go), which expects literal "event-ledger" projection status
// for every phaseNReportRegistry leaf that is not a documentReadModel or
// financeMode leaf.
func TestPhaseNWave2PromotedLeavesStillSatisfyRegistryInvariant(t *testing.T) {
	for _, kind := range []string{
		"sale-detail-inv-wise",
	} {
		spec, ok := phaseNReportRegistry[kind]
		if !ok {
			t.Fatalf("%s missing from phaseNReportRegistry", kind)
		}
		if spec.documentReadModel || spec.financeMode != "" {
			t.Fatalf("%s unexpectedly gained documentReadModel/financeMode; update this guard deliberately if that is intended", kind)
		}
		if definition := reportDefinitionFor(kind); definition.ProjectionStatus != "event-ledger" {
			t.Errorf("%s projection status = %q, want event-ledger", kind, definition.ProjectionStatus)
		}
	}
}

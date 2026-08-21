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
// Later sibling promotion (TestPhaseNSiblingLeavesReuseExistingSalesModes)
// reuses existing line-detail / invoice-summary / hour-summary / item-summary
// contracts for additional N leaves. category-wise monthly sales still needs
// a new salesMode and remains unpromoted.
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
func TestPhaseNSiblingLeavesReuseExistingSalesModes(t *testing.T) {
	cases := []struct {
		kind string
		mode string
	}{
		{"sale-detail-format-2", "line-detail"},
		{"sale-detail-inv-wise-with-diff-col", "line-detail"},
		{"sale-summary-inv-cust-wise", "invoice-summary"},
		{"sale-summary-machine-and-invoice-range-wise", "invoice-summary"},
		{"selected-sales-and-summaries-report", "invoice-summary"},
		{"hourly-sales-graph", "hour-summary"},
		{"item-wise-item-wise-net-sales", "item-summary"},
		{"slow-fast-moving-items", "item-summary"},
		{"net-sale-summary", "invoice-summary"},
	}
	for _, test := range cases {
		spec, ok := reportSpecForKey(test.kind)
		if !ok || !spec.salesReadModel || spec.salesMode != test.mode {
			t.Fatalf("%s spec = %+v (ok=%v), want salesMode %s", test.kind, spec, ok, test.mode)
		}
		query := salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
		want := salesReadModelQueryMode(reportSaleAggregate, test.mode, "LIMIT $6 OFFSET $7")
		if query != want {
			t.Errorf("%s query diverges from the shared %s contract", test.kind, test.mode)
		}
		definition := reportDefinitionFor(test.kind)
		if definition.ProjectionStatus != "event-ledger" {
			t.Errorf("%s projection status = %q, want event-ledger", test.kind, definition.ProjectionStatus)
		}
		if test.mode == "line-detail" {
			if len(definition.Columns) != 11 || definition.Columns[0].Label != "Alias" {
				t.Errorf("%s columns = %+v, want line-detail", test.kind, definition.Columns)
			}
		} else if len(definition.Columns) != 6 || definition.Columns[0].Label == "Event / Document" {
			t.Errorf("%s columns = %+v, want %s summary", test.kind, definition.Columns, test.mode)
		}
	}
}

func TestPhaseNReturnActivityReusesItemSummary(t *testing.T) {
	spec, ok := reportSpecForKey("item-wise-item-sale-and-return-activity")
	if !ok || !spec.salesReadModel || spec.salesMode != "item-summary" {
		t.Fatalf("item-wise-item-sale-and-return-activity spec = %+v (ok=%v), want item-summary", spec, ok)
	}
	query := salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
	want := salesReadModelQueryMode(spec.aggregateCondition, "item-summary", "LIMIT $6 OFFSET $7")
	if query != want {
		t.Errorf("item-wise-item-sale-and-return-activity query diverges from item-summary")
	}
	if definition := reportDefinitionFor("item-wise-item-sale-and-return-activity"); definition.ProjectionStatus != "event-ledger" {
		t.Errorf("projection status = %q, want event-ledger", definition.ProjectionStatus)
	}
}

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

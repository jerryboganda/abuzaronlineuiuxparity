package httpapi

import (
	"strings"
	"testing"
)

// TestCustomerSalesDetailReportUsesLineDetailProjection locks in the Phase N
// promotion of the "Reports > Sales Reports > Customer Sales > Detail" leaf
// (registry key customer-sales-detail) from the generic six-column
// event-ledger grid to the same source-backed line-detail contract already
// verified for the sibling "sale-detail" leaf (Daily Reports > Sale > Sale
// detail) and for customer-sales-customer-category-wise-sales-customer-
// category-wise-sales-detail-report. The underlying query, column shape, and
// aggregate filter are unchanged infrastructure reused via salesMode; no new
// SQL or column function was introduced for this leaf.
func TestCustomerSalesDetailReportUsesLineDetailProjection(t *testing.T) {
	spec, ok := reportSpecForKey("customer-sales-detail")
	if !ok || !spec.salesReadModel || spec.salesMode != "line-detail" {
		t.Fatalf("customer-sales-detail spec = %+v (ok=%v), want line-detail sales projection", spec, ok)
	}
	if spec.aggregateCondition != reportSaleAggregate {
		t.Fatalf("customer-sales-detail aggregate condition = %q, want %q", spec.aggregateCondition, reportSaleAggregate)
	}

	query := salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"business_document_lines",
		"AliasName",
		"sales_detail_read_model",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("customer-sales-detail query is missing %q", fragment)
		}
	}
	// Reusing the exact line-detail mode/aggregate pair means the query text
	// must be byte-identical to the already-verified sale-detail leaf.
	if want := salesReadModelQueryMode(reportSaleAggregate, "line-detail", "LIMIT $6 OFFSET $7"); query != want {
		t.Errorf("customer-sales-detail query diverges from the sale-detail line-detail query it is meant to reuse")
	}

	definition := reportDefinitionFor("customer-sales-detail")
	if definition.Title != "Detail" {
		t.Errorf("customer-sales-detail title = %q, want %q", definition.Title, "Detail")
	}
	if len(definition.Columns) != 11 || definition.Columns[0].Key != "alias" || definition.Columns[0].Label != "Alias" {
		t.Errorf("customer-sales-detail definition columns = %+v, want the 11-field dailySaleDetailColumns contract", definition.Columns)
	}
	// The literal projectionStatus string intentionally stays "event-ledger"
	// for salesReadModel leaves (see TestPhaseNReportRegistryDefinitionsAndAggregateFilters);
	// only the column/query shape changes on promotion. Asserting it here
	// documents that this is expected, not a regression.
	if definition.ProjectionStatus != "event-ledger" {
		t.Errorf("customer-sales-detail projection status = %q, want event-ledger (real column shape, event-ledger status by design)", definition.ProjectionStatus)
	}
	if !strings.Contains(definition.ProjectionNote, "line-detail contract") {
		t.Errorf("customer-sales-detail projection note = %q, want it to describe the line-detail contract", definition.ProjectionNote)
	}
}

// TestCustomerSalesSummaryReportUsesInvoiceSummaryProjection locks in the
// Phase N promotion of the "Reports > Sales Reports > Customer Sales >
// Summary" leaf (registry key customer-sales-summary) from the generic
// six-column event-ledger grid to the same one-row-per-invoice contract
// already verified for the sibling "sale-summary" leaf (Daily Reports > Sale
// > Sale summary). The underlying query, column shape, and aggregate filter
// are unchanged infrastructure reused via salesMode.
func TestCustomerSalesSummaryReportUsesInvoiceSummaryProjection(t *testing.T) {
	spec, ok := reportSpecForKey("customer-sales-summary")
	if !ok || !spec.salesReadModel || spec.salesMode != "invoice-summary" {
		t.Fatalf("customer-sales-summary spec = %+v (ok=%v), want invoice-summary sales projection", spec, ok)
	}
	if spec.aggregateCondition != reportSaleAggregate {
		t.Fatalf("customer-sales-summary aggregate condition = %q, want %q", spec.aggregateCondition, reportSaleAggregate)
	}

	query := salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"invoice_rows",
		"GROUP BY document",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("customer-sales-summary query is missing %q", fragment)
		}
	}
	if want := salesReadModelQueryMode(reportSaleAggregate, "invoice-summary", "LIMIT $6 OFFSET $7"); query != want {
		t.Errorf("customer-sales-summary query diverges from the sale-summary invoice-summary query it is meant to reuse")
	}

	definition := reportDefinitionFor("customer-sales-summary")
	if definition.Title != "Summary" {
		t.Errorf("customer-sales-summary title = %q, want %q", definition.Title, "Summary")
	}
	if len(definition.Columns) != 6 || definition.Columns[0].Key != "document" || definition.Columns[0].Label != "Invoice" {
		t.Errorf("customer-sales-summary definition columns = %+v, want the invoice-summary contract", definition.Columns)
	}
	if definition.ProjectionStatus != "event-ledger" {
		t.Errorf("customer-sales-summary projection status = %q, want event-ledger (real column shape, event-ledger status by design)", definition.ProjectionStatus)
	}
	if !strings.Contains(definition.ProjectionNote, "invoice-summary") {
		t.Errorf("customer-sales-summary projection note = %q, want it to describe the invoice-summary projection", definition.ProjectionNote)
	}
}

// TestPhaseNPromotedLeavesStillSatisfyRegistryInvariant is a guard so the
// two promotions above cannot silently regress the existing Phase N
// registry-wide invariant test (TestPhaseNReportRegistryDefinitionsAndAggregateFilters
// in server_test.go), which still expects literal "event-ledger" status for
// every phaseNReportRegistry leaf that is not a documentReadModel or
// financeMode leaf.
func TestPhaseNPromotedLeavesStillSatisfyRegistryInvariant(t *testing.T) {
	for _, kind := range []string{"customer-sales-detail", "customer-sales-summary"} {
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

package httpapi

import (
	"strings"
	"testing"
)

func TestSalesReadModelIncludesCanonicalLinesAndPartyScope(t *testing.T) {
	query := salesReadModelQuery(reportSaleAggregate, "LIMIT $6")
	for _, fragment := range []string{
		"FROM business_documents bd",
		"business_document_lines bl",
		"master_parties mp",
		"bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid",
		"bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id",
		"bd.total_amount::text AS amount",
		"bd.status = 'posted'",
		"sd.status = 'posted'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("canonical sales read model is missing %q", fragment)
		}
	}
}

func TestSalesReadModelDoesNotDuplicateCompatibilityDocuments(t *testing.T) {
	query := salesReadModelQuery(reportSaleAggregate, "LIMIT $6")
	for _, fragment := range []string{
		"UNION ALL",
		"NOT EXISTS (",
		"FROM business_documents bd",
		"FROM sales_documents sd",
		"bd.id = sd.id OR bd.document_number = sd.document_number",
		"sd.id = se.aggregate_id",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("sales read model is missing duplicate guard %q", fragment)
		}
	}
	if strings.Contains(query, "se.aggregate IN ('sale', 'sale_return')") {
		t.Fatal("sale-only read model unexpectedly included sale returns")
	}
	if !strings.Contains(salesReadModelQuery(reportSaleOrReturn, "LIMIT $6"), "se.aggregate IN ('sale', 'sale_return')") {
		t.Fatal("sale/return read model did not include both compatibility aggregates")
	}
	combined := salesReadModelQuery(reportSaleOrReturn, "LIMIT $6")
	if !strings.Contains(combined, "'open-cash-return', 'open-credit-return'") {
		t.Fatal("sale/return read model did not include canonical open returns")
	}
}

func TestSaleReturnReadModelIncludesCanonicalAndCompatibilitySources(t *testing.T) {
	query := saleReturnReadModelQuery("LIMIT $6")
	for _, fragment := range []string{
		"FROM business_documents bd",
		"bd.kind IN ('cash-return', 'credit-return'",
		"'open-cash-return', 'open-credit-return'",
		"business_document_lines bl",
		"FROM sync_events se",
		"se.aggregate IN ('sale_return', 'sale-return')",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"bd.id = se.aggregate_id",
		"bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("sale-return read model is missing %q", fragment)
		}
	}
}

func TestInvoiceSummaryReadModelsGroupRowsOncePerDocument(t *testing.T) {
	for name, query := range map[string]string{
		"sale":   salesReadModelQueryMode(reportSaleAggregate, "invoice-summary", "LIMIT $6 OFFSET $7"),
		"return": saleReturnReadModelQueryMode("invoice-summary", "LIMIT $6 OFFSET $7"),
	} {
		for _, fragment := range []string{
			"WITH invoice_rows AS (",
			"GROUP BY document",
			"SUM(CASE WHEN quantity ~",
			"MAX(CASE WHEN amount ~",
			"LIMIT $6 OFFSET $7",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s invoice summary is missing %q", name, fragment)
			}
		}
	}
	sale, saleOK := reportSpecForKey("sale-summary")
	if !saleOK || !sale.salesReadModel || sale.salesMode != "invoice-summary" {
		t.Fatalf("sale-summary spec = %+v (ok=%v), want invoice-summary read model", sale, saleOK)
	}
	saleReturn, saleReturnOK := reportSpecForKey("sales-return-summary")
	if !saleReturnOK || !saleReturn.salesReadModel || saleReturn.salesMode != "invoice-summary" {
		t.Fatalf("sales-return-summary spec = %+v (ok=%v), want invoice-summary read model", saleReturn, saleReturnOK)
	}
}

func TestSalesReadModelAlwaysCarriesTenantAndBranchPredicates(t *testing.T) {
	for _, condition := range []string{reportSaleAggregate, reportSaleOrReturn} {
		query := salesReadModelQuery(condition, "LIMIT $6")
		if strings.Count(query, "se.tenant_id = $1::uuid AND se.branch_id = $2::uuid") == 0 {
			t.Fatalf("compatibility event branch is not tenant/branch scoped for %q", condition)
		}
		if strings.Count(query, "sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid") == 0 {
			t.Fatalf("compatibility projection is not tenant/branch scoped for %q", condition)
		}
	}
}

func TestSalesReportDefinitionsDescribeCanonicalUnionAndReturnCompatibility(t *testing.T) {
	sale := reportDefinitionFor("sale-detail")
	if !strings.Contains(sale.ProjectionNote, "canonical cash/credit business_documents") {
		t.Fatalf("sale definition note does not describe canonical union: %q", sale.ProjectionNote)
	}
	saleReturn := reportDefinitionFor("sales-return-detail")
	if !strings.Contains(saleReturn.ProjectionNote, "business_documents") {
		t.Fatalf("sales-return definition does not describe canonical documents: %q", saleReturn.ProjectionNote)
	}
	if !strings.Contains(saleReturn.ProjectionNote, "compatibility sale_return events") {
		t.Fatalf("sales-return definition lost compatibility projection note: %q", saleReturn.ProjectionNote)
	}
}

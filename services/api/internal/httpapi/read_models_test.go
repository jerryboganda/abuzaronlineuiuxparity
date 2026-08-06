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
	if strings.Contains(saleReturn.ProjectionNote, "business_documents") {
		t.Fatalf("sales-return definition claimed canonical sale documents: %q", saleReturn.ProjectionNote)
	}
	if !strings.Contains(saleReturn.ProjectionNote, "immutable event-ledger") {
		t.Fatalf("sales-return definition lost compatibility projection note: %q", saleReturn.ProjectionNote)
	}
}

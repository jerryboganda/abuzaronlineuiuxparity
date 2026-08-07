package httpapi

import (
	"strings"
	"testing"
)

func TestCanonicalPurchaseHistoryKindsAreScopedDocumentFamilies(t *testing.T) {
	for _, kind := range []string{"pack-purchase", "loose-purchase", "opening-purchase", "purchase-return", "purchase-order"} {
		if !isCanonicalPurchaseHistoryKind(kind) {
			t.Fatalf("%s was not recognized as a canonical purchase history kind", kind)
		}
		if _, ok := historyAggregates[kind]; !ok {
			t.Fatalf("%s was not registered in historyAggregates", kind)
		}
	}
	for _, kind := range []string{"sale", "receiving", "unknown"} {
		if isCanonicalPurchaseHistoryKind(kind) {
			t.Fatalf("%s was incorrectly recognized as a canonical purchase history kind", kind)
		}
	}
}

func TestSalesHistoryQueriesExposeCanonicalDocumentIdentity(t *testing.T) {
	queries := map[string]string{
		"sales":     salesReadModelQuery(reportSaleAggregate, "LIMIT $6", true),
		"returns":   saleReturnReadModelQuery("LIMIT $6", true),
		"quotation": documentReadModelQuery("quotation", "quotation", "", "LIMIT $6", true),
		"refused":   documentReadModelQuery("refused-sale", "refused_sale", "", "LIMIT $6", true),
	}
	for name, query := range queries {
		if !strings.Contains(query, "bd.id::text AS document_id") {
			t.Errorf("%s history query does not retain canonical document identity", name)
		}
		if !strings.Contains(query, "SELECT document_id, document, occurred_at::text") {
			t.Errorf("%s history query does not project document identity", name)
		}
		if !strings.Contains(query, "NULL::text AS document_id") {
			t.Errorf("%s history query does not label compatibility rows without identity", name)
		}
	}
}

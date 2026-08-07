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

func TestCanonicalSalesHistoryQuerySupportsItemIdentityFiltering(t *testing.T) {
	query := salesReadModelQuery(reportSaleAggregate, "LIMIT $6", true)
	for _, fragment := range []string{
		"COALESCE(bl.item_legacy_id, '') AS item_legacy_id",
		"COALESCE(se.payload->>'itemLegacyId', se.payload->'rows'->0->>'itemLegacyId', '')",
		"OR item_legacy_id ILIKE '%' || $5 || '%'",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("canonical sales history query is missing %q", fragment)
		}
	}
}

func TestCanonicalPurchaseHistoryQuerySupportsItemIdentityFiltering(t *testing.T) {
	query := canonicalPurchaseHistoryQuery()
	for _, fragment := range []string{
		"SELECT item_legacy_id, item_name, quantity",
		"COALESCE(line.item_legacy_id, '') ILIKE '%' || $6 || '%'",
		"d.kind = $3",
		"d.deleted_at IS NULL",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("canonical purchase history query is missing %q", fragment)
		}
	}
}

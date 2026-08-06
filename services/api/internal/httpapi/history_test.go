package httpapi

import "testing"

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

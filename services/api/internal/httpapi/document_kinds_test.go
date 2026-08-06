package httpapi

import "testing"

func TestCanonicalNoStockSaleKindsAreRegisteredWithoutStockProjection(t *testing.T) {
	for _, kind := range []string{"quotation", "refused-sale"} {
		if _, ok := businessDocumentKinds[kind]; !ok {
			t.Fatalf("%s is not registered as a canonical business document kind", kind)
		}
		if isPurchaseDocumentKind(kind) {
			t.Fatalf("%s was incorrectly classified as a purchase", kind)
		}
		if isStockAndFinanceSaleKind(kind) {
			t.Fatalf("%s was incorrectly classified as stock/finance-bearing", kind)
		}
		if got := operatorDocumentKind(kind); got != kind {
			t.Fatalf("operatorDocumentKind(%q) = %q", kind, got)
		}
	}
}

func TestCanonicalStockAndFinanceSaleKindsRemainExplicit(t *testing.T) {
	for _, kind := range []string{"cash-sale", "credit-sale"} {
		if !isStockAndFinanceSaleKind(kind) {
			t.Fatalf("%s lost its stock/finance projection classification", kind)
		}
	}
}

func TestCanonicalSaleReturnKindsRemainStockAndFinanceBearing(t *testing.T) {
	for _, kind := range []string{"cash-return", "credit-return", "open-cash-return", "open-credit-return"} {
		if _, ok := businessDocumentKinds[kind]; !ok {
			t.Fatalf("%s is not registered as a canonical business document kind", kind)
		}
		if !isSaleReturnDocumentKind(kind) {
			t.Fatalf("%s lost its sale-return classification", kind)
		}
		if isStockAndFinanceSaleKind(kind) {
			t.Fatalf("%s was incorrectly classified as a source sale", kind)
		}
	}
	if isOpenSaleReturnDocumentKind("cash-return") || !isOpenSaleReturnDocumentKind("open-cash-return") {
		t.Fatal("source-bound and open sale returns were not kept distinct")
	}
}

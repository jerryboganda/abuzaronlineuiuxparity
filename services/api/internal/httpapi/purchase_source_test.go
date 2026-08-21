package httpapi

import "testing"

func TestPurchaseReceiptKindClassification(t *testing.T) {
	for _, kind := range []string{"pack-purchase", "loose-purchase", "opening-purchase"} {
		if !isPurchaseReceiptKind(kind) {
			t.Fatalf("%s should be a purchase receipt kind", kind)
		}
	}
	for _, kind := range []string{"purchase-order", "purchase-return", "cash-sale"} {
		if isPurchaseReceiptKind(kind) {
			t.Fatalf("%s should not be a purchase receipt kind", kind)
		}
	}
}

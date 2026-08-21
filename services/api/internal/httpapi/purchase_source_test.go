package httpapi

import "testing"

func TestRemainingQuantityNeverNegative(t *testing.T) {
	ordered, err := parseQuantity("5")
	if err != nil {
		t.Fatal(err)
	}
	already, err := parseQuantity("8")
	if err != nil {
		t.Fatal(err)
	}
	remaining := ordered - already
	if remaining < 0 {
		remaining = 0
	}
	if got := formatQuantity(remaining); got != "0" {
		t.Fatalf("remaining should clamp at 0, got %s", got)
	}
}

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

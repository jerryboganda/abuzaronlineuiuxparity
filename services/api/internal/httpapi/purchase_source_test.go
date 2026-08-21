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

func TestCopyRemarksIntoEmptyLineNotes(t *testing.T) {
	lines := []documentLineRequest{
		{Notes: ""},
		{Notes: "keep"},
		{Notes: "   "},
	}
	copyRemarksIntoEmptyLineNotes("  header remarks  ", lines)
	if lines[0].Notes != "header remarks" {
		t.Fatalf("empty notes = %q, want header remarks", lines[0].Notes)
	}
	if lines[1].Notes != "keep" {
		t.Fatalf("existing notes overwritten: %q", lines[1].Notes)
	}
	if lines[2].Notes != "header remarks" {
		t.Fatalf("whitespace notes = %q, want header remarks", lines[2].Notes)
	}
	copyRemarksIntoEmptyLineNotes("   ", lines)
	if lines[1].Notes != "keep" {
		t.Fatalf("blank remarks mutated existing notes: %q", lines[1].Notes)
	}
}

func TestRemarksCopyPreferenceKinds(t *testing.T) {
	if _, _, ok := remarksCopyPreference("cash-sale"); ok {
		t.Fatal("cash-sale should not copy remarks")
	}
	if category, caption, ok := remarksCopyPreference("purchase-order"); !ok || category != "Purchase Order" || caption != "Copy Remarks in Item Description" {
		t.Fatalf("purchase-order copy remarks pref = %s/%s ok=%v", category, caption, ok)
	}
	if category, caption, ok := remarksCopyPreference("purchase-return"); !ok || category != "Purchase Return" {
		t.Fatalf("purchase-return copy remarks pref = %s/%s ok=%v", category, caption, ok)
	}
}

func TestApplyPurchaseRegistryDefaultsFillsEmptyBatchAndExpiry(t *testing.T) {
	lines := []documentLineRequest{
		{BatchNumber: "", ExpiryDate: ""},
		{BatchNumber: "KEEP", ExpiryDate: "2027-01-01"},
	}
	applyPurchaseRegistryDefaults(lines)
	if lines[0].BatchNumber != "." {
		t.Fatalf("empty batch default = %q, want .", lines[0].BatchNumber)
	}
	if lines[0].ExpiryDate != "2030-12-12" {
		t.Fatalf("empty expiry default = %q, want 2030-12-12", lines[0].ExpiryDate)
	}
	if lines[1].BatchNumber != "KEEP" || lines[1].ExpiryDate != "2027-01-01" {
		t.Fatalf("explicit batch/expiry were overwritten: %+v", lines[1])
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

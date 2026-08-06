package hardware

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSaleSlipMatchesByteGolden(t *testing.T) {
	got, err := RenderSaleSlip(SaleSlip{
		Header:        "WASEELA ABUZAR",
		Store:         "Demo Pharmacy",
		InvoiceNumber: "CS-0001",
		Date:          "2026-08-06 02:00",
		Customer:      "CASH SALES CUSTOMER",
		Lines: []SaleSlipLine{
			{ItemName: "Paracetamol", Quantity: "2", Total: "200.00"},
			{ItemName: "Vitamin C", Quantity: "1", Total: "50.00"},
		},
		Subtotal: "250.00",
		Discount: "0.00",
		Tax:      "45.00",
		Total:    "295.00",
		Footer:   "Thank you",
	})
	if err != nil {
		t.Fatalf("render sale slip: %v", err)
	}
	assertByteGolden(t, "sale-slip.hex", got)
}

func TestRenderPurchaseLabelsMatchesByteGolden(t *testing.T) {
	got, err := RenderPurchaseLabels(PurchaseLabelBatch{
		Labels: []PurchaseLabel{{
			ItemName: "Paracetamol 500mg",
			Batch:    "B-01",
			Expiry:   "2027-06",
			MRP:      "120.00",
			Quantity: "10",
		}},
		CutAfter: true,
	})
	if err != nil {
		t.Fatalf("render purchase labels: %v", err)
	}
	assertByteGolden(t, "purchase-label.hex", got)
}

func TestRenderRejectsIncompleteJobs(t *testing.T) {
	if _, err := RenderSaleSlip(SaleSlip{InvoiceNumber: "1"}); err != ErrInvalidPrintJob && !strings.Contains(err.Error(), ErrInvalidPrintJob.Error()) {
		t.Fatalf("sale validation error = %v", err)
	}
	if _, err := RenderPurchaseLabels(PurchaseLabelBatch{}); err != ErrInvalidPrintJob && !strings.Contains(err.Error(), ErrInvalidPrintJob.Error()) {
		t.Fatalf("label validation error = %v", err)
	}
}

func assertByteGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	expectedText, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	expected, err := hex.DecodeString(strings.TrimSpace(string(expectedText)))
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}
	if string(got) != string(expected) {
		t.Fatalf("golden %s mismatch:\n got: %s\nwant: %s", name, hex.EncodeToString(got), hex.EncodeToString(expected))
	}
}

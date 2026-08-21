package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPurchaseCommandValidationRequiresReceiptMetadataButKeepsPONeutral(t *testing.T) {
	for _, kind := range []string{"pack-purchase", "loose-purchase", "opening-purchase"} {
		command := purchaseValidationCommand(kind, "post")
		if err := validateDocumentCommand(command, kind); err == nil {
			t.Fatalf("%s accepted without batch/cost metadata", kind)
		}
	}
	po := purchaseValidationCommand("purchase-order", "post")
	po.Document.ID = "00000000-0000-0000-0000-000000000043"
	if err := validateDocumentCommand(po, "purchase-order"); err != nil {
		t.Fatalf("purchase order incorrectly requires receipt metadata: %v", err)
	}
	missingSourceLine := purchaseValidationCommand("purchase-return", "post")
	missingSourceLine.Document.ID = "00000000-0000-0000-0000-000000000046"
	missingSourceLine.Document.Lines[0].Allocations = []documentAllocationRequest{{BatchNumber: "B-1", Quantity: "1"}}
	if err := validateDocumentCommand(missingSourceLine, "purchase-return"); err == nil || !strings.Contains(err.Error(), "sourceLineId") {
		t.Fatalf("purchase return without source line was accepted: %v", err)
	}
	ret := purchaseValidationCommand("purchase-return", "post")
	ret.Document.ID = "00000000-0000-0000-0000-000000000044"
	ret.Document.Lines[0].SourceLineID = "00000000-0000-0000-0000-000000000045"
	ret.Document.Lines[0].Allocations = []documentAllocationRequest{{BatchNumber: "B-1", Quantity: "1"}}
	if err := validateDocumentCommand(ret, "purchase-return"); err != nil {
		t.Fatalf("purchase return with explicit allocation rejected: %v", err)
	}
}

func TestPurchasePostFillsDefaultBatchFromRegistry(t *testing.T) {
	post := purchaseValidationCommand("pack-purchase", "post")
	post.Document.ID = "00000000-0000-0000-0000-000000000043"
	post.Document.Lines[0].UnitCost = "10.00"
	if err := validateDocumentCommand(post, "pack-purchase"); err != nil {
		t.Fatalf("post with empty batch should fill Default Batch: %v", err)
	}
	if post.Document.Lines[0].BatchNumber != "." {
		t.Fatalf("batch = %q, want .", post.Document.Lines[0].BatchNumber)
	}
	if post.Document.Lines[0].ExpiryDate != "2030-12-12" {
		t.Fatalf("expiry = %q, want 2030-12-12", post.Document.Lines[0].ExpiryDate)
	}
}

func TestPurchaseReceiptAllowsOptionalSourceLineID(t *testing.T) {
	for _, kind := range []string{"pack-purchase", "loose-purchase", "opening-purchase"} {
		save := purchaseValidationCommand(kind, "save")
		save.Document.Lines[0].SourceLineID = "00000000-0000-0000-0000-000000000099"
		if err := validateDocumentCommand(save, kind); err != nil {
			t.Fatalf("%s save with sourceLineId rejected: %v", kind, err)
		}
		post := purchaseValidationCommand(kind, "post")
		post.Document.ID = "00000000-0000-0000-0000-000000000043"
		post.Document.Lines[0].SourceLineID = "00000000-0000-0000-0000-000000000099"
		post.Document.Lines[0].BatchNumber = "B-1"
		post.Document.Lines[0].UnitCost = "10.00"
		if err := validateDocumentCommand(post, kind); err != nil {
			t.Fatalf("%s post with sourceLineId rejected: %v", kind, err)
		}
	}
	po := purchaseValidationCommand("purchase-order", "save")
	po.Document.Lines[0].SourceLineID = "00000000-0000-0000-0000-000000000099"
	if err := validateDocumentCommand(po, "purchase-order"); err == nil || !strings.Contains(err.Error(), "only valid") {
		t.Fatalf("purchase order with sourceLineId should be rejected: %v", err)
	}
}

func TestPurchaseMigrationExtendsKindsAndPreservesSaleCompatibility(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "014_purchase_documents.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read purchase migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"pack-purchase", "loose-purchase", "opening-purchase",
		"purchase-return", "purchase-order",
		"ADD COLUMN IF NOT EXISTS supplier_id",
		"ADD COLUMN IF NOT EXISTS source_document_id",
		"ADD COLUMN IF NOT EXISTS batch_number",
		"ADD COLUMN IF NOT EXISTS expiry_date",
		"accounts_payable", "input_tax",
		"counterparty_kind IN ('customer', 'supplier', 'cash')",
		"ALTER TABLE business_documents",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("purchase migration is missing contract %q", required)
		}
	}
	if strings.Contains(migration, "DROP TABLE") || strings.Contains(migration, "DELETE FROM business_documents") {
		t.Fatal("purchase migration deletes existing business document data")
	}
}

func purchaseValidationCommand(kind, action string) documentCommandRequest {
	return documentCommandRequest{
		CommandID: "00000000-0000-0000-0000-000000000041", Kind: kind,
		Action: action, IdempotencyKey: "purchase-validation-" + kind,
		OccurredAt: "2026-08-06T00:00:00Z",
		Document: &documentDraftRequest{
			Kind: kind, OccurredAt: "2026-08-06T00:00:00Z",
			Lines: []documentLineRequest{{
				LineNumber: 1, ItemID: "00000000-0000-0000-0000-000000000042",
				Quantity: "1", UnitPrice: "10.00",
			}},
		},
	}
}

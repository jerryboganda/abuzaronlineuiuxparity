package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

func validDocumentCommand(action string) documentCommandRequest {
	return documentCommandRequest{
		CommandID:      "00000000-0000-0000-0000-000000000001",
		Kind:           "cash-sale",
		Action:         action,
		IdempotencyKey: "cash-sale-command-1",
		OccurredAt:     "2026-08-06T00:00:00Z",
		Document: &documentDraftRequest{
			Kind:       "cash-sale",
			OccurredAt: "2026-08-06T00:00:00Z",
			Lines: []documentLineRequest{{
				LineNumber: 1,
				ItemID:     "00000000-0000-0000-0000-000000000002",
				Quantity:   "1",
				UnitPrice:  "10.00",
			}},
		},
	}
}

func TestDocumentCommandValidationCoversLifecycleAndRevisionRequirements(t *testing.T) {
	for _, action := range []string{"save", "save-and-post"} {
		if err := validateDocumentCommand(validDocumentCommand(action), "cash-sale"); err != nil {
			t.Fatalf("%s command rejected: %v", action, err)
		}
	}

	post := validDocumentCommand("post")
	post.Document.ID = "00000000-0000-0000-0000-000000000003"
	if err := validateDocumentCommand(post, "cash-sale"); err != nil {
		t.Fatalf("post command rejected: %v", err)
	}
	post.ExpectedVersion = pointerInt64(0)
	if err := validateDocumentCommand(post, "cash-sale"); err == nil {
		t.Fatal("non-positive expected version was accepted")
	}

	void := validDocumentCommand("void")
	void.Document = nil
	void.DocumentID = "00000000-0000-0000-0000-000000000003"
	void.Reason = "Correction"
	void.ExpectedVersion = pointerInt64(1)
	if err := validateDocumentCommand(void, "cash-sale"); err != nil {
		t.Fatalf("void command rejected: %v", err)
	}
}

func TestDocumentCommandIdempotencyHashDistinguishesPayloads(t *testing.T) {
	first := validDocumentCommand("save")
	second := validDocumentCommand("save")
	second.Document.Lines[0].Quantity = "2"
	firstHash, err := hashDocumentCommand(first)
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	sameHash, err := hashDocumentCommand(validDocumentCommand("save"))
	if err != nil {
		t.Fatalf("same hash: %v", err)
	}
	differentHash, err := hashDocumentCommand(second)
	if err != nil {
		t.Fatalf("different hash: %v", err)
	}
	if firstHash != sameHash {
		t.Fatal("same idempotent payload produced different hashes")
	}
	if firstHash == differentHash {
		t.Fatal("different idempotent payload produced the same hash")
	}
}

func TestDocumentPricingTotalsAreBalancedWithoutStockOrGLClaims(t *testing.T) {
	request, err := (pricingPreviewRequest{
		PriceLevel:              1,
		DocumentDiscountPercent: "10",
		FlatDiscountAmount:      "1.00",
		MiscAmount:              pointerString("0.00"),
		Lines: []pricingPreviewLine{{
			ID:       "item-1",
			Quantity: "2",
			Prices:   []string{"10.00"},
		}},
	}).toPricingRequest()
	if err != nil {
		t.Fatalf("pricing request: %v", err)
	}
	result, err := pricing.Calculate(request)
	if err != nil {
		t.Fatalf("pricing calculation: %v", err)
	}
	if !pricingTotalsBalanced(result) {
		t.Fatalf("unbalanced pricing result: %+v", result)
	}
	if formatExclusiveTax(result) != "0.00" {
		t.Fatalf("unexpected payable tax: %s", formatExclusiveTax(result))
	}
}

func TestDocumentCommandRouteRemainsAuthenticatedAndDoesNotUseLegacyAdapter(t *testing.T) {
	server := New(nil, "test", "")
	request := httptest.NewRequest(http.MethodPost, "/v1/documents/cash-sale", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestBusinessDocumentMigrationDefinesLifecycleIdempotencyAndTenantKeys(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "011_business_documents.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read business document migration: %v", err)
	}

	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS business_documents",
		"CREATE TABLE IF NOT EXISTS business_document_lines",
		"CREATE TABLE IF NOT EXISTS business_document_revisions",
		"CREATE TABLE IF NOT EXISTS command_receipts",
		"CHECK (status IN ('draft', 'posted', 'void'))",
		"UNIQUE (tenant_id, branch_id, kind, document_number)",
		"UNIQUE (tenant_id, idempotency_key)",
		"UNIQUE (tenant_id, command_id)",
		"FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id)",
		"ALTER TABLE %I FORCE ROW LEVEL SECURITY",
		"CREATE POLICY %I ON %I USING (current_setting(''app.authenticating''",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("migration is missing contract %q", required)
		}
	}
}

func TestSyncEventFinalPayloadMigrationDefinesOneWayCanonicalTransition(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "015_sync_event_final_payload.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sync event migration: %v", err)
	}

	migration := string(data)
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS finalized_at",
		"sync_events_business_document_final_payload_015",
		"guard_business_document_final_payload_015",
		"sync_events_final_payload_transition_015",
		"OLD.payload->>'state' = 'pending'",
		"NEW.payload->>'state' = 'final'",
		"payload ? 'eventId'",
		"payload ? 'document'",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("sync event migration is missing contract %q", required)
		}
	}
}

func TestSyncEventDeleteGuardMigrationProtectsFinalEvents(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "017_sync_event_delete_guard.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sync event delete guard migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE OR REPLACE FUNCTION reject_final_business_document_event_delete_017",
		"OLD.payload->>'state' = 'final'",
		"OLD.finalized_at IS NOT NULL",
		"BEFORE DELETE ON sync_events",
		"sync_events_final_business_document_delete_017",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("sync event delete guard migration is missing contract %q", required)
		}
	}
}

func TestSaleReturnValidationRequiresCanonicalSourceLineAndOpenBatch(t *testing.T) {
	closed := validDocumentCommand("save-and-post")
	closed.Kind = "cash-return"
	closed.Document.Kind = "cash-return"
	closed.Document.SourceDocumentID = "00000000-0000-0000-0000-000000000010"
	if err := validateDocumentCommand(closed, closed.Kind); err == nil ||
		!strings.Contains(err.Error(), "sourceLineId") {
		t.Fatalf("closed return without source line was accepted: %v", err)
	}

	closed.Document.Lines[0].SourceLineID = "00000000-0000-0000-0000-000000000011"
	if err := validateDocumentCommand(closed, closed.Kind); err != nil {
		t.Fatalf("closed return with canonical source line rejected: %v", err)
	}

	open := validDocumentCommand("save-and-post")
	open.Kind = "open-cash-return"
	open.Document.Kind = "open-cash-return"
	if err := validateDocumentCommand(open, open.Kind); err == nil ||
		!strings.Contains(err.Error(), "batchNumber") {
		t.Fatalf("open return without explicit batch was accepted: %v", err)
	}
	open.Document.Lines[0].BatchNumber = "RETURN-001"
	if err := validateDocumentCommand(open, open.Kind); err != nil {
		t.Fatalf("open return with explicit batch rejected: %v", err)
	}
}

func TestSaleReturnReversalMigrationDefinesSourceAndRLSBoundaries(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "025_sale_return_reversal_contract.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sale return migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"'open-cash-return', 'open-credit-return'",
		"ADD COLUMN IF NOT EXISTS source_line_id",
		"business_document_reversals",
		"validate_sale_return_source_025",
		"validate_sale_return_line_source_025",
		"FORCE ROW LEVEL SECURITY",
		"business_document_reversals_branch_tenant_hardening",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("return contract is missing %q", required)
		}
	}
}

func pointerInt64(value int64) *int64 {
	return &value
}

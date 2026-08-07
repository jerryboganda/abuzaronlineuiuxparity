package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPurchaseOrderLineContractIsCanonicalAndDependencyBound(t *testing.T) {
	implementation, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read implementation contract: %v", err)
	}
	contract := sourceQuery + "\n" + string(implementation)
	for _, want := range []string{
		"FROM dbo.PurOrderDetail",
		"PurOrderDetail:",
		"PurOrderHeader",
		"business_document_lines",
		"legacy_id_mappings",
		"migration_exceptions",
		"ON CONFLICT (tenant_id, branch_id, legacy_import_key)",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("purchase-order line contract does not contain %q", want)
		}
	}
}

func TestPurchaseOrderLineExceptionDetailsPreserveSourceFields(t *testing.T) {
	values := make([]any, 16)
	values[0] = int64(123)
	values[1] = int64(7)
	values[2] = int64(42)
	values[3] = "0"
	values[4] = "13.00"
	values[9] = "B-01"
	values[10] = "2027-01-01"
	values[11] = "2.00"
	values[12] = "1"
	values[13] = "0"
	values[14] = "0"
	values[15] = "awaiting receipt"

	encoded, err := orderLineExceptionDetails(values, "123:7", "non_positive_quantity")
	if err != nil {
		t.Fatalf("encode exception details: %v", err)
	}
	var details map[string]any
	if err := json.Unmarshal(encoded, &details); err != nil {
		t.Fatalf("decode exception details: %v", err)
	}
	for key, want := range map[string]string{
		"legacy_id": "123:7", "reason": "non_positive_quantity", "quantity": "0",
		"Batch": "B-01", "Rate": "13.00", "Remarks": "awaiting receipt",
	} {
		if got := details[key]; got != want {
			t.Fatalf("details[%q] = %v, want %q", key, got, want)
		}
	}
}

func TestPurchaseOrderLineExceptionDetailsRejectShortRows(t *testing.T) {
	if _, err := orderLineExceptionDetails(make([]any, 15), "1:1", "invalid"); err == nil {
		t.Fatal("short source row was accepted")
	}
}

func TestPurchaseOrderLineQuantityMustBePositive(t *testing.T) {
	for _, value := range []string{"0", "-1", "not-a-number"} {
		if positiveDecimal(value) {
			t.Fatalf("quantity %q was accepted", value)
		}
	}
	if !positiveDecimal("0.0001") {
		t.Fatal("positive fractional quantity was rejected")
	}
}

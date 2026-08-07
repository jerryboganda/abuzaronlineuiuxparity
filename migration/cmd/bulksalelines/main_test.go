package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSaleLineSourceContractIsCanonicalAndDependencyBound(t *testing.T) {
	implementation, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read implementation contract: %v", err)
	}
	contract := sourceQuery + "\n" + string(implementation)
	for _, want := range []string{
		"FROM dbo.Saledetail",
		"Saledetail:",
		"SaleLedger",
		"business_document_lines",
		"legacy_id_mappings",
		"migration_exceptions",
		"ON CONFLICT (tenant_id, branch_id, legacy_import_key)",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("sale-line contract does not contain %q", want)
		}
	}
}

func TestSaleLineExceptionDetailsPreserveSourceQuantityInputs(t *testing.T) {
	values := make([]any, 24)
	values[0] = int64(123)
	values[1] = int64(7)
	values[2] = int64(42)
	values[3] = "0"
	values[14] = "0"
	values[15] = "0"
	values[16] = "10"
	values[17] = "15.00"
	values[18] = "13.00"
	values[19] = "2.00"
	values[20] = "1.25"
	values[21] = "0.50"
	values[22] = "HS-1"
	values[23] = "PACK"

	encoded, err := saleLineExceptionDetails(values, "123:7", "non_positive_quantity")
	if err != nil {
		t.Fatalf("encode exception details: %v", err)
	}
	var details map[string]any
	if err := json.Unmarshal(encoded, &details); err != nil {
		t.Fatalf("decode exception details: %v", err)
	}
	for key, want := range map[string]string{
		"legacy_id": "123:7", "reason": "non_positive_quantity", "quantity": "0",
		"PackQty": "0", "LooseQty": "0", "PackUnits": "10", "SalePrice": "15.00",
	} {
		if got := details[key]; got != want {
			t.Fatalf("details[%q] = %v, want %q", key, got, want)
		}
	}
}

func TestSaleLineExceptionDetailsRejectShortRows(t *testing.T) {
	if _, err := saleLineExceptionDetails(make([]any, 23), "1:1", "invalid"); err == nil {
		t.Fatal("short source row was accepted")
	}
}

func TestSaleLineQuantityMustBePositive(t *testing.T) {
	for _, value := range []string{"0", "-1", "not-a-number"} {
		if positiveDecimal(value) {
			t.Fatalf("quantity %q was accepted", value)
		}
	}
	if !positiveDecimal("0.0001") {
		t.Fatal("positive fractional quantity was rejected")
	}
}

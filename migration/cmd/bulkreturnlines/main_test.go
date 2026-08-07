package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestReturnLineModesAreFixedAndDependencyBound(t *testing.T) {
	implementation, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read implementation contract: %v", err)
	}
	contract := string(implementation)
	for _, want := range []string{
		"FROM dbo.SRdetail",
		"FROM dbo.PRdetail",
		"SRLedger",
		"PRLedger",
		"cash-sale-return",
		"purchase-return",
		"business_document_lines",
		"legacy_id_mappings",
		"migration_exceptions",
		"ON CONFLICT (tenant_id, branch_id, legacy_import_key)",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("return-line contract does not contain %q", want)
		}
	}
	if returnModes["sale"].sourceQuery != saleReturnSourceQuery || returnModes["purchase"].sourceQuery != purchaseReturnSourceQuery {
		t.Fatal("return modes are not bound to fixed reviewed source queries")
	}
}

func TestReturnLineExceptionDetailsPreserveModeSpecificSourceFields(t *testing.T) {
	values := make([]any, 22)
	values[0] = "123"
	values[1] = "7"
	values[2] = "42"
	values[3] = "0"
	values[11] = "B-01"
	values[12] = "2027-01-01"
	values[14] = "0"
	values[15] = "0"
	values[16] = "10"
	values[17] = "15.00"
	values[18] = "2.00"
	values[21] = "sale-line-9"

	encoded, err := returnLineExceptionDetails(values, returnModes["sale"], "123:7", "non_positive_quantity")
	if err != nil {
		t.Fatalf("encode exception details: %v", err)
	}
	var details map[string]any
	if err := json.Unmarshal(encoded, &details); err != nil {
		t.Fatalf("decode exception details: %v", err)
	}
	for key, want := range map[string]string{
		"legacy_id": "123:7", "reason": "non_positive_quantity", "SRInvcode": "123",
		"Batch": "B-01", "SRPrice": "15.00", "SaleRowId": "sale-line-9",
	} {
		if got := details[key]; got != want {
			t.Fatalf("details[%q] = %v, want %q", key, got, want)
		}
	}
}

func TestReturnLineExceptionDetailsRejectShortRows(t *testing.T) {
	if _, err := returnLineExceptionDetails(make([]any, 21), returnModes["purchase"], "1:1", "invalid"); err == nil {
		t.Fatal("short source row was accepted")
	}
}

func TestReturnLinePayloadUsesReviewedModeKeys(t *testing.T) {
	values := make([]any, 22)
	values[0] = "PR-10"
	values[2] = "ITEM-7"
	values[11] = "B-02"
	values[12] = "2027-02-01"
	values[14] = "1"
	values[15] = "2"
	values[16] = "10"
	values[17] = "20.00"
	values[18] = "1.00"
	values[19] = "17.00"
	values[20] = "9"
	values[21] = "HB-9"

	encoded, err := returnPayload(values, returnModes["purchase"])
	if err != nil {
		t.Fatalf("encode purchase return payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode purchase return payload: %v", err)
	}
	for key, want := range map[string]string{
		"PRInvCode": "PR-10", "ICode": "ITEM-7", "PRPrice": "20.00",
		"PrRowId": "9", "UnitSalesTax": "17.00", "HistoricalBatch": "HB-9",
	} {
		if got := payload[key]; got != want {
			t.Fatalf("payload[%q] = %v, want %q", key, got, want)
		}
	}
	if _, exists := payload["RowId"]; exists {
		t.Fatal("purchase return payload used the sale-only RowId key")
	}
}

func TestReturnLineQuantityMustBePositive(t *testing.T) {
	for _, value := range []string{"0", "-1", "not-a-number"} {
		if positiveDecimal(value) {
			t.Fatalf("quantity %q was accepted", value)
		}
	}
	if !positiveDecimal("0.0001") {
		t.Fatal("positive fractional quantity was rejected")
	}
}

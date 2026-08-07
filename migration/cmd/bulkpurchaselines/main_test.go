package main

import (
	"encoding/json"
	"testing"
)

func TestPurchaseLineExceptionDetailsPreserveSourceQuantityInputs(t *testing.T) {
	values := make([]any, 27)
	values[0] = int64(123)
	values[1] = int64(7)
	values[2] = int64(42)
	values[3] = "0"
	values[14] = "0"
	values[15] = "0"
	values[16] = "10"
	values[17] = "12.50"
	values[18] = "13.00"
	values[19] = "2.00"
	values[20] = "18.00"
	values[21] = "1.25"
	values[22] = "PCT-1"
	values[23] = "GST-1"
	values[24] = "11.00"
	values[25] = "1"
	values[26] = "0.50"

	encoded, err := purchaseLineExceptionDetails(values, "123:7", "non_positive_quantity")
	if err != nil {
		t.Fatalf("encode exception details: %v", err)
	}
	var details map[string]any
	if err := json.Unmarshal(encoded, &details); err != nil {
		t.Fatalf("decode exception details: %v", err)
	}
	for key, want := range map[string]string{
		"legacy_id": "123:7", "reason": "non_positive_quantity", "quantity": "0",
		"PackQty": "0", "LooseQty": "0", "PackUnits": "10", "GSTPerc": "18.00",
	} {
		if got := details[key]; got != want {
			t.Fatalf("details[%q] = %v, want %q", key, got, want)
		}
	}
}

func TestPurchaseLineExceptionDetailsRejectShortRows(t *testing.T) {
	if _, err := purchaseLineExceptionDetails(make([]any, 26), "1:1", "invalid"); err == nil {
		t.Fatal("short source row was accepted")
	}
}

package main

import (
	"encoding/json"
	"strings"
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

func TestPurchaseLineSourceWindowUsesStableOrderingAndExclusiveEnd(t *testing.T) {
	query, args, err := sourceRowsQuery(2000, 3500)
	if err != nil {
		t.Fatalf("source window rejected: %v", err)
	}
	for _, want := range []string{"ORDER BY pur_inv_code", "TRY_CONVERT(bigint, pur_row_id)", "OFFSET ? ROWS", "FETCH NEXT ? ROWS ONLY"} {
		if !strings.Contains(query, want) {
			t.Fatalf("source window query does not contain %q", want)
		}
	}
	if len(args) != 2 || args[0] != 2000 || args[1] != 1500 {
		t.Fatalf("source window args = %#v, want [2000 1500]", args)
	}
}

func TestPurchaseLineSourceWindowPreservesFullRunAndRejectsInvalidBounds(t *testing.T) {
	query, args, err := sourceRowsQuery(0, -1)
	if err != nil {
		t.Fatalf("full source window rejected: %v", err)
	}
	if query != sourceQuery || len(args) != 0 {
		t.Fatalf("full source window changed the unbounded query or args")
	}
	for _, bounds := range [][2]int{{-1, 10}, {10, 10}, {10, 5}} {
		if _, _, err := sourceRowsQuery(bounds[0], bounds[1]); err == nil {
			t.Fatalf("invalid source window %#v was accepted", bounds)
		}
	}
	if rowWindowEnd(-1) != "end" || rowWindowEnd(3500) != "3500" {
		t.Fatal("source row window display is not deterministic")
	}
}

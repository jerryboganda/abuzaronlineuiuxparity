package httpapi

import (
	"testing"
	"time"
)

func TestPriceLevelAllowedHonorsGroupAllowedPrice(t *testing.T) {
	admin := &sessionContext{Roles: []string{"tenant_admin"}}
	if !priceLevelAllowed(admin, 9) {
		t.Fatal("tenant admin must be allowed every price level")
	}
	unscoped := &sessionContext{Roles: []string{"cashier"}, Scopes: map[string]map[string]bool{}}
	if !priceLevelAllowed(unscoped, 3) {
		t.Fatal("missing price allow-list must preserve current permission behavior")
	}
	restricted := &sessionContext{
		Roles:  []string{"cashier"},
		Scopes: map[string]map[string]bool{"price": {"1": true, "SalePrice#2": true}},
	}
	if !priceLevelAllowed(restricted, 1) || !priceLevelAllowed(restricted, 2) {
		t.Fatal("allowed price keys were rejected")
	}
	if priceLevelAllowed(restricted, 3) {
		t.Fatal("price level 3 must be rejected when GroupAllowedPrice omits it")
	}
}

func TestSelectPricePolicyTierPicksHighestQualifyingUnexpiredRow(t *testing.T) {
	asOf := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	tiers := []itemPricePolicyTierResponse{
		{QuantityLimit: 1, Price: "10.00", ExpiryDate: "2020-01-01"},
		{QuantityLimit: 5, Price: "9.00", ExpiryDate: "2030-01-01"},
		{QuantityLimit: 12, Price: "8.00", ExpiryDate: "2030-01-01"},
		{QuantityLimit: 20, Price: "7.00"},
	}
	matched := selectPricePolicyTier(tiers, 12, asOf)
	if matched == nil || matched.Price != "8.00" {
		t.Fatalf("matched = %+v, want qty 12 price 8.00", matched)
	}
	if overlay := overlayPricePolicyOnTiers([]string{"10", "11"}, 1, "8.00"); overlay[0] != "8.00" {
		t.Fatalf("overlay = %v", overlay)
	}
	if overlay := overlayPricePolicyOnTiers([]string{"10"}, 1, "0"); overlay[0] != "10" {
		t.Fatalf("zero policy price must not replace the catalog tier: %v", overlay)
	}
}

func TestParseGodownTransferRequestRejectsSameGodown(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	item := "22222222-2222-2222-2222-222222222222"
	_, err := parseGodownTransferRequest(map[string]any{
		"sourceGodownId":      id,
		"destinationGodownId": id,
		"lines":               []any{map[string]any{"itemId": item, "quantity": "1"}},
	})
	if err == nil {
		t.Fatal("same source and destination must be rejected")
	}
}

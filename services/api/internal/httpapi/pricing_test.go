package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPricingRequestParsersUseExactDecimalBoundaries(t *testing.T) {
	moneyValue, err := parseMoney("1,000.00")
	if err == nil || moneyValue != 0 {
		t.Fatalf("comma-formatted amount was accepted: value=%d err=%v", moneyValue, err)
	}
	moneyValue, err = parseMoney("1000.25")
	if err != nil || moneyValue != 100025 {
		t.Fatalf("parseMoney(1000.25) = %d, %v; want 100025 minor units", moneyValue, err)
	}
	percentValue, err := parsePercent("7.50")
	if err != nil || percentValue != 750 {
		t.Fatalf("parsePercent(7.50) = %d, %v; want 750 basis points", percentValue, err)
	}
	if _, err := parseMoney("1.001"); err == nil {
		t.Fatal("amount with more than two decimal places was accepted")
	}
	if _, err := parseQuantity("1.5"); err == nil {
		t.Fatal("fractional quantity was accepted")
	}
}

func TestPricingPreviewRequestMapsTiersDiscountsAndTaxes(t *testing.T) {
	request, err := (pricingPreviewRequest{
		PriceLevel:              2,
		GroupDiscountPercent:    "5",
		CustomerDiscountPercent: pointerString("7.5"),
		DocumentDiscountPercent: "1.25",
		FlatDiscountAmount:      "2.00",
		MiscAmount:              pointerString("0.50"),
		Lines: []pricingPreviewLine{{
			ID:                  "item-1",
			Quantity:            "2",
			Prices:              []string{"10.00", "12.00"},
			ItemDiscountPercent: "1",
			SupplierScheme: &pricingPreviewSupplierScheme{
				DiscountPercent:    "2",
				QualifyingQuantity: "2",
				BonusQuantity:      "1",
			},
		}},
		Taxes: pricingPreviewTaxes{GST: &pricingPreviewTax{Rate: "18", Inclusive: false}},
	}).toPricingRequest()
	if err != nil {
		t.Fatalf("toPricingRequest returned error: %v", err)
	}
	if request.PriceLevel != 2 || len(request.Lines) != 1 || request.Lines[0].Prices[1] != 1200 {
		t.Fatalf("tier mapping was not preserved: %+v", request)
	}
	if request.Customer.GroupPercent != 500 || request.Customer.CustomerPercent == nil || *request.Customer.CustomerPercent != 750 {
		t.Fatalf("customer discounts were not mapped: %+v", request.Customer)
	}
	if request.Taxes.GST == nil || request.Taxes.GST.Rate != 1800 || !strings.EqualFold("gst", taxKindName(request.Taxes.GST.Kind)) {
		t.Fatalf("GST policy was not mapped: %+v", request.Taxes.GST)
	}
}

func TestSalePricingValidationRejectsForgedTotal(t *testing.T) {
	payload := salePayload{
		TotalAmount: "100.00",
		PricingRequest: &pricingPreviewRequest{
			Lines:      []pricingPreviewLine{{ID: "item-1", Quantity: "1", Prices: []string{"100.00"}}},
			MiscAmount: pointerString("0.00"),
		},
	}
	if err := validateSalePricing(payload); err != nil {
		t.Fatalf("matching sale pricing was rejected: %v", err)
	}
	payload.TotalAmount = "99.99"
	if err := validateSalePricing(payload); err == nil {
		t.Fatal("forged sale total was accepted")
	}
}

func TestInventoryQuantityValidationRejectsEmptyAndNonPositiveValues(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "not-a-number"} {
		if err := validateInventoryQuantity(value); err == nil {
			t.Errorf("quantity %q was accepted", value)
		}
	}
	if err := validateInventoryQuantity("1.25"); err != nil {
		t.Fatalf("positive decimal quantity was rejected: %v", err)
	}
}

func TestPricingPreviewRouteRemainsAuthenticated(t *testing.T) {
	server := New(nil, "test", "")
	request := httptest.NewRequest(http.MethodPost, "/v1/transactions/preview", strings.NewReader(`{"lines":[]}`))
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func pointerString(value string) *string { return &value }

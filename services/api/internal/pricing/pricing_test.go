package pricing

import (
	"errors"
	"reflect"
	"testing"
)

func money(value int64) Money {
	return Money(value)
}

func percent(value int64) BasisPoints {
	return BasisPoints(value)
}

func tiers(values ...int64) PriceTiers {
	var result PriceTiers
	for index, value := range values {
		result[index] = money(value)
	}
	return result
}

func pointer[T any](value T) *T {
	return &value
}

func baseRequest(line LineInput) Request {
	return Request{
		PriceLevel: 1,
		Lines:      []LineInput{line},
		Misc:       pointer(money(0)),
		Rounding:   DefaultRoundingPolicy(),
	}
}

func TestCalculateGoldenCases(t *testing.T) {
	tests := []struct {
		name                 string
		request              Request
		wantSubtotal         Money
		wantLineDiscount     Money
		wantLineGross        Money
		wantItemDiscount     Money
		wantCustomerDiscount Money
		wantCustomerRate     BasisPoints
		wantCustomerSource   CustomerDiscountSource
		wantBonus            Quantity
		wantDocumentDiscount Money
		wantFlatDiscount     Money
		wantDocumentTotal    Money
		wantMisc             Money
		wantTaxableBase      Money
		wantTaxKinds         []TaxKind
		wantTaxAmounts       []Money
		wantTotalDiscount    Money
		wantTotal            Money
	}{
		{
			name: "selected price tier",
			request: func() Request {
				request := baseRequest(LineInput{
					ID:       "tiered",
					Quantity: 2,
					Prices:   tiers(100, 110, 125, 140, 155, 170, 185, 200, 215, 230),
				})
				request.PriceLevel = 3
				return request
			}(),
			wantSubtotal:       money(250),
			wantLineDiscount:   money(0),
			wantLineGross:      money(250),
			wantCustomerSource: CustomerDiscountFromGroup,
			wantMisc:           money(0),
			wantTaxableBase:    money(250),
			wantTotal:          money(250),
		},
		{
			name: "item and customer override precedence",
			request: func() Request {
				request := baseRequest(LineInput{
					ID:           "override",
					Quantity:     1,
					Prices:       tiers(1000),
					ItemDiscount: percent(1000),
				})
				request.Customer = CustomerDiscounts{
					GroupPercent:    percent(500),
					CustomerPercent: pointer(percent(700)),
					OverridePercent: pointer(percent(900)),
				}
				return request
			}(),
			wantSubtotal:         money(819),
			wantLineDiscount:     money(181),
			wantLineGross:        money(1000),
			wantItemDiscount:     money(100),
			wantCustomerDiscount: money(81),
			wantCustomerRate:     percent(900),
			wantCustomerSource:   CustomerDiscountFromOverride,
			wantMisc:             money(0),
			wantTaxableBase:      money(819),
			wantTotalDiscount:    money(181),
			wantTotal:            money(819),
		},
		{
			name: "group discount when no customer override exists",
			request: func() Request {
				request := baseRequest(LineInput{
					ID:       "group",
					Quantity: 1,
					Prices:   tiers(1000),
				})
				request.Customer.GroupPercent = percent(500)
				return request
			}(),
			wantSubtotal:         money(950),
			wantLineDiscount:     money(50),
			wantLineGross:        money(1000),
			wantCustomerDiscount: money(50),
			wantCustomerRate:     percent(500),
			wantCustomerSource:   CustomerDiscountFromGroup,
			wantMisc:             money(0),
			wantTaxableBase:      money(950),
			wantTotalDiscount:    money(50),
			wantTotal:            money(950),
		},
		{
			name: "document percent flat and default misc",
			request: func() Request {
				request := baseRequest(LineInput{
					ID:       "document",
					Quantity: 1,
					Prices:   tiers(1000),
				})
				request.Misc = nil
				request.DocumentPercent = percent(1000)
				request.FlatDiscount = money(50)
				return request
			}(),
			wantSubtotal:         money(1000),
			wantLineDiscount:     money(0),
			wantLineGross:        money(1000),
			wantDocumentDiscount: money(100),
			wantFlatDiscount:     money(50),
			wantDocumentTotal:    money(150),
			wantMisc:             money(100),
			wantTaxableBase:      money(950),
			wantTotalDiscount:    money(150),
			wantTotal:            money(950),
		},
		{
			name: "inclusive GST",
			request: func() Request {
				request := baseRequest(LineInput{ID: "gst-in", Quantity: 1, Prices: tiers(1100)})
				request.Taxes.GST = &TaxRule{Kind: TaxGST, Rate: percent(1000), Inclusive: true}
				return request
			}(),
			wantSubtotal:     money(1100),
			wantLineDiscount: money(0),
			wantLineGross:    money(1100),
			wantMisc:         money(0),
			wantTaxableBase:  money(1100),
			wantTaxKinds:     []TaxKind{TaxGST},
			wantTaxAmounts:   []Money{money(100)},
			wantTotal:        money(1100),
		},
		{
			name: "exclusive GST",
			request: func() Request {
				request := baseRequest(LineInput{ID: "gst-out", Quantity: 1, Prices: tiers(1000)})
				request.Taxes.GST = &TaxRule{Kind: TaxGST, Rate: percent(1000)}
				return request
			}(),
			wantSubtotal:     money(1000),
			wantLineDiscount: money(0),
			wantLineGross:    money(1000),
			wantMisc:         money(0),
			wantTaxableBase:  money(1000),
			wantTaxKinds:     []TaxKind{TaxGST},
			wantTaxAmounts:   []Money{money(100)},
			wantTotal:        money(1100),
		},
		{
			name: "exclusive PCT then advance tax",
			request: func() Request {
				request := baseRequest(LineInput{ID: "taxes", Quantity: 1, Prices: tiers(1000)})
				request.Taxes.PCT = &TaxRule{Kind: TaxPCT, Rate: percent(500)}
				request.Taxes.AdvanceTax = &TaxRule{Kind: TaxAdvance, Rate: percent(1000)}
				return request
			}(),
			wantSubtotal:     money(1000),
			wantLineDiscount: money(0),
			wantLineGross:    money(1000),
			wantMisc:         money(0),
			wantTaxableBase:  money(1000),
			wantTaxKinds:     []TaxKind{TaxPCT, TaxAdvance},
			wantTaxAmounts:   []Money{money(50), money(105)},
			wantTotal:        money(1155),
		},
		{
			name: "supplier scheme discount and bonus",
			request: func() Request {
				return baseRequest(LineInput{
					ID:       "scheme",
					Quantity: 10,
					Prices:   tiers(100),
					SupplierScheme: &SupplierScheme{
						DiscountPercent:    percent(1000),
						QualifyingQuantity: 5,
						BonusQuantity:      1,
					},
				})
			}(),
			wantSubtotal:      money(900),
			wantLineDiscount:  money(100),
			wantLineGross:     money(900),
			wantBonus:         Quantity(2),
			wantMisc:          money(0),
			wantTaxableBase:   money(900),
			wantTotalDiscount: money(100),
			wantTotal:         money(900),
		},
		{
			name: "explicit rounding boundaries",
			request: func() Request {
				request := baseRequest(LineInput{
					ID:           "rounding",
					Quantity:     1,
					Prices:       tiers(101),
					ItemDiscount: percent(3333),
				})
				request.DocumentPercent = percent(3333)
				request.Rounding = RoundingPolicy{
					Line:     RoundDown,
					Tax:      RoundHalfUp,
					Document: RoundHalfEven,
				}
				return request
			}(),
			wantSubtotal:         money(68),
			wantLineDiscount:     money(33),
			wantLineGross:        money(101),
			wantItemDiscount:     money(33),
			wantDocumentDiscount: money(23),
			wantDocumentTotal:    money(23),
			wantMisc:             money(0),
			wantTaxableBase:      money(45),
			wantTotalDiscount:    money(56),
			wantTotal:            money(45),
		},
		{
			name: "half up rounds a half minor unit upward",
			request: baseRequest(LineInput{
				ID:           "half-up",
				Quantity:     1,
				Prices:       tiers(1),
				ItemDiscount: percent(5000),
			}),
			wantSubtotal:       money(0),
			wantLineDiscount:   money(1),
			wantLineGross:      money(1),
			wantItemDiscount:   money(1),
			wantMisc:           money(0),
			wantTaxableBase:    money(0),
			wantTotalDiscount:  money(1),
			wantTotal:          money(0),
			wantCustomerSource: CustomerDiscountFromGroup,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Calculate(test.request)
			if err != nil {
				t.Fatalf("Calculate returned error: %v", err)
			}
			if got.Subtotal != test.wantSubtotal ||
				got.LineDiscountTotal != test.wantLineDiscount ||
				got.DocumentPercentDiscount != test.wantDocumentDiscount ||
				got.FlatDiscount != test.wantFlatDiscount ||
				got.DocumentDiscountTotal != test.wantDocumentTotal ||
				got.Misc != test.wantMisc ||
				got.TaxableBase != test.wantTaxableBase ||
				got.TotalDiscount != test.wantTotalDiscount ||
				got.Total != test.wantTotal {
				t.Fatalf("summary = %+v", got)
			}
			line := got.Lines[0]
			if line.LineGross != test.wantLineGross ||
				line.ItemDiscount != test.wantItemDiscount ||
				line.CustomerDiscount != test.wantCustomerDiscount ||
				line.CustomerDiscountRate != test.wantCustomerRate ||
				line.CustomerDiscountSource != test.wantCustomerSource ||
				line.SupplierBonusQuantity != test.wantBonus {
				t.Fatalf("line = %+v", line)
			}
			if len(got.Taxes) != len(test.wantTaxKinds) {
				t.Fatalf("tax count = %d, want %d", len(got.Taxes), len(test.wantTaxKinds))
			}
			for index, tax := range got.Taxes {
				if tax.Kind != test.wantTaxKinds[index] || tax.Amount != test.wantTaxAmounts[index] {
					t.Fatalf("tax[%d] = %+v", index, tax)
				}
			}
		})
	}
}

func TestCalculateRejectsInvalidAndOverflowInputs(t *testing.T) {
	validLine := LineInput{ID: "line", Quantity: 1, Prices: tiers(100)}
	tests := []struct {
		name    string
		request Request
		wantErr error
	}{
		{
			name:    "invalid price level",
			request: Request{PriceLevel: 11, Lines: []LineInput{validLine}},
			wantErr: ErrInvalidInput,
		},
		{
			name: "negative price",
			request: Request{
				PriceLevel: 1,
				Lines:      []LineInput{{ID: "negative", Quantity: 1, Prices: tiers(-1)}},
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "negative item discount",
			request: Request{
				PriceLevel: 1,
				Lines:      []LineInput{{ID: "negative", Quantity: 1, Prices: tiers(100), ItemDiscount: -1}},
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "flat discount exceeds subtotal",
			request: func() Request {
				request := baseRequest(validLine)
				request.FlatDiscount = money(101)
				return request
			}(),
			wantErr: ErrInvalidInput,
		},
		{
			name: "invalid tax rate",
			request: func() Request {
				request := baseRequest(validLine)
				request.Taxes.GST = &TaxRule{Kind: TaxGST, Rate: percent(10001)}
				return request
			}(),
			wantErr: ErrInvalidInput,
		},
		{
			name: "line multiplication overflow",
			request: Request{
				PriceLevel: 1,
				Lines: []LineInput{{
					ID:       "overflow",
					Quantity: Quantity(maxInt64Value),
					Prices:   tiers(maxInt64Value),
				}},
				Misc: pointer(money(0)),
			},
			wantErr: ErrOverflow,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Calculate(test.request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, test.wantErr)
			}
		})
	}
}

func TestCalculateIsDeterministic(t *testing.T) {
	request := baseRequest(LineInput{
		ID:           "stable",
		Quantity:     3,
		Prices:       tiers(123),
		ItemDiscount: percent(1250),
	})
	request.Taxes.GST = &TaxRule{Kind: TaxGST, Rate: percent(1750)}
	first, err := Calculate(request)
	if err != nil {
		t.Fatalf("first Calculate returned error: %v", err)
	}
	second, err := Calculate(request)
	if err != nil {
		t.Fatalf("second Calculate returned error: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("results differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestMoneyString(t *testing.T) {
	if got := Money(12345).String(); got != "123.45" {
		t.Fatalf("Money.String() = %q, want 123.45", got)
	}
}

// TestCalculateReproducesRealLegacySaleInvoiceLines is a golden-replay
// regression test built from real historical sale lines in the
// legacy-reference-sandbox tenant (eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee),
// cross-checked directly against dbo.Saledetail on the read-only SQL Server
// canonical database (FazalDinPP19DataBaseV2) on 2026-08-08.
//
// Each case's per-unit Prices[0] is dbo.Saledetail.Rate (the tax-inclusive
// price actually charged for that line), expressed in minor units. Applying
// an 18% inclusive GST rule reproduces, to the paisa, both the legacy
// dbo.Saledetail.SalePrice (net/taxable base, i.e. Total here since inclusive
// tax does not change Total) and dbo.Saledetail.UnitSalesTax*Quantity (the
// tax component) for every one of these lines. This does not mean historical
// invoices were produced by this engine -- migrated business_document_lines
// rows are a direct copy of Saledetail.Rate (see
// migration/maps/phase-e-historical-documents.json) -- it demonstrates that
// Calculate's inclusive-tax arithmetic is legacy-faithful when exercised
// against real invoice numbers. See docs/PHASE_G_PRICING_GOLDEN_REPLAY_2026-08-08.md.
func TestCalculateReproducesRealLegacySaleInvoiceLines(t *testing.T) {
	gst := func() *TaxRule { return &TaxRule{Kind: TaxGST, Rate: percent(1800), Inclusive: true} }

	tests := []struct {
		name         string // SaleInvcode/RowID/ICode for traceability back to dbo.Saledetail
		unitRate     int64  // dbo.Saledetail.Rate for this line, in minor units (paisa)
		quantity     int64
		wantGross    Money // dbo.Saledetail.Rate * Quantity (also Subtotal/TaxableBase/Total)
		wantTaxBase  Money
		wantTaxAmt   Money // extracted GST; dbo.Saledetail.UnitSalesTax * Quantity
		wantNetTotal Money // TaxableBase - wantTaxAmt; dbo.Saledetail.SalePrice * Quantity
	}{
		{name: "SaleInvcode 780563 RowID 411860 ICode 23468", unitRate: 205000, quantity: 1, wantGross: 205000, wantTaxBase: 205000, wantTaxAmt: 31271, wantNetTotal: 173729},
		{name: "SaleInvcode 788230 RowID 428238 ICode 151", unitRate: 463500, quantity: 1, wantGross: 463500, wantTaxBase: 463500, wantTaxAmt: 70703, wantNetTotal: 392797},
		{name: "SaleInvcode 834232 RowID 525751 ICode 10941", unitRate: 25000, quantity: 1, wantGross: 25000, wantTaxBase: 25000, wantTaxAmt: 3814, wantNetTotal: 21186},
		{name: "SaleInvcode 596551 RowID 17189 ICode 29032", unitRate: 9000, quantity: 3, wantGross: 27000, wantTaxBase: 27000, wantTaxAmt: 4119, wantNetTotal: 22881},
		{name: "SaleInvcode 611887 RowID 51120 ICode 10625", unitRate: 5200, quantity: 2, wantGross: 10400, wantTaxBase: 10400, wantTaxAmt: 1586, wantNetTotal: 8814},
		{name: "SaleInvcode 849566 RowID 557924 ICode 12801", unitRate: 8246, quantity: 2, wantGross: 16492, wantTaxBase: 16492, wantTaxAmt: 2516, wantNetTotal: 13976},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := baseRequest(LineInput{
				ID:       "line",
				Quantity: Quantity(test.quantity),
				Prices:   tiers(test.unitRate),
			})
			request.Taxes.GST = gst()
			got, err := Calculate(request)
			if err != nil {
				t.Fatalf("Calculate returned error: %v", err)
			}
			if got.Subtotal != test.wantGross {
				t.Fatalf("Subtotal = %v, want %v", got.Subtotal, test.wantGross)
			}
			if got.TaxableBase != test.wantTaxBase {
				t.Fatalf("TaxableBase = %v, want %v", got.TaxableBase, test.wantTaxBase)
			}
			if len(got.Taxes) != 1 || got.Taxes[0].Amount != test.wantTaxAmt {
				t.Fatalf("GST amount = %+v, want %v", got.Taxes, test.wantTaxAmt)
			}
			if wantNet := test.wantTaxBase - test.wantTaxAmt; wantNet != test.wantNetTotal {
				t.Fatalf("test fixture inconsistent: TaxableBase-TaxAmt = %v, want wantNetTotal %v", wantNet, test.wantNetTotal)
			}
			if got.Total != test.wantGross {
				t.Fatalf("Total = %v, want %v (inclusive GST must not change the charged total)", got.Total, test.wantGross)
			}
		})
	}
}

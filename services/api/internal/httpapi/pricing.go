package httpapi

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

// pricingPreviewRequest is intentionally transport-specific. The document
// contracts keep monetary values as decimal strings, while the pricing
// package calculates in integer minor units and basis points.
type pricingPreviewRequest struct {
	PriceLevel              int                  `json:"priceLevel"`
	Lines                   []pricingPreviewLine `json:"lines"`
	GroupDiscountPercent    string               `json:"groupDiscountPercent"`
	CustomerDiscountPercent *string              `json:"customerDiscountPercent"`
	OverrideDiscountPercent *string              `json:"overrideDiscountPercent"`
	DocumentDiscountPercent string               `json:"documentDiscountPercent"`
	FlatDiscountAmount      string               `json:"flatDiscountAmount"`
	MiscAmount              *string              `json:"miscAmount"`
	Taxes                   pricingPreviewTaxes  `json:"taxes"`
}

type pricingPreviewLine struct {
	ID                  string                        `json:"id"`
	Quantity            string                        `json:"quantity"`
	Prices              []string                      `json:"prices"`
	UnitPrice           string                        `json:"unitPrice"`
	ItemDiscountPercent string                        `json:"itemDiscountPercent"`
	SupplierScheme      *pricingPreviewSupplierScheme `json:"supplierScheme"`
}

type pricingPreviewSupplierScheme struct {
	DiscountPercent    string `json:"discountPercent"`
	QualifyingQuantity string `json:"qualifyingQuantity"`
	BonusQuantity      string `json:"bonusQuantity"`
}

type pricingPreviewTaxes struct {
	GST        *pricingPreviewTax `json:"gst"`
	PCT        *pricingPreviewTax `json:"pct"`
	AdvanceTax *pricingPreviewTax `json:"advanceTax"`
}

type pricingPreviewTax struct {
	Rate      string `json:"rate"`
	Inclusive bool   `json:"inclusive"`
}

// previewPricing returns the same deterministic totals used by a document
// post. It is scoped by the authenticated tenant and deliberately does not
// write anything, so the UI can recalculate while a legacy form is edited.
func (s *Server) previewPricing(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.read") {
		return
	}
	var input pricingPreviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&input); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The pricing request could not be parsed.")
		return
	}
	request, err := input.toPricingRequest()
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_pricing_request", "Invalid pricing request", err.Error())
		return
	}
	result, err := pricing.Calculate(request)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "pricing_rejected", "Pricing rejected", err.Error())
		return
	}

	taxes := make([]map[string]any, 0, len(result.Taxes))
	for _, tax := range result.Taxes {
		taxes = append(taxes, map[string]any{
			"kind":      taxKindName(tax.Kind),
			"rate":      formatPercent(tax.Rate),
			"inclusive": tax.Inclusive,
			"base":      formatMoney(tax.Base),
			"amount":    formatMoney(tax.Amount),
		})
	}
	lines := make([]map[string]any, 0, len(result.Lines))
	for _, line := range result.Lines {
		lines = append(lines, map[string]any{
			"id":                     line.ID,
			"priceLevel":             line.PriceLevel,
			"resolvedUnitPrice":      formatMoney(line.ResolvedUnitPrice),
			"supplierDiscount":       formatMoney(line.SupplierDiscount),
			"supplierBonusQuantity":  formatQuantity(line.SupplierBonusQuantity),
			"lineGross":              formatMoney(line.LineGross),
			"itemDiscount":           formatMoney(line.ItemDiscount),
			"customerDiscount":       formatMoney(line.CustomerDiscount),
			"customerDiscountRate":   formatPercent(line.CustomerDiscountRate),
			"customerDiscountSource": customerDiscountSourceName(line.CustomerDiscountSource),
			"net":                    formatMoney(line.Net),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenantId":                operator.TenantID,
		"branchId":                operator.BranchID,
		"priceLevel":              request.PriceLevel,
		"lines":                   lines,
		"subtotal":                formatMoney(result.Subtotal),
		"lineDiscountTotal":       formatMoney(result.LineDiscountTotal),
		"documentPercentDiscount": formatMoney(result.DocumentPercentDiscount),
		"flatDiscount":            formatMoney(result.FlatDiscount),
		"documentDiscountTotal":   formatMoney(result.DocumentDiscountTotal),
		"misc":                    formatMoney(result.Misc),
		"taxableBase":             formatMoney(result.TaxableBase),
		"taxes":                   taxes,
		"totalDiscount":           formatMoney(result.TotalDiscount),
		"total":                   formatMoney(result.Total),
	})
}

func (input pricingPreviewRequest) toPricingRequest() (pricing.Request, error) {
	level := input.PriceLevel
	if level == 0 {
		level = 1
	}
	request := pricing.Request{PriceLevel: level, Rounding: pricing.DefaultRoundingPolicy()}
	var err error
	request.Customer.GroupPercent, err = parsePercent(input.GroupDiscountPercent)
	if err != nil {
		return pricing.Request{}, fmt.Errorf("group discount: %w", err)
	}
	if input.CustomerDiscountPercent != nil {
		value, parseErr := parsePercent(*input.CustomerDiscountPercent)
		if parseErr != nil {
			return pricing.Request{}, fmt.Errorf("customer discount: %w", parseErr)
		}
		request.Customer.CustomerPercent = &value
	}
	if input.OverrideDiscountPercent != nil {
		value, parseErr := parsePercent(*input.OverrideDiscountPercent)
		if parseErr != nil {
			return pricing.Request{}, fmt.Errorf("override discount: %w", parseErr)
		}
		request.Customer.OverridePercent = &value
	}
	request.DocumentPercent, err = parsePercent(input.DocumentDiscountPercent)
	if err != nil {
		return pricing.Request{}, fmt.Errorf("document discount: %w", err)
	}
	request.FlatDiscount, err = parseMoney(input.FlatDiscountAmount)
	if err != nil {
		return pricing.Request{}, fmt.Errorf("flat discount: %w", err)
	}
	if input.MiscAmount != nil && strings.TrimSpace(*input.MiscAmount) != "" {
		value, parseErr := parseMoney(*input.MiscAmount)
		if parseErr != nil {
			return pricing.Request{}, fmt.Errorf("misc: %w", parseErr)
		}
		request.Misc = &value
	}
	request.Taxes, err = input.Taxes.toPricingPolicy()
	if err != nil {
		return pricing.Request{}, err
	}
	request.Lines = make([]pricing.LineInput, 0, len(input.Lines))
	for index, line := range input.Lines {
		id := strings.TrimSpace(line.ID)
		if id == "" {
			return pricing.Request{}, fmt.Errorf("line %d ID is required", index+1)
		}
		quantity, parseErr := parseQuantity(line.Quantity)
		if parseErr != nil {
			return pricing.Request{}, fmt.Errorf("line %q quantity: %w", id, parseErr)
		}
		prices := pricing.PriceTiers{}
		priceValues := line.Prices
		if len(priceValues) == 0 && strings.TrimSpace(line.UnitPrice) != "" {
			priceValues = []string{line.UnitPrice}
		}
		if len(priceValues) == 0 {
			return pricing.Request{}, fmt.Errorf("line %q needs at least one price", id)
		}
		if len(priceValues) > len(prices) {
			return pricing.Request{}, fmt.Errorf("line %q has more than 10 price tiers", id)
		}
		for tier, value := range priceValues {
			prices[tier], parseErr = parseMoney(value)
			if parseErr != nil {
				return pricing.Request{}, fmt.Errorf("line %q price %d: %w", id, tier+1, parseErr)
			}
		}
		itemDiscount, parseErr := parsePercent(line.ItemDiscountPercent)
		if parseErr != nil {
			return pricing.Request{}, fmt.Errorf("line %q item discount: %w", id, parseErr)
		}
		converted := pricing.LineInput{ID: id, Quantity: quantity, Prices: prices, ItemDiscount: itemDiscount}
		if line.SupplierScheme != nil {
			discount, schemeErr := parsePercent(line.SupplierScheme.DiscountPercent)
			if schemeErr != nil {
				return pricing.Request{}, fmt.Errorf("line %q supplier discount: %w", id, schemeErr)
			}
			qualifying, schemeErr := parseNonNegativeQuantity(line.SupplierScheme.QualifyingQuantity)
			if schemeErr != nil {
				return pricing.Request{}, fmt.Errorf("line %q supplier qualifying quantity: %w", id, schemeErr)
			}
			bonus, schemeErr := parseNonNegativeQuantity(line.SupplierScheme.BonusQuantity)
			if schemeErr != nil {
				return pricing.Request{}, fmt.Errorf("line %q supplier bonus quantity: %w", id, schemeErr)
			}
			converted.SupplierScheme = &pricing.SupplierScheme{DiscountPercent: discount, QualifyingQuantity: qualifying, BonusQuantity: bonus}
		}
		request.Lines = append(request.Lines, converted)
	}
	return request, nil
}

func (taxes pricingPreviewTaxes) toPricingPolicy() (pricing.TaxPolicy, error) {
	var result pricing.TaxPolicy
	for _, item := range []struct {
		input *pricingPreviewTax
		kind  pricing.TaxKind
		name  string
		out   **pricing.TaxRule
	}{
		{taxes.GST, pricing.TaxGST, "GST", &result.GST},
		{taxes.PCT, pricing.TaxPCT, "PCT", &result.PCT},
		{taxes.AdvanceTax, pricing.TaxAdvance, "advance tax", &result.AdvanceTax},
	} {
		if item.input == nil || strings.TrimSpace(item.input.Rate) == "" {
			continue
		}
		rate, err := parsePercent(item.input.Rate)
		if err != nil {
			return pricing.TaxPolicy{}, fmt.Errorf("%s rate: %w", item.name, err)
		}
		*item.out = &pricing.TaxRule{Kind: item.kind, Rate: rate, Inclusive: item.input.Inclusive}
	}
	return result, nil
}

func parseMoney(value string) (pricing.Money, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("%q is not a decimal amount", value)
	}
	rat.Mul(rat, big.NewRat(100, 1))
	if !rat.IsInt() || rat.Sign() < 0 || !rat.Num().IsInt64() {
		return 0, fmt.Errorf("%q must be a non-negative amount with at most two decimal places", value)
	}
	return pricing.Money(rat.Num().Int64()), nil
}

func parsePercent(value string) (pricing.BasisPoints, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("%q is not a decimal percentage", value)
	}
	rat.Mul(rat, big.NewRat(100, 1))
	if !rat.IsInt() || rat.Sign() < 0 || !rat.Num().IsInt64() {
		return 0, fmt.Errorf("%q must be a non-negative percentage with at most two decimal places", value)
	}
	return pricing.BasisPoints(rat.Num().Int64()), nil
}

func parseQuantity(value string) (pricing.Quantity, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("quantity is required")
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || !rat.IsInt() || rat.Sign() <= 0 || !rat.Num().IsInt64() {
		return 0, fmt.Errorf("%q must be a positive whole quantity", value)
	}
	return pricing.Quantity(rat.Num().Int64()), nil
}

func parseNonNegativeQuantity(value string) (pricing.Quantity, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || !rat.IsInt() || rat.Sign() < 0 || !rat.Num().IsInt64() {
		return 0, fmt.Errorf("%q must be a non-negative whole quantity", value)
	}
	return pricing.Quantity(rat.Num().Int64()), nil
}

func formatMoney(value pricing.Money) string {
	minor := int64(value)
	return fmt.Sprintf("%d.%02d", minor/100, minor%100)
}

func formatPercent(value pricing.BasisPoints) string {
	return fmt.Sprintf("%d.%02d", int64(value)/100, int64(value)%100)
}

func formatStoredTaxRate(value pricing.BasisPoints) string {
	return fmt.Sprintf("%d.%04d", int64(value)/100, (int64(value)%100)*100)
}

func formatQuantity(value pricing.Quantity) string {
	return fmt.Sprintf("%d", int64(value))
}

func taxKindName(kind pricing.TaxKind) string {
	switch kind {
	case pricing.TaxGST:
		return "gst"
	case pricing.TaxPCT:
		return "pct"
	case pricing.TaxAdvance:
		return "advance_tax"
	default:
		return "unknown"
	}
}

func customerDiscountSourceName(source pricing.CustomerDiscountSource) string {
	switch source {
	case pricing.CustomerDiscountFromCustomer:
		return "customer"
	case pricing.CustomerDiscountFromOverride:
		return "override"
	default:
		return "group"
	}
}

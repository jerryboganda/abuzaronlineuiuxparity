package httpapi

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

func isPricedSaleDocumentKind(kind string) bool {
	switch kind {
	case "cash-sale", "credit-sale", "quotation", "refused-sale":
		return true
	default:
		return false
	}
}

func defaultSalePriceLevel(ctx context.Context, tx *sql.Tx, operator *sessionContext, kind string) int {
	caption := ""
	switch kind {
	case "cash-sale":
		caption = "Price # in Cash Sale:"
	case "credit-sale":
		caption = "Price # in Credit Sale:"
	}
	if caption == "" {
		return 1
	}
	value, err := effectivePreferenceValue(ctx, tx, operator, "Sale", caption)
	if err != nil || strings.TrimSpace(value) == "" {
		if definition, ok := preferenceDefinitionMap("Sale")[caption]; ok {
			value = strings.TrimSpace(definition.Default)
		}
	}
	level, convErr := strconv.Atoi(strings.TrimSpace(value))
	if convErr != nil || level < 1 || level > 10 {
		return 1
	}
	return level
}

func priceLevelAllowed(operator *sessionContext, level int) bool {
	if operator == nil {
		return false
	}
	if hasTenantAdminRole(operator.Roles) {
		return true
	}
	entries, scoped := operator.Scopes["price"]
	if !scoped || len(entries) == 0 {
		return true
	}
	if entries["*"] {
		return true
	}
	for _, key := range priceLevelScopeKeys(level) {
		if entries[key] {
			return true
		}
	}
	return false
}

func priceLevelScopeKeys(level int) []string {
	return []string{
		strconv.Itoa(level),
		"SalePrice#" + strconv.Itoa(level),
		"saleprice" + strconv.Itoa(level),
		"Price" + strconv.Itoa(level),
	}
}

func applyItemPricePolicy(ctx context.Context, tx *sql.Tx, tenantID, itemID, quantity string, priceLevel int, tiers []string, discountPercent, occurredAt string) ([]string, string, error) {
	policy, err := queryItemPricePolicy(ctx, tx, tenantID, itemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return tiers, discountPercent, nil
		}
		return tiers, discountPercent, err
	}
	if policy.Policy == nil || len(policy.Tiers) == 0 {
		return tiers, discountPercent, nil
	}
	asOf := time.Now().UTC()
	if strings.TrimSpace(occurredAt) != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, occurredAt); parseErr == nil {
			asOf = parsed
		}
	}
	matched := selectPricePolicyTier(policy.Tiers, quantityFloor(quantity), asOf)
	if matched == nil {
		return tiers, discountPercent, nil
	}
	tiers = overlayPricePolicyOnTiers(tiers, priceLevel, matched.Price)
	if strings.TrimSpace(discountPercent) == "" && policyDiscountApplies(matched.DiscountPercent) {
		discountPercent = strings.TrimSpace(matched.DiscountPercent)
	}
	return tiers, discountPercent, nil
}

func selectPricePolicyTier(tiers []itemPricePolicyTierResponse, quantity int, asOf time.Time) *itemPricePolicyTierResponse {
	day := asOf.UTC().Truncate(24 * time.Hour)
	var best *itemPricePolicyTierResponse
	for index := range tiers {
		tier := &tiers[index]
		if tier.QuantityLimit > quantity {
			continue
		}
		if strings.TrimSpace(tier.ExpiryDate) != "" {
			expiry, err := time.Parse("2006-01-02", strings.TrimSpace(tier.ExpiryDate))
			if err != nil || expiry.Before(day) {
				continue
			}
		}
		if best == nil || tier.QuantityLimit > best.QuantityLimit {
			best = tier
		}
	}
	return best
}

func overlayPricePolicyOnTiers(tiers []string, priceLevel int, policyPrice string) []string {
	if !policyPriceApplies(policyPrice) {
		return tiers
	}
	out := append([]string(nil), tiers...)
	if priceLevel >= 1 && priceLevel <= len(out) {
		out[priceLevel-1] = strings.TrimSpace(policyPrice)
	}
	return out
}

func policyPriceApplies(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}
	return parsed > 0
}

func policyDiscountApplies(value string) bool {
	return policyPriceApplies(value)
}

func quantityFloor(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return int(parsed)
}

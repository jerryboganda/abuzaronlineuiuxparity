// Package pricing contains the isolated, policy-driven pricing calculation
// used by future document workflows. It deliberately has no database or
// transport dependencies.
//
// Calculation order:
//  1. Select the requested price tier.
//  2. Apply a supplier scheme discount and expose any qualifying bonus units.
//  3. Apply the item discount, then the selected customer/group discount.
//  4. Sum lines, apply the document percentage discount, then the flat
//     discount, and finally add Misc (1.00 when it is not supplied).
//  5. Apply GST, PCT, and advance tax in that order. Each tax may be
//     inclusive or exclusive.
//
// Money is integer minor units (paisa/cents) and percentages are basis points
// (10000 = 100%). All percentage arithmetic is performed with math/big.Rat;
// floating point is intentionally not used. Rounding happens only at the
// configured line, tax, and document boundaries.
package pricing

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
)

const (
	maxInt64Value int64 = 1<<63 - 1
	moneyScale          = int64(100)
	percentScale        = int64(10000)
	defaultMisc         = Money(moneyScale)
)

// Money is a non-negative amount in the smallest document currency unit.
// For the current policy that unit is a paisa/cents. Values are validated by
// Calculate; a direct conversion such as Money(-1) is therefore rejected.
type Money int64

// BasisPoints is a percentage multiplied by 100: 7.5% is 750.
type BasisPoints int64

// Quantity is a whole-unit quantity. Supplier bonus quantities are reported
// separately and are not billed by this engine.
type Quantity int64

// RoundingMode controls conversion of an exact rational amount to Money.
type RoundingMode uint8

const (
	// RoundHalfUp rounds a half away from zero. All engine amounts are
	// non-negative, so this is the usual "5 rounds up" policy.
	RoundHalfUp RoundingMode = iota
	// RoundHalfEven rounds ties to the nearest even minor unit.
	RoundHalfEven
	// RoundDown truncates toward zero (floor for the non-negative amounts here).
	RoundDown
)

// RoundingPolicy makes every monetary rounding boundary explicit.
type RoundingPolicy struct {
	Line     RoundingMode
	Tax      RoundingMode
	Document RoundingMode
}

// DefaultRoundingPolicy is deterministic and intentionally not a claim of
// parity with the legacy application.
func DefaultRoundingPolicy() RoundingPolicy {
	return RoundingPolicy{
		Line:     RoundHalfUp,
		Tax:      RoundHalfUp,
		Document: RoundHalfUp,
	}
}

// PriceTiers contains SalePrice#1 through SalePrice#10, in order.
type PriceTiers [10]Money

// SupplierScheme describes a supplier quantity scheme. For every
// QualifyingQuantity billed units, BonusQuantity free units are exposed in the
// result. Bonus units do not increase the billed line amount.
type SupplierScheme struct {
	DiscountPercent    BasisPoints
	QualifyingQuantity Quantity
	BonusQuantity      Quantity
}

// CustomerDiscounts defines precedence explicitly:
// OverridePercent > CustomerPercent > GroupPercent.
// Pointers distinguish an explicit 0% override from an omitted value.
type CustomerDiscounts struct {
	GroupPercent    BasisPoints
	CustomerPercent *BasisPoints
	OverridePercent *BasisPoints
}

// LineInput is the complete policy input for one priced line.
type LineInput struct {
	ID             string
	Quantity       Quantity
	Prices         PriceTiers
	ItemDiscount   BasisPoints
	SupplierScheme *SupplierScheme
}

// TaxKind identifies one of the supported taxes. TaxPolicy applies fields in
// this fixed order: GST, PCT, then advance income tax.
type TaxKind uint8

const (
	TaxGST TaxKind = iota
	TaxPCT
	TaxAdvance
)

// TaxRule applies Rate to the current taxable amount. Inclusive means the
// current amount already contains this tax; exclusive adds it.
type TaxRule struct {
	Kind      TaxKind
	Rate      BasisPoints
	Inclusive bool
}

// TaxPolicy is intentionally a fixed-shape policy so tax ordering cannot be
// changed accidentally by a caller.
type TaxPolicy struct {
	GST        *TaxRule
	PCT        *TaxRule
	AdvanceTax *TaxRule
}

// Request contains all policy inputs needed for one deterministic calculation.
type Request struct {
	PriceLevel      int
	Lines           []LineInput
	Customer        CustomerDiscounts
	DocumentPercent BasisPoints
	FlatDiscount    Money
	Misc            *Money
	Taxes           TaxPolicy
	Rounding        RoundingPolicy
}

// CustomerDiscountSource records which customer policy won precedence.
type CustomerDiscountSource uint8

const (
	CustomerDiscountFromGroup CustomerDiscountSource = iota
	CustomerDiscountFromCustomer
	CustomerDiscountFromOverride
)

// LineResult exposes each line boundary for auditability. Document discounts
// are not allocated back into lines.
type LineResult struct {
	ID                     string
	PriceLevel             int
	ResolvedUnitPrice      Money
	SupplierDiscount       Money
	SupplierBonusQuantity  Quantity
	LineGross              Money
	ItemDiscount           Money
	CustomerDiscount       Money
	CustomerDiscountRate   BasisPoints
	CustomerDiscountSource CustomerDiscountSource
	Net                    Money
}

// TaxResult exposes the base and rounded amount for each applied tax.
type TaxResult struct {
	Kind      TaxKind
	Rate      BasisPoints
	Inclusive bool
	Base      Money
	Amount    Money
}

// Result is the complete auditable output of Calculate.
type Result struct {
	Lines                   []LineResult
	Subtotal                Money
	LineDiscountTotal       Money
	DocumentPercentDiscount Money
	FlatDiscount            Money
	DocumentDiscountTotal   Money
	Misc                    Money
	TaxableBase             Money
	Taxes                   []TaxResult
	TotalDiscount           Money
	Total                   Money
}

var (
	// ErrInvalidInput means a policy value violates the engine contract.
	ErrInvalidInput = errors.New("invalid pricing input")
	// ErrOverflow means a valid calculation cannot be represented by Money or
	// Quantity's signed 64-bit storage.
	ErrOverflow = errors.New("pricing calculation overflow")
)

// Calculate applies the documented policy order without external state.
func Calculate(request Request) (Result, error) {
	if err := validateRequest(request); err != nil {
		return Result{}, err
	}

	result := Result{
		Lines: make([]LineResult, 0, len(request.Lines)),
		Taxes: make([]TaxResult, 0, 3),
	}
	seenIDs := make(map[string]struct{}, len(request.Lines))
	for _, line := range request.Lines {
		if _, exists := seenIDs[line.ID]; exists {
			return Result{}, invalid("duplicate line ID %q", line.ID)
		}
		seenIDs[line.ID] = struct{}{}

		price := line.Prices[request.PriceLevel-1]
		var err error
		baseGrossRat := new(big.Rat).Mul(moneyRat(price), new(big.Rat).SetInt64(int64(line.Quantity)))
		grossRat := new(big.Rat).Set(baseGrossRat)
		supplierDiscount := Money(0)
		bonusQuantity := Quantity(0)
		if line.SupplierScheme != nil {
			scheme := line.SupplierScheme
			supplierFactor := new(big.Rat).SetFrac(
				big.NewInt(percentScale-int64(scheme.DiscountPercent)),
				big.NewInt(percentScale),
			)
			grossRat.Mul(grossRat, supplierFactor)
		}

		lineGross, err := roundMoney(grossRat, request.Rounding.Line)
		if err != nil {
			return Result{}, fmt.Errorf("line %q gross: %w", line.ID, err)
		}
		if line.SupplierScheme != nil {
			supplierDiscount, err = roundMoney(
				new(big.Rat).Sub(baseGrossRat, moneyRat(lineGross)),
				request.Rounding.Line,
			)
			if err != nil {
				return Result{}, fmt.Errorf("line %q supplier discount: %w", line.ID, err)
			}
			if line.SupplierScheme.BonusQuantity > 0 {
				bonusQuantity, err = calculateBonusQuantity(line.Quantity, *line.SupplierScheme)
				if err != nil {
					return Result{}, fmt.Errorf("line %q supplier bonus: %w", line.ID, err)
				}
			}
		}
		itemDiscount, err := percentageOf(lineGross, line.ItemDiscount, request.Rounding.Line)
		if err != nil {
			return Result{}, fmt.Errorf("line %q item discount: %w", line.ID, err)
		}
		afterItem, err := subtractMoney(lineGross, itemDiscount)
		if err != nil {
			return Result{}, fmt.Errorf("line %q after item discount: %w", line.ID, err)
		}
		customerRate, customerSource := request.Customer.selected()
		customerDiscount, err := percentageOf(afterItem, customerRate, request.Rounding.Line)
		if err != nil {
			return Result{}, fmt.Errorf("line %q customer discount: %w", line.ID, err)
		}
		net, err := subtractMoney(afterItem, customerDiscount)
		if err != nil {
			return Result{}, fmt.Errorf("line %q net: %w", line.ID, err)
		}
		lineDiscountTotal, err := addMoney(supplierDiscount, itemDiscount)
		if err == nil {
			lineDiscountTotal, err = addMoney(lineDiscountTotal, customerDiscount)
		}
		if err != nil {
			return Result{}, fmt.Errorf("line %q discount total: %w", line.ID, err)
		}
		result.LineDiscountTotal, err = addMoney(result.LineDiscountTotal, lineDiscountTotal)
		if err != nil {
			return Result{}, fmt.Errorf("line discount total: %w", err)
		}
		result.Subtotal, err = addMoney(result.Subtotal, net)
		if err != nil {
			return Result{}, fmt.Errorf("subtotal: %w", err)
		}
		result.Lines = append(result.Lines, LineResult{
			ID:                     line.ID,
			PriceLevel:             request.PriceLevel,
			ResolvedUnitPrice:      price,
			SupplierDiscount:       supplierDiscount,
			SupplierBonusQuantity:  bonusQuantity,
			LineGross:              lineGross,
			ItemDiscount:           itemDiscount,
			CustomerDiscount:       customerDiscount,
			CustomerDiscountRate:   customerRate,
			CustomerDiscountSource: customerSource,
			Net:                    net,
		})
	}

	var err error
	result.DocumentPercentDiscount, err = percentageOf(
		result.Subtotal, request.DocumentPercent, request.Rounding.Document,
	)
	if err != nil {
		return Result{}, fmt.Errorf("document percentage discount: %w", err)
	}
	afterPercent, err := subtractMoney(result.Subtotal, result.DocumentPercentDiscount)
	if err != nil {
		return Result{}, fmt.Errorf("after document percentage discount: %w", err)
	}
	result.FlatDiscount = request.FlatDiscount
	afterFlat, err := subtractMoney(afterPercent, result.FlatDiscount)
	if err != nil {
		return Result{}, fmt.Errorf("flat discount: %w", err)
	}
	if request.Misc == nil {
		result.Misc = defaultMisc
	} else {
		result.Misc = *request.Misc
	}
	result.TaxableBase, err = addMoney(afterFlat, result.Misc)
	if err != nil {
		return Result{}, fmt.Errorf("taxable base: %w", err)
	}
	result.DocumentDiscountTotal, err = addMoney(result.DocumentPercentDiscount, result.FlatDiscount)
	if err != nil {
		return Result{}, fmt.Errorf("document discount total: %w", err)
	}
	result.TotalDiscount, err = addMoney(result.LineDiscountTotal, result.DocumentDiscountTotal)
	if err != nil {
		return Result{}, fmt.Errorf("total discount: %w", err)
	}

	current := result.TaxableBase
	total := result.TaxableBase
	for _, tax := range orderedTaxes(request.Taxes) {
		base := current
		var amount Money
		if tax.Inclusive {
			taxRat := new(big.Rat).Mul(
				moneyRat(base),
				new(big.Rat).SetFrac(
					big.NewInt(int64(tax.Rate)),
					big.NewInt(percentScale+int64(tax.Rate)),
				),
			)
			amount, err = roundMoney(taxRat, request.Rounding.Tax)
			if err == nil {
				current, err = subtractMoney(base, amount)
			}
		} else {
			amount, err = percentageOf(base, tax.Rate, request.Rounding.Tax)
			if err == nil {
				current, err = addMoney(base, amount)
			}
			if err == nil {
				total, err = addMoney(total, amount)
			}
		}
		if err != nil {
			return Result{}, fmt.Errorf("%s tax: %w", taxName(tax.Kind), err)
		}
		result.Taxes = append(result.Taxes, TaxResult{
			Kind:      tax.Kind,
			Rate:      tax.Rate,
			Inclusive: tax.Inclusive,
			Base:      base,
			Amount:    amount,
		})
	}
	result.Total = total
	return result, nil
}

func validateRequest(request Request) error {
	if request.PriceLevel < 1 || request.PriceLevel > 10 {
		return invalid("price level must be between 1 and 10")
	}
	if len(request.Lines) == 0 {
		return invalid("at least one line is required")
	}
	if err := validatePercent(request.DocumentPercent, "document percentage"); err != nil {
		return err
	}
	if request.FlatDiscount < 0 {
		return invalid("flat discount cannot be negative")
	}
	if request.Misc != nil && *request.Misc < 0 {
		return invalid("misc cannot be negative")
	}
	if err := validatePercent(request.Customer.GroupPercent, "customer group percentage"); err != nil {
		return err
	}
	if request.Customer.CustomerPercent != nil {
		if err := validatePercent(*request.Customer.CustomerPercent, "customer percentage"); err != nil {
			return err
		}
	}
	if request.Customer.OverridePercent != nil {
		if err := validatePercent(*request.Customer.OverridePercent, "customer override percentage"); err != nil {
			return err
		}
	}
	if err := validateTaxRule(request.Taxes.GST, TaxGST); err != nil {
		return err
	}
	if err := validateTaxRule(request.Taxes.PCT, TaxPCT); err != nil {
		return err
	}
	if err := validateTaxRule(request.Taxes.AdvanceTax, TaxAdvance); err != nil {
		return err
	}
	if err := validateRounding(request.Rounding); err != nil {
		return err
	}

	seenIDs := make(map[string]struct{}, len(request.Lines))
	for _, line := range request.Lines {
		if line.ID == "" {
			return invalid("line ID cannot be empty")
		}
		if _, exists := seenIDs[line.ID]; exists {
			return invalid("duplicate line ID %q", line.ID)
		}
		seenIDs[line.ID] = struct{}{}
		if line.Quantity <= 0 {
			return invalid("line %q quantity must be positive", line.ID)
		}
		for tier, price := range line.Prices {
			if price < 0 {
				return invalid("line %q price tier %d cannot be negative", line.ID, tier+1)
			}
		}
		if err := validatePercent(line.ItemDiscount, "item discount"); err != nil {
			return fmt.Errorf("line %q: %w", line.ID, err)
		}
		if line.SupplierScheme != nil {
			scheme := line.SupplierScheme
			if err := validatePercent(scheme.DiscountPercent, "supplier scheme discount"); err != nil {
				return fmt.Errorf("line %q: %w", line.ID, err)
			}
			if scheme.QualifyingQuantity < 0 || scheme.BonusQuantity < 0 {
				return invalid("line %q supplier quantities cannot be negative", line.ID)
			}
			if scheme.BonusQuantity > 0 && scheme.QualifyingQuantity <= 0 {
				return invalid("line %q supplier qualifying quantity must be positive when bonus is used", line.ID)
			}
		}
	}
	return nil
}

func validateTaxRule(rule *TaxRule, expected TaxKind) error {
	if rule == nil {
		return nil
	}
	if rule.Kind != expected {
		return invalid("%s tax has the wrong tax kind", taxName(expected))
	}
	if err := validatePercent(rule.Rate, taxName(expected)+" rate"); err != nil {
		return err
	}
	return nil
}

func validatePercent(value BasisPoints, name string) error {
	if value < 0 || value > BasisPoints(percentScale) {
		return invalid("%s must be between 0 and 100 percent", name)
	}
	return nil
}

func validateRounding(policy RoundingPolicy) error {
	if policy.Line > RoundDown {
		return invalid("line rounding mode is unsupported")
	}
	if policy.Tax > RoundDown {
		return invalid("tax rounding mode is unsupported")
	}
	if policy.Document > RoundDown {
		return invalid("document rounding mode is unsupported")
	}
	return nil
}

func (discounts CustomerDiscounts) selected() (BasisPoints, CustomerDiscountSource) {
	if discounts.OverridePercent != nil {
		return *discounts.OverridePercent, CustomerDiscountFromOverride
	}
	if discounts.CustomerPercent != nil {
		return *discounts.CustomerPercent, CustomerDiscountFromCustomer
	}
	return discounts.GroupPercent, CustomerDiscountFromGroup
}

func percentageOf(value Money, rate BasisPoints, mode RoundingMode) (Money, error) {
	rat := new(big.Rat).Mul(
		moneyRat(value),
		new(big.Rat).SetFrac(big.NewInt(int64(rate)), big.NewInt(percentScale)),
	)
	return roundMoney(rat, mode)
}

func calculateBonusQuantity(quantity Quantity, scheme SupplierScheme) (Quantity, error) {
	qualifyingUnits := int64(quantity) / int64(scheme.QualifyingQuantity)
	bonus := new(big.Int).Mul(
		big.NewInt(qualifyingUnits),
		big.NewInt(int64(scheme.BonusQuantity)),
	)
	if bonus.Cmp(big.NewInt(maxInt64Value)) > 0 {
		return 0, ErrOverflow
	}
	return Quantity(bonus.Int64()), nil
}

func roundMoney(value *big.Rat, mode RoundingMode) (Money, error) {
	if value.Sign() < 0 {
		return 0, invalid("calculation produced a negative amount")
	}
	numerator := new(big.Int).Set(value.Num())
	denominator := new(big.Int).Set(value.Denom())
	quotient, remainder := new(big.Int).QuoRem(numerator, denominator, new(big.Int))
	roundUp := false
	switch mode {
	case RoundDown:
	case RoundHalfUp, RoundHalfEven:
		twiceRemainder := new(big.Int).Lsh(new(big.Int).Set(remainder), 1)
		comparison := twiceRemainder.Cmp(denominator)
		roundUp = comparison > 0 ||
			(comparison == 0 && (mode == RoundHalfUp || quotient.Bit(0) == 1))
	default:
		return 0, invalid("unsupported rounding mode %d", mode)
	}
	if roundUp {
		quotient.Add(quotient, big.NewInt(1))
	}
	if quotient.Cmp(big.NewInt(maxInt64Value)) > 0 {
		return 0, ErrOverflow
	}
	return Money(quotient.Int64()), nil
}

func moneyRat(value Money) *big.Rat {
	return new(big.Rat).SetInt64(int64(value))
}

func addMoney(left, right Money) (Money, error) {
	if left < 0 || right < 0 {
		return 0, invalid("calculation received a negative amount")
	}
	if int64(left) > maxInt64Value-int64(right) {
		return 0, ErrOverflow
	}
	return left + right, nil
}

func subtractMoney(left, right Money) (Money, error) {
	if left < 0 || right < 0 {
		return 0, invalid("calculation received a negative amount")
	}
	if right > left {
		return 0, invalid("discount exceeds the amount it applies to")
	}
	return left - right, nil
}

func orderedTaxes(policy TaxPolicy) []*TaxRule {
	taxes := make([]*TaxRule, 0, 3)
	if policy.GST != nil {
		taxes = append(taxes, policy.GST)
	}
	if policy.PCT != nil {
		taxes = append(taxes, policy.PCT)
	}
	if policy.AdvanceTax != nil {
		taxes = append(taxes, policy.AdvanceTax)
	}
	return taxes
}

func taxName(kind TaxKind) string {
	switch kind {
	case TaxGST:
		return "GST"
	case TaxPCT:
		return "PCT"
	case TaxAdvance:
		return "advance"
	default:
		return "unknown"
	}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}

// String renders Money as major.minor for diagnostics without using floats.
func (money Money) String() string {
	sign := ""
	value := int64(money)
	if value < 0 {
		sign = "-"
		value = -value
	}
	return sign + strconv.FormatInt(value/moneyScale, 10) + "." +
		strconv.FormatInt(value%moneyScale, 10)
}

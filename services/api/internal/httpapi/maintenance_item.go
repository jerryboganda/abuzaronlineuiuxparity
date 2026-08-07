package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// canonicalMaintenanceMutation describes a maintenance action that changes a
// normalized item rather than only recording a preference-shaped value. The
// mutation and its operation audit are committed by maintenanceAction in one
// transaction.
type canonicalMaintenanceMutation struct {
	status       string
	message      string
	auditPayload map[string]any
}

func isCanonicalItemMaintenanceKind(kind string) bool {
	switch kind {
	case "change-items-price", "change-item-discount", "update-item-basic-data", "change-item-reorder-qty":
		return true
	default:
		return false
	}
}

func maintenancePayloadText(payload map[string]any, key string) (string, error) {
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be text", key)
	}
	return strings.TrimSpace(text), nil
}

func maintenanceNumericPayloadText(payload map[string]any, key string) (string, error) {
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	var text string
	switch value := value.(type) {
	case string:
		text = value
	case json.Number:
		text = value.String()
	case float64:
		text = strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		text = strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		text = strconv.Itoa(value)
	case int64:
		text = strconv.FormatInt(value, 10)
	case uint:
		text = strconv.FormatUint(uint64(value), 10)
	case uint64:
		text = strconv.FormatUint(value, 10)
	default:
		return "", fmt.Errorf("%s must be a number or decimal text", key)
	}
	return strings.TrimSpace(text), nil
}

func validateCanonicalItemMaintenancePayload(kind string, payload map[string]any) error {
	if !isCanonicalItemMaintenanceKind(kind) {
		return nil
	}
	if itemCode, err := maintenancePayloadText(payload, "itemCode"); err != nil || itemCode == "" {
		if err != nil {
			return err
		}
		return errors.New("itemCode is required")
	}
	switch kind {
	case "change-items-price":
		priceType, err := maintenancePayloadText(payload, "priceType")
		if err != nil || (priceType != "Sale Price" && priceType != "Purchase Price") {
			return errors.New("priceType must be Sale Price or Purchase Price")
		}
		price, err := maintenanceNumericPayloadText(payload, "price")
		if err != nil || price == "" {
			if err != nil {
				return err
			}
			return errors.New("price is required")
		}
		if _, err := parseMoney(price); err != nil {
			return fmt.Errorf("price: %w", err)
		}
		if effectiveDate, ok := payload["effectiveDate"]; ok {
			value, ok := effectiveDate.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return errors.New("effectiveDate must be an ISO date when supplied")
			}
			if _, err := time.Parse("2006-01-02", strings.TrimSpace(value)); err != nil {
				return errors.New("effectiveDate must be an ISO date when supplied")
			}
		}
	case "change-item-discount":
		discountType, err := maintenancePayloadText(payload, "discountType")
		if err != nil || (discountType != "Percent" && discountType != "Amount") {
			return errors.New("discountType must be Percent or Amount")
		}
		discount, err := maintenanceNumericPayloadText(payload, "discount")
		if err != nil || discount == "" {
			if err != nil {
				return err
			}
			return errors.New("discount is required")
		}
		if discountType == "Percent" {
			value, parseErr := parsePercent(discount)
			if parseErr != nil {
				return fmt.Errorf("discount: %w", parseErr)
			}
			if value > 10000 {
				return errors.New("discount percentage may not exceed 100")
			}
		} else if _, parseErr := parseMoney(discount); parseErr != nil {
			return fmt.Errorf("discount: %w", parseErr)
		}
	case "update-item-basic-data":
		field, err := maintenancePayloadText(payload, "field")
		if err != nil {
			return err
		}
		if _, ok := map[string]struct{}{"Name": {}, "Manufacturer": {}, "Category": {}, "Class": {}, "Location": {}}[field]; !ok {
			return fmt.Errorf("field %q is not supported by the canonical item maintenance workflow", field)
		}
		value, err := maintenancePayloadText(payload, "value")
		if err != nil || value == "" {
			if err != nil {
				return err
			}
			return errors.New("value is required")
		}
	case "change-item-reorder-qty":
		reorder, reorderErr := optionalNonNegativeMaintenanceQuantity(payload, "reorderQty")
		minimum, minimumErr := optionalNonNegativeMaintenanceQuantity(payload, "minimumQty")
		if reorderErr != nil {
			return reorderErr
		}
		if minimumErr != nil {
			return minimumErr
		}
		if !reorder && !minimum {
			return errors.New("reorderQty or minimumQty is required")
		}
	}
	return nil
}

func validateBatchLockMaintenancePayload(kind string, payload map[string]any) error {
	if kind != "lock-item-batches" {
		return nil
	}
	itemCode, err := maintenancePayloadText(payload, "itemCode")
	if err != nil || itemCode == "" {
		if err != nil {
			return err
		}
		return errors.New("itemCode is required")
	}
	batch, err := maintenancePayloadText(payload, "batch")
	if err != nil || batch == "" {
		if err != nil {
			return err
		}
		return errors.New("batch is required")
	}
	if _, err := maintenanceBooleanPayload(payload, "locked"); err != nil {
		return err
	}
	return nil
}

func maintenanceBooleanPayload(payload map[string]any, key string) (bool, error) {
	value, ok := payload[key]
	if !ok {
		return false, fmt.Errorf("%s is required", key)
	}
	switch value := value.(type) {
	case bool:
		return value, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "yes", "1", "on":
			return true, nil
		case "false", "no", "0", "off":
			return false, nil
		}
	}
	return false, fmt.Errorf("%s must be true/false or Yes/No", key)
}

func optionalNonNegativeMaintenanceQuantity(payload map[string]any, key string) (bool, error) {
	_, exists := payload[key]
	if !exists {
		return false, nil
	}
	text, err := maintenanceNumericPayloadText(payload, key)
	if err != nil {
		return false, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return false, nil
	}
	rat, ok := new(big.Rat).SetString(text)
	if !ok || rat.Sign() < 0 {
		return false, fmt.Errorf("%s must be a non-negative decimal", key)
	}
	if !new(big.Rat).Mul(rat, big.NewRat(10000, 1)).IsInt() {
		return false, fmt.Errorf("%s must have at most four decimal places", key)
	}
	return true, nil
}

func maintenanceJSONText(fields map[string]any, key string) string {
	value, ok := fields[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func lockMaintenanceItem(ctx context.Context, tx *sql.Tx, operator *sessionContext, identifier string) (string, string, string, map[string]any, error) {
	var itemID, code, legacyID string
	var rawPayload []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, code, legacy_id, payload
		FROM master_items
		WHERE tenant_id = $1::uuid AND (code = $2 OR legacy_id = $2)
		ORDER BY CASE WHEN code = $2 THEN 0 ELSE 1 END
		LIMIT 1
		FOR UPDATE
	`, operator.TenantID, identifier).Scan(&itemID, &code, &legacyID, &rawPayload)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", nil, fmt.Errorf("canonical item %q was not found", identifier)
	}
	if err != nil {
		return "", "", "", nil, err
	}
	fields := make(map[string]any)
	if len(rawPayload) > 0 && string(rawPayload) != "null" {
		if err := json.Unmarshal(rawPayload, &fields); err != nil {
			return "", "", "", nil, errors.New("canonical item payload is invalid")
		}
	}
	return itemID, code, legacyID, fields, nil
}

func updateMaintenanceItem(ctx context.Context, tx *sql.Tx, operator *sessionContext, itemID, name string, fields map[string]any, updateName bool) error {
	encoded, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var result sql.Result
	if updateName {
		result, err = tx.ExecContext(ctx, `
			UPDATE master_items
			SET name = $3, payload = $4::jsonb, updated_at = now()
			WHERE tenant_id = $1::uuid AND id = $2::uuid
		`, operator.TenantID, itemID, name, encoded)
	} else {
		result, err = tx.ExecContext(ctx, `
			UPDATE master_items
			SET payload = $3::jsonb, updated_at = now()
			WHERE tenant_id = $1::uuid AND id = $2::uuid
		`, operator.TenantID, itemID, encoded)
	}
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errors.New("canonical item was not updated")
	}
	return nil
}

func applyCanonicalItemMaintenance(ctx context.Context, tx *sql.Tx, operator *sessionContext, kind string, payload map[string]any) (canonicalMaintenanceMutation, error) {
	if !isCanonicalItemMaintenanceKind(kind) {
		return canonicalMaintenanceMutation{}, nil
	}
	identifier, _ := maintenancePayloadText(payload, "itemCode")
	itemID, code, legacyID, fields, err := lockMaintenanceItem(ctx, tx, operator, identifier)
	if err != nil {
		return canonicalMaintenanceMutation{}, err
	}
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["itemId"] = itemID
	auditPayload["canonicalCode"] = code
	auditPayload["canonicalLegacyId"] = legacyID
	updateName := false
	newName := ""
	message := ""

	switch kind {
	case "change-items-price":
		priceType, _ := maintenancePayloadText(payload, "priceType")
		priceText, _ := maintenanceNumericPayloadText(payload, "price")
		price, _ := parseMoney(priceText)
		formatted := formatMoney(price)
		if priceType == "Sale Price" {
			auditPayload["field"] = "SalePrice1"
			auditPayload["previousValue"] = maintenanceJSONText(fields, "SalePrice1")
			fields["SalePrice1"] = formatted
			fields["SalePrice"] = formatted
		} else {
			auditPayload["field"] = "PurchasePrice"
			auditPayload["previousValue"] = maintenanceJSONText(fields, "PurchasePrice")
			fields["PurchasePrice"] = formatted
		}
		auditPayload["newValue"] = formatted
		message = fmt.Sprintf("%s for item %s updated in the canonical item master.", priceType, code)
	case "change-item-discount":
		discountType, _ := maintenancePayloadText(payload, "discountType")
		discountText, _ := maintenanceNumericPayloadText(payload, "discount")
		if discountType == "Percent" {
			discount, _ := parsePercent(discountText)
			formatted := formatPercent(discount)
			auditPayload["field"] = "SaleDiscPercent"
			auditPayload["previousValue"] = maintenanceJSONText(fields, "SaleDiscPercent")
			fields["SaleDiscPercent"] = formatted
			fields["DiscPerc"] = formatted
			auditPayload["newValue"] = formatted
		} else {
			discount, _ := parseMoney(discountText)
			formatted := formatMoney(discount)
			auditPayload["field"] = "SaleFlatDiscount"
			auditPayload["previousValue"] = maintenanceJSONText(fields, "SaleFlatDiscount")
			fields["SaleFlatDiscount"] = formatted
			fields["itemflatdisc"] = formatted
			auditPayload["newValue"] = formatted
		}
		message = fmt.Sprintf("Sale discount for item %s updated in the canonical item master.", code)
	case "update-item-basic-data":
		field, _ := maintenancePayloadText(payload, "field")
		value, _ := maintenancePayloadText(payload, "value")
		auditPayload["field"] = field
		auditPayload["previousValue"] = maintenanceJSONText(fields, field)
		auditPayload["newValue"] = value
		fields[field] = value
		if field == "Name" {
			updateName = true
			newName = value
		}
		message = fmt.Sprintf("%s for item %s updated in the canonical item master.", field, code)
	case "change-item-reorder-qty":
		changed := make([]string, 0, 2)
		for _, key := range []string{"reorderQty", "minimumQty"} {
			_, exists := payload[key]
			if !exists {
				continue
			}
			valueText, valueErr := maintenanceNumericPayloadText(payload, key)
			if valueErr != nil || strings.TrimSpace(valueText) == "" {
				continue
			}
			rat, _ := new(big.Rat).SetString(strings.TrimSpace(valueText))
			formatted := formatStockQuantity(rat)
			field := map[string]string{"reorderQty": "ReorderQuantity", "minimumQty": "MinimumQuantity"}[key]
			auditPayload[field] = map[string]string{"previous": maintenanceJSONText(fields, field), "new": formatted}
			fields[field] = formatted
			changed = append(changed, field)
		}
		auditPayload["fields"] = changed
		message = fmt.Sprintf("Reorder settings for item %s updated in the canonical item master.", code)
	}
	if err := updateMaintenanceItem(ctx, tx, operator, itemID, newName, fields, updateName); err != nil {
		return canonicalMaintenanceMutation{}, err
	}
	return canonicalMaintenanceMutation{status: "completed", message: message, auditPayload: auditPayload}, nil
}

func applyBatchLockMaintenance(ctx context.Context, tx *sql.Tx, operator *sessionContext, payload map[string]any) (canonicalMaintenanceMutation, error) {
	identifier, _ := maintenancePayloadText(payload, "itemCode")
	batch, _ := maintenancePayloadText(payload, "batch")
	locked, _ := maintenanceBooleanPayload(payload, "locked")

	var itemID, code, legacyID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, code, legacy_id
		FROM master_items
		WHERE tenant_id = $1::uuid AND (code = $2 OR legacy_id = $2)
		ORDER BY CASE WHEN code = $2 THEN 0 ELSE 1 END
		LIMIT 1
		FOR UPDATE
	`, operator.TenantID, identifier).Scan(&itemID, &code, &legacyID)
	if errors.Is(err, sql.ErrNoRows) {
		return canonicalMaintenanceMutation{}, fmt.Errorf("canonical item %q was not found", identifier)
	}
	if err != nil {
		return canonicalMaintenanceMutation{}, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE stock_batches
		SET locked = $5
		WHERE tenant_id = $1::uuid
		  AND branch_id = $2::uuid
		  AND item_id = $3::uuid
		  AND batch_number = $4
	`, operator.TenantID, operator.BranchID, itemID, batch, locked)
	if err != nil {
		return canonicalMaintenanceMutation{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return canonicalMaintenanceMutation{}, err
	}
	if affected == 0 {
		return canonicalMaintenanceMutation{}, fmt.Errorf("batch %q for item %s was not found in the current branch", batch, code)
	}

	state := "unlocked"
	if locked {
		state = "locked"
	}
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["itemId"] = itemID
	auditPayload["canonicalCode"] = code
	auditPayload["canonicalLegacyId"] = legacyID
	auditPayload["affectedBatches"] = affected
	auditPayload["locked"] = locked
	return canonicalMaintenanceMutation{
		status:       "completed",
		message:      fmt.Sprintf("%d batch row(s) for item %s were %s in the current branch.", affected, code, state),
		auditPayload: auditPayload,
	}, nil
}

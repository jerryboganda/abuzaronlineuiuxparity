package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

func receivedQuantityAgainstPOLine(ctx context.Context, tx *sql.Tx, tenantID, sourceLineID, excludeDocumentID string) (pricing.Quantity, error) {
	var prior string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(l.quantity), 0)::text
		FROM business_document_lines l
		JOIN business_documents d
		  ON d.tenant_id = l.tenant_id AND d.id = l.document_id
		WHERE l.tenant_id = $1::uuid AND l.source_line_id = $2::uuid
		  AND d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
		  AND d.status IN ('draft', 'posted') AND d.deleted_at IS NULL
		    AND CASE WHEN BTRIM($3) = '' THEN TRUE ELSE d.id <> BTRIM($3)::uuid END
	`, tenantID, sourceLineID, strings.TrimSpace(excludeDocumentID)).Scan(&prior); err != nil {
		return 0, err
	}
	already, err := parseQuantity(prior)
	if err != nil {
		return 0, nil
	}
	return already, nil
}

func remainingQuantityForPOLine(ctx context.Context, tx *sql.Tx, tenantID, poLineID, orderedText, excludeDocumentID string) (string, error) {
	ordered, err := parseQuantity(orderedText)
	if err != nil {
		return "0", err
	}
	already, err := receivedQuantityAgainstPOLine(ctx, tx, tenantID, poLineID, excludeDocumentID)
	if err != nil {
		return "0", err
	}
	remaining := ordered - already
	if remaining < 0 {
		remaining = 0
	}
	return formatQuantity(remaining), nil
}

func remainingQuantityForSourceLine(ctx context.Context, tx *sql.Tx, tenantID, sourceLineID, excludeDocumentID string) (string, error) {
	var ordered string
	err := tx.QueryRowContext(ctx, `
		SELECT quantity::text
		FROM business_document_lines
		WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, sourceLineID).Scan(&ordered)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return remainingQuantityForPOLine(ctx, tx, tenantID, sourceLineID, ordered, excludeDocumentID)
}

func attachPurchaseRemainingQuantities(ctx context.Context, tx *sql.Tx, operator *sessionContext, document *documentResponse) error {
	if document == nil || operator == nil {
		return nil
	}
	excludeID := ""
	if isPurchaseReceiptKind(document.Kind) {
		excludeID = document.ID
	}
	for index := range document.Lines {
		if document.Kind == "purchase-order" {
			remaining, err := remainingQuantityForPOLine(ctx, tx, operator.TenantID, document.Lines[index].ID, document.Lines[index].Quantity, "")
			if err != nil {
				return err
			}
			document.Lines[index].RemainingQuantity = remaining
			continue
		}
		sourceLineID := strings.TrimSpace(document.Lines[index].SourceLineID)
		if !isPurchaseReceiptKind(document.Kind) || sourceLineID == "" {
			continue
		}
		remaining, err := remainingQuantityForSourceLine(ctx, tx, operator.TenantID, sourceLineID, excludeID)
		if err != nil {
			return err
		}
		document.Lines[index].RemainingQuantity = remaining
	}
	return nil
}

func validatePurchaseOrderSource(ctx context.Context, tx *sql.Tx, operator *sessionContext, currentDocumentID, supplierID, sourceDocumentID string, lines []documentLineRequest) error {
	var sourceKind, sourceStatus, sourceSupplier string
	err := tx.QueryRowContext(ctx, `
		SELECT kind, status, COALESCE(supplier_id::text, '')
		FROM business_documents
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $3::uuid AND deleted_at IS NULL
		FOR UPDATE
	`, operator.TenantID, operator.BranchID, sourceDocumentID).Scan(&sourceKind, &sourceStatus, &sourceSupplier)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("source purchase order was not found in the authenticated tenant and branch")
	}
	if err != nil {
		return err
	}
	if sourceKind != "purchase-order" {
		return errors.New("sourceDocumentId must reference a purchase order")
	}
	if sourceStatus != "posted" {
		return errors.New("source purchase order must be posted")
	}
	if sourceSupplier != strings.TrimSpace(supplierID) {
		return errors.New("purchase supplier does not match the source purchase order")
	}
	for index, line := range lines {
		sourceLineID := strings.TrimSpace(line.SourceLineID)
		if sourceLineID == "" {
			continue
		}
		if !documentUUIDPattern.MatchString(sourceLineID) {
			return fmt.Errorf("line %d sourceLineId must be a UUID", index+1)
		}
		var poItemID, poQuantity string
		err := tx.QueryRowContext(ctx, `
			SELECT item_id::text, quantity::text
			FROM business_document_lines
			WHERE tenant_id = $1::uuid AND document_id = $2::uuid AND id = $3::uuid
		`, operator.TenantID, sourceDocumentID, sourceLineID).Scan(&poItemID, &poQuantity)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("line %d sourceLineId is not a line on the source purchase order", index+1)
		}
		if err != nil {
			return err
		}
		if poItemID != strings.TrimSpace(line.ItemID) {
			return fmt.Errorf("line %d item does not match the source purchase-order line", index+1)
		}
		ordered, err := parseQuantity(poQuantity)
		if err != nil {
			return fmt.Errorf("line %d source purchase-order quantity is invalid", index+1)
		}
		requested, err := parseQuantity(line.Quantity)
		if err != nil {
			return fmt.Errorf("line %d quantity: %w", index+1, err)
		}
		already, err := receivedQuantityAgainstPOLine(ctx, tx, operator.TenantID, sourceLineID, currentDocumentID)
		if err != nil {
			return err
		}
		remaining := ordered - already
		if requested > remaining {
			return fmt.Errorf("line %d quantity exceeds remaining purchase-order quantity %s", index+1, formatQuantity(remaining))
		}
	}
	return nil
}

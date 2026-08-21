package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

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
		var prior string
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(l.quantity), 0)::text
			FROM business_document_lines l
			JOIN business_documents d
			  ON d.tenant_id = l.tenant_id AND d.id = l.document_id
			WHERE l.tenant_id = $1::uuid AND l.source_line_id = $2::uuid
			  AND d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
			  AND d.status IN ('draft', 'posted') AND d.deleted_at IS NULL
			  AND ($3 = '' OR d.id <> $3::uuid)
		`, operator.TenantID, sourceLineID, strings.TrimSpace(currentDocumentID)).Scan(&prior); err != nil {
			return err
		}
		already, err := parseQuantity(prior)
		if err != nil {
			already = 0
		}
		remaining := ordered - already
		if requested > remaining {
			return fmt.Errorf("line %d quantity exceeds remaining purchase-order quantity %s", index+1, formatQuantity(remaining))
		}
	}
	return nil
}

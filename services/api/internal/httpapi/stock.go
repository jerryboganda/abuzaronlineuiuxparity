package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

type stockBatchRow struct {
	ID          string
	BatchNumber string
	ExpiryDate  sql.NullString
	UnitCost    string
}

type stockAllocationChoice struct {
	batch    stockBatchRow
	quantity *big.Rat
}

func stockAllocationPolicy() (string, error) {
	policy := strings.ToLower(strings.TrimSpace(os.Getenv("ABUZAR_STOCK_ALLOCATION_POLICY")))
	if policy == "" {
		policy = "fifo"
	}
	switch policy {
	case "fifo":
		return policy, nil
	case "legacy":
		// Placeholder only: the legacy ordering has not yet been reconciled
		// against the source StockReport. It intentionally uses the same stable
		// FIFO ordering until that evidence exists, rather than silently
		// selecting an undocumented policy.
		return policy, nil
	default:
		return "", fmt.Errorf("unsupported stock allocation policy %q; configure fifo or legacy", policy)
	}
}

func projectPostedPurchaseStock(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest, document documentResponse, eventID string) error {
	if draft == nil {
		return errors.New("posted purchase requires a document payload")
	}
	if !isPurchasePostingKind(document.Kind) {
		return fmt.Errorf("stock projection is not implemented for %s", document.Kind)
	}
	if len(draft.Lines) != len(document.Lines) {
		return errors.New("document lines changed before purchase stock projection")
	}
	if strings.TrimSpace(draft.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(draft.GodownID)) {
		return errors.New("godownId is required when posting a purchase")
	}
	var godownExists bool
	if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM master_godowns
				WHERE tenant_id = $1::uuid AND id = $2::uuid AND active
			)
		`, operator.TenantID, draft.GodownID).Scan(&godownExists); err != nil {
		return err
	}
	if !godownExists {
		return errors.New("godownId is not an active canonical godown in the authenticated tenant")
	}
	replayed, err := assertPurchaseStockProjectionReplay(ctx, tx, operator, document.ID, eventID)
	if err != nil {
		return err
	}
	if replayed {
		return nil
	}
	if document.Kind == "purchase-return" {
		return projectPurchaseReturnStock(ctx, tx, operator, draft, document, eventID)
	}
	return projectPurchaseReceiptStock(ctx, tx, operator, draft, document, eventID)
}

func assertPurchaseStockProjectionReplay(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID, eventID string) (bool, error) {
	var count int
	var allSame bool
	if err := tx.QueryRowContext(ctx, `
			SELECT count(*), COALESCE(bool_and(source_event_id = $4::uuid), true)
			FROM stock_ledger
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND source_document_id = $3::uuid
		`, operator.TenantID, operator.BranchID, documentID, eventID).Scan(&count, &allSame); err != nil {
		return false, err
	}
	if count > 0 && allSame {
		return true, nil
	}
	if count > 0 {
		return false, fmt.Errorf("purchase stock projection already exists for document %s under another event", documentID)
	}
	return false, nil
}

func projectPurchaseReceiptStock(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest, document documentResponse, eventID string) error {
	occurredAt, err := time.Parse(time.RFC3339, draft.OccurredAt)
	if err != nil {
		return errors.New("document occurredAt must be RFC3339")
	}
	for index, line := range draft.Lines {
		quantity, err := parseStockQuantity(line.Quantity)
		if err != nil {
			return fmt.Errorf("line %d quantity: %w", index+1, err)
		}
		batchNumber := strings.TrimSpace(line.BatchNumber)
		if batchNumber == "" {
			return fmt.Errorf("line %d batchNumber is required when posting a purchase", index+1)
		}
		expiry := strings.TrimSpace(line.ExpiryDate)
		if expiry != "" {
			expiryDate, parseErr := time.Parse("2006-01-02", expiry)
			if parseErr != nil || expiryDate.Before(occurredAt.UTC().Truncate(24*time.Hour)) {
				return fmt.Errorf("line %d expiryDate is invalid or expired for the document date", index+1)
			}
		}
		cost, err := parseMoney(line.UnitCost)
		if err != nil || cost < 0 {
			return fmt.Errorf("line %d unitCost is required and must be a non-negative amount", index+1)
		}
		batchID, existingCost, err := lockOrCreatePurchaseBatch(ctx, tx, operator, document.Lines[index].ItemID,
			document.Lines[index].ItemLegacyID, draft.GodownID, batchNumber, expiry, formatMoney(cost), occurredAt)
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		if _, err := tx.ExecContext(ctx, `
				UPDATE stock_batches
				SET unit_cost = $4::numeric
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $3::uuid
			`, operator.TenantID, operator.BranchID, batchID, formatMoney(cost)); err != nil {
			return err
		}
		if existingCost != "" {
			// Existing movements retain their own immutable unit cost. The batch
			// cache reflects the latest explicit receipt cost for future sales.
		}
		qty := formatStockQuantity(quantity)
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO stock_balances
					(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, $7::numeric)
				ON CONFLICT (tenant_id, branch_id, batch_id) DO UPDATE
				SET on_hand = stock_balances.on_hand + EXCLUDED.on_hand, updated_at = now()
			`, operator.TenantID, operator.BranchID, batchID, document.Lines[index].ItemID,
			document.Lines[index].ItemLegacyID, draft.GodownID, qty); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO stock_ledger
					(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
					 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7,
					'in', 1, $8::numeric, $9::numeric, $10::timestamptz)
			`, operator.TenantID, operator.BranchID, batchID, eventID, document.ID, document.Lines[index].ID,
			fmt.Sprintf("purchase-line-%d", document.Lines[index].LineNumber), qty, formatMoney(cost), document.OccurredAt); err != nil {
			return err
		}
	}
	return nil
}

func lockOrCreatePurchaseBatch(ctx context.Context, tx *sql.Tx, operator *sessionContext, itemID, itemLegacyID,
	godownID, batchNumber, expiry, unitCost string, occurredAt time.Time) (string, string, error) {
	var batchID, existingCost string
	err := tx.QueryRowContext(ctx, `
			SELECT id::text, unit_cost::text
			FROM stock_batches
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid
			  AND godown_id = $4::uuid AND batch_number = $5
			  AND expiry_date IS NOT DISTINCT FROM NULLIF($6, '')::date
			  AND NOT locked
			  AND (expiry_date IS NULL OR expiry_date >= $7::date)
			FOR UPDATE
		`, operator.TenantID, operator.BranchID, itemID, godownID, batchNumber, expiry, occurredAt.Format("2006-01-02")).Scan(&batchID, &existingCost)
	if errors.Is(err, sql.ErrNoRows) {
		if _, insertErr := tx.ExecContext(ctx, `
				INSERT INTO stock_batches
					(tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost, received_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, NULLIF($7, '')::date, $8::numeric, $9::timestamptz)
			`, operator.TenantID, operator.BranchID, itemID, itemLegacyID, godownID, batchNumber, expiry, unitCost, occurredAt); insertErr != nil {
			return "", "", insertErr
		}
		if err := tx.QueryRowContext(ctx, `
				SELECT id::text, unit_cost::text
				FROM stock_batches
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid
				  AND godown_id = $4::uuid AND batch_number = $5
				  AND expiry_date IS NOT DISTINCT FROM NULLIF($6, '')::date
				FOR UPDATE
			`, operator.TenantID, operator.BranchID, itemID, godownID, batchNumber, expiry).Scan(&batchID, &existingCost); err != nil {
			return "", "", err
		}
		return batchID, existingCost, nil
	}
	if err != nil {
		return "", "", err
	}
	return batchID, existingCost, nil
}

func projectPurchaseReturnStock(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest, document documentResponse, eventID string) error {
	sourceID := strings.TrimSpace(draft.SourceDocumentID)
	if !documentUUIDPattern.MatchString(sourceID) {
		return errors.New("purchase-return requires a valid sourceDocumentId")
	}
	var sourceSupplier, sourceKind string
	var sourceStatus string
	if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(supplier_id::text, ''), kind, status
			FROM business_documents
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $3::uuid
		`, operator.TenantID, operator.BranchID, sourceID).Scan(&sourceSupplier, &sourceKind, &sourceStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("purchase-return source document was not found in the authenticated tenant and branch")
		}
		return err
	}
	if sourceStatus != "posted" || (sourceKind != "pack-purchase" && sourceKind != "loose-purchase" && sourceKind != "opening-purchase") {
		return errors.New("purchase-return source must be a posted pack, loose, or opening purchase")
	}
	if sourceSupplier != document.SupplierID {
		return errors.New("purchase-return supplier does not match the source purchase")
	}
	for index, line := range draft.Lines {
		quantity, err := parseStockQuantity(line.Quantity)
		if err != nil {
			return fmt.Errorf("line %d quantity: %w", index+1, err)
		}
		var sourceLineID, sourceQuantity string
		if err := tx.QueryRowContext(ctx, `
				SELECT id::text, quantity::text
				FROM business_document_lines
				WHERE tenant_id = $1::uuid AND document_id = $2::uuid AND item_id = $3::uuid
			`, operator.TenantID, sourceID, document.Lines[index].ItemID).Scan(&sourceLineID, &sourceQuantity); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("line %d item is not present on the source purchase", index+1)
			}
			return err
		}
		var priorReturns string
		if err := tx.QueryRowContext(ctx, `
				SELECT COALESCE(SUM(l.quantity), 0)::text
				FROM business_documents d
				JOIN business_document_lines l
				  ON l.tenant_id = d.tenant_id AND l.document_id = d.id
				WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
				  AND d.kind = 'purchase-return' AND d.status = 'posted'
				  AND d.source_document_id = $3::uuid AND l.item_id = $4::uuid
				  AND d.id <> $5::uuid
			`, operator.TenantID, operator.BranchID, sourceID, document.Lines[index].ItemID, document.ID).Scan(&priorReturns); err != nil {
			return err
		}
		sourceQty, ok := new(big.Rat).SetString(sourceQuantity)
		if !ok {
			return fmt.Errorf("line %d source quantity is invalid", index+1)
		}
		priorQty, ok := new(big.Rat).SetString(priorReturns)
		if !ok {
			return fmt.Errorf("line %d prior return quantity is invalid", index+1)
		}
		remaining := new(big.Rat).Sub(sourceQty, priorQty)
		if quantity.Cmp(remaining) > 0 {
			return fmt.Errorf("line %d exceeds the unreturned source quantity (requested %s, source %s, already returned %s)",
				index+1, formatStockQuantity(quantity), formatStockQuantity(remaining), formatStockQuantity(priorQty))
		}
		choices, err := resolveStockChoices(ctx, tx, operator, document.Lines[index].ItemID, draft.GodownID, line.Allocations, mustParseTime(document.OccurredAt))
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		allocated := new(big.Rat)
		for allocationIndex, choice := range choices {
			var sourceBatch bool
			if err := tx.QueryRowContext(ctx, `
					SELECT EXISTS (
						SELECT 1 FROM stock_ledger
						WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
						  AND source_document_id = $3::uuid AND batch_id = $4::uuid AND direction = 'in'
					)
				`, operator.TenantID, operator.BranchID, sourceID, choice.batch.ID).Scan(&sourceBatch); err != nil {
				return err
			}
			if !sourceBatch {
				return fmt.Errorf("line %d allocation %d is not a batch received by the source purchase", index+1, allocationIndex+1)
			}
			available, err := lockStockBalance(ctx, tx, operator, choice.batch.ID)
			if err != nil {
				return err
			}
			if available.Cmp(choice.quantity) < 0 {
				return fmt.Errorf("line %d allocation %d exceeds available stock", index+1, allocationIndex+1)
			}
			qty := formatStockQuantity(choice.quantity)
			if _, err := tx.ExecContext(ctx, `
					UPDATE stock_balances SET on_hand = on_hand - $4::numeric, updated_at = now()
					WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
				`, operator.TenantID, operator.BranchID, choice.batch.ID, qty); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
					INSERT INTO stock_ledger
						(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
						 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
					VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7,
						'out', 1, $8::numeric, $9::numeric, $10::timestamptz)
				`, operator.TenantID, operator.BranchID, choice.batch.ID, eventID, document.ID, document.Lines[index].ID,
				fmt.Sprintf("purchase-return-line-%d-%d", document.Lines[index].LineNumber, allocationIndex),
				qty, choice.batch.UnitCost, document.OccurredAt); err != nil {
				return err
			}
			allocated.Add(allocated, choice.quantity)
		}
		if allocated.Cmp(quantity) != 0 {
			return fmt.Errorf("line %d batch allocations total %s but line quantity is %s", index+1, formatStockQuantity(allocated), formatStockQuantity(quantity))
		}
		_ = sourceLineID
	}
	return nil
}

// projectPostedSaleReturnStock restores the exact batches allocated by a
// posted cash/credit sale. Closed returns are source-bound and cannot exceed
// the unreturned quantity of their source line. The movement and allocation
// rows are written in the same transaction as the document command.
func projectPostedSaleReturnStock(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest, document documentResponse, eventID string) error {
	if draft == nil {
		return errors.New("posted sale return requires a document payload")
	}
	if !isSaleReturnDocumentKind(document.Kind) {
		return fmt.Errorf("stock restoration is not implemented for %s", document.Kind)
	}
	if strings.TrimSpace(draft.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(draft.GodownID)) {
		return errors.New("godownId is required when posting a sale return")
	}
	sourceID := strings.TrimSpace(draft.SourceDocumentID)
	if !documentUUIDPattern.MatchString(sourceID) {
		return errors.New("sale-return requires a valid sourceDocumentId")
	}
	var sourceCustomer, sourceKind, sourceStatus string
	if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(customer_id::text, ''), kind, status
			FROM business_documents
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $3::uuid
		`, operator.TenantID, operator.BranchID, sourceID).Scan(&sourceCustomer, &sourceKind, &sourceStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("sale-return source document was not found in the authenticated tenant and branch")
		}
		return err
	}
	if sourceStatus != "posted" || !isStockAndFinanceSaleKind(sourceKind) {
		return errors.New("sale-return source must be a posted cash or credit sale")
	}
	if document.Kind == "credit-return" && strings.TrimSpace(document.CustomerID) != sourceCustomer {
		return errors.New("credit-return customer does not match the source sale")
	}
	var alreadyProjected bool
	if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM stock_ledger
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
				  AND source_document_id = $3::uuid AND direction = 'in'
			)
		`, operator.TenantID, operator.BranchID, document.ID).Scan(&alreadyProjected); err != nil {
		return err
	}
	if alreadyProjected {
		return nil
	}
	if len(draft.Lines) != len(document.Lines) {
		return errors.New("document lines changed before sale-return stock restoration")
	}
	occurredAt, err := time.Parse(time.RFC3339, draft.OccurredAt)
	if err != nil {
		return errors.New("document occurredAt must be RFC3339")
	}
	for index, line := range draft.Lines {
		quantity, err := parseStockQuantity(line.Quantity)
		if err != nil {
			return fmt.Errorf("line %d quantity: %w", index+1, err)
		}
		sourceLineID := strings.TrimSpace(line.SourceLineID)
		if !documentUUIDPattern.MatchString(sourceLineID) {
			return fmt.Errorf("line %d requires a valid sourceLineId", index+1)
		}
		var sourceQuantity, sourceItemID string
		if err := tx.QueryRowContext(ctx, `
				SELECT quantity::text, item_id::text
				FROM business_document_lines
				WHERE tenant_id = $1::uuid AND document_id = $2::uuid AND id = $3::uuid
				FOR UPDATE
			`, operator.TenantID, sourceID, sourceLineID).Scan(&sourceQuantity, &sourceItemID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("line %d source sale line was not found", index+1)
			}
			return err
		}
		if sourceItemID != document.Lines[index].ItemID {
			return fmt.Errorf("line %d source sale line item does not match the return item", index+1)
		}
		var priorReturns string
		if err := tx.QueryRowContext(ctx, `
				SELECT COALESCE(SUM(l.quantity), 0)::text
				FROM business_documents d
				JOIN business_document_lines l
				  ON l.tenant_id = d.tenant_id AND l.document_id = d.id
				WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
				  AND d.kind IN ('cash-return', 'credit-return') AND d.status = 'posted'
				  AND d.source_document_id = $3::uuid AND l.source_line_id = $4::uuid
				  AND d.id <> $5::uuid
			`, operator.TenantID, operator.BranchID, sourceID, sourceLineID, document.ID).Scan(&priorReturns); err != nil {
			return err
		}
		sourceQty, ok := new(big.Rat).SetString(sourceQuantity)
		if !ok {
			return fmt.Errorf("line %d source quantity is invalid", index+1)
		}
		priorQty, ok := new(big.Rat).SetString(priorReturns)
		if !ok {
			return fmt.Errorf("line %d prior return quantity is invalid", index+1)
		}
		remaining := new(big.Rat).Sub(sourceQty, priorQty)
		if quantity.Cmp(remaining) > 0 {
			return fmt.Errorf("line %d exceeds the unreturned source quantity (requested %s, source %s, already returned %s)", index+1, formatStockQuantity(quantity), formatStockQuantity(remaining), formatStockQuantity(priorQty))
		}
		choices, err := saleReturnStockChoices(ctx, tx, operator, sourceID, sourceLineID, document.Lines[index].ItemID, draft.GodownID, line.Allocations, quantity, occurredAt)
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		allocated := new(big.Rat)
		for allocationIndex, choice := range choices {
			qty := formatStockQuantity(choice.quantity)
			result, err := tx.ExecContext(ctx, `
					UPDATE stock_balances SET on_hand = on_hand + $4::numeric, updated_at = now()
					WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
				`, operator.TenantID, operator.BranchID, choice.batch.ID, qty)
			if err != nil {
				return err
			}
			if affected, affectedErr := result.RowsAffected(); affectedErr != nil {
				return affectedErr
			} else if affected != 1 {
				return fmt.Errorf("return batch %s has no stock balance", choice.batch.BatchNumber)
			}
			var movementID string
			if err := tx.QueryRowContext(ctx, `
					INSERT INTO stock_ledger
						(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
						 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
					VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7,
						'in', 1, $8::numeric, $9::numeric, $10::timestamptz)
					RETURNING id::text
				`, operator.TenantID, operator.BranchID, choice.batch.ID, eventID, document.ID, document.Lines[index].ID,
				fmt.Sprintf("sale-return-line-%d-%d", document.Lines[index].LineNumber, allocationIndex), qty, choice.batch.UnitCost, document.OccurredAt).Scan(&movementID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
					INSERT INTO stock_allocations
						(tenant_id, branch_id, document_id, document_line_id, batch_id, movement_id, quantity, unit_cost)
					VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7::numeric, $8::numeric)
				`, operator.TenantID, operator.BranchID, document.ID, document.Lines[index].ID, choice.batch.ID, movementID, qty, choice.batch.UnitCost); err != nil {
				return err
			}
			allocated.Add(allocated, choice.quantity)
		}
		if allocated.Cmp(quantity) != 0 {
			return fmt.Errorf("line %d batch allocations total %s but line quantity is %s", index+1, formatStockQuantity(allocated), formatStockQuantity(quantity))
		}
	}
	return nil
}

// projectPostedOpenSaleReturnStock handles the legacy Open Sale Return leaves.
// Unlike a source-bound return, an open return has no originating invoice. The
// operator supplies the godown and optional batch; an omitted batch is given a
// deterministic document-scoped batch so the stock and GL projections remain
// auditable and idempotent.
func projectPostedOpenSaleReturnStock(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest, document documentResponse, eventID string) error {
	if draft == nil || !isOpenSaleReturnDocumentKind(document.Kind) {
		return errors.New("posted open sale return requires a document payload")
	}
	if strings.TrimSpace(draft.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(draft.GodownID)) {
		return errors.New("godownId is required when posting an open sale return")
	}
	var godownExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM master_godowns WHERE tenant_id = $1::uuid AND id = $2::uuid AND active)
	`, operator.TenantID, draft.GodownID).Scan(&godownExists); err != nil {
		return err
	}
	if !godownExists {
		return errors.New("godownId is not an active canonical godown in the authenticated tenant")
	}
	var alreadyProjected bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM stock_ledger WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			AND source_document_id = $3::uuid AND direction = 'in')
	`, operator.TenantID, operator.BranchID, document.ID).Scan(&alreadyProjected); err != nil {
		return err
	}
	if alreadyProjected {
		return nil
	}
	if len(draft.Lines) != len(document.Lines) {
		return errors.New("document lines changed before open sale-return stock projection")
	}
	occurredAt, err := time.Parse(time.RFC3339, draft.OccurredAt)
	if err != nil {
		return errors.New("document occurredAt must be RFC3339")
	}
	for index, line := range draft.Lines {
		quantity, err := parseStockQuantity(line.Quantity)
		if err != nil {
			return fmt.Errorf("line %d quantity: %w", index+1, err)
		}
		batchNumber := strings.TrimSpace(line.BatchNumber)
		if batchNumber == "" {
			batchNumber = fmt.Sprintf("OPEN-RETURN-%s-%03d", document.DocumentNumber, document.Lines[index].LineNumber)
		}
		expiry := strings.TrimSpace(line.ExpiryDate)
		if expiry != "" {
			expiryDate, parseErr := time.Parse("2006-01-02", expiry)
			if parseErr != nil || expiryDate.Before(occurredAt.UTC().Truncate(24*time.Hour)) {
				return fmt.Errorf("line %d expiryDate is invalid or expired for the document date", index+1)
			}
		}
		unitCost := strings.TrimSpace(line.UnitCost)
		if unitCost == "" {
			unitCost = strings.TrimSpace(line.UnitPrice)
		}
		cost, err := parseMoney(unitCost)
		if err != nil || cost < 0 {
			return fmt.Errorf("line %d unit cost is required and must be non-negative", index+1)
		}
		if err := assertOpenReturnBatchIdentity(ctx, tx, operator, document.Lines[index].ItemID,
			draft.GodownID, batchNumber, expiry); err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		batchID, _, err := lockOrCreatePurchaseBatch(ctx, tx, operator, document.Lines[index].ItemID,
			document.Lines[index].ItemLegacyID, draft.GodownID, batchNumber, expiry, formatMoney(cost), occurredAt)
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		qty := formatStockQuantity(quantity)
		if _, err := tx.ExecContext(ctx, `
			UPDATE stock_batches SET unit_cost = $4::numeric
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $3::uuid
		`, operator.TenantID, operator.BranchID, batchID, formatMoney(cost)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_balances (tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, $7::numeric)
			ON CONFLICT (tenant_id, branch_id, batch_id) DO UPDATE
			SET on_hand = stock_balances.on_hand + EXCLUDED.on_hand, updated_at = now()
		`, operator.TenantID, operator.BranchID, batchID, document.Lines[index].ItemID,
			document.Lines[index].ItemLegacyID, draft.GodownID, qty); err != nil {
			return err
		}
		var movementID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO stock_ledger
				(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
				 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7,
				'in', 1, $8::numeric, $9::numeric, $10::timestamptz)
			RETURNING id::text
		`, operator.TenantID, operator.BranchID, batchID, eventID, document.ID, document.Lines[index].ID,
			fmt.Sprintf("open-sale-return-line-%d", document.Lines[index].LineNumber), qty, formatMoney(cost), document.OccurredAt).Scan(&movementID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_allocations
				(tenant_id, branch_id, document_id, document_line_id, batch_id, movement_id, quantity, unit_cost)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7::numeric, $8::numeric)
		`, operator.TenantID, operator.BranchID, document.ID, document.Lines[index].ID, batchID, movementID, qty, formatMoney(cost)); err != nil {
			return err
		}
	}
	return nil
}

func assertOpenReturnBatchIdentity(ctx context.Context, tx *sql.Tx, operator *sessionContext,
	itemID, godownID, batchNumber, expiry string) error {
	var count int
	if err := tx.QueryRowContext(ctx, `
			SELECT count(*)
			FROM stock_batches
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND item_id = $3::uuid AND godown_id = $4::uuid
			  AND batch_number = $5
		`, operator.TenantID, operator.BranchID, itemID, godownID, batchNumber).Scan(&count); err != nil {
		return err
	}
	if count > 1 && expiry == "" {
		return errors.New("batch identity is ambiguous; provide expiryDate")
	}
	if count == 1 && expiry == "" {
		var existingExpiry sql.NullString
		if err := tx.QueryRowContext(ctx, `
				SELECT expiry_date::text
				FROM stock_batches
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
				  AND item_id = $3::uuid AND godown_id = $4::uuid
				  AND batch_number = $5
			`, operator.TenantID, operator.BranchID, itemID, godownID, batchNumber).Scan(&existingExpiry); err != nil {
			return err
		}
		if existingExpiry.Valid && existingExpiry.String != "" {
			return errors.New("batch identity requires expiryDate")
		}
	}
	return nil
}

func saleReturnStockChoices(ctx context.Context, tx *sql.Tx, operator *sessionContext, sourceDocumentID, sourceLineID, itemID, godownID string, allocations []documentAllocationRequest, requested *big.Rat, occurredAt time.Time) ([]stockAllocationChoice, error) {
	if len(allocations) > 0 {
		choices, err := resolveStockChoices(ctx, tx, operator, itemID, godownID, allocations, occurredAt)
		if err != nil {
			return nil, err
		}
		allocated := new(big.Rat)
		for _, choice := range choices {
			var sourceAllocation bool
			if err := tx.QueryRowContext(ctx, `
					SELECT EXISTS (
						SELECT 1 FROM stock_allocations
						WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
						  AND document_id = $3::uuid AND document_line_id = $4::uuid AND batch_id = $5::uuid
					)
				`, operator.TenantID, operator.BranchID, sourceDocumentID, sourceLineID, choice.batch.ID).Scan(&sourceAllocation); err != nil {
				return nil, err
			}
			if !sourceAllocation {
				return nil, errors.New("allocation batch was not used by the source sale")
			}
			var remainingText string
			if err := tx.QueryRowContext(ctx, `
					SELECT (a.quantity - COALESCE(SUM(ra.quantity), 0))::text
					FROM stock_allocations a
					LEFT JOIN business_document_lines rl
					  ON rl.tenant_id = a.tenant_id AND rl.source_line_id = a.document_line_id
					LEFT JOIN business_documents rd
					  ON rd.tenant_id = rl.tenant_id AND rd.id = rl.document_id
					 AND rd.branch_id = a.branch_id AND rd.source_document_id = $3::uuid
					 AND rd.kind IN ('cash-return', 'credit-return') AND rd.status = 'posted'
					LEFT JOIN stock_allocations ra
					  ON ra.tenant_id = rd.tenant_id AND ra.branch_id = rd.branch_id
					 AND ra.document_line_id = rl.id AND ra.batch_id = a.batch_id
					WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid
					  AND a.document_id = $3::uuid AND a.document_line_id = $4::uuid
					  AND a.batch_id = $5::uuid
					GROUP BY a.quantity
				`, operator.TenantID, operator.BranchID, sourceDocumentID, sourceLineID, choice.batch.ID).Scan(&remainingText); err != nil {
				return nil, err
			}
			remaining, ok := new(big.Rat).SetString(remainingText)
			if !ok || remaining.Sign() <= 0 {
				return nil, errors.New("source batch has already been fully returned")
			}
			if choice.quantity.Cmp(remaining) > 0 {
				return nil, fmt.Errorf("batch %s exceeds its unreturned source allocation", choice.batch.BatchNumber)
			}
			allocated.Add(allocated, choice.quantity)
		}
		if allocated.Cmp(requested) != 0 {
			return nil, fmt.Errorf("batch allocations total %s but line quantity is %s", formatStockQuantity(allocated), formatStockQuantity(requested))
		}
		return choices, nil
	}
	rows, err := tx.QueryContext(ctx, `
			SELECT b.id::text, b.batch_number, COALESCE(b.expiry_date::text, ''), b.unit_cost::text,
			       (a.quantity - COALESCE(SUM(ra.quantity), 0))::text
			FROM stock_allocations a
			JOIN stock_batches b
			  ON b.tenant_id = a.tenant_id AND b.branch_id = a.branch_id AND b.id = a.batch_id
			LEFT JOIN business_document_lines rl
			  ON rl.tenant_id = a.tenant_id AND rl.source_line_id = a.document_line_id
			LEFT JOIN business_documents rd
			  ON rd.tenant_id = rl.tenant_id AND rd.id = rl.document_id
			 AND rd.branch_id = a.branch_id AND rd.source_document_id = $3::uuid
			 AND rd.kind IN ('cash-return', 'credit-return') AND rd.status = 'posted'
			LEFT JOIN stock_allocations ra
			  ON ra.tenant_id = rd.tenant_id AND ra.branch_id = rd.branch_id
			 AND ra.document_line_id = rl.id AND ra.batch_id = a.batch_id
			WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid
			  AND a.document_id = $3::uuid AND a.document_line_id = $4::uuid AND b.item_id = $5::uuid AND b.godown_id = $6::uuid
			  AND NOT b.locked AND (b.expiry_date IS NULL OR b.expiry_date >= $7::date)
			GROUP BY a.id, a.quantity, b.id, b.batch_number, b.expiry_date, b.unit_cost
			HAVING a.quantity - COALESCE(SUM(ra.quantity), 0) > 0
			ORDER BY COALESCE(b.expiry_date, DATE '9999-12-31'), b.batch_number, b.id
		`, operator.TenantID, operator.BranchID, sourceDocumentID, sourceLineID, itemID, godownID, occurredAt.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	remaining := new(big.Rat).Set(requested)
	result := make([]stockAllocationChoice, 0)
	for rows.Next() {
		var id, number, unitCost, sourceAllocated string
		var expiry sql.NullString
		if err := rows.Scan(&id, &number, &expiry, &unitCost, &sourceAllocated); err != nil {
			return nil, err
		}
		available, ok := new(big.Rat).SetString(sourceAllocated)
		if !ok || available.Sign() <= 0 {
			continue
		}
		take := new(big.Rat).Set(available)
		if take.Cmp(remaining) > 0 {
			take.Set(remaining)
		}
		result = append(result, stockAllocationChoice{batch: stockBatchRow{ID: id, BatchNumber: number, ExpiryDate: expiry, UnitCost: unitCost}, quantity: take})
		remaining.Sub(remaining, take)
		if remaining.Sign() == 0 {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if remaining.Sign() != 0 {
		return nil, errors.New("source sale has no matching batch allocation for the requested return")
	}
	return result, nil
}

func mustParseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}

func projectPostedSaleStock(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest, document documentResponse, eventID string) error {
	if draft == nil {
		return errors.New("posted sale requires a document payload")
	}
	if document.Kind != "cash-sale" && document.Kind != "credit-sale" {
		return fmt.Errorf("stock allocation is not implemented for %s", document.Kind)
	}
	if strings.TrimSpace(draft.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(draft.GodownID)) {
		return errors.New("godownId is required when posting a sale; stock cannot be selected implicitly")
	}
	policy, err := stockAllocationPolicy()
	if err != nil {
		return err
	}
	var alreadyAllocated bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM stock_allocations
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND document_id = $3::uuid
		)
	`, operator.TenantID, operator.BranchID, document.ID).Scan(&alreadyAllocated); err != nil {
		return err
	}
	if alreadyAllocated {
		// The command receipt normally prevents this path. Keeping the
		// projection itself idempotent also makes a replayed post safe during
		// sync/rebuild recovery.
		return nil
	}
	if len(draft.Lines) != len(document.Lines) {
		return errors.New("document lines changed before stock allocation")
	}
	occurredAt, err := time.Parse(time.RFC3339, draft.OccurredAt)
	if err != nil {
		return errors.New("document occurredAt must be RFC3339")
	}
	var godownExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM master_godowns
			WHERE tenant_id = $1::uuid AND id = $2::uuid AND active
		)
	`, operator.TenantID, draft.GodownID).Scan(&godownExists); err != nil {
		return err
	}
	if !godownExists {
		return errors.New("godownId is not an active canonical godown in the authenticated tenant")
	}

	for index, line := range draft.Lines {
		quantity, err := parseStockQuantity(line.Quantity)
		if err != nil {
			return fmt.Errorf("line %d quantity: %w", index+1, err)
		}
		choices, err := resolveStockChoices(ctx, tx, operator, document.Lines[index].ItemID, draft.GodownID, line.Allocations, occurredAt)
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		if len(choices) == 0 {
			choices, err = fifoStockChoices(ctx, tx, operator, document.Lines[index].ItemID, draft.GodownID, occurredAt, policy)
			if err != nil {
				return fmt.Errorf("line %d: %w", index+1, err)
			}
		} else {
			var selected *big.Rat
			for _, choice := range choices {
				if selected == nil {
					selected = new(big.Rat)
				}
				selected.Add(selected, choice.quantity)
			}
			if selected == nil || selected.Cmp(quantity) != 0 {
				return fmt.Errorf("explicit batch allocations total %s but line quantity is %s", formatStockQuantity(selected), formatStockQuantity(quantity))
			}
		}
		if err := allocateStockChoices(ctx, tx, operator, document, document.Lines[index], eventID, choices, quantity); err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
	}
	return nil
}

func resolveStockChoices(ctx context.Context, tx *sql.Tx, operator *sessionContext, itemID, godownID string, requests []documentAllocationRequest, occurredAt time.Time) ([]stockAllocationChoice, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	result := make([]stockAllocationChoice, 0, len(requests))
	seen := make(map[string]struct{}, len(requests))
	for index, request := range requests {
		if strings.TrimSpace(request.BatchID) == "" && strings.TrimSpace(request.BatchNumber) == "" {
			return nil, fmt.Errorf("allocation %d must identify a batchId or batchNumber", index+1)
		}
		quantity, err := parseStockQuantity(request.Quantity)
		if err != nil {
			return nil, fmt.Errorf("allocation %d quantity: %w", index+1, err)
		}
		batch, err := findStockBatch(ctx, tx, operator, itemID, godownID, request, occurredAt)
		if err != nil {
			return nil, fmt.Errorf("allocation %d: %w", index+1, err)
		}
		if _, ok := seen[batch.ID]; ok {
			return nil, fmt.Errorf("batch %s was selected more than once", batch.BatchNumber)
		}
		seen[batch.ID] = struct{}{}
		result = append(result, stockAllocationChoice{batch: batch, quantity: quantity})
	}
	return result, nil
}

func findStockBatch(ctx context.Context, tx *sql.Tx, operator *sessionContext, itemID, godownID string, request documentAllocationRequest, occurredAt time.Time) (stockBatchRow, error) {
	if request.BatchID != "" && !documentUUIDPattern.MatchString(strings.TrimSpace(request.BatchID)) {
		return stockBatchRow{}, errors.New("batchId must be a UUID")
	}
	query := `
		SELECT id::text, batch_number, COALESCE(expiry_date::text, ''), unit_cost::text
		FROM stock_batches
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid
		  AND godown_id = $4::uuid AND NOT locked
		  AND (expiry_date IS NULL OR expiry_date >= $5::date)`
	args := []any{operator.TenantID, operator.BranchID, itemID, godownID, occurredAt.Format("2006-01-02")}
	if request.BatchID != "" {
		query += ` AND id = $6::uuid`
		args = append(args, request.BatchID)
	}
	if request.BatchNumber != "" {
		query += ` AND batch_number = $` + fmt.Sprint(len(args)+1)
		args = append(args, strings.TrimSpace(request.BatchNumber))
	}
	query += ` FOR UPDATE`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return stockBatchRow{}, err
	}
	defer rows.Close()
	var result stockBatchRow
	count := 0
	for rows.Next() {
		if err := rows.Scan(&result.ID, &result.BatchNumber, &result.ExpiryDate, &result.UnitCost); err != nil {
			return stockBatchRow{}, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return stockBatchRow{}, err
	}
	if count == 0 {
		return stockBatchRow{}, errors.New("batch was not found, is locked, or is expired")
	}
	if count > 1 {
		return stockBatchRow{}, errors.New("batch identity is ambiguous; provide batchId")
	}
	return result, nil
}

func fifoStockChoices(ctx context.Context, tx *sql.Tx, operator *sessionContext, itemID, godownID string, occurredAt time.Time, policy string) ([]stockAllocationChoice, error) {
	if policy != "fifo" && policy != "legacy" {
		return nil, fmt.Errorf("stock allocation policy %q is not supported", policy)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT b.id::text, b.batch_number, COALESCE(b.expiry_date::text, ''), b.unit_cost::text
		FROM stock_batches b
		JOIN stock_balances sb
		  ON sb.tenant_id = b.tenant_id AND sb.branch_id = b.branch_id AND sb.batch_id = b.id
		WHERE b.tenant_id = $1::uuid AND b.branch_id = $2::uuid AND b.item_id = $3::uuid
		  AND b.godown_id = $4::uuid AND NOT b.locked
		  AND (b.expiry_date IS NULL OR b.expiry_date >= $5::date)
		  AND sb.on_hand > 0
		ORDER BY b.received_at, b.id
		FOR UPDATE OF b
	`, operator.TenantID, operator.BranchID, itemID, godownID, occurredAt.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	batches := make([]stockBatchRow, 0)
	for rows.Next() {
		var batch stockBatchRow
		if err := rows.Scan(&batch.ID, &batch.BatchNumber, &batch.ExpiryDate, &batch.UnitCost); err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]stockAllocationChoice, 0, len(batches))
	for _, batch := range batches {
		balance, err := lockStockBalance(ctx, tx, operator, batch.ID)
		if err != nil {
			return nil, err
		}
		if balance.Sign() > 0 {
			result = append(result, stockAllocationChoice{batch: batch, quantity: new(big.Rat).Set(balance)})
		}
	}
	if len(result) == 0 {
		return nil, errors.New("insufficient stock; expired and locked batches are not available")
	}
	return result, nil
}

func allocateStockChoices(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, line documentLineResponse, eventID string, choices []stockAllocationChoice, required *big.Rat) error {
	remaining := new(big.Rat).Set(required)
	for index, choice := range choices {
		available, err := lockStockBalance(ctx, tx, operator, choice.batch.ID)
		if err != nil {
			return err
		}
		take := new(big.Rat).Set(choice.quantity)
		if take.Cmp(remaining) > 0 {
			take.Set(remaining)
		}
		if take.Sign() <= 0 || available.Cmp(take) < 0 {
			return errors.New("insufficient stock after locking the selected batch")
		}
		qty := formatStockQuantity(take)
		if _, err := tx.ExecContext(ctx, `
			UPDATE stock_balances
			SET on_hand = on_hand - $4::numeric, updated_at = now()
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
			  AND on_hand >= $4::numeric
		`, operator.TenantID, operator.BranchID, choice.batch.ID, qty); err != nil {
			return err
		}
		var movementID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO stock_ledger
				(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
				 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7,
				'out', 1, $8::numeric, $9::numeric, $10::timestamptz)
			RETURNING id::text
		`, operator.TenantID, operator.BranchID, choice.batch.ID, eventID, document.ID, line.ID,
			fmt.Sprintf("sale-line-%d-%d", line.LineNumber, index), qty, choice.batch.UnitCost, document.OccurredAt).Scan(&movementID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_allocations
				(tenant_id, branch_id, document_id, document_line_id, batch_id, movement_id, quantity, unit_cost)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7::numeric, $8::numeric)
		`, operator.TenantID, operator.BranchID, document.ID, line.ID, choice.batch.ID, movementID, qty, choice.batch.UnitCost); err != nil {
			return err
		}
		remaining.Sub(remaining, take)
		if remaining.Sign() == 0 {
			return nil
		}
	}
	return errors.New("insufficient stock; the available batches could not satisfy the sale")
}

func lockStockBalance(ctx context.Context, tx *sql.Tx, operator *sessionContext, batchID string) (*big.Rat, error) {
	var value string
	if err := tx.QueryRowContext(ctx, `
		SELECT on_hand::text FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
		FOR UPDATE
	`, operator.TenantID, operator.BranchID, batchID).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return new(big.Rat), nil
		}
		return nil, err
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("stock balance %q is not a decimal", value)
	}
	return rat, nil
}

func parseStockQuantity(value string) (*big.Rat, error) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() <= 0 {
		return nil, errors.New("quantity must be a positive decimal")
	}
	scaled := new(big.Rat).Mul(rat, big.NewRat(10000, 1))
	if !scaled.IsInt() {
		return nil, errors.New("quantity must have at most four decimal places")
	}
	return rat, nil
}

func formatStockQuantity(value *big.Rat) string {
	if value == nil {
		return "0.0000"
	}
	text := value.FloatString(4)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func stockCommandErrorStatus(err error) (int, string) {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "insufficient"),
		strings.Contains(message, "expired"),
		strings.Contains(message, "locked"),
		strings.Contains(message, "ambiguous"),
		strings.Contains(message, "selected more than once"):
		return http.StatusConflict, "stock_rejected"
	default:
		return http.StatusUnprocessableEntity, "stock_invalid"
	}
}

func (s *Server) stockAvailability(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.read") {
		return
	}
	item := strings.TrimSpace(r.URL.Query().Get("itemLegacyId"))
	godown := strings.TrimSpace(r.URL.Query().Get("godownId"))
	if item == "" || godown == "" || !documentUUIDPattern.MatchString(godown) {
		writeProblem(w, http.StatusBadRequest, "stock_scope_required", "Stock scope required", "Provide itemLegacyId and godownId; stock is never selected across godowns implicitly.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The stock store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `
		SELECT b.id::text, b.batch_number, COALESCE(b.expiry_date::text, ''), b.locked,
		       sb.on_hand::text, b.unit_cost::text
		FROM stock_batches b
		JOIN stock_balances sb ON sb.tenant_id = b.tenant_id AND sb.branch_id = b.branch_id AND sb.batch_id = b.id
		WHERE b.tenant_id = $1::uuid AND b.branch_id = $2::uuid AND b.item_legacy_id = $3
		  AND b.godown_id = $4::uuid AND NOT b.locked
		  AND (b.expiry_date IS NULL OR b.expiry_date >= CURRENT_DATE) AND sb.on_hand > 0
		ORDER BY b.received_at, b.id
	`, operator.TenantID, operator.BranchID, item, godown)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_read_failed", "Unable to read stock", "The stock availability query failed.")
		return
	}
	type availableBatch struct {
		ID          string `json:"batchId"`
		BatchNumber string `json:"batchNumber"`
		ExpiryDate  string `json:"expiryDate,omitempty"`
		Quantity    string `json:"quantity"`
		UnitCost    string `json:"unitCost"`
	}
	batches := make([]availableBatch, 0)
	for rows.Next() {
		var batch availableBatch
		var locked bool
		if err := rows.Scan(&batch.ID, &batch.BatchNumber, &batch.ExpiryDate, &locked, &batch.Quantity, &batch.UnitCost); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "stock_read_failed", "Unable to read stock", "The stock availability response could not be decoded.")
			return
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "stock_read_failed", "Unable to read stock", "The stock availability response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_read_failed", "Unable to read stock", "The stock availability transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenantId": operator.TenantID, "branchId": operator.BranchID, "itemLegacyId": item, "godownId": godown, "batches": batches})
}

func (s *Server) rebuildStockBalances(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "maintenance.write") {
		return
	}
	policy, err := stockAllocationPolicy()
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "stock_policy_invalid", "Stock policy invalid", err.Error())
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The stock rebuild could not be opened.")
		return
	}
	defer tx.Rollback()
	var rebuildID string
	if err := tx.QueryRowContext(r.Context(), `
		INSERT INTO stock_balance_rebuilds (tenant_id, branch_id, status, allocation_policy)
		VALUES ($1::uuid, $2::uuid, 'running', $3)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, policy).Scan(&rebuildID); err != nil {
		writeProblem(w, http.StatusConflict, "stock_rebuild_conflict", "Stock rebuild unavailable", "A stock rebuild could not be started.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`, operator.TenantID, operator.BranchID); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_rebuild_failed", "Stock rebuild failed", "The balance cache could not be cleared.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO stock_balances
			(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
		SELECT b.tenant_id, b.branch_id, b.id, b.item_id, b.item_legacy_id, b.godown_id,
		       COALESCE(SUM(CASE
		           WHEN l.direction = 'in' THEN l.quantity
		           WHEN l.direction = 'out' THEN -l.quantity
		           ELSE l.quantity * l.adjustment_sign
		       END), 0)
		FROM stock_batches b
		LEFT JOIN stock_ledger l
		  ON l.tenant_id = b.tenant_id AND l.branch_id = b.branch_id AND l.batch_id = b.id
		WHERE b.tenant_id = $1::uuid AND b.branch_id = $2::uuid
		GROUP BY b.tenant_id, b.branch_id, b.id, b.item_id, b.item_legacy_id, b.godown_id
	`, operator.TenantID, operator.BranchID); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_rebuild_failed", "Stock rebuild failed", "The balance cache could not be rebuilt.")
		return
	}
	var batchCount, balanceCount int64
	if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM stock_batches WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`, operator.TenantID, operator.BranchID).Scan(&batchCount); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_rebuild_failed", "Stock rebuild failed", "The rebuilt batch count could not be read.")
		return
	}
	if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`, operator.TenantID, operator.BranchID).Scan(&balanceCount); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_rebuild_failed", "Stock rebuild failed", "The rebuilt balance count could not be read.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE stock_balance_rebuilds
		SET status = 'completed', finished_at = now(), batch_count = $3, balance_count = $4
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $5::uuid
	`, operator.TenantID, operator.BranchID, batchCount, balanceCount, rebuildID); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_rebuild_failed", "Stock rebuild failed", "The rebuild metadata could not be completed.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "stock_rebuild_failed", "Stock rebuild failed", "The stock rebuild transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": rebuildID, "status": "completed", "allocationPolicy": policy, "batchCount": batchCount, "balanceCount": balanceCount})
}

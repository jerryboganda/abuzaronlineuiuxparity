package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type godownTransferRequest struct {
	SourceGodownID      string               `json:"sourceGodownId"`
	DestinationGodownID string               `json:"destinationGodownId"`
	OccurredAt          string               `json:"occurredAt"`
	IdempotencyKey      string               `json:"idempotencyKey"`
	Lines               []godownTransferLine `json:"lines"`
}

type godownTransferLine struct {
	ItemID      string `json:"itemId"`
	Quantity    string `json:"quantity"`
	BatchNumber string `json:"batchNumber,omitempty"`
}

func parseGodownTransferRequest(payload map[string]any) (godownTransferRequest, error) {
	var request godownTransferRequest
	encoded, err := json.Marshal(payload)
	if err != nil {
		return request, errors.New("godown transfer payload could not be encoded")
	}
	if err := json.Unmarshal(encoded, &request); err != nil {
		return request, errors.New("godown transfer payload is invalid")
	}
	request.SourceGodownID = strings.TrimSpace(request.SourceGodownID)
	request.DestinationGodownID = strings.TrimSpace(request.DestinationGodownID)
	request.OccurredAt = strings.TrimSpace(request.OccurredAt)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.SourceGodownID == "" || !documentUUIDPattern.MatchString(request.SourceGodownID) {
		return request, errors.New("sourceGodownId must be a UUID")
	}
	if request.DestinationGodownID == "" || !documentUUIDPattern.MatchString(request.DestinationGodownID) {
		return request, errors.New("destinationGodownId must be a UUID")
	}
	if request.SourceGodownID == request.DestinationGodownID {
		return request, errors.New("source and destination godowns must be different")
	}
	if len(request.Lines) == 0 {
		return request, errors.New("at least one transfer line is required")
	}
	if len(request.Lines) > 100 {
		return request, errors.New("a godown transfer may contain no more than 100 lines")
	}
	for index := range request.Lines {
		request.Lines[index].ItemID = strings.TrimSpace(request.Lines[index].ItemID)
		request.Lines[index].Quantity = strings.TrimSpace(request.Lines[index].Quantity)
		request.Lines[index].BatchNumber = strings.TrimSpace(request.Lines[index].BatchNumber)
		if request.Lines[index].ItemID == "" || !documentUUIDPattern.MatchString(request.Lines[index].ItemID) {
			return request, fmt.Errorf("line %d itemId must be a UUID", index+1)
		}
		if _, err := parseStockQuantity(request.Lines[index].Quantity); err != nil {
			return request, fmt.Errorf("line %d quantity: %w", index+1, err)
		}
	}
	if request.OccurredAt == "" {
		request.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	} else if _, err := time.Parse(time.RFC3339, request.OccurredAt); err != nil {
		return request, errors.New("occurredAt must be RFC3339")
	}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = fmt.Sprintf("godown-transfer-%s-%s-%s", request.SourceGodownID, request.DestinationGodownID, request.OccurredAt)
	}
	return request, nil
}

func (s *Server) handleGodownTransfer(w http.ResponseWriter, r *http.Request, operator *sessionContext, payload map[string]any) {
	request, err := parseGodownTransferRequest(payload)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_godown_transfer", "Invalid godown transfer", err.Error())
		return
	}
	if !s.requireScope(r, w, operator, "godown", request.SourceGodownID) || !s.requireScope(r, w, operator, "godown", request.DestinationGodownID) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The stock store could not be opened.")
		return
	}
	defer tx.Rollback()
	moved, eventID, err := projectGodownTransfer(r.Context(), tx, operator, request)
	if err != nil {
		status, code := stockCommandErrorStatus(err)
		if status == http.StatusInternalServerError {
			status, code = http.StatusUnprocessableEntity, "godown_transfer_rejected"
		}
		writeProblem(w, status, code, "Godown transfer rejected", err.Error())
		return
	}
	auditPayload := map[string]any{
		"sourceGodownId":      request.SourceGodownID,
		"destinationGodownId": request.DestinationGodownID,
		"occurredAt":          request.OccurredAt,
		"idempotencyKey":      request.IdempotencyKey,
		"eventId":             eventID,
		"moved":               moved,
	}
	operationID, err := recordMaintenanceOperation(r, tx, operator, "godown-transfer", "completed", "Godown transfer posted.", auditPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The godown transfer could not be recorded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The godown transfer could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":        "godown-transfer",
		"operationId": operationID,
		"eventId":     eventID,
		"status":      "completed",
		"outcome":     "completed",
		"moved":       moved,
		"message":     "Stock moved between godowns.",
	})
}

func projectGodownTransfer(ctx context.Context, tx *sql.Tx, operator *sessionContext, request godownTransferRequest) (int, string, error) {
	if err := requireActiveGodown(ctx, tx, operator.TenantID, request.SourceGodownID); err != nil {
		return 0, "", err
	}
	if err := requireActiveGodown(ctx, tx, operator.TenantID, request.DestinationGodownID); err != nil {
		return 0, "", err
	}
	occurredAt, err := time.Parse(time.RFC3339, request.OccurredAt)
	if err != nil {
		return 0, "", errors.New("occurredAt must be RFC3339")
	}
	var eventID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO sync_events
			(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id,
			 idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4::uuid, 'inventory',
			gen_random_uuid(), $5, 1, $6::jsonb, $7::timestamptz)
		RETURNING event_id::text
	`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, request.IdempotencyKey,
		mustJSON(map[string]any{
			"kind":                "godown-transfer",
			"sourceGodownId":      request.SourceGodownID,
			"destinationGodownId": request.DestinationGodownID,
			"lines":               request.Lines,
		}), occurredAt).Scan(&eventID)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, "", errors.New("godown transfer idempotency key already exists")
		}
		return 0, "", err
	}
	moved := 0
	for index, line := range request.Lines {
		if err := transferOneLine(ctx, tx, operator, request, line, index, eventID, occurredAt); err != nil {
			return 0, "", fmt.Errorf("line %d: %w", index+1, err)
		}
		moved++
	}
	return moved, eventID, nil
}

func transferOneLine(ctx context.Context, tx *sql.Tx, operator *sessionContext, request godownTransferRequest, line godownTransferLine, index int, eventID string, occurredAt time.Time) error {
	var itemID, itemLegacyID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, legacy_id
		FROM master_items
		WHERE tenant_id = $1::uuid AND id = $2::uuid AND active
	`, operator.TenantID, line.ItemID).Scan(&itemID, &itemLegacyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("item is not an active canonical item in the authenticated tenant")
		}
		return err
	}
	quantity, err := parseStockQuantity(line.Quantity)
	if err != nil {
		return err
	}
	policy, err := stockAllocationPolicy()
	if err != nil {
		return err
	}
	var choices []stockAllocationChoice
	if line.BatchNumber != "" {
		choices, err = resolveStockChoices(ctx, tx, operator, itemID, request.SourceGodownID, []documentAllocationRequest{{
			BatchNumber: line.BatchNumber, Quantity: line.Quantity,
		}}, occurredAt)
		if err != nil {
			return err
		}
	}
	if len(choices) == 0 {
		choices, err = fifoStockChoices(ctx, tx, operator, itemID, request.SourceGodownID, occurredAt, policy)
		if err != nil {
			return err
		}
	}
	remaining := new(big.Rat).Set(quantity)
	for choiceIndex, choice := range choices {
		if remaining.Sign() <= 0 {
			break
		}
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
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_ledger
				(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
				 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, NULL, NULL, $5,
				'out', 1, $6::numeric, $7::numeric, $8::timestamptz)
		`, operator.TenantID, operator.BranchID, choice.batch.ID, eventID,
			fmt.Sprintf("transfer-out-%d-%d", index+1, choiceIndex), qty, choice.batch.UnitCost, occurredAt); err != nil {
			return err
		}
		expiry := ""
		if choice.batch.ExpiryDate.Valid {
			expiry = strings.TrimSpace(choice.batch.ExpiryDate.String)
		}
		destBatchID, _, err := lockOrCreatePurchaseBatch(ctx, tx, operator, itemID, itemLegacyID,
			request.DestinationGodownID, choice.batch.BatchNumber, expiry, choice.batch.UnitCost, occurredAt)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_balances
				(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, $7::numeric)
			ON CONFLICT (tenant_id, branch_id, batch_id) DO UPDATE
			SET on_hand = stock_balances.on_hand + EXCLUDED.on_hand, updated_at = now()
		`, operator.TenantID, operator.BranchID, destBatchID, itemID, itemLegacyID, request.DestinationGodownID, qty); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_ledger
				(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
				 source_document_line_id, source_line_key, direction, adjustment_sign, quantity, unit_cost, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, NULL, NULL, $5,
				'in', 1, $6::numeric, $7::numeric, $8::timestamptz)
		`, operator.TenantID, operator.BranchID, destBatchID, eventID,
			fmt.Sprintf("transfer-in-%d-%d", index+1, choiceIndex), qty, choice.batch.UnitCost, occurredAt); err != nil {
			return err
		}
		remaining.Sub(remaining, take)
	}
	if remaining.Sign() > 0 {
		return errors.New("insufficient stock; the available batches could not satisfy the transfer")
	}
	return nil
}

func requireActiveGodown(ctx context.Context, tx *sql.Tx, tenantID, godownID string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM master_godowns
			WHERE tenant_id = $1::uuid AND id = $2::uuid AND active
		)
	`, tenantID, godownID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("godownId is not an active canonical godown in the authenticated tenant")
	}
	return nil
}

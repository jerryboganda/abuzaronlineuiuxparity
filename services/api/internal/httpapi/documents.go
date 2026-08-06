package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

var documentUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var businessDocumentKinds = map[string]struct{}{
	"cash-sale":        {},
	"credit-sale":      {},
	"pack-purchase":    {},
	"loose-purchase":   {},
	"opening-purchase": {},
	"purchase-return":  {},
	"purchase-order":   {},
}

var purchaseDocumentKinds = map[string]struct{}{
	"pack-purchase":    {},
	"loose-purchase":   {},
	"opening-purchase": {},
	"purchase-return":  {},
	"purchase-order":   {},
}

func isPurchaseDocumentKind(kind string) bool {
	_, ok := purchaseDocumentKinds[kind]
	return ok
}

func isPurchasePostingKind(kind string) bool {
	return isPurchaseDocumentKind(kind) && kind != "purchase-order"
}

type documentCommandRequest struct {
	CommandID       string                `json:"commandId"`
	Kind            string                `json:"kind"`
	Action          string                `json:"action"`
	IdempotencyKey  string                `json:"idempotencyKey"`
	OccurredAt      string                `json:"occurredAt"`
	ExpectedVersion *int64                `json:"expectedVersion,omitempty"`
	Document        *documentDraftRequest `json:"document,omitempty"`
	DocumentID      string                `json:"documentId,omitempty"`
	Reason          string                `json:"reason,omitempty"`
}

type documentDraftRequest struct {
	ID                      string                  `json:"id,omitempty"`
	Kind                    string                  `json:"kind"`
	DocumentNumber          string                  `json:"documentNumber,omitempty"`
	OccurredAt              string                  `json:"occurredAt"`
	CustomerID              string                  `json:"customerId,omitempty"`
	SupplierID              string                  `json:"supplierId,omitempty"`
	SourceDocumentID        string                  `json:"sourceDocumentId,omitempty"`
	SourceDocumentNumber    string                  `json:"sourceDocumentNumber,omitempty"`
	GodownID                string                  `json:"godownId,omitempty"`
	Reference               string                  `json:"reference,omitempty"`
	Remarks                 string                  `json:"remarks,omitempty"`
	Lines                   []documentLineRequest   `json:"lines"`
	PriceLevel              int                     `json:"priceLevel,omitempty"`
	FlatDiscountAmount      string                  `json:"flatDiscountAmount,omitempty"`
	MiscAmount              *string                 `json:"miscAmount,omitempty"`
	DocumentDiscountPercent string                  `json:"documentDiscountPercent,omitempty"`
	Pricing                 *documentPricingRequest `json:"pricing,omitempty"`
}

type documentLineRequest struct {
	LineNumber      int                           `json:"lineNumber"`
	ItemID          string                        `json:"itemId"`
	Quantity        string                        `json:"quantity"`
	UnitPrice       string                        `json:"unitPrice,omitempty"`
	DiscountPercent string                        `json:"discountPercent,omitempty"`
	SupplierScheme  *pricingPreviewSupplierScheme `json:"supplierScheme,omitempty"`
	Allocations     []documentAllocationRequest   `json:"allocations,omitempty"`
	BatchNumber     string                        `json:"batchNumber,omitempty"`
	ExpiryDate      string                        `json:"expiryDate,omitempty"`
	UnitCost        string                        `json:"unitCost,omitempty"`
	GSTRate         string                        `json:"gstRate,omitempty"`
	PCTRate         string                        `json:"pctRate,omitempty"`
	AdvanceTaxRate  string                        `json:"advanceTaxRate,omitempty"`
	Notes           string                        `json:"notes,omitempty"`
}

type documentAllocationRequest struct {
	BatchID     string `json:"batchId,omitempty"`
	BatchNumber string `json:"batchNumber,omitempty"`
	Quantity    string `json:"quantity"`
}

type documentPricingRequest struct {
	PriceLevel              int                 `json:"priceLevel,omitempty"`
	GroupDiscountPercent    string              `json:"groupDiscountPercent,omitempty"`
	CustomerDiscountPercent *string             `json:"customerDiscountPercent,omitempty"`
	OverrideDiscountPercent *string             `json:"overrideDiscountPercent,omitempty"`
	DocumentDiscountPercent string              `json:"documentDiscountPercent,omitempty"`
	FlatDiscountAmount      string              `json:"flatDiscountAmount,omitempty"`
	MiscAmount              *string             `json:"miscAmount,omitempty"`
	Taxes                   pricingPreviewTaxes `json:"taxes,omitempty"`
}

type documentCommandError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type documentCommandResponse struct {
	Accepted    bool                   `json:"accepted"`
	Duplicate   bool                   `json:"duplicate"`
	EventID     string                 `json:"eventId"`
	AggregateID string                 `json:"aggregateId"`
	Kind        string                 `json:"kind"`
	Action      string                 `json:"action"`
	Status      string                 `json:"status"`
	Document    documentResponse       `json:"document"`
	Errors      []documentCommandError `json:"errors,omitempty"`
}

type documentResponse struct {
	ID                   string                  `json:"id"`
	Kind                 string                  `json:"kind"`
	Status               string                  `json:"status"`
	DocumentNumber       string                  `json:"documentNumber"`
	TenantID             string                  `json:"tenantId"`
	BranchID             string                  `json:"branchId"`
	CounterID            string                  `json:"counterId"`
	OperatorID           string                  `json:"operatorId"`
	CustomerID           string                  `json:"customerId,omitempty"`
	SupplierID           string                  `json:"supplierId,omitempty"`
	SourceDocumentID     string                  `json:"sourceDocumentId,omitempty"`
	SourceDocumentNumber string                  `json:"sourceDocumentNumber,omitempty"`
	OccurredAt           string                  `json:"occurredAt"`
	GodownID             string                  `json:"godownId,omitempty"`
	Reference            string                  `json:"reference,omitempty"`
	Remarks              string                  `json:"remarks,omitempty"`
	VoidReason           string                  `json:"voidReason,omitempty"`
	Lines                []documentLineResponse  `json:"lines"`
	Totals               documentTotals          `json:"totals"`
	Pricing              json.RawMessage         `json:"pricing,omitempty"`
	Finance              *documentFinanceSummary `json:"finance,omitempty"`
	CreatedAt            string                  `json:"createdAt"`
	UpdatedAt            string                  `json:"updatedAt"`
	Version              int64                   `json:"version"`
}

type documentLineResponse struct {
	ID           string                       `json:"id"`
	LineNumber   int                          `json:"lineNumber"`
	ItemID       string                       `json:"itemId"`
	ItemLegacyID string                       `json:"itemLegacyId,omitempty"`
	ItemCode     string                       `json:"itemCode"`
	ItemName     string                       `json:"itemName"`
	Quantity     string                       `json:"quantity"`
	Price        documentPrice                `json:"price"`
	LineTotal    string                       `json:"lineTotal"`
	Stock        documentStockSummary         `json:"stock"`
	Allocations  []documentAllocationResponse `json:"allocations,omitempty"`
	BatchNumber  string                       `json:"batchNumber,omitempty"`
	ExpiryDate   string                       `json:"expiryDate,omitempty"`
	UnitCost     string                       `json:"unitCost,omitempty"`
	TaxAmount    string                       `json:"taxAmount,omitempty"`
	Notes        string                       `json:"notes,omitempty"`
}

type documentAllocationResponse struct {
	BatchID     string `json:"batchId"`
	BatchNumber string `json:"batchNumber"`
	Quantity    string `json:"quantity"`
	UnitCost    string `json:"unitCost"`
}

type documentPrice struct {
	PriceTier       int    `json:"priceTier"`
	UnitPrice       string `json:"unitPrice"`
	GrossAmount     string `json:"grossAmount"`
	DiscountPercent string `json:"discountPercent"`
	DiscountAmount  string `json:"discountAmount"`
	NetAmount       string `json:"netAmount"`
}

type documentStockSummary struct {
	Direction string `json:"direction"`
	Quantity  string `json:"quantity"`
}

type documentTotals struct {
	Subtotal       string `json:"subtotal"`
	DiscountAmount string `json:"discountAmount"`
	MiscAmount     string `json:"miscAmount"`
	TaxAmount      string `json:"taxAmount"`
	TotalAmount    string `json:"totalAmount"`
	PaidAmount     string `json:"paidAmount"`
	BalanceAmount  string `json:"balanceAmount"`
}

type pricedDocumentLine struct {
	request  documentLineRequest
	itemID   string
	legacyID string
	code     string
	name     string
	result   pricing.LineResult
}

type pricedDocument struct {
	lines       []pricedDocumentLine
	request     pricingPreviewRequest
	result      pricing.Result
	pricingJSON json.RawMessage
}

func (s *Server) documentCommand(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	kind := strings.ToLower(strings.TrimSpace(r.PathValue("kind")))
	if _, ok := businessDocumentKinds[kind]; !ok {
		writeProblem(w, http.StatusBadRequest, "invalid_document_kind", "Invalid business document kind", "Only cash-sale and credit-sale lifecycle commands are implemented in this slice.")
		return
	}
	writePermission := "sales.write"
	if isPurchaseDocumentKind(kind) {
		writePermission = "purchases.write"
	}
	if !s.requirePermission(r, w, operator, writePermission) {
		return
	}
	if operator.BranchID == "" || operator.CounterID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch and counter required", "Select a branch and counter before saving a business document.")
		return
	}

	var request documentCommandRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_document_command", "Invalid document command", "The document command could not be parsed.")
		return
	}
	request.Kind = strings.ToLower(strings.TrimSpace(request.Kind))
	if request.Kind != kind {
		writeProblem(w, http.StatusBadRequest, "document_kind_mismatch", "Document kind mismatch", "The command kind must match the URL document family.")
		return
	}
	if err := validateDocumentCommand(request, kind); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_document_command", "Invalid document command", err.Error())
		return
	}
	payloadHash, err := hashDocumentCommand(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_document_command", "Invalid document command", "The command could not be normalized.")
		return
	}

	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The business document store could not be opened.")
		return
	}
	defer tx.Rollback()

	receiptID, duplicate, err := claimDocumentCommand(r.Context(), tx, operator, request, payloadHash)
	if err != nil {
		if errors.Is(err, errDocumentCommandConflict) {
			_ = tx.Commit()
			writeProblem(w, http.StatusConflict, "document_command_conflict", "Document command conflict", "The idempotency key or command ID is already associated with a different payload.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "document_command_rejected", "Document command rejected", "The command receipt could not be recorded.")
		return
	}
	if duplicate {
		var response documentCommandResponse
		var storedResponse []byte
		if err := tx.QueryRowContext(r.Context(), `SELECT response FROM command_receipts WHERE id = $1::uuid AND tenant_id = $2::uuid`, receiptID, operator.TenantID).Scan(&storedResponse); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "document_receipt_read_failed", "Document receipt could not be read", "The idempotent command response is unavailable.")
			return
		}
		if err := json.Unmarshal(storedResponse, &response); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "document_receipt_read_failed", "Document receipt could not be read", "The idempotent command response is invalid.")
			return
		}
		response.Duplicate = true
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "document_receipt_read_failed", "Document receipt could not be read", "The idempotent command transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	var response documentCommandResponse
	var documentID string
	if request.Action == "void" {
		documentID, response, err = s.voidBusinessDocument(r.Context(), tx, operator, request)
	} else {
		documentID, response, err = s.saveBusinessDocument(r.Context(), tx, operator, request)
	}
	if err != nil {
		status, code := documentCommandErrorStatus(err)
		writeProblem(w, status, code, "Document command rejected", err.Error())
		return
	}
	response.Accepted = true
	response.Duplicate = false
	response.AggregateID = documentID
	response.Kind = kind
	response.Action = request.Action
	response.Status = response.Document.Status
	eventID, err := emitDocumentSyncEvent(r.Context(), tx, operator, request, response)
	if err != nil {
		if isUniqueViolation(err) {
			writeProblem(w, http.StatusConflict, "document_event_conflict", "Document event conflict", "The document command conflicts with an existing synchronization event.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "document_event_failed", "Document command failed", "The immutable document event could not be stored.")
		return
	}
	// eventId is the immutable sync event identity. The command receipt has a
	// separate internal identity and is deliberately not exposed as eventId.
	response.EventID = eventID
	if response.Document.Status == "posted" {
		if isPurchaseDocumentKind(response.Document.Kind) {
			if isPurchasePostingKind(response.Document.Kind) {
				if err := projectPostedPurchaseStock(r.Context(), tx, operator, request.Document, response.Document, eventID); err != nil {
					status, code := stockCommandErrorStatus(err)
					writeProblem(w, status, code, "Document stock rejected", err.Error())
					return
				}
				if err := projectPostedPurchaseFinance(r.Context(), tx, operator, response.Document, eventID); err != nil {
					writeProblem(w, http.StatusUnprocessableEntity, "document_finance_rejected", "Document finance rejected", err.Error())
					return
				}
			}
		} else {
			if err := projectPostedSaleStock(r.Context(), tx, operator, request.Document, response.Document, eventID); err != nil {
				status, code := stockCommandErrorStatus(err)
				writeProblem(w, status, code, "Document stock rejected", err.Error())
				return
			}
			// Finance is deliberately posted in this same transaction, after stock
			// allocation provides authoritative COGS, and before the receipt,
			// revision, audit, and commit can succeed.
			if err := projectPostedSaleFinance(r.Context(), tx, operator, response.Document, eventID); err != nil {
				writeProblem(w, http.StatusUnprocessableEntity, "document_finance_rejected", "Document finance rejected", err.Error())
				return
			}
		}
		// Stock allocation is part of the same transaction. Re-read the
		// response so the committed document exposes its authoritative stock
		// allocation and finance summary.
		response.Document, err = readBusinessDocument(r.Context(), tx, operator, documentID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "document_read_failed", "Document command failed", "The posted document could not be re-read after stock allocation.")
			return
		}
		response.Status = response.Document.Status
	}
	if err := finalizeDocumentSyncEvent(r.Context(), tx, operator, request, eventID, response.Document); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_event_finalize_failed", "Document command failed", "The immutable document event could not be finalized.")
		return
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "document_response_failed", "Document response failed", "The document command response could not be encoded.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO business_document_revisions
			(tenant_id, branch_id, document_id, revision_number, action, status, snapshot)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::jsonb)
	`, operator.TenantID, operator.BranchID, documentID, response.Document.Version, request.Action, response.Document.Status, responseBytes); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_revision_failed", "Document command failed", "The document revision could not be stored.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE command_receipts
		SET document_id = $1::uuid, response = $2::jsonb
		WHERE id = $3::uuid AND tenant_id = $4::uuid
	`, documentID, responseBytes, receiptID, operator.TenantID); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_receipt_failed", "Document command failed", "The atomic command receipt could not be completed.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'business_document', $5::uuid, $6::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, "document."+request.Action, documentID, responseBytes); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_audit_failed", "Document command failed", "The document audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_commit_failed", "Document command failed", "The business document and command receipt could not be committed.")
		return
	}
	status := http.StatusOK
	if request.Action == "save" && request.Document != nil && strings.TrimSpace(request.Document.ID) == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, response)
}

func emitDocumentSyncEvent(ctx context.Context, tx *sql.Tx, operator *sessionContext, command documentCommandRequest, response documentCommandResponse) (string, error) {
	payload := map[string]any{
		"schemaVersion": 1,
		"state":         "pending",
		"commandId":     command.CommandID,
	}
	var eventID string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO sync_events
			(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id,
			 idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'business_document',
			$5::uuid, $6, 1, $7::jsonb, $8::timestamptz)
		RETURNING event_id::text
	`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID,
		response.Document.ID, command.IdempotencyKey, mustJSON(payload), response.Document.OccurredAt).Scan(&eventID)
	return eventID, err
}

func finalizeDocumentSyncEvent(ctx context.Context, tx *sql.Tx, operator *sessionContext, command documentCommandRequest, eventID string, document documentResponse) error {
	payload := map[string]any{
		"schemaVersion": 1,
		"state":         "final",
		"eventId":       eventID,
		"commandId":     command.CommandID,
		"kind":          command.Kind,
		"action":        command.Action,
		"status":        document.Status,
		"document":      document,
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE sync_events
		SET schema_version = 1, payload = $1::jsonb, finalized_at = now()
		WHERE tenant_id = $2::uuid AND event_id = $3::uuid
	`, mustJSON(payload), operator.TenantID, eventID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("immutable document event %s was not found for finalization", eventID)
	}
	return nil
}

var errDocumentCommandConflict = errors.New("document command conflicts with an existing receipt")

func claimDocumentCommand(ctx context.Context, tx *sql.Tx, operator *sessionContext, request documentCommandRequest, payloadHash string) (string, bool, error) {
	var receiptID string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO command_receipts
			(tenant_id, branch_id, operator_id, command_id, idempotency_key, kind, action, payload_hash, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8, 'accepted')
		ON CONFLICT DO NOTHING
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, operator.UserID, request.CommandID, request.IdempotencyKey, request.Kind, request.Action, payloadHash).Scan(&receiptID)
	if err == nil {
		return receiptID, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	var existingHash string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, payload_hash
		FROM command_receipts
		WHERE tenant_id = $1::uuid AND (idempotency_key = $2 OR command_id = $3::uuid)
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE
	`, operator.TenantID, request.IdempotencyKey, request.CommandID).Scan(&receiptID, &existingHash); err != nil {
		return "", false, err
	}
	if existingHash != payloadHash {
		return "", false, errDocumentCommandConflict
	}
	return receiptID, true, nil
}

func (s *Server) saveBusinessDocument(ctx context.Context, tx *sql.Tx, operator *sessionContext, command documentCommandRequest) (string, documentCommandResponse, error) {
	draft := command.Document
	priced, err := s.priceDocument(ctx, tx, operator, draft)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	status := "draft"
	if command.Action == "post" || command.Action == "save-and-post" {
		status = "posted"
	}
	documentID := strings.TrimSpace(draft.ID)
	var version int64
	if documentID != "" {
		if command.ExpectedVersion == nil {
			return "", documentCommandResponse{}, errors.New("expectedVersion is required when updating an existing document")
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT version FROM business_documents
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4
			FOR UPDATE
		`, documentID, operator.TenantID, operator.BranchID, command.Kind).Scan(&version); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", documentCommandResponse{}, errors.New("document was not found in the authenticated tenant and branch")
			}
			return "", documentCommandResponse{}, err
		}
		if version != *command.ExpectedVersion {
			return "", documentCommandResponse{}, fmt.Errorf("expectedVersion %d does not match current document version %d", *command.ExpectedVersion, version)
		}
		var currentStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM business_documents WHERE id = $1::uuid`, documentID).Scan(&currentStatus); err != nil {
			return "", documentCommandResponse{}, err
		}
		if currentStatus == "void" || currentStatus == "posted" {
			return "", documentCommandResponse{}, fmt.Errorf("document with status %q cannot be edited", currentStatus)
		}
		if command.Action == "post" && currentStatus != "draft" {
			return "", documentCommandResponse{}, fmt.Errorf("document with status %q cannot be posted", currentStatus)
		}
	} else if command.Action == "post" {
		return "", documentCommandResponse{}, errors.New("post requires an existing draft document")
	} else if command.ExpectedVersion != nil {
		return "", documentCommandResponse{}, errors.New("expectedVersion is only valid when updating an existing document")
	}

	customerID := strings.TrimSpace(draft.CustomerID)
	supplierID := strings.TrimSpace(draft.SupplierID)
	sourceDocumentID := strings.TrimSpace(draft.SourceDocumentID)
	if isPurchaseDocumentKind(command.Kind) {
		if customerID != "" {
			return "", documentCommandResponse{}, errors.New("purchase documents cannot specify customerId")
		}
		if supplierID == "" || !documentUUIDPattern.MatchString(supplierID) {
			return "", documentCommandResponse{}, errors.New("purchase documents require a canonical supplierId")
		}
		var supplierExists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM master_parties
				WHERE tenant_id = $1::uuid AND id = $2::uuid
				  AND party_type = 'supplier' AND active
			)
		`, operator.TenantID, supplierID).Scan(&supplierExists); err != nil {
			return "", documentCommandResponse{}, err
		}
		if !supplierExists {
			return "", documentCommandResponse{}, errors.New("supplierId is not an active canonical supplier in the authenticated tenant")
		}
		if sourceDocumentID != "" && !documentUUIDPattern.MatchString(sourceDocumentID) {
			return "", documentCommandResponse{}, errors.New("sourceDocumentId must be a UUID")
		}
		if command.Kind == "purchase-return" && (command.Action == "post" || command.Action == "save-and-post") && sourceDocumentID == "" {
			return "", documentCommandResponse{}, errors.New("purchase-return posting requires sourceDocumentId")
		}
		if command.Kind != "purchase-order" && (command.Action == "post" || command.Action == "save-and-post") {
			if strings.TrimSpace(draft.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(draft.GodownID)) {
				return "", documentCommandResponse{}, errors.New("purchase posting requires an explicit godownId")
			}
		}
	} else if supplierID != "" || sourceDocumentID != "" {
		return "", documentCommandResponse{}, errors.New("supplier and source document fields are only valid for purchases")
	}
	if command.Kind == "credit-sale" {
		if customerID == "" || !documentUUIDPattern.MatchString(customerID) {
			return "", documentCommandResponse{}, errors.New("credit-sale requires a canonical customerId")
		}
		var customerExists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM master_parties
				WHERE tenant_id = $1::uuid AND id = $2::uuid AND party_type = 'customer' AND active
			)
		`, operator.TenantID, customerID).Scan(&customerExists); err != nil {
			return "", documentCommandResponse{}, err
		}
		if !customerExists {
			return "", documentCommandResponse{}, errors.New("customerId is not an active canonical customer in the authenticated tenant")
		}
	} else if customerID != "" && !documentUUIDPattern.MatchString(customerID) {
		return "", documentCommandResponse{}, errors.New("customerId must be a UUID")
	}
	if documentID == "" {
		documentNumber, numberErr := nextBusinessDocumentNumber(ctx, tx, operator.TenantID, operator.BranchID, command.Kind)
		if numberErr != nil {
			return "", documentCommandResponse{}, numberErr
		}
		err = tx.QueryRowContext(ctx, `
			INSERT INTO business_documents
				(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status, occurred_at,
				customer_id, supplier_id, source_document_id, source_document_number, godown_id, reference, remarks, price_level, subtotal, discount_amount, misc_amount,
				 tax_amount, total_amount, paid_amount, balance_amount, pricing_inputs, pricing_result)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8::timestamptz,
				NULLIF($9, '')::uuid, NULLIF($10, '')::uuid, NULLIF($11, '')::uuid, $12, NULLIF($13, '')::uuid, $14, $15, $16, $17, $18, $19, $20, $21, 0, $21, $22::jsonb, $23::jsonb)
			RETURNING id::text
		`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, command.Kind, documentNumber, status,
			draft.OccurredAt, customerID, supplierID, sourceDocumentID, strings.TrimSpace(draft.SourceDocumentNumber),
			strings.TrimSpace(draft.GodownID), strings.TrimSpace(draft.Reference), strings.TrimSpace(draft.Remarks), priced.request.PriceLevel,
			formatMoney(priced.result.Subtotal+priced.result.LineDiscountTotal), formatMoney(priced.result.TotalDiscount),
			formatMoney(priced.result.Misc), formatTotalTax(priced.result), formatMoney(priced.result.Total),
			mustJSON(priced.request), priced.pricingJSON).Scan(&documentID)
		if err != nil {
			return "", documentCommandResponse{}, err
		}
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE business_documents
			SET status = $1, occurred_at = $2::timestamptz, customer_id = NULLIF($3, '')::uuid,
				supplier_id = NULLIF($4, '')::uuid, source_document_id = NULLIF($5, '')::uuid,
				source_document_number = $6, godown_id = NULLIF($7, '')::uuid, reference = $8, remarks = $9, price_level = $10, subtotal = $11, discount_amount = $12,
				misc_amount = $13, tax_amount = $14, total_amount = $15, paid_amount = 0,
				balance_amount = $15, pricing_inputs = $16::jsonb, pricing_result = $17::jsonb,
				version = version + 1, updated_at = now()
			WHERE id = $18::uuid AND tenant_id = $19::uuid AND branch_id = $20::uuid
		`, status, draft.OccurredAt, customerID, supplierID, sourceDocumentID, strings.TrimSpace(draft.SourceDocumentNumber),
			strings.TrimSpace(draft.GodownID), strings.TrimSpace(draft.Reference), strings.TrimSpace(draft.Remarks),
			priced.request.PriceLevel, formatMoney(priced.result.Subtotal+priced.result.LineDiscountTotal),
			formatMoney(priced.result.TotalDiscount), formatMoney(priced.result.Misc), formatTotalTax(priced.result),
			formatMoney(priced.result.Total), mustJSON(priced.request), priced.pricingJSON, documentID, operator.TenantID, operator.BranchID)
		if err != nil {
			return "", documentCommandResponse{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM business_document_lines WHERE tenant_id = $1::uuid AND document_id = $2::uuid`, operator.TenantID, documentID); err != nil {
		return "", documentCommandResponse{}, err
	}
	for index, line := range priced.lines {
		lineTaxes, lineTaxAmount := lineTaxSnapshot(priced.result, index)
		linePricing := map[string]any{
			"supplierDiscount":       formatMoney(line.result.SupplierDiscount),
			"supplierBonusQuantity":  formatQuantity(line.result.SupplierBonusQuantity),
			"itemDiscount":           formatMoney(line.result.ItemDiscount),
			"customerDiscount":       formatMoney(line.result.CustomerDiscount),
			"customerDiscountRate":   formatPercent(line.result.CustomerDiscountRate),
			"customerDiscountSource": customerDiscountSourceName(line.result.CustomerDiscountSource),
			"taxes":                  lineTaxes,
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO business_document_lines
				(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id, item_code, item_name,
				 quantity, unit_price, line_gross, supplier_discount, item_discount, customer_discount, line_total,
				 pricing, batch_number, expiry_date, unit_cost, gst_rate, pct_rate, advance_tax_rate, tax_amount, notes)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			        $16::jsonb, $17, NULLIF($18, '')::date, $19, $20, $21, $22, $23, $24)
		`, operator.TenantID, operator.BranchID, documentID, index+1, line.itemID, line.legacyID, line.code, line.name,
			formatQuantityString(line.request.Quantity), formatMoney(line.result.ResolvedUnitPrice), formatMoney(line.result.LineGross),
			formatMoney(line.result.SupplierDiscount), formatMoney(line.result.ItemDiscount), formatMoney(line.result.CustomerDiscount),
			formatMoney(line.result.Net), mustJSON(linePricing), strings.TrimSpace(line.request.BatchNumber), strings.TrimSpace(line.request.ExpiryDate),
			purchaseUnitCost(line), decimalOrZero(line.request.GSTRate), decimalOrZero(line.request.PCTRate),
			decimalOrZero(line.request.AdvanceTaxRate), formatMoney(lineTaxAmount), strings.TrimSpace(line.request.Notes)); err != nil {
			return "", documentCommandResponse{}, err
		}
	}
	response, err := readBusinessDocument(ctx, tx, operator, documentID)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	return documentID, documentCommandResponse{Document: response}, nil
}

func (s *Server) voidBusinessDocument(ctx context.Context, tx *sql.Tx, operator *sessionContext, command documentCommandRequest) (string, documentCommandResponse, error) {
	if command.ExpectedVersion == nil {
		return "", documentCommandResponse{}, errors.New("expectedVersion is required when voiding a document")
	}
	var documentID, status string
	var version int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, status, version FROM business_documents
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4
		FOR UPDATE
	`, command.DocumentID, operator.TenantID, operator.BranchID, command.Kind).Scan(&documentID, &status, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", documentCommandResponse{}, errors.New("document was not found in the authenticated tenant and branch")
		}
		return "", documentCommandResponse{}, err
	}
	if version != *command.ExpectedVersion {
		return "", documentCommandResponse{}, fmt.Errorf("expectedVersion %d does not match current document version %d", *command.ExpectedVersion, version)
	}
	if status == "void" {
		return "", documentCommandResponse{}, errors.New("document is already void")
	}
	if status == "posted" && command.Kind != "purchase-order" {
		return "", documentCommandResponse{}, errors.New("posted document cannot be voided until a stock reversal workflow exists")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE business_documents SET status = 'void', void_reason = $3, updated_at = now(), version = version + 1
		WHERE id = $1::uuid AND tenant_id = $2::uuid
	`, documentID, operator.TenantID, strings.TrimSpace(command.Reason)); err != nil {
		return "", documentCommandResponse{}, err
	}
	response, err := readBusinessDocument(ctx, tx, operator, documentID)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	return documentID, documentCommandResponse{Document: response}, nil
}

func (s *Server) priceDocument(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest) (pricedDocument, error) {
	if draft == nil {
		return pricedDocument{}, errors.New("document is required for this action")
	}
	if draft.Kind != operatorDocumentKind(draft.Kind) {
		return pricedDocument{}, errors.New("document kind is invalid")
	}
	priceLevel := draft.PriceLevel
	if priceLevel == 0 {
		priceLevel = 1
	}
	if draft.Pricing != nil && draft.Pricing.PriceLevel != 0 {
		priceLevel = draft.Pricing.PriceLevel
	}
	if priceLevel < 1 || priceLevel > 10 {
		return pricedDocument{}, errors.New("priceLevel must be between 1 and 10")
	}
	priced := pricedDocument{lines: make([]pricedDocumentLine, 0, len(draft.Lines))}
	preview := pricingPreviewRequest{PriceLevel: priceLevel, Lines: make([]pricingPreviewLine, 0, len(draft.Lines))}
	if draft.Pricing != nil {
		preview.GroupDiscountPercent = draft.Pricing.GroupDiscountPercent
		preview.CustomerDiscountPercent = draft.Pricing.CustomerDiscountPercent
		preview.OverrideDiscountPercent = draft.Pricing.OverrideDiscountPercent
		preview.DocumentDiscountPercent = draft.Pricing.DocumentDiscountPercent
		preview.FlatDiscountAmount = draft.Pricing.FlatDiscountAmount
		preview.MiscAmount = draft.Pricing.MiscAmount
		preview.Taxes = draft.Pricing.Taxes
	}
	if preview.DocumentDiscountPercent == "" {
		preview.DocumentDiscountPercent = draft.DocumentDiscountPercent
	}
	if preview.FlatDiscountAmount == "" {
		preview.FlatDiscountAmount = draft.FlatDiscountAmount
	}
	if preview.MiscAmount == nil {
		preview.MiscAmount = draft.MiscAmount
	}
	for index, line := range draft.Lines {
		if line.LineNumber == 0 {
			line.LineNumber = index + 1
		}
		var item struct {
			ID       string
			LegacyID string
			Code     string
			Name     string
			Payload  []byte
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT id::text, legacy_id, code, name, payload
			FROM master_items
			WHERE tenant_id = $1::uuid AND id = $2::uuid AND active
		`, operator.TenantID, line.ItemID).Scan(&item.ID, &item.LegacyID, &item.Code, &item.Name, &item.Payload); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return pricedDocument{}, fmt.Errorf("line %d item is not an active canonical item in the authenticated tenant", index+1)
			}
			return pricedDocument{}, err
		}
		tiers, err := canonicalItemPriceTiers(item.Payload, line.UnitPrice, priceLevel)
		if err != nil {
			return pricedDocument{}, fmt.Errorf("line %d: %w", index+1, err)
		}
		preview.Lines = append(preview.Lines, pricingPreviewLine{
			ID:                  item.ID,
			Quantity:            line.Quantity,
			Prices:              tiers,
			ItemDiscountPercent: line.DiscountPercent,
			SupplierScheme:      line.SupplierScheme,
		})
		priced.lines = append(priced.lines, pricedDocumentLine{request: line, itemID: item.ID, legacyID: item.LegacyID, code: item.Code, name: item.Name})
	}
	resolvedTaxes, err := resolveDocumentTaxPolicy(ctx, tx, operator, draft)
	if err != nil {
		return pricedDocument{}, err
	}
	preview.Taxes = resolvedTaxes
	for index := range priced.lines {
		priced.lines[index].request.GSTRate = resolvedTaxRate(resolvedTaxes.GST)
		priced.lines[index].request.PCTRate = resolvedTaxRate(resolvedTaxes.PCT)
		priced.lines[index].request.AdvanceTaxRate = resolvedTaxRate(resolvedTaxes.AdvanceTax)
	}
	calculated, err := preview.toPricingRequest()
	if err != nil {
		return pricedDocument{}, err
	}
	result, err := pricing.Calculate(calculated)
	if err != nil {
		return pricedDocument{}, fmt.Errorf("pricing calculation: %w", err)
	}
	if !pricingTotalsBalanced(result) {
		return pricedDocument{}, errors.New("pricing totals are not balanced")
	}
	for index := range priced.lines {
		priced.lines[index].result = result.Lines[index]
	}
	priced.request = preview
	priced.result = result
	priced.pricingJSON = mustJSON(buildPricingResult(result))
	return priced, nil
}

func validateDocumentCommand(request documentCommandRequest, kind string) error {
	if !documentUUIDPattern.MatchString(strings.TrimSpace(request.CommandID)) {
		return errors.New("commandId must be a UUID")
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return errors.New("idempotencyKey is required")
	}
	if len(request.IdempotencyKey) > 200 {
		return errors.New("idempotencyKey is too long")
	}
	if strings.TrimSpace(request.OccurredAt) == "" {
		return errors.New("occurredAt is required")
	}
	if _, err := time.Parse(time.RFC3339, request.OccurredAt); err != nil {
		return errors.New("occurredAt must be RFC3339")
	}
	switch request.Action {
	case "save", "post", "save-and-post":
		if request.Document == nil {
			return errors.New("document is required for this action")
		}
		if request.Document.Kind != kind {
			return errors.New("document kind must match the URL document family")
		}
		if request.Document.OccurredAt == "" {
			return errors.New("document occurredAt is required")
		}
		if _, err := time.Parse(time.RFC3339, request.Document.OccurredAt); err != nil {
			return errors.New("document occurredAt must be RFC3339")
		}
		if len(request.Document.Lines) == 0 || len(request.Document.Lines) > 500 {
			return errors.New("document must contain between 1 and 500 lines")
		}
		if request.Document.ID != "" && !documentUUIDPattern.MatchString(request.Document.ID) {
			return errors.New("document id must be a UUID")
		}
		if request.Action == "post" && request.Document.ID == "" {
			return errors.New("post requires document.id")
		}
		if request.ExpectedVersion != nil && *request.ExpectedVersion < 1 {
			return errors.New("expectedVersion must be positive")
		}
		for index, line := range request.Document.Lines {
			if !documentUUIDPattern.MatchString(strings.TrimSpace(line.ItemID)) {
				return fmt.Errorf("line %d itemId must be a UUID", index+1)
			}
			if _, err := parseQuantity(line.Quantity); err != nil {
				return fmt.Errorf("line %d quantity: %w", index+1, err)
			}
			if _, err := parsePercent(line.DiscountPercent); err != nil {
				return fmt.Errorf("line %d discountPercent: %w", index+1, err)
			}
			if isPurchaseDocumentKind(kind) {
				if err := validatePurchaseLineMetadata(line, kind, request.Action, index); err != nil {
					return err
				}
			}
		}
	case "void":
		if !documentUUIDPattern.MatchString(strings.TrimSpace(request.DocumentID)) {
			return errors.New("documentId must be a UUID for void")
		}
		if strings.TrimSpace(request.Reason) == "" {
			return errors.New("reason is required for void")
		}
		if request.ExpectedVersion == nil || *request.ExpectedVersion < 1 {
			return errors.New("expectedVersion must be positive for void")
		}
	default:
		return errors.New("action must be save, post, save-and-post, or void")
	}
	return nil
}

func validatePurchaseLineMetadata(line documentLineRequest, kind, action string, index int) error {
	if strings.TrimSpace(line.ExpiryDate) != "" {
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(line.ExpiryDate)); err != nil {
			return fmt.Errorf("line %d expiryDate must be YYYY-MM-DD", index+1)
		}
	}
	if strings.TrimSpace(line.UnitCost) != "" {
		if _, err := parseMoney(line.UnitCost); err != nil {
			return fmt.Errorf("line %d unitCost: %w", index+1, err)
		}
	}
	for name, value := range map[string]string{
		"gstRate": line.GSTRate, "pctRate": line.PCTRate, "advanceTaxRate": line.AdvanceTaxRate,
	} {
		if _, err := parsePercent(value); err != nil {
			return fmt.Errorf("line %d %s: %w", index+1, name, err)
		}
	}
	if action == "post" || action == "save-and-post" {
		switch kind {
		case "pack-purchase", "loose-purchase", "opening-purchase":
			if strings.TrimSpace(line.BatchNumber) == "" {
				return fmt.Errorf("line %d batchNumber is required when posting a purchase", index+1)
			}
			if strings.TrimSpace(line.UnitCost) == "" {
				return fmt.Errorf("line %d unitCost is required when posting a purchase", index+1)
			}
		case "purchase-return":
			if len(line.Allocations) == 0 {
				return fmt.Errorf("line %d requires an explicit batch allocation for purchase return", index+1)
			}
		}
	}
	return nil
}

func hashDocumentCommand(request documentCommandRequest) (string, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func operatorDocumentKind(kind string) string {
	if _, ok := businessDocumentKinds[kind]; ok {
		return kind
	}
	return ""
}

func canonicalItemPriceTiers(payload []byte, fallback string, priceLevel int) ([]string, error) {
	tiers := make([]string, 10)
	var values map[string]json.RawMessage
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &values); err != nil {
			return nil, errors.New("canonical item pricing payload is invalid")
		}
		for key, raw := range values {
			normalized := strings.ToLower(strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' {
					return r
				}
				return -1
			}, key))
			if !strings.HasPrefix(normalized, "saleprice") {
				continue
			}
			level, err := strconv.Atoi(strings.TrimPrefix(normalized, "saleprice"))
			if err != nil || level < 1 || level > 10 {
				continue
			}
			var value string
			if len(raw) > 0 && raw[0] == '"' {
				if err := json.Unmarshal(raw, &value); err != nil {
					continue
				}
			} else {
				value = string(raw)
			}
			if _, err := parseMoney(value); err == nil {
				tiers[level-1] = strings.TrimSpace(value)
			}
		}
	}
	if strings.TrimSpace(tiers[priceLevel-1]) == "" && strings.TrimSpace(fallback) != "" {
		tiers[priceLevel-1] = strings.TrimSpace(fallback)
	}
	if strings.TrimSpace(tiers[priceLevel-1]) == "" {
		return nil, fmt.Errorf("no SalePrice#%d or explicit unitPrice was supplied for the canonical item", priceLevel)
	}
	return tiers, nil
}

func formatExclusiveTax(result pricing.Result) string {
	var total pricing.Money
	for _, tax := range result.Taxes {
		if !tax.Inclusive {
			total += tax.Amount
		}
	}
	return formatMoney(total)
}

func formatTotalTax(result pricing.Result) string {
	var total pricing.Money
	for _, tax := range result.Taxes {
		total += tax.Amount
	}
	return formatMoney(total)
}

func resolvedTaxRate(rule *pricingPreviewTax) string {
	if rule == nil {
		return ""
	}
	return rule.Rate
}

func pricingTotalsBalanced(result pricing.Result) bool {
	gross := result.Subtotal + result.LineDiscountTotal
	return gross-result.TotalDiscount+result.Misc+exclusiveTax(result) == result.Total
}

func exclusiveTax(result pricing.Result) pricing.Money {
	var total pricing.Money
	for _, tax := range result.Taxes {
		if !tax.Inclusive {
			total += tax.Amount
		}
	}
	return total
}

func buildPricingResult(result pricing.Result) map[string]any {
	taxes := make([]map[string]any, 0, len(result.Taxes))
	for _, tax := range result.Taxes {
		taxes = append(taxes, map[string]any{
			"kind": taxKindName(tax.Kind), "rate": formatPercent(tax.Rate),
			"inclusive": tax.Inclusive, "base": formatMoney(tax.Base), "amount": formatMoney(tax.Amount),
		})
	}
	lines := make([]map[string]any, 0, len(result.Lines))
	for _, line := range result.Lines {
		lines = append(lines, map[string]any{
			"id": line.ID, "priceLevel": line.PriceLevel, "resolvedUnitPrice": formatMoney(line.ResolvedUnitPrice),
			"supplierDiscount": formatMoney(line.SupplierDiscount), "supplierBonusQuantity": formatQuantity(line.SupplierBonusQuantity),
			"lineGross": formatMoney(line.LineGross), "itemDiscount": formatMoney(line.ItemDiscount),
			"customerDiscount": formatMoney(line.CustomerDiscount), "customerDiscountRate": formatPercent(line.CustomerDiscountRate),
			"customerDiscountSource": customerDiscountSourceName(line.CustomerDiscountSource), "net": formatMoney(line.Net),
		})
	}
	return map[string]any{
		"priceLevel": result.Lines[0].PriceLevel, "lines": lines,
		"subtotal": formatMoney(result.Subtotal), "lineDiscountTotal": formatMoney(result.LineDiscountTotal),
		"documentPercentDiscount": formatMoney(result.DocumentPercentDiscount), "flatDiscount": formatMoney(result.FlatDiscount),
		"documentDiscountTotal": formatMoney(result.DocumentDiscountTotal), "misc": formatMoney(result.Misc),
		"taxableBase": formatMoney(result.TaxableBase), "taxes": taxes,
		"totalDiscount": formatMoney(result.TotalDiscount), "total": formatMoney(result.Total),
	}
}

func readBusinessDocument(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID string) (documentResponse, error) {
	var document documentResponse
	var pricingJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, kind, status, document_number, tenant_id::text, branch_id::text,
			counter_id::text, operator_id::text, COALESCE(customer_id::text, ''),
			COALESCE(supplier_id::text, ''), COALESCE(source_document_id::text, ''),
			source_document_number, occurred_at::text, COALESCE(godown_id::text, ''),
			reference, remarks, void_reason,
			pricing_result, created_at::text, updated_at::text, version,
			subtotal::text, discount_amount::text, misc_amount::text, tax_amount::text,
			total_amount::text, paid_amount::text, balance_amount::text
		FROM business_documents
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid
	`, documentID, operator.TenantID, operator.BranchID).Scan(
		&document.ID, &document.Kind, &document.Status, &document.DocumentNumber, &document.TenantID,
		&document.BranchID, &document.CounterID, &document.OperatorID, &document.CustomerID, &document.SupplierID,
		&document.SourceDocumentID, &document.SourceDocumentNumber, &document.OccurredAt, &document.GodownID,
		&document.Reference, &document.Remarks, &document.VoidReason, &pricingJSON, &document.CreatedAt, &document.UpdatedAt,
		&document.Version, &document.Totals.Subtotal, &document.Totals.DiscountAmount, &document.Totals.MiscAmount,
		&document.Totals.TaxAmount, &document.Totals.TotalAmount, &document.Totals.PaidAmount, &document.Totals.BalanceAmount,
	)
	if err != nil {
		return documentResponse{}, err
	}
	document.Lines = make([]documentLineResponse, 0)
	document.Pricing = json.RawMessage(pricingJSON)
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text, line_number, item_id::text, item_legacy_id, item_code, item_name, quantity::text,
			unit_price::text, line_gross::text, supplier_discount::text, item_discount::text,
			customer_discount::text, line_total::text, batch_number, COALESCE(expiry_date::text, ''),
			unit_cost::text, tax_amount::text, notes
		FROM business_document_lines
		WHERE tenant_id = $1::uuid AND document_id = $2::uuid
		ORDER BY line_number
	`, operator.TenantID, documentID)
	if err != nil {
		return documentResponse{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var line documentLineResponse
		var quantity, unitPrice, gross, supplierDiscount, itemDiscount, customerDiscount, total string
		if err := rows.Scan(&line.ID, &line.LineNumber, &line.ItemID, &line.ItemLegacyID, &line.ItemCode, &line.ItemName, &quantity,
			&unitPrice, &gross, &supplierDiscount, &itemDiscount, &customerDiscount, &total,
			&line.BatchNumber, &line.ExpiryDate, &line.UnitCost, &line.TaxAmount, &line.Notes); err != nil {
			return documentResponse{}, err
		}
		line.Quantity = quantity
		line.Price = documentPrice{
			PriceTier: 1, UnitPrice: unitPrice, GrossAmount: gross, DiscountPercent: "0.00",
			DiscountAmount: addMoneyStrings(supplierDiscount, itemDiscount, customerDiscount), NetAmount: total,
		}
		line.Stock = documentStockSummary{Direction: "none", Quantity: quantity}
		document.Lines = append(document.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return documentResponse{}, err
	}
	rows.Close()
	for index := range document.Lines {
		allocations, allocationErr := readDocumentAllocations(ctx, tx, operator, document.Lines[index].ID)
		if allocationErr != nil {
			return documentResponse{}, allocationErr
		}
		document.Lines[index].Allocations = allocations
		if len(allocations) > 0 {
			document.Lines[index].Stock = documentStockSummary{Direction: "out", Quantity: document.Lines[index].Quantity}
		}
	}
	finance, financeErr := readDocumentFinanceSummary(ctx, tx, operator, document.ID)
	if financeErr != nil {
		return documentResponse{}, financeErr
	}
	document.Finance = finance
	return document, nil
}

func readDocumentFinanceSummary(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID string) (*documentFinanceSummary, error) {
	var summary documentFinanceSummary
	err := tx.QueryRowContext(ctx, `
		SELECT j.id::text,
		       COALESCE(SUM(l.debit_amount), 0)::text,
		       COALESCE(SUM(l.credit_amount), 0)::text,
		       COALESCE(SUM(l.debit_amount), 0) = COALESCE(SUM(l.credit_amount), 0),
		       COALESCE(p.id::text, ''), COALESCE(p.balance_after::text, '')
		FROM gl_journals j
		LEFT JOIN gl_lines l
		  ON l.tenant_id = j.tenant_id AND l.branch_id = j.branch_id AND l.journal_id = j.id
		LEFT JOIN party_ledger_entries p
		  ON p.tenant_id = j.tenant_id AND p.branch_id = j.branch_id
		 AND p.source_document_id = j.source_document_id
		WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
		  AND j.source_document_id = $3::uuid
		GROUP BY j.id, p.id, p.balance_after
	`, operator.TenantID, operator.BranchID, documentID).Scan(
		&summary.JournalID, &summary.DebitTotal, &summary.CreditTotal,
		&summary.Balanced, &summary.PartyLedgerEntryID, &summary.PartyBalanceAfter,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func readDocumentAllocations(ctx context.Context, tx *sql.Tx, operator *sessionContext, lineID string) ([]documentAllocationResponse, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT a.batch_id::text, b.batch_number, a.quantity::text, a.unit_cost::text
		FROM stock_allocations a
		JOIN stock_batches b
		  ON b.tenant_id = a.tenant_id AND b.branch_id = a.branch_id AND b.id = a.batch_id
		WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid AND a.document_line_id = $3::uuid
		ORDER BY b.received_at, a.batch_id
	`, operator.TenantID, operator.BranchID, lineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]documentAllocationResponse, 0)
	for rows.Next() {
		var allocation documentAllocationResponse
		if err := rows.Scan(&allocation.BatchID, &allocation.BatchNumber, &allocation.Quantity, &allocation.UnitCost); err != nil {
			return nil, err
		}
		result = append(result, allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func nextBusinessDocumentNumber(ctx context.Context, tx *sql.Tx, tenantID, branchID, kind string) (string, error) {
	sequenceName := "document:" + kind
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_sequences (tenant_id, branch_id, sequence_name, next_value)
		VALUES ($1::uuid, $2::uuid, $3, 1)
		ON CONFLICT (tenant_id, branch_id, sequence_name) DO NOTHING
	`, tenantID, branchID, sequenceName); err != nil {
		return "", err
	}
	var value int64
	if err := tx.QueryRowContext(ctx, `
		UPDATE tenant_sequences SET next_value = next_value + 1, updated_at = now()
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND sequence_name = $3
		RETURNING next_value - 1
	`, tenantID, branchID, sequenceName).Scan(&value); err != nil {
		return "", err
	}
	branchPrefix := strings.ReplaceAll(branchID, "-", "")
	if len(branchPrefix) > 8 {
		branchPrefix = branchPrefix[:8]
	}
	return fmt.Sprintf("%s-%s-%06d", strings.ToUpper(strings.ReplaceAll(kind, "-", "")), branchPrefix, value), nil
}

func documentCommandErrorStatus(err error) (int, string) {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "expectedversion"), strings.Contains(message, "cannot be edited"),
		strings.Contains(message, "cannot be posted"), strings.Contains(message, "already void"):
		return http.StatusConflict, "document_revision_conflict"
	case strings.Contains(message, "not found"):
		return http.StatusNotFound, "document_not_found"
	default:
		return http.StatusUnprocessableEntity, "document_rejected"
	}
}

func mustJSON(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func formatQuantityString(value string) string {
	quantity, err := parseQuantity(value)
	if err != nil {
		return value
	}
	return formatQuantity(quantity)
}

func decimalOrZero(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0"
	}
	return strings.TrimSpace(value)
}

func purchaseUnitCost(line pricedDocumentLine) string {
	if strings.TrimSpace(line.request.UnitCost) != "" {
		value, err := parseMoney(line.request.UnitCost)
		if err == nil {
			return formatMoney(value)
		}
	}
	return formatMoney(line.result.ResolvedUnitPrice)
}

func addMoneyStrings(values ...string) string {
	var total int64
	for _, value := range values {
		parsed, err := parseMoney(value)
		if err != nil {
			return "0.00"
		}
		total += int64(parsed)
	}
	return formatMoney(pricing.Money(total))
}

func lineTaxSnapshot(result pricing.Result, lineIndex int) ([]map[string]any, pricing.Money) {
	if len(result.Taxes) == 0 || lineIndex < 0 || lineIndex >= len(result.Lines) {
		return []map[string]any{}, 0
	}
	var netTotal pricing.Money
	for _, line := range result.Lines {
		netTotal += line.Net
	}
	if netTotal == 0 {
		return []map[string]any{}, 0
	}
	taxes := make([]map[string]any, 0, len(result.Taxes))
	var total pricing.Money
	for taxIndex, tax := range result.Taxes {
		allocated := allocateTaxAmount(tax.Amount, result.Lines, netTotal, lineIndex, taxIndex)
		total += allocated
		taxes = append(taxes, map[string]any{
			"kind": taxKindName(tax.Kind), "rate": formatPercent(tax.Rate),
			"inclusive": tax.Inclusive, "base": formatMoney(result.Lines[lineIndex].Net),
			"amount": formatMoney(allocated),
		})
	}
	return taxes, total
}

func allocateTaxAmount(total pricing.Money, lines []pricing.LineResult, netTotal pricing.Money, lineIndex, taxIndex int) pricing.Money {
	if lineIndex == len(lines)-1 {
		var prior pricing.Money
		for index := 0; index < lineIndex; index++ {
			prior += allocateTaxAmount(total, lines, netTotal, index, taxIndex)
		}
		if total >= prior {
			return total - prior
		}
		return 0
	}
	numerator := new(big.Int).Mul(big.NewInt(int64(total)), big.NewInt(int64(lines[lineIndex].Net)))
	quotient := new(big.Int).Quo(numerator, big.NewInt(int64(netTotal)))
	return pricing.Money(quotient.Int64())
}

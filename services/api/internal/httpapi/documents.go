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
	"cash-sale":          {},
	"credit-sale":        {},
	"cash-return":        {},
	"credit-return":      {},
	"open-cash-return":   {},
	"open-credit-return": {},
	"quotation":          {},
	"refused-sale":       {},
	"pack-purchase":      {},
	"loose-purchase":     {},
	"opening-purchase":   {},
	"purchase-return":    {},
	"purchase-order":     {},
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

func isStockAndFinanceSaleKind(kind string) bool {
	return kind == "cash-sale" || kind == "credit-sale"
}

func isSaleReturnDocumentKind(kind string) bool {
	return kind == "cash-return" || kind == "credit-return" || kind == "open-cash-return" || kind == "open-credit-return"
}

func isOpenSaleReturnDocumentKind(kind string) bool {
	return kind == "open-cash-return" || kind == "open-credit-return"
}

func isSourceBoundSaleReturnDocumentKind(kind string) bool {
	return kind == "cash-return" || kind == "credit-return"
}

func isSourceBoundPurchaseReturnDocumentKind(kind string) bool {
	return kind == "purchase-return"
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
	CreditDays              string                  `json:"creditDays,omitempty"`
	DueDate                 string                  `json:"dueDate,omitempty"`
	Lines                   []documentLineRequest   `json:"lines"`
	PriceLevel              int                     `json:"priceLevel,omitempty"`
	FlatDiscountAmount      string                  `json:"flatDiscountAmount,omitempty"`
	MiscAmount              *string                 `json:"miscAmount,omitempty"`
	DocumentDiscountPercent string                  `json:"documentDiscountPercent,omitempty"`
	Pricing                 *documentPricingRequest `json:"pricing,omitempty"`
	Payment                 *documentPaymentRequest `json:"payment,omitempty"`
}

type documentLineRequest struct {
	LineNumber      int                           `json:"lineNumber"`
	ItemID          string                        `json:"itemId"`
	SourceLineID    string                        `json:"sourceLineId,omitempty"`
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
	ID                   string                   `json:"id"`
	Kind                 string                   `json:"kind"`
	Status               string                   `json:"status"`
	DocumentNumber       string                   `json:"documentNumber"`
	TenantID             string                   `json:"tenantId"`
	BranchID             string                   `json:"branchId"`
	CounterID            string                   `json:"counterId"`
	OperatorID           string                   `json:"operatorId"`
	CustomerID           string                   `json:"customerId,omitempty"`
	SupplierID           string                   `json:"supplierId,omitempty"`
	SourceDocumentID     string                   `json:"sourceDocumentId,omitempty"`
	SourceDocumentNumber string                   `json:"sourceDocumentNumber,omitempty"`
	OccurredAt           string                   `json:"occurredAt"`
	GodownID             string                   `json:"godownId,omitempty"`
	Reference            string                   `json:"reference,omitempty"`
	Remarks              string                   `json:"remarks,omitempty"`
	CreditDays           string                   `json:"creditDays,omitempty"`
	DueDate              string                   `json:"dueDate,omitempty"`
	VoidReason           string                   `json:"voidReason,omitempty"`
	DeletedAt            string                   `json:"deletedAt,omitempty"`
	Lines                []documentLineResponse   `json:"lines"`
	Totals               documentTotals           `json:"totals"`
	Payment              *documentPaymentResponse `json:"payment,omitempty"`
	Pricing              json.RawMessage          `json:"pricing,omitempty"`
	Finance              *documentFinanceSummary  `json:"finance,omitempty"`
	CreatedAt            string                   `json:"createdAt"`
	UpdatedAt            string                   `json:"updatedAt"`
	Version              int64                    `json:"version"`
}

type documentLineResponse struct {
	ID           string                       `json:"id"`
	LineNumber   int                          `json:"lineNumber"`
	ItemID       string                       `json:"itemId"`
	SourceLineID string                       `json:"sourceLineId,omitempty"`
	ItemLegacyID string                       `json:"itemLegacyId,omitempty"`
	ItemCode     string                       `json:"itemCode"`
	ItemName     string                       `json:"itemName"`
	Quantity     string                       `json:"quantity"`
	Price        documentPrice                `json:"price"`
	Tax          documentTaxSummary           `json:"tax"`
	LineTotal    string                       `json:"lineTotal"`
	Stock        documentStockSummary         `json:"stock"`
	Allocations  []documentAllocationResponse `json:"allocations,omitempty"`
	BatchNumber  string                       `json:"batchNumber,omitempty"`
	ExpiryDate   string                       `json:"expiryDate,omitempty"`
	UnitCost     string                       `json:"unitCost,omitempty"`
	TaxAmount    string                       `json:"taxAmount,omitempty"`
	Notes        string                       `json:"notes,omitempty"`
}

type documentTaxLine struct {
	Kind          string `json:"kind,omitempty"`
	Rate          string `json:"rate"`
	TaxableAmount string `json:"taxableAmount"`
	Amount        string `json:"amount"`
}

type documentTaxSummary struct {
	TaxableAmount string            `json:"taxableAmount"`
	Amount        string            `json:"amount"`
	Lines         []documentTaxLine `json:"lines"`
}

type documentLinePricingSnapshot struct {
	DiscountPercent      string                `json:"discountPercent"`
	CustomerDiscountRate string                `json:"customerDiscountRate"`
	Taxes                []documentTaxSnapshot `json:"taxes"`
}

type documentTaxSnapshot struct {
	Kind          string `json:"kind"`
	Rate          string `json:"rate"`
	Base          string `json:"base"`
	TaxableAmount string `json:"taxableAmount"`
	Amount        string `json:"amount"`
}

func isZeroDecimal(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	parsed, err := parseMoney(value)
	return err == nil && parsed == 0
}

func firstNonZeroDecimal(values ...string) string {
	for _, value := range values {
		if !isZeroDecimal(value) {
			return value
		}
	}
	return "0.00"
}

func normalizeDocumentPayment(kind string, input *documentPaymentRequest, total pricing.Money) (*documentPaymentResponse, pricing.Money, pricing.Money, error) {
	if kind != "cash-sale" {
		if input != nil && (strings.TrimSpace(input.Mode) != "" || strings.TrimSpace(input.Received) != "" || strings.TrimSpace(input.Tendered) != "" || strings.TrimSpace(input.Change) != "" || strings.TrimSpace(input.AccountCode) != "") {
			return nil, 0, total, errors.New("payment fields are only valid for cash-sale")
		}
		return nil, 0, total, nil
	}
	payment := &documentPaymentResponse{Mode: "cash"}
	if input != nil && strings.TrimSpace(input.Mode) != "" {
		payment.Mode = strings.ToLower(strings.TrimSpace(input.Mode))
	}
	if payment.Mode != "cash" {
		return nil, 0, total, errors.New("cash-sale payment mode must be cash")
	}
	received := total
	var err error
	if input != nil && strings.TrimSpace(input.Received) != "" {
		received, err = parseMoney(input.Received)
		if err != nil {
			return nil, 0, total, fmt.Errorf("cash received: %w", err)
		}
	}
	if received != total {
		return nil, 0, total, errors.New("cash-sale cash received must equal the calculated total")
	}
	tendered := received
	if input != nil && strings.TrimSpace(input.Tendered) != "" {
		tendered, err = parseMoney(input.Tendered)
		if err != nil {
			return nil, 0, total, fmt.Errorf("cash tendered: %w", err)
		}
	}
	if tendered < received {
		return nil, 0, total, errors.New("cash tendered cannot be less than the calculated total")
	}
	change := tendered - received
	if input != nil && strings.TrimSpace(input.Change) != "" {
		declaredChange, parseErr := parseMoney(input.Change)
		if parseErr != nil {
			return nil, 0, total, fmt.Errorf("cash back: %w", parseErr)
		}
		if declaredChange != change {
			return nil, 0, total, errors.New("cash back must equal cash tendered minus the calculated total")
		}
	}
	payment.Received = formatMoney(received)
	payment.Tendered = formatMoney(tendered)
	payment.Change = formatMoney(change)
	payment.AccountCode = strings.TrimSpace(inputAccountCode(input))
	return payment, received, total - received, nil
}

func normalizeDocumentCreditDays(kind, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !isPurchaseDocumentKind(kind) {
		if trimmed != "" {
			return "", errors.New("creditDays are only valid for purchase documents")
		}
		return "", nil
	}
	if trimmed == "" {
		return "", nil
	}
	days, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || days < -36500 || days > 36500 {
		return "", errors.New("creditDays must be a whole number between -36500 and 36500")
	}
	return strconv.FormatInt(days, 10), nil
}

func normalizeDocumentDueDate(kind, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if kind != "credit-sale" {
		if trimmed != "" {
			return "", errors.New("dueDate is only valid for credit-sale documents")
		}
		return "", nil
	}
	if trimmed == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return "", errors.New("dueDate must be a valid ISO calendar date")
	}
	return trimmed, nil
}

func inputAccountCode(input *documentPaymentRequest) string {
	if input == nil {
		return ""
	}
	return input.AccountCode
}

func withPaymentPricingSnapshot(raw []byte, payment *documentPaymentResponse) ([]byte, error) {
	if payment == nil {
		return raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("posted pricing result is invalid for payment snapshot")
	}
	payload["payment"] = payment
	return json.Marshal(payload)
}

func withCreditDaysPricingSnapshot(raw []byte, creditDays string) ([]byte, error) {
	if strings.TrimSpace(creditDays) == "" {
		return raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("posted pricing result is invalid for credit-term snapshot")
	}
	payload["creditDays"] = creditDays
	return json.Marshal(payload)
}

func withDueDatePricingSnapshot(raw []byte, dueDate string) ([]byte, error) {
	if strings.TrimSpace(dueDate) == "" {
		return raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("posted pricing result is invalid for due-date snapshot")
	}
	payload["dueDate"] = dueDate
	return json.Marshal(payload)
}

func legacyPayloadString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func legacyPayloadObject(raw []byte) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func legacyPayloadField(raw []byte, key string) string {
	payload := legacyPayloadObject(raw)
	if payload == nil {
		return ""
	}
	return legacyPayloadString(payload[key])
}

func legacyPaymentFromPayload(kind string, raw []byte, total string) *documentPaymentResponse {
	if kind != "cash-sale" || len(raw) == 0 {
		return nil
	}
	payload := legacyPayloadObject(raw)
	if payload == nil {
		return nil
	}
	value := func(key string) string { return legacyPayloadString(payload[key]) }
	mode, received, tendered, change, accountCode := value("PaymentMode"), value("CashReceived"), value("CashTendered"), value("CashBack"), value("PaymentAccCode")
	if mode == "" && received == "" && tendered == "" && change == "" && accountCode == "" {
		return nil
	}
	if mode == "" {
		mode = "cash"
	} else {
		mode = strings.ToLower(mode)
	}
	if received == "" {
		received = total
	}
	if tendered == "" {
		tendered = received
	}
	if change == "" {
		receivedAmount, receivedErr := parseMoney(received)
		tenderedAmount, tenderedErr := parseMoney(tendered)
		if receivedErr == nil && tenderedErr == nil && tenderedAmount >= receivedAmount {
			change = formatMoney(tenderedAmount - receivedAmount)
		}
	}
	return &documentPaymentResponse{Mode: mode, Received: received, Tendered: tendered, Change: change, AccountCode: accountCode}
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

type documentPaymentRequest struct {
	Mode        string `json:"mode,omitempty"`
	Received    string `json:"received,omitempty"`
	Tendered    string `json:"tendered,omitempty"`
	Change      string `json:"change,omitempty"`
	AccountCode string `json:"accountCode,omitempty"`
}

type documentPaymentResponse struct {
	Mode        string `json:"mode"`
	Received    string `json:"received"`
	Tendered    string `json:"tendered"`
	Change      string `json:"change"`
	AccountCode string `json:"accountCode,omitempty"`
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
		writeProblem(w, http.StatusBadRequest, "invalid_document_kind", "Invalid business document kind", "The requested business document family is not supported.")
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
	if request.Document != nil && strings.TrimSpace(request.Document.GodownID) != "" &&
		!s.requireScope(r, w, operator, "godown", strings.TrimSpace(request.Document.GodownID)) {
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
	if (request.Action == "void" || request.Action == "delete") && request.Document == nil {
		var godownID string
		err := tx.QueryRowContext(r.Context(), `
			SELECT COALESCE(godown_id::text, '')
			FROM business_documents
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4
		`, request.DocumentID, operator.TenantID, operator.BranchID, request.Kind).Scan(&godownID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusServiceUnavailable, "document_scope_read_failed", "Document scope could not be read", "The canonical document godown scope could not be resolved.")
			return
		}
		if err == nil && godownID != "" && !scopeAllowedAtEnforcementBoundary(operator, "godown", godownID) {
			_ = tx.Rollback()
			s.requireScope(r, w, operator, "godown", godownID)
			return
		}
	}

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
	} else if request.Action == "delete" {
		documentID, response, err = s.deleteBusinessDocument(r.Context(), tx, operator, request)
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
		} else if isStockAndFinanceSaleKind(response.Document.Kind) {
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
		} else if isSaleReturnDocumentKind(response.Document.Kind) {
			stockErr := error(nil)
			if isOpenSaleReturnDocumentKind(response.Document.Kind) {
				stockErr = projectPostedOpenSaleReturnStock(r.Context(), tx, operator, request.Document, response.Document, eventID)
			} else {
				stockErr = projectPostedSaleReturnStock(r.Context(), tx, operator, request.Document, response.Document, eventID)
			}
			if stockErr != nil {
				status, code := stockCommandErrorStatus(stockErr)
				writeProblem(w, status, code, "Document stock rejected", stockErr.Error())
				return
			}
			if err := projectPostedSaleReturnFinance(r.Context(), tx, operator, response.Document, eventID); err != nil {
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
	if request.Action == "void" && response.Document.Status == "void" && isReversiblePostedDocumentKind(response.Document.Kind) {
		if err := projectPostedDocumentVoid(r.Context(), tx, operator, response.Document, eventID, request.Reason, request.OccurredAt); err != nil {
			status, code := documentCommandErrorStatus(err)
			writeProblem(w, status, code, "Document void rejected", err.Error())
			return
		}
		response.Document, err = readBusinessDocument(r.Context(), tx, operator, documentID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "document_read_failed", "Document void failed", "The voided document could not be re-read after compensating reversal.")
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

// documentDetail is the read side of the canonical document lifecycle. It is
// intentionally separate from the command route so history/population flows
// can hydrate the complete document without granting mutation semantics.
func (s *Server) documentDetail(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	documentID := strings.TrimSpace(r.PathValue("id"))
	if !documentUUIDPattern.MatchString(documentID) {
		writeProblem(w, http.StatusBadRequest, "invalid_document_id", "Invalid document identifier", "The document identifier must be a UUID.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The business document store could not be opened.")
		return
	}
	defer tx.Rollback()
	var kind string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT kind
		FROM business_documents
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid
		  AND deleted_at IS NULL
	`, documentID, operator.TenantID, operator.BranchID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "document_not_found", "Document not found", "The document is outside the authenticated tenant and branch scope.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "document_read_failed", "Document could not be read", "The canonical document identity could not be resolved.")
		return
	}
	permission := "sales.read"
	if isPurchaseDocumentKind(kind) {
		permission = "purchases.read"
	}
	if !hasPermission(operator, permission) {
		s.auditAuthorizationDenied(r, operator, permission)
		writeProblem(w, http.StatusForbidden, "permission_required", "Permission required", "The authenticated operator does not have permission to read this document.")
		return
	}
	document, err := readBusinessDocument(r.Context(), tx, operator, documentID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_read_failed", "Document could not be read", "The canonical document detail could not be assembled.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "document_read_failed", "Document could not be read", "The canonical document read transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, document)
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
	payment, paidAmount, balanceAmount, err := normalizeDocumentPayment(command.Kind, draft.Payment, priced.result.Total)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	creditDays, err := normalizeDocumentCreditDays(command.Kind, draft.CreditDays)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	dueDate, err := normalizeDocumentDueDate(command.Kind, draft.DueDate)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	pricingJSON, err := withPaymentPricingSnapshot(priced.pricingJSON, payment)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	pricingJSON, err = withCreditDaysPricingSnapshot(pricingJSON, creditDays)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	pricingJSON, err = withDueDatePricingSnapshot(pricingJSON, dueDate)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	status := "draft"
	if command.Action == "post" || command.Action == "save-and-post" {
		status = "posted"
	}
	documentID := strings.TrimSpace(draft.ID)
	var version int64
	var currentStatus, deletedAt string
	if documentID != "" {
		if command.ExpectedVersion == nil {
			return "", documentCommandResponse{}, errors.New("expectedVersion is required when updating an existing document")
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT version, status, COALESCE(deleted_at::text, '')
			FROM business_documents
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4
			FOR UPDATE
		`, documentID, operator.TenantID, operator.BranchID, command.Kind).Scan(&version, &currentStatus, &deletedAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", documentCommandResponse{}, errors.New("document was not found in the authenticated tenant and branch")
			}
			return "", documentCommandResponse{}, err
		}
		if version != *command.ExpectedVersion {
			return "", documentCommandResponse{}, fmt.Errorf("expectedVersion %d does not match current document version %d", *command.ExpectedVersion, version)
		}
		if deletedAt != "" {
			return "", documentCommandResponse{}, errors.New("document has already been deleted")
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
	} else if supplierID != "" || (sourceDocumentID != "" && !isSaleReturnDocumentKind(command.Kind)) {
		return "", documentCommandResponse{}, errors.New("supplier and source document fields are only valid for purchases")
	}
	if isSaleReturnDocumentKind(command.Kind) {
		if sourceDocumentID != "" && !documentUUIDPattern.MatchString(sourceDocumentID) {
			return "", documentCommandResponse{}, errors.New("sourceDocumentId must be a UUID")
		}
		if isOpenSaleReturnDocumentKind(command.Kind) && sourceDocumentID != "" {
			return "", documentCommandResponse{}, errors.New("open sale returns cannot specify a sourceDocumentId")
		}
		if command.Action == "post" || command.Action == "save-and-post" {
			if isSourceBoundSaleReturnDocumentKind(command.Kind) && sourceDocumentID == "" {
				return "", documentCommandResponse{}, errors.New("sale-return posting requires sourceDocumentId")
			}
			if strings.TrimSpace(draft.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(draft.GodownID)) {
				return "", documentCommandResponse{}, errors.New("sale-return posting requires an explicit godownId")
			}
		}
	}
	if command.Kind == "credit-sale" || command.Kind == "credit-return" || command.Kind == "open-credit-return" {
		if customerID == "" || !documentUUIDPattern.MatchString(customerID) {
			return "", documentCommandResponse{}, errors.New("credit sale/return requires a canonical customerId")
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
				NULLIF($9, '')::uuid, NULLIF($10, '')::uuid, NULLIF($11, '')::uuid, $12, NULLIF($13, '')::uuid, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24::jsonb, $25::jsonb)
			RETURNING id::text
		`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, command.Kind, documentNumber, status,
			draft.OccurredAt, customerID, supplierID, sourceDocumentID, strings.TrimSpace(draft.SourceDocumentNumber),
			strings.TrimSpace(draft.GodownID), strings.TrimSpace(draft.Reference), strings.TrimSpace(draft.Remarks), priced.request.PriceLevel,
			formatMoney(priced.result.Subtotal+priced.result.LineDiscountTotal), formatMoney(priced.result.TotalDiscount),
			formatMoney(priced.result.Misc), formatTotalTax(priced.result), formatMoney(priced.result.Total),
			formatMoney(paidAmount), formatMoney(balanceAmount), mustJSON(priced.request), pricingJSON).Scan(&documentID)
		if err != nil {
			return "", documentCommandResponse{}, err
		}
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE business_documents
			SET status = $1, occurred_at = $2::timestamptz, customer_id = NULLIF($3, '')::uuid,
				supplier_id = NULLIF($4, '')::uuid, source_document_id = NULLIF($5, '')::uuid,
				source_document_number = $6, godown_id = NULLIF($7, '')::uuid, reference = $8, remarks = $9, price_level = $10, subtotal = $11, discount_amount = $12,
				misc_amount = $13, tax_amount = $14, total_amount = $15, paid_amount = $16,
				balance_amount = $17, pricing_inputs = $18::jsonb, pricing_result = $19::jsonb,
				version = version + 1, updated_at = now()
			WHERE id = $20::uuid AND tenant_id = $21::uuid AND branch_id = $22::uuid
		`, status, draft.OccurredAt, customerID, supplierID, sourceDocumentID, strings.TrimSpace(draft.SourceDocumentNumber),
			strings.TrimSpace(draft.GodownID), strings.TrimSpace(draft.Reference), strings.TrimSpace(draft.Remarks),
			priced.request.PriceLevel, formatMoney(priced.result.Subtotal+priced.result.LineDiscountTotal),
			formatMoney(priced.result.TotalDiscount), formatMoney(priced.result.Misc), formatTotalTax(priced.result),
			formatMoney(priced.result.Total), formatMoney(paidAmount), formatMoney(balanceAmount), mustJSON(priced.request), pricingJSON,
			documentID, operator.TenantID, operator.BranchID)
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
			"discountPercent":        formatPercentOrZero(line.request.DiscountPercent),
			"customerDiscount":       formatMoney(line.result.CustomerDiscount),
			"customerDiscountRate":   formatPercent(line.result.CustomerDiscountRate),
			"customerDiscountSource": customerDiscountSourceName(line.result.CustomerDiscountSource),
			"taxes":                  lineTaxes,
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO business_document_lines
				(tenant_id, branch_id, document_id, line_number, item_id, source_line_id, item_legacy_id, item_code, item_name,
				 quantity, unit_price, line_gross, supplier_discount, item_discount, customer_discount, line_total,
				 pricing, batch_number, expiry_date, unit_cost, gst_rate, pct_rate, advance_tax_rate, tax_amount, notes)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, NULLIF($6, '')::uuid, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			        $17::jsonb, $18, NULLIF($19, '')::date, $20, $21, $22, $23, $24, $25)
		`, operator.TenantID, operator.BranchID, documentID, index+1, line.itemID, line.request.SourceLineID,
			line.legacyID, line.code, line.name, formatQuantityString(line.request.Quantity),
			formatMoney(line.result.ResolvedUnitPrice), formatMoney(line.result.LineGross),
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
	var documentID, status, deletedAt string
	var version int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, status, version, COALESCE(deleted_at::text, '') FROM business_documents
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4
		FOR UPDATE
	`, command.DocumentID, operator.TenantID, operator.BranchID, command.Kind).Scan(&documentID, &status, &version, &deletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", documentCommandResponse{}, errors.New("document was not found in the authenticated tenant and branch")
		}
		return "", documentCommandResponse{}, err
	}
	if version != *command.ExpectedVersion {
		return "", documentCommandResponse{}, fmt.Errorf("expectedVersion %d does not match current document version %d", *command.ExpectedVersion, version)
	}
	if deletedAt != "" {
		return "", documentCommandResponse{}, errors.New("document is already deleted")
	}
	if status == "void" {
		return "", documentCommandResponse{}, errors.New("document is already void")
	}
	if status == "posted" && isReversiblePostedDocumentKind(command.Kind) {
		var hasStockProjection, hasFinanceProjection bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM stock_ledger
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND source_document_id = $3::uuid
			), EXISTS (
				SELECT 1 FROM gl_journals
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND source_document_id = $3::uuid
			)
		`, operator.TenantID, operator.BranchID, documentID).Scan(&hasStockProjection, &hasFinanceProjection); err != nil {
			return "", documentCommandResponse{}, err
		}
		if !hasStockProjection || !hasFinanceProjection {
			return "", documentCommandResponse{}, errors.New("posted document cannot be voided without completed stock and finance projections")
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE business_documents SET status = 'void', void_reason = $3, updated_at = now(), version = version + 1
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $4::uuid AND kind = $5 AND deleted_at IS NULL
	`, documentID, operator.TenantID, strings.TrimSpace(command.Reason), operator.BranchID, command.Kind); err != nil {
		return "", documentCommandResponse{}, err
	}
	response, err := readBusinessDocument(ctx, tx, operator, documentID)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	return documentID, documentCommandResponse{Document: response}, nil
}

func (s *Server) deleteBusinessDocument(ctx context.Context, tx *sql.Tx, operator *sessionContext, command documentCommandRequest) (string, documentCommandResponse, error) {
	if command.ExpectedVersion == nil {
		return "", documentCommandResponse{}, errors.New("expectedVersion is required when deleting a document")
	}
	var documentID, status, deletedAt string
	var version int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, status, version, COALESCE(deleted_at::text, '') FROM business_documents
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4
		FOR UPDATE
	`, command.DocumentID, operator.TenantID, operator.BranchID, command.Kind).Scan(&documentID, &status, &version, &deletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", documentCommandResponse{}, errors.New("document was not found in the authenticated tenant and branch")
		}
		return "", documentCommandResponse{}, err
	}
	if version != *command.ExpectedVersion {
		return "", documentCommandResponse{}, fmt.Errorf("expectedVersion %d does not match current document version %d", *command.ExpectedVersion, version)
	}
	if deletedAt != "" {
		return "", documentCommandResponse{}, errors.New("document is already deleted")
	}
	if status != "draft" {
		return "", documentCommandResponse{}, fmt.Errorf("document with status %q cannot be deleted; only draft documents can be deleted", status)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE business_documents
		SET deleted_at = now(), updated_at = now(), version = version + 1
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND kind = $4 AND deleted_at IS NULL
	`, documentID, operator.TenantID, operator.BranchID, command.Kind)
	if err != nil {
		return "", documentCommandResponse{}, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		if err != nil {
			return "", documentCommandResponse{}, err
		}
		return "", documentCommandResponse{}, errors.New("document was not deleted because its state changed")
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
	supplierSchemeCache := make(map[string]*pricingPreviewSupplierScheme)
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
		supplierScheme := line.SupplierScheme
		if supplierScheme == nil && isPurchaseDocumentKind(draft.Kind) && strings.TrimSpace(draft.SupplierID) != "" {
			cacheKey := item.ID + "|" + strings.TrimSpace(draft.SupplierID)
			if cached, ok := supplierSchemeCache[cacheKey]; ok {
				supplierScheme = cached
			} else {
				supplierScheme, err = resolveCanonicalSupplierScheme(ctx, tx, operator.TenantID, item.ID, draft.SupplierID)
				if err != nil {
					return pricedDocument{}, fmt.Errorf("line %d supplier scheme: %w", index+1, err)
				}
				supplierSchemeCache[cacheKey] = supplierScheme
			}
		}
		preview.Lines = append(preview.Lines, pricingPreviewLine{
			ID:                  item.ID,
			Quantity:            line.Quantity,
			Prices:              tiers,
			ItemDiscountPercent: line.DiscountPercent,
			SupplierScheme:      supplierScheme,
		})
		line.SupplierScheme = supplierScheme
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

func resolveCanonicalSupplierScheme(ctx context.Context, tx *sql.Tx, tenantID, itemID, supplierID string) (*pricingPreviewSupplierScheme, error) {
	var discountPercent, qualifyingQuantity, bonusQuantity string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(s.discount_percent::text, ''), COALESCE(s.quantity::text, ''), COALESCE(s.bonus::text, '')
		FROM item_suppliers s
		WHERE s.tenant_id = $1::uuid AND s.item_id = $2::uuid
		  AND (s.supplier_id = $3::uuid OR s.legacy_supplier_id = (
			SELECT legacy_id FROM master_parties
			WHERE tenant_id = $1::uuid AND id = $3::uuid AND party_type = 'supplier'
		  ))
		ORDER BY s.priority NULLS LAST, s.legacy_supplier_id
		LIMIT 1
	`, tenantID, itemID, supplierID).Scan(&discountPercent, &qualifyingQuantity, &bonusQuantity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(discountPercent) == "" && strings.TrimSpace(qualifyingQuantity) == "" && strings.TrimSpace(bonusQuantity) == "" {
		return nil, nil
	}
	discount, err := parsePercent(discountPercent)
	if err != nil {
		return nil, fmt.Errorf("discount percent: %w", err)
	}
	qualifying, err := parseNonNegativeQuantity(qualifyingQuantity)
	if err != nil {
		return nil, fmt.Errorf("qualifying quantity: %w", err)
	}
	bonus, err := parseNonNegativeQuantity(bonusQuantity)
	if err != nil {
		return nil, fmt.Errorf("bonus quantity: %w", err)
	}
	if discount == 0 && bonus == 0 {
		return nil, nil
	}
	if bonus > 0 && qualifying == 0 {
		// A source row with a bonus but no qualifying quantity is retained in
		// the migration tables, but cannot be promoted into a billable scheme
		// without inventing a legacy rule.
		return nil, nil
	}
	return &pricingPreviewSupplierScheme{
		DiscountPercent:    formatPercent(discount),
		QualifyingQuantity: formatQuantity(qualifying),
		BonusQuantity:      formatQuantity(bonus),
	}, nil
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
			if isSourceBoundSaleReturnDocumentKind(kind) || isSourceBoundPurchaseReturnDocumentKind(kind) {
				if strings.TrimSpace(line.SourceLineID) != "" &&
					!documentUUIDPattern.MatchString(strings.TrimSpace(line.SourceLineID)) {
					return fmt.Errorf("line %d sourceLineId must be a UUID", index+1)
				}
				if (request.Action == "post" || request.Action == "save-and-post") &&
					strings.TrimSpace(line.SourceLineID) == "" {
					return fmt.Errorf("line %d sourceLineId is required when posting a %s", index+1, kind)
				}
			} else if strings.TrimSpace(line.SourceLineID) != "" {
				return fmt.Errorf("line %d sourceLineId is only valid for a source-bound sale or purchase return", index+1)
			}
			if isOpenSaleReturnDocumentKind(kind) &&
				(request.Action == "post" || request.Action == "save-and-post") &&
				strings.TrimSpace(line.BatchNumber) == "" {
				return fmt.Errorf("line %d batchNumber is required when posting an open sale return", index+1)
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
	case "delete":
		if !documentUUIDPattern.MatchString(strings.TrimSpace(request.DocumentID)) {
			return errors.New("documentId must be a UUID for delete")
		}
		if strings.TrimSpace(request.Reason) == "" {
			return errors.New("reason is required for delete")
		}
		if request.ExpectedVersion == nil || *request.ExpectedVersion < 1 {
			return errors.New("expectedVersion must be positive for delete")
		}
	default:
		return errors.New("action must be save, post, save-and-post, void, or delete")
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
	var legacyPayload []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, kind, status, document_number, tenant_id::text, branch_id::text,
			counter_id::text, operator_id::text, COALESCE(customer_id::text, ''),
			COALESCE(supplier_id::text, ''), COALESCE(source_document_id::text, ''),
			source_document_number, occurred_at::text, COALESCE(godown_id::text, ''),
			reference, remarks, void_reason, COALESCE(deleted_at::text, ''),
			pricing_result, legacy_payload, created_at::text, updated_at::text, version,
			subtotal::text, discount_amount::text, misc_amount::text, tax_amount::text,
			total_amount::text, paid_amount::text, balance_amount::text
		FROM business_documents
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid
	`, documentID, operator.TenantID, operator.BranchID).Scan(
		&document.ID, &document.Kind, &document.Status, &document.DocumentNumber, &document.TenantID,
		&document.BranchID, &document.CounterID, &document.OperatorID, &document.CustomerID, &document.SupplierID,
		&document.SourceDocumentID, &document.SourceDocumentNumber, &document.OccurredAt, &document.GodownID,
		&document.Reference, &document.Remarks, &document.VoidReason, &document.DeletedAt, &pricingJSON, &legacyPayload, &document.CreatedAt, &document.UpdatedAt,
		&document.Version, &document.Totals.Subtotal, &document.Totals.DiscountAmount, &document.Totals.MiscAmount,
		&document.Totals.TaxAmount, &document.Totals.TotalAmount, &document.Totals.PaidAmount, &document.Totals.BalanceAmount,
	)
	if err != nil {
		return documentResponse{}, err
	}
	document.Lines = make([]documentLineResponse, 0)
	document.Pricing = json.RawMessage(pricingJSON)
	var pricingSnapshot struct {
		Payment    *documentPaymentResponse `json:"payment"`
		CreditDays string                   `json:"creditDays"`
		DueDate    string                   `json:"dueDate"`
	}
	if err := json.Unmarshal(pricingJSON, &pricingSnapshot); err == nil {
		document.Payment = pricingSnapshot.Payment
		document.CreditDays = pricingSnapshot.CreditDays
		document.DueDate = pricingSnapshot.DueDate
	}
	if document.CreditDays == "" && isPurchaseDocumentKind(document.Kind) {
		document.CreditDays = legacyPayloadField(legacyPayload, "CreditDays")
	}
	if document.DueDate == "" && document.Kind == "credit-sale" {
		document.DueDate = legacyPayloadField(legacyPayload, "DueDate")
	}
	if document.Payment == nil {
		document.Payment = legacyPaymentFromPayload(document.Kind, legacyPayload, document.Totals.TotalAmount)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text, line_number, item_id::text, COALESCE(source_line_id::text, ''), item_legacy_id, item_code, item_name, quantity::text,
			unit_price::text, line_gross::text, supplier_discount::text, item_discount::text,
			customer_discount::text, line_total::text, pricing, gst_rate::text, pct_rate::text, advance_tax_rate::text,
			batch_number, COALESCE(expiry_date::text, ''), unit_cost::text, tax_amount::text, notes
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
		var linePricingJSON []byte
		var gstRate, pctRate, advanceTaxRate string
		if err := rows.Scan(&line.ID, &line.LineNumber, &line.ItemID, &line.SourceLineID, &line.ItemLegacyID, &line.ItemCode, &line.ItemName, &quantity,
			&unitPrice, &gross, &supplierDiscount, &itemDiscount, &customerDiscount, &total,
			&linePricingJSON, &gstRate, &pctRate, &advanceTaxRate,
			&line.BatchNumber, &line.ExpiryDate, &line.UnitCost, &line.TaxAmount, &line.Notes); err != nil {
			return documentResponse{}, err
		}
		var linePricing documentLinePricingSnapshot
		_ = json.Unmarshal(linePricingJSON, &linePricing)
		line.Quantity = quantity
		line.Price = documentPrice{
			PriceTier: 1, UnitPrice: unitPrice, GrossAmount: gross,
			DiscountPercent: firstNonZeroDecimal(linePricing.DiscountPercent, linePricing.CustomerDiscountRate),
			DiscountAmount:  addMoneyStrings(supplierDiscount, itemDiscount, customerDiscount), NetAmount: total,
		}
		taxLines := make([]documentTaxLine, 0, len(linePricing.Taxes))
		for _, tax := range linePricing.Taxes {
			taxLines = append(taxLines, documentTaxLine{
				Kind: tax.Kind, Rate: tax.Rate,
				TaxableAmount: firstNonZeroDecimal(tax.TaxableAmount, tax.Base, total),
				Amount:        tax.Amount,
			})
		}
		line.Tax = documentTaxSummary{TaxableAmount: total, Amount: line.TaxAmount, Lines: taxLines}
		if len(line.Tax.Lines) == 0 {
			for _, tax := range []struct {
				kind, rate string
			}{
				{kind: "gst", rate: gstRate},
				{kind: "pct", rate: pctRate},
				{kind: "advance_tax", rate: advanceTaxRate},
			} {
				if isZeroDecimal(tax.rate) {
					continue
				}
				line.Tax.Lines = append(line.Tax.Lines, documentTaxLine{Kind: tax.kind, Rate: tax.rate, TaxableAmount: total, Amount: line.TaxAmount})
			}
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
		       j.total_debit::text,
		       j.total_credit::text,
		       j.total_debit = j.total_credit,
		       COALESCE(p.id::text, ''), COALESCE(p.balance_after::text, '')
		FROM gl_journals j
		LEFT JOIN LATERAL (
			SELECT id, balance_after
			FROM party_ledger_entries
			WHERE tenant_id = j.tenant_id AND branch_id = j.branch_id
			  AND source_document_id = j.source_document_id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) p ON true
		WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
		  AND j.source_document_id = $3::uuid
		ORDER BY j.created_at DESC, j.id DESC
		LIMIT 1
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

func formatPercentOrZero(value string) string {
	parsed, err := parsePercent(value)
	if err != nil {
		return decimalOrZero(value)
	}
	return formatPercent(parsed)
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
			"kind": taxKindName(tax.Kind), "rate": formatStoredTaxRate(tax.Rate),
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

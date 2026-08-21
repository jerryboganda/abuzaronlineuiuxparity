package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

type voucherPostRequest struct {
	CommandID      string             `json:"commandId"`
	IdempotencyKey string             `json:"idempotencyKey"`
	CategoryCode   string             `json:"categoryCode"`
	PartyID        string             `json:"partyId"`
	Amount         string             `json:"amount"`
	Description    string             `json:"description"`
	OccurredAt     string             `json:"occurredAt"`
	Lines          []voucherLineDraft `json:"lines"`
}

type voucherLineDraft struct {
	AccountID string `json:"accountId"`
	Debit     string `json:"debit"`
	Credit    string `json:"credit"`
	Memo      string `json:"memo"`
}

func voucherDocumentKind(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "receipt":
		return "receipt-voucher"
	case "payment":
		return "payment-voucher"
	case "journal":
		return "journal-voucher"
	default:
		return ""
	}
}

func parseVoucherPostRequest(payload []byte) (voucherPostRequest, error) {
	var request voucherPostRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return request, errors.New("voucher payload is invalid")
	}
	request.CommandID = strings.TrimSpace(request.CommandID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.CategoryCode = strings.ToLower(strings.TrimSpace(request.CategoryCode))
	request.PartyID = strings.TrimSpace(request.PartyID)
	request.Amount = strings.TrimSpace(request.Amount)
	request.Description = strings.TrimSpace(request.Description)
	request.OccurredAt = strings.TrimSpace(request.OccurredAt)
	if voucherDocumentKind(request.CategoryCode) == "" {
		return request, errors.New("categoryCode must be receipt, payment, or journal")
	}
	if request.OccurredAt == "" {
		request.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	} else if _, err := time.Parse(time.RFC3339, request.OccurredAt); err != nil {
		return request, errors.New("occurredAt must be RFC3339")
	}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = fmt.Sprintf("voucher-%s-%s-%s", request.CategoryCode, request.PartyID, request.OccurredAt)
	}
	if request.CategoryCode == "journal" {
		if len(request.Lines) < 2 {
			return request, errors.New("journal voucher requires at least two lines")
		}
		var debitTotal, creditTotal pricing.Money
		for index, line := range request.Lines {
			request.Lines[index].AccountID = strings.TrimSpace(line.AccountID)
			request.Lines[index].Debit = strings.TrimSpace(line.Debit)
			request.Lines[index].Credit = strings.TrimSpace(line.Credit)
			request.Lines[index].Memo = strings.TrimSpace(line.Memo)
			if !documentUUIDPattern.MatchString(request.Lines[index].AccountID) {
				return request, fmt.Errorf("line %d accountId must be a UUID", index+1)
			}
			debit, err := parseMoney(orZero(request.Lines[index].Debit))
			if err != nil {
				return request, fmt.Errorf("line %d debit is invalid", index+1)
			}
			credit, err := parseMoney(orZero(request.Lines[index].Credit))
			if err != nil {
				return request, fmt.Errorf("line %d credit is invalid", index+1)
			}
			if (debit > 0 && credit > 0) || (debit == 0 && credit == 0) {
				return request, fmt.Errorf("line %d must have either a debit or a credit", index+1)
			}
			debitTotal += debit
			creditTotal += credit
		}
		if debitTotal != creditTotal || debitTotal <= 0 {
			return request, errors.New("journal voucher lines must balance and be greater than zero")
		}
		request.Amount = formatMoney(debitTotal)
		return request, nil
	}
	if request.PartyID == "" || !documentUUIDPattern.MatchString(request.PartyID) {
		return request, errors.New("receipt and payment vouchers require partyId")
	}
	amount, err := parseMoney(request.Amount)
	if err != nil || amount <= 0 {
		return request, errors.New("amount must be a positive decimal")
	}
	return request, nil
}

func orZero(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0"
	}
	return strings.TrimSpace(value)
}

func (s *Server) financeVoucher(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "finance.write") {
		return
	}
	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_voucher", "Invalid voucher", "The voucher payload could not be read.")
		return
	}
	request, err := parseVoucherPostRequest(payload)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_voucher", "Invalid voucher", err.Error())
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The finance store could not be opened.")
		return
	}
	defer tx.Rollback()
	documentID, eventID, voucherID, err := projectPostedVoucher(r.Context(), tx, operator, request)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "voucher_rejected", "Voucher rejected", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "voucher_commit_failed", "Voucher failed", "The voucher could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":       voucherDocumentKind(request.CategoryCode),
		"documentId": documentID,
		"eventId":    eventID,
		"voucherId":  voucherID,
		"status":     "posted",
		"message":    "Voucher posted.",
	})
}

func projectPostedVoucher(ctx context.Context, tx *sql.Tx, operator *sessionContext, request voucherPostRequest) (string, string, string, error) {
	kind := voucherDocumentKind(request.CategoryCode)
	if operator == nil || !documentUUIDPattern.MatchString(operator.CounterID) {
		return "", "", "", errors.New("counter context is required to post a voucher")
	}
	occurredAt, err := time.Parse(time.RFC3339, request.OccurredAt)
	if err != nil {
		return "", "", "", errors.New("occurredAt must be RFC3339")
	}
	if request.CategoryCode != "journal" {
		partyType := "customer"
		if request.CategoryCode == "payment" {
			partyType = "supplier"
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM master_parties
				WHERE tenant_id = $1::uuid AND id = $2::uuid AND party_type = $3 AND active
			)
		`, operator.TenantID, request.PartyID, partyType).Scan(&exists); err != nil {
			return "", "", "", err
		}
		if !exists {
			return "", "", "", fmt.Errorf("%s voucher party is not an active %s in the authenticated tenant", request.CategoryCode, partyType)
		}
	}
	documentNumber, err := nextBusinessDocumentNumber(ctx, tx, operator.TenantID, operator.BranchID, kind)
	if err != nil {
		return "", "", "", err
	}
	customerID, supplierID := "", ""
	if request.CategoryCode == "receipt" {
		customerID = request.PartyID
	}
	if request.CategoryCode == "payment" {
		supplierID = request.PartyID
	}
	var documentID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status, occurred_at,
			 customer_id, supplier_id, reference, remarks, price_level, subtotal, discount_amount, misc_amount,
			 tax_amount, total_amount, paid_amount, balance_amount, pricing_inputs, pricing_result)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, 'posted', $7::timestamptz,
			NULLIF($8, '')::uuid, NULLIF($9, '')::uuid, $10, $11, 1, $12, 0, 0, 0, $12, $12, 0, '{}'::jsonb, '{}'::jsonb)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, kind, documentNumber, occurredAt,
		customerID, supplierID, request.CategoryCode, request.Description, request.Amount).Scan(&documentID); err != nil {
		return "", "", "", err
	}
	var eventID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO sync_events
			(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id,
			 idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4::uuid, 'finance',
			$5::uuid, $6, 1, $7::jsonb, $8::timestamptz)
		RETURNING event_id::text
	`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, documentID, request.IdempotencyKey,
		mustJSON(map[string]any{
			"kind":         kind,
			"categoryCode": request.CategoryCode,
			"partyId":      request.PartyID,
			"amount":       request.Amount,
			"description":  request.Description,
		}), occurredAt).Scan(&eventID)
	if err != nil {
		if isUniqueViolation(err) {
			return "", "", "", errors.New("voucher idempotency key already exists")
		}
		return "", "", "", err
	}
	var voucherID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO voucher_entries
			(tenant_id, branch_id, category_code, source_event_id, source_document_id, status, amount, description)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5::uuid, 'posted', $6::numeric, $7)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, request.CategoryCode, eventID, documentID, request.Amount, request.Description).Scan(&voucherID); err != nil {
		return "", "", "", err
	}
	if err := projectVoucherFinance(ctx, tx, operator, request, documentID, eventID, occurredAt); err != nil {
		return "", "", "", err
	}
	return documentID, eventID, voucherID, nil
}

func projectVoucherFinance(ctx context.Context, tx *sql.Tx, operator *sessionContext, request voucherPostRequest, documentID, eventID string, occurredAt time.Time) error {
	amount, err := parseMoney(request.Amount)
	if err != nil {
		return err
	}
	var journalID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO gl_journals
			(tenant_id, branch_id, source_event_id, source_document_id, kind,
			 description, posted_at, total_debit, total_credit)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::timestamptz, $8::numeric, $8::numeric)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, eventID, documentID, voucherDocumentKind(request.CategoryCode),
		strings.TrimSpace(request.Description+" "+request.CategoryCode), occurredAt, formatMoney(amount)).Scan(&journalID); err != nil {
		return err
	}
	if request.CategoryCode == "journal" {
		for index, line := range request.Lines {
			if err := insertGLLine(ctx, tx, operator, journalID, index+1, line.AccountID, "", orZero(line.Debit), orZero(line.Credit), line.Memo); err != nil {
				return err
			}
		}
		return nil
	}
	keys := []string{"cash"}
	if request.CategoryCode == "receipt" {
		keys = append(keys, "accounts_receivable")
	} else {
		keys = append(keys, "accounts_payable")
	}
	accounts, err := configuredFinanceAccounts(ctx, tx, operator, keys)
	if err != nil {
		return err
	}
	if request.CategoryCode == "receipt" {
		if err := insertGLLine(ctx, tx, operator, journalID, 1, accounts["cash"], "", formatMoney(amount), "0", "Cash receipt"); err != nil {
			return err
		}
		if err := insertGLLine(ctx, tx, operator, journalID, 2, accounts["accounts_receivable"], request.PartyID, "0", formatMoney(amount), "Customer receipt"); err != nil {
			return err
		}
		return upsertVoucherPartyLedger(ctx, tx, operator, request.PartyID, "customer", eventID, documentID, "0", formatMoney(amount), occurredAt, "Receipt voucher")
	}
	if err := insertGLLine(ctx, tx, operator, journalID, 1, accounts["accounts_payable"], request.PartyID, formatMoney(amount), "0", "Supplier payment"); err != nil {
		return err
	}
	if err := insertGLLine(ctx, tx, operator, journalID, 2, accounts["cash"], "", "0", formatMoney(amount), "Cash payment"); err != nil {
		return err
	}
	return upsertVoucherPartyLedger(ctx, tx, operator, request.PartyID, "supplier", eventID, documentID, formatMoney(amount), "0", occurredAt, "Payment voucher")
}

func upsertVoucherPartyLedger(ctx context.Context, tx *sql.Tx, operator *sessionContext, partyID, counterparty, eventID, documentID, debit, credit string, occurredAt time.Time, description string) error {
	var balanceAfter string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO party_ledger_balances
			(tenant_id, branch_id, party_id, debit_total, credit_total, balance)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::numeric, $5::numeric, ($4::numeric - $5::numeric))
		ON CONFLICT (tenant_id, branch_id, party_id) DO UPDATE
		SET debit_total = party_ledger_balances.debit_total + EXCLUDED.debit_total,
		    credit_total = party_ledger_balances.credit_total + EXCLUDED.credit_total,
		    balance = party_ledger_balances.balance + EXCLUDED.balance,
		    updated_at = now()
		RETURNING balance::text
	`, operator.TenantID, operator.BranchID, partyID, debit, credit).Scan(&balanceAfter); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO party_ledger_entries
			(tenant_id, branch_id, party_id, counterparty_kind, source_event_id,
			 source_document_id, entry_kind, debit_amount, credit_amount,
			 balance_after, occurred_at, description)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6::uuid,
		        'voucher', $7::numeric, $8::numeric, $9::numeric, $10::timestamptz, $11)
	`, operator.TenantID, operator.BranchID, partyID, counterparty, eventID, documentID, debit, credit, balanceAfter, occurredAt, description)
	return err
}

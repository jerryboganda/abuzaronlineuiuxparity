package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

type documentFinanceSummary struct {
	JournalID          string `json:"journalId"`
	DebitTotal         string `json:"debitTotal"`
	CreditTotal        string `json:"creditTotal"`
	Balanced           bool   `json:"balanced"`
	PartyLedgerEntryID string `json:"partyLedgerEntryId,omitempty"`
	PartyBalanceAfter  string `json:"partyBalanceAfter,omitempty"`
}

type financeTaxResult struct {
	Amount string `json:"amount"`
}

type financePricingResult struct {
	Taxes []financeTaxResult `json:"taxes"`
}

var financeAccountKeys = []string{
	"cash",
	"accounts_receivable",
	"inventory",
	"sales_revenue",
	"cogs",
	"output_tax",
}

func projectPostedPurchaseFinance(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, eventID string) error {
	if document.Status != "posted" {
		return errors.New("finance posting requires a posted document")
	}
	if !isPurchasePostingKind(document.Kind) {
		return fmt.Errorf("purchase finance posting is not implemented for %s", document.Kind)
	}
	if document.SupplierID == "" {
		return errors.New("purchase finance posting requires a supplier")
	}
	var supplierExists bool
	if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM master_parties
				WHERE tenant_id = $1::uuid AND id = $2::uuid
				  AND party_type = 'supplier' AND active
			)
		`, operator.TenantID, document.SupplierID).Scan(&supplierExists); err != nil {
		return err
	}
	if !supplierExists {
		return errors.New("purchase supplier is not active in the authenticated tenant")
	}

	var existingJournal, existingEvent string
	err := tx.QueryRowContext(ctx, `
			SELECT id::text, source_event_id::text
			FROM gl_journals
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND (source_document_id = $3::uuid OR source_event_id = $4::uuid)
			FOR UPDATE
		`, operator.TenantID, operator.BranchID, document.ID, eventID).Scan(&existingJournal, &existingEvent)
	if err == nil {
		if existingEvent != eventID {
			return errors.New("document already has a finance journal for a different source event")
		}
		var ledgerExists bool
		if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM party_ledger_entries
					WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
					  AND source_document_id = $3::uuid
				)
			`, operator.TenantID, operator.BranchID, document.ID).Scan(&ledgerExists); err != nil {
			return err
		}
		if !ledgerExists {
			return errors.New("existing purchase GL journal has no supplier ledger entry")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	taxAmount, err := financeTaxAmount(document.Pricing)
	if err != nil {
		return err
	}
	total, err := parseMoney(document.Totals.TotalAmount)
	if err != nil {
		return fmt.Errorf("purchase total is invalid for finance posting: %w", err)
	}
	if taxAmount > total {
		return errors.New("purchase input tax exceeds the document total")
	}
	net := total - taxAmount
	keys := []string{"inventory", "accounts_payable"}
	if taxAmount > 0 {
		keys = append(keys, "input_tax")
	}
	accounts, err := configuredFinanceAccounts(ctx, tx, operator, keys)
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM master_parties
				WHERE tenant_id = $1::uuid AND id = $2::uuid
				  AND party_type = 'supplier' AND active
			)
		`, operator.TenantID, document.SupplierID).Scan(&supplierExists); err != nil {
		return err
	}
	if !supplierExists {
		return errors.New("purchase supplier is not active in the authenticated tenant")
	}

	returnKind := document.Kind == "purchase-return"
	journalDebit, journalCredit := total, total
	if err := tx.QueryRowContext(ctx, `
			INSERT INTO gl_journals
				(tenant_id, branch_id, source_event_id, source_document_id, kind,
				 description, posted_at, total_debit, total_credit)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::timestamptz,
			        $8::numeric, $9::numeric)
			RETURNING id::text
		`, operator.TenantID, operator.BranchID, eventID, document.ID, document.Kind,
		"Posted "+document.Kind+" "+document.DocumentNumber, document.OccurredAt,
		formatMoney(journalDebit), formatMoney(journalCredit)).Scan(&existingJournal); err != nil {
		return err
	}
	line := 1
	if returnKind {
		if err := insertGLLine(ctx, tx, operator, existingJournal, line, accounts["accounts_payable"], document.SupplierID, formatMoney(total), "0", "Supplier payable reversal"); err != nil {
			return err
		}
		line++
		if net > 0 {
			if err := insertGLLine(ctx, tx, operator, existingJournal, line, accounts["inventory"], "", "0", formatMoney(net), "Inventory returned to supplier"); err != nil {
				return err
			}
			line++
		}
		if taxAmount > 0 {
			if err := insertGLLine(ctx, tx, operator, existingJournal, line, accounts["input_tax"], "", "0", formatMoney(taxAmount), "Input tax reversed"); err != nil {
				return err
			}
		}
	} else {
		if net > 0 {
			if err := insertGLLine(ctx, tx, operator, existingJournal, line, accounts["inventory"], "", formatMoney(net), "0", "Inventory received"); err != nil {
				return err
			}
			line++
		}
		if taxAmount > 0 {
			if err := insertGLLine(ctx, tx, operator, existingJournal, line, accounts["input_tax"], "", formatMoney(taxAmount), "0", "Input tax"); err != nil {
				return err
			}
			line++
		}
		if err := insertGLLine(ctx, tx, operator, existingJournal, line, accounts["accounts_payable"], document.SupplierID, "0", formatMoney(total), "Supplier payable"); err != nil {
			return err
		}
	}

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
		`, operator.TenantID, operator.BranchID, document.SupplierID,
		func() string {
			if returnKind {
				return formatMoney(total)
			}
			return "0"
		}(),
		func() string {
			if returnKind {
				return "0"
			}
			return formatMoney(total)
		}()).Scan(&balanceAfter); err != nil {
		return err
	}
	debit, credit := "0", formatMoney(total)
	entryKind := "purchase"
	if returnKind {
		debit, credit, entryKind = formatMoney(total), "0", "purchase-return"
	}
	if _, err := tx.ExecContext(ctx, `
			INSERT INTO party_ledger_entries
				(tenant_id, branch_id, party_id, counterparty_kind, source_event_id,
				 source_document_id, entry_kind, debit_amount, credit_amount,
				 balance_after, occurred_at, description)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'supplier', $4::uuid, $5::uuid,
			        $6, $7::numeric, $8::numeric, $9::numeric, $10::timestamptz, $11)
		`, operator.TenantID, operator.BranchID, document.SupplierID, eventID, document.ID,
		entryKind, debit, credit, balanceAfter, document.OccurredAt,
		"Posted "+document.Kind+" "+document.DocumentNumber); err != nil {
		return err
	}
	return nil
}

func configuredFinanceAccounts(ctx context.Context, tx *sql.Tx, operator *sessionContext, keys []string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `
			SELECT system_key, id::text
			FROM finance_accounts
			WHERE tenant_id = $1::uuid AND active AND system_key = ANY($2::text[])
		`, operator.TenantID, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make(map[string]string, len(keys))
	for rows.Next() {
		var key, id string
		if err := rows.Scan(&key, &id); err != nil {
			return nil, err
		}
		accounts[key] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, key := range keys {
		if accounts[key] == "" {
			return nil, fmt.Errorf("required finance account %q is not configured for tenant", key)
		}
	}
	return accounts, nil
}
func projectPostedSaleFinance(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, eventID string) error {
	if document.Status != "posted" {
		return errors.New("finance posting requires a posted document")
	}
	if document.Kind != "cash-sale" && document.Kind != "credit-sale" {
		return fmt.Errorf("finance posting is not implemented for %s", document.Kind)
	}

	var existingJournal, existingEvent string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, source_event_id::text
		FROM gl_journals
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND (source_document_id = $3::uuid OR source_event_id = $4::uuid)
		FOR UPDATE
	`, operator.TenantID, operator.BranchID, document.ID, eventID).Scan(&existingJournal, &existingEvent)
	if err == nil {
		if existingEvent != eventID {
			return fmt.Errorf("document already has a finance journal for a different source event")
		}
		var ledgerExists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM party_ledger_entries
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
				  AND source_document_id = $3::uuid
			)
		`, operator.TenantID, operator.BranchID, document.ID).Scan(&ledgerExists); err != nil {
			return err
		}
		if !ledgerExists {
			return errors.New("existing GL journal has no party ledger entry")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	accounts, err := requiredFinanceAccounts(ctx, tx, operator, document)
	if err != nil {
		return err
	}
	taxAmount, err := financeTaxAmount(document.Pricing)
	if err != nil {
		return err
	}
	total, err := parseMoney(document.Totals.TotalAmount)
	if err != nil {
		return fmt.Errorf("document total is invalid for finance posting: %w", err)
	}
	if taxAmount > total {
		return errors.New("output tax exceeds the document total")
	}
	revenue := total - taxAmount
	cogs, hasCost, err := saleCOGS(ctx, tx, operator, document.ID)
	if err != nil {
		return err
	}
	if !hasCost {
		cogs = "0"
	}

	var journalID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO gl_journals
			(tenant_id, branch_id, source_event_id, source_document_id, kind,
			 description, posted_at, total_debit, total_credit)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::timestamptz,
		        ($8::numeric + $9::numeric), ($10::numeric + $9::numeric))
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, eventID, document.ID, document.Kind,
		"Posted "+document.Kind+" "+document.DocumentNumber, document.OccurredAt,
		formatMoney(total), cogs, formatMoney(total)).Scan(&journalID); err != nil {
		return err
	}

	cashOrReceivable := accounts["cash"]
	if document.Kind == "credit-sale" {
		cashOrReceivable = accounts["accounts_receivable"]
	}
	if err := insertGLLine(ctx, tx, operator, journalID, 1, cashOrReceivable, document.CustomerID,
		formatMoney(total), "0", "Sale settlement"); err != nil {
		return err
	}
	if err := insertGLLine(ctx, tx, operator, journalID, 2, accounts["sales_revenue"], "",
		"0", formatMoney(revenue), "Sales revenue"); err != nil {
		return err
	}
	lineNumber := 3
	if taxAmount > 0 {
		if err := insertGLLine(ctx, tx, operator, journalID, lineNumber, accounts["output_tax"], "",
			"0", formatMoney(taxAmount), "Output tax"); err != nil {
			return err
		}
		lineNumber++
	}
	if hasCost {
		if err := insertGLLine(ctx, tx, operator, journalID, lineNumber, accounts["cogs"], "",
			cogs, "0", "Cost of goods sold"); err != nil {
			return err
		}
		if err := insertGLLine(ctx, tx, operator, journalID, lineNumber+1, accounts["inventory"], "",
			"0", cogs, "Inventory at allocated cost"); err != nil {
			return err
		}
	}

	counterpartyKind := "cash"
	if document.Kind == "credit-sale" {
		counterpartyKind = "customer"
		if document.CustomerID == "" {
			return errors.New("credit-sale finance posting requires a customer")
		}
	}
	balanceAfter := ""
	if document.CustomerID != "" {
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO party_ledger_balances
				(tenant_id, branch_id, party_id, debit_total, credit_total, balance)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::numeric, 0, $4::numeric)
			ON CONFLICT (tenant_id, branch_id, party_id) DO UPDATE
			SET debit_total = party_ledger_balances.debit_total + EXCLUDED.debit_total,
			    balance = party_ledger_balances.balance + EXCLUDED.debit_total,
			    updated_at = now()
			RETURNING balance::text
		`, operator.TenantID, operator.BranchID, document.CustomerID, formatMoney(total)).Scan(&balanceAfter); err != nil {
			return err
		}
	}
	var entryID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO party_ledger_entries
			(tenant_id, branch_id, party_id, counterparty_kind, source_event_id,
			 source_document_id, entry_kind, debit_amount, credit_amount,
			 balance_after, occurred_at, description)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5::uuid, $6::uuid,
		        'sale', $7::numeric, 0, NULLIF($8, '')::numeric, $9::timestamptz, $10)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, document.CustomerID, counterpartyKind,
		eventID, document.ID, formatMoney(total), balanceAfter, document.OccurredAt,
		"Posted "+document.Kind+" "+document.DocumentNumber).Scan(&entryID); err != nil {
		return err
	}
	_ = entryID
	return nil
}

func requiredFinanceAccounts(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse) (map[string]string, error) {
	keys := append([]string(nil), financeAccountKeys...)
	taxAmount, err := financeTaxAmount(document.Pricing)
	if err != nil {
		return nil, err
	}
	if taxAmount == 0 {
		keys = []string{"cash", "accounts_receivable", "inventory", "sales_revenue", "cogs"}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT system_key, id::text
		FROM finance_accounts
		WHERE tenant_id = $1::uuid AND active AND system_key = ANY($2::text[])
	`, operator.TenantID, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make(map[string]string, len(keys))
	for rows.Next() {
		var key, id string
		if err := rows.Scan(&key, &id); err != nil {
			return nil, err
		}
		accounts[key] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, key := range keys {
		if accounts[key] == "" {
			return nil, fmt.Errorf("required finance account %q is not configured for tenant", key)
		}
	}
	return accounts, nil
}

func insertGLLine(ctx context.Context, tx *sql.Tx, operator *sessionContext, journalID string, lineNumber int, accountID, partyID, debit, credit, memo string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO gl_lines
			(tenant_id, branch_id, journal_id, line_number, account_id, party_id,
			 debit_amount, credit_amount, memo)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, NULLIF($6, '')::uuid,
		        $7::numeric, $8::numeric, $9)
	`, operator.TenantID, operator.BranchID, journalID, lineNumber, accountID, partyID, debit, credit, memo)
	return err
}

func saleCOGS(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID string) (string, bool, error) {
	var cogs string
	var allocationCount int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(a.quantity * a.unit_cost), 0)::numeric(19,4)::text,
		       COUNT(*)
		FROM stock_allocations a
		WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid
		  AND a.document_id = $3::uuid
	`, operator.TenantID, operator.BranchID, documentID).Scan(&cogs, &allocationCount); err != nil {
		return "", false, err
	}
	value, ok := new(big.Rat).SetString(strings.TrimSpace(cogs))
	if !ok || value.Sign() < 0 {
		return "", false, fmt.Errorf("stock allocation cost %q is invalid", cogs)
	}
	return cogs, allocationCount > 0 && value.Sign() > 0, nil
}

func financeTaxAmount(raw json.RawMessage) (pricing.Money, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	var result financePricingResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, errors.New("posted pricing result is invalid for finance posting")
	}
	var total pricing.Money
	for _, tax := range result.Taxes {
		amount, err := parseMoney(tax.Amount)
		if err != nil {
			return 0, fmt.Errorf("pricing tax amount is invalid: %w", err)
		}
		if total > pricing.Money(^uint64(0)>>1)-amount {
			return 0, errors.New("pricing tax total overflowed finance currency")
		}
		total += amount
	}
	return total, nil
}

func (s *Server) financeAccounts(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "reports.read") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read finance accounts", "The finance store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `
		SELECT a.id::text, COALESCE(a.system_key, ''), a.code, a.name, c.code, a.active
		FROM finance_accounts a
		JOIN finance_account_categories c
		  ON c.tenant_id = a.tenant_id AND c.code = a.category_code
		WHERE a.tenant_id = $1::uuid
		ORDER BY a.code
	`, operator.TenantID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read finance accounts", "The account query failed.")
		return
	}
	type account struct {
		ID        string `json:"id"`
		SystemKey string `json:"systemKey,omitempty"`
		Code      string `json:"code"`
		Name      string `json:"name"`
		Category  string `json:"category"`
		Active    bool   `json:"active"`
	}
	accounts := make([]account, 0)
	for rows.Next() {
		var item account
		if err := rows.Scan(&item.ID, &item.SystemKey, &item.Code, &item.Name, &item.Category, &item.Active); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read finance accounts", "The account response could not be decoded.")
			return
		}
		accounts = append(accounts, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read finance accounts", "The account response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read finance accounts", "The account transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (s *Server) financeJournals(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "reports.read") {
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			writeProblem(w, http.StatusBadRequest, "invalid_finance_limit", "Invalid finance limit", "Limit must be between 1 and 500.")
			return
		}
		limit = value
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read journals", "The finance store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `
		SELECT j.id::text, j.source_document_id::text, j.source_event_id::text,
		       j.kind, j.posted_at::text, j.total_debit::text, j.total_credit::text
		FROM gl_journals j
		WHERE j.tenant_id = $1::uuid AND ($2 = '' OR j.branch_id::text = $2)
		ORDER BY j.posted_at DESC, j.id DESC
		LIMIT $3
	`, operator.TenantID, operator.BranchID, limit)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read journals", "The journal query failed.")
		return
	}
	type journal struct {
		ID          string `json:"id"`
		DocumentID  string `json:"documentId"`
		EventID     string `json:"eventId"`
		Kind        string `json:"kind"`
		PostedAt    string `json:"postedAt"`
		DebitTotal  string `json:"debitTotal"`
		CreditTotal string `json:"creditTotal"`
	}
	journals := make([]journal, 0)
	for rows.Next() {
		var item journal
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.EventID, &item.Kind, &item.PostedAt, &item.DebitTotal, &item.CreditTotal); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read journals", "The journal response could not be decoded.")
			return
		}
		journals = append(journals, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read journals", "The journal response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read journals", "The journal transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"journals": journals})
}

func (s *Server) financeLedger(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "reports.read") {
		return
	}
	partyID := strings.TrimSpace(r.URL.Query().Get("partyId"))
	if !documentUUIDPattern.MatchString(partyID) {
		writeProblem(w, http.StatusBadRequest, "party_required", "Party is required", "Provide a canonical customer partyId.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read party ledger", "The finance store could not be opened.")
		return
	}
	defer tx.Rollback()
	balance := "0.0000"
	if err := tx.QueryRowContext(r.Context(), `
		SELECT balance::text
		FROM party_ledger_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
	`, operator.TenantID, operator.BranchID, partyID).Scan(&balance); err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read party ledger", "The party balance query failed.")
		return
	}
	type entry struct {
		ID           string `json:"id"`
		DocumentID   string `json:"documentId"`
		Debit        string `json:"debit"`
		Credit       string `json:"credit"`
		BalanceAfter string `json:"balanceAfter,omitempty"`
		OccurredAt   string `json:"occurredAt"`
		Description  string `json:"description"`
	}
	rows, err := tx.QueryContext(r.Context(), `
		SELECT id::text, source_document_id::text, debit_amount::text, credit_amount::text,
		       COALESCE(balance_after::text, ''), occurred_at::text, description
		FROM party_ledger_entries
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
		ORDER BY occurred_at, id
	`, operator.TenantID, operator.BranchID, partyID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read party ledger", "The party ledger query failed.")
		return
	}
	entries := make([]entry, 0)
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.Debit, &item.Credit, &item.BalanceAfter, &item.OccurredAt, &item.Description); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read party ledger", "The party ledger response could not be decoded.")
			return
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read party ledger", "The party ledger response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "finance_read_failed", "Unable to read party ledger", "The ledger transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"partyId": partyID, "balance": balance, "entries": entries})
}

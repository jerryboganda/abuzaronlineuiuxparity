package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

type voidStockMovement struct {
	id             string
	batchID        string
	sourceLineID   sql.NullString
	direction      string
	adjustmentSign int
	quantity       string
	unitCost       string
}

type voidFinanceLine struct {
	accountID string
	partyID   sql.NullString
	debit     string
	credit    string
	memo      string
}

type voidPartyEntry struct {
	partyID          sql.NullString
	counterpartyKind string
	debit            string
	credit           string
}

func isReversiblePostedDocumentKind(kind string) bool {
	return isStockAndFinanceSaleKind(kind) || isPurchasePostingKind(kind) || isSaleReturnDocumentKind(kind)
}

// projectPostedDocumentVoid performs the compensating side of a legacy Void
// action. All source rows remain immutable; inverse stock and finance rows are
// linked to the new void command event and committed with the document status.
func projectPostedDocumentVoid(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, eventID, reason, voidedAt string) error {
	if document.Status != "void" {
		return errors.New("void reversal requires a document already marked void")
	}
	if !isReversiblePostedDocumentKind(document.Kind) {
		return nil
	}
	if strings.TrimSpace(voidedAt) == "" {
		voidedAt = document.OccurredAt
	}

	var existingEvent string
	err := tx.QueryRowContext(ctx, `
		SELECT reversal_event_id::text
		FROM business_document_void_reversals
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND source_document_id = $3::uuid
		FOR UPDATE
	`, operator.TenantID, operator.BranchID, document.ID).Scan(&existingEvent)
	if err == nil {
		if existingEvent != eventID {
			return fmt.Errorf("document %s already has a void reversal under another event", document.ID)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var dependentNumber, dependentKind string
	if err := tx.QueryRowContext(ctx, `
		SELECT document_number, kind
		FROM business_documents
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND source_document_id = $3::uuid AND status = 'posted'
		ORDER BY occurred_at, id
		LIMIT 1
	`, operator.TenantID, operator.BranchID, document.ID).Scan(&dependentNumber, &dependentKind); err == nil {
		return fmt.Errorf("document cannot be voided while posted dependent document %s (%s) exists", dependentNumber, dependentKind)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	movements, err := loadVoidStockMovements(ctx, tx, operator, document.ID)
	if err != nil {
		return err
	}
	if len(movements) == 0 {
		return fmt.Errorf("document %s has no immutable stock projection to reverse", document.DocumentNumber)
	}
	for _, movement := range movements {
		if err := insertInverseStockMovement(ctx, tx, operator, document, eventID, voidedAt, movement); err != nil {
			return err
		}
	}

	journalID, lines, err := loadVoidFinanceProjection(ctx, tx, operator, document.ID)
	if err != nil {
		return err
	}
	if journalID == "" || len(lines) == 0 {
		return fmt.Errorf("document %s has no immutable finance projection to reverse", document.DocumentNumber)
	}
	party, err := loadVoidPartyEntry(ctx, tx, operator, document.ID)
	if err != nil {
		return err
	}
	reversalJournalID, err := insertVoidJournal(ctx, tx, operator, document, eventID, voidedAt, journalID, lines)
	if err != nil {
		return err
	}
	if err := insertVoidPartyEntry(ctx, tx, operator, document, eventID, voidedAt, party); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO business_document_void_reversals
			(tenant_id, branch_id, source_document_id, reversal_event_id, reversal_journal_id, reason)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6)
	`, operator.TenantID, operator.BranchID, document.ID, eventID, reversalJournalID, strings.TrimSpace(reason)); err != nil {
		return err
	}
	return nil
}

func loadVoidStockMovements(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID string) ([]voidStockMovement, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text, batch_id::text, source_document_line_id::text,
		       direction, adjustment_sign, quantity::text, unit_cost::text
		FROM stock_ledger
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND source_document_id = $3::uuid
		ORDER BY occurred_at, id
	`, operator.TenantID, operator.BranchID, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]voidStockMovement, 0)
	for rows.Next() {
		var movement voidStockMovement
		if err := rows.Scan(&movement.id, &movement.batchID, &movement.sourceLineID, &movement.direction,
			&movement.adjustmentSign, &movement.quantity, &movement.unitCost); err != nil {
			return nil, err
		}
		result = append(result, movement)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func insertInverseStockMovement(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, eventID, voidedAt string, source voidStockMovement) error {
	quantity, err := parseStockQuantity(source.quantity)
	if err != nil || quantity.Sign() <= 0 {
		return fmt.Errorf("source stock movement %s has invalid quantity %q", source.id, source.quantity)
	}
	currentText, err := lockVoidStockBalance(ctx, tx, operator, source.batchID)
	if err != nil {
		return err
	}
	current, ok := new(big.Rat).SetString(strings.TrimSpace(currentText))
	if !ok || current.Sign() < 0 {
		return fmt.Errorf("stock balance for batch %s is invalid", source.batchID)
	}
	deltaSign := inverseStockDeltaSign(source.direction, source.adjustmentSign)
	if deltaSign == 0 {
		return fmt.Errorf("source stock movement %s has unsupported direction %q", source.id, source.direction)
	}
	next := new(big.Rat).Set(current)
	if deltaSign < 0 {
		if current.Cmp(quantity) < 0 {
			return fmt.Errorf("void reversal would make batch %s stock negative (available %s, required %s)", source.batchID, formatStockQuantity(current), formatStockQuantity(quantity))
		}
		next.Sub(next, quantity)
	} else {
		next.Add(next, quantity)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE stock_balances
		SET on_hand = $4::numeric, updated_at = now()
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
	`, operator.TenantID, operator.BranchID, source.batchID, formatStockQuantity(next)); err != nil {
		return err
	}

	direction := source.direction
	adjustmentSign := source.adjustmentSign
	switch source.direction {
	case "in":
		direction = "out"
	case "out":
		direction = "in"
	case "adjustment":
		adjustmentSign = -adjustmentSign
	default:
		return fmt.Errorf("source stock movement %s has unsupported direction %q", source.id, source.direction)
	}
	sourceLineID := ""
	if source.sourceLineID.Valid {
		sourceLineID = source.sourceLineID.String
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO stock_ledger
			(tenant_id, branch_id, batch_id, source_event_id, source_document_id,
			 source_document_line_id, source_line_key, direction, adjustment_sign,
			 quantity, unit_cost, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid,
			NULLIF($6, '')::uuid, $7, $8, $9, $10::numeric, $11::numeric, $12::timestamptz)
	`, operator.TenantID, operator.BranchID, source.batchID, eventID, document.ID,
		sourceLineID, "void-reversal-"+source.id, direction, adjustmentSign,
		formatStockQuantity(quantity), source.unitCost, voidedAt)
	return err
}

func lockVoidStockBalance(ctx context.Context, tx *sql.Tx, operator *sessionContext, batchID string) (string, error) {
	var current string
	if err := tx.QueryRowContext(ctx, `
		SELECT on_hand::text
		FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
		FOR UPDATE
	`, operator.TenantID, operator.BranchID, batchID).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("stock balance for batch %s is missing", batchID)
		}
		return "", err
	}
	return current, nil
}

func inverseStockDeltaSign(direction string, adjustmentSign int) int {
	switch direction {
	case "in":
		return -1
	case "out":
		return 1
	case "adjustment":
		if adjustmentSign < 0 {
			return 1
		}
		return -1
	default:
		return 0
	}
}

func loadVoidFinanceProjection(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID string) (string, []voidFinanceLine, error) {
	var journalID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM gl_journals
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND source_document_id = $3::uuid AND kind <> 'void-reversal'
		ORDER BY created_at, id
		LIMIT 1
		FOR SHARE
	`, operator.TenantID, operator.BranchID, documentID).Scan(&journalID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, fmt.Errorf("document %s has no immutable finance journal", documentID)
		}
		return "", nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT account_id::text, party_id::text, debit_amount::text, credit_amount::text, memo
		FROM gl_lines
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND journal_id = $3::uuid
		ORDER BY line_number
	`, operator.TenantID, operator.BranchID, journalID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	lines := make([]voidFinanceLine, 0)
	for rows.Next() {
		var line voidFinanceLine
		if err := rows.Scan(&line.accountID, &line.partyID, &line.debit, &line.credit, &line.memo); err != nil {
			return "", nil, err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	return journalID, lines, nil
}

func loadVoidPartyEntry(ctx context.Context, tx *sql.Tx, operator *sessionContext, documentID string) (voidPartyEntry, error) {
	var entry voidPartyEntry
	if err := tx.QueryRowContext(ctx, `
		SELECT party_id::text, counterparty_kind, debit_amount::text, credit_amount::text
		FROM party_ledger_entries
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND source_document_id = $3::uuid AND entry_kind <> 'void'
		ORDER BY created_at, id
		LIMIT 1
		FOR SHARE
	`, operator.TenantID, operator.BranchID, documentID).Scan(&entry.partyID, &entry.counterpartyKind, &entry.debit, &entry.credit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return voidPartyEntry{}, fmt.Errorf("document %s has no immutable party-ledger entry", documentID)
		}
		return voidPartyEntry{}, err
	}
	return entry, nil
}

func insertVoidJournal(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, eventID, voidedAt, sourceJournalID string, lines []voidFinanceLine) (string, error) {
	var sourceDebit, sourceCredit string
	if err := tx.QueryRowContext(ctx, `
		SELECT total_debit::text, total_credit::text
		FROM gl_journals
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND id = $3::uuid
	`, operator.TenantID, operator.BranchID, sourceJournalID).Scan(&sourceDebit, &sourceCredit); err != nil {
		return "", err
	}
	var journalID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO gl_journals
			(tenant_id, branch_id, source_event_id, source_document_id, kind,
			 description, posted_at, total_debit, total_credit, reversal_of_journal_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'void-reversal', $5,
			$6::timestamptz, $7::numeric, $8::numeric, $9::uuid)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, eventID, document.ID,
		"Void reversal of "+document.Kind+" "+document.DocumentNumber, voidedAt,
		sourceCredit, sourceDebit, sourceJournalID).Scan(&journalID); err != nil {
		return "", err
	}
	for index, line := range lines {
		if err := insertGLLine(ctx, tx, operator, journalID, index+1, line.accountID,
			nullableString(line.partyID), line.credit, line.debit, "Void reversal: "+line.memo); err != nil {
			return "", err
		}
	}
	return journalID, nil
}

func insertVoidPartyEntry(ctx context.Context, tx *sql.Tx, operator *sessionContext, document documentResponse, eventID, voidedAt string, source voidPartyEntry) error {
	debitAfter, creditAfter := source.credit, source.debit
	balanceAfter := ""
	partyID := nullableString(source.partyID)
	if partyID != "" {
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
		`, operator.TenantID, operator.BranchID, partyID, debitAfter, creditAfter).Scan(&balanceAfter); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO party_ledger_entries
			(tenant_id, branch_id, party_id, counterparty_kind, source_event_id,
			 source_document_id, entry_kind, debit_amount, credit_amount,
			 balance_after, occurred_at, description)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5::uuid,
			$6::uuid, 'void', $7::numeric, $8::numeric, NULLIF($9, '')::numeric,
			$10::timestamptz, $11)
	`, operator.TenantID, operator.BranchID, partyID, source.counterpartyKind,
		eventID, document.ID, debitAfter, creditAfter, balanceAfter, voidedAt,
		"Void reversal of "+document.Kind+" "+document.DocumentNumber)
	return err
}

func nullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

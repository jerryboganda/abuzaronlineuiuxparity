package httpapi

import (
	"net/http"
	"strings"
)

const maxItemUnpostedTransactions = 200

type itemUnpostedTransactionResponse struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	DocumentNo   string `json:"documentNumber"`
	Status       string `json:"status"`
	OccurredAt   string `json:"occurredAt"`
	LineNumber   int    `json:"lineNumber"`
	ItemLegacyID string `json:"itemLegacyId"`
	ItemName     string `json:"itemName"`
	Quantity     string `json:"quantity"`
	UnitPrice    string `json:"unitPrice"`
	LineTotal    string `json:"lineTotal"`
}

func itemUnpostedTransactionsQuery() string {
	return `
		SELECT d.id::text, d.kind, d.document_number, d.status,
		       d.occurred_at::text, l.line_number, l.item_legacy_id,
		       l.item_name, l.quantity::text, l.unit_price::text, l.line_total::text
		FROM business_documents d
		JOIN business_document_lines l
		  ON l.tenant_id = d.tenant_id
		 AND l.branch_id = d.branch_id
		 AND l.document_id = d.id
		WHERE d.tenant_id = $1::uuid
		  AND d.branch_id = $2::uuid
		  AND l.item_id = $3::uuid
		  AND d.status = 'draft'
		  AND d.deleted_at IS NULL
		ORDER BY d.occurred_at DESC, d.document_number, l.line_number
		LIMIT $4`
}

func (s *Server) itemUnpostedTransactions(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.read") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	itemID := strings.TrimSpace(r.PathValue("id"))
	if !documentUUIDPattern.MatchString(itemID) {
		writeProblem(w, http.StatusBadRequest, "invalid_item_id", "Invalid item", "The Item identifier must be a UUID.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The unposted transaction store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), itemUnpostedTransactionsQuery(), operator.TenantID, operator.BranchID, itemID, maxItemUnpostedTransactions+1)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_unposted_read_failed", "Unable to read unposted transactions", "The scoped draft transaction query failed.")
		return
	}
	transactions := make([]itemUnpostedTransactionResponse, 0)
	for rows.Next() {
		var transaction itemUnpostedTransactionResponse
		if err := rows.Scan(
			&transaction.ID, &transaction.Kind, &transaction.DocumentNo, &transaction.Status,
			&transaction.OccurredAt, &transaction.LineNumber, &transaction.ItemLegacyID,
			&transaction.ItemName, &transaction.Quantity, &transaction.UnitPrice, &transaction.LineTotal,
		); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_unposted_read_failed", "Unable to read unposted transactions", "The scoped draft transaction response could not be decoded.")
			return
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "item_unposted_read_failed", "Unable to read unposted transactions", "The scoped draft transaction response could not be read.")
		return
	}
	rows.Close()
	truncated := len(transactions) > maxItemUnpostedTransactions
	if truncated {
		transactions = transactions[:maxItemUnpostedTransactions]
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_unposted_read_failed", "Unable to read unposted transactions", "The draft transaction query could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"itemId":       itemID,
		"transactions": transactions,
		"truncated":    truncated,
	})
}

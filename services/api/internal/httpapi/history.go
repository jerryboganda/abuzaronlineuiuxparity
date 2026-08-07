package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

var historyAggregates = map[string]struct{}{
	"sale": {}, "sale_return": {}, "quotation": {}, "refused_sale": {},
	"receiving": {}, "return": {}, "purchase_order": {}, "inventory": {},
	"pack-purchase": {}, "loose-purchase": {}, "opening-purchase": {}, "purchase-return": {}, "purchase-order": {},
}

func isCanonicalPurchaseHistoryKind(kind string) bool {
	switch kind {
	case "pack-purchase", "loose-purchase", "opening-purchase", "purchase-return", "purchase-order":
		return true
	default:
		return false
	}
}

func (s *Server) transactionHistory(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if _, ok := historyAggregates[kind]; !ok {
		writeProblem(w, http.StatusBadRequest, "invalid_transaction_kind", "Invalid transaction history", "The requested transaction history is not supported.")
		return
	}
	operator := currentSession(r)
	readPermission := "sales.read"
	if kind == "receiving" || kind == "return" || kind == "purchase_order" || isCanonicalPurchaseHistoryKind(kind) {
		readPermission = "purchases.read"
	}
	if !s.requirePermission(r, w, operator, readPermission) {
		return
	}
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" {
		from = "1900-01-01"
	}
	if to == "" {
		to = "2999-12-31"
	}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_history_date", "Invalid history date", "The from date must use YYYY-MM-DD.")
		return
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_history_date", "Invalid history date", "The to date must use YYYY-MM-DD.")
		return
	}
	if from > to {
		writeProblem(w, http.StatusBadRequest, "invalid_history_date_range", "Invalid history date range", "The from date must be on or before the to date.")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			writeProblem(w, http.StatusBadRequest, "invalid_history_limit", "Invalid history limit", "Limit must be between 1 and 500.")
			return
		}
		limit = parsed
	}
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The transaction history store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows := make([]reportRow, 0)
	var query string
	var args []any
	if kind == "sale" {
		query = salesReadModelQuery(reportSaleAggregate, "LIMIT $6", true)
		args = []any{operator.TenantID, operator.BranchID, from, to, filter, limit}
	} else if kind == "sale_return" {
		query = saleReturnReadModelQuery("LIMIT $6", true)
		args = []any{operator.TenantID, operator.BranchID, from, to, filter, limit}
	} else if kind == "quotation" || kind == "refused_sale" {
		documentKind := "quotation"
		if kind == "refused_sale" {
			documentKind = "refused-sale"
		}
		query = documentReadModelQuery(documentKind, kind, "", "LIMIT $6", true)
		args = []any{operator.TenantID, operator.BranchID, from, to, filter, limit}
	} else if isCanonicalPurchaseHistoryKind(kind) {
		query = `
			SELECT d.id::text, d.document_number, d.occurred_at::text,
			       COALESCE(mp.name, ''), COALESCE(line.item_name, ''),
			       COALESCE(line.quantity::text, ''), d.total_amount::text
			FROM business_documents d
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = d.tenant_id AND mp.id = d.supplier_id AND mp.party_type = 'supplier'
			LEFT JOIN LATERAL (
				SELECT item_name, quantity
				FROM business_document_lines l
				WHERE l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
				ORDER BY line_number
				LIMIT 1
			) line ON true
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid AND d.kind = $3
			  AND d.occurred_at >= $4::date
			  AND d.occurred_at < ($5::date + INTERVAL '1 day')
			  AND ($6 = '' OR d.document_number ILIKE '%' || $6 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $6 || '%'
			       OR COALESCE(line.item_name, '') ILIKE '%' || $6 || '%')
			ORDER BY d.occurred_at DESC LIMIT $7`
		args = []any{operator.TenantID, operator.BranchID, kind, from, to, filter, limit}
	} else {
		query = `
			SELECT event_id::text, occurred_at::text,
			       COALESCE(payload->>'supplierName', payload->>'supplier', payload->>'customerName', payload->>'customer', aggregate),
			       COALESCE(payload->>'itemName', payload->'rows'->0->>'itemName', ''),
			       COALESCE(payload->'rows'->0->>'quantity', ''),
			       COALESCE(payload->>'totalAmount', payload->>'amount', '')
			FROM sync_events
			WHERE tenant_id = $1::uuid AND aggregate = $2
			  AND ($3 = '' OR branch_id::text = $3)
			  AND occurred_at >= $4::date
			  AND occurred_at < ($5::date + INTERVAL '1 day')
			  AND ($6 = '' OR event_id::text ILIKE '%' || $6 || '%' OR payload::text ILIKE '%' || $6 || '%' OR COALESCE(payload->'rows'->0->>'itemName', '') ILIKE '%' || $6 || '%')
			ORDER BY occurred_at DESC LIMIT $7`
		args = []any{operator.TenantID, kind, operator.BranchID, from, to, filter, limit}
	}
	result, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "history_read_failed", "Unable to read transaction history", "The scoped transaction history query failed.")
		return
	}
	defer result.Close()
	for result.Next() {
		var row reportRow
		if kind == "sale" || kind == "sale_return" || kind == "quotation" || kind == "refused_sale" || isCanonicalPurchaseHistoryKind(kind) {
			if err := result.Scan(&row.DocumentID, &row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity, &row.Amount); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "history_read_failed", "Unable to read transaction history", "The transaction history response could not be decoded.")
				return
			}
		} else if err := result.Scan(&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity, &row.Amount); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "history_read_failed", "Unable to read transaction history", "The transaction history response could not be decoded.")
			return
		}
		rows = append(rows, row)
	}
	if err := result.Err(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "history_read_failed", "Unable to read transaction history", "The transaction history response could not be read.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "history_read_failed", "Unable to read transaction history", "The history transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "rows": rows})
}

package httpapi

import (
	"net/http"
	"strings"
)

// inventoryBalance exposes the server-authoritative item/godown balance used
// by legacy item grids. stock_balances is the normalized Phase J cache. The
// old movement table is consulted only when that exact normalized scope has
// no rows, and the response labels that compatibility path explicitly.
func (s *Server) inventoryBalance(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.read") {
		return
	}
	itemLegacyID := strings.TrimSpace(r.URL.Query().Get("itemLegacyId"))
	if itemLegacyID == "" {
		itemLegacyID = strings.TrimSpace(r.URL.Query().Get("itemId"))
	}
	if itemLegacyID == "" {
		writeProblem(w, http.StatusBadRequest, "item_required", "Item is required", "Provide itemLegacyId to read the branch stock balance.")
		return
	}
	godownID := strings.TrimSpace(r.URL.Query().Get("godownId"))
	if godownID == "" || !documentUUIDPattern.MatchString(godownID) {
		writeProblem(w, http.StatusBadRequest, "stock_scope_required", "Stock scope required", "Provide godownId; stock is never selected across godowns implicitly.")
		return
	}
	if !s.requireScope(r, w, operator, "godown", godownID) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The inventory store could not be opened.")
		return
	}
	defer tx.Rollback()
	var available string
	var normalizedRows bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(SUM(on_hand), 0)::text,
		       EXISTS (
				SELECT 1
				FROM stock_balances
				WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
				  AND item_legacy_id = $3 AND godown_id = $4::uuid
		       )
		FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND item_legacy_id = $3 AND godown_id = $4::uuid
	`, operator.TenantID, operator.BranchID, itemLegacyID, godownID).Scan(&available, &normalizedRows); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "inventory_read_failed", "Unable to read stock", "The branch inventory balance could not be calculated.")
		return
	}
	source := "stock_balances"
	compatibilityNote := ""
	if !normalizedRows {
		if err := tx.QueryRowContext(r.Context(), `
			SELECT COALESCE(SUM(CASE
				WHEN im.direction = 'in' THEN im.quantity
				WHEN im.direction = 'out' THEN -im.quantity
				ELSE im.quantity
			END), 0)::text
			FROM inventory_movements im
			JOIN sync_events se
			  ON se.tenant_id = im.tenant_id AND se.event_id = im.source_event_id
			WHERE im.tenant_id = $1::uuid AND im.branch_id = $2::uuid
			  AND im.item_legacy_id = $3
			  AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements(
					CASE WHEN jsonb_typeof(se.payload->'rows') = 'array'
					     THEN se.payload->'rows' ELSE jsonb_build_array(se.payload) END
				) AS payload_row
				WHERE payload_row->>'godownId' = $4
			  )
		`, operator.TenantID, operator.BranchID, itemLegacyID, godownID).Scan(&available); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "inventory_read_failed", "Unable to read stock", "The compatibility inventory balance could not be calculated.")
			return
		}
		source = "compatibility_inventory_movements"
		compatibilityNote = "No normalized stock_balances rows exist for this tenant, branch, item, and godown; legacy inventory_movements fallback is labeled and limited to events carrying the requested godownId."
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "inventory_read_failed", "Unable to read stock", "The inventory transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"tenantId":          operator.TenantID,
		"branchId":          operator.BranchID,
		"itemLegacyId":      itemLegacyID,
		"godownId":          godownID,
		"available":         available,
		"source":            source,
		"compatibilityNote": compatibilityNote,
	})
}

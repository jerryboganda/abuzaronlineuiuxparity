package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

type preferenceItem struct {
	Caption  string `json:"caption"`
	Value    string `json:"value"`
	Position int    `json:"position"`
}

type preferencesRequest struct {
	Category string           `json:"category"`
	Items    []preferenceItem `json:"items"`
}

func (s *Server) preferences(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "preferences.read") {
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category == "" {
		category = "General"
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The preference store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `SELECT caption, value, position FROM tenant_preferences WHERE tenant_id = $1::uuid AND category = $2 ORDER BY position, caption`, operator.TenantID, category)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_read_failed", "Unable to read preferences", "The preference store could not be queried.")
		return
	}
	items := make([]preferenceItem, 0)
	for rows.Next() {
		var item preferenceItem
		if err := rows.Scan(&item.Caption, &item.Value, &item.Position); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "preferences_read_failed", "Unable to read preferences", "The preference response could not be decoded.")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "preferences_read_failed", "Unable to read preferences", "The preference response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_read_failed", "Unable to read preferences", "The preference transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": category, "items": items})
}

func (s *Server) savePreferences(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "preferences.write") {
		return
	}
	var request preferencesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The preference request could not be parsed.")
		return
	}
	request.Category = strings.TrimSpace(request.Category)
	if request.Category == "" || len(request.Items) > 500 {
		writeProblem(w, http.StatusBadRequest, "invalid_preferences", "Invalid preferences", "A category and no more than 500 preference rows are required.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The preference store could not be opened.")
		return
	}
	defer tx.Rollback()
	for index, item := range request.Items {
		item.Caption = strings.TrimSpace(item.Caption)
		if item.Caption == "" {
			continue
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO tenant_preferences (tenant_id, category, caption, value, position, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, category, caption) DO UPDATE SET value = EXCLUDED.value, position = EXCLUDED.position, updated_at = now()
		`, operator.TenantID, request.Category, item.Caption, item.Value, index); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "A preference value could not be stored.")
			return
		}
	}
	encoded, _ := json.Marshal(request.Items)
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, payload) VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, 'preferences.updated', 'tenant_preferences', NULLIF($4, '')::jsonb)`, operator.TenantID, operator.BranchID, operator.UserID, string(encoded)); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "The preference audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "The preference transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": request.Category, "saved": len(request.Items)})
}

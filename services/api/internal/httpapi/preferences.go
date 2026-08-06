package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type preferenceItem struct {
	Caption  string `json:"caption"`
	Value    string `json:"value"`
	Position int    `json:"position"`
}

type preferenceFieldResponse struct {
	Caption       string              `json:"caption"`
	Type          preferenceValueType `json:"type"`
	Default       string              `json:"default"`
	Value         string              `json:"value"`
	Allowed       []string            `json:"allowed,omitempty"`
	Minimum       *float64            `json:"minimum,omitempty"`
	Maximum       *float64            `json:"maximum,omitempty"`
	Behavior      string              `json:"behavior"`
	RuntimeStatus string              `json:"runtimeStatus"`
	Position      int                 `json:"position"`
}

type preferencesRequest struct {
	Category string           `json:"category"`
	Items    []preferenceItem `json:"items"`
}

func preferenceCategories() []string {
	return []string{
		"General", "Sale", "Sale Return", "Purchase", "Purchase Return",
		"Report", "BasicData", "Quotation", "Schedule", "Adjustment",
		"Purchase Order", "Others", "Point of Sale", "Cashier Job Activity",
		"Email", "SMS", "Dashboard",
	}
}

func preferenceDefinitionMap(category string) map[string]preferenceDefinition {
	definitions := make(map[string]preferenceDefinition)
	for _, definition := range reviewedPreferenceRegistry() {
		if definition.Category == category {
			definitions[definition.Caption] = definition
		}
	}
	return definitions
}

func preferenceDivergences() []preferenceDivergence {
	return []preferenceDivergence{
		{
			Category: "Schedule",
			Status:   "not_configured",
			Detail:   "Legacy SQL-Agent/msdb jobs are not reported as configured. PostgreSQL-native scheduling/worker deployment is required; saving these values does not run a job.",
		},
		{
			Category: "Point of Sale",
			Status:   "not_configured",
			Detail:   "Cash drawer, barcode, LCD, and printer actions remain branch-edge hardware adapters and are not claimed by preference persistence.",
		},
	}
}

func preferenceScope(operator *sessionContext) map[string]string {
	return map[string]string{"tenantId": operator.TenantID, "branchId": operator.BranchID}
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
	if !isPreferenceCategory(category) {
		writeProblem(w, http.StatusBadRequest, "unknown_preference_category", "Unknown preference category", "The requested captured preference tab is not registered.")
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select an operational branch before reading preferences.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The preference store could not be opened.")
		return
	}
	defer tx.Rollback()
	items, err := readEffectivePreferences(r, tx, operator, category)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_read_failed", "Unable to read preferences", "The preference store could not be queried.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_read_failed", "Unable to read preferences", "The preference transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"category":      category,
		"scope":         preferenceScope(operator),
		"items":         items,
		"registry":      preferenceFields(category, items),
		"divergences":   preferenceDivergences(),
		"registryCount": len(reviewedPreferenceRegistry()),
	})
}

func (s *Server) savePreferences(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "preferences.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select an operational branch before saving preferences.")
		return
	}
	var request preferencesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The preference request could not be parsed.")
		return
	}
	request.Category = strings.TrimSpace(request.Category)
	definitions := preferenceDefinitionMap(request.Category)
	if len(definitions) == 0 || len(request.Items) > 500 {
		writeProblem(w, http.StatusBadRequest, "invalid_preferences", "Invalid preferences", "A registered category and no more than 500 preference rows are required.")
		return
	}
	for _, item := range request.Items {
		item.Caption = strings.TrimSpace(item.Caption)
		definition, ok := definitions[item.Caption]
		if item.Caption == "" || !ok {
			writeProblem(w, http.StatusBadRequest, "invalid_preference", "Invalid preference", "Every saved caption must be present in the reviewed category registry.")
			return
		}
		if err := preferenceValidationError(definition, item.Value); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_preference_value", "Invalid preference value", definition.Caption+" "+err.Error()+".")
			return
		}
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The preference store could not be opened.")
		return
	}
	defer tx.Rollback()
	for index, item := range request.Items {
		item.Caption = strings.TrimSpace(item.Caption)
		if err := upsertScopedPreference(r, tx, operator, request.Category, item.Caption, item.Value, index); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "A preference value could not be stored.")
			return
		}
	}
	if request.Category == "Report" {
		if err := writeReportPreferenceCompatibility(r, tx, operator, request.Items); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "The report preference compatibility values could not be stored.")
			return
		}
	}
	auditedItems := make([]preferenceItem, 0, len(request.Items))
	for _, item := range request.Items {
		if definition := definitions[item.Caption]; definition.Type == preferenceSecret {
			item.Value = "[redacted]"
		}
		auditedItems = append(auditedItems, item)
	}
	encoded, _ := json.Marshal(auditedItems)
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, payload)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'preferences.updated', 'tenant_preferences', $4::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, string(encoded)); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "The preference audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "preferences_write_failed", "Unable to save preferences", "The preference transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"category": request.Category, "saved": len(request.Items),
		"scope": preferenceScope(operator), "divergences": preferenceDivergences(),
	})
}

func isPreferenceCategory(category string) bool {
	for _, registered := range preferenceCategories() {
		if category == registered {
			return true
		}
	}
	return false
}

func readEffectivePreferences(r *http.Request, tx *sql.Tx, operator *sessionContext, category string) ([]preferenceItem, error) {
	rows, err := tx.QueryContext(r.Context(), `
		SELECT caption, value, position
		FROM (
			SELECT caption, value, position,
			       row_number() OVER (
			           PARTITION BY caption
			           ORDER BY CASE WHEN branch_id = $2::uuid THEN 0 ELSE 1 END
			       ) AS preference_rank
			FROM tenant_preferences
			WHERE tenant_id = $1::uuid AND category = $3
			  AND (branch_id IS NULL OR branch_id = $2::uuid)
		) scoped
		WHERE preference_rank = 1
		ORDER BY position, caption
	`, operator.TenantID, operator.BranchID, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]preferenceItem, 0)
	for rows.Next() {
		var item preferenceItem
		if err := rows.Scan(&item.Caption, &item.Value, &item.Position); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func preferenceFields(category string, items []preferenceItem) []preferenceFieldResponse {
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.Caption] = item.Value
	}
	result := make([]preferenceFieldResponse, 0)
	for _, definition := range reviewedPreferenceRegistry() {
		if definition.Category != category {
			continue
		}
		value, exists := values[definition.Caption]
		if !exists {
			value = definition.Default
		}
		result = append(result, preferenceFieldResponse{
			Caption: definition.Caption, Type: definition.Type, Default: definition.Default,
			Value: value, Allowed: definition.Allowed, Minimum: definition.Minimum,
			Maximum: definition.Maximum, Behavior: definition.Behavior,
			RuntimeStatus: definition.RuntimeStatus, Position: definition.Position,
		})
	}
	return result
}

func upsertScopedPreference(r *http.Request, tx *sql.Tx, operator *sessionContext, category, caption, value string, position int) error {
	// branch_id is nullable for imported tenant defaults. The API always writes
	// the authenticated branch and therefore uses an update-then-insert path
	// rather than relying on NULL-sensitive ON CONFLICT inference.
	result, err := tx.ExecContext(r.Context(), `
		UPDATE tenant_preferences
		SET value = $5, position = $6, updated_at = now()
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND category = $3 AND caption = $4
	`, operator.TenantID, operator.BranchID, category, caption, value, position)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected > 0 {
		return nil
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO tenant_preferences (tenant_id, branch_id, category, caption, value, position, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, now())
	`, operator.TenantID, operator.BranchID, category, caption, value, position)
	return err
}

func writeReportPreferenceCompatibility(r *http.Request, tx *sql.Tx, operator *sessionContext, items []preferenceItem) error {
	for _, item := range items {
		if item.Caption != "Default Header On Report" {
			continue
		}
		result, err := tx.ExecContext(r.Context(), `
			UPDATE tenant_preferences SET value = $3, updated_at = now()
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND category = 'report:letterhead' AND caption = 'name'
		`, operator.TenantID, operator.BranchID, item.Value)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			_, err = tx.ExecContext(r.Context(), `
				INSERT INTO tenant_preferences (tenant_id, branch_id, category, caption, value, position, updated_at)
				VALUES ($1::uuid, $2::uuid, 'report:letterhead', 'name', $3, 0, now())
			`, operator.TenantID, operator.BranchID, item.Value)
		}
		return err
	}
	return nil
}

// preferenceReadValues is used by report and other read models that need the
// effective branch value without exposing a second preference API contract.
func preferenceReadValues(r *http.Request, tx *sql.Tx, operator *sessionContext, categoryPrefix string) (map[string]map[string]string, error) {
	rows, err := tx.QueryContext(r.Context(), `
		SELECT category, caption, value
		FROM (
			SELECT category, caption, value,
			       row_number() OVER (
			           PARTITION BY category, caption
			           ORDER BY CASE WHEN branch_id = $2::uuid THEN 0 ELSE 1 END
			       ) AS preference_rank
			FROM tenant_preferences
			WHERE tenant_id = $1::uuid AND category LIKE $3
			  AND (branch_id IS NULL OR branch_id = $2::uuid)
		) scoped
		WHERE preference_rank = 1
		ORDER BY category, caption
	`, operator.TenantID, operator.BranchID, categoryPrefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]map[string]string)
	for rows.Next() {
		var category, caption, value string
		if err := rows.Scan(&category, &caption, &value); err != nil {
			return nil, err
		}
		if values[category] == nil {
			values[category] = make(map[string]string)
		}
		values[category][strings.ToLower(strings.TrimSpace(caption))] = value
	}
	return values, rows.Err()
}

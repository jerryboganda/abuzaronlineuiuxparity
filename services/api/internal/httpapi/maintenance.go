package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type maintenanceCheck struct {
	Table  string `json:"table"`
	Rows   int64  `json:"rows"`
	Status string `json:"status"`
}

type maintenanceOperation struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Message    string          `json:"message"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// These are application-level checks. Physical PostgreSQL checks and backup
// execution remain deployment-owned; the API must not imply that either was
// performed by counting application rows.
var maintenanceIntegrityTables = []string{
	"tenants",
	"branches",
	"counters",
	"users",
	"master_records",
	"sales_documents",
	"inventory_movements",
	"sync_events",
}

func maintenancePermission(kind string) string {
	if strings.HasPrefix(kind, "manage-") {
		return "manage.users"
	}
	return "maintenance.write"
}

func validMaintenanceKind(kind string) bool {
	return kind != "" && len(kind) <= 96 && !strings.ContainsAny(kind, `/\?&= `)
}

func maintenanceExternalOutcome(kind string) (string, string) {
	normalized := strings.ToLower(kind)
	switch {
	case normalized == "check-database-integrity":
		return "completed", "Application-scope integrity checks completed. Physical PostgreSQL checks were not run by the API."
	case strings.Contains(normalized, "backup"), strings.Contains(normalized, "restore"):
		return "not_configured", "No deployment backup/restore adapter is configured; no database backup or restore was performed."
	case strings.Contains(normalized, "import"):
		return "not_configured", "No server-side import adapter is configured; no records were imported."
	case strings.Contains(normalized, "export"):
		return "not_configured", "No server-side export adapter is configured; no export artifact was produced."
	case strings.Contains(normalized, "send") || strings.Contains(normalized, "test-sms") || strings.Contains(normalized, "test-email"):
		return "not_configured", "No SMS or email adapter is configured; no external message was sent."
	case normalized == "inplace-initialization" || strings.Contains(normalized, "delete"):
		return "not_configured", "This destructive operation requires an explicit deployment policy; no data was changed."
	default:
		return "saved", "Maintenance configuration saved for the authenticated tenant and branch."
	}
}

func validateMaintenancePayload(kind string, payload map[string]any) error {
	if len(payload) > 100 {
		return errors.New("a maintenance request may contain no more than 100 fields")
	}
	for key, value := range payload {
		if strings.TrimSpace(key) == "" || len(key) > 120 {
			return errors.New("maintenance field names must be between 1 and 120 characters")
		}
		if text, ok := value.(string); ok && len(text) > 4096 {
			return errors.New("maintenance field values may not exceed 4096 characters")
		}
	}
	if strings.Contains(strings.ToLower(kind), "import") {
		source, _ := payload["sourceFile"].(string)
		source = strings.TrimSpace(source)
		if source == "" {
			return errors.New("sourceFile is required for import validation")
		}
		// A client-supplied server path is never opened by this API. Accept only
		// a logical file name so a future upload adapter can bind it safely.
		if strings.Contains(source, "..") || strings.ContainsAny(source, `/\:`) {
			return errors.New("sourceFile must be a logical uploaded file name, not a server path")
		}
	}
	return nil
}

func (s *Server) maintenanceAction(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	kind := strings.TrimSpace(r.PathValue("kind"))
	if !validMaintenanceKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_maintenance_kind", "Invalid maintenance workflow", "The requested maintenance workflow is not supported.")
		return
	}
	if !s.requirePermission(r, w, operator, maintenancePermission(kind)) {
		return
	}

	payload := make(map[string]any)
	if r.Body != nil {
		err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&payload)
		if err != nil && !errors.Is(err, io.EOF) {
			writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The maintenance request could not be parsed.")
			return
		}
	}
	if err := validateMaintenancePayload(kind, payload); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_maintenance_request", "Invalid maintenance request", err.Error())
		return
	}

	if kind == "check-database-integrity" {
		s.handleIntegrityCheck(w, r, operator, kind, payload)
		return
	}

	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The maintenance store could not be opened.")
		return
	}
	defer tx.Rollback()

	saved := 0
	for caption, value := range payload {
		caption = strings.TrimSpace(caption)
		if caption == "" || isSensitiveMaintenanceField(caption) {
			continue
		}
		encodedValue, marshalErr := maintenanceValue(value)
		if marshalErr != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_maintenance_value", "Invalid maintenance value", "A maintenance value could not be encoded.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO tenant_preferences (tenant_id, category, caption, value, position, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, now())
			ON CONFLICT (tenant_id, category, caption) DO UPDATE
			SET value = EXCLUDED.value, position = EXCLUDED.position, updated_at = now()
		`, operator.TenantID, "maintenance:"+kind, caption, encodedValue, saved); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be saved", "The maintenance settings store rejected the value.")
			return
		}
		saved++
	}

	status, message := maintenanceExternalOutcome(kind)
	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, payload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The maintenance operation could not be recorded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The maintenance operation could not be committed.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"kind":        kind,
		"operationId": operationID,
		"job":         map[string]any{"id": operationID, "kind": kind, "status": status},
		"status":      status,
		"outcome":     status,
		"saved":       saved,
		"message":     message,
	})
}

func (s *Server) handleIntegrityCheck(w http.ResponseWriter, r *http.Request, operator *sessionContext, kind string, payload map[string]any) {
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The integrity check could not be opened.")
		return
	}
	defer tx.Rollback()

	checks := make([]maintenanceCheck, 0, len(maintenanceIntegrityTables))
	for _, table := range maintenanceIntegrityTables {
		var count int64
		query := "SELECT count(*) FROM " + table + " WHERE tenant_id = $1::uuid"
		if table == "tenants" {
			query = "SELECT count(*) FROM tenants WHERE id = $1::uuid"
		}
		if err := tx.QueryRowContext(r.Context(), query, operator.TenantID).Scan(&count); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "integrity_check_failed", "Integrity check failed", "A scoped application table could not be checked.")
			return
		}
		checks = append(checks, maintenanceCheck{Table: table, Rows: count, Status: "ok"})
	}
	status := "completed"
	message := "Application-scope integrity checks completed. Physical PostgreSQL checks were not run by the API."
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["checks"] = checks
	auditPayload["scope"] = map[string]string{"tenantId": operator.TenantID, "branchId": operator.BranchID}
	auditPayload["physicalDatabaseCheck"] = "not_configured"
	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, auditPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The integrity check could not be audited.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The integrity check could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":                  kind,
		"operationId":           operationID,
		"status":                status,
		"scope":                 map[string]string{"tenantId": operator.TenantID, "branchId": operator.BranchID},
		"physicalDatabaseCheck": "not_configured",
		"checks":                checks,
		"message":               message,
	})
}

func (s *Server) maintenanceState(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if !validMaintenanceKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_maintenance_kind", "Invalid maintenance workflow", "The requested maintenance workflow is not supported.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, maintenancePermission(kind)) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The maintenance store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `SELECT caption, value, position FROM tenant_preferences WHERE tenant_id = $1::uuid AND category = $2 ORDER BY position, caption`, operator.TenantID, "maintenance:"+kind)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance settings store could not be queried.")
		return
	}
	items := make([]map[string]any, 0)
	for rows.Next() {
		var caption, value string
		var position int
		if err := rows.Scan(&caption, &value, &position); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance settings response could not be decoded.")
			return
		}
		items = append(items, map[string]any{"caption": caption, "value": value, "position": position})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance settings response could not be read.")
		return
	}
	rows.Close()

	operations := make([]maintenanceOperation, 0, 10)
	operationRows, err := tx.QueryContext(r.Context(), `
		SELECT id::text, action, payload, occurred_at::text
		FROM audit_events
		WHERE tenant_id = $1::uuid
		  AND action = $2
		  AND (branch_id IS NULL OR branch_id = NULLIF($3, '')::uuid)
		ORDER BY occurred_at DESC
		LIMIT 10`, operator.TenantID, "maintenance."+kind, operator.BranchID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance operation ledger could not be queried.")
		return
	}
	for operationRows.Next() {
		var operation maintenanceOperation
		var action string
		var rawPayload []byte
		if err := operationRows.Scan(&operation.ID, &action, &rawPayload, &operation.OccurredAt); err != nil {
			operationRows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance operation response could not be decoded.")
			return
		}
		operation.Payload = json.RawMessage(rawPayload)
		operation.Kind = kind
		var details struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(operation.Payload, &details)
		operation.Status = details.Status
		operation.Message = details.Message
		operations = append(operations, operation)
	}
	if err := operationRows.Err(); err != nil {
		operationRows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance operation response could not be read.")
		return
	}
	operationRows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance state transaction could not be committed.")
		return
	}
	var lastOperation any
	if len(operations) > 0 {
		lastOperation = operations[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "items": items, "operations": operations, "lastOperation": lastOperation})
}

func (s *Server) sessionMonitor(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.users") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "branch_required", "Branch required", "Select an operational branch before viewing sessions.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The session monitor could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `
		SELECT se.user_id::text, u.username, u.display_name,
		       se.branch_id::text, COALESCE(se.counter_id::text, ''),
		       se.created_at::text, se.last_seen_at::text, se.expires_at::text,
		       (se.token_hash = $3)
		FROM sessions se
		JOIN users u ON u.id = se.user_id AND u.tenant_id = se.tenant_id
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid AND se.expires_at > now()
		ORDER BY se.last_seen_at DESC`, operator.TenantID, operator.BranchID, operator.TokenHash)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The scoped session store could not be queried.")
		return
	}
	sessions := make([]map[string]any, 0)
	for rows.Next() {
		var userID, username, displayName, branchID, counterID, createdAt, lastSeenAt, expiresAt string
		var current bool
		if err := rows.Scan(&userID, &username, &displayName, &branchID, &counterID, &createdAt, &lastSeenAt, &expiresAt, &current); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The session response could not be decoded.")
			return
		}
		sessions = append(sessions, map[string]any{
			"userId": userID, "username": username, "displayName": displayName,
			"branchId": branchID, "counterId": counterID, "createdAt": createdAt,
			"lastSeenAt": lastSeenAt, "expiresAt": expiresAt, "current": current,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The session response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The session monitor transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenantId": operator.TenantID, "branchId": operator.BranchID, "sessions": sessions})
}

func maintenanceValue(value any) (string, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func isSensitiveMaintenanceField(caption string) bool {
	lower := strings.ToLower(caption)
	return strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token")
}

func copyMaintenancePayload(payload map[string]any) map[string]any {
	copy := make(map[string]any, len(payload)+3)
	for key, value := range payload {
		if !isSensitiveMaintenanceField(key) {
			copy[key] = value
		}
	}
	return copy
}

func recordMaintenanceOperation(r *http.Request, tx *sql.Tx, operator *sessionContext, kind, status, message string, payload map[string]any) (string, error) {
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["status"] = status
	auditPayload["message"] = message
	encoded, err := json.Marshal(auditPayload)
	if err != nil {
		return "", err
	}
	var operationID string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, $4, 'maintenance_operation', $5::jsonb)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, operator.UserID, "maintenance."+kind, encoded).Scan(&operationID)
	return operationID, err
}

// Kept as a small compatibility helper for callers that only need an audit
// event and do not need the operation identifier.
func appendMaintenanceAudit(r *http.Request, tx *sql.Tx, operator *sessionContext, kind string, payload map[string]any) error {
	_, err := recordMaintenanceOperation(r, tx, operator, kind, "saved", "Maintenance action recorded.", payload)
	return err
}

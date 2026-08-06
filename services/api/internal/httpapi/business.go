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
	"time"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

type tenantResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	LegalName string `json:"legalName"`
	Active    bool   `json:"active"`
}

type branchResponse struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Active   bool   `json:"active"`
}

type counterResponse struct {
	ID       string `json:"id"`
	BranchID string `json:"branchId"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
}

type operatorResponse struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Active      bool     `json:"active"`
	Roles       []string `json:"roles"`
	BranchID    string   `json:"branchId,omitempty"`
	CounterID   string   `json:"counterId,omitempty"`
}

type shiftResponse struct {
	ID            string `json:"id"`
	BranchID      string `json:"branchId"`
	CounterID     string `json:"counterId"`
	OperatorID    string `json:"operatorId"`
	OpenedAt      string `json:"openedAt"`
	ClosedAt      string `json:"closedAt,omitempty"`
	Status        string `json:"status"`
	OpeningAmount string `json:"openingAmount"`
	ClosingAmount string `json:"closingAmount,omitempty"`
}

type shiftAmountRequest struct {
	Amount string `json:"amount"`
}

func (s *Server) shifts(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.read") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The shift store could not be opened.")
		return
	}
	defer tx.Rollback()
	query := `SELECT id::text, branch_id::text, counter_id::text, operator_id::text, opened_at::text, COALESCE(closed_at::text, ''), status, opening_amount::text, COALESCE(closing_amount::text, '') FROM shifts WHERE tenant_id = $1::uuid AND ($2 = '' OR branch_id::text = $2) AND ($3 = '' OR counter_id::text = $3) ORDER BY opened_at DESC LIMIT 200`
	rows, err := tx.QueryContext(r.Context(), query, operator.TenantID, operator.BranchID, operator.CounterID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "shift_read_failed", "Unable to read shifts", "The shift store could not be queried.")
		return
	}
	items := make([]shiftResponse, 0)
	for rows.Next() {
		var item shiftResponse
		if err := rows.Scan(&item.ID, &item.BranchID, &item.CounterID, &item.OperatorID, &item.OpenedAt, &item.ClosedAt, &item.Status, &item.OpeningAmount, &item.ClosingAmount); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "shift_read_failed", "Unable to read shifts", "The shift response could not be decoded.")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "shift_read_failed", "Unable to read shifts", "The shift response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "shift_read_failed", "Unable to read shifts", "The shift transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shifts": items})
}

type syncEvent struct {
	EventID        string          `json:"eventId"`
	Aggregate      string          `json:"aggregate"`
	AggregateID    string          `json:"aggregateId"`
	TenantID       string          `json:"tenantId"`
	BranchID       string          `json:"branchId"`
	CounterID      string          `json:"counterId"`
	OperatorID     string          `json:"operatorId"`
	OccurredAt     string          `json:"occurredAt"`
	IdempotencyKey string          `json:"idempotencyKey"`
	SchemaVersion  int             `json:"schemaVersion"`
	Payload        json.RawMessage `json:"payload"`
}

type syncPushRequest struct {
	Events []syncEvent `json:"events"`
}

type syncPullResponse struct {
	Events     []syncEvent `json:"events"`
	NextCursor int64       `json:"nextCursor"`
}

type salePayload struct {
	DocumentNumber string                 `json:"documentNumber"`
	Status         string                 `json:"status"`
	TotalAmount    string                 `json:"totalAmount"`
	OccurredAt     string                 `json:"occurredAt"`
	Rows           []inventoryRowPayload  `json:"rows"`
	PricingRequest *pricingPreviewRequest `json:"pricingRequest"`
}

type inventoryRowPayload struct {
	ItemLegacyID   string          `json:"itemLegacyId"`
	ItemName       string          `json:"itemName"`
	Quantity       json.RawMessage `json:"quantity"`
	Direction      string          `json:"direction"`
	AdjustmentSign *int            `json:"adjustmentSign"`
	GodownID       string          `json:"godownId"`
	BatchID        string          `json:"batchId"`
	BatchNumber    string          `json:"batchNumber"`
	ExpiryDate     string          `json:"expiryDate"`
	UnitCost       string          `json:"unitCost"`
}

type inventoryPayload struct {
	Rows           []inventoryRowPayload `json:"rows"`
	ItemLegacyID   string                `json:"itemLegacyId"`
	ItemName       string                `json:"itemName"`
	Quantity       json.RawMessage       `json:"quantity"`
	Direction      string                `json:"direction"`
	AdjustmentSign *int                  `json:"adjustmentSign"`
	GodownID       string                `json:"godownId"`
	BatchID        string                `json:"batchId"`
	BatchNumber    string                `json:"batchNumber"`
	ExpiryDate     string                `json:"expiryDate"`
	UnitCost       string                `json:"unitCost"`
}

type masterRecordResponse struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	LegacyID  string                 `json:"legacyId,omitempty"`
	Code      string                 `json:"code"`
	Name      string                 `json:"name"`
	Payload   json.RawMessage        `json:"payload"`
	Active    bool                   `json:"active"`
	CreatedAt string                 `json:"createdAt"`
	UpdatedAt string                 `json:"updatedAt"`
	Suppliers []itemSupplierResponse `json:"suppliers,omitempty"`
}

type masterRecordRequest struct {
	LegacyID string          `json:"legacyId"`
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Payload  json.RawMessage `json:"payload"`
	Active   *bool           `json:"active"`
}

func (s *Server) tenants(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "tenant.read") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The tenant store could not be opened.")
		return
	}
	defer tx.Rollback()
	var tenant tenantResponse
	if err := tx.QueryRowContext(r.Context(), `SELECT id::text, code, legal_name, active FROM tenants WHERE id = $1::uuid`, operator.TenantID).Scan(&tenant.ID, &tenant.Code, &tenant.LegalName, &tenant.Active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "tenant_not_found", "Tenant not found", "The authenticated tenant no longer exists.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "tenant_read_failed", "Unable to read tenant", "The tenant store could not be queried.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tenant_read_failed", "Unable to read tenant", "The tenant transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": []tenantResponse{tenant}})
}

func (s *Server) branches(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "branch.read") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The branch store could not be opened.")
		return
	}
	defer tx.Rollback()
	query := `SELECT b.id::text, b.code, b.name, b.timezone, b.active FROM branches b WHERE b.tenant_id = $1::uuid`
	args := []any{operator.TenantID}
	if !hasTenantAdminRole(operator.Roles) {
		query += ` AND EXISTS (SELECT 1 FROM user_branch_assignments a WHERE a.user_id = $2::uuid AND a.tenant_id = b.tenant_id AND a.branch_id = b.id)`
		args = append(args, operator.UserID)
	}
	query += ` ORDER BY b.code`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "branch_read_failed", "Unable to read branches", "The branch store could not be queried.")
		return
	}
	branches := make([]branchResponse, 0)
	for rows.Next() {
		var item branchResponse
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Timezone, &item.Active); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "branch_read_failed", "Unable to read branches", "The branch response could not be decoded.")
			return
		}
		if !scopeAllowed(operator, "branch", item.ID) {
			continue
		}
		branches = append(branches, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "branch_read_failed", "Unable to read branches", "The branch response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "branch_read_failed", "Unable to read branches", "The branch transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"branches": branches})
}

func (s *Server) counters(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "branch.read") {
		return
	}
	branchID := strings.TrimSpace(r.URL.Query().Get("branchId"))
	if branchID == "" {
		branchID = operator.BranchID
	}
	if branchID == "" {
		writeProblem(w, http.StatusBadRequest, "branch_required", "Branch is required", "Select a branch before loading counters.")
		return
	}
	if !s.requireScope(r, w, operator, "branch", branchID) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The counter store could not be opened.")
		return
	}
	defer tx.Rollback()
	query := `SELECT c.id::text, c.branch_id::text, c.code, c.name, c.active FROM counters c WHERE c.tenant_id = $1::uuid AND c.branch_id = $2::uuid`
	args := []any{operator.TenantID, branchID}
	if !hasTenantAdminRole(operator.Roles) {
		query += ` AND EXISTS (SELECT 1 FROM user_counter_assignments a WHERE a.user_id = $3::uuid AND a.tenant_id = c.tenant_id AND a.counter_id = c.id)`
		args = append(args, operator.UserID)
	}
	query += ` ORDER BY c.code`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "counter_read_failed", "Unable to read counters", "The counter store could not be queried.")
		return
	}
	counters := make([]counterResponse, 0)
	for rows.Next() {
		var item counterResponse
		if err := rows.Scan(&item.ID, &item.BranchID, &item.Code, &item.Name, &item.Active); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "counter_read_failed", "Unable to read counters", "The counter response could not be decoded.")
			return
		}
		counters = append(counters, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "counter_read_failed", "Unable to read counters", "The counter response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "counter_read_failed", "Unable to read counters", "The counter transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"counters": counters})
}

func (s *Server) masterRecords(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if _, ok := canonicalMasterSpec(kind); ok {
		s.canonicalMasterRecords(w, r, kind)
		return
	}
	if !validMasterKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "The requested master-data kind is not supported.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.read") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	query := `SELECT id::text, kind, COALESCE(legacy_id, ''), code, name, payload, active, created_at::text, updated_at::text FROM master_records WHERE tenant_id = $1::uuid AND kind = $2`
	args := []any{operator.TenantID, kind}
	if search != "" {
		query += ` AND (code ILIKE $3 OR name ILIKE $3)`
		args = append(args, "%"+search+"%")
	}
	query += ` ORDER BY name, code LIMIT 500`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The master-data query failed.")
		return
	}
	items := make([]masterRecordResponse, 0)
	for rows.Next() {
		var item masterRecordResponse
		if err := rows.Scan(&item.ID, &item.Kind, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The master-data response could not be decoded.")
			return
		}
		items = append(items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The master-data response could not be read.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": items})
}

func (s *Server) createMasterRecord(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if _, ok := canonicalMasterSpec(kind); ok {
		s.createCanonicalMasterRecord(w, r, kind)
		return
	}
	if !validMasterKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "The requested master-data kind is not supported.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.write") {
		return
	}
	var request masterRecordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil || strings.TrimSpace(request.Code) == "" || strings.TrimSpace(request.Name) == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_record", "Invalid master-data record", "Code and name are required.")
		return
	}
	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	var item masterRecordResponse
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO master_records (tenant_id, kind, legacy_id, code, name, payload, active)
		VALUES ($1::uuid, $2, NULLIF($3, ''), $4, $5, $6::jsonb, $7)
		RETURNING id::text, kind, COALESCE(legacy_id, ''), code, name, payload, active, created_at::text, updated_at::text
	`, operator.TenantID, kind, strings.TrimSpace(request.LegacyID), strings.TrimSpace(request.Code), strings.TrimSpace(request.Name), request.Payload, active).Scan(&item.ID, &item.Kind, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		writeProblem(w, http.StatusConflict, "master_create_failed", "Master-data record was not created", "The code may already exist in this tenant.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_create_failed", "Master-data record was not created", "The master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateMasterRecord(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if _, ok := canonicalMasterSpec(kind); ok {
		s.updateCanonicalMasterRecord(w, r, kind)
		return
	}
	if !validMasterKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "The requested master-data kind is not supported.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.write") {
		return
	}
	var request masterRecordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_master_record", "Invalid master-data record", "The update body could not be parsed.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	set := []string{"updated_at = now()"}
	args := []any{}
	if strings.TrimSpace(request.Code) != "" {
		args = append(args, strings.TrimSpace(request.Code))
		set = append(set, fmt.Sprintf("code = $%d", len(args)))
	}
	if strings.TrimSpace(request.Name) != "" {
		args = append(args, strings.TrimSpace(request.Name))
		set = append(set, fmt.Sprintf("name = $%d", len(args)))
	}
	if len(request.Payload) > 0 {
		args = append(args, request.Payload)
		set = append(set, fmt.Sprintf("payload = $%d::jsonb", len(args)))
	}
	if request.Active != nil {
		args = append(args, *request.Active)
		set = append(set, fmt.Sprintf("active = $%d", len(args)))
	}
	args = append(args, operator.TenantID, kind, r.PathValue("id"))
	query := fmt.Sprintf(`UPDATE master_records SET %s WHERE tenant_id = $%d::uuid AND kind = $%d AND id = $%d::uuid RETURNING id::text, kind, COALESCE(legacy_id, ''), code, name, payload, active, created_at::text, updated_at::text`, strings.Join(set, ", "), len(args)-2, len(args)-1, len(args))
	var item masterRecordResponse
	if err := tx.QueryRowContext(r.Context(), query, args...).Scan(&item.ID, &item.Kind, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The record is outside the authenticated tenant scope.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_update_failed", "Master-data record was not updated", "The master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func validMasterKind(kind string) bool {
	known := map[string]struct{}{
		"customer": {}, "customer-category": {}, "sale-promotion": {}, "customer-sector": {},
		"supplier": {}, "supplier-category": {}, "item": {}, "manufacturer": {}, "item-class": {},
		"item-category": {}, "generic-item": {}, "item-basic-data": {}, "price-policy": {},
		"item-alert": {}, "sales-tax-schedule": {}, "pct-codes": {}, "generic-item-type": {},
		"item-thickness": {}, "lock-reason": {}, "category-segment": {}, "manufacturer-type": {},
		"sale-template": {}, "tax-category": {}, "user": {}, "category": {}, "template": {},
		"godown": {}, "item-group": {}, "item_group": {}, "items": {},
	}
	_, ok := known[kind]
	return ok
}

func (s *Server) operators(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.users") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The operator store could not be opened.")
		return
	}
	defer tx.Rollback()
	query := `
		SELECT u.id::text, u.username, u.display_name, u.active,
		       COALESCE(array_to_string(array_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), ','), ''),
		       COALESCE((SELECT a.branch_id::text FROM user_branch_assignments a WHERE a.user_id = u.id AND a.tenant_id = u.tenant_id ORDER BY a.branch_id LIMIT 1), ''),
		       COALESCE((SELECT a.counter_id::text FROM user_counter_assignments a WHERE a.user_id = u.id AND a.tenant_id = u.tenant_id ORDER BY a.counter_id LIMIT 1), '')
		FROM users u
		LEFT JOIN user_memberships m ON m.user_id = u.id AND m.tenant_id = u.tenant_id
		LEFT JOIN roles r ON r.id = m.role_id AND r.tenant_id = m.tenant_id
		WHERE u.tenant_id = $1::uuid`
	args := []any{operator.TenantID}
	if !hasTenantAdminRole(operator.Roles) {
		if operator.BranchID != "" {
			query += ` AND EXISTS (SELECT 1 FROM user_branch_assignments a WHERE a.user_id = u.id AND a.tenant_id = u.tenant_id AND a.branch_id = $2::uuid)`
			args = append(args, operator.BranchID)
		}
		if operator.CounterID != "" {
			placeholder := len(args) + 1
			query += ` AND EXISTS (SELECT 1 FROM user_counter_assignments a WHERE a.user_id = u.id AND a.tenant_id = u.tenant_id AND a.counter_id = $` + strconv.Itoa(placeholder) + `::uuid)`
			args = append(args, operator.CounterID)
		}
	}
	query += ` GROUP BY u.id, u.username, u.display_name, u.active ORDER BY u.username`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "operator_read_failed", "Unable to read operators", "The operator store could not be queried.")
		return
	}
	operators := make([]operatorResponse, 0)
	for rows.Next() {
		var item operatorResponse
		var roleCodes string
		if err := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.Active, &roleCodes, &item.BranchID, &item.CounterID); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "operator_read_failed", "Unable to read operators", "The operator response could not be decoded.")
			return
		}
		if roleCodes != "" {
			item.Roles = strings.Split(roleCodes, ",")
		} else {
			item.Roles = []string{}
		}
		operators = append(operators, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "operator_read_failed", "Unable to read operators", "The operator response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "operator_read_failed", "Unable to read operators", "The operator transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operators": operators})
}

func (s *Server) openShift(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.write") {
		return
	}
	if operator.BranchID == "" || operator.CounterID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch and counter required", "Select a branch and counter before opening a shift.")
		return
	}
	var request shiftAmountRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The opening amount could not be parsed.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The shift store could not be opened.")
		return
	}
	defer tx.Rollback()
	var shift shiftResponse
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO shifts (tenant_id, branch_id, counter_id, operator_id, opened_at, status, opening_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, now(), 'open', COALESCE(NULLIF($5, '')::numeric, 0))
		RETURNING id::text, branch_id::text, counter_id::text, operator_id::text, opened_at::text, status, opening_amount::text
	`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, request.Amount).Scan(&shift.ID, &shift.BranchID, &shift.CounterID, &shift.OperatorID, &shift.OpenedAt, &shift.Status, &shift.OpeningAmount)
	if err != nil {
		writeProblem(w, http.StatusConflict, "shift_already_open", "Shift could not be opened", "This counter already has an open shift or the amount is invalid.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'shift.opened', 'shift', $4::uuid, jsonb_build_object('openingAmount', $5::text))
	`, operator.TenantID, operator.BranchID, operator.UserID, shift.ID, request.Amount); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record shift", "The shift audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "shift_commit_failed", "Unable to open shift", "The shift transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusCreated, shift)
}

func (s *Server) closeShift(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select a branch before closing a shift.")
		return
	}
	shiftID := r.PathValue("id")
	if shiftID == "" {
		writeProblem(w, http.StatusBadRequest, "shift_required", "Shift is required", "Provide a shift identifier.")
		return
	}
	var request shiftAmountRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The closing amount could not be parsed.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The shift store could not be opened.")
		return
	}
	defer tx.Rollback()
	var shift shiftResponse
	query := `
		UPDATE shifts
		SET closed_at = now(), status = 'closed', closing_amount = COALESCE(NULLIF($4, '')::numeric, 0)
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND status = 'open'`
	args := []any{shiftID, operator.TenantID, operator.BranchID, request.Amount}
	if !hasTenantAdminRole(operator.Roles) {
		query += ` AND operator_id = $5::uuid`
		args = append(args, operator.UserID)
	}
	query += ` RETURNING id::text, branch_id::text, counter_id::text, operator_id::text, opened_at::text, closed_at::text, status, opening_amount::text, closing_amount::text`
	err = tx.QueryRowContext(r.Context(), query, args...).Scan(&shift.ID, &shift.BranchID, &shift.CounterID, &shift.OperatorID, &shift.OpenedAt, &shift.ClosedAt, &shift.Status, &shift.OpeningAmount, &shift.ClosingAmount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusConflict, "shift_not_open", "Shift could not be closed", "The shift is missing, already closed, or outside the authenticated scope.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "shift_close_failed", "Shift could not be closed", "The closing amount is invalid.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'shift.closed', 'shift', $4::uuid, jsonb_build_object('closingAmount', $5::text))
	`, operator.TenantID, operator.BranchID, operator.UserID, shift.ID, request.Amount); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record shift", "The shift audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "shift_commit_failed", "Unable to close shift", "The shift transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, shift)
}

func (s *Server) createSale(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sales.write") {
		return
	}
	var event syncEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&event); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The sale event could not be parsed.")
		return
	}
	if event.Aggregate == "" {
		event.Aggregate = "sale"
	}
	if event.Aggregate != "sale" {
		writeProblem(w, http.StatusBadRequest, "invalid_aggregate", "Invalid transaction type", "This endpoint accepts sale events only.")
		return
	}
	if err := validateEventScope(event, operator); err != nil {
		writeProblem(w, http.StatusForbidden, "scope_mismatch", "Transaction scope rejected", err.Error())
		return
	}
	if event.OperatorID == "" {
		event.OperatorID = operator.UserID
	}
	var payload salePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_sale_payload", "Invalid sale payload", "The sale payload must be JSON.")
		return
	}
	if payload.Status == "" {
		payload.Status = "posted"
	}
	if payload.OccurredAt == "" {
		payload.OccurredAt = event.OccurredAt
	}
	if payload.DocumentNumber == "" {
		payload.DocumentNumber = ""
	}
	if event.EventID == "" || event.AggregateID == "" || event.IdempotencyKey == "" || event.OccurredAt == "" {
		writeProblem(w, http.StatusBadRequest, "missing_event_fields", "Missing transaction fields", "eventId, aggregateId, idempotencyKey, and occurredAt are required.")
		return
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = 1
	}

	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The transaction store could not be opened.")
		return
	}
	defer tx.Rollback()
	inserted, conflict, err := insertSyncEvent(r.Context(), tx, event, operator)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "transaction_rejected", "Transaction rejected", "The transaction could not be stored.")
		return
	}
	if conflict {
		_ = tx.Commit()
		writeProblem(w, http.StatusConflict, "transaction_conflict", "Transaction conflict", "The idempotency key already belongs to a different event.")
		return
	}
	if inserted {
		if err := projectEvent(r.Context(), tx, event); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "sale_projection_failed", "Sale projection failed", "The sale event was not projected into the sales ledger.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "transaction_commit_failed", "Transaction commit failed", "The transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": inserted, "duplicate": !inserted, "eventId": event.EventID, "aggregateId": event.AggregateID})
}

func (s *Server) createTransaction(aggregate string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operator := currentSession(r)
		permission := "sales.write"
		if aggregate == "receiving" || aggregate == "return" || aggregate == "purchase_order" {
			permission = "purchases.write"
		} else if aggregate == "inventory" {
			permission = "maintenance.write"
		}
		if !s.requirePermission(r, w, operator, permission) {
			return
		}
		var event syncEvent
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&event); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The transaction event could not be parsed.")
			return
		}
		if event.Aggregate == "" {
			event.Aggregate = aggregate
		}
		if event.Aggregate != aggregate {
			writeProblem(w, http.StatusBadRequest, "invalid_aggregate", "Invalid transaction type", "The transaction endpoint received a different aggregate type.")
			return
		}
		if err := validateEventScope(event, operator); err != nil {
			writeProblem(w, http.StatusForbidden, "scope_mismatch", "Transaction scope rejected", err.Error())
			return
		}
		if event.OperatorID == "" {
			event.OperatorID = operator.UserID
		}
		if event.SchemaVersion == 0 {
			event.SchemaVersion = 1
		}
		if event.EventID == "" || event.AggregateID == "" || event.IdempotencyKey == "" || event.OccurredAt == "" {
			writeProblem(w, http.StatusBadRequest, "missing_event_fields", "Missing transaction fields", "eventId, aggregateId, idempotencyKey, and occurredAt are required.")
			return
		}
		tx, err := s.beginScopedTx(r.Context(), operator)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The transaction store could not be opened.")
			return
		}
		defer tx.Rollback()
		inserted, conflict, err := insertSyncEvent(r.Context(), tx, event, operator)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "transaction_rejected", "Transaction rejected", "The transaction could not be stored.")
			return
		}
		if conflict {
			_ = tx.Commit()
			writeProblem(w, http.StatusConflict, "transaction_conflict", "Transaction conflict", "The idempotency key already belongs to a different event.")
			return
		}
		if inserted {
			if err := projectEvent(r.Context(), tx, event); err != nil {
				writeProblem(w, http.StatusUnprocessableEntity, "transaction_projection_failed", "Transaction projection failed", "The transaction was not projected into the central ledger.")
				return
			}
		}
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "transaction_commit_failed", "Transaction commit failed", "The transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": inserted, "duplicate": !inserted, "eventId": event.EventID, "aggregateId": event.AggregateID})
	})
}

func nextDocumentNumber(ctx context.Context, tx *sql.Tx, tenantID, branchID string) (string, error) {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_sequences (tenant_id, branch_id, sequence_name, next_value)
		VALUES ($1::uuid, $2::uuid, 'sale', 1)
		ON CONFLICT (tenant_id, branch_id, sequence_name) DO NOTHING
	`, tenantID, branchID); err != nil {
		return "", err
	}
	var value int64
	if err := tx.QueryRowContext(ctx, `
		UPDATE tenant_sequences
		SET next_value = next_value + 1, updated_at = now()
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND sequence_name = 'sale'
		RETURNING next_value - 1
	`, tenantID, branchID).Scan(&value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%06d", strings.ReplaceAll(branchID, "-", "")[:8], value), nil
}

func (s *Server) pushSync(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sync.push") {
		return
	}
	var request syncPushRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The synchronization envelope could not be parsed.")
		return
	}
	if len(request.Events) == 0 || len(request.Events) > 500 {
		writeProblem(w, http.StatusBadRequest, "invalid_event_batch", "Invalid event batch", "Provide between 1 and 500 events.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The synchronization store could not be opened.")
		return
	}
	defer tx.Rollback()
	accepted, duplicates, conflicts := 0, 0, 0
	for _, event := range request.Events {
		if err := validateSyncEventScope(r.Context(), tx, event, operator); err != nil {
			writeProblem(w, http.StatusForbidden, "scope_mismatch", "Synchronization scope rejected", err.Error())
			return
		}
		if event.OperatorID == "" {
			event.OperatorID = operator.UserID
		}
		if event.SchemaVersion == 0 {
			event.SchemaVersion = 1
		}
		inserted, conflict, err := insertSyncEvent(r.Context(), tx, event, operator)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "event_rejected", "Event rejected", "A synchronization event could not be stored.")
			return
		}
		if conflict {
			conflicts++
		} else if inserted {
			if err := projectEvent(r.Context(), tx, event); err != nil {
				writeProblem(w, http.StatusUnprocessableEntity, "event_projection_failed", "Event projection failed", "A synchronization event was stored but its business projection could not be committed.")
				return
			}
			accepted++
		} else {
			duplicates++
		}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "sync_commit_failed", "Synchronization commit failed", "The synchronization batch could not be committed.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"accepted": accepted, "duplicates": duplicates, "conflicts": conflicts})
}

func (s *Server) pullSync(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sync.pull") {
		return
	}
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	if cursor < 0 {
		cursor = 0
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The synchronization store could not be opened.")
		return
	}
	defer tx.Rollback()
	query := `
		SELECT sequence_no, event_id::text, aggregate, aggregate_id::text, tenant_id::text, branch_id::text,
		       COALESCE(counter_id::text, ''), COALESCE(operator_id::text, ''), occurred_at::text,
		       idempotency_key, schema_version, payload
		FROM sync_events
		WHERE tenant_id = $1::uuid AND sequence_no > $2`
	args := []any{operator.TenantID, cursor}
	if operator.BranchID != "" {
		query += ` AND branch_id = $3::uuid`
		args = append(args, operator.BranchID)
	}
	if operator.CounterID != "" && !hasTenantAdminRole(operator.Roles) {
		placeholder := len(args) + 1
		query += ` AND counter_id = $` + strconv.Itoa(placeholder) + `::uuid`
		args = append(args, operator.CounterID)
	}
	query += ` ORDER BY sequence_no LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "sync_read_failed", "Unable to read synchronization events", "The central synchronization store could not be queried.")
		return
	}
	response := syncPullResponse{Events: make([]syncEvent, 0), NextCursor: cursor}
	for rows.Next() {
		var sequence int64
		var event syncEvent
		var occurredAt string
		var payload []byte
		if err := rows.Scan(&sequence, &event.EventID, &event.Aggregate, &event.AggregateID, &event.TenantID, &event.BranchID, &event.CounterID, &event.OperatorID, &occurredAt, &event.IdempotencyKey, &event.SchemaVersion, &payload); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "sync_read_failed", "Unable to read synchronization events", "The synchronization response could not be decoded.")
			return
		}
		event.OccurredAt = occurredAt
		event.Payload = json.RawMessage(payload)
		response.Events = append(response.Events, event)
		response.NextCursor = sequence
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "sync_read_failed", "Unable to read synchronization events", "The synchronization response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "sync_read_failed", "Unable to read synchronization events", "The synchronization transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) conflicts(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sync.review") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The conflict store could not be opened.")
		return
	}
	defer tx.Rollback()
	query := `
		SELECT id::text, entity_type, entity_id::text, tenant_id::text, branch_id::text, status,
		       local_value, server_value, created_at::text
		FROM conflict_records WHERE tenant_id = $1::uuid`
	args := []any{operator.TenantID}
	if operator.BranchID != "" {
		query += ` AND branch_id = $2::uuid`
		args = append(args, operator.BranchID)
	}
	query += ` ORDER BY created_at DESC LIMIT 500`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "conflict_read_failed", "Unable to read conflicts", "The conflict store could not be queried.")
		return
	}
	type conflict struct {
		ID          string          `json:"id"`
		EntityType  string          `json:"entityType"`
		EntityID    string          `json:"entityId"`
		TenantID    string          `json:"tenantId"`
		BranchID    string          `json:"branchId"`
		Status      string          `json:"status"`
		LocalValue  json.RawMessage `json:"localValue"`
		ServerValue json.RawMessage `json:"serverValue"`
		CreatedAt   string          `json:"createdAt"`
	}
	conflicts := make([]conflict, 0)
	for rows.Next() {
		var item conflict
		var localValue, serverValue []byte
		if err := rows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.TenantID, &item.BranchID, &item.Status, &localValue, &serverValue, &item.CreatedAt); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "conflict_read_failed", "Unable to read conflicts", "The conflict response could not be decoded.")
			return
		}
		item.LocalValue = json.RawMessage(localValue)
		item.ServerValue = json.RawMessage(serverValue)
		conflicts = append(conflicts, item)
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "conflict_read_failed", "Unable to read conflicts", "The conflict transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}

func (s *Server) resolveConflict(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "sync.review") {
		return
	}
	conflictID := r.PathValue("id")
	if conflictID == "" {
		writeProblem(w, http.StatusBadRequest, "conflict_required", "Conflict is required", "Provide a conflict identifier.")
		return
	}
	var request struct {
		Status     string          `json:"status"`
		Resolution json.RawMessage `json:"resolution"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The conflict resolution could not be parsed.")
		return
	}
	if request.Status != "resolved" && request.Status != "dismissed" {
		writeProblem(w, http.StatusBadRequest, "invalid_resolution_status", "Invalid resolution status", "Use resolved or dismissed.")
		return
	}
	if len(request.Resolution) == 0 {
		request.Resolution = json.RawMessage(`{}`)
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The conflict store could not be opened.")
		return
	}
	defer tx.Rollback()
	var resolvedID string
	query := `
		UPDATE conflict_records
		SET status = $1, resolution = $2::jsonb, resolved_at = now()
		WHERE id = $3::uuid AND tenant_id = $4::uuid AND status = 'open'`
	args := []any{request.Status, []byte(request.Resolution), conflictID, operator.TenantID}
	if operator.BranchID != "" {
		query += ` AND branch_id = $5::uuid`
		args = append(args, operator.BranchID)
	}
	query += ` RETURNING id::text`
	if err := tx.QueryRowContext(r.Context(), query, args...).Scan(&resolvedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusConflict, "conflict_not_open", "Conflict is not open", "The conflict is missing, already resolved, or outside the authenticated scope.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "conflict_update_failed", "Unable to resolve conflict", "The conflict store rejected the resolution.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, 'conflict.resolved', 'conflict', $4::uuid, $5::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, resolvedID, []byte(request.Resolution)); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record resolution", "The conflict audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "conflict_commit_failed", "Unable to resolve conflict", "The conflict transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": resolvedID, "status": request.Status})
}

// projectEvent applies the server-authoritative business projection for event
// types that have a concrete central ledger in this migration wave. The event
// is inserted first in the same transaction, so a projection failure rolls
// back the event and makes a retry safe.
func projectEvent(ctx context.Context, tx *sql.Tx, event syncEvent) error {
	switch event.Aggregate {
	case "sale":
		var payload salePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		if payload.Status == "" {
			payload.Status = "posted"
		}
		if payload.Status != "draft" && payload.Status != "posted" && payload.Status != "voided" {
			return fmt.Errorf("invalid sale status %q", payload.Status)
		}
		if payload.OccurredAt == "" {
			payload.OccurredAt = event.OccurredAt
		}
		if event.CounterID == "" || event.OperatorID == "" {
			return errors.New("sale event requires counter and operator scope")
		}
		if err := validateSalePricing(payload); err != nil {
			return err
		}
		if payload.DocumentNumber == "" {
			var err error
			payload.DocumentNumber, err = nextDocumentNumber(ctx, tx, event.TenantID, event.BranchID)
			if err != nil {
				return err
			}
		}
		var projectedID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO sales_documents (id, tenant_id, branch_id, counter_id, operator_id, document_number, status, total_amount, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6, $7, NULLIF($8, '')::numeric, $9::timestamptz)
			ON CONFLICT (tenant_id, branch_id, document_number) DO NOTHING
			RETURNING id::text
		`, event.AggregateID, event.TenantID, event.BranchID, event.CounterID, event.OperatorID, payload.DocumentNumber, payload.Status, payload.TotalAmount, payload.OccurredAt).Scan(&projectedID)
		if errors.Is(err, sql.ErrNoRows) {
			var existingID, existingStatus string
			if lookupErr := tx.QueryRowContext(ctx, `SELECT id::text, status FROM sales_documents WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND document_number = $3`, event.TenantID, event.BranchID, payload.DocumentNumber).Scan(&existingID, &existingStatus); lookupErr != nil {
				return lookupErr
			}
			if existingStatus == "posted" && payload.Status == "draft" {
				return fmt.Errorf("posted document %q cannot be returned to draft", payload.DocumentNumber)
			}
			if existingStatus == "voided" && payload.Status != "voided" {
				return fmt.Errorf("voided document %q cannot be changed", payload.DocumentNumber)
			}
			if existingID != event.AggregateID && existingStatus != "draft" {
				return fmt.Errorf("document number %q already belongs to another event", payload.DocumentNumber)
			}
			projectedID = existingID
			if existingID != event.AggregateID || existingStatus != payload.Status {
				if _, updateErr := tx.ExecContext(ctx, `
					UPDATE sales_documents
					SET counter_id = $1::uuid, operator_id = $2::uuid, status = $3,
					    total_amount = NULLIF($4, '')::numeric, occurred_at = $5::timestamptz
					WHERE id = $6::uuid AND tenant_id = $7::uuid AND branch_id = $8::uuid
				`, event.CounterID, event.OperatorID, payload.Status, payload.TotalAmount, payload.OccurredAt, existingID, event.TenantID, event.BranchID); updateErr != nil {
					return updateErr
				}
			}
		}
		if err != nil {
			return err
		}
		if payload.Status == "posted" && projectedID != "" && len(payload.Rows) > 0 {
			encoded, marshalErr := json.Marshal(inventoryPayload{Rows: payload.Rows})
			if marshalErr != nil {
				return marshalErr
			}
			inventoryEvent := event
			inventoryEvent.Payload = encoded
			if err := projectInventoryEvent(ctx, tx, inventoryEvent, "out"); err != nil {
				return err
			}
		}
		return nil
	case "receiving":
		return projectInventoryEvent(ctx, tx, event, "in")
	case "return":
		return projectInventoryEvent(ctx, tx, event, "out")
	case "inventory":
		return projectInventoryEvent(ctx, tx, event, "adjustment")
	case "sale_return":
		// A sale return puts the returned goods back into branch stock.
		return projectInventoryEvent(ctx, tx, event, "in")
	case "quotation", "refused_sale", "purchase_order":
		// These documents remain immutable events until their workflow-specific
		// ledgers are added; retaining them here keeps retries and reporting safe.
		return nil
	default:
		// Other aggregate projections are added in later parity waves; the
		// immutable event remains available for replay and migration tooling.
		return nil
	}
}

// validateSalePricing replays the client-visible pricing request inside the
// same transaction that stores the immutable event. Clients may omit the
// optional request for older/offline envelopes, but when it is present the
// server rejects forged totals or altered discount/tax inputs.
func validateSalePricing(payload salePayload) error {
	if payload.PricingRequest == nil {
		return nil
	}
	request, err := payload.PricingRequest.toPricingRequest()
	if err != nil {
		return fmt.Errorf("pricing request: %w", err)
	}
	result, err := pricing.Calculate(request)
	if err != nil {
		return fmt.Errorf("pricing calculation: %w", err)
	}
	if strings.TrimSpace(payload.TotalAmount) == "" {
		return nil
	}
	expected, err := parseMoney(payload.TotalAmount)
	if err != nil {
		return fmt.Errorf("total amount: %w", err)
	}
	if expected != result.Total {
		return fmt.Errorf("total amount %s does not match the validated pricing total %s", payload.TotalAmount, formatMoney(result.Total))
	}
	return nil
}

// projectInventoryEvent keeps receiving, returns, and adjustments tied to the
// immutable sync event while exposing a tenant/branch-scoped stock ledger. The
// payload accepts the legacy grid's row shape; blank rows are ignored so the
// UI can preserve its editable empty-row behavior.
func projectInventoryEvent(ctx context.Context, tx *sql.Tx, event syncEvent, defaultDirection string) error {
	var payload inventoryPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	rows := payload.Rows
	if len(rows) == 0 && (payload.ItemLegacyID != "" || payload.ItemName != "") {
		rows = []inventoryRowPayload{{
			ItemLegacyID: payload.ItemLegacyID, ItemName: payload.ItemName, Quantity: payload.Quantity,
			Direction: payload.Direction, AdjustmentSign: payload.AdjustmentSign,
			GodownID: payload.GodownID, BatchID: payload.BatchID,
			BatchNumber: payload.BatchNumber, ExpiryDate: payload.ExpiryDate, UnitCost: payload.UnitCost,
		}}
	}
	for rowIndex, row := range rows {
		item := strings.TrimSpace(row.ItemLegacyID)
		if item == "" {
			item = strings.TrimSpace(row.ItemName)
		}
		if item == "" {
			continue
		}
		quantity := jsonScalarText(row.Quantity)
		if quantity == "" {
			return fmt.Errorf("item %q quantity is required", item)
		}
		direction := strings.TrimSpace(row.Direction)
		if direction == "" {
			direction = defaultDirection
		}
		if direction != "in" && direction != "out" && direction != "adjustment" {
			return fmt.Errorf("invalid inventory direction %q", direction)
		}
		adjustmentSign, err := inventoryAdjustmentSign(direction, row.AdjustmentSign)
		if err != nil {
			return fmt.Errorf("item %q: %w", item, err)
		}
		if direction == "adjustment" && !stockPayloadHasIdentity(row) {
			return fmt.Errorf("item %q adjustment requires godownId and batchNumber or batchId", item)
		}
		if err := validateInventoryQuantity(quantity); err != nil {
			return fmt.Errorf("item %q quantity: %w", item, err)
		}
		if stockPayloadHasIdentity(row) {
			parsedQuantity, err := parseStockQuantity(quantity)
			if err != nil {
				return fmt.Errorf("item %q quantity: %w", item, err)
			}
			quantity = formatStockQuantity(parsedQuantity)
			if err := projectStockInventoryRow(ctx, tx, event, row, item, quantity, direction, adjustmentSign, rowIndex); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO inventory_movements (tenant_id, branch_id, source_event_id, item_legacy_id, quantity, direction, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, NULLIF($5, '')::numeric, $6, $7::timestamptz)
			ON CONFLICT (tenant_id, source_event_id, item_legacy_id) DO NOTHING
		`, event.TenantID, event.BranchID, event.EventID, item, quantity, direction, event.OccurredAt); err != nil {
			return err
		}
	}
	return nil
}

func inventoryAdjustmentSign(direction string, sign *int) (int, error) {
	if sign != nil && *sign != -1 && *sign != 1 {
		return 0, errors.New("adjustmentSign must be -1 or 1")
	}
	if direction == "adjustment" {
		if sign == nil {
			return 0, errors.New("adjustmentSign is required for adjustment movements")
		}
		return *sign, nil
	}
	if sign != nil && *sign != 1 {
		return 0, errors.New("adjustmentSign must be 1 for in and out movements")
	}
	return 1, nil
}

func stockPayloadHasIdentity(row inventoryRowPayload) bool {
	return strings.TrimSpace(row.GodownID) != "" &&
		(strings.TrimSpace(row.BatchID) != "" || strings.TrimSpace(row.BatchNumber) != "")
}

func projectStockInventoryRow(ctx context.Context, tx *sql.Tx, event syncEvent, row inventoryRowPayload, itemKey, quantity, direction string, adjustmentSign, rowCount int) error {
	if strings.TrimSpace(row.GodownID) == "" || !documentUUIDPattern.MatchString(strings.TrimSpace(row.GodownID)) {
		return fmt.Errorf("item %q stock projection requires a UUID godownId", itemKey)
	}
	if strings.TrimSpace(row.BatchNumber) == "" && strings.TrimSpace(row.BatchID) == "" {
		return fmt.Errorf("item %q stock projection requires batchNumber or batchId", itemKey)
	}
	if row.BatchID != "" && !documentUUIDPattern.MatchString(strings.TrimSpace(row.BatchID)) {
		return fmt.Errorf("item %q stock projection batchId must be a UUID", itemKey)
	}
	var itemID, legacyID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, legacy_id FROM master_items
		WHERE tenant_id = $1::uuid AND active
		  AND (legacy_id = $2 OR code = $2)
		ORDER BY legacy_id LIMIT 1
	`, event.TenantID, itemKey).Scan(&itemID, &legacyID); err != nil {
		return fmt.Errorf("item %q is not a canonical item for stock projection", itemKey)
	}
	var godownExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM master_godowns WHERE tenant_id = $1::uuid AND id = $2::uuid AND active)
	`, event.TenantID, row.GodownID).Scan(&godownExists); err != nil {
		return err
	}
	if !godownExists {
		return fmt.Errorf("godown %q is not an active canonical godown", row.GodownID)
	}
	expiry := strings.TrimSpace(row.ExpiryDate)
	if expiry != "" {
		if _, err := time.Parse("2006-01-02", expiry); err != nil {
			return fmt.Errorf("item %q expiryDate must be YYYY-MM-DD", itemKey)
		}
	}
	unitCost := strings.TrimSpace(row.UnitCost)
	if unitCost == "" {
		unitCost = "0.00"
	}
	if _, err := parseMoney(unitCost); err != nil {
		return fmt.Errorf("item %q unitCost: %w", itemKey, err)
	}
	batchQuery := `
		SELECT id::text, locked, unit_cost::text
		FROM stock_batches
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid
		  AND godown_id = $4::uuid`
	args := []any{event.TenantID, event.BranchID, itemID, row.GodownID}
	if row.BatchID != "" {
		batchQuery += ` AND id = $5::uuid`
		args = append(args, row.BatchID)
	} else {
		batchQuery += ` AND batch_number = $5`
		args = append(args, strings.TrimSpace(row.BatchNumber))
		if expiry != "" {
			batchQuery += ` AND expiry_date = $6::date`
			args = append(args, expiry)
		}
	}
	batchQuery += ` FOR UPDATE`
	var batchID, existingCost string
	var locked bool
	err := tx.QueryRowContext(ctx, batchQuery, args...).Scan(&batchID, &locked, &existingCost)
	if errors.Is(err, sql.ErrNoRows) {
		if direction == "out" || direction == "adjustment" {
			return fmt.Errorf("item %q batch was not found for stock projection", itemKey)
		}
		if expiry != "" && expiry < strings.Split(event.OccurredAt, "T")[0] {
			return fmt.Errorf("item %q receiving batch is expired", itemKey)
		}
		if row.BatchID != "" {
			return fmt.Errorf("item %q batchId does not exist", itemKey)
		}
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO stock_batches
				(tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost, received_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, NULLIF($7, '')::date, $8::numeric, $9::timestamptz)
			RETURNING id::text
		`, event.TenantID, event.BranchID, itemID, legacyID, row.GodownID, strings.TrimSpace(row.BatchNumber), expiry, unitCost, event.OccurredAt).Scan(&batchID); err != nil {
			return err
		}
		existingCost = unitCost
	} else if err != nil {
		return err
	} else if locked {
		return fmt.Errorf("item %q batch is locked", itemKey)
	}
	if direction == "in" && strings.TrimSpace(row.UnitCost) != "" && existingCost != unitCost {
		return fmt.Errorf("item %q existing batch cost differs; create a distinct batch identity", itemKey)
	}
	if direction == "out" || direction == "adjustment" {
		result, err := tx.ExecContext(ctx, `
			UPDATE stock_balances SET on_hand = on_hand +
				CASE WHEN $4 = 'out' THEN -$5::numeric ELSE $5::numeric * $6::integer END, updated_at = now()
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = $3::uuid
			  AND (on_hand + CASE WHEN $4 = 'out' THEN -$5::numeric ELSE $5::numeric * $6::integer END) >= 0
		`, event.TenantID, event.BranchID, batchID, direction, quantity, adjustmentSign)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil {
			return err
		} else if affected != 1 {
			return fmt.Errorf("item %q has insufficient stock for the projected %s movement", itemKey, direction)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_balances
				(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, $7::numeric)
			ON CONFLICT (tenant_id, branch_id, batch_id)
			DO UPDATE SET on_hand = stock_balances.on_hand + EXCLUDED.on_hand, updated_at = now()
		`, event.TenantID, event.BranchID, batchID, itemID, legacyID, row.GodownID, quantity); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO stock_ledger
			(tenant_id, branch_id, batch_id, source_event_id, source_line_key, direction,
			 adjustment_sign, quantity, unit_cost, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::smallint, $8::numeric, $9::numeric, $10::timestamptz)
		ON CONFLICT (tenant_id, source_event_id, source_line_key, batch_id, direction) DO NOTHING
	`, event.TenantID, event.BranchID, batchID, event.EventID, fmt.Sprintf("inventory-row-%d", rowCount), direction, adjustmentSign, quantity, existingCost, event.OccurredAt)
	return err
}

func validateInventoryQuantity(value string) error {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() <= 0 {
		return errors.New("quantity must be a positive decimal")
	}
	return nil
}

func jsonScalarText(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	var text string
	if strings.HasPrefix(value, `"`) && json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	return value
}

func (s *Server) beginScopedTx(ctx context.Context, operator *sessionContext) (*sql.Tx, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	setConfig := func(key, value string) error {
		_, err := tx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, key, value)
		return err
	}
	if err := setConfig("app.tenant_id", operator.TenantID); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := setConfig("app.branch_id", operator.BranchID); err != nil {
		tx.Rollback()
		return nil, err
	}
	allowTenant := "false"
	if hasTenantAdminRole(operator.Roles) {
		allowTenant = "true"
	}
	if err := setConfig("app.allow_tenant_scope", allowTenant); err != nil {
		tx.Rollback()
		return nil, err
	}
	statementTimeout := s.dbTimeout
	if statementTimeout <= 0 {
		statementTimeout = 5 * time.Second
	}
	lockTimeout := s.lockTimeout
	if lockTimeout <= 0 {
		lockTimeout = time.Second
	}
	if err := setConfig("statement_timeout", formatPostgresTimeout(statementTimeout)); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := setConfig("lock_timeout", formatPostgresTimeout(lockTimeout)); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func formatPostgresTimeout(value time.Duration) string {
	if value < time.Millisecond {
		value = time.Millisecond
	}
	return fmt.Sprintf("%dms", value.Milliseconds())
}

func validateEventScope(event syncEvent, operator *sessionContext) error {
	if event.TenantID != operator.TenantID {
		return errors.New("event tenant does not match the authenticated tenant")
	}
	if operator.BranchID == "" || event.BranchID != operator.BranchID {
		return errors.New("event branch does not match the authenticated branch")
	}
	if operator.CounterID != "" && event.CounterID != operator.CounterID {
		return errors.New("event counter does not match the authenticated counter")
	}
	if event.OperatorID != "" && event.OperatorID != operator.UserID {
		return errors.New("event operator does not match the authenticated operator")
	}
	return nil
}

// validateSyncEventScope permits a branch-scoped tenant administrator to
// forward events produced by the branch's assigned operators and counters.
// Direct browser transaction endpoints continue to use validateEventScope and
// therefore cannot impersonate another operator.
func validateSyncEventScope(ctx context.Context, tx *sql.Tx, event syncEvent, operator *sessionContext) error {
	if event.TenantID != operator.TenantID {
		return errors.New("event tenant does not match the authenticated tenant")
	}
	if operator.BranchID == "" || event.BranchID != operator.BranchID {
		return errors.New("event branch does not match the authenticated branch")
	}
	if event.CounterID == "" || event.OperatorID == "" {
		return errors.New("synchronization events require counter and operator scope")
	}
	if operator.CounterID != "" && event.CounterID != operator.CounterID {
		if !hasTenantAdminRole(operator.Roles) {
			return errors.New("event counter does not match the authenticated counter")
		}
	}
	if event.OperatorID != operator.UserID && !hasTenantAdminRole(operator.Roles) {
		return errors.New("event operator does not match the authenticated operator")
	}
	var counterExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM counters
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND active
		)
	`, event.CounterID, event.TenantID, event.BranchID).Scan(&counterExists); err != nil {
		return err
	}
	if !counterExists {
		return errors.New("event counter is not active in the authenticated branch")
	}
	var operatorExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN user_branch_assignments ba ON ba.user_id = u.id AND ba.tenant_id = u.tenant_id AND ba.branch_id = $3::uuid
			JOIN user_counter_assignments ca ON ca.user_id = u.id AND ca.tenant_id = u.tenant_id AND ca.counter_id = $4::uuid
			WHERE u.id = $1::uuid AND u.tenant_id = $2::uuid AND u.active
		)
	`, event.OperatorID, event.TenantID, event.BranchID, event.CounterID).Scan(&operatorExists); err != nil {
		return err
	}
	if !operatorExists {
		return errors.New("event operator is not assigned to the authenticated branch and counter")
	}
	return nil
}

func insertSyncEvent(ctx context.Context, tx *sql.Tx, event syncEvent, operator *sessionContext) (inserted bool, conflict bool, err error) {
	if event.EventID == "" || event.Aggregate == "" || event.AggregateID == "" || event.BranchID == "" || event.CounterID == "" || event.IdempotencyKey == "" || event.OccurredAt == "" {
		return false, false, errors.New("event is missing required fields including branch and counter scope")
	}
	if event.OperatorID == "" {
		event.OperatorID = operator.UserID
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO sync_events (event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id, idempotency_key, schema_version, payload, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, NULLIF($4, '')::uuid, $5::uuid, $6, $7::uuid, $8, $9, $10::jsonb, $11::timestamptz)
		ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
	`, event.EventID, event.TenantID, event.BranchID, event.CounterID, event.OperatorID, event.Aggregate, event.AggregateID, event.IdempotencyKey, event.SchemaVersion, []byte(event.Payload), event.OccurredAt)
	if err != nil {
		return false, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, false, err
	}
	if rows == 1 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'sync.accepted', $4, $5::uuid, $6::jsonb, $7::timestamptz)
		`, event.TenantID, event.BranchID, event.OperatorID, event.Aggregate, event.AggregateID, []byte(event.Payload), event.OccurredAt); err != nil {
			return false, false, err
		}
		return true, false, nil
	}
	var existingID string
	var existingPayload []byte
	if err := tx.QueryRowContext(ctx, `SELECT event_id::text, payload FROM sync_events WHERE tenant_id = $1::uuid AND idempotency_key = $2`, event.TenantID, event.IdempotencyKey).Scan(&existingID, &existingPayload); err != nil {
		return false, false, err
	}
	if existingID == event.EventID && jsonEqual(existingPayload, event.Payload) {
		return false, false, nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO conflict_records (tenant_id, branch_id, entity_type, entity_id, local_value, server_value)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5::jsonb, $6::jsonb)
	`, event.TenantID, event.BranchID, event.Aggregate, event.AggregateID, []byte(event.Payload), existingPayload)
	return false, true, err
}

func jsonEqual(left, right []byte) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return string(left) == string(right)
	}
	leftBytes, _ := json.Marshal(leftValue)
	rightBytes, _ := json.Marshal(rightValue)
	return string(leftBytes) == string(rightBytes)
}

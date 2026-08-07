package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type canonicalMasterDefinition struct {
	table         string
	kind          string
	discriminator string
}

func canonicalMasterSpecFor(kind string) (canonicalMasterDefinition, bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "item", "items":
		return canonicalMasterDefinition{table: "master_items", kind: "item"}, true
	case "customer":
		return canonicalMasterDefinition{table: "master_parties", kind: "customer", discriminator: "customer"}, true
	case "supplier":
		return canonicalMasterDefinition{table: "master_parties", kind: "supplier", discriminator: "supplier"}, true
	case "manufacturer":
		return canonicalMasterDefinition{table: "master_manufacturers", kind: "manufacturer"}, true
	case "item-group", "item_group", "item-class":
		return canonicalMasterDefinition{table: "master_item_groups", kind: "item-group"}, true
	case "category":
		return canonicalMasterDefinition{table: "master_categories", kind: "category", discriminator: "category"}, true
	case "item-category":
		return canonicalMasterDefinition{table: "master_categories", kind: "item-category", discriminator: "item_category"}, true
	case "customer-category":
		return canonicalMasterDefinition{table: "master_categories", kind: "customer-category", discriminator: "customer_category"}, true
	case "supplier-category":
		return canonicalMasterDefinition{table: "master_categories", kind: "supplier-category", discriminator: "supplier_category"}, true
	case "manufacturer-category":
		return canonicalMasterDefinition{table: "master_categories", kind: "manufacturer-category", discriminator: "manufacturer_category"}, true
	case "godown":
		return canonicalMasterDefinition{table: "master_godowns", kind: "godown"}, true
	default:
		return canonicalMasterDefinition{}, false
	}
}

// canonicalMasterSpec is kept as a small compatibility helper for the
// existing master handlers and makes the route dispatch unambiguous.
func canonicalMasterSpec(kind string) (canonicalMasterDefinition, bool) {
	return canonicalMasterSpecFor(kind)
}

func (s *Server) requireCanonicalMasterScope(r *http.Request, w http.ResponseWriter, operator *sessionContext) bool {
	if operator.BranchID != "" && !s.requireScope(r, w, operator, "branch", operator.BranchID) {
		return false
	}
	return true
}

func canonicalFilter(spec canonicalMasterDefinition, args *[]any) string {
	if spec.discriminator == "" {
		return ""
	}
	*args = append(*args, spec.discriminator)
	if spec.table == "master_parties" {
		return " AND party_type = $" + strconv.Itoa(len(*args))
	}
	return " AND category_kind = $" + strconv.Itoa(len(*args))
}

func canonicalResponseKind(spec canonicalMasterDefinition) string {
	return spec.kind
}

func canonicalScanMaster(scanner interface{ Scan(...any) error }, spec canonicalMasterDefinition, item *masterRecordResponse) error {
	var payload []byte
	err := scanner.Scan(
		&item.ID, &item.LegacyID, &item.Code, &item.Name, &payload,
		&item.Active, &item.CreatedAt, &item.UpdatedAt,
	)
	item.Payload = json.RawMessage(payload)
	return err
}

func canonicalSelect(spec canonicalMasterDefinition) string {
	return fmt.Sprintf(
		`SELECT id::text, legacy_id, code, name, payload, active,
			created_at::text, updated_at::text FROM %s WHERE tenant_id = $1::uuid`,
		spec.table,
	)
}

func (s *Server) canonicalMasterRecords(w http.ResponseWriter, r *http.Request, kind string) {
	spec, _ := canonicalMasterSpec(kind)
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.read") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The normalized master-data store could not be opened.")
		return
	}
	defer tx.Rollback()

	args := []any{operator.TenantID}
	query := canonicalSelect(spec) + canonicalFilter(spec, &args)
	if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
		args = append(args, likePattern(search))
		placeholder := strconv.Itoa(len(args))
		query += ` AND (code ILIKE $` + placeholder + ` ESCAPE '\' OR name ILIKE $` + placeholder + ` ESCAPE '\' OR legacy_id ILIKE $` + placeholder + ` ESCAPE '\')`
	}
	query += ` ORDER BY name, code LIMIT 500`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The normalized master-data query failed.")
		return
	}
	records := make([]masterRecordResponse, 0)
	for rows.Next() {
		var item masterRecordResponse
		if err := canonicalScanMaster(rows, spec, &item); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The normalized master-data response could not be decoded.")
			return
		}
		if spec.kind == "godown" && !canonicalGodownScopeAllowed(operator, item.ID) {
			continue
		}
		item.Kind = canonicalResponseKind(spec)
		records = append(records, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The normalized master-data response could not be read.")
		return
	}
	rows.Close()
	// Phase E imports preserve the original item rows in the generic
	// master_records table before normalized master_items are approved. Expose
	// that scoped catalog as a read-only lookup fallback instead of silently
	// replacing a migrated tenant's real item list with the demo hardcoded list.
	if len(records) == 0 && spec.kind == "item" {
		legacyArgs := []any{operator.TenantID}
		legacyQuery := `SELECT id::text, COALESCE(legacy_id, ''), code, name, payload, active,
			created_at::text, updated_at::text FROM master_records
			WHERE tenant_id = $1::uuid AND kind IN ('item', 'items')`
		if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
			legacyArgs = append(legacyArgs, likePattern(search))
			legacyQuery += ` AND (code ILIKE $2 ESCAPE '\' OR name ILIKE $2 ESCAPE '\' OR legacy_id ILIKE $2 ESCAPE '\')`
		}
		legacyQuery += ` ORDER BY name, code LIMIT 500`
		legacyRows, legacyErr := tx.QueryContext(r.Context(), legacyQuery, legacyArgs...)
		if legacyErr != nil {
			writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read item data", "The imported item catalog could not be queried.")
			return
		}
		for legacyRows.Next() {
			var item masterRecordResponse
			if err := canonicalScanMaster(legacyRows, spec, &item); err != nil {
				legacyRows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read item data", "The imported item catalog could not be decoded.")
				return
			}
			item.Kind = canonicalResponseKind(spec)
			records = append(records, item)
		}
		if err := legacyRows.Err(); err != nil {
			legacyRows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read item data", "The imported item catalog could not be read.")
			return
		}
		legacyRows.Close()
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The normalized master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) createCanonicalMasterRecord(w http.ResponseWriter, r *http.Request, kind string) {
	spec, _ := canonicalMasterSpec(kind)
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.write") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	var request masterRecordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_master_record", "Invalid master-data record", "The record could not be parsed.")
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	request.LegacyID = strings.TrimSpace(request.LegacyID)
	if request.LegacyID == "" {
		request.LegacyID = request.Code
	}
	if request.Code == "" || request.Name == "" || request.LegacyID == "" || len(request.Code) > 160 || len(request.Name) > 240 || len(request.LegacyID) > 160 {
		writeProblem(w, http.StatusBadRequest, "invalid_master_record", "Invalid master-data record", "Code, name, and legacyId are required.")
		return
	}
	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(request.Payload) {
		writeProblem(w, http.StatusBadRequest, "invalid_master_payload", "Invalid master-data payload", "Payload must be a JSON object.")
		return
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The normalized master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	args := []any{operator.TenantID, request.LegacyID, request.Code, request.Name, request.Payload, active}
	columns := "tenant_id, legacy_id, code, name, payload, active"
	values := "$1::uuid, $2, $3, $4, $5::jsonb, $6"
	if spec.table == "master_parties" {
		columns = "tenant_id, party_type, legacy_id, code, name, payload, active"
		values = "$1::uuid, $7, $2, $3, $4, $5::jsonb, $6"
		args = append(args, spec.discriminator)
	}
	if spec.table == "master_categories" {
		columns = "tenant_id, category_kind, legacy_id, code, name, payload, active"
		values = "$1::uuid, $7, $2, $3, $4, $5::jsonb, $6"
		args = append(args, spec.discriminator)
	}
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)
		RETURNING id::text, legacy_id, code, name, payload, active, created_at::text, updated_at::text`,
		spec.table, columns, values)
	var item masterRecordResponse
	if err := tx.QueryRowContext(r.Context(), query, args...).Scan(&item.ID, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			writeProblem(w, http.StatusConflict, "master_legacy_id_exists", "Master-data identifier already exists", "The legacy identifier or code already exists in this tenant.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "master_create_failed", "Master-data record was not created", "The normalized master-data store rejected the record.")
		return
	}
	item.Kind = canonicalResponseKind(spec)
	if spec.kind == "item" {
		if err := syncItemPayloadAliases(r.Context(), tx, operator.TenantID, item.ID, item.Payload); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "master_alias_write_failed", "Item aliases were not saved", "The item lookup aliases could not be updated.")
			return
		}
		item.Suppliers = []itemSupplierResponse{}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_create_failed", "Master-data record was not created", "The normalized master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) canonicalMasterDetail(w http.ResponseWriter, r *http.Request, kind string) {
	spec, _ := canonicalMasterSpec(kind)
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.read") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	if spec.kind == "godown" && !s.requireScope(r, w, operator, "godown", r.PathValue("id")) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The normalized master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	args := []any{operator.TenantID, r.PathValue("id")}
	query := canonicalSelect(spec) + ` AND id = $2::uuid` + canonicalFilter(spec, &args)
	var item masterRecordResponse
	if err := tx.QueryRowContext(r.Context(), query, args...).Scan(&item.ID, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The record is outside the authenticated tenant scope.")
		return
	}
	item.Kind = canonicalResponseKind(spec)
	if spec.kind == "item" {
		suppliers, err := queryItemSuppliers(r.Context(), tx, operator.TenantID, item.ID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_supplier_read_failed", "Unable to read item suppliers", "The item supplier grid could not be loaded.")
			return
		}
		item.Suppliers = suppliers
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The normalized master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) updateCanonicalMasterRecord(w http.ResponseWriter, r *http.Request, kind string) {
	spec, _ := canonicalMasterSpec(kind)
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.write") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	if spec.kind == "godown" && !s.requireScope(r, w, operator, "godown", r.PathValue("id")) {
		return
	}
	var request masterRecordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_master_record", "Invalid master-data record", "The update body could not be parsed.")
		return
	}
	if len(request.Payload) > 0 && !json.Valid(request.Payload) {
		writeProblem(w, http.StatusBadRequest, "invalid_master_payload", "Invalid master-data payload", "Payload must be valid JSON.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The normalized master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	set := []string{"updated_at = now()"}
	args := []any{}
	if value := strings.TrimSpace(request.LegacyID); value != "" {
		args = append(args, value)
		set = append(set, fmt.Sprintf("legacy_id = $%d", len(args)))
	}
	if value := strings.TrimSpace(request.Code); value != "" {
		args = append(args, value)
		set = append(set, fmt.Sprintf("code = $%d", len(args)))
	}
	if value := strings.TrimSpace(request.Name); value != "" {
		args = append(args, value)
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
	if len(set) == 1 {
		writeProblem(w, http.StatusBadRequest, "empty_master_update", "Empty master-data update", "Provide at least one master-data field to update.")
		return
	}
	tenantPos := len(args) + 1
	idPos := len(args) + 2
	args = append(args, operator.TenantID, r.PathValue("id"))
	query := fmt.Sprintf(`UPDATE %s SET %s WHERE tenant_id = $%d::uuid AND id = $%d::uuid`,
		spec.table, strings.Join(set, ", "), tenantPos, idPos)
	query += canonicalFilter(spec, &args)
	query += ` RETURNING id::text, legacy_id, code, name, payload, active, created_at::text, updated_at::text`
	var item masterRecordResponse
	if err := tx.QueryRowContext(r.Context(), query, args...).Scan(&item.ID, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			writeProblem(w, http.StatusConflict, "master_legacy_id_exists", "Master-data identifier already exists", "The legacy identifier or code already exists in this tenant.")
			return
		}
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The record is outside the authenticated tenant scope.")
		return
	}
	item.Kind = canonicalResponseKind(spec)
	if spec.kind == "item" {
		if err := syncItemPayloadAliases(r.Context(), tx, operator.TenantID, item.ID, item.Payload); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "master_alias_write_failed", "Item aliases were not saved", "The item lookup aliases could not be updated.")
			return
		}
		item.Suppliers, err = queryItemSuppliers(r.Context(), tx, operator.TenantID, item.ID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_supplier_read_failed", "Unable to read item suppliers", "The item supplier grid could not be loaded.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_update_failed", "Master-data record was not updated", "The normalized master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) masterRecordDetail(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if _, ok := canonicalMasterSpec(kind); ok {
		s.canonicalMasterDetail(w, r, kind)
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
	var item masterRecordResponse
	if err := tx.QueryRowContext(r.Context(), `
		SELECT id::text, kind, COALESCE(legacy_id, ''), code, name, payload, active, created_at::text, updated_at::text
		FROM master_records WHERE tenant_id = $1::uuid AND kind = $2 AND id = $3::uuid
	`, operator.TenantID, kind, r.PathValue("id")).Scan(&item.ID, &item.Kind, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The record is outside the authenticated tenant scope.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_read_failed", "Unable to read master data", "The master-data transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteMasterRecord(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if spec, ok := canonicalMasterSpec(kind); ok {
		s.deleteCanonicalMasterRecord(w, r, spec)
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
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `DELETE FROM master_records WHERE tenant_id = $1::uuid AND kind = $2 AND id = $3::uuid`, operator.TenantID, kind, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "master_delete_failed", "Master-data record was not deleted", "The master-data store rejected the delete.")
		return
	}
	if count, _ := result.RowsAffected(); count == 0 {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The record is outside the authenticated tenant scope.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_delete_failed", "Master-data record was not deleted", "The master-data transaction could not be committed.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteCanonicalMasterRecord(w http.ResponseWriter, r *http.Request, spec canonicalMasterDefinition) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.write") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	if spec.kind == "godown" && !s.requireScope(r, w, operator, "godown", r.PathValue("id")) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The normalized master-data store could not be opened.")
		return
	}
	defer tx.Rollback()
	args := []any{operator.TenantID, r.PathValue("id")}
	query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1::uuid AND id = $2::uuid", spec.table) + canonicalFilter(spec, &args)
	result, err := tx.ExecContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "master_delete_failed", "Master-data record was not deleted", "The normalized master-data store rejected the delete.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The record is outside the authenticated tenant scope.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "master_delete_failed", "Master-data record was not deleted", "The normalized master-data transaction could not be committed.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type itemLookupCandidate struct {
	Name     string
	Code     string
	LegacyID string
	Aliases  []string
}

func normalizeMasterLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func itemLookupMatches(candidate itemLookupCandidate, query string) bool {
	query = normalizeMasterLookup(query)
	if query == "" {
		return false
	}
	for _, value := range append([]string{candidate.Name, candidate.Code, candidate.LegacyID}, candidate.Aliases...) {
		if strings.Contains(normalizeMasterLookup(value), query) {
			return true
		}
	}
	return false
}

type itemLookupResponse struct {
	ID       string          `json:"id"`
	LegacyID string          `json:"legacyId"`
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Payload  json.RawMessage `json:"payload"`
	Active   bool            `json:"active"`
	Aliases  []string        `json:"aliases"`
}

func likePattern(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return "%" + value + "%"
}

func syncItemPayloadAliases(ctx context.Context, tx *sql.Tx, tenantID, itemID string, payload json.RawMessage) error {
	var fields map[string]any
	if len(payload) == 0 || string(payload) == "null" {
		fields = map[string]any{}
	} else if err := json.Unmarshal(payload, &fields); err != nil {
		return err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM master_aliases
		WHERE tenant_id = $1::uuid AND item_id = $2::uuid
		  AND alias_kind IN ('alias', 'barcode')`, tenantID, itemID); err != nil {
		return err
	}
	for key, kind := range map[string]string{
		"CustomICode": "alias",
		"AliasName":   "alias",
		"Barcode":     "barcode",
		"BarCode":     "barcode",
	} {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			value = fmt.Sprint(raw)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
			VALUES ($1::uuid, $2::uuid, $3, $4, lower($4))
			ON CONFLICT (tenant_id, alias_kind, normalized_value) DO NOTHING`,
			tenantID, itemID, kind, value); err != nil {
			return err
		}
	}
	if raw, ok := fields["AlternateItemAliases"].([]any); ok {
		for _, candidate := range raw {
			value, ok := candidate.(string)
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
				VALUES ($1::uuid, $2::uuid, 'alternate_alias', $3, lower($3))
				ON CONFLICT (tenant_id, alias_kind, normalized_value) DO NOTHING`,
				tenantID, itemID, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) itemLookup(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.read") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	queryText := strings.TrimSpace(r.URL.Query().Get("q"))
	if queryText == "" {
		queryText = strings.TrimSpace(r.URL.Query().Get("search"))
	}
	if queryText == "" {
		queryText = strings.TrimSpace(r.URL.Query().Get("name"))
	}
	if queryText == "" {
		queryText = strings.TrimSpace(r.URL.Query().Get("alias"))
	}
	barcode := strings.TrimSpace(r.URL.Query().Get("barcode"))
	legacyID := strings.TrimSpace(r.URL.Query().Get("legacyId"))
	if legacyID == "" {
		legacyID = strings.TrimSpace(r.URL.Query().Get("legacyID"))
	}
	if queryText == "" && barcode == "" && legacyID == "" {
		writeProblem(w, http.StatusBadRequest, "lookup_query_required", "Lookup query required", "Provide q, barcode, or legacyId.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item lookup store could not be opened.")
		return
	}
	defer tx.Rollback()
	args := []any{operator.TenantID, likePattern(queryText), strings.ToLower(barcode), legacyID}
	rows, err := tx.QueryContext(r.Context(), `
		SELECT i.id::text, i.legacy_id, i.code, i.name, i.payload, i.active
		FROM master_items i
		WHERE i.tenant_id = $1::uuid AND i.active
		  AND (
		    ($2 <> '%%' AND (
		      i.name ILIKE $2 ESCAPE '\' OR i.code ILIKE $2 ESCAPE '\' OR i.legacy_id ILIKE $2 ESCAPE '\'
		      OR EXISTS (SELECT 1 FROM master_aliases a WHERE a.tenant_id = i.tenant_id AND a.item_id = i.id AND a.active AND a.normalized_value LIKE lower($2) ESCAPE '\')
		    ))
		    OR ($3 <> '' AND EXISTS (
		      SELECT 1 FROM master_aliases a WHERE a.tenant_id = i.tenant_id AND a.item_id = i.id
		        AND a.active AND a.alias_kind IN ('barcode', 'alias') AND a.normalized_value = $3
		    ))
		    OR ($4 <> '' AND i.legacy_id = $4)
		  )
		ORDER BY i.name, i.code
		LIMIT 100
	`, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_lookup_failed", "Unable to look up items", "The item lookup query failed.")
		return
	}
	items := make([]itemLookupResponse, 0)
	for rows.Next() {
		var item itemLookupResponse
		if err := rows.Scan(&item.ID, &item.LegacyID, &item.Code, &item.Name, &item.Payload, &item.Active); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_lookup_failed", "Unable to look up items", "The item lookup response could not be decoded.")
			return
		}
		item.Aliases, err = queryItemAliases(r.Context(), tx, operator.TenantID, item.ID)
		if err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_lookup_failed", "Unable to look up items", "The item aliases could not be loaded.")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "item_lookup_failed", "Unable to look up items", "The item lookup response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_lookup_failed", "Unable to look up items", "The item lookup transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type itemSupplierResponse struct {
	ID               string `json:"id"`
	LegacySupplierID string `json:"legacySupplierId"`
	SupplierID       string `json:"supplierId,omitempty"`
	Priority         *int   `json:"priority,omitempty"`
	Rate             string `json:"rate,omitempty"`
	DiscountPercent  string `json:"discountPercent,omitempty"`
	Quantity         string `json:"quantity,omitempty"`
	Bonus            string `json:"bonus,omitempty"`
	Days             *int   `json:"days,omitempty"`
}

type itemSupplierRequest struct {
	LegacySupplierID string          `json:"legacySupplierId"`
	SupplierID       string          `json:"supplierId"`
	Priority         *int            `json:"priority"`
	Rate             string          `json:"rate"`
	DiscountPercent  string          `json:"discountPercent"`
	Quantity         string          `json:"quantity"`
	Bonus            string          `json:"bonus"`
	Days             *int            `json:"days"`
	Payload          json.RawMessage `json:"payload"`
}

func queryItemAliases(ctx context.Context, tx *sql.Tx, tenantID, itemID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT alias_value FROM master_aliases
		WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND active
		ORDER BY alias_kind, alias_value`, tenantID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	aliases := make([]string, 0)
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func queryItemSuppliers(ctx context.Context, tx *sql.Tx, tenantID, itemID string) ([]itemSupplierResponse, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT s.id::text, s.legacy_supplier_id, COALESCE(s.supplier_id::text, ''),
		       s.priority, COALESCE(s.rate::text, ''), COALESCE(s.discount_percent::text, ''),
		       COALESCE(s.quantity::text, ''), COALESCE(s.bonus::text, ''), s.days
		FROM item_suppliers s
		WHERE s.tenant_id = $1::uuid AND s.item_id = $2::uuid
		ORDER BY s.priority NULLS LAST, s.legacy_supplier_id`, tenantID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]itemSupplierResponse, 0)
	for rows.Next() {
		var item itemSupplierResponse
		var priority, days sql.NullInt64
		if err := rows.Scan(&item.ID, &item.LegacySupplierID, &item.SupplierID, &priority, &item.Rate, &item.DiscountPercent, &item.Quantity, &item.Bonus, &days); err != nil {
			return nil, err
		}
		if priority.Valid {
			value := int(priority.Int64)
			item.Priority = &value
		}
		if days.Valid {
			value := int(days.Int64)
			item.Days = &value
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type alternateItemAliasesRequest struct {
	Aliases []string `json:"aliases"`
}

const maxAlternateItemAliases = 100

func normalizeAlternateItemAliases(values []string) ([]string, error) {
	if len(values) > maxAlternateItemAliases {
		return nil, fmt.Errorf("an item may have at most %d alternate aliases", maxAlternateItemAliases)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("alternate item aliases cannot be blank")
		}
		if len(value) > 160 {
			return nil, fmt.Errorf("alternate item aliases cannot exceed 160 characters")
		}
		normalized := normalizeMasterLookup(value)
		if _, exists := seen[normalized]; exists {
			return nil, fmt.Errorf("alternate item aliases must be unique")
		}
		seen[normalized] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (s *Server) itemAliases(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Alternate aliases are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item alias store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var exists bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (SELECT 1 FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid)
	`, operator.TenantID, itemID).Scan(&exists); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_alias_read_failed", "Unable to read item aliases", "The item alias scope could not be checked.")
		return
	}
	if !exists {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT alias_value FROM master_aliases
			WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND alias_kind = 'alternate_alias' AND active
			ORDER BY alias_value
		`, operator.TenantID, itemID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_alias_read_failed", "Unable to read item aliases", "The item aliases could not be queried.")
			return
		}
		aliases := make([]string, 0)
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "item_alias_read_failed", "Unable to read item aliases", "The item aliases could not be decoded.")
				return
			}
			aliases = append(aliases, value)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_alias_read_failed", "Unable to read item aliases", "The item aliases could not be read.")
			return
		}
		rows.Close()
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_alias_read_failed", "Unable to read item aliases", "The item alias transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases})
		return
	}
	var request alternateItemAliasesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_aliases", "Invalid item aliases", "The aliases payload could not be parsed.")
		return
	}
	aliases, err := normalizeAlternateItemAliases(request.Aliases)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_aliases", "Invalid item aliases", err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM master_aliases
		WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND alias_kind = 'alternate_alias'
	`, operator.TenantID, itemID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_alias_write_failed", "Item aliases were not saved", "The existing alternate aliases could not be replaced.")
		return
	}
	for _, value := range aliases {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
			VALUES ($1::uuid, $2::uuid, 'alternate_alias', $3, lower($3))
		`, operator.TenantID, itemID, value); err != nil {
			if isUniqueViolation(err) {
				writeProblem(w, http.StatusConflict, "item_alias_exists", "Item alias already exists", "An alternate alias is already assigned to another item.")
				return
			}
			writeProblem(w, http.StatusUnprocessableEntity, "item_alias_write_failed", "Item aliases were not saved", "The alternate aliases were rejected by the database.")
			return
		}
	}
	encodedAliases, err := json.Marshal(aliases)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "item_alias_write_failed", "Item aliases were not saved", "The alternate aliases could not be encoded.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE master_items
		SET payload = jsonb_set(COALESCE(payload, '{}'::jsonb), '{AlternateItemAliases}', $3::jsonb, true), updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID, string(encodedAliases)); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_alias_write_failed", "Item aliases were not saved", "The item alias metadata could not be updated.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_alias_write_failed", "Item aliases were not saved", "The item alias transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases})
}

type itemImageRequest struct {
	RowID            int    `json:"rowId"`
	ImageDescription string `json:"imageDescription"`
	ImageData        string `json:"imageData"`
	ImageType        string `json:"imageType"`
}

type itemImageResponse struct {
	ID               string `json:"id,omitempty"`
	RowID            int    `json:"rowId"`
	ImageDescription string `json:"imageDescription"`
	ImageData        string `json:"imageData"`
	ImageType        string `json:"imageType"`
}

type normalizedItemImage struct {
	RowID            int
	ImageDescription string
	ImageData        []byte
	ImageType        string
}

const (
	maxItemImages          = 50
	maxItemImageBytes      = 8 << 20
	maxItemImageTotalBytes = 32 << 20
)

func decodeItemImageData(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 {
			return nil, fmt.Errorf("image data URL is missing its payload")
		}
		value = value[comma+1:]
	}
	if value == "" {
		return nil, fmt.Errorf("image data is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("image data must be valid base64")
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("image data is required")
	}
	return decoded, nil
}

func normalizeItemImages(values []itemImageRequest) ([]normalizedItemImage, error) {
	if len(values) > maxItemImages {
		return nil, fmt.Errorf("an item may have at most %d images", maxItemImages)
	}
	result := make([]normalizedItemImage, 0, len(values))
	seenRows := make(map[int]struct{}, len(values))
	totalBytes := 0
	for index, raw := range values {
		rowID := raw.RowID
		if rowID <= 0 {
			rowID = index + 1
		}
		if _, exists := seenRows[rowID]; exists {
			return nil, fmt.Errorf("image row identifiers must be unique")
		}
		seenRows[rowID] = struct{}{}
		description := strings.TrimSpace(raw.ImageDescription)
		if len(description) > 240 {
			return nil, fmt.Errorf("image descriptions cannot exceed 240 characters")
		}
		imageType := strings.TrimSpace(raw.ImageType)
		if len(imageType) > 100 {
			return nil, fmt.Errorf("image types cannot exceed 100 characters")
		}
		data, err := decodeItemImageData(raw.ImageData)
		if err != nil {
			return nil, err
		}
		if len(data) > maxItemImageBytes {
			return nil, fmt.Errorf("each image cannot exceed 8 MiB")
		}
		totalBytes += len(data)
		if totalBytes > maxItemImageTotalBytes {
			return nil, fmt.Errorf("an item's image collection cannot exceed 32 MiB")
		}
		result = append(result, normalizedItemImage{RowID: rowID, ImageDescription: description, ImageData: data, ImageType: imageType})
	}
	return result, nil
}

func (s *Server) itemImages(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Images are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item image store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var legacyItemID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT legacy_id FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID).Scan(&legacyItemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT id::text, row_id, image_description, image_type, image_data
			FROM master_item_images
			WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND active
			ORDER BY row_id
		`, operator.TenantID, itemID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_image_read_failed", "Unable to read item images", "The item image collection could not be queried.")
			return
		}
		images := make([]itemImageResponse, 0)
		for rows.Next() {
			var image itemImageResponse
			var data []byte
			if err := rows.Scan(&image.ID, &image.RowID, &image.ImageDescription, &image.ImageType, &data); err != nil {
				rows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "item_image_read_failed", "Unable to read item images", "The item image collection could not be decoded.")
				return
			}
			image.ImageData = base64.StdEncoding.EncodeToString(data)
			images = append(images, image)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_image_read_failed", "Unable to read item images", "The item image collection could not be read.")
			return
		}
		rows.Close()
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_image_read_failed", "Unable to read item images", "The item image transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"images": images})
		return
	}
	var request struct {
		Images []itemImageRequest `json:"images"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 48<<20)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_images", "Invalid item images", "The image collection could not be parsed.")
		return
	}
	images, err := normalizeItemImages(request.Images)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_images", "Invalid item images", err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM master_item_images WHERE tenant_id = $1::uuid AND item_id = $2::uuid
	`, operator.TenantID, itemID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_image_write_failed", "Item images were not saved", "The existing item image collection could not be replaced.")
		return
	}
	result := make([]itemImageResponse, 0, len(images))
	for _, image := range images {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO master_item_images (
				tenant_id, item_id, legacy_item_id, row_id, image_description, image_data, image_type)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		`, operator.TenantID, itemID, legacyItemID, image.RowID, image.ImageDescription, image.ImageData, image.ImageType); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "item_image_write_failed", "Item images were not saved", "The item image collection was rejected by the database.")
			return
		}
		result = append(result, itemImageResponse{
			RowID: image.RowID, ImageDescription: image.ImageDescription,
			ImageData: base64.StdEncoding.EncodeToString(image.ImageData), ImageType: image.ImageType,
		})
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE master_items
		SET payload = jsonb_set(COALESCE(payload, '{}'::jsonb), '{ItemImageCount}', to_jsonb($3::integer), true), updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID, len(images)); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_image_write_failed", "Item images were not saved", "The item image metadata could not be updated.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_image_write_failed", "Item images were not saved", "The item image transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": result})
}

type itemNotesRequest struct {
	NotesData string `json:"notesData"`
}

type itemNotesResponse struct {
	NotesData string `json:"notesData"`
}

const maxItemNotesBytes = 8 << 20

func decodeItemNotesData(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 {
			return nil, fmt.Errorf("notes data URL is missing its payload")
		}
		value = value[comma+1:]
	}
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil {
		return nil, fmt.Errorf("notes data must be valid base64")
	}
	if len(decoded) > maxItemNotesBytes {
		return nil, fmt.Errorf("item notes cannot exceed 8 MiB")
	}
	return decoded, nil
}

func (s *Server) itemNotes(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Notes are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item notes store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var legacyItemID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT legacy_id FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID).Scan(&legacyItemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		var data []byte
		err := tx.QueryRowContext(r.Context(), `
			SELECT notes_data
			FROM master_item_notes
			WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND active
		`, operator.TenantID, itemID).Scan(&data)
		if err != nil && err != sql.ErrNoRows {
			writeProblem(w, http.StatusServiceUnavailable, "item_notes_read_failed", "Unable to read item notes", "The item notes blob could not be read.")
			return
		}
		response := itemNotesResponse{}
		if err == nil && len(data) > 0 {
			response.NotesData = base64.StdEncoding.EncodeToString(data)
		}
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_notes_read_failed", "Unable to read item notes", "The item notes transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	var request itemNotesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_notes", "Invalid item notes", "The item notes payload could not be parsed.")
		return
	}
	data, err := decodeItemNotesData(request.NotesData)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_notes", "Invalid item notes", err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO master_item_notes (tenant_id, item_id, legacy_item_id, notes_data, active)
		VALUES ($1::uuid, $2::uuid, $3, $4, true)
		ON CONFLICT (tenant_id, item_id) DO UPDATE SET
			legacy_item_id = EXCLUDED.legacy_item_id,
			notes_data = EXCLUDED.notes_data,
			active = true,
			updated_at = now()
	`, operator.TenantID, itemID, legacyItemID, data); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_notes_write_failed", "Item notes were not saved", "The item notes blob was rejected by the database.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_notes_write_failed", "Item notes were not saved", "The item notes transaction could not be committed.")
		return
	}
	response := itemNotesResponse{}
	if len(data) > 0 {
		response.NotesData = base64.StdEncoding.EncodeToString(data)
	}
	writeJSON(w, http.StatusOK, response)
}

type itemAssociationRequest struct {
	LegacyItemIDs []string `json:"legacyItemIds"`
}

type itemAssociationResponse struct {
	ID           string `json:"id,omitempty"`
	LegacyItemID string `json:"legacyItemId"`
	Code         string `json:"code,omitempty"`
	Name         string `json:"name,omitempty"`
}

const maxItemAssociations = 100

func normalizeItemAssociationIDs(values []string) ([]string, error) {
	if len(values) > maxItemAssociations {
		return nil, fmt.Errorf("an item may have at most %d associations", maxItemAssociations)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("associated item identifiers cannot be blank")
		}
		if len(value) > 160 {
			return nil, fmt.Errorf("associated item identifiers cannot exceed 160 characters")
		}
		normalized := strings.ToLower(value)
		if _, exists := seen[normalized]; exists {
			return nil, fmt.Errorf("associated item identifiers must be unique")
		}
		seen[normalized] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (s *Server) itemAssociations(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Associations are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item association store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var legacyItemID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT legacy_id FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID).Scan(&legacyItemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT a.id::text, a.associated_legacy_item_id, COALESCE(i.code, ''), COALESCE(i.name, '')
			FROM master_item_associations a
			LEFT JOIN master_items i ON i.tenant_id = a.tenant_id AND i.id = a.associated_item_id
			WHERE a.tenant_id = $1::uuid AND a.item_id = $2::uuid AND a.active
			ORDER BY a.associated_legacy_item_id
		`, operator.TenantID, itemID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_association_read_failed", "Unable to read item associations", "The item association list could not be queried.")
			return
		}
		associations := make([]itemAssociationResponse, 0)
		for rows.Next() {
			var association itemAssociationResponse
			if err := rows.Scan(&association.ID, &association.LegacyItemID, &association.Code, &association.Name); err != nil {
				rows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "item_association_read_failed", "Unable to read item associations", "The item association list could not be decoded.")
				return
			}
			associations = append(associations, association)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_association_read_failed", "Unable to read item associations", "The item association list could not be read.")
			return
		}
		rows.Close()
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_association_read_failed", "Unable to read item associations", "The item association transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"associations": associations})
		return
	}
	var request itemAssociationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_associations", "Invalid item associations", "The item association payload could not be parsed.")
		return
	}
	legacyIDs, err := normalizeItemAssociationIDs(request.LegacyItemIDs)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_associations", "Invalid item associations", err.Error())
		return
	}
	for _, associatedLegacyID := range legacyIDs {
		if strings.EqualFold(associatedLegacyID, legacyItemID) {
			writeProblem(w, http.StatusBadRequest, "invalid_item_associations", "Invalid item associations", "An item cannot be associated with itself.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM master_item_associations WHERE tenant_id = $1::uuid AND item_id = $2::uuid
	`, operator.TenantID, itemID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_association_write_failed", "Item associations were not saved", "The existing item association list could not be replaced.")
		return
	}
	associations := make([]itemAssociationResponse, 0, len(legacyIDs))
	for _, associatedLegacyID := range legacyIDs {
		var association itemAssociationResponse
		if err := tx.QueryRowContext(r.Context(), `
			SELECT id::text, code, name
			FROM master_items
			WHERE tenant_id = $1::uuid AND legacy_id = $2
		`, operator.TenantID, associatedLegacyID).Scan(&association.ID, &association.Code, &association.Name); err != nil {
			if err == sql.ErrNoRows {
				writeProblem(w, http.StatusBadRequest, "item_association_not_found", "Associated item was not found", "Every associated item must exist in the authenticated tenant scope.")
				return
			}
			writeProblem(w, http.StatusServiceUnavailable, "item_association_read_failed", "Unable to resolve item association", "The associated item lookup failed.")
			return
		}
		association.LegacyItemID = associatedLegacyID
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO master_item_associations (
				tenant_id, item_id, associated_item_id, legacy_item_id, associated_legacy_item_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
		`, operator.TenantID, itemID, association.ID, legacyItemID, associatedLegacyID); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "item_association_write_failed", "Item associations were not saved", "The item association was rejected by the database.")
			return
		}
		associations = append(associations, association)
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_association_write_failed", "Item associations were not saved", "The item association transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"associations": associations})
}

type itemAuthorRequest struct {
	AuthorCode int `json:"authorCode"`
	Priority   int `json:"priority"`
	RowID      int `json:"rowId"`
}

type itemAuthorResponse struct {
	ID         string `json:"id,omitempty"`
	AuthorCode int    `json:"authorCode"`
	Priority   int    `json:"priority"`
	RowID      int    `json:"rowId"`
}

const maxItemAuthors = 50

func normalizeItemAuthors(values []itemAuthorRequest) ([]itemAuthorRequest, error) {
	if len(values) > maxItemAuthors {
		return nil, fmt.Errorf("an item may have at most %d authors", maxItemAuthors)
	}
	result := make([]itemAuthorRequest, 0, len(values))
	seenAuthors := make(map[int]struct{}, len(values))
	seenRows := make(map[int]struct{}, len(values))
	for index, raw := range values {
		if raw.AuthorCode <= 0 {
			return nil, fmt.Errorf("author codes must be positive")
		}
		if raw.Priority < 0 || raw.Priority > 255 {
			return nil, fmt.Errorf("author priorities must be between 0 and 255")
		}
		rowID := raw.RowID
		if rowID <= 0 {
			rowID = index + 1
		}
		if _, exists := seenAuthors[raw.AuthorCode]; exists {
			return nil, fmt.Errorf("author codes must be unique")
		}
		if _, exists := seenRows[rowID]; exists {
			return nil, fmt.Errorf("author row identifiers must be unique")
		}
		seenAuthors[raw.AuthorCode] = struct{}{}
		seenRows[rowID] = struct{}{}
		result = append(result, itemAuthorRequest{AuthorCode: raw.AuthorCode, Priority: raw.Priority, RowID: rowID})
	}
	return result, nil
}

func (s *Server) itemAuthors(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Authors are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item author store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var legacyItemID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT legacy_id FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID).Scan(&legacyItemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT id::text, author_code, priority, row_id
			FROM master_item_authors
			WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND active
			ORDER BY priority, row_id
		`, operator.TenantID, itemID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_author_read_failed", "Unable to read item authors", "The item author list could not be queried.")
			return
		}
		authors := make([]itemAuthorResponse, 0)
		for rows.Next() {
			var author itemAuthorResponse
			if err := rows.Scan(&author.ID, &author.AuthorCode, &author.Priority, &author.RowID); err != nil {
				rows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "item_author_read_failed", "Unable to read item authors", "The item author list could not be decoded.")
				return
			}
			authors = append(authors, author)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_author_read_failed", "Unable to read item authors", "The item author list could not be read.")
			return
		}
		rows.Close()
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_author_read_failed", "Unable to read item authors", "The item author transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"authors": authors})
		return
	}
	var request struct {
		Authors []itemAuthorRequest `json:"authors"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_authors", "Invalid item authors", "The item author payload could not be parsed.")
		return
	}
	authors, err := normalizeItemAuthors(request.Authors)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_authors", "Invalid item authors", err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM master_item_authors WHERE tenant_id = $1::uuid AND item_id = $2::uuid
	`, operator.TenantID, itemID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_author_write_failed", "Item authors were not saved", "The existing item author list could not be replaced.")
		return
	}
	result := make([]itemAuthorResponse, 0, len(authors))
	for _, author := range authors {
		var saved itemAuthorResponse
		if err := tx.QueryRowContext(r.Context(), `
			INSERT INTO master_item_authors (tenant_id, item_id, legacy_item_id, author_code, priority, row_id)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
			RETURNING id::text, author_code, priority, row_id
		`, operator.TenantID, itemID, legacyItemID, author.AuthorCode, author.Priority, author.RowID).Scan(&saved.ID, &saved.AuthorCode, &saved.Priority, &saved.RowID); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "item_author_write_failed", "Item authors were not saved", "The item author relationship was rejected by the database.")
			return
		}
		result = append(result, saved)
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_author_write_failed", "Item authors were not saved", "The item author transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authors": result})
}

type itemModelResponse struct {
	ID        string `json:"id,omitempty"`
	ModelCode int    `json:"modelCode"`
}

const maxItemModels = 100

func normalizeItemModelCodes(values []int) ([]int, error) {
	if len(values) > maxItemModels {
		return nil, fmt.Errorf("an item may have at most %d models", maxItemModels)
	}
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < -32768 || value > 32767 {
			return nil, fmt.Errorf("model codes must fit the captured smallint range")
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("model codes must be unique")
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (s *Server) itemModels(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Models are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item model store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var legacyItemID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT legacy_id FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID).Scan(&legacyItemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT id::text, model_code
			FROM master_item_models
			WHERE tenant_id = $1::uuid AND item_id = $2::uuid AND active
			ORDER BY model_code
		`, operator.TenantID, itemID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_model_read_failed", "Unable to read item models", "The item model list could not be queried.")
			return
		}
		models := make([]itemModelResponse, 0)
		for rows.Next() {
			var model itemModelResponse
			if err := rows.Scan(&model.ID, &model.ModelCode); err != nil {
				rows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "item_model_read_failed", "Unable to read item models", "The item model list could not be decoded.")
				return
			}
			models = append(models, model)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_model_read_failed", "Unable to read item models", "The item model list could not be read.")
			return
		}
		rows.Close()
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_model_read_failed", "Unable to read item models", "The item model transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"models": models})
		return
	}
	var request struct {
		ModelCodes []int `json:"modelCodes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_models", "Invalid item models", "The item model payload could not be parsed.")
		return
	}
	modelCodes, err := normalizeItemModelCodes(request.ModelCodes)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_models", "Invalid item models", err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM master_item_models WHERE tenant_id = $1::uuid AND item_id = $2::uuid
	`, operator.TenantID, itemID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_model_write_failed", "Item models were not saved", "The existing item model list could not be replaced.")
		return
	}
	result := make([]itemModelResponse, 0, len(modelCodes))
	for _, modelCode := range modelCodes {
		var model itemModelResponse
		if err := tx.QueryRowContext(r.Context(), `
			INSERT INTO master_item_models (tenant_id, item_id, legacy_item_id, model_code)
			VALUES ($1::uuid, $2::uuid, $3, $4)
			RETURNING id::text, model_code
		`, operator.TenantID, itemID, legacyItemID, modelCode).Scan(&model.ID, &model.ModelCode); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "item_model_write_failed", "Item models were not saved", "The item model membership was rejected by the database.")
			return
		}
		result = append(result, model)
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_model_write_failed", "Item models were not saved", "The item model transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": result})
}

type itemPricePolicyHeaderResponse struct {
	PolicyCode   string `json:"policyCode"`
	Name         string `json:"name"`
	LegacyItemID string `json:"legacyItemId"`
}

type itemPricePolicyTierRequest struct {
	ID              string `json:"id"`
	QuantityLimit   int    `json:"quantityLimit"`
	Price           string `json:"price"`
	ExpiryDate      string `json:"expiryDate"`
	FlatDiscount    string `json:"flatDiscount"`
	DiscountPercent string `json:"discountPercent"`
}

type itemPricePolicyTierResponse struct {
	ID              string `json:"id"`
	QuantityLimit   int    `json:"quantityLimit"`
	Price           string `json:"price"`
	ExpiryDate      string `json:"expiryDate"`
	FlatDiscount    string `json:"flatDiscount"`
	DiscountPercent string `json:"discountPercent"`
}

type itemPricePolicyResponse struct {
	Policy *itemPricePolicyHeaderResponse `json:"policy"`
	Tiers  []itemPricePolicyTierResponse  `json:"tiers"`
}

type normalizedItemPricePolicyTier struct {
	ID              string
	QuantityLimit   int
	Price           string
	ExpiryDate      string
	FlatDiscount    string
	DiscountPercent string
}

const maxItemPricePolicyTiers = 100

func normalizePricePolicyDecimal(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0", nil
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok {
		return "", fmt.Errorf("%s must be a decimal", field)
	}
	if !new(big.Rat).Mul(rat, big.NewRat(10000, 1)).IsInt() {
		return "", fmt.Errorf("%s must have at most four decimal places", field)
	}
	return value, nil
}

func isUUIDText(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func normalizeItemPricePolicyTiers(values []itemPricePolicyTierRequest) ([]normalizedItemPricePolicyTier, error) {
	if len(values) > maxItemPricePolicyTiers {
		return nil, fmt.Errorf("an item price policy may have at most %d tiers", maxItemPricePolicyTiers)
	}
	result := make([]normalizedItemPricePolicyTier, 0, len(values))
	seenIDs := make(map[string]struct{}, len(values))
	seenKeys := make(map[string]struct{}, len(values))
	for index, raw := range values {
		id := strings.TrimSpace(raw.ID)
		if id != "" {
			if !isUUIDText(id) {
				return nil, fmt.Errorf("tier %d has an invalid row identifier", index+1)
			}
			if _, exists := seenIDs[id]; exists {
				return nil, fmt.Errorf("price policy tier row identifiers must be unique")
			}
			seenIDs[id] = struct{}{}
		}
		expiryDate := strings.TrimSpace(raw.ExpiryDate)
		if expiryDate != "" {
			if _, err := time.Parse("2006-01-02", expiryDate); err != nil {
				return nil, fmt.Errorf("tier %d expiry date must be an ISO date", index+1)
			}
		}
		key := fmt.Sprintf("%d|%s", raw.QuantityLimit, expiryDate)
		if _, exists := seenKeys[key]; exists {
			return nil, fmt.Errorf("price policy quantity and expiry pairs must be unique")
		}
		seenKeys[key] = struct{}{}
		price, err := normalizePricePolicyDecimal(raw.Price, "price")
		if err != nil {
			return nil, fmt.Errorf("tier %d: %w", index+1, err)
		}
		flatDiscount, err := normalizePricePolicyDecimal(raw.FlatDiscount, "flat discount")
		if err != nil {
			return nil, fmt.Errorf("tier %d: %w", index+1, err)
		}
		discountPercent, err := normalizePricePolicyDecimal(raw.DiscountPercent, "discount percent")
		if err != nil {
			return nil, fmt.Errorf("tier %d: %w", index+1, err)
		}
		result = append(result, normalizedItemPricePolicyTier{
			ID:              id,
			QuantityLimit:   raw.QuantityLimit,
			Price:           price,
			ExpiryDate:      expiryDate,
			FlatDiscount:    flatDiscount,
			DiscountPercent: discountPercent,
		})
	}
	return result, nil
}

func itemPricePolicyPayload(policyCode string, tier normalizedItemPricePolicyTier) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"PricePolicyCode": policyCode,
		"QtyLimit":        tier.QuantityLimit,
		"Price":           tier.Price,
		"ExpiryDate":      tier.ExpiryDate,
		"ItemFlatDisc":    tier.FlatDiscount,
		"DiscPerc":        tier.DiscountPercent,
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func queryItemPricePolicy(ctx context.Context, tx *sql.Tx, tenantID, itemID string) (itemPricePolicyResponse, error) {
	var response itemPricePolicyResponse
	response.Tiers = make([]itemPricePolicyTierResponse, 0)
	var legacyItemID, itemCode string
	if err := tx.QueryRowContext(ctx, `
		SELECT legacy_id, code
		FROM master_items
		WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, itemID).Scan(&legacyItemID, &itemCode); err != nil {
		return response, err
	}
	var policy itemPricePolicyHeaderResponse
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(mr.legacy_id, mr.code), mr.name
		FROM master_records mr
		WHERE mr.tenant_id = $1::uuid
		  AND mr.kind IN ('price_policy', 'price-policy')
		  AND (mr.payload->>'ICode' = $2 OR mr.payload->>'ICode' = $3)
		ORDER BY CASE WHEN mr.payload->>'ICode' = $2 THEN 0 ELSE 1 END, mr.legacy_id, mr.code
		LIMIT 1
	`, tenantID, legacyItemID, itemCode).Scan(&policy.PolicyCode, &policy.Name); err != nil {
		if err == sql.ErrNoRows {
			return response, nil
		}
		return response, err
	}
	policy.LegacyItemID = legacyItemID
	response.Policy = &policy
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text, quantity_limit, price::text, COALESCE(expiry_date::text, ''),
		       flat_discount::text, discount_percent::text
		FROM price_policy_tiers
		WHERE tenant_id = $1::uuid AND legacy_policy_id = $2
		ORDER BY quantity_limit, expiry_date NULLS LAST, id
	`, tenantID, policy.PolicyCode)
	if err != nil {
		return response, err
	}
	defer rows.Close()
	for rows.Next() {
		var tier itemPricePolicyTierResponse
		if err := rows.Scan(&tier.ID, &tier.QuantityLimit, &tier.Price, &tier.ExpiryDate, &tier.FlatDiscount, &tier.DiscountPercent); err != nil {
			return response, err
		}
		response.Tiers = append(response.Tiers, tier)
	}
	return response, rows.Err()
}

func (s *Server) itemPricePolicy(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Price policies are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPut {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item price-policy store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	if r.Method == http.MethodGet {
		response, err := queryItemPricePolicy(r.Context(), tx, operator.TenantID, itemID)
		if err == sql.ErrNoRows {
			writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
			return
		}
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The item price-policy rows could not be queried.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The item price-policy transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	var request struct {
		PolicyCode string                       `json:"policyCode"`
		Tiers      []itemPricePolicyTierRequest `json:"tiers"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_price_policy", "Invalid item price policy", "The item price-policy payload could not be parsed.")
		return
	}
	request.PolicyCode = strings.TrimSpace(request.PolicyCode)
	if request.PolicyCode == "" || len(request.PolicyCode) > 160 {
		writeProblem(w, http.StatusBadRequest, "invalid_item_price_policy", "Invalid item price policy", "A source price-policy code is required.")
		return
	}
	tiers, err := normalizeItemPricePolicyTiers(request.Tiers)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_price_policy", "Invalid item price policy", err.Error())
		return
	}
	var legacyItemID, itemCode, policyName string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT i.legacy_id, i.code, mr.name
		FROM master_items i
		JOIN master_records mr
		  ON mr.tenant_id = i.tenant_id
		 AND mr.kind IN ('price_policy', 'price-policy')
		 AND COALESCE(mr.legacy_id, mr.code) = $2
		 AND (mr.payload->>'ICode' = i.legacy_id OR mr.payload->>'ICode' = i.code)
		WHERE i.tenant_id = $1::uuid AND i.id = $3::uuid
	`, operator.TenantID, request.PolicyCode, itemID).Scan(&legacyItemID, &itemCode, &policyName); err != nil {
		if err == sql.ErrNoRows {
			writeProblem(w, http.StatusNotFound, "item_price_policy_not_found", "Item price policy not found", "The selected source price policy is not linked to this item.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The item and source price-policy relationship could not be checked.")
		return
	}
	_ = legacyItemID
	_ = itemCode
	currentIDs := make(map[string]struct{})
	rows, err := tx.QueryContext(r.Context(), `
		SELECT id::text FROM price_policy_tiers
		WHERE tenant_id = $1::uuid AND legacy_policy_id = $2
	`, operator.TenantID, request.PolicyCode)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The existing price-policy rows could not be queried.")
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The existing price-policy rows could not be decoded.")
			return
		}
		currentIDs[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The existing price-policy rows could not be read.")
		return
	}
	rows.Close()
	keptIDs := make(map[string]struct{}, len(tiers))
	for _, tier := range tiers {
		payload, err := itemPricePolicyPayload(request.PolicyCode, tier)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "item_price_policy_write_failed", "Item price policy was not saved", "The price-policy payload could not be encoded.")
			return
		}
		if tier.ID != "" {
			if _, exists := currentIDs[tier.ID]; !exists {
				writeProblem(w, http.StatusBadRequest, "invalid_item_price_policy", "Invalid item price policy", "Every supplied tier row must belong to the selected item policy.")
				return
			}
			keptIDs[tier.ID] = struct{}{}
			result, err := tx.ExecContext(r.Context(), `
				UPDATE price_policy_tiers
				SET quantity_limit = $1, price = $2::numeric, expiry_date = NULLIF($3, '')::date,
				    flat_discount = $4::numeric, discount_percent = $5::numeric,
				    payload = $6::jsonb, updated_at = now()
				WHERE tenant_id = $7::uuid AND legacy_policy_id = $8 AND id = $9::uuid
			`, tier.QuantityLimit, tier.Price, tier.ExpiryDate, tier.FlatDiscount, tier.DiscountPercent, payload, operator.TenantID, request.PolicyCode, tier.ID)
			if err != nil {
				writeProblem(w, http.StatusUnprocessableEntity, "item_price_policy_write_failed", "Item price policy was not saved", "The existing price-policy tier was rejected by PostgreSQL.")
				return
			}
			if count, _ := result.RowsAffected(); count != 1 {
				writeProblem(w, http.StatusNotFound, "item_price_policy_not_found", "Item price policy not found", "The selected price-policy tier is outside the authenticated tenant scope.")
				return
			}
			continue
		}
		legacyTierID := fmt.Sprintf("%s:%d:%s", request.PolicyCode, tier.QuantityLimit, tier.ExpiryDate)
		var insertedID string
		if err := tx.QueryRowContext(r.Context(), `
			INSERT INTO price_policy_tiers (
				tenant_id, legacy_id, legacy_policy_id, quantity_limit, price, expiry_date,
				flat_discount, discount_percent, source_table, source_legacy_id, payload
			) VALUES ($1::uuid, $2, $3, $4, $5::numeric, NULLIF($6, '')::date,
			          $7::numeric, $8::numeric, 'PricePolicyDetail', $3, $9::jsonb)
			RETURNING id::text
		`, operator.TenantID, legacyTierID, request.PolicyCode, tier.QuantityLimit, tier.Price, tier.ExpiryDate, tier.FlatDiscount, tier.DiscountPercent, payload).Scan(&insertedID); err != nil {
			if isUniqueViolation(err) {
				writeProblem(w, http.StatusConflict, "item_price_policy_tier_exists", "Price-policy tier already exists", "The quantity and expiry pair already exists for this item policy.")
				return
			}
			writeProblem(w, http.StatusUnprocessableEntity, "item_price_policy_write_failed", "Item price policy was not saved", "The new price-policy tier was rejected by PostgreSQL.")
			return
		}
		keptIDs[insertedID] = struct{}{}
	}
	for id := range currentIDs {
		if _, keep := keptIDs[id]; keep {
			continue
		}
		if _, err := tx.ExecContext(r.Context(), `
			DELETE FROM price_policy_tiers
			WHERE tenant_id = $1::uuid AND legacy_policy_id = $2 AND id = $3::uuid
		`, operator.TenantID, request.PolicyCode, id); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "item_price_policy_write_failed", "Item price policy was not saved", "The removed price-policy tier could not be deleted.")
			return
		}
	}
	response, err := queryItemPricePolicy(r.Context(), tx, operator.TenantID, itemID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_read_failed", "Unable to read item price policy", "The saved price-policy rows could not be reloaded.")
		return
	}
	if response.Policy == nil {
		response.Policy = &itemPricePolicyHeaderResponse{PolicyCode: request.PolicyCode, Name: policyName, LegacyItemID: legacyItemID}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_price_policy_write_failed", "Item price policy was not saved", "The item price-policy transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type itemRegistrationRequestResponse struct {
	ID                string          `json:"id"`
	RequestCode       int             `json:"requestCode"`
	LegacyItemID      string          `json:"legacyItemId"`
	RequestedAt       string          `json:"requestedAt"`
	ServerName        string          `json:"serverName"`
	MachineName       string          `json:"machineName"`
	Sent              string          `json:"sent"`
	SentOn            string          `json:"sentOn"`
	SentBy            *int            `json:"sentBy,omitempty"`
	ServerRequestCode *int            `json:"serverRequestCode,omitempty"`
	Payload           json.RawMessage `json:"payload"`
}

func queryLatestItemRegistrationRequest(ctx context.Context, tx *sql.Tx, tenantID, itemID string) (*itemRegistrationRequestResponse, error) {
	var request itemRegistrationRequestResponse
	var sentBy, serverRequestCode sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, request_code, legacy_item_id, requested_at::text,
		       server_name, machine_name, sent, COALESCE(sent_on::text, ''),
		       sent_by, server_request_code, payload
		FROM master_item_registration_requests
		WHERE tenant_id = $1::uuid AND item_id = $2::uuid
		ORDER BY requested_at DESC, id DESC
		LIMIT 1
	`, tenantID, itemID).Scan(&request.ID, &request.RequestCode, &request.LegacyItemID, &request.RequestedAt,
		&request.ServerName, &request.MachineName, &request.Sent, &request.SentOn,
		&sentBy, &serverRequestCode, &request.Payload)
	if err != nil {
		return nil, err
	}
	if sentBy.Valid {
		value := int(sentBy.Int64)
		request.SentBy = &value
	}
	if serverRequestCode.Valid {
		value := int(serverRequestCode.Int64)
		request.ServerRequestCode = &value
	}
	return &request, nil
}

func normalizeItemRegistrationPayload(raw json.RawMessage, requestCode int, legacyItemID, itemName string, requestedAt time.Time) (json.RawMessage, error) {
	payload := make(map[string]any)
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("item payload is not a JSON object")
		}
	}
	payload["ItemRegReqCode"] = requestCode
	payload["Date"] = requestedAt.Format(time.RFC3339)
	payload["ICode"] = legacyItemID
	payload["Name"] = itemName
	payload["Sent"] = "N"
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func (s *Server) itemRegistrationRequest(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Registration requests are only available for items.")
		return
	}
	operator := currentSession(r)
	permission := "master.read"
	if r.Method == http.MethodPost {
		permission = "master.write"
	}
	if !s.requirePermission(r, w, operator, permission) || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item registration-request store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.PathValue("id"))
	var legacyItemID, itemName string
	var itemPayload json.RawMessage
	if err := tx.QueryRowContext(r.Context(), `
		SELECT legacy_id, name, payload
		FROM master_items
		WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, operator.TenantID, itemID).Scan(&legacyItemID, &itemName, &itemPayload); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Master-data record not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if r.Method == http.MethodGet {
		request, err := queryLatestItemRegistrationRequest(r.Context(), tx, operator.TenantID, itemID)
		if err != nil && err != sql.ErrNoRows {
			writeProblem(w, http.StatusServiceUnavailable, "item_registration_request_read_failed", "Unable to read item registration request", "The item registration-request history could not be queried.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "item_registration_request_read_failed", "Unable to read item registration request", "The item registration-request transaction could not be committed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"request": request})
		return
	}
	requestedAt := time.Now().UTC()
	var requestCode64 int64
	if err := tx.QueryRowContext(r.Context(), `SELECT nextval('master_item_registration_request_code_seq')`).Scan(&requestCode64); err != nil || requestCode64 > int64(^uint32(0)>>1) {
		writeProblem(w, http.StatusServiceUnavailable, "item_registration_request_write_failed", "Item registration request was not populated", "A source-sized request code could not be allocated.")
		return
	}
	requestCode := int(requestCode64)
	payload, err := normalizeItemRegistrationPayload(itemPayload, requestCode, legacyItemID, itemName, requestedAt)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_registration_request_write_failed", "Item registration request was not populated", "The source-shaped item payload could not be preserved.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO master_item_registration_requests (
			tenant_id, item_id, legacy_item_id, request_code, requested_at,
			server_name, machine_name, sent, payload
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, '', '', 'N', $6::jsonb)
	`, operator.TenantID, itemID, legacyItemID, requestCode, requestedAt, payload); err != nil {
		if isUniqueViolation(err) {
			writeProblem(w, http.StatusConflict, "item_registration_request_exists", "Item registration request was not populated", "The generated source request code already exists.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "item_registration_request_write_failed", "Item registration request was not populated", "The request snapshot was rejected by PostgreSQL.")
		return
	}
	request, err := queryLatestItemRegistrationRequest(r.Context(), tx, operator.TenantID, itemID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_registration_request_read_failed", "Unable to read item registration request", "The populated request snapshot could not be reloaded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_registration_request_write_failed", "Item registration request was not populated", "The item registration-request transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"request": request})
}

func (s *Server) itemSuppliers(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Suppliers are only available for items.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.read") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item supplier store could not be opened.")
		return
	}
	defer tx.Rollback()
	var itemID string
	if err := tx.QueryRowContext(r.Context(), `SELECT id::text FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid`, operator.TenantID, r.PathValue("id")).Scan(&itemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Item not found", "The item is outside the authenticated tenant scope.")
		return
	}
	suppliers, err := queryItemSuppliers(r.Context(), tx, operator.TenantID, itemID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_supplier_read_failed", "Unable to read item suppliers", "The item supplier grid could not be loaded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_supplier_read_failed", "Unable to read item suppliers", "The item supplier transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suppliers": suppliers})
}

func (s *Server) replaceItemSuppliers(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(strings.TrimSpace(r.PathValue("kind"))) != "item" {
		writeProblem(w, http.StatusBadRequest, "invalid_master_kind", "Invalid master-data kind", "Suppliers are only available for items.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "master.write") || !s.requireCanonicalMasterScope(r, w, operator) {
		return
	}
	var request struct {
		Suppliers []itemSupplierRequest `json:"suppliers"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_suppliers", "Invalid item suppliers", "The supplier grid could not be parsed.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The item supplier store could not be opened.")
		return
	}
	defer tx.Rollback()
	var itemID, legacyItemID string
	if err := tx.QueryRowContext(r.Context(), `SELECT id::text, legacy_id FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid`, operator.TenantID, r.PathValue("id")).Scan(&itemID, &legacyItemID); err != nil {
		writeProblem(w, http.StatusNotFound, "master_not_found", "Item not found", "The item is outside the authenticated tenant scope.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM item_suppliers WHERE tenant_id = $1::uuid AND (item_id = $2::uuid OR legacy_item_id = $3)`, operator.TenantID, itemID, legacyItemID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_supplier_write_failed", "Item suppliers were not saved", "The existing supplier grid could not be replaced.")
		return
	}
	seen := make(map[string]struct{}, len(request.Suppliers))
	for _, supplier := range request.Suppliers {
		supplier.LegacySupplierID = strings.TrimSpace(supplier.LegacySupplierID)
		if supplier.LegacySupplierID == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_item_suppliers", "Invalid item suppliers", "Each supplier requires a legacySupplierId.")
			return
		}
		if _, exists := seen[supplier.LegacySupplierID]; exists {
			writeProblem(w, http.StatusConflict, "duplicate_item_supplier", "Duplicate item supplier", "An item cannot contain the same supplier more than once.")
			return
		}
		seen[supplier.LegacySupplierID] = struct{}{}
		if len(supplier.Payload) == 0 {
			supplier.Payload = json.RawMessage(`{}`)
		}
		if !json.Valid(supplier.Payload) {
			writeProblem(w, http.StatusBadRequest, "invalid_item_suppliers", "Invalid item suppliers", "Supplier payload must be valid JSON.")
			return
		}
		if supplier.SupplierID != "" {
			var supplierExists bool
			if err := tx.QueryRowContext(r.Context(), `
				SELECT EXISTS (
					SELECT 1 FROM master_parties
					WHERE id = $1::uuid AND tenant_id = $2::uuid
					  AND party_type = 'supplier' AND active
				)`, supplier.SupplierID, operator.TenantID).Scan(&supplierExists); err != nil || !supplierExists {
				writeProblem(w, http.StatusBadRequest, "supplier_not_allowed", "Supplier not allowed", "The supplier is not active in the authenticated tenant.")
				return
			}
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO item_suppliers (
				tenant_id, item_id, supplier_id, legacy_item_id, legacy_supplier_id,
				priority, rate, discount_percent, quantity, bonus, days, payload
			)
			VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5,
			        $6, NULLIF($7, '')::numeric, NULLIF($8, '')::numeric,
			        NULLIF($9, '')::numeric, NULLIF($10, '')::numeric, $11, $12::jsonb)
		`, operator.TenantID, itemID, strings.TrimSpace(supplier.SupplierID), legacyItemID, supplier.LegacySupplierID,
			supplier.Priority, supplier.Rate, supplier.DiscountPercent, supplier.Quantity, supplier.Bonus, supplier.Days, supplier.Payload); err != nil {
			if isUniqueViolation(err) {
				writeProblem(w, http.StatusConflict, "duplicate_item_supplier", "Duplicate item supplier", "The item supplier link already exists.")
				return
			}
			writeProblem(w, http.StatusUnprocessableEntity, "item_supplier_write_failed", "Item suppliers were not saved", "The supplier grid was rejected by the database.")
			return
		}
	}
	suppliers, err := queryItemSuppliers(r.Context(), tx, operator.TenantID, itemID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_supplier_read_failed", "Unable to read item suppliers", "The saved supplier grid could not be loaded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "item_supplier_write_failed", "Item suppliers were not saved", "The supplier transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suppliers": suppliers})
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique")
}

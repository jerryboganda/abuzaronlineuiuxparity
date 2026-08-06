package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

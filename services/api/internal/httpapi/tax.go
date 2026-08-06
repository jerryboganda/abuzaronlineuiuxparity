package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type taxRateRequest struct {
	TaxKind        string `json:"taxKind"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Rate           string `json:"rate"`
	Inclusive      bool   `json:"inclusive"`
	EffectiveFrom  string `json:"effectiveFrom"`
	EffectiveTo    string `json:"effectiveTo,omitempty"`
	SourceTable    string `json:"sourceTable,omitempty"`
	SourceLegacyID string `json:"sourceLegacyId,omitempty"`
	Active         *bool  `json:"active,omitempty"`
}

type taxRateResponse struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenantId"`
	BranchID       string `json:"branchId"`
	TaxKind        string `json:"taxKind"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Rate           string `json:"rate"`
	Inclusive      bool   `json:"inclusive"`
	EffectiveFrom  string `json:"effectiveFrom"`
	EffectiveTo    string `json:"effectiveTo,omitempty"`
	SourceTable    string `json:"sourceTable,omitempty"`
	SourceLegacyID string `json:"sourceLegacyId,omitempty"`
	Active         bool   `json:"active"`
}

type taxAssignmentRequest struct {
	TaxRateID      string `json:"taxRateId"`
	EffectiveFrom  string `json:"effectiveFrom"`
	EffectiveTo    string `json:"effectiveTo,omitempty"`
	SourceTable    string `json:"sourceTable,omitempty"`
	SourceLegacyID string `json:"sourceLegacyId,omitempty"`
}

type taxAssignmentResponse struct {
	ID             string          `json:"id"`
	TargetKind     string          `json:"targetKind"`
	ItemID         string          `json:"itemId,omitempty"`
	PartyID        string          `json:"partyId,omitempty"`
	TaxRate        taxRateResponse `json:"taxRate"`
	EffectiveFrom  string          `json:"effectiveFrom"`
	EffectiveTo    string          `json:"effectiveTo,omitempty"`
	SourceTable    string          `json:"sourceTable,omitempty"`
	SourceLegacyID string          `json:"sourceLegacyId,omitempty"`
}

type applyItemGSTRequest struct {
	RateID         string   `json:"rateId,omitempty"`
	Rate           string   `json:"rate,omitempty"`
	Inclusive      bool     `json:"inclusive"`
	EffectiveFrom  string   `json:"effectiveFrom"`
	EffectiveTo    string   `json:"effectiveTo,omitempty"`
	ItemIDs        []string `json:"itemIds,omitempty"`
	SourceTable    string   `json:"sourceTable,omitempty"`
	SourceLegacyID string   `json:"sourceLegacyId,omitempty"`
}

func (s *Server) taxRates(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if r.Method == http.MethodGet {
		if !s.requirePermission(r, w, operator, "tax.read") {
			return
		}
	} else if !s.requirePermission(r, w, operator, "tax.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select a branch before accessing tax configuration.")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.readTaxRates(w, r, operator)
	case http.MethodPost:
		s.createTaxRate(w, r, operator)
	case http.MethodPatch:
		s.updateTaxRate(w, r, operator)
	case http.MethodDelete:
		s.deleteTaxRate(w, r, operator)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) readTaxRates(w http.ResponseWriter, r *http.Request, operator *sessionContext) {
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax rates", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	args := []any{operator.TenantID, operator.BranchID}
	query := `SELECT id::text, tenant_id::text, branch_id::text, tax_kind, code, name,
		rate::text, inclusive, effective_from::text, COALESCE(effective_to::text, ''),
		source_table, source_legacy_id, active
		FROM tax_rates WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`
	if effectiveAt := strings.TrimSpace(r.URL.Query().Get("effectiveAt")); effectiveAt != "" {
		if _, err := parseTaxDate(effectiveAt, "effectiveAt"); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_tax_date", "Invalid tax date", err.Error())
			return
		}
		args = append(args, effectiveAt)
		query += ` AND effective_from <= $3::date AND (effective_to IS NULL OR effective_to >= $3::date)`
	}
	query += ` ORDER BY tax_kind, effective_from DESC, code`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax rates", "The tax-rate query failed.")
		return
	}
	rates, err := scanTaxRates(rows)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax rates", "The tax-rate response could not be decoded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax rates", "The tax transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rates": rates})
}

func (s *Server) createTaxRate(w http.ResponseWriter, r *http.Request, operator *sessionContext) {
	var request taxRateRequest
	if err := decodeTaxJSON(w, r, &request); err != nil {
		return
	}
	normalized, err := normalizeTaxRateRequest(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_tax_rate", "Invalid tax rate", err.Error())
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax rate could not be created", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	active := true
	if normalized.Active != nil {
		active = *normalized.Active
	}
	var response taxRateResponse
	err = tx.QueryRowContext(r.Context(), `INSERT INTO tax_rates
		(tenant_id, branch_id, tax_kind, code, name, rate, inclusive, effective_from,
		 effective_to, source_table, source_legacy_id, active)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::numeric, $7, $8::date,
		        NULLIF($9, '')::date, $10, $11, $12)
		RETURNING id::text, tenant_id::text, branch_id::text, tax_kind, code, name,
		          rate::text, inclusive, effective_from::text, COALESCE(effective_to::text, ''),
		          source_table, source_legacy_id, active`,
		operator.TenantID, operator.BranchID, normalized.TaxKind, normalized.Code,
		normalized.Name, normalized.Rate, normalized.Inclusive,
		normalized.EffectiveFrom, normalized.EffectiveTo, normalized.SourceTable,
		normalized.SourceLegacyID, active).Scan(taxRateScanArgs(&response)...)
	if err != nil {
		if isUniqueViolation(err) {
			writeProblem(w, http.StatusConflict, "tax_rate_exists", "Tax rate already exists", "The tax code already exists in this tenant branch.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "tax_rate_create_failed", "Tax rate could not be created", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax rate could not be created", "The tax transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) updateTaxRate(w http.ResponseWriter, r *http.Request, operator *sessionContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !documentUUIDPattern.MatchString(id) {
		writeProblem(w, http.StatusBadRequest, "tax_rate_required", "Tax rate required", "Provide a canonical tax-rate identifier.")
		return
	}
	var request taxRateRequest
	if err := decodeTaxJSON(w, r, &request); err != nil {
		return
	}
	normalized, err := normalizeTaxRateRequest(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_tax_rate", "Invalid tax rate", err.Error())
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax rate could not be updated", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	active := true
	if normalized.Active != nil {
		active = *normalized.Active
	}
	var response taxRateResponse
	err = tx.QueryRowContext(r.Context(), `UPDATE tax_rates
		SET tax_kind = $1, code = $2, name = $3, rate = $4::numeric, inclusive = $5,
		    effective_from = $6::date, effective_to = NULLIF($7, '')::date,
		    source_table = $8, source_legacy_id = $9, active = $10, updated_at = now()
		WHERE id = $11::uuid AND tenant_id = $12::uuid AND branch_id = $13::uuid
		RETURNING id::text, tenant_id::text, branch_id::text, tax_kind, code, name,
		          rate::text, inclusive, effective_from::text, COALESCE(effective_to::text, ''),
		          source_table, source_legacy_id, active`,
		normalized.TaxKind, normalized.Code, normalized.Name, normalized.Rate,
		normalized.Inclusive, normalized.EffectiveFrom, normalized.EffectiveTo,
		normalized.SourceTable, normalized.SourceLegacyID, active, id, operator.TenantID,
		operator.BranchID).Scan(taxRateScanArgs(&response)...)
	if errors.Is(err, sql.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "tax_rate_not_found", "Tax rate not found", "The tax rate is outside the authenticated branch scope.")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeProblem(w, http.StatusConflict, "tax_rate_exists", "Tax rate already exists", "The tax code already exists in this tenant branch.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "tax_rate_update_failed", "Tax rate could not be updated", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax rate could not be updated", "The tax transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) deleteTaxRate(w http.ResponseWriter, r *http.Request, operator *sessionContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !documentUUIDPattern.MatchString(id) {
		writeProblem(w, http.StatusBadRequest, "tax_rate_required", "Tax rate required", "Provide a canonical tax-rate identifier.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax rate could not be deleted", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `DELETE FROM tax_rates
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid`,
		id, operator.TenantID, operator.BranchID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "violates foreign key") {
			writeProblem(w, http.StatusConflict, "tax_rate_in_use", "Tax rate is in use", "Remove its item and party assignments before deleting it.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "tax_rate_delete_failed", "Tax rate could not be deleted", err.Error())
		return
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		writeProblem(w, http.StatusNotFound, "tax_rate_not_found", "Tax rate not found", "The tax rate is outside the authenticated branch scope.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax rate could not be deleted", "The tax transaction could not be committed.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) taxAssignments(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if r.Method == http.MethodGet {
		if !s.requirePermission(r, w, operator, "tax.read") {
			return
		}
	} else if !s.requirePermission(r, w, operator, "tax.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select a branch before accessing tax assignments.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.readTaxAssignments(w, r, operator)
	case http.MethodDelete:
		s.deleteTaxAssignment(w, r, operator)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) replaceItemTaxAssignments(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "tax.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select a branch before replacing tax assignments.")
		return
	}
	s.replaceTaxAssignments(w, r, operator, "item", strings.TrimSpace(r.PathValue("itemId")))
}

func (s *Server) replacePartyTaxAssignments(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "tax.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select a branch before replacing tax assignments.")
		return
	}
	s.replaceTaxAssignments(w, r, operator, "party", strings.TrimSpace(r.PathValue("partyId")))
}

func (s *Server) replaceTaxAssignments(w http.ResponseWriter, r *http.Request, operator *sessionContext, targetKind, targetID string) {
	if !documentUUIDPattern.MatchString(targetID) {
		writeProblem(w, http.StatusBadRequest, "tax_target_required", "Tax target required", "Provide a canonical item or party identifier.")
		return
	}
	var body struct {
		Assignments []taxAssignmentRequest `json:"assignments"`
	}
	if err := decodeTaxJSON(w, r, &body); err != nil {
		return
	}
	if len(body.Assignments) > 20 {
		writeProblem(w, http.StatusBadRequest, "too_many_tax_assignments", "Too many tax assignments", "A target may have at most 20 effective assignments.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax assignments could not be replaced", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	table := "item_tax_assignments"
	targetColumn := "item_id"
	if targetKind == "party" {
		table = "party_tax_assignments"
		targetColumn = "party_id"
	}
	var targetExists bool
	targetTable := "master_items"
	targetWhere := "active"
	if targetKind == "party" {
		targetTable = "master_parties"
		targetWhere = "active AND party_type IN ('customer', 'supplier')"
	}
	if err := tx.QueryRowContext(r.Context(), fmt.Sprintf(`SELECT EXISTS (
		SELECT 1 FROM %s WHERE tenant_id = $1::uuid AND id = $2::uuid AND %s
	)`, targetTable, targetWhere), operator.TenantID, targetID).Scan(&targetExists); err != nil || !targetExists {
		writeProblem(w, http.StatusBadRequest, "tax_target_not_found", "Tax target not found", "The target is not active in the authenticated tenant.")
		return
	}
	type checkedAssignment struct {
		request taxAssignmentRequest
		kind    string
	}
	checked := make([]checkedAssignment, 0, len(body.Assignments))
	for _, assignment := range body.Assignments {
		if !documentUUIDPattern.MatchString(strings.TrimSpace(assignment.TaxRateID)) {
			writeProblem(w, http.StatusBadRequest, "invalid_tax_assignment", "Invalid tax assignment", "taxRateId must be a UUID.")
			return
		}
		from, to, dateErr := validateTaxDates(assignment.EffectiveFrom, assignment.EffectiveTo)
		if dateErr != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_tax_date", "Invalid tax date", dateErr.Error())
			return
		}
		var kind string
		var active bool
		if err := tx.QueryRowContext(r.Context(), `SELECT tax_kind, active FROM tax_rates
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid`,
			assignment.TaxRateID, operator.TenantID, operator.BranchID).Scan(&kind, &active); err != nil || !active {
			writeProblem(w, http.StatusBadRequest, "tax_rate_not_found", "Tax rate not found", "Every assignment must reference an active rate in the authenticated branch.")
			return
		}
		assignment.TaxRateID = strings.TrimSpace(assignment.TaxRateID)
		assignment.EffectiveFrom, assignment.EffectiveTo = from, to
		for _, existing := range checked {
			if existing.kind != kind {
				continue
			}
			if rangesOverlap(existing.request.EffectiveFrom, existing.request.EffectiveTo, from, to) &&
				existing.request.TaxRateID != assignment.TaxRateID {
				writeProblem(w, http.StatusConflict, "tax_assignment_conflict", "Conflicting tax assignment", "Overlapping assignments for one target and tax kind must use one rate.")
				return
			}
		}
		checked = append(checked, checkedAssignment{request: assignment, kind: kind})
	}
	if _, err := tx.ExecContext(r.Context(), fmt.Sprintf(`DELETE FROM %s
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND %s = $3::uuid`, table, targetColumn),
		operator.TenantID, operator.BranchID, targetID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "tax_assignment_replace_failed", "Tax assignments could not be replaced", err.Error())
		return
	}
	for _, assignment := range checked {
		if _, err := tx.ExecContext(r.Context(), fmt.Sprintf(`INSERT INTO %s
			(tenant_id, branch_id, %s, tax_rate_id, effective_from, effective_to, source_table, source_legacy_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::date, NULLIF($6, '')::date, $7, $8)`,
			table, targetColumn), operator.TenantID, operator.BranchID, targetID,
			assignment.request.TaxRateID, assignment.request.EffectiveFrom, assignment.request.EffectiveTo,
			assignment.request.SourceTable, assignment.request.SourceLegacyID); err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "tax_assignment_replace_failed", "Tax assignments could not be replaced", err.Error())
			return
		}
	}
	assignments, err := queryTaxAssignments(r.Context(), tx, operator, targetKind, targetID, "")
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Tax assignments could not be read", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax assignments could not be replaced", "The tax transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assignments": assignments})
}

func (s *Server) readTaxAssignments(w http.ResponseWriter, r *http.Request, operator *sessionContext) {
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax assignments", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	itemID := strings.TrimSpace(r.URL.Query().Get("itemId"))
	partyID := strings.TrimSpace(r.URL.Query().Get("partyId"))
	if itemID != "" && !documentUUIDPattern.MatchString(itemID) || partyID != "" && !documentUUIDPattern.MatchString(partyID) {
		writeProblem(w, http.StatusBadRequest, "invalid_tax_target", "Invalid tax target", "itemId and partyId must be UUIDs.")
		return
	}
	effectiveAt := strings.TrimSpace(r.URL.Query().Get("effectiveAt"))
	if effectiveAt != "" {
		if _, err := parseTaxDate(effectiveAt, "effectiveAt"); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_tax_date", "Invalid tax date", err.Error())
			return
		}
	}
	items, err := queryTaxAssignments(r.Context(), tx, operator, "item", itemID, effectiveAt)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax assignments", err.Error())
		return
	}
	parties, err := queryTaxAssignments(r.Context(), tx, operator, "party", partyID, effectiveAt)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax assignments", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_read_failed", "Unable to read tax assignments", "The tax transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "parties": parties})
}

func (s *Server) deleteTaxAssignment(w http.ResponseWriter, r *http.Request, operator *sessionContext) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !documentUUIDPattern.MatchString(id) {
		writeProblem(w, http.StatusBadRequest, "tax_assignment_required", "Tax assignment required", "Provide a canonical assignment identifier.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax assignment could not be deleted", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `DELETE FROM item_tax_assignments
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid`,
		id, operator.TenantID, operator.BranchID)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "tax_assignment_delete_failed", "Tax assignment could not be deleted", err.Error())
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		result, err = tx.ExecContext(r.Context(), `DELETE FROM party_tax_assignments
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid`,
			id, operator.TenantID, operator.BranchID)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "tax_assignment_delete_failed", "Tax assignment could not be deleted", err.Error())
			return
		}
		count, _ = result.RowsAffected()
	}
	if count == 0 {
		writeProblem(w, http.StatusNotFound, "tax_assignment_not_found", "Tax assignment not found", "The assignment is outside the authenticated branch scope.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Tax assignment could not be deleted", "The tax transaction could not be committed.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) applyItemGST(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "tax.write") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "scope_required", "Branch required", "Select a branch before applying item GST.")
		return
	}
	var request applyItemGSTRequest
	if err := decodeTaxJSON(w, r, &request); err != nil {
		return
	}
	if request.RateID == "" && strings.TrimSpace(request.Rate) == "" {
		writeProblem(w, http.StatusBadRequest, "gst_rate_required", "GST rate required", "Provide rateId or rate.")
		return
	}
	from, to, err := validateTaxDates(request.EffectiveFrom, request.EffectiveTo)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_tax_date", "Invalid tax date", err.Error())
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Item GST could not be applied", "The tax store could not be opened.")
		return
	}
	defer tx.Rollback()
	rateID := strings.TrimSpace(request.RateID)
	if rateID != "" && !documentUUIDPattern.MatchString(rateID) {
		writeProblem(w, http.StatusBadRequest, "invalid_tax_rate", "Invalid tax rate", "rateId must be a UUID.")
		return
	}
	if rateID == "" {
		rate, parseErr := parsePercent(request.Rate)
		if parseErr != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_tax_rate", "Invalid tax rate", parseErr.Error())
			return
		}
		code := "GST-" + formatPercent(rate)
		err = tx.QueryRowContext(r.Context(), `INSERT INTO tax_rates
			(tenant_id, branch_id, tax_kind, code, name, rate, inclusive, effective_from, effective_to,
			 source_table, source_legacy_id)
			VALUES ($1::uuid, $2::uuid, 'gst', $3, $4, $5::numeric, $6, $7::date, NULLIF($8, '')::date, $9, $10)
			ON CONFLICT (tenant_id, branch_id, tax_kind, code) DO UPDATE
			SET rate = EXCLUDED.rate, inclusive = EXCLUDED.inclusive,
			    effective_from = EXCLUDED.effective_from, effective_to = EXCLUDED.effective_to,
			    updated_at = now()
			RETURNING id::text`,
			operator.TenantID, operator.BranchID, code, "GST "+formatPercent(rate)+"%",
			formatPercent(rate), request.Inclusive, from, to, request.SourceTable,
			request.SourceLegacyID).Scan(&rateID)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "tax_rate_create_failed", "GST rate could not be configured", err.Error())
			return
		}
	}
	var rateKind string
	if err := tx.QueryRowContext(r.Context(), `SELECT tax_kind FROM tax_rates
		WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND active`,
		rateID, operator.TenantID, operator.BranchID).Scan(&rateKind); err != nil {
		writeProblem(w, http.StatusBadRequest, "tax_rate_not_found", "GST rate not found", "The rate is outside the authenticated branch scope.")
		return
	}
	if rateKind != "gst" {
		writeProblem(w, http.StatusBadRequest, "tax_rate_kind_mismatch", "Tax rate kind mismatch", "Apply Item GST requires a GST rate.")
		return
	}
	itemIDs := request.ItemIDs
	if len(itemIDs) == 0 {
		rows, queryErr := tx.QueryContext(r.Context(), `SELECT id::text FROM master_items
			WHERE tenant_id = $1::uuid AND active ORDER BY id`, operator.TenantID)
		if queryErr != nil {
			writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Item GST could not be applied", queryErr.Error())
			return
		}
		for rows.Next() {
			var itemID string
			if scanErr := rows.Scan(&itemID); scanErr != nil {
				rows.Close()
				writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Item GST could not be applied", scanErr.Error())
				return
			}
			itemIDs = append(itemIDs, itemID)
		}
		rows.Close()
	}
	for _, itemID := range itemIDs {
		if !documentUUIDPattern.MatchString(strings.TrimSpace(itemID)) {
			writeProblem(w, http.StatusBadRequest, "invalid_item_id", "Invalid item", "Every itemId must be a UUID.")
			return
		}
		if err := insertOrRejectItemTaxAssignment(r.Context(), tx, operator, itemID, rateID, from, to, request.SourceTable, request.SourceLegacyID); err != nil {
			writeProblem(w, http.StatusConflict, "tax_assignment_conflict", "Item GST could not be applied", err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tax_write_failed", "Item GST could not be applied", "The tax transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rateId": rateID, "itemsApplied": len(itemIDs), "effectiveFrom": from, "effectiveTo": to})
}

func decodeTaxJSON(w http.ResponseWriter, r *http.Request, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_tax_json", "Invalid tax configuration", "The tax configuration could not be parsed.")
		return err
	}
	return nil
}

func normalizeTaxRateRequest(request taxRateRequest) (taxRateRequest, error) {
	request.TaxKind = strings.ToLower(strings.TrimSpace(request.TaxKind))
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	request.SourceTable = strings.TrimSpace(request.SourceTable)
	request.SourceLegacyID = strings.TrimSpace(request.SourceLegacyID)
	if request.TaxKind != "gst" && request.TaxKind != "pct" && request.TaxKind != "advance" {
		return request, errors.New("taxKind must be gst, pct, or advance")
	}
	if request.Code == "" || request.Name == "" || len(request.Code) > 120 || len(request.Name) > 240 {
		return request, errors.New("code and name are required")
	}
	rate, err := parsePercent(request.Rate)
	if err != nil {
		return request, fmt.Errorf("rate: %w", err)
	}
	if rate > 10000 {
		return request, errors.New("rate must be between 0 and 100 percent")
	}
	request.Rate = formatPercent(rate)
	if _, _, err := validateTaxDates(request.EffectiveFrom, request.EffectiveTo); err != nil {
		return request, err
	}
	request.EffectiveFrom, request.EffectiveTo, _ = validateTaxDates(request.EffectiveFrom, request.EffectiveTo)
	return request, nil
}

func validateTaxDates(fromValue, toValue string) (string, string, error) {
	from, err := parseTaxDate(fromValue, "effectiveFrom")
	if err != nil {
		return "", "", err
	}
	to := ""
	if strings.TrimSpace(toValue) != "" {
		parsed, parseErr := parseTaxDate(toValue, "effectiveTo")
		if parseErr != nil {
			return "", "", parseErr
		}
		if parsed.Before(from) {
			return "", "", errors.New("effectiveTo cannot be before effectiveFrom")
		}
		to = parsed.Format("2006-01-02")
	}
	return from.Format("2006-01-02"), to, nil
}

func parseTaxDate(value, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return parsed, nil
}

func rangesOverlap(firstFrom, firstTo, secondFrom, secondTo string) bool {
	firstEnd, secondEnd := "9999-12-31", "9999-12-31"
	if firstTo != "" {
		firstEnd = firstTo
	}
	if secondTo != "" {
		secondEnd = secondTo
	}
	return firstFrom <= secondEnd && secondFrom <= firstEnd
}

func taxRateScanArgs(response *taxRateResponse) []any {
	return []any{
		&response.ID, &response.TenantID, &response.BranchID, &response.TaxKind,
		&response.Code, &response.Name, &response.Rate, &response.Inclusive,
		&response.EffectiveFrom, &response.EffectiveTo, &response.SourceTable,
		&response.SourceLegacyID, &response.Active,
	}
}

func scanTaxRates(rows *sql.Rows) ([]taxRateResponse, error) {
	defer rows.Close()
	result := make([]taxRateResponse, 0)
	for rows.Next() {
		var rate taxRateResponse
		if err := rows.Scan(taxRateScanArgs(&rate)...); err != nil {
			return nil, err
		}
		result = append(result, rate)
	}
	return result, rows.Err()
}

func queryTaxAssignments(ctx context.Context, tx *sql.Tx, operator *sessionContext, targetKind, targetID, effectiveAt string) ([]taxAssignmentResponse, error) {
	args := []any{operator.TenantID, operator.BranchID}
	query := `SELECT a.id::text, a.item_id::text, ''::text, a.effective_from::text,
		COALESCE(a.effective_to::text, ''), a.source_table, a.source_legacy_id,
		r.id::text, r.tenant_id::text, r.branch_id::text, r.tax_kind, r.code, r.name,
		r.rate::text, r.inclusive, r.effective_from::text, COALESCE(r.effective_to::text, ''),
		r.source_table, r.source_legacy_id, r.active
		FROM item_tax_assignments a JOIN tax_rates r
		  ON r.tenant_id = a.tenant_id AND r.branch_id = a.branch_id AND r.id = a.tax_rate_id
		WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid`
	if targetKind == "party" {
		query = `SELECT a.id::text, ''::text, a.party_id::text, a.effective_from::text,
			COALESCE(a.effective_to::text, ''), a.source_table, a.source_legacy_id,
			r.id::text, r.tenant_id::text, r.branch_id::text, r.tax_kind, r.code, r.name,
			r.rate::text, r.inclusive, r.effective_from::text, COALESCE(r.effective_to::text, ''),
			r.source_table, r.source_legacy_id, r.active
			FROM party_tax_assignments a JOIN tax_rates r
			  ON r.tenant_id = a.tenant_id AND r.branch_id = a.branch_id AND r.id = a.tax_rate_id
			WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid`
	}
	if targetID != "" {
		args = append(args, targetID)
		query += fmt.Sprintf(" AND a.%s_id = $%d::uuid", targetKind, len(args))
	}
	if effectiveAt != "" {
		args = append(args, effectiveAt)
		query += fmt.Sprintf(" AND a.effective_from <= $%d::date AND (a.effective_to IS NULL OR a.effective_to >= $%d::date)", len(args), len(args))
	}
	query += " ORDER BY a.effective_from DESC, r.tax_kind, r.code"
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]taxAssignmentResponse, 0)
	for rows.Next() {
		var item taxAssignmentResponse
		if err := rows.Scan(&item.ID, &item.ItemID, &item.PartyID, &item.EffectiveFrom,
			&item.EffectiveTo, &item.SourceTable, &item.SourceLegacyID,
			&item.TaxRate.ID, &item.TaxRate.TenantID, &item.TaxRate.BranchID,
			&item.TaxRate.TaxKind, &item.TaxRate.Code, &item.TaxRate.Name,
			&item.TaxRate.Rate, &item.TaxRate.Inclusive, &item.TaxRate.EffectiveFrom,
			&item.TaxRate.EffectiveTo, &item.TaxRate.SourceTable,
			&item.TaxRate.SourceLegacyID, &item.TaxRate.Active); err != nil {
			return nil, err
		}
		item.TargetKind = targetKind
		result = append(result, item)
	}
	return result, rows.Err()
}

func insertOrRejectItemTaxAssignment(ctx context.Context, tx *sql.Tx, operator *sessionContext, itemID, rateID, from, to, sourceTable, sourceLegacyID string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM master_items
		WHERE tenant_id = $1::uuid AND id = $2::uuid AND active)`,
		operator.TenantID, itemID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("item is not an active canonical item in the authenticated tenant")
	}
	var conflict bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM item_tax_assignments a
		JOIN tax_rates old_rate ON old_rate.tenant_id = a.tenant_id
		  AND old_rate.branch_id = a.branch_id AND old_rate.id = a.tax_rate_id
		JOIN tax_rates new_rate ON new_rate.tenant_id = $1::uuid
		  AND new_rate.branch_id = $2::uuid AND new_rate.id = $4::uuid
		WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid AND a.item_id = $3::uuid
		  AND old_rate.tax_kind = new_rate.tax_kind
		  AND a.effective_from <= COALESCE(NULLIF($6, '')::date, '9999-12-31'::date)
		  AND COALESCE(a.effective_to, '9999-12-31'::date) >= $5::date
		  AND a.tax_rate_id <> $4::uuid
	)`, operator.TenantID, operator.BranchID, itemID, rateID, from, to).Scan(&conflict); err != nil {
		return err
	}
	if conflict {
		return errors.New("an effective assignment for this item and tax kind already has a different rate")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO item_tax_assignments
		(tenant_id, branch_id, item_id, tax_rate_id, effective_from, effective_to, source_table, source_legacy_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::date, NULLIF($6, '')::date, $7, $8)
		ON CONFLICT (tenant_id, branch_id, item_id, tax_rate_id, effective_from) DO UPDATE
		SET effective_to = EXCLUDED.effective_to, source_table = EXCLUDED.source_table,
		    source_legacy_id = EXCLUDED.source_legacy_id, updated_at = now()`,
		operator.TenantID, operator.BranchID, itemID, rateID, from, to, sourceTable, sourceLegacyID)
	return err
}

func resolveDocumentTaxPolicy(ctx context.Context, tx *sql.Tx, operator *sessionContext, draft *documentDraftRequest) (pricingPreviewTaxes, error) {
	var resolved pricingPreviewTaxes
	explicit := explicitDocumentTaxPolicy(draft)
	if explicit {
		if !hasPermission(operator, "tax.override") {
			return resolved, errors.New("explicit tax pricing requires tax.override permission")
		}
		return normalizeExplicitDocumentTaxes(draft)
	}
	when, err := time.Parse(time.RFC3339, draft.OccurredAt)
	if err != nil {
		return resolved, errors.New("document occurredAt must be RFC3339")
	}
	effectiveAt := when.UTC().Format("2006-01-02")
	partyID := strings.TrimSpace(draft.CustomerID)
	if isPurchaseDocumentKind(draft.Kind) {
		partyID = strings.TrimSpace(draft.SupplierID)
	}
	type taxCandidate struct {
		rule pricingPreviewTax
	}
	candidates := make(map[string]taxCandidate)
	add := func(kind string, rate string, inclusive bool) error {
		parsed, parseErr := parsePercent(rate)
		if parseErr != nil {
			return fmt.Errorf("canonical %s rate is invalid: %w", kind, parseErr)
		}
		candidate := taxCandidate{rule: pricingPreviewTax{Rate: formatPercent(parsed), Inclusive: inclusive}}
		if existing, ok := candidates[kind]; ok {
			if existing.rule.Rate != candidate.rule.Rate || existing.rule.Inclusive != candidate.rule.Inclusive {
				return fmt.Errorf("conflicting effective %s tax assignments were found", kind)
			}
			return nil
		}
		candidates[kind] = candidate
		return nil
	}
	for index, line := range draft.Lines {
		rows, queryErr := tx.QueryContext(ctx, `SELECT r.tax_kind, r.rate::text, r.inclusive
				FROM item_tax_assignments a
				JOIN tax_rates r ON r.tenant_id = a.tenant_id AND r.branch_id = a.branch_id
				  AND r.id = a.tax_rate_id
				WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid AND a.item_id = $3::uuid
				  AND a.effective_from <= $4::date
				  AND (a.effective_to IS NULL OR a.effective_to >= $4::date)
				  AND r.active AND r.effective_from <= $4::date
				  AND (r.effective_to IS NULL OR r.effective_to >= $4::date)
				ORDER BY r.tax_kind, a.effective_from DESC`, operator.TenantID, operator.BranchID, line.ItemID, effectiveAt)
		if queryErr != nil {
			return resolved, queryErr
		}
		for rows.Next() {
			var kind, rate string
			var inclusive bool
			if scanErr := rows.Scan(&kind, &rate, &inclusive); scanErr != nil {
				rows.Close()
				return resolved, scanErr
			}
			if addErr := add(kind, rate, inclusive); addErr != nil {
				rows.Close()
				return resolved, fmt.Errorf("line %d: %w", index+1, addErr)
			}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return resolved, rowsErr
		}
		rows.Close()
	}
	if partyID != "" {
		rows, queryErr := tx.QueryContext(ctx, `SELECT r.tax_kind, r.rate::text, r.inclusive
				FROM party_tax_assignments a
				JOIN tax_rates r ON r.tenant_id = a.tenant_id AND r.branch_id = a.branch_id
				  AND r.id = a.tax_rate_id
				WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid AND a.party_id = $3::uuid
				  AND a.effective_from <= $4::date
				  AND (a.effective_to IS NULL OR a.effective_to >= $4::date)
				  AND r.active AND r.effective_from <= $4::date
				  AND (r.effective_to IS NULL OR r.effective_to >= $4::date)
				ORDER BY r.tax_kind, a.effective_from DESC`, operator.TenantID, operator.BranchID, partyID, effectiveAt)
		if queryErr != nil {
			return resolved, queryErr
		}
		for rows.Next() {
			var kind, rate string
			var inclusive bool
			if scanErr := rows.Scan(&kind, &rate, &inclusive); scanErr != nil {
				rows.Close()
				return resolved, scanErr
			}
			if addErr := add(kind, rate, inclusive); addErr != nil {
				rows.Close()
				return resolved, fmt.Errorf("party tax assignment: %w", addErr)
			}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return resolved, rowsErr
		}
		rows.Close()
	}
	for kind, candidate := range candidates {
		switch kind {
		case "gst":
			resolved.GST = &candidate.rule
		case "pct":
			resolved.PCT = &candidate.rule
		case "advance":
			resolved.AdvanceTax = &candidate.rule
		default:
			return resolved, fmt.Errorf("unsupported canonical tax kind %q", kind)
		}
	}
	return resolved, nil
}

func explicitDocumentTaxPolicy(draft *documentDraftRequest) bool {
	if draft.Pricing != nil {
		for _, tax := range []*pricingPreviewTax{draft.Pricing.Taxes.GST, draft.Pricing.Taxes.PCT, draft.Pricing.Taxes.AdvanceTax} {
			if tax != nil && strings.TrimSpace(tax.Rate) != "" {
				return true
			}
		}
	}
	for _, line := range draft.Lines {
		if strings.TrimSpace(line.GSTRate) != "" || strings.TrimSpace(line.PCTRate) != "" || strings.TrimSpace(line.AdvanceTaxRate) != "" {
			return true
		}
	}
	return false
}

func normalizeExplicitDocumentTaxes(draft *documentDraftRequest) (pricingPreviewTaxes, error) {
	var result pricingPreviewTaxes
	if draft.Pricing != nil {
		result = draft.Pricing.Taxes
	}
	for _, candidate := range []struct {
		name  string
		value string
		out   **pricingPreviewTax
	}{
		{"GST", "", &result.GST},
		{"PCT", "", &result.PCT},
		{"advance tax", "", &result.AdvanceTax},
	} {
		for _, line := range draft.Lines {
			switch candidate.name {
			case "GST":
				candidate.value = line.GSTRate
			case "PCT":
				candidate.value = line.PCTRate
			default:
				candidate.value = line.AdvanceTaxRate
			}
			if strings.TrimSpace(candidate.value) == "" {
				continue
			}
			parsed, err := parsePercent(candidate.value)
			if err != nil {
				return result, fmt.Errorf("%s rate: %w", candidate.name, err)
			}
			if *candidate.out != nil && strings.TrimSpace((*candidate.out).Rate) != "" {
				existing, parseErr := parsePercent((*candidate.out).Rate)
				if parseErr != nil || existing != parsed {
					return result, fmt.Errorf("conflicting explicit %s rates were supplied", candidate.name)
				}
			} else {
				*candidate.out = &pricingPreviewTax{Rate: formatPercent(parsed), Inclusive: false}
			}
		}
		if *candidate.out != nil && strings.TrimSpace((*candidate.out).Rate) != "" {
			parsed, err := parsePercent((*candidate.out).Rate)
			if err != nil {
				return result, fmt.Errorf("%s rate: %w", candidate.name, err)
			}
			(*candidate.out).Rate = formatPercent(parsed)
		}
	}
	return result, nil
}

package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
)

type roleResponse struct {
	ID              string   `json:"id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	LegacyGroupCode string   `json:"legacyGroupCode,omitempty"`
	MemberCount     int      `json:"memberCount"`
	Permissions     []string `json:"permissions"`
}

type roleRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

var supportedRolePermissions = map[string]struct{}{
	"sales.read": {}, "sales.write": {}, "purchases.read": {}, "purchases.write": {},
	"reports.read": {}, "master.read": {}, "master.write": {}, "maintenance.write": {},
	"manage.users": {}, "manage.groups": {}, "sync.review": {}, "sync.push": {}, "sync.pull": {},
	"tenant.read": {}, "branch.read": {}, "preferences.read": {}, "preferences.write": {},
	"tax.read": {}, "tax.write": {}, "tax.override": {},
}

func normalizeRolePermissions(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		permission := strings.ToLower(strings.TrimSpace(raw))
		if permission == "" {
			continue
		}
		if _, ok := supportedRolePermissions[permission]; !ok {
			return nil, errors.New("unsupported permission " + permission)
		}
		seen[permission] = struct{}{}
	}
	permissions := make([]string, 0, len(seen))
	for permission := range seen {
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	return permissions, nil
}

func splitPermissionCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	permissions, err := normalizeRolePermissions(parts)
	if err != nil {
		return []string{}
	}
	return permissions
}

type operatorCreateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Active      *bool  `json:"active"`
	RoleCode    string `json:"roleCode"`
	BranchID    string `json:"branchId"`
	CounterID   string `json:"counterId"`
}

type operatorUpdateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Active      *bool  `json:"active"`
	RoleCode    string `json:"roleCode"`
	BranchID    string `json:"branchId"`
	CounterID   string `json:"counterId"`
}

func (s *Server) roles(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.groups") {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The role store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `
		SELECT r.id::text, r.code, r.name, COALESCE(lg.legacy_group_code, ''),
			count(DISTINCT m.user_id)::int,
			COALESCE((
				SELECT string_agg(effective.permission, ',' ORDER BY effective.permission)
				FROM (
					SELECT p.permission
					FROM role_permissions p
					WHERE p.tenant_id = r.tenant_id AND p.role_id = r.id AND p.allowed
					  AND NOT EXISTS (
						SELECT 1 FROM group_rights denied
						WHERE denied.tenant_id = p.tenant_id AND denied.role_id = p.role_id
						  AND lower(COALESCE(denied.permission, denied.right_code)) = lower(p.permission) AND NOT denied.allowed
					  )
					UNION
					SELECT lower(COALESCE(g.permission, g.right_code))
					FROM group_rights g
					WHERE g.tenant_id = r.tenant_id AND g.role_id = r.id
					  AND g.allowed AND COALESCE(g.permission, g.right_code) <> ''
				) effective
			), '')
		FROM roles r
		LEFT JOIN user_memberships m ON m.role_id = r.id AND m.tenant_id = r.tenant_id
		LEFT JOIN legacy_groups lg ON lg.role_id = r.id AND lg.tenant_id = r.tenant_id
		WHERE r.tenant_id = $1::uuid
		GROUP BY r.id, r.code, r.name, lg.legacy_group_code
		ORDER BY r.code`, operator.TenantID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "role_read_failed", "Unable to read groups", "The role store could not be queried.")
		return
	}
	roles := make([]roleResponse, 0)
	for rows.Next() {
		var role roleResponse
		var permissionCSV string
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.LegacyGroupCode, &role.MemberCount, &permissionCSV); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "role_read_failed", "Unable to read groups", "The role response could not be decoded.")
			return
		}
		role.Permissions = splitPermissionCSV(permissionCSV)
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "role_read_failed", "Unable to read groups", "The role response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "role_read_failed", "Unable to read groups", "The role transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.groups") {
		return
	}
	var request roleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The group could not be parsed.")
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	if request.Code == "" || request.Name == "" || len(request.Code) > 80 || len(request.Name) > 160 {
		writeProblem(w, http.StatusBadRequest, "invalid_role", "Invalid group", "A group code and name are required.")
		return
	}
	permissions, err := normalizeRolePermissions(request.Permissions)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_role_permissions", "Invalid group permissions", err.Error())
		return
	}
	request.Permissions = permissions
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The role store could not be opened.")
		return
	}
	defer tx.Rollback()
	var role roleResponse
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (tenant_id, code, name)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text, code, name`, operator.TenantID, request.Code, request.Name).Scan(&role.ID, &role.Code, &role.Name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeProblem(w, http.StatusConflict, "role_exists", "Group already exists", "Choose a different group code.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "role_create_failed", "Group could not be created", "The role store rejected the new group.")
		return
	}
	role.Permissions = permissions
	if err := replaceRolePermissions(r.Context(), tx, operator.TenantID, role.ID, permissions); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "role_permissions_failed", "Group permissions could not be saved", "The role permissions could not be stored.")
		return
	}
	if err := appendRoleAudit(r, tx, operator, "role.created", role.ID, request); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record group", "The group audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "role_commit_failed", "Group could not be created", "The group transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.groups") {
		return
	}
	roleID := strings.TrimSpace(r.PathValue("id"))
	if roleID == "" {
		writeProblem(w, http.StatusBadRequest, "role_required", "Group required", "Provide a group identifier.")
		return
	}
	var request roleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The group could not be parsed.")
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	if request.Code == "" || request.Name == "" || len(request.Code) > 80 || len(request.Name) > 160 {
		writeProblem(w, http.StatusBadRequest, "invalid_role", "Invalid group", "A group code and name are required.")
		return
	}
	permissions, err := normalizeRolePermissions(request.Permissions)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_role_permissions", "Invalid group permissions", err.Error())
		return
	}
	request.Permissions = permissions
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The role store could not be opened.")
		return
	}
	defer tx.Rollback()
	var role roleResponse
	err = tx.QueryRowContext(r.Context(), `
		UPDATE roles SET code = $1, name = $2
		WHERE id = $3::uuid AND tenant_id = $4::uuid
		RETURNING id::text, code, name`, request.Code, request.Name, roleID, operator.TenantID).Scan(&role.ID, &role.Code, &role.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "role_not_found", "Group not found", "The group is outside the authenticated tenant scope.")
			return
		}
		writeProblem(w, http.StatusConflict, "role_update_failed", "Group could not be updated", "The group code may already be in use.")
		return
	}
	role.Permissions = permissions
	if err := replaceRolePermissions(r.Context(), tx, operator.TenantID, role.ID, permissions); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "role_permissions_failed", "Group permissions could not be saved", "The role permissions could not be stored.")
		return
	}
	if err := appendRoleAudit(r, tx, operator, "role.updated", role.ID, request); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record group", "The group audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "role_commit_failed", "Group could not be updated", "The group transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func replaceRolePermissions(ctx context.Context, tx *sql.Tx, tenantID, roleID string, permissions []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE tenant_id = $1::uuid AND role_id = $2::uuid`, tenantID, roleID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO role_permissions (tenant_id, role_id, permission, allowed)
			VALUES ($1::uuid, $2::uuid, $3, true)`, tenantID, roleID, permission); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createOperator(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.users") {
		return
	}
	var request operatorCreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The operator could not be parsed.")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.RoleCode = strings.TrimSpace(request.RoleCode)
	request.BranchID = strings.TrimSpace(request.BranchID)
	request.CounterID = strings.TrimSpace(request.CounterID)
	if request.Username == "" || request.DisplayName == "" || len(request.Username) > 80 || len(request.DisplayName) > 160 {
		writeProblem(w, http.StatusBadRequest, "invalid_operator", "Invalid operator", "A username and display name are required.")
		return
	}
	if len(request.Password) < 8 || len(request.Password) > 256 {
		writeProblem(w, http.StatusBadRequest, "invalid_password", "Invalid password", "The operator password must contain at least 8 characters.")
		return
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	roleCode := request.RoleCode
	if roleCode == "" || strings.EqualFold(roleCode, "admin") {
		roleCode = "tenant_admin"
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The operator store could not be opened.")
		return
	}
	defer tx.Rollback()
	var created operatorResponse
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO users (tenant_id, username, display_name, password_hash, active)
		VALUES ($1::uuid, $2, $3, crypt($4, gen_salt('bf')), $5)
		RETURNING id::text, username, display_name, active`, operator.TenantID, request.Username, request.DisplayName, request.Password, active).Scan(&created.ID, &created.Username, &created.DisplayName, &created.Active)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeProblem(w, http.StatusConflict, "operator_exists", "Operator already exists", "Choose a different username.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "operator_create_failed", "Operator could not be created", "The operator store rejected the new user.")
		return
	}
	var roleID string
	if err := tx.QueryRowContext(r.Context(), `SELECT id::text FROM roles WHERE tenant_id = $1::uuid AND lower(code) = lower($2)`, operator.TenantID, roleCode).Scan(&roleID); err != nil {
		writeProblem(w, http.StatusBadRequest, "role_not_found", "Group not found", "Create the selected group before adding an operator.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_memberships (user_id, tenant_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, created.ID, operator.TenantID, roleID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "operator_membership_failed", "Operator could not be assigned", "The operator group assignment could not be stored.")
		return
	}
	branchID := request.BranchID
	if branchID == "" {
		if err := tx.QueryRowContext(r.Context(), `SELECT id::text FROM branches WHERE tenant_id = $1::uuid AND active ORDER BY code LIMIT 1`, operator.TenantID).Scan(&branchID); err != nil {
			writeProblem(w, http.StatusBadRequest, "branch_not_found", "Branch not found", "Create an active branch before adding an operator.")
			return
		}
	}
	var branchActive bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid AND active)`, branchID, operator.TenantID).Scan(&branchActive); err != nil || !branchActive {
		writeProblem(w, http.StatusBadRequest, "branch_not_allowed", "Branch not allowed", "Select an active branch in the current tenant.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_branch_assignments (user_id, tenant_id, branch_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, created.ID, operator.TenantID, branchID); err != nil {
		writeProblem(w, http.StatusBadRequest, "branch_not_allowed", "Branch not allowed", "The operator could not be assigned to the selected branch.")
		return
	}
	counterID := request.CounterID
	if counterID == "" {
		if err := tx.QueryRowContext(r.Context(), `SELECT id::text FROM counters WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND active ORDER BY code LIMIT 1`, operator.TenantID, branchID).Scan(&counterID); err != nil {
			writeProblem(w, http.StatusBadRequest, "counter_not_found", "Counter not found", "Create an active counter before adding an operator.")
			return
		}
	}
	var counterActive bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM counters WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND active)`, counterID, operator.TenantID, branchID).Scan(&counterActive); err != nil || !counterActive {
		writeProblem(w, http.StatusBadRequest, "counter_not_allowed", "Counter not allowed", "Select an active counter assigned to the selected branch.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_counter_assignments (user_id, tenant_id, counter_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, created.ID, operator.TenantID, counterID); err != nil {
		writeProblem(w, http.StatusBadRequest, "counter_not_allowed", "Counter not allowed", "The operator could not be assigned to the selected counter.")
		return
	}
	if err := appendOperatorAudit(r, tx, operator, "operator.created", created.ID, request); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record operator", "The operator audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "operator_commit_failed", "Operator could not be created", "The operator transaction could not be committed.")
		return
	}
	created.Roles = []string{roleCode}
	created.BranchID = branchID
	created.CounterID = counterID
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateOperator(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.users") {
		return
	}
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeProblem(w, http.StatusBadRequest, "operator_required", "Operator required", "Provide an operator identifier.")
		return
	}
	var request operatorUpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The operator could not be parsed.")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.Password = strings.TrimSpace(request.Password)
	request.RoleCode = strings.TrimSpace(request.RoleCode)
	request.BranchID = strings.TrimSpace(request.BranchID)
	request.CounterID = strings.TrimSpace(request.CounterID)
	if request.Username == "" || request.DisplayName == "" || len(request.Username) > 80 || len(request.DisplayName) > 160 {
		writeProblem(w, http.StatusBadRequest, "invalid_operator", "Invalid operator", "A username and display name are required.")
		return
	}
	if request.Password != "" && (len(request.Password) < 8 || len(request.Password) > 256) {
		writeProblem(w, http.StatusBadRequest, "invalid_password", "Invalid password", "The operator password must contain at least 8 characters when supplied.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The operator store could not be opened.")
		return
	}
	defer tx.Rollback()
	var currentActive bool
	var currentRole string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT u.active, COALESCE((SELECT r.code FROM user_memberships m JOIN roles r ON r.id = m.role_id AND r.tenant_id = m.tenant_id WHERE m.user_id = u.id AND m.tenant_id = u.tenant_id ORDER BY r.code LIMIT 1), '')
		FROM users u WHERE u.id = $1::uuid AND u.tenant_id = $2::uuid`, userID, operator.TenantID).Scan(&currentActive, &currentRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "operator_not_found", "Operator not found", "The operator is outside the authenticated tenant scope.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "operator_read_failed", "Operator could not be loaded", "The operator store could not be queried.")
		return
	}
	active := currentActive
	if request.Active != nil {
		active = *request.Active
	}
	roleCode := request.RoleCode
	if roleCode == "" {
		roleCode = currentRole
	}
	if roleCode == "" || strings.EqualFold(roleCode, "admin") {
		roleCode = "tenant_admin"
	}
	var updated operatorResponse
	if err := tx.QueryRowContext(r.Context(), `
		UPDATE users
		SET username = $1, display_name = $2, active = $3,
		    password_hash = CASE WHEN $4 = '' THEN password_hash ELSE crypt($4, gen_salt('bf')) END,
		    updated_at = now()
		WHERE id = $5::uuid AND tenant_id = $6::uuid
		RETURNING id::text, username, display_name, active`, request.Username, request.DisplayName, active, request.Password, userID, operator.TenantID).
		Scan(&updated.ID, &updated.Username, &updated.DisplayName, &updated.Active); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeProblem(w, http.StatusConflict, "operator_exists", "Operator already exists", "Choose a different username.")
			return
		}
		writeProblem(w, http.StatusUnprocessableEntity, "operator_update_failed", "Operator could not be updated", "The operator store rejected the update.")
		return
	}
	var roleID string
	if err := tx.QueryRowContext(r.Context(), `SELECT id::text FROM roles WHERE tenant_id = $1::uuid AND lower(code) = lower($2)`, operator.TenantID, roleCode).Scan(&roleID); err != nil {
		writeProblem(w, http.StatusBadRequest, "role_not_found", "Group not found", "Create the selected group before updating the operator.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM user_memberships WHERE tenant_id = $1::uuid AND user_id = $2::uuid`, operator.TenantID, updated.ID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "operator_membership_failed", "Operator group could not be updated", "The operator group assignment could not be replaced.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_memberships (user_id, tenant_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, updated.ID, operator.TenantID, roleID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "operator_membership_failed", "Operator group could not be updated", "The operator group assignment could not be stored.")
		return
	}
	branchID := request.BranchID
	if branchID == "" {
		_ = tx.QueryRowContext(r.Context(), `SELECT branch_id::text FROM user_branch_assignments WHERE tenant_id = $1::uuid AND user_id = $2::uuid ORDER BY branch_id LIMIT 1`, operator.TenantID, updated.ID).Scan(&branchID)
	}
	if branchID == "" {
		_ = tx.QueryRowContext(r.Context(), `SELECT id::text FROM branches WHERE tenant_id = $1::uuid AND active ORDER BY code LIMIT 1`, operator.TenantID).Scan(&branchID)
	}
	var branchActive bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid AND active)`, branchID, operator.TenantID).Scan(&branchActive); err != nil || !branchActive {
		writeProblem(w, http.StatusBadRequest, "branch_not_allowed", "Branch not allowed", "Select an active branch in the current tenant.")
		return
	}
	counterID := request.CounterID
	if counterID == "" {
		_ = tx.QueryRowContext(r.Context(), `SELECT counter_id::text FROM user_counter_assignments WHERE tenant_id = $1::uuid AND user_id = $2::uuid ORDER BY counter_id LIMIT 1`, operator.TenantID, updated.ID).Scan(&counterID)
	}
	if counterID == "" {
		_ = tx.QueryRowContext(r.Context(), `SELECT id::text FROM counters WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND active ORDER BY code LIMIT 1`, operator.TenantID, branchID).Scan(&counterID)
	}
	var counterActive bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM counters WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND active)`, counterID, operator.TenantID, branchID).Scan(&counterActive); err != nil || !counterActive {
		writeProblem(w, http.StatusBadRequest, "counter_not_allowed", "Counter not allowed", "Select an active counter assigned to the selected branch.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM user_branch_assignments WHERE tenant_id = $1::uuid AND user_id = $2::uuid`, operator.TenantID, updated.ID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "branch_not_allowed", "Branch assignment failed", "The operator branch assignment could not be replaced.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_branch_assignments (user_id, tenant_id, branch_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, updated.ID, operator.TenantID, branchID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "branch_not_allowed", "Branch assignment failed", "The operator branch assignment could not be stored.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM user_counter_assignments WHERE tenant_id = $1::uuid AND user_id = $2::uuid`, operator.TenantID, updated.ID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "counter_not_allowed", "Counter assignment failed", "The operator counter assignment could not be replaced.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_counter_assignments (user_id, tenant_id, counter_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, updated.ID, operator.TenantID, counterID); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "counter_not_allowed", "Counter assignment failed", "The operator counter assignment could not be stored.")
		return
	}
	request.BranchID = branchID
	request.CounterID = counterID
	updated.BranchID = branchID
	updated.CounterID = counterID
	if err := appendOperatorUpdateAudit(r, tx, operator, updated.ID, request, roleCode); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit_write_failed", "Unable to record operator", "The operator audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "operator_commit_failed", "Operator could not be updated", "The operator transaction could not be committed.")
		return
	}
	updated.Roles = []string{roleCode}
	writeJSON(w, http.StatusOK, updated)
}

func appendRoleAudit(r *http.Request, tx *sql.Tx, operator *sessionContext, action, roleID string, request roleRequest) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, $4, 'role', $5::uuid, $6::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, action, roleID, payload)
	return err
}

func appendOperatorAudit(r *http.Request, tx *sql.Tx, operator *sessionContext, action, userID string, request operatorCreateRequest) error {
	payload, err := json.Marshal(map[string]any{
		"username":    request.Username,
		"displayName": request.DisplayName,
		"active":      request.Active,
		"roleCode":    request.RoleCode,
		"branchId":    request.BranchID,
		"counterId":   request.CounterID,
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, $4, 'user', $5::uuid, $6::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, action, userID, payload)
	return err
}

func appendOperatorUpdateAudit(r *http.Request, tx *sql.Tx, operator *sessionContext, userID string, request operatorUpdateRequest, roleCode string) error {
	payload, err := json.Marshal(map[string]any{
		"username": request.Username, "displayName": request.DisplayName, "active": request.Active, "roleCode": roleCode, "branchId": request.BranchID, "counterId": request.CounterID, "passwordChanged": request.Password != "",
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, 'operator.updated', 'user', $4::uuid, $5::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, userID, payload)
	return err
}

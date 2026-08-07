package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "abuzar_session"

var errAuthenticationRequired = errors.New("authentication required")

type contextKey string

const sessionContextKey contextKey = "abuzar.session"

type sessionContext struct {
	TokenHash    string                     `json:"-"`
	UserID       string                     `json:"operatorId"`
	Username     string                     `json:"username"`
	DisplayName  string                     `json:"displayName"`
	TenantID     string                     `json:"tenantId"`
	TenantCode   string                     `json:"tenantCode"`
	BranchID     string                     `json:"branchId,omitempty"`
	CounterID    string                     `json:"counterId,omitempty"`
	Roles        []string                   `json:"roles"`
	Permissions  []string                   `json:"permissions"`
	Scopes       map[string]map[string]bool `json:"scopes,omitempty"`
	LegacyRights []legacyRight              `json:"-"`
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	TenantCode string `json:"tenantCode"`
	BranchID   string `json:"branchId,omitempty"`
	CounterID  string `json:"counterId,omitempty"`
}

type changeUserRequest struct {
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	TenantCode string `json:"tenantCode,omitempty"`
	BranchID   string `json:"branchId,omitempty"`
	CounterID  string `json:"counterId,omitempty"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type sessionContextRequest struct {
	BranchID  string `json:"branchId"`
	CounterID string `json:"counterId"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_not_configured", "Database is not configured", "Configure DATABASE_URL before signing in.")
		return
	}
	var request loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The login request body could not be parsed.")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.TenantCode = strings.TrimSpace(request.TenantCode)
	if request.Username == "" || request.Password == "" || request.TenantCode == "" {
		writeProblem(w, http.StatusBadRequest, "missing_login_fields", "Missing login fields", "Username, password, and tenantCode are required.")
		return
	}

	operator, err := s.authenticate(r.Context(), request)
	if err != nil {
		if errors.Is(err, errAuthenticationRequired) {
			writeProblem(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials", "The username, password, tenant, or assignment is not valid.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "authentication_unavailable", "Authentication unavailable", "The identity store could not be reached.")
		return
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "session_creation_failed", "Unable to create session", "Secure session creation failed.")
		return
	}
	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	if _, err := s.database.ExecContext(r.Context(), `
		INSERT INTO sessions (token_hash, user_id, tenant_id, branch_id, counter_id, expires_at)
		VALUES ($1, $2::uuid, $3::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6)
	`, tokenHash, operator.UserID, operator.TenantID, operator.BranchID, operator.CounterID, expiresAt); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "session_creation_failed", "Unable to create session", "The identity store rejected the new session.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(s.sessionTTL / time.Second),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "context": operator, "expiresAt": expiresAt.Format(time.RFC3339)})
}

func (s *Server) authenticate(ctx context.Context, request loginRequest) (*sessionContext, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.authenticating', 'true', true)`); err != nil {
		return nil, err
	}
	var operator sessionContext
	if err := tx.QueryRowContext(ctx, `
		SELECT u.id::text, u.username, u.display_name, u.tenant_id::text, t.code
		FROM users u
		JOIN tenants t ON t.id = u.tenant_id
		WHERE u.username = $1 AND t.code = $3 AND u.active AND t.active
		  AND crypt($2, u.password_hash) = u.password_hash
	`, request.Username, request.Password, request.TenantCode).Scan(&operator.UserID, &operator.Username, &operator.DisplayName, &operator.TenantID, &operator.TenantCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errAuthenticationRequired
		}
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT r.code
		FROM user_memberships m
		JOIN roles r ON r.id = m.role_id AND r.tenant_id = m.tenant_id
		WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
		ORDER BY r.code
	`, operator.UserID, operator.TenantID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		operator.Roles = append(operator.Roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := loadOperatorAccess(ctx, tx, &operator); err != nil {
		return nil, err
	}

	admin := hasTenantAdminRole(operator.Roles)
	if request.BranchID != "" {
		allowed, err := s.assignmentAllowedOn(ctx, tx, operator, request.BranchID, "branch", admin)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, errAuthenticationRequired
		}
		operator.BranchID = request.BranchID
	}
	if request.CounterID != "" {
		if operator.BranchID == "" {
			return nil, errAuthenticationRequired
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM counters c
				WHERE c.id = $1::uuid AND c.tenant_id = $2::uuid AND c.branch_id = $3::uuid AND c.active
			)
		`, request.CounterID, operator.TenantID, operator.BranchID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, errAuthenticationRequired
		}
		if !admin {
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM user_counter_assignments a
					WHERE a.user_id = $1::uuid AND a.tenant_id = $2::uuid AND a.counter_id = $3::uuid
				)
			`, operator.UserID, operator.TenantID, request.CounterID).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				return nil, errAuthenticationRequired
			}
		}
		operator.CounterID = request.CounterID
	}
	// A single-branch/single-counter tenant should be immediately usable after
	// the legacy-style login. We only infer a context when it is unambiguous;
	// multi-branch tenants must continue to select their operational context.
	if operator.BranchID == "" {
		branchID, err := soleOperationalID(ctx, tx, `SELECT id::text FROM branches WHERE tenant_id = $1::uuid AND active ORDER BY code LIMIT 2`, operator.TenantID)
		if err != nil {
			return nil, err
		}
		operator.BranchID = branchID
	}
	if operator.BranchID != "" && operator.CounterID == "" {
		counterID, err := soleOperationalID(ctx, tx, `SELECT id::text FROM counters WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND active ORDER BY code LIMIT 2`, operator.TenantID, operator.BranchID)
		if err != nil {
			return nil, err
		}
		operator.CounterID = counterID
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &operator, nil
}

func soleOperationalID(ctx context.Context, database queryerRows, query string, args ...any) (string, error) {
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	return "", nil
}

func (s *Server) assignmentAllowed(ctx context.Context, operator sessionContext, id, kind string, admin bool) (bool, error) {
	return s.assignmentAllowedOn(ctx, s.database, operator, id, kind, admin)
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type queryerRows interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Server) assignmentAllowedOn(ctx context.Context, database queryer, operator sessionContext, id, kind string, admin bool) (bool, error) {
	if admin {
		var exists bool
		query := `SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid AND active)`
		if kind == "counter" {
			query = `SELECT EXISTS (SELECT 1 FROM counters WHERE id = $1::uuid AND tenant_id = $2::uuid AND active)`
		}
		if err := database.QueryRowContext(ctx, query, id, operator.TenantID).Scan(&exists); err != nil {
			return false, err
		}
		return exists, nil
	}
	var exists bool
	if kind == "branch" {
		err := database.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_branch_assignments a
			JOIN branches b ON b.id = a.branch_id AND b.tenant_id = a.tenant_id
			WHERE a.user_id = $1::uuid AND a.tenant_id = $2::uuid AND a.branch_id = $3::uuid AND b.active
		)
	`, operator.UserID, operator.TenantID, id).Scan(&exists)
		return exists, err
	}
	return false, nil
}

func hasTenantAdminRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(role) {
		case "owner", "tenant_admin", "admin", "administrator":
			return true
		}
	}
	return false
}

func hasPermission(operator *sessionContext, permission string) bool {
	if operator == nil {
		return false
	}
	if hasTenantAdminRole(operator.Roles) {
		return true
	}
	permission = strings.ToLower(strings.TrimSpace(permission))
	for _, candidate := range operator.Permissions {
		if strings.EqualFold(candidate, permission) {
			return true
		}
	}
	return false
}

func (s *Server) requirePermission(r *http.Request, w http.ResponseWriter, operator *sessionContext, permission string) bool {
	if hasPermission(operator, permission) {
		return true
	}
	s.auditAuthorizationDenied(r, operator, permission)
	writeProblem(w, http.StatusForbidden, "permission_required", "Permission required", "The authenticated operator does not have permission to perform this action.")
	return false
}

func loadOperatorAccess(ctx context.Context, database queryerRows, operator *sessionContext) error {
	operator.Permissions = make([]string, 0)
	const effectivePermissions = `
		SELECT DISTINCT effective.permission
		FROM (
			SELECT p.permission
			FROM user_memberships m
			JOIN role_permissions p
			  ON p.role_id = m.role_id AND p.tenant_id = m.tenant_id AND p.allowed
			WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
			  AND NOT EXISTS (
				SELECT 1
				FROM user_memberships denied_membership
				JOIN group_rights denied
				  ON denied.tenant_id = denied_membership.tenant_id
				 AND denied.role_id = denied_membership.role_id
				WHERE denied_membership.user_id = m.user_id
				  AND denied_membership.tenant_id = m.tenant_id
				  AND lower(COALESCE(denied.permission, denied.right_code)) = lower(p.permission)
				  AND NOT denied.allowed
			  )
			UNION ALL
			SELECT lower(COALESCE(g.permission, g.right_code))
			FROM user_memberships m
			JOIN group_rights g
			  ON g.role_id = m.role_id AND g.tenant_id = m.tenant_id
			WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
			  AND g.allowed AND COALESCE(g.permission, g.right_code) <> ''
			  AND NOT EXISTS (
				SELECT 1
				FROM user_memberships denied_membership
				JOIN group_rights denied
				  ON denied.tenant_id = denied_membership.tenant_id
				 AND denied.role_id = denied_membership.role_id
				WHERE denied_membership.user_id = m.user_id
				  AND denied_membership.tenant_id = m.tenant_id
				  AND lower(COALESCE(denied.permission, denied.right_code)) =
				      lower(COALESCE(g.permission, g.right_code))
				  AND NOT denied.allowed
			  )
		) effective
		ORDER BY effective.permission`
	rows, err := database.QueryContext(ctx, effectivePermissions, operator.UserID, operator.TenantID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			rows.Close()
			return err
		}
		operator.Permissions = append(operator.Permissions, permission)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	operator.Scopes = make(map[string]map[string]bool)
	scopeRows, err := database.QueryContext(ctx, `
		SELECT g.scope_kind, g.scope_key, bool_and(g.allowed)
		FROM user_memberships m
		JOIN group_allowed_scopes g
		  ON g.role_id = m.role_id AND g.tenant_id = m.tenant_id
		WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
		GROUP BY g.scope_kind, g.scope_key
		ORDER BY g.scope_kind, g.scope_key
	`, operator.UserID, operator.TenantID)
	if err != nil {
		return err
	}
	for scopeRows.Next() {
		var kind, key string
		var allowed bool
		if err := scopeRows.Scan(&kind, &key, &allowed); err != nil {
			scopeRows.Close()
			return err
		}
		if operator.Scopes[kind] == nil {
			operator.Scopes[kind] = make(map[string]bool)
		}
		operator.Scopes[kind][key] = allowed
	}
	if err := scopeRows.Err(); err != nil {
		scopeRows.Close()
		return err
	}
	scopeRows.Close()
	rightRows, err := database.QueryContext(ctx, `
		SELECT g.right_code, COALESCE(g.permission, ''), g.allowed,
		       COALESCE(g.legacy_status, '')
		FROM user_memberships m
		JOIN group_rights g
		  ON g.role_id = m.role_id AND g.tenant_id = m.tenant_id
		WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
		ORDER BY lower(g.right_code)
	`, operator.UserID, operator.TenantID)
	if err != nil {
		return err
	}
	imported := make([]legacyRight, 0)
	for rightRows.Next() {
		var right legacyRight
		if err := rightRows.Scan(&right.RightCode, &right.Permission, &right.Allowed, &right.LegacyStatus); err != nil {
			rightRows.Close()
			return err
		}
		imported = append(imported, right)
	}
	if err := rightRows.Err(); err != nil {
		rightRows.Close()
		return err
	}
	rightRows.Close()
	operator.LegacyRights = resolveLegacyRights(imported)
	return nil
}

func loadLegacyScopeRows(ctx context.Context, database queryerRows, userID, tenantID string) ([]legacyScope, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT g.scope_kind, g.scope_key, COALESCE(g.scope_label, ''),
		       bool_and(g.allowed), COALESCE(g.legacy_table, '')
		FROM user_memberships m
		JOIN group_allowed_scopes g
		  ON g.role_id = m.role_id AND g.tenant_id = m.tenant_id
		WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
		GROUP BY g.scope_kind, g.scope_key, g.scope_label, g.legacy_table
		ORDER BY g.scope_kind, g.scope_key
	`, userID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]legacyScope, 0)
	for rows.Next() {
		var scope legacyScope
		if err := rows.Scan(&scope.ScopeKind, &scope.ScopeKey, &scope.ScopeLabel, &scope.Allowed, &scope.LegacyTable); err != nil {
			return nil, err
		}
		result = append(result, scope)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resolveLegacyScopes(result), nil
}

func scopeAllowed(operator *sessionContext, kind, key string) bool {
	if operator == nil {
		return false
	}
	if hasTenantAdminRole(operator.Roles) {
		return true
	}
	entries, scoped := operator.Scopes[strings.ToLower(strings.TrimSpace(kind))]
	if !scoped || len(entries) == 0 {
		// No imported allow-list for this scope preserves the existing
		// role-permission behavior until that legacy table is migrated.
		return true
	}
	key = strings.TrimSpace(key)
	return entries["*"] || entries[key]
}

// canonicalGodownScopeAllowed only applies an imported godown allow-list when
// its keys are already canonical UUIDs (or an explicit wildcard). The captured
// GroupAllowedGodown source rows currently use composite legacy keys such as
// GroupCode/GCode/Module/Priority; treating those as canonical godown IDs would
// be an unverified mapping and could deny valid operators.
func canonicalGodownScopeAllowed(operator *sessionContext, key string) bool {
	if operator == nil || hasTenantAdminRole(operator.Roles) {
		return true
	}
	entries, scoped := operator.Scopes["godown"]
	if !scoped || len(entries) == 0 {
		return true
	}
	canonicalEntries := false
	for entry := range entries {
		if entry == "*" || documentUUIDPattern.MatchString(strings.TrimSpace(entry)) {
			canonicalEntries = true
			break
		}
	}
	if !canonicalEntries {
		return true
	}
	key = strings.TrimSpace(key)
	return entries["*"] || entries[key]
}

func scopeAllowedAtEnforcementBoundary(operator *sessionContext, kind, key string) bool {
	if strings.EqualFold(strings.TrimSpace(kind), "godown") {
		return canonicalGodownScopeAllowed(operator, key)
	}
	return scopeAllowed(operator, kind, key)
}

func (s *Server) requireScope(r *http.Request, w http.ResponseWriter, operator *sessionContext, kind, key string) bool {
	if scopeAllowedAtEnforcementBoundary(operator, kind, key) {
		return true
	}
	s.auditAuthorizationDenied(r, operator, kind+":"+key)
	writeProblem(w, http.StatusForbidden, "scope_not_allowed", "Scope not allowed", "The authenticated operator is not allowed to access the requested scope.")
	return false
}

func (s *Server) auditAuthorizationDenied(r *http.Request, operator *sessionContext, permission string) {
	if s.database == nil || operator == nil || operator.TenantID == "" {
		return
	}
	tx, err := s.database.BeginTx(r.Context(), nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	set := func(key, value string) error {
		_, setErr := tx.ExecContext(r.Context(), `SELECT set_config($1, $2, true)`, key, value)
		return setErr
	}
	if set("app.authenticating", "false") != nil ||
		set("app.tenant_id", operator.TenantID) != nil ||
		set("app.branch_id", operator.BranchID) != nil {
		return
	}
	allowTenant := "false"
	if hasTenantAdminRole(operator.Roles) {
		allowTenant = "true"
	}
	if set("app.allow_tenant_scope", allowTenant) != nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"permission": permission,
		"method":     r.Method,
		"path":       r.URL.Path,
		"requestId":  r.Header.Get("X-Request-ID"),
	})
	if err != nil {
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, 'authorization.denied', 'permission', $4::jsonb)
	`, operator.TenantID, operator.BranchID, operator.UserID, payload); err != nil {
		return
	}
	_ = tx.Commit()
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	operator, err := s.loadSession(r.Context(), r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "context": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "context": operator})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" && s.database != nil {
		_, _ = s.database.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, hashSessionToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changeUser(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_not_configured", "Database is not configured", "Configure DATABASE_URL before changing user.")
		return
	}

	var currentTenantCode string
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		var tenantCode string
		_ = s.database.QueryRowContext(r.Context(), `
			SELECT t.code FROM sessions se JOIN tenants t ON t.id = se.tenant_id WHERE se.token_hash = $1
		`, hashSessionToken(cookie.Value)).Scan(&tenantCode)
		if tenantCode != "" {
			currentTenantCode = tenantCode
		}
		_, _ = s.database.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, hashSessionToken(cookie.Value))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	var req changeUserRequest
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req)
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.TenantCode = strings.TrimSpace(req.TenantCode)
	if req.TenantCode == "" {
		req.TenantCode = currentTenantCode
	}
	if req.TenantCode == "" {
		req.TenantCode = "demo"
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "changeUser": true})
		return
	}

	operator, err := s.authenticate(r.Context(), loginRequest{
		Username:   req.Username,
		Password:   req.Password,
		TenantCode: req.TenantCode,
		BranchID:   req.BranchID,
		CounterID:  req.CounterID,
	})
	if err != nil {
		if errors.Is(err, errAuthenticationRequired) {
			writeProblem(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials", "The username, password, tenant, or assignment is not valid.")
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "authentication_unavailable", "Authentication unavailable", "The identity store could not be reached.")
		return
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "session_creation_failed", "Unable to create session", "Secure session creation failed.")
		return
	}
	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	if _, err := s.database.ExecContext(r.Context(), `
		INSERT INTO sessions (token_hash, user_id, tenant_id, branch_id, counter_id, expires_at)
		VALUES ($1, $2::uuid, $3::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6)
	`, tokenHash, operator.UserID, operator.TenantID, operator.BranchID, operator.CounterID, expiresAt); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "session_creation_failed", "Unable to create session", "The identity store rejected the new session.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(s.sessionTTL / time.Second),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "context": operator, "expiresAt": expiresAt.Format(time.RFC3339)})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	var request changePasswordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The password request could not be parsed.")
		return
	}
	if request.CurrentPassword == "" || len(request.NewPassword) < 8 || request.NewPassword != request.ConfirmPassword {
		writeProblem(w, http.StatusBadRequest, "invalid_password_change", "Invalid password change", "Provide the current password, a matching new password of at least 8 characters, and confirmation.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The identity store could not be opened.")
		return
	}
	defer tx.Rollback()
	var valid bool
	if err := tx.QueryRowContext(r.Context(), `SELECT crypt($1, password_hash) = password_hash FROM users WHERE id = $2::uuid AND tenant_id = $3::uuid AND active`, request.CurrentPassword, operator.UserID, operator.TenantID).Scan(&valid); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "password_change_failed", "Password change failed", "The identity store could not be queried.")
		return
	}
	if !valid {
		writeProblem(w, http.StatusUnauthorized, "current_password_invalid", "Current password invalid", "The current password was not accepted.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE users SET password_hash = crypt($1, gen_salt('bf')), updated_at = now() WHERE id = $2::uuid AND tenant_id = $3::uuid`, request.NewPassword, operator.UserID, operator.TenantID); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "password_change_failed", "Password change failed", "The new password could not be stored.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, entity_id) VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, 'user.password_changed', 'user', $3::uuid)`, operator.TenantID, operator.BranchID, operator.UserID); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "password_change_failed", "Password change failed", "The password audit event could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "password_change_failed", "Password change failed", "The password change could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": true})
}

func (s *Server) setSessionContext(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "branch.read") {
		return
	}
	var request sessionContextRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The operational context could not be parsed.")
		return
	}
	request.BranchID = strings.TrimSpace(request.BranchID)
	request.CounterID = strings.TrimSpace(request.CounterID)
	if request.BranchID == "" || request.CounterID == "" {
		writeProblem(w, http.StatusBadRequest, "context_required", "Branch and counter required", "Select both a branch and a counter before continuing.")
		return
	}
	if !s.requireScope(r, w, operator, "branch", request.BranchID) {
		return
	}
	tx, err := s.database.BeginTx(r.Context(), nil)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The identity store could not be opened.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.authenticating', 'true', true)`); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "context_failed", "Context change failed", "The identity context could not be prepared.")
		return
	}
	admin := hasTenantAdminRole(operator.Roles)
	var branchExists bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid AND active)`, request.BranchID, operator.TenantID).Scan(&branchExists); err != nil || !branchExists {
		writeProblem(w, http.StatusForbidden, "branch_not_allowed", "Branch not allowed", "The selected branch is not active or not part of this tenant.")
		return
	}
	if !admin {
		var assigned bool
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM user_branch_assignments WHERE user_id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid)`, operator.UserID, operator.TenantID, request.BranchID).Scan(&assigned); err != nil || !assigned {
			writeProblem(w, http.StatusForbidden, "branch_not_allowed", "Branch not allowed", "The operator is not assigned to the selected branch.")
			return
		}
	}
	var counterExists bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM counters WHERE id = $1::uuid AND tenant_id = $2::uuid AND branch_id = $3::uuid AND active)`, request.CounterID, operator.TenantID, request.BranchID).Scan(&counterExists); err != nil || !counterExists {
		writeProblem(w, http.StatusForbidden, "counter_not_allowed", "Counter not allowed", "The selected counter is not active in the selected branch.")
		return
	}
	if !admin {
		var assigned bool
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM user_counter_assignments WHERE user_id = $1::uuid AND tenant_id = $2::uuid AND counter_id = $3::uuid)`, operator.UserID, operator.TenantID, request.CounterID).Scan(&assigned); err != nil || !assigned {
			writeProblem(w, http.StatusForbidden, "counter_not_allowed", "Counter not allowed", "The operator is not assigned to the selected counter.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE sessions SET branch_id = $1::uuid, counter_id = $2::uuid, last_seen_at = now() WHERE token_hash = $3`, request.BranchID, request.CounterID, operator.TokenHash); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "context_failed", "Context change failed", "The operational context could not be stored.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "context_failed", "Context change failed", "The operational context could not be committed.")
		return
	}
	operator.BranchID = request.BranchID
	operator.CounterID = request.CounterID
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "context": operator})
}

func (s *Server) loadSession(ctx context.Context, r *http.Request) (*sessionContext, error) {
	if s.database == nil {
		return nil, errAuthenticationRequired
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, errAuthenticationRequired
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.authenticating', 'true', true)`); err != nil {
		return nil, err
	}
	var operator sessionContext
	var branchID, counterID sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT u.id::text, u.username, u.display_name, se.tenant_id::text, t.code,
		       se.branch_id::text, se.counter_id::text
		FROM sessions se
		JOIN users u ON u.id = se.user_id AND u.tenant_id = se.tenant_id
		JOIN tenants t ON t.id = se.tenant_id
		WHERE se.token_hash = $1 AND se.expires_at > now() AND u.active AND t.active
	`, hashSessionToken(cookie.Value)).Scan(&operator.UserID, &operator.Username, &operator.DisplayName, &operator.TenantID, &operator.TenantCode, &branchID, &counterID); err != nil {
		return nil, errAuthenticationRequired
	}
	operator.BranchID = branchID.String
	operator.CounterID = counterID.String
	rows, err := tx.QueryContext(ctx, `
		SELECT r.code FROM user_memberships m JOIN roles r ON r.id = m.role_id AND r.tenant_id = m.tenant_id
		WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid ORDER BY r.code
	`, operator.UserID, operator.TenantID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		operator.Roles = append(operator.Roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := loadOperatorAccess(ctx, tx, &operator); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, operator.TenantID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.branch_id', $1, true)`, operator.BranchID); err != nil {
		return nil, err
	}
	allowTenant := "false"
	if hasTenantAdminRole(operator.Roles) {
		allowTenant = "true"
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.allow_tenant_scope', $1, true)`, allowTenant); err != nil {
		return nil, err
	}
	if err := s.validateLoadedScopeOn(ctx, tx, &operator); err != nil {
		return nil, err
	}
	_, _ = tx.ExecContext(ctx, `UPDATE sessions SET last_seen_at = now() WHERE token_hash = $1`, hashSessionToken(cookie.Value))
	operator.TokenHash = hashSessionToken(cookie.Value)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &operator, nil
}

func (s *Server) validateLoadedScope(ctx context.Context, operator *sessionContext) error {
	return s.validateLoadedScopeOn(ctx, s.database, operator)
}

func (s *Server) validateLoadedScopeOn(ctx context.Context, database queryer, operator *sessionContext) error {
	admin := hasTenantAdminRole(operator.Roles)
	if operator.BranchID != "" {
		allowed, err := s.assignmentAllowedOn(ctx, database, *operator, operator.BranchID, "branch", admin)
		if err != nil || !allowed {
			return errAuthenticationRequired
		}
	}
	if operator.CounterID != "" {
		var allowed bool
		if err := database.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM counters c
				WHERE c.id = $1::uuid AND c.tenant_id = $2::uuid AND c.branch_id = $3::uuid AND c.active
			)
		`, operator.CounterID, operator.TenantID, operator.BranchID).Scan(&allowed); err != nil || !allowed {
			return errAuthenticationRequired
		}
		if !admin {
			if err := database.QueryRowContext(ctx, `
				SELECT EXISTS (SELECT 1 FROM user_counter_assignments WHERE user_id = $1::uuid AND tenant_id = $2::uuid AND counter_id = $3::uuid)
			`, operator.UserID, operator.TenantID, operator.CounterID).Scan(&allowed); err != nil || !allowed {
				return errAuthenticationRequired
			}
		}
	}
	return nil
}

func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operator, err := s.loadSession(r.Context(), r)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "authentication_required", "Authentication required", "Sign in with a tenant-scoped operator account before accessing this resource.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, operator)))
	})
}

func currentSession(r *http.Request) *sessionContext {
	operator, _ := r.Context().Value(sessionContextKey).(*sessionContext)
	return operator
}

func newSessionToken() (string, string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw[:])
	return token, hashSessionToken(token), nil
}

func hashSessionToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

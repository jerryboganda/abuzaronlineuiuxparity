package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestImportedGroupScopeUpdateIsTenantScopedAndAudited(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	ctx := context.Background()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	suffix := formatTestSuffix(time.Now().UnixNano())
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "scope-"+suffix)
	defer database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
	operator := &sessionContext{
		UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID,
		TokenHash: "scope-session-" + suffix, Roles: []string{"tenant_admin"},
	}

	var roleID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO roles (tenant_id, code, name, legacy_group_id)
		VALUES ($1::uuid, $2, 'Scope Editor', $3)
		RETURNING id::text
	`, tenantID, "scope-editor-"+suffix, "9001-"+suffix).Scan(&roleID); err != nil {
		t.Fatalf("seed scope role: %v", err)
	}
	for _, scope := range []struct {
		kind, key, table string
	}{
		{kind: "price", key: "2:1:1", table: "GroupAllowedPrice"},
		{kind: "header", key: "2:10:1:1", table: "GroupAllowedHeader"},
		{kind: "godown", key: "2:10:1:1", table: "GroupAllowedGodown"},
	} {
		if _, err := database.ExecContext(ctx, `
			INSERT INTO group_allowed_scopes (tenant_id, role_id, scope_kind, scope_key, allowed, legacy_table, legacy_payload)
			VALUES ($1::uuid, $2::uuid, $3, $4, true, $5, '{}'::jsonb)
		`, tenantID, roleID, scope.kind, scope.key, scope.table); err != nil {
			t.Fatalf("seed %s scope: %v", scope.kind, err)
		}
	}

	server := &Server{database: database}
	readRequest := httptest.NewRequest(http.MethodGet, "/v1/roles/"+roleID+"/rights", nil)
	readRequest.SetPathValue("id", roleID)
	readRequest = readRequest.WithContext(context.WithValue(readRequest.Context(), sessionContextKey, operator))
	readRecorder := httptest.NewRecorder()
	server.roleRights(readRecorder, readRequest)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("role rights status = %d, body=%s", readRecorder.Code, readRecorder.Body.String())
	}
	var before roleRightsResponse
	if err := json.NewDecoder(readRecorder.Body).Decode(&before); err != nil {
		t.Fatalf("decode role rights: %v", err)
	}
	if len(before.Scopes) != 3 {
		t.Fatalf("role scope count = %d, want 3", len(before.Scopes))
	}

	updateRequest := httptest.NewRequest(http.MethodPatch, "/v1/roles/"+roleID+"/rights", strings.NewReader(`{
		"scopes":[{"scopeKind":"price","scopeKey":"2:1:1","allowed":false}]
	}`))
	updateRequest.SetPathValue("id", roleID)
	updateRequest = updateRequest.WithContext(context.WithValue(updateRequest.Context(), sessionContextKey, operator))
	updateRecorder := httptest.NewRecorder()
	server.updateRoleRights(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("role rights update status = %d, body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}

	var priceAllowed, headerAllowed, godownAllowed bool
	if err := database.QueryRowContext(ctx, `
		SELECT allowed FROM group_allowed_scopes
		WHERE tenant_id = $1::uuid AND role_id = $2::uuid AND scope_kind = 'price' AND scope_key = '2:1:1'
	`, tenantID, roleID).Scan(&priceAllowed); err != nil {
		t.Fatalf("read price scope: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT allowed FROM group_allowed_scopes
		WHERE tenant_id = $1::uuid AND role_id = $2::uuid AND scope_kind = $3 AND scope_key = $4
	`, tenantID, roleID, "header", "2:10:1:1").Scan(&headerAllowed); err != nil {
		t.Fatalf("read header scope: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT allowed FROM group_allowed_scopes
		WHERE tenant_id = $1::uuid AND role_id = $2::uuid AND scope_kind = $3 AND scope_key = $4
	`, tenantID, roleID, "godown", "2:10:1:1").Scan(&godownAllowed); err != nil {
		t.Fatalf("read godown scope: %v", err)
	}
	if priceAllowed || !headerAllowed || !godownAllowed {
		t.Fatalf("scope update leaked across kinds: price=%t header=%t godown=%t", priceAllowed, headerAllowed, godownAllowed)
	}

	var action, entityType string
	if err := database.QueryRowContext(ctx, `
		SELECT action, entity_type FROM audit_events
		WHERE tenant_id = $1::uuid AND entity_id = $2::uuid
		ORDER BY occurred_at DESC LIMIT 1
	`, tenantID, roleID).Scan(&action, &entityType); err != nil {
		t.Fatalf("read scope audit: %v", err)
	}
	if action != "role.access.updated" || entityType != "role" {
		t.Fatalf("scope audit = %s/%s", action, entityType)
	}
}

func TestCanonicalGodownLookupHonorsExplicitUUIDScopes(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	ctx := context.Background()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	suffix := formatTestSuffix(time.Now().UnixNano())
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "godown-scope-"+suffix)
	defer database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
	var allowedID, deniedID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_godowns (tenant_id, legacy_id, code, name)
		VALUES ($1::uuid, $2, $2, 'Allowed Godown')
		RETURNING id::text
	`, tenantID, "allowed-"+suffix).Scan(&allowedID); err != nil {
		t.Fatalf("seed allowed godown: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_godowns (tenant_id, legacy_id, code, name)
		VALUES ($1::uuid, $2, $2, 'Denied Godown')
		RETURNING id::text
	`, tenantID, "denied-"+suffix).Scan(&deniedID); err != nil {
		t.Fatalf("seed denied godown: %v", err)
	}
	operator := &sessionContext{
		UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID,
		Roles: []string{"operator"}, Permissions: []string{"master.read"},
		Scopes: map[string]map[string]bool{"godown": {allowedID: true, deniedID: false}},
	}
	server := &Server{database: database}
	listRecorder := httptest.NewRecorder()
	server.masterRecords(listRecorder, masterTestRequest(http.MethodGet, "/v1/master/godown", "", operator))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("godown list status = %d, body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listed struct {
		Records []masterRecordResponse `json:"records"`
	}
	if err := json.NewDecoder(listRecorder.Body).Decode(&listed); err != nil {
		t.Fatalf("decode godown list: %v", err)
	}
	if len(listed.Records) != 1 || listed.Records[0].ID != allowedID {
		t.Fatalf("godown list = %+v, want only allowed godown %s", listed.Records, allowedID)
	}

	detailRequest := masterTestRequest(http.MethodGet, "/v1/master/godown/"+deniedID, "", operator)
	detailRecorder := httptest.NewRecorder()
	server.masterRecordDetail(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusForbidden {
		t.Fatalf("denied godown detail status = %d, body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
}

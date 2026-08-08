package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// cleanupIsolatedLegacyTenant tears down a tenant seeded by seedDocumentTenant
// (plus any roles/audit rows a test added on top of it) in FK-safe order.
//
// tenants is referenced by branches, counters, users, roles, and audit_events
// with NO ACTION (non-cascading) foreign keys, so a bare
// `DELETE FROM tenants WHERE id = $1` fails and — when only called via a bare
// `defer database.ExecContext(...)` that never inspects the returned error —
// fails silently, leaving the tenant and its children behind. Every test in
// this file used that pattern; fixed here so repeated runs stop accumulating
// orphaned "document-test-*" tenants (389 were found accumulated in the
// shared dev database while investigating this).
func cleanupIsolatedLegacyTenant(ctx context.Context, database *sql.DB, tenantID string) {
	_, _ = database.ExecContext(ctx, `DELETE FROM audit_events WHERE tenant_id = $1::uuid`, tenantID)
	_, _ = database.ExecContext(ctx, `DELETE FROM users WHERE tenant_id = $1::uuid`, tenantID)
	_, _ = database.ExecContext(ctx, `DELETE FROM roles WHERE tenant_id = $1::uuid`, tenantID)
	_, _ = database.ExecContext(ctx, `DELETE FROM counters WHERE tenant_id = $1::uuid`, tenantID)
	_, _ = database.ExecContext(ctx, `DELETE FROM branches WHERE tenant_id = $1::uuid`, tenantID)
	_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
}

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
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)
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
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)
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

// legacyGroupRightsMatrixSourceTenantID is the tenant whose group_rights table
// was populated by the Phase R legacy security import (009/019 migrations).
// The tests below read it read-only and mirror sampled rows into an isolated,
// disposable tenant so the assertions exercise the exact loadOperatorAccess /
// requirePermission code paths against real migrated right codes without
// mutating the shared tenant's live membership state.
const legacyGroupRightsMatrixSourceTenantID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

var legacyGroupRightsFourGroups = []string{"ADMINISTRATOR", "REMOTE", "SALES OFFICER", "SHIFT INCHARGE"}

type sampledLegacyRightRow struct {
	RightCode string
	Allowed   bool
}

// TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups verifies
// that loadOperatorAccess (the function s.authenticate and the /v1/access,
// /v1/roles/{id}/rights handlers all rely on) derives its effective
// permission/legacy-right output directly from the migrated group_rights
// table, row by row, for every one of the 4 legacy groups produced by the
// 009/019 import — rather than from any hardcoded or partial Go table.
//
// For each group it:
//  1. Samples up to 19 real rows (all 6 for REMOTE, whose entire migrated
//     matrix only has 6 right codes) from the live migrated table for
//     tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee and copies them verbatim
//     (right_code + allowed) into an isolated disposable tenant so a real
//     user_memberships -> group_rights join can be exercised safely.
//  2. Adds one synthetic allow/deny pair sharing a single normalized
//     permission string, because the real migrated table (both tenants that
//     have imported data) currently contains zero allowed=false rows and no
//     populated `permission` column — see the accompanying verification
//     report for that finding. The pair proves the deny-wins precedence
//     that the SQL implements is real, not vestigial.
//  3. Confirms none of the sampled real right codes happen to collide with
//     any of the ~20 permission names the Go middleware actually gates on,
//     documenting that the migrated numeric right codes do not currently
//     resolve to any modern permission string.
func TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups(t *testing.T) {
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

	var sourceTotal int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM group_rights WHERE tenant_id = $1::uuid`, legacyGroupRightsMatrixSourceTenantID).Scan(&sourceTotal); err != nil {
		t.Fatalf("count migrated group_rights: %v", err)
	}
	if sourceTotal != 726 {
		t.Fatalf("migrated group_rights row count for tenant %s = %d, want 726 (source data changed since this test was written)", legacyGroupRightsMatrixSourceTenantID, sourceTotal)
	}

	suffix := formatTestSuffix(time.Now().UnixNano())
	tenantID, _, _, _ := seedDocumentTenant(t, ctx, database, "rights-matrix-"+suffix)
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)

	roleIDByCode := map[string]string{}
	for _, code := range legacyGroupRightsFourGroups {
		var id string
		if err := database.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE tenant_id = $1::uuid AND code = $2`, tenantID, code).Scan(&id); err != nil {
			t.Fatalf("legacy group %q missing from freshly seeded tenant (seed_legacy_groups_for_tenant trigger not firing): %v", code, err)
		}
		roleIDByCode[code] = id
	}

	sampleByGroup := map[string][]sampledLegacyRightRow{}
	for _, code := range legacyGroupRightsFourGroups {
		rows, err := database.QueryContext(ctx, `
			SELECT gr.right_code, gr.allowed
			FROM group_rights gr
			JOIN roles r ON r.id = gr.role_id AND r.tenant_id = gr.tenant_id
			WHERE gr.tenant_id = $1::uuid AND r.code = $2
			ORDER BY gr.right_code::int
			LIMIT 19`, legacyGroupRightsMatrixSourceTenantID, code)
		if err != nil {
			t.Fatalf("sample migrated rights for %q: %v", code, err)
		}
		sample := make([]sampledLegacyRightRow, 0, 19)
		for rows.Next() {
			var right sampledLegacyRightRow
			if err := rows.Scan(&right.RightCode, &right.Allowed); err != nil {
				rows.Close()
				t.Fatalf("scan sampled right for %q: %v", code, err)
			}
			sample = append(sample, right)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("iterate sampled rights for %q: %v", code, err)
		}
		rows.Close()
		if code == "REMOTE" {
			if len(sample) != 6 {
				t.Fatalf("REMOTE migrated right sample = %d rows, want 6 (its entire real matrix)", len(sample))
			}
		} else if len(sample) < 19 {
			t.Fatalf("%s migrated right sample = %d rows, want 19 (source data shrank below the documented count)", code, len(sample))
		}
		sampleByGroup[code] = sample
	}

	// Every sampled real row must be an explicit allow: this documents, as
	// part of the verification, that the migrated table currently contains
	// no allowed=false rows at all (system-wide, not just for this tenant).
	for code, sample := range sampleByGroup {
		for _, right := range sample {
			if !right.Allowed {
				t.Fatalf("unexpected allowed=false row in real migrated sample for %s (right_code=%s); the deny-wins synthetic pair below is no longer the only source of denies", code, right.RightCode)
			}
		}
	}

	syntheticPermissionByGroup := map[string]string{}
	for _, code := range legacyGroupRightsFourGroups {
		roleID := roleIDByCode[code]
		for _, right := range sampleByGroup[code] {
			if _, err := database.ExecContext(ctx, `
				INSERT INTO group_rights (tenant_id, role_id, right_code, allowed)
				VALUES ($1::uuid, $2::uuid, $3, $4)`, tenantID, roleID, right.RightCode, right.Allowed); err != nil {
				t.Fatalf("copy migrated right %s for %s into isolated tenant: %v", right.RightCode, code, err)
			}
		}
		slug := strings.ToLower(strings.ReplaceAll(code, " ", "-"))
		permission := fmt.Sprintf("synthetic.%s.%s", slug, suffix)
		syntheticPermissionByGroup[code] = permission
		if _, err := database.ExecContext(ctx, `
			INSERT INTO group_rights (tenant_id, role_id, right_code, permission, allowed)
			VALUES ($1::uuid, $2::uuid, $3, $4, true)`, tenantID, roleID, "synthetic-allow-"+slug+"-"+suffix, permission); err != nil {
			t.Fatalf("seed synthetic allow right for %s: %v", code, err)
		}
		if _, err := database.ExecContext(ctx, `
			INSERT INTO group_rights (tenant_id, role_id, right_code, permission, allowed)
			VALUES ($1::uuid, $2::uuid, $3, $4, false)`, tenantID, roleID, "synthetic-deny-"+slug+"-"+suffix, permission); err != nil {
			t.Fatalf("seed synthetic deny right for %s: %v", code, err)
		}
	}

	for _, code := range legacyGroupRightsFourGroups {
		t.Run(code, func(t *testing.T) {
			var userID string
			if err := database.QueryRowContext(ctx, `
				INSERT INTO users (tenant_id, username, display_name, password_hash)
				VALUES ($1::uuid, $2, $2, 'unused-hash') RETURNING id::text`,
				tenantID, "rights-matrix-"+strings.ToLower(strings.ReplaceAll(code, " ", "-"))+"-"+suffix).Scan(&userID); err != nil {
				t.Fatalf("seed operator for %s: %v", code, err)
			}
			if _, err := database.ExecContext(ctx, `
				INSERT INTO user_memberships (user_id, tenant_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
				userID, tenantID, roleIDByCode[code]); err != nil {
				t.Fatalf("bind operator to %s: %v", code, err)
			}

			operator := &sessionContext{UserID: userID, TenantID: tenantID, Roles: []string{code}}
			if err := loadOperatorAccess(ctx, database, operator); err != nil {
				t.Fatalf("loadOperatorAccess for %s: %v", code, err)
			}

			permissionSet := make(map[string]bool, len(operator.Permissions))
			for _, permission := range operator.Permissions {
				permissionSet[permission] = true
			}
			legacyByCode := make(map[string]legacyRight, len(operator.LegacyRights))
			for _, right := range operator.LegacyRights {
				legacyByCode[strings.ToLower(right.RightCode)] = right
			}

			// 1. Every sampled real allowed right_code must appear, unchanged,
			//    as an effective permission and as an allowed legacy-right row —
			//    proving loadOperatorAccess reads the migrated table row by row.
			for _, right := range sampleByGroup[code] {
				key := strings.ToLower(right.RightCode)
				if !permissionSet[key] {
					t.Fatalf("%s: migrated allowed right_code %q did not surface as an effective permission", code, right.RightCode)
				}
				resolved, ok := legacyByCode[key]
				if !ok {
					t.Fatalf("%s: migrated right_code %q missing from resolved LegacyRights", code, right.RightCode)
				}
				if !resolved.Allowed {
					t.Fatalf("%s: migrated right_code %q resolved as denied, want allowed (source table says allowed=true)", code, right.RightCode)
				}
			}

			// 2. None of the sampled real right codes accidentally collide with
			//    a modern permission name the Go middleware gates on — this is
			//    the concrete evidence that the raw legacy numeric right codes
			//    do not currently unlock any requirePermission-gated endpoint.
			for _, right := range sampleByGroup[code] {
				key := strings.ToLower(right.RightCode)
				if _, known := supportedRolePermissions[key]; known {
					t.Fatalf("%s: migrated right_code %q unexpectedly equals a modern permission name; requirePermission gating assumptions must be revisited", code, right.RightCode)
				}
			}

			// 3. The synthetic allow/deny pair on the SAME normalized permission
			//    proves deny-wins precedence is real: the permission must be
			//    absent from the effective set even though an allow row exists.
			permission := syntheticPermissionByGroup[code]
			if permissionSet[permission] {
				t.Fatalf("%s: explicit group_rights deny for %q did not win over the paired allow row", code, permission)
			}
			slug := strings.ToLower(strings.ReplaceAll(code, " ", "-"))
			allowRow, ok := legacyByCode["synthetic-allow-"+slug+"-"+suffix]
			if !ok || !allowRow.Allowed {
				t.Fatalf("%s: synthetic allow right_code missing or not marked allowed in LegacyRights", code)
			}
			denyRow, ok := legacyByCode["synthetic-deny-"+slug+"-"+suffix]
			if !ok || denyRow.Allowed {
				t.Fatalf("%s: synthetic deny right_code missing or not marked denied in LegacyRights", code)
			}
		})
	}
}

// TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt closes
// the loop from a group_rights row to an actual HTTP-level requirePermission
// decision, and demonstrates the one documented exception to "fully
// data-driven": the ADMINISTRATOR legacy group's role code matches the
// hardcoded hasTenantAdminRole allow-list ("administrator"), so its access is
// governed by that role-name check, not by its 486-row slice of the migrated
// table. A SALES OFFICER operator is used as the non-admin control group to
// prove the same mechanism genuinely is table-driven once a right's
// `permission` column is populated.
func TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt(t *testing.T) {
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
	tenantID, _, _, _ := seedDocumentTenant(t, ctx, database, "rights-http-"+suffix)
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)

	var salesRoleID, adminRoleID string
	if err := database.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE tenant_id = $1::uuid AND code = 'SALES OFFICER'`, tenantID).Scan(&salesRoleID); err != nil {
		t.Fatalf("locate SALES OFFICER role: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE tenant_id = $1::uuid AND code = 'ADMINISTRATOR'`, tenantID).Scan(&adminRoleID); err != nil {
		t.Fatalf("locate ADMINISTRATOR role: %v", err)
	}

	// SALES OFFICER: grant reports.read purely via an imported right whose
	// permission column IS populated (simulating a completed right-code ->
	// permission mapping), and grant + explicitly revoke master.write the
	// same way, so requirePermission's outcome can be attributed solely to
	// the group_rights table content.
	seedRight := func(roleID, rightCode, permission string, allowed bool) {
		t.Helper()
		if _, err := database.ExecContext(ctx, `
			INSERT INTO group_rights (tenant_id, role_id, right_code, permission, allowed)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5)`, tenantID, roleID, rightCode, permission, allowed); err != nil {
			t.Fatalf("seed group_rights right_code=%s permission=%s allowed=%v: %v", rightCode, permission, allowed, err)
		}
	}
	seedRight(salesRoleID, "http-trace-1", "reports.read", true)
	seedRight(salesRoleID, "http-trace-2", "master.write", true)
	seedRight(salesRoleID, "http-trace-2-deny", "master.write", false)
	// ADMINISTRATOR: explicitly deny manage.groups via the table, with no
	// competing allow anywhere, to isolate the bypass from the deny-wins path.
	seedRight(adminRoleID, "http-trace-3", "manage.groups", false)

	newOperator := func(t *testing.T, roleID, roleCode, usernameSuffix string) *sessionContext {
		t.Helper()
		var userID string
		if err := database.QueryRowContext(ctx, `
			INSERT INTO users (tenant_id, username, display_name, password_hash)
			VALUES ($1::uuid, $2, $2, 'unused-hash') RETURNING id::text`,
			tenantID, "rights-http-"+usernameSuffix+"-"+suffix).Scan(&userID); err != nil {
			t.Fatalf("seed operator: %v", err)
		}
		if _, err := database.ExecContext(ctx, `
			INSERT INTO user_memberships (user_id, tenant_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
			userID, tenantID, roleID); err != nil {
			t.Fatalf("bind operator: %v", err)
		}
		operator := &sessionContext{UserID: userID, TenantID: tenantID, Roles: []string{roleCode}}
		if err := loadOperatorAccess(ctx, database, operator); err != nil {
			t.Fatalf("loadOperatorAccess: %v", err)
		}
		return operator
	}

	salesOperator := newOperator(t, salesRoleID, "SALES OFFICER", "sales")
	adminOperator := newOperator(t, adminRoleID, "ADMINISTRATOR", "admin")

	server := &Server{database: database}
	checkPermission := func(t *testing.T, operator *sessionContext, permission string) bool {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/v1/trace-check", nil)
		request = request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
		recorder := httptest.NewRecorder()
		return server.requirePermission(request, recorder, operator, permission)
	}

	if !checkPermission(t, salesOperator, "reports.read") {
		t.Fatal("SALES OFFICER: table-granted reports.read was rejected by requirePermission")
	}
	if checkPermission(t, salesOperator, "master.write") {
		t.Fatal("SALES OFFICER: master.write should be denied — the imported deny row must win over the imported allow row")
	}
	if checkPermission(t, salesOperator, "manage.groups") {
		t.Fatal("SALES OFFICER: manage.groups was never granted by the table and must remain denied")
	}
	if !checkPermission(t, adminOperator, "manage.groups") {
		t.Fatal("ADMINISTRATOR: expected the hardcoded tenant-admin bypass to grant manage.groups despite the table's explicit deny row — enforcement for this group is role-name-driven, not table-driven")
	}
}

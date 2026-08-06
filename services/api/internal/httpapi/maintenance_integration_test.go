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

func TestMaintenanceManageOperationsIntegration(t *testing.T) {
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

	suffix := time.Now().UnixNano()
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "maintenance-"+formatTestSuffix(suffix))
	defer database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
	operator := &sessionContext{
		UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID,
		TokenHash: "maintenance-current-session", Roles: []string{"tenant_admin"},
	}
	server := &Server{database: database}

	t.Run("integrity returns scoped checks and an operation id", func(t *testing.T) {
		request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/check-database-integrity", `{}`, operator)
		recorder := httptest.NewRecorder()
		server.maintenanceAction(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("integrity status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			OperationID string             `json:"operationId"`
			Status      string             `json:"status"`
			Checks      []maintenanceCheck `json:"checks"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode integrity response: %v", err)
		}
		if response.OperationID == "" || response.Status != "completed" || len(response.Checks) == 0 {
			t.Fatalf("integrity response = %+v", response)
		}
		var action, entityType string
		if err := database.QueryRowContext(ctx, `SELECT action, entity_type FROM audit_events WHERE id = $1::uuid`, response.OperationID).Scan(&action, &entityType); err != nil {
			t.Fatalf("read integrity audit: %v", err)
		}
		if action != "maintenance.check-database-integrity" || entityType != "maintenance_operation" {
			t.Fatalf("integrity audit = %s/%s", action, entityType)
		}
	})

	t.Run("backup is audited as not configured", func(t *testing.T) {
		request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/backup-database", `{"destination":"C:\\backup\\db.bak"}`, operator)
		recorder := httptest.NewRecorder()
		server.maintenanceAction(recorder, request)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("backup status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			OperationID string `json:"operationId"`
			Status      string `json:"status"`
			Message     string `json:"message"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode backup response: %v", err)
		}
		if response.OperationID == "" || response.Status != "not_configured" || !strings.Contains(response.Message, "no database backup") {
			t.Fatalf("backup response = %+v", response)
		}
		var status string
		if err := database.QueryRowContext(ctx, `SELECT payload->>'status' FROM audit_events WHERE id = $1::uuid`, response.OperationID).Scan(&status); err != nil {
			t.Fatalf("read backup audit: %v", err)
		}
		if status != "not_configured" {
			t.Fatalf("backup audit status = %q", status)
		}
	})

	t.Run("password change is a real audited action", func(t *testing.T) {
		if _, err := database.ExecContext(ctx, `UPDATE users SET password_hash = crypt('old-password', gen_salt('bf')) WHERE id = $1::uuid`, operatorID); err != nil {
			t.Fatalf("seed password: %v", err)
		}
		request := maintenanceTestRequest(http.MethodPost, "/v1/auth/change-password", `{"currentPassword":"old-password","newPassword":"new-password","confirmPassword":"new-password"}`, operator)
		recorder := httptest.NewRecorder()
		server.changePassword(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("password status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var valid bool
		if err := database.QueryRowContext(ctx, `SELECT crypt('new-password', password_hash) = password_hash FROM users WHERE id = $1::uuid`, operatorID).Scan(&valid); err != nil {
			t.Fatalf("verify password: %v", err)
		}
		if !valid {
			t.Fatal("password was not changed")
		}
	})
}

func TestSessionMonitorIsBranchScopedIntegration(t *testing.T) {
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
	suffix := time.Now().UnixNano()
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "session-"+formatTestSuffix(suffix))
	defer database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
	var otherBranchID, otherCounterID, otherUserID string
	if err := database.QueryRowContext(ctx, `INSERT INTO branches (tenant_id, code, name) VALUES ($1::uuid, $2, 'Other Branch') RETURNING id::text`, tenantID, "other-"+formatTestSuffix(suffix)).Scan(&otherBranchID); err != nil {
		t.Fatalf("seed other branch: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO counters (tenant_id, branch_id, code, name) VALUES ($1::uuid, $2::uuid, $3, 'Other Counter') RETURNING id::text`, tenantID, otherBranchID, "other-counter-"+formatTestSuffix(suffix)).Scan(&otherCounterID); err != nil {
		t.Fatalf("seed other counter: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO users (tenant_id, username, display_name, password_hash) VALUES ($1::uuid, $2, 'Other User', 'unused') RETURNING id::text`, tenantID, "other-user-"+formatTestSuffix(suffix)).Scan(&otherUserID); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, tenant_id, branch_id, counter_id, expires_at)
		VALUES
			('maintenance-current-session', $1::uuid, $2::uuid, $3::uuid, $4::uuid, now() + interval '1 hour'),
			('maintenance-other-session', $5::uuid, $2::uuid, $6::uuid, $7::uuid, now() + interval '1 hour')`,
		operatorID, tenantID, branchID, counterID, otherUserID, otherBranchID, otherCounterID); err != nil {
		t.Fatalf("seed sessions: %v", err)
	}
	operator := &sessionContext{UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID, TokenHash: "maintenance-current-session", Roles: []string{"tenant_admin"}}
	recorder := httptest.NewRecorder()
	server := &Server{database: database}
	server.sessionMonitor(recorder, maintenanceTestRequest(http.MethodGet, "/v1/session-monitor", "", operator))
	if recorder.Code != http.StatusOK {
		t.Fatalf("session monitor status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Sessions []struct {
			BranchID string `json:"branchId"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(response.Sessions) != 1 || response.Sessions[0].BranchID != branchID {
		t.Fatalf("branch-scoped sessions = %+v", response.Sessions)
	}
}

func maintenanceTestRequest(method, target, body string, operator *sessionContext) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	return request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
}

func formatTestSuffix(value int64) string {
	return strings.ReplaceAll(strings.TrimSpace(time.Unix(0, value).Format("20060102150405.000000000")), ".", "-")
}

package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)
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

	t.Run("backup performs a real pg_dump and audits the resulting file", func(t *testing.T) {
		// This dumps whatever database DATABASE_URL points at for this test
		// run. It never restores anything, so it is non-destructive even if
		// DATABASE_URL happens to point at a shared database.
		backupDir := t.TempDir()
		t.Setenv("ABUZAR_MAINTENANCE_BACKUP_DIR", backupDir)
		request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/backup-database", `{"destination":"C:\\backup\\db.bak"}`, operator)
		recorder := httptest.NewRecorder()
		server.maintenanceAction(recorder, request)
		if recorder.Code != http.StatusOK {
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
		if response.OperationID == "" {
			t.Fatalf("backup response = %+v", response)
		}
		// pg_dump is expected on PATH in this environment (real adapter), but
		// this assertion also tolerates a machine without it, where the
		// handler must honestly fall back to not_configured rather than
		// pretending a backup happened.
		if response.Status != "completed" && response.Status != "not_configured" {
			t.Fatalf("backup status = %q, want completed or not_configured", response.Status)
		}
		var auditedStatus, filePath string
		if err := database.QueryRowContext(ctx, `SELECT payload->>'status', COALESCE(payload->>'filePath', '') FROM audit_events WHERE id = $1::uuid`, response.OperationID).Scan(&auditedStatus, &filePath); err != nil {
			t.Fatalf("read backup audit: %v", err)
		}
		if auditedStatus != response.Status {
			t.Fatalf("backup audit status = %q, want %q", auditedStatus, response.Status)
		}
		if response.Status == "completed" {
			if filePath == "" {
				t.Fatal("completed backup audit did not record a filePath")
			}
			info, statErr := os.Stat(filePath)
			if statErr != nil {
				t.Fatalf("completed backup did not produce a file on disk: %v", statErr)
			}
			if info.Size() == 0 {
				t.Fatal("completed backup produced an empty file")
			}
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

	t.Run("item maintenance updates canonical payloads and audit history", func(t *testing.T) {
		itemCode := "ITEM-" + formatTestSuffix(suffix)
		var itemID string
		if err := database.QueryRowContext(ctx, `
			INSERT INTO master_items (tenant_id, legacy_id, code, name, payload)
			VALUES ($1::uuid, $2, $2, 'Maintenance Item', '{"SalePrice1":"10.00","SalePrice2":"11.00","PurchasePrice":"7.50"}'::jsonb)
			RETURNING id::text
		`, tenantID, itemCode).Scan(&itemID); err != nil {
			t.Fatalf("seed maintenance item: %v", err)
		}
		supplierLegacyID := "SUP-" + formatTestSuffix(suffix)
		var supplierID string
		if err := database.QueryRowContext(ctx, `
			INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name)
			VALUES ($1::uuid, 'supplier', $2, $2, 'Maintenance Supplier')
			RETURNING id::text
		`, tenantID, supplierLegacyID).Scan(&supplierID); err != nil {
			t.Fatalf("seed maintenance supplier: %v", err)
		}
		var godownID, otherBranchID string
		if err := database.QueryRowContext(ctx, `
			INSERT INTO master_godowns (tenant_id, legacy_id, code, name)
			VALUES ($1::uuid, $2, $2, 'Maintenance Godown')
			RETURNING id::text
		`, tenantID, "G-"+formatTestSuffix(suffix)).Scan(&godownID); err != nil {
			t.Fatalf("seed maintenance godown: %v", err)
		}
		if err := database.QueryRowContext(ctx, `
			INSERT INTO stock_batches (tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, unit_cost)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, 'LOCK-ME', 7.50)
			RETURNING branch_id::text
		`, tenantID, branchID, itemID, itemCode, godownID).Scan(&branchID); err != nil {
			t.Fatalf("seed maintenance batch: %v", err)
		}
		if err := database.QueryRowContext(ctx, `INSERT INTO branches (tenant_id, code, name) VALUES ($1::uuid, $2, 'Other Maintenance Branch') RETURNING id::text`, tenantID, "other-maintenance-"+formatTestSuffix(suffix)).Scan(&otherBranchID); err != nil {
			t.Fatalf("seed other maintenance branch: %v", err)
		}
		if _, err := database.ExecContext(ctx, `
			INSERT INTO stock_batches (tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, unit_cost)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, 'LOCK-ME', 7.50)
		`, tenantID, otherBranchID, itemID, itemCode, godownID); err != nil {
			t.Fatalf("seed other-branch maintenance batch: %v", err)
		}

		run := func(kind, body string) string {
			request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/"+kind, body, operator)
			recorder := httptest.NewRecorder()
			server.maintenanceAction(recorder, request)
			if recorder.Code != http.StatusAccepted {
				t.Fatalf("%s status = %d, body=%s", kind, recorder.Code, recorder.Body.String())
			}
			var response struct {
				OperationID string `json:"operationId"`
				Status      string `json:"status"`
				Message     string `json:"message"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode %s response: %v", kind, err)
			}
			if response.OperationID == "" || response.Status != "completed" {
				t.Fatalf("%s response = %+v", kind, response)
			}
			return response.OperationID
		}

		lockAudit := run("lock-item-batches", fmt.Sprintf(`{"itemCode":%q,"batch":"LOCK-ME","locked":"Yes"}`, itemCode))
		var locked bool
		if err := database.QueryRowContext(ctx, `
			SELECT locked FROM stock_batches
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid AND batch_number = 'LOCK-ME'
		`, tenantID, branchID, itemID).Scan(&locked); err != nil {
			t.Fatalf("read locked maintenance batch: %v", err)
		}
		if !locked {
			t.Fatal("maintenance batch lock did not update the current branch")
		}
		var affected string
		if err := database.QueryRowContext(ctx, `SELECT payload->>'affectedBatches' FROM audit_events WHERE id = $1::uuid`, lockAudit).Scan(&affected); err != nil {
			t.Fatalf("read batch lock audit: %v", err)
		}
		if affected != "1" {
			t.Fatalf("batch lock audit affectedBatches = %q, want 1", affected)
		}
		run("lock-item-batches", fmt.Sprintf(`{"itemCode":%q,"batch":"LOCK-ME","locked":false}`, itemCode))
		if err := database.QueryRowContext(ctx, `
			SELECT locked FROM stock_batches
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid AND batch_number = 'LOCK-ME'
		`, tenantID, branchID, itemID).Scan(&locked); err != nil {
			t.Fatalf("read unlocked maintenance batch: %v", err)
		}
		if locked {
			t.Fatal("maintenance batch unlock did not update the current branch")
		}
		if err := database.QueryRowContext(ctx, `
			SELECT locked FROM stock_batches
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_id = $3::uuid AND batch_number = 'LOCK-ME'
		`, tenantID, otherBranchID, itemID).Scan(&locked); err != nil {
			t.Fatalf("read other-branch maintenance batch: %v", err)
		}
		if locked {
			t.Fatal("batch mutation escaped the current branch scope")
		}

		priceAudit := run("change-items-price", fmt.Sprintf(`{"itemCode":%q,"priceType":"Sale Price","price":12.75,"effectiveDate":"2026-08-07"}`, itemCode))
		var salePrice1, salePrice, salePrice2, purchasePrice, previousPrice, newPrice string
		if err := database.QueryRowContext(ctx, `
			SELECT payload->>'SalePrice1', payload->>'SalePrice', payload->>'SalePrice2', payload->>'PurchasePrice'
			FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
		`, tenantID, itemID).Scan(&salePrice1, &salePrice, &salePrice2, &purchasePrice); err != nil {
			t.Fatalf("read updated price payload: %v", err)
		}
		if salePrice1 != "12.75" || salePrice != "12.75" || salePrice2 != "11.00" || purchasePrice != "7.50" {
			t.Fatalf("price payload = %s/%s/%s/%s", salePrice1, salePrice, salePrice2, purchasePrice)
		}
		if err := database.QueryRowContext(ctx, `SELECT payload->>'previousValue', payload->>'newValue' FROM audit_events WHERE id = $1::uuid`, priceAudit).Scan(&previousPrice, &newPrice); err != nil {
			t.Fatalf("read price audit: %v", err)
		}
		if previousPrice != "10.00" || newPrice != "12.75" {
			t.Fatalf("price audit = %s/%s", previousPrice, newPrice)
		}

		run("change-item-discount", fmt.Sprintf(`{"itemCode":%q,"discountType":"Percent","discount":5.5}`, itemCode))
		run("update-item-basic-data", fmt.Sprintf(`{"itemCode":%q,"field":"Name","value":"Renamed Maintenance Item"}`, itemCode))
		run("change-item-reorder-qty", fmt.Sprintf(`{"itemCode":%q,"reorderQty":12.5,"minimumQty":3}`, itemCode))
		supplierAudit := run("update-item-suppliers", fmt.Sprintf(`{"itemCode":%q,"supplier":%q,"purchasePrice":8.25,"priority":2}`, itemCode, supplierLegacyID))
		var name, saleDiscount, legacyDiscount, reorder, minimum string
		if err := database.QueryRowContext(ctx, `
		SELECT name, payload->>'SaleDiscPercent', payload->>'DiscPerc', payload->>'ReorderQuantity', payload->>'MinimumQuantity'
		FROM master_items WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, itemID).Scan(&name, &saleDiscount, &legacyDiscount, &reorder, &minimum); err != nil {
			t.Fatalf("read canonical item maintenance result: %v", err)
		}
		if name != "Renamed Maintenance Item" || saleDiscount != "5.50" || legacyDiscount != "5.50" || reorder != "12.5" || minimum != "3" {
			t.Fatalf("canonical item maintenance result = %s/%s/%s/%s/%s", name, saleDiscount, legacyDiscount, reorder, minimum)
		}
		var linkedSupplierID, supplierRate string
		var supplierPriority int
		if err := database.QueryRowContext(ctx, `
			SELECT supplier_id::text, priority, rate::text
			FROM item_suppliers
			WHERE tenant_id = $1::uuid AND legacy_item_id = $2 AND legacy_supplier_id = $3
		`, tenantID, itemCode, supplierLegacyID).Scan(&linkedSupplierID, &supplierPriority, &supplierRate); err != nil {
			t.Fatalf("read canonical item supplier maintenance result: %v", err)
		}
		if linkedSupplierID != supplierID || supplierPriority != 2 || supplierRate != "8.2500" {
			t.Fatalf("canonical item supplier result = %s/%d/%s", linkedSupplierID, supplierPriority, supplierRate)
		}
		var supplierAuditField string
		if err := database.QueryRowContext(ctx, `SELECT payload->>'supplierLegacyId' FROM audit_events WHERE id = $1::uuid`, supplierAudit).Scan(&supplierAuditField); err != nil {
			t.Fatalf("read item supplier audit: %v", err)
		}
		if supplierAuditField != supplierLegacyID {
			t.Fatalf("item supplier audit supplierLegacyId = %q, want %q", supplierAuditField, supplierLegacyID)
		}
	})
}

// TestChannelSendMaintenanceIntegration exercises the "test-email"/
// "test-sms" maintenance actions end-to-end against a local httptest fake
// standing in for a branch-edge instance. It never contacts any real SMTP
// server or SMS gateway.
func TestChannelSendMaintenanceIntegration(t *testing.T) {
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
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "channel-"+formatTestSuffix(suffix))
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)
	operator := &sessionContext{
		UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID,
		TokenHash: "channel-send-current-session", Roles: []string{"tenant_admin"},
	}
	server := &Server{database: database}

	t.Run("not configured without an edge channel URL", func(t *testing.T) {
		t.Setenv("ABUZAR_EDGE_CHANNEL_URL", "")
		t.Setenv("ABUZAR_EDGE_CHANNEL_SECRET", "")
		request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/test-email", `{"to":"customer@example.test","subject":"Hi","body":"Test"}`, operator)
		recorder := httptest.NewRecorder()
		server.maintenanceAction(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if response.Status != "not_configured" {
			t.Fatalf("status = %q, want not_configured", response.Status)
		}
	})

	t.Run("completed when the edge adapter accepts the message", func(t *testing.T) {
		fakeEdge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/hardware/email/send" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"sent":true}`))
		}))
		defer fakeEdge.Close()
		t.Setenv("ABUZAR_EDGE_CHANNEL_URL", fakeEdge.URL)
		t.Setenv("ABUZAR_EDGE_CHANNEL_SECRET", "test-secret")

		request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/test-email", `{"to":"customer@example.test","subject":"Hi","body":"Test"}`, operator)
		recorder := httptest.NewRecorder()
		server.maintenanceAction(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if response.Status != "completed" {
			t.Fatalf("status = %q, want completed, body=%s", response.Status, recorder.Body.String())
		}
	})

	t.Run("not configured when the edge reports no adapter", func(t *testing.T) {
		fakeEdge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"hardware_adapter_unavailable","detail":"No physical hardware adapter is configured for this branch."}`))
		}))
		defer fakeEdge.Close()
		t.Setenv("ABUZAR_EDGE_CHANNEL_URL", fakeEdge.URL)
		t.Setenv("ABUZAR_EDGE_CHANNEL_SECRET", "")

		request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/test-sms", `{"to":"923001234567","message":"Hi"}`, operator)
		recorder := httptest.NewRecorder()
		server.maintenanceAction(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if response.Status != "not_configured" {
			t.Fatalf("status = %q, want not_configured, body=%s", response.Status, recorder.Body.String())
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
	currentTokenHash := fmt.Sprintf("maintenance-current-%d", suffix)
	otherTokenHash := fmt.Sprintf("maintenance-other-%d", suffix)
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "session-"+formatTestSuffix(suffix))
	defer cleanupIsolatedLegacyTenant(ctx, database, tenantID)
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
			($1, $2::uuid, $3::uuid, $4::uuid, $5::uuid, now() + interval '1 hour'),
			($6, $7::uuid, $3::uuid, $8::uuid, $9::uuid, now() + interval '1 hour')`,
		currentTokenHash, operatorID, tenantID, branchID, counterID,
		otherTokenHash, otherUserID, otherBranchID, otherCounterID); err != nil {
		t.Fatalf("seed sessions: %v", err)
	}
	operator := &sessionContext{UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID, TokenHash: currentTokenHash, Roles: []string{"tenant_admin"}}
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
	if kind := strings.TrimPrefix(request.URL.Path, "/v1/maintenance/"); kind != request.URL.Path {
		request.SetPathValue("kind", strings.Trim(kind, "/"))
	}
	return request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
}

func formatTestSuffix(value int64) string {
	return strings.ReplaceAll(strings.TrimSpace(time.Unix(0, value).Format("20060102150405.000000000")), ".", "-")
}

// pgAdminTestDSN connects to the always-present "postgres" maintenance
// database on the local instance, used only to CREATE/DROP the throwaway
// databases these tests own. It never touches abuzar_next.
func pgAdminTestDSN() string {
	return "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable"
}

func maintenanceTestDatabaseDSN(name string) string {
	return fmt.Sprintf("postgres://postgres@127.0.0.1:5432/%s?sslmode=disable", name)
}

// maintenanceTestDatabaseSuffix produces a unique suffix made only of the
// characters validMaintenanceDatabaseName accepts (letters, digits,
// underscore), so throwaway test database names are always valid restore
// targets.
func maintenanceTestDatabaseSuffix() string {
	return strings.ReplaceAll(formatTestSuffix(time.Now().UnixNano()), "-", "_")
}

// createMaintenanceTestDatabase creates a throwaway PostgreSQL database via a
// plain CREATE DATABASE statement (equivalent to running `createdb <name>`)
// and registers a cleanup that drops it, guaranteeing the database never
// outlives the test even on failure.
func createMaintenanceTestDatabase(t *testing.T, name string) {
	t.Helper()
	if !validMaintenanceDatabaseName(name) {
		t.Fatalf("unsafe test database name %q", name)
	}
	if name == "abuzar_next" {
		t.Fatal("refusing to use the live abuzar_next database as a test fixture")
	}
	admin, err := sql.Open("pgx", pgAdminTestDSN())
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("create throwaway database %s: %v", name, err)
	}
	t.Cleanup(func() {
		dropMaintenanceTestDatabase(t, name)
	})
}

func dropMaintenanceTestDatabase(t *testing.T, name string) {
	t.Helper()
	if !validMaintenanceDatabaseName(name) || name == "abuzar_next" {
		return
	}
	admin, err := sql.Open("pgx", pgAdminTestDSN())
	if err != nil {
		t.Logf("open postgres admin connection for cleanup of %s: %v", name, err)
		return
	}
	defer admin.Close()
	// Terminate any lingering backends (e.g. an idle pooled connection left
	// open by this test) so DROP DATABASE does not fail with "database is
	// being accessed by other users".
	_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, name)
	if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + name); err != nil {
		t.Logf("drop throwaway database %s: %v", name, err)
	}
}

// TestBackupRestorePgToolsRoundTrip exercises the real pg_dump/pg_restore
// wrappers used by handleDatabaseBackup/handleDatabaseRestore directly
// against two throwaway databases this test creates and drops itself
// (never abuzar_next): seed a source database, back it up with runPgDump,
// drop the source database entirely, restore the dump into a brand new
// database with runPgRestore, and confirm the seeded rows came back.
func TestBackupRestorePgToolsRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skip("pg_dump is not on PATH")
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		t.Skip("pg_restore is not on PATH")
	}
	suffix := maintenanceTestDatabaseSuffix()
	sourceDB := "abuzar_maintenance_test_src_" + suffix
	targetDB := "abuzar_maintenance_test_dst_" + suffix

	createMaintenanceTestDatabase(t, sourceDB)

	seedConn, err := sql.Open("pgx", maintenanceTestDatabaseDSN(sourceDB))
	if err != nil {
		t.Fatalf("open source database: %v", err)
	}
	if _, err := seedConn.Exec(`CREATE TABLE maintenance_probe (id serial PRIMARY KEY, label text NOT NULL)`); err != nil {
		seedConn.Close()
		t.Fatalf("seed schema: %v", err)
	}
	for _, label := range []string{"alpha", "beta", "gamma"} {
		if _, err := seedConn.Exec(`INSERT INTO maintenance_probe (label) VALUES ($1)`, label); err != nil {
			seedConn.Close()
			t.Fatalf("seed row %s: %v", label, err)
		}
	}
	if err := seedConn.Close(); err != nil {
		t.Fatalf("close source database connection: %v", err)
	}

	conn := maintenanceDBConn{Host: "127.0.0.1", Port: "5432", User: "postgres", SSLMode: "disable"}
	backupPath := filepath.Join(t.TempDir(), "roundtrip.dump")

	size, err := runPgDump(context.Background(), conn, sourceDB, backupPath)
	if err != nil {
		t.Fatalf("runPgDump: %v", err)
	}
	if size <= 0 {
		t.Fatalf("runPgDump reported non-positive size: %d", size)
	}
	if info, statErr := os.Stat(backupPath); statErr != nil || info.Size() != size {
		t.Fatalf("backup file mismatch: stat=%v size=%d reportedSize=%d", statErr, info.Size(), size)
	}

	// Drop the source database entirely before restoring, so the restore
	// step below can only be satisfied by the dump file, never by leftover
	// live data.
	dropMaintenanceTestDatabase(t, sourceDB)

	createMaintenanceTestDatabase(t, targetDB)
	if err := runPgRestore(context.Background(), conn, targetDB, backupPath); err != nil {
		t.Fatalf("runPgRestore: %v", err)
	}

	verifyConn, err := sql.Open("pgx", maintenanceTestDatabaseDSN(targetDB))
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer verifyConn.Close()
	rows, err := verifyConn.Query(`SELECT label FROM maintenance_probe ORDER BY id`)
	if err != nil {
		t.Fatalf("query restored rows: %v", err)
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			t.Fatalf("scan restored row: %v", err)
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate restored rows: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(labels) != len(want) {
		t.Fatalf("restored labels = %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("restored labels = %v, want %v", labels, want)
		}
	}
	if sourceDB == "abuzar_next" || targetDB == "abuzar_next" {
		t.Fatal("test fixture accidentally targeted abuzar_next")
	}
}

// TestBackupDatabaseHandlerRoundTrip drives the actual HTTP maintenance
// handlers (handleDatabaseBackup/handleDatabaseRestore, dispatched from
// maintenanceAction exactly as the router would) against a throwaway
// database standing in for the API's normal connection. It confirms the
// audit trail is written via the same recordMaintenanceOperation path as
// every other maintenance action, that the restore safety check refuses to
// target the live/connected database, and that data placed in the source
// database round-trips through a real backup and restore into a second,
// independent throwaway database. abuzar_next is never created, backed up,
// restored into, or referenced by this test.
func TestBackupDatabaseHandlerRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skip("pg_dump is not on PATH")
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		t.Skip("pg_restore is not on PATH")
	}
	// Pin connection resolution to the local trust-auth instance regardless
	// of what the ambient shell has exported, so this test is deterministic.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ABUZAR_APP_DATABASE_URL", "")
	t.Setenv("PGHOST", "127.0.0.1")
	t.Setenv("PGPORT", "5432")
	t.Setenv("PGUSER", "postgres")
	t.Setenv("PGPASSWORD", "")
	backupDir := t.TempDir()
	t.Setenv("ABUZAR_MAINTENANCE_BACKUP_DIR", backupDir)

	suffix := maintenanceTestDatabaseSuffix()
	liveDB := "abuzar_maintenance_test_live_" + suffix
	restoreDB := "abuzar_maintenance_test_restore_" + suffix

	createMaintenanceTestDatabase(t, liveDB)

	live, err := sql.Open("pgx", maintenanceTestDatabaseDSN(liveDB))
	if err != nil {
		t.Fatalf("open live database: %v", err)
	}
	defer live.Close()

	// Minimal schema: just enough for beginScopedTx + recordMaintenanceOperation
	// (the same audit trail every other maintenance action in this file uses)
	// to work, plus one small "tenant data" table to prove a real round trip.
	schema := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE tenants (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			legal_name text NOT NULL,
			code text NOT NULL UNIQUE,
			active boolean NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE branches (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id uuid NOT NULL REFERENCES tenants(id),
			code text NOT NULL,
			name text NOT NULL,
			UNIQUE (tenant_id, code)
		)`,
		`CREATE TABLE users (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id uuid NOT NULL REFERENCES tenants(id),
			username text NOT NULL,
			display_name text NOT NULL,
			password_hash text NOT NULL
		)`,
		`CREATE TABLE audit_events (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id uuid NOT NULL REFERENCES tenants(id),
			branch_id uuid REFERENCES branches(id),
			operator_id uuid REFERENCES users(id),
			action text NOT NULL,
			entity_type text NOT NULL,
			entity_id uuid,
			payload jsonb NOT NULL DEFAULT '{}'::jsonb,
			occurred_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE tenant_widgets (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id uuid NOT NULL REFERENCES tenants(id),
			label text NOT NULL
		)`,
	}
	for _, statement := range schema {
		if _, err := live.Exec(statement); err != nil {
			t.Fatalf("apply schema statement %q: %v", statement, err)
		}
	}

	var tenantID string
	if err := live.QueryRow(`INSERT INTO tenants (legal_name, code) VALUES ('Maintenance Test Tenant', $1) RETURNING id::text`, "maint-"+suffix).Scan(&tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	var operatorID string
	if err := live.QueryRow(`INSERT INTO users (tenant_id, username, display_name, password_hash) VALUES ($1::uuid, $2, 'Maintenance Test Operator', 'unused') RETURNING id::text`, tenantID, "maint-op-"+suffix).Scan(&operatorID); err != nil {
		t.Fatalf("seed operator: %v", err)
	}
	for _, label := range []string{"widget-alpha", "widget-beta", "widget-gamma"} {
		if _, err := live.Exec(`INSERT INTO tenant_widgets (tenant_id, label) VALUES ($1::uuid, $2)`, tenantID, label); err != nil {
			t.Fatalf("seed tenant widget %s: %v", label, err)
		}
	}

	operator := &sessionContext{UserID: operatorID, TenantID: tenantID, BranchID: "", TokenHash: "maintenance-backup-test", Roles: []string{"tenant_admin"}}
	server := &Server{database: live}

	// --- Backup ---
	backupRequest := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/backup-database", `{}`, operator)
	backupRecorder := httptest.NewRecorder()
	server.maintenanceAction(backupRecorder, backupRequest)
	if backupRecorder.Code != http.StatusOK {
		t.Fatalf("backup status = %d, body=%s", backupRecorder.Code, backupRecorder.Body.String())
	}
	var backupResponse struct {
		OperationID string `json:"operationId"`
		Status      string `json:"status"`
		Database    string `json:"database"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(backupRecorder.Body).Decode(&backupResponse); err != nil {
		t.Fatalf("decode backup response: %v", err)
	}
	if backupResponse.Status != "completed" {
		t.Fatalf("backup status = %q, want completed (pg_dump is on PATH): %s", backupResponse.Status, backupRecorder.Body.String())
	}
	if backupResponse.Database != liveDB {
		t.Fatalf("backup database = %q, want %q", backupResponse.Database, liveDB)
	}

	var auditedFilePath string
	var auditedSize int64
	if err := live.QueryRow(`SELECT payload->>'filePath', (payload->>'fileSizeBytes')::bigint FROM audit_events WHERE id = $1::uuid`, backupResponse.OperationID).Scan(&auditedFilePath, &auditedSize); err != nil {
		t.Fatalf("read backup audit: %v", err)
	}
	if auditedSize <= 0 {
		t.Fatalf("audited backup file size = %d, want > 0", auditedSize)
	}
	if info, statErr := os.Stat(auditedFilePath); statErr != nil || info.Size() != auditedSize {
		t.Fatalf("backup file on disk does not match audit: stat=%v", statErr)
	}
	if filepath.Dir(auditedFilePath) != backupDir {
		t.Fatalf("backup file %s was not written under the configured backup dir %s", auditedFilePath, backupDir)
	}
	backupFileName := filepath.Base(auditedFilePath)

	// --- Restore safety check: refusing to target the live/connected database ---
	unsafeRestoreRequest := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/restore-database", fmt.Sprintf(`{"targetDatabase":%q,"backupFile":%q}`, liveDB, backupFileName), operator)
	unsafeRecorder := httptest.NewRecorder()
	server.maintenanceAction(unsafeRecorder, unsafeRestoreRequest)
	if unsafeRecorder.Code != http.StatusConflict {
		t.Fatalf("restore-into-live status = %d, want %d, body=%s", unsafeRecorder.Code, http.StatusConflict, unsafeRecorder.Body.String())
	}

	// --- Restore into an independent throwaway database ---
	createMaintenanceTestDatabase(t, restoreDB)
	restoreRequest := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/restore-database", fmt.Sprintf(`{"targetDatabase":%q,"backupFile":%q}`, restoreDB, backupFileName), operator)
	restoreRecorder := httptest.NewRecorder()
	server.maintenanceAction(restoreRecorder, restoreRequest)
	if restoreRecorder.Code != http.StatusOK {
		t.Fatalf("restore status = %d, body=%s", restoreRecorder.Code, restoreRecorder.Body.String())
	}
	var restoreResponse struct {
		OperationID    string `json:"operationId"`
		Status         string `json:"status"`
		TargetDatabase string `json:"targetDatabase"`
	}
	if err := json.NewDecoder(restoreRecorder.Body).Decode(&restoreResponse); err != nil {
		t.Fatalf("decode restore response: %v", err)
	}
	if restoreResponse.Status != "completed" || restoreResponse.TargetDatabase != restoreDB {
		t.Fatalf("restore response = %+v", restoreResponse)
	}

	restored, err := sql.Open("pgx", maintenanceTestDatabaseDSN(restoreDB))
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer restored.Close()
	rows, err := restored.Query(`SELECT label FROM tenant_widgets ORDER BY label`)
	if err != nil {
		t.Fatalf("query restored tenant_widgets: %v", err)
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			t.Fatalf("scan restored widget: %v", err)
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate restored widgets: %v", err)
	}
	want := []string{"widget-alpha", "widget-beta", "widget-gamma"}
	if len(labels) != len(want) {
		t.Fatalf("restored tenant_widgets labels = %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("restored tenant_widgets labels = %v, want %v", labels, want)
		}
	}

	if liveDB == "abuzar_next" || restoreDB == "abuzar_next" {
		t.Fatal("test fixture accidentally targeted abuzar_next")
	}
}

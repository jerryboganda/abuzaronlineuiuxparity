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

package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestGodownTransferMovesStock(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "transfer-"+fmt.Sprint(time.Now().UnixNano()))
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)
	var destGodownID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_godowns (tenant_id, legacy_id, code, name)
		VALUES ($1::uuid, $2, $2, 'Transit Godown')
		RETURNING id::text
	`, fixture.tenantID, "godown-dest-"+fmt.Sprint(time.Now().UnixNano())).Scan(&destGodownID); err != nil {
		t.Fatalf("seed destination godown: %v", err)
	}
	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "transfer-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "XFER-001", ExpiryDate: "2030-01-01", Quantity: json.RawMessage(`5`), UnitCost: "4.00",
	})
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"}, Permissions: []string{"maintenance.write"},
	}
	tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin receiving: %v", err)
	}
	if err := projectEvent(ctx, tx, receive); err != nil {
		_ = tx.Rollback()
		t.Fatalf("project receiving: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit receiving: %v", err)
	}

	server := &Server{database: database}
	body := fmt.Sprintf(`{"sourceGodownId":%q,"destinationGodownId":%q,"occurredAt":"2026-08-09T00:00:00Z","idempotencyKey":"xfer-1","lines":[{"itemId":%q,"quantity":"3"}]}`,
		fixture.godownID, destGodownID, fixture.itemID)
	request := maintenanceTestRequest(http.MethodPost, "/v1/maintenance/godown-transfer", body, operator)
	recorder := httptest.NewRecorder()
	server.maintenanceAction(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("godown transfer status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	sourceBalance := godownOnHand(t, ctx, database, fixture.tenantID, fixture.branchID, fixture.itemLegacyID, fixture.godownID)
	destBalance := godownOnHand(t, ctx, database, fixture.tenantID, fixture.branchID, fixture.itemLegacyID, destGodownID)
	if sourceBalance != "2.0000" {
		t.Fatalf("source balance = %s, want 2.0000", sourceBalance)
	}
	if destBalance != "3.0000" {
		t.Fatalf("destination balance = %s, want 3.0000", destBalance)
	}
}

func godownOnHand(t *testing.T, ctx context.Context, database *sql.DB, tenantID, branchID, itemLegacyID, godownID string) string {
	t.Helper()
	var balance string
	if err := database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(on_hand), 0)::text
		FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND item_legacy_id = $3 AND godown_id = $4::uuid
	`, tenantID, branchID, itemLegacyID, godownID).Scan(&balance); err != nil {
		t.Fatalf("read godown balance: %v", err)
	}
	return balance
}

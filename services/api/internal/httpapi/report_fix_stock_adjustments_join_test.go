package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestHistoricalStockAdjustmentReportResolvesLegacyLineNames proves that the
// historical_stock_adjustment_lines half of the stock-adjustments UNION now
// resolves item/godown names via master_items/master_godowns, matching the
// stock_ledger half (Phase Q Finding F). Before the fix this branch always
// projected raw legacy codes even when item_id/godown_id were populated.
func TestHistoricalStockAdjustmentReportResolvesLegacyLineNames(t *testing.T) {
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

	suffix := "historical-adjustment-legacy-join-" + time.Now().Format("150405.000000")
	fixture := seedStockTenant(t, ctx, database, suffix)
	defer func() {
		cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)
	}()

	// Seed a historical_stock_adjustment_lines row whose item_id/godown_id
	// FK columns are populated (as the bulk-historical importer does), but
	// whose legacy_id text columns intentionally differ from the resolved
	// master-data names, so the assertion can distinguish "resolved name"
	// from "raw legacy code fallback".
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_stock_adjustment_lines
			(tenant_id, branch_id, legacy_id, adjustment_legacy_id, item_id, item_legacy_id,
			 godown_id, godown_legacy_id, batch, occurred_at, loose_quantity, price,
			 user_legacy_id, posted, source_table, source_table_row)
		VALUES
			($1::uuid, $2::uuid, 'legacy-line-1', 'legacy-adj-1', $3::uuid, 'raw-item-code',
			 $4::uuid, 'raw-godown-code', 'BATCH1', '2026-08-06T09:00:00Z', 3, 12.50,
			 'legacy-user', true, 'AdjDetail', 'row-1')
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.godownID); err != nil {
		t.Fatalf("seed historical stock adjustment line: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/item-reports-stock-adjustments-stock-adjustments-detail?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", "item-reports-stock-adjustments-stock-adjustments-detail")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("legacy-line adjustment report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode legacy-line adjustment report: %v", err)
	}
	if len(body.Rows) != 1 {
		t.Fatalf("legacy-line adjustment rows = %+v, want exactly 1 row", body.Rows)
	}
	row := body.Rows[0]
	if row.Item != "Stock Item / BATCH1" {
		t.Fatalf("legacy-line adjustment item = %q, want resolved master_items name (Stock Item / BATCH1), not the raw legacy code", row.Item)
	}
	if row.Party != "Main Godown / legacy-user" {
		t.Fatalf("legacy-line adjustment party = %q, want resolved master_godowns name (Main Godown / legacy-user), not the raw legacy code", row.Party)
	}
}

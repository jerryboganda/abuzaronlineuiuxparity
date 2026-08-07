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

func TestHistoricalReportsReadRetainedSourceRowsWithinTenantBranch(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "historical-"+time.Now().Format("150405.000000"))
	other := seedStockTenant(t, ctx, database, "historical-other-"+time.Now().Format("150405.000000"))
	defer func() {
		for _, query := range []string{
			`DELETE FROM historical_item_changes WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM historical_stock_adjustment_lines WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM stock_batches WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM user_memberships WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM roles WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM users WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM counters WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM branches WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM tenants WHERE id IN ($1::uuid, $2::uuid)`,
		} {
			if _, cleanupErr := database.ExecContext(ctx, query, fixture.tenantID, other.tenantID); cleanupErr != nil {
				t.Errorf("cleanup historical fixtures: %v", cleanupErr)
				return
			}
		}
	}()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_item_changes (
			tenant_id, branch_id, report_kind, source_legacy_id, item_id,
			item_legacy_id, occurred_at, old_name, new_name, user_legacy_id,
			change_reason, source_table, source_table_row, payload
		) VALUES ($1::uuid, $2::uuid, 'item-name', 'log-1', $3::uuid, $4,
			'2026-08-06T10:00:00Z', 'Old Item', 'New Item', '42', 'rename',
			'ItemLog', 'log-1', '{"Name":"New Item"}'::jsonb),
		($5::uuid, $6::uuid, 'item-name', 'other-log', $7::uuid, $8,
			'2026-08-06T10:00:00Z', 'Other Old', 'Other New', '43', 'rename',
			'ItemLog', 'other-log', '{"Name":"Other New"}'::jsonb)`,
		fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID,
		other.tenantID, other.branchID, other.itemID, other.itemLegacyID); err != nil {
		t.Fatalf("seed historical item rows: %v", err)
	}
	var godownLegacyID string
	if err := database.QueryRowContext(ctx, `SELECT legacy_id FROM master_godowns WHERE id = $1::uuid`, fixture.godownID).Scan(&godownLegacyID); err != nil {
		t.Fatalf("read fixture godown legacy id: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_stock_adjustment_lines (
			tenant_id, branch_id, legacy_id, adjustment_legacy_id, item_id,
			item_legacy_id, godown_id, godown_legacy_id, batch, occurred_at,
			loose_quantity, price, average_price, sale_price, purchase_price,
			recent_purchase_price, pack_units, user_legacy_id, posted, remarks,
			source_table, source_table_row, payload
		) VALUES ($1::uuid, $2::uuid, 'adj-1', '100', $3::uuid, $4, $5::uuid,
			$6, 'B-1', '2026-08-06T11:00:00Z', 3.5, 10, 10, 12, 8, 8, 10,
			'42', true, 'count correction', 'AdjDetail', '100:1',
			'{"AdjCode":100,"LooseQty":3.5}'::jsonb)`,
		fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID,
		fixture.godownID, godownLegacyID); err != nil {
		t.Fatalf("seed historical adjustment row: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	server := &Server{database: database}
	for _, test := range []struct {
		kind, wantDocument, wantItem string
	}{
		{"item-reports-history-item-name-changes", "log-1", fixture.itemLegacyID},
		{"item-reports-stock-adjustments-stock-adjustments-detail", "100", fixture.itemLegacyID + " / B-1"},
	} {
		request := readModelRequest(http.MethodGet, "/v1/reports/"+test.kind+"?from=2026-08-06&to=2026-08-06", operator)
		request.SetPathValue("kind", test.kind)
		recorder := httptest.NewRecorder()
		server.report(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", test.kind, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Rows       []reportRow      `json:"rows"`
			Definition reportDefinition `json:"definition"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("%s decode: %v", test.kind, err)
		}
		if len(body.Rows) != 1 || body.Rows[0].Document != test.wantDocument || body.Rows[0].Item != test.wantItem {
			t.Fatalf("%s rows = %+v, want one tenant-scoped source row", test.kind, body.Rows)
		}
		if body.Definition.ProjectionStatus != "real" || body.Definition.ProjectionNote == "" {
			t.Fatalf("%s definition did not disclose normalized source-backed status: %+v", test.kind, body.Definition)
		}
	}
}

func TestHistoricalStockAdjustmentReportIncludesNormalizedLedgerRows(t *testing.T) {
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
	fixture := seedStockTenant(t, ctx, database, "historical-adjustment-ledger-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()
	event := insertInventoryEvent(t, ctx, database, fixture, "inventory", "normalized-adjustment-report", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID,
		GodownID:     fixture.godownID,
		BatchNumber:  "EXPIRED",
		Quantity:     json.RawMessage(`2`),
		Direction:    "adjustment",
		UnitCost:     "4.00",
	})
	var batchID string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text FROM stock_batches
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3 AND batch_number = 'EXPIRED'
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&batchID); err != nil {
		t.Fatalf("read adjustment batch: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO stock_balances
			(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, 5)
	`, fixture.tenantID, fixture.branchID, batchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID); err != nil {
		t.Fatalf("seed adjustment balance: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO stock_ledger
			(tenant_id, branch_id, batch_id, source_event_id, source_line_key, direction,
			 adjustment_sign, quantity, unit_cost, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'adjustment-report-1',
		        'adjustment', -1, 2, 4.00, '2026-08-06T10:00:00Z')
	`, fixture.tenantID, fixture.branchID, batchID, event.EventID); err != nil {
		t.Fatalf("seed normalized adjustment ledger: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/item-reports-stock-adjustments-stock-adjustments-detail?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", "item-reports-stock-adjustments-stock-adjustments-detail")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("normalized adjustment report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode normalized adjustment report: %v", err)
	}
	if len(body.Rows) != 1 || body.Rows[0].Item != "Stock Item / EXPIRED" ||
		!reportNumericEqual(body.Rows[0].Quantity, "-2") || !reportNumericEqual(body.Rows[0].Amount, "4") {
		t.Fatalf("normalized adjustment rows = %+v, want signed stock-ledger row", body.Rows)
	}
}

func TestHistoricalStockBackDateReportUsesImportedStockReportFields(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "historical-stock-"+time.Now().Format("150405.000000"))
	other := seedStockTenant(t, ctx, database, "historical-stock-other-"+time.Now().Format("150405.000000"))
	defer func() {
		for _, query := range []string{
			`DELETE FROM historical_stock_snapshots WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM stock_batches WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM user_memberships WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM roles WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM users WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM counters WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM branches WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM tenants WHERE id IN ($1::uuid, $2::uuid)`,
		} {
			if _, cleanupErr := database.ExecContext(ctx, query, fixture.tenantID, other.tenantID); cleanupErr != nil {
				t.Errorf("cleanup historical stock fixtures: %v", cleanupErr)
				return
			}
		}
	}()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_stock_snapshots
			(tenant_id, branch_id, legacy_id, item_id, item_legacy_id, godown_id,
			 as_of, quantity, purchase_price, sale_price, average_price,
			 recent_purchase_price, pack_units, source_table, source_legacy_id, payload)
		VALUES
			($1::uuid, $2::uuid, 'stock-row-2026-08-06', $3::uuid, $4, $5::uuid,
			 '2026-08-06', 12.5000, 8.2500, 11.7500, 9.5000, 9.1000, 10,
			 'StockReport', 'stock-row-2026-08-06', '{"Date":"2026-08-06"}'::jsonb),
			($1::uuid, $2::uuid, 'stock-row-2026-08-05', $3::uuid, $4, $5::uuid,
			 '2026-08-05', 99.0000, 1.0000, 2.0000, 1.5000, 1.2500, 1,
			 'StockReport', 'stock-row-2026-08-05', '{"Date":"2026-08-05"}'::jsonb),
			($6::uuid, $7::uuid, 'other-stock-row-2026-08-06', $8::uuid, $9, $10::uuid,
			 '2026-08-06', 77.0000, 3.0000, 4.0000, 3.5000, 3.2500, 2,
			 'StockReport', 'other-stock-row-2026-08-06', '{"Date":"2026-08-06"}'::jsonb)
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID,
		other.tenantID, other.branchID, other.itemID, other.itemLegacyID, other.godownID); err != nil {
		t.Fatalf("seed historical stock snapshots: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/stock-in-hand-back-date?from=2026-08-06&to=2026-08-06&filter=Stock%20Item&godownId="+fixture.godownID, operator)
	request.SetPathValue("kind", "stock-in-hand-back-date")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("historical stock report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows       []reportRow      `json:"rows"`
		Definition reportDefinition `json:"definition"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode historical stock report: %v", err)
	}
	if len(body.Rows) != 1 {
		t.Fatalf("historical stock rows = %d, want one scoped as-of row: %+v", len(body.Rows), body.Rows)
	}
	row := body.Rows[0]
	if row.Document != "stock-row-2026-08-06" || row.OccurredAt != "2026-08-06" || row.Party != "Main Godown" ||
		row.Item != "Stock Item" || !reportNumericEqual(row.Quantity, "12.5") ||
		!reportNumericEqual(row.PurchasePrice, "8.25") || !reportNumericEqual(row.SalePrice, "11.75") ||
		!reportNumericEqual(row.AveragePrice, "9.5") || !reportNumericEqual(row.RecentPurchasePrice, "9.1") || row.PackUnits != "10" ||
		!reportNumericEqual(row.Amount, "9.5") {
		t.Fatalf("historical stock row = %+v, want captured StockReport fields", row)
	}
	if body.Definition.ProjectionStatus != "real" || !strings.Contains(body.Definition.ProjectionNote, "historical_stock_snapshots") {
		t.Fatalf("historical stock definition = %+v, want source disclosure", body.Definition)
	}
}

func TestStockLevelReportsUseMaintenanceThresholdFallbacks(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "stock-level-"+time.Now().Format("150405.000000"))
	defer func() {
		if _, cleanupErr := database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID); cleanupErr != nil {
			t.Errorf("cleanup stock-level fixture: %v", cleanupErr)
		}
	}()
	if _, err := database.ExecContext(ctx, `
		UPDATE master_items
		SET payload = '{"ReorderQuantity":"10","OptimumQuantity":"15","MinimumQuantity":"3"}'::jsonb
		WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, fixture.tenantID, fixture.itemID); err != nil {
		t.Fatalf("set stock-level thresholds: %v", err)
	}
	var batchID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO stock_batches
			(tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, 'LEVEL-001', '2030-01-01', 4.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID).Scan(&batchID); err != nil {
		t.Fatalf("seed stock-level batch: %v", err)
	}
	sign := 1
	event := insertInventoryEvent(t, ctx, database, fixture, "inventory", "stock-level-report", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "LEVEL-001", Quantity: json.RawMessage(`5`), Direction: "adjustment", AdjustmentSign: &sign,
	})
	if _, err := database.ExecContext(ctx, `
		INSERT INTO stock_ledger
			(tenant_id, branch_id, batch_id, source_event_id, source_line_key, direction,
			 adjustment_sign, quantity, unit_cost, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'row-0', 'adjustment', 1, 5.0000, 4.0000, '2026-08-06T00:00:00Z')
	`, fixture.tenantID, fixture.branchID, batchID, event.EventID); err != nil {
		t.Fatalf("seed stock-level ledger: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO stock_balances
			(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand, updated_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, 5.0000, '2026-08-06T00:00:00Z')
	`, fixture.tenantID, fixture.branchID, batchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID); err != nil {
		t.Fatalf("seed stock-level balance: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	for _, test := range []struct {
		kind      string
		wantRows  int
		wantValue string
	}{
		{kind: "reorder-level-report", wantRows: 1, wantValue: "10"},
		{kind: "optimum-level-report", wantRows: 1, wantValue: "15"},
		{kind: "minimum-level-report", wantRows: 0},
		{kind: "reorder-optimum-level-report", wantRows: 1, wantValue: "15"},
	} {
		t.Run(test.kind, func(t *testing.T) {
			request := readModelRequest(http.MethodGet, "/v1/reports/"+test.kind+"?from=2026-08-06&to=2026-08-06&godownId="+fixture.godownID, operator)
			request.SetPathValue("kind", test.kind)
			recorder := httptest.NewRecorder()
			(&Server{database: database}).report(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("stock-level report status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var body struct {
				Rows []reportRow `json:"rows"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatalf("decode stock-level report: %v", err)
			}
			if len(body.Rows) != test.wantRows {
				t.Fatalf("stock-level rows = %d, want %d: %+v", len(body.Rows), test.wantRows, body.Rows)
			}
			if test.wantRows == 1 {
				row := body.Rows[0]
				if row.Document != "LEVEL-001" || row.Party != "Main Godown" || row.Item != "Stock Item" ||
					!reportNumericEqual(row.Quantity, "5") || !reportNumericEqual(row.ReorderQuantity, "10") ||
					!reportNumericEqual(row.OptimumQuantity, "15") || !reportNumericEqual(row.MinimumQuantity, "3") {
					t.Fatalf("stock-level row = %+v, want maintenance fallback thresholds", row)
				}
				if test.kind == "reorder-level-report" && !reportNumericEqual(row.ReorderQuantity, test.wantValue) {
					t.Fatalf("reorder quantity = %s, want %s", row.ReorderQuantity, test.wantValue)
				}
			}
		})
	}
}

func TestHistoricalGLJournalReportUsesImportedVirtualGLFields(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "historical-gl-"+time.Now().Format("150405.000000"))
	other := seedStockTenant(t, ctx, database, "historical-gl-other-"+time.Now().Format("150405.000000"))
	defer func() {
		for _, query := range []string{
			`DELETE FROM historical_gl_entries WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM stock_batches WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM user_memberships WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM roles WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM users WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM counters WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM branches WHERE tenant_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM tenants WHERE id IN ($1::uuid, $2::uuid)`,
		} {
			if _, cleanupErr := database.ExecContext(ctx, query, fixture.tenantID, other.tenantID); cleanupErr != nil {
				t.Errorf("cleanup historical GL fixtures: %v", cleanupErr)
				return
			}
		}
	}()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_gl_entries (
			tenant_id, branch_id, legacy_id, document_code, document_type,
			account_code, alternate_account_code, debit_amount, credit_amount,
			occurred_at, user_legacy_id, invoice_code, remarks, payload
		) VALUES
			($1::uuid, $2::uuid, 'legacy-gl-row-1', '880233', 'SV', '2', '19',
			 420.0000, 0.0000, '2026-08-06T10:00:00+05', '4', '880233',
			 'Cash receipt on sale #: 880233', '{}'::jsonb),
			($1::uuid, $2::uuid, 'legacy-gl-old', '880233-old', 'SV', '2', '19',
			 1.0000, 0.0000, '2026-08-05T10:00:00+05', '4', '880233-old',
			 'Older row excluded by date', '{}'::jsonb),
			($3::uuid, $4::uuid, 'other-gl-row-1', '880233', 'SV', '2', '19',
			 999.0000, 0.0000, '2026-08-06T10:00:00+05', '9', '880233',
			 'Other tenant excluded by scope', '{}'::jsonb)
	`, fixture.tenantID, fixture.branchID, other.tenantID, other.branchID); err != nil {
		t.Fatalf("seed historical GL entries: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/gl-journal?from=2026-08-06&to=2026-08-06&filter=880233", operator)
	request.SetPathValue("kind", "gl-journal")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("historical GL report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows       []reportRow      `json:"rows"`
		Definition reportDefinition `json:"definition"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode historical GL report: %v", err)
	}
	if len(body.Rows) != 1 {
		t.Fatalf("historical GL rows = %d, want one scoped date row: %+v", len(body.Rows), body.Rows)
	}
	row := body.Rows[0]
	if row.Document != "legacy-gl-row-1" || !strings.Contains(row.OccurredAt, "2026-08-06") || row.DocumentType != "SV" ||
		row.Party != "2" || row.AlternateAccountCode != "19" || row.InvoiceCode != "880233" || row.UserLegacyID != "4" ||
		row.Item != "Cash receipt on sale #: 880233" || !reportNumericEqual(row.Quantity, "420") || !reportNumericEqual(row.Amount, "0") {
		t.Fatalf("historical GL row = %+v, want captured VirtualGl fields", row)
	}
	if body.Definition.ProjectionStatus != "real" || !strings.Contains(body.Definition.ProjectionNote, "historical_gl_entries") ||
		len(body.Definition.Columns) != 10 {
		t.Fatalf("historical GL definition = %+v, want source disclosure and ten columns", body.Definition)
	}
}

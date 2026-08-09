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

// TestProfitMarginDetailReportUsesStockLedgerCogsFallback is the regression
// test for the gap documented in
// docs/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md and
// docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_B_2026-08-09.md:
// salesProfitMarginReadModelQuery (backing both
// "customer-sales-invoice-wise-profit-margin-detail" and
// "customer-sales-customer-category-wise-sales-customer-wise-gross-profit")
// read cost exclusively from stock_allocations, which is confirmed empty for
// the sandbox tenant (and database-wide at real scale), so every row's
// cost/gross-profit/margin columns came back blank.
//
// stock_ledger (unit_cost + source_document_line_id, filtered
// direction='out') is the same populated cost source that already backs the
// Phase J moving-average stock valuation engine
// (services/api/internal/httpapi/stock.go, weightedAverageStockCost) and
// resolves COGS for 620,613/620,615 (99.9997%) posted sale lines at real
// scale. The fix adds a stock_ledger-based COGS join as the primary cost
// source, falling back to stock_allocations only when the ledger has no
// matching row for that line.
//
// This test posts a real sale (so both stock_ledger and stock_allocations
// are populated the normal way), then deletes the stock_allocations row to
// reproduce the real-world condition ("stock_allocations empty"), and
// asserts the profit-margin-detail report still reports the correct
// cost/gross-profit/margin from stock_ledger. Before the fix this would have
// reported blank cost/grossProfit/marginPercent for the row.
func TestProfitMarginDetailReportUsesStockLedgerCogsFallback(t *testing.T) {
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

	suffix := "cogs-fix-" + time.Now().Format("150405.000000")
	fixture := seedStockTenant(t, ctx, database, suffix)
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}

	// Receive 10 units at a known cost of 5.00 each.
	receive := insertInventoryEventAt(t, ctx, database, fixture, "receiving", "cogs-fix-receive", "2026-08-06T00:00:00Z", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "COGS-001", ExpiryDate: "2030-01-01", Quantity: json.RawMessage(`10`), UnitCost: "5.00",
	})
	{
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
	}

	// Post a cash-sale of 3 units at 10.00 each; this populates both
	// stock_ledger (direction='out') and stock_allocations the normal way.
	server := &Server{database: database}
	command := stockDocumentCommand(fixture, "save-and-post", "cogs-fix-sale", "3")
	tx, err := server.beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin sale: %v", err)
	}
	_, response, err := server.saveBusinessDocument(ctx, tx, operator, command)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("save-and-post sale: %v", err)
	}
	eventID := insertEventInTransaction(t, ctx, tx, operator, "business_document", response.Document.ID, command.IdempotencyKey)
	if err := projectPostedSaleStock(ctx, tx, operator, command.Document, response.Document, eventID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("allocate sale stock: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit sale: %v", err)
	}

	// Sanity check: stock_ledger has the expected outbound cost row.
	var ledgerCost string
	if err := database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(sl.quantity * sl.unit_cost), 0)::numeric(19,4)::text
		FROM stock_ledger sl
		WHERE sl.tenant_id = $1::uuid AND sl.branch_id = $2::uuid
		  AND sl.direction = 'out' AND sl.source_document_id = $3::uuid
	`, fixture.tenantID, fixture.branchID, response.Document.ID).Scan(&ledgerCost); err != nil {
		t.Fatalf("read stock_ledger cost: %v", err)
	}
	if ledgerCost != "15.0000" {
		t.Fatalf("stock_ledger cost = %s, want 15.0000 (3 * 5.00)", ledgerCost)
	}

	// Reproduce the real-world condition documented in the Phase N golden
	// verification docs: stock_allocations has no matching row for this
	// document's line. If the report still depended exclusively on
	// stock_allocations, deleting this row would make cost/gross-profit/
	// margin blank again.
	if _, err := database.ExecContext(ctx, `
		DELETE FROM stock_allocations
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND document_id = $3::uuid
	`, fixture.tenantID, fixture.branchID, response.Document.ID); err != nil {
		t.Fatalf("delete stock_allocations to simulate empty allocations: %v", err)
	}
	var remainingAllocations int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM stock_allocations
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND document_id = $3::uuid
	`, fixture.tenantID, fixture.branchID, response.Document.ID).Scan(&remainingAllocations); err != nil {
		t.Fatalf("count remaining stock_allocations: %v", err)
	}
	if remainingAllocations != 0 {
		t.Fatalf("expected stock_allocations to be empty for this document, found %d rows", remainingAllocations)
	}

	request := readModelRequest(http.MethodGet, "/v1/reports/customer-sales-invoice-wise-profit-margin-detail?from=2026-08-06&to=2026-08-06&pageSize=50", operator)
	request.SetPathValue("kind", "customer-sales-invoice-wise-profit-margin-detail")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("profit-margin-detail report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode profit-margin-detail report: %v", err)
	}

	var row *reportRow
	for i := range body.Rows {
		if body.Rows[i].Document == response.Document.DocumentNumber {
			row = &body.Rows[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("profit-margin-detail report did not return the posted sale (document %s): %+v", response.Document.DocumentNumber, body.Rows)
	}

	// The core regression assertion: with stock_allocations empty for this
	// line, cost must still be populated from stock_ledger, not blank.
	if row.PurchasePrice != "15.0000" {
		t.Fatalf("profit-margin-detail cost = %q, want %q (stock_ledger fallback: 3 * 5.00)", row.PurchasePrice, "15.0000")
	}
	if row.GrossProfit == "" {
		t.Fatal("profit-margin-detail gross profit is blank; want it populated now that cost is sourced from stock_ledger")
	}
	if row.MarginPercent == "" {
		t.Fatal("profit-margin-detail margin percent is blank; want it populated now that cost is sourced from stock_ledger")
	}

	amount, err := parseDecimalForTest(row.Amount)
	if err != nil {
		t.Fatalf("parse amount %q: %v", row.Amount, err)
	}
	tax, err := parseDecimalForTest(row.SalesTaxValue)
	if err != nil {
		t.Fatalf("parse sales tax %q: %v", row.SalesTaxValue, err)
	}
	cost, err := parseDecimalForTest(row.PurchasePrice)
	if err != nil {
		t.Fatalf("parse cost %q: %v", row.PurchasePrice, err)
	}
	grossProfit, err := parseDecimalForTest(row.GrossProfit)
	if err != nil {
		t.Fatalf("parse gross profit %q: %v", row.GrossProfit, err)
	}
	wantGrossProfit := amount - tax - cost
	if diff := grossProfit - wantGrossProfit; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("gross profit = %v, want %v (amount %v - tax %v - cost %v)", grossProfit, wantGrossProfit, amount, tax, cost)
	}
}

func parseDecimalForTest(value string) (float64, error) {
	var result float64
	_, err := fmt.Sscan(value, &result)
	return result, err
}

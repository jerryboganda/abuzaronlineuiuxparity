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

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestAdjustmentSummaryModesGroupCorrectly is a regression test for the
// previously-deferred grouping gap (Phase Q Finding B,
// docs/PHASE_Q_GOLDEN_VERIFICATION_ADJUSTMENT_HISTORY_2026-08-09.md, and the
// comment on stockReadModelQuery's "adjustment" mode family, reports.go).
//
// Two of the six adjustment-* leaves now get real aggregation:
//
//   - adjustment-adjustment-summary (stockMode "adjustment-summary"): grouped
//     by day, adjustment type (Increase/Decrease from adjustment_sign),
//     godown, and item.
//   - adjustment-item-wise-adjustment-summary (stockMode
//     "adjustment-item-summary"): grouped by item, godown, and day.
//
// The remaining three leaves (adjustment-adjustment-summary-inv-wise,
// adjustment-adjustment-detail-inv-wise, adjustment-adjustment-summary-detail)
// stay on the ungrouped row-level "adjustment" query -- documented, not
// fabricated, because stock_ledger adjustment rows carry no
// adjustment-document/reference identity to group on.
//
// This test seeds two items across two godown/day/sign combinations and
// proves: (1) adjustment-summary collapses same-day/type/godown/item rows
// into one SUM'd row per (type, item) and keeps different types/items apart;
// (2) item-wise-adjustment-summary further collapses across type into one
// SUM'd row per item; and (3) the three still-ungrouped leaves keep returning
// one row per underlying stock_ledger adjustment row (no accidental
// aggregation was introduced there).
func TestAdjustmentSummaryModesGroupCorrectly(t *testing.T) {
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

	suffix := "adjustment-grouping-" + time.Now().Format("150405.000000")
	fixture := seedStockTenant(t, ctx, database, suffix)
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	// A second item/batch in the same godown, to prove item-wise grouping
	// keeps distinct items apart instead of collapsing everything into one
	// row.
	secondItemLegacyID := "item-b-" + suffix
	var secondItemID, secondBatchID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_items (tenant_id, legacy_id, code, name, payload)
		VALUES ($1::uuid, $2, $2, 'Stock Item B', '{"SalePrice1":"10.00"}')
		RETURNING id::text
	`, fixture.tenantID, secondItemLegacyID).Scan(&secondItemID); err != nil {
		t.Fatalf("seed second item: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		INSERT INTO stock_batches (tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, 'B-BATCH', CURRENT_DATE - 1, 2.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, secondItemID, secondItemLegacyID, fixture.godownID).Scan(&secondBatchID); err != nil {
		t.Fatalf("seed second batch: %v", err)
	}

	var firstBatchID string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text FROM stock_batches
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3 AND batch_number = 'EXPIRED'
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&firstBatchID); err != nil {
		t.Fatalf("read first fixture batch: %v", err)
	}

	adjEvent := insertInventoryEvent(t, ctx, database, fixture, "inventory", "adjustment-grouping-event", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID, BatchNumber: "EXPIRED",
		Quantity: json.RawMessage(`1`), Direction: "adjustment", UnitCost: "4.00",
	})

	seedLedgerRow := func(batchID, key, direction string, sign int, quantity, unitCost, occurredAt string) {
		t.Helper()
		if _, err := database.ExecContext(ctx, `
			INSERT INTO stock_ledger
				(tenant_id, branch_id, batch_id, source_event_id, source_line_key, direction,
				 adjustment_sign, quantity, unit_cost, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8::numeric, $9::numeric, $10::timestamptz)
		`, fixture.tenantID, fixture.branchID, batchID, adjEvent.EventID, key, direction, sign, quantity, unitCost, occurredAt); err != nil {
			t.Fatalf("seed stock_ledger row %s: %v", key, err)
		}
	}

	// Item A: two same-day INCREASE rows (must SUM into one row: 3 + 2 = 5
	// quantity, 12.0000 + 8.0000 = 20.0000 value) and one DECREASE row
	// (-1 quantity, -4.0000 value), all unit_cost 4.00.
	seedLedgerRow(firstBatchID, "adjustment-grouping-a-plus-1", "adjustment", 1, "3", "4.00", "2026-08-06T09:00:00Z")
	seedLedgerRow(firstBatchID, "adjustment-grouping-a-plus-2", "adjustment", 1, "2", "4.00", "2026-08-06T09:30:00Z")
	seedLedgerRow(firstBatchID, "adjustment-grouping-a-minus", "adjustment", -1, "1", "4.00", "2026-08-06T10:00:00Z")
	// A sale/purchase-shaped "in" row that every adjustment mode must exclude.
	seedLedgerRow(firstBatchID, "adjustment-grouping-a-in", "in", 1, "10", "5.00", "2026-08-06T11:00:00Z")
	// Item B: single INCREASE row, unit_cost 2.00 (7 quantity, 14.0000 value).
	seedLedgerRow(secondBatchID, "adjustment-grouping-b-plus", "adjustment", 1, "7", "2.00", "2026-08-06T09:15:00Z")

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	server := &Server{database: database}

	fetch := func(kind string) []reportRow {
		t.Helper()
		request := readModelRequest(http.MethodGet, "/v1/reports/"+kind+"?from=2026-08-06&to=2026-08-06", operator)
		request.SetPathValue("kind", kind)
		recorder := httptest.NewRecorder()
		server.report(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", kind, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Rows []reportRow `json:"rows"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s decode: %v, body = %s", kind, err, recorder.Body.String())
		}
		return body.Rows
	}

	// (1) adjustment-adjustment-summary: grouped by day/type/godown/item, so
	// 3 rows are expected -- item A INCREASE (5 / 20.0000), item A DECREASE
	// (-1 / -4.0000), item B INCREASE (7 / 14.0000).
	summaryRows := fetch("adjustment-adjustment-summary")
	if len(summaryRows) != 3 {
		t.Fatalf("adjustment-adjustment-summary rows = %+v, want 3 grouped rows", summaryRows)
	}
	// Columns for this mode are (document=date, occurredAt=adjustment type,
	// party=godown, item, quantity, amount) per stockReportColumns'
	// isStockAdjustmentSummaryMode branch.
	foundAIncrease, foundADecrease, foundBIncrease := false, false, false
	for _, row := range summaryRows {
		switch {
		case row.Item == "Stock Item" && row.OccurredAt == "INCREASE":
			foundAIncrease = true
			if !reportNumericEqual(row.Quantity, "5") || !reportNumericEqual(row.Amount, "20.0000") {
				t.Errorf("item A INCREASE row = %+v, want quantity 5 / amount 20.0000", row)
			}
		case row.Item == "Stock Item" && row.OccurredAt == "DECREASE":
			foundADecrease = true
			if !reportNumericEqual(row.Quantity, "-1") || !reportNumericEqual(row.Amount, "-4.0000") {
				t.Errorf("item A DECREASE row = %+v, want quantity -1 / amount -4.0000", row)
			}
		case row.Item == "Stock Item B" && row.OccurredAt == "INCREASE":
			foundBIncrease = true
			if !reportNumericEqual(row.Quantity, "7") || !reportNumericEqual(row.Amount, "14.0000") {
				t.Errorf("item B INCREASE row = %+v, want quantity 7 / amount 14.0000", row)
			}
		}
	}
	if !foundAIncrease || !foundADecrease || !foundBIncrease {
		t.Fatalf("adjustment-adjustment-summary rows = %+v, missing expected grouped rows (foundAIncrease=%v foundADecrease=%v foundBIncrease=%v)", summaryRows, foundAIncrease, foundADecrease, foundBIncrease)
	}

	// (2) adjustment-item-wise-adjustment-summary: grouped by item/godown/day
	// only (collapsing across type), so 2 rows are expected -- item A net
	// (5 - 1 = 4 / 20.0000 - 4.0000 = 16.0000), item B (7 / 14.0000).
	itemSummaryRows := fetch("adjustment-item-wise-adjustment-summary")
	if len(itemSummaryRows) != 2 {
		t.Fatalf("adjustment-item-wise-adjustment-summary rows = %+v, want 2 grouped rows", itemSummaryRows)
	}
	foundA, foundB := false, false
	for _, row := range itemSummaryRows {
		switch row.Item {
		case "Stock Item":
			foundA = true
			if !reportNumericEqual(row.Quantity, "4") || !reportNumericEqual(row.Amount, "16.0000") {
				t.Errorf("item A summary row = %+v, want net quantity 4 / net amount 16.0000", row)
			}
		case "Stock Item B":
			foundB = true
			if !reportNumericEqual(row.Quantity, "7") || !reportNumericEqual(row.Amount, "14.0000") {
				t.Errorf("item B summary row = %+v, want net quantity 7 / net amount 14.0000", row)
			}
		}
	}
	if !foundA || !foundB {
		t.Fatalf("adjustment-item-wise-adjustment-summary rows = %+v, missing expected grouped rows (foundA=%v foundB=%v)", itemSummaryRows, foundA, foundB)
	}

	// (3) The three leaves left on the documented ungrouped fallback must
	// still return one row per underlying stock_ledger adjustment row (4
	// rows: 2 item-A increases + 1 item-A decrease + 1 item-B increase),
	// proving no unintended aggregation was introduced there.
	for _, kind := range []string{
		"adjustment-adjustment-summary-inv-wise",
		"adjustment-adjustment-detail-inv-wise",
		"adjustment-adjustment-summary-detail",
		"adjustment-adjustment-detail",
	} {
		rows := fetch(kind)
		if len(rows) != 4 {
			t.Errorf("%s rows = %+v, want 4 ungrouped rows (documented fallback, not summary/item-summary)", kind, rows)
		}
	}
}

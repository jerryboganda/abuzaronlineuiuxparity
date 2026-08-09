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

// TestAdjustmentReportLeavesReturnNormalizedRowsAfterParameterGapFix is a
// Phase Q golden-verification regression test for the six
// stockMode="adjustment" leaves (reports.go phaseQReportRegistry, lines
// ~437-453):
//
//	adjustment-adjustment-summary, adjustment-adjustment-detail,
//	adjustment-adjustment-summary-inv-wise, adjustment-adjustment-detail-inv-wise,
//	adjustment-adjustment-summary-detail, adjustment-item-wise-adjustment-summary
//
// PHASE Q FINDING A (fixed): stockReadModelQuery's mode=="adjustment" branch
// (reports.go ~line 3964-3990) used to never reference placeholders $6 or $7
// in its SQL text, while the sole call site (reports.go ~line 5125-5126)
// always binds 9 positional arguments (tenantID, branchID, from, to, filter,
// godownID, batchNumber, pageSize+1, offset) against a pagination footer of
// "LIMIT $8 OFFSET $9". PostgreSQL's extended query protocol cannot infer a
// type for a parameter that never appears in the query text, so every single
// invocation of any of the six adjustment-* leaves failed at the database
// with SQLSTATE 42P18 ("could not determine data type of parameter $6"),
// surfaced by the handler as HTTP 503 report_read_failed. The fix adds the
// same "($6 = '' OR b.godown_id = $6::uuid) AND ($7 = '' OR b.batch_number
// ILIKE ...)" godown/batch-number predicates the sibling `movement` /
// `narcotics-movement` branch already uses, so $6/$7 are now referenced and
// inferable. See
// docs/PHASE_Q_GOLDEN_VERIFICATION_ADJUSTMENT_HISTORY_2026-08-09.md.
//
// Finding B (all six leaves resolving to the same ungrouped row-level query,
// with no summary/detail/invoice-wise/item-wise differentiation) has since
// been partially closed: adjustment-adjustment-summary and
// adjustment-item-wise-adjustment-summary now use real grouped queries (see
// stockReadModelQuery's isStockAdjustmentSummaryMode/
// isStockAdjustmentItemSummaryMode branches, and
// TestAdjustmentSummaryModesGroupCorrectly in
// report_fix_adjustment_grouping_test.go for grouping-specific assertions).
// The remaining four leaves stay on the shared ungrouped "adjustment" query,
// documented there. This test only asserts that all six leaves return a
// correct, error-free result instead of a 503; for the two now-grouped
// leaves it seeds a single adjustment row per item so the grouped and
// ungrouped shapes coincide (one summary row == one detail row) and the
// original [-1, 3] two-row assertion below still applies to all six.
//
// This test proves the fix two ways: (1) replaying the exact query text
// directly against Postgres and asserting it now succeeds and returns the
// objectively correct signed/ordered rows; and (2) driving all six report
// leaves through the real s.report handler and asserting 200 OK with those
// same rows. Both are checked against the independent recompute of [-1, 3]
// (occurred_at DESC, the sale/purchase-shaped "in" row excluded) so this test
// still fails loudly if a future change reintroduces the parameter gap or
// breaks the adjustment filter/ordering.
func TestAdjustmentReportLeavesReturnNormalizedRowsAfterParameterGapFix(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "adjustment-golden-"+time.Now().Format("150405.000000"))
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	adjEvent := insertInventoryEvent(t, ctx, database, fixture, "inventory", "adjustment-golden-adj-event", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID, BatchNumber: "EXPIRED",
		Quantity: json.RawMessage(`3`), Direction: "adjustment", UnitCost: "4.00",
	})
	purchaseEvent := insertInventoryEvent(t, ctx, database, fixture, "inventory", "adjustment-golden-in-event", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID, BatchNumber: "EXPIRED",
		Quantity: json.RawMessage(`10`), Direction: "in", UnitCost: "5.00",
	})

	var batchID string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text FROM stock_batches
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3 AND batch_number = 'EXPIRED'
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&batchID); err != nil {
		t.Fatalf("read fixture batch: %v", err)
	}

	seedLedgerRow := func(key, direction string, sign int, quantity, unitCost, occurredAt, eventID string) {
		t.Helper()
		if _, err := database.ExecContext(ctx, `
			INSERT INTO stock_ledger
				(tenant_id, branch_id, batch_id, source_event_id, source_line_key, direction,
				 adjustment_sign, quantity, unit_cost, occurred_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8::numeric, $9::numeric, $10::timestamptz)
		`, fixture.tenantID, fixture.branchID, batchID, eventID, key, direction, sign, quantity, unitCost, occurredAt); err != nil {
			t.Fatalf("seed stock_ledger row %s: %v", key, err)
		}
	}
	// Manual adjustment, quantity increased by 3, earlier in the day.
	seedLedgerRow("adjustment-golden-plus", "adjustment", 1, "3", "4.00", "2026-08-06T09:00:00Z", adjEvent.EventID)
	// Manual adjustment, quantity reduced by 1, later in the day.
	seedLedgerRow("adjustment-golden-minus", "adjustment", -1, "1", "4.00", "2026-08-06T10:00:00Z", adjEvent.EventID)
	// A sale/purchase-shaped "in" movement that a correct query must exclude.
	seedLedgerRow("adjustment-golden-purchase", "in", 1, "10", "5.00", "2026-08-06T11:00:00Z", purchaseEvent.EventID)

	// (1) Independent recompute: a hand-written query, distinct from
	// stockReadModelQuery's "adjustment" branch, establishing what the
	// correct isolated/signed adjustment rows are from first principles.
	independentRows, err := database.QueryContext(ctx, `
		SELECT (l.quantity * l.adjustment_sign)::text
		FROM stock_ledger l
		WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid AND l.batch_id = $3::uuid
		  AND l.direction = 'adjustment'
		ORDER BY l.occurred_at DESC
	`, fixture.tenantID, fixture.branchID, batchID)
	if err != nil {
		t.Fatalf("independent recompute query: %v", err)
	}
	defer independentRows.Close()
	var independentQuantities []string
	for independentRows.Next() {
		var quantity string
		if err := independentRows.Scan(&quantity); err != nil {
			t.Fatalf("scan independent row: %v", err)
		}
		independentQuantities = append(independentQuantities, quantity)
	}
	if len(independentQuantities) != 2 || !reportNumericEqual(independentQuantities[0], "-1") || !reportNumericEqual(independentQuantities[1], "3") {
		t.Fatalf("independent recompute quantities = %v, want [-1, 3] (occurred_at DESC, purchase row excluded) -- this is the correct target once the parameter gap below is fixed", independentQuantities)
	}

	// (2) Replay the exact (fixed) query text from stockReadModelQuery's
	// mode=="adjustment" branch (reports.go ~line 3964-3990), bound the same
	// way the handler binds it (9 positional args, pagination "LIMIT $8
	// OFFSET $9"), directly against Postgres -- independent of the HTTP
	// handler -- and confirm it now succeeds (no more SQLSTATE 42P18) and
	// returns the objectively correct signed/ordered rows established by the
	// independent recompute above.
	verbatimAdjustmentQuery := `
		SELECT l.id::text, l.occurred_at::text, l.direction,
		       COALESCE(NULLIF(i.name, ''), b.item_legacy_id),
		       (l.quantity * l.adjustment_sign)::text, l.unit_cost::text
		FROM stock_ledger l
		JOIN sync_events se
		  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
		JOIN stock_batches b
		  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id AND b.id = l.batch_id
		LEFT JOIN master_items i
		  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
		WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
		  AND l.direction = 'adjustment'
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND l.occurred_at >= $3::date
		  AND l.occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR l.id::text ILIKE '%' || $5 || '%'
		       OR b.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%')
		  AND ($6 = '' OR b.godown_id = $6::uuid)
		  AND ($7 = '' OR b.batch_number ILIKE '%' || $7 || '%')
		ORDER BY l.occurred_at DESC, l.id
		LIMIT $8 OFFSET $9`
	verbatimRows, queryErr := database.QueryContext(ctx, verbatimAdjustmentQuery,
		fixture.tenantID, fixture.branchID, "1900-01-01", "2999-12-31", "", "", "", 51, 0)
	if queryErr != nil {
		t.Fatalf("verbatim adjustment query failed (want success now that $6/$7 are referenced): %v", queryErr)
	}
	defer verbatimRows.Close()
	var verbatimQuantities []string
	for verbatimRows.Next() {
		var id, occurredAt, direction, item, quantity, unitCost string
		if err := verbatimRows.Scan(&id, &occurredAt, &direction, &item, &quantity, &unitCost); err != nil {
			t.Fatalf("scan verbatim row: %v", err)
		}
		verbatimQuantities = append(verbatimQuantities, quantity)
	}
	if err := verbatimRows.Err(); err != nil {
		t.Fatalf("iterate verbatim rows: %v", err)
	}
	if len(verbatimQuantities) != 2 || !reportNumericEqual(verbatimQuantities[0], "-1") || !reportNumericEqual(verbatimQuantities[1], "3") {
		t.Fatalf("verbatim adjustment query quantities = %v, want [-1, 3] (occurred_at DESC, purchase row excluded)", verbatimQuantities)
	}

	// (3) Confirm the fix surfaces through every one of the six
	// differently-titled leaves via the real HTTP handler, proving this is
	// not specific to one leaf's title/columns but to the single shared
	// query branch all six leaves resolve to. All six currently share the
	// same ungrouped row-level projection (Finding B, documented follow-up),
	// so every leaf is expected to return the identical two rows.
	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	server := &Server{database: database}
	leaves := []string{
		"adjustment-adjustment-summary",
		"adjustment-adjustment-detail",
		"adjustment-adjustment-summary-inv-wise",
		"adjustment-adjustment-detail-inv-wise",
		"adjustment-adjustment-summary-detail",
		"adjustment-item-wise-adjustment-summary",
	}
	for _, kind := range leaves {
		request := readModelRequest(http.MethodGet, "/v1/reports/"+kind+"?from=2026-08-06&to=2026-08-06", operator)
		request.SetPathValue("kind", kind)
		recorder := httptest.NewRecorder()
		server.report(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s status = %d, body = %s; want 200 OK now that the parameter gap is fixed", kind, recorder.Code, recorder.Body.String())
			continue
		}
		var body struct {
			Rows []reportRow `json:"rows"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Errorf("%s decode: %v, body = %s", kind, err, recorder.Body.String())
			continue
		}
		if kind == "adjustment-item-wise-adjustment-summary" {
			// This leaf is grouped by item/godown/day (see
			// isStockAdjustmentItemSummaryMode), so the two same-item,
			// same-day rows collapse into one net row: 3 - 1 = 2 quantity,
			// (3*4.00) + (-1*4.00) = 8.0000 value.
			if len(body.Rows) != 1 {
				t.Errorf("%s rows = %+v, want exactly 1 grouped row (item/day summary collapses the 2 seeded rows)", kind, body.Rows)
				continue
			}
			if !reportNumericEqual(body.Rows[0].Quantity, "2") || !reportNumericEqual(body.Rows[0].Amount, "8.0000") {
				t.Errorf("%s row = %+v, want net quantity 2 / net amount 8.0000", kind, body.Rows[0])
			}
			continue
		}
		if len(body.Rows) != 2 {
			t.Errorf("%s rows = %+v, want exactly the 2 seeded adjustment rows", kind, body.Rows)
			continue
		}
		if !reportNumericEqual(body.Rows[0].Quantity, "-1") || !reportNumericEqual(body.Rows[1].Quantity, "3") {
			t.Errorf("%s quantities = [%q, %q], want [-1, 3] (occurred_at DESC, purchase row excluded)", kind, body.Rows[0].Quantity, body.Rows[1].Quantity)
		}
	}
}

// TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn is a Phase
// Q golden-verification test for the item-reports-history-sale-price-difference
// and item-reports-history-item-sale-price-changes leaves (historyMode
// "item-price-difference" / "item-sale-price").
//
// PHASE Q FINDING (documented, not fixed here): historicalReportQuery's
// default branch (reports.go ~line 4725-4730) computes the Previous/Current
// columns for these two report kinds as
// COALESCE(h.old_sale_price::text, h.old_name, '') /
// COALESCE(h.new_sale_price::text, h.new_name, ''). old_sale_price is a
// nullable numeric(19,4) column and old_name is an unrelated NOT NULL text
// column (the item's name at that point in the log). Whenever
// historical_item_changes.old_sale_price (or new_sale_price) is NULL --
// which the loader in migration/cmd/bulk-historical/main.go's
// importItemHistory allows whenever the previous ItemLog snapshot had a
// blank SalePrice while Name was still populated -- COALESCE silently falls
// through to the item's NAME string instead of leaving the price column
// blank. The "Previous"/"Current" columns are declared DataType "text" (not
// "currency") for this generic historyMode branch, so nothing at the
// transport layer catches an item name landing in what is supposed to be a
// price value. See
// docs/PHASE_Q_GOLDEN_VERIFICATION_ADJUSTMENT_HISTORY_2026-08-09.md.
func TestItemHistoryPriceDifferenceReportLeaksItemNameIntoPriceColumn(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "item-history-golden-"+time.Now().Format("150405.000000"))
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	// Mirrors a legacy ItemLog row where the previous snapshot had a
	// populated Name but a blank/NULL SalePrice -- old_sale_price is
	// genuinely NULL, old_name is a real (non-empty) item name.
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_item_changes (
			tenant_id, branch_id, report_kind, source_legacy_id, item_id,
			item_legacy_id, occurred_at, old_name, new_name, old_sale_price,
			new_sale_price, user_legacy_id, change_reason, source_table,
			source_table_row, payload
		) VALUES ($1::uuid, $2::uuid, 'item-price-difference', 'price-log-1', $3::uuid, $4,
			'2026-08-06T10:00:00Z', 'Panadol 500mg', 'Panadol 500mg', NULL,
			99.5000, '42', 'price update', 'ItemLog', 'price-log-1', '{}'::jsonb)
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed historical item price row: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/item-reports-history-sale-price-difference?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", "item-reports-history-sale-price-difference")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Rows) != 1 {
		t.Fatalf("rows = %+v, want exactly 1 seeded row", body.Rows)
	}
	// Fixed behavior: Quantity ("Previous") is blank, since old_sale_price
	// is NULL and there is no previous price to show. The COALESCE fallback
	// to h.old_name (the item's name) has been removed from
	// historicalReportQuery's default branch, so the item name no longer
	// leaks into the price column.
	if body.Rows[0].Quantity != "" {
		t.Fatalf("Previous price column = %q, want \"\" (old_sale_price is NULL; must not leak the item name \"Panadol 500mg\")", body.Rows[0].Quantity)
	}
	if body.Rows[0].Amount != "99.5000" {
		t.Fatalf("Current price column = %q, want \"99.5000\" (new_sale_price was non-NULL so this half is unaffected)", body.Rows[0].Amount)
	}
}

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

// TestItemHistoryPriceReportsNoLongerLeakItemName is a regression test for
// the Phase Q Finding D fix: historicalReportQuery's default branch
// (services/api/internal/httpapi/reports.go, historyMode "item-price-difference"
// and "item-sale-price") used to compute the "Previous"/"Current" columns as
//
//	COALESCE(h.old_sale_price::text, h.old_name, '')
//	COALESCE(h.new_sale_price::text, h.new_name, '')
//
// which coalesced a nullable numeric(19,4) price column with an unrelated
// NOT NULL text name column: whenever a price was NULL but the item name was
// populated, the price column silently rendered the item's NAME instead of
// blank. The fix drops the h.old_name/h.new_name fallback entirely for these
// two report kinds, so a NULL price now renders as an empty string.
//
// This test seeds one row per affected leaf (item-price-difference and
// item-sale-price) with old_sale_price NULL / new_sale_price NULL and a
// populated old_name/new_name, then asserts the price columns are blank, not
// the item name, through the real HTTP handler for both leaves.
func TestItemHistoryPriceReportsNoLongerLeakItemName(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "item-history-fix-"+time.Now().Format("150405.000000"))
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	cases := []struct {
		leaf       string
		reportKind string
		sourceID   string
	}{
		{"item-reports-history-sale-price-difference", "item-price-difference", "price-diff-fix-1"},
		{"item-reports-history-item-sale-price-changes", "item-sale-price", "sale-price-fix-1"},
	}

	for _, tc := range cases {
		// old_sale_price is NULL (no previous price on record) while
		// old_name/new_name are populated real item names -- exactly the
		// mismatched-type case that used to leak into the price column.
		// new_sale_price is populated so the "Current" side stays a genuine
		// price value, unaffected by this defect, in the same row.
		if _, err := database.ExecContext(ctx, `
			INSERT INTO historical_item_changes (
				tenant_id, branch_id, report_kind, source_legacy_id, item_id,
				item_legacy_id, occurred_at, old_name, new_name, old_sale_price,
				new_sale_price, user_legacy_id, change_reason, source_table,
				source_table_row, payload
			) VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6,
				'2026-08-06T10:00:00Z', 'Panadol 500mg', 'Panadol 500mg Forte', NULL,
				42.7500, '42', 'price update', 'ItemLog', $4, '{}'::jsonb)
		`, fixture.tenantID, fixture.branchID, tc.reportKind, tc.sourceID, fixture.itemID, fixture.itemLegacyID); err != nil {
			t.Fatalf("%s: seed historical item price row: %v", tc.leaf, err)
		}

		operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
		request := readModelRequest(http.MethodGet, "/v1/reports/"+tc.leaf+"?from=2026-08-06&to=2026-08-06", operator)
		request.SetPathValue("kind", tc.leaf)
		recorder := httptest.NewRecorder()
		(&Server{database: database}).report(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", tc.leaf, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Rows []reportRow `json:"rows"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("%s: decode: %v", tc.leaf, err)
		}
		if len(body.Rows) != 1 {
			t.Fatalf("%s: rows = %+v, want exactly 1 seeded row", tc.leaf, body.Rows)
		}
		// "Previous" (Quantity column) must be blank -- old_sale_price is
		// NULL and must never render as "Panadol 500mg".
		if body.Rows[0].Quantity != "" {
			t.Errorf("%s: Previous price column = %q, want \"\" (old_sale_price is NULL; must not leak old_name %q)", tc.leaf, body.Rows[0].Quantity, "Panadol 500mg")
		}
		// "Current" (Amount column) must still show the real, non-NULL
		// price, proving the fix did not also blank out populated prices.
		if !reportNumericEqual(body.Rows[0].Amount, "42.7500") {
			t.Errorf("%s: Current price column = %q, want \"42.7500\" (new_sale_price was non-NULL)", tc.leaf, body.Rows[0].Amount)
		}
	}
}

// TestItemHistoryBasicDataReportShowsChangedFieldNotName is a regression
// test for the Phase Q Finding E fix: historicalReportQuery's default branch
// used to project h.old_name/h.new_name into the "Previous"/"Current"
// columns for historyMode "item-basic-data" (report_kind =
// 'item-basic-data') exactly as it does for the unrelated "item-name" report
// kind, even though item-basic-data rows are emitted from a 17-field
// fingerprint (customicode, stock, active, iccode, icatcode, packcode,
// manfcode, packunits, location1, remarks1, reorderqty, optimumqty,
// restricted, taxable, itemtypecode, measureunitcode, genericcode, gcode)
// that is entirely independent of the item's name. A row whose sole purpose
// was signaling e.g. a PackUnits change showed the item's name (often
// unchanged) before/after, carrying zero information about the actual
// change.
//
// The fix looks up the immediately preceding historical_item_changes row for
// the same item (by occurred_at) and diffs the 17 fingerprint fields between
// that row's payload and the current row's payload, projecting only the
// fields that actually differ as "field=value; field=value" into
// "Previous"/"Current" instead of the item name.
//
// This test seeds a previous snapshot row and a current item-basic-data row
// whose payloads differ only in "packunits" and "reorderqty" (both item
// names identical, so the old buggy projection would show identical,
// uninformative values), then asserts the report's Previous/Current columns
// surface the actual changed fields and values, not the (unchanged) name.
func TestItemHistoryBasicDataReportShowsChangedFieldNotName(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "item-basic-fix-"+time.Now().Format("150405.000000"))
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	sameName := "Panadol 500mg"

	// Previous snapshot: an earlier historical_item_changes row for the same
	// item (any report_kind), whose payload carries the "before" values of
	// the 17-field basic-data fingerprint.
	previousPayload := `{
		"customicode": "CUST1", "stock": "10", "active": "1", "iccode": "IC1",
		"icatcode": "CAT1", "packcode": "PACK1", "manfcode": "MANF1",
		"packunits": "10", "location1": "A1", "remarks1": "ok",
		"reorderqty": "5", "optimumqty": "20", "restricted": "0",
		"taxable": "1", "itemtypecode": "T1", "measureunitcode": "U1",
		"genericcode": "G1", "gcode": "GC1"
	}`
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_item_changes (
			tenant_id, branch_id, report_kind, source_legacy_id, item_id,
			item_legacy_id, occurred_at, old_name, new_name, old_sale_price,
			new_sale_price, user_legacy_id, change_reason, source_table,
			source_table_row, payload
		) VALUES ($1::uuid, $2::uuid, 'item-first-observed', 'basic-fix-prev', $3::uuid, $4,
			'2026-08-05T10:00:00Z', '', $5, NULL, NULL, '42', '', 'ItemLog', 'basic-fix-prev', $6::jsonb)
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID, sameName, previousPayload); err != nil {
		t.Fatalf("seed previous snapshot row: %v", err)
	}

	// Current item-basic-data row: same item name (old_name == new_name, so
	// the old buggy projection would show identical, uninformative values on
	// both sides), but packunits changed 10 -> 12 and reorderqty changed
	// 5 -> 8.
	currentPayload := `{
		"customicode": "CUST1", "stock": "10", "active": "1", "iccode": "IC1",
		"icatcode": "CAT1", "packcode": "PACK1", "manfcode": "MANF1",
		"packunits": "12", "location1": "A1", "remarks1": "ok",
		"reorderqty": "8", "optimumqty": "20", "restricted": "0",
		"taxable": "1", "itemtypecode": "T1", "measureunitcode": "U1",
		"genericcode": "G1", "gcode": "GC1"
	}`
	if _, err := database.ExecContext(ctx, `
		INSERT INTO historical_item_changes (
			tenant_id, branch_id, report_kind, source_legacy_id, item_id,
			item_legacy_id, occurred_at, old_name, new_name, old_sale_price,
			new_sale_price, user_legacy_id, change_reason, source_table,
			source_table_row, payload
		) VALUES ($1::uuid, $2::uuid, 'item-basic-data', 'basic-fix-cur', $3::uuid, $4,
			'2026-08-06T10:00:00Z', $5, $5, NULL, NULL, '42', 'basic data update', 'ItemLog', 'basic-fix-cur', $6::jsonb)
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID, sameName, currentPayload); err != nil {
		t.Fatalf("seed current basic-data row: %v", err)
	}

	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	const leaf = "item-reports-history-item-basic-data-changes"
	request := readModelRequest(http.MethodGet, "/v1/reports/"+leaf+"?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", leaf)
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
		t.Fatalf("rows = %+v, want exactly 1 seeded item-basic-data row", body.Rows)
	}

	previous := body.Rows[0].Quantity
	current := body.Rows[0].Amount

	// Must not be the item name -- that was the bug (Finding E).
	if previous == sameName || current == sameName {
		t.Fatalf("Previous/Current = %q / %q, want the changed basic-data fields, not the item name %q", previous, current, sameName)
	}
	// Must surface the fields that actually changed (packunits, reorderqty)
	// with their old/new values, and must NOT include fields that did not
	// change (e.g. customicode).
	for _, want := range []string{"packunits=10", "reorderqty=5"} {
		if !strings.Contains(previous, want) {
			t.Errorf("Previous = %q, want it to contain %q", previous, want)
		}
	}
	for _, want := range []string{"packunits=12", "reorderqty=8"} {
		if !strings.Contains(current, want) {
			t.Errorf("Current = %q, want it to contain %q", current, want)
		}
	}
	if strings.Contains(previous, "customicode") || strings.Contains(current, "customicode") {
		t.Errorf("Previous/Current = %q / %q, want unchanged fields like customicode to be excluded from the diff", previous, current)
	}
}

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

// TestStockBalanceReportGoldenReconciliation is a Phase P golden cross-check
// for the "balance" mode stockReadModel leaves (stock-in-hand-others,
// stock-in-hand-batch-priority-wise, stock-in-hand-stock-quantity-format,
// stock-in-hand-stock-in-hand-audit-purpose, and
// stock-in-hand-batch-priority-wise-audit-purposes all share this query
// shape). It hand-computes the on-hand quantity for four batches from raw
// stock_ledger IN/OUT/ADJUSTMENT rows and drives the balance cache through
// the real production pipeline (POST /v1/inventory/rebuild, i.e.
// (*Server).rebuildStockBalances), independently confirming the rebuilt
// stock_balances cache matches the hand-computed net for each batch.
//
// It then calls the stock-in-hand-others report endpoint and asserts the
// golden row values. This test previously pinned a confirmed reports.go bug
// (found while writing this test, 2026-08-09): the "balance" mode branch of
// stockReadModelQuery built its date filter from a bare, unqualified
// `updated_at` column, which Postgres rejected as ambiguous across the three
// joined tables that each have their own `updated_at` (stock_balances,
// master_godowns, master_items), so every request failed with
// `503 report_read_failed` regardless of data. That column reference is now
// qualified as `sb.updated_at` in reports.go, and this test asserts the
// resulting golden rows instead of the crash. See
// docs/evidence/PHASE_P_GOLDEN_VERIFICATION_BALANCE_2026-08-09.md for the original
// root-cause writeup. This test uses a disposable fixture tenant so the
// ledger -> cache reconciliation half of the pipeline stays provable in
// isolation from the separate, still-open finding that the shared
// legacy-reference-sandbox tenant (eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee)
// currently has zero stock_balances rows despite 10,507 stock_batches and
// 781,203 posted stock_ledger rows -- that is an operational/migration gap,
// not a reports.go defect, and is out of scope for this test.
func TestStockBalanceReportGoldenReconciliation(t *testing.T) {
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

	suffix := "stock-balance-golden-" + time.Now().Format("150405.000000")
	fixture := seedStockTenant(t, ctx, database, suffix)
	defer func() {
		// Best-effort cleanup, matching every sibling stock fixture: posted
		// stock_ledger rows are immutable by design, so a tenant that posted
		// inventory movements cannot always be fully deleted on the disposable
		// test cluster; the timestamp-suffixed orphan is inert.
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()

	// A second item and a second godown let the golden set exercise
	// multi-item, multi-godown grouping and the on_hand<>0 exclusion in one
	// pass, instead of only ever proving a single-row report.
	var itemBID string
	itemBLegacyID := "item-b-" + suffix
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_items (tenant_id, legacy_id, code, name, payload)
		VALUES ($1::uuid, $2, $2, 'Second Stock Item', '{}') RETURNING id::text
	`, fixture.tenantID, itemBLegacyID).Scan(&itemBID); err != nil {
		t.Fatalf("seed second item: %v", err)
	}
	var godownBID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_godowns (tenant_id, legacy_id, code, name)
		VALUES ($1::uuid, $2, $2, 'Second Godown') RETURNING id::text
	`, fixture.tenantID, "godown-b-"+suffix).Scan(&godownBID); err != nil {
		t.Fatalf("seed second godown: %v", err)
	}

	type ledgerLine struct {
		direction string
		sign      int
		quantity  string
	}
	type batchSpec struct {
		batchNumber string
		itemID      string
		itemLegacy  string
		godownID    string
		unitCost    string
		lines       []ledgerLine
		wantOnHand  string // hand-computed net; rebuildStockBalances writes a cache row even when this is "0"
	}
	batches := []batchSpec{
		{
			// 100 in - 30 out - 5 adjustment(-1) = 65
			batchNumber: "GOLDEN-A", itemID: fixture.itemID, itemLegacy: fixture.itemLegacyID,
			godownID: fixture.godownID, unitCost: "5.00",
			lines: []ledgerLine{
				{"in", 1, "100"}, {"out", 1, "30"}, {"adjustment", -1, "5"},
			},
			wantOnHand: "65",
		},
		{
			// 40 in + 10 in = 50, same item, different godown
			batchNumber: "GOLDEN-B", itemID: fixture.itemID, itemLegacy: fixture.itemLegacyID,
			godownID: godownBID, unitCost: "5.50",
			lines: []ledgerLine{
				{"in", 1, "40"}, {"in", 1, "10"},
			},
			wantOnHand: "50",
		},
		{
			// 20 in - 20 out = 0: the cache still carries this row (rebuild has
			// no HAVING filter), but the report's on_hand<>0 predicate must
			// exclude it from the response.
			batchNumber: "GOLDEN-C", itemID: itemBID, itemLegacy: itemBLegacyID,
			godownID: fixture.godownID, unitCost: "12.25",
			lines: []ledgerLine{
				{"in", 1, "20"}, {"out", 1, "20"},
			},
			wantOnHand: "0",
		},
		{
			// 15 in + 3 adjustment(+1) = 18, second item, second godown
			batchNumber: "GOLDEN-D", itemID: itemBID, itemLegacy: itemBLegacyID,
			godownID: godownBID, unitCost: "8.00",
			lines: []ledgerLine{
				{"in", 1, "15"}, {"adjustment", 1, "3"},
			},
			wantOnHand: "18",
		},
	}

	occurredAt := "2026-08-06T00:00:00Z"
	for _, batch := range batches {
		var batchID string
		if err := database.QueryRowContext(ctx, `
			INSERT INTO stock_batches
				(tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, '2030-01-01', $7::numeric)
			RETURNING id::text
		`, fixture.tenantID, fixture.branchID, batch.itemID, batch.itemLegacy, batch.godownID, batch.batchNumber, batch.unitCost).Scan(&batchID); err != nil {
			t.Fatalf("seed batch %s: %v", batch.batchNumber, err)
		}
		for lineIndex, line := range batch.lines {
			var eventID string
			if err := database.QueryRowContext(ctx, `
				INSERT INTO sync_events (event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id, idempotency_key, schema_version, payload, occurred_at)
				VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'inventory', gen_random_uuid(), $5, 1, '{}'::jsonb, $6::timestamptz)
				RETURNING event_id::text
			`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID,
				batch.batchNumber+"-line-"+line.direction+"-"+time.Now().Format("150405.000000000")+"-"+string(rune('a'+lineIndex)), occurredAt).Scan(&eventID); err != nil {
				t.Fatalf("seed ledger event for %s: %v", batch.batchNumber, err)
			}
			if _, err := database.ExecContext(ctx, `
				INSERT INTO stock_ledger
					(tenant_id, branch_id, batch_id, source_event_id, source_line_key, direction,
					 adjustment_sign, quantity, unit_cost, occurred_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'row-0', $5, $6, $7::numeric, $8::numeric, $9::timestamptz)
			`, fixture.tenantID, fixture.branchID, batchID, eventID, line.direction, line.sign, line.quantity, batch.unitCost, occurredAt); err != nil {
				t.Fatalf("seed ledger line for %s: %v", batch.batchNumber, err)
			}
		}
	}

	// Drive the cache through the real production rebuild endpoint rather
	// than inserting stock_balances directly, so this test also exercises
	// the exact code path (stock.go rebuildStockBalances) that is
	// responsible for populating the cache the report reads from.
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	rebuildRequest := httptest.NewRequest(http.MethodPost, "/v1/inventory/rebuild", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, operator))
	rebuildRecorder := httptest.NewRecorder()
	(&Server{database: database}).rebuildStockBalances(rebuildRecorder, rebuildRequest)
	if rebuildRecorder.Code != http.StatusOK {
		t.Fatalf("rebuild status = %d, body = %s", rebuildRecorder.Code, rebuildRecorder.Body.String())
	}

	// Independently confirm the cache the rebuild produced matches the
	// hand-computed net before comparing it to the report's HTTP output, so
	// a report-layer bug cannot be masked by a rebuild-layer bug.
	for _, batch := range batches {
		var onHand string
		if err := database.QueryRowContext(ctx, `
			SELECT sb.on_hand::text FROM stock_balances sb
			JOIN stock_batches b ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id AND b.id = sb.batch_id
			WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid AND b.batch_number = $3
		`, fixture.tenantID, fixture.branchID, batch.batchNumber).Scan(&onHand); err != nil {
			t.Fatalf("read rebuilt stock_balances for %s: %v", batch.batchNumber, err)
		}
		if !reportNumericEqual(onHand, batch.wantOnHand) {
			t.Fatalf("rebuilt stock_balances on_hand for %s = %s, want %s (hand-computed from seeded stock_ledger rows)", batch.batchNumber, onHand, batch.wantOnHand)
		}
	}

	from := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	to := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	request := readModelRequest(http.MethodGet, "/v1/reports/stock-in-hand-others?from="+from+"&to="+to, operator)
	request.SetPathValue("kind", "stock-in-hand-others")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)

	// reports.go's stockReadModelQuery default ("balance") branch now
	// qualifies its date filter as `sb.updated_at` (previously a bare,
	// ambiguous `updated_at` that Postgres rejected outright -- see the
	// package doc comment above and
	// docs/evidence/PHASE_P_GOLDEN_VERIFICATION_BALANCE_2026-08-09.md), so the report
	// now succeeds and returns the golden rows computed from the fixture
	// above instead of crashing.
	if recorder.Code != http.StatusOK {
		t.Fatalf("stock-in-hand-others report status = %d, body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode stock-in-hand-others report: %v", err)
	}

	// Only GOLDEN-A, GOLDEN-B, and GOLDEN-D are expected: the seed fixture's
	// own "EXPIRED" batch and GOLDEN-C both net to zero on-hand, and the
	// query's `sb.on_hand <> 0` predicate must exclude both -- proving the
	// fix does not merely stop the crash but reads real, correctly filtered
	// data end-to-end. Expected order follows the query's
	// `ORDER BY sb.updated_at DESC, b.batch_number, sb.batch_id`: every row
	// shares the same rebuild-transaction `updated_at`, so batches sort
	// alphabetically by batch_number.
	wantRows := []struct {
		document, party, item, quantity, amount string
	}{
		{"GOLDEN-A", "Main Godown", "Stock Item", "65", "5.00"},
		{"GOLDEN-B", "Second Godown", "Stock Item", "50", "5.50"},
		{"GOLDEN-D", "Second Godown", "Second Stock Item", "18", "8.00"},
	}
	if len(body.Rows) != len(wantRows) {
		t.Fatalf("stock-in-hand-others rows = %d, want %d: %+v", len(body.Rows), len(wantRows), body.Rows)
	}
	for index, want := range wantRows {
		got := body.Rows[index]
		if got.Document != want.document {
			t.Fatalf("row %d document = %q, want %q (full row: %+v)", index, got.Document, want.document, got)
		}
		if got.OccurredAt != "2030-01-01" {
			t.Fatalf("row %d occurredAt = %q, want the seeded batch expiry_date 2030-01-01 (full row: %+v)", index, got.OccurredAt, got)
		}
		if got.Party != want.party {
			t.Fatalf("row %d party (godown) = %q, want %q (full row: %+v)", index, got.Party, want.party, got)
		}
		if got.Item != want.item {
			t.Fatalf("row %d item = %q, want %q (full row: %+v)", index, got.Item, want.item, got)
		}
		if !reportNumericEqual(got.Quantity, want.quantity) {
			t.Fatalf("row %d quantity = %q, want %q (full row: %+v)", index, got.Quantity, want.quantity, got)
		}
		if !reportNumericEqual(got.Amount, want.amount) {
			t.Fatalf("row %d amount (unit_cost, balance mode) = %q, want %q (full row: %+v)", index, got.Amount, want.amount, got)
		}
	}
}

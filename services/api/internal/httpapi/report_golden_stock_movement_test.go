package httpapi

import (
	"strings"
	"testing"
)

// TestPhaseGoldenStockMovementLeavesResolveToExpectedStockMode locks the
// registry wiring for the 12 Phase P stock movement/register leaves verified
// in docs/PHASE_P_GOLDEN_VERIFICATION_MOVEMENT_2026-08-09.md against
// independent psql cross-checks on stock_ledger/stock_balances/stock_allocations.
func TestPhaseGoldenStockMovementLeavesResolveToExpectedStockMode(t *testing.T) {
	tests := map[string]struct {
		mode  string
		title string
	}{
		"stock-register":                             {"movement", "Stock Register"},
		"item-stock-register-summary":                {"item-summary", "Item Stock Register Summary"},
		"stock-register-for-narcotics":               {"narcotics-movement", "Stock Register(For Narcotics)"},
		"stock-and-sales":                            {"stock-sales", "Stock and Sales"},
		"item-activity":                              {"movement", "Item Activity"},
		"stock-register-narcotics-format2":           {"narcotics-movement", "Stock Register(Narcotics Format2)"},
		"expiry-report":                              {"expiry", "Expiry Report"},
		"expiry-report-class-wise":                   {"expiry-class", "Expiry Report(Class Wise)"},
		"daily-stock-in-out":                         {"movement-summary", "Daily Stock IN/OUT"},
		"stock-in-out-date-wise":                     {"movement-summary", "Stock IN/OUT(Date Wise)"},
		"stock-management-report":                    {"management-summary", "Stock Management Report"},
		"norcotics-stock-register-generic-type-wise": {"narcotics-generic", "Norcotics Stock Register-Generic Type Wise"},
	}
	for kind, want := range tests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if !spec.stockReadModel {
			t.Fatalf("%s: stockReadModel = false, want true", kind)
		}
		if spec.stockMode != want.mode {
			t.Fatalf("%s: stockMode = %q, want %q", kind, spec.stockMode, want.mode)
		}
		if spec.title != want.title {
			t.Fatalf("%s: title = %q, want %q", kind, spec.title, want.title)
		}
	}
}

// TestStockLedgerNativeModesNeverReadTheBalanceCache documents and locks the
// golden-verification finding that the movement/item-summary/movement-summary/
// narcotics modes compute directly from posted stock_ledger rows (verified to
// reconcile exactly against independent GROUP BY aggregation on 2026-06-01
// sandbox data: 327 in / 764 out rows, net qty 3548, net value 5275102.1364)
// and therefore are immune to the stock_balances cache-staleness gap found for
// the balance-cache-dependent modes below.
func TestStockLedgerNativeModesNeverReadTheBalanceCache(t *testing.T) {
	pagination := "LIMIT $8 OFFSET $9"
	for _, mode := range []string{"movement", "item-summary", "movement-summary", "narcotics-movement", "narcotics-generic"} {
		query := stockReadModelQuery(mode, pagination)
		if !strings.Contains(query, "stock_ledger") {
			t.Errorf("mode %q: query does not read stock_ledger:\n%s", mode, query)
		}
		if !strings.Contains(query, "COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'") {
			t.Errorf("mode %q: query does not gate on posted status:\n%s", mode, query)
		}
		if strings.Contains(query, "FROM stock_balances") {
			t.Errorf("mode %q: query unexpectedly reads the stock_balances cache table", mode)
		}
	}
}

// TestStockBalanceCacheDependentModesReadFromStockBalances documents the
// counterpart finding: expiry/expiry-class/stock-sales/management-summary
// read on-hand from the stock_balances cache table, which independent psql
// verification found to be empty (0 rows) for the sandbox tenant/branch
// despite 781,203 posted stock_ledger rows and 10,507 stock_batches rows with
// typed expiry dates (7,312 of which would carry non-zero on-hand if
// stock_balances were rebuilt via the same aggregation as
// rebuildStockBalances in stock.go). This is a data/cache population gap in
// the verification environment, not a reports.go logic defect, but it means
// these four leaves are VACUOUS-NO-DATA in this environment and cannot be
// golden-verified against non-empty output here.
func TestStockBalanceCacheDependentModesReadFromStockBalances(t *testing.T) {
	pagination := "LIMIT $8 OFFSET $9"
	for _, mode := range []string{"expiry", "expiry-class", "stock-sales", "management-summary"} {
		query := stockReadModelQuery(mode, pagination)
		if !strings.Contains(query, "FROM stock_balances") {
			t.Errorf("mode %q: expected query to read FROM stock_balances (cache-dependent), got:\n%s", mode, query)
		}
	}
	// stock-sales additionally cross-references stock_allocations for the
	// sale-side consumption; verified separately (sandbox tenant has 0
	// stock_allocations rows despite 291,361 posted cash/credit sales, so the
	// cross-reference itself is also unverifiable in this environment).
	salesQuery := stockReadModelQuery("stock-sales", pagination)
	if !strings.Contains(salesQuery, "stock_allocations") {
		t.Errorf("stock-sales mode: expected query to reference stock_allocations, got:\n%s", salesQuery)
	}
}

// TestStockMovementSignedQuantityConvention locks the sign convention used by
// the movement/item-summary/movement-summary/narcotics modes: OUT is negated,
// ADJUSTMENT is multiplied by adjustment_sign, IN passes through unsigned.
// This convention was independently verified against raw stock_ledger rows
// (e.g. an 'out' row with quantity 10 must contribute -10 to net totals).
func TestStockMovementSignedQuantityConvention(t *testing.T) {
	pagination := "LIMIT $8 OFFSET $9"
	// "movement" (stock-register / item-activity) applies the CASE to the
	// posted_stock_ledger CTE's own already-unqualified output columns;
	// item-summary/movement-summary apply it directly to the "l." aliased
	// stock_ledger columns. Both were independently verified against raw
	// stock_ledger rows to negate OUT and multiply ADJUSTMENT by
	// adjustment_sign.
	wantCaseByMode := map[string]string{
		"movement":         "WHEN direction = 'out' THEN -quantity",
		"item-summary":     "WHEN l.direction = 'out' THEN -l.quantity",
		"movement-summary": "WHEN l.direction = 'out' THEN -l.quantity",
	}
	for mode, wantCase := range wantCaseByMode {
		query := stockReadModelQuery(mode, pagination)
		if !strings.Contains(query, wantCase) {
			t.Errorf("mode %q: missing expected signed-quantity CASE branch %q", mode, wantCase)
		}
		if !strings.Contains(query, "adjustment_sign") {
			t.Errorf("mode %q: missing adjustment_sign handling", mode)
		}
	}
}

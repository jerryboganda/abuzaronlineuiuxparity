package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// This file regression-tests the fix for the "category-wise-purchase" bug
// documented in docs/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md
// ("item-summary x receiving (category-wise-purchase)"): the report is
// titled "Category Wise Purchase" but its query
// (purchaseItemSummaryReadModelQuery, purchaseMode "item-summary", the mode
// used exclusively by this report leaf per phaseNReportRegistry) grouped by
// item name / supplier instead of resolved item category, so the report
// never actually grouped by category despite its name.
//
// The verified fix, from docs/PHASE_N_LEGACY_SEMANTICS_RESEARCH_2026-08-09.md
// and docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_A_2026-08-09.md: join
// master_items.payload->>'ICatCode' to master_categories.legacy_id (kind
// 'item_category'), which resolves 100% of master_items rows in the
// sandbox tenant to a real category name (MEDICINES, NARCOTICS, CONSUMER,
// COUNSELING, DIAGNOSTIC, MILK, DR KHALID MUGHAL).

// TestCategoryWisePurchaseQueryJoinsMasterCategories asserts the generated
// SQL for purchaseMode "item-summary" now joins master_categories on the
// item_category kind, keyed by master_items.payload->>'ICatCode', and
// groups/orders by the resolved category instead of item/party.
func TestCategoryWisePurchaseQueryJoinsMasterCategories(t *testing.T) {
	query := purchaseItemSummaryReadModelQuery("se.aggregate = 'receiving'", "")

	if !containsAll(query,
		"LEFT JOIN master_items mi",
		"LEFT JOIN master_categories mc",
		"mc.category_kind = 'item_category'",
		"mc.legacy_id = mi.payload->>'ICatCode'",
		"COALESCE(mc.name, 'UNCATEGORIZED') AS category",
		"GROUP BY category",
		"ORDER BY category",
	) {
		t.Fatalf("expected purchaseItemSummaryReadModelQuery to join master_categories via "+
			"master_items.payload->>'ICatCode' and group/order by the resolved category name - "+
			"query changed, re-verify against docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_A_2026-08-09.md "+
			"before updating this test:\n%s", query)
	}

	// The pre-fix query grouped by item/party; guard against silently
	// reintroducing that shape alongside (or instead of) the category join.
	if strings.Contains(query, "GROUP BY item, party") {
		t.Errorf("purchaseItemSummaryReadModelQuery still groups by item, party (the pre-fix defective shape):\n%s", query)
	}
}

// TestCategoryWisePurchaseQueryCoversAllAggregateConditions confirms the
// canonicalKinds/eventAggregate switch inside purchaseItemSummaryReadModelQuery
// (mirrored from purchaseLineDetailReadModelQuery) still resolves correctly
// for every aggregateCondition purchaseReadModelQueryMode can pass through
// for the "item-summary" mode, so the category fix does not silently break
// the underlying document-kind filter.
func TestCategoryWisePurchaseQueryCoversAllAggregateConditions(t *testing.T) {
	tests := []struct {
		name      string
		aggregate string
		wantKinds string
		wantAggr  string
	}{
		{"receiving (category-wise-purchase)", "se.aggregate = 'receiving'", "'pack-purchase', 'loose-purchase', 'opening-purchase'", "se.aggregate = 'receiving'"},
		{"return", "se.aggregate = 'return'", "'purchase-return'", "se.aggregate = 'return'"},
		{"purchase_order", "se.aggregate = 'purchase_order'", "'purchase-order'", "se.aggregate = 'purchase_order'"},
	}
	for _, tc := range tests {
		query := purchaseItemSummaryReadModelQuery(tc.aggregate, "")
		if !strings.Contains(query, tc.wantKinds) {
			t.Errorf("%s: expected query to filter d.kind IN (%s):\n%s", tc.name, tc.wantKinds, query)
		}
	}
}

// TestCategoryWisePurchaseReportGroupsByRealCategoryNames cross-checks the
// fixed category-wise-purchase report against the legacy-reference-sandbox
// tenant's live data via an HTTP call to the report handler, proving real
// category names (not item names) appear in the response and that the
// summed amount across all returned category rows equals the independently
// computed grand total for the same date range. Skipped unless DATABASE_URL
// is configured.
func TestCategoryWisePurchaseReportGroupsByRealCategoryNames(t *testing.T) {
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

	const (
		sandboxTenantID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		sandboxBranchID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		sampleDate      = "2026-07-01"
	)

	var branchExists bool
	if err := database.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid)`,
		sandboxBranchID, sandboxTenantID).Scan(&branchExists); err != nil {
		t.Skipf("legacy-reference-sandbox tenant/branch not present in this database: %v", err)
	}
	if !branchExists {
		t.Skip("legacy-reference-sandbox branch not present in this database")
	}

	// Independent baseline: compute the expected per-category totals for
	// the sample date directly from the base tables, using the same
	// verified JOIN pattern the fix applies.
	type categoryTotal struct {
		category string
		quantity float64
		amount   float64
	}
	wantRows := map[string]categoryTotal{}
	baseRows, err := database.QueryContext(ctx, `
		SELECT COALESCE(mc.name, 'UNCATEGORIZED') AS category,
		       COALESCE(SUM(COALESCE(stock.stock_quantity, l.quantity)), 0) AS quantity,
		       COALESCE(SUM(COALESCE(
		           NULLIF(l.legacy_payload->>'Amount', ''),
		           NULLIF(l.legacy_payload->>'LineTotal', ''),
		           l.line_total::text,
		           '0'
		       )::numeric), 0) AS amount
		FROM business_documents d
		JOIN business_document_lines l
		  ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
		LEFT JOIN master_items mi ON mi.tenant_id = l.tenant_id AND mi.id = l.item_id
		LEFT JOIN master_categories mc
		  ON mc.tenant_id = mi.tenant_id AND mc.category_kind = 'item_category'
		 AND mc.legacy_id = mi.payload->>'ICatCode'
		LEFT JOIN LATERAL (
			SELECT SUM(sl.quantity * sl.adjustment_sign) AS stock_quantity
			FROM stock_ledger sl
			WHERE sl.tenant_id = d.tenant_id AND sl.branch_id = d.branch_id
			  AND sl.source_document_id = d.id AND sl.source_document_line_id = l.id
		) stock ON TRUE
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
		  AND d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
		  AND d.status = 'posted'
		  AND d.occurred_at >= $3::date AND d.occurred_at < ($3::date + INTERVAL '1 day')
		GROUP BY mc.name
	`, sandboxTenantID, sandboxBranchID, sampleDate)
	if err != nil {
		t.Skipf("baseline category totals not available: %v", err)
	}
	defer baseRows.Close()
	for baseRows.Next() {
		var ct categoryTotal
		if err := baseRows.Scan(&ct.category, &ct.quantity, &ct.amount); err != nil {
			t.Fatalf("scan baseline row: %v", err)
		}
		wantRows[ct.category] = ct
	}
	if err := baseRows.Err(); err != nil {
		t.Fatalf("iterate baseline rows: %v", err)
	}
	if len(wantRows) == 0 {
		t.Skipf("no posted receiving documents for tenant/branch on %s; fixture data has drifted", sampleDate)
	}

	operator := &sessionContext{
		UserID:    "00000000-0000-0000-0000-0000000000e1",
		TenantID:  sandboxTenantID,
		BranchID:  sandboxBranchID,
		CounterID: "00000000-0000-0000-0000-0000000000e2",
		Roles:     []string{"tenant_admin"},
	}
	server := &Server{database: database}

	request := readModelRequest(http.MethodGet,
		"/v1/reports/category-wise-purchase?pageSize=1000&from="+sampleDate+"&to="+sampleDate,
		operator)
	request.SetPathValue("kind", "category-wise-purchase")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("category-wise-purchase status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Rows    []reportRow `json:"rows"`
		HasMore bool        `json:"hasMore"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Rows) == 0 {
		t.Fatalf("category-wise-purchase returned no rows for %s, want at least one category row", sampleDate)
	}

	// Known real category names for the sandbox tenant's item_category
	// rows (docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_A_2026-08-09.md).
	knownCategories := map[string]bool{
		"MEDICINES": true, "NARCOTICS": true, "CONSUMER": true,
		"COUNSELING": true, "DIAGNOSTIC": true, "MILK": true,
		"DR KHALID MUGHAL": true, "UNCATEGORIZED": true,
	}

	seen := map[string]bool{}
	for _, row := range body.Rows {
		if seen[row.Document] {
			t.Errorf("category %q appears more than once in the response; expected one row per category", row.Document)
		}
		seen[row.Document] = true
		if !knownCategories[row.Document] {
			t.Errorf("row Document %q is not a known real category name for this tenant "+
				"(want one of the 7 master_categories item_category names or UNCATEGORIZED); "+
				"this suggests the report is still grouping by item, not category", row.Document)
		}
	}

	if len(seen) != len(wantRows) {
		t.Errorf("category-wise-purchase returned %d distinct categories, want %d (baseline: %v)",
			len(seen), len(wantRows), wantRows)
	}

	gotTotalAmount := 0.0
	for _, row := range body.Rows {
		want, ok := wantRows[row.Document]
		if !ok {
			t.Errorf("category %q in response has no matching baseline row", row.Document)
			continue
		}
		gotAmount, err := strconv.ParseFloat(row.Amount, 64)
		if err != nil {
			t.Fatalf("category %q: amount %q did not parse as a number: %v", row.Document, row.Amount, err)
		}
		if diff := gotAmount - want.amount; diff > 0.01 || diff < -0.01 {
			t.Errorf("category %q: amount = %v, want %v (baseline)", row.Document, gotAmount, want.amount)
		}
		gotTotalAmount += gotAmount
	}

	wantGrandTotal := 0.0
	for _, ct := range wantRows {
		wantGrandTotal += ct.amount
	}
	if diff := gotTotalAmount - wantGrandTotal; diff > 0.01 || diff < -0.01 {
		t.Fatalf("category-wise-purchase grand total across returned rows = %v, want %v (independently computed baseline)",
			gotTotalAmount, wantGrandTotal)
	}
}

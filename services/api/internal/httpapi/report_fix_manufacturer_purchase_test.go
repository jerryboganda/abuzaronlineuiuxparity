package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestManufacturerPurchaseQueriesJoinMasterManufacturers is the static
// regression guard for the fix documented in
// docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md
// ("manufacturer-wise-detail", "manufacturer-wise-monthly-stock-movement",
// "supplier-manufacturer-wise-g-p" have no manufacturer join/grouping at
// all). Each of the three leaves must now route through a query that joins
// master_manufacturers via master_items.payload->>'ManfCode', the join
// verified against real data in
// docs/evidence/PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md.
func TestManufacturerPurchaseQueriesJoinMasterManufacturers(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	aggregate := "se.aggregate = 'receiving'"

	cases := []struct {
		name  string
		query string
	}{
		{"manufacturer-wise-detail", purchaseReadModelQueryMode(aggregate, "manufacturer-detail", pagination)},
		{"manufacturer-wise-monthly-stock-movement", purchaseReadModelQueryMode(aggregate, "manufacturer-month-summary", pagination)},
		{"supplier-manufacturer-wise-g-p", purchaseReadModelQueryMode(aggregate, "supplier-manufacturer-summary", pagination)},
	}
	for _, tc := range cases {
		if tc.query == "" {
			t.Fatalf("%s: empty query", tc.name)
		}
		if !containsAll(tc.query,
			"master_manufacturers",
			"mf.code = i.payload->>'ManfCode'",
		) {
			t.Errorf("%s: expected query to join master_manufacturers via master_items.payload->>'ManfCode', "+
				"query changed, re-verify against docs/evidence/PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md:\n%s",
				tc.name, tc.query)
		}
	}
}

// TestSupplierManufacturerRegistryUsesManufacturerModes confirms the three
// captured O-phase registry entries were repointed off the generic
// supplier-only "detail"/"month-summary"/"supplier-summary" modes (which
// carried no manufacturer dimension) onto the new manufacturer-aware modes.
func TestSupplierManufacturerRegistryUsesManufacturerModes(t *testing.T) {
	wantModes := map[string]string{
		"manufacturer-wise-detail":                 "manufacturer-detail",
		"manufacturer-wise-monthly-stock-movement": "manufacturer-month-summary",
		"supplier-manufacturer-wise-g-p":           "supplier-manufacturer-summary",
	}
	for kind, wantMode := range wantModes {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not found in report registry", kind)
		}
		if !spec.purchaseReadModel {
			t.Fatalf("%s: expected purchaseReadModel spec", kind)
		}
		if spec.purchaseMode != wantMode {
			t.Errorf("%s: purchaseMode = %q, want %q", kind, spec.purchaseMode, wantMode)
		}
	}
}

// TestManufacturerWiseDetailReturnsRealManufacturerNames is a live
// end-to-end check against the legacy-reference-sandbox tenant: posted
// purchase lines must surface a resolved master_manufacturers.name (not a
// blank/bare code, and not the supplier name that the pre-fix query
// returned). Skipped unless DATABASE_URL is configured.
func TestManufacturerWiseDetailReturnsRealManufacturerNames(t *testing.T) {
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

	// Independent baseline: a single posted purchase document/line with a
	// resolvable manufacturer, and its expected line_total. Filtering by the
	// exact document number keeps the report's free-text search (which also
	// matches item names) from pulling in unrelated rows, unlike filtering
	// by manufacturer name alone.
	var (
		wantDocument     string
		wantManufacturer string
		wantAmount       float64
	)
	err = database.QueryRowContext(ctx, `
		SELECT d.document_number, mf.name, l.line_total
		FROM business_documents d
		JOIN business_document_lines l ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
		JOIN master_items i ON i.tenant_id = d.tenant_id AND i.id = l.item_id
		JOIN master_manufacturers mf ON mf.tenant_id = i.tenant_id AND mf.code = i.payload->>'ManfCode'
		LEFT JOIN master_parties mp ON mp.tenant_id = d.tenant_id AND mp.id = d.supplier_id AND mp.party_type = 'supplier'
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
		  AND d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
		  AND d.status = 'posted'
		  AND COALESCE(mp.name, '') <> mf.name
		ORDER BY d.occurred_at DESC
		LIMIT 1
	`, sandboxTenantID, sandboxBranchID).Scan(&wantDocument, &wantManufacturer, &wantAmount)
	if err != nil {
		t.Skipf("no posted purchase line with a resolvable manufacturer in this database: %v", err)
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
		"/v1/reports/manufacturer-wise-detail?pageSize=1000&startDate=2000-01-01&endDate=2030-01-01&q="+url.QueryEscape(wantDocument),
		operator)
	request.SetPathValue("kind", "manufacturer-wise-detail")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("manufacturer-wise-detail status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Rows    []reportRow `json:"rows"`
		HasMore bool        `json:"hasMore"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	found := false
	for _, row := range body.Rows {
		if row.Document != wantDocument {
			continue
		}
		if row.Party != wantManufacturer {
			continue
		}
		amount, parseErr := strconv.ParseFloat(row.Amount, 64)
		if parseErr != nil {
			continue
		}
		if diff := amount - wantAmount; diff <= 0.01 && diff >= -0.01 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("manufacturer-wise-detail: expected a row for document %q with party (manufacturer) %q and "+
			"amount %.2f, but none matched among %d returned rows (pre-fix query grouped/joined nothing by "+
			"manufacturer, so party would show the supplier instead): %+v",
			wantDocument, wantManufacturer, wantAmount, len(body.Rows), body.Rows)
	}
}

// TestSupplierManufacturerGPGroupsBothDimensions is a live check that
// supplier-manufacturer-wise-g-p now groups by BOTH supplier and
// manufacturer (previously it silently collapsed to a supplier-only
// summary with the manufacturer dimension entirely absent). It does not
// assert a gross-profit figure, since no verified purchase-side G/P formula
// exists yet (see the comment on purchaseSupplierManufacturerSummaryReadModelQuery).
func TestSupplierManufacturerGPGroupsBothDimensions(t *testing.T) {
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

	var distinctManufacturersForOneSupplier int
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT mf.name)
		FROM business_documents d
		JOIN business_document_lines l ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
		JOIN master_items i ON i.tenant_id = d.tenant_id AND i.id = l.item_id
		JOIN master_manufacturers mf ON mf.tenant_id = i.tenant_id AND mf.code = i.payload->>'ManfCode'
		JOIN master_parties mp ON mp.tenant_id = d.tenant_id AND mp.id = d.supplier_id AND mp.party_type = 'supplier'
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
		  AND d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
		  AND d.status = 'posted'
		  AND mp.id = (
		      SELECT d2.supplier_id
		      FROM business_documents d2
		      WHERE d2.tenant_id = $1::uuid AND d2.branch_id = $2::uuid
		        AND d2.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
		        AND d2.status = 'posted' AND d2.supplier_id IS NOT NULL
		      GROUP BY d2.supplier_id
		      ORDER BY COUNT(*) DESC
		      LIMIT 1
		  )
	`, sandboxTenantID, sandboxBranchID).Scan(&distinctManufacturersForOneSupplier)
	if err != nil {
		t.Skipf("baseline manufacturer count not available: %v", err)
	}
	if distinctManufacturersForOneSupplier < 2 {
		t.Skip("no supplier in this database purchases from >1 manufacturer; cannot demonstrate grouping fix")
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
		"/v1/reports/supplier-manufacturer-wise-g-p?pageSize=1000&startDate=2000-01-01&endDate=2030-01-01",
		operator)
	request.SetPathValue("kind", "supplier-manufacturer-wise-g-p")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("supplier-manufacturer-wise-g-p status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Rows    []reportRow `json:"rows"`
		HasMore bool        `json:"hasMore"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	bySupplier := map[string]map[string]bool{}
	for _, row := range body.Rows {
		if bySupplier[row.Party] == nil {
			bySupplier[row.Party] = map[string]bool{}
		}
		bySupplier[row.Party][row.Item] = true
	}

	maxManufacturersSeen := 0
	for _, manufacturers := range bySupplier {
		if len(manufacturers) > maxManufacturersSeen {
			maxManufacturersSeen = len(manufacturers)
		}
	}
	if maxManufacturersSeen < 2 {
		t.Errorf("supplier-manufacturer-wise-g-p: expected at least one supplier to show >1 distinct "+
			"manufacturer row (report is supposed to group by supplier AND manufacturer), "+
			"but max distinct manufacturers seen for any single supplier = %d", maxManufacturersSeen)
	}
}

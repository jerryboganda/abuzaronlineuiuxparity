package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestItemListClassWiseUsesRealMasterRecordsKind locks in the fix for the
// "listing-item-list-class-wise" leaf: the registry entry previously carried
// adminKind "item_class" (underscore), a literal that master_records.kind's
// CHECK constraint has never allowed and that no row in the table (any
// tenant) has ever carried - the leaf was structurally guaranteed to return
// zero rows. The corrected adminKind is "item_category", a real, populated
// master_records.kind value (confirmed 7 rows for the legacy-reference-sandbox
// tenant per docs/PHASE_N_GOLDEN_VERIFICATION_CATEGORY_A_2026-08-09.md, and
// matching the "item-category"/"item_category" family already used by the
// sibling admin-listing leaves in this same registry block).
func TestItemListClassWiseUsesRealMasterRecordsKind(t *testing.T) {
	spec, ok := reportSpecForKey("listing-item-list-class-wise")
	if !ok {
		t.Fatal("listing-item-list-class-wise: not present in the report registry")
	}
	if spec.adminKind != "item_category" {
		t.Fatalf("listing-item-list-class-wise adminKind = %q, want \"item_category\"", spec.adminKind)
	}

	query := adminReadModelQuery(spec.adminKind, "LIMIT $6 OFFSET $7")
	if query == "" {
		t.Fatal("empty query for adminKind \"item_category\"")
	}
	if !containsAll(query, "FROM master_records mr", "mr.kind = 'item_category'") {
		t.Fatalf("expected the admin query to filter on the real, populated mr.kind = 'item_category':\n%s", query)
	}
}

// TestItemListClassWiseReturnsRealDataForSandboxTenant is an integration
// test (skipped unless DATABASE_URL is configured) that drives the live
// /v1/reports/listing-item-list-class-wise HTTP endpoint for the
// legacy-reference-sandbox tenant and confirms it now returns real,
// non-empty rows matching an independent master_records baseline count -
// proof the leaf is no longer structurally guaranteed to return zero rows.
func TestItemListClassWiseReturnsRealDataForSandboxTenant(t *testing.T) {
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
	var branchCode string
	if err := database.QueryRowContext(ctx,
		`SELECT code FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid`,
		sandboxBranchID, sandboxTenantID).Scan(&branchCode); err != nil {
		t.Skipf("legacy-reference-sandbox tenant/branch not present in this database: %v", err)
	}
	if branchCode != "SANDBOX" {
		t.Fatalf("unexpected branch code %q for sandbox branch id, refusing to run against the wrong tenant", branchCode)
	}

	if _, err := database.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, false)`, sandboxTenantID); err != nil {
		t.Fatalf("set app.tenant_id: %v", err)
	}
	if _, err := database.ExecContext(ctx, `SELECT set_config('app.branch_id', '', false)`); err != nil {
		t.Fatalf("set app.branch_id: %v", err)
	}
	if _, err := database.ExecContext(ctx, `SELECT set_config('app.allow_tenant_scope', 'true', false)`); err != nil {
		t.Fatalf("set app.allow_tenant_scope: %v", err)
	}
	if _, err := database.ExecContext(ctx, `SELECT set_config('app.authenticating', 'false', false)`); err != nil {
		t.Fatalf("set app.authenticating: %v", err)
	}

	var wantTotal int
	if err := database.QueryRowContext(ctx,
		`SELECT count(*) FROM master_records WHERE tenant_id = $1::uuid AND kind = 'item_category'`,
		sandboxTenantID).Scan(&wantTotal); err != nil {
		t.Fatalf("independent baseline count query failed: %v", err)
	}
	if wantTotal == 0 {
		t.Fatal("independent baseline master_records item_category count = 0 for the sandbox tenant - " +
			"cannot prove a non-empty fix against an empty baseline; re-check the sandbox fixture")
	}

	operator := &sessionContext{
		UserID:    "00000000-0000-0000-0000-0000000000e1",
		TenantID:  sandboxTenantID,
		BranchID:  sandboxBranchID,
		CounterID: "00000000-0000-0000-0000-0000000000e2",
		Roles:     []string{"tenant_admin"},
	}
	server := &Server{database: database}

	request := readModelRequest(http.MethodGet, "/v1/reports/listing-item-list-class-wise?pageSize=1000", operator)
	request.SetPathValue("kind", "listing-item-list-class-wise")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("listing-item-list-class-wise status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Rows    []reportRow `json:"rows"`
		HasMore bool        `json:"hasMore"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.HasMore {
		t.Fatalf("listing-item-list-class-wise returned hasMore=true with pageSize=1000; "+
			"the %d-row expectation would only cover a sample", wantTotal)
	}
	if len(body.Rows) == 0 {
		t.Fatal("listing-item-list-class-wise returned zero rows for the sandbox tenant - the fix did not take effect")
	}
	if len(body.Rows) != wantTotal {
		t.Fatalf("listing-item-list-class-wise row count = %d, want %d (raw master_records item_category count for the sandbox tenant)",
			len(body.Rows), wantTotal)
	}

	for _, row := range body.Rows {
		if row.Party != "item_category" {
			t.Errorf("listing-item-list-class-wise row for %q has Kind column %q, want \"item_category\"", row.Item, row.Party)
		}
		if row.Item == "" {
			t.Errorf("listing-item-list-class-wise row has a blank Name column")
		}
	}
}

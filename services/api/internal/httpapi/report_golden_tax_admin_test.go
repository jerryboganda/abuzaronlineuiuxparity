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

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestPhaseGoldenTaxAdminLeavesResolveToExpectedMode locks the registry
// wiring for the 13 Phase Q tax/admin leaves verified in
// docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md against
// independent psql cross-checks on business_document_lines/business_documents/
// master_parties/master_items/master_manufacturers/roles/group_rights/users.
func TestPhaseGoldenTaxAdminLeavesResolveToExpectedMode(t *testing.T) {
	financeTests := map[string]string{
		"customer-sales-customer-wise-advance-tax":                            "tax-advance",
		"supplier-wise-advance-income-tax":                                    "tax-advance-input",
		"customer-sales-customer-category-wise-sales-output-sales-tax-report": "tax-output",
		"customer-sales-customer-ntn-wise-sales-tax-report":                   "tax-output",
		"sales-tax-report":                              "tax-output",
		"supplier-category-wise-input-sales-tax-report": "tax-input",
	}
	for kind, wantMode := range financeTests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if spec.financeMode != wantMode {
			t.Errorf("%s: financeMode = %q, want %q", kind, spec.financeMode, wantMode)
		}
	}

	adminTests := map[string]string{
		"listing-supplier-list":               "supplier",
		"listing-items-list":                  "item",
		"listing-manufacturer-list":           "manufacturer",
		"listing-group-rights-list":           "roles",
		"listing-item-list-class-wise":        "item_category",
		"listing-groupwise-user-list":         "users",
		"listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict": "users",
	}
	for kind, wantKind := range adminTests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if spec.adminKind != wantKind {
			t.Errorf("%s: adminKind = %q, want %q", kind, spec.adminKind, wantKind)
		}
	}
}

// TestAdvanceTaxFallbackFindsRateOnAnyLine documents the fix (2026-08-09,
// bug-fix wave) for a confirmed golden-verification bug (see doc
// §"supplier-wise-advance-income-tax"): the tax-advance/tax-advance-input
// LATERAL fallback used to attach the invoice-level legacy AdvanceTaxAmt only
// to the document's lowest line_number, while the outer WHERE separately
// required THAT SAME physical line to carry advance_tax_rate > 0. Independent
// psql verification against the legacy-reference-sandbox tenant found 94
// posted purchase documents (PKR 9,589.64 of advance tax; e.g.
// document_number 2864, PKR 1,574.17) where a non-first line carried the
// positive advance_tax_rate while the first line's rate was 0 - those
// invoices' advance tax silently vanished from the report instead of
// appearing on any row. The inner MIN(line_number) subquery now also
// requires advance_tax_rate > 0, so the fallback lands on the first line
// that actually carries the rate, matching the outer filter. This test now
// asserts the fixed shape; re-verified after the fix that the 94-document/
// PKR 9,589.64 gap is recovered (see report_fix_advance_tax_test.go).
func TestAdvanceTaxFallbackFindsRateOnAnyLine(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	for _, mode := range []string{"tax-advance", "tax-advance-input"} {
		query := financeReadModelQuery(mode, pagination)
		if query == "" {
			t.Fatalf("mode %q: empty query", mode)
		}
		if !containsAll(query,
			"l.line_number = (",
			"SELECT MIN(l2.line_number)",
			"d.legacy_payload->>'AdvanceTaxAmt'",
			"AND advance_tax.amount > 0 AND l.advance_tax_rate > 0",
		) {
			t.Errorf("mode %q: expected query to still fall back to the document's MIN(line_number) "+
				"row that carries advance_tax_rate > 0, and to require l.advance_tax_rate > 0 on that "+
				"same outer row - query changed, re-verify against "+
				"docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md before updating this test:\n%s",
				mode, query)
		}
		if !strings.Contains(query, "l2.advance_tax_rate > 0") {
			t.Errorf("mode %q: expected the inner MIN(line_number) subquery to also require "+
				"advance_tax_rate > 0 (the 2026-08-09 fix) so the fallback lands on a line that "+
				"actually carries the rate, not just the first line by number:\n%s", mode, query)
		}
	}
}

// TestGroupRightsListMatchesGroupRightsAfterJoinFix documents the fix for a
// confirmed golden-verification bug: "listing-group-rights-list" (adminKind
// "roles") was wired to LEFT JOIN role_permissions, a modern/runtime RBAC
// grant table that holds 0 rows for the legacy-reference-sandbox tenant,
// instead of group_rights, the table actually populated by migration with
// the full legacy right-code data (726 rows across 4 roles: ADMINISTRATOR
// 486, SHIFT INCHARGE 123, SALES OFFICER 111, REMOTE 6 - matching the 486
// legacy right-code figure documented for Phase R). adminReadModelQuery's
// "roles" case now JOINs group_rights instead of role_permissions, so the
// report surfaces the true 726-row group-rights list. The first half of this
// test locks the fixed query shape; the second half (skipped unless
// DATABASE_URL is configured) cross-checks the live HTTP report output
// against an independent group_rights count for the legacy-reference-sandbox
// tenant.
func TestGroupRightsListMatchesGroupRightsAfterJoinFix(t *testing.T) {
	query := adminReadModelQuery("roles", "LIMIT $6 OFFSET $7")
	if query == "" {
		t.Fatal("empty query for adminKind \"roles\"")
	}
	if !containsAll(query, "FROM roles r", "JOIN group_rights gr") {
		t.Fatalf("expected the roles admin query to read FROM roles JOIN group_rights "+
			"(the fixed query shape) - query changed, re-verify against "+
			"docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md before updating this test:\n%s", query)
	}
	if strings.Contains(query, "role_permissions") {
		t.Errorf("roles admin query still references role_permissions, the empty table this bug fix "+
			"was meant to stop reading from:\n%s", query)
	}

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

	wantByRole := map[string]int{}
	wantTotal := 0
	roleRows, err := database.QueryContext(ctx, `
		SELECT r.code, count(*)
		FROM group_rights gr
		JOIN roles r ON r.tenant_id = gr.tenant_id AND r.id = gr.role_id
		WHERE gr.tenant_id = $1::uuid
		GROUP BY r.code
	`, sandboxTenantID)
	if err != nil {
		t.Fatalf("independent baseline per-role count query failed: %v", err)
	}
	for roleRows.Next() {
		var code string
		var n int
		if err := roleRows.Scan(&code, &n); err != nil {
			roleRows.Close()
			t.Fatalf("scan per-role baseline row: %v", err)
		}
		wantByRole[code] = n
		wantTotal += n
	}
	if err := roleRows.Err(); err != nil {
		t.Fatalf("per-role baseline query: %v", err)
	}
	roleRows.Close()

	if wantTotal != 726 {
		t.Fatalf("group_rights row count for the sandbox tenant = %d, want 726 (docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md figure) -- re-verify before trusting this test", wantTotal)
	}
	for code, want := range map[string]int{
		"ADMINISTRATOR":  486,
		"SHIFT INCHARGE": 123,
		"SALES OFFICER":  111,
		"REMOTE":         6,
	} {
		if wantByRole[code] != want {
			t.Fatalf("independent baseline group_rights count for role %q = %d, want %d (docs figure) -- re-verify before trusting this test", code, wantByRole[code], want)
		}
	}

	operator := &sessionContext{
		UserID:    "00000000-0000-0000-0000-0000000000e1",
		TenantID:  sandboxTenantID,
		BranchID:  sandboxBranchID,
		CounterID: "00000000-0000-0000-0000-0000000000e2",
		Roles:     []string{"tenant_admin"},
	}
	server := &Server{database: database}

	request := readModelRequest(http.MethodGet, "/v1/reports/listing-group-rights-list?pageSize=1000", operator)
	request.SetPathValue("kind", "listing-group-rights-list")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("listing-group-rights-list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows    []reportRow `json:"rows"`
		HasMore bool        `json:"hasMore"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.HasMore {
		t.Fatalf("listing-group-rights-list returned hasMore=true with pageSize=1000; the 726-row expectation would only cover a sample")
	}
	if len(body.Rows) != wantTotal {
		t.Fatalf("listing-group-rights-list row count = %d, want %d (raw group_rights count for the sandbox tenant)", len(body.Rows), wantTotal)
	}

	gotByRole := map[string]int{}
	for _, row := range body.Rows {
		gotByRole[row.Document]++
		if row.Item == "" {
			t.Errorf("listing-group-rights-list row for group %q has a blank Right column - expected group_rights.right_code", row.Document)
		}
	}
	for code, want := range wantByRole {
		if gotByRole[code] != want {
			t.Errorf("listing-group-rights-list row count for group %q = %d, want %d", code, gotByRole[code], want)
		}
	}
}

// The former TestItemClassAdminKindCanNeverMatchAMasterRecordsRow (which
// locked the "listing-item-list-class-wise" adminKind "item_class" bug as a
// regression guard) was superseded 2026-08-09 by the fix: the registry entry
// now uses the real, populated adminKind "item_category". See
// TestItemListClassWiseUsesRealMasterRecordsKind and
// TestItemListClassWiseReturnsRealDataForSandboxTenant in
// report_fix_item_class_listing_test.go for the tests locking the fixed
// behavior.

// TestTaxOutputModeIsSharedVerbatimAcrossDifferentlyDimensionedLeaves
// documents a confirmed golden-verification finding: "sales-tax-report",
// "customer-sales-customer-ntn-wise-sales-tax-report" (NTN wise), and
// "customer-sales-customer-category-wise-sales-output-sales-tax-report"
// (category wise) all resolve to the identical financeMode "tax-output" and
// therefore execute byte-identical SQL with no NTN or customer-category
// column, filter, or grouping at all - despite master_parties.payload
// carrying NTNNo and CustCatCode for every migrated customer. Independent
// psql verification against the sandbox tenant found 28,184 tax-bearing
// sale lines, 0 of which have a populated NTNNo, meaning a correctly
// NTN-filtered report should currently render empty while the shared query
// renders all 28,184 lines. This test locks the current shared-query shape
// as a regression guard.
func TestTaxOutputModeIsSharedVerbatimAcrossDifferentlyDimensionedLeaves(t *testing.T) {
	sharedKinds := []string{
		"sales-tax-report",
		"customer-sales-customer-ntn-wise-sales-tax-report",
		"customer-sales-customer-category-wise-sales-output-sales-tax-report",
	}
	var first string
	for i, kind := range sharedKinds {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if spec.financeMode != "tax-output" {
			t.Fatalf("%s: financeMode = %q, want \"tax-output\"", kind, spec.financeMode)
		}
		query := financeReadModelQuery(spec.financeMode, "LIMIT $6 OFFSET $7")
		if i == 0 {
			first = query
			continue
		}
		if query != first {
			t.Errorf("%s: tax-output query diverged from %s - if this is an intentional NTN/category "+
				"dimensioning fix, update docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md",
				kind, sharedKinds[0])
		}
	}
	if containsAll(first, "NTNNo") || containsAll(first, "CustCatCode") {
		t.Errorf("tax-output query now references NTNNo/CustCatCode - if this is an intentional fix, "+
			"update docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md accordingly:\n%s", first)
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

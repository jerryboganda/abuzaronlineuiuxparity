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

// TestAdvanceTaxFallbackFindsRateOnAnyLineNotJustFirst is the regression
// guard for the fix to the bug documented in
// docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md,
// "supplier-wise-advance-income-tax" (financeMode "tax-advance-input"):
// the LATERAL fallback that attaches invoice-level legacy AdvanceTaxAmt to a
// document was gated to fire only on the document's MIN(line_number) row,
// while the outer WHERE separately required THAT SAME physical row to carry
// advance_tax_rate > 0. Because advance_tax_rate is populated per line item
// (not necessarily on the first line), 94 posted purchase documents tenant-
// wide (PKR 9,589.64; e.g. document_number 2864, PKR 1,574.17) had their
// advance tax silently dropped from the report.
//
// The fix changes the MIN(line_number) subquery to only consider lines that
// already carry advance_tax_rate > 0, so the fallback amount lands on the
// same physical row the outer filter selects, regardless of where that row
// sits in the document's line ordering.
func TestAdvanceTaxFallbackFindsRateOnAnyLineNotJustFirst(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	for _, mode := range []string{"tax-advance", "tax-advance-input"} {
		query := financeReadModelQuery(mode, pagination)
		if query == "" {
			t.Fatalf("mode %q: empty query", mode)
		}
		if !containsAll(query,
			"SELECT MIN(l2.line_number)",
			"AND l2.advance_tax_rate > 0",
			"d.legacy_payload->>'AdvanceTaxAmt'",
			"AND advance_tax.amount > 0 AND l.advance_tax_rate > 0",
		) {
			t.Errorf("mode %q: expected the AdvanceTaxAmt fallback's MIN(line_number) subquery to be "+
				"restricted to lines with advance_tax_rate > 0, so it lands on the same physical row the "+
				"outer WHERE selects - query changed, re-verify against "+
				"docs/PHASE_Q_GOLDEN_VERIFICATION_TAX_ADMIN_2026-08-09.md before updating this test:\n%s",
				mode, query)
		}
		if strings.Contains(query, "AND l2.document_id = d.id\n\t\t\t\t                       )") {
			t.Errorf("mode %q: MIN(line_number) subquery still closes without an advance_tax_rate filter "+
				"(the pre-fix defective shape)", mode)
		}
	}
}

// TestAdvanceTaxFallbackRecoversDroppedDocuments cross-checks the fixed
// tax-advance-input report against the legacy-reference-sandbox tenant's
// data via a live HTTP call to the report handler, using document_number
// 2864 (documented PKR 1,574.17 missed advance tax; its advance_tax_rate
// lives on line_number 120982, not the document's first line 120981) as the
// concrete repro. Skipped unless DATABASE_URL is configured.
func TestAdvanceTaxFallbackRecoversDroppedDocuments(t *testing.T) {
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

	// RLS is enforced on this connection role, so a dedicated *sql.Conn
	// carrying the tenant/branch GUCs (per the standard verification
	// preamble) is needed for the independent baseline queries below.
	conn, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, false)`, sandboxTenantID); err != nil {
		t.Fatalf("set RLS session config (app.tenant_id): %v", err)
	}
	for _, stmt := range []string{
		`SELECT set_config('app.branch_id', '', false)`,
		`SELECT set_config('app.allow_tenant_scope', 'true', false)`,
		`SELECT set_config('app.authenticating', 'false', false)`,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("set RLS session config (%s): %v", stmt, err)
		}
	}

	var branchExists bool
	if err := conn.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM branches WHERE id = $1::uuid AND tenant_id = $2::uuid)`,
		sandboxBranchID, sandboxTenantID).Scan(&branchExists); err != nil {
		t.Skipf("legacy-reference-sandbox tenant/branch not present in this database: %v", err)
	}
	if !branchExists {
		t.Skip("legacy-reference-sandbox branch not present in this database")
	}

	// Independent baseline: does document 2864 in this branch have its
	// advance_tax_rate on a non-first line, with a valid invoice-level
	// AdvanceTaxAmt? If the fixture data has drifted, skip rather than
	// assert against stale expectations.
	var wantAmount sql.NullFloat64
	err = conn.QueryRowContext(ctx, `
		SELECT (d.legacy_payload->>'AdvanceTaxAmt')::numeric
		FROM business_documents d
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
		  AND d.document_number = '2864' AND d.status = 'posted'
		  AND NULLIF(d.legacy_payload->>'AdvanceTaxAmt', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
	`, sandboxTenantID, sandboxBranchID).Scan(&wantAmount)
	if err != nil || !wantAmount.Valid {
		t.Skipf("document 2864 baseline AdvanceTaxAmt not available in this database: %v", err)
	}

	var minLine, rateLine int
	err = conn.QueryRowContext(ctx, `
		SELECT MIN(l.line_number) FROM business_document_lines l
		JOIN business_documents d ON d.id = l.document_id AND d.tenant_id = l.tenant_id AND d.branch_id = l.branch_id
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid AND d.document_number = '2864'
	`, sandboxTenantID, sandboxBranchID).Scan(&minLine)
	if err != nil {
		t.Skipf("document 2864 lines not available: %v", err)
	}
	err = conn.QueryRowContext(ctx, `
		SELECT MIN(l.line_number) FROM business_document_lines l
		JOIN business_documents d ON d.id = l.document_id AND d.tenant_id = l.tenant_id AND d.branch_id = l.branch_id
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid AND d.document_number = '2864'
		  AND l.advance_tax_rate > 0
	`, sandboxTenantID, sandboxBranchID).Scan(&rateLine)
	if err != nil {
		t.Skipf("document 2864 advance_tax_rate lines not available: %v", err)
	}
	if rateLine == minLine {
		t.Skip("document 2864's advance-tax-bearing line is now the first line in this database; " +
			"this fixture no longer reproduces the documented bug shape")
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
		"/v1/reports/supplier-wise-advance-income-tax?pageSize=1000&from=1900-01-01&to=2999-12-31&filter=2864",
		operator)
	request.SetPathValue("kind", "supplier-wise-advance-income-tax")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("supplier-wise-advance-income-tax status = %d, body = %s", recorder.Code, recorder.Body.String())
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
		if row.Document == "2864" {
			found = true
		}
	}
	if !found {
		t.Fatalf("supplier-wise-advance-income-tax: document 2864 is missing from the report after the fix "+
			"(want it present with advance tax amount %.2f, since its advance_tax_rate is on a non-first line)",
			wantAmount.Float64)
	}
}

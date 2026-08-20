package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Phase Q golden-verification tests for the financeMode-based report leaves
// assigned to this pass (see docs/evidence/PHASE_Q_GOLDEN_VERIFICATION_FINANCE_2026-08-09.md
// for the full investigation, including the 4 leaves -- voucher-register,
// withholding-tax-deduction, receivables-aging, payables-aging -- that either
// have no real data to verify against in this database or whose due-date
// bucketing is unexercised by any currently-migrated row):
//
//	accounts-reports-ledger-reports-accounts-ledger (financeMode="gl")
//	gl-journal (financeMode="historical-gl")
//	trial-balance (financeMode="trial-balance")
//	supplier-statement (financeMode="party-supplier")
//	tax-register (financeMode="tax-output")
//
// Unlike most golden tests in this package, this one deliberately does NOT
// seed an isolated tenant: it reads the pre-existing "legacy-reference-sandbox"
// tenant (id eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee, branch
// ffffffff-ffff-ffff-ffff-ffffffffffff), which was populated by the Phase E
// bulk-historical import from legacy SQL Server. It is strictly READ-ONLY --
// it seeds nothing and cleans up nothing -- and cross-checks each report
// leaf's real HTTP output against an independently hand-written SQL
// aggregate over the same source tables (historical_gl_entries, gl_journals/
// gl_lines, party_ledger_entries, business_documents/business_document_lines),
// using a query window narrow enough that every matching row fits on one
// report page (<=1000 rows), so the comparison covers every row rather than
// a sample.
func TestPhaseQFinanceGoldenVerification(t *testing.T) {
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

	operator := &sessionContext{
		UserID:    "00000000-0000-0000-0000-0000000000e1",
		TenantID:  sandboxTenantID,
		BranchID:  sandboxBranchID,
		CounterID: "00000000-0000-0000-0000-0000000000e2",
		Roles:     []string{"tenant_admin"},
	}
	server := &Server{database: database}

	fetchReport := func(t *testing.T, kind, query string) []reportRow {
		t.Helper()
		request := readModelRequest(http.MethodGet, "/v1/reports/"+kind+"?"+query+"&pageSize=1000", operator)
		request.SetPathValue("kind", kind)
		recorder := httptest.NewRecorder()
		server.report(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", kind, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Rows    []reportRow `json:"rows"`
			HasMore bool        `json:"hasMore"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("%s decode: %v", kind, err)
		}
		if body.HasMore {
			t.Fatalf("%s returned hasMore=true; the chosen window has more than 1000 rows, so this golden check would only cover a sample -- narrow the window instead of trusting a partial comparison", kind)
		}
		return body.Rows
	}

	sumDecimal := func(values ...string) *big.Rat {
		total := new(big.Rat)
		for _, value := range values {
			parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value))
			if !ok {
				continue
			}
			total.Add(total, parsed)
		}
		return total
	}
	scanScalarRat := func(t *testing.T, query string, args ...any) *big.Rat {
		t.Helper()
		var text string
		if err := database.QueryRowContext(ctx, query, args...).Scan(&text); err != nil {
			t.Fatalf("independent baseline query failed: %v\n%s", err, query)
		}
		value, ok := new(big.Rat).SetString(strings.TrimSpace(text))
		if !ok {
			t.Fatalf("independent baseline query returned non-numeric %q", text)
		}
		return value
	}
	scanScalarInt := func(t *testing.T, query string, args ...any) int {
		t.Helper()
		var n int
		if err := database.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("independent baseline count query failed: %v\n%s", err, query)
		}
		return n
	}

	// --- gl-journal (historical-gl): 2025-01-01 is a low-volume day (244
	// historical_gl_entries rows for this tenant/branch, comfortably under
	// the 1000-row page cap) used as an exhaustive, not sampled, check. ---
	t.Run("gl-journal historical-gl matches historical_gl_entries for a single day", func(t *testing.T) {
		wantCount := scanScalarInt(t, `
			SELECT count(*) FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)
		wantDebit := scanScalarRat(t, `
			SELECT COALESCE(sum(debit_amount), 0)::text FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)
		wantCredit := scanScalarRat(t, `
			SELECT COALESCE(sum(credit_amount), 0)::text FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)

		rows := fetchReport(t, "gl-journal", "from=2025-01-01&to=2025-01-01")
		if len(rows) != wantCount {
			t.Fatalf("gl-journal row count = %d, want %d (raw historical_gl_entries count for the same day)", len(rows), wantCount)
		}
		gotDebit, gotCredit := new(big.Rat), new(big.Rat)
		for _, row := range rows {
			gotDebit.Add(gotDebit, sumDecimal(row.Quantity)) // Quantity carries debit_amount for historical-gl rows
			gotCredit.Add(gotCredit, sumDecimal(row.Amount)) // Amount carries credit_amount for historical-gl rows
		}
		if gotDebit.Cmp(wantDebit) != 0 {
			t.Errorf("gl-journal sum(debit) = %s, want %s", gotDebit.RatString(), wantDebit.RatString())
		}
		if gotCredit.Cmp(wantCredit) != 0 {
			t.Errorf("gl-journal sum(credit) = %s, want %s", gotCredit.RatString(), wantCredit.RatString())
		}
	})

	// --- accounts-reports-ledger-reports-accounts-ledger ("gl" mode) and
	// trial-balance: cross-checked against the SAME independent baseline as
	// gl-journal above, because gl_journals/gl_lines is empty for this
	// tenant (confirmed separately), so all three modes reduce to the same
	// historical_gl_entries slice for this window. ---
	t.Run("accounts-ledger gl mode matches historical_gl_entries for the same day", func(t *testing.T) {
		wantDebit := scanScalarRat(t, `
			SELECT COALESCE(sum(debit_amount), 0)::text FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)
		wantCredit := scanScalarRat(t, `
			SELECT COALESCE(sum(credit_amount), 0)::text FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)

		rows := fetchReport(t, "accounts-reports-ledger-reports-accounts-ledger", "from=2025-01-01&to=2025-01-01")
		gotDebit, gotCredit := new(big.Rat), new(big.Rat)
		for _, row := range rows {
			gotDebit.Add(gotDebit, sumDecimal(row.Quantity))
			gotCredit.Add(gotCredit, sumDecimal(row.Amount))
		}
		if gotDebit.Cmp(wantDebit) != 0 || gotCredit.Cmp(wantCredit) != 0 {
			t.Errorf("accounts-ledger sum(debit,credit) = (%s,%s), want (%s,%s)", gotDebit.RatString(), gotCredit.RatString(), wantDebit.RatString(), wantCredit.RatString())
		}
	})

	t.Run("trial-balance grouped totals reconcile to the same day's ungrouped totals", func(t *testing.T) {
		wantDebit := scanScalarRat(t, `
			SELECT COALESCE(sum(debit_amount), 0)::text FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)
		wantCredit := scanScalarRat(t, `
			SELECT COALESCE(sum(credit_amount), 0)::text FROM historical_gl_entries
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
			  AND occurred_at >= '2025-01-01'::date AND occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)

		rows := fetchReport(t, "trial-balance", "from=2025-01-01&to=2025-01-01")
		if len(rows) == 0 {
			t.Fatal("trial-balance returned no account groups for a day known to have GL activity")
		}
		gotDebit, gotCredit := new(big.Rat), new(big.Rat)
		for _, row := range rows {
			gotDebit.Add(gotDebit, sumDecimal(row.Quantity))
			gotCredit.Add(gotCredit, sumDecimal(row.Amount))
		}
		if gotDebit.Cmp(wantDebit) != 0 || gotCredit.Cmp(wantCredit) != 0 {
			t.Errorf("trial-balance re-summed groups = (%s,%s), want ungrouped totals (%s,%s) -- grouping by account must not lose or double-count any debit/credit value", gotDebit.RatString(), gotCredit.RatString(), wantDebit.RatString(), wantCredit.RatString())
		}
	})

	// --- supplier-statement: January 2025 has 392 posted supplier
	// party_ledger_entries for this tenant across 30+ real distinct
	// suppliers, comfortably under the page cap -- an exhaustive, not
	// sampled, check across real substantial data. (An earlier version of
	// this test tried to isolate one supplier via the text filter param,
	// e.g. filter=112, but that param ILIKE-matches document_id, party_name,
	// AND description -- and document_id is a UUID whose hex digits
	// routinely contain "112" as a substring by chance, so it silently
	// over-matched unrelated suppliers' rows. Comparing the full unfiltered
	// window instead avoids that footgun.) ---
	t.Run("supplier-statement matches party_ledger_entries for a real multi-supplier window", func(t *testing.T) {
		wantCount := scanScalarInt(t, `
			SELECT count(*) FROM party_ledger_entries ple
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = 'supplier'
			  AND ple.occurred_at >= '2025-01-01'::date AND ple.occurred_at < '2025-02-01'::date
		`, sandboxTenantID, sandboxBranchID)
		wantDebit := scanScalarRat(t, `
			SELECT COALESCE(sum(ple.debit_amount), 0)::text FROM party_ledger_entries ple
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = 'supplier'
			  AND ple.occurred_at >= '2025-01-01'::date AND ple.occurred_at < '2025-02-01'::date
		`, sandboxTenantID, sandboxBranchID)
		wantCredit := scanScalarRat(t, `
			SELECT COALESCE(sum(ple.credit_amount), 0)::text FROM party_ledger_entries ple
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = 'supplier'
			  AND ple.occurred_at >= '2025-01-01'::date AND ple.occurred_at < '2025-02-01'::date
		`, sandboxTenantID, sandboxBranchID)

		rows := fetchReport(t, "supplier-statement", "from=2025-01-01&to=2025-01-31")
		if len(rows) != wantCount {
			t.Fatalf("supplier-statement row count = %d, want %d (raw party_ledger_entries count for January 2025)", len(rows), wantCount)
		}
		distinctParties := map[string]bool{}
		gotDebit, gotCredit := new(big.Rat), new(big.Rat)
		for _, row := range rows {
			distinctParties[row.Party] = true
			gotDebit.Add(gotDebit, sumDecimal(row.Quantity))
			gotCredit.Add(gotCredit, sumDecimal(row.Amount))
		}
		if len(distinctParties) < 3 {
			t.Errorf("supplier-statement returned only %d distinct suppliers for January 2025, want a real multi-supplier sample (>=3)", len(distinctParties))
		}
		if gotDebit.Cmp(wantDebit) != 0 || gotCredit.Cmp(wantCredit) != 0 {
			t.Errorf("supplier-statement sum(debit,credit) = (%s,%s), want (%s,%s)", gotDebit.RatString(), gotCredit.RatString(), wantDebit.RatString(), wantCredit.RatString())
		}
	})

	// --- tax-register (tax-output): 2025-01-01 has only 6 taxed
	// cash-sale/credit-sale lines for this tenant, an exhaustive not
	// sampled check. ---
	t.Run("tax-register matches business_document_lines for a single day", func(t *testing.T) {
		wantCount := scanScalarInt(t, `
			SELECT count(*) FROM business_documents d
			JOIN business_document_lines l ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid AND d.status = 'posted'
			  AND d.kind IN ('cash-sale', 'credit-sale') AND l.tax_amount > 0
			  AND d.occurred_at >= '2025-01-01'::date AND d.occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)
		wantBase := scanScalarRat(t, `
			SELECT COALESCE(sum(l.line_total), 0)::text FROM business_documents d
			JOIN business_document_lines l ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid AND d.status = 'posted'
			  AND d.kind IN ('cash-sale', 'credit-sale') AND l.tax_amount > 0
			  AND d.occurred_at >= '2025-01-01'::date AND d.occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)
		wantTax := scanScalarRat(t, `
			SELECT COALESCE(sum(l.tax_amount), 0)::text FROM business_documents d
			JOIN business_document_lines l ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid AND d.status = 'posted'
			  AND d.kind IN ('cash-sale', 'credit-sale') AND l.tax_amount > 0
			  AND d.occurred_at >= '2025-01-01'::date AND d.occurred_at < '2025-01-02'::date
		`, sandboxTenantID, sandboxBranchID)

		rows := fetchReport(t, "tax-register", "from=2025-01-01&to=2025-01-01")
		if len(rows) != wantCount {
			t.Fatalf("tax-register row count = %d, want %d (raw taxed-line count for the same day)", len(rows), wantCount)
		}
		gotBase, gotTax := new(big.Rat), new(big.Rat)
		for _, row := range rows {
			gotBase.Add(gotBase, sumDecimal(row.Quantity)) // Quantity carries the taxable base for tax-output rows
			gotTax.Add(gotTax, sumDecimal(row.Amount))     // Amount carries the tax amount for tax-output rows
		}
		if gotBase.Cmp(wantBase) != 0 {
			t.Errorf("tax-register sum(taxable base) = %s, want %s", gotBase.RatString(), wantBase.RatString())
		}
		if gotTax.Cmp(wantTax) != 0 {
			t.Errorf("tax-register sum(tax) = %s, want %s", gotTax.RatString(), wantTax.RatString())
		}
	})
}

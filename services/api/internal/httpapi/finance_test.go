package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuzar/abuzar-next/services/api/internal/pricing"
)

func TestFinanceTaxAmountUsesSuppliedPricingTaxes(t *testing.T) {
	raw := mustJSON(map[string]any{
		"taxes": []map[string]any{
			{"kind": "gst", "amount": "1.10"},
			{"kind": "pct", "amount": "0.25"},
		},
	})
	got, err := financeTaxAmount(raw)
	if err != nil {
		t.Fatalf("finance tax amount: %v", err)
	}
	if got != pricing.Money(135) {
		t.Fatalf("finance tax amount = %s, want 1.35", got)
	}
	if _, err := financeTaxAmount([]byte(`{"taxes":[{"amount":"not-money"}]}`)); err == nil {
		t.Fatal("invalid pricing tax amount was accepted")
	}
}

func TestFinanceReadsRemainAuthenticated(t *testing.T) {
	handler := New(nil, "test", "")
	for _, path := range []string{
		"/v1/finance/accounts",
		"/v1/finance/journals",
		"/v1/finance/ledger?partyId=00000000-0000-0000-0000-000000000001",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s returned %d, want %d", path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestFinanceMigrationDefinesScopedBalancedPostingAndSafeSeeds(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "013_finance_ledgers.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read finance migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS finance_account_categories",
		"CREATE TABLE IF NOT EXISTS finance_accounts",
		"CREATE TABLE IF NOT EXISTS gl_journals",
		"CREATE TABLE IF NOT EXISTS gl_lines",
		"CREATE TABLE IF NOT EXISTS party_ledger_entries",
		"CREATE TABLE IF NOT EXISTS party_ledger_balances",
		"CREATE TABLE IF NOT EXISTS voucher_categories",
		"CREATE TABLE IF NOT EXISTS voucher_entries",
		"UNIQUE (tenant_id, branch_id, source_event_id)",
		"UNIQUE (tenant_id, branch_id, source_document_id)",
		"CREATE CONSTRAINT TRIGGER gl_journals_balance_check",
		"ALTER TABLE %I FORCE ROW LEVEL SECURITY",
		"CREATE TRIGGER tenants_seed_finance_chart",
		"Sales revenue",
		"Cost of goods sold",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("finance migration is missing contract %q", required)
		}
	}
	if strings.Contains(migration, "opening_balance") || strings.Contains(migration, "legacy_balance") {
		t.Fatal("finance migration invents or imports historical balances")
	}
}

func TestFinanceLedgerIncludesSourceBackedPaymentAllocations(t *testing.T) {
	path := filepath.Join(".", "finance.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read finance handler: %v", err)
	}
	code := string(data)
	for _, required := range []string{
		"historical_party_payment_allocations",
		"historical_party_ledger_adjustments",
		"historical_party_return_allocations",
		"running_entries AS",
		"SUM(debit_amount - credit_amount) OVER",
		"ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW",
		"p.posted = true",
		"p.payment_amount <> 0",
		"a.posted = true",
		"receivable-adjustment",
		"return-allocation",
		"UNION ALL",
		"source_document_legacy_id",
	} {
		if !strings.Contains(code, required) {
			t.Errorf("finance ledger handler is missing source payment fragment %q", required)
		}
	}
}

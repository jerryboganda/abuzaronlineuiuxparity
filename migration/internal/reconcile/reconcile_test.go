package reconcile

import (
	"strings"
	"testing"
)

func TestIdentifierQuotingEscapesDelimiters(t *testing.T) {
	if got := QuoteSQLServer("a]b"); got != "[a]]b]" {
		t.Fatalf("SQL Server quote = %q", got)
	}
	if got := QuotePostgres(`a"b`); got != `"a""b"` {
		t.Fatalf("PostgreSQL quote = %q", got)
	}
}
func TestReadOnlyMetricQuery(t *testing.T) {
	valid := []string{
		"SELECT COUNT(*) FROM dbo.sales",
		" select COALESCE(SUM(total), 0) FROM public.sales_documents",
		"SELECT updated_at FROM public.sales_documents",
	}
	for _, query := range valid {
		if !ReadOnlySelect(query) {
			t.Fatalf("expected read-only query: %q", query)
		}
	}
	invalid := []string{
		"UPDATE sales SET total = 0",
		"SELECT COUNT(*) FROM sales; DELETE FROM sales",
		"DROP TABLE sales",
	}
	for _, query := range invalid {
		if ReadOnlySelect(query) {
			t.Fatalf("expected query to be rejected: %q", query)
		}
	}
}

func TestDecimalMetricString(t *testing.T) {
	if got := (DecimalMetric{Raw: "12.50", Value: 12.5}).String(); got != "12.50" {
		t.Fatalf("raw metric string = %q", got)
	}
	if got := (DecimalMetric{Value: 12.5}).String(); got != "12.50000000" {
		t.Fatalf("formatted metric string = %q", got)
	}
}

func TestValidateSourceDatabase(t *testing.T) {
	if err := ValidateSourceDatabase("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference", false); err == nil {
		t.Fatal("canonical database was accepted for a reviewed mapping")
	}
	if err := ValidateSourceDatabase("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference", true); err != nil {
		t.Fatalf("explicit canonical opt-in was rejected: %v", err)
	}
	if err := ValidateSourceDatabase("sqlserver://127.0.0.1?database=AbuzarLegacyReference", "AbuzarLegacyReference", false); err != nil {
		t.Fatalf("sandbox database was rejected: %v", err)
	}
}

func TestApplyTenantOverrideRewritesMappingScope(t *testing.T) {
	config := MappingConfig{TenantID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", Tables: []TableMapping{{Inject: map[string]string{"tenant_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "kind": "item"}}}}
	ApplyMappingScopeOverrides(&config, "11111111-2222-3333-4444-555555555555", "", "")
	if config.TenantID != "11111111-2222-3333-4444-555555555555" || config.Tables[0].Inject["tenant_id"] != config.TenantID {
		t.Fatalf("tenant scope was not rewritten: %+v", config)
	}
}

func TestApplyMappingScopeOverridesRewritesBranchAndCounter(t *testing.T) {
	config := MappingConfig{TenantID: "tenant", Tables: []TableMapping{{Inject: map[string]string{"tenant_id": "tenant", "branch_id": "branch", "counter_id": "counter"}}}}
	ApplyMappingScopeOverrides(&config, "new-tenant", "new-branch", "new-counter")
	inject := config.Tables[0].Inject
	if inject["tenant_id"] != "new-tenant" || inject["branch_id"] != "new-branch" || inject["counter_id"] != "new-counter" {
		t.Fatalf("scope injections were not rewritten: %+v", inject)
	}
}

func TestRewriteMetricTenantOnlyReplacesReviewedSandboxLiteral(t *testing.T) {
	query := "SELECT COUNT(*) FROM public.users WHERE tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'"
	got := RewriteMetricTenant(query, "11111111-2222-3333-4444-555555555555")
	want := "SELECT COUNT(*) FROM public.users WHERE tenant_id = '11111111-2222-3333-4444-555555555555'"
	if got != want {
		t.Fatalf("metric tenant rewrite = %q, want %q", got, want)
	}
	if RewriteMetricTenant(query, "") != query {
		t.Fatal("empty metric tenant unexpectedly changed query")
	}
}

func TestBookkeepingStatusRequiresBothExceptionTablesToBeClear(t *testing.T) {
	tests := []struct {
		name            string
		openExceptions  int64
		openAmbiguities int64
		want            string
	}{
		{name: "clear", want: "clear"},
		{name: "open migration exception", openExceptions: 1, want: "review_required"},
		{name: "open ambiguity", openAmbiguities: 1, want: "review_required"},
		{name: "both open", openExceptions: 2, openAmbiguities: 3, want: "review_required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := BookkeepingStatus(test.openExceptions, test.openAmbiguities); got != test.want {
				t.Fatalf("bookkeeping status = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBookkeepingCountsDistinctOpenSourceCases(t *testing.T) {
	if !containsAll(BookkeepingMigrationExceptionQuery,
		"COUNT(*) AS open_rows",
		"COUNT(DISTINCT (source_schema, source_table, legacy_id, reason_code)) AS open_cases",
		"status = 'open'",
	) {
		t.Fatal("bookkeeping query does not expose raw rows and distinct open cases")
	}
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}

func TestQueryMetricTypes(t *testing.T) {
	// Test byte slice conversion
	mBytes := DecimalMetric{Value: 123.456, Raw: string([]byte("123.456"))}
	if mBytes.String() != "123.456" {
		t.Fatalf("byte slice raw string mismatch: got %s, want 123.456", mBytes.String())
	}
	if mBytes.Value != 123.456 {
		t.Fatalf("byte slice value mismatch: got %f, want 123.456", mBytes.Value)
	}

	// Test zero value default float formatting
	mZero := DecimalMetric{Value: 0}
	if mZero.String() != "0.00000000" {
		t.Fatalf("zero value string mismatch: got %s, want 0.00000000", mZero.String())
	}
}

func TestReadOnlySelectSecurityEdgeCases(t *testing.T) {
	rejectedQueries := []string{
		"SELECT * FROM sales; DROP TABLE users",
		"SELECT * FROM sales -- comment with DELETE",
		"INSERT INTO audit_log SELECT * FROM sales",
		"UPDATE accounts SET balance = 0",
		"EXEC sp_executesql N'SELECT 1'",
		"CREATE TABLE temp (id int)",
		"TRUNCATE TABLE logs",
		"ALTER TABLE users ADD COLUMN bad int",
		"",
		"   ",
	}
	for _, q := range rejectedQueries {
		if ReadOnlySelect(q) {
			t.Fatalf("readOnlySelect unexpectedly approved dangerous/invalid query: %q", q)
		}
	}
}

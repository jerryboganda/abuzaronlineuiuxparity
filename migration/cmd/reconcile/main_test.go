package main

import "testing"

func TestIdentifierQuotingEscapesDelimiters(t *testing.T) {
	if got := quoteSQLServer("a]b"); got != "[a]]b]" {
		t.Fatalf("SQL Server quote = %q", got)
	}
	if got := quotePostgres(`a"b`); got != `"a""b"` {
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
		if !readOnlySelect(query) {
			t.Fatalf("expected read-only query: %q", query)
		}
	}
	invalid := []string{
		"UPDATE sales SET total = 0",
		"SELECT COUNT(*) FROM sales; DELETE FROM sales",
		"DROP TABLE sales",
	}
	for _, query := range invalid {
		if readOnlySelect(query) {
			t.Fatalf("expected query to be rejected: %q", query)
		}
	}
}

func TestDecimalMetricString(t *testing.T) {
	if got := (decimalMetric{Raw: "12.50", Value: 12.5}).String(); got != "12.50" {
		t.Fatalf("raw metric string = %q", got)
	}
	if got := (decimalMetric{Value: 12.5}).String(); got != "12.50000000" {
		t.Fatalf("formatted metric string = %q", got)
	}
}

func TestValidateSourceDatabase(t *testing.T) {
	if err := validateSourceDatabase("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference", false); err == nil {
		t.Fatal("canonical database was accepted for a reviewed mapping")
	}
	if err := validateSourceDatabase("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference", true); err != nil {
		t.Fatalf("explicit canonical opt-in was rejected: %v", err)
	}
	if err := validateSourceDatabase("sqlserver://127.0.0.1?database=AbuzarLegacyReference", "AbuzarLegacyReference", false); err != nil {
		t.Fatalf("sandbox database was rejected: %v", err)
	}
}

func TestApplyTenantOverrideRewritesMappingScope(t *testing.T) {
	config := mappingConfig{TenantID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", Tables: []tableMapping{{Inject: map[string]string{"tenant_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "kind": "item"}}}}
	applyMappingScopeOverrides(&config, "11111111-2222-3333-4444-555555555555", "", "")
	if config.TenantID != "11111111-2222-3333-4444-555555555555" || config.Tables[0].Inject["tenant_id"] != config.TenantID {
		t.Fatalf("tenant scope was not rewritten: %+v", config)
	}
}

func TestApplyMappingScopeOverridesRewritesBranchAndCounter(t *testing.T) {
	config := mappingConfig{TenantID: "tenant", Tables: []tableMapping{{Inject: map[string]string{"tenant_id": "tenant", "branch_id": "branch", "counter_id": "counter"}}}}
	applyMappingScopeOverrides(&config, "new-tenant", "new-branch", "new-counter")
	inject := config.Tables[0].Inject
	if inject["tenant_id"] != "new-tenant" || inject["branch_id"] != "new-branch" || inject["counter_id"] != "new-counter" {
		t.Fatalf("scope injections were not rewritten: %+v", inject)
	}
}

func TestRewriteMetricTenantOnlyReplacesReviewedSandboxLiteral(t *testing.T) {
	query := "SELECT COUNT(*) FROM public.users WHERE tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'"
	got := rewriteMetricTenant(query, "11111111-2222-3333-4444-555555555555")
	want := "SELECT COUNT(*) FROM public.users WHERE tenant_id = '11111111-2222-3333-4444-555555555555'"
	if got != want {
		t.Fatalf("metric tenant rewrite = %q, want %q", got, want)
	}
	if rewriteMetricTenant(query, "") != query {
		t.Fatal("empty metric tenant unexpectedly changed query")
	}
}

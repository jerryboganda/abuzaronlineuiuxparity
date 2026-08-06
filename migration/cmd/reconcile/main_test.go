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
	if err := validateSourceDatabase("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference"); err == nil {
		t.Fatal("canonical database was accepted for a reviewed mapping")
	}
	if err := validateSourceDatabase("sqlserver://127.0.0.1?database=AbuzarLegacyReference", "AbuzarLegacyReference"); err != nil {
		t.Fatalf("sandbox database was rejected: %v", err)
	}
}

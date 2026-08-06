package main

import "testing"

func TestImportConfigRequiresExplicitConflictKey(t *testing.T) {
	config := importConfig{TenantID: "tenant", Tables: []tableMapping{{
		Source: tableRef{Schema: "dbo", Table: "items"}, Target: tableRef{Schema: "public", Table: "products"},
		SourceID: "id", TargetID: "id", Columns: map[string]string{"id": "id"},
	}}}
	if err := config.validate(); err == nil {
		t.Fatal("mapping without conflictColumns was accepted")
	}
}

func TestImportConfigAcceptsTenantBranchInjection(t *testing.T) {
	config := importConfig{TenantID: "tenant", Tables: []tableMapping{{
		Source: tableRef{Schema: "dbo", Table: "items"}, Target: tableRef{Schema: "public", Table: "products"},
		SourceID: "legacy_id", TargetID: "id", Columns: map[string]string{"legacy_id": "legacy_id", "name": "name"},
		Inject: map[string]string{"tenant_id": "tenant", "branch_id": "branch"}, ConflictColumn: []string{"tenant_id", "legacy_id"},
	}}}
	if err := config.validate(); err != nil {
		t.Fatalf("valid mapping rejected: %v", err)
	}
}

func TestImportConfigRejectsEmptyScopeInjection(t *testing.T) {
	config := importConfig{TenantID: "tenant", Tables: []tableMapping{{
		Source: tableRef{Schema: "dbo", Table: "items"}, Target: tableRef{Schema: "public", Table: "products"},
		SourceID: "legacy_id", TargetID: "id", Columns: map[string]string{"legacy_id": "legacy_id"},
		Inject: map[string]string{"tenant_id": ""}, ConflictColumn: []string{"tenant_id", "legacy_id"},
	}}}
	if err := config.validate(); err == nil {
		t.Fatal("empty tenant scope injection was accepted")
	}
}

func TestImportConfigRejectsEmptyColumnIdentifier(t *testing.T) {
	config := importConfig{TenantID: "tenant", Tables: []tableMapping{{
		Source: tableRef{Schema: "dbo", Table: "items"}, Target: tableRef{Schema: "public", Table: "products"},
		SourceID: "legacy_id", TargetID: "id", Columns: map[string]string{"": "legacy_id"},
		ConflictColumn: []string{"legacy_id"},
	}}}
	if err := config.validate(); err == nil {
		t.Fatal("empty column identifier was accepted")
	}
}

func TestIdentifierQuotingEscapesDelimiters(t *testing.T) {
	if got := quoteSQLServer("a]b"); got != "[a]]b]" {
		t.Fatalf("SQL Server quote = %q", got)
	}
	if got := quotePostgres(`a"b`); got != `"a""b"` {
		t.Fatalf("PostgreSQL quote = %q", got)
	}
}

func TestImportSourceRejectsCanonicalDatabase(t *testing.T) {
	if err := validateImportSource("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference"); err == nil {
		t.Fatal("canonical database was accepted for import")
	}
	if err := validateImportSource("sqlserver://127.0.0.1?database=AbuzarLegacyReference", "AbuzarLegacyReference"); err != nil {
		t.Fatalf("sandbox database was rejected: %v", err)
	}
}

func TestCoerceBoolean(t *testing.T) {
	for _, value := range []any{int64(1), int32(1), 1, "true", "Y"} {
		got, err := coerceValue(value, "boolean")
		if err != nil || got != true {
			t.Fatalf("coerce %v = %v, %v", value, got, err)
		}
	}

	for _, value := range []any{int64(0), "false", "N"} {
		got, err := coerceValue(value, "boolean")
		if err != nil || got != false {
			t.Fatalf("coerce %v = %v, %v", value, got, err)
		}
	}
}

func TestImportConfigAcceptsDerivedColumnsAndLookups(t *testing.T) {
	config := importConfig{TenantID: "tenant", Tables: []tableMapping{{
		Source:            tableRef{Schema: "dbo", Table: "GroupAllowedGodown"},
		Target:            tableRef{Schema: "public", Table: "group_allowed_scopes"},
		SourceID:          "GroupCode",
		SourceIDColumns:   []string{"GroupCode", "GCode", "Module", "Priority"},
		TargetID:          "role_id",
		TargetIDGenerated: true,
		Columns: map[string]string{
			"role_id":           "GroupCode",
			"legacy_group_code": "GroupCode",
			"scope_key":         "GroupCode",
			"legacy_scope_id":   "GroupCode",
		},
		DerivedColumns: map[string][]string{
			"scope_key":       {"GroupCode", "GCode", "Module", "Priority"},
			"legacy_scope_id": {"GroupCode", "GCode", "Module", "Priority"},
		},
		Lookups: map[string]lookupSpec{
			"role_id": {
				Target:       tableRef{Schema: "public", Table: "roles"},
				TargetColumn: "legacy_group_id",
				SourceColumn: "GroupCode",
			},
		},
		Inject:         map[string]string{"tenant_id": "tenant", "scope_kind": "godown", "allowed": "true"},
		ConflictColumn: []string{"tenant_id", "role_id", "scope_kind", "scope_key"},
	}}}
	if err := config.validate(); err != nil {
		t.Fatalf("derived/lookup mapping rejected: %v", err)
	}
}

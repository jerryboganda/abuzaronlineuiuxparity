package main

import (
	"database/sql"
	"errors"
	"testing"
)

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

func TestImportConfigAcceptsReviewedUpsertPolicy(t *testing.T) {
	config := importConfig{TenantID: "tenant", Upsert: true, Tables: []tableMapping{{
		Source: tableRef{Schema: "dbo", Table: "groups"}, Target: tableRef{Schema: "public", Table: "roles"},
		SourceID: "id", TargetID: "id", Columns: map[string]string{"code": "code", "name": "name"},
		Inject: map[string]string{"tenant_id": "tenant"}, ConflictColumn: []string{"tenant_id", "code"},
	}}}
	if err := config.validate(); err != nil {
		t.Fatalf("reviewed upsert mapping rejected: %v", err)
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
	if err := validateImportSource("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference", false); err == nil {
		t.Fatal("canonical database was accepted for import")
	}
	if err := validateImportSource("sqlserver://127.0.0.1?database=FazalDinPP19DataBaseV2", "AbuzarLegacyReference", true); err != nil {
		t.Fatalf("explicit canonical opt-in was rejected: %v", err)
	}
	if err := validateImportSource("sqlserver://127.0.0.1?database=AbuzarLegacyReference", "AbuzarLegacyReference", false); err != nil {
		t.Fatalf("sandbox database was rejected: %v", err)
	}
}

func TestApplyScopeOverridesRewritesReviewedTenantInjections(t *testing.T) {
	config := importConfig{
		TenantID:      "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
		DefaultBranch: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		Tables: []tableMapping{
			{Inject: map[string]string{"tenant_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "kind": "item"}},
			{Inject: map[string]string{"tenant_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"}},
		},
	}
	if err := applyScopeOverrides(&config, "11111111-2222-3333-4444-555555555555", "66666666-7777-8888-9999-000000000000", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"); err != nil {
		t.Fatalf("apply scope overrides: %v", err)
	}
	if config.TenantID != "11111111-2222-3333-4444-555555555555" || config.DefaultBranch != "66666666-7777-8888-9999-000000000000" {
		t.Fatalf("config scope was not overridden: %+v", config)
	}
	if got := config.Tables[0].Inject["tenant_id"]; got != config.TenantID {
		t.Fatalf("tenant injection = %q, want %q", got, config.TenantID)
	}
	if _, present := config.Tables[0].Inject["branch_id"]; present {
		t.Fatal("branch_id was added to a mapping that did not declare it")
	}
}

func TestHasInjectedScopeDetectsOnlyDeclaredScope(t *testing.T) {
	config := importConfig{Tables: []tableMapping{{Inject: map[string]string{"tenant_id": "tenant", "counter_id": "counter"}}}}
	if !hasInjectedScope(config, "counter_id") {
		t.Fatal("declared counter scope was not detected")
	}
	if hasInjectedScope(config, "branch_id") {
		t.Fatal("undeclared branch scope was detected")
	}
}

func TestHasInjectedScopeInRangeIgnoresUnselectedTables(t *testing.T) {
	config := importConfig{Tables: []tableMapping{
		{Inject: map[string]string{"tenant_id": "tenant", "branch_id": "branch"}},
		{Inject: map[string]string{"tenant_id": "tenant"}},
	}}
	if hasInjectedScopeInRange(config, "branch_id", 1, 2) {
		t.Fatal("branch scope from an unselected mapping was detected")
	}
	if !hasInjectedScopeInRange(config, "branch_id", 0, 1) {
		t.Fatal("selected branch scope was not detected")
	}
}

func TestIsNoRowsAcceptsDriverText(t *testing.T) {
	if !isNoRows(sql.ErrNoRows) || !isNoRows(errors.New("sql: no rows in result set")) {
		t.Fatal("driver no-row variants were not recognized")
	}
	if isNoRows(errors.New("duplicate row could not be resolved")) {
		t.Fatal("non-no-row error was misclassified")
	}
}

func TestLookupCacheKeyIsStableAcrossPredicateOrder(t *testing.T) {
	base := lookupSpec{
		Target:       tableRef{Schema: "public", Table: "master_parties"},
		TargetColumn: "legacy_id",
		ValueColumn:  "id",
		Predicates:   map[string]string{"party_type": "supplier", "active": "true"},
	}
	reordered := lookupSpec{
		Target:       base.Target,
		TargetColumn: base.TargetColumn,
		ValueColumn:  base.ValueColumn,
		Predicates:   map[string]string{"active": "true", "party_type": "supplier"},
	}
	first := lookupCacheKey("tenant", base, "42")
	second := lookupCacheKey("tenant", reordered, "42")
	if first != second {
		t.Fatalf("lookup cache key changed with predicate order: %q != %q", first, second)
	}
	if first == lookupCacheKey("other-tenant", base, "42") {
		t.Fatal("lookup cache key ignored tenant scope")
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

func TestStableUUIDIsRestartSafeAndScoped(t *testing.T) {
	mapping := tableMapping{
		Source: tableRef{Schema: "dbo", Table: "Saledetail"},
	}
	first := stableUUID("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", mapping, "10:2", "line")
	second := stableUUID("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", mapping, "10:2", "line")
	if first == "" || first != second {
		t.Fatalf("stable UUID was not deterministic: %q %q", first, second)
	}
	if first == stableUUID("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", mapping, "10:2", "document") {
		t.Fatal("generated UUID scopes collided")
	}
}

func TestImportConfigAcceptsHistoricalExpressionsAndRangeFeatures(t *testing.T) {
	config := importConfig{TenantID: "tenant", Tables: []tableMapping{{
		Source:            tableRef{Schema: "dbo", Table: "Saledetail"},
		Target:            tableRef{Schema: "public", Table: "business_document_lines"},
		SourceID:          "RowID",
		TargetID:          "id",
		TargetIDGenerated: true,
		Columns:           map[string]string{"legacy_id": "RowID"},
		GeneratedColumns:  map[string]string{"id": "line"},
		SourceExpressions: map[string]string{
			"legacy_import_key": "CONCAT('Saledetail:', RowID)",
		},
		SourceFilter: "RowID > 0",
		Lookups: map[string]lookupSpec{
			"document_id": {
				Target:        tableRef{Schema: "public", Table: "business_documents"},
				TargetColumn:  "legacy_id",
				ValueColumn:   "id",
				SourceColumns: []string{"SaleInvcode", "RowID"},
			},
		},
		Inject:         map[string]string{"tenant_id": "tenant"},
		ConflictColumn: []string{"tenant_id", "legacy_import_key"},
	}}}
	if err := config.validate(); err != nil {
		t.Fatalf("historical mapping rejected: %v", err)
	}
}

func TestCoerceText(t *testing.T) {
	got, err := coerceValue(int64(42), "text")
	if err != nil || got != "42" {
		t.Fatalf("coerce text = %v, %v", got, err)
	}
}

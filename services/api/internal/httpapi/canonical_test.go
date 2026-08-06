package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalMasterQueriesAlwaysCarryTenantScope(t *testing.T) {
	for _, kind := range []string{"item", "customer", "supplier", "manufacturer", "item-group", "category", "godown"} {
		spec, ok := canonicalMasterSpec(kind)
		if !ok {
			t.Fatalf("canonical kind %q was not registered", kind)
		}
		query := canonicalSelect(spec)
		if !strings.Contains(query, "tenant_id = $1::uuid") {
			t.Errorf("%s query does not constrain tenant_id: %s", kind, query)
		}
	}
}

func TestItemLookupMatchesNameAliasBarcodeAndLegacyID(t *testing.T) {
	candidate := itemLookupCandidate{
		Name:     "Amoxi Capsule",
		Code:     "I-001",
		LegacyID: "42",
		Aliases:  []string{"AMOXI-500", "890123"},
	}
	for _, query := range []string{"amoxi", "amoxi-500", "890123", "42"} {
		if !itemLookupMatches(candidate, query) {
			t.Errorf("lookup %q did not match the item", query)
		}
	}
	if itemLookupMatches(candidate, "unrelated") {
		t.Fatal("unrelated lookup matched the item")
	}
}

func TestCanonicalMasterRoutesRemainAuthenticated(t *testing.T) {
	server := New(nil, "test", "")
	for _, path := range []string{
		"/v1/items/lookup?q=amoxi",
		"/v1/master/items/lookup?alias=amoxi",
		"/v1/master/item",
		"/v1/master/item/00000000-0000-0000-0000-000000000001",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/suppliers",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want %d", path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestNormalizedMasterMigrationRetainsLegacyUniquenessAndSupplierFields(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "010_master_normalized.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read normalized migration: %v", err)
	}
	migration := string(data)
	for _, constraint := range []string{
		"UNIQUE (tenant_id, legacy_id)",
		"UNIQUE (tenant_id, party_type, legacy_id)",
		"UNIQUE (tenant_id, category_kind, legacy_id)",
		"UNIQUE (tenant_id, legacy_item_id, legacy_supplier_id)",
	} {
		if !strings.Contains(migration, constraint) {
			t.Errorf("migration is missing duplicate-protection contract %q", constraint)
		}
	}
	for _, field := range []string{"priority", "rate", "discount_percent", "quantity", "bonus", "days"} {
		if !strings.Contains(migration, field) {
			t.Errorf("migration is missing item-supplier field %q", field)
		}
	}
}

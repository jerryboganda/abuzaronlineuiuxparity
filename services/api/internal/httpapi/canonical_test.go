package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestNormalizeAlternateItemAliasesKeepsOrderAndRejectsAmbiguity(t *testing.T) {
	aliases, err := normalizeAlternateItemAliases([]string{"  BOX-1  ", "Box-2"})
	if err != nil {
		t.Fatalf("normalize alternate aliases: %v", err)
	}
	if strings.Join(aliases, "|") != "BOX-1|Box-2" {
		t.Fatalf("normalized aliases = %#v", aliases)
	}
	for _, values := range [][]string{{""}, {"Alias", " alias "}} {
		if _, err := normalizeAlternateItemAliases(values); err == nil {
			t.Errorf("normalize alternate aliases accepted invalid values %#v", values)
		}
	}
	tooMany := make([]string, maxAlternateItemAliases+1)
	for index := range tooMany {
		tooMany[index] = "alias-" + strconv.Itoa(index)
	}
	if _, err := normalizeAlternateItemAliases(tooMany); err == nil {
		t.Fatal("normalize alternate aliases accepted more than the bounded maximum")
	}
}

func TestNormalizeItemImagesPreservesRowsAndBoundsBlobPayloads(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("image-bytes"))
	images, err := normalizeItemImages([]itemImageRequest{{ImageData: encoded, ImageDescription: " front ", ImageType: "image/png"}})
	if err != nil {
		t.Fatalf("normalize item images: %v", err)
	}
	if len(images) != 1 || images[0].RowID != 1 || string(images[0].ImageData) != "image-bytes" || images[0].ImageDescription != "front" {
		t.Fatalf("normalized item images = %#v", images)
	}
	for _, values := range [][]itemImageRequest{
		{{ImageData: "not-base64"}},
		{{RowID: 1, ImageData: encoded}, {RowID: 1, ImageData: encoded}},
	} {
		if _, err := normalizeItemImages(values); err == nil {
			t.Errorf("normalize item images accepted invalid values %#v", values)
		}
	}
}

func TestDecodeItemNotesDataRoundTripsTextAndBoundsBlobPayloads(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`{\\rtf1\\ansi Item note}`))
	data, err := decodeItemNotesData("data:application/rtf;base64," + encoded)
	if err != nil {
		t.Fatalf("decode item notes: %v", err)
	}
	if string(data) != `{\\rtf1\\ansi Item note}` {
		t.Fatalf("decoded item notes = %q", data)
	}
	if empty, err := decodeItemNotesData(""); err != nil || len(empty) != 0 {
		t.Fatalf("empty item notes = %v, %v", empty, err)
	}
	for _, value := range []string{"not-base64", base64.StdEncoding.EncodeToString(make([]byte, maxItemNotesBytes+1))} {
		if _, err := decodeItemNotesData(value); err == nil {
			t.Errorf("decode item notes accepted invalid value of length %d", len(value))
		}
	}
}

func TestNormalizeItemAssociationIDsRejectsAmbiguityAndSelfIsHandledByRoute(t *testing.T) {
	ids, err := normalizeItemAssociationIDs([]string{"  ITEM-2  ", "ITEM-3"})
	if err != nil {
		t.Fatalf("normalize item associations: %v", err)
	}
	if strings.Join(ids, "|") != "ITEM-2|ITEM-3" {
		t.Fatalf("normalized item associations = %#v", ids)
	}
	for _, values := range [][]string{{""}, {"ITEM-2", " item-2 "}} {
		if _, err := normalizeItemAssociationIDs(values); err == nil {
			t.Errorf("normalize item associations accepted invalid values %#v", values)
		}
	}
	tooMany := make([]string, maxItemAssociations+1)
	for index := range tooMany {
		tooMany[index] = "item-" + strconv.Itoa(index)
	}
	if _, err := normalizeItemAssociationIDs(tooMany); err == nil {
		t.Fatal("normalize item associations accepted more than the bounded maximum")
	}
}

func TestNormalizeItemAuthorsPreservesOrderAndRejectsAmbiguousRows(t *testing.T) {
	authors, err := normalizeItemAuthors([]itemAuthorRequest{{AuthorCode: 12, Priority: 2}, {AuthorCode: 13, Priority: 1, RowID: 7}})
	if err != nil {
		t.Fatalf("normalize item authors: %v", err)
	}
	if len(authors) != 2 || authors[0].RowID != 1 || authors[1].RowID != 7 || authors[1].Priority != 1 {
		t.Fatalf("normalized item authors = %#v", authors)
	}
	for _, values := range [][]itemAuthorRequest{
		{{AuthorCode: 0}},
		{{AuthorCode: 12}, {AuthorCode: 12}},
		{{AuthorCode: 12, RowID: 4}, {AuthorCode: 13, RowID: 4}},
		{{AuthorCode: 12, Priority: 256}},
	} {
		if _, err := normalizeItemAuthors(values); err == nil {
			t.Errorf("normalize item authors accepted invalid values %#v", values)
		}
	}
	tooMany := make([]itemAuthorRequest, maxItemAuthors+1)
	for index := range tooMany {
		tooMany[index] = itemAuthorRequest{AuthorCode: index + 1}
	}
	if _, err := normalizeItemAuthors(tooMany); err == nil {
		t.Fatal("normalize item authors accepted more than the bounded maximum")
	}
}

func TestNormalizeItemModelCodesPreservesMembershipAndBoundsSourceType(t *testing.T) {
	models, err := normalizeItemModelCodes([]int{12, -4, 32767})
	if err != nil {
		t.Fatalf("normalize item models: %v", err)
	}
	if strings.Join([]string{strconv.Itoa(models[0]), strconv.Itoa(models[1]), strconv.Itoa(models[2])}, "|") != "12|-4|32767" {
		t.Fatalf("normalized item models = %#v", models)
	}
	for _, values := range [][]int{{12, 12}, {-32769}, {32768}} {
		if _, err := normalizeItemModelCodes(values); err == nil {
			t.Errorf("normalize item models accepted invalid values %#v", values)
		}
	}
	tooMany := make([]int, maxItemModels+1)
	for index := range tooMany {
		tooMany[index] = index
	}
	if _, err := normalizeItemModelCodes(tooMany); err == nil {
		t.Fatal("normalize item models accepted more than the bounded maximum")
	}
}

func TestNormalizeItemPricePolicyTiersPreservesSourceDecimalsAndRejectsAmbiguity(t *testing.T) {
	tiers, err := normalizeItemPricePolicyTiers([]itemPricePolicyTierRequest{
		{ID: "11111111-1111-4111-8111-111111111111", QuantityLimit: 1, Price: "12.5000", ExpiryDate: "2026-12-31", FlatDiscount: "0.00", DiscountPercent: "2.00"},
		{QuantityLimit: 10, Price: "11.7500", ExpiryDate: "", FlatDiscount: "1.2500", DiscountPercent: "0"},
	})
	if err != nil {
		t.Fatalf("normalize item price policy: %v", err)
	}
	if len(tiers) != 2 || tiers[0].Price != "12.5000" || tiers[1].FlatDiscount != "1.2500" {
		t.Fatalf("normalized item price policy tiers = %#v", tiers)
	}
	for _, values := range [][]itemPricePolicyTierRequest{
		{{QuantityLimit: 1, Price: "12.12345"}},
		{{QuantityLimit: 1, Price: "12", ExpiryDate: "2026-12-31"}, {QuantityLimit: 1, Price: "13", ExpiryDate: "2026-12-31"}},
		{{ID: "not-a-uuid", QuantityLimit: 1, Price: "12"}},
	} {
		if _, err := normalizeItemPricePolicyTiers(values); err == nil {
			t.Errorf("normalize item price policy accepted invalid values %#v", values)
		}
	}
}

func TestNormalizeItemRegistrationPayloadPreservesItemFieldsAndAddsRequestMetadata(t *testing.T) {
	requestedAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	payload, err := normalizeItemRegistrationPayload(json.RawMessage(`{"SalePrice":"12.50","CustomICode":"ALT-1"}`), 42, "ITEM-1", "Example item", requestedAt)
	if err != nil {
		t.Fatalf("normalize item registration payload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode normalized registration payload: %v", err)
	}
	if decoded["SalePrice"] != "12.50" || decoded["CustomICode"] != "ALT-1" || decoded["ICode"] != "ITEM-1" || decoded["Name"] != "Example item" || decoded["ItemRegReqCode"] != float64(42) || decoded["Sent"] != "N" {
		t.Fatalf("normalized registration payload = %#v", decoded)
	}
}

func TestCanonicalMasterRoutesRemainAuthenticated(t *testing.T) {
	server := New(nil, "test", "")
	for _, path := range []string{
		"/v1/items/lookup?q=amoxi",
		"/v1/master/items/lookup?alias=amoxi",
		"/v1/master/item",
		"/v1/master/item/00000000-0000-0000-0000-000000000001",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/aliases",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/images",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/notes",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/associations",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/authors",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/models",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/price-policy",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/registration-request",
		"/v1/master/item/00000000-0000-0000-0000-000000000001/unposted-transactions",
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
	if !strings.Contains(migration, "'alternate_alias'") {
		t.Error("migration is missing the alternate item alias kind")
	}
}

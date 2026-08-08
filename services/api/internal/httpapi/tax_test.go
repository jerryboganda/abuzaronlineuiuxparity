package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestTaxRateValidationCoversKindsAndEffectiveDates(t *testing.T) {
	for _, kind := range []string{"gst", "pct", "advance"} {
		rate, err := normalizeTaxRateRequest(taxRateRequest{
			TaxKind: kind, Code: "rate-1", Name: "Rate 1", Rate: "18.50",
			EffectiveFrom: "2026-01-01", EffectiveTo: "2026-12-31",
		})
		if err != nil || rate.Rate != "18.50" {
			t.Fatalf("%s normalization = %+v, %v", kind, rate, err)
		}
	}
	for _, request := range []taxRateRequest{
		{TaxKind: "invalid", Code: "x", Name: "X", Rate: "1", EffectiveFrom: "2026-01-01"},
		{TaxKind: "gst", Code: "x", Name: "X", Rate: "101", EffectiveFrom: "2026-01-01"},
		{TaxKind: "gst", Code: "x", Name: "X", Rate: "1", EffectiveFrom: "2026-02-01", EffectiveTo: "2026-01-01"},
	} {
		if _, err := normalizeTaxRateRequest(request); err == nil {
			t.Fatalf("invalid tax rate was accepted: %+v", request)
		}
	}
}

func TestExplicitDocumentTaxesRequireOverrideAndRejectConflicts(t *testing.T) {
	draft := &documentDraftRequest{
		Kind:  "cash-sale",
		Lines: []documentLineRequest{{GSTRate: "18"}, {GSTRate: "18.00"}},
	}
	if !explicitDocumentTaxPolicy(draft) {
		t.Fatal("line tax metadata was not identified as explicit pricing")
	}
	operator := &sessionContext{Roles: []string{"sales officer"}}
	if _, err := resolveDocumentTaxPolicy(nil, nil, operator, draft); err == nil {
		t.Fatal("explicit tax pricing was accepted without tax.override")
	}
	operator.Roles = []string{"tenant_admin"}
	resolved, err := resolveDocumentTaxPolicy(nil, nil, operator, draft)
	if err != nil || resolved.GST == nil || resolved.GST.Rate != "18.00" {
		t.Fatalf("explicit tax normalization = %+v, %v", resolved, err)
	}
	draft.Lines[1].GSTRate = "17"
	if _, err := resolveDocumentTaxPolicy(nil, nil, operator, draft); err == nil {
		t.Fatal("conflicting explicit line tax rates were accepted")
	}
}

func TestTaxRoutesRemainAuthenticated(t *testing.T) {
	handler := New(nil, "test", "")
	for _, testCase := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/tax-rates"},
		{http.MethodGet, "/v1/tax-assignments"},
		{http.MethodPost, "/v1/tax-assignments/apply-item-gst"},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s returned %d, want %d", testCase.method, testCase.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestTaxMigrationDefinesEffectiveScopedConfiguration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "018_tax_configuration.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tax migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS tax_rates",
		"CREATE TABLE IF NOT EXISTS item_tax_assignments",
		"CREATE TABLE IF NOT EXISTS party_tax_assignments",
		"source_legacy_id",
		"effective_from date NOT NULL",
		"tax_kind IN ('gst', 'pct', 'advance')",
		"ALTER TABLE %I FORCE ROW LEVEL SECURITY",
		"AS RESTRICTIVE FOR ALL",
		"branch_id = NULLIF(current_setting(''app.branch_id''",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("tax migration is missing contract %q", required)
		}
	}
}

func TestTaxConfigurationResolvesProfilesEffectiveDatesAndPostedGL(t *testing.T) {
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
	fixture := seedStockTenant(t, ctx, database, "tax-"+fmt.Sprint(time.Now().UnixNano()))
	other := seedStockTenant(t, ctx, database, "other-tax-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() {
		cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID, other.tenantID)
	}()
	var customerID, supplierID, gstID, pctID, advanceID, futureGSTID string
	if err := database.QueryRowContext(ctx, `INSERT INTO master_parties
			(tenant_id, party_type, legacy_id, code, name) VALUES ($1::uuid, 'customer', $2, $2, 'Tax Customer')
			RETURNING id::text`, fixture.tenantID, "customer-"+fixture.itemLegacyID).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO master_parties
			(tenant_id, party_type, legacy_id, code, name) VALUES ($1::uuid, 'supplier', $2, $2, 'Tax Supplier')
			RETURNING id::text`, fixture.tenantID, "supplier-"+fixture.itemLegacyID).Scan(&supplierID); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}
	insertRate := func(kind, code, rate string, from, to string) string {
		t.Helper()
		var id string
		if err := database.QueryRowContext(ctx, `INSERT INTO tax_rates
				(tenant_id, branch_id, tax_kind, code, name, rate, inclusive, effective_from, effective_to, source_table, source_legacy_id)
				VALUES ($1::uuid, $2::uuid, $3, $4, $4, $5::numeric, false, $6::date, NULLIF($7, '')::date, 'test', $4)
				RETURNING id::text`, fixture.tenantID, fixture.branchID, kind, code, rate, from, to).Scan(&id); err != nil {
			t.Fatalf("seed %s tax rate: %v", kind, err)
		}
		return id
	}
	gstID = insertRate("gst", "gst-18", "18", "2026-01-01", "2026-12-31")
	pctID = insertRate("pct", "pct-5", "5", "2026-01-01", "")
	advanceID = insertRate("advance", "advance-2", "2", "2026-01-01", "")
	futureGSTID = insertRate("gst", "gst-20", "20", "2027-01-01", "")
	if _, err := database.ExecContext(ctx, `INSERT INTO item_tax_assignments
			(tenant_id, branch_id, item_id, tax_rate_id, effective_from, effective_to, source_table, source_legacy_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '2026-01-01', '2026-12-31', 'Item', $5),
			       ($1::uuid, $2::uuid, $3::uuid, $6::uuid, '2027-01-01', NULL, 'Item', $5)`,
		fixture.tenantID, fixture.branchID, fixture.itemID, gstID, fixture.itemLegacyID, futureGSTID); err != nil {
		t.Fatalf("seed item GST assignments: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO item_tax_assignments
			(tenant_id, branch_id, item_id, tax_rate_id, effective_from, source_table, source_legacy_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '2026-01-01', 'Item', $5)`,
		fixture.tenantID, fixture.branchID, fixture.itemID, pctID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed item PCT assignment: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO party_tax_assignments
			(tenant_id, branch_id, party_id, tax_rate_id, effective_from, source_table, source_legacy_id)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '2026-01-01', 'Customer', $5),
			       ($1::uuid, $2::uuid, $6::uuid, $7::uuid, '2026-01-01', 'Supplier', $5)`,
		fixture.tenantID, fixture.branchID, customerID, advanceID, customerID, supplierID, gstID); err != nil {
		t.Fatalf("seed party tax assignments: %v", err)
	}
	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "tax-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "TAX-001", ExpiryDate: "2030-01-01", Quantity: []byte(`20`), UnitCost: "4.00",
	})
	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin receiving: %v", err)
	}
	if err := projectEvent(ctx, tx, receive); err != nil {
		_ = tx.Rollback()
		t.Fatalf("project receiving: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit receiving: %v", err)
	}
	command := stockDocumentCommand(fixture, "save", "tax-draft", "1")
	command.CommandID = "00000000-0000-0000-0000-000000000030"
	command.Document.CustomerID = customerID
	command.Document.OccurredAt = "2026-06-01T00:00:00Z"
	command.OccurredAt = command.Document.OccurredAt
	server := &Server{database: database}
	draftStatus, draftResponse, draftBody := executeDocumentHandler(t, server, operator, command)
	if draftStatus != http.StatusCreated || draftResponse.Document.Status != "draft" || draftResponse.Document.Finance != nil {
		t.Fatalf("draft tax response = status %d response %+v body=%s", draftStatus, draftResponse, draftBody)
	}
	var draftTax string
	if err := database.QueryRowContext(ctx, `SELECT tax_amount::text FROM business_documents WHERE id = $1::uuid`, draftResponse.Document.ID).Scan(&draftTax); err != nil {
		t.Fatalf("read draft tax: %v", err)
	}
	if draftTax != "2.9000" {
		t.Fatalf("draft tax = %s, pricing=%s", draftTax, string(draftResponse.Document.Pricing))
	}
	var linePricing []byte
	var lineGST string
	if err := database.QueryRowContext(ctx, `SELECT pricing, gst_rate::text
		FROM business_document_lines WHERE document_id = $1::uuid`, draftResponse.Document.ID).Scan(&linePricing, &lineGST); err != nil {
		t.Fatalf("read line tax snapshot: %v", err)
	}
	if lineGST != "18.0000" || !strings.Contains(string(linePricing), `"kind": "gst"`) {
		t.Fatalf("line tax snapshot = gst %s pricing %s", lineGST, string(linePricing))
	}
	post := command
	post.Action = "post"
	post.CommandID = "00000000-0000-0000-0000-000000000031"
	post.IdempotencyKey = "tax-post"
	post.Document.ID = draftResponse.Document.ID
	post.ExpectedVersion = &draftResponse.Document.Version
	postedStatus, postedResponse, _ := executeDocumentHandler(t, server, operator, post)
	if postedStatus != http.StatusOK || postedResponse.Document.Finance == nil || !postedResponse.Document.Finance.Balanced {
		t.Fatalf("posted tax response = status %d response %+v", postedStatus, postedResponse)
	}
	var outputTax string
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(SUM(l.credit_amount), 0)::text
			FROM gl_lines l JOIN gl_journals j ON j.tenant_id = l.tenant_id AND j.branch_id = l.branch_id AND j.id = l.journal_id
			JOIN finance_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
			WHERE j.source_document_id = $1::uuid AND a.system_key = 'output_tax'`, postedResponse.Document.ID).Scan(&outputTax); err != nil {
		t.Fatalf("read output tax: %v", err)
	}
	if outputTax != "2.9000" {
		t.Fatalf("output tax = %s, want 2.9000", outputTax)
	}
	_, replayed, _ := executeDocumentHandler(t, server, operator, post)
	if !replayed.Duplicate {
		t.Fatal("tax post replay was not idempotent")
	}
	supplierTx, err := server.beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin supplier tax read: %v", err)
	}
	supplierAssignments, err := queryTaxAssignments(ctx, supplierTx, operator, "party", supplierID, "")
	if err != nil {
		_ = supplierTx.Rollback()
		t.Fatalf("supplier tax read: %v", err)
	}
	_ = supplierTx.Rollback()
	if len(supplierAssignments) != 1 || supplierAssignments[0].TaxRate.TaxKind != "gst" {
		t.Fatalf("supplier tax assignments = %+v", supplierAssignments)
	}
	otherOperator := &sessionContext{UserID: other.operatorID, TenantID: other.tenantID, BranchID: other.branchID, CounterID: other.counterID, Roles: []string{"tenant_admin"}}
	otherTx, err := server.beginScopedTx(ctx, otherOperator)
	if err != nil {
		t.Fatalf("begin other tenant tax read: %v", err)
	}
	assignments, err := queryTaxAssignments(ctx, otherTx, otherOperator, "item", fixture.itemID, "")
	if err != nil {
		_ = otherTx.Rollback()
		t.Fatalf("other tenant tax read: %v", err)
	}
	_ = otherTx.Rollback()
	if len(assignments) != 0 {
		t.Fatalf("other tenant saw %d foreign tax assignments", len(assignments))
	}
}

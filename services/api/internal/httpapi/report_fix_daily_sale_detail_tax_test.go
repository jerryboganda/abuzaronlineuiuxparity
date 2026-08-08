package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestDailySaleDetailReportSalesTaxValuePrefersLineTaxAmount is the
// regression test for the "SalesTax Value" masking bug documented in
// docs/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md ("Leaf 1 --
// sale-detail / Daily Sale Detail").
//
// dailySalesDetailReadModelQuery's sales_tax_value column used to be
//
//	COALESCE(NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), bl.tax_amount::text, '0.00')
//
// which put the legacy display string first. That string was found to be
// the literal "0.00" for 620,615/620,615 (100%) of real sale lines in the
// legacy-reference-sandbox tenant, so NULLIF never nulled it out and the
// COALESCE always stopped there -- permanently masking the canonical
// bl.tax_amount fallback (28,184/620,615, 4.54%, have real non-zero tax).
//
// bl.tax_amount is a NOT NULL numeric(19,4) column that is reliably
// populated for every line that has a business_document_lines row: computed
// at posting time from the pricing engine's tax snapshot for app-created
// documents (documents.go, lineTaxSnapshot), and imported as
// UnitSalesTax*quantity from the legacy Saledetail rows for historical
// lines (migration/cmd/bulksalelines/main.go) -- independent of the broken
// legacy_payload->>'SalesTax' display string. The fix reorders the COALESCE
// to trust bl.tax_amount first (including when it is legitimately 0.00 for
// a tax-exempt line), only falling back to the legacy payload/pricing JSON
// when no business_document_lines row is joined at all.
//
// This test seeds one line with real, non-zero tax whose legacy_payload
// carries the same misleading literal "0.00" seen tenant-wide, and one line
// with legitimately zero tax, then asserts the "sale-detail" report (i.e.
// dailySalesDetailReadModelQuery, reached via salesReadModelQueryMode
// mode="line-detail") reports each one correctly. Before the fix, the first
// assertion below would have failed (report would show "0.00" instead of
// "15.75").
func TestDailySaleDetailReportSalesTaxValuePrefersLineTaxAmount(t *testing.T) {
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

	suffix := "tax-fix-" + time.Now().Format("150405.000000")
	fixture := seedStockTenant(t, ctx, database, suffix)
	defer cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)

	var customerID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name)
		VALUES ($1::uuid, 'customer', $2, $2, 'Tax Fix Customer')
		RETURNING id::text
	`, fixture.tenantID, "customer-"+suffix).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	// TAXED-1: a line with real, non-zero canonical tax (bl.tax_amount =
	// 15.75) whose legacy_payload carries the same misleading literal
	// "0.00" observed for 100% of real sale lines in the sandbox tenant.
	// Before the fix this reported "0.00"; after the fix it must report
	// the real tax amount.
	var taxedDocumentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, customer_id, total_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'credit-sale', 'TAXED-1', 'posted',
		        '2026-08-06T10:00:00Z', $5::uuid, 115.75)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, customerID).Scan(&taxedDocumentID); err != nil {
		t.Fatalf("seed taxed document: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_document_lines
			(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id,
			 item_code, item_name, quantity, unit_price, line_gross, item_discount, line_total,
			 batch_number, expiry_date, tax_amount, legacy_payload)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5, $5, 'Taxed Item',
		        1, 100.00, 100.00, 0, 100.00, 'B-TAX', '2027-12-31', 15.75,
		        '{"SalePrice":"100.00","SalesTax":"0.00"}'::jsonb)
	`, fixture.tenantID, fixture.branchID, taxedDocumentID, fixture.itemID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed taxed line: %v", err)
	}

	// EXEMPT-1: a line with legitimately zero tax (bl.tax_amount = 0.00),
	// indistinguishable from TAXED-1 at the legacy_payload level (also
	// literal "0.00"). Must still correctly report "0.00" after the fix --
	// the fix must not force a non-zero value onto genuinely tax-exempt
	// lines.
	var exemptDocumentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, customer_id, total_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'credit-sale', 'EXEMPT-1', 'posted',
		        '2026-08-06T10:05:00Z', $5::uuid, 50.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, customerID).Scan(&exemptDocumentID); err != nil {
		t.Fatalf("seed exempt document: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_document_lines
			(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id,
			 item_code, item_name, quantity, unit_price, line_gross, item_discount, line_total,
			 batch_number, expiry_date, tax_amount, legacy_payload)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5, $5, 'Exempt Item',
		        1, 50.00, 50.00, 0, 50.00, 'B-EXEMPT', '2027-12-31', 0.00,
		        '{"SalePrice":"50.00","SalesTax":"0.00"}'::jsonb)
	`, fixture.tenantID, fixture.branchID, exemptDocumentID, fixture.itemID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed exempt line: %v", err)
	}

	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	server := &Server{database: database}

	request := readModelRequest(http.MethodGet, "/v1/reports/sale-detail?from=2026-08-06&to=2026-08-06&pageSize=50", operator)
	request.SetPathValue("kind", "sale-detail")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("sale-detail report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode sale-detail report: %v", err)
	}

	var taxedRow, exemptRow *reportRow
	for i := range body.Rows {
		switch body.Rows[i].Item {
		case "Taxed Item":
			taxedRow = &body.Rows[i]
		case "Exempt Item":
			exemptRow = &body.Rows[i]
		}
	}
	if taxedRow == nil {
		t.Fatalf("sale-detail report did not return the taxed line: %+v", body.Rows)
	}
	if exemptRow == nil {
		t.Fatalf("sale-detail report did not return the exempt line: %+v", body.Rows)
	}

	// The core regression assertion: the line with real, non-zero
	// bl.tax_amount now reports it, instead of the masked "0.00" a
	// legacy_payload->>'SalesTax'-first COALESCE would have produced.
	if taxedRow.SalesTaxValue != "15.75" {
		t.Fatalf("taxed line salesTaxValue = %q, want %q (real bl.tax_amount, not the legacy_payload literal 0.00)", taxedRow.SalesTaxValue, "15.75")
	}
	// The caution from the fix: a genuinely tax-exempt line must still
	// report 0.00, not be forced to some non-zero value.
	if exemptRow.SalesTaxValue != "0.00" {
		t.Fatalf("exempt line salesTaxValue = %q, want %q (legitimately zero tax)", exemptRow.SalesTaxValue, "0.00")
	}
}

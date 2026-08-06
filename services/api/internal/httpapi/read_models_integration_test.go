package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestReadModelsExposeCanonicalSalesWithoutDuplicateCompatibilityRows(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "read-model-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	insertCanonicalSalesReadFixture(t, ctx, database, fixture)
	insertCompatibilitySalesReadFixture(t, ctx, database, fixture)
	insertNonPostedSalesReadFixtures(t, ctx, database, fixture)
	server := &Server{database: database}

	for _, path := range []string{
		"/v1/reports/daily-sales-detail?from=2026-08-06&to=2026-08-06",
		"/v1/transactions/sale?from=2026-08-06&to=2026-08-06",
	} {
		request := readModelRequest(http.MethodGet, path, operator)
		if strings.HasPrefix(path, "/v1/reports/") {
			request.SetPathValue("kind", "daily-sales-detail")
		} else {
			request.SetPathValue("kind", "sale")
		}
		recorder := httptest.NewRecorder()
		if strings.HasPrefix(path, "/v1/reports/") {
			server.report(recorder, request)
		} else {
			server.transactionHistory(recorder, request)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Rows []reportRow `json:"rows"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("%s decode: %v", path, err)
		}
		if len(body.Rows) != 2 {
			t.Fatalf("%s rows = %d, want canonical plus one compatibility row: %+v", path, len(body.Rows), body.Rows)
		}
		seenCanonical := false
		seenCompatibility := false
		for _, row := range body.Rows {
			switch row.Document {
			case "CANONICAL-1":
				seenCanonical = true
			case "COMPAT-1":
				seenCompatibility = true
			}
		}
		if !seenCanonical || !seenCompatibility {
			t.Fatalf("%s rows did not include both sources: %+v", path, body.Rows)
		}
	}

	other := seedStockTenant(t, ctx, database, "read-model-other-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, other.tenantID)
	}()
	request := readModelRequest(http.MethodGet, "/v1/reports/daily-sales-detail?from=2026-08-06&to=2026-08-06&filter=CANONICAL-1", &sessionContext{
		UserID: other.operatorID, TenantID: other.tenantID, BranchID: other.branchID,
		CounterID: other.counterID, Roles: []string{"tenant_admin"},
	})
	request.SetPathValue("kind", "daily-sales-detail")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("cross-tenant report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var isolated struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&isolated); err != nil {
		t.Fatalf("decode isolated report: %v", err)
	}
	if len(isolated.Rows) != 0 {
		t.Fatalf("cross-tenant report returned rows: %+v", isolated.Rows)
	}
}

func TestReadModelsExposeCanonicalSaleReturnsWithoutDuplicateCompatibilityRows(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "read-model-return-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	var documentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, total_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'open-cash-return', 'RETURN-CANONICAL-1', 'posted',
		        '2026-08-06T10:00:00Z', 4.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID).Scan(&documentID); err != nil {
		t.Fatalf("seed canonical sale return: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_document_lines
			(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id,
			 item_code, item_name, quantity, unit_price, line_gross, line_total)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5, $5, 'Returned Item',
		        2, 2.00, 4.00, 4.00)
	`, fixture.tenantID, fixture.branchID, documentID, fixture.itemID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed canonical sale-return line: %v", err)
	}
	var compatibilityID string
	if err := database.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&compatibilityID); err != nil {
		t.Fatalf("generate compatibility sale-return id: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sync_events
			(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id,
			 idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'sale_return',
		        $5::uuid, 'compatibility-sale-return-read-model-event', 1,
		        '{"documentNumber":"RETURN-COMPAT-1","customerName":"Compatibility Customer","totalAmount":"3.00","rows":[{"itemName":"Compatibility Returned Item","quantity":"1"}]}',
		        '2026-08-06T09:00:00Z')
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, compatibilityID); err != nil {
		t.Fatalf("seed compatibility sale-return event: %v", err)
	}

	server := &Server{database: database}
	request := readModelRequest(http.MethodGet, "/v1/reports/sales-return-detail?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", "sales-return-detail")
	recorder := httptest.NewRecorder()
	server.report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("sale-return report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows       []reportRow      `json:"rows"`
		Definition reportDefinition `json:"definition"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode sale-return report: %v", err)
	}
	if len(body.Rows) != 2 {
		t.Fatalf("sale-return report rows = %d, want canonical plus compatibility row: %+v", len(body.Rows), body.Rows)
	}
	seenCanonical, seenCompatibility := false, false
	for _, row := range body.Rows {
		switch row.Document {
		case "RETURN-CANONICAL-1":
			seenCanonical = true
		case "RETURN-COMPAT-1":
			seenCompatibility = true
		}
	}
	if !seenCanonical || !seenCompatibility {
		t.Fatalf("sale-return report rows did not include both sources: %+v", body.Rows)
	}
	if !strings.Contains(body.Definition.ProjectionNote, "business_documents") {
		t.Fatalf("sale-return definition did not disclose canonical projection: %q", body.Definition.ProjectionNote)
	}
}

func TestInvoiceSummaryReportGroupsCanonicalLinesOnce(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "read-model-summary-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()
	var documentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, total_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'cash-sale', 'SUMMARY-CANONICAL-1', 'posted',
		        '2026-08-06T10:00:00Z', 30.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID).Scan(&documentID); err != nil {
		t.Fatalf("seed summary document: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_document_lines
			(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id,
			 item_code, item_name, quantity, unit_price, line_gross, line_total)
		VALUES
			($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5, $5, 'Summary Item A', 1, 10.00, 10.00, 10.00),
			($1::uuid, $2::uuid, $3::uuid, 2, $4::uuid, $5, $5, 'Summary Item B', 2, 10.00, 20.00, 20.00)
	`, fixture.tenantID, fixture.branchID, documentID, fixture.itemID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed summary lines: %v", err)
	}
	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/sale-summary?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", "sale-summary")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("sale-summary report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode sale-summary report: %v", err)
	}
	if len(body.Rows) != 1 {
		t.Fatalf("sale-summary rows = %d, want one invoice row: %+v", len(body.Rows), body.Rows)
	}
	if body.Rows[0].Document != "SUMMARY-CANONICAL-1" || body.Rows[0].Quantity != "3.0000" || body.Rows[0].Amount != "30.0000" {
		t.Fatalf("sale-summary row = %+v, want one grouped invoice with quantity 3 and amount 30", body.Rows[0])
	}
}

func TestPurchaseReturnReportUsesCanonicalReadModel(t *testing.T) {
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
	fixture := seedStockTenant(t, ctx, database, "read-model-purchase-return-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()
	supplierID := seedPurchaseSupplier(t, ctx, database, fixture.tenantID, "supplier-"+fixture.itemLegacyID)
	var documentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, supplier_id, total_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'purchase-return', 'PR-CANONICAL-1', 'posted',
		        '2026-08-06T10:00:00Z', $5::uuid, 7.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, supplierID).Scan(&documentID); err != nil {
		t.Fatalf("seed purchase-return document: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_document_lines
			(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id,
			 item_code, item_name, quantity, unit_price, line_gross, line_total)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5, $5, 'Returned Purchase Item',
		        1, 7.00, 7.00, 7.00)
	`, fixture.tenantID, fixture.branchID, documentID, fixture.itemID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed purchase-return line: %v", err)
	}
	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	request := readModelRequest(http.MethodGet, "/v1/reports/purchase-return?from=2026-08-06&to=2026-08-06", operator)
	request.SetPathValue("kind", "purchase-return")
	recorder := httptest.NewRecorder()
	(&Server{database: database}).report(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("purchase-return report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode purchase-return report: %v", err)
	}
	if len(body.Rows) != 1 || body.Rows[0].Document != "PR-CANONICAL-1" || body.Rows[0].Item != "Returned Purchase Item" {
		t.Fatalf("purchase-return report rows = %+v, want canonical row", body.Rows)
	}
}

func TestInventoryBalancePrefersNormalizedGodownScopedRows(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "balance-read-"+time.Now().Format("150405.000000"))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID)
	}()
	var batchID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO stock_batches
			(tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, 'ACTIVE', '2030-01-01', 4.00)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID).Scan(&batchID); err != nil {
		t.Fatalf("seed active batch: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO stock_balances
			(tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, 7)
	`, fixture.tenantID, fixture.branchID, batchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID); err != nil {
		t.Fatalf("seed normalized balance: %v", err)
	}
	legacyEvent := insertInventoryEvent(t, ctx, database, fixture, "receiving", "balance-compat", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "LEGACY", Quantity: json.RawMessage(`99`), UnitCost: "4.00",
	})
	if _, err := database.ExecContext(ctx, `
		INSERT INTO inventory_movements
			(tenant_id, branch_id, source_event_id, item_legacy_id, quantity, direction, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 99, 'in', '2026-08-06T00:00:00Z')
	`, fixture.tenantID, fixture.branchID, legacyEvent.EventID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed compatibility movement: %v", err)
	}

	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	request := readModelRequest(http.MethodGet, "/v1/inventory/balance?itemLegacyId="+fixture.itemLegacyID+"&godownId="+fixture.godownID, operator)
	recorder := httptest.NewRecorder()
	(&Server{database: database}).inventoryBalance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("balance status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	if body["available"] != "7.0000" && body["available"] != "7" {
		t.Fatalf("normalized balance = %q, want 7: %+v", body["available"], body)
	}
	if body["source"] != "stock_balances" {
		t.Fatalf("balance source = %q, want stock_balances", body["source"])
	}

	fallbackItem := fixture.itemLegacyID + "-fallback"
	fallbackEvent := insertInventoryEvent(t, ctx, database, fixture, "receiving", "balance-compat-fallback", inventoryRowPayload{
		ItemLegacyID: fallbackItem, GodownID: fixture.godownID,
		BatchNumber: "LEGACY-FALLBACK", Quantity: json.RawMessage(`3`), UnitCost: "4.00",
	})
	if _, err := database.ExecContext(ctx, `
		INSERT INTO inventory_movements
			(tenant_id, branch_id, source_event_id, item_legacy_id, quantity, direction, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 3, 'in', '2026-08-06T00:00:00Z')
	`, fixture.tenantID, fixture.branchID, fallbackEvent.EventID, fallbackItem); err != nil {
		t.Fatalf("seed fallback movement: %v", err)
	}
	fallbackRequest := readModelRequest(http.MethodGet, "/v1/inventory/balance?itemLegacyId="+fallbackItem+"&godownId="+fixture.godownID, operator)
	fallbackRecorder := httptest.NewRecorder()
	(&Server{database: database}).inventoryBalance(fallbackRecorder, fallbackRequest)
	if fallbackRecorder.Code != http.StatusOK {
		t.Fatalf("fallback balance status = %d, body = %s", fallbackRecorder.Code, fallbackRecorder.Body.String())
	}
	var fallbackBody map[string]string
	if err := json.NewDecoder(fallbackRecorder.Body).Decode(&fallbackBody); err != nil {
		t.Fatalf("decode fallback balance: %v", err)
	}
	if fallbackBody["available"] != "3.0000" && fallbackBody["available"] != "3" {
		t.Fatalf("fallback balance = %q, want 3: %+v", fallbackBody["available"], fallbackBody)
	}
	if fallbackBody["source"] != "compatibility_inventory_movements" {
		t.Fatalf("fallback source = %q, want labeled compatibility source", fallbackBody["source"])
	}
}

func insertCanonicalSalesReadFixture(t *testing.T, ctx context.Context, database *sql.DB, fixture stockFixture) string {
	t.Helper()
	var customerID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name)
		VALUES ($1::uuid, 'customer', 'customer-read-model', 'customer-read-model', 'Canonical Customer')
		RETURNING id::text
	`, fixture.tenantID).Scan(&customerID); err != nil {
		t.Fatalf("seed canonical customer: %v", err)
	}
	var documentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, customer_id, total_amount)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'credit-sale', 'CANONICAL-1', 'posted',
		        '2026-08-06T10:00:00Z', $5::uuid, 12.50)
		RETURNING id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, customerID).Scan(&documentID); err != nil {
		t.Fatalf("seed canonical document: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_document_lines
			(tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id,
			 item_code, item_name, quantity, unit_price, line_gross, line_total)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5, $6, 'Canonical Item',
		        2, 6.25, 12.50, 12.50)
	`, fixture.tenantID, fixture.branchID, documentID, fixture.itemID, fixture.itemLegacyID, fixture.itemLegacyID); err != nil {
		t.Fatalf("seed canonical line: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sync_events
			(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id,
			 idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'business_document',
		        $5::uuid, 'canonical-read-model-event', 1, '{}'::jsonb, '2026-08-06T10:00:00Z')
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, documentID); err != nil {
		t.Fatalf("seed canonical event: %v", err)
	}
	return documentID
}

func insertCompatibilitySalesReadFixture(t *testing.T, ctx context.Context, database *sql.DB, fixture stockFixture) {
	t.Helper()
	var aggregateID string
	if err := database.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&aggregateID); err != nil {
		t.Fatalf("generate compatibility aggregate: %v", err)
	}
	payload := `{"documentNumber":"COMPAT-1","customerName":"Compatibility Customer","totalAmount":"9.00","rows":[{"itemName":"Compatibility Item","quantity":"1"}]}`
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sync_events
			(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id,
			 idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'sale',
		        $5::uuid, 'compatibility-read-model-event', 1, $6::jsonb, '2026-08-06T09:00:00Z')
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, aggregateID, payload); err != nil {
		t.Fatalf("seed compatibility event: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sales_documents
			(id, tenant_id, branch_id, counter_id, operator_id, document_number, status,
			 total_amount, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, 'COMPAT-1', 'posted',
		        9.00, '2026-08-06T09:00:00Z')
	`, aggregateID, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID); err != nil {
		t.Fatalf("seed compatibility document: %v", err)
	}
}

func insertNonPostedSalesReadFixtures(t *testing.T, ctx context.Context, database *sql.DB, fixture stockFixture) {
	t.Helper()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO business_documents
			(tenant_id, branch_id, counter_id, operator_id, kind, document_number, status,
			 occurred_at, total_amount)
		VALUES
			($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'cash-sale', 'DRAFT-CANONICAL', 'draft',
			 '2026-08-06T08:00:00Z', 5.00),
			($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'cash-sale', 'VOID-CANONICAL', 'void',
			 '2026-08-06T08:30:00Z', 6.00)
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID); err != nil {
		t.Fatalf("seed non-posted canonical documents: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sales_documents
			(id, tenant_id, branch_id, counter_id, operator_id, document_number, status,
			 total_amount, occurred_at)
		VALUES
			(gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'DRAFT-COMPAT', 'draft',
			 7.00, '2026-08-06T07:00:00Z'),
			(gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'VOID-COMPAT', 'voided',
			 8.00, '2026-08-06T07:30:00Z')
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID); err != nil {
		t.Fatalf("seed non-posted compatibility documents: %v", err)
	}
}

func readModelRequest(method, target string, operator *sessionContext) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	return request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
}

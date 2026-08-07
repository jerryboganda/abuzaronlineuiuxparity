package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPurchaseVerticalSliceIntegration(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "purchase-"+fmt.Sprint(time.Now().UnixNano()))
	other := seedStockTenant(t, ctx, database, "other-purchase-"+fmt.Sprint(time.Now().UnixNano()))
	supplierID := seedPurchaseSupplier(t, ctx, database, fixture.tenantID, "supplier-"+fixture.itemLegacyID)
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id IN ($1::uuid, $2::uuid)`, fixture.tenantID, other.tenantID)
	}()

	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	otherOperator := &sessionContext{
		UserID: other.operatorID, TenantID: other.tenantID, BranchID: other.branchID,
		CounterID: other.counterID, Roles: []string{"tenant_admin"},
	}
	server := &Server{database: database}

	po := purchaseDocumentCommand("purchase-order", "save", "purchase-po", fixture, supplierID, "")
	po.Document.GodownID = ""
	beforeStock := countStockLedger(t, ctx, database, fixture.tenantID)
	beforeJournals := countFinanceJournals(t, ctx, database, fixture.tenantID)
	_, poResponse, _ := executeDocumentHandler(t, server, operator, po)
	if poResponse.Document.Status != "draft" {
		t.Fatalf("purchase order status = %s, want draft", poResponse.Document.Status)
	}
	assertPurchaseCounts(t, ctx, database, fixture.tenantID, beforeStock, beforeJournals)

	poPost := po
	poPost.Action = "post"
	poPost.CommandID = "00000000-0000-0000-0000-000000000031"
	poPost.IdempotencyKey = "purchase-po-post"
	poPost.Document = clonePurchaseDraft(po.Document)
	poPost.Document.ID = poResponse.Document.ID
	poPost.ExpectedVersion = pointerInt64(poResponse.Document.Version)
	_, poPosted, _ := executeDocumentHandler(t, server, operator, poPost)
	if poPosted.Document.Status != "posted" {
		t.Fatalf("purchase order post status = %s, want posted", poPosted.Document.Status)
	}
	assertPurchaseCounts(t, ctx, database, fixture.tenantID, beforeStock, beforeJournals)

	receipt := purchaseDocumentCommand("pack-purchase", "save-and-post", "purchase-receipt", fixture, supplierID, "")
	receiptStatus, receiptResponse, receiptBody := executeDocumentHandlerStatus(t, server, operator, receipt)
	if receiptStatus >= http.StatusBadRequest {
		t.Fatalf("receipt status = %d, body=%s", receiptStatus, receiptBody)
	}
	if receiptResponse.Document.Status != "posted" || receiptResponse.Document.Finance == nil || !receiptResponse.Document.Finance.Balanced {
		t.Fatalf("receipt response = %+v", receiptResponse.Document)
	}
	var balance, movementDirection, payableBalance string
	if err := database.QueryRowContext(ctx, `
		SELECT sb.on_hand::text
		FROM stock_balances sb
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  AND sb.item_legacy_id = $3
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&balance); err != nil {
		t.Fatalf("read purchase stock balance: %v", err)
	}
	if balance != "1.0000" {
		t.Fatalf("purchase stock balance = %s, want 1.0000", balance)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT direction FROM stock_ledger
		WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid
	`, fixture.tenantID, receiptResponse.Document.ID).Scan(&movementDirection); err != nil {
		t.Fatalf("read purchase stock movement: %v", err)
	}
	if movementDirection != "in" {
		t.Fatalf("purchase movement direction = %s, want in", movementDirection)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT balance::text FROM party_ledger_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
	`, fixture.tenantID, fixture.branchID, supplierID).Scan(&payableBalance); err != nil {
		t.Fatalf("read supplier payable balance: %v", err)
	}
	if payableBalance != "-11.0000" {
		t.Fatalf("supplier payable balance = %s, want -11.0000", payableBalance)
	}

	beforeReplayStock := countStockLedger(t, ctx, database, fixture.tenantID)
	beforeReplayJournals := countFinanceJournals(t, ctx, database, fixture.tenantID)
	_, replay, _ := executeDocumentHandler(t, server, operator, receipt)
	if !replay.Duplicate {
		t.Fatal("purchase replay was not marked duplicate")
	}
	assertPurchaseCounts(t, ctx, database, fixture.tenantID, beforeReplayStock, beforeReplayJournals)

	if _, err := database.ExecContext(ctx, `UPDATE finance_accounts SET active = false WHERE tenant_id = $1::uuid AND system_key = 'accounts_payable'`, fixture.tenantID); err != nil {
		t.Fatalf("disable payable account: %v", err)
	}
	failedReceipt := purchaseDocumentCommand("pack-purchase", "save-and-post", "purchase-receipt-failure", fixture, supplierID, "")
	failedReceipt.Document.Lines[0].BatchNumber = "PUR-FAIL"
	failedBeforeStock := countStockLedger(t, ctx, database, fixture.tenantID)
	failedBeforeDocuments := countBusinessDocuments(t, ctx, database, fixture.tenantID)
	failedStatus, _, failedBody := executeDocumentHandlerStatus(t, server, operator, failedReceipt)
	_, _ = database.ExecContext(ctx, `UPDATE finance_accounts SET active = true WHERE tenant_id = $1::uuid AND system_key = 'accounts_payable'`, fixture.tenantID)
	if failedStatus != http.StatusUnprocessableEntity {
		t.Fatalf("failed purchase status = %d, body=%s", failedStatus, failedBody)
	}
	if countStockLedger(t, ctx, database, fixture.tenantID) != failedBeforeStock ||
		countBusinessDocuments(t, ctx, database, fixture.tenantID) != failedBeforeDocuments {
		t.Fatal("failed purchase finance post mutated document or stock state")
	}

	invalidReturn := purchaseDocumentCommand("purchase-return", "save-and-post", "purchase-return-invalid", fixture, supplierID, "00000000-0000-0000-0000-000000000099")
	invalidReturn.Document.Lines[0].Allocations = []documentAllocationRequest{{BatchNumber: "PUR-001", Quantity: "1"}}
	invalidBeforeStock := countStockLedger(t, ctx, database, fixture.tenantID)
	invalidBeforeDocuments := countBusinessDocuments(t, ctx, database, fixture.tenantID)
	invalidStatus, _, _ := executeDocumentHandlerStatus(t, server, operator, invalidReturn)
	if invalidStatus != http.StatusConflict && invalidStatus != http.StatusUnprocessableEntity {
		t.Fatalf("invalid purchase return status = %d", invalidStatus)
	}
	if countStockLedger(t, ctx, database, fixture.tenantID) != invalidBeforeStock ||
		countBusinessDocuments(t, ctx, database, fixture.tenantID) != invalidBeforeDocuments {
		t.Fatal("invalid purchase return mutated stock or document state")
	}

	validReturn := purchaseDocumentCommand("purchase-return", "save-and-post", "purchase-return-valid", fixture, supplierID, receiptResponse.Document.ID)
	validReturn.Document.Lines[0].SourceLineID = receiptResponse.Document.Lines[0].ID
	validReturn.Document.Lines[0].Allocations = []documentAllocationRequest{{BatchNumber: "PUR-001", Quantity: "1"}}
	returnStatus, returnResponse, returnBody := executeDocumentHandlerStatus(t, server, operator, validReturn)
	if returnStatus >= http.StatusBadRequest {
		t.Fatalf("return status = %d, body=%s", returnStatus, returnBody)
	}
	if returnResponse.Document.Status != "posted" || returnResponse.Document.Finance == nil {
		t.Fatalf("return response = %+v", returnResponse.Document)
	}
	var returnedBalance string
	if err := database.QueryRowContext(ctx, `
		SELECT sb.on_hand::text
		FROM stock_balances sb
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  AND sb.item_legacy_id = $3
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&returnedBalance); err != nil {
		t.Fatalf("read returned stock balance: %v", err)
	}
	if returnedBalance != "0.0000" {
		t.Fatalf("returned stock balance = %s, want 0.0000", returnedBalance)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT balance::text FROM party_ledger_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
	`, fixture.tenantID, fixture.branchID, supplierID).Scan(&payableBalance); err != nil {
		t.Fatalf("read supplier balance after return: %v", err)
	}
	if payableBalance != "0.0000" {
		t.Fatalf("supplier balance after return = %s, want 0.0000", payableBalance)
	}

	blockedReceiptVoid := documentCommandRequest{
		CommandID: purchaseCommandID("purchase-receipt-void-blocked"), Kind: "pack-purchase", Action: "void",
		IdempotencyKey: "purchase-receipt-void-blocked", OccurredAt: "2026-08-06T01:00:00Z",
		DocumentID: receiptResponse.Document.ID, ExpectedVersion: pointerInt64(receiptResponse.Document.Version),
		Reason: "Receipt correction",
	}
	beforeBlockedStock := countStockLedger(t, ctx, database, fixture.tenantID)
	beforeBlockedJournals := countFinanceJournals(t, ctx, database, fixture.tenantID)
	blockedStatus, _, blockedBody := executeDocumentHandlerStatus(t, server, operator, blockedReceiptVoid)
	if blockedStatus != http.StatusConflict && blockedStatus != http.StatusUnprocessableEntity {
		t.Fatalf("posted purchase with dependent return void status=%d body=%s; want dependency rejection", blockedStatus, blockedBody)
	}
	if countStockLedger(t, ctx, database, fixture.tenantID) != beforeBlockedStock || countFinanceJournals(t, ctx, database, fixture.tenantID) != beforeBlockedJournals {
		t.Fatal("dependent purchase void rejection mutated stock or finance projections")
	}

	voidPurchaseReturn := documentCommandRequest{
		CommandID: purchaseCommandID("purchase-return-void"), Kind: "purchase-return", Action: "void",
		IdempotencyKey: "purchase-return-void", OccurredAt: "2026-08-06T02:00:00Z",
		DocumentID: returnResponse.Document.ID, ExpectedVersion: pointerInt64(returnResponse.Document.Version),
		Reason: "Purchase return correction",
	}
	voidReturnStatus, voidedPurchaseReturn, voidReturnBody := executeDocumentHandler(t, server, operator, voidPurchaseReturn)
	if voidReturnStatus != http.StatusOK || !voidedPurchaseReturn.Accepted || voidedPurchaseReturn.Document.Status != "void" || voidedPurchaseReturn.Document.Finance == nil || !voidedPurchaseReturn.Document.Finance.Balanced {
		t.Fatalf("purchase return void status=%d body=%s response=%+v; want balanced compensating reversal", voidReturnStatus, voidReturnBody, voidedPurchaseReturn)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT sb.on_hand::text
		FROM stock_balances sb
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  AND sb.item_legacy_id = $3
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&returnedBalance); err != nil {
		t.Fatalf("read stock after purchase return void: %v", err)
	}
	if returnedBalance != "1.0000" {
		t.Fatalf("stock after purchase return void = %s, want 1.0000", returnedBalance)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT balance::text FROM party_ledger_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
	`, fixture.tenantID, fixture.branchID, supplierID).Scan(&payableBalance); err != nil {
		t.Fatalf("read supplier balance after purchase return void: %v", err)
	}
	if payableBalance != "-11.0000" {
		t.Fatalf("supplier balance after purchase return void = %s, want -11.0000", payableBalance)
	}

	crossTenant := purchaseDocumentCommand("pack-purchase", "save", "purchase-cross-tenant", other, supplierID, "")
	crossTenant.Document.SupplierID = supplierID
	crossStatus, _, _ := executeDocumentHandlerStatus(t, server, otherOperator, crossTenant)
	if crossStatus != http.StatusUnprocessableEntity {
		t.Fatalf("cross-tenant purchase status = %d, want %d", crossStatus, http.StatusUnprocessableEntity)
	}
}

func TestCanonicalPurchaseHistoryHydratesDocumentIdentityAndDetail(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "purchase-history-"+fmt.Sprint(time.Now().UnixNano()))
	supplierID := seedPurchaseSupplier(t, ctx, database, fixture.tenantID, "supplier-history-"+fixture.itemLegacyID)
	defer func() { _, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID) }()
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	server := &Server{database: database}
	command := purchaseDocumentCommand("pack-purchase", "save", "purchase-history-detail", fixture, supplierID, "")
	command.Document.Lines[0].DiscountPercent = "5"
	command.Document.Lines[0].GSTRate = "18"
	status, saved, body := executeDocumentHandlerStatus(t, server, operator, command)
	if status >= http.StatusBadRequest {
		t.Fatalf("purchase save status = %d, body=%s", status, body)
	}

	historyRequest := httptest.NewRequest(http.MethodGet, "/v1/transactions/pack-purchase?from=2026-08-06&to=2026-08-06", nil)
	historyRequest.SetPathValue("kind", "pack-purchase")
	historyRequest = historyRequest.WithContext(context.WithValue(historyRequest.Context(), sessionContextKey, operator))
	historyRecorder := httptest.NewRecorder()
	server.transactionHistory(historyRecorder, historyRequest)
	if historyRecorder.Code != http.StatusOK {
		t.Fatalf("history status = %d, body=%s", historyRecorder.Code, historyRecorder.Body.String())
	}
	var historyResponse struct {
		Rows []reportRow `json:"rows"`
	}
	if err := json.Unmarshal(historyRecorder.Body.Bytes(), &historyResponse); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	var historyRow *reportRow
	for index := range historyResponse.Rows {
		if historyResponse.Rows[index].Document == saved.Document.DocumentNumber {
			historyRow = &historyResponse.Rows[index]
			break
		}
	}
	if historyRow == nil || historyRow.DocumentID != saved.Document.ID {
		t.Fatalf("history row identity = %+v, want document id %s", historyRow, saved.Document.ID)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/v1/documents/"+saved.Document.ID, nil)
	detailRequest.SetPathValue("id", saved.Document.ID)
	detailRequest = detailRequest.WithContext(context.WithValue(detailRequest.Context(), sessionContextKey, operator))
	detailRecorder := httptest.NewRecorder()
	server.documentDetail(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail documentResponse
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ID != saved.Document.ID || detail.SupplierID != supplierID || len(detail.Lines) != 1 {
		t.Fatalf("detail identity = %+v, want id/supplier/one-line", detail)
	}
	if detail.Lines[0].ItemID != fixture.itemID || detail.Lines[0].BatchNumber != "PUR-001" || detail.Lines[0].ExpiryDate != "2030-01-01" {
		t.Fatalf("detail line = %+v, want canonical item/batch/expiry", detail.Lines[0])
	}
	if detail.Lines[0].Price.DiscountPercent != "5.00" || len(detail.Lines[0].Tax.Lines) != 1 || detail.Lines[0].Tax.Lines[0].Rate != "18.0000" {
		t.Fatalf("detail pricing/tax = %+v, want persisted discount and GST metadata", detail.Lines[0])
	}
}

func TestCanonicalPurchaseLoadsItemSupplierScheme(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "purchase-scheme-"+fmt.Sprint(time.Now().UnixNano()))
	supplierID := seedPurchaseSupplier(t, ctx, database, fixture.tenantID, "supplier-scheme-"+fixture.itemLegacyID)
	defer func() { _, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID) }()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO item_suppliers (
			tenant_id, item_id, supplier_id, legacy_item_id, legacy_supplier_id,
			priority, discount_percent, quantity, bonus, payload
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, 1, 10.00, 2.00, 1.00, '{"DiscPerc":"10.00","SaleQty":2,"BonusQty":1}'::jsonb)
	`, fixture.tenantID, fixture.itemID, supplierID, fixture.itemLegacyID, "supplier-scheme-"+fixture.itemLegacyID); err != nil {
		t.Fatalf("seed item supplier scheme: %v", err)
	}
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
	command := purchaseDocumentCommand("pack-purchase", "save", "purchase-scheme", fixture, supplierID, "")
	command.Document.Lines[0].Quantity = "4"
	command.Document.Lines[0].UnitPrice = "10.00"
	command.Document.Lines[0].UnitCost = "10.00"
	status, response, body := executeDocumentHandlerStatus(t, &Server{database: database}, operator, command)
	if status >= http.StatusBadRequest {
		t.Fatalf("purchase status = %d, body=%s", status, body)
	}
	var supplierDiscount, lineTotal, pricingJSON string
	if err := database.QueryRowContext(ctx, `
		SELECT supplier_discount::text, line_total::text, pricing::text
		FROM business_document_lines
		WHERE tenant_id = $1::uuid AND document_id = $2::uuid AND line_number = 1
	`, fixture.tenantID, response.Document.ID).Scan(&supplierDiscount, &lineTotal, &pricingJSON); err != nil {
		t.Fatalf("read supplier scheme pricing: %v", err)
	}
	if supplierDiscount != "4.0000" || lineTotal != "36.0000" {
		t.Fatalf("supplier scheme amounts = discount %s total %s, want 4.0000 and 36.0000", supplierDiscount, lineTotal)
	}
	var linePricing map[string]any
	if err := json.Unmarshal([]byte(pricingJSON), &linePricing); err != nil {
		t.Fatalf("decode line pricing: %v", err)
	}
	if linePricing["supplierBonusQuantity"] != "2" {
		t.Fatalf("supplier bonus quantity = %#v, want 2", linePricing["supplierBonusQuantity"])
	}
}

func purchaseDocumentCommand(kind, action, key string, fixture stockFixture, supplierID, sourceID string) documentCommandRequest {
	return documentCommandRequest{
		CommandID: purchaseCommandID(key), Kind: kind, Action: action,
		IdempotencyKey: key, OccurredAt: "2026-08-06T00:00:00Z",
		Document: &documentDraftRequest{
			Kind: kind, OccurredAt: "2026-08-06T00:00:00Z", SupplierID: supplierID,
			SourceDocumentID: sourceID, GodownID: fixture.godownID,
			Lines: []documentLineRequest{{
				LineNumber: 1, ItemID: fixture.itemID, Quantity: "1", UnitPrice: "10.00",
				UnitCost: "4.00", BatchNumber: "PUR-001", ExpiryDate: "2030-01-01",
			}},
		},
	}
}

func purchaseCommandID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[0:4]) + "-" +
		hex.EncodeToString(sum[4:6]) + "-" +
		hex.EncodeToString(sum[6:8]) + "-" +
		hex.EncodeToString(sum[8:10]) + "-" +
		hex.EncodeToString(sum[10:16])
}

func executeDocumentHandlerStatus(t *testing.T, server *Server, operator *sessionContext, command documentCommandRequest) (int, documentCommandResponse, string) {
	t.Helper()
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("marshal purchase command: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/documents/"+command.Kind, bytes.NewReader(payload))
	request.SetPathValue("kind", command.Kind)
	request = request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
	recorder := httptest.NewRecorder()
	server.documentCommand(recorder, request)
	var response documentCommandResponse
	if recorder.Code < http.StatusBadRequest {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode purchase response: %v", err)
		}
	}
	return recorder.Code, response, recorder.Body.String()
}

func clonePurchaseDraft(source *documentDraftRequest) *documentDraftRequest {
	copy := *source
	copy.Lines = append([]documentLineRequest(nil), source.Lines...)
	return &copy
}

func seedPurchaseSupplier(t *testing.T, ctx context.Context, database *sql.DB, tenantID, legacyID string) string {
	t.Helper()
	var id string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name)
		VALUES ($1::uuid, 'supplier', $2, $2, 'Purchase Supplier')
		RETURNING id::text
	`, tenantID, legacyID).Scan(&id); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}
	return id
}

func assertPurchaseCounts(t *testing.T, ctx context.Context, database *sql.DB, tenantID string, stock, journals int) {
	t.Helper()
	if got := countStockLedger(t, ctx, database, tenantID); got != stock {
		t.Fatalf("stock ledger count = %d, want %d", got, stock)
	}
	if got := countFinanceJournals(t, ctx, database, tenantID); got != journals {
		t.Fatalf("finance journal count = %d, want %d", got, journals)
	}
}

package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSaleReturnLifecycleIntegration(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "sale-return-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() { _, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, fixture.tenantID) }()
	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "sale-return-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "RET-001", ExpiryDate: "2030-01-01", Quantity: []byte(`3`), UnitCost: "4.00",
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

	server := &Server{database: database}
	sale := stockDocumentCommand(fixture, "save-and-post", "sale-return-source-sale", "1")
	sale.CommandID = "00000000-0000-4000-8000-000000000201"
	status, saleResponse, body := executeDocumentHandler(t, server, operator, sale)
	if status != 200 || saleResponse.Document.Status != "posted" {
		t.Fatalf("source sale status=%d response=%+v body=%s", status, saleResponse, body)
	}
	returnCommand := documentCommandRequest{
		CommandID: "00000000-0000-4000-8000-000000000202", Kind: "cash-return", Action: "save-and-post",
		IdempotencyKey: "sale-return-post", OccurredAt: "2026-08-06T00:00:00Z",
		Document: &documentDraftRequest{Kind: "cash-return", OccurredAt: "2026-08-06T00:00:00Z", SourceDocumentID: saleResponse.Document.ID,
			SourceDocumentNumber: saleResponse.Document.DocumentNumber, GodownID: fixture.godownID,
			Lines: []documentLineRequest{{ItemID: fixture.itemID, SourceLineID: saleResponse.Document.Lines[0].ID,
				Quantity: "1", UnitPrice: "10.00"}}},
	}
	status, returnResponse, body := executeDocumentHandler(t, server, operator, returnCommand)
	if status != 200 || returnResponse.Document.Status != "posted" || returnResponse.Document.Finance == nil || !returnResponse.Document.Finance.Balanced {
		t.Fatalf("return status=%d response=%+v body=%s", status, returnResponse, body)
	}
	var balance, direction, entryKind string
	if err := database.QueryRowContext(ctx, `SELECT on_hand::text FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_id = (SELECT id FROM stock_batches WHERE tenant_id = $1::uuid AND batch_number = 'RET-001')`, fixture.tenantID, fixture.branchID).Scan(&balance); err != nil {
		t.Fatalf("read returned stock balance: %v", err)
	}
	if balance != "3.0000" {
		t.Fatalf("returned stock balance = %s, want 3.0000", balance)
	}
	if err := database.QueryRowContext(ctx, `SELECT direction FROM stock_ledger WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, returnResponse.Document.ID).Scan(&direction); err != nil {
		t.Fatalf("read return stock ledger: %v", err)
	}
	if direction != "in" {
		t.Fatalf("return stock direction = %s, want in", direction)
	}
	if err := database.QueryRowContext(ctx, `SELECT entry_kind FROM party_ledger_entries WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, returnResponse.Document.ID).Scan(&entryKind); err != nil {
		t.Fatalf("read return party ledger: %v", err)
	}
	if entryKind != "sale-return" {
		t.Fatalf("return party entry kind = %s, want sale-return", entryKind)
	}
	replayStatus, replay, replayBody := executeDocumentHandler(t, server, operator, returnCommand)
	if replayStatus != 200 || !replay.Duplicate || replayBody == "" {
		t.Fatalf("return replay status=%d duplicate=%v body=%s", replayStatus, replay.Duplicate, replayBody)
	}

	overReturn := returnCommand
	overReturn.CommandID = "00000000-0000-4000-8000-000000000204"
	overReturn.IdempotencyKey = "sale-return-over"
	overDraft := *returnCommand.Document
	overDraft.Lines = append([]documentLineRequest(nil), returnCommand.Document.Lines...)
	overReturn.Document = &overDraft
	beforeOverDocuments := countBusinessDocuments(t, ctx, database, fixture.tenantID)
	beforeOverStock := countStockLedger(t, ctx, database, fixture.tenantID)
	overStatus, _, overBody := executeDocumentHandlerStatus(t, server, operator, overReturn)
	if overStatus != http.StatusConflict && overStatus != http.StatusUnprocessableEntity {
		t.Fatalf("over-return status=%d body=%s; want rejection", overStatus, overBody)
	}
	if countBusinessDocuments(t, ctx, database, fixture.tenantID) != beforeOverDocuments ||
		countStockLedger(t, ctx, database, fixture.tenantID) != beforeOverStock {
		t.Fatal("over-return rejection mutated the document or stock projection")
	}

	openReturnCommand := documentCommandRequest{
		CommandID: "00000000-0000-4000-8000-000000000203", Kind: "open-cash-return", Action: "save-and-post",
		IdempotencyKey: "open-sale-return-post", OccurredAt: "2026-08-06T00:00:00Z",
		Document: &documentDraftRequest{Kind: "open-cash-return", OccurredAt: "2026-08-06T00:00:00Z", GodownID: fixture.godownID,
			Lines: []documentLineRequest{{ItemID: fixture.itemID, Quantity: "2", UnitPrice: "5.00",
				BatchNumber: "OPEN-RETURN-001"}}},
	}
	openStatus, openResponse, openBody := executeDocumentHandler(t, server, operator, openReturnCommand)
	if openStatus != 200 || openResponse.Document.Status != "posted" || openResponse.Document.SourceDocumentID != "" || openResponse.Document.Finance == nil || !openResponse.Document.Finance.Balanced {
		t.Fatalf("open return status=%d response=%+v body=%s", openStatus, openResponse, openBody)
	}
	var openBalance string
	openBatchNumber := "OPEN-RETURN-001"
	if err := database.QueryRowContext(ctx, `
		SELECT on_hand::text FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND batch_id = (SELECT id FROM stock_batches WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		                  AND batch_number = $3)
	`, fixture.tenantID, fixture.branchID, openBatchNumber).Scan(&openBalance); err != nil {
		t.Fatalf("read open return stock balance: %v", err)
	}
	if openBalance != "2.0000" {
		t.Fatalf("open return stock balance = %s, want 2.0000", openBalance)
	}
}

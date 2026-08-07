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
	other := seedStockTenant(t, ctx, database, "sale-return-other-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id IN ($1::uuid, $2::uuid)`, fixture.tenantID, other.tenantID)
	}()
	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "sale-return-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "RET-001", ExpiryDate: "2030-01-01", Quantity: []byte(`3`), UnitCost: "4.00",
	})
	operator := &sessionContext{UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID, CounterID: fixture.counterID, Roles: []string{"tenant_admin"}}
	otherOperator := &sessionContext{UserID: other.operatorID, TenantID: other.tenantID, BranchID: other.branchID, CounterID: other.counterID, Roles: []string{"tenant_admin"}}
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
	beforeReplayStock := countStockLedger(t, ctx, database, fixture.tenantID)
	beforeReplayJournals := countFinanceJournals(t, ctx, database, fixture.tenantID)
	beforeReplayParty := countPartyLedgerEntries(t, ctx, database, fixture.tenantID)
	beforeReplayReversals := countReturnReversals(t, ctx, database, fixture.tenantID)
	replayStatus, replay, replayBody := executeDocumentHandler(t, server, operator, returnCommand)
	if replayStatus != 200 || !replay.Duplicate || replayBody == "" {
		t.Fatalf("return replay status=%d duplicate=%v body=%s", replayStatus, replay.Duplicate, replayBody)
	}
	if countStockLedger(t, ctx, database, fixture.tenantID) != beforeReplayStock ||
		countFinanceJournals(t, ctx, database, fixture.tenantID) != beforeReplayJournals ||
		countPartyLedgerEntries(t, ctx, database, fixture.tenantID) != beforeReplayParty ||
		countReturnReversals(t, ctx, database, fixture.tenantID) != beforeReplayReversals {
		t.Fatal("idempotent return replay changed stock, GL, party ledger, or reversal rows")
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

	voidReturn := returnCommand
	voidReturn.CommandID = "00000000-0000-4000-8000-000000000205"
	voidReturn.IdempotencyKey = "sale-return-void"
	voidReturn.Action = "void"
	voidReturn.Document = nil
	voidReturn.DocumentID = returnResponse.Document.ID
	voidReturn.ExpectedVersion = pointerInt64(returnResponse.Document.Version)
	voidReturn.Reason = "Return correction"
	beforeVoidDocuments := countBusinessDocuments(t, ctx, database, fixture.tenantID)
	beforeVoidStock := countStockLedger(t, ctx, database, fixture.tenantID)
	beforeVoidJournals := countFinanceJournals(t, ctx, database, fixture.tenantID)
	beforeVoidParty := countPartyLedgerEntries(t, ctx, database, fixture.tenantID)
	voidStatus, voidedReturn, voidBody := executeDocumentHandler(t, server, operator, voidReturn)
	if voidStatus != http.StatusOK || !voidedReturn.Accepted || voidedReturn.Document.Status != "void" || voidedReturn.Document.Finance == nil || !voidedReturn.Document.Finance.Balanced {
		t.Fatalf("posted return void status=%d body=%s response=%+v; want balanced compensating reversal", voidStatus, voidBody, voidedReturn)
	}
	if countBusinessDocuments(t, ctx, database, fixture.tenantID) != beforeVoidDocuments ||
		countStockLedger(t, ctx, database, fixture.tenantID) != beforeVoidStock+1 ||
		countFinanceJournals(t, ctx, database, fixture.tenantID) != beforeVoidJournals+1 ||
		countPartyLedgerEntries(t, ctx, database, fixture.tenantID) != beforeVoidParty+1 {
		t.Fatal("posted return void did not append exactly one stock, GL, and party reversal")
	}
	var voidStockDirection, voidJournalKind, voidPartyKind string
	if err := database.QueryRowContext(ctx, `SELECT direction FROM stock_ledger WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid ORDER BY created_at DESC, id DESC LIMIT 1`, fixture.tenantID, returnResponse.Document.ID).Scan(&voidStockDirection); err != nil {
		t.Fatalf("read return void stock reversal: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT kind FROM gl_journals WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid ORDER BY created_at DESC, id DESC LIMIT 1`, fixture.tenantID, returnResponse.Document.ID).Scan(&voidJournalKind); err != nil {
		t.Fatalf("read return void journal reversal: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT entry_kind FROM party_ledger_entries WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid ORDER BY created_at DESC, id DESC LIMIT 1`, fixture.tenantID, returnResponse.Document.ID).Scan(&voidPartyKind); err != nil {
		t.Fatalf("read return void party reversal: %v", err)
	}
	if voidStockDirection != "out" || voidJournalKind != "void-reversal" || voidPartyKind != "void" {
		t.Fatalf("return void reversal kinds stock=%q journal=%q party=%q", voidStockDirection, voidJournalKind, voidPartyKind)
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

	beforeOpenReplayStock := countStockLedger(t, ctx, database, fixture.tenantID)
	beforeOpenReplayJournals := countFinanceJournals(t, ctx, database, fixture.tenantID)
	beforeOpenReplayParty := countPartyLedgerEntries(t, ctx, database, fixture.tenantID)
	openReplayStatus, openReplay, openReplayBody := executeDocumentHandler(t, server, operator, openReturnCommand)
	if openReplayStatus != http.StatusOK || !openReplay.Duplicate || openReplayBody == "" {
		t.Fatalf("open return replay status=%d duplicate=%v body=%s", openReplayStatus, openReplay.Duplicate, openReplayBody)
	}
	if countStockLedger(t, ctx, database, fixture.tenantID) != beforeOpenReplayStock ||
		countFinanceJournals(t, ctx, database, fixture.tenantID) != beforeOpenReplayJournals ||
		countPartyLedgerEntries(t, ctx, database, fixture.tenantID) != beforeOpenReplayParty {
		t.Fatal("idempotent open-return replay changed stock, GL, or party ledger rows")
	}

	crossSource := returnCommand
	crossSource.CommandID = "00000000-0000-4000-8000-000000000206"
	crossSource.IdempotencyKey = "cross-tenant-sale-return"
	crossSource.Document = &documentDraftRequest{
		Kind: "cash-return", OccurredAt: returnCommand.Document.OccurredAt,
		SourceDocumentID: saleResponse.Document.ID, SourceDocumentNumber: saleResponse.Document.DocumentNumber,
		GodownID: other.godownID,
		Lines:    []documentLineRequest{{ItemID: other.itemID, SourceLineID: saleResponse.Document.Lines[0].ID, Quantity: "1", UnitPrice: "10.00"}},
	}
	beforeOtherDocuments := countBusinessDocuments(t, ctx, database, other.tenantID)
	beforeOtherStock := countStockLedger(t, ctx, database, other.tenantID)
	crossSourceStatus, _, crossSourceBody := executeDocumentHandlerStatus(t, server, otherOperator, crossSource)
	if crossSourceStatus != http.StatusUnprocessableEntity {
		t.Fatalf("cross-tenant source return status=%d body=%s; want rejection", crossSourceStatus, crossSourceBody)
	}
	if countBusinessDocuments(t, ctx, database, other.tenantID) != beforeOtherDocuments ||
		countStockLedger(t, ctx, database, other.tenantID) != beforeOtherStock {
		t.Fatal("cross-tenant source return changed the other tenant")
	}

	crossOpen := openReturnCommand
	crossOpen.CommandID = "00000000-0000-4000-8000-000000000207"
	crossOpen.IdempotencyKey = "cross-tenant-open-return"
	crossOpen.Document = &documentDraftRequest{
		Kind: "open-cash-return", OccurredAt: openReturnCommand.Document.OccurredAt,
		GodownID: fixture.godownID,
		Lines:    []documentLineRequest{{ItemID: other.itemID, Quantity: "1", UnitPrice: "5.00", BatchNumber: "CROSS-OPEN-001"}},
	}
	beforeOtherOpenDocuments := countBusinessDocuments(t, ctx, database, other.tenantID)
	crossOpenStatus, _, crossOpenBody := executeDocumentHandlerStatus(t, server, otherOperator, crossOpen)
	if crossOpenStatus != http.StatusUnprocessableEntity {
		t.Fatalf("cross-tenant open return status=%d body=%s; want rejection", crossOpenStatus, crossOpenBody)
	}
	if countBusinessDocuments(t, ctx, database, other.tenantID) != beforeOtherOpenDocuments {
		t.Fatal("cross-tenant open return changed the other tenant")
	}
}

func countPartyLedgerEntries(t *testing.T, ctx context.Context, database *sql.DB, tenantID string) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM party_ledger_entries WHERE tenant_id = $1::uuid`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count party ledger entries: %v", err)
	}
	return count
}

func countReturnReversals(t *testing.T, ctx context.Context, database *sql.DB, tenantID string) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_document_reversals WHERE tenant_id = $1::uuid`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count return reversals: %v", err)
	}
	return count
}

package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostedDocumentVoidUsesAtomicCompensatingReversal(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "void-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() { cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID) }()
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}

	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "void-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "VOID-001", ExpiryDate: "2030-01-01", Quantity: json.RawMessage(`5`), UnitCost: "4.00",
	})
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
	post := stockDocumentCommand(fixture, "save-and-post", "void-sale", "1")
	post.CommandID = "00000000-0000-4000-8000-000000000701"
	post.OccurredAt = "2026-08-07T00:00:00Z"
	post.Document.OccurredAt = post.OccurredAt
	status, posted, body := executeDocumentHandler(t, server, operator, post)
	if status != 200 || posted.Document.Status != "posted" || posted.Document.Finance == nil || !posted.Document.Finance.Balanced {
		t.Fatalf("post status=%d body=%s response=%+v", status, body, posted)
	}
	var beforeBalance string
	if err := database.QueryRowContext(ctx, `
		SELECT on_hand::text FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3
		  AND batch_id = (SELECT id FROM stock_batches WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_number = 'VOID-001')
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&beforeBalance); err != nil {
		t.Fatalf("read balance before void: %v", err)
	}
	if beforeBalance != "4.0000" {
		t.Fatalf("balance before void = %s, want 4.0000", beforeBalance)
	}

	void := documentCommandRequest{
		CommandID: "00000000-0000-4000-8000-000000000702", Kind: "cash-sale", Action: "void",
		IdempotencyKey: "void-sale-command", OccurredAt: "2026-08-07T01:00:00Z",
		DocumentID: posted.Document.ID, ExpectedVersion: &posted.Document.Version, Reason: "Operator voided the sale",
	}
	voidStatus, voided, voidBody := executeDocumentHandler(t, server, operator, void)
	if voidStatus != 200 || !voided.Accepted || voided.Document.Status != "void" || voided.Document.Finance == nil || !voided.Document.Finance.Balanced {
		t.Fatalf("void status=%d body=%s response=%+v", voidStatus, voidBody, voided)
	}

	var afterBalance string
	if err := database.QueryRowContext(ctx, `
		SELECT on_hand::text FROM stock_balances
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3
		  AND batch_id = (SELECT id FROM stock_batches WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_number = 'VOID-001')
	`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&afterBalance); err != nil {
		t.Fatalf("read balance after void: %v", err)
	}
	if afterBalance != "5.0000" {
		t.Fatalf("balance after void = %s, want 5.0000", afterBalance)
	}
	var stockRows, journalRows, partyRows, reversalRows int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM stock_ledger WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, posted.Document.ID).Scan(&stockRows); err != nil {
		t.Fatalf("count stock reversal rows: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM gl_journals WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, posted.Document.ID).Scan(&journalRows); err != nil {
		t.Fatalf("count finance reversal rows: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM party_ledger_entries WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, posted.Document.ID).Scan(&partyRows); err != nil {
		t.Fatalf("count party reversal rows: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_document_void_reversals WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, posted.Document.ID).Scan(&reversalRows); err != nil {
		t.Fatalf("count void reversal rows: %v", err)
	}
	if stockRows != 2 || journalRows != 2 || partyRows != 2 || reversalRows != 1 {
		t.Fatalf("reversal counts stock=%d journals=%d party=%d records=%d; want 2,2,2,1", stockRows, journalRows, partyRows, reversalRows)
	}
	var reversalKind, entryKind string
	if err := database.QueryRowContext(ctx, `SELECT kind FROM gl_journals WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid ORDER BY created_at DESC LIMIT 1`, fixture.tenantID, posted.Document.ID).Scan(&reversalKind); err != nil {
		t.Fatalf("read reversal journal kind: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT entry_kind FROM party_ledger_entries WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid ORDER BY created_at DESC LIMIT 1`, fixture.tenantID, posted.Document.ID).Scan(&entryKind); err != nil {
		t.Fatalf("read reversal party entry kind: %v", err)
	}
	if reversalKind != "void-reversal" || entryKind != "void" {
		t.Fatalf("reversal kinds journal=%q party=%q", reversalKind, entryKind)
	}

	replayStatus, replay, replayBody := executeDocumentHandler(t, server, operator, void)
	if replayStatus != 200 || !replay.Duplicate || replayBody == "" {
		t.Fatalf("void replay status=%d duplicate=%v body=%s", replayStatus, replay.Duplicate, replayBody)
	}
	var replayRows int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_document_void_reversals WHERE tenant_id = $1::uuid AND source_document_id = $2::uuid`, fixture.tenantID, posted.Document.ID).Scan(&replayRows); err != nil {
		t.Fatalf("count replay reversal rows: %v", err)
	}
	if replayRows != 1 {
		t.Fatalf("void replay created additional reversal rows: %d", replayRows)
	}
}

func TestVoidReversalStockDeltaSign(t *testing.T) {
	for _, test := range []struct {
		direction string
		sign      int
		want      int
	}{
		{direction: "in", sign: 1, want: -1},
		{direction: "out", sign: 1, want: 1},
		{direction: "adjustment", sign: 1, want: -1},
		{direction: "adjustment", sign: -1, want: 1},
		{direction: "unknown", sign: 1, want: 0},
	} {
		if got := inverseStockDeltaSign(test.direction, test.sign); got != test.want {
			t.Fatalf("inverseStockDeltaSign(%q, %d) = %d, want %d", test.direction, test.sign, got, test.want)
		}
	}
}

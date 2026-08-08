package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestBusinessDocumentLifecycleIntegration(t *testing.T) {
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

	tenantID, branchID, counterID, operatorID, itemID := seedDocumentFixture(t, ctx, database)
	tenantTwoID, branchTwoID, counterTwoID, operatorTwoID := seedDocumentTenant(t, ctx, database, fmt.Sprintf("other-%d", time.Now().UnixNano()))
	defer func() {
		cleanupIsolatedLegacyTenant(ctx, database, tenantID, tenantTwoID)
	}()

	server := &Server{database: database}
	operator := &sessionContext{
		UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID,
		Roles: []string{"tenant_admin"},
	}
	command := validDocumentCommand("save")
	command.Document.Lines[0].ItemID = itemID
	initialCommand := command
	var updateCommand documentCommandRequest

	t.Run("save creates a draft and revision", func(t *testing.T) {
		response := executeDocumentCommandForTest(t, ctx, server, operator, command)
		if response.Document.Status != "draft" || response.Document.Version != 1 {
			t.Fatalf("draft response = %+v", response.Document)
		}
		var eventExists bool
		if err := database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sync_events WHERE event_id = $1::uuid AND aggregate = 'business_document')`, response.EventID).Scan(&eventExists); err != nil {
			t.Fatalf("event lookup: %v", err)
		}
		if !eventExists {
			t.Fatalf("eventId %q did not identify the immutable sync event", response.EventID)
		}
		var revisions int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_document_revisions WHERE tenant_id = $1::uuid`, tenantID).Scan(&revisions); err != nil {
			t.Fatalf("revision count: %v", err)
		}
		if revisions != 1 {
			t.Fatalf("revisions = %d, want 1", revisions)
		}
		updateCommand = command
		updateDraft := *command.Document
		updateDraft.Lines = append([]documentLineRequest(nil), command.Document.Lines...)
		updateCommand.Document = &updateDraft
		updateCommand.CommandID = "00000000-0000-0000-0000-000000000010"
		updateCommand.IdempotencyKey = "cash-sale-update-1"
		updateCommand.Document.ID = response.Document.ID
		updateCommand.ExpectedVersion = pointerInt64(response.Document.Version)
	})

	t.Run("same key replays and different payload conflicts", func(t *testing.T) {
		replayed := executeDocumentCommandForTest(t, ctx, server, operator, initialCommand)
		if !replayed.Duplicate {
			t.Fatal("same command was not marked duplicate")
		}
		if replayed.EventID == "" {
			t.Fatal("replayed command lost the immutable eventId")
		}
		conflicting := initialCommand
		conflicting.CommandID = "00000000-0000-0000-0000-000000000011"
		conflicting.Document.Lines[0].Quantity = "2"
		tx, err := server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin conflict transaction: %v", err)
		}
		_, duplicate, err := claimDocumentCommand(ctx, tx, operator, conflicting, mustDocumentHash(t, conflicting))
		if !errors.Is(err, errDocumentCommandConflict) || duplicate {
			t.Fatalf("conflicting command result duplicate=%v err=%v", duplicate, err)
		}
		_ = tx.Rollback()
	})

	t.Run("revision conflict leaves document unchanged", func(t *testing.T) {
		conflicting := updateCommand
		conflicting.CommandID = "00000000-0000-0000-0000-000000000012"
		conflicting.IdempotencyKey = "cash-sale-revision-conflict"
		conflicting.ExpectedVersion = pointerInt64(99)
		tx, err := server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin revision transaction: %v", err)
		}
		_, _, err = server.saveBusinessDocument(ctx, tx, operator, conflicting)
		if err == nil {
			t.Fatal("stale revision was accepted")
		}
		_ = tx.Rollback()
		var version int64
		if err := database.QueryRowContext(ctx, `SELECT version FROM business_documents WHERE id = $1::uuid`, updateCommand.Document.ID).Scan(&version); err != nil {
			t.Fatalf("read version: %v", err)
		}
		if version != 1 {
			t.Fatalf("version after rejected update = %d, want 1", version)
		}
	})

	t.Run("posted documents without completed projections reject void", func(t *testing.T) {
		post := updateCommand
		post.Action = "post"
		post.CommandID = "00000000-0000-0000-0000-000000000015"
		post.IdempotencyKey = "cash-sale-post-1"
		posted := executeDocumentCommandForTest(t, ctx, server, operator, post)
		if posted.Document.Status != "posted" || posted.Document.Version != 2 {
			t.Fatalf("posted response = %+v", posted.Document)
		}
		void := post
		void.Action = "void"
		void.CommandID = "00000000-0000-0000-0000-000000000016"
		void.IdempotencyKey = "cash-sale-void-1"
		void.Document = nil
		void.DocumentID = posted.Document.ID
		void.Reason = "Integration cleanup"
		void.ExpectedVersion = pointerInt64(posted.Document.Version)
		tx, err := server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin posted void: %v", err)
		}
		if _, _, err := server.voidBusinessDocument(ctx, tx, operator, void); err == nil {
			_ = tx.Rollback()
			t.Fatal("posted document was voided without completed reversal projections")
		}
		_ = tx.Rollback()
		var revisions, stockRows, syncEvents int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_document_revisions WHERE tenant_id = $1::uuid`, tenantID).Scan(&revisions); err != nil {
			t.Fatalf("revision count: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM inventory_movements WHERE tenant_id = $1::uuid`, tenantID).Scan(&stockRows); err != nil {
			t.Fatalf("stock count: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM sync_events WHERE tenant_id = $1::uuid AND aggregate = 'business_document'`, tenantID).Scan(&syncEvents); err != nil {
			t.Fatalf("sync event count: %v", err)
		}
		if revisions != 2 || syncEvents != 2 || stockRows != 0 {
			t.Fatalf("revisions=%d syncEvents=%d stockRows=%d; rejected void must not mutate state", revisions, syncEvents, stockRows)
		}
	})

	t.Run("failed canonical validation makes no mutation", func(t *testing.T) {
		invalid := validDocumentCommand("save")
		invalid.CommandID = "00000000-0000-0000-0000-000000000013"
		invalid.IdempotencyKey = "cash-sale-invalid-item"
		invalid.Document.Lines[0].ItemID = "00000000-0000-0000-0000-000000000099"
		before := countBusinessDocuments(t, ctx, database, tenantID)
		tx, err := server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin validation transaction: %v", err)
		}
		_, _, err = server.saveBusinessDocument(ctx, tx, operator, invalid)
		if err == nil {
			t.Fatal("inactive/missing canonical item was accepted")
		}
		_ = tx.Rollback()
		if after := countBusinessDocuments(t, ctx, database, tenantID); after != before {
			t.Fatalf("document count changed after failed validation: before=%d after=%d", before, after)
		}
	})

	t.Run("tenant isolation rejects another tenant's item", func(t *testing.T) {
		other := &sessionContext{
			UserID: operatorTwoID, TenantID: tenantTwoID, BranchID: branchTwoID, CounterID: counterTwoID,
			Roles: []string{"tenant_admin"},
		}
		crossTenant := validDocumentCommand("save")
		crossTenant.CommandID = "00000000-0000-0000-0000-000000000014"
		crossTenant.IdempotencyKey = "cross-tenant-item"
		crossTenant.Document.Lines[0].ItemID = itemID
		tx, err := server.beginScopedTx(ctx, other)
		if err != nil {
			t.Fatalf("begin tenant transaction: %v", err)
		}
		_, _, err = server.saveBusinessDocument(ctx, tx, other, crossTenant)
		if err == nil {
			t.Fatal("cross-tenant canonical item was accepted")
		}
		_ = tx.Rollback()
	})
}

func executeDocumentCommandForTest(t *testing.T, ctx context.Context, server *Server, operator *sessionContext, command documentCommandRequest) documentCommandResponse {
	t.Helper()
	tx, err := server.beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin document transaction: %v", err)
	}
	defer tx.Rollback()
	hash := mustDocumentHash(t, command)
	receiptID, duplicate, err := claimDocumentCommand(ctx, tx, operator, command, hash)
	if err != nil {
		t.Fatalf("claim command: %v", err)
	}
	if duplicate {
		var raw []byte
		if err := tx.QueryRowContext(ctx, `SELECT response FROM command_receipts WHERE id = $1::uuid`, receiptID).Scan(&raw); err != nil {
			t.Fatalf("read duplicate receipt: %v", err)
		}
		var response documentCommandResponse
		if err := jsonUnmarshal(raw, &response); err != nil {
			t.Fatalf("decode duplicate receipt: %v", err)
		}
		response.Duplicate = true
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit duplicate: %v", err)
		}
		return response
	}
	var documentID string
	var response documentCommandResponse
	if command.Action == "void" {
		documentID, response, err = server.voidBusinessDocument(ctx, tx, operator, command)
	} else {
		documentID, response, err = server.saveBusinessDocument(ctx, tx, operator, command)
	}
	if err != nil {
		t.Fatalf("save document: %v", err)
	}
	response.Accepted = true
	response.AggregateID = documentID
	response.Kind = command.Kind
	response.Action = command.Action
	response.Status = response.Document.Status
	eventID, err := emitDocumentSyncEvent(ctx, tx, operator, command, response)
	if err != nil {
		t.Fatalf("emit sync event: %v", err)
	}
	response.EventID = eventID
	if err := finalizeDocumentSyncEvent(ctx, tx, operator, command, eventID, response.Document); err != nil {
		t.Fatalf("finalize sync event: %v", err)
	}
	raw := mustJSON(response)
	if _, err := tx.ExecContext(ctx, `UPDATE command_receipts SET document_id = $1::uuid, response = $2::jsonb WHERE id = $3::uuid`, documentID, raw, receiptID); err != nil {
		t.Fatalf("complete receipt: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO business_document_revisions
			(tenant_id, branch_id, document_id, revision_number, action, status, snapshot)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::jsonb)
	`, operator.TenantID, operator.BranchID, documentID, response.Document.Version, command.Action, response.Document.Status, raw); err != nil {
		t.Fatalf("save revision: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit document: %v", err)
	}
	return response
}

func seedDocumentFixture(t *testing.T, ctx context.Context, database *sql.DB) (string, string, string, string, string) {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, suffix)
	var itemID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_items (tenant_id, legacy_id, code, name, payload)
		VALUES ($1::uuid, $2, $3, 'Integration Item', '{"SalePrice1":"10.00"}'::jsonb)
		RETURNING id::text
	`, tenantID, "integration-"+suffix, "integration-"+suffix).Scan(&itemID); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	return tenantID, branchID, counterID, operatorID, itemID
}

func seedDocumentTenant(t *testing.T, ctx context.Context, database *sql.DB, suffix string) (string, string, string, string) {
	t.Helper()
	var tenantID, branchID, counterID, operatorID string
	if err := database.QueryRowContext(ctx, `INSERT INTO tenants (code, legal_name) VALUES ($1, $2) RETURNING id::text`, "document-test-"+suffix, "Document Test").Scan(&tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO branches (tenant_id, code, name) VALUES ($1::uuid, $2, 'Test Branch') RETURNING id::text`, tenantID, "branch-"+suffix).Scan(&branchID); err != nil {
		t.Fatalf("seed branch: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO counters (tenant_id, branch_id, code, name) VALUES ($1::uuid, $2::uuid, $3, 'Test Counter') RETURNING id::text`, tenantID, branchID, "counter-"+suffix).Scan(&counterID); err != nil {
		t.Fatalf("seed counter: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO users (tenant_id, username, display_name, password_hash) VALUES ($1::uuid, $2, 'Document Test', 'unused-hash') RETURNING id::text`, tenantID, "document-user-"+suffix).Scan(&operatorID); err != nil {
		t.Fatalf("seed operator: %v", err)
	}
	return tenantID, branchID, counterID, operatorID
}

func countBusinessDocuments(t *testing.T, ctx context.Context, database *sql.DB, tenantID string) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE tenant_id = $1::uuid`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	return count
}

func mustDocumentHash(t *testing.T, command documentCommandRequest) string {
	t.Helper()
	hash, err := hashDocumentCommand(command)
	if err != nil {
		t.Fatalf("hash command: %v", err)
	}
	return hash
}

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

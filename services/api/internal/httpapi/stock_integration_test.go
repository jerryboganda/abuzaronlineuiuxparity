package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestStockLifecycleIntegration(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "stock-"+fmt.Sprint(time.Now().UnixNano()))
	other := seedStockTenant(t, ctx, database, "other-stock-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id IN ($1::uuid, $2::uuid)`, fixture.tenantID, other.tenantID)
	}()
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}

	t.Run("receiving projects a batch into the new ledger", func(t *testing.T) {
		event := insertInventoryEvent(t, ctx, database, fixture, "receiving", "receive-1", inventoryRowPayload{
			ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
			BatchNumber: "B-001", ExpiryDate: "2030-01-01", Quantity: json.RawMessage(`7`), UnitCost: "4.00",
		})
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin receiving: %v", err)
		}
		if err := projectEvent(ctx, tx, event); err != nil {
			_ = tx.Rollback()
			t.Fatalf("project receiving: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit receiving: %v", err)
		}
		var balance string
		if err := database.QueryRowContext(ctx, `SELECT on_hand::text FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&balance); err != nil {
			t.Fatalf("read receiving balance: %v", err)
		}
		if balance != "7.0000" {
			t.Fatalf("receiving balance = %s, want 7.0000", balance)
		}
	})

	t.Run("signed adjustments persist and rebuild with their sign", func(t *testing.T) {
		sign := -1
		event := insertInventoryEvent(t, ctx, database, fixture, "inventory", "adjust-negative-1", inventoryRowPayload{
			ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
			BatchNumber: "B-001", Quantity: json.RawMessage(`2`), Direction: "adjustment",
			AdjustmentSign: &sign,
		})
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin signed adjustment: %v", err)
		}
		if err := projectEvent(ctx, tx, event); err != nil {
			_ = tx.Rollback()
			t.Fatalf("project signed adjustment: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit signed adjustment: %v", err)
		}
		var balance string
		if err := database.QueryRowContext(ctx, `SELECT on_hand::text FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&balance); err != nil {
			t.Fatalf("read adjusted balance: %v", err)
		}
		if balance != "5.0000" {
			t.Fatalf("adjusted balance = %s, want 5.0000", balance)
		}
		var persisted int
		if err := database.QueryRowContext(ctx, `SELECT adjustment_sign FROM stock_ledger WHERE tenant_id = $1::uuid AND source_event_id = $2::uuid`, fixture.tenantID, event.EventID).Scan(&persisted); err != nil {
			t.Fatalf("read persisted adjustment sign: %v", err)
		}
		if persisted != -1 {
			t.Fatalf("persisted adjustment sign = %d, want -1", persisted)
		}
	})

	t.Run("missing adjustment sign is rejected atomically", func(t *testing.T) {
		before := countStockLedger(t, ctx, database, fixture.tenantID)
		event := insertInventoryEvent(t, ctx, database, fixture, "inventory", "adjust-missing-sign", inventoryRowPayload{
			ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
			BatchNumber: "B-001", Quantity: json.RawMessage(`1`), Direction: "adjustment",
		})
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin missing-sign adjustment: %v", err)
		}
		if err := projectEvent(ctx, tx, event); err == nil {
			_ = tx.Rollback()
			t.Fatal("missing adjustment sign was accepted")
		}
		_ = tx.Rollback()
		if after := countStockLedger(t, ctx, database, fixture.tenantID); after != before {
			t.Fatalf("rejected adjustment changed ledger count from %d to %d", before, after)
		}
	})

	t.Run("draft save does not mutate stock", func(t *testing.T) {
		before := countStockLedger(t, ctx, database, fixture.tenantID)
		command := stockDocumentCommand(fixture, "save", "draft-stock", "1")
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin draft: %v", err)
		}
		if _, _, err := (&Server{database: database}).saveBusinessDocument(ctx, tx, operator, command); err != nil {
			_ = tx.Rollback()
			t.Fatalf("save draft: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit draft: %v", err)
		}
		if after := countStockLedger(t, ctx, database, fixture.tenantID); after != before {
			t.Fatalf("draft changed ledger count from %d to %d", before, after)
		}
	})

	var posted documentCommandResponse
	var postedCommand documentCommandRequest
	t.Run("post allocates FIFO stock and a replay is idempotent", func(t *testing.T) {
		server := &Server{database: database}
		save := stockDocumentCommand(fixture, "save", "sale-save", "1")
		tx, err := server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin sale draft: %v", err)
		}
		documentID, draft, err := server.saveBusinessDocument(ctx, tx, operator, save)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("save sale draft: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit sale draft: %v", err)
		}
		post := stockDocumentCommand(fixture, "post", "sale-post", "1")
		post.Document.ID = documentID
		post.ExpectedVersion = pointerInt64(draft.Document.Version)
		postedCommand = post
		tx, err = server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin sale post: %v", err)
		}
		_, posted, err = server.saveBusinessDocument(ctx, tx, operator, post)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("save sale post: %v", err)
		}
		eventID := insertEventInTransaction(t, ctx, tx, operator, "business_document", documentID, "sale-post")
		if err := projectPostedSaleStock(ctx, tx, operator, post.Document, posted.Document, eventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("allocate sale stock: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit sale post: %v", err)
		}
		var balance string
		if err := database.QueryRowContext(ctx, `SELECT on_hand::text FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&balance); err != nil {
			t.Fatalf("read sale balance: %v", err)
		}
		if balance != "4.0000" {
			t.Fatalf("sale balance = %s, want 4.0000", balance)
		}
		before := countStockLedger(t, ctx, database, fixture.tenantID)
		tx, err = server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin sale replay: %v", err)
		}
		if err := projectPostedSaleStock(ctx, tx, operator, postedCommand.Document, posted.Document, eventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("replay post: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit sale replay: %v", err)
		}
		if after := countStockLedger(t, ctx, database, fixture.tenantID); after != before {
			t.Fatalf("idempotent post added ledger rows: before=%d after=%d", before, after)
		}
	})

	t.Run("expired, locked, and insufficient stock are rejected", func(t *testing.T) {
		expired := stockDocumentCommand(fixture, "save-and-post", "expired-sale", "1")
		expired.Document.Lines[0].Allocations = []documentAllocationRequest{{BatchNumber: "EXPIRED", Quantity: "1"}}
		if err := assertStockPostRejected(t, ctx, database, operator, expired, "expired"); err == nil {
			t.Fatal("expired batch was accepted")
		}
		lockEvent := insertInventoryEvent(t, ctx, database, fixture, "receiving", "receive-locked", inventoryRowPayload{
			ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
			BatchNumber: "LOCKED", ExpiryDate: "2030-01-01", Quantity: json.RawMessage(`2`), UnitCost: "4.00",
		})
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin locked receiving: %v", err)
		}
		if err := projectEvent(ctx, tx, lockEvent); err != nil {
			_ = tx.Rollback()
			t.Fatalf("project locked receiving: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit locked receiving: %v", err)
		}
		if _, err := database.ExecContext(ctx, `UPDATE stock_batches SET locked = true WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_number = 'LOCKED'`, fixture.tenantID, fixture.branchID); err != nil {
			t.Fatalf("lock batch: %v", err)
		}
		locked := stockDocumentCommand(fixture, "save-and-post", "locked-sale", "1")
		locked.Document.Lines[0].Allocations = []documentAllocationRequest{{BatchNumber: "LOCKED", Quantity: "1"}}
		if err := assertStockPostRejected(t, ctx, database, operator, locked, "locked"); err == nil {
			t.Fatal("locked batch was accepted")
		}
		insufficient := stockDocumentCommand(fixture, "save-and-post", "insufficient-sale", "99")
		if err := assertStockPostRejected(t, ctx, database, operator, insufficient, "insufficient"); err == nil {
			t.Fatal("insufficient sale was accepted")
		}
	})

	t.Run("tenant isolation and balance rebuild hold", func(t *testing.T) {
		otherOperator := &sessionContext{UserID: other.operatorID, TenantID: other.tenantID, BranchID: other.branchID, CounterID: other.counterID, Roles: []string{"tenant_admin"}}
		isolationTx := mustBeginScopedTx(t, database, otherOperator)
		if _, err := findStockBatch(ctx, isolationTx, otherOperator, fixture.itemID, fixture.godownID, documentAllocationRequest{BatchNumber: "B-001", Quantity: "1"}, time.Now()); err == nil {
			_ = isolationTx.Rollback()
			t.Fatal("other tenant resolved a foreign batch")
		}
		_ = isolationTx.Rollback()
		if _, err := database.ExecContext(ctx, `UPDATE stock_balances SET on_hand = 99 WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`, fixture.tenantID, fixture.branchID); err != nil {
			t.Fatalf("corrupt cache for rebuild test: %v", err)
		}
		request := httptest.NewRequest("POST", "/v1/inventory/rebuild", nil)
		request = request.WithContext(context.WithValue(request.Context(), sessionContextKey, &sessionContext{
			UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
			CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
		}))
		recorder := httptest.NewRecorder()
		(&Server{database: database}).rebuildStockBalances(recorder, request)
		if recorder.Code != 200 {
			t.Fatalf("rebuild status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var balance string
		if err := database.QueryRowContext(ctx, `SELECT on_hand::text FROM stock_balances WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND item_legacy_id = $3 AND batch_id = (SELECT id FROM stock_batches WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND batch_number = 'B-001')`, fixture.tenantID, fixture.branchID, fixture.itemLegacyID).Scan(&balance); err != nil {
			t.Fatalf("read rebuilt balance: %v", err)
		}
		if balance != "4.0000" {
			t.Fatalf("rebuilt balance = %s, want 4.0000", balance)
		}
	})
}

type stockFixture struct {
	tenantID, branchID, counterID, operatorID string
	itemID, itemLegacyID, godownID            string
}

func seedStockTenant(t *testing.T, ctx context.Context, database *sql.DB, suffix string) stockFixture {
	t.Helper()
	var fixture stockFixture
	if err := database.QueryRowContext(ctx, `INSERT INTO tenants (code, legal_name) VALUES ($1, 'Stock Test') RETURNING id::text`, suffix).Scan(&fixture.tenantID); err != nil {
		t.Fatalf("seed stock tenant: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO branches (tenant_id, code, name) VALUES ($1::uuid, 'main', 'Main') RETURNING id::text`, fixture.tenantID).Scan(&fixture.branchID); err != nil {
		t.Fatalf("seed stock branch: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO counters (tenant_id, branch_id, code, name) VALUES ($1::uuid, $2::uuid, 'counter', 'Counter') RETURNING id::text`, fixture.tenantID, fixture.branchID).Scan(&fixture.counterID); err != nil {
		t.Fatalf("seed stock counter: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO users (tenant_id, username, display_name, password_hash) VALUES ($1::uuid, $2, 'Stock User', 'unused') RETURNING id::text`, fixture.tenantID, suffix).Scan(&fixture.operatorID); err != nil {
		t.Fatalf("seed stock operator: %v", err)
	}
	fixture.itemLegacyID = "item-" + suffix
	if err := database.QueryRowContext(ctx, `INSERT INTO master_items (tenant_id, legacy_id, code, name, payload) VALUES ($1::uuid, $2, $2, 'Stock Item', '{"SalePrice1":"10.00"}') RETURNING id::text`, fixture.tenantID, fixture.itemLegacyID).Scan(&fixture.itemID); err != nil {
		t.Fatalf("seed stock item: %v", err)
	}
	if err := database.QueryRowContext(ctx, `INSERT INTO master_godowns (tenant_id, legacy_id, code, name) VALUES ($1::uuid, $2, $2, 'Main Godown') RETURNING id::text`, fixture.tenantID, "godown-"+suffix).Scan(&fixture.godownID); err != nil {
		t.Fatalf("seed stock godown: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO stock_batches (tenant_id, branch_id, item_id, item_legacy_id, godown_id, batch_number, expiry_date, unit_cost) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, 'EXPIRED', CURRENT_DATE - 1, 4.00)`, fixture.tenantID, fixture.branchID, fixture.itemID, fixture.itemLegacyID, fixture.godownID); err != nil {
		t.Fatalf("seed expired batch: %v", err)
	}
	return fixture
}

func stockDocumentCommand(fixture stockFixture, action, key, quantity string) documentCommandRequest {
	return documentCommandRequest{
		CommandID: key + "-command-00000000-0000-0000-0000-000000000001",
		Kind:      "cash-sale", Action: action, IdempotencyKey: key, OccurredAt: "2026-08-06T00:00:00Z",
		Document: &documentDraftRequest{Kind: "cash-sale", OccurredAt: "2026-08-06T00:00:00Z", GodownID: fixture.godownID,
			Lines: []documentLineRequest{{ItemID: fixture.itemID, Quantity: quantity, UnitPrice: "10.00"}}},
	}
}

func insertInventoryEvent(t *testing.T, ctx context.Context, database *sql.DB, fixture stockFixture, aggregate, key string, row inventoryRowPayload) syncEvent {
	t.Helper()
	payload := mustJSON(inventoryPayload{Rows: []inventoryRowPayload{row}})
	var event syncEvent
	if err := database.QueryRowContext(ctx, `
		INSERT INTO sync_events (event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id, idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, gen_random_uuid(), $6, 1, $7::jsonb, '2026-08-06T00:00:00Z')
		RETURNING event_id::text, aggregate_id::text
	`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, aggregate, key, payload).Scan(&event.EventID, &event.AggregateID); err != nil {
		t.Fatalf("insert inventory event: %v", err)
	}
	event.TenantID, event.BranchID, event.CounterID, event.OperatorID = fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID
	event.Aggregate, event.IdempotencyKey, event.SchemaVersion, event.OccurredAt, event.Payload = aggregate, key, 1, "2026-08-06T00:00:00Z", payload
	return event
}

func insertEventInTransaction(t *testing.T, ctx context.Context, tx *sql.Tx, operator *sessionContext, aggregate, aggregateID, key string) string {
	t.Helper()
	var eventID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO sync_events (event_id, tenant_id, branch_id, counter_id, operator_id, aggregate, aggregate_id, idempotency_key, schema_version, payload, occurred_at)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::uuid, $7, 1, '{}'::jsonb, '2026-08-06T00:00:00Z')
		RETURNING event_id::text
	`, operator.TenantID, operator.BranchID, operator.CounterID, operator.UserID, aggregate, aggregateID, key).Scan(&eventID); err != nil {
		t.Fatalf("insert document event: %v", err)
	}
	return eventID
}

func assertStockPostRejected(t *testing.T, ctx context.Context, database *sql.DB, operator *sessionContext, command documentCommandRequest, expected string) error {
	t.Helper()
	server := &Server{database: database}
	tx, err := server.beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin rejected post: %v", err)
	}
	_, response, err := server.saveBusinessDocument(ctx, tx, operator, command)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("%s stock post was rejected before stock projection: %v", expected, err)
	}
	eventID := insertEventInTransaction(t, ctx, tx, operator, "business_document", response.Document.ID, command.IdempotencyKey)
	err = projectPostedSaleStock(ctx, tx, operator, command.Document, response.Document, eventID)
	if err == nil {
		_ = tx.Rollback()
		t.Fatalf("%s stock post was accepted", expected)
	}
	_ = tx.Rollback()
	return err
}

func countStockLedger(t *testing.T, ctx context.Context, database *sql.DB, tenantID string) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM stock_ledger WHERE tenant_id = $1::uuid`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count stock ledger: %v", err)
	}
	return count
}

func mustBeginScopedTx(t *testing.T, database *sql.DB, operator *sessionContext) *sql.Tx {
	t.Helper()
	tx, err := (&Server{database: database}).beginScopedTx(context.Background(), operator)
	if err != nil {
		t.Fatalf("begin isolation transaction: %v", err)
	}
	return tx
}

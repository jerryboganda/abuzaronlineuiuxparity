package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFinanceSalePostingIntegration(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "finance-"+fmt.Sprint(time.Now().UnixNano()))
	other := seedStockTenant(t, ctx, database, "other-finance-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() {
		cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID, other.tenantID)
	}()
	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "finance-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "FIN-001", ExpiryDate: "2030-01-01", Quantity: []byte(`20`), UnitCost: "4.00",
	})
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
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

	var customerID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO master_parties
			(tenant_id, party_type, legacy_id, code, name)
		VALUES ($1::uuid, 'customer', $2, $2, 'Finance Customer')
		RETURNING id::text
	`, fixture.tenantID, "customer-"+fixture.itemLegacyID).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	t.Run("HTTP document command posts finance and preserves idempotency", func(t *testing.T) {
		server := &Server{database: database}
		command := documentCommandRequest{
			CommandID:      "00000000-0000-0000-0000-000000000021",
			Kind:           "cash-sale",
			Action:         "save-and-post",
			IdempotencyKey: "finance-handler-post",
			OccurredAt:     "2026-08-06T00:00:00Z",
			Document: &documentDraftRequest{
				Kind:       "cash-sale",
				OccurredAt: "2026-08-06T00:00:00Z",
				GodownID:   fixture.godownID,
				Lines: []documentLineRequest{{
					LineNumber: 1, ItemID: fixture.itemID, Quantity: "1", UnitPrice: "10.00",
				}},
			},
		}
		status, response, body := executeDocumentHandler(t, server, operator, command)
		if status != http.StatusOK {
			t.Fatalf("HTTP post status=%d body=%s", status, body)
		}
		if response.Document.Status != "posted" || response.Document.Finance == nil || !response.Document.Finance.Balanced {
			t.Fatalf("HTTP post response finance=%+v status=%s", response.Document.Finance, response.Document.Status)
		}
		var eventPayload struct {
			State    string           `json:"state"`
			EventID  string           `json:"eventId"`
			Document documentResponse `json:"document"`
		}
		var rawEventPayload []byte
		if err := database.QueryRowContext(ctx, `
			SELECT payload FROM sync_events
			WHERE tenant_id = $1::uuid AND event_id = $2::uuid
		`, fixture.tenantID, response.EventID).Scan(&rawEventPayload); err != nil {
			t.Fatalf("read finalized document event: %v", err)
		}
		if err := json.Unmarshal(rawEventPayload, &eventPayload); err != nil {
			t.Fatalf("decode finalized document event: %v", err)
		}
		if eventPayload.State != "final" || eventPayload.EventID != response.EventID ||
			eventPayload.Document.Finance == nil || !eventPayload.Document.Finance.Balanced ||
			len(eventPayload.Document.Lines) == 0 || len(eventPayload.Document.Lines[0].Allocations) == 0 {
			t.Fatalf("sync event did not contain final stock/finance summary: %+v", eventPayload)
		}
		var finalized bool
		if err := database.QueryRowContext(ctx, `
			SELECT finalized_at IS NOT NULL
			FROM sync_events
			WHERE tenant_id = $1::uuid AND event_id = $2::uuid
		`, fixture.tenantID, response.EventID).Scan(&finalized); err != nil {
			t.Fatalf("read finalized timestamp: %v", err)
		}
		if !finalized {
			t.Fatal("canonical final event did not record finalized_at")
		}
		tx, err := server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin final event immutability probe: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE sync_events
			SET payload = payload
			WHERE tenant_id = $1::uuid AND event_id = $2::uuid
		`, fixture.tenantID, response.EventID); err == nil {
			_ = tx.Rollback()
			t.Fatal("finalized business document event was mutable")
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM sync_events
			WHERE tenant_id = $1::uuid AND event_id = $2::uuid
		`, fixture.tenantID, response.EventID); err == nil {
			_ = tx.Rollback()
			t.Fatal("finalized business document event was deletable")
		}
		_ = tx.Rollback()
		tx, err = server.beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin compatible event delete probe: %v", err)
		}
		var pendingEventID, legacyEventID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO sync_events
				(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate,
				 aggregate_id, idempotency_key, schema_version, payload, occurred_at)
			VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid,
			        'business_document', $5::uuid, 'delete-pending-probe', 1,
			        '{"state":"pending"}'::jsonb, '2026-08-06T00:00:00Z')
			RETURNING event_id::text
		`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, response.Document.ID).Scan(&pendingEventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert pending event probe: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sync_events WHERE event_id = $1::uuid`, pendingEventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("pending event should remain deletable inside its transaction: %v", err)
		}
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO sync_events
				(event_id, tenant_id, branch_id, counter_id, operator_id, aggregate,
				 aggregate_id, idempotency_key, schema_version, payload, occurred_at)
			VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4::uuid,
			        'legacy_event', $5::uuid, 'delete-legacy-probe', 1,
			        '{}'::jsonb, '2026-08-06T00:00:00Z')
			RETURNING event_id::text
		`, fixture.tenantID, fixture.branchID, fixture.counterID, fixture.operatorID, response.Document.ID).Scan(&legacyEventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert legacy event probe: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sync_events WHERE event_id = $1::uuid`, legacyEventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("legacy state-less event should remain deletable: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit compatible event delete probe: %v", err)
		}
		beforeReplay := countFinanceJournals(t, ctx, database, fixture.tenantID)
		replayStatus, replay, replayBody := executeDocumentHandler(t, server, operator, command)
		if replayStatus != http.StatusOK || !replay.Duplicate || replayBody == "" {
			t.Fatalf("HTTP replay status=%d duplicate=%v body=%s", replayStatus, replay.Duplicate, replayBody)
		}
		if afterReplay := countFinanceJournals(t, ctx, database, fixture.tenantID); afterReplay != beforeReplay {
			t.Fatalf("HTTP replay added journals: before=%d after=%d", beforeReplay, afterReplay)
		}

		draftCommand := command
		draftCommand.CommandID = "00000000-0000-0000-0000-000000000022"
		draftCommand.IdempotencyKey = "finance-handler-draft"
		draftCommand.Action = "save"
		draftCommand.Document = &documentDraftRequest{
			Kind: "cash-sale", OccurredAt: command.OccurredAt,
			Lines: command.Document.Lines,
		}
		draftStatus, draftResponse, draftBody := executeDocumentHandler(t, server, operator, draftCommand)
		if draftStatus != http.StatusCreated || draftBody == "" || draftResponse.Document.Status != "draft" || draftResponse.Document.Finance != nil {
			t.Fatalf("HTTP draft status=%d response=%+v body=%s", draftStatus, draftResponse, draftBody)
		}

		var documentsBefore, stockBefore, eventsBefore int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&documentsBefore); err != nil {
			t.Fatalf("count documents before HTTP finance failure: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM stock_ledger WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&stockBefore); err != nil {
			t.Fatalf("count stock before HTTP finance failure: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM sync_events WHERE tenant_id = $1::uuid AND aggregate = 'business_document'`, fixture.tenantID).Scan(&eventsBefore); err != nil {
			t.Fatalf("count events before HTTP finance failure: %v", err)
		}
		if _, err := database.ExecContext(ctx, `UPDATE finance_accounts SET active = false WHERE tenant_id = $1::uuid AND system_key = 'cogs'`, fixture.tenantID); err != nil {
			t.Fatalf("disable required finance account: %v", err)
		}
		failedCommand := command
		failedCommand.CommandID = "00000000-0000-0000-0000-000000000023"
		failedCommand.IdempotencyKey = "finance-handler-failure"
		failedStatus, _, failedBody := executeDocumentHandler(t, server, operator, failedCommand)
		_, _ = database.ExecContext(ctx, `UPDATE finance_accounts SET active = true WHERE tenant_id = $1::uuid AND system_key = 'cogs'`, fixture.tenantID)
		if failedStatus != http.StatusUnprocessableEntity || failedBody == "" {
			t.Fatalf("HTTP finance failure status=%d body=%s", failedStatus, failedBody)
		}
		var documentsAfter, stockAfter, eventsAfter int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&documentsAfter); err != nil {
			t.Fatalf("count documents after HTTP finance failure: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM stock_ledger WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&stockAfter); err != nil {
			t.Fatalf("count stock after HTTP finance failure: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM sync_events WHERE tenant_id = $1::uuid AND aggregate = 'business_document'`, fixture.tenantID).Scan(&eventsAfter); err != nil {
			t.Fatalf("count events after HTTP finance failure: %v", err)
		}
		if documentsAfter != documentsBefore || stockAfter != stockBefore || eventsAfter != eventsBefore {
			t.Fatalf("HTTP finance failure mutated documents/stock/events: before=%d/%d/%d after=%d/%d/%d", documentsBefore, stockBefore, eventsBefore, documentsAfter, stockAfter, eventsAfter)
		}
	})

	t.Run("draft has no finance projection", func(t *testing.T) {
		command := stockDocumentCommand(fixture, "save", "finance-draft", "1")
		before := countFinanceJournals(t, ctx, database, fixture.tenantID)
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
		if after := countFinanceJournals(t, ctx, database, fixture.tenantID); after != before {
			t.Fatalf("draft created %d finance journals", after-before)
		}
	})

	t.Run("database rejects an unbalanced journal", func(t *testing.T) {
		command := stockDocumentCommand(fixture, "save", "finance-unbalanced-document", "1")
		documentID, _ := saveFinanceDraft(t, ctx, database, operator, command)
		var cashAccountID string
		if err := database.QueryRowContext(ctx, `
			SELECT id::text FROM finance_accounts
			WHERE tenant_id = $1::uuid AND system_key = 'cash'
		`, fixture.tenantID).Scan(&cashAccountID); err != nil {
			t.Fatalf("read cash account: %v", err)
		}
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin unbalanced journal: %v", err)
		}
		eventID := insertEventInTransaction(t, ctx, tx, operator, "business_document", documentID, "finance-unbalanced-event")
		var journalID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO gl_journals
				(tenant_id, branch_id, source_event_id, source_document_id, kind,
				 posted_at, total_debit, total_credit)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'cash-sale',
			        '2026-08-06T00:00:00Z', 1, 1)
			RETURNING id::text
		`, fixture.tenantID, fixture.branchID, eventID, documentID).Scan(&journalID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert unbalanced journal: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO gl_lines
				(tenant_id, branch_id, journal_id, line_number, account_id, debit_amount, credit_amount)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, 1, 0)
		`, fixture.tenantID, fixture.branchID, journalID, cashAccountID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert unbalanced line: %v", err)
		}
		if err := tx.Commit(); err == nil {
			t.Fatal("unbalanced journal committed")
		}
	})

	t.Run("cash sale posts cash, revenue, and stock cost", func(t *testing.T) {
		command := stockDocumentCommand(fixture, "save", "finance-cash-save", "1")
		documentID, draft := saveFinanceDraft(t, ctx, database, operator, command)
		post := stockDocumentCommand(fixture, "post", "finance-cash-post", "1")
		post.Document.ID = documentID
		post.ExpectedVersion = pointerInt64(draft.Document.Version)
		response, eventID := postFinanceSale(t, ctx, database, operator, post)
		if response.Document.Finance == nil || !response.Document.Finance.Balanced {
			t.Fatalf("cash finance summary = %+v", response.Document.Finance)
		}
		assertFinanceLines(t, ctx, database, fixture.tenantID, fixture.branchID, response.Document.ID,
			map[string][2]string{
				"cash":          {"11.0000", "0.0000"},
				"sales_revenue": {"0.0000", "11.0000"},
				"cogs":          {"4.0000", "0.0000"},
				"inventory":     {"0.0000", "4.0000"},
			})
		before := countFinanceJournals(t, ctx, database, fixture.tenantID)
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin finance replay: %v", err)
		}
		if err := projectPostedSaleFinance(ctx, tx, operator, response.Document, eventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("finance replay: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit finance replay: %v", err)
		}
		if after := countFinanceJournals(t, ctx, database, fixture.tenantID); after != before {
			t.Fatalf("finance replay added journals: before=%d after=%d", before, after)
		}
	})

	t.Run("credit sale posts receivable, output tax, and customer ledger", func(t *testing.T) {
		command := stockDocumentCommand(fixture, "save", "finance-credit-save", "1")
		command.Kind = "credit-sale"
		command.Document.Kind = "credit-sale"
		command.Document.CustomerID = customerID
		command.Document.Pricing = &documentPricingRequest{
			Taxes: pricingPreviewTaxes{GST: &pricingPreviewTax{Rate: "10"}},
		}
		documentID, draft := saveFinanceDraft(t, ctx, database, operator, command)
		post := command
		post.Action = "post"
		post.IdempotencyKey = "finance-credit-post"
		post.Document.ID = documentID
		post.ExpectedVersion = pointerInt64(draft.Document.Version)
		response, _ := postFinanceSale(t, ctx, database, operator, post)
		if response.Document.Finance == nil || response.Document.Finance.PartyLedgerEntryID == "" {
			t.Fatalf("credit finance summary = %+v", response.Document.Finance)
		}
		assertFinanceLines(t, ctx, database, fixture.tenantID, fixture.branchID, response.Document.ID,
			map[string][2]string{
				"accounts_receivable": {"12.1000", "0.0000"},
				"sales_revenue":       {"0.0000", "11.0000"},
				"output_tax":          {"0.0000", "1.1000"},
				"cogs":                {"4.0000", "0.0000"},
				"inventory":           {"0.0000", "4.0000"},
			})
		var balance string
		if err := database.QueryRowContext(ctx, `
			SELECT balance::text FROM party_ledger_balances
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
		`, fixture.tenantID, fixture.branchID, customerID).Scan(&balance); err != nil {
			t.Fatalf("read customer balance: %v", err)
		}
		if balance != "12.1000" {
			t.Fatalf("customer balance = %s, want 12.1000", balance)
		}
	})

	t.Run("tenant isolation and finance failure roll back the post", func(t *testing.T) {
		var otherJournals int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM gl_journals WHERE tenant_id = $1::uuid`, other.tenantID).Scan(&otherJournals); err != nil {
			t.Fatalf("count other tenant journals: %v", err)
		}
		if otherJournals != 0 {
			t.Fatalf("other tenant saw %d journals", otherJournals)
		}
		var documentCount, stockCount int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&documentCount); err != nil {
			t.Fatalf("count documents before failure: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM stock_ledger WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&stockCount); err != nil {
			t.Fatalf("count stock before failure: %v", err)
		}
		if _, err := database.ExecContext(ctx, `UPDATE finance_accounts SET active = false WHERE tenant_id = $1::uuid AND system_key = 'cogs'`, fixture.tenantID); err != nil {
			t.Fatalf("remove required account: %v", err)
		}
		command := stockDocumentCommand(fixture, "save-and-post", "finance-failure", "1")
		tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
		if err != nil {
			t.Fatalf("begin failed post: %v", err)
		}
		_, response, err := (&Server{database: database}).saveBusinessDocument(ctx, tx, operator, command)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("save failed-post document: %v", err)
		}
		eventID := insertEventInTransaction(t, ctx, tx, operator, "business_document", response.Document.ID, command.IdempotencyKey)
		if err := projectPostedSaleStock(ctx, tx, operator, command.Document, response.Document, eventID); err != nil {
			_ = tx.Rollback()
			t.Fatalf("stock setup for finance failure: %v", err)
		}
		if err := projectPostedSaleFinance(ctx, tx, operator, response.Document, eventID); err == nil {
			_ = tx.Rollback()
			t.Fatal("finance posted with a missing required account")
		}
		_ = tx.Rollback()
		var afterDocuments, afterStock int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&afterDocuments); err != nil {
			t.Fatalf("count documents after failure: %v", err)
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM stock_ledger WHERE tenant_id = $1::uuid`, fixture.tenantID).Scan(&afterStock); err != nil {
			t.Fatalf("count stock after failure: %v", err)
		}
		if afterDocuments != documentCount || afterStock != stockCount {
			t.Fatalf("failed finance post mutated documents/stock: before %d/%d after %d/%d", documentCount, stockCount, afterDocuments, afterStock)
		}
	})
}

func executeDocumentHandler(t *testing.T, server *Server, operator *sessionContext, command documentCommandRequest) (int, documentCommandResponse, string) {
	t.Helper()
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("marshal HTTP document command: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/documents/"+command.Kind, bytes.NewReader(payload))
	request.SetPathValue("kind", command.Kind)
	request = request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
	recorder := httptest.NewRecorder()
	server.documentCommand(recorder, request)
	body := recorder.Body.String()
	var response documentCommandResponse
	if recorder.Code < http.StatusBadRequest {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode HTTP document response: %v; body=%s", err, body)
		}
	}
	return recorder.Code, response, body
}

func saveFinanceDraft(t *testing.T, ctx context.Context, database *sql.DB, operator *sessionContext, command documentCommandRequest) (string, documentCommandResponse) {
	t.Helper()
	tx, err := (&Server{database: database}).beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin finance draft: %v", err)
	}
	documentID, response, err := (&Server{database: database}).saveBusinessDocument(ctx, tx, operator, command)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("save finance draft: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit finance draft: %v", err)
	}
	return documentID, response
}

func postFinanceSale(t *testing.T, ctx context.Context, database *sql.DB, operator *sessionContext, command documentCommandRequest) (documentCommandResponse, string) {
	t.Helper()
	server := &Server{database: database}
	tx, err := server.beginScopedTx(ctx, operator)
	if err != nil {
		t.Fatalf("begin finance post: %v", err)
	}
	_, response, err := server.saveBusinessDocument(ctx, tx, operator, command)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("save finance post: %v", err)
	}
	eventID := insertEventInTransaction(t, ctx, tx, operator, "business_document", response.Document.ID, command.IdempotencyKey)
	if err := projectPostedSaleStock(ctx, tx, operator, command.Document, response.Document, eventID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("stock finance post: %v", err)
	}
	if err := projectPostedSaleFinance(ctx, tx, operator, response.Document, eventID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("finance post: %v", err)
	}
	response.Document, err = readBusinessDocument(ctx, tx, operator, response.Document.ID)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("read finance post: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit finance post: %v", err)
	}
	return response, eventID
}

func countFinanceJournals(t *testing.T, ctx context.Context, database *sql.DB, tenantID string) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM gl_journals WHERE tenant_id = $1::uuid`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count finance journals: %v", err)
	}
	return count
}

func assertFinanceLines(t *testing.T, ctx context.Context, database *sql.DB, tenantID, branchID, documentID string, expected map[string][2]string) {
	t.Helper()
	rows, err := database.QueryContext(ctx, `
		SELECT a.system_key, l.debit_amount::text, l.credit_amount::text
		FROM gl_lines l
		JOIN gl_journals j ON j.tenant_id = l.tenant_id AND j.branch_id = l.branch_id AND j.id = l.journal_id
		JOIN finance_accounts a ON a.tenant_id = l.tenant_id AND a.id = l.account_id
		WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid AND j.source_document_id = $3::uuid
	`, tenantID, branchID, documentID)
	if err != nil {
		t.Fatalf("query finance lines: %v", err)
	}
	defer rows.Close()
	actual := make(map[string][2]string)
	for rows.Next() {
		var key, debit, credit string
		if err := rows.Scan(&key, &debit, &credit); err != nil {
			t.Fatalf("scan finance line: %v", err)
		}
		actual[key] = [2]string{debit, credit}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read finance lines: %v", err)
	}
	for key, line := range expected {
		if actual[key] != line {
			t.Errorf("finance line %s = %v, want %v", key, actual[key], line)
		}
	}
}

// TestCreditLimitEnforcement exercises enforceCreditSaleLimit (documents.go)
// through the same HTTP document-command path production traffic uses,
// covering: a credit-sale that stays within a positive CrLimit, one that
// would exceed it, and customers whose CrLimit is unset or explicitly 0 —
// both of which must always succeed regardless of their existing balance.
func TestCreditLimitEnforcement(t *testing.T) {
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

	fixture := seedStockTenant(t, ctx, database, "credit-limit-"+fmt.Sprint(time.Now().UnixNano()))
	defer func() {
		cleanupIsolatedLegacyTenant(ctx, database, fixture.tenantID)
	}()
	receive := insertInventoryEvent(t, ctx, database, fixture, "receiving", "credit-limit-receive", inventoryRowPayload{
		ItemLegacyID: fixture.itemLegacyID, GodownID: fixture.godownID,
		BatchNumber: "CRLIM-001", ExpiryDate: "2030-01-01", Quantity: []byte(`20`), UnitCost: "4.00",
	})
	operator := &sessionContext{
		UserID: fixture.operatorID, TenantID: fixture.tenantID, BranchID: fixture.branchID,
		CounterID: fixture.counterID, Roles: []string{"tenant_admin"},
	}
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

	newCustomer := func(t *testing.T, name, payload string) string {
		t.Helper()
		var customerID string
		key := "customer-" + name + "-" + fmt.Sprint(time.Now().UnixNano())
		if err := database.QueryRowContext(ctx, `
			INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name, payload)
			VALUES ($1::uuid, 'customer', $2, $2, $3, $4::jsonb)
			RETURNING id::text
		`, fixture.tenantID, key, name, payload).Scan(&customerID); err != nil {
			t.Fatalf("seed customer %s: %v", name, err)
		}
		return customerID
	}

	seedBalance := func(t *testing.T, customerID string, amount string) {
		t.Helper()
		if _, err := database.ExecContext(ctx, `
			INSERT INTO party_ledger_balances (tenant_id, branch_id, party_id, debit_total, credit_total, balance)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::numeric, 0, $4::numeric)
		`, fixture.tenantID, fixture.branchID, customerID, amount); err != nil {
			t.Fatalf("seed pre-existing balance: %v", err)
		}
	}

	readBalance := func(t *testing.T, customerID string) string {
		t.Helper()
		var balance string
		if err := database.QueryRowContext(ctx, `
			SELECT balance::text FROM party_ledger_balances
			WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
		`, fixture.tenantID, fixture.branchID, customerID).Scan(&balance); err != nil {
			t.Fatalf("read balance: %v", err)
		}
		return balance
	}

	creditSaleCommand := func(customerID, commandID, key, quantity string) documentCommandRequest {
		return documentCommandRequest{
			CommandID: commandID,
			Kind:      "credit-sale", Action: "save-and-post", IdempotencyKey: key, OccurredAt: "2026-08-06T00:00:00Z",
			Document: &documentDraftRequest{
				Kind: "credit-sale", OccurredAt: "2026-08-06T00:00:00Z", GodownID: fixture.godownID,
				CustomerID: customerID,
				Lines:      []documentLineRequest{{ItemID: fixture.itemID, Quantity: quantity, UnitPrice: "10.00"}},
			},
		}
	}

	// A single line of quantity 1 at UnitPrice "10.00" does not necessarily
	// post a total of exactly 10.00 (default tax/pricing policy can add to
	// it). Probe the real per-unit total once, under a customer with an
	// effectively unlimited CrLimit, so the within/exceeding-limit fixtures
	// below can be sized precisely off the actual total rather than an
	// assumption about tax configuration.
	probeCustomer := newCustomer(t, "Probe", `{"CrLimit":"999999.99"}`)
	probeCommand := creditSaleCommand(probeCustomer, "00000000-0000-0000-0000-0000000000c0", "credit-limit-probe", "1")
	probeStatus, probeResponse, probeBody := executeDocumentHandler(t, server, operator, probeCommand)
	if probeStatus != http.StatusOK || probeResponse.Document.Status != "posted" {
		t.Fatalf("probe credit-sale status=%d body=%s", probeStatus, probeBody)
	}
	unitTotal, err := parseMoney(probeResponse.Document.Totals.TotalAmount)
	if err != nil || unitTotal <= 0 {
		t.Fatalf("parse probe total %q: %v", probeResponse.Document.Totals.TotalAmount, err)
	}
	// A limit at 1.5x a single line's total sits strictly between one line's
	// total (must be allowed) and two lines' total (must be rejected).
	limit := unitTotal + unitTotal/2
	limitPayload := fmt.Sprintf(`{"CrLimit":"%s"}`, formatMoney(limit))
	wantBalanceAfterOne := formatMoney(unitTotal)

	t.Run("credit-sale within limit succeeds", func(t *testing.T) {
		customerID := newCustomer(t, "WithinLimit", limitPayload)
		command := creditSaleCommand(customerID, "00000000-0000-0000-0000-0000000000c1", "credit-limit-within", "1")
		status, response, body := executeDocumentHandler(t, server, operator, command)
		if status != http.StatusOK || response.Document.Status != "posted" {
			t.Fatalf("within-limit credit-sale status=%d body=%s", status, body)
		}
		balance, err := parseMoney(readBalance(t, customerID))
		if err != nil || balance != unitTotal {
			t.Fatalf("balance = %v (err=%v), want %s", balance, err, wantBalanceAfterOne)
		}
	})

	t.Run("credit-sale exceeding limit is rejected", func(t *testing.T) {
		customerID := newCustomer(t, "OverLimit", limitPayload)
		first := creditSaleCommand(customerID, "00000000-0000-0000-0000-0000000000c2", "credit-limit-over-first", "1")
		firstStatus, _, firstBody := executeDocumentHandler(t, server, operator, first)
		if firstStatus != http.StatusOK {
			t.Fatalf("first credit-sale status=%d body=%s", firstStatus, firstBody)
		}
		if balance, err := parseMoney(readBalance(t, customerID)); err != nil || balance != unitTotal {
			t.Fatalf("balance after first sale = %v (err=%v), want %s", balance, err, wantBalanceAfterOne)
		}
		second := creditSaleCommand(customerID, "00000000-0000-0000-0000-0000000000c3", "credit-limit-over-second", "1")
		secondStatus, _, secondBody := executeDocumentHandler(t, server, operator, second)
		if secondStatus != http.StatusUnprocessableEntity {
			t.Fatalf("second credit-sale status=%d body=%s, want %d rejection", secondStatus, secondBody, http.StatusUnprocessableEntity)
		}
		if !strings.Contains(secondBody, "credit limit") {
			t.Fatalf("rejection body did not mention the credit limit: %s", secondBody)
		}
		if balance, err := parseMoney(readBalance(t, customerID)); err != nil || balance != unitTotal {
			t.Fatalf("balance after rejected post = %v (err=%v), want %s (unchanged)", balance, err, wantBalanceAfterOne)
		}
	})

	t.Run("unset CrLimit always succeeds regardless of balance", func(t *testing.T) {
		customerID := newCustomer(t, "NoLimit", `{}`)
		seedBalance(t, customerID, "500.00")
		command := creditSaleCommand(customerID, "00000000-0000-0000-0000-0000000000c4", "credit-limit-none", "1")
		status, response, body := executeDocumentHandler(t, server, operator, command)
		if status != http.StatusOK || response.Document.Status != "posted" {
			t.Fatalf("no-limit credit-sale status=%d body=%s", status, body)
		}
	})

	t.Run("explicit zero CrLimit always succeeds regardless of balance", func(t *testing.T) {
		customerID := newCustomer(t, "ZeroLimit", `{"CrLimit":"0.00"}`)
		seedBalance(t, customerID, "500.00")
		command := creditSaleCommand(customerID, "00000000-0000-0000-0000-0000000000c5", "credit-limit-zero", "1")
		status, response, body := executeDocumentHandler(t, server, operator, command)
		if status != http.StatusOK || response.Document.Status != "posted" {
			t.Fatalf("zero-limit credit-sale status=%d body=%s", status, body)
		}
	})
}

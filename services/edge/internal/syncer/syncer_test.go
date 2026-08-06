package syncer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuzar/abuzar-next/services/edge/internal/store"
)

func TestSyncOncePushesAndPullsWithSessionCookie(t *testing.T) {
	local, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer local.Close()
	event := store.Event{
		EventID: "11111111-1111-1111-1111-111111111111", Aggregate: "sale", AggregateID: "22222222-2222-2222-2222-222222222222",
		TenantID: "33333333-3333-3333-3333-333333333333", BranchID: "44444444-4444-4444-4444-444444444444",
		CounterID: "55555555-5555-5555-5555-555555555555", OperatorID: "66666666-6666-6666-6666-666666666666",
		OccurredAt: "2026-08-05T00:00:00Z", IdempotencyKey: "sync-test-1", SchemaVersion: 1,
		Payload: json.RawMessage(`{"totalAmount":"3.25","status":"posted"}`),
	}
	if inserted, err := local.InsertEvent(context.Background(), event); err != nil || !inserted {
		t.Fatalf("insert local event: inserted=%v err=%v", inserted, err)
	}
	pulledEvent := event
	pulledEvent.EventID = "77777777-7777-7777-7777-777777777777"
	pulledEvent.AggregateID = "88888888-8888-8888-8888-888888888888"
	pulledEvent.IdempotencyKey = "sync-test-pulled"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "abuzar_session=central-session" {
			t.Fatalf("cookie = %q", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sync/push":
			var request struct {
				Events []store.Event `json:"events"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Events) != 1 {
				t.Fatalf("push body: %v events=%d", err, len(request.Events))
			}
			json.NewEncoder(w).Encode(map[string]int{"accepted": 1})
		case "/v1/sync/pull":
			json.NewEncoder(w).Encode(map[string]any{"events": []store.Event{pulledEvent}, "nextCursor": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(local, server.URL, "central-session")
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}
	result, err := client.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync once: %v", err)
	}
	if result.Pushed != 1 || result.Pulled != 1 {
		t.Fatalf("result = %+v", result)
	}
	pushedCursor, _ := local.Cursor(context.Background(), "central_pushed_sequence")
	pullCursor, _ := local.Cursor(context.Background(), "central_pull")
	if pushedCursor != "1" || pullCursor != "1" {
		t.Fatalf("cursors = pushed %q pull %q", pushedCursor, pullCursor)
	}
	pending, err := local.PendingCount(context.Background())
	if err != nil || pending != 0 {
		t.Fatalf("pending after sync = %d err=%v", pending, err)
	}
}

func TestNewRejectsMissingConfiguration(t *testing.T) {
	local, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer local.Close()
	if _, err := New(local, "", "token"); err == nil {
		t.Fatal("empty central URL accepted")
	}
	if _, err := New(local, "http://127.0.0.1:8080", ""); err == nil {
		t.Fatal("empty session token accepted")
	}
}

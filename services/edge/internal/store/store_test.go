package store

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEventIdempotency(t *testing.T) {
	localStore, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()

	event := Event{
		EventID:        "event-1",
		Aggregate:      "sale",
		AggregateID:    "sale-1",
		TenantID:       "tenant-1",
		BranchID:       "branch-1",
		CounterID:      "counter-1",
		OperatorID:     "operator-1",
		OccurredAt:     "2026-08-05T00:00:00Z",
		IdempotencyKey: "sale-1",
		SchemaVersion:  1,
		Payload:        json.RawMessage(`{"total":12}`),
	}
	inserted, err := localStore.InsertEvent(context.Background(), event)
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v", inserted, err)
	}
	inserted, err = localStore.InsertEvent(context.Background(), event)
	if err != nil || inserted {
		t.Fatalf("duplicate insert: inserted=%v err=%v", inserted, err)
	}

	events, cursor, err := localStore.EventsAfter(context.Background(), 0, 100)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if cursor != 1 || len(events) != 1 {
		t.Fatalf("unexpected event page: cursor=%d count=%d", cursor, len(events))
	}
	if string(events[0].Payload) != string(event.Payload) {
		t.Fatalf("payload round-trip changed: got %s want %s", events[0].Payload, event.Payload)
	}
}

func TestEventValidationRejectsMissingScope(t *testing.T) {
	event := Event{Payload: json.RawMessage(`{"total":12}`), SchemaVersion: 1}
	if err := event.Validate(); err == nil {
		t.Fatal("event without identity/scope was accepted")
	}
}

func TestEventValidationRejectsInvalidPayload(t *testing.T) {
	event := Event{
		EventID:        "event-1",
		Aggregate:      "sale",
		AggregateID:    "sale-1",
		TenantID:       "tenant-1",
		BranchID:       "branch-1",
		CounterID:      "counter-1",
		OperatorID:     "operator-1",
		OccurredAt:     "2026-08-05T00:00:00Z",
		IdempotencyKey: "sale-1",
		SchemaVersion:  1,
		Payload:        json.RawMessage(`not-json`),
	}
	if err := event.Validate(); err == nil {
		t.Fatal("invalid payload was accepted")
	}
}

func TestPulledEventsDoNotBecomeOutgoing(t *testing.T) {
	localStore, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	base := Event{
		EventID: "event-local", Aggregate: "sale", AggregateID: "sale-local", TenantID: "tenant", BranchID: "branch",
		CounterID: "counter", OperatorID: "operator", OccurredAt: "2026-08-05T00:00:00Z", IdempotencyKey: "local",
		SchemaVersion: 1, Payload: json.RawMessage(`{"total":1}`),
	}
	if inserted, err := localStore.InsertEvent(context.Background(), base); err != nil || !inserted {
		t.Fatalf("insert local event: inserted=%v err=%v", inserted, err)
	}
	pulled := base
	pulled.EventID = "event-central"
	pulled.AggregateID = "sale-central"
	pulled.IdempotencyKey = "central"
	if inserted, err := localStore.InsertPulledEvent(context.Background(), pulled); err != nil || !inserted {
		t.Fatalf("insert central event: inserted=%v err=%v", inserted, err)
	}
	events, cursor, err := localStore.OutgoingAfter(context.Background(), 0, 100)
	if err != nil || len(events) != 1 || cursor != 1 || events[0].EventID != base.EventID {
		t.Fatalf("outgoing events = %#v cursor=%d err=%v", events, cursor, err)
	}
	pending, err := localStore.PendingCount(context.Background())
	if err != nil || pending != 1 {
		t.Fatalf("pending before ack = %d err=%v", pending, err)
	}
	if err := localStore.SetCursor(context.Background(), "central_pushed_sequence", "1"); err != nil {
		t.Fatalf("ack local event: %v", err)
	}
	pending, err = localStore.PendingCount(context.Background())
	if err != nil || pending != 0 {
		t.Fatalf("pending after ack = %d err=%v", pending, err)
	}
}

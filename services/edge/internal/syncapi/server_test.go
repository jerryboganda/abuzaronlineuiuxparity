package syncapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuzar/abuzar-next/services/edge/internal/store"
)

func TestProtectedEdgeEndpointsRequireSharedSecret(t *testing.T) {
	localStore, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	handler := New(localStore, "test", "secret")
	event := store.Event{
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
	body, _ := json.Marshal(map[string]any{"events": []store.Event{event}})

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewReader(body)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorized.Code)
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", health.Code)
	}
	if health.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("health cache-control = %q, want no-store", health.Header().Get("Cache-Control"))
	}

	authorizedRequest := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewReader(body))
	authorizedRequest.Header.Set("Authorization", "Bearer secret")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusAccepted {
		t.Fatalf("authorized status = %d, want 202", authorized.Code)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/sync/status", nil)
	statusRequest.Header.Set("Authorization", "Bearer secret")
	status := httptest.NewRecorder()
	handler.ServeHTTP(status, statusRequest)
	if status.Code != http.StatusOK {
		t.Fatalf("status endpoint = %d, want 200", status.Code)
	}

	ackRequest := httptest.NewRequest(http.MethodPost, "/v1/sync/ack", bytes.NewBufferString(`{"cursor":1}`))
	ackRequest.Header.Set("Authorization", "Bearer secret")
	ackRequest.Header.Set("Content-Type", "application/json")
	ack := httptest.NewRecorder()
	handler.ServeHTTP(ack, ackRequest)
	if ack.Code != http.StatusAccepted {
		t.Fatalf("ack endpoint = %d, want 202", ack.Code)
	}
}

func TestPushRejectsOversizedBatches(t *testing.T) {
	localStore, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	handler := New(localStore, "test", "secret")
	events := make([]store.Event, 501)
	body, _ := json.Marshal(map[string]any{"events": events})
	request := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized batch status = %d, want 400", response.Code)
	}
}

func TestHardwareEndpointsDegradeWithoutConfiguredAdapters(t *testing.T) {
	localStore, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	handler := New(localStore, "test", "secret")

	normalize := httptest.NewRequest(http.MethodPost, "/v1/hardware/barcode/normalize", bytes.NewBufferString(`{"raw":" 890123\r\n"}`))
	normalize.Header.Set("Authorization", "secret")
	normalized := httptest.NewRecorder()
	handler.ServeHTTP(normalized, normalize)
	if normalized.Code != http.StatusOK || !bytes.Contains(normalized.Body.Bytes(), []byte(`"barcode":"890123"`)) {
		t.Fatalf("normalize response = %d %s", normalized.Code, normalized.Body.String())
	}

	kick := httptest.NewRequest(http.MethodPost, "/v1/hardware/cash-drawer/kick", bytes.NewBufferString(`{}`))
	kick.Header.Set("Authorization", "secret")
	kicked := httptest.NewRecorder()
	handler.ServeHTTP(kicked, kick)
	if kicked.Code != http.StatusServiceUnavailable {
		t.Fatalf("kick without adapter status = %d, want 503", kicked.Code)
	}

	printSlip := httptest.NewRequest(http.MethodPost, "/v1/hardware/print/sale-slip", bytes.NewBufferString(`{"invoiceNumber":"1","total":"1.00"}`))
	printSlip.Header.Set("Authorization", "secret")
	printed := httptest.NewRecorder()
	handler.ServeHTTP(printed, printSlip)
	if printed.Code != http.StatusServiceUnavailable {
		t.Fatalf("print without adapter status = %d, want 503", printed.Code)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/v1/hardware/cash-drawer/kick", bytes.NewBufferString(`{}`)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("hardware unauthorized status = %d, want 401", unauthorized.Code)
	}
}

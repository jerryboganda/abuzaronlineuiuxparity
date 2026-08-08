package syncapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuzar/abuzar-next/services/edge/internal/hardware"
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

	capabilities := httptest.NewRequest(http.MethodGet, "/v1/hardware/capabilities", nil)
	capabilities.Header.Set("Authorization", "secret")
	capabilityResponse := httptest.NewRecorder()
	handler.ServeHTTP(capabilityResponse, capabilities)
	if capabilityResponse.Code != http.StatusOK ||
		bytes.Contains(capabilityResponse.Body.Bytes(), []byte(`"available":true`)) {
		t.Fatalf("capabilities response = %d %s", capabilityResponse.Code, capabilityResponse.Body.String())
	}

	readiness := httptest.NewRequest(http.MethodGet, "/v1/hardware/readiness", nil)
	readiness.Header.Set("Authorization", "secret")
	readinessResponse := httptest.NewRecorder()
	handler.ServeHTTP(readinessResponse, readiness)
	if readinessResponse.Code != http.StatusOK ||
		!bytes.Contains(readinessResponse.Body.Bytes(), []byte(`"ready":false`)) ||
		!bytes.Contains(readinessResponse.Body.Bytes(), []byte(`"status":"unavailable"`)) {
		t.Fatalf("readiness response = %d %s", readinessResponse.Code, readinessResponse.Body.String())
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

	verify := httptest.NewRequest(http.MethodPost, "/v1/hardware/biometric/verify", bytes.NewBufferString(`{"sample":"AQI="}`))
	verify.Header.Set("Authorization", "secret")
	verified := httptest.NewRecorder()
	handler.ServeHTTP(verified, verify)
	if verified.Code != http.StatusServiceUnavailable {
		t.Fatalf("biometric verify without adapter status = %d, want 503", verified.Code)
	}

	sendEmail := httptest.NewRequest(http.MethodPost, "/v1/hardware/email/send", bytes.NewBufferString(`{"to":"a@example.test","subject":"s","body":"b"}`))
	sendEmail.Header.Set("Authorization", "secret")
	emailResponse := httptest.NewRecorder()
	handler.ServeHTTP(emailResponse, sendEmail)
	if emailResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("email send without adapter status = %d, want 503", emailResponse.Code)
	}

	sendSMS := httptest.NewRequest(http.MethodPost, "/v1/hardware/sms/send", bytes.NewBufferString(`{"to":"923001234567","message":"hi"}`))
	sendSMS.Header.Set("Authorization", "secret")
	smsResponse := httptest.NewRecorder()
	handler.ServeHTTP(smsResponse, sendSMS)
	if smsResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("sms send without adapter status = %d, want 503", smsResponse.Code)
	}
}

type routeTestBiometric struct{ result bool }

func (b routeTestBiometric) Verify(context.Context, []byte) (bool, error) { return b.result, nil }

type routeTestEmail struct{}

func (routeTestEmail) Send(context.Context, string, string, string) error { return nil }

type routeTestSMS struct{}

func (routeTestSMS) Send(context.Context, string, string) error { return nil }

func TestConfiguredChannelHardwareRoutesReportSuccess(t *testing.T) {
	localStore, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	registry := hardware.NewWithConfig(hardware.Config{
		Biometric: routeTestBiometric{result: true},
		Email:     routeTestEmail{},
		SMS:       routeTestSMS{},
	})
	handler := NewWithHardware(localStore, "test", "secret", registry)

	verify := httptest.NewRequest(http.MethodPost, "/v1/hardware/biometric/verify", bytes.NewBufferString(`{"sample":"AQI="}`))
	verify.Header.Set("Authorization", "secret")
	verified := httptest.NewRecorder()
	handler.ServeHTTP(verified, verify)
	if verified.Code != http.StatusOK || !bytes.Contains(verified.Body.Bytes(), []byte(`"verified":true`)) {
		t.Fatalf("biometric verify response = %d %s", verified.Code, verified.Body.String())
	}

	sendEmail := httptest.NewRequest(http.MethodPost, "/v1/hardware/email/send", bytes.NewBufferString(`{"to":"a@example.test","subject":"s","body":"b"}`))
	sendEmail.Header.Set("Authorization", "secret")
	emailResponse := httptest.NewRecorder()
	handler.ServeHTTP(emailResponse, sendEmail)
	if emailResponse.Code != http.StatusAccepted || !bytes.Contains(emailResponse.Body.Bytes(), []byte(`"sent":true`)) {
		t.Fatalf("email send response = %d %s", emailResponse.Code, emailResponse.Body.String())
	}

	sendSMS := httptest.NewRequest(http.MethodPost, "/v1/hardware/sms/send", bytes.NewBufferString(`{"to":"923001234567","message":"hi"}`))
	sendSMS.Header.Set("Authorization", "secret")
	smsResponse := httptest.NewRecorder()
	handler.ServeHTTP(smsResponse, sendSMS)
	if smsResponse.Code != http.StatusAccepted || !bytes.Contains(smsResponse.Body.Bytes(), []byte(`"sent":true`)) {
		t.Fatalf("sms send response = %d %s", smsResponse.Code, smsResponse.Body.String())
	}
}

func TestInvalidHardwareConfigurationNeverAttemptsOperation(t *testing.T) {
	localStore, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	registry := hardware.NewWithConfig(hardware.Config{PrinterProvider: "orphan-provider"})
	handler := NewWithHardware(localStore, "test", "secret", registry)

	request := httptest.NewRequest(http.MethodPost, "/v1/hardware/print/sale-slip", bytes.NewBufferString(`{"invoiceNumber":"1","total":"1.00"}`))
	request.Header.Set("Authorization", "secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable ||
		!bytes.Contains(response.Body.Bytes(), []byte(`"code":"hardware_configuration_invalid"`)) {
		t.Fatalf("invalid config response = %d %s", response.Code, response.Body.String())
	}
}

package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChannelMaintenanceKindClassification(t *testing.T) {
	for _, kind := range []string{"test-email", "send-email", "Test-Email"} {
		if !isEmailMaintenanceKind(kind) {
			t.Errorf("isEmailMaintenanceKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"test-sms", "send-sms", "Send-SMS"} {
		if !isSMSMaintenanceKind(kind) {
			t.Errorf("isSMSMaintenanceKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"backup-database", "check-database-integrity", "email"} {
		if isEmailMaintenanceKind(kind) || isSMSMaintenanceKind(kind) {
			t.Errorf("kind %q was misclassified as a channel-send maintenance kind", kind)
		}
	}
}

func TestEdgeChannelConfigFromEnv(t *testing.T) {
	t.Setenv("ABUZAR_EDGE_CHANNEL_URL", "")
	t.Setenv("ABUZAR_EDGE_CHANNEL_SECRET", "")
	if edgeChannelConfigFromEnv().configured() {
		t.Fatal("empty ABUZAR_EDGE_CHANNEL_URL must report unconfigured")
	}

	t.Setenv("ABUZAR_EDGE_CHANNEL_URL", "http://127.0.0.1:8091/")
	t.Setenv("ABUZAR_EDGE_CHANNEL_SECRET", "shh")
	cfg := edgeChannelConfigFromEnv()
	if !cfg.configured() || cfg.BaseURL != "http://127.0.0.1:8091" || cfg.Secret != "shh" {
		t.Fatalf("edgeChannelConfigFromEnv() = %+v, want trimmed trailing slash and secret", cfg)
	}
}

func TestSendViaEdgeReportsSentOnSuccess(t *testing.T) {
	var gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"sent":true}`))
	}))
	defer server.Close()

	cfg := edgeChannelConfig{BaseURL: server.URL, Secret: "branch-secret"}
	outcome, err := sendViaEdge(context.Background(), cfg, "/v1/hardware/email/send", map[string]string{"to": "a@b.test"})
	if err != nil || outcome != edgeChannelSent {
		t.Fatalf("sendViaEdge = %v, %v, want edgeChannelSent, nil", outcome, err)
	}
	if gotAuth != "Bearer branch-secret" {
		t.Fatalf("Authorization header = %q, want Bearer branch-secret", gotAuth)
	}
	if !strings.Contains(gotBody, "a@b.test") {
		t.Fatalf("request body = %q, want to contain recipient", gotBody)
	}
}

func TestSendViaEdgeReportsUnavailableWhenEdgeAdapterUnconfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"hardware_adapter_unavailable","detail":"no adapter"}`))
	}))
	defer server.Close()

	outcome, err := sendViaEdge(context.Background(), edgeChannelConfig{BaseURL: server.URL}, "/v1/hardware/sms/send", map[string]string{"to": "1"})
	if err != nil || outcome != edgeChannelUnavailable {
		t.Fatalf("sendViaEdge = %v, %v, want edgeChannelUnavailable, nil", outcome, err)
	}
}

func TestSendViaEdgeReturnsErrorOnUnexpectedFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"edge_status_failed","detail":"boom"}`))
	}))
	defer server.Close()

	outcome, err := sendViaEdge(context.Background(), edgeChannelConfig{BaseURL: server.URL}, "/v1/hardware/email/send", map[string]string{"to": "1"})
	if err == nil || outcome != edgeChannelUnavailable {
		t.Fatalf("sendViaEdge = %v, %v, want an error and edgeChannelUnavailable", outcome, err)
	}
}

func TestSendViaEdgeReturnsErrorWhenUnreachable(t *testing.T) {
	// A closed local port: guaranteed unreachable, never a real external host.
	listener := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachableURL := listener.URL
	listener.Close()

	_, err := sendViaEdge(context.Background(), edgeChannelConfig{BaseURL: unreachableURL}, "/v1/hardware/email/send", map[string]string{"to": "1"})
	if err == nil {
		t.Fatal("expected an error when the branch-edge service is unreachable")
	}
}

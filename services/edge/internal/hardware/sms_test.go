package hardware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSMSGatewayAdapterBuildsTemplatedRequestAndAcceptsSuccess(t *testing.T) {
	var (
		gotMethod string
		gotQuery  url.Values
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK: message queued"))
	}))
	defer server.Close()

	adapter, err := NewSMSGatewayAdapter(SMSGatewayConfig{
		URLTemplate:         server.URL + "/send?user={user}&password={password}&mask={mask}&apikey={apikey}&to={to}&msg={message}",
		Method:              http.MethodGet,
		User:                "abuzar",
		Password:            "s3cr3t",
		Mask:                "PHARMA",
		APIKey:              "key-123",
		SuccessBodyContains: "OK",
	})
	if err != nil {
		t.Fatalf("build sms gateway adapter: %v", err)
	}

	if err := adapter.Send(context.Background(), "923001234567", "Your order is ready"); err != nil {
		t.Fatalf("send via local fake gateway: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Fatalf("method = %s, want GET", gotMethod)
	}
	if gotQuery.Get("user") != "abuzar" || gotQuery.Get("password") != "s3cr3t" ||
		gotQuery.Get("mask") != "PHARMA" || gotQuery.Get("apikey") != "key-123" ||
		gotQuery.Get("to") != "923001234567" || gotQuery.Get("msg") != "Your order is ready" {
		t.Fatalf("query = %+v, placeholders were not substituted correctly", gotQuery)
	}
}

func TestSMSGatewayAdapterRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("gateway error"))
	}))
	defer server.Close()

	adapter, err := NewSMSGatewayAdapter(SMSGatewayConfig{
		URLTemplate: server.URL + "/send?to={to}&message={message}",
	})
	if err != nil {
		t.Fatalf("build sms gateway adapter: %v", err)
	}
	if err := adapter.Send(context.Background(), "923001234567", "hello"); err == nil {
		t.Fatal("expected 500 gateway status to be treated as failure")
	}
}

func TestSMSGatewayAdapterRejectsMissingSuccessMarker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ERROR: insufficient balance"))
	}))
	defer server.Close()

	adapter, err := NewSMSGatewayAdapter(SMSGatewayConfig{
		URLTemplate:         server.URL + "/send?to={to}&message={message}",
		SuccessBodyContains: "OK",
	})
	if err != nil {
		t.Fatalf("build sms gateway adapter: %v", err)
	}
	if err := adapter.Send(context.Background(), "923001234567", "hello"); err == nil {
		t.Fatal("expected missing success marker to be treated as failure")
	}
}

func TestSMSGatewayAdapterHonorsExplicitSuccessStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	adapter, err := NewSMSGatewayAdapter(SMSGatewayConfig{
		URLTemplate:       server.URL + "/send?to={to}&message={message}",
		SuccessStatusCode: http.StatusAccepted,
	})
	if err != nil {
		t.Fatalf("build sms gateway adapter: %v", err)
	}
	if err := adapter.Send(context.Background(), "923001234567", "hello"); err != nil {
		t.Fatalf("send with explicit success status: %v", err)
	}

	// 200 must now be rejected since only 202 is the configured success code.
	otherServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer otherServer.Close()
	otherAdapter, err := NewSMSGatewayAdapter(SMSGatewayConfig{
		URLTemplate:       otherServer.URL + "/send?to={to}&message={message}",
		SuccessStatusCode: http.StatusAccepted,
	})
	if err != nil {
		t.Fatalf("build sms gateway adapter: %v", err)
	}
	if err := otherAdapter.Send(context.Background(), "923001234567", "hello"); err == nil {
		t.Fatal("expected status 200 to be rejected when success status is pinned to 202")
	}
}

func TestSMSGatewayAdapterSupportsPostMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := NewSMSGatewayAdapter(SMSGatewayConfig{
		URLTemplate: server.URL + "/send?to={to}&message={message}",
		Method:      http.MethodPost,
	})
	if err != nil {
		t.Fatalf("build sms gateway adapter: %v", err)
	}
	if err := adapter.Send(context.Background(), "923001234567", "hello"); err != nil {
		t.Fatalf("post send: %v", err)
	}
}

func TestSMSGatewayConfigValidateRejectsIncompleteConfiguration(t *testing.T) {
	for _, config := range []SMSGatewayConfig{
		{},
		{URLTemplate: "https://gateway.test/send", Method: "PUT"},
	} {
		if _, err := NewSMSGatewayAdapter(config); !errors.Is(err, ErrSMSConfigInvalid) {
			t.Fatalf("NewSMSGatewayAdapter(%+v) = %v, want ErrSMSConfigInvalid", config, err)
		}
	}
}

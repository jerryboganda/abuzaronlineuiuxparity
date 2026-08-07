package syncapi

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuzar/abuzar-next/services/edge/internal/store"
)

//go:embed testdata/unavailable-hardware-acceptance.json
var unavailableHardwareAcceptanceFixture []byte

type unavailableHardwareCase struct {
	Name            string          `json:"name"`
	Path            string          `json:"path"`
	Body            json.RawMessage `json:"body"`
	ForbiddenFields []string        `json:"forbiddenFields"`
}

func TestUnavailableHardwareAcceptanceFixtureNeverReportsSuccess(t *testing.T) {
	var cases []unavailableHardwareCase
	if err := json.Unmarshal(unavailableHardwareAcceptanceFixture, &cases); err != nil {
		t.Fatalf("decode unavailable hardware fixture: %v", err)
	}

	localStore, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer localStore.Close()
	handler := New(localStore, "test", "secret")

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, testCase.Path, bytes.NewReader(testCase.Body))
			request.Header.Set("Authorization", "secret")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
			}
			if !bytes.Contains(response.Body.Bytes(), []byte(`"code":"hardware_adapter_unavailable"`)) {
				t.Fatalf("body = %s, want hardware_adapter_unavailable", response.Body.String())
			}
			for _, field := range testCase.ForbiddenFields {
				if bytes.Contains(response.Body.Bytes(), []byte(`"`+field+`"`)) {
					t.Fatalf("body = %s, must not contain success field %q", response.Body.String(), field)
				}
			}
		})
	}
}

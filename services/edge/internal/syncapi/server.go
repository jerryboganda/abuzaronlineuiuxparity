package syncapi

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abuzar/abuzar-next/services/edge/internal/hardware"
	"github.com/abuzar/abuzar-next/services/edge/internal/store"
)

type Server struct {
	store        *store.Store
	version      string
	sharedSecret string
	hardware     *hardware.Registry
}

type pushRequest struct {
	Events []store.Event `json:"events"`
}

func New(localStore *store.Store, version, sharedSecret string) http.Handler {
	return NewWithHardware(localStore, version, sharedSecret, hardware.New())
}

func NewWithHardware(localStore *store.Store, version, sharedSecret string, registry *hardware.Registry) http.Handler {
	if registry == nil {
		registry = hardware.New()
	}
	server := &Server{store: localStore, version: version, sharedSecret: sharedSecret, hardware: registry}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", server.health)
	mux.HandleFunc("GET /v1/sync/status", server.status)
	mux.HandleFunc("GET /v1/hardware/capabilities", server.hardwareCapabilities)
	mux.HandleFunc("GET /v1/hardware/readiness", server.hardwareReadiness)
	mux.HandleFunc("POST /v1/hardware/print/sale-slip", server.printSaleSlip)
	mux.HandleFunc("POST /v1/hardware/print/purchase-labels", server.printPurchaseLabels)
	mux.HandleFunc("POST /v1/hardware/barcode/normalize", server.normalizeBarcode)
	mux.HandleFunc("POST /v1/hardware/barcode/lookup", server.lookupBarcode)
	mux.HandleFunc("POST /v1/hardware/cash-drawer/kick", server.kickCashDrawer)
	mux.HandleFunc("POST /v1/hardware/biometric/verify", server.verifyBiometric)
	mux.HandleFunc("POST /v1/hardware/email/send", server.sendEmail)
	mux.HandleFunc("POST /v1/hardware/sms/send", server.sendSMS)
	mux.Handle("POST /v1/transactions/sales", server.transaction("sale"))
	mux.Handle("POST /v1/transactions/returns", server.transaction("return"))
	mux.Handle("POST /v1/transactions/receiving", server.transaction("receiving"))
	mux.Handle("POST /v1/transactions/inventory", server.transaction("inventory"))
	mux.Handle("POST /v1/shifts/open", server.transaction("shift.open"))
	mux.HandleFunc("POST /v1/shifts/{id}/close", server.shiftClose)
	mux.HandleFunc("POST /v1/sync/push", server.push)
	mux.HandleFunc("POST /v1/sync/ack", server.ack)
	mux.HandleFunc("GET /v1/sync/pull", server.pull)
	mux.HandleFunc("/", server.notFound)
	return withCORS(server.requireSecret(mux))
}

func (s *Server) requireSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/v1/health" || s.sharedSecret == "" {
			next.ServeHTTP(w, r)
			return
		}
		authorization := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if authorization == "" || subtle.ConstantTimeCompare([]byte(authorization), []byte(s.sharedSecret)) != 1 {
			writeProblem(w, http.StatusUnauthorized, "edge_authentication_required", "Edge authentication required", "The branch-edge shared secret is missing or invalid.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	database := "ok"
	status := "ok"
	if err := s.store.Ping(ctx); err != nil {
		database = "error"
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"service":  "edge",
		"database": database,
		"version":  s.version,
		"time":     time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	pending, err := s.store.PendingCount(ctx)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "edge_status_failed", "Unable to read edge status", "The local queue status could not be read.")
		return
	}
	cursor, err := s.store.Cursor(ctx, "central_pull")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "edge_status_failed", "Unable to read edge status", "The local synchronization cursor could not be read.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pendingEvents": pending, "centralCursor": cursor})
}

func (s *Server) hardwareCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"capabilities": s.hardware.Capabilities(r.Context())})
}

func (s *Server) hardwareReadiness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hardware.Readiness(r.Context()))
}

func (s *Server) printSaleSlip(w http.ResponseWriter, r *http.Request) {
	var slip hardware.SaleSlip
	if !decodeHardwareJSON(w, r, &slip) {
		return
	}
	result, err := s.hardware.PrintSaleSlip(r.Context(), slip)
	if err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) printPurchaseLabels(w http.ResponseWriter, r *http.Request) {
	var batch hardware.PurchaseLabelBatch
	if !decodeHardwareJSON(w, r, &batch) {
		return
	}
	result, err := s.hardware.PrintPurchaseLabels(r.Context(), batch)
	if err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) normalizeBarcode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Raw string `json:"raw"`
	}
	if !decodeHardwareJSON(w, r, &request) {
		return
	}
	code, err := hardware.NormalizeBarcode(request.Raw)
	if err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"barcode": code})
}

func (s *Server) lookupBarcode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Raw string `json:"raw"`
	}
	if !decodeHardwareJSON(w, r, &request) {
		return
	}
	item, err := s.hardware.LookupBarcode(r.Context(), request.Raw)
	if err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) kickCashDrawer(w http.ResponseWriter, r *http.Request) {
	if err := s.hardware.KickCashDrawer(r.Context()); err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"kicked": true})
}

// verifyBiometric accepts a base64-encoded sample (e.g. a fingerprint
// template) and asks the configured BiometricAdapter to verify it. Without
// an injected adapter this always reports hardware_adapter_unavailable; no
// biometric matching happens in this service.
func (s *Server) verifyBiometric(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Sample string `json:"sample"`
	}
	if !decodeHardwareJSON(w, r, &request) {
		return
	}
	sample, err := base64.StdEncoding.DecodeString(request.Sample)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_hardware_request", "Invalid hardware request", "The biometric sample must be base64-encoded bytes.")
		return
	}
	verified, err := s.hardware.VerifyBiometric(r.Context(), sample)
	if err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"verified": verified})
}

func (s *Server) sendEmail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if !decodeHardwareJSON(w, r, &request) {
		return
	}
	if err := s.hardware.SendEmail(r.Context(), request.To, request.Subject, request.Body); err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"sent": true})
}

func (s *Server) sendSMS(w http.ResponseWriter, r *http.Request) {
	var request struct {
		To      string `json:"to"`
		Message string `json:"message"`
	}
	if !decodeHardwareJSON(w, r, &request) {
		return
	}
	if err := s.hardware.SendSMS(r.Context(), request.To, request.Message); err != nil {
		writeHardwareProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"sent": true})
}

func decodeHardwareJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(value); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_hardware_request", "Invalid hardware request", "The hardware request could not be parsed.")
		return false
	}
	return true
}

func writeHardwareProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, hardware.ErrAdapterUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "hardware_adapter_unavailable", "Hardware adapter unavailable", "No physical hardware adapter is configured for this branch.")
	case errors.Is(err, hardware.ErrInvalidConfiguration):
		writeProblem(w, http.StatusServiceUnavailable, "hardware_configuration_invalid", "Hardware configuration invalid", "The branch hardware configuration is invalid; no hardware operation was attempted.")
	case errors.Is(err, hardware.ErrInvalidBarcode), errors.Is(err, hardware.ErrInvalidPrintJob),
		errors.Is(err, hardware.ErrInvalidBiometricInput), errors.Is(err, hardware.ErrInvalidEmailAddress), errors.Is(err, hardware.ErrInvalidSMSRecipient):
		writeProblem(w, http.StatusBadRequest, "invalid_hardware_request", "Invalid hardware request", err.Error())
	default:
		writeProblem(w, http.StatusBadGateway, "hardware_operation_failed", "Hardware operation failed", "The configured hardware adapter did not complete the operation.")
	}
}

func (s *Server) push(w http.ResponseWriter, r *http.Request) {
	var request pushRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", err.Error())
		return
	}
	if len(request.Events) == 0 || len(request.Events) > 500 {
		writeProblem(w, http.StatusBadRequest, "invalid_event_batch", "Invalid event batch", "Provide between 1 and 500 events.")
		return
	}
	accepted := 0
	duplicates := 0
	for _, event := range request.Events {
		inserted, err := s.store.InsertEvent(r.Context(), event)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "event_rejected", "Event rejected", err.Error())
			return
		}
		if inserted {
			accepted++
		} else {
			duplicates++
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"accepted": accepted, "duplicates": duplicates})
}

func (s *Server) ack(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Cursor int64 `json:"cursor"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil || request.Cursor < 0 {
		writeProblem(w, http.StatusBadRequest, "invalid_cursor", "Invalid synchronization cursor", "Provide a non-negative cursor value.")
		return
	}
	if err := s.store.SetCursor(r.Context(), "central_pushed_sequence", strconv.FormatInt(request.Cursor, 10)); err != nil {
		writeProblem(w, http.StatusInternalServerError, "sync_ack_failed", "Unable to acknowledge synchronization", "The local synchronization cursor could not be updated.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"cursor": request.Cursor})
}

func (s *Server) transaction(aggregate string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event store.Event
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&event); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The transaction event could not be parsed.")
			return
		}
		if event.Aggregate == "" {
			event.Aggregate = aggregate
		}
		if event.Aggregate != aggregate {
			writeProblem(w, http.StatusBadRequest, "invalid_aggregate", "Invalid transaction type", "The transaction endpoint received a different aggregate type.")
			return
		}
		if event.SchemaVersion == 0 {
			event.SchemaVersion = 1
		}
		inserted, err := s.store.InsertEvent(r.Context(), event)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "transaction_rejected", "Transaction rejected", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": inserted, "duplicate": !inserted, "eventId": event.EventID})
	})
}

func (s *Server) shiftClose(w http.ResponseWriter, r *http.Request) {
	var event store.Event
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&event); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The shift-close event could not be parsed.")
		return
	}
	if event.Aggregate == "" {
		event.Aggregate = "shift.close"
	}
	if event.Aggregate != "shift.close" {
		writeProblem(w, http.StatusBadRequest, "invalid_aggregate", "Invalid transaction type", "This endpoint accepts shift-close events only.")
		return
	}
	if event.AggregateID == "" {
		event.AggregateID = r.PathValue("id")
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = 1
	}
	inserted, err := s.store.InsertEvent(r.Context(), event)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "shift_rejected", "Shift event rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": inserted, "duplicate": !inserted, "eventId": event.EventID})
}

func (s *Server) pull(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, nextCursor, err := s.store.EventsAfter(r.Context(), cursor, limit)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "sync_read_failed", "Unable to read local sync queue", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "nextCursor": nextCursor})
}

func (s *Server) notFound(w http.ResponseWriter, _ *http.Request) {
	writeProblem(w, http.StatusNotFound, "not_found", "Not found", "The requested edge endpoint does not exist.")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "https://abuzar.invalid/problems/" + code,
		"title":  title,
		"status": status,
		"detail": detail,
		"code":   code,
	})
}

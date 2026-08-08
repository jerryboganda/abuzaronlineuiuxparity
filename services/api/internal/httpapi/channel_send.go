package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// channelKind identifies which real external channel a maintenance action
// is attempting to exercise.
type channelKind string

const (
	channelKindEmail channelKind = "email"
	channelKindSMS   channelKind = "sms"
)

func isEmailMaintenanceKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "test-email", "send-email":
		return true
	default:
		return false
	}
}

func isSMSMaintenanceKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "test-sms", "send-sms":
		return true
	default:
		return false
	}
}

// edgeChannelConfig carries the parameters used to reach a branch-edge
// instance's real SMS/email adapters from the central API. This is a
// deliberately explicit, env-only configuration surface: the central API
// has no per-branch service-discovery mechanism today, so it never guesses
// at a branch's edge URL. When these are unset, channel maintenance actions
// are honestly reported as not_configured, exactly like the pg_dump /
// pg_restore "tool not on PATH" fallback in handleDatabaseBackup /
// handleDatabaseRestore above.
type edgeChannelConfig struct {
	BaseURL string
	Secret  string
}

func edgeChannelConfigFromEnv() edgeChannelConfig {
	return edgeChannelConfig{
		BaseURL: strings.TrimSuffix(strings.TrimSpace(os.Getenv("ABUZAR_EDGE_CHANNEL_URL")), "/"),
		Secret:  strings.TrimSpace(os.Getenv("ABUZAR_EDGE_CHANNEL_SECRET")),
	}
}

func (c edgeChannelConfig) configured() bool {
	return c.BaseURL != ""
}

// edgeChannelHTTPClient issues the outbound call to the branch-edge service.
// Tests substitute a client pinned to a local httptest.Server transport.
var edgeChannelHTTPClient = &http.Client{Timeout: 15 * time.Second}

type edgeProblemBody struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// edgeChannelOutcome classifies the direct result of a real branch-edge
// call so the caller can decide whether to report "completed" or an honest
// "not_configured" (the edge service is reachable but has no adapter of its
// own configured) without ever inferring success on ambiguous data.
type edgeChannelOutcome int

const (
	edgeChannelSent edgeChannelOutcome = iota
	edgeChannelUnavailable
)

// sendViaEdge posts the given JSON body to the branch-edge channel path.
// A network failure or a non-hardware_adapter_unavailable error response is
// returned as err; a 2xx result reports edgeChannelSent; an edge-reported
// hardware_adapter_unavailable problem reports edgeChannelUnavailable with a
// nil error so the caller can render an honest not_configured outcome.
func sendViaEdge(ctx context.Context, cfg edgeChannelConfig, path string, body any) (edgeChannelOutcome, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return edgeChannelUnavailable, fmt.Errorf("encode branch-edge channel request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return edgeChannelUnavailable, fmt.Errorf("build branch-edge channel request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if cfg.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.Secret)
	}
	response, err := edgeChannelHTTPClient.Do(request)
	if err != nil {
		return edgeChannelUnavailable, fmt.Errorf("branch-edge channel service could not be reached: %w", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return edgeChannelSent, nil
	}
	var problem edgeProblemBody
	_ = json.Unmarshal(raw, &problem)
	if problem.Code == "hardware_adapter_unavailable" {
		return edgeChannelUnavailable, nil
	}
	detail := strings.TrimSpace(problem.Detail)
	if detail == "" {
		detail = fmt.Sprintf("status %d", response.StatusCode)
	}
	return edgeChannelUnavailable, fmt.Errorf("branch-edge channel service rejected the request: %s", detail)
}

// handleChannelSend backs the "test-email"/"send-email"/"test-sms"/
// "send-sms" maintenance kinds. When ABUZAR_EDGE_CHANNEL_URL is configured
// it attempts a real send through that branch-edge instance's real SMTP/SMS
// adapter; otherwise (or when the edge itself reports no adapter
// configured) it honestly reports not_configured, exactly like every other
// "no adapter available" maintenance outcome in this file. It never reports
// "completed" without a genuine 2xx acknowledgement from the edge service.
func (s *Server) handleChannelSend(w http.ResponseWriter, r *http.Request, operator *sessionContext, kind string, payload map[string]any, channel channelKind) {
	cfg := edgeChannelConfigFromEnv()

	status := "not_configured"
	message := notConfiguredChannelMessage(channel)
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["channel"] = string(channel)
	auditPayload["edgeConfigured"] = cfg.configured()

	if cfg.configured() {
		to, _ := payload["to"].(string)
		to = strings.TrimSpace(to)
		if to == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_channel_recipient", "Invalid recipient", "A \"to\" recipient is required to send a "+string(channel)+" message.")
			return
		}

		var (
			edgePath string
			body     any
		)
		switch channel {
		case channelKindEmail:
			subject, _ := payload["subject"].(string)
			bodyText, _ := payload["body"].(string)
			edgePath = "/v1/hardware/email/send"
			body = map[string]string{"to": to, "subject": subject, "body": bodyText}
		case channelKindSMS:
			messageText, _ := payload["message"].(string)
			edgePath = "/v1/hardware/sms/send"
			body = map[string]string{"to": to, "message": messageText}
		}

		outcome, sendErr := sendViaEdge(r.Context(), cfg, edgePath, body)
		switch {
		case sendErr != nil:
			writeProblem(w, http.StatusBadGateway, "channel_send_failed", "Channel send failed", sendErr.Error())
			return
		case outcome == edgeChannelSent:
			status = "completed"
			message = fmt.Sprintf("The branch-edge %s adapter accepted the message for delivery to %s.", channel, to)
		default:
			status = "not_configured"
			message = fmt.Sprintf("The branch-edge service at %s has no %s adapter configured; no message was sent.", cfg.BaseURL, channel)
		}
	}

	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The channel send operation could not be opened.")
		return
	}
	defer tx.Rollback()

	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, auditPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The channel send operation could not be recorded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The channel send operation could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":        kind,
		"operationId": operationID,
		"status":      status,
		"message":     message,
	})
}

func notConfiguredChannelMessage(channel channelKind) string {
	return fmt.Sprintf("No branch-edge channel URL is configured (ABUZAR_EDGE_CHANNEL_URL); no %s was sent.", channel)
}

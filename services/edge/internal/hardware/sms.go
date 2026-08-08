package hardware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrSMSConfigInvalid is returned by NewSMSGatewayAdapter when the supplied
// SMSGatewayConfig cannot possibly reach a gateway (missing URL template or
// an unrecognized method).
var ErrSMSConfigInvalid = errors.New("invalid SMS gateway adapter configuration")

// SMSGatewayConfig configures a generic, templated Web-SMS-gateway HTTP
// client. It matches the legacy "SMS Method / Web SMS Provider / User ID /
// Password / Mask / API Key" preference tab: Pakistani telco SMS gateways
// (Zong and similar) commonly expose a single GET/POST endpoint with
// query-string placeholders for the account credentials, sender mask, and
// message, and their success/failure contract varies by provider. Nothing
// about a specific vendor is hardcoded here — the URL template and the
// success rule are both configuration.
type SMSGatewayConfig struct {
	// URLTemplate may reference {user}, {password}, {mask}, {apikey}, {to},
	// and {message} placeholders. Each is substituted with its
	// URL-query-escaped value before the request is issued.
	URLTemplate string
	// Method is "GET" or "POST"; empty defaults to GET.
	Method   string
	User     string
	Password string
	Mask     string
	APIKey   string
	// SuccessStatusCode, when non-zero, is the only HTTP status code
	// treated as success. Zero means "any 2xx status is success".
	SuccessStatusCode int
	// SuccessBodyContains, when non-empty, must appear in the response body
	// for the send to be treated as successful, in addition to the status
	// code check above. Gateways vary widely here (some return "OK",
	// others a numeric message id, others a provider-specific code), so
	// this is left as free-form configuration rather than assumed.
	SuccessBodyContains string
	// Timeout bounds the HTTP request. Defaults to 10s.
	Timeout time.Duration

	// httpClient lets tests substitute a client pinned to a local
	// httptest.Server transport; production callers leave this nil.
	httpClient *http.Client
}

func (c SMSGatewayConfig) validate() error {
	if strings.TrimSpace(c.URLTemplate) == "" {
		return fmt.Errorf("%w: URL template is required", ErrSMSConfigInvalid)
	}
	method := strings.ToUpper(strings.TrimSpace(c.Method))
	if method != "" && method != http.MethodGet && method != http.MethodPost {
		return fmt.Errorf("%w: method must be GET or POST (got %q)", ErrSMSConfigInvalid, c.Method)
	}
	return nil
}

// SMSGatewayAdapter is a real SMSAdapter implementation that issues an HTTP
// request built from a configurable URL template and interprets a
// configurable success rule.
type SMSGatewayAdapter struct {
	config SMSGatewayConfig
	client *http.Client
}

// NewSMSGatewayAdapter validates config and returns a ready-to-use adapter.
func NewSMSGatewayAdapter(config SMSGatewayConfig) (*SMSGatewayAdapter, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	client := config.httpClient
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	return &SMSGatewayAdapter{config: config, client: client}, nil
}

// Send implements SMSAdapter. It substitutes the configured template
// placeholders, issues the HTTP request, and applies the configured success
// rule to the response.
func (a *SMSGatewayAdapter) Send(ctx context.Context, to, message string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("sms send: recipient is required")
	}

	replacer := strings.NewReplacer(
		"{user}", url.QueryEscape(a.config.User),
		"{password}", url.QueryEscape(a.config.Password),
		"{mask}", url.QueryEscape(a.config.Mask),
		"{apikey}", url.QueryEscape(a.config.APIKey),
		"{to}", url.QueryEscape(to),
		"{message}", url.QueryEscape(message),
	)
	target := replacer.Replace(a.config.URLTemplate)

	method := strings.ToUpper(strings.TrimSpace(a.config.Method))
	if method == "" {
		method = http.MethodGet
	}

	request, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return fmt.Errorf("build sms gateway request: %w", err)
	}

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call sms gateway: %w", err)
	}
	defer response.Body.Close()
	rawBody, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))

	if a.config.SuccessStatusCode != 0 {
		if response.StatusCode != a.config.SuccessStatusCode {
			return fmt.Errorf("sms gateway returned status %d, want %d: %s", response.StatusCode, a.config.SuccessStatusCode, strings.TrimSpace(string(rawBody)))
		}
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sms gateway returned status %d: %s", response.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	if a.config.SuccessBodyContains != "" && !bytes.Contains(rawBody, []byte(a.config.SuccessBodyContains)) {
		return fmt.Errorf("sms gateway response did not contain expected success marker %q: %s", a.config.SuccessBodyContains, strings.TrimSpace(string(rawBody)))
	}
	return nil
}

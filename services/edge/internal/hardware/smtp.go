package hardware

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPEncryption selects how the adapter establishes transport security
// before talking SMTP, matching the three values captured from the legacy
// "SMTP Encryption Type" preference.
type SMTPEncryption string

const (
	SMTPEncryptionNone SMTPEncryption = "None"
	// SMTPEncryptionSSL dials straight into TLS (implicit TLS, e.g. port 465).
	SMTPEncryptionSSL SMTPEncryption = "SSL"
	// SMTPEncryptionTLS connects in the clear and upgrades with STARTTLS
	// (e.g. port 587).
	SMTPEncryptionTLS SMTPEncryption = "TLS"
)

// ErrSMTPConfigInvalid is returned by NewSMTPClientAdapter when the supplied
// SMTPConfig cannot possibly reach a server (missing host/port/from, or an
// unrecognized encryption mode). It is checked at construction time so a
// misconfigured adapter is never silently wired into the registry.
var ErrSMTPConfigInvalid = errors.New("invalid SMTP adapter configuration")

// SMTPConfig carries the connection parameters mapped from the legacy Email
// preference tab (SMTP Server / Port / User / Password / Encryption Type)
// plus a From address. It is resolved once, at construction time, by
// whatever the branch host chooses (the edge process reads it from
// environment variables); this package never reads tenant_preferences
// itself.
type SMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	Encryption SMTPEncryption
	// Timeout bounds dialing and the SMTP conversation. Defaults to 10s.
	Timeout time.Duration
}

func (c SMTPConfig) validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrSMTPConfigInvalid)
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrSMTPConfigInvalid)
	}
	if strings.TrimSpace(c.From) == "" {
		return fmt.Errorf("%w: from address is required", ErrSMTPConfigInvalid)
	}
	switch c.Encryption {
	case SMTPEncryptionNone, SMTPEncryptionSSL, SMTPEncryptionTLS:
	default:
		return fmt.Errorf("%w: encryption must be None, SSL, or TLS (got %q)", ErrSMTPConfigInvalid, c.Encryption)
	}
	return nil
}

// SMTPClientAdapter is a real EmailAdapter implementation built on Go's
// standard net/smtp. It supports plaintext, implicit TLS (SSL), and
// STARTTLS (TLS) connections and optional AUTH PLAIN authentication.
type SMTPClientAdapter struct {
	config SMTPConfig
}

// NewSMTPClientAdapter validates config and returns a ready-to-use adapter.
// It performs no network I/O; dialing happens per Send call.
func NewSMTPClientAdapter(config SMTPConfig) (*SMTPClientAdapter, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	return &SMTPClientAdapter{config: config}, nil
}

// Send implements EmailAdapter. It dials the configured SMTP server,
// establishes the configured transport security, authenticates if
// credentials are present, and sends a single plain-text message.
func (a *SMTPClientAdapter) Send(ctx context.Context, to, subject, body string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("smtp send: recipient address is required")
	}

	address := net.JoinHostPort(a.config.Host, strconv.Itoa(a.config.Port))
	dialer := &net.Dialer{Timeout: a.config.Timeout}

	var (
		conn net.Conn
		err  error
	)
	if a.config.Encryption == SMTPEncryptionSSL {
		tlsDialer := &tls.Dialer{
			NetDialer: dialer,
			Config:    &tls.Config{ServerName: a.config.Host, MinVersion: tls.VersionTLS12},
		}
		conn, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("dial smtp server %s: %w", address, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(a.config.Timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	client, err := smtp.NewClient(conn, a.config.Host)
	if err != nil {
		return fmt.Errorf("initialize smtp session: %w", err)
	}
	defer client.Close()

	if a.config.Encryption == SMTPEncryptionTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp server at %s does not advertise STARTTLS", address)
		}
		tlsConfig := &tls.Config{ServerName: a.config.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls handshake: %w", err)
		}
	}

	if strings.TrimSpace(a.config.Username) != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", a.config.Username, a.config.Password, a.config.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp authentication: %w", err)
			}
		}
	}

	if err := client.Mail(a.config.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := writer.Write(buildRFC822Message(a.config.From, to, subject, body)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write smtp message body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message body: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp QUIT: %w", err)
	}
	return nil
}

func buildRFC822Message(from, to, subject, body string) []byte {
	var builder strings.Builder
	builder.WriteString("From: " + sanitizeHeaderValue(from) + "\r\n")
	builder.WriteString("To: " + sanitizeHeaderValue(to) + "\r\n")
	builder.WriteString("Subject: " + sanitizeHeaderValue(subject) + "\r\n")
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(body)
	builder.WriteString("\r\n")
	return []byte(builder.String())
}

// sanitizeHeaderValue strips CR/LF so a caller-supplied subject/address can
// never inject additional SMTP headers into the message.
func sanitizeHeaderValue(value string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ")
	return replacer.Replace(value)
}

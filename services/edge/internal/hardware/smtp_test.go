package hardware

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer is a minimal, plaintext-only SMTP command handler used to
// prove SMTPClientAdapter dials, authenticates, and sends a message to a
// LOCAL test listener. It never talks to any real mail server.
type fakeSMTPServer struct {
	listener net.Listener
	received chan fakeSMTPMessage
}

type fakeSMTPMessage struct {
	authUser string
	from     string
	to       string
	dataBody string
}

func startFakeSMTPServer(t *testing.T, requireAuth bool) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake smtp server: %v", err)
	}
	server := &fakeSMTPServer{listener: listener, received: make(chan fakeSMTPMessage, 1)}
	go server.serveOne(t, requireAuth)
	return server
}

func (s *fakeSMTPServer) addr() string {
	return s.listener.Addr().String()
}

func (s *fakeSMTPServer) close() {
	_ = s.listener.Close()
}

// serveOne handles exactly one connection with just enough of the SMTP
// state machine (EHLO/AUTH PLAIN/MAIL/RCPT/DATA/QUIT) for the client under
// test to complete a real send.
func (s *fakeSMTPServer) serveOne(t *testing.T, requireAuth bool) {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	writer := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	reader := bufio.NewReader(conn)

	writer("220 fake.smtp.test ESMTP")
	var message fakeSMTPMessage
	inData := false
	var dataLines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				inData = false
				message.dataBody = strings.Join(dataLines, "\n")
				writer("250 OK: queued")
				continue
			}
			dataLines = append(dataLines, line)
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			writer("250-fake.smtp.test greets you")
			if requireAuth {
				writer("250-AUTH PLAIN")
			}
			writer("250 OK")
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			parts := strings.SplitN(line, " ", 3)
			if len(parts) == 3 {
				message.authUser = parts[2] // base64 blob, presence is enough for the test
			}
			writer("235 Authentication succeeded")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			message.from = line
			writer("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			message.to = line
			writer("250 OK")
		case upper == "DATA":
			inData = true
			writer("354 Start mail input")
		case upper == "QUIT":
			writer("221 Bye")
			s.received <- message
			return
		default:
			writer("500 unrecognized command")
		}
	}
}

func TestSMTPClientAdapterSendsToLocalFakeServer(t *testing.T) {
	server := startFakeSMTPServer(t, true)
	defer server.close()

	host, portText, err := net.SplitHostPort(server.addr())
	if err != nil {
		t.Fatalf("split fake smtp address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse fake smtp port: %v", err)
	}

	adapter, err := NewSMTPClientAdapter(SMTPConfig{
		Host:       host,
		Port:       port,
		Username:   "sender@example.test",
		Password:   "secret",
		From:       "sender@example.test",
		Encryption: SMTPEncryptionNone,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("build smtp adapter: %v", err)
	}

	err = adapter.Send(context.Background(), "recipient@example.test", "Test subject", "Test body")
	if err != nil {
		t.Fatalf("send via local fake smtp server: %v", err)
	}

	select {
	case message := <-server.received:
		if message.authUser == "" {
			t.Fatalf("expected AUTH PLAIN to be exercised, got %+v", message)
		}
		if !strings.Contains(strings.ToLower(message.from), "sender@example.test") {
			t.Fatalf("MAIL FROM = %q, want sender@example.test", message.from)
		}
		if !strings.Contains(strings.ToLower(message.to), "recipient@example.test") {
			t.Fatalf("RCPT TO = %q, want recipient@example.test", message.to)
		}
		if !strings.Contains(message.dataBody, "Test subject") || !strings.Contains(message.dataBody, "Test body") {
			t.Fatalf("DATA body = %q, want subject and body present", message.dataBody)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fake smtp server never received a completed message")
	}
}

func TestSMTPConfigValidateRejectsIncompleteConfiguration(t *testing.T) {
	for _, config := range []SMTPConfig{
		{Port: 25, From: "a@b.test", Encryption: SMTPEncryptionNone},
		{Host: "smtp.test", From: "a@b.test", Encryption: SMTPEncryptionNone},
		{Host: "smtp.test", Port: 25, Encryption: SMTPEncryptionNone},
		{Host: "smtp.test", Port: 25, From: "a@b.test", Encryption: "Other"},
		{Host: "smtp.test", Port: 70000, From: "a@b.test", Encryption: SMTPEncryptionNone},
	} {
		if _, err := NewSMTPClientAdapter(config); !errors.Is(err, ErrSMTPConfigInvalid) {
			t.Fatalf("NewSMTPClientAdapter(%+v) = %v, want ErrSMTPConfigInvalid", config, err)
		}
	}
}

func TestSMTPClientAdapterWrapsDialFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve closed port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // now guaranteed nothing is listening on this port

	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	adapter, err := NewSMTPClientAdapter(SMTPConfig{
		Host: host, Port: port, From: "a@b.test", Encryption: SMTPEncryptionNone, Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("build adapter: %v", err)
	}
	if err := adapter.Send(context.Background(), "to@example.test", "s", "b"); err == nil {
		t.Fatal("expected send to a closed local port to fail")
	}
}

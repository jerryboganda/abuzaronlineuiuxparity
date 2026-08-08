package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abuzar/abuzar-next/services/edge/internal/hardware"
	"github.com/abuzar/abuzar-next/services/edge/internal/store"
	"github.com/abuzar/abuzar-next/services/edge/internal/syncapi"
	"github.com/abuzar/abuzar-next/services/edge/internal/syncer"
)

const version = "0.1.0"

func main() {
	addr := getenv("ABUZAR_EDGE_ADDR", ":8091")
	dbPath := getenv("ABUZAR_EDGE_DB", "./data/branch-edge.sqlite")
	sharedSecret := os.Getenv("ABUZAR_EDGE_SHARED_SECRET")
	centralURL := os.Getenv("ABUZAR_EDGE_CENTRAL_URL")
	centralSession := os.Getenv("ABUZAR_EDGE_CENTRAL_SESSION")
	syncInterval := parseDuration(getenv("ABUZAR_EDGE_SYNC_INTERVAL", "30s"), 30*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	localStore, err := store.Open(ctx, dbPath)
	if err != nil {
		log.Fatalf("open edge store: %v", err)
	}
	defer localStore.Close()

	registry := hardware.NewWithConfig(buildHardwareConfig())
	handler := syncapi.NewWithHardware(localStore, version, sharedSecret, registry)
	if centralURL != "" || centralSession != "" {
		client, err := syncer.New(localStore, centralURL, centralSession)
		if err != nil {
			log.Fatalf("configure central synchronizer: %v", err)
		}
		go client.Run(ctx, syncInterval, func(result syncer.Result, err error) {
			if err != nil {
				log.Printf("central synchronization failed: %v", err)
				return
			}
			if result.Pushed > 0 || result.Pulled > 0 || result.Conflicts > 0 {
				log.Printf("central synchronization pushed=%d pulled=%d duplicates=%d conflicts=%d", result.Pushed, result.Pulled, result.Duplicates, result.Conflicts)
			}
		})
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("Abuzar Next branch edge %s listening on %s using %s", version, addr, dbPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// buildHardwareConfig resolves every hardware adapter this process can
// construct on its own from environment variables. Printer, barcode,
// cash-drawer, and biometric adapters have no concrete implementation in
// this codebase yet (they require a vendor SDK or a physical driver the
// branch host must supply), so they are deliberately left nil here; the
// registry reports them as unconfigured rather than pretending otherwise.
func buildHardwareConfig() hardware.Config {
	emailAdapter, emailProvider := buildSMTPAdapter()
	smsAdapter, smsProvider := buildSMSAdapter()
	return hardware.Config{
		Email:         emailAdapter,
		EmailProvider: emailProvider,
		SMS:           smsAdapter,
		SMSProvider:   smsProvider,
	}
}

// buildSMTPAdapter constructs a real SMTP client adapter from SMTP_* process
// environment variables. It is deliberately env-var-only for this pass: it
// does not read per-tenant SMTP preferences (see preference_registry.go's
// Email category behavior notes for the honest statement of what is and is
// not wired). Absent SMTP_HOST, email stays unconfigured.
func buildSMTPAdapter() (hardware.EmailAdapter, string) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return nil, ""
	}
	port, err := strconv.Atoi(getenv("SMTP_PORT", "587"))
	if err != nil {
		log.Printf("invalid SMTP_PORT %q; email adapter disabled: %v", os.Getenv("SMTP_PORT"), err)
		return nil, ""
	}
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		from = strings.TrimSpace(os.Getenv("SMTP_USER"))
	}
	adapter, err := hardware.NewSMTPClientAdapter(hardware.SMTPConfig{
		Host:       host,
		Port:       port,
		Username:   os.Getenv("SMTP_USER"),
		Password:   os.Getenv("SMTP_PASSWORD"),
		From:       from,
		Encryption: hardware.SMTPEncryption(getenv("SMTP_ENCRYPTION", "TLS")),
	})
	if err != nil {
		log.Printf("invalid SMTP configuration; email adapter disabled: %v", err)
		return nil, ""
	}
	return adapter, "smtp"
}

// buildSMSAdapter constructs a real Web-SMS-gateway HTTP client adapter from
// SMS_GATEWAY_* process environment variables. As with SMTP above, this is
// env-var-only for this pass; no per-tenant SMS preference is read yet.
// Absent SMS_GATEWAY_URL_TEMPLATE, SMS stays unconfigured.
func buildSMSAdapter() (hardware.SMSAdapter, string) {
	template := strings.TrimSpace(os.Getenv("SMS_GATEWAY_URL_TEMPLATE"))
	if template == "" {
		return nil, ""
	}
	successStatus := 0
	if raw := strings.TrimSpace(os.Getenv("SMS_GATEWAY_SUCCESS_STATUS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			log.Printf("invalid SMS_GATEWAY_SUCCESS_STATUS %q; sms adapter disabled: %v", raw, err)
			return nil, ""
		}
		successStatus = parsed
	}
	adapter, err := hardware.NewSMSGatewayAdapter(hardware.SMSGatewayConfig{
		URLTemplate:         template,
		Method:              getenv("SMS_GATEWAY_METHOD", "GET"),
		User:                os.Getenv("SMS_GATEWAY_USER"),
		Password:            os.Getenv("SMS_GATEWAY_PASSWORD"),
		Mask:                os.Getenv("SMS_GATEWAY_MASK"),
		APIKey:              os.Getenv("SMS_GATEWAY_API_KEY"),
		SuccessStatusCode:   successStatus,
		SuccessBodyContains: os.Getenv("SMS_GATEWAY_SUCCESS_CONTAINS"),
	})
	if err != nil {
		log.Printf("invalid SMS gateway configuration; sms adapter disabled: %v", err)
		return nil, ""
	}
	return adapter, "web-sms-gateway"
}

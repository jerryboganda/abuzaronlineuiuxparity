package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	handler := syncapi.New(localStore, version, sharedSecret)
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

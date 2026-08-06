package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abuzar/abuzar-next/services/api/internal/db"
	"github.com/abuzar/abuzar-next/services/api/internal/httpapi"
)

const version = "0.1.0"

func main() {
	addr := getenv("ABUZAR_API_ADDR", ":8080")
	dsn := os.Getenv("DATABASE_URL")
	corsOrigins := getenv("ABUZAR_CORS_ORIGINS", "http://localhost:5173")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if database != nil {
		defer database.Close()
	}

	handler := httpapi.New(database, version, corsOrigins)
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

	log.Printf("Abuzar Next API %s listening on %s", version, addr)
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

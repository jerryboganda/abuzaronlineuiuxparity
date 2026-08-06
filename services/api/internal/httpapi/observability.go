package httpapi

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// requestMetrics is deliberately process-local. It is a low-cardinality
// operational signal, not an audit trail and never contains request payloads,
// cookies, query strings, or tenant identifiers.
type requestMetrics struct {
	started  time.Time
	requests atomic.Int64
	errors   atomic.Int64
	slow     atomic.Int64
}

func newRequestMetrics() *requestMetrics {
	return &requestMetrics{started: time.Now().UTC()}
}

func (m *requestMetrics) slowThreshold() time.Duration {
	return environmentDuration("ABUZAR_SLOW_REQUEST_MS", time.Second)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (s *Server) withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = strconv.FormatInt(time.Now().UnixNano(), 36)
		}
		w.Header().Set("X-Request-ID", requestID)
		start := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := time.Since(start)
		s.metrics.requests.Add(1)
		if status >= http.StatusInternalServerError {
			s.metrics.errors.Add(1)
		}
		if elapsed >= s.metrics.slowThreshold() {
			s.metrics.slow.Add(1)
			log.Printf("slow request method=%s path=%s status=%d duration_ms=%d request_id=%s",
				r.Method, r.URL.Path, status, elapsed.Milliseconds(), requestID)
		}
	})
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Metrics contain only process counters. They are safe for a local probe;
	// deployments that expose the endpoint externally should protect it at the
	// reverse proxy or set ABUZAR_METRICS_ENABLED=0.
	if value := os.Getenv("ABUZAR_METRICS_ENABLED"); value != "" && value != "1" && value != "true" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"startedAt":       s.metrics.started.Format(time.RFC3339Nano),
		"requestsTotal":   s.metrics.requests.Load(),
		"serverErrors":    s.metrics.errors.Load(),
		"slowRequests":    s.metrics.slow.Load(),
		"slowThresholdMs": s.metrics.slowThreshold().Milliseconds(),
		"uptimeSeconds":   time.Since(s.metrics.started).Seconds(),
	})
}

func environmentDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds < 1 || milliseconds > 10*60*1000 {
		return fallback
	}
	return time.Duration(milliseconds) * time.Millisecond
}

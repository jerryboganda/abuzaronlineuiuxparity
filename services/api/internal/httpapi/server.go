package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

type Server struct {
	database      *sql.DB
	version       string
	origins       map[string]struct{}
	cookieSecure  bool
	sessionTTL    time.Duration
	dbTimeout     time.Duration
	lockTimeout   time.Duration
	reportTimeout time.Duration
	metrics       *requestMetrics
}

func New(database *sql.DB, version, corsOrigins string) http.Handler {
	server := &Server{
		database:      database,
		version:       version,
		origins:       parseOrigins(corsOrigins),
		cookieSecure:  os.Getenv("ABUZAR_COOKIE_SECURE") == "1" || strings.EqualFold(os.Getenv("ABUZAR_COOKIE_SECURE"), "true"),
		sessionTTL:    8 * time.Hour,
		dbTimeout:     environmentDuration("ABUZAR_DB_STATEMENT_TIMEOUT_MS", 5000*time.Millisecond),
		lockTimeout:   environmentDuration("ABUZAR_DB_LOCK_TIMEOUT_MS", 1000*time.Millisecond),
		reportTimeout: environmentDuration("ABUZAR_REPORT_TIMEOUT_MS", 5000*time.Millisecond),
		metrics:       newRequestMetrics(),
	}
	return server.withObservability(server.withCORS(server.routes()))
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/metrics", s.metricsHandler)
	mux.HandleFunc("GET /v1/session", s.session)
	mux.HandleFunc("POST /v1/auth/login", s.login)
	mux.HandleFunc("POST /v1/auth/logout", s.logout)
	mux.Handle("POST /v1/auth/change-password", s.authenticated(http.HandlerFunc(s.changePassword)))
	mux.Handle("POST /v1/session/context", s.authenticated(http.HandlerFunc(s.setSessionContext)))
	mux.Handle("GET /v1/tenants", s.authenticated(http.HandlerFunc(s.tenants)))
	mux.Handle("GET /v1/branches", s.authenticated(http.HandlerFunc(s.branches)))
	mux.Handle("GET /v1/counters", s.authenticated(http.HandlerFunc(s.counters)))
	mux.Handle("GET /v1/operators", s.authenticated(http.HandlerFunc(s.operators)))
	mux.Handle("POST /v1/operators", s.authenticated(http.HandlerFunc(s.createOperator)))
	mux.Handle("PATCH /v1/operators/{id}", s.authenticated(http.HandlerFunc(s.updateOperator)))
	mux.Handle("GET /v1/roles", s.authenticated(http.HandlerFunc(s.roles)))
	mux.Handle("POST /v1/roles", s.authenticated(http.HandlerFunc(s.createRole)))
	mux.Handle("PATCH /v1/roles/{id}", s.authenticated(http.HandlerFunc(s.updateRole)))
	mux.Handle("GET /v1/roles/{id}/rights", s.authenticated(http.HandlerFunc(s.roleRights)))
	mux.Handle("PATCH /v1/roles/{id}/rights", s.authenticated(http.HandlerFunc(s.updateRoleRights)))
	mux.Handle("GET /v1/access", s.authenticated(http.HandlerFunc(s.access)))
	mux.Handle("GET /v1/items/lookup", s.authenticated(http.HandlerFunc(s.itemLookup)))
	mux.Handle("GET /v1/master/items/lookup", s.authenticated(http.HandlerFunc(s.itemLookup)))
	mux.Handle("GET /v1/master/{kind}", s.authenticated(http.HandlerFunc(s.masterRecords)))
	mux.Handle("POST /v1/master/{kind}", s.authenticated(http.HandlerFunc(s.createMasterRecord)))
	mux.Handle("GET /v1/master/{kind}/{id}", s.authenticated(http.HandlerFunc(s.masterRecordDetail)))
	mux.Handle("PATCH /v1/master/{kind}/{id}", s.authenticated(http.HandlerFunc(s.updateMasterRecord)))
	mux.Handle("DELETE /v1/master/{kind}/{id}", s.authenticated(http.HandlerFunc(s.deleteMasterRecord)))
	mux.Handle("GET /v1/master/{kind}/{id}/suppliers", s.authenticated(http.HandlerFunc(s.itemSuppliers)))
	mux.Handle("PUT /v1/master/{kind}/{id}/suppliers", s.authenticated(http.HandlerFunc(s.replaceItemSuppliers)))
	mux.Handle("GET /v1/tax-rates", s.authenticated(http.HandlerFunc(s.taxRates)))
	mux.Handle("POST /v1/tax-rates", s.authenticated(http.HandlerFunc(s.taxRates)))
	mux.Handle("PATCH /v1/tax-rates/{id}", s.authenticated(http.HandlerFunc(s.taxRates)))
	mux.Handle("DELETE /v1/tax-rates/{id}", s.authenticated(http.HandlerFunc(s.taxRates)))
	mux.Handle("GET /v1/tax-assignments", s.authenticated(http.HandlerFunc(s.taxAssignments)))
	mux.Handle("DELETE /v1/tax-assignments/{id}", s.authenticated(http.HandlerFunc(s.taxAssignments)))
	mux.Handle("PUT /v1/tax-assignments/items/{itemId}", s.authenticated(http.HandlerFunc(s.replaceItemTaxAssignments)))
	mux.Handle("PUT /v1/tax-assignments/parties/{partyId}", s.authenticated(http.HandlerFunc(s.replacePartyTaxAssignments)))
	mux.Handle("POST /v1/tax-assignments/apply-item-gst", s.authenticated(http.HandlerFunc(s.applyItemGST)))
	mux.Handle("GET /v1/reports/{kind}", s.authenticated(http.HandlerFunc(s.report)))
	mux.Handle("GET /v1/transactions/{kind}", s.authenticated(http.HandlerFunc(s.transactionHistory)))
	mux.Handle("POST /v1/maintenance/{kind}", s.authenticated(http.HandlerFunc(s.maintenanceAction)))
	mux.Handle("GET /v1/maintenance/{kind}", s.authenticated(http.HandlerFunc(s.maintenanceState)))
	mux.Handle("GET /v1/session-monitor", s.authenticated(http.HandlerFunc(s.sessionMonitor)))
	mux.Handle("GET /v1/preferences", s.authenticated(http.HandlerFunc(s.preferences)))
	mux.Handle("PUT /v1/preferences", s.authenticated(http.HandlerFunc(s.savePreferences)))
	mux.Handle("POST /v1/shifts/open", s.authenticated(http.HandlerFunc(s.openShift)))
	mux.Handle("GET /v1/shifts", s.authenticated(http.HandlerFunc(s.shifts)))
	mux.Handle("POST /v1/shifts/{id}/close", s.authenticated(http.HandlerFunc(s.closeShift)))
	mux.Handle("POST /v1/transactions/sales", s.authenticated(http.HandlerFunc(s.createSale)))
	mux.Handle("POST /v1/transactions/preview", s.authenticated(http.HandlerFunc(s.previewPricing)))
	mux.Handle("GET /v1/inventory/balance", s.authenticated(http.HandlerFunc(s.inventoryBalance)))
	mux.Handle("GET /v1/inventory/availability", s.authenticated(http.HandlerFunc(s.stockAvailability)))
	mux.Handle("POST /v1/inventory/rebuild", s.authenticated(http.HandlerFunc(s.rebuildStockBalances)))
	mux.Handle("GET /v1/finance/accounts", s.authenticated(http.HandlerFunc(s.financeAccounts)))
	mux.Handle("GET /v1/finance/journals", s.authenticated(http.HandlerFunc(s.financeJournals)))
	mux.Handle("GET /v1/finance/ledger", s.authenticated(http.HandlerFunc(s.financeLedger)))
	mux.Handle("POST /v1/transactions/sale-returns", s.authenticated(s.createTransaction("sale_return")))
	mux.Handle("POST /v1/transactions/quotations", s.authenticated(s.createTransaction("quotation")))
	mux.Handle("POST /v1/transactions/refused-sales", s.authenticated(s.createTransaction("refused_sale")))
	mux.Handle("POST /v1/transactions/returns", s.authenticated(s.createTransaction("return")))
	mux.Handle("POST /v1/transactions/receiving", s.authenticated(s.createTransaction("receiving")))
	mux.Handle("POST /v1/transactions/purchase-orders", s.authenticated(s.createTransaction("purchase_order")))
	mux.Handle("POST /v1/transactions/inventory", s.authenticated(s.createTransaction("inventory")))
	mux.Handle("POST /v1/documents/{kind}", s.authenticated(http.HandlerFunc(s.documentCommand)))
	mux.Handle("POST /v1/sync/push", s.authenticated(http.HandlerFunc(s.pushSync)))
	mux.Handle("GET /v1/sync/pull", s.authenticated(http.HandlerFunc(s.pullSync)))
	mux.Handle("GET /v1/conflicts", s.authenticated(http.HandlerFunc(s.conflicts)))
	mux.Handle("POST /v1/conflicts/{id}/resolve", s.authenticated(http.HandlerFunc(s.resolveConflict)))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "abuzar-next-api",
			"version": s.version,
		})
	})
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	databaseStatus := "not_configured"
	status := "degraded"
	if s.database != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		err := s.database.PingContext(ctx)
		cancel()
		if err != nil {
			databaseStatus = "error"
			status = "degraded"
		} else {
			databaseStatus = "ok"
			status = "ok"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"service":  "api",
		"database": databaseStatus,
		"version":  s.version,
		"time":     time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := s.origins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseOrigins(value string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
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

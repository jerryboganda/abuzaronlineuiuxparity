package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPreferencesRoundTripAndBranchIsolationIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	ctx := context.Background()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	var branchColumn bool
	if err := database.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'tenant_preferences' AND column_name = 'branch_id'
		)
	`).Scan(&branchColumn); err != nil {
		t.Fatalf("check preference migration: %v", err)
	}
	if !branchColumn {
		t.Skip("preference branch-scope migration is not installed")
	}

	suffix := time.Now().UnixNano()
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "preferences-"+formatTestSuffix(suffix))
	defer database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
	var otherBranchID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO branches (tenant_id, code, name)
		VALUES ($1::uuid, $2, 'Other Branch') RETURNING id::text
	`, tenantID, "other-"+formatTestSuffix(suffix)).Scan(&otherBranchID); err != nil {
		t.Fatalf("seed other branch: %v", err)
	}
	operator := &sessionContext{UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID, Roles: []string{"tenant_admin"}}
	otherOperator := *operator
	otherOperator.BranchID = otherBranchID
	server := &Server{database: database}

	save := func(operator *sessionContext, value string) {
		body := `{"category":"General","items":[{"caption":"Enable Alias Name:","value":"` + value + `","position":0}]}`
		recorder := httptest.NewRecorder()
		server.savePreferences(recorder, preferenceTestRequestWithBody(http.MethodPut, "/v1/preferences", body, operator))
		if recorder.Code != http.StatusOK {
			t.Fatalf("save preference status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
	}
	read := func(operator *sessionContext) string {
		recorder := httptest.NewRecorder()
		server.preferences(recorder, preferenceTestRequest(http.MethodGet, "/v1/preferences?category=General", operator))
		if recorder.Code != http.StatusOK {
			t.Fatalf("read preference status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Items []preferenceItem `json:"items"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode preferences: %v", err)
		}
		for _, item := range response.Items {
			if item.Caption == "Enable Alias Name:" {
				return item.Value
			}
		}
		return ""
	}

	save(operator, "Yes")
	if got := read(operator); got != "Yes" {
		t.Fatalf("same-branch round trip = %q, want Yes", got)
	}
	save(&otherOperator, "No")
	if got := read(operator); got != "Yes" {
		t.Fatalf("branch A was contaminated by branch B: %q", got)
	}
	if got := read(&otherOperator); got != "No" {
		t.Fatalf("branch B round trip = %q, want No", got)
	}
}

func preferenceTestRequestWithBody(method, target, body string, operator *sessionContext) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	return request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
}

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

func TestAuxiliaryMasterCRUDIntegration(t *testing.T) {
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
	var kindConstraint string
	if err := database.QueryRowContext(ctx, `
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'master_records'::regclass AND conname = 'master_records_kind_check'
	`).Scan(&kindConstraint); err != nil {
		t.Fatalf("read auxiliary master kind constraint: %v", err)
	}
	for _, kind := range []string{"price-policy", "sale-promotion", "tax-category"} {
		if !strings.Contains(kindConstraint, "'"+kind+"'") {
			t.Fatalf("auxiliary master kind %q is not enabled by the schema constraint: %s", kind, kindConstraint)
		}
	}

	suffix := formatTestSuffix(time.Now().UnixNano())
	tenantID, branchID, counterID, operatorID := seedDocumentTenant(t, ctx, database, "aux-master-"+suffix)
	defer database.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1::uuid`, tenantID)
	operator := &sessionContext{
		UserID: operatorID, TenantID: tenantID, BranchID: branchID, CounterID: counterID,
		TokenHash: "auxiliary-master-session", Roles: []string{"tenant_admin"},
	}
	server := &Server{database: database}

	createRequest := masterTestRequest(http.MethodPost, "/v1/master/price-policy", `{
		"code":"PP-1","name":"Retail Policy","payload":{"PricePolicyCode":"PP-1","ICode":"ITEM-1","QtyLimit":"1","Price":"12.50"},"active":true
	}`, operator)
	createRecorder := httptest.NewRecorder()
	server.createMasterRecord(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created masterRecordResponse
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode created master: %v", err)
	}
	if created.Kind != "price-policy" || created.Code != "PP-1" || created.Name != "Retail Policy" {
		t.Fatalf("created auxiliary master = %+v", created)
	}

	listRecorder := httptest.NewRecorder()
	server.masterRecords(listRecorder, masterTestRequest(http.MethodGet, "/v1/master/price-policy", "", operator))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listed struct {
		Records []masterRecordResponse `json:"records"`
	}
	if err := json.NewDecoder(listRecorder.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed masters: %v", err)
	}
	if len(listed.Records) != 1 || listed.Records[0].ID != created.ID {
		t.Fatalf("listed auxiliary masters = %+v", listed.Records)
	}

	updateRecorder := httptest.NewRecorder()
	server.updateMasterRecord(updateRecorder, masterTestRequest(http.MethodPatch, "/v1/master/price-policy/"+created.ID, `{
		"name":"Updated Retail Policy","payload":{"PricePolicyCode":"PP-1","ICode":"ITEM-1","QtyLimit":"2","Price":"13.00"},"active":false
	}`, operator))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated masterRecordResponse
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated master: %v", err)
	}
	if updated.Name != "Updated Retail Policy" || updated.Active || string(updated.Payload) == "{}" {
		t.Fatalf("updated auxiliary master = %+v", updated)
	}

	var kind, name, quantity string
	if err := database.QueryRowContext(ctx, `
		SELECT kind, name, payload->>'QtyLimit'
		FROM master_records WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, created.ID).Scan(&kind, &name, &quantity); err != nil {
		t.Fatalf("read auxiliary master: %v", err)
	}
	if kind != "price-policy" || name != "Updated Retail Policy" || quantity != "2" {
		t.Fatalf("stored auxiliary master = %s/%s/%s", kind, name, quantity)
	}

	deleteRecorder := httptest.NewRecorder()
	server.deleteMasterRecord(deleteRecorder, masterTestRequest(http.MethodDelete, "/v1/master/price-policy/"+created.ID, "", operator))
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	var remaining int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM master_records WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, created.ID).Scan(&remaining); err != nil {
		t.Fatalf("check deleted auxiliary master: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("deleted auxiliary master count = %d", remaining)
	}
}

func masterTestRequest(method, target, body string, operator *sessionContext) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	path := strings.TrimPrefix(request.URL.Path, "/v1/master/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		request.SetPathValue("kind", parts[0])
	}
	if len(parts) > 1 && parts[1] != "" {
		request.SetPathValue("id", parts[1])
	}
	return request.WithContext(context.WithValue(request.Context(), sessionContextKey, operator))
}

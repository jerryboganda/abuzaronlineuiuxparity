package httpapi

import (
	"strings"
	"testing"
)

func TestMaintenanceIntegrityContractIsApplicationScoped(t *testing.T) {
	if len(maintenanceIntegrityTables) < 5 {
		t.Fatalf("integrity table contract has %d tables", len(maintenanceIntegrityTables))
	}
	for _, table := range maintenanceIntegrityTables {
		if table == "" || strings.ContainsAny(table, " ;'\"") {
			t.Fatalf("unsafe integrity table name %q", table)
		}
	}
	status, message := maintenanceExternalOutcome("check-database-integrity")
	if status != "completed" || !strings.Contains(message, "Application-scope") {
		t.Fatalf("integrity outcome = %q/%q, want application-scoped completion", status, message)
	}
}

func TestMaintenanceBackupNeverClaimsPhysicalSuccess(t *testing.T) {
	for _, kind := range []string{"backup-database", "restore-database", "export-data", "import-previous-sales", "send-email"} {
		status, message := maintenanceExternalOutcome(kind)
		if status != "not_configured" {
			t.Errorf("%s status = %q, want not_configured", kind, status)
		}
		if strings.Contains(strings.ToLower(message), "succeeded") || strings.Contains(strings.ToLower(message), "produced") && kind != "export-data" {
			t.Errorf("%s message falsely claims an external operation: %q", kind, message)
		}
	}
}

func TestMaintenanceImportValidationRejectsServerPaths(t *testing.T) {
	if err := validateMaintenancePayload("import-previous-sales", map[string]any{"sourceFile": "sales.csv"}); err != nil {
		t.Fatalf("logical import filename rejected: %v", err)
	}
	for _, source := range []string{"", `C:\data\sales.csv`, "../sales.csv", "tenant/data.csv"} {
		if err := validateMaintenancePayload("import-previous-sales", map[string]any{"sourceFile": source}); err == nil {
			t.Errorf("unsafe or empty import source %q was accepted", source)
		}
	}
}

func TestMaintenancePermissionSeparatesManageFromMaintenance(t *testing.T) {
	if got := maintenancePermission("manage-email"); got != "manage.users" {
		t.Fatalf("manage permission = %q, want manage.users", got)
	}
	if got := maintenancePermission("backup-database"); got != "maintenance.write" {
		t.Fatalf("maintenance permission = %q, want maintenance.write", got)
	}
}

func TestMaintenanceAuditPayloadRedactsSecrets(t *testing.T) {
	payload := copyMaintenancePayload(map[string]any{
		"username": "ADMIN",
		"password": "must-not-audit",
		"apiToken": "must-not-audit",
	})
	if _, found := payload["password"]; found {
		t.Fatal("password was retained in maintenance audit payload")
	}
	if _, found := payload["apiToken"]; found {
		t.Fatal("token was retained in maintenance audit payload")
	}
	if payload["username"] != "ADMIN" {
		t.Fatalf("non-sensitive audit field was lost: %#v", payload)
	}
}

func TestCanonicalItemMaintenanceValidation(t *testing.T) {
	valid := map[string]any{
		"itemCode": "ITEM-1", "priceType": "Sale Price", "price": "12.50",
		"effectiveDate": "2026-08-07",
	}
	if err := validateCanonicalItemMaintenancePayload("change-items-price", valid); err != nil {
		t.Fatalf("valid price maintenance rejected: %v", err)
	}
	numeric := map[string]any{"itemCode": "ITEM-1", "priceType": "Sale Price", "price": 12.75}
	if err := validateCanonicalItemMaintenancePayload("change-items-price", numeric); err != nil {
		t.Fatalf("numeric browser price maintenance rejected: %v", err)
	}
	supplier := map[string]any{"itemCode": "ITEM-1", "supplier": "SUP-1", "purchasePrice": 12.5, "priority": 1}
	if err := validateCanonicalItemMaintenancePayload("update-item-suppliers", supplier); err != nil {
		t.Fatalf("numeric supplier maintenance rejected: %v", err)
	}
	invalidCases := []struct {
		kind    string
		payload map[string]any
	}{
		{"change-items-price", map[string]any{"itemCode": "ITEM-1", "priceType": "Sale Price", "price": "12.501"}},
		{"change-item-discount", map[string]any{"itemCode": "ITEM-1", "discountType": "Percent", "discount": "100.01"}},
		{"update-item-basic-data", map[string]any{"itemCode": "ITEM-1", "field": "Unsupported", "value": "x"}},
		{"change-item-reorder-qty", map[string]any{"itemCode": "ITEM-1", "reorderQty": "-1"}},
		{"update-item-suppliers", map[string]any{"itemCode": "ITEM-1", "supplier": "SUP-1", "purchasePrice": "12.501"}},
		{"update-item-suppliers", map[string]any{"itemCode": "ITEM-1", "supplier": "SUP-1", "priority": -1}},
	}
	for _, testCase := range invalidCases {
		if err := validateCanonicalItemMaintenancePayload(testCase.kind, testCase.payload); err == nil {
			t.Errorf("invalid %s payload was accepted: %#v", testCase.kind, testCase.payload)
		}
	}
}

func TestBatchLockMaintenanceValidation(t *testing.T) {
	valid := map[string]any{"itemCode": "ITEM-1", "batch": "B-1", "locked": "Yes"}
	if err := validateBatchLockMaintenancePayload("lock-item-batches", valid); err != nil {
		t.Fatalf("valid batch lock rejected: %v", err)
	}
	if err := validateBatchLockMaintenancePayload("lock-item-batches", map[string]any{"itemCode": "ITEM-1", "batch": "B-1", "locked": false}); err != nil {
		t.Fatalf("boolean batch unlock rejected: %v", err)
	}
	if err := validateBatchLockMaintenancePayload("lock-item-batches", map[string]any{"itemCode": "ITEM-1", "batch": "B-1", "locked": "maybe"}); err == nil {
		t.Fatal("invalid batch lock state was accepted")
	}
}

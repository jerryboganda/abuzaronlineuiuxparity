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

package rlsprobe

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestApplicationRoleRLSProbe is deliberately independent of the existing
// business fixtures. Those fixtures currently seed through an owner
// connection. The CI job seeds this small fixture as the owner, then opens a
// second connection as the production-style non-owner application role.
func TestApplicationRoleRLSProbe(t *testing.T) {
	dsn := os.Getenv("ABUZAR_APP_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("ABUZAR_RLS_PROBE_REQUIRED") == "1" {
			t.Fatal("ABUZAR_APP_DATABASE_URL is required for the RLS probe")
		}
		t.Skip("ABUZAR_APP_DATABASE_URL is not configured")
	}
	role := os.Getenv("ABUZAR_APP_ROLE")
	if role == "" {
		t.Fatal("ABUZAR_APP_ROLE is required for the RLS probe")
	}

	tenantA := requiredProbeEnv(t, "ABUZAR_RLS_TENANT_A_ID")
	tenantB := requiredProbeEnv(t, "ABUZAR_RLS_TENANT_B_ID")
	branchA := requiredProbeEnv(t, "ABUZAR_RLS_BRANCH_A_ID")
	branchB := requiredProbeEnv(t, "ABUZAR_RLS_BRANCH_B_ID")

	ctx := context.Background()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open application-role database: %v", err)
	}
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping application-role database: %v", err)
	}

	var currentRole string
	var superuser, bypassRLS, createDB, createRole, ownerOfProtectedTable, schemaCreate, syncEventsDelete bool
	if err := database.QueryRowContext(ctx, `
		SELECT current_user, r.rolsuper, r.rolbypassrls, r.rolcreatedb, r.rolcreaterole,
		       EXISTS (
		           SELECT 1
		           FROM pg_class c
		           JOIN pg_namespace n ON n.oid = c.relnamespace
		           WHERE n.nspname = 'public'
		             AND c.relkind IN ('r', 'p')
		             AND c.relowner = r.oid
		       ),
		       has_schema_privilege(current_user, 'public', 'CREATE'),
		       has_table_privilege(current_user, 'public.sync_events', 'DELETE')
		FROM pg_roles r
		WHERE r.rolname = current_user
	`).Scan(&currentRole, &superuser, &bypassRLS, &createDB, &createRole, &ownerOfProtectedTable, &schemaCreate, &syncEventsDelete); err != nil {
		t.Fatalf("inspect application role: %v", err)
	}
	if currentRole != role {
		t.Fatalf("connected role = %q, want %q", currentRole, role)
	}
	if superuser || bypassRLS || createDB || createRole || ownerOfProtectedTable || schemaCreate || syncEventsDelete {
		t.Fatalf("application role is not least privilege: superuser=%v bypassrls=%v createdb=%v createrole=%v owner=%v schema_create=%v sync_events_delete=%v",
			superuser, bypassRLS, createDB, createRole, ownerOfProtectedTable, schemaCreate, syncEventsDelete)
	}
	t.Logf("role=%s superuser=%v bypassrls=%v createdb=%v createrole=%v protected_table_owner=%v schema_create=%v sync_events_delete=%v",
		currentRole, superuser, bypassRLS, createDB, createRole, ownerOfProtectedTable, schemaCreate, syncEventsDelete)

	var unscopedItems int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM master_items`).Scan(&unscopedItems); err != nil {
		t.Fatalf("unscoped item query: %v", err)
	}
	if unscopedItems != 0 {
		t.Fatalf("unscoped application-role query returned %d items; want 0", unscopedItems)
	}
	t.Logf("unscoped master_items=%d (fail-closed)", unscopedItems)

	withoutContext, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin unscoped write probe: %v", err)
	}
	_, writeErr := withoutContext.ExecContext(ctx, `
		INSERT INTO master_items (tenant_id, legacy_id, code, name)
		VALUES ($1::uuid, 'rls-unscoped-write', 'rls-unscoped-write', 'must fail')
	`, tenantA)
	_ = withoutContext.Rollback()
	if writeErr == nil {
		t.Fatal("unscoped application-role write was accepted")
	}
	t.Log("unscoped write rejected by RLS")

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tenant probe: %v", err)
	}
	defer tx.Rollback()
	setProbeScope := func(tenantID, branchID string) {
		t.Helper()
		for _, setting := range []struct{ key, value string }{
			{"app.authenticating", "false"},
			{"app.allow_tenant_scope", "false"},
			{"app.tenant_id", tenantID},
			{"app.branch_id", branchID},
		} {
			if _, err := tx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, setting.key, setting.value); err != nil {
				t.Fatalf("set %s: %v", setting.key, err)
			}
		}
	}
	setProbeScope(tenantA, branchA)

	var visibleTenants, visibleItems, visibleDocuments, crossBranchDocuments, foreignTenantItems int
	var branchBBatches, branchBVouchers int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM tenants`).Scan(&visibleTenants); err != nil {
		t.Fatalf("tenant visibility query: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM master_items`).Scan(&visibleItems); err != nil {
		t.Fatalf("item visibility query: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM business_documents`).Scan(&visibleDocuments); err != nil {
		t.Fatalf("document visibility query: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE branch_id = $1::uuid`, branchB).Scan(&crossBranchDocuments); err != nil {
		t.Fatalf("cross-branch visibility query: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_batches WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`, tenantA, branchB).Scan(&branchBBatches); err != nil {
		t.Fatalf("cross-branch stock visibility query: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM voucher_entries WHERE tenant_id = $1::uuid AND branch_id = $2::uuid`, tenantA, branchB).Scan(&branchBVouchers); err != nil {
		t.Fatalf("cross-branch voucher visibility query: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM master_items WHERE tenant_id = $1::uuid`, tenantB).Scan(&foreignTenantItems); err != nil {
		t.Fatalf("foreign-tenant visibility query: %v", err)
	}
	if visibleTenants != 1 || visibleItems != 1 || visibleDocuments != 1 ||
		crossBranchDocuments != 0 || branchBBatches != 0 || branchBVouchers != 0 ||
		foreignTenantItems != 0 {
		t.Fatalf("unexpected scoped visibility: tenants=%d items=%d documents=%d cross_branch=%d stock_batches=%d vouchers=%d foreign_tenant=%d",
			visibleTenants, visibleItems, visibleDocuments, crossBranchDocuments,
			branchBBatches, branchBVouchers, foreignTenantItems)
	}
	t.Logf("tenant=%s branch=%s visible tenants=%d items=%d documents=%d cross_branch=%d stock_batches=%d vouchers=%d foreign_tenant=%d",
		tenantA, branchA, visibleTenants, visibleItems, visibleDocuments, crossBranchDocuments,
		branchBBatches, branchBVouchers, foreignTenantItems)

	expectRejected := func(name, statement string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, `SAVEPOINT rls_write_probe`); err != nil {
			t.Fatalf("begin %s write probe: %v", name, err)
		}
		_, err := tx.ExecContext(ctx, statement, args...)
		if err == nil {
			t.Fatalf("%s application-role write was accepted", name)
		}
		if _, rollbackErr := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT rls_write_probe`); rollbackErr != nil {
			t.Fatalf("reset %s write probe: %v", name, rollbackErr)
		}
		if _, releaseErr := tx.ExecContext(ctx, `RELEASE SAVEPOINT rls_write_probe`); releaseErr != nil {
			t.Fatalf("release %s write probe: %v", name, releaseErr)
		}
	}
	expectRejected("cross-tenant", `
		INSERT INTO master_items (tenant_id, legacy_id, code, name)
		VALUES ($1::uuid, 'rls-foreign-write', 'rls-foreign-write', 'must fail')
	`, tenantB)
	t.Log("cross-tenant write rejected by RLS")
	expectRejected("cross-branch", `
		INSERT INTO business_documents (
			tenant_id, branch_id, counter_id, operator_id, kind, document_number,
			status, occurred_at
		) VALUES ($1::uuid, $2::uuid, '10000000-0000-0000-0000-000000000302',
		          '10000000-0000-0000-0000-000000000502', 'cash-sale',
		          'RLS-A-CROSS-BRANCH', 'draft', '2026-08-06T00:00:00Z')
	`, tenantA, branchB)
	t.Log("cross-branch write rejected by RLS")
	expectRejected("finalized-sync-event-delete", `
		DELETE FROM sync_events
		WHERE event_id = '10000000-0000-0000-0000-00000000a001'::uuid
	`)
	t.Log("finalized sync event delete rejected")

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback probe transaction: %v", err)
	}

	adminTx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tenant-admin probe: %v", err)
	}
	defer adminTx.Rollback()
	for _, setting := range []struct{ key, value string }{
		{"app.authenticating", "false"},
		{"app.allow_tenant_scope", "true"},
		{"app.tenant_id", tenantA},
		{"app.branch_id", branchA},
	} {
		if _, err := adminTx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, setting.key, setting.value); err != nil {
			t.Fatalf("set tenant-admin %s: %v", setting.key, err)
		}
	}
	var adminDocuments int
	if err := adminTx.QueryRowContext(ctx, `SELECT count(*) FROM business_documents WHERE tenant_id = $1::uuid`, tenantA).Scan(&adminDocuments); err != nil {
		t.Fatalf("tenant-admin document query: %v", err)
	}
	if adminDocuments != 3 {
		t.Fatalf("tenant-admin document visibility = %d, want 3", adminDocuments)
	}
	if _, err := adminTx.ExecContext(ctx, `
		UPDATE business_documents
		SET remarks = remarks
		WHERE id = '10000000-0000-0000-0000-000000000902'
	`); err != nil {
		t.Fatalf("tenant-admin branch-wide write: %v", err)
	}
	t.Logf("tenant-admin scope retained: tenant=%s documents=%d branch_b_write=accepted", tenantA, adminDocuments)

	// Ensure the IDs supplied to the test are UUIDs before reporting success;
	// this catches a miswired fixture rather than silently testing no rows.
	for name, value := range map[string]string{
		"tenant_a": tenantA, "tenant_b": tenantB, "branch_a": branchA,
		"branch_b": branchB,
	} {
		if len(value) != 36 {
			t.Fatalf("%s is not a UUID-shaped fixture id", name)
		}
	}
}

func requiredProbeEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required for the RLS probe", key)
	}
	return value
}

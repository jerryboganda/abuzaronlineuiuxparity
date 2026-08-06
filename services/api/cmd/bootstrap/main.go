// Command bootstrap creates or updates the first tenant, branch, counter, and
// tenant-admin operator in a provisioned PostgreSQL database. All values come
// from the protected environment; the password is never printed or stored in
// a file by this command.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/abuzar/abuzar-next/services/api/internal/db"
)

type configuration struct {
	DatabaseURL      string
	TenantCode       string
	TenantName       string
	BranchCode       string
	BranchName       string
	CounterCode      string
	CounterName      string
	OperatorUsername string
	OperatorName     string
	OperatorPassword string
	Timezone         string
}

func main() {
	config, err := readConfiguration()
	if err != nil {
		fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := db.Open(ctx, config.DatabaseURL)
	if err != nil {
		fatal(fmt.Errorf("open database: %w", err))
	}
	defer database.Close()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		fatal(fmt.Errorf("begin bootstrap transaction: %w", err))
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.authenticating', 'true', true)`); err != nil {
		fatal(fmt.Errorf("set bootstrap scope: %w", err))
	}

	var tenantID, branchID, counterID, operatorID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO tenants (code, legal_name, active)
		VALUES ($1, $2, true)
		ON CONFLICT (code) DO UPDATE SET legal_name = EXCLUDED.legal_name, active = true, updated_at = now()
		RETURNING id::text
	`, config.TenantCode, config.TenantName).Scan(&tenantID); err != nil {
		fatal(fmt.Errorf("provision tenant: %w", err))
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO branches (tenant_id, code, name, timezone, active)
		VALUES ($1::uuid, $2, $3, $4, true)
		ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name, timezone = EXCLUDED.timezone, active = true, updated_at = now()
		RETURNING id::text
	`, tenantID, config.BranchCode, config.BranchName, config.Timezone).Scan(&branchID); err != nil {
		fatal(fmt.Errorf("provision branch: %w", err))
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO counters (tenant_id, branch_id, code, name, active)
		VALUES ($1::uuid, $2::uuid, $3, $4, true)
		ON CONFLICT (tenant_id, branch_id, code) DO UPDATE SET name = EXCLUDED.name, active = true
		RETURNING id::text
	`, tenantID, branchID, config.CounterCode, config.CounterName).Scan(&counterID); err != nil {
		fatal(fmt.Errorf("provision counter: %w", err))
	}
	var roleID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO roles (tenant_id, code, name)
		VALUES ($1::uuid, 'tenant_admin', 'Tenant administrator')
		ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id::text
	`, tenantID).Scan(&roleID); err != nil {
		fatal(fmt.Errorf("provision tenant-admin role: %w", err))
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (tenant_id, username, display_name, password_hash, active)
		VALUES ($1::uuid, $2, $3, crypt($4, gen_salt('bf')), true)
		ON CONFLICT (tenant_id, username) DO UPDATE SET display_name = EXCLUDED.display_name, password_hash = EXCLUDED.password_hash, active = true, updated_at = now()
		RETURNING id::text
	`, tenantID, config.OperatorUsername, config.OperatorName, config.OperatorPassword).Scan(&operatorID); err != nil {
		fatal(fmt.Errorf("provision operator: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_memberships (user_id, tenant_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, operatorID, tenantID, roleID); err != nil {
		fatal(fmt.Errorf("assign tenant-admin role: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_branch_assignments (user_id, tenant_id, branch_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT (user_id, branch_id) DO NOTHING
	`, operatorID, tenantID, branchID); err != nil {
		fatal(fmt.Errorf("assign branch: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_counter_assignments (user_id, tenant_id, counter_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT (user_id, counter_id) DO NOTHING
	`, operatorID, tenantID, counterID); err != nil {
		fatal(fmt.Errorf("assign counter: %w", err))
	}
	if err := tx.Commit(); err != nil {
		fatal(fmt.Errorf("commit bootstrap: %w", err))
	}
	fmt.Printf("Provisioned tenant=%s branch=%s counter=%s operator=%s\n", tenantID, branchID, counterID, operatorID)
}

func readConfiguration() (configuration, error) {
	config := configuration{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		TenantCode:       os.Getenv("ABUZAR_BOOTSTRAP_TENANT_CODE"),
		TenantName:       os.Getenv("ABUZAR_BOOTSTRAP_TENANT_NAME"),
		BranchCode:       os.Getenv("ABUZAR_BOOTSTRAP_BRANCH_CODE"),
		BranchName:       os.Getenv("ABUZAR_BOOTSTRAP_BRANCH_NAME"),
		CounterCode:      os.Getenv("ABUZAR_BOOTSTRAP_COUNTER_CODE"),
		CounterName:      os.Getenv("ABUZAR_BOOTSTRAP_COUNTER_NAME"),
		OperatorUsername: os.Getenv("ABUZAR_BOOTSTRAP_OPERATOR_USERNAME"),
		OperatorName:     os.Getenv("ABUZAR_BOOTSTRAP_OPERATOR_NAME"),
		OperatorPassword: os.Getenv("ABUZAR_BOOTSTRAP_OPERATOR_PASSWORD"),
		Timezone:         os.Getenv("ABUZAR_BOOTSTRAP_TIMEZONE"),
	}
	if config.Timezone == "" {
		config.Timezone = "Asia/Karachi"
	}
	for name, value := range map[string]string{
		"DATABASE_URL":                       config.DatabaseURL,
		"ABUZAR_BOOTSTRAP_TENANT_CODE":       config.TenantCode,
		"ABUZAR_BOOTSTRAP_TENANT_NAME":       config.TenantName,
		"ABUZAR_BOOTSTRAP_BRANCH_CODE":       config.BranchCode,
		"ABUZAR_BOOTSTRAP_BRANCH_NAME":       config.BranchName,
		"ABUZAR_BOOTSTRAP_COUNTER_CODE":      config.CounterCode,
		"ABUZAR_BOOTSTRAP_COUNTER_NAME":      config.CounterName,
		"ABUZAR_BOOTSTRAP_OPERATOR_USERNAME": config.OperatorUsername,
		"ABUZAR_BOOTSTRAP_OPERATOR_NAME":     config.OperatorName,
		"ABUZAR_BOOTSTRAP_OPERATOR_PASSWORD": config.OperatorPassword,
		"ABUZAR_BOOTSTRAP_TIMEZONE":          config.Timezone,
	} {
		if strings.TrimSpace(value) == "" {
			return configuration{}, fmt.Errorf("%s is required", name)
		}
	}
	if len(config.OperatorPassword) < 12 {
		return configuration{}, errors.New("ABUZAR_BOOTSTRAP_OPERATOR_PASSWORD must contain at least 12 characters")
	}
	return config, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/abuzar/abuzar-next/migration/internal/reconcile"
)

func main() {
	source := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL")
	target := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "read-only PostgreSQL connection URL")
	tables := flag.String("tables", "", "optional comma-separated source tables in schema.table form")
	configPath := flag.String("config", "", "optional declarative mapping configuration")
	metrics := flag.String("metrics", os.Getenv("ABUZAR_RECONCILE_METRICS"), "optional JSON metric-check configuration")
	tenant := flag.String("tenant", os.Getenv("ABUZAR_RECONCILE_TENANT_ID"), "optional target tenant UUID used to evaluate PostgreSQL RLS")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical FazalDinPP19DataBaseV2 source")
	branchOverride := flag.String("branch-id", "", "optional target branch UUID override for mapping scope")
	counterOverride := flag.String("counter-id", "", "optional target counter UUID override for mapping scope")
	fromTable := flag.Int("from-table", 0, "zero-based first mapping table to reconcile")
	toTable := flag.Int("to-table", -1, "exclusive mapping table limit; -1 reconciles through the end")
	out := flag.String("out", filepath.Join("parity", "catalog", "migration-reconciliation.json"), "report output path")
	failOnOpenBookkeeping := flag.Bool("fail-on-open-bookkeeping", false, "fail after writing the report when target migration exceptions or ambiguities remain open")
	timeout := flag.Duration("timeout", reconcile.ReconcileTimeoutDefault(), "overall deadline covering both DB pings, every configured metric, and the bookkeeping check (env ABUZAR_RECONCILE_TIMEOUT, e.g. \"10m\")")
	flag.Parse()
	if *source == "" || *target == "" {
		fatal("source and target are required; provide protected environment variables or flags")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	sourceDB, err := reconcile.OpenSourceDB(*source)
	if err != nil {
		fatal(err.Error())
	}
	defer sourceDB.Close()
	targetDB, err := reconcile.OpenTargetDB(*target)
	if err != nil {
		fatal(err.Error())
	}
	defer targetDB.Close()
	if err := sourceDB.PingContext(ctx); err != nil {
		fatal(fmt.Sprintf("source ping failed: %v", err))
	}
	if err := targetDB.PingContext(ctx); err != nil {
		fatal(fmt.Sprintf("target ping failed: %v", err))
	}
	targetTx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		fatal(fmt.Sprintf("begin target reconciliation transaction: %v", err))
	}
	defer targetTx.Rollback()
	if strings.TrimSpace(*tenant) != "" {
		if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strings.TrimSpace(*tenant)); err != nil {
			fatal(fmt.Sprintf("set target tenant scope: %v", err))
		}
		if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.branch_id', '', true)`); err != nil {
			fatal(fmt.Sprintf("set target branch scope: %v", err))
		}
		if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.allow_tenant_scope', 'true', true)`); err != nil {
			fatal(fmt.Sprintf("set target tenant-wide scope: %v", err))
		}
	}

	var config reconcile.MappingConfig
	if strings.TrimSpace(*configPath) != "" {
		config, err = reconcile.ReadMappingConfig(*configPath)
		if err != nil {
			fatal(err.Error())
		}
		if strings.TrimSpace(*tenant) == "" {
			if *allowCanonical {
				fatal("-tenant is required when -allow-canonical is enabled; canonical reconciliation must use the dedicated target tenant")
			}
			*tenant = config.TenantID
		} else if !strings.EqualFold(strings.TrimSpace(*tenant), strings.TrimSpace(config.TenantID)) {
			if !*allowCanonical {
				fatal("reconciliation tenant does not match mapping tenantId")
			}
			reconcile.ApplyMappingScopeOverrides(&config, strings.TrimSpace(*tenant), *branchOverride, *counterOverride)
		}
		if *allowCanonical && reconcile.HasInjectedScope(config, "branch_id") && strings.TrimSpace(*branchOverride) == "" {
			fatal("-branch-id is required for this canonical mapping because it declares branch_id")
		}
		if *allowCanonical && reconcile.HasInjectedScope(config, "counter_id") && strings.TrimSpace(*counterOverride) == "" {
			fatal("-counter-id is required for this canonical mapping because it declares counter_id")
		}
		if strings.TrimSpace(*branchOverride) != "" || strings.TrimSpace(*counterOverride) != "" {
			reconcile.ApplyMappingScopeOverrides(&config, strings.TrimSpace(*tenant), *branchOverride, *counterOverride)
		}
		if strings.TrimSpace(config.SourceDatabase) != "" {
			if err := reconcile.ValidateSourceDatabase(*source, config.SourceDatabase, *allowCanonical); err != nil {
				fatal(err.Error())
			}
		}
		if *fromTable < 0 || *fromTable > len(config.Tables) ||
			(*toTable != -1 && (*toTable < *fromTable || *toTable > len(config.Tables))) {
			fatal("mapping table range is outside the reviewed configuration")
		}
		if strings.TrimSpace(*tenant) != "" {
			if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strings.TrimSpace(*tenant)); err != nil {
				fatal(fmt.Sprintf("set target tenant scope: %v", err))
			}
		}
	}

	results := make([]reconcile.TableResult, 0)
	if strings.TrimSpace(*configPath) != "" {
		endTable := *toTable
		if endTable == -1 {
			endTable = len(config.Tables)
		}
		results = reconcile.ReconcileMappings(ctx, sourceDB, targetTx, config.Tables[*fromTable:endTable], strings.TrimSpace(*tenant))
	} else {
		refs, err := reconcile.RequestedTables(ctx, sourceDB, *tables)
		if err != nil {
			fatal(err.Error())
		}
		results = make([]reconcile.TableResult, 0, len(refs))
		for _, ref := range refs {
			result := reconcile.TableResult{SourceSchema: ref.Schema, SourceTable: ref.Table, TargetSchema: ref.Schema, TargetTable: ref.Table}
			results = append(results, reconcile.ReconcileTable(ctx, sourceDB, targetTx, result, nil, ""))
		}
	}
	metricTenant := ""
	if *allowCanonical {
		metricTenant = strings.TrimSpace(*tenant)
	}
	metricResults, err := reconcile.RunMetrics(ctx, sourceDB, targetTx, *metrics, metricTenant)
	if err != nil {
		fatal(err.Error())
	}
	bookkeeping, err := reconcile.ReadBookkeeping(ctx, targetTx, strings.TrimSpace(*tenant))
	if err != nil {
		fatal(fmt.Sprintf("read target migration bookkeeping: %v", err))
	}
	result := reconcile.Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Source:      "redacted SQL Server connection",
		Target:      "redacted PostgreSQL connection",
		TenantID:    strings.TrimSpace(*tenant),
		Tables:      results,
		Metrics:     metricResults,
		Bookkeeping: bookkeeping,
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err.Error())
	}
	file, err := os.Create(*out)
	if err != nil {
		fatal(err.Error())
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatal(err.Error())
	}
	if err := targetTx.Rollback(); err != nil && err != sql.ErrTxDone {
		fatal(fmt.Sprintf("rollback target reconciliation transaction: %v", err))
	}
	fmt.Printf("Wrote reconciliation for %d tables to %s (bookkeeping: %s, open exception cases: %d, open exception rows: %d, open ambiguities: %d)\n", len(results), *out, bookkeeping.Status, bookkeeping.OpenMigrationExceptions, bookkeeping.OpenMigrationExceptionRows, bookkeeping.OpenMigrationAmbiguities)
	if *failOnOpenBookkeeping && bookkeeping.Status != "clear" {
		fatal("target migration bookkeeping is not clear")
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

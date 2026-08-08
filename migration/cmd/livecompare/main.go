// Command livecompare is the "parallel trading day" watcher: it runs the
// same table-count and business-metric comparisons as migration/cmd/reconcile,
// but repeatedly — on an interval, for a run duration — against the live
// legacy SQL Server source and the live migrated PostgreSQL target, instead
// of once against a frozen snapshot.
//
// It is a *complement* to cmd/reconcile, not a replacement: cmd/reconcile's
// one-shot batch run against a frozen snapshot is still the authoritative
// evidence attached to the cutover gate (see docs/RUNBOOK_CUTOVER.md). This
// tool exists so a human watching a physical parallel trading day sees
// discrepancies as they're entered — "these transactions diverged in the
// last 15 minutes" — instead of only finding out at day's end.
//
// See docs/PARALLEL_DAY_WATCHER.md for the operator runbook.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/abuzar/abuzar-next/migration/internal/livewatch"
	"github.com/abuzar/abuzar-next/migration/internal/reconcile"
)

func main() {
	source := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL (the live legacy system)")
	target := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "read-only PostgreSQL connection URL (the live migrated target)")
	tables := flag.String("tables", "", "optional comma-separated source tables in schema.table form (used when -config is not set)")
	configPath := flag.String("config", "", "declarative mapping configuration (same format as cmd/reconcile -config); prefer a focused subset for fast polling")
	metricsPath := flag.String("metrics", os.Getenv("ABUZAR_RECONCILE_METRICS"), "optional JSON metric-check configuration (same format as cmd/reconcile -metrics)")
	tenant := flag.String("tenant", os.Getenv("ABUZAR_RECONCILE_TENANT_ID"), "target tenant UUID used to evaluate PostgreSQL RLS")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical FazalDinPP19DataBaseV2 source")
	branchOverride := flag.String("branch-id", "", "optional target branch UUID override for mapping scope")
	counterOverride := flag.String("counter-id", "", "optional target counter UUID override for mapping scope")
	fromTable := flag.Int("from-table", 0, "zero-based first mapping table to compare each poll")
	toTable := flag.Int("to-table", -1, "exclusive mapping table limit; -1 compares through the end")

	interval := flag.Duration("interval", 15*time.Minute, "how often to poll (env ABUZAR_LIVECOMPARE_INTERVAL)")
	duration := flag.Duration("duration", 9*time.Hour, "total run duration, e.g. a business day (env ABUZAR_LIVECOMPARE_DURATION); the watcher exits cleanly once elapsed")
	pollTimeout := flag.Duration("poll-timeout", 5*time.Minute, "deadline for a single poll's DB work")
	confirmAfter := flag.Int("confirm-after", livewatch.DefaultConfirmAfter, "consecutive polls a discrepancy must persist before it is reported as confirmed rather than pending (see docs/PARALLEL_DAY_WATCHER.md)")
	outDir := flag.String("out-dir", filepath.Join("parity", "catalog", "parallel-day"), "directory for the per-poll JSON audit trail and the running JSONL log")
	once := flag.Bool("once", false, "run a single poll and exit (for smoke-testing the watcher's wiring, not for the actual parallel day)")
	failOnConfirmed := flag.Bool("fail-on-confirmed-discrepancy", false, "exit non-zero as soon as a discrepancy is confirmed, instead of continuing to poll")
	flag.Parse()

	if *source == "" || *target == "" {
		fatal("source and target are required; provide protected environment variables or flags")
	}
	if strings.TrimSpace(*configPath) == "" && strings.TrimSpace(*tables) == "" {
		fatal("either -config or -tables is required")
	}
	if *interval <= 0 {
		fatal("-interval must be positive")
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err.Error())
	}
	jsonlPath := filepath.Join(*outDir, "parallel-day-log.jsonl")
	jsonlFile, err := os.OpenFile(jsonlPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatal(err.Error())
	}
	defer jsonlFile.Close()

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

	config, resolvedTenant, err := prepareConfig(*configPath, *source, *tenant, *allowCanonical, *branchOverride, *counterOverride)
	if err != nil {
		fatal(err.Error())
	}
	fromIdx, toIdx := *fromTable, *toTable
	if config != nil {
		if toIdx == -1 {
			toIdx = len(config.Tables)
		}
		if fromIdx < 0 || fromIdx > len(config.Tables) || toIdx < fromIdx || toIdx > len(config.Tables) {
			fatal("mapping table range is outside the reviewed configuration")
		}
	}
	var metricChecks []reconcile.MetricCheck
	if strings.TrimSpace(*metricsPath) != "" {
		metricChecks, err = reconcile.LoadMetricConfig(*metricsPath)
		if err != nil {
			fatal(err.Error())
		}
	}
	metricTenant := ""
	if *allowCanonical {
		metricTenant = resolvedTenant
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deadline := time.Now().Add(*duration)
	tracker := livewatch.NewTracker(*confirmAfter)
	sequence := 0
	confirmedSeen := false

	fmt.Printf("livecompare: watching %s and %s every %s for up to %s (Ctrl+C to stop early)\n", redactedLabel(*source, "source"), redactedLabel(*target, "target"), interval.String(), duration.String())
	fmt.Printf("livecompare: audit trail under %s\n", *outDir)

	for {
		pollCtx, cancel := context.WithTimeout(ctx, *pollTimeout)
		sequence++
		windowEnd := time.Now().UTC()
		poll, pollErr := runPoll(pollCtx, sourceDB, targetDB, pollOptions{
			sequence:     sequence,
			windowEnd:    windowEnd,
			config:       config,
			tables:       *tables,
			tenant:       resolvedTenant,
			metricChecks: metricChecks,
			metricTenant: metricTenant,
			fromTable:    fromIdx,
			toTable:      toIdx,
		})
		cancel()
		if pollErr != nil {
			fmt.Fprintf(os.Stderr, "livecompare: poll %d failed: %v\n", sequence, pollErr)
		} else {
			diff := tracker.Observe(poll)
			record := pollRecord{Poll: poll, Diff: diff}
			if err := writePollFiles(*outDir, jsonlFile, record); err != nil {
				fmt.Fprintf(os.Stderr, "livecompare: poll %d: failed to write audit trail: %v\n", sequence, err)
			}
			printSummary(record)
			if diff.HasConfirmedDiscrepancy() {
				confirmedSeen = true
				if *failOnConfirmed {
					os.Exit(1)
				}
			}
		}

		if *once {
			break
		}
		if time.Now().Add(*interval).After(deadline) {
			fmt.Printf("livecompare: run duration elapsed; stopping after poll %d\n", sequence)
			break
		}
		select {
		case <-ctx.Done():
			fmt.Println("livecompare: stopped")
			return
		case <-time.After(*interval):
		}
	}

	if confirmedSeen {
		fmt.Println("livecompare: one or more discrepancies were confirmed during this run; review the audit trail before signing the Functional UAT gate")
	} else {
		fmt.Println("livecompare: no confirmed discrepancies observed during this run")
	}
}

// prepareConfig loads and validates a mapping configuration the same way
// cmd/reconcile does (tenant resolution, canonical-source and branch/counter
// override rules), returning nil when no -config was given (the -tables
// path). It is intentionally separate from cmd/reconcile's main() so that
// file is left untouched; the underlying primitives it calls
// (reconcile.ReadMappingConfig, reconcile.ApplyMappingScopeOverrides,
// reconcile.HasInjectedScope, reconcile.ValidateSourceDatabase) are the
// exact same ones cmd/reconcile uses.
func prepareConfig(configPath, sourceURL, tenant string, allowCanonical bool, branchOverride, counterOverride string) (*reconcile.MappingConfig, string, error) {
	tenant = strings.TrimSpace(tenant)
	if strings.TrimSpace(configPath) == "" {
		return nil, tenant, nil
	}
	config, err := reconcile.ReadMappingConfig(configPath)
	if err != nil {
		return nil, "", err
	}
	if tenant == "" {
		if allowCanonical {
			return nil, "", fmt.Errorf("-tenant is required when -allow-canonical is enabled; canonical reconciliation must use the dedicated target tenant")
		}
		tenant = config.TenantID
	} else if !strings.EqualFold(tenant, strings.TrimSpace(config.TenantID)) {
		if !allowCanonical {
			return nil, "", fmt.Errorf("reconciliation tenant does not match mapping tenantId")
		}
		reconcile.ApplyMappingScopeOverrides(&config, tenant, branchOverride, counterOverride)
	}
	if allowCanonical && reconcile.HasInjectedScope(config, "branch_id") && strings.TrimSpace(branchOverride) == "" {
		return nil, "", fmt.Errorf("-branch-id is required for this canonical mapping because it declares branch_id")
	}
	if allowCanonical && reconcile.HasInjectedScope(config, "counter_id") && strings.TrimSpace(counterOverride) == "" {
		return nil, "", fmt.Errorf("-counter-id is required for this canonical mapping because it declares counter_id")
	}
	if strings.TrimSpace(branchOverride) != "" || strings.TrimSpace(counterOverride) != "" {
		reconcile.ApplyMappingScopeOverrides(&config, tenant, branchOverride, counterOverride)
	}
	if strings.TrimSpace(config.SourceDatabase) != "" {
		if err := reconcile.ValidateSourceDatabase(sourceURL, config.SourceDatabase, allowCanonical); err != nil {
			return nil, "", err
		}
	}
	return &config, tenant, nil
}

type pollOptions struct {
	sequence     int
	windowEnd    time.Time
	config       *reconcile.MappingConfig
	tables       string
	tenant       string
	metricChecks []reconcile.MetricCheck
	metricTenant string
	fromTable    int
	toTable      int
}

// runPoll performs exactly one comparison pass, reusing the shared
// reconcile package's connection/config/comparison logic: it scopes a
// read-only target transaction to the resolved tenant (rolling it back
// unconditionally, the same discipline cmd/reconcile uses so the watcher
// can never mutate target state), reconciles table counts, evaluates
// metric checks, and reads bookkeeping.
func runPoll(ctx context.Context, sourceDB, targetDB *sql.DB, opts pollOptions) (livewatch.Poll, error) {
	windowStart := opts.windowEnd // no fixed lookback is applied at the query level; see docs/PARALLEL_DAY_WATCHER.md and the livewatch package doc for why timing skew is instead handled by confirming across polls.

	targetTx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return livewatch.Poll{}, fmt.Errorf("begin target poll transaction: %w", err)
	}
	defer targetTx.Rollback()

	if opts.tenant != "" {
		if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, opts.tenant); err != nil {
			return livewatch.Poll{}, fmt.Errorf("set target tenant scope: %w", err)
		}
		if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.branch_id', '', true)`); err != nil {
			return livewatch.Poll{}, fmt.Errorf("set target branch scope: %w", err)
		}
		if _, err := targetTx.ExecContext(ctx, `SELECT set_config('app.allow_tenant_scope', 'true', true)`); err != nil {
			return livewatch.Poll{}, fmt.Errorf("set target tenant-wide scope: %w", err)
		}
	}

	var tableResults []reconcile.TableResult
	if opts.config != nil {
		tableResults = reconcile.ReconcileMappings(ctx, sourceDB, targetTx, opts.config.Tables[opts.fromTable:opts.toTable], opts.tenant)
	} else {
		refs, err := reconcile.RequestedTables(ctx, sourceDB, opts.tables)
		if err != nil {
			return livewatch.Poll{}, err
		}
		tableResults = make([]reconcile.TableResult, 0, len(refs))
		for _, ref := range refs {
			result := reconcile.TableResult{SourceSchema: ref.Schema, SourceTable: ref.Table, TargetSchema: ref.Schema, TargetTable: ref.Table}
			tableResults = append(tableResults, reconcile.ReconcileTable(ctx, sourceDB, targetTx, result, nil, ""))
		}
	}

	metricResults, err := reconcile.RunMetricChecks(ctx, sourceDB, targetTx, opts.metricChecks, opts.metricTenant)
	if err != nil {
		return livewatch.Poll{}, err
	}

	bookkeeping, err := reconcile.ReadBookkeeping(ctx, targetTx, opts.tenant)
	if err != nil {
		return livewatch.Poll{}, fmt.Errorf("read target migration bookkeeping: %w", err)
	}

	if err := targetTx.Rollback(); err != nil && err != sql.ErrTxDone {
		return livewatch.Poll{}, fmt.Errorf("rollback target poll transaction: %w", err)
	}

	poll := livewatch.Poll{
		Sequence:    opts.sequence,
		PolledAt:    time.Now().UTC(),
		WindowStart: windowStart,
		WindowEnd:   opts.windowEnd,
		Tables:      make([]livewatch.TableResult, len(tableResults)),
		Metrics:     make([]livewatch.MetricResult, len(metricResults)),
		Bookkeeping: livewatch.BookkeepingResult{
			OpenMigrationExceptions:    bookkeeping.OpenMigrationExceptions,
			OpenMigrationExceptionRows: bookkeeping.OpenMigrationExceptionRows,
			OpenMigrationAmbiguities:   bookkeeping.OpenMigrationAmbiguities,
			Status:                     bookkeeping.Status,
		},
	}
	for i, r := range tableResults {
		poll.Tables[i] = livewatch.TableResult{
			SourceSchema: r.SourceSchema, SourceTable: r.SourceTable,
			TargetSchema: r.TargetSchema, TargetTable: r.TargetTable,
			SourceCount: r.SourceCount, TargetCount: r.TargetCount,
			Status: r.Status, Error: r.Error,
		}
	}
	for i, m := range metricResults {
		poll.Metrics[i] = livewatch.MetricResult{
			Name: m.Name, SourceValue: m.SourceValue, TargetValue: m.TargetValue,
			Difference: m.Difference, Tolerance: m.Tolerance, Status: m.Status, Error: m.Error,
		}
	}
	return poll, nil
}

type pollRecord struct {
	Poll livewatch.Poll `json:"poll"`
	Diff livewatch.Diff `json:"diff"`
}

func writePollFiles(outDir string, jsonlFile *os.File, record pollRecord) error {
	timestamped := filepath.Join(outDir, fmt.Sprintf("poll-%04d-%s.json", record.Poll.Sequence, record.Poll.PolledAt.Format("20060102T150405Z")))
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(timestamped, data, 0o644); err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	_, err = jsonlFile.Write(line)
	return err
}

func printSummary(record pollRecord) {
	diff := record.Diff
	label := map[string]string{"match": "MATCH", "pending": "PENDING", "discrepancy": "DISCREPANCY"}[diff.Status]
	if label == "" {
		label = strings.ToUpper(diff.Status)
	}
	fmt.Printf("\n[poll %d] %s — status: %s\n", record.Poll.Sequence, record.Poll.PolledAt.Format(time.RFC3339), label)

	if len(diff.NewTableDiscrepancies) == 0 && len(diff.OngoingTableDiscrepancies) == 0 &&
		len(diff.NewMetricDiscrepancies) == 0 && len(diff.OngoingMetricDiscrepancies) == 0 && !diff.BookkeepingChanged {
		fmt.Println("  no discrepancies since the previous poll")
	}
	for _, c := range diff.NewTableDiscrepancies {
		fmt.Printf("  NEW   table %s.%s -> %s.%s: %s%s\n", c.SourceSchema, c.SourceTable, c.TargetSchema, c.TargetTable, c.CurrentStatus, tableCountSuffix(c))
	}
	for _, c := range diff.OngoingTableDiscrepancies {
		confirmedTag := "pending"
		if c.Confirmed {
			confirmedTag = "CONFIRMED"
		}
		fmt.Printf("  %-8s table %s.%s -> %s.%s: %s (seen %d consecutive polls since poll %d)%s\n", confirmedTag, c.SourceSchema, c.SourceTable, c.TargetSchema, c.TargetTable, c.CurrentStatus, c.ConsecutivePolls, c.FirstSeenSequence, tableCountSuffix(c))
	}
	for _, c := range diff.ResolvedTableDiscrepancies {
		fmt.Printf("  RESOLVED table %s.%s -> %s.%s (was %s for %d poll(s))\n", c.SourceSchema, c.SourceTable, c.TargetSchema, c.TargetTable, c.PreviousStatus, c.ConsecutivePolls)
	}
	for _, c := range diff.NewMetricDiscrepancies {
		fmt.Printf("  NEW   metric %s: source=%s target=%s diff=%s\n", c.Name, c.CurrentSourceValue, c.CurrentTargetValue, c.CurrentDifference)
	}
	for _, c := range diff.OngoingMetricDiscrepancies {
		confirmedTag := "pending"
		if c.Confirmed {
			confirmedTag = "CONFIRMED"
		}
		fmt.Printf("  %-8s metric %s: source=%s target=%s diff=%s (seen %d consecutive polls since poll %d)\n", confirmedTag, c.Name, c.CurrentSourceValue, c.CurrentTargetValue, c.CurrentDifference, c.ConsecutivePolls, c.FirstSeenSequence)
	}
	for _, c := range diff.ResolvedMetricDiscrepancies {
		fmt.Printf("  RESOLVED metric %s (was %s for %d poll(s))\n", c.Name, c.PreviousStatus, c.ConsecutivePolls)
	}
	if diff.BookkeepingChanged {
		fmt.Printf("  BOOKKEEPING changed: %s -> %s (open exception cases: %d, rows: %d, ambiguities: %d)\n",
			bookkeepingStatusOr(diff.PreviousBookkeeping), diff.CurrentBookkeeping.Status,
			diff.CurrentBookkeeping.OpenMigrationExceptions, diff.CurrentBookkeeping.OpenMigrationExceptionRows, diff.CurrentBookkeeping.OpenMigrationAmbiguities)
	}
}

func bookkeepingStatusOr(b *livewatch.BookkeepingResult) string {
	if b == nil {
		return "unknown"
	}
	return b.Status
}

func tableCountSuffix(c livewatch.TableChange) string {
	if c.CurrentSourceCount == nil || c.CurrentTargetCount == nil {
		if c.Error != "" {
			return " (error: " + c.Error + ")"
		}
		return ""
	}
	gap := *c.CurrentSourceCount - *c.CurrentTargetCount
	suffix := fmt.Sprintf(" (source=%d target=%d gap=%s", *c.CurrentSourceCount, *c.CurrentTargetCount, formatSignedInt(gap))
	if c.CountGapDelta != nil {
		suffix += fmt.Sprintf(", gap changed by %s since last poll", formatSignedInt(*c.CountGapDelta))
	}
	return suffix + ")"
}

func formatSignedInt(v int64) string {
	if v >= 0 {
		return "+" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

func redactedLabel(rawURL, side string) string {
	databaseName := ""
	if index := strings.Index(rawURL, "?"); index >= 0 {
		for _, pair := range strings.Split(rawURL[index+1:], "&") {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "database") {
				databaseName = parts[1]
			}
		}
	}
	if databaseName == "" {
		return "redacted " + side + " connection"
	}
	return "redacted " + side + " connection (database=" + databaseName + ")"
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

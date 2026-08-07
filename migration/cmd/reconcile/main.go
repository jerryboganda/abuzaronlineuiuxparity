package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type tableRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

type tableMapping struct {
	Source            tableRef              `json:"source"`
	Target            tableRef              `json:"target"`
	SourceID          string                `json:"sourceId"`
	SourceIDColumns   []string              `json:"sourceIdColumns,omitempty"`
	TargetID          string                `json:"targetId"`
	TargetIDGenerated bool                  `json:"targetIdGenerated,omitempty"`
	Columns           map[string]string     `json:"columns"`
	PayloadColumns    map[string]string     `json:"payloadColumns,omitempty"`
	PayloadTarget     string                `json:"payloadTarget,omitempty"`
	DerivedColumns    map[string][]string   `json:"derivedColumns,omitempty"`
	GeneratedColumns  map[string]string     `json:"generatedColumns,omitempty"`
	SourceExpressions map[string]string     `json:"sourceExpressions,omitempty"`
	SourceFilter      string                `json:"sourceFilter,omitempty"`
	Lookups           map[string]lookupSpec `json:"lookups,omitempty"`
	Inject            map[string]string     `json:"inject,omitempty"`
	TargetCountJoin   *targetCountJoin      `json:"targetCountJoin,omitempty"`
	ConflictColumn    []string              `json:"conflictColumns"`
}

type lookupSpec struct {
	Target       tableRef          `json:"target"`
	TargetColumn string            `json:"targetColumn"`
	ValueColumn  string            `json:"valueColumn,omitempty"`
	SourceColumn string            `json:"sourceColumn"`
	Predicates   map[string]string `json:"predicates,omitempty"`
}

type targetCountJoin struct {
	Table         tableRef          `json:"table"`
	LocalColumn   string            `json:"localColumn"`
	ForeignColumn string            `json:"foreignColumn"`
	Predicates    map[string]string `json:"predicates,omitempty"`
}

type mappingConfig struct {
	TenantID       string         `json:"tenantId"`
	DefaultBranch  string         `json:"defaultBranchId,omitempty"`
	SourceDatabase string         `json:"sourceDatabase,omitempty"`
	Tables         []tableMapping `json:"tables"`
}

type tableResult struct {
	SourceSchema string `json:"sourceSchema"`
	SourceTable  string `json:"sourceTable"`
	SourceCount  *int64 `json:"sourceCount,omitempty"`
	TargetSchema string `json:"targetSchema"`
	TargetTable  string `json:"targetTable"`
	TargetCount  *int64 `json:"targetCount,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

type report struct {
	GeneratedAt string            `json:"generatedAt"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	TenantID    string            `json:"tenantId,omitempty"`
	Tables      []tableResult     `json:"tables"`
	Metrics     []metricResult    `json:"metrics,omitempty"`
	Bookkeeping bookkeepingResult `json:"bookkeeping"`
}

type bookkeepingResult struct {
	OpenMigrationExceptions    int64  `json:"openMigrationExceptionCount"`
	OpenMigrationExceptionRows int64  `json:"openMigrationExceptionRowCount"`
	OpenMigrationAmbiguities   int64  `json:"openMigrationAmbiguityCount"`
	Status                     string `json:"status"`
}

const bookkeepingMigrationExceptionQuery = `
	SELECT COUNT(*) AS open_rows,
	       COUNT(DISTINCT (source_schema, source_table, legacy_id, reason_code)) AS open_cases
	FROM public.migration_exceptions
	WHERE status = 'open'
	  AND ($1 = '' OR tenant_id::text = $1)`

type metricCheck struct {
	Name        string  `json:"name"`
	SourceQuery string  `json:"sourceQuery"`
	TargetQuery string  `json:"targetQuery"`
	Tolerance   float64 `json:"tolerance,omitempty"`
}

type metricConfig struct {
	Metrics []metricCheck `json:"metrics"`
}

type metricResult struct {
	Name        string  `json:"name"`
	SourceValue string  `json:"sourceValue,omitempty"`
	TargetValue string  `json:"targetValue,omitempty"`
	Difference  string  `json:"difference,omitempty"`
	Tolerance   float64 `json:"tolerance"`
	Status      string  `json:"status"`
	Error       string  `json:"error,omitempty"`
}

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
	flag.Parse()
	if *source == "" || *target == "" {
		fatal("source and target are required; provide protected environment variables or flags")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	sourceDB, err := sql.Open("sqlserver", *source)
	if err != nil {
		fatal(err.Error())
	}
	defer sourceDB.Close()
	targetDB, err := sql.Open("pgx", *target)
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

	var config mappingConfig
	if strings.TrimSpace(*configPath) != "" {
		config, err = readMappingConfig(*configPath)
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
			applyMappingScopeOverrides(&config, strings.TrimSpace(*tenant), *branchOverride, *counterOverride)
		}
		if *allowCanonical && hasInjectedScope(config, "branch_id") && strings.TrimSpace(*branchOverride) == "" {
			fatal("-branch-id is required for this canonical mapping because it declares branch_id")
		}
		if *allowCanonical && hasInjectedScope(config, "counter_id") && strings.TrimSpace(*counterOverride) == "" {
			fatal("-counter-id is required for this canonical mapping because it declares counter_id")
		}
		if strings.TrimSpace(*branchOverride) != "" || strings.TrimSpace(*counterOverride) != "" {
			applyMappingScopeOverrides(&config, strings.TrimSpace(*tenant), *branchOverride, *counterOverride)
		}
		if strings.TrimSpace(config.SourceDatabase) != "" {
			if err := validateSourceDatabase(*source, config.SourceDatabase, *allowCanonical); err != nil {
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

	results := make([]tableResult, 0)
	if strings.TrimSpace(*configPath) != "" {
		endTable := *toTable
		if endTable == -1 {
			endTable = len(config.Tables)
		}
		results = reconcileMappings(ctx, sourceDB, targetTx, config.Tables[*fromTable:endTable], strings.TrimSpace(*tenant))
	} else {
		refs, err := requestedTables(ctx, sourceDB, *tables)
		if err != nil {
			fatal(err.Error())
		}
		results = make([]tableResult, 0, len(refs))
		for _, ref := range refs {
			result := tableResult{SourceSchema: ref.Schema, SourceTable: ref.Table, TargetSchema: ref.Schema, TargetTable: ref.Table}
			results = append(results, reconcileTable(ctx, sourceDB, targetTx, result, nil, ""))
		}
	}
	metricTenant := ""
	if *allowCanonical {
		metricTenant = strings.TrimSpace(*tenant)
	}
	metricResults, err := runMetrics(ctx, sourceDB, targetTx, *metrics, metricTenant)
	if err != nil {
		fatal(err.Error())
	}
	bookkeeping, err := readBookkeeping(ctx, targetTx, strings.TrimSpace(*tenant))
	if err != nil {
		fatal(fmt.Sprintf("read target migration bookkeeping: %v", err))
	}
	result := report{
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

func readBookkeeping(ctx context.Context, target queryer, tenant string) (bookkeepingResult, error) {
	result := bookkeepingResult{}
	if err := target.QueryRowContext(ctx, bookkeepingMigrationExceptionQuery, tenant).Scan(
		&result.OpenMigrationExceptionRows, &result.OpenMigrationExceptions,
	); err != nil {
		return bookkeepingResult{}, fmt.Errorf("count open migration exceptions: %w", err)
	}
	const ambiguityQuery = `
		SELECT COUNT(*)
		FROM public.migration_ambiguous_records
		WHERE status = 'open'
		  AND ($1 = '' OR tenant_id::text = $1)`
	if err := target.QueryRowContext(ctx, ambiguityQuery, tenant).Scan(&result.OpenMigrationAmbiguities); err != nil {
		return bookkeepingResult{}, fmt.Errorf("count open migration ambiguities: %w", err)
	}
	result.Status = bookkeepingStatus(result.OpenMigrationExceptions, result.OpenMigrationAmbiguities)
	return result, nil
}

func bookkeepingStatus(openExceptions, openAmbiguities int64) string {
	if openExceptions == 0 && openAmbiguities == 0 {
		return "clear"
	}
	return "review_required"
}

func reconcileMappings(ctx context.Context, source, target queryer, mappings []tableMapping, tenant string) []tableResult {
	results := make([]tableResult, 0, len(mappings))
	for _, mapping := range mappings {
		result := tableResult{
			SourceSchema: mapping.Source.Schema,
			SourceTable:  mapping.Source.Table,
			TargetSchema: mapping.Target.Schema,
			TargetTable:  mapping.Target.Table,
		}
		results = append(results, reconcileTable(ctx, source, target, result, &mapping, tenant))
	}
	return results
}

func reconcileTable(ctx context.Context, source, target queryer, result tableResult, mapping *tableMapping, tenant string) tableResult {
	sourceFilter := ""
	if mapping != nil {
		sourceFilter = mapping.SourceFilter
	}
	sourceCount, err := countSQLServer(ctx, source, tableRef{Schema: result.SourceSchema, Table: result.SourceTable}, sourceFilter)
	if err != nil {
		result.Status = "exception"
		result.Error = err.Error()
		return result
	}
	result.SourceCount = &sourceCount
	targetRef := tableRef{Schema: result.TargetSchema, Table: result.TargetTable}
	present, err := targetTableExists(ctx, target, targetRef)
	if err != nil {
		result.Status = "exception"
		result.Error = err.Error()
		return result
	}
	if !present {
		result.Status = "missing_target"
		return result
	}
	var targetCount int64
	if mapping == nil {
		targetCount, err = countPostgres(ctx, target, targetRef)
	} else {
		targetCount, err = countPostgresMapping(ctx, target, targetRef, *mapping, tenant)
	}
	if err != nil {
		result.Status = "exception"
		result.Error = err.Error()
		return result
	}
	result.TargetCount = &targetCount
	if sourceCount == targetCount {
		result.Status = "matched"
	} else {
		result.Status = "mismatched"
	}
	return result
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readMappingConfig(path string) (mappingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mappingConfig{}, fmt.Errorf("read mapping config: %w", err)
	}
	var config mappingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return mappingConfig{}, fmt.Errorf("parse mapping config: %w", err)
	}
	if strings.TrimSpace(config.TenantID) == "" || len(config.Tables) == 0 {
		return mappingConfig{}, fmt.Errorf("mapping config requires tenantId and at least one table")
	}
	return config, nil
}

func validateSourceDatabase(rawURL, expected string, allowCanonical bool) error {
	databaseName := ""
	if index := strings.Index(rawURL, "?"); index >= 0 {
		for _, pair := range strings.Split(rawURL[index+1:], "&") {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "database") {
				databaseName = parts[1]
			}
		}
	}
	if strings.TrimSpace(databaseName) == "" {
		return errors.New("source URL must name a database for a reviewed mapping")
	}
	if strings.EqualFold(databaseName, "FazalDinPP19DataBaseV2") {
		if !allowCanonical {
			return errors.New("canonical FazalDinPP19DataBaseV2 is not an import/reconciliation mapping source; pass -allow-canonical with an explicit -tenant")
		}
		return nil
	}
	if !strings.EqualFold(databaseName, expected) {
		return fmt.Errorf("source database %q does not match reviewed mapping database", databaseName)
	}
	return nil
}

func applyMappingScopeOverrides(config *mappingConfig, tenantID, branchID, counterID string) {
	config.TenantID = tenantID
	branchID = strings.TrimSpace(branchID)
	counterID = strings.TrimSpace(counterID)
	for index := range config.Tables {
		inject := config.Tables[index].Inject
		if inject == nil {
			continue
		}
		if tenantID != "" {
			if _, present := inject["tenant_id"]; present {
				inject["tenant_id"] = tenantID
			}
		}
		if branchID != "" {
			if _, present := inject["branch_id"]; present {
				inject["branch_id"] = branchID
			}
		}
		if counterID != "" {
			if _, present := inject["counter_id"]; present {
				inject["counter_id"] = counterID
			}
		}
		config.Tables[index].Inject = inject
	}
}

func hasInjectedScope(config mappingConfig, key string) bool {
	for _, mapping := range config.Tables {
		if _, present := mapping.Inject[key]; present {
			return true
		}
	}
	return false
}

func countPostgresMapping(ctx context.Context, database queryer, ref tableRef, mapping tableMapping, tenant string) (int64, error) {
	predicates := []string{"target.\"tenant_id\" = $1"}
	args := []any{tenant}
	injectedColumns := make([]string, 0, len(mapping.Inject))
	for column := range mapping.Inject {
		if column != "tenant_id" {
			injectedColumns = append(injectedColumns, column)
		}
	}
	sort.Strings(injectedColumns)
	for _, column := range injectedColumns {
		predicates = append(predicates, "target."+quotePostgres(column)+" = $"+strconv.Itoa(len(args)+1))
		args = append(args, mapping.Inject[column])
	}
	from := quotePostgres(ref.Schema) + "." + quotePostgres(ref.Table) + " AS target"
	if mapping.TargetCountJoin != nil {
		join := mapping.TargetCountJoin
		if strings.TrimSpace(join.Table.Schema) == "" || strings.TrimSpace(join.Table.Table) == "" ||
			strings.TrimSpace(join.LocalColumn) == "" || strings.TrimSpace(join.ForeignColumn) == "" {
			return 0, errors.New("targetCountJoin requires table, localColumn, and foreignColumn")
		}
		from += " JOIN " + quotePostgres(join.Table.Schema) + "." + quotePostgres(join.Table.Table) + " AS count_join ON count_join." + quotePostgres(join.ForeignColumn) + " = target." + quotePostgres(join.LocalColumn) + " AND count_join.\"tenant_id\" = target.\"tenant_id\""
		if containsString(injectedColumns, "branch_id") {
			from += " AND count_join.\"branch_id\" = target.\"branch_id\""
		}
		predicateNames := make([]string, 0, len(join.Predicates))
		for predicate := range join.Predicates {
			predicateNames = append(predicateNames, predicate)
		}
		sort.Strings(predicateNames)
		for _, predicate := range predicateNames {
			if strings.TrimSpace(predicate) == "" {
				return 0, errors.New("targetCountJoin contains an empty predicate")
			}
			predicates = append(predicates, "count_join."+quotePostgres(predicate)+" = $"+strconv.Itoa(len(args)+1))
			args = append(args, join.Predicates[predicate])
		}
	}
	query := "SELECT COUNT(*) FROM " + from + " WHERE " + strings.Join(predicates, " AND ")
	var count int64
	if err := database.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count target %s.%s: %w", ref.Schema, ref.Table, err)
	}
	return count, nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func runMetrics(ctx context.Context, source, target queryer, path, tenantOverride string) ([]metricResult, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metric config: %w", err)
	}
	var config metricConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse metric config: %w", err)
	}
	results := make([]metricResult, 0, len(config.Metrics))
	for _, check := range config.Metrics {
		result := metricResult{Name: check.Name, Tolerance: check.Tolerance, Status: "exception"}
		if result.Tolerance <= 0 {
			result.Tolerance = 0.0001
		}
		if strings.TrimSpace(check.Name) == "" || !readOnlySelect(check.SourceQuery) || !readOnlySelect(check.TargetQuery) {
			result.Error = "metric name and read-only SELECT queries are required"
			results = append(results, result)
			continue
		}
		sourceValue, err := queryMetric(ctx, source, check.SourceQuery)
		if err != nil {
			result.Error = "source metric failed: " + err.Error()
			results = append(results, result)
			continue
		}
		targetQuery := rewriteMetricTenant(check.TargetQuery, tenantOverride)
		targetValue, err := queryMetric(ctx, target, targetQuery)
		if err != nil {
			result.Error = "target metric failed: " + err.Error()
			results = append(results, result)
			continue
		}
		result.SourceValue = sourceValue.String()
		result.TargetValue = targetValue.String()
		result.Difference = fmt.Sprintf("%.8f", sourceValue.Value-targetValue.Value)
		if math.Abs(sourceValue.Value-targetValue.Value) <= result.Tolerance {
			result.Status = "matched"
		} else {
			result.Status = "mismatched"
		}
		results = append(results, result)
	}
	return results, nil
}

func rewriteMetricTenant(query, tenantOverride string) string {
	tenantOverride = strings.TrimSpace(tenantOverride)
	if tenantOverride == "" || strings.EqualFold(tenantOverride, "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee") {
		return query
	}
	return strings.ReplaceAll(query, "'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'", "'"+tenantOverride+"'")
}

type decimalMetric struct {
	Value float64
	Raw   string
}

func (value decimalMetric) String() string {
	if value.Raw != "" {
		return value.Raw
	}
	return fmt.Sprintf("%.8f", value.Value)
}

func queryMetric(ctx context.Context, database queryer, query string) (decimalMetric, error) {
	var raw any
	if err := database.QueryRowContext(ctx, query).Scan(&raw); err != nil {
		return decimalMetric{}, err
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if bytes, ok := raw.([]byte); ok {
		text = strings.TrimSpace(string(bytes))
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return decimalMetric{}, fmt.Errorf("metric is not numeric: %w", err)
	}
	return decimalMetric{Value: value, Raw: text}, nil
}

func readOnlySelect(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" || strings.Contains(trimmed, ";") || !strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
		return false
	}
	if dangerousMetricKeyword.MatchString(trimmed) {
		return false
	}
	return true
}

var dangerousMetricKeyword = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|EXEC|CALL|MERGE)([^[:alnum:]_]|$)`)

func requestedTables(ctx context.Context, database *sql.DB, value string) ([]tableRef, error) {
	if strings.TrimSpace(value) != "" {
		refs := make([]tableRef, 0)
		for _, raw := range strings.Split(value, ",") {
			parts := strings.SplitN(strings.TrimSpace(raw), ".", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("invalid table reference %q; use schema.table", raw)
			}
			refs = append(refs, tableRef{Schema: parts[0], Table: parts[1]})
		}
		return refs, nil
	}
	rows, err := database.QueryContext(ctx, `
		SELECT TABLE_SCHEMA, TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`)
	if err != nil {
		return nil, fmt.Errorf("list source tables: %w", err)
	}
	defer rows.Close()
	refs := make([]tableRef, 0)
	for rows.Next() {
		var ref tableRef
		if err := rows.Scan(&ref.Schema, &ref.Table); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Schema == refs[j].Schema {
			return refs[i].Table < refs[j].Table
		}
		return refs[i].Schema < refs[j].Schema
	})
	return refs, nil
}

func countSQLServer(ctx context.Context, database queryer, ref tableRef, sourceFilter string) (int64, error) {
	var count int64
	query := "SELECT COUNT_BIG(*) FROM " + quoteSQLServer(ref.Schema) + "." + quoteSQLServer(ref.Table)
	if strings.TrimSpace(sourceFilter) != "" {
		query += " WHERE " + sourceFilter
	}
	if err := database.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count source %s.%s: %w", ref.Schema, ref.Table, err)
	}
	return count, nil
}

func targetTableExists(ctx context.Context, database queryer, ref tableRef) (bool, error) {
	var exists bool
	err := database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`, ref.Schema, ref.Table).Scan(&exists)
	return exists, err
}

func countPostgres(ctx context.Context, database queryer, ref tableRef) (int64, error) {
	var count int64
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quotePostgres(ref.Schema)+"."+quotePostgres(ref.Table)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count target %s.%s: %w", ref.Schema, ref.Table, err)
	}
	return count, nil
}

func quoteSQLServer(identifier string) string {
	return "[" + strings.ReplaceAll(identifier, "]", "]]") + "]"
}

func quotePostgres(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

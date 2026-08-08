// Package reconcile holds the connection-handling, mapping-config, and
// comparison logic shared by migration/cmd/reconcile (the one-shot batch
// reconciler run at cutover) and migration/cmd/livecompare (the repeated
// watcher run during a parallel trading day). It was extracted verbatim out
// of cmd/reconcile/main.go so both commands compare source (SQL Server) and
// target (PostgreSQL) the same way instead of maintaining two copies of the
// same queries. cmd/reconcile's own behavior and output format are
// unchanged by this move — every exported symbol here is the same code,
// under the same names save for the leading capital letter needed to export
// it across a package boundary.
package reconcile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TableRef names one schema-qualified table on either side of a comparison.
type TableRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

// LookupSpec resolves a source column's value against a target lookup table;
// it is part of the declarative mapping format but is not evaluated by the
// count-based reconciliation in this package (import tooling consumes it).
type LookupSpec struct {
	Target       TableRef          `json:"target"`
	TargetColumn string            `json:"targetColumn"`
	ValueColumn  string            `json:"valueColumn,omitempty"`
	SourceColumn string            `json:"sourceColumn"`
	Predicates   map[string]string `json:"predicates,omitempty"`
}

// TargetCountJoin optionally joins an additional target table into a
// mapping's target COUNT(*) — used when the reconciled row count lives one
// hop away from the mapped table itself.
type TargetCountJoin struct {
	Table         TableRef          `json:"table"`
	LocalColumn   string            `json:"localColumn"`
	ForeignColumn string            `json:"foreignColumn"`
	Predicates    map[string]string `json:"predicates,omitempty"`
}

// TableMapping declares how one source table maps onto one target table,
// including any injected scope columns (tenant/branch/counter) and filters.
type TableMapping struct {
	Source            TableRef              `json:"source"`
	Target            TableRef              `json:"target"`
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
	Lookups           map[string]LookupSpec `json:"lookups,omitempty"`
	Inject            map[string]string     `json:"inject,omitempty"`
	TargetCountJoin   *TargetCountJoin      `json:"targetCountJoin,omitempty"`
	ConflictColumn    []string              `json:"conflictColumns"`
}

// MappingConfig is the declarative reconciliation/import configuration
// format read from a JSON file (see migration/maps/*.json).
type MappingConfig struct {
	TenantID       string         `json:"tenantId"`
	DefaultBranch  string         `json:"defaultBranchId,omitempty"`
	SourceDatabase string         `json:"sourceDatabase,omitempty"`
	Tables         []TableMapping `json:"tables"`
}

// TableResult is one table's row-count comparison outcome.
type TableResult struct {
	SourceSchema string `json:"sourceSchema"`
	SourceTable  string `json:"sourceTable"`
	SourceCount  *int64 `json:"sourceCount,omitempty"`
	TargetSchema string `json:"targetSchema"`
	TargetTable  string `json:"targetTable"`
	TargetCount  *int64 `json:"targetCount,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

// Report is the full reconciliation output: table counts, business metrics,
// and target migration bookkeeping. cmd/reconcile writes this shape as its
// JSON report; cmd/livecompare writes one per poll.
type Report struct {
	GeneratedAt string            `json:"generatedAt"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	TenantID    string            `json:"tenantId,omitempty"`
	Tables      []TableResult     `json:"tables"`
	Metrics     []MetricResult    `json:"metrics,omitempty"`
	Bookkeeping BookkeepingResult `json:"bookkeeping"`
}

// BookkeepingResult summarizes open target-side migration exceptions and
// ambiguities. It reads only target state, so — unlike table/metric checks —
// it carries no source/target timing race.
type BookkeepingResult struct {
	OpenMigrationExceptions    int64  `json:"openMigrationExceptionCount"`
	OpenMigrationExceptionRows int64  `json:"openMigrationExceptionRowCount"`
	OpenMigrationAmbiguities   int64  `json:"openMigrationAmbiguityCount"`
	Status                     string `json:"status"`
}

// BookkeepingMigrationExceptionQuery counts open target migration
// exceptions, both as raw rows and as distinct source cases.
const BookkeepingMigrationExceptionQuery = `
	SELECT COUNT(*) AS open_rows,
	       COUNT(DISTINCT (source_schema, source_table, legacy_id, reason_code)) AS open_cases
	FROM public.migration_exceptions
	WHERE status = 'open'
	  AND ($1 = '' OR tenant_id::text = $1)`

// MetricCheck is one declarative business-metric comparison: a read-only
// SELECT against the source and another against the target, compared within
// Tolerance.
type MetricCheck struct {
	Name        string  `json:"name"`
	SourceQuery string  `json:"sourceQuery"`
	TargetQuery string  `json:"targetQuery"`
	Tolerance   float64 `json:"tolerance,omitempty"`
}

// MetricConfig is the JSON file format read by RunMetrics.
type MetricConfig struct {
	Metrics []MetricCheck `json:"metrics"`
}

// MetricResult is one metric check's outcome.
type MetricResult struct {
	Name        string  `json:"name"`
	SourceValue string  `json:"sourceValue,omitempty"`
	TargetValue string  `json:"targetValue,omitempty"`
	Difference  string  `json:"difference,omitempty"`
	Tolerance   float64 `json:"tolerance"`
	Status      string  `json:"status"`
	Error       string  `json:"error,omitempty"`
}

// Queryer is the minimal *sql.DB / *sql.Tx surface this package needs, so
// callers may pass either a plain connection pool or a transaction (as
// cmd/reconcile does, to pin a consistent RLS-scoped read against target).
type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// ReadMappingConfig loads and minimally validates a declarative mapping
// configuration file.
func ReadMappingConfig(path string) (MappingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MappingConfig{}, fmt.Errorf("read mapping config: %w", err)
	}
	var config MappingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return MappingConfig{}, fmt.Errorf("parse mapping config: %w", err)
	}
	if strings.TrimSpace(config.TenantID) == "" || len(config.Tables) == 0 {
		return MappingConfig{}, fmt.Errorf("mapping config requires tenantId and at least one table")
	}
	return config, nil
}

// ValidateSourceDatabase confirms the source connection URL names the
// database the reviewed mapping expects, rejecting the protected canonical
// source unless allowCanonical is explicitly set.
func ValidateSourceDatabase(rawURL, expected string, allowCanonical bool) error {
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

// ApplyMappingScopeOverrides rewrites a mapping config's tenant/branch/
// counter injected scope columns in place, for the reviewed canonical-source
// override path.
func ApplyMappingScopeOverrides(config *MappingConfig, tenantID, branchID, counterID string) {
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

// HasInjectedScope reports whether any table mapping declares the given
// injected scope column (e.g. "branch_id").
func HasInjectedScope(config MappingConfig, key string) bool {
	for _, mapping := range config.Tables {
		if _, present := mapping.Inject[key]; present {
			return true
		}
	}
	return false
}

// ReconcileMappings runs ReconcileTable over every mapping in order,
// returning one TableResult per mapping.
func ReconcileMappings(ctx context.Context, source, target Queryer, mappings []TableMapping, tenant string) []TableResult {
	results := make([]TableResult, 0, len(mappings))
	for _, mapping := range mappings {
		result := TableResult{
			SourceSchema: mapping.Source.Schema,
			SourceTable:  mapping.Source.Table,
			TargetSchema: mapping.Target.Schema,
			TargetTable:  mapping.Target.Table,
		}
		results = append(results, ReconcileTable(ctx, source, target, result, &mapping, tenant))
	}
	return results
}

// ReconcileTable compares one table's source and target row counts. Pass a
// nil mapping to count the raw identically-named table on both sides (the
// -tables flag path); pass a mapping to apply its scope injections, filter,
// and optional count join.
func ReconcileTable(ctx context.Context, source, target Queryer, result TableResult, mapping *TableMapping, tenant string) TableResult {
	sourceFilter := ""
	if mapping != nil {
		sourceFilter = mapping.SourceFilter
	}
	sourceCount, err := CountSQLServer(ctx, source, TableRef{Schema: result.SourceSchema, Table: result.SourceTable}, sourceFilter)
	if err != nil {
		result.Status = "exception"
		result.Error = err.Error()
		return result
	}
	result.SourceCount = &sourceCount
	targetRef := TableRef{Schema: result.TargetSchema, Table: result.TargetTable}
	present, err := TargetTableExists(ctx, target, targetRef)
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
		targetCount, err = CountPostgres(ctx, target, targetRef)
	} else {
		targetCount, err = CountPostgresMapping(ctx, target, targetRef, *mapping, tenant)
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

// CountPostgresMapping counts target rows for a mapping, applying its
// tenant scope, any other injected scope columns, and its optional count
// join.
func CountPostgresMapping(ctx context.Context, database Queryer, ref TableRef, mapping TableMapping, tenant string) (int64, error) {
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
		predicates = append(predicates, "target."+QuotePostgres(column)+" = $"+strconv.Itoa(len(args)+1))
		args = append(args, mapping.Inject[column])
	}
	from := QuotePostgres(ref.Schema) + "." + QuotePostgres(ref.Table) + " AS target"
	if mapping.TargetCountJoin != nil {
		join := mapping.TargetCountJoin
		if strings.TrimSpace(join.Table.Schema) == "" || strings.TrimSpace(join.Table.Table) == "" ||
			strings.TrimSpace(join.LocalColumn) == "" || strings.TrimSpace(join.ForeignColumn) == "" {
			return 0, errors.New("targetCountJoin requires table, localColumn, and foreignColumn")
		}
		from += " JOIN " + QuotePostgres(join.Table.Schema) + "." + QuotePostgres(join.Table.Table) + " AS count_join ON count_join." + QuotePostgres(join.ForeignColumn) + " = target." + QuotePostgres(join.LocalColumn) + " AND count_join.\"tenant_id\" = target.\"tenant_id\""
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
			predicates = append(predicates, "count_join."+QuotePostgres(predicate)+" = $"+strconv.Itoa(len(args)+1))
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

// RunMetrics evaluates every metric check in the JSON file at path against
// source and target, returning nil if path is empty. tenantOverride, when
// non-empty, rewrites the reviewed sandbox tenant literal embedded in each
// target query (see RewriteMetricTenant).
func RunMetrics(ctx context.Context, source, target Queryer, path, tenantOverride string) ([]MetricResult, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metric config: %w", err)
	}
	var config MetricConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse metric config: %w", err)
	}
	return RunMetricChecks(ctx, source, target, config.Metrics, tenantOverride)
}

// RunMetricChecks evaluates an already-loaded slice of metric checks. It is
// the part of RunMetrics that does not touch the filesystem, split out so a
// repeated poller can load the metric config once and re-run the checks on
// every interval instead of re-reading and re-parsing the file each time.
func RunMetricChecks(ctx context.Context, source, target Queryer, checks []MetricCheck, tenantOverride string) ([]MetricResult, error) {
	results := make([]MetricResult, 0, len(checks))
	for _, check := range checks {
		result := MetricResult{Name: check.Name, Tolerance: check.Tolerance, Status: "exception"}
		if result.Tolerance <= 0 {
			result.Tolerance = 0.0001
		}
		if strings.TrimSpace(check.Name) == "" || !ReadOnlySelect(check.SourceQuery) || !ReadOnlySelect(check.TargetQuery) {
			result.Error = "metric name and read-only SELECT queries are required"
			results = append(results, result)
			continue
		}
		sourceValue, err := QueryMetric(ctx, source, check.SourceQuery)
		if err != nil {
			result.Error = "source metric failed: " + err.Error()
			results = append(results, result)
			continue
		}
		targetQuery := RewriteMetricTenant(check.TargetQuery, tenantOverride)
		targetValue, err := QueryMetric(ctx, target, targetQuery)
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

// LoadMetricConfig reads and parses a metric-check JSON file without
// evaluating it, so a caller can load it once and reuse it across many
// RunMetricChecks calls.
func LoadMetricConfig(path string) ([]MetricCheck, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metric config: %w", err)
	}
	var config MetricConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse metric config: %w", err)
	}
	return config.Metrics, nil
}

// RewriteMetricTenant replaces the reviewed sandbox tenant literal embedded
// in a target metric query with tenantOverride, when set.
func RewriteMetricTenant(query, tenantOverride string) string {
	tenantOverride = strings.TrimSpace(tenantOverride)
	if tenantOverride == "" || strings.EqualFold(tenantOverride, "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee") {
		return query
	}
	return strings.ReplaceAll(query, "'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'", "'"+tenantOverride+"'")
}

// DecimalMetric is a metric value carried alongside its original database
// text representation, so exact-text sums (e.g. numeric(19,4) literals) are
// preserved in reports even though comparisons happen on the parsed float.
type DecimalMetric struct {
	Value float64
	Raw   string
}

func (value DecimalMetric) String() string {
	if value.Raw != "" {
		return value.Raw
	}
	return fmt.Sprintf("%.8f", value.Value)
}

// QueryMetric runs a single-value SELECT and parses the result as a decimal.
func QueryMetric(ctx context.Context, database Queryer, query string) (DecimalMetric, error) {
	var raw any
	if err := database.QueryRowContext(ctx, query).Scan(&raw); err != nil {
		return DecimalMetric{}, err
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if bytes, ok := raw.([]byte); ok {
		text = strings.TrimSpace(string(bytes))
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return DecimalMetric{}, fmt.Errorf("metric is not numeric: %w", err)
	}
	return DecimalMetric{Value: value, Raw: text}, nil
}

// ReadOnlySelect reports whether query is a single, non-dangerous, read-only
// SELECT statement — the only shape allowed for a metric check query.
func ReadOnlySelect(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" || strings.Contains(trimmed, ";") || !strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
		return false
	}
	if DangerousMetricKeyword.MatchString(trimmed) {
		return false
	}
	return true
}

// DangerousMetricKeyword matches any mutating SQL keyword, used to reject
// metric queries that are not plain read-only SELECTs.
var DangerousMetricKeyword = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|EXEC|CALL|MERGE)([^[:alnum:]_]|$)`)

// RequestedTables resolves the -tables flag value (a comma-separated list of
// schema.table references) or, when empty, lists every base table in the
// source database.
func RequestedTables(ctx context.Context, database *sql.DB, value string) ([]TableRef, error) {
	if strings.TrimSpace(value) != "" {
		refs := make([]TableRef, 0)
		for _, raw := range strings.Split(value, ",") {
			parts := strings.SplitN(strings.TrimSpace(raw), ".", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("invalid table reference %q; use schema.table", raw)
			}
			refs = append(refs, TableRef{Schema: parts[0], Table: parts[1]})
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
	refs := make([]TableRef, 0)
	for rows.Next() {
		var ref TableRef
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

// CountSQLServer runs COUNT_BIG(*) against a source table, optionally
// filtered by sourceFilter.
func CountSQLServer(ctx context.Context, database Queryer, ref TableRef, sourceFilter string) (int64, error) {
	var count int64
	query := "SELECT COUNT_BIG(*) FROM " + QuoteSQLServer(ref.Schema) + "." + QuoteSQLServer(ref.Table)
	if strings.TrimSpace(sourceFilter) != "" {
		query += " WHERE " + sourceFilter
	}
	if err := database.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count source %s.%s: %w", ref.Schema, ref.Table, err)
	}
	return count, nil
}

// TargetTableExists reports whether a target schema.table exists.
func TargetTableExists(ctx context.Context, database Queryer, ref TableRef) (bool, error) {
	var exists bool
	err := database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`, ref.Schema, ref.Table).Scan(&exists)
	return exists, err
}

// CountPostgres runs an unscoped COUNT(*) against a target table (the
// -tables flag path, with no mapping and therefore no tenant scope to
// apply).
func CountPostgres(ctx context.Context, database Queryer, ref TableRef) (int64, error) {
	var count int64
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+QuotePostgres(ref.Schema)+"."+QuotePostgres(ref.Table)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count target %s.%s: %w", ref.Schema, ref.Table, err)
	}
	return count, nil
}

// QuoteSQLServer bracket-quotes a SQL Server identifier.
func QuoteSQLServer(identifier string) string {
	return "[" + strings.ReplaceAll(identifier, "]", "]]") + "]"
}

// QuotePostgres double-quotes a PostgreSQL identifier.
func QuotePostgres(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// ReadBookkeeping reads open target migration exception and ambiguity
// counts, optionally scoped to tenant.
func ReadBookkeeping(ctx context.Context, target Queryer, tenant string) (BookkeepingResult, error) {
	result := BookkeepingResult{}
	if err := target.QueryRowContext(ctx, BookkeepingMigrationExceptionQuery, tenant).Scan(
		&result.OpenMigrationExceptionRows, &result.OpenMigrationExceptions,
	); err != nil {
		return BookkeepingResult{}, fmt.Errorf("count open migration exceptions: %w", err)
	}
	const ambiguityQuery = `
		SELECT COUNT(*)
		FROM public.migration_ambiguous_records
		WHERE status = 'open'
		  AND ($1 = '' OR tenant_id::text = $1)`
	if err := target.QueryRowContext(ctx, ambiguityQuery, tenant).Scan(&result.OpenMigrationAmbiguities); err != nil {
		return BookkeepingResult{}, fmt.Errorf("count open migration ambiguities: %w", err)
	}
	result.Status = BookkeepingStatus(result.OpenMigrationExceptions, result.OpenMigrationAmbiguities)
	return result, nil
}

// BookkeepingStatus is "clear" only when no open exceptions or ambiguities
// remain on the target.
func BookkeepingStatus(openExceptions, openAmbiguities int64) string {
	if openExceptions == 0 && openAmbiguities == 0 {
		return "clear"
	}
	return "review_required"
}

// OpenSourceDB opens (without connecting) the SQL Server source pool.
func OpenSourceDB(dsn string) (*sql.DB, error) {
	return sql.Open("sqlserver", dsn)
}

// OpenTargetDB opens (without connecting) the PostgreSQL target pool.
func OpenTargetDB(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}

// ReconcileTimeoutDefault returns the overall reconcile deadline: the
// ABUZAR_RECONCILE_TIMEOUT env var (a Go duration string, e.g. "15m") if
// set and valid, otherwise 10 minutes. The original hardcoded 2*time.Minute
// covered DB pings, every configured metric query against both the source
// and target, and the bookkeeping check in one shared budget; that was
// already tight and became insufficient once value-sum metrics (SUM over
// full source tables, heavier than a COUNT(*)) were added alongside the
// existing count metrics.
func ReconcileTimeoutDefault() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("ABUZAR_RECONCILE_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Minute
}

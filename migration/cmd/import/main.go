// Command import copies explicitly mapped SQL Server rows into PostgreSQL.
// The source is opened read-only by policy; no UPDATE/DELETE/DDL is issued to
// it. Mapping files are required so unknown legacy schemas cannot be guessed.
package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	Columns           map[string]string     `json:"columns"` // target column -> source column
	PayloadColumns    map[string]string     `json:"payloadColumns,omitempty"`
	PayloadTarget     string                `json:"payloadTarget,omitempty"`
	DerivedColumns    map[string][]string   `json:"derivedColumns,omitempty"`
	GeneratedColumns  map[string]string     `json:"generatedColumns,omitempty"`
	SourceExpressions map[string]string     `json:"sourceExpressions,omitempty"`
	SourceFilter      string                `json:"sourceFilter,omitempty"`
	Lookups           map[string]lookupSpec `json:"lookups,omitempty"`
	Coerce            map[string]string     `json:"coerce,omitempty"`
	Inject            map[string]string     `json:"inject,omitempty"`
	ConflictColumn    []string              `json:"conflictColumns"`
}

type lookupSpec struct {
	Target        tableRef          `json:"target"`
	TargetColumn  string            `json:"targetColumn"`
	ValueColumn   string            `json:"valueColumn,omitempty"`
	SourceColumn  string            `json:"sourceColumn"`
	SourceColumns []string          `json:"sourceColumns,omitempty"`
	Predicates    map[string]string `json:"predicates,omitempty"`
}

type importConfig struct {
	TenantID       string         `json:"tenantId"`
	DefaultBranch  string         `json:"defaultBranchId,omitempty"`
	SourceDatabase string         `json:"sourceDatabase,omitempty"`
	Upsert         bool           `json:"upsert,omitempty"`
	Tables         []tableMapping `json:"tables"`
}

type tableReport struct {
	SourceSchema string `json:"sourceSchema"`
	SourceTable  string `json:"sourceTable"`
	TargetSchema string `json:"targetSchema"`
	TargetTable  string `json:"targetTable"`
	Read         int    `json:"read"`
	Imported     int    `json:"imported"`
	Duplicates   int    `json:"duplicates"`
	Exceptions   int    `json:"exceptions"`
}

type report struct {
	GeneratedAt string        `json:"generatedAt"`
	Source      string        `json:"source"`
	Target      string        `json:"target"`
	TenantID    string        `json:"tenantId"`
	Tables      []tableReport `json:"tables"`
}

func main() {
	configPath := flag.String("config", os.Getenv("ABUZAR_IMPORT_CONFIG"), "JSON mapping configuration")
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL target connection URL")
	out := flag.String("out", filepath.Join("parity", "catalog", "migration-import.json"), "redacted import report")
	fromTable := flag.Int("from-table", 0, "zero-based first mapping table to run")
	toTable := flag.Int("to-table", -1, "exclusive mapping table limit; -1 runs through the end")
	sourceFilter := flag.String("source-filter", "", "optional reviewed predicate override for the selected mapping range")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical FazalDinPP19DataBaseV2 source")
	tenantOverride := flag.String("tenant-id", "", "optional target tenant UUID override for every tenant_id injection")
	branchOverride := flag.String("branch-id", "", "optional target branch UUID override for every branch_id injection")
	counterOverride := flag.String("counter-id", "", "optional target counter UUID override for every counter_id injection")
	promoteNormalized := flag.Bool("promote-normalized", false, "refresh normalized master targets for the imported tenant after the mapping wave")
	upsertOverride := flag.Bool("upsert", false, "explicitly update existing rows on reviewed conflict keys for this run")
	flag.Parse()
	if *configPath == "" || *sourceURL == "" || *targetURL == "" {
		fatal("config, source, and target are required; use protected environment variables or flags")
	}

	config, err := readConfig(*configPath)
	if err != nil {
		fatal(err.Error())
	}
	if err := config.validate(); err != nil {
		fatal(err.Error())
	}
	if err := validateImportSource(*sourceURL, config.SourceDatabase, *allowCanonical); err != nil {
		fatal(err.Error())
	}
	if *allowCanonical && strings.TrimSpace(*tenantOverride) == "" {
		fatal("-tenant-id is required when -allow-canonical is enabled; canonical imports must use a dedicated target tenant")
	}
	if *fromTable < 0 || *fromTable > len(config.Tables) ||
		(*toTable != -1 && (*toTable < *fromTable || *toTable > len(config.Tables))) {
		fatal("mapping table range is outside the reviewed configuration")
	}
	endTable := *toTable
	if endTable == -1 {
		endTable = len(config.Tables)
	}
	if *allowCanonical && hasInjectedScopeInRange(config, "branch_id", *fromTable, endTable) && strings.TrimSpace(*branchOverride) == "" {
		fatal("-branch-id is required for this canonical mapping because the selected range declares branch_id")
	}
	if *allowCanonical && hasInjectedScopeInRange(config, "counter_id", *fromTable, endTable) && strings.TrimSpace(*counterOverride) == "" {
		fatal("-counter-id is required for this canonical mapping because the selected range declares counter_id")
	}
	if err := applyScopeOverrides(&config, *tenantOverride, *branchOverride, *counterOverride); err != nil {
		fatal(err.Error())
	}
	if *upsertOverride {
		config.Upsert = true
	}
	if strings.Contains(*sourceFilter, ";") || strings.Contains(*sourceFilter, "--") ||
		strings.Contains(*sourceFilter, "/*") || strings.Contains(*sourceFilter, "*/") {
		fatal("source-filter must be a single read-only predicate")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	source, err := sql.Open("sqlserver", *sourceURL)
	if err != nil {
		fatal(fmt.Sprintf("open source: %v", err))
	}
	defer source.Close()
	target, err := sql.Open("pgx", *targetURL)
	if err != nil {
		fatal(fmt.Sprintf("open target: %v", err))
	}
	defer target.Close()
	if err := source.PingContext(ctx); err != nil {
		fatal(fmt.Sprintf("source ping failed: %v", err))
	}
	if err := target.PingContext(ctx); err != nil {
		fatal(fmt.Sprintf("target ping failed: %v", err))
	}

	result := report{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: "redacted SQL Server connection", Target: "redacted PostgreSQL connection", TenantID: config.TenantID, Tables: make([]tableReport, 0, endTable-*fromTable)}
	for _, mapping := range config.Tables[*fromTable:endTable] {
		if strings.TrimSpace(*sourceFilter) != "" {
			mapping.SourceFilter = *sourceFilter
		}
		item, err := importTable(ctx, source, target, config, mapping)
		if err != nil {
			fatal(err.Error())
		}
		result.Tables = append(result.Tables, item)
	}
	if *promoteNormalized {
		if err := promoteNormalizedMasters(ctx, target, config.TenantID); err != nil {
			fatal(fmt.Sprintf("promote normalized masters: %v", err))
		}
	}
	if err := writeReport(*out, result); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Imported %d mapped tables for tenant %s; report: %s\n", len(result.Tables), config.TenantID, *out)
}

func readConfig(path string) (importConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return importConfig{}, fmt.Errorf("read mapping config: %w", err)
	}
	var config importConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return importConfig{}, fmt.Errorf("parse mapping config: %w", err)
	}
	return config, nil
}

func (c importConfig) validate() error {
	if strings.TrimSpace(c.TenantID) == "" {
		return errors.New("tenantId is required in the mapping config")
	}
	if len(c.Tables) == 0 {
		return errors.New("mapping config must contain at least one table")
	}
	for index, mapping := range c.Tables {
		if err := mapping.validate(); err != nil {
			return fmt.Errorf("tables[%d]: %w", index, err)
		}
	}
	return nil
}

func (m tableMapping) validate() error {
	for name, ref := range map[string]tableRef{"source": m.Source, "target": m.Target} {
		if strings.TrimSpace(ref.Schema) == "" || strings.TrimSpace(ref.Table) == "" {
			return fmt.Errorf("%s schema and table are required", name)
		}
	}
	if strings.TrimSpace(m.SourceID) == "" && len(m.SourceIDColumns) == 0 {
		return errors.New("sourceId or sourceIdColumns is required")
	}
	if strings.TrimSpace(m.TargetID) == "" {
		return errors.New("sourceId and targetId are required")
	}
	if len(m.SourceIDColumns) > 0 {
		for _, sourceColumn := range m.SourceIDColumns {
			if strings.TrimSpace(sourceColumn) == "" {
				return errors.New("sourceIdColumns cannot contain empty identifiers")
			}
		}
	}
	if len(m.Columns) == 0 {
		return errors.New("columns must map at least one target column")
	}
	if len(m.ConflictColumn) == 0 {
		return errors.New("conflictColumns are required for idempotent import")
	}
	for _, conflictColumn := range m.ConflictColumn {
		if _, mapped := m.Columns[conflictColumn]; !mapped {
			if _, injected := m.Inject[conflictColumn]; !injected {
				if _, derived := m.DerivedColumns[conflictColumn]; !derived {
					if _, generated := m.GeneratedColumns[conflictColumn]; !generated {
						if _, expressed := m.SourceExpressions[conflictColumn]; !expressed {
							if _, lookedUp := m.Lookups[conflictColumn]; !lookedUp {
								return fmt.Errorf("conflict column %q is not mapped or injected", conflictColumn)
							}
						}
					}
				}
			}
		}
	}
	for targetColumn := range m.Inject {
		if strings.TrimSpace(targetColumn) == "" {
			return errors.New("inject contains an empty target column")
		}
		if strings.TrimSpace(m.Inject[targetColumn]) == "" {
			return fmt.Errorf("inject value for %q must not be empty", targetColumn)
		}
	}
	for targetColumn, sourceColumn := range m.Columns {
		if strings.TrimSpace(targetColumn) == "" || strings.TrimSpace(sourceColumn) == "" {
			return errors.New("columns cannot contain empty target or source identifiers")
		}
	}
	for payloadName, sourceColumn := range m.PayloadColumns {
		if strings.TrimSpace(payloadName) == "" || strings.TrimSpace(sourceColumn) == "" {
			return errors.New("payloadColumns cannot contain empty names or source identifiers")
		}
	}
	if strings.TrimSpace(m.PayloadTarget) != "" && strings.TrimSpace(m.PayloadTarget) == "id" {
		return errors.New("payloadTarget cannot replace the target identity")
	}
	for targetColumn, sourceColumns := range m.DerivedColumns {
		if strings.TrimSpace(targetColumn) == "" || len(sourceColumns) == 0 {
			return errors.New("derivedColumns require a target and at least one source column")
		}
		for _, sourceColumn := range sourceColumns {
			if strings.TrimSpace(sourceColumn) == "" {
				return errors.New("derivedColumns cannot contain empty source identifiers")
			}
		}
	}
	for targetColumn, scope := range m.GeneratedColumns {
		if strings.TrimSpace(targetColumn) == "" || strings.TrimSpace(scope) == "" {
			return errors.New("generatedColumns require a target and a scope")
		}
	}
	if strings.Contains(m.SourceFilter, ";") || strings.Contains(m.SourceFilter, "--") ||
		strings.Contains(m.SourceFilter, "/*") || strings.Contains(m.SourceFilter, "*/") {
		return errors.New("sourceFilter must be a single read-only predicate")
	}
	for targetColumn, expression := range m.SourceExpressions {
		if strings.TrimSpace(targetColumn) == "" || strings.TrimSpace(expression) == "" {
			return errors.New("sourceExpressions require a target and expression")
		}
		if strings.Contains(expression, ";") || strings.Contains(expression, "--") ||
			strings.Contains(expression, "/*") || strings.Contains(expression, "*/") {
			return errors.New("sourceExpressions must be single read-only expressions")
		}
	}
	for targetColumn, lookup := range m.Lookups {
		if strings.TrimSpace(targetColumn) == "" ||
			strings.TrimSpace(lookup.Target.Schema) == "" ||
			strings.TrimSpace(lookup.Target.Table) == "" ||
			strings.TrimSpace(lookup.TargetColumn) == "" ||
			(strings.TrimSpace(lookup.SourceColumn) == "" && len(lookup.SourceColumns) == 0) {
			return errors.New("lookups require target, targetColumn, and sourceColumn")
		}
		for _, sourceColumn := range lookup.SourceColumns {
			if strings.TrimSpace(sourceColumn) == "" {
				return errors.New("lookup sourceColumns cannot contain empty identifiers")
			}
		}
		for predicate, value := range lookup.Predicates {
			if strings.TrimSpace(predicate) == "" || strings.TrimSpace(value) == "" {
				return errors.New("lookup predicates cannot contain empty names or values")
			}
		}
	}
	return nil
}

func importTable(ctx context.Context, source, target *sql.DB, config importConfig, mapping tableMapping) (tableReport, error) {
	result := tableReport{SourceSchema: mapping.Source.Schema, SourceTable: mapping.Source.Table, TargetSchema: mapping.Target.Schema, TargetTable: mapping.Target.Table}
	columns := make(map[string]string, len(mapping.Columns)+1)
	for targetColumn, sourceColumn := range mapping.Columns {
		columns[targetColumn] = sourceColumn
	}
	if !mapping.TargetIDGenerated {
		columns[mapping.TargetID] = mapping.SourceID
	}
	targetColumns := make([]string, 0, len(columns)+len(mapping.Inject))
	for targetColumn := range columns {
		targetColumns = append(targetColumns, targetColumn)
	}
	for targetColumn := range mapping.Inject {
		if _, ok := columns[targetColumn]; !ok {
			targetColumns = append(targetColumns, targetColumn)
		}
	}
	for targetColumn := range mapping.GeneratedColumns {
		if !contains(targetColumns, targetColumn) {
			targetColumns = append(targetColumns, targetColumn)
		}
	}
	for targetColumn := range mapping.SourceExpressions {
		if !contains(targetColumns, targetColumn) {
			targetColumns = append(targetColumns, targetColumn)
		}
	}
	for targetColumn := range mapping.DerivedColumns {
		if !contains(targetColumns, targetColumn) {
			targetColumns = append(targetColumns, targetColumn)
		}
	}
	for targetColumn := range mapping.Lookups {
		if !contains(targetColumns, targetColumn) {
			targetColumns = append(targetColumns, targetColumn)
		}
	}
	payloadTarget := mapping.PayloadTarget
	if payloadTarget == "" {
		payloadTarget = "payload"
	}
	if len(mapping.PayloadColumns) > 0 {
		targetColumns = append(targetColumns, payloadTarget)
	}
	sort.Strings(targetColumns)
	sourceColumns := make([]string, 0, len(targetColumns))
	for _, targetColumn := range targetColumns {
		if sourceColumn, ok := columns[targetColumn]; ok {
			sourceColumns = append(sourceColumns, sourceColumn)
		}
	}
	for _, sourceColumn := range mapping.SourceIDColumns {
		sourceColumns = append(sourceColumns, sourceColumn)
	}
	for _, sourceColumn := range mapping.PayloadColumns {
		sourceColumns = append(sourceColumns, sourceColumn)
	}
	for _, sourceColumnsForTarget := range mapping.DerivedColumns {
		sourceColumns = append(sourceColumns, sourceColumnsForTarget...)
	}
	for _, lookup := range mapping.Lookups {
		if strings.TrimSpace(lookup.SourceColumn) != "" {
			sourceColumns = append(sourceColumns, lookup.SourceColumn)
		}
		sourceColumns = append(sourceColumns, lookup.SourceColumns...)
	}
	sourceColumns = uniqueStrings(sourceColumns)
	sort.Strings(sourceColumns)
	selectParts := make([]string, 0, len(sourceColumns)+len(mapping.SourceExpressions))
	for _, sourceColumn := range sourceColumns {
		selectParts = append(selectParts, quoteSQLServer(sourceColumn))
	}
	expressionTargets := make([]string, 0, len(mapping.SourceExpressions))
	for targetColumn := range mapping.SourceExpressions {
		expressionTargets = append(expressionTargets, targetColumn)
	}
	sort.Strings(expressionTargets)
	for _, targetColumn := range expressionTargets {
		selectParts = append(selectParts, mapping.SourceExpressions[targetColumn]+" AS "+quoteSQLServer(targetColumn))
	}
	selectSQL := "SELECT " + strings.Join(selectParts, ", ") + " FROM " + quoteSQLServer(mapping.Source.Schema) + "." + quoteSQLServer(mapping.Source.Table)
	if strings.TrimSpace(mapping.SourceFilter) != "" {
		selectSQL += " WHERE " + mapping.SourceFilter
	}
	rows, err := source.QueryContext(ctx, selectSQL)
	if err != nil {
		return result, fmt.Errorf("read %s.%s: %w", mapping.Source.Schema, mapping.Source.Table, err)
	}
	defer rows.Close()
	sourceColumnNames, err := rows.Columns()
	if err != nil {
		return result, err
	}
	sourceIndex := make(map[string]int, len(sourceColumnNames))
	for index, name := range sourceColumnNames {
		sourceIndex[strings.ToLower(name)] = index
	}

	targetDBTx, err := beginImportTransaction(ctx, target, config)
	if err != nil {
		return result, fmt.Errorf("begin target transaction: %w", err)
	}
	defer targetDBTx.Rollback()

	placeholders := make([]string, len(targetColumns))
	for index := range targetColumns {
		placeholders[index] = "$" + strconv.Itoa(index+1)
	}
	insertColumns := joinQuoted(targetColumns, quotePostgres)
	conflicts := make([]string, len(mapping.ConflictColumn))
	for index, column := range mapping.ConflictColumn {
		conflicts[index] = quotePostgres(column)
	}
	insertSQL := "INSERT INTO " + quotePostgres(mapping.Target.Schema) + "." + quotePostgres(mapping.Target.Table) + " (" + insertColumns + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT (" + strings.Join(conflicts, ", ") + ") DO NOTHING RETURNING " + quotePostgres(mapping.TargetID)
	if config.Upsert {
		updates := make([]string, 0, len(targetColumns))
		for _, column := range targetColumns {
			if column == mapping.TargetID || contains(mapping.ConflictColumn, column) {
				continue
			}
			updates = append(updates, quotePostgres(column)+" = EXCLUDED."+quotePostgres(column))
		}
		if len(updates) > 0 {
			insertSQL = "INSERT INTO " + quotePostgres(mapping.Target.Schema) + "." + quotePostgres(mapping.Target.Table) + " (" + insertColumns + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT (" + strings.Join(conflicts, ", ") + ") DO UPDATE SET " + strings.Join(updates, ", ") + " RETURNING " + quotePostgres(mapping.TargetID)
		}
	}
	lookupCache := make(map[string]any)

	for rows.Next() {
		result.Read++
		values := make([]any, len(sourceColumnNames))
		pointers := make([]any, len(sourceColumnNames))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return result, fmt.Errorf("scan %s.%s row %d: %w", mapping.Source.Schema, mapping.Source.Table, result.Read, err)
		}
		sourceID := sourceIdentity(mapping, sourceIndex, values)
		if sourceID == "" {
			return result, fmt.Errorf("source ID %q was not mapped", mapping.SourceID)
		}
		rowValues := make([]any, len(targetColumns))
		for index, targetColumn := range targetColumns {
			if injected, ok := mapping.Inject[targetColumn]; ok {
				rowValues[index] = injected
				continue
			}
			if scope, ok := mapping.GeneratedColumns[targetColumn]; ok {
				rowValues[index] = stableUUID(config.TenantID, mapping, sourceID, scope)
				continue
			}
			if derived, ok := mapping.DerivedColumns[targetColumn]; ok {
				parts := make([]string, 0, len(derived))
				for _, sourceColumn := range derived {
					valueIndex, found := sourceIndex[strings.ToLower(sourceColumn)]
					if !found {
						return result, fmt.Errorf("source column %q was not returned by SQL Server", sourceColumn)
					}
					parts = append(parts, fmt.Sprint(normalizeValue(values[valueIndex])))
				}
				rowValues[index] = strings.Join(parts, ":")
				continue
			}
			if lookup, ok := mapping.Lookups[targetColumn]; ok {
				lookupValue := lookupSourceValue(lookup, sourceIndex, values)
				if lookupValue == "" {
					return result, fmt.Errorf("lookup source column %q was not returned by SQL Server", lookup.SourceColumn)
				}
				var lookupErr error
				rowValues[index], lookupErr = resolveLookup(ctx, targetDBTx, config.TenantID, lookup, lookupValue, lookupCache)
				if lookupErr != nil {
					return result, fmt.Errorf("resolve %s for %s.%s row %d: %w", targetColumn, mapping.Source.Schema, mapping.Source.Table, result.Read, lookupErr)
				}
				continue
			}
			if targetColumn == payloadTarget && len(mapping.PayloadColumns) > 0 {
				payload := make(map[string]any, len(mapping.PayloadColumns))
				for payloadName, sourceColumn := range mapping.PayloadColumns {
					valueIndex, ok := sourceIndex[strings.ToLower(sourceColumn)]
					if !ok {
						return result, fmt.Errorf("source column %q was not returned by SQL Server", sourceColumn)
					}
					payload[payloadName] = normalizeValue(values[valueIndex])
				}
				rowValues[index], err = json.Marshal(payload)
				if err != nil {
					return result, fmt.Errorf("encode payload for %s.%s row %d: %w", mapping.Source.Schema, mapping.Source.Table, result.Read, err)
				}
				continue
			}
			if expression, ok := mapping.SourceExpressions[targetColumn]; ok {
				valueIndex, found := sourceIndex[strings.ToLower(targetColumn)]
				if !found {
					return result, fmt.Errorf("source expression %q was not returned by SQL Server", expression)
				}
				rowValues[index] = normalizeValue(values[valueIndex])
			} else {
				sourceColumn := columns[targetColumn]
				valueIndex, ok := sourceIndex[strings.ToLower(sourceColumn)]
				if !ok {
					return result, fmt.Errorf("source column %q was not returned by SQL Server", sourceColumn)
				}
				rowValues[index] = normalizeValue(values[valueIndex])
			}
			if targetTextColumn(targetColumn) {
				rowValues[index] = fmt.Sprint(rowValues[index])
			}
			if coercion := mapping.Coerce[targetColumn]; coercion != "" {
				var err error
				rowValues[index], err = coerceValue(rowValues[index], coercion)
				if err != nil {
					return result, fmt.Errorf("coerce %s.%s column %q row %d: %w", mapping.Source.Schema, mapping.Source.Table, targetColumn, result.Read, err)
				}
			}
			if targetColumn == mapping.TargetID {
				// The source identity is retained separately from a generated
				// target UUID. It is used for legacy_id_mappings below.
			}
		}

		if _, err := targetDBTx.ExecContext(ctx, "SAVEPOINT import_row"); err != nil {
			return result, err
		}
		var targetID any
		duplicate := false
		err := targetDBTx.QueryRowContext(ctx, insertSQL, rowValues...).Scan(&targetID)
		if isNoRows(err) {
			duplicate = true
			result.Duplicates++
			lookupSQL := "SELECT " + quotePostgres(mapping.TargetID) + " FROM " + quotePostgres(mapping.Target.Schema) + "." + quotePostgres(mapping.Target.Table) + " WHERE " + equalityPredicate(mapping.ConflictColumn) + " LIMIT 1"
			lookupArgs := conflictValues(mapping, targetColumns, rowValues)
			lookupErr := error(nil)
			if lookupArgs != nil {
				lookupErr = targetDBTx.QueryRowContext(ctx, lookupSQL, lookupArgs...).Scan(&targetID)
			}
			if lookupArgs == nil || lookupErr != nil {
				err = errors.New("duplicate row could not be resolved")
			} else {
				err = nil
			}
		}
		if err != nil {
			_, _ = targetDBTx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT import_row")
			_, _ = targetDBTx.ExecContext(ctx, "RELEASE SAVEPOINT import_row")
			result.Exceptions++
			if exceptionErr := recordException(ctx, targetDBTx, config, mapping, sourceID, err); exceptionErr != nil {
				return result, exceptionErr
			}
			continue
		}
		if _, err := targetDBTx.ExecContext(ctx, "RELEASE SAVEPOINT import_row"); err != nil {
			return result, err
		}
		if err := recordMapping(ctx, targetDBTx, config, mapping, sourceID, targetID); err != nil {
			return result, err
		}
		if err := resolveExceptions(ctx, targetDBTx, config, mapping, sourceID); err != nil {
			return result, err
		}
		if !duplicate {
			result.Imported++
		}
		// A large legacy table can exceed PostgreSQL's shared-memory lock
		// budget when every row remains in one transaction. Committing small
		// idempotent batches preserves restart safety and keeps the source
		// cursor read-only.
		if result.Read%50 == 0 {
			if err := targetDBTx.Commit(); err != nil {
				return result, fmt.Errorf("commit import batch for %s.%s: %w", mapping.Target.Schema, mapping.Target.Table, err)
			}
			targetDBTx, err = beginImportTransaction(ctx, target, config)
			if err != nil {
				return result, fmt.Errorf("begin import batch for %s.%s: %w", mapping.Target.Schema, mapping.Target.Table, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if err := targetDBTx.Commit(); err != nil {
		return result, fmt.Errorf("commit %s.%s: %w", mapping.Target.Schema, mapping.Target.Table, err)
	}
	return result, nil
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no rows")
}

func beginImportTransaction(ctx context.Context, target *sql.DB, config importConfig) (*sql.Tx, error) {
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range map[string]string{
		"app.tenant_id":          config.TenantID,
		"app.branch_id":          config.DefaultBranch,
		"app.allow_tenant_scope": "true",
	} {
		if _, err := tx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, key, value); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	return tx, nil
}

func promoteNormalizedMasters(ctx context.Context, target *sql.DB, tenantID string) error {
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range map[string]string{
		"app.authenticating":     "true",
		"app.tenant_id":          tenantID,
		"app.allow_tenant_scope": "true",
	} {
		if _, err := tx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, key, value); err != nil {
			return err
		}
	}
	statements := []string{
		`INSERT INTO master_items (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
		 SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
		 FROM master_records WHERE tenant_id = $1::uuid AND kind = 'item'
		 ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at`,
		`INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name, payload, active, created_at, updated_at)
		 SELECT tenant_id, kind, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
		 FROM master_records WHERE tenant_id = $1::uuid AND kind IN ('customer', 'supplier')
		 ON CONFLICT (tenant_id, party_type, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at`,
		`INSERT INTO master_manufacturers (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
		 SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
		 FROM master_records WHERE tenant_id = $1::uuid AND kind = 'manufacturer'
		 ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at`,
		`INSERT INTO master_categories (tenant_id, category_kind, legacy_id, code, name, payload, active, created_at, updated_at)
		 SELECT tenant_id, kind, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
		 FROM master_records WHERE tenant_id = $1::uuid AND kind IN ('category', 'item_category', 'customer_category', 'supplier_category', 'manufacturer_category')
		 ON CONFLICT (tenant_id, category_kind, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at`,
		`INSERT INTO master_godowns (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
		 SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
		 FROM master_records WHERE tenant_id = $1::uuid AND kind = 'godown'
		 ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at`,
		`INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
		 SELECT i.tenant_id, i.id, 'legacy_id', i.legacy_id, lower(i.legacy_id)
		 FROM master_items i WHERE i.tenant_id = $1::uuid
		 ON CONFLICT (tenant_id, alias_kind, normalized_value) DO NOTHING`,
		`INSERT INTO item_suppliers (tenant_id, item_id, supplier_id, legacy_item_id, legacy_supplier_id, priority, rate, discount_percent, quantity, bonus, days, payload, created_at, updated_at)
		 SELECT l.tenant_id, i.id, s.id, l.legacy_item_id, l.legacy_supplier_id, l.priority, l.rate, l.discount_percent, l.sale_quantity, l.bonus_quantity, l.scheme_days, l.payload, l.created_at, l.updated_at
		 FROM item_supplier_links l
		 LEFT JOIN master_items i ON i.tenant_id=l.tenant_id AND i.legacy_id=l.legacy_item_id
		 LEFT JOIN master_parties s ON s.tenant_id=l.tenant_id AND s.party_type='supplier' AND s.legacy_id=l.legacy_supplier_id
		 WHERE l.tenant_id = $1::uuid
		 ON CONFLICT (tenant_id, legacy_item_id, legacy_supplier_id) DO UPDATE SET item_id=EXCLUDED.item_id, supplier_id=EXCLUDED.supplier_id, priority=EXCLUDED.priority, rate=EXCLUDED.rate, discount_percent=EXCLUDED.discount_percent, quantity=EXCLUDED.quantity, bonus=EXCLUDED.bonus, days=EXCLUDED.days, payload=EXCLUDED.payload, updated_at=EXCLUDED.updated_at`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, tenantID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func recordMapping(ctx context.Context, tx *sql.Tx, config importConfig, mapping tableMapping, sourceID, targetID any) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO legacy_id_mappings (tenant_id, source_system, source_schema, source_table, legacy_id, target_table, target_id, status, note)
		VALUES ($1::uuid, 'sqlserver', $2, $3, $4, $5, $6, 'mapped', 'declarative import')
		ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
		DO NOTHING
	`, config.TenantID, mapping.Source.Schema, mapping.Source.Table, fmt.Sprint(sourceID), mapping.Target.Schema+"."+mapping.Target.Table, fmt.Sprint(targetID))
	return err
}

func recordException(ctx context.Context, tx *sql.Tx, config importConfig, mapping tableMapping, sourceID any, cause error) error {
	details, _ := json.Marshal(map[string]string{"error": cause.Error(), "sourceId": fmt.Sprint(sourceID)})
	_, err := tx.ExecContext(ctx, `
		INSERT INTO migration_exceptions (tenant_id, source_schema, source_table, legacy_id, reason_code, details)
		VALUES ($1::uuid, $2, $3, $4, 'row_import_failed', $5::jsonb)
	`, config.TenantID, mapping.Source.Schema, mapping.Source.Table, fmt.Sprint(sourceID), details)
	return err
}

func resolveExceptions(ctx context.Context, tx *sql.Tx, config importConfig, mapping tableMapping, sourceID any) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE migration_exceptions
		SET status = 'resolved', resolved_at = now()
		WHERE tenant_id = $1::uuid
		  AND source_schema = $2
		  AND source_table = $3
		  AND legacy_id = $4
		  AND status = 'open'
	`, config.TenantID, mapping.Source.Schema, mapping.Source.Table, fmt.Sprint(sourceID))
	return err
}

func conflictValues(mapping tableMapping, columns []string, values []any) []any {
	result := make([]any, len(mapping.ConflictColumn))
	for index, conflictColumn := range mapping.ConflictColumn {
		for valueIndex, column := range columns {
			if column == conflictColumn {
				result[index] = values[valueIndex]
				break
			}
		}
		if result[index] == nil {
			return nil
		}
	}
	return result
}

func equalityPredicate(columns []string) string {
	predicates := make([]string, len(columns))
	for index, column := range columns {
		predicates[index] = quotePostgres(column) + " = $" + strconv.Itoa(index+1)
	}
	return strings.Join(predicates, " AND ")
}

func normalizeValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func resolveLookup(ctx context.Context, tx *sql.Tx, tenantID string, lookup lookupSpec, value any, cache map[string]any) (any, error) {
	lookupValue := strings.TrimSpace(fmt.Sprintf("%v", value))
	if lookupValue == "" || strings.EqualFold(lookupValue, "<nil>") {
		return nil, errors.New("lookup value is empty")
	}
	cacheKey := lookupCacheKey(tenantID, lookup, lookupValue)
	if cache != nil {
		if cached, ok := cache[cacheKey]; ok {
			return cached, nil
		}
	}
	valueColumn := lookup.ValueColumn
	if valueColumn == "" {
		valueColumn = "id"
	}
	args := []any{tenantID, lookupValue}
	query := "SELECT " + quotePostgres(valueColumn) + " FROM " + quotePostgres(lookup.Target.Schema) + "." + quotePostgres(lookup.Target.Table) +
		" WHERE \"tenant_id\" = $1 AND " + quotePostgres(lookup.TargetColumn) + " = $2"
	predicateNames := make([]string, 0, len(lookup.Predicates))
	for predicate := range lookup.Predicates {
		predicateNames = append(predicateNames, predicate)
	}
	sort.Strings(predicateNames)
	for _, predicate := range predicateNames {
		args = append(args, lookup.Predicates[predicate])
		query += " AND " + quotePostgres(predicate) + " = $" + strconv.Itoa(len(args))
	}
	query += " LIMIT 1"
	var id any
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		return nil, err
	}
	if cache != nil {
		cache[cacheKey] = id
	}
	return id, nil
}

func lookupCacheKey(tenantID string, lookup lookupSpec, value string) string {
	predicateNames := make([]string, 0, len(lookup.Predicates))
	for predicate := range lookup.Predicates {
		predicateNames = append(predicateNames, predicate)
	}
	sort.Strings(predicateNames)
	parts := []string{tenantID, lookup.Target.Schema, lookup.Target.Table, lookup.TargetColumn, lookup.ValueColumn, value}
	for _, predicate := range predicateNames {
		parts = append(parts, predicate, lookup.Predicates[predicate])
	}
	return strings.Join(parts, "\x00")
}

func coerceValue(value any, kind string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "boolean":
		switch raw := value.(type) {
		case bool:
			return raw, nil
		case int64:
			return raw != 0, nil
		case int32:
			return raw != 0, nil
		case int:
			return raw != 0, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(raw)) {
			case "1", "true", "t", "yes", "y":
				return true, nil
			case "0", "false", "f", "no", "n":
				return false, nil
			}
		}
		return nil, fmt.Errorf("cannot convert %v to boolean", value)
	case "text":
		return fmt.Sprint(normalizeValue(value)), nil
	default:
		return nil, fmt.Errorf("unsupported coercion %q", kind)
	}
}

func targetTextColumn(column string) bool {
	switch column {
	case "legacy_id", "code", "name", "legacy_item_id", "legacy_supplier_id",
		"document_number", "source_document_number", "source_line_key", "batch_number",
		"item_code", "item_name", "reference", "remarks", "description", "idempotency_key",
		"source_legacy_id", "document_code", "document_type", "account_code",
		"alternate_account_code", "user_legacy_id", "invoice_code", "legacy_import_key":
		return true
	case "legacy_group_id", "legacy_user_id", "legacy_group_code", "legacy_user_code", "legacy_scope_id",
		"right_code", "legacy_status", "legacy_table", "scope_key":
		return true
	default:
		return false
	}
}

func lookupSourceValue(lookup lookupSpec, sourceIndex map[string]int, values []any) string {
	columns := lookup.SourceColumns
	if len(columns) == 0 {
		columns = []string{lookup.SourceColumn}
	}
	parts := make([]string, 0, len(columns))
	for _, sourceColumn := range columns {
		valueIndex, found := sourceIndex[strings.ToLower(sourceColumn)]
		if !found {
			return ""
		}
		parts = append(parts, fmt.Sprint(normalizeValue(values[valueIndex])))
	}
	return strings.Join(parts, ":")
}

func sourceIdentity(mapping tableMapping, sourceIndex map[string]int, values []any) string {
	columns := mapping.SourceIDColumns
	if len(columns) == 0 {
		columns = []string{mapping.SourceID}
	}
	parts := make([]string, 0, len(columns))
	for _, sourceColumn := range columns {
		valueIndex, found := sourceIndex[strings.ToLower(sourceColumn)]
		if !found {
			return ""
		}
		parts = append(parts, fmt.Sprint(normalizeValue(values[valueIndex])))
	}
	return strings.Join(parts, ":")
}

// stableUUID creates a UUIDv5-like value without adding a dependency to the
// migration workbench. The tenant is the namespace, and the reviewed mapping
// plus source identity form the name. It makes generated IDs restart-safe and
// keeps IDs from different source tables distinct.
func stableUUID(tenantID string, mapping tableMapping, sourceID, scope string) string {
	namespace := make([]byte, 16)
	if parsed, err := parseUUID(tenantID); err == nil {
		copy(namespace, parsed)
	}
	name := mapping.Source.Schema + "." + mapping.Source.Table + ":" + scope + ":" + sourceID
	hash := sha1.Sum(append(namespace, []byte(name)...))
	hash[6] = (hash[6] & 0x0f) | 0x50
	hash[8] = (hash[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}

func parseUUID(value string) ([]byte, error) {
	hexValue := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	if len(hexValue) != 32 {
		return nil, errors.New("invalid UUID namespace")
	}
	result := make([]byte, 16)
	for index := range result {
		parsed, err := strconv.ParseUint(hexValue[index*2:index*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		result[index] = byte(parsed)
	}
	return result, nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func joinQuoted(values []string, quote func(string) string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = quote(value)
	}
	return strings.Join(quoted, ", ")
}

func quoteSQLServer(identifier string) string {
	return "[" + strings.ReplaceAll(identifier, "]", "]]") + "]"
}

func quotePostgres(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func writeReport(path string, value report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

func validateImportSource(rawURL, expectedDatabase string, allowCanonical bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse source URL: %w", err)
	}
	databaseName := parsed.Query().Get("database")
	if strings.TrimSpace(databaseName) == "" {
		return errors.New("source URL must name a database; imports require the reviewed sandbox database")
	}
	if strings.EqualFold(databaseName, "FazalDinPP19DataBaseV2") {
		if !allowCanonical {
			return errors.New("refusing import from canonical FazalDinPP19DataBaseV2; pass -allow-canonical with a dedicated -tenant-id after reviewing the map")
		}
		return nil
	}
	if strings.TrimSpace(expectedDatabase) != "" && !strings.EqualFold(databaseName, expectedDatabase) {
		return fmt.Errorf("source database %q does not match reviewed mapping database", databaseName)
	}
	return nil
}

func applyScopeOverrides(config *importConfig, tenantID, branchID, counterID string) error {
	tenantID = strings.TrimSpace(tenantID)
	branchID = strings.TrimSpace(branchID)
	counterID = strings.TrimSpace(counterID)
	if tenantID == "" && branchID == "" && counterID == "" {
		return nil
	}
	if tenantID != "" {
		config.TenantID = tenantID
	}
	if branchID != "" {
		config.DefaultBranch = branchID
	}
	for index := range config.Tables {
		inject := config.Tables[index].Inject
		if tenantID != "" {
			if inject == nil {
				inject = make(map[string]string)
			}
			if _, present := inject["tenant_id"]; present {
				inject["tenant_id"] = tenantID
			}
		}
		if branchID != "" {
			if inject == nil {
				inject = make(map[string]string)
			}
			if _, present := inject["branch_id"]; present {
				inject["branch_id"] = branchID
			}
		}
		if counterID != "" {
			if inject == nil {
				inject = make(map[string]string)
			}
			if _, present := inject["counter_id"]; present {
				inject["counter_id"] = counterID
			}
		}
		config.Tables[index].Inject = inject
	}
	return nil
}

func hasInjectedScope(config importConfig, key string) bool {
	for _, mapping := range config.Tables {
		if _, present := mapping.Inject[key]; present {
			return true
		}
	}
	return false
}

func hasInjectedScopeInRange(config importConfig, key string, from, to int) bool {
	if from < 0 {
		from = 0
	}
	if to > len(config.Tables) {
		to = len(config.Tables)
	}
	if from > to {
		return false
	}
	for _, mapping := range config.Tables[from:to] {
		if _, present := mapping.Inject[key]; present {
			return true
		}
	}
	return false
}

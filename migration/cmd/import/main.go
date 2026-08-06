// Command import copies explicitly mapped SQL Server rows into PostgreSQL.
// The source is opened read-only by policy; no UPDATE/DELETE/DDL is issued to
// it. Mapping files are required so unknown legacy schemas cannot be guessed.
package main

import (
	"context"
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
	Lookups           map[string]lookupSpec `json:"lookups,omitempty"`
	Coerce            map[string]string     `json:"coerce,omitempty"`
	Inject            map[string]string     `json:"inject,omitempty"`
	ConflictColumn    []string              `json:"conflictColumns"`
}

type lookupSpec struct {
	Target       tableRef `json:"target"`
	TargetColumn string   `json:"targetColumn"`
	SourceColumn string   `json:"sourceColumn"`
}

type importConfig struct {
	TenantID       string         `json:"tenantId"`
	DefaultBranch  string         `json:"defaultBranchId,omitempty"`
	SourceDatabase string         `json:"sourceDatabase,omitempty"`
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
	if err := validateImportSource(*sourceURL, config.SourceDatabase); err != nil {
		fatal(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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

	result := report{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: "redacted SQL Server connection", Target: "redacted PostgreSQL connection", TenantID: config.TenantID, Tables: make([]tableReport, 0, len(config.Tables))}
	for _, mapping := range config.Tables {
		item, err := importTable(ctx, source, target, config, mapping)
		if err != nil {
			fatal(err.Error())
		}
		result.Tables = append(result.Tables, item)
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
					return fmt.Errorf("conflict column %q is not mapped or injected", conflictColumn)
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
	for targetColumn, lookup := range m.Lookups {
		if strings.TrimSpace(targetColumn) == "" ||
			strings.TrimSpace(lookup.Target.Schema) == "" ||
			strings.TrimSpace(lookup.Target.Table) == "" ||
			strings.TrimSpace(lookup.TargetColumn) == "" ||
			strings.TrimSpace(lookup.SourceColumn) == "" {
			return errors.New("lookups require target, targetColumn, and sourceColumn")
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
		sourceColumns = append(sourceColumns, lookup.SourceColumn)
	}
	sourceColumns = uniqueStrings(sourceColumns)
	sort.Strings(sourceColumns)
	selectSQL := "SELECT " + joinQuoted(sourceColumns, quoteSQLServer) + " FROM " + quoteSQLServer(mapping.Source.Schema) + "." + quoteSQLServer(mapping.Source.Table)
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

	targetDBTx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin target transaction: %w", err)
	}
	defer targetDBTx.Rollback()
	for key, value := range map[string]string{"app.tenant_id": config.TenantID, "app.branch_id": config.DefaultBranch, "app.allow_tenant_scope": "true"} {
		if _, err := targetDBTx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, key, value); err != nil {
			return result, fmt.Errorf("set target scope: %w", err)
		}
	}

	placeholders := make([]string, len(targetColumns))
	for index := range targetColumns {
		placeholders[index] = "$" + strconv.Itoa(index+1)
	}
	insertColumns := joinQuoted(targetColumns, quotePostgres)
	conflicts := make([]string, len(mapping.ConflictColumn))
	for index, column := range mapping.ConflictColumn {
		conflicts[index] = quotePostgres(column)
	}
	updates := make([]string, 0, len(targetColumns))
	for _, column := range targetColumns {
		if column == mapping.TargetID || contains(mapping.ConflictColumn, column) {
			continue
		}
		updates = append(updates, quotePostgres(column)+" = EXCLUDED."+quotePostgres(column))
	}
	insertSQL := "INSERT INTO " + quotePostgres(mapping.Target.Schema) + "." + quotePostgres(mapping.Target.Table) + " (" + insertColumns + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT (" + strings.Join(conflicts, ", ") + ") DO UPDATE SET " + strings.Join(updates, ", ") + " RETURNING " + quotePostgres(mapping.TargetID)
	if len(updates) == 0 {
		insertSQL = "INSERT INTO " + quotePostgres(mapping.Target.Schema) + "." + quotePostgres(mapping.Target.Table) + " (" + insertColumns + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT (" + strings.Join(conflicts, ", ") + ") DO NOTHING RETURNING " + quotePostgres(mapping.TargetID)
	}

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
		rowValues := make([]any, len(targetColumns))
		var sourceID any
		for index, targetColumn := range targetColumns {
			if injected, ok := mapping.Inject[targetColumn]; ok {
				rowValues[index] = injected
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
				valueIndex, found := sourceIndex[strings.ToLower(lookup.SourceColumn)]
				if !found {
					return result, fmt.Errorf("lookup source column %q was not returned by SQL Server", lookup.SourceColumn)
				}
				var lookupErr error
				rowValues[index], lookupErr = resolveLookup(ctx, targetDBTx, config.TenantID, lookup, normalizeValue(values[valueIndex]))
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
			sourceColumn := columns[targetColumn]
			valueIndex, ok := sourceIndex[strings.ToLower(sourceColumn)]
			if !ok {
				return result, fmt.Errorf("source column %q was not returned by SQL Server", sourceColumn)
			}
			rowValues[index] = normalizeValue(values[valueIndex])
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
				sourceID = rowValues[index]
			}
		}
		if len(mapping.SourceIDColumns) > 0 {
			parts := make([]string, 0, len(mapping.SourceIDColumns))
			for _, sourceColumn := range mapping.SourceIDColumns {
				valueIndex, found := sourceIndex[strings.ToLower(sourceColumn)]
				if !found {
					return result, fmt.Errorf("source ID column %q was not returned by SQL Server", sourceColumn)
				}
				parts = append(parts, fmt.Sprint(normalizeValue(values[valueIndex])))
			}
			sourceID = strings.Join(parts, ":")
		} else if sourceID == nil {
			sourceColumn := mapping.SourceID
			if mapping.TargetIDGenerated {
				sourceColumn = mapping.SourceID
			} else if mapped, ok := columns[mapping.TargetID]; ok {
				sourceColumn = mapped
			}
			if valueIndex, found := sourceIndex[strings.ToLower(sourceColumn)]; found {
				sourceID = normalizeValue(values[valueIndex])
			}
		}
		if sourceID == nil {
			return result, fmt.Errorf("source ID %q was not mapped", mapping.SourceID)
		}

		if _, err := targetDBTx.ExecContext(ctx, "SAVEPOINT import_row"); err != nil {
			return result, err
		}
		var targetID any
		duplicate := false
		err := targetDBTx.QueryRowContext(ctx, insertSQL, rowValues...).Scan(&targetID)
		if errors.Is(err, sql.ErrNoRows) {
			duplicate = true
			result.Duplicates++
			lookupSQL := "SELECT " + quotePostgres(mapping.TargetID) + " FROM " + quotePostgres(mapping.Target.Schema) + "." + quotePostgres(mapping.Target.Table) + " WHERE " + equalityPredicate(mapping.ConflictColumn) + " LIMIT 1"
			lookupArgs := conflictValues(mapping, targetColumns, rowValues)
			if lookupArgs == nil || targetDBTx.QueryRowContext(ctx, lookupSQL, lookupArgs...).Scan(&targetID) != nil {
				err = errors.New("duplicate row could not be resolved")
			}
		}
		if err != nil {
			_, _ = targetDBTx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT import_row")
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
		if !duplicate {
			result.Imported++
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

func recordMapping(ctx context.Context, tx *sql.Tx, config importConfig, mapping tableMapping, sourceID, targetID any) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO legacy_id_mappings (tenant_id, source_system, source_schema, source_table, legacy_id, target_table, target_id, status, note)
		VALUES ($1::uuid, 'sqlserver', $2, $3, $4, $5, $6, 'mapped', 'declarative import')
		ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
		DO UPDATE SET target_table = EXCLUDED.target_table, target_id = EXCLUDED.target_id, status = 'mapped', note = EXCLUDED.note
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

func resolveLookup(ctx context.Context, tx *sql.Tx, tenantID string, lookup lookupSpec, value any) (any, error) {
	lookupValue, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprintf("%v", value)), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("lookup value is not a reviewed numeric legacy ID: %w", err)
	}
	query := "SELECT \"id\" FROM " + quotePostgres(lookup.Target.Schema) + "." + quotePostgres(lookup.Target.Table) +
		" WHERE \"tenant_id\" = $1 AND " + quotePostgres(lookup.TargetColumn) +
		" = '" + strconv.FormatInt(lookupValue, 10) + "' LIMIT 1"
	var id any
	if err := tx.QueryRowContext(ctx, query, tenantID).Scan(&id); err != nil {
		return nil, err
	}
	return id, nil
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
	default:
		return nil, fmt.Errorf("unsupported coercion %q", kind)
	}
}

func targetTextColumn(column string) bool {
	switch column {
	case "legacy_id", "code", "name", "legacy_item_id", "legacy_supplier_id":
		return true
	case "legacy_group_id", "legacy_user_id", "legacy_group_code", "legacy_user_code", "legacy_scope_id",
		"right_code", "legacy_status", "legacy_table", "scope_key":
		return true
	default:
		return false
	}
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

func validateImportSource(rawURL, expectedDatabase string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse source URL: %w", err)
	}
	databaseName := parsed.Query().Get("database")
	if strings.TrimSpace(databaseName) == "" {
		return errors.New("source URL must name a database; imports require the reviewed sandbox database")
	}
	if strings.EqualFold(databaseName, "FazalDinPP19DataBaseV2") {
		return errors.New("refusing import from canonical FazalDinPP19DataBaseV2; use AbuzarLegacyReference")
	}
	if strings.TrimSpace(expectedDatabase) != "" && !strings.EqualFold(databaseName, expectedDatabase) {
		return fmt.Errorf("source database %q does not match reviewed mapping database", databaseName)
	}
	return nil
}

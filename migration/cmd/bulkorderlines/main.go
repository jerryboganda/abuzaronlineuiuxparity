// Command bulkorderlines imports the reviewed canonical PurOrderDetail slice
// with PostgreSQL COPY and set-based dependency joins. SQL Server remains
// read-only; every target row is scoped, idempotent, and auditable.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jackc/pgx/v5"
)

const (
	canonicalDatabase = "FazalDinPP19DataBaseV2"
	canonicalTenant   = "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01"
	canonicalBranch   = "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02"
	targetTable       = "business_document_lines"
)

type tableReport struct {
	SourceSchema     string         `json:"sourceSchema"`
	SourceTable      string         `json:"sourceTable"`
	TargetSchema     string         `json:"targetSchema"`
	TargetTable      string         `json:"targetTable"`
	Read             int            `json:"read"`
	Imported         int            `json:"imported"`
	Duplicates       int            `json:"duplicates"`
	Exceptions       int            `json:"exceptions"`
	ExceptionReasons map[string]int `json:"exceptionReasons,omitempty"`
	ExceptionSamples []string       `json:"exceptionSamples,omitempty"`
}

type report struct {
	GeneratedAt string        `json:"generatedAt"`
	Source      string        `json:"source"`
	Target      string        `json:"target"`
	TenantID    string        `json:"tenantId"`
	Tables      []tableReport `json:"tables"`
}

type sourceRow struct {
	LegacyID     string
	LegacyKey    string
	OrderID      string
	RowID        int
	ItemLegacyID string
	ItemCode     string
	Quantity     string
	UnitPrice    string
	LineGross    string
	LineTotal    string
	UnitCost     string
	GSTRate      string
	BatchNumber  string
	ExpiryDate   *string
	Payload      []byte
}

type exceptionRow struct {
	LegacyID string
	Reason   string
	Details  []byte
}

// Keep this projection aligned with migration/maps/phase-e-historical-orders.json.
// Qty is retained at source precision before the target's declared numeric scale.
const sourceQuery = `
SELECT
  CONVERT(varchar(100), d.POCode) AS po_code,
  CONVERT(varchar(30), d.PORowId) AS po_row_id,
  CONVERT(varchar(100), d.ICode) AS item_legacy_id,
  q.quantity,
  CAST(COALESCE(d.Rate, 0) AS decimal(19,4)) AS unit_price,
  CAST(CAST(COALESCE(d.Rate, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS line_gross,
  CAST(CAST(COALESCE(d.Rate, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS line_total,
  CAST(COALESCE(d.PurPrice, 0) AS decimal(19,4)) AS unit_cost,
  CAST(COALESCE(d.GSTPerc, 0) AS decimal(9,4)) AS gst_rate,
  CONVERT(varchar(200), d.Batch) AS batch_number,
  CONVERT(varchar(10), d.Expiry, 23) AS expiry_date,
  d.DiscPerc,
  d.BonusQty,
  d.Stock,
  d.ReceiptQty,
  d.Remarks
FROM dbo.PurOrderDetail AS d
CROSS APPLY (
  SELECT CAST(COALESCE(d.Qty, 0) AS decimal(19,8)) AS quantity
) AS q`

func main() {
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL target connection URL")
	tenant := flag.String("tenant-id", "", "dedicated target tenant UUID")
	branch := flag.String("branch-id", canonicalBranch, "dedicated target branch UUID")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical source")
	out := flag.String("out", filepath.Join("parity", "catalog", "canonical-first-tenant-purchase-order-lines-import.json"), "report path")
	flag.Parse()
	if strings.TrimSpace(*sourceURL) == "" || strings.TrimSpace(*targetURL) == "" {
		fatal("source and target are required")
	}
	if !*allowCanonical {
		fatal("-allow-canonical is required for the protected canonical source")
	}
	if strings.TrimSpace(*tenant) != canonicalTenant {
		fatal("this bounded command only accepts the provisioned canonical tenant")
	}
	if strings.TrimSpace(*branch) != canonicalBranch {
		fatal("this bounded command only accepts the provisioned canonical branch")
	}
	if !strings.Contains(strings.ToLower(*sourceURL), "database="+strings.ToLower(canonicalDatabase)) {
		fatal("source URL must name the canonical database")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	source, err := sql.Open("sqlserver", *sourceURL)
	if err != nil {
		fatal(fmt.Sprintf("open source: %v", err))
	}
	defer source.Close()
	if err := source.PingContext(ctx); err != nil {
		fatal(fmt.Sprintf("source ping failed: %v", err))
	}
	target, err := pgx.Connect(ctx, *targetURL)
	if err != nil {
		fatal(fmt.Sprintf("open target: %v", err))
	}
	defer target.Close(ctx)

	rows, duplicates, invalid, err := readRows(ctx, source)
	if err != nil {
		fatal(err.Error())
	}
	result := tableReport{
		SourceSchema:     "dbo",
		SourceTable:      "PurOrderDetail",
		TargetSchema:     "public",
		TargetTable:      targetTable,
		Read:             len(rows) + duplicates + len(invalid),
		Duplicates:       duplicates,
		ExceptionReasons: make(map[string]int),
	}
	for _, item := range invalid {
		result.ExceptionReasons[item.Reason]++
		appendExceptionSample(&result, item.LegacyID+": "+item.Reason)
	}

	tx, err := target.Begin(ctx)
	if err != nil {
		fatal(fmt.Sprintf("begin target transaction: %v", err))
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true), set_config('app.branch_id', $2, true), set_config('app.allow_tenant_scope', 'true', true)`, *tenant, *branch); err != nil {
		fatal(fmt.Sprintf("set target scope: %v", err))
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE purchase_order_line_stage (
		order_id text NOT NULL,
		row_id integer NOT NULL,
		legacy_id text NOT NULL,
		legacy_import_key text NOT NULL,
		item_legacy_id text NOT NULL,
		item_code text NOT NULL,
		quantity numeric(19,8) NOT NULL,
		unit_price numeric(19,4) NOT NULL,
		line_gross numeric(19,4) NOT NULL,
		line_total numeric(19,4) NOT NULL,
		unit_cost numeric(19,4) NOT NULL,
		gst_rate numeric(9,4) NOT NULL,
		batch_number text NOT NULL,
		expiry_date date,
		payload jsonb NOT NULL
	) ON COMMIT DROP`); err != nil {
		fatal(fmt.Sprintf("create purchase-order-line staging table: %v", err))
	}
	copyRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		copyRows = append(copyRows, []any{
			row.OrderID, row.RowID, row.LegacyID, row.LegacyKey,
			row.ItemLegacyID, row.ItemCode, row.Quantity, row.UnitPrice,
			row.LineGross, row.LineTotal, row.UnitCost, row.GSTRate,
			row.BatchNumber, nullableString(row.ExpiryDate), row.Payload,
		})
	}
	if len(copyRows) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"purchase_order_line_stage"}, []string{
			"order_id", "row_id", "legacy_id", "legacy_import_key", "item_legacy_id", "item_code",
			"quantity", "unit_price", "line_gross", "line_total", "unit_cost", "gst_rate",
			"batch_number", "expiry_date", "payload",
		}, pgx.CopyFromRows(copyRows)); err != nil {
			fatal(fmt.Sprintf("copy purchase-order-line staging rows: %v", err))
		}
	}

	missing, err := dependencyExceptions(ctx, tx, *tenant, *branch)
	if err != nil {
		fatal(fmt.Sprintf("check purchase-order-line dependencies: %v", err))
	}
	for _, item := range missing {
		result.ExceptionReasons[item.Reason]++
		appendExceptionSample(&result, item.LegacyID+": "+item.Reason)
	}
	result.Exceptions = len(invalid) + len(missing)
	if len(invalid) > 0 {
		if err := writeExceptions(ctx, tx, *tenant, invalid); err != nil {
			fatal(fmt.Sprintf("record invalid purchase-order-line mappings: %v", err))
		}
	}
	if len(missing) > 0 {
		if err := writeExceptions(ctx, tx, *tenant, missing); err != nil {
			fatal(fmt.Sprintf("record dependency exceptions: %v", err))
		}
	}

	var before int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM business_document_lines l JOIN purchase_order_line_stage s ON s.legacy_import_key = l.legacy_import_key WHERE l.tenant_id = $1 AND l.branch_id = $2`, *tenant, *branch).Scan(&before); err != nil {
		fatal(fmt.Sprintf("count existing purchase-order lines: %v", err))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO business_document_lines (
		tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id, item_code, item_name,
		quantity, unit_price, line_gross, line_total, batch_number, expiry_date, unit_cost,
		gst_rate, legacy_source_table, legacy_id, legacy_payload, legacy_import_key, notes
	)
	SELECT $1, $2, d.id, s.row_id, i.id, s.item_legacy_id, s.item_code, i.name,
		s.quantity, s.unit_price, s.line_gross, s.line_total, s.batch_number, s.expiry_date, s.unit_cost,
		s.gst_rate, 'PurOrderDetail', s.legacy_id, s.payload, s.legacy_import_key,
		COALESCE(s.payload->>'Remarks', '')
	FROM purchase_order_line_stage s
	JOIN business_documents d
	  ON d.tenant_id = $1 AND d.branch_id = $2 AND d.kind = 'purchase-order'
	 AND d.legacy_source_table = 'PurOrderHeader' AND d.legacy_id = s.order_id
	JOIN master_items i
	  ON i.tenant_id = $1 AND i.legacy_id = s.item_legacy_id
	ON CONFLICT (tenant_id, branch_id, legacy_import_key) DO UPDATE SET
		document_id = EXCLUDED.document_id,
		line_number = EXCLUDED.line_number,
		item_id = EXCLUDED.item_id,
		item_legacy_id = EXCLUDED.item_legacy_id,
		item_code = EXCLUDED.item_code,
		item_name = EXCLUDED.item_name,
		quantity = EXCLUDED.quantity,
		unit_price = EXCLUDED.unit_price,
		line_gross = EXCLUDED.line_gross,
		line_total = EXCLUDED.line_total,
		batch_number = EXCLUDED.batch_number,
		expiry_date = EXCLUDED.expiry_date,
		unit_cost = EXCLUDED.unit_cost,
		gst_rate = EXCLUDED.gst_rate,
		legacy_source_table = EXCLUDED.legacy_source_table,
		legacy_id = EXCLUDED.legacy_id,
		legacy_payload = EXCLUDED.legacy_payload,
		notes = EXCLUDED.notes`, *tenant, *branch); err != nil {
		fatal(fmt.Sprintf("upsert purchase-order lines: %v", err))
	}
	var after int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM business_document_lines l JOIN purchase_order_line_stage s ON s.legacy_import_key = l.legacy_import_key WHERE l.tenant_id = $1 AND l.branch_id = $2`, *tenant, *branch).Scan(&after); err != nil {
		fatal(fmt.Sprintf("count imported purchase-order lines: %v", err))
	}
	result.Imported = after - before
	result.Duplicates += before
	if err := writeMappedMappings(ctx, tx, *tenant); err != nil {
		fatal(fmt.Sprintf("record purchase-order-line mappings: %v", err))
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(fmt.Sprintf("commit purchase-order lines: %v", err))
	}
	writeReport(*out, report{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: "redacted SQL Server connection", Target: "redacted PostgreSQL connection", TenantID: *tenant, Tables: []tableReport{result}})
	fmt.Printf("Bulk processed %d purchase-order lines for tenant %s; imported %d, exceptions %d; report: %s\n", result.Read, *tenant, result.Imported, result.Exceptions, *out)
}

func readRows(ctx context.Context, source *sql.DB) ([]sourceRow, int, []exceptionRow, error) {
	rows, err := source.QueryContext(ctx, sourceQuery)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("read dbo.PurOrderDetail: %w", err)
	}
	defer rows.Close()
	result := make([]sourceRow, 0, 114000)
	seen := make(map[string]struct{}, 114000)
	duplicates := 0
	exceptions := make([]exceptionRow, 0)
	for rows.Next() {
		values := make([]any, 16)
		pointers := make([]any, len(values))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, 0, nil, fmt.Errorf("scan PurOrderDetail row %d: %w", len(result)+len(exceptions)+duplicates+1, err)
		}
		orderID := normalizeText(values[0])
		rowText := normalizeText(values[1])
		itemID := normalizeText(values[2])
		legacyID := orderID + ":" + rowText
		legacyKey := "PurOrderDetail:" + legacyID
		if _, ok := seen[legacyKey]; ok {
			duplicates++
			continue
		}
		seen[legacyKey] = struct{}{}
		rowID, parseErr := strconv.Atoi(rowText)
		quantity := normalizeText(values[3])
		reason := ""
		if orderID == "" {
			reason = "missing_purchase_order_id"
		} else if parseErr != nil || rowID <= 0 {
			reason = "invalid_line_number"
		} else if !positiveDecimal(quantity) {
			reason = "non_positive_quantity"
		} else if itemID == "" {
			reason = "missing_item_id"
		}
		if reason != "" {
			details, err := orderLineExceptionDetails(values, legacyID, reason)
			if err != nil {
				return nil, 0, nil, fmt.Errorf("encode PurOrderDetail exception %s: %w", legacyID, err)
			}
			exceptions = append(exceptions, exceptionRow{LegacyID: legacyID, Reason: reason, Details: details})
			continue
		}
		payload, err := json.Marshal(map[string]any{
			"POCode":     normalizeValue(values[0]),
			"ICode":      normalizeValue(values[2]),
			"Batch":      normalizeValue(values[9]),
			"Expiry":     normalizeValue(values[10]),
			"Qty":        normalizeValue(values[3]),
			"Rate":       normalizeValue(values[4]),
			"DiscPerc":   normalizeValue(values[11]),
			"BonusQty":   normalizeValue(values[12]),
			"Stock":      normalizeValue(values[13]),
			"ReceiptQty": normalizeValue(values[14]),
			"Remarks":    normalizeValue(values[15]),
			"PORowId":    normalizeValue(values[1]),
		})
		if err != nil {
			return nil, 0, nil, fmt.Errorf("encode PurOrderDetail row %s: %w", legacyID, err)
		}
		result = append(result, sourceRow{
			LegacyID: legacyID, LegacyKey: legacyKey, OrderID: orderID, RowID: rowID,
			ItemLegacyID: itemID, ItemCode: itemID, Quantity: quantity,
			UnitPrice: normalizeText(values[4]), LineGross: normalizeText(values[5]),
			LineTotal: normalizeText(values[6]), UnitCost: normalizeText(values[7]),
			GSTRate: normalizeText(values[8]), BatchNumber: normalizeText(values[9]),
			ExpiryDate: normalizeDate(values[10]), Payload: payload,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("read PurOrderDetail rows: %w", err)
	}
	return result, duplicates, exceptions, nil
}

func dependencyExceptions(ctx context.Context, tx pgx.Tx, tenant, branch string) ([]exceptionRow, error) {
	rows, err := tx.Query(ctx, `SELECT s.legacy_id,
		CASE WHEN d.id IS NULL THEN 'missing_document' ELSE 'missing_item' END AS reason
		FROM purchase_order_line_stage s
		LEFT JOIN business_documents d ON d.tenant_id = $1 AND d.branch_id = $2 AND d.kind = 'purchase-order' AND d.legacy_source_table = 'PurOrderHeader' AND d.legacy_id = s.order_id
		LEFT JOIN master_items i ON i.tenant_id = $1 AND i.legacy_id = s.item_legacy_id
		WHERE d.id IS NULL OR i.id IS NULL`, tenant, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]exceptionRow, 0)
	for rows.Next() {
		var item exceptionRow
		if err := rows.Scan(&item.LegacyID, &item.Reason); err != nil {
			return nil, err
		}
		item.Details = genericExceptionDetails(item.LegacyID, item.Reason)
		result = append(result, item)
	}
	return result, rows.Err()
}

func writeExceptions(ctx context.Context, tx pgx.Tx, tenant string, items []exceptionRow) error {
	if len(items) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS purchase_order_line_exception_stage (
		legacy_id text NOT NULL,
		reason text NOT NULL,
		details jsonb NOT NULL
	) ON COMMIT DROP`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `TRUNCATE purchase_order_line_exception_stage`); err != nil {
		return err
	}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		details := item.Details
		if len(details) == 0 {
			details = genericExceptionDetails(item.LegacyID, item.Reason)
		}
		rows = append(rows, []any{item.LegacyID, item.Reason, details})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"purchase_order_line_exception_stage"}, []string{"legacy_id", "reason", "details"}, pgx.CopyFromRows(rows)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO legacy_id_mappings (
		tenant_id, source_system, source_schema, source_table, legacy_id, target_table, target_id, status, note
	)
	SELECT $1, 'sqlserver', 'dbo', 'PurOrderDetail', legacy_id, $2, NULL, 'exception', reason
	FROM purchase_order_line_exception_stage
	ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
		DO UPDATE SET target_table = EXCLUDED.target_table, target_id = NULL, status = 'exception', note = EXCLUDED.note`, tenant, targetTable); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM migration_exceptions
		WHERE tenant_id = $1 AND source_schema = 'dbo' AND source_table = 'PurOrderDetail'
		  AND legacy_id IN (SELECT legacy_id FROM purchase_order_line_exception_stage)`, tenant); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO migration_exceptions (
			tenant_id, source_schema, source_table, legacy_id, reason_code, details, status
		)
		SELECT $1, 'dbo', 'PurOrderDetail', legacy_id, reason,
			details, 'open'
		FROM purchase_order_line_exception_stage`, tenant)
	return err
}

func writeMappedMappings(ctx context.Context, tx pgx.Tx, tenant string) error {
	_, err := tx.Exec(ctx, `INSERT INTO legacy_id_mappings (
		tenant_id, source_system, source_schema, source_table, legacy_id, target_table, target_id, status, note
	)
	SELECT $1, 'sqlserver', 'dbo', 'PurOrderDetail', s.legacy_id, $2, l.id::text, 'mapped', NULL
	FROM purchase_order_line_stage s
	JOIN business_document_lines l ON l.tenant_id = $1 AND l.legacy_import_key = s.legacy_import_key
	ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
	DO UPDATE SET target_id = EXCLUDED.target_id, target_table = EXCLUDED.target_table, status = 'mapped', note = NULL`, tenant, targetTable)
	return err
}

func positiveDecimal(text string) bool {
	value, ok := new(big.Rat).SetString(strings.TrimSpace(text))
	return ok && value.Sign() > 0
}

func orderLineExceptionDetails(values []any, legacyID, reason string) ([]byte, error) {
	if len(values) < 16 {
		return nil, fmt.Errorf("PurOrderDetail row has %d values; want at least 16", len(values))
	}
	return json.Marshal(map[string]any{
		"legacy_id": legacyID, "reason": reason,
		"POCode": normalizeValue(values[0]), "PORowId": normalizeValue(values[1]),
		"ICode": normalizeValue(values[2]), "quantity": normalizeValue(values[3]),
		"Rate": normalizeValue(values[4]), "Batch": normalizeValue(values[9]),
		"Expiry": normalizeValue(values[10]), "DiscPerc": normalizeValue(values[11]),
		"BonusQty": normalizeValue(values[12]), "Stock": normalizeValue(values[13]),
		"ReceiptQty": normalizeValue(values[14]), "Remarks": normalizeValue(values[15]),
	})
}

func genericExceptionDetails(legacyID, reason string) []byte {
	details, _ := json.Marshal(map[string]string{"reason": reason, "legacy_id": legacyID})
	return details
}

func normalizeText(value any) string {
	return strings.TrimSpace(fmt.Sprint(normalizeValue(value)))
}

func normalizeValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func normalizeDate(value any) *string {
	text := strings.TrimSpace(fmt.Sprint(normalizeValue(value)))
	if text == "" || text == "<nil>" {
		return nil
	}
	if parsed, err := time.Parse("2006-01-02", text); err == nil {
		result := parsed.Format("2006-01-02")
		return &result
	}
	if parsed, err := time.Parse(time.RFC3339, text); err == nil {
		result := parsed.UTC().Format("2006-01-02")
		return &result
	}
	return &text
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func appendExceptionSample(result *tableReport, sample string) {
	if len(result.ExceptionSamples) < 20 {
		result.ExceptionSamples = append(result.ExceptionSamples, sample)
	}
}

func writeReport(path string, value report) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fatal(fmt.Sprintf("create report directory: %v", err))
	}
	file, err := os.Create(path)
	if err != nil {
		fatal(fmt.Sprintf("create report: %v", err))
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatal(fmt.Sprintf("write report: %v", err))
	}
}

func fatal(message string) {
	if strings.TrimSpace(message) == "" {
		message = errors.New("unknown error").Error()
	}
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

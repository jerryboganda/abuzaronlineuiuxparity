// Command bulkreturnlines imports the reviewed SRdetail or PRdetail slice.
// The mode is fixed to a reviewed source table; SQL Server is read-only and
// target writes are scoped, idempotent, and auditable.
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

type returnMode struct {
	name          string
	sourceTable   string
	headerTable   string
	documentKind  string
	sourceQuery   string
	headerIDLabel string
	itemIDLabel   string
	rowIDLabel    string
	priceLabel    string
}

var returnModes = map[string]returnMode{
	"sale": {
		name:          "sale",
		sourceTable:   "SRdetail",
		headerTable:   "SRLedger",
		documentKind:  "cash-sale-return",
		headerIDLabel: "SRInvcode",
		itemIDLabel:   "Icode",
		rowIDLabel:    "RowId",
		priceLabel:    "SRPrice",
		sourceQuery:   saleReturnSourceQuery,
	},
	"purchase": {
		name:          "purchase",
		sourceTable:   "PRdetail",
		headerTable:   "PRLedger",
		documentKind:  "purchase-return",
		headerIDLabel: "PRInvCode",
		itemIDLabel:   "ICode",
		rowIDLabel:    "PrRowId",
		priceLabel:    "PRPrice",
		sourceQuery:   purchaseReturnSourceQuery,
	},
}

type tableReport struct {
	SourceSchema     string         `json:"sourceSchema"`
	SourceTable      string         `json:"sourceTable"`
	TargetSchema     string         `json:"targetSchema"`
	TargetTable      string         `json:"targetTable"`
	FromRow          int            `json:"fromRow"`
	ToRow            int            `json:"toRow"`
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
	LegacyID       string
	LegacyKey      string
	HeaderID       string
	RowID          int
	ItemLegacyID   string
	ItemCode       string
	Quantity       string
	UnitPrice      string
	LineGross      string
	LineTotal      string
	UnitCost       string
	GSTRate        string
	AdvanceTaxRate string
	TaxAmount      string
	BatchNumber    string
	ExpiryDate     *string
	Payload        []byte
}

type exceptionRow struct {
	LegacyID string
	Reason   string
	Details  []byte
}

// Both queries expose the same 22-column scan contract. The expressions mirror
// phase-e-historical-documents.json; mode-specific fields occupy columns 20/21.
const saleReturnSourceQuery = `
SELECT
  CONVERT(varchar(100), d.SRInvcode) AS return_id,
  CONVERT(varchar(30), d.RowId) AS row_id_text,
  CONVERT(varchar(100), d.Icode) AS item_legacy_id,
  q.quantity AS quantity,
  CAST(COALESCE(d.SRPrice, 0) AS decimal(19,4)) AS unit_price,
  CAST(CAST(COALESCE(d.SRPrice, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS line_gross,
  CAST(CAST(COALESCE(d.SRPrice, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS line_total,
  CAST(COALESCE(d.AvgPrice, 0) AS decimal(19,4)) AS unit_cost,
  CAST(COALESCE(d.GSTPerc, 0) AS decimal(9,4)) AS gst_rate,
  CAST(COALESCE(d.ItemAdvanceTaxPerc, 0) AS decimal(9,4)) AS advance_tax_rate,
  CAST(CAST(COALESCE(d.UnitSalesTax, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS tax_amount,
  CONVERT(varchar(200), d.Batch) AS batch_number,
  CONVERT(varchar(10), d.Expiry, 23) AS expiry_date,
  d.Gcode AS gcode,
  d.PackQty AS pack_qty,
  d.LooseQty AS loose_qty,
  d.PackUnits AS pack_units,
  d.SRPrice AS source_price,
  d.DiscPerc AS discount_percent,
  d.SalesTax AS sales_tax,
  d.RowId AS row_id_source,
  d.SaleRowId AS sale_row_id
FROM dbo.SRdetail AS d
CROSS APPLY (
  SELECT CAST(COALESCE(d.PackQty, 0) AS decimal(19,8)) +
    CASE
      WHEN COALESCE(d.PackUnits, 0) = 0 THEN CAST(0 AS decimal(19,8))
      ELSE CAST(COALESCE(d.LooseQty, 0) AS decimal(19,8)) /
        NULLIF(CAST(d.PackUnits AS decimal(19,8)), 0)
    END AS quantity
) AS q`

const purchaseReturnSourceQuery = `
SELECT
  CONVERT(varchar(100), d.PRInvCode) AS return_id,
  CONVERT(varchar(30), d.PrRowId) AS row_id_text,
  CONVERT(varchar(100), d.ICode) AS item_legacy_id,
  q.quantity AS quantity,
  CAST(COALESCE(d.PRPrice, 0) AS decimal(19,4)) AS unit_price,
  CAST(CAST(COALESCE(d.PRPrice, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS line_gross,
  CAST(CAST(COALESCE(d.PRPrice, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS line_total,
  CAST(COALESCE(d.AvgPrice, 0) AS decimal(19,4)) AS unit_cost,
  CAST(COALESCE(d.GSTPerc, 0) AS decimal(9,4)) AS gst_rate,
  CAST(0 AS decimal(9,4)) AS advance_tax_rate,
  CAST(CAST(COALESCE(d.UnitSalesTax, 0) AS decimal(19,4)) * q.quantity AS decimal(19,4)) AS tax_amount,
  CONVERT(varchar(200), d.Batch) AS batch_number,
  CONVERT(varchar(10), d.Expiry, 23) AS expiry_date,
  d.Gcode AS gcode,
  d.PackQty AS pack_qty,
  d.LooseQty AS loose_qty,
  d.PackUnits AS pack_units,
  d.PRPrice AS source_price,
  d.DiscPerc AS discount_percent,
  d.UnitSalesTax AS unit_sales_tax,
  d.PrRowId AS row_id_source,
  d.HistoricalBatch AS historical_batch
FROM dbo.PRdetail AS d
CROSS APPLY (
  SELECT CAST(COALESCE(d.PackQty, 0) AS decimal(19,8)) +
    CASE
      WHEN COALESCE(d.PackUnits, 0) = 0 THEN CAST(0 AS decimal(19,8))
      ELSE CAST(COALESCE(d.LooseQty, 0) AS decimal(19,8)) /
        NULLIF(CAST(d.PackUnits AS decimal(19,8)), 0)
    END AS quantity
) AS q`

func main() {
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL target connection URL")
	kind := flag.String("kind", "sale", "return source mode: sale (SRdetail) or purchase (PRdetail)")
	tenant := flag.String("tenant-id", "", "dedicated target tenant UUID")
	branch := flag.String("branch-id", canonicalBranch, "dedicated target branch UUID")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical source")
	fromRow := flag.Int("from-row", 0, "zero-based source row offset after stable ordering")
	toRow := flag.Int("to-row", -1, "exclusive source row offset; -1 reads through the end")
	out := flag.String("out", "", "report path; defaults to a mode-specific parity report")
	flag.Parse()
	mode, ok := returnModes[strings.ToLower(strings.TrimSpace(*kind))]
	if !ok {
		fatal("kind must be sale or purchase")
	}
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
	if *fromRow < 0 || (*toRow != -1 && *toRow <= *fromRow) {
		fatal("source row window must satisfy from-row >= 0 and to-row > from-row, or to-row=-1")
	}
	outPath := strings.TrimSpace(*out)
	if outPath == "" {
		outPath = filepath.Join("parity", "catalog", fmt.Sprintf("canonical-first-tenant-%s-return-lines-import.json", mode.name))
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

	rows, duplicates, invalid, err := readRows(ctx, source, mode, *fromRow, *toRow)
	if err != nil {
		fatal(err.Error())
	}
	result := tableReport{
		SourceSchema:     "dbo",
		SourceTable:      mode.sourceTable,
		TargetSchema:     "public",
		TargetTable:      targetTable,
		FromRow:          *fromRow,
		ToRow:            *toRow,
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
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE return_line_stage (
		header_id text NOT NULL,
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
		advance_tax_rate numeric(9,4) NOT NULL,
		tax_amount numeric(19,4) NOT NULL,
		batch_number text NOT NULL,
		expiry_date date,
		payload jsonb NOT NULL
	) ON COMMIT DROP`); err != nil {
		fatal(fmt.Sprintf("create return-line staging table: %v", err))
	}
	copyRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		copyRows = append(copyRows, []any{
			row.HeaderID, row.RowID, row.LegacyID, row.LegacyKey,
			row.ItemLegacyID, row.ItemCode, row.Quantity, row.UnitPrice,
			row.LineGross, row.LineTotal, row.UnitCost, row.GSTRate,
			row.AdvanceTaxRate, row.TaxAmount, row.BatchNumber, nullableString(row.ExpiryDate), row.Payload,
		})
	}
	if len(copyRows) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"return_line_stage"}, []string{
			"header_id", "row_id", "legacy_id", "legacy_import_key", "item_legacy_id", "item_code",
			"quantity", "unit_price", "line_gross", "line_total", "unit_cost", "gst_rate",
			"advance_tax_rate", "tax_amount", "batch_number", "expiry_date", "payload",
		}, pgx.CopyFromRows(copyRows)); err != nil {
			fatal(fmt.Sprintf("copy return-line staging rows: %v", err))
		}
	}

	missing, err := dependencyExceptions(ctx, tx, mode, *tenant, *branch)
	if err != nil {
		fatal(fmt.Sprintf("check return-line dependencies: %v", err))
	}
	for _, item := range missing {
		result.ExceptionReasons[item.Reason]++
		appendExceptionSample(&result, item.LegacyID+": "+item.Reason)
	}
	result.Exceptions = len(invalid) + len(missing)
	if len(invalid) > 0 {
		if err := writeExceptions(ctx, tx, mode, *tenant, invalid); err != nil {
			fatal(fmt.Sprintf("record invalid return-line mappings: %v", err))
		}
	}
	if len(missing) > 0 {
		if err := writeExceptions(ctx, tx, mode, *tenant, missing); err != nil {
			fatal(fmt.Sprintf("record return-line dependency exceptions: %v", err))
		}
	}

	var before int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM business_document_lines l JOIN return_line_stage s ON s.legacy_import_key = l.legacy_import_key WHERE l.tenant_id = $1 AND l.branch_id = $2`, *tenant, *branch).Scan(&before); err != nil {
		fatal(fmt.Sprintf("count existing return lines: %v", err))
	}
	insertSQL := fmt.Sprintf(`INSERT INTO business_document_lines (
		tenant_id, branch_id, document_id, line_number, item_id, item_legacy_id, item_code, item_name,
		quantity, unit_price, line_gross, line_total, batch_number, expiry_date, unit_cost,
		gst_rate, advance_tax_rate, tax_amount, legacy_source_table, legacy_id, legacy_payload, legacy_import_key
	)
	SELECT $1, $2, d.id, s.row_id, i.id, s.item_legacy_id, s.item_code, i.name,
		s.quantity, s.unit_price, s.line_gross, s.line_total, s.batch_number, s.expiry_date, s.unit_cost,
		s.gst_rate, s.advance_tax_rate, s.tax_amount, '%s', s.legacy_id, s.payload, s.legacy_import_key
	FROM return_line_stage s
	JOIN business_documents d
	  ON d.tenant_id = $1 AND d.branch_id = $2 AND d.kind = '%s'
	 AND d.legacy_source_table = '%s' AND d.legacy_id = s.header_id
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
		advance_tax_rate = EXCLUDED.advance_tax_rate,
		tax_amount = EXCLUDED.tax_amount,
		legacy_source_table = EXCLUDED.legacy_source_table,
		legacy_id = EXCLUDED.legacy_id,
		legacy_payload = EXCLUDED.legacy_payload`, mode.sourceTable, mode.documentKind, mode.headerTable)
	if _, err := tx.Exec(ctx, insertSQL, *tenant, *branch); err != nil {
		fatal(fmt.Sprintf("upsert return lines: %v", err))
	}
	var after int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM business_document_lines l JOIN return_line_stage s ON s.legacy_import_key = l.legacy_import_key WHERE l.tenant_id = $1 AND l.branch_id = $2`, *tenant, *branch).Scan(&after); err != nil {
		fatal(fmt.Sprintf("count imported return lines: %v", err))
	}
	result.Imported = after - before
	result.Duplicates += before
	if err := writeMappedMappings(ctx, tx, mode, *tenant); err != nil {
		fatal(fmt.Sprintf("record return-line mappings: %v", err))
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(fmt.Sprintf("commit return lines: %v", err))
	}
	writeReport(outPath, report{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: "redacted SQL Server connection", Target: "redacted PostgreSQL connection", TenantID: *tenant, Tables: []tableReport{result}})
	fmt.Printf("Bulk processed %s return rows %d-%s for tenant %s; read %d, imported %d, exceptions %d; report: %s\n", mode.name, *fromRow, rowWindowEnd(*toRow), *tenant, result.Read, result.Imported, result.Exceptions, outPath)
}

func readRows(ctx context.Context, source *sql.DB, mode returnMode, fromRow, toRow int) ([]sourceRow, int, []exceptionRow, error) {
	query, args, err := sourceRowsQuery(mode, fromRow, toRow)
	if err != nil {
		return nil, 0, nil, err
	}
	rows, err := source.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("read dbo.%s: %w", mode.sourceTable, err)
	}
	defer rows.Close()
	result := make([]sourceRow, 0, 45000)
	seen := make(map[string]struct{}, 45000)
	duplicates := 0
	exceptions := make([]exceptionRow, 0)
	for rows.Next() {
		values := make([]any, 22)
		pointers := make([]any, len(values))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, 0, nil, fmt.Errorf("scan %s row %d: %w", mode.sourceTable, len(result)+len(exceptions)+duplicates+1, err)
		}
		headerID := normalizeText(values[0])
		rowText := normalizeText(values[1])
		itemID := normalizeText(values[2])
		legacyID := headerID + ":" + rowText
		legacyKey := mode.sourceTable + ":" + legacyID
		if _, ok := seen[legacyKey]; ok {
			duplicates++
			continue
		}
		seen[legacyKey] = struct{}{}
		rowID, parseErr := strconv.Atoi(rowText)
		quantity := normalizeText(values[3])
		reason := ""
		if headerID == "" {
			reason = "missing_return_id"
		} else if parseErr != nil || rowID <= 0 {
			reason = "invalid_line_number"
		} else if !positiveDecimal(quantity) {
			reason = "non_positive_quantity"
		} else if itemID == "" {
			reason = "missing_item_id"
		}
		if reason != "" {
			details, err := returnLineExceptionDetails(values, mode, legacyID, reason)
			if err != nil {
				return nil, 0, nil, fmt.Errorf("encode %s exception %s: %w", mode.sourceTable, legacyID, err)
			}
			exceptions = append(exceptions, exceptionRow{LegacyID: legacyID, Reason: reason, Details: details})
			continue
		}
		payload, err := returnPayload(values, mode)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("encode %s row %s: %w", mode.sourceTable, legacyID, err)
		}
		result = append(result, sourceRow{
			LegacyID: legacyID, LegacyKey: legacyKey, HeaderID: headerID, RowID: rowID,
			ItemLegacyID: itemID, ItemCode: itemID, Quantity: quantity,
			UnitPrice: normalizeText(values[4]), LineGross: normalizeText(values[5]),
			LineTotal: normalizeText(values[6]), UnitCost: normalizeText(values[7]),
			GSTRate: normalizeText(values[8]), AdvanceTaxRate: normalizeText(values[9]),
			TaxAmount: normalizeText(values[10]), BatchNumber: normalizeText(values[11]),
			ExpiryDate: normalizeDate(values[12]), Payload: payload,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("read %s rows: %w", mode.sourceTable, err)
	}
	return result, duplicates, exceptions, nil
}

func sourceRowsQuery(mode returnMode, fromRow, toRow int) (string, []any, error) {
	if fromRow < 0 || (toRow != -1 && toRow <= fromRow) {
		return "", nil, errors.New("source row window must satisfy from-row >= 0 and to-row > from-row, or to-row=-1")
	}
	if fromRow == 0 && toRow == -1 {
		return mode.sourceQuery, nil, nil
	}
	query := `SELECT * FROM (
` + mode.sourceQuery + `
) AS source_rows
ORDER BY return_id,
         TRY_CONVERT(bigint, row_id_text),
         row_id_text,
         item_legacy_id
OFFSET ? ROWS`
	args := []any{fromRow}
	if toRow != -1 {
		query += "\nFETCH NEXT ? ROWS ONLY"
		args = append(args, toRow-fromRow)
	}
	return query, args, nil
}

func rowWindowEnd(toRow int) string {
	if toRow == -1 {
		return "end"
	}
	return strconv.Itoa(toRow)
}

func returnPayload(values []any, mode returnMode) ([]byte, error) {
	payload := map[string]any{
		mode.headerIDLabel: normalizeValue(values[0]),
		mode.itemIDLabel:   normalizeValue(values[2]),
		"Gcode":            normalizeValue(values[13]),
		"PackQty":          normalizeValue(values[14]),
		"LooseQty":         normalizeValue(values[15]),
		"PackUnits":        normalizeValue(values[16]),
		mode.priceLabel:    normalizeValue(values[17]),
		"DiscPerc":         normalizeValue(values[18]),
		"Batch":            normalizeValue(values[11]),
		"Expiry":           normalizeValue(values[12]),
	}
	payload[mode.rowIDLabel] = normalizeValue(values[20])
	if mode.name == "sale" {
		payload["SalesTax"] = normalizeValue(values[19])
		payload["SaleRowId"] = normalizeValue(values[21])
	} else {
		payload["UnitSalesTax"] = normalizeValue(values[19])
		payload["HistoricalBatch"] = normalizeValue(values[21])
	}
	return json.Marshal(payload)
}

func dependencyExceptions(ctx context.Context, tx pgx.Tx, mode returnMode, tenant, branch string) ([]exceptionRow, error) {
	query := fmt.Sprintf(`SELECT s.legacy_id,
		CASE WHEN d.id IS NULL THEN 'missing_document' ELSE 'missing_item' END AS reason
		FROM return_line_stage s
		LEFT JOIN business_documents d ON d.tenant_id = $1 AND d.branch_id = $2 AND d.kind = '%s' AND d.legacy_source_table = '%s' AND d.legacy_id = s.header_id
		LEFT JOIN master_items i ON i.tenant_id = $1 AND i.legacy_id = s.item_legacy_id
		WHERE d.id IS NULL OR i.id IS NULL`, mode.documentKind, mode.headerTable)
	rows, err := tx.Query(ctx, query, tenant, branch)
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

func writeExceptions(ctx context.Context, tx pgx.Tx, mode returnMode, tenant string, items []exceptionRow) error {
	if len(items) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS return_line_exception_stage (
		legacy_id text NOT NULL,
		reason text NOT NULL,
		details jsonb NOT NULL
	) ON COMMIT DROP`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `TRUNCATE return_line_exception_stage`); err != nil {
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
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"return_line_exception_stage"}, []string{"legacy_id", "reason", "details"}, pgx.CopyFromRows(rows)); err != nil {
		return err
	}
	writeMappingSQL := fmt.Sprintf(`INSERT INTO legacy_id_mappings (
		tenant_id, source_system, source_schema, source_table, legacy_id, target_table, target_id, status, note
	)
	SELECT $1, 'sqlserver', 'dbo', '%s', legacy_id, $2, NULL, 'exception', reason
	FROM return_line_exception_stage
	ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
		DO UPDATE SET target_table = EXCLUDED.target_table, target_id = NULL, status = 'exception', note = EXCLUDED.note`, mode.sourceTable)
	if _, err := tx.Exec(ctx, writeMappingSQL, tenant, targetTable); err != nil {
		return err
	}
	clearSQL := fmt.Sprintf(`DELETE FROM migration_exceptions
		WHERE tenant_id = $1 AND source_schema = 'dbo' AND source_table = '%s'
		  AND legacy_id IN (SELECT legacy_id FROM return_line_exception_stage)`, mode.sourceTable)
	if _, err := tx.Exec(ctx, clearSQL, tenant); err != nil {
		return err
	}
	exceptionSQL := fmt.Sprintf(`INSERT INTO migration_exceptions (
			tenant_id, source_schema, source_table, legacy_id, reason_code, details, status
		)
		SELECT $1, 'dbo', '%s', legacy_id, reason, details, 'open'
		FROM return_line_exception_stage`, mode.sourceTable)
	_, err := tx.Exec(ctx, exceptionSQL, tenant)
	return err
}

func writeMappedMappings(ctx context.Context, tx pgx.Tx, mode returnMode, tenant string) error {
	query := fmt.Sprintf(`INSERT INTO legacy_id_mappings (
		tenant_id, source_system, source_schema, source_table, legacy_id, target_table, target_id, status, note
	)
	SELECT $1, 'sqlserver', 'dbo', '%s', s.legacy_id, $2, l.id::text, 'mapped', NULL
	FROM return_line_stage s
	JOIN business_document_lines l ON l.tenant_id = $1 AND l.legacy_import_key = s.legacy_import_key
	ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
	DO UPDATE SET target_id = EXCLUDED.target_id, target_table = EXCLUDED.target_table, status = 'mapped', note = NULL`, mode.sourceTable)
	_, err := tx.Exec(ctx, query, tenant, targetTable)
	return err
}

func positiveDecimal(text string) bool {
	value, ok := new(big.Rat).SetString(strings.TrimSpace(text))
	return ok && value.Sign() > 0
}

func returnLineExceptionDetails(values []any, mode returnMode, legacyID, reason string) ([]byte, error) {
	if len(values) < 22 {
		return nil, fmt.Errorf("%s row has %d values; want at least 22", mode.sourceTable, len(values))
	}
	details := map[string]any{
		"legacy_id": legacyID, "reason": reason,
		mode.headerIDLabel: normalizeValue(values[0]), mode.rowIDLabel: normalizeValue(values[1]),
		mode.itemIDLabel: normalizeValue(values[2]), "quantity": normalizeValue(values[3]),
		mode.priceLabel: normalizeValue(values[17]), "Batch": normalizeValue(values[11]),
		"Expiry": normalizeValue(values[12]), "DiscPerc": normalizeValue(values[18]),
		"PackQty": normalizeValue(values[14]), "LooseQty": normalizeValue(values[15]),
		"PackUnits": normalizeValue(values[16]),
	}
	if mode.name == "sale" {
		details["SaleRowId"] = normalizeValue(values[21])
	} else {
		details["HistoricalBatch"] = normalizeValue(values[21])
	}
	return json.Marshal(details)
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

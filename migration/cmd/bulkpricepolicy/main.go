// Command bulkpricepolicy imports the reviewed canonical PricePolicyDetail
// slice with PostgreSQL COPY. It is intentionally narrow: the source query is
// read-only, the canonical database requires explicit opt-in, and the target
// scope is supplied explicitly. The generic importer remains the default for
// all other maps.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	targetTable       = "price_policy_tiers"
)

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

type sourceRow struct {
	LegacyID        string
	LegacyPolicyID  string
	QuantityLimit   int64
	Price           any
	ExpiryDate      *time.Time
	FlatDiscount    any
	DiscountPercent any
	Payload         []byte
}

func main() {
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL connection URL")
	tenant := flag.String("tenant-id", "", "dedicated target tenant UUID")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical source")
	out := flag.String("out", filepath.Join("parity", "catalog", "canonical-first-tenant-price-policy-import.json"), "report path")
	flag.Parse()
	if strings.TrimSpace(*sourceURL) == "" || strings.TrimSpace(*targetURL) == "" {
		fatal("source and target are required")
	}
	if !*allowCanonical {
		fatal("-allow-canonical is required for the protected canonical source")
	}
	if strings.TrimSpace(*tenant) == "" {
		fatal("-tenant-id is required for canonical import")
	}
	if !strings.EqualFold(strings.TrimSpace(*tenant), canonicalTenant) {
		fatal("this bounded command only accepts the provisioned canonical tenant")
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

	rows, err := readRows(ctx, source)
	if err != nil {
		fatal(err.Error())
	}
	tx, err := target.Begin(ctx)
	if err != nil {
		fatal(fmt.Sprintf("begin target transaction: %v", err))
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true), set_config('app.allow_tenant_scope', 'true', true)`, *tenant); err != nil {
		fatal(fmt.Sprintf("set target scope: %v", err))
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE price_policy_stage (
		legacy_id text NOT NULL,
		legacy_policy_id text NOT NULL,
		quantity_limit integer NOT NULL,
		price numeric(19,4) NOT NULL DEFAULT 0,
		expiry_date date,
		flat_discount numeric(19,4) NOT NULL DEFAULT 0,
		discount_percent numeric(9,4) NOT NULL DEFAULT 0,
		source_legacy_id text NOT NULL,
		payload jsonb NOT NULL
	) ON COMMIT DROP`); err != nil {
		fatal(fmt.Sprintf("create staging table: %v", err))
	}
	copyRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		copyRows = append(copyRows, []any{row.LegacyID, row.LegacyPolicyID, row.QuantityLimit, row.Price, row.ExpiryDate, row.FlatDiscount, row.DiscountPercent, row.LegacyPolicyID, row.Payload})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"price_policy_stage"}, []string{"legacy_id", "legacy_policy_id", "quantity_limit", "price", "expiry_date", "flat_discount", "discount_percent", "source_legacy_id", "payload"}, pgx.CopyFromRows(copyRows)); err != nil {
		fatal(fmt.Sprintf("copy staging rows: %v", err))
	}
	var before int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM price_policy_tiers WHERE tenant_id = $1 AND legacy_id IN (SELECT legacy_id FROM price_policy_stage)`, *tenant).Scan(&before); err != nil {
		fatal(fmt.Sprintf("count existing pricing rows: %v", err))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO price_policy_tiers (
		tenant_id, legacy_id, legacy_policy_id, quantity_limit, price, expiry_date,
		flat_discount, discount_percent, source_table, source_legacy_id, payload
	)
	SELECT $1, legacy_id, legacy_policy_id, quantity_limit, price, expiry_date,
		flat_discount, discount_percent, 'PricePolicyDetail', source_legacy_id, payload
	FROM price_policy_stage
	ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET
		legacy_policy_id = EXCLUDED.legacy_policy_id,
		quantity_limit = EXCLUDED.quantity_limit,
		price = EXCLUDED.price,
		expiry_date = EXCLUDED.expiry_date,
		flat_discount = EXCLUDED.flat_discount,
		discount_percent = EXCLUDED.discount_percent,
		source_table = EXCLUDED.source_table,
		source_legacy_id = EXCLUDED.source_legacy_id,
		payload = EXCLUDED.payload,
		updated_at = now()`, *tenant); err != nil {
		fatal(fmt.Sprintf("upsert pricing rows: %v", err))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO legacy_id_mappings (
		tenant_id, source_system, source_schema, source_table, legacy_id,
		target_table, target_id, status, note
	)
	SELECT $1, 'sqlserver', 'dbo', 'PricePolicyDetail', t.legacy_id,
		'price_policy_tiers', t.id::text, 'mapped', NULL
	FROM price_policy_tiers t
	JOIN price_policy_stage s ON s.legacy_id = t.legacy_id
	WHERE t.tenant_id = $1
	ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
	DO UPDATE SET target_id = EXCLUDED.target_id, status = 'mapped', note = NULL`, *tenant); err != nil {
		fatal(fmt.Sprintf("record pricing mappings: %v", err))
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(fmt.Sprintf("commit pricing rows: %v", err))
	}
	item := tableReport{SourceSchema: "dbo", SourceTable: "PricePolicyDetail", TargetSchema: "public", TargetTable: targetTable, Read: len(rows), Imported: len(rows) - before, Duplicates: before, Exceptions: 0}
	writeReport(*out, report{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: "redacted SQL Server connection", Target: "redacted PostgreSQL connection", TenantID: *tenant, Tables: []tableReport{item}})
	fmt.Printf("Bulk imported %d pricing rows for tenant %s; report: %s\n", len(rows), *tenant, *out)
}

func readRows(ctx context.Context, source *sql.DB) ([]sourceRow, error) {
	rows, err := source.QueryContext(ctx, `SELECT PricePolicyCode, QtyLimit, Price, ExpiryDate, ItemFlatDisc, DiscPerc FROM dbo.PricePolicyDetail`)
	if err != nil {
		return nil, fmt.Errorf("read dbo.PricePolicyDetail: %w", err)
	}
	defer rows.Close()
	result := make([]sourceRow, 0, 30052)
	for rows.Next() {
		values := make([]any, 6)
		pointers := make([]any, len(values))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("scan PricePolicyDetail row %d: %w", len(result)+1, err)
		}
		policyID := normalizeText(values[0])
		quantityText := normalizeText(values[1])
		quantity, err := strconv.ParseInt(quantityText, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse QtyLimit %q: %w", quantityText, err)
		}
		var expiry *time.Time
		if value, ok := normalizeValue(values[3]).(time.Time); ok {
			utc := value.UTC()
			expiry = &utc
		}
		legacyID := strings.Join([]string{policyID, quantityText, fmt.Sprint(normalizeValue(values[3]))}, ":")
		payload, err := json.Marshal(map[string]any{
			"PricePolicyCode": normalizeValue(values[0]),
			"QtyLimit":        normalizeValue(values[1]),
			"Price":           normalizeValue(values[2]),
			"ExpiryDate":      normalizeValue(values[3]),
			"ItemFlatDisc":    normalizeValue(values[4]),
			"DiscPerc":        normalizeValue(values[5]),
		})
		if err != nil {
			return nil, fmt.Errorf("encode PricePolicyDetail row %d: %w", len(result)+1, err)
		}
		result = append(result, sourceRow{LegacyID: legacyID, LegacyPolicyID: policyID, QuantityLimit: quantity, Price: normalizeValue(values[2]), ExpiryDate: expiry, FlatDiscount: normalizeValue(values[4]), DiscountPercent: normalizeValue(values[5]), Payload: payload})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read PricePolicyDetail rows: %w", err)
	}
	return result, nil
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

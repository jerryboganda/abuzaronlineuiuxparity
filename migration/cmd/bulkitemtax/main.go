// Command bulkitemtax imports the reviewed canonical Item GST/PCT assignment
// slice with PostgreSQL COPY. It is intentionally bounded and fail-closed:
// the source is read-only, canonical opt-in is explicit, and every item/tax
// lookup must resolve before the target transaction can commit.
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
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jackc/pgx/v5"
)

const (
	canonicalDatabase = "FazalDinPP19DataBaseV2"
	canonicalTenant   = "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01"
	canonicalBranch   = "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02"
)

type assignment struct {
	ItemCode       string
	TaxCode        string
	TaxKind        string
	SourceLegacyID string
	EffectiveFrom  string
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
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server connection URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL connection URL")
	tenant := flag.String("tenant-id", "", "dedicated target tenant UUID")
	branch := flag.String("branch-id", "", "dedicated target branch UUID")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical source")
	out := flag.String("out", filepath.Join("parity", "catalog", "canonical-first-tenant-item-tax-import.json"), "report path")
	flag.Parse()
	if strings.TrimSpace(*sourceURL) == "" || strings.TrimSpace(*targetURL) == "" {
		fatal("source and target are required")
	}
	if !*allowCanonical {
		fatal("-allow-canonical is required for the protected canonical source")
	}
	if !strings.EqualFold(strings.TrimSpace(*tenant), canonicalTenant) {
		fatal("this bounded command only accepts the provisioned canonical tenant")
	}
	if !strings.EqualFold(strings.TrimSpace(*branch), canonicalBranch) {
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

	assignments, err := readAssignments(ctx, source)
	if err != nil {
		fatal(err.Error())
	}
	tx, err := target.Begin(ctx)
	if err != nil {
		fatal(fmt.Sprintf("begin target transaction: %v", err))
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true), set_config('app.branch_id', $2, true), set_config('app.allow_tenant_scope', 'true', true)`, *tenant, *branch); err != nil {
		fatal(fmt.Sprintf("set target scope: %v", err))
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE item_tax_stage (
		item_code text NOT NULL,
		tax_code text NOT NULL,
		tax_kind text NOT NULL,
		source_legacy_id text NOT NULL,
		effective_from date NOT NULL
	) ON COMMIT DROP`); err != nil {
		fatal(fmt.Sprintf("create staging table: %v", err))
	}
	copyRows := make([][]any, 0, len(assignments))
	for _, row := range assignments {
		copyRows = append(copyRows, []any{row.ItemCode, row.TaxCode, row.TaxKind, row.SourceLegacyID, row.EffectiveFrom})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"item_tax_stage"}, []string{"item_code", "tax_code", "tax_kind", "source_legacy_id", "effective_from"}, pgx.CopyFromRows(copyRows)); err != nil {
		fatal(fmt.Sprintf("copy staging rows: %v", err))
	}
	var resolvable int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*)
		FROM item_tax_stage s
		JOIN master_items i ON i.tenant_id = $1 AND i.legacy_id = s.item_code
		JOIN tax_rates t ON t.tenant_id = $1 AND t.branch_id = $2 AND t.tax_kind = s.tax_kind AND t.code = s.tax_code`, *tenant, *branch).Scan(&resolvable); err != nil {
		fatal(fmt.Sprintf("check item-tax dependencies: %v", err))
	}
	if resolvable != len(assignments) {
		fatal(fmt.Sprintf("item-tax dependency check resolved %d of %d rows", resolvable, len(assignments)))
	}
	beforeGST, err := countExisting(ctx, tx, *tenant, *branch, "gst")
	if err != nil {
		fatal(err.Error())
	}
	beforePCT, err := countExisting(ctx, tx, *tenant, *branch, "pct")
	if err != nil {
		fatal(err.Error())
	}
	if _, err := tx.Exec(ctx, `INSERT INTO item_tax_assignments (
		tenant_id, branch_id, item_id, tax_rate_id, effective_from, source_table, source_legacy_id
	)
	SELECT $1, $2, i.id, t.id, s.effective_from, 'Item', s.source_legacy_id
	FROM item_tax_stage s
	JOIN master_items i ON i.tenant_id = $1 AND i.legacy_id = s.item_code
	JOIN tax_rates t ON t.tenant_id = $1 AND t.branch_id = $2 AND t.tax_kind = s.tax_kind AND t.code = s.tax_code
	ON CONFLICT (tenant_id, branch_id, item_id, tax_rate_id, effective_from) DO UPDATE SET
		source_table = EXCLUDED.source_table,
		source_legacy_id = EXCLUDED.source_legacy_id,
		updated_at = now()`, *tenant, *branch); err != nil {
		fatal(fmt.Sprintf("upsert item-tax assignments: %v", err))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO legacy_id_mappings (
		tenant_id, source_system, source_schema, source_table, legacy_id,
		target_table, target_id, status, note
	)
	SELECT $1, 'sqlserver', 'dbo', 'Item', s.source_legacy_id,
		'public.item_tax_assignments', MIN(a.id::text), 'mapped', 'GST/PCT assignment bulk import'
	FROM item_tax_stage s
	JOIN item_tax_assignments a ON a.tenant_id = $1 AND a.branch_id = $2 AND a.source_table = 'Item' AND a.source_legacy_id = s.source_legacy_id
	GROUP BY s.source_legacy_id
	ON CONFLICT (tenant_id, source_system, source_schema, source_table, legacy_id)
	DO UPDATE SET target_id = EXCLUDED.target_id, status = 'mapped', note = EXCLUDED.note`, *tenant, *branch); err != nil {
		fatal(fmt.Sprintf("record item-tax mappings: %v", err))
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(fmt.Sprintf("commit item-tax assignments: %v", err))
	}
	gstRead := countKind(assignments, "gst")
	pctRead := countKind(assignments, "pct")
	writeReport(*out, report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Source:      "redacted SQL Server connection", Target: "redacted PostgreSQL connection", TenantID: *tenant,
		Tables: []tableReport{
			{SourceSchema: "dbo", SourceTable: "Item (GST)", TargetSchema: "public", TargetTable: "item_tax_assignments", Read: gstRead, Imported: gstRead - beforeGST, Duplicates: beforeGST, Exceptions: 0},
			{SourceSchema: "dbo", SourceTable: "Item (PCT)", TargetSchema: "public", TargetTable: "item_tax_assignments", Read: pctRead, Imported: pctRead - beforePCT, Duplicates: beforePCT, Exceptions: 0},
		},
	})
	fmt.Printf("Bulk imported %d GST/PCT item-tax assignments for tenant %s; report: %s\n", len(assignments), *tenant, *out)
}

func readAssignments(ctx context.Context, source *sql.DB) ([]assignment, error) {
	rows, err := source.QueryContext(ctx, `SELECT ICode, SalesTaxScheduleCode, PCTCode FROM dbo.Item`)
	if err != nil {
		return nil, fmt.Errorf("read dbo.Item tax codes: %w", err)
	}
	defer rows.Close()
	result := make([]assignment, 0, 60104)
	for rows.Next() {
		values := make([]any, 3)
		pointers := make([]any, len(values))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("scan Item tax row %d: %w", len(result)+1, err)
		}
		itemCode := normalizeText(values[0])
		gstCode := normalizeText(values[1])
		pctCode := normalizeText(values[2])
		if itemCode == "" {
			return nil, fmt.Errorf("Item row %d has an empty ICode", len(result)+1)
		}
		if gstCode != "" && !strings.EqualFold(gstCode, "<nil>") {
			result = append(result, assignment{ItemCode: itemCode, TaxCode: gstCode, TaxKind: "gst", SourceLegacyID: itemCode, EffectiveFrom: "2000-01-01"})
		}
		if pctCode != "" && !strings.EqualFold(pctCode, "<nil>") {
			result = append(result, assignment{ItemCode: itemCode, TaxCode: pctCode, TaxKind: "pct", SourceLegacyID: itemCode, EffectiveFrom: "2000-01-01"})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Item tax rows: %w", err)
	}
	return result, nil
}

func countExisting(ctx context.Context, tx pgx.Tx, tenant, branch, kind string) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM item_tax_assignments a JOIN tax_rates t ON t.tenant_id = a.tenant_id AND t.branch_id = a.branch_id AND t.id = a.tax_rate_id WHERE a.tenant_id = $1 AND a.branch_id = $2 AND t.tax_kind = $3`, tenant, branch, kind).Scan(&count); err != nil {
		return 0, fmt.Errorf("count existing %s assignments: %w", kind, err)
	}
	return count, nil
}

func countKind(rows []assignment, kind string) int {
	count := 0
	for _, row := range rows {
		if row.TaxKind == kind {
			count++
		}
	}
	return count
}

func normalizeText(value any) string {
	if bytes, ok := value.([]byte); ok {
		value = string(bytes)
	}
	return strings.TrimSpace(fmt.Sprint(value))
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

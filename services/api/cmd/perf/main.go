// Command perf runs bounded, tenant-scoped read probes for the Phase W
// disposable fixture. It intentionally does not post documents or mutate a
// non-disposable database.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultTenant = "90000000-0000-0000-0000-000000000001"
	defaultBranch = "90000000-0000-0000-0000-000000000101"
	defaultGodown = "90000000-0000-0000-0000-000000000201"
	defaultParty  = "90000000-0000-0000-0000-000000000301"
)

type workload struct {
	Name  string
	SQL   string
	Plan  string
	Args  func(tenant, branch, godown, party string) []any
	Limit int
}

type result struct {
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	SamplesMS  []float64 `json:"samplesMs"`
	Rows       int       `json:"rowsLastSample"`
	P50MS      float64   `json:"p50Ms"`
	P95MS      float64   `json:"p95Ms"`
	Plan       any       `json:"plan,omitempty"`
	PlanError  string    `json:"planError,omitempty"`
	MeasureErr string    `json:"measureError,omitempty"`
}

type artifact struct {
	GeneratedAt string `json:"generatedAt"`
	Database    string `json:"database"`
	Server      string `json:"serverVersion"`
	Fixture     struct {
		Tenant string `json:"tenantId"`
		Branch string `json:"branchId"`
		Godown string `json:"godownId"`
		Party  string `json:"partyId"`
	} `json:"fixture"`
	Volumes map[string]int64 `json:"volumes"`
	Budgets map[string]any   `json:"budgets"`
	Results []result         `json:"results"`
}

func main() {
	dsn := flag.String("dsn", os.Getenv("ABUZAR_PERF_DATABASE_URL"), "disposable PostgreSQL DSN")
	output := flag.String("output", "tmp/phase-w-performance.json", "artifact path")
	iterations := flag.Int("iterations", 15, "samples per workload")
	tenant := flag.String("tenant", defaultTenant, "fixture tenant UUID")
	branch := flag.String("branch", defaultBranch, "fixture branch UUID")
	godown := flag.String("godown", defaultGodown, "fixture godown UUID")
	party := flag.String("party", defaultParty, "fixture party UUID")
	flag.Parse()
	if *dsn == "" {
		fail("ABUZAR_PERF_DATABASE_URL or -dsn is required")
	}
	if *iterations < 3 || *iterations > 100 {
		fail("-iterations must be between 3 and 100")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", *dsn)
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		fail("ping database: %v", err)
	}

	serverVersion := ""
	_ = db.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&serverVersion)
	volumes := map[string]int64{}
	for _, table := range []string{
		"stock_ledger", "stock_batches", "stock_balances", "business_documents",
		"business_document_lines", "sync_events", "gl_journals", "gl_lines",
		"party_ledger_entries",
	} {
		var count int64
		query := fmt.Sprintf("SELECT count(*) FROM %s WHERE tenant_id = $1::uuid AND branch_id = $2::uuid", table)
		if err := db.QueryRowContext(ctx, query, *tenant, *branch).Scan(&count); err != nil {
			// Every Phase W fixture table has the scope columns. Preserve a
			// truthful error instead of silently treating a missing table as 0.
			fail("count %s: %v", table, err)
		}
		volumes[table] = count
	}

	workloads := workloads(*tenant, *branch, *godown, *party)
	results := make([]result, 0, len(workloads))
	for _, work := range workloads {
		item := result{Name: work.Name, Kind: "bounded_read_proxy"}
		plan, planErr := explain(ctx, db, work.Plan, *tenant, *branch, *godown, *party)
		if planErr != nil {
			item.PlanError = planErr.Error()
		} else {
			item.Plan = plan
		}
		for i := 0; i < *iterations; i++ {
			start := time.Now()
			rows, count, measureErr := measure(ctx, db, work, *tenant, *branch, *godown, *party)
			if measureErr != nil {
				item.MeasureErr = measureErr.Error()
				break
			}
			item.SamplesMS = append(item.SamplesMS, float64(time.Since(start).Microseconds())/1000)
			item.Rows = count
			_ = rows
		}
		if len(item.SamplesMS) > 0 {
			sort.Float64s(item.SamplesMS)
			item.P50MS = percentile(item.SamplesMS, 0.50)
			item.P95MS = percentile(item.SamplesMS, 0.95)
		}
		results = append(results, item)
	}

	var reportCount = volumes["stock_ledger"]
	fullVolume := reportCount >= 3231846 && volumes["gl_journals"] >= 1040590
	budgets := map[string]any{
		"posLineAddProxyMs":   150,
		"documentPostProxyMs": 1000,
		"heavyReportP95Ms":    5000,
		"fullVolumeLoaded":    fullVolume,
		"acceptance":          "pending",
		"note":                "Proxies are bounded tenant-scoped DB reads; this command does not claim end-to-end HTTP post latency.",
	}
	if fullVolume {
		budgets["acceptance"] = "review_required"
	}

	var out artifact
	out.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	out.Database = "disposable fixture (DSN omitted)"
	out.Server = serverVersion
	out.Fixture.Tenant, out.Fixture.Branch = *tenant, *branch
	out.Fixture.Godown, out.Fixture.Party = *godown, *party
	out.Volumes, out.Budgets, out.Results = volumes, budgets, results
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fail("encode artifact: %v", err)
	}
	if err := os.MkdirAll(directory(*output), 0755); err != nil {
		fail("create artifact directory: %v", err)
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		fail("write artifact: %v", err)
	}
	fmt.Printf("wrote %s (fullVolumeLoaded=%t)\n", *output, fullVolume)
}

func workloads(tenant, branch, godown, party string) []workload {
	availability := `
		SELECT b.id::text, b.batch_number, COALESCE(b.expiry_date::text, ''),
		       sb.on_hand::text, b.unit_cost::text
		FROM stock_batches b
		JOIN stock_balances sb
		  ON sb.tenant_id = b.tenant_id AND sb.branch_id = b.branch_id
		 AND sb.batch_id = b.id
		WHERE b.tenant_id = $1::uuid AND b.branch_id = $2::uuid
		  AND b.item_legacy_id = 'ITEM-000001' AND b.godown_id = $3::uuid
		  AND NOT b.locked AND (b.expiry_date IS NULL OR b.expiry_date >= CURRENT_DATE)
		  AND sb.on_hand > 0
		ORDER BY b.received_at, b.id LIMIT 101`
	stockReport := `
		SELECT l.id::text, l.occurred_at, l.direction, b.item_legacy_id,
		       l.quantity, l.unit_cost
		FROM stock_ledger l
		JOIN stock_batches b
		  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id AND b.id = l.batch_id
		JOIN sync_events se
		  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
		WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
		  AND l.occurred_at >= CURRENT_DATE - INTERVAL '365 days'
		  AND l.occurred_at < CURRENT_DATE + INTERVAL '1 day'
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		ORDER BY l.occurred_at DESC, l.id LIMIT 101`
	salesReport := `
		SELECT d.document_number, d.occurred_at, p.name, l.item_name,
		       l.quantity, l.line_total
		FROM business_documents d
		JOIN business_document_lines l
		  ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id AND l.document_id = d.id
		LEFT JOIN master_parties p
		  ON p.tenant_id = d.tenant_id AND p.id = d.customer_id
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
		  AND d.kind IN ('cash-sale', 'credit-sale') AND d.status = 'posted'
		  AND d.occurred_at >= CURRENT_DATE - INTERVAL '365 days'
		  AND d.occurred_at < CURRENT_DATE + INTERVAL '1 day'
		ORDER BY d.occurred_at DESC, d.document_number, l.item_name LIMIT 101`
	journals := `
		SELECT j.id, j.source_document_id, j.source_event_id, j.kind,
		       j.posted_at, j.total_debit, j.total_credit
		FROM gl_journals j
		WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
		ORDER BY j.posted_at DESC, j.id DESC LIMIT 101`
	ledger := `
		SELECT id, source_document_id, debit_amount, credit_amount,
		       COALESCE(balance_after, 0), occurred_at, description
		FROM party_ledger_entries
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid AND party_id = $3::uuid
		ORDER BY occurred_at, id LIMIT 101`
	glLines := `
		SELECT id, journal_id, party_id, debit_amount, credit_amount
		FROM gl_lines
		WHERE tenant_id = $1::uuid AND branch_id = $2::uuid
		  AND account_id = (
		      SELECT id FROM finance_accounts
		      WHERE tenant_id = $1::uuid AND system_key = 'sales_revenue'
		  )
		ORDER BY created_at, id LIMIT 101`
	ledgerPlan := strings.ReplaceAll(ledger, "$1", "'"+tenant+"'::uuid")
	ledgerPlan = strings.ReplaceAll(ledgerPlan, "$2", "'"+branch+"'::uuid")
	ledgerPlan = strings.ReplaceAll(ledgerPlan, "$3", "'"+party+"'::uuid")
	return []workload{
		{Name: "pos-line-add", SQL: availability, Plan: availability,
			Args: func(t, b, g, p string) []any { return []any{t, b, g} }},
		{Name: "heavy-stock-report", SQL: stockReport, Plan: stockReport,
			Args: func(t, b, g, p string) []any { return []any{t, b} }},
		{Name: "heavy-sales-report", SQL: salesReport, Plan: salesReport,
			Args: func(t, b, g, p string) []any { return []any{t, b} }},
		{Name: "finance-journals", SQL: journals, Plan: journals,
			Args: func(t, b, g, p string) []any { return []any{t, b} }},
		{Name: "party-ledger", SQL: ledger, Plan: ledgerPlan,
			Args: func(t, b, g, p string) []any { return []any{t, b, p} }},
		{Name: "gl-account-lines", SQL: glLines, Plan: glLines,
			Args: func(t, b, g, p string) []any { return []any{t, b} }},
	}
}

func measure(ctx context.Context, db *sql.DB, work workload, tenant, branch, godown, party string) (bool, int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	for key, value := range map[string]string{
		"app.tenant_id": tenant, "app.branch_id": branch,
		"app.allow_tenant_scope": "false", "app.authenticating": "false",
		"statement_timeout": "5000ms", "lock_timeout": "1000ms",
	} {
		if _, err := tx.ExecContext(ctx, "SELECT set_config($1, $2, true)", key, value); err != nil {
			return false, 0, err
		}
	}
	rows, err := tx.QueryContext(ctx, work.SQL, work.Args(tenant, branch, godown, party)...)
	if err != nil {
		return false, 0, err
	}
	count := 0
	columns, _ := rows.Columns()
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return false, count, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, count, err
	}
	rows.Close()
	return true, count, tx.Commit()
}

func explain(ctx context.Context, db *sql.DB, query, tenant, branch, godown, party string) (any, error) {
	// Parameters are replaced only with fixed UUID/date fixture values. They
	// never come from an external request and are safe for this benchmark.
	planSQL := "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + query
	planSQL = replacePlanArgs(planSQL, tenant, branch, godown, party)
	var raw []byte
	if err := db.QueryRowContext(ctx, planSQL).Scan(&raw); err != nil {
		return nil, err
	}
	var plan any
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func replacePlanArgs(query, tenant, branch, godown, party string) string {
	// Workload plans use PostgreSQL parameters in a fixed order. Replacing from
	// highest to lowest avoids changing the lower-numbered placeholders.
	replacements := map[string]string{
		"$4": "'" + party + "'::uuid", "$3": "'" + godown + "'::uuid",
		"$2": "'" + branch + "'::uuid", "$1": "'" + tenant + "'::uuid",
	}
	for old, value := range replacements {
		query = strings.ReplaceAll(query, old, value)
	}
	return query
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}

func directory(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			if i == 0 {
				return path[:1]
			}
			return path[:i]
		}
	}
	return "."
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

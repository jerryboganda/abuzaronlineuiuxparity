// Command bulk-historical promotes the two high-volume historical snapshots
// with COPY-backed, restart-safe batches. SQL Server is read-only; PostgreSQL
// receives only rows selected from the reviewed AbuzarLegacyReference source.
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
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jackc/pgx/v5"
)

const (
	tenantID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	branchID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
)

func main() {
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL URL")
	wave := flag.String("wave", "both", "stock, gl, or both")
	batchSize := flag.Int("batch-size", 5000, "rows per committed target batch")
	flag.Parse()
	if *sourceURL == "" || *targetURL == "" {
		fatal("source and target are required")
	}
	if err := validateSource(*sourceURL); err != nil {
		fatal(err.Error())
	}
	if *batchSize < 100 || *batchSize > 50000 {
		fatal("batch-size must be between 100 and 50000")
	}
	if *wave != "stock" && *wave != "gl" && *wave != "both" {
		fatal("wave must be stock, gl, or both")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	source, err := sql.Open("sqlserver", *sourceURL)
	if err != nil {
		fatal(fmt.Sprintf("open source: %v", err))
	}
	defer source.Close()
	target, err := pgx.Connect(ctx, *targetURL)
	if err != nil {
		fatal(fmt.Sprintf("open target: %v", err))
	}
	defer target.Close(ctx)
	if err := source.PingContext(ctx); err != nil {
		fatal(fmt.Sprintf("source ping failed: %v", err))
	}
	if err := target.Ping(ctx); err != nil {
		fatal(fmt.Sprintf("target ping failed: %v", err))
	}
	for key, value := range map[string]string{
		"app.tenant_id": tenantID, "app.branch_id": branchID,
		"app.allow_tenant_scope": "true",
	} {
		if _, err := target.Exec(ctx, "SELECT set_config($1,$2,false)", key, value); err != nil {
			fatal(fmt.Sprintf("set target scope: %v", err))
		}
	}

	if *wave == "stock" || *wave == "both" {
		fmt.Printf("stock snapshot: ")
		count, err := importStock(ctx, source, target, *batchSize)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "gl" || *wave == "both" {
		fmt.Printf("historical GL: ")
		count, err := importGL(ctx, source, target, *batchSize)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
}

type stockRow struct {
	legacyID, itemLegacyID, godownLegacyID    string
	asOf                                      time.Time
	quantity, purchase, sale, average, recent string
	packUnits                                 int64
	payload                                   string
}

func importStock(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int) (int64, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT [Date], [GCode], [ICode], [Stock], [PurchasePrice], [SalePrice],
		       [AvgPrice], [RecentPurchasePrice], [PackUnits]
		FROM [dbo].[StockReport]`)
	if err != nil {
		return 0, fmt.Errorf("read StockReport: %w", err)
	}
	defer rows.Close()
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_e_stock_batch (
		legacy_id text NOT NULL, item_legacy_id text NOT NULL, godown_legacy_id text NOT NULL,
		as_of date NOT NULL, quantity numeric(19,4) NOT NULL, purchase_price numeric(19,4) NOT NULL,
		sale_price numeric(19,4) NOT NULL, average_price numeric(19,4) NOT NULL,
		recent_purchase_price numeric(19,4) NOT NULL, pack_units integer NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create stock staging table: %w", err)
	}
	var total int64
	batch := make([]stockRow, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "TRUNCATE phase_e_stock_batch"); err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_e_stock_batch"},
				[]string{"legacy_id", "item_legacy_id", "godown_legacy_id", "as_of", "quantity", "purchase_price", "sale_price", "average_price", "recent_purchase_price", "pack_units", "payload"},
				pgx.CopyFromRows(stockValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_stock_snapshots
					(tenant_id, branch_id, legacy_id, item_id, item_legacy_id, godown_id,
					 as_of, quantity, purchase_price, sale_price, average_price,
					 recent_purchase_price, pack_units, source_table, source_legacy_id, payload)
				SELECT $1::uuid, $2::uuid, s.legacy_id, i.id, s.item_legacy_id, g.id,
				       s.as_of, s.quantity, s.purchase_price, s.sale_price, s.average_price,
				       s.recent_purchase_price, s.pack_units, 'StockReport', s.item_legacy_id, s.payload
				FROM phase_e_stock_batch s
				JOIN master_items i ON i.tenant_id=$1::uuid AND i.legacy_id=s.item_legacy_id
				JOIN master_godowns g ON g.tenant_id=$1::uuid AND g.legacy_id=s.godown_legacy_id
				ON CONFLICT (tenant_id, branch_id, legacy_id) DO UPDATE SET
					item_id=EXCLUDED.item_id, godown_id=EXCLUDED.godown_id,
					as_of=EXCLUDED.as_of, quantity=EXCLUDED.quantity,
					purchase_price=EXCLUDED.purchase_price, sale_price=EXCLUDED.sale_price,
					average_price=EXCLUDED.average_price,
					recent_purchase_price=EXCLUDED.recent_purchase_price,
					pack_units=EXCLUDED.pack_units, payload=EXCLUDED.payload,
					updated_at=now()`, tenantID, branchID)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err != nil {
			return err
		}
		total += int64(len(batch))
		batch = batch[:0]
		return nil
	}
	for rows.Next() {
		var dateValue, gcode, icode, stock, purchase, sale, average, recent, pack any
		if err := rows.Scan(&dateValue, &gcode, &icode, &stock, &purchase, &sale, &average, &recent, &pack); err != nil {
			return total, err
		}
		date, err := sourceTime(dateValue)
		if err != nil {
			return total, err
		}
		item := text(icode)
		godown := text(gcode)
		payload, _ := json.Marshal(map[string]any{"Date": date, "GCode": godown, "ICode": item, "Stock": normalize(stock), "PurchasePrice": normalize(purchase), "SalePrice": normalize(sale), "AvgPrice": normalize(average), "RecentPurchasePrice": normalize(recent), "PackUnits": normalize(pack)})
		batch = append(batch, stockRow{
			legacyID:     fmt.Sprintf("%s:%s:%s", date.Format(time.RFC3339), godown, item),
			itemLegacyID: item, godownLegacyID: godown, asOf: date,
			quantity: text(stock), purchase: text(purchase), sale: text(sale),
			average: text(average), recent: text(recent), packUnits: integer(pack),
			payload: string(payload),
		})
		if len(batch) == batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("stock batch at %d: %w", total, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, err
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func stockValues(rows []stockRow) [][]any {
	values := make([][]any, len(rows))
	for i, row := range rows {
		values[i] = []any{row.legacyID, row.itemLegacyID, row.godownLegacyID, row.asOf, row.quantity, row.purchase, row.sale, row.average, row.recent, row.packUnits, row.payload}
	}
	return values
}

type glRow struct {
	legacyID, documentCode, documentType, accountCode, alternate, debit, credit string
	occurred                                                                    time.Time
	user, invoice, remarks, payload                                             string
}

func importGL(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int) (int64, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT [DocumentCode], [DocumentType], [AccCode], [AlternateAccCode],
		       [Debit], [Credit], [Date], [UserCode], [INVOICECODE], [Remarks], [VRow]
		FROM [dbo].[VirtualGl]`)
	if err != nil {
		return 0, fmt.Errorf("read VirtualGl: %w", err)
	}
	defer rows.Close()
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_e_gl_batch (
		legacy_id text NOT NULL, document_code text NOT NULL, document_type text NOT NULL,
		account_code text NOT NULL, alternate_account_code text NOT NULL,
		debit_amount numeric(19,4) NOT NULL, credit_amount numeric(19,4) NOT NULL,
		occurred_at timestamptz NOT NULL, user_legacy_id text NOT NULL,
		invoice_code text NOT NULL, remarks text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create GL staging table: %w", err)
	}
	var total int64
	batch := make([]glRow, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_e_gl_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_e_gl_batch"},
				[]string{"legacy_id", "document_code", "document_type", "account_code", "alternate_account_code", "debit_amount", "credit_amount", "occurred_at", "user_legacy_id", "invoice_code", "remarks", "payload"},
				pgx.CopyFromRows(glValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_gl_entries
					(tenant_id, branch_id, legacy_id, document_code, document_type,
					 account_code, alternate_account_code, debit_amount, credit_amount,
					 occurred_at, user_legacy_id, invoice_code, remarks, payload)
				SELECT $1::uuid, $2::uuid, legacy_id, document_code, document_type,
				       account_code, alternate_account_code, debit_amount, credit_amount,
				       occurred_at, user_legacy_id, invoice_code, remarks, payload
				FROM phase_e_gl_batch
				ON CONFLICT (tenant_id, branch_id, legacy_id) DO UPDATE SET
					document_code=EXCLUDED.document_code, document_type=EXCLUDED.document_type,
					account_code=EXCLUDED.account_code,
					alternate_account_code=EXCLUDED.alternate_account_code,
					debit_amount=EXCLUDED.debit_amount, credit_amount=EXCLUDED.credit_amount,
					occurred_at=EXCLUDED.occurred_at, user_legacy_id=EXCLUDED.user_legacy_id,
					invoice_code=EXCLUDED.invoice_code, remarks=EXCLUDED.remarks,
					payload=EXCLUDED.payload`, tenantID, branchID)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else if tx != nil {
			_ = tx.Rollback(ctx)
		}
		if err != nil {
			return err
		}
		total += int64(len(batch))
		batch = batch[:0]
		return nil
	}
	for rows.Next() {
		var document, kind, account, alternate, debit, credit, dateValue, user, invoice, remarks, row any
		if err := rows.Scan(&document, &kind, &account, &alternate, &debit, &credit, &dateValue, &user, &invoice, &remarks, &row); err != nil {
			return total, err
		}
		occurred, err := sourceTime(dateValue)
		if err != nil {
			return total, err
		}
		doc := text(document)
		vrow := text(row)
		acct := text(account)
		payload, _ := json.Marshal(map[string]any{"DocumentCode": document, "DocumentType": kind, "AccCode": account, "AlternateAccCode": alternate, "Debit": normalize(debit), "Credit": normalize(credit), "Date": occurred, "UserCode": user, "INVOICECODE": invoice, "Remarks": remarks, "VRow": row})
		batch = append(batch, glRow{
			legacyID:     fmt.Sprintf("%s:%s:%s", doc, vrow, acct),
			documentCode: doc, documentType: text(kind), accountCode: acct, alternate: text(alternate),
			debit: text(debit), credit: text(credit), occurred: occurred,
			user: text(user), invoice: text(invoice), remarks: text(remarks), payload: string(payload),
		})
		if len(batch) == batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("GL batch at %d: %w", total, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, err
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func glValues(rows []glRow) [][]any {
	values := make([][]any, len(rows))
	for i, row := range rows {
		values[i] = []any{row.legacyID, row.documentCode, row.documentType, row.accountCode, row.alternate, row.debit, row.credit, row.occurred, row.user, row.invoice, row.remarks, row.payload}
	}
	return values
}

func sourceTime(value any) (time.Time, error) {
	switch value := value.(type) {
	case time.Time:
		return value, nil
	default:
		return time.Time{}, fmt.Errorf("source date is not a SQL datetime: %T", value)
	}
}

func normalize(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func text(value any) string {
	return fmt.Sprint(normalize(value))
}

func integer(value any) int64 {
	raw, _ := strconv.ParseInt(text(value), 10, 64)
	return raw
}

func validateSource(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	database := parsed.Query().Get("database")
	if strings.EqualFold(database, "FazalDinPP19DataBaseV2") {
		return errors.New("refusing canonical FazalDinPP19DataBaseV2")
	}
	if !strings.EqualFold(database, "AbuzarLegacyReference") {
		return fmt.Errorf("source database must be AbuzarLegacyReference, got %q", database)
	}
	return nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

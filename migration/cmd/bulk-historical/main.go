// Command bulk-historical promotes the reviewed high-volume historical snapshots
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
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jackc/pgx/v5"
)

const (
	sandboxTenantID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	sandboxBranchID   = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	canonicalDatabase = "FazalDinPP19DataBaseV2"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func main() {
	sourceURL := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "read-only SQL Server URL")
	targetURL := flag.String("target", os.Getenv("ABUZAR_TARGET_POSTGRES_URL"), "PostgreSQL URL")
	wave := flag.String("wave", "both", "stock, gl, history, adjustments, deleted-sale-items, withholding, payments, party-adjustments, return-allocations, both, or all")
	batchSize := flag.Int("batch-size", 5000, "rows per committed target batch")
	allowCanonical := flag.Bool("allow-canonical", false, "explicitly allow the protected canonical FazalDinPP19DataBaseV2 source")
	tenantOverride := flag.String("tenant-id", "", "target tenant UUID; required for canonical imports")
	branchOverride := flag.String("branch-id", "", "target branch UUID; required for canonical imports")
	flag.Parse()
	if *sourceURL == "" || *targetURL == "" {
		fatal("source and target are required")
	}
	if err := validateSource(*sourceURL, *allowCanonical); err != nil {
		fatal(err.Error())
	}
	targetTenantID := strings.TrimSpace(*tenantOverride)
	targetBranchID := strings.TrimSpace(*branchOverride)
	if *allowCanonical {
		if targetTenantID == "" || targetBranchID == "" {
			fatal("-tenant-id and -branch-id are required when -allow-canonical is enabled; canonical historical imports must use a dedicated target scope")
		}
	} else {
		if targetTenantID == "" {
			targetTenantID = sandboxTenantID
		}
		if targetBranchID == "" {
			targetBranchID = sandboxBranchID
		}
	}
	if err := validateUUIDScope(targetTenantID, targetBranchID); err != nil {
		fatal(err.Error())
	}
	if *batchSize < 100 || *batchSize > 50000 {
		fatal("batch-size must be between 100 and 50000")
	}
	if !validWave(*wave) {
		fatal("wave must be stock, gl, history, adjustments, deleted-sale-items, withholding, payments, party-adjustments, return-allocations, both, or all")
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
		"app.tenant_id": targetTenantID, "app.branch_id": targetBranchID,
		"app.allow_tenant_scope": "true",
	} {
		if _, err := target.Exec(ctx, "SELECT set_config($1,$2,false)", key, value); err != nil {
			fatal(fmt.Sprintf("set target scope: %v", err))
		}
	}

	if *wave == "stock" || *wave == "both" || *wave == "all" {
		fmt.Printf("stock snapshot: ")
		count, err := importStock(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "gl" || *wave == "both" || *wave == "all" {
		fmt.Printf("historical GL: ")
		count, err := importGL(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "history" || *wave == "all" {
		fmt.Printf("item history: ")
		count, err := importItemHistory(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d normalized rows processed\n", count)
	}
	if *wave == "adjustments" || *wave == "all" {
		fmt.Printf("stock adjustments: ")
		count, err := importAdjustments(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d normalized rows processed\n", count)
	}
	if *wave == "deleted-sale-items" || *wave == "all" {
		fmt.Printf("deleted sale items: ")
		count, err := importDeletedSaleItems(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "withholding" || *wave == "all" {
		fmt.Printf("withholding tax: ")
		count, err := importWithholding(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "payments" || *wave == "all" {
		fmt.Printf("party payments: ")
		count, err := importPayments(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "party-adjustments" || *wave == "all" {
		fmt.Printf("party adjustments: ")
		count, err := importPartyAdjustments(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("%d source rows processed\n", count)
	}
	if *wave == "return-allocations" || *wave == "all" {
		fmt.Printf("party return allocations: ")
		count, err := importPartyReturnAllocations(ctx, source, target, *batchSize, targetTenantID, targetBranchID)
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

func importStock(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
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
			var eligible int64
			err = tx.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM phase_e_stock_batch s
				JOIN master_items i ON i.tenant_id = $1::uuid AND i.legacy_id = s.item_legacy_id
				JOIN master_godowns g ON g.tenant_id = $1::uuid AND g.legacy_id = s.godown_legacy_id`, tenantID).Scan(&eligible)
			if err == nil && eligible != int64(len(batch)) {
				err = fmt.Errorf("stock batch at %d has %d rows without canonical item/godown dependencies; refusing silent loss", total, int64(len(batch))-eligible)
			}
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

func importGL(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
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

type withholdingRow struct {
	legacyID, paymentCode, purchaseInvoiceCode, supplierLegacyID string
	occurred                                                     time.Time
	posted                                                       bool
	accountCode, taxableBase, rate, amount                       string
	checkNumber, remarks, userLegacyID, payload                  string
}

// importWithholding preserves PurPayment's payment-level withholding fields.
// It deliberately does not derive rows from purchase-line advance tax.
func importWithholding(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT p.[PurPaymentCode], p.[Date], p.[UserCode], p.[PurInvCode],
		       p.[Posted], p.[WHTaxAccCode], p.[WHTaxPerc], p.[WHTaxBaseAmt],
		       p.[WHTaxAmt], p.[WHTaxCheckNo], p.[WHTaxRemarks],
		       l.[SuppCode], l.[SuppInvCode]
		FROM [dbo].[PurPayment] p
		LEFT JOIN [dbo].[Purledger] l ON l.[PurInvCode] = p.[PurInvCode]
		ORDER BY p.[PurPaymentCode]`)
	if err != nil {
		return 0, fmt.Errorf("read PurPayment: %w", err)
	}
	defer rows.Close()
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_e_withholding_batch (
		legacy_id text NOT NULL, payment_code text NOT NULL, purchase_invoice_code text NOT NULL,
		supplier_legacy_id text NOT NULL, occurred_at timestamptz NOT NULL, posted boolean NOT NULL,
		account_code text NOT NULL, taxable_base numeric(19,4) NOT NULL, rate numeric(19,4) NOT NULL,
		amount numeric(19,4) NOT NULL, check_number text NOT NULL, remarks text NOT NULL,
		user_legacy_id text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create withholding staging table: %w", err)
	}
	var total int64
	batch := make([]withholdingRow, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_e_withholding_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_e_withholding_batch"},
				[]string{"legacy_id", "payment_code", "purchase_invoice_code", "supplier_legacy_id", "occurred_at", "posted", "account_code", "taxable_base", "rate", "amount", "check_number", "remarks", "user_legacy_id", "payload"},
				pgx.CopyFromRows(withholdingValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_withholding_tax_entries (
					tenant_id, branch_id, legacy_id, payment_code, purchase_invoice_code,
					supplier_legacy_id, occurred_at, posted, account_code, taxable_base,
					rate, amount, check_number, remarks, user_legacy_id, source_table,
					source_table_row, payload)
				SELECT $1::uuid, $2::uuid, legacy_id, payment_code, purchase_invoice_code,
				       supplier_legacy_id, occurred_at, posted, account_code, taxable_base,
				       rate, amount, check_number, remarks, user_legacy_id, 'PurPayment',
				       payment_code, payload
				FROM phase_e_withholding_batch
				ON CONFLICT (tenant_id, branch_id, legacy_id) DO UPDATE SET
					payment_code = EXCLUDED.payment_code,
					purchase_invoice_code = EXCLUDED.purchase_invoice_code,
					supplier_legacy_id = EXCLUDED.supplier_legacy_id,
					occurred_at = EXCLUDED.occurred_at, posted = EXCLUDED.posted,
					account_code = EXCLUDED.account_code,
					taxable_base = EXCLUDED.taxable_base, rate = EXCLUDED.rate,
					amount = EXCLUDED.amount, check_number = EXCLUDED.check_number,
					remarks = EXCLUDED.remarks, user_legacy_id = EXCLUDED.user_legacy_id,
					payload = EXCLUDED.payload, updated_at = now()`, tenantID, branchID)
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
		var payment, dateValue, user, invoice, posted, account, rate, base, amount, checkNumber, remarks, supplier, supplierInvoice any
		if err := rows.Scan(&payment, &dateValue, &user, &invoice, &posted, &account, &rate, &base, &amount, &checkNumber, &remarks, &supplier, &supplierInvoice); err != nil {
			return total, err
		}
		occurred, err := sourceTime(dateValue)
		if err != nil {
			return total, err
		}
		paymentCode := sourceText(payment)
		if paymentCode == "" {
			return total, errors.New("PurPayment row is missing PurPaymentCode")
		}
		invoiceCode := sourceText(invoice)
		payloadBytes, err := json.Marshal(map[string]any{
			"PurPaymentCode": payment, "Date": normalizeValue(dateValue), "UserCode": user,
			"PurInvCode": invoice, "Posted": normalizeValue(posted), "WHTaxAccCode": account,
			"WHTaxPerc": normalizeValue(rate), "WHTaxBaseAmt": normalizeValue(base),
			"WHTaxAmt": normalizeValue(amount), "WHTaxCheckNo": checkNumber,
			"WHTaxRemarks": remarks, "SuppCode": supplier, "SuppInvCode": supplierInvoice,
		})
		if err != nil {
			return total, fmt.Errorf("marshal PurPayment %s: %w", paymentCode, err)
		}
		batch = append(batch, withholdingRow{
			legacyID: fmt.Sprintf("PurPayment:%s", paymentCode), paymentCode: paymentCode,
			purchaseInvoiceCode: invoiceCode, supplierLegacyID: sourceText(supplier), occurred: occurred,
			posted: sourceBool(posted), accountCode: sourceText(account), taxableBase: numeric(base),
			rate: numeric(rate), amount: numeric(amount), checkNumber: sourceText(checkNumber),
			remarks: sourceText(remarks), userLegacyID: sourceText(user), payload: string(payloadBytes),
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("withholding batch at %d: %w", total, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, fmt.Errorf("read PurPayment rows: %w", err)
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func withholdingValues(rows []withholdingRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		values[index] = []any{
			row.legacyID, row.paymentCode, row.purchaseInvoiceCode, row.supplierLegacyID,
			row.occurred, row.posted, row.accountCode, row.taxableBase, row.rate, row.amount,
			row.checkNumber, row.remarks, row.userLegacyID, row.payload,
		}
	}
	return values
}

type paymentAllocationRow struct {
	legacyID, paymentCode, counterpartyKind, partyLegacyID string
	sourceDocumentTable, sourceDocumentLegacyID            string
	occurred                                               time.Time
	posted                                                 bool
	paymentAmount, netAmount, outstandingAmount            string
	paymentMode, accountCode, paymentAccountCode           string
	checkNumber, reference, remarks, userLegacyID          string
	sourceTable, sourceTableRow, payload                   string
}

// importPayments retains source payment rows separately from the invoice
// party-ledger entry. It covers supplier PurPayment rows, installment receipt
// details, and direct SaleLedger/Purledger payment snapshots where the source
// has no child payment row. Unresolved party/document lookups remain in the
// target row with their legacy identities intact.
func importPayments(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_e_payment_batch (
		legacy_id text NOT NULL, payment_code text NOT NULL, counterparty_kind text NOT NULL,
		party_legacy_id text NOT NULL, source_document_table text NOT NULL,
		source_document_legacy_id text NOT NULL, occurred_at timestamptz NOT NULL,
		posted boolean NOT NULL, payment_amount numeric(19,4) NOT NULL,
		net_amount numeric(19,4) NOT NULL, outstanding_amount numeric(19,4) NOT NULL,
		payment_mode text NOT NULL, account_code text NOT NULL,
		payment_account_code text NOT NULL, check_number text NOT NULL,
		reference text NOT NULL, remarks text NOT NULL, user_legacy_id text NOT NULL,
		source_table text NOT NULL, source_table_row text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create payment staging table: %w", err)
	}
	var total int64
	batch := make([]paymentAllocationRow, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_e_payment_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_e_payment_batch"},
				[]string{
					"legacy_id", "payment_code", "counterparty_kind", "party_legacy_id",
					"source_document_table", "source_document_legacy_id", "occurred_at", "posted",
					"payment_amount", "net_amount", "outstanding_amount", "payment_mode",
					"account_code", "payment_account_code", "check_number", "reference", "remarks",
					"user_legacy_id", "source_table", "source_table_row", "payload",
				}, pgx.CopyFromRows(paymentAllocationValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_party_payment_allocations (
					tenant_id, branch_id, party_id, party_legacy_id, counterparty_kind,
					source_document_id, source_document_table, source_document_legacy_id,
					payment_code, payment_amount, net_amount, outstanding_amount, occurred_at,
					posted, payment_mode, account_code, payment_account_code, check_number,
					reference, remarks, user_legacy_id, source_table, source_table_row,
					source_legacy_id, payload)
				SELECT $1::uuid, $2::uuid, mp.id, s.party_legacy_id, s.counterparty_kind,
				       d.id, s.source_document_table, s.source_document_legacy_id,
				       s.payment_code, s.payment_amount, s.net_amount, s.outstanding_amount,
				       s.occurred_at, s.posted, s.payment_mode, s.account_code,
				       s.payment_account_code, s.check_number, s.reference, s.remarks,
				       s.user_legacy_id, s.source_table, s.source_table_row,
				       s.legacy_id, s.payload
				FROM phase_e_payment_batch s
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = $1::uuid
				 AND mp.party_type = s.counterparty_kind
				 AND mp.legacy_id = s.party_legacy_id
				LEFT JOIN business_documents d
				  ON d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
				 AND d.legacy_source_table = s.source_document_table
				 AND d.legacy_id = s.source_document_legacy_id
				ON CONFLICT (tenant_id, branch_id, source_legacy_id) DO UPDATE SET
					party_id = EXCLUDED.party_id,
					party_legacy_id = EXCLUDED.party_legacy_id,
					counterparty_kind = EXCLUDED.counterparty_kind,
					source_document_id = EXCLUDED.source_document_id,
					source_document_table = EXCLUDED.source_document_table,
					source_document_legacy_id = EXCLUDED.source_document_legacy_id,
					payment_code = EXCLUDED.payment_code,
					payment_amount = EXCLUDED.payment_amount,
					net_amount = EXCLUDED.net_amount,
					outstanding_amount = EXCLUDED.outstanding_amount,
					occurred_at = EXCLUDED.occurred_at,
					posted = EXCLUDED.posted,
					payment_mode = EXCLUDED.payment_mode,
					account_code = EXCLUDED.account_code,
					payment_account_code = EXCLUDED.payment_account_code,
					check_number = EXCLUDED.check_number,
					reference = EXCLUDED.reference,
					remarks = EXCLUDED.remarks,
					user_legacy_id = EXCLUDED.user_legacy_id,
					source_table = EXCLUDED.source_table,
					source_table_row = EXCLUDED.source_table_row,
					payload = EXCLUDED.payload,
					updated_at = now()`, tenantID, branchID)
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
	appendRow := func(row paymentAllocationRow) error {
		batch = append(batch, row)
		if len(batch) >= batchSize {
			return flush()
		}
		return nil
	}

	rows, err := source.QueryContext(ctx, `
		SELECT p.[PurPaymentCode], p.[Date], p.[UserCode], p.[PurInvCode],
		       p.[NetAmt], p.[OutstandingAmt], p.[AccCode], p.[Posted],
		       p.[PaymentMode], p.[PaymentAccCode], p.[PaymentAmt],
		       p.[PaymentCheckNo], p.[PaymentRemarks], p.[PostedBy], p.[PostDate],
		       p.[GLVochCode], l.[SuppCode], l.[SuppInvCode]
		FROM [dbo].[PurPayment] p
		LEFT JOIN [dbo].[Purledger] l ON l.[PurInvCode] = p.[PurInvCode]
		ORDER BY p.[PurPaymentCode]`)
	if err != nil {
		return 0, fmt.Errorf("read PurPayment payments: %w", err)
	}
	for rows.Next() {
		var payment, dateValue, user, invoice, net, outstanding, account, posted any
		var paymentMode, paymentAccount, paymentAmount, checkNumber, remarks, postedBy, postDate, voucher any
		var supplier, supplierInvoice any
		if err := rows.Scan(&payment, &dateValue, &user, &invoice, &net, &outstanding, &account, &posted,
			&paymentMode, &paymentAccount, &paymentAmount, &checkNumber, &remarks, &postedBy, &postDate, &voucher,
			&supplier, &supplierInvoice); err != nil {
			rows.Close()
			return total, err
		}
		occurred, err := sourceTime(dateValue)
		if err != nil {
			rows.Close()
			return total, err
		}
		paymentCode := sourceText(payment)
		if paymentCode == "" {
			rows.Close()
			return total, errors.New("PurPayment row is missing PurPaymentCode")
		}
		payloadBytes, err := json.Marshal(map[string]any{
			"PurPaymentCode": payment, "Date": normalizeValue(dateValue), "UserCode": user,
			"PurInvCode": invoice, "NetAmt": normalizeValue(net), "OutstandingAmt": normalizeValue(outstanding),
			"AccCode": account, "Posted": normalizeValue(posted), "PaymentMode": paymentMode,
			"PaymentAccCode": paymentAccount, "PaymentAmt": normalizeValue(paymentAmount),
			"PaymentCheckNo": checkNumber, "PaymentRemarks": remarks, "PostedBy": postedBy,
			"PostDate": normalizeValue(postDate), "GLVochCode": voucher,
			"SuppCode": supplier, "SuppInvCode": supplierInvoice,
		})
		if err != nil {
			rows.Close()
			return total, fmt.Errorf("marshal PurPayment %s: %w", paymentCode, err)
		}
		if err := appendRow(paymentAllocationRow{
			legacyID: fmt.Sprintf("PurPayment:%s", paymentCode), paymentCode: paymentCode,
			counterpartyKind: "supplier", partyLegacyID: sourceText(supplier),
			sourceDocumentTable: "Purledger", sourceDocumentLegacyID: sourceText(invoice),
			occurred: occurred, posted: sourceBool(posted), paymentAmount: numeric(paymentAmount),
			netAmount: numeric(net), outstandingAmount: numeric(outstanding),
			paymentMode: sourceText(paymentMode), accountCode: sourceText(account),
			paymentAccountCode: sourceText(paymentAccount), checkNumber: sourceText(checkNumber),
			remarks: sourceText(remarks), userLegacyID: sourceText(user),
			sourceTable: "PurPayment", sourceTableRow: paymentCode, payload: string(payloadBytes),
		}); err != nil {
			rows.Close()
			return total, fmt.Errorf("PurPayment batch at %d: %w", total, err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return total, fmt.Errorf("read PurPayment payment rows: %w", err)
	}
	rows.Close()

	rows, err = source.QueryContext(ctx, `
		SELECT d.[ReceiptCode], d.[InstallmentRowID], d.[ReceivedOn], r.[UserCode],
		       r.[SaleInvCode], r.[NetAmt], r.[OutstandingAmt],
		       d.[InstallmentAmount], d.[InstallmentBalance], d.[AmtReceived],
		       d.[ReceivedAccCode], d.[PaymentMode], d.[RefNo], d.[Remarks], r.[Remarks],
		       l.[CustCode], l.[Posted], l.[PaymentAccCode]
		FROM [dbo].[InstallmentReceiptDetail] d
		JOIN [dbo].[InstallmentReceipt] r
		  ON r.[ReceiptCode] = d.[ReceiptCode]
		 AND r.[InstallmentRowID] = d.[InstallmentRowID]
		LEFT JOIN [dbo].[SaleLedger] l ON l.[SaleInvCode] = r.[SaleInvCode]
		ORDER BY d.[ReceiptCode], d.[InstallmentRowID]`)
	if err != nil {
		return total, fmt.Errorf("read InstallmentReceiptDetail payments: %w", err)
	}
	for rows.Next() {
		var receipt, installmentRow, receivedOn, user, invoice, net, outstanding any
		var installmentAmount, installmentBalance, paymentAmount, account, paymentMode, reference any
		var detailRemarks, headerRemarks, customer, posted, paymentAccount any
		if err := rows.Scan(&receipt, &installmentRow, &receivedOn, &user, &invoice, &net, &outstanding,
			&installmentAmount, &installmentBalance, &paymentAmount, &account, &paymentMode, &reference,
			&detailRemarks, &headerRemarks, &customer, &posted, &paymentAccount); err != nil {
			rows.Close()
			return total, err
		}
		occurred, err := sourceTime(receivedOn)
		if err != nil {
			rows.Close()
			return total, err
		}
		receiptCode := sourceText(receipt)
		rowCode := sourceText(installmentRow)
		if receiptCode == "" || rowCode == "" {
			rows.Close()
			return total, errors.New("InstallmentReceiptDetail row is missing receipt identity")
		}
		remarksText := sourceText(detailRemarks)
		if remarksText == "" {
			remarksText = sourceText(headerRemarks)
		}
		paymentAccountText := sourceText(paymentAccount)
		if paymentAccountText == "" {
			paymentAccountText = sourceText(account)
		}
		payloadBytes, err := json.Marshal(map[string]any{
			"ReceiptCode": receipt, "InstallmentRowID": installmentRow, "ReceivedOn": normalizeValue(receivedOn),
			"UserCode": user, "SaleInvCode": invoice, "NetAmt": normalizeValue(net),
			"OutstandingAmt": normalizeValue(outstanding), "InstallmentAmount": normalizeValue(installmentAmount),
			"InstallmentBalance": normalizeValue(installmentBalance), "AmtReceived": normalizeValue(paymentAmount),
			"ReceivedAccCode": account, "PaymentMode": paymentMode, "RefNo": reference,
			"DetailRemarks": detailRemarks, "HeaderRemarks": headerRemarks, "CustCode": customer,
			"Posted": normalizeValue(posted), "PaymentAccCode": paymentAccount,
		})
		if err != nil {
			rows.Close()
			return total, fmt.Errorf("marshal installment receipt %s/%s: %w", receiptCode, rowCode, err)
		}
		if err := appendRow(paymentAllocationRow{
			legacyID:    fmt.Sprintf("InstallmentReceiptDetail:%s:%s", receiptCode, rowCode),
			paymentCode: receiptCode, counterpartyKind: "customer", partyLegacyID: sourceText(customer),
			sourceDocumentTable: "SaleLedger", sourceDocumentLegacyID: sourceText(invoice),
			occurred: occurred, posted: sourceBool(posted), paymentAmount: numeric(paymentAmount),
			netAmount: numeric(net), outstandingAmount: numeric(installmentBalance),
			paymentMode: sourceText(paymentMode), accountCode: sourceText(account),
			paymentAccountCode: paymentAccountText, reference: sourceText(reference), remarks: remarksText,
			userLegacyID: sourceText(user), sourceTable: "InstallmentReceiptDetail",
			sourceTableRow: fmt.Sprintf("%s:%s", receiptCode, rowCode), payload: string(payloadBytes),
		}); err != nil {
			rows.Close()
			return total, fmt.Errorf("installment receipt batch at %d: %w", total, err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return total, fmt.Errorf("read installment receipt rows: %w", err)
	}
	rows.Close()

	rows, err = source.QueryContext(ctx, `
		SELECT l.[SaleInvCode], COALESCE(l.[AmtDate], l.[date]), l.[PostedBy], l.[CustCode],
		       l.[Amt], l.[OutstandingAmt], l.[PaymentMode], l.[PaymentAccCode],
		       l.[CashAccCode], l.[AmtReference], l.[Remarks], l.[Posted], l.[CustRefNo],
		       l.[CashReceived], l.[CashTendered], l.[CashBack]
		FROM [dbo].[SaleLedger] l
		WHERE l.[Amt] <> 0
		  AND l.[CustCode] <> 19
		  AND NOT EXISTS (
			SELECT 1 FROM [dbo].[InstallmentReceipt] r
			WHERE r.[SaleInvCode] = l.[SaleInvCode]
		  )
		ORDER BY l.[SaleInvCode]`)
	if err != nil {
		return total, fmt.Errorf("read SaleLedger payment snapshots: %w", err)
	}
	for rows.Next() {
		var invoice, occurredValue, postedBy, customer, paymentAmount, outstanding any
		var paymentMode, paymentAccount, cashAccount, reference, remarks, posted any
		var customerReference, cashReceived, cashTendered, cashBack any
		if err := rows.Scan(&invoice, &occurredValue, &postedBy, &customer, &paymentAmount, &outstanding,
			&paymentMode, &paymentAccount, &cashAccount, &reference, &remarks, &posted, &customerReference,
			&cashReceived, &cashTendered, &cashBack); err != nil {
			rows.Close()
			return total, err
		}
		occurred, err := sourceTime(occurredValue)
		if err != nil {
			rows.Close()
			return total, err
		}
		invoiceCode := sourceText(invoice)
		if invoiceCode == "" {
			rows.Close()
			return total, errors.New("SaleLedger payment row is missing SaleInvCode")
		}
		paymentAccountText := sourceText(paymentAccount)
		if paymentAccountText == "" {
			paymentAccountText = sourceText(cashAccount)
		}
		payloadBytes, err := json.Marshal(map[string]any{
			"SaleInvCode": invoice, "PaymentDate": normalizeValue(occurredValue), "PostedBy": postedBy,
			"CustCode": customer, "Amt": normalizeValue(paymentAmount), "OutstandingAmt": normalizeValue(outstanding),
			"PaymentMode": paymentMode, "PaymentAccCode": paymentAccount, "CashAccCode": cashAccount,
			"AmtReference": reference, "Remarks": remarks, "Posted": normalizeValue(posted),
			"CustRefNo": customerReference, "CashReceived": normalizeValue(cashReceived),
			"CashTendered": normalizeValue(cashTendered), "CashBack": normalizeValue(cashBack),
		})
		if err != nil {
			rows.Close()
			return total, fmt.Errorf("marshal SaleLedger payment %s: %w", invoiceCode, err)
		}
		if err := appendRow(paymentAllocationRow{
			legacyID: fmt.Sprintf("SaleLedger:%s", invoiceCode), paymentCode: invoiceCode,
			counterpartyKind: "customer", partyLegacyID: sourceText(customer),
			sourceDocumentTable: "SaleLedger", sourceDocumentLegacyID: invoiceCode,
			occurred: occurred, posted: sourceBool(posted), paymentAmount: numeric(paymentAmount),
			netAmount: numeric(paymentAmount), outstandingAmount: numeric(outstanding),
			paymentMode: sourceText(paymentMode), accountCode: paymentAccountText,
			paymentAccountCode: paymentAccountText, reference: sourceText(reference), remarks: sourceText(remarks),
			userLegacyID: sourceText(postedBy), sourceTable: "SaleLedger", sourceTableRow: invoiceCode,
			payload: string(payloadBytes),
		}); err != nil {
			rows.Close()
			return total, fmt.Errorf("SaleLedger payment batch at %d: %w", total, err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return total, fmt.Errorf("read SaleLedger payment rows: %w", err)
	}
	rows.Close()

	rows, err = source.QueryContext(ctx, `
		SELECT l.[PurInvCode], COALESCE(l.[AmtDate], l.[Date]), l.[PostedBy], l.[SuppCode],
		       l.[Amt], l.[OutstandingAmt], l.[PaymentMode], l.[PaymentAccCode],
		       l.[AmtReference], l.[Remarks], l.[Posted], l.[SuppInvCode]
		FROM [dbo].[Purledger] l
		WHERE l.[Amt] <> 0
		  AND NOT EXISTS (
			SELECT 1 FROM [dbo].[PurPayment] p
			WHERE p.[PurInvCode] = l.[PurInvCode]
		  )
		ORDER BY l.[PurInvCode]`)
	if err != nil {
		return total, fmt.Errorf("read Purledger payment snapshots: %w", err)
	}
	for rows.Next() {
		var invoice, occurredValue, postedBy, supplier, paymentAmount, outstanding any
		var paymentMode, paymentAccount, reference, remarks, posted, supplierInvoice any
		if err := rows.Scan(&invoice, &occurredValue, &postedBy, &supplier, &paymentAmount, &outstanding,
			&paymentMode, &paymentAccount, &reference, &remarks, &posted, &supplierInvoice); err != nil {
			rows.Close()
			return total, err
		}
		occurred, err := sourceTime(occurredValue)
		if err != nil {
			rows.Close()
			return total, err
		}
		invoiceCode := sourceText(invoice)
		if invoiceCode == "" {
			rows.Close()
			return total, errors.New("Purledger payment row is missing PurInvCode")
		}
		payloadBytes, err := json.Marshal(map[string]any{
			"PurInvCode": invoice, "PaymentDate": normalizeValue(occurredValue), "PostedBy": postedBy,
			"SuppCode": supplier, "Amt": normalizeValue(paymentAmount), "OutstandingAmt": normalizeValue(outstanding),
			"PaymentMode": paymentMode, "PaymentAccCode": paymentAccount, "AmtReference": reference,
			"Remarks": remarks, "Posted": normalizeValue(posted), "SuppInvCode": supplierInvoice,
		})
		if err != nil {
			rows.Close()
			return total, fmt.Errorf("marshal Purledger payment %s: %w", invoiceCode, err)
		}
		if err := appendRow(paymentAllocationRow{
			legacyID: fmt.Sprintf("Purledger:%s", invoiceCode), paymentCode: invoiceCode,
			counterpartyKind: "supplier", partyLegacyID: sourceText(supplier),
			sourceDocumentTable: "Purledger", sourceDocumentLegacyID: invoiceCode,
			occurred: occurred, posted: sourceBool(posted), paymentAmount: numeric(paymentAmount),
			netAmount: numeric(paymentAmount), outstandingAmount: numeric(outstanding),
			paymentMode: sourceText(paymentMode), accountCode: sourceText(paymentAccount),
			paymentAccountCode: sourceText(paymentAccount), reference: sourceText(reference), remarks: sourceText(remarks),
			userLegacyID: sourceText(postedBy), sourceTable: "Purledger", sourceTableRow: invoiceCode,
			payload: string(payloadBytes),
		}); err != nil {
			rows.Close()
			return total, fmt.Errorf("Purledger payment batch at %d: %w", total, err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return total, fmt.Errorf("read Purledger payment rows: %w", err)
	}
	rows.Close()
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func paymentAllocationValues(rows []paymentAllocationRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		values[index] = []any{
			row.legacyID, row.paymentCode, row.counterpartyKind, row.partyLegacyID,
			row.sourceDocumentTable, row.sourceDocumentLegacyID, row.occurred, row.posted,
			row.paymentAmount, row.netAmount, row.outstandingAmount, row.paymentMode,
			row.accountCode, row.paymentAccountCode, row.checkNumber, row.reference,
			row.remarks, row.userLegacyID, row.sourceTable, row.sourceTableRow, row.payload,
		}
	}
	return values
}

type partyAdjustmentRow struct {
	legacyID, partyLegacyID, counterpartyKind                    string
	sourceDocumentTable, sourceDocumentLegacyID                  string
	occurred                                                     *time.Time
	posted                                                       bool
	debitAmount, creditAmount, accountCode, checkNumber, remarks string
	userLegacyID, sourceTable, sourceTableRow, payload           string
}

// importPartyAdjustments retains SaleReceivableAdj as a distinct debit/credit
// stream. It does not reinterpret an adjustment as a payment and keeps rows
// whose SaleLedger parent cannot be resolved, including a nullable source date.
func importPartyAdjustments(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_e_party_adjustment_batch (
		legacy_id text NOT NULL, party_legacy_id text NOT NULL, counterparty_kind text NOT NULL,
		source_document_table text NOT NULL, source_document_legacy_id text NOT NULL,
		occurred_at timestamptz, posted boolean NOT NULL, debit_amount numeric(19,4) NOT NULL,
		credit_amount numeric(19,4) NOT NULL, account_code text NOT NULL, check_number text NOT NULL,
		remarks text NOT NULL, user_legacy_id text NOT NULL, source_table text NOT NULL,
		source_table_row text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create party adjustment staging table: %w", err)
	}
	var total int64
	batch := make([]partyAdjustmentRow, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_e_party_adjustment_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_e_party_adjustment_batch"},
				[]string{
					"legacy_id", "party_legacy_id", "counterparty_kind", "source_document_table",
					"source_document_legacy_id", "occurred_at", "posted", "debit_amount", "credit_amount",
					"account_code", "check_number", "remarks", "user_legacy_id", "source_table",
					"source_table_row", "payload",
				}, pgx.CopyFromRows(partyAdjustmentValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_party_ledger_adjustments (
					tenant_id, branch_id, party_id, party_legacy_id, counterparty_kind,
					source_document_id, source_document_table, source_document_legacy_id,
					debit_amount, credit_amount, occurred_at, posted, account_code,
					check_number, remarks, user_legacy_id, source_table, source_table_row,
					source_legacy_id, payload)
				SELECT $1::uuid, $2::uuid, mp.id, s.party_legacy_id, s.counterparty_kind,
				       d.id, s.source_document_table, s.source_document_legacy_id,
				       s.debit_amount, s.credit_amount, s.occurred_at, s.posted,
				       s.account_code, s.check_number, s.remarks, s.user_legacy_id,
				       s.source_table, s.source_table_row, s.legacy_id, s.payload
				FROM phase_e_party_adjustment_batch s
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = $1::uuid
				 AND mp.party_type = s.counterparty_kind
				 AND mp.legacy_id = s.party_legacy_id
				LEFT JOIN business_documents d
				  ON d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
				 AND d.legacy_source_table = s.source_document_table
				 AND d.legacy_id = s.source_document_legacy_id
				ON CONFLICT (tenant_id, branch_id, source_legacy_id) DO UPDATE SET
					party_id = EXCLUDED.party_id,
					party_legacy_id = EXCLUDED.party_legacy_id,
					counterparty_kind = EXCLUDED.counterparty_kind,
					source_document_id = EXCLUDED.source_document_id,
					source_document_table = EXCLUDED.source_document_table,
					source_document_legacy_id = EXCLUDED.source_document_legacy_id,
					debit_amount = EXCLUDED.debit_amount,
					credit_amount = EXCLUDED.credit_amount,
					occurred_at = EXCLUDED.occurred_at,
					posted = EXCLUDED.posted,
					account_code = EXCLUDED.account_code,
					check_number = EXCLUDED.check_number,
					remarks = EXCLUDED.remarks,
					user_legacy_id = EXCLUDED.user_legacy_id,
					source_table = EXCLUDED.source_table,
					source_table_row = EXCLUDED.source_table_row,
					payload = EXCLUDED.payload,
					updated_at = now()`, tenantID, branchID)
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
	appendRow := func(row partyAdjustmentRow) error {
		batch = append(batch, row)
		if len(batch) >= batchSize {
			return flush()
		}
		return nil
	}

	rows, err := source.QueryContext(ctx, `
		SELECT a.[SaleInvCode], a.[AccCode], a.[Debit], a.[Credit], a.[CheckNo],
		       a.[Remarks], a.[ROWID], l.[date], l.[CustCode], l.[Posted], l.[PostedBy]
		FROM [dbo].[SaleReceivableAdj] a
		LEFT JOIN [dbo].[SaleLedger] l ON l.[SaleInvCode] = a.[SaleInvCode]
		ORDER BY a.[SaleInvCode], a.[ROWID]`)
	if err != nil {
		return 0, fmt.Errorf("read SaleReceivableAdj: %w", err)
	}
	for rows.Next() {
		var invoice, account, debit, credit, checkNumber, remarks, rowID any
		var dateValue, customer, posted, postedBy any
		if err := rows.Scan(&invoice, &account, &debit, &credit, &checkNumber, &remarks, &rowID,
			&dateValue, &customer, &posted, &postedBy); err != nil {
			rows.Close()
			return total, err
		}
		occurred, err := sourceOptionalTime(dateValue)
		if err != nil {
			rows.Close()
			return total, err
		}
		invoiceCode := sourceText(invoice)
		rowCode := sourceText(rowID)
		if rowCode == "" {
			rows.Close()
			return total, errors.New("SaleReceivableAdj row is missing ROWID")
		}
		payloadBytes, err := json.Marshal(map[string]any{
			"SaleInvCode": invoice, "AccCode": account, "Debit": normalizeValue(debit),
			"Credit": normalizeValue(credit), "CheckNo": checkNumber, "Remarks": remarks,
			"ROWID": rowID, "SaleDate": normalizeValue(dateValue), "CustCode": customer,
			"Posted": normalizeValue(posted), "PostedBy": postedBy,
		})
		if err != nil {
			rows.Close()
			return total, fmt.Errorf("marshal SaleReceivableAdj %s/%s: %w", invoiceCode, rowCode, err)
		}
		if err := appendRow(partyAdjustmentRow{
			legacyID:      fmt.Sprintf("SaleReceivableAdj:%s:%s", invoiceCode, rowCode),
			partyLegacyID: sourceText(customer), counterpartyKind: "customer",
			sourceDocumentTable: "SaleLedger", sourceDocumentLegacyID: invoiceCode,
			occurred: occurred, posted: sourceBool(posted), debitAmount: numeric(debit),
			creditAmount: numeric(credit), accountCode: sourceText(account),
			checkNumber: sourceText(checkNumber), remarks: sourceText(remarks),
			userLegacyID: sourceText(postedBy), sourceTable: "SaleReceivableAdj",
			sourceTableRow: fmt.Sprintf("%s:%s", invoiceCode, rowCode), payload: string(payloadBytes),
		}); err != nil {
			rows.Close()
			return total, fmt.Errorf("SaleReceivableAdj batch at %d: %w", total, err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return total, fmt.Errorf("read SaleReceivableAdj rows: %w", err)
	}
	rows.Close()
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func partyAdjustmentValues(rows []partyAdjustmentRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		var occurred any
		if row.occurred != nil {
			occurred = *row.occurred
		}
		values[index] = []any{
			row.legacyID, row.partyLegacyID, row.counterpartyKind,
			row.sourceDocumentTable, row.sourceDocumentLegacyID, occurred, row.posted,
			row.debitAmount, row.creditAmount, row.accountCode, row.checkNumber,
			row.remarks, row.userLegacyID, row.sourceTable, row.sourceTableRow, row.payload,
		}
	}
	return values
}

type partyReturnAllocationRow struct {
	legacyID, partyLegacyID, counterpartyKind, returnKind          string
	returnSourceTable, returnSourceLegacyID, sourceDocumentTable   string
	sourceDocumentLegacyID, allocationCode, allocationRowID        string
	occurred                                                       time.Time
	posted                                                         bool
	allocationAmount, outstandingAmount, debitAmount, creditAmount string
	userLegacyID, sourceTable, sourceTableRow, payload             string
}

// importPartyReturnAllocations retains SR/PR allocation detail as an explicit
// historical stream. It does not reinterpret these rows as payments and does
// not update canonical document balances or aging projections.
func importPartyReturnAllocations(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_e_party_return_allocation_batch (
		legacy_id text NOT NULL, party_legacy_id text NOT NULL, counterparty_kind text NOT NULL,
		return_kind text NOT NULL, return_source_table text NOT NULL, return_source_legacy_id text NOT NULL,
		source_document_table text NOT NULL, source_document_legacy_id text NOT NULL,
		allocation_code text NOT NULL, allocation_row_id text NOT NULL, occurred_at timestamptz NOT NULL,
		posted boolean NOT NULL, allocation_amount numeric(19,4) NOT NULL, outstanding_amount numeric(19,4) NOT NULL,
		debit_amount numeric(19,4) NOT NULL, credit_amount numeric(19,4) NOT NULL,
		user_legacy_id text NOT NULL, source_table text NOT NULL, source_table_row text NOT NULL,
		payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create party return allocation staging table: %w", err)
	}
	var total int64
	batch := make([]partyReturnAllocationRow, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_e_party_return_allocation_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_e_party_return_allocation_batch"},
				[]string{
					"legacy_id", "party_legacy_id", "counterparty_kind", "return_kind",
					"return_source_table", "return_source_legacy_id", "source_document_table",
					"source_document_legacy_id", "allocation_code", "allocation_row_id", "occurred_at",
					"posted", "allocation_amount", "outstanding_amount", "debit_amount", "credit_amount",
					"user_legacy_id", "source_table", "source_table_row", "payload",
				}, pgx.CopyFromRows(partyReturnAllocationValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_party_return_allocations (
					tenant_id, branch_id, party_id, party_legacy_id, counterparty_kind, return_kind,
					return_document_id, return_source_table, return_source_legacy_id,
					source_document_id, source_document_table, source_document_legacy_id,
					allocation_code, allocation_row_id, allocation_amount, outstanding_amount,
					debit_amount, credit_amount, occurred_at, posted, user_legacy_id, source_table,
					source_table_row, source_legacy_id, payload)
				SELECT $1::uuid, $2::uuid, mp.id, s.party_legacy_id, s.counterparty_kind, s.return_kind,
				       NULL, s.return_source_table, s.return_source_legacy_id,
				       d.id, s.source_document_table, s.source_document_legacy_id,
				       s.allocation_code, s.allocation_row_id, s.allocation_amount, s.outstanding_amount,
				       s.debit_amount, s.credit_amount, s.occurred_at, s.posted, s.user_legacy_id,
				       s.source_table, s.source_table_row, s.legacy_id, s.payload
				FROM phase_e_party_return_allocation_batch s
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = $1::uuid
				 AND mp.party_type = s.counterparty_kind
				 AND mp.legacy_id = s.party_legacy_id
				LEFT JOIN business_documents d
				  ON d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
				 AND d.legacy_source_table = s.source_document_table
				 AND d.legacy_id = s.source_document_legacy_id
				ON CONFLICT (tenant_id, branch_id, source_legacy_id) DO UPDATE SET
					party_id = EXCLUDED.party_id,
					party_legacy_id = EXCLUDED.party_legacy_id,
					counterparty_kind = EXCLUDED.counterparty_kind,
					return_kind = EXCLUDED.return_kind,
					return_source_table = EXCLUDED.return_source_table,
					return_source_legacy_id = EXCLUDED.return_source_legacy_id,
					source_document_id = EXCLUDED.source_document_id,
					source_document_table = EXCLUDED.source_document_table,
					source_document_legacy_id = EXCLUDED.source_document_legacy_id,
					allocation_code = EXCLUDED.allocation_code,
					allocation_row_id = EXCLUDED.allocation_row_id,
					allocation_amount = EXCLUDED.allocation_amount,
					outstanding_amount = EXCLUDED.outstanding_amount,
					debit_amount = EXCLUDED.debit_amount,
					credit_amount = EXCLUDED.credit_amount,
					occurred_at = EXCLUDED.occurred_at,
					posted = EXCLUDED.posted,
					user_legacy_id = EXCLUDED.user_legacy_id,
					source_table = EXCLUDED.source_table,
					source_table_row = EXCLUDED.source_table_row,
					payload = EXCLUDED.payload,
					updated_at = now()`, tenantID, branchID)
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
	appendRow := func(row partyReturnAllocationRow) error {
		batch = append(batch, row)
		if len(batch) >= batchSize {
			return flush()
		}
		return nil
	}

	process := func(rows *sql.Rows, returnKind, counterpartyKind, returnSourceTable, detailTable, invoiceTable string, isSale bool) error {
		defer rows.Close()
		for rows.Next() {
			var allocationCode, occurredValue, returnInvoice, user, posted any
			var invoice, outstanding, amount, rowID, utn, party any
			if err := rows.Scan(&allocationCode, &occurredValue, &returnInvoice, &user, &posted,
				&invoice, &outstanding, &amount, &rowID, &utn, &party); err != nil {
				return err
			}
			occurred, err := sourceTime(occurredValue)
			if err != nil {
				return err
			}
			allocationCodeText := sourceText(allocationCode)
			rowIDText := sourceText(rowID)
			if allocationCodeText == "" || rowIDText == "" {
				return errors.New("party return allocation row is missing allocation code or row identity")
			}
			invoiceCode := sourceText(invoice)
			returnInvoiceCode := sourceText(returnInvoice)
			amountText := numeric(amount)
			debitAmount, creditAmount := "0", "0"
			if isSale {
				creditAmount = amountText
			} else {
				debitAmount = amountText
			}
			payloadBytes, err := json.Marshal(map[string]any{
				"AllocationCode": allocationCode, "ReturnInvoiceCode": returnInvoice,
				"InvoiceCode": invoice, "OutstandingAmt": normalizeValue(outstanding),
				"Amt": normalizeValue(amount), "AllocationROWID": rowID, "AllocationUTN": utn,
				"Date": normalizeValue(occurredValue), "UserCode": user, "Posted": normalizeValue(posted),
				"PartyCode": party,
			})
			if err != nil {
				return err
			}
			legacyID := fmt.Sprintf("%sAllocationDetail:%s:%s", returnKind, allocationCodeText, rowIDText)
			if err := appendRow(partyReturnAllocationRow{
				legacyID: legacyID, partyLegacyID: sourceText(party), counterpartyKind: counterpartyKind,
				returnKind: returnKind, returnSourceTable: returnSourceTable, returnSourceLegacyID: returnInvoiceCode,
				sourceDocumentTable: invoiceTable, sourceDocumentLegacyID: invoiceCode,
				allocationCode: allocationCodeText, allocationRowID: rowIDText, occurred: occurred,
				posted: sourceBool(posted), allocationAmount: amountText, outstandingAmount: numeric(outstanding),
				debitAmount: debitAmount, creditAmount: creditAmount, userLegacyID: sourceText(user),
				sourceTable: detailTable, sourceTableRow: allocationCodeText + ":" + rowIDText,
				payload: string(payloadBytes),
			}); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	rows, err := source.QueryContext(ctx, `
		SELECT h.[SRAllocationCode], h.[Date], h.[SRInvCode], h.[UserCode], h.[Posted],
		       d.[SaleInvCode], d.[OutstandingAmt], d.[Amt], d.[SRAllocationROWID],
		       d.[SRAllocationUTN], l.[CustCode]
		FROM [dbo].[SRAllocationHeader] h
		JOIN [dbo].[SRAllocationDetail] d ON d.[SRAllocationCode] = h.[SRAllocationCode]
		LEFT JOIN [dbo].[SaleLedger] l ON l.[SaleInvCode] = d.[SaleInvCode]
		ORDER BY h.[SRAllocationCode], d.[SRAllocationROWID]`)
	if err != nil {
		return 0, fmt.Errorf("read SRAllocationHeader/Detail: %w", err)
	}
	if err := process(rows, "sale", "customer", "SRAllocationHeader", "SRAllocationDetail", "SaleLedger", true); err != nil {
		return total, fmt.Errorf("process SRAllocationHeader/Detail: %w", err)
	}

	rows, err = source.QueryContext(ctx, `
		SELECT h.[PRAllocationCode], h.[Date], h.[PRInvCode], h.[UserCode], h.[Posted],
		       d.[PurInvCode], d.[OutstandingAmt], d.[Amt], d.[PRAllocationROWID],
		       d.[PRAllocationUTN], l.[SuppCode]
		FROM [dbo].[PRAllocationHeader] h
		JOIN [dbo].[PRAllocationDetail] d ON d.[PRAllocationCode] = h.[PRAllocationCode]
		LEFT JOIN [dbo].[Purledger] l ON l.[PurInvCode] = d.[PurInvCode]
		ORDER BY h.[PRAllocationCode], d.[PRAllocationROWID]`)
	if err != nil {
		return total, fmt.Errorf("read PRAllocationHeader/Detail: %w", err)
	}
	if err := process(rows, "purchase", "supplier", "PRAllocationHeader", "PRAllocationDetail", "Purledger", false); err != nil {
		return total, fmt.Errorf("process PRAllocationHeader/Detail: %w", err)
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func partyReturnAllocationValues(rows []partyReturnAllocationRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		values[index] = []any{
			row.legacyID, row.partyLegacyID, row.counterpartyKind, row.returnKind,
			row.returnSourceTable, row.returnSourceLegacyID, row.sourceDocumentTable,
			row.sourceDocumentLegacyID, row.allocationCode, row.allocationRowID, row.occurred,
			row.posted, row.allocationAmount, row.outstandingAmount, row.debitAmount,
			row.creditAmount, row.userLegacyID, row.sourceTable, row.sourceTableRow, row.payload,
		}
	}
	return values
}

type itemLogSnapshot struct {
	name, salePrice, newSalePrice, basicFingerprint string
}

type itemHistoryRow struct {
	reportKind, sourceLegacyID, itemLegacyID string
	occurred                                 time.Time
	oldName, newName                         string
	oldSalePrice, newSalePrice, stock        string
	userLegacyID, changeReason, payload      string
}

func importItemHistory(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT *
		FROM [dbo].[ItemLog]
		ORDER BY [ICode], [LogDate], [ItemRowID]`)
	if err != nil {
		return 0, fmt.Errorf("read ItemLog: %w", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("read ItemLog columns: %w", err)
	}
	columnIndex := make(map[string]int, len(columns))
	for index, column := range columns {
		columnIndex[strings.ToLower(column)] = index
	}
	for _, required := range []string{"itemrowid", "logdate", "icode", "name", "saleprice", "newsaleprice", "usercode", "changereason"} {
		if _, ok := columnIndex[required]; !ok {
			return 0, fmt.Errorf("ItemLog is missing required column %q", required)
		}
	}
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_q_item_history_batch (
		report_kind text NOT NULL, source_legacy_id text NOT NULL, item_legacy_id text NOT NULL,
		occurred_at timestamptz NOT NULL, old_name text NOT NULL, new_name text NOT NULL,
		old_sale_price numeric(19,4), new_sale_price numeric(19,4), stock numeric(19,4),
		user_legacy_id text NOT NULL, change_reason text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create item history staging table: %w", err)
	}
	previous := make(map[string]itemLogSnapshot)
	batch := make([]itemHistoryRow, 0, batchSize)
	var total int64
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_q_item_history_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_q_item_history_batch"},
				[]string{"report_kind", "source_legacy_id", "item_legacy_id", "occurred_at", "old_name", "new_name", "old_sale_price", "new_sale_price", "stock", "user_legacy_id", "change_reason", "payload"},
				pgx.CopyFromRows(itemHistoryValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_item_changes (
					tenant_id, branch_id, report_kind, source_legacy_id, item_id,
					item_legacy_id, occurred_at, old_name, new_name, old_sale_price,
					new_sale_price, stock, user_legacy_id, change_reason, source_table,
					source_table_row, payload
				)
				SELECT $1::uuid, $2::uuid, s.report_kind, s.source_legacy_id, i.id,
				       s.item_legacy_id, s.occurred_at, s.old_name, s.new_name,
				       s.old_sale_price, s.new_sale_price, s.stock, s.user_legacy_id,
				       s.change_reason, 'ItemLog', s.source_legacy_id, s.payload
				FROM phase_q_item_history_batch s
				LEFT JOIN master_items i
				  ON i.tenant_id = $1::uuid AND i.legacy_id = s.item_legacy_id
				ON CONFLICT (tenant_id, branch_id, report_kind, source_legacy_id) DO UPDATE SET
					item_id = EXCLUDED.item_id, item_legacy_id = EXCLUDED.item_legacy_id,
					occurred_at = EXCLUDED.occurred_at, old_name = EXCLUDED.old_name,
					new_name = EXCLUDED.new_name, old_sale_price = EXCLUDED.old_sale_price,
					new_sale_price = EXCLUDED.new_sale_price, stock = EXCLUDED.stock,
					user_legacy_id = EXCLUDED.user_legacy_id,
					change_reason = EXCLUDED.change_reason, payload = EXCLUDED.payload,
					updated_at = now()`, tenantID, branchID)
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
		values := make([]any, len(columns))
		pointers := make([]any, len(values))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return total, fmt.Errorf("scan ItemLog row: %w", err)
		}
		itemLegacyID := sourceColumnText(values, columnIndex, "icode")
		if itemLegacyID == "" {
			return total, errors.New("ItemLog row has an empty ICode")
		}
		occurred, err := sourceTime(sourceColumn(values, columnIndex, "logdate"))
		if err != nil {
			return total, fmt.Errorf("ItemLog %s date: %w", itemLegacyID, err)
		}
		current := itemLogSnapshot{
			name:             sourceColumnText(values, columnIndex, "name"),
			salePrice:        sourceColumnText(values, columnIndex, "saleprice"),
			newSalePrice:     sourceColumnText(values, columnIndex, "newsaleprice"),
			basicFingerprint: itemLogBasicFingerprint(values, columnIndex),
		}
		previousSnapshot, hasPrevious := previous[itemLegacyID]
		priceDifference := current.newSalePrice != "" && current.salePrice != "" && !numericTextEqual(current.newSalePrice, current.salePrice)
		priceChanged := priceDifference || (hasPrevious && !numericTextEqual(current.salePrice, previousSnapshot.salePrice))
		nameChanged := hasPrevious && current.name != previousSnapshot.name
		basicChanged := hasPrevious && current.basicFingerprint != previousSnapshot.basicFingerprint
		oldName := ""
		oldSalePrice := ""
		if hasPrevious {
			oldName = previousSnapshot.name
			oldSalePrice = previousSnapshot.salePrice
		}
		newSalePrice := current.newSalePrice
		if newSalePrice == "" {
			newSalePrice = current.salePrice
		}
		payload := make(map[string]any, len(columns))
		for index, column := range columns {
			payload[column] = normalizeValue(values[index])
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return total, fmt.Errorf("marshal ItemLog %s: %w", itemLegacyID, err)
		}
		base := itemHistoryRow{
			sourceLegacyID: sourceColumnText(values, columnIndex, "itemrowid"),
			itemLegacyID:   itemLegacyID,
			occurred:       occurred,
			oldName:        oldName,
			newName:        current.name,
			oldSalePrice:   oldSalePrice,
			newSalePrice:   newSalePrice,
			stock:          sourceColumnText(values, columnIndex, "stock"),
			userLegacyID:   sourceColumnText(values, columnIndex, "usercode"),
			changeReason:   sourceColumnText(values, columnIndex, "changereason"),
			payload:        string(payloadBytes),
		}
		if base.sourceLegacyID == "" {
			return total, fmt.Errorf("ItemLog row for %s has an empty ItemRowID", itemLegacyID)
		}
		if !hasPrevious {
			base.reportKind = "item-first-observed"
			batch = append(batch, base)
		}
		if priceDifference {
			row := base
			row.reportKind = "item-price-difference"
			batch = append(batch, row)
		}
		if priceChanged {
			row := base
			row.reportKind = "item-sale-price"
			batch = append(batch, row)
		}
		if basicChanged {
			row := base
			row.reportKind = "item-basic-data"
			batch = append(batch, row)
		}
		if nameChanged {
			row := base
			row.reportKind = "item-name"
			batch = append(batch, row)
		}
		previous[itemLegacyID] = current
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("ItemLog batch at %d: %w", total, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, fmt.Errorf("read ItemLog rows: %w", err)
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func itemHistoryValues(rows []itemHistoryRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		values[index] = []any{
			row.reportKind, row.sourceLegacyID, row.itemLegacyID, row.occurred,
			row.oldName, row.newName, nullableText(row.oldSalePrice),
			nullableText(row.newSalePrice), nullableText(row.stock), row.userLegacyID,
			row.changeReason, row.payload,
		}
	}
	return values
}

func itemLogBasicFingerprint(values []any, columns map[string]int) string {
	parts := make([]string, 0, 18)
	for _, column := range []string{
		"customicode", "stock", "active", "iccode", "icatcode", "packcode",
		"manfcode", "packunits", "location1", "remarks1", "reorderqty",
		"optimumqty", "restricted", "taxable", "itemtypecode", "measureunitcode",
		"genericcode", "gcode",
	} {
		parts = append(parts, column+"="+sourceColumnText(values, columns, column))
	}
	return strings.Join(parts, "|")
}

type deletedSaleItemRow struct {
	legacyID, itemLegacyID, godownLegacyID     string
	packUnits                                  int64
	quantity, bonusQuantity, salePrice         string
	discountPercent, itemFlatDiscount          string
	unitSalesTax, gstPercent                   string
	occurred                                   time.Time
	machineName, userLegacyID, saleInvoiceCode string
	sourceTableRow, payload                    string
}

// importDeletedSaleItems retains the legacy DeletedSaleItem audit stream in a
// separate target table. It never attempts to recreate a live sale line and it
// refuses rows without the source item/godown identities needed to audit them.
func importDeletedSaleItems(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT [ICode], [GCode], [PackUnits], [Qty], [BonusQty], [SalePrice],
		       [DiscPerc], [ItemFlatDisc], [UnitSalesTax], [GSTPerc], [Date],
		       [MachineName], [UserCode], [SaleInvCode],
		       ROW_NUMBER() OVER (
			       ORDER BY [Date], COALESCE([SaleInvCode], 0), [ICode], [GCode],
			                [UserCode], [MachineName]
		       ) AS [SourceRow]
		FROM [dbo].[DeletedSaleItem]`)
	if err != nil {
		return 0, fmt.Errorf("read DeletedSaleItem: %w", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("read DeletedSaleItem columns: %w", err)
	}
	columnIndex := make(map[string]int, len(columns))
	for index, column := range columns {
		columnIndex[strings.ToLower(column)] = index
	}
	for _, required := range []string{
		"icode", "gcode", "packunits", "qty", "bonusqty", "saleprice",
		"discperc", "itemflatdisc", "unitsalestax", "gstperc", "date",
		"machinename", "usercode", "saleinvcode", "sourcerow",
	} {
		if _, ok := columnIndex[required]; !ok {
			return 0, fmt.Errorf("DeletedSaleItem is missing required column %q", required)
		}
	}
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_q_deleted_sale_items_batch (
		legacy_id text NOT NULL, item_legacy_id text NOT NULL, godown_legacy_id text NOT NULL,
		pack_units integer NOT NULL, quantity numeric(19,4) NOT NULL,
		bonus_quantity numeric(19,4) NOT NULL, sale_price numeric(19,4) NOT NULL,
		discount_percent numeric(19,4) NOT NULL, item_flat_discount numeric(19,4) NOT NULL,
		unit_sales_tax numeric(19,4) NOT NULL, gst_percent numeric(19,4) NOT NULL,
		occurred_at timestamptz NOT NULL, machine_name text NOT NULL,
		user_legacy_id text NOT NULL, sale_invoice_code text NOT NULL,
		source_table_row text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create deleted-sale-item staging table: %w", err)
	}
	batch := make([]deletedSaleItemRow, 0, batchSize)
	var total int64
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_q_deleted_sale_items_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_q_deleted_sale_items_batch"},
				[]string{
					"legacy_id", "item_legacy_id", "godown_legacy_id", "pack_units",
					"quantity", "bonus_quantity", "sale_price", "discount_percent",
					"item_flat_discount", "unit_sales_tax", "gst_percent", "occurred_at",
					"machine_name", "user_legacy_id", "sale_invoice_code", "source_table_row", "payload",
				}, pgx.CopyFromRows(deletedSaleItemValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_deleted_sale_items (
					tenant_id, branch_id, legacy_id, item_id, item_legacy_id, godown_legacy_id,
					pack_units, quantity, bonus_quantity, sale_price, discount_percent,
					item_flat_discount, unit_sales_tax, gst_percent, occurred_at, machine_name,
					user_legacy_id, sale_invoice_code, source_table, source_table_row, payload
				)
				SELECT $1::uuid, $2::uuid, s.legacy_id, i.id, s.item_legacy_id, s.godown_legacy_id,
				       s.pack_units, s.quantity, s.bonus_quantity, s.sale_price, s.discount_percent,
				       s.item_flat_discount, s.unit_sales_tax, s.gst_percent, s.occurred_at,
				       s.machine_name, s.user_legacy_id, s.sale_invoice_code, 'DeletedSaleItem',
				       s.source_table_row, s.payload
				FROM phase_q_deleted_sale_items_batch s
				LEFT JOIN master_items i
				  ON i.tenant_id = $1::uuid AND i.legacy_id = s.item_legacy_id
				ON CONFLICT (tenant_id, branch_id, legacy_id) DO UPDATE SET
					item_id = EXCLUDED.item_id, item_legacy_id = EXCLUDED.item_legacy_id,
					godown_legacy_id = EXCLUDED.godown_legacy_id, pack_units = EXCLUDED.pack_units,
					quantity = EXCLUDED.quantity, bonus_quantity = EXCLUDED.bonus_quantity,
					sale_price = EXCLUDED.sale_price, discount_percent = EXCLUDED.discount_percent,
					item_flat_discount = EXCLUDED.item_flat_discount,
					unit_sales_tax = EXCLUDED.unit_sales_tax, gst_percent = EXCLUDED.gst_percent,
					occurred_at = EXCLUDED.occurred_at, machine_name = EXCLUDED.machine_name,
					user_legacy_id = EXCLUDED.user_legacy_id,
					sale_invoice_code = EXCLUDED.sale_invoice_code,
					source_table_row = EXCLUDED.source_table_row, payload = EXCLUDED.payload,
					updated_at = now()`, tenantID, branchID)
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
		values := make([]any, len(columns))
		pointers := make([]any, len(values))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return total, fmt.Errorf("scan DeletedSaleItem row: %w", err)
		}
		itemLegacyID := sourceColumnText(values, columnIndex, "icode")
		godownLegacyID := sourceColumnText(values, columnIndex, "gcode")
		if itemLegacyID == "" || godownLegacyID == "" {
			return total, errors.New("DeletedSaleItem row has an empty ICode or GCode")
		}
		occurred, err := sourceTime(sourceColumn(values, columnIndex, "date"))
		if err != nil {
			return total, fmt.Errorf("DeletedSaleItem %s date: %w", itemLegacyID, err)
		}
		payload := make(map[string]any, len(columns))
		for index, column := range columns {
			payload[column] = normalizeValue(values[index])
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return total, fmt.Errorf("marshal DeletedSaleItem %s: %w", itemLegacyID, err)
		}
		sourceRow := sourceColumnText(values, columnIndex, "sourcerow")
		if sourceRow == "" {
			return total, fmt.Errorf("DeletedSaleItem row for %s has an empty source row", itemLegacyID)
		}
		batch = append(batch, deletedSaleItemRow{
			legacyID:         "DeletedSaleItem:" + sourceRow,
			itemLegacyID:     itemLegacyID,
			godownLegacyID:   godownLegacyID,
			packUnits:        integer(sourceColumn(values, columnIndex, "packunits")),
			quantity:         numeric(sourceColumn(values, columnIndex, "qty")),
			bonusQuantity:    numeric(sourceColumn(values, columnIndex, "bonusqty")),
			salePrice:        numeric(sourceColumn(values, columnIndex, "saleprice")),
			discountPercent:  numeric(sourceColumn(values, columnIndex, "discperc")),
			itemFlatDiscount: numeric(sourceColumn(values, columnIndex, "itemflatdisc")),
			unitSalesTax:     numeric(sourceColumn(values, columnIndex, "unitsalestax")),
			gstPercent:       numeric(sourceColumn(values, columnIndex, "gstperc")),
			occurred:         occurred,
			machineName:      sourceColumnText(values, columnIndex, "machinename"),
			userLegacyID:     sourceColumnText(values, columnIndex, "usercode"),
			saleInvoiceCode:  sourceColumnText(values, columnIndex, "saleinvcode"),
			sourceTableRow:   sourceRow,
			payload:          string(payloadBytes),
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("DeletedSaleItem batch at %d: %w", total, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, fmt.Errorf("read DeletedSaleItem rows: %w", err)
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func deletedSaleItemValues(rows []deletedSaleItemRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		values[index] = []any{
			row.legacyID, row.itemLegacyID, row.godownLegacyID, row.packUnits,
			row.quantity, row.bonusQuantity, row.salePrice, row.discountPercent,
			row.itemFlatDiscount, row.unitSalesTax, row.gstPercent, row.occurred,
			row.machineName, row.userLegacyID, row.saleInvoiceCode, row.sourceTableRow, row.payload,
		}
	}
	return values
}

type adjustmentRow struct {
	legacyID, adjustmentLegacyID, itemLegacyID, godownLegacyID, batch string
	expiry                                                            *time.Time
	occurred                                                          time.Time
	looseQuantity, currentStock, price, averagePrice, newAveragePrice string
	salePrice, purchasePrice, recentPurchasePrice, userLegacyID       string
	packUnits, priority                                               int64
	posted                                                            bool
	remarks, sourceTableRow, payload                                  string
}

func importAdjustments(ctx context.Context, source *sql.DB, target *pgx.Conn, batchSize int, tenantID, branchID string) (int64, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT h.[AdjCode], h.[Date], h.[UserCode], h.[Remarks], h.[Posted],
		       d.[GCode], d.[ICode], d.[Batch], d.[Expiry], d.[LooseQty],
		       d.[Priority], d.[Price], d.[AvgPrice], d.[PackUnits], d.[SalePrice],
		       d.[PurPrice], d.[RecentPurPrice], d.[AdjUtn], d.[NewAvgPrice],
		       d.[CurrStock], d.[AlternateCustomICode],
		       h.[AdjCatCode], h.[ModifiedDate], h.[ModifiedBy], h.[PostedDate],
		       h.[PostedBy], h.[HeaderNo], h.[HeaderInvNo], h.[CRS_Transfered],
		       h.[CRS_TransferedOn], h.[AccCode], h.[Synced], h.[SyncedOn],
		       h.[SyncedBy], d.[UpdateAvgPrice]
		FROM [dbo].[AdjDetail] d
		LEFT JOIN [dbo].[AdjHeader] h ON h.[AdjCode] = d.[AdjCode]
		ORDER BY d.[AdjCode], d.[AdjUtn], d.[ICode], d.[Batch]`)
	if err != nil {
		return 0, fmt.Errorf("read AdjHeader/AdjDetail: %w", err)
	}
	defer rows.Close()
	if _, err := target.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS phase_q_adjustment_batch (
		legacy_id text NOT NULL, adjustment_legacy_id text NOT NULL, item_legacy_id text NOT NULL,
		godown_legacy_id text NOT NULL, batch text NOT NULL, expiry_date date,
		occurred_at timestamptz NOT NULL, loose_quantity numeric(19,8) NOT NULL,
		current_stock numeric(19,8), priority integer NOT NULL, price numeric(19,4) NOT NULL,
		average_price numeric(19,4) NOT NULL, new_average_price numeric(19,4),
		sale_price numeric(19,4) NOT NULL, purchase_price numeric(19,4) NOT NULL,
		recent_purchase_price numeric(19,4) NOT NULL, pack_units integer NOT NULL,
		user_legacy_id text NOT NULL, posted boolean NOT NULL, remarks text NOT NULL,
		source_table_row text NOT NULL, payload jsonb NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		return 0, fmt.Errorf("create adjustment staging table: %w", err)
	}
	batch := make([]adjustmentRow, 0, batchSize)
	var total int64
	var rowNumber int64
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := target.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, "TRUNCATE phase_q_adjustment_batch")
		}
		if err == nil {
			_, err = tx.CopyFrom(ctx, pgx.Identifier{"phase_q_adjustment_batch"},
				[]string{"legacy_id", "adjustment_legacy_id", "item_legacy_id", "godown_legacy_id", "batch", "expiry_date", "occurred_at", "loose_quantity", "current_stock", "priority", "price", "average_price", "new_average_price", "sale_price", "purchase_price", "recent_purchase_price", "pack_units", "user_legacy_id", "posted", "remarks", "source_table_row", "payload"},
				pgx.CopyFromRows(adjustmentValues(batch)))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO historical_stock_adjustment_lines (
					tenant_id, branch_id, legacy_id, adjustment_legacy_id, item_id,
					item_legacy_id, godown_id, godown_legacy_id, batch, expiry_date,
					occurred_at, loose_quantity, current_stock, priority, price,
					average_price, new_average_price, sale_price, purchase_price,
					recent_purchase_price, pack_units, user_legacy_id, posted, remarks,
					source_table, source_table_row, payload
				)
				SELECT $1::uuid, $2::uuid, s.legacy_id, s.adjustment_legacy_id,
				       i.id, s.item_legacy_id, g.id, s.godown_legacy_id, s.batch,
				       s.expiry_date, s.occurred_at, s.loose_quantity, s.current_stock,
				       s.priority, s.price, s.average_price, s.new_average_price,
				       s.sale_price, s.purchase_price, s.recent_purchase_price,
				       s.pack_units, s.user_legacy_id, s.posted, s.remarks,
				       'AdjDetail', s.source_table_row, s.payload
				FROM phase_q_adjustment_batch s
				LEFT JOIN master_items i
				  ON i.tenant_id = $1::uuid AND i.legacy_id = s.item_legacy_id
				LEFT JOIN master_godowns g
				  ON g.tenant_id = $1::uuid AND g.legacy_id = s.godown_legacy_id
				ON CONFLICT (tenant_id, branch_id, legacy_id) DO UPDATE SET
					adjustment_legacy_id = EXCLUDED.adjustment_legacy_id,
					item_id = EXCLUDED.item_id, item_legacy_id = EXCLUDED.item_legacy_id,
					godown_id = EXCLUDED.godown_id, godown_legacy_id = EXCLUDED.godown_legacy_id,
					batch = EXCLUDED.batch, expiry_date = EXCLUDED.expiry_date,
					occurred_at = EXCLUDED.occurred_at, loose_quantity = EXCLUDED.loose_quantity,
					current_stock = EXCLUDED.current_stock, priority = EXCLUDED.priority,
					price = EXCLUDED.price, average_price = EXCLUDED.average_price,
					new_average_price = EXCLUDED.new_average_price, sale_price = EXCLUDED.sale_price,
					purchase_price = EXCLUDED.purchase_price,
					recent_purchase_price = EXCLUDED.recent_purchase_price,
					pack_units = EXCLUDED.pack_units, user_legacy_id = EXCLUDED.user_legacy_id,
					posted = EXCLUDED.posted, remarks = EXCLUDED.remarks,
					source_table_row = EXCLUDED.source_table_row, payload = EXCLUDED.payload,
					updated_at = now()`, tenantID, branchID)
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
		rowNumber++
		values := make([]any, 35)
		pointers := make([]any, len(values))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return total, fmt.Errorf("scan adjustment row %d: %w", rowNumber, err)
		}
		occurred, err := sourceTime(values[1])
		if err != nil {
			return total, fmt.Errorf("adjustment row %d date: %w", rowNumber, err)
		}
		expiry, err := sourceOptionalTime(values[8])
		if err != nil {
			return total, fmt.Errorf("adjustment row %d expiry: %w", rowNumber, err)
		}
		adjustmentLegacyID := sourceText(values[0])
		itemLegacyID := sourceText(values[6])
		godownLegacyID := sourceText(values[5])
		if adjustmentLegacyID == "" || itemLegacyID == "" || godownLegacyID == "" {
			return total, fmt.Errorf("adjustment row %d is missing AdjCode, ICode, or GCode", rowNumber)
		}
		sourceTableRow := fmt.Sprintf("%s:%s:%s:%s", adjustmentLegacyID, sourceText(values[17]), itemLegacyID, sourceText(values[7]))
		legacyID := fmt.Sprintf("%s:%d", sourceTableRow, rowNumber)
		payload := map[string]any{
			"AdjCode": adjustmentLegacyID, "Date": normalizeValue(values[1]),
			"UserCode": normalizeValue(values[2]), "Remarks": normalizeValue(values[3]),
			"Posted": normalizeValue(values[4]), "GCode": godownLegacyID,
			"ICode": itemLegacyID, "Batch": normalizeValue(values[7]),
			"Expiry": normalizeValue(values[8]), "LooseQty": normalizeValue(values[9]),
			"Priority": normalizeValue(values[10]), "Price": normalizeValue(values[11]),
			"AvgPrice": normalizeValue(values[12]), "PackUnits": normalizeValue(values[13]),
			"SalePrice": normalizeValue(values[14]), "PurPrice": normalizeValue(values[15]),
			"RecentPurPrice": normalizeValue(values[16]), "AdjUtn": normalizeValue(values[17]),
			"NewAvgPrice": normalizeValue(values[18]), "CurrStock": normalizeValue(values[19]),
			"AlternateCustomICode": normalizeValue(values[20]),
			"AdjCatCode":           normalizeValue(values[21]), "ModifiedDate": normalizeValue(values[22]),
			"ModifiedBy": normalizeValue(values[23]), "PostedDate": normalizeValue(values[24]),
			"PostedBy": normalizeValue(values[25]), "HeaderNo": normalizeValue(values[26]),
			"HeaderInvNo": normalizeValue(values[27]), "CRS_Transfered": normalizeValue(values[28]),
			"CRS_TransferedOn": normalizeValue(values[29]), "AccCode": normalizeValue(values[30]),
			"Synced": normalizeValue(values[31]), "SyncedOn": normalizeValue(values[32]),
			"SyncedBy": normalizeValue(values[33]), "UpdateAvgPrice": normalizeValue(values[34]),
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return total, fmt.Errorf("marshal adjustment row %d: %w", rowNumber, err)
		}
		batch = append(batch, adjustmentRow{
			legacyID: legacyID, adjustmentLegacyID: adjustmentLegacyID,
			itemLegacyID: itemLegacyID, godownLegacyID: godownLegacyID,
			batch: sourceText(values[7]), expiry: expiry, occurred: occurred,
			looseQuantity: sourceText(values[9]), currentStock: sourceText(values[19]),
			priority: integer(values[10]), price: sourceText(values[11]),
			averagePrice: sourceText(values[12]), newAveragePrice: sourceText(values[18]),
			salePrice: sourceText(values[14]), purchasePrice: sourceText(values[15]),
			recentPurchasePrice: sourceText(values[16]), packUnits: integer(values[13]),
			userLegacyID: sourceText(values[2]), posted: sourceBool(values[4]),
			remarks: sourceText(values[3]), sourceTableRow: sourceTableRow,
			payload: string(payloadBytes),
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("adjustment batch at %d: %w", total, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, fmt.Errorf("read adjustment rows: %w", err)
	}
	if err := flush(); err != nil {
		return total, err
	}
	return total, nil
}

func adjustmentValues(rows []adjustmentRow) [][]any {
	values := make([][]any, len(rows))
	for index, row := range rows {
		var expiry any
		if row.expiry != nil {
			expiry = row.expiry
		}
		values[index] = []any{
			row.legacyID, row.adjustmentLegacyID, row.itemLegacyID, row.godownLegacyID,
			row.batch, expiry, row.occurred, row.looseQuantity, nullableText(row.currentStock),
			row.priority, row.price, row.averagePrice, nullableText(row.newAveragePrice),
			row.salePrice, row.purchasePrice, row.recentPurchasePrice, row.packUnits,
			row.userLegacyID, row.posted, row.remarks, row.sourceTableRow, row.payload,
		}
	}
	return values
}

func sourceColumn(values []any, columns map[string]int, name string) any {
	index, ok := columns[strings.ToLower(name)]
	if !ok || index < 0 || index >= len(values) {
		return nil
	}
	return values[index]
}

func sourceColumnText(values []any, columns map[string]int, name string) string {
	return sourceText(sourceColumn(values, columns, name))
}

func sourceText(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(normalizeValue(value)))
}

func sourceOptionalTime(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	result, err := sourceTime(value)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func sourceBool(value any) bool {
	switch strings.ToLower(sourceText(value)) {
	case "1", "true", "t", "yes", "y", "posted":
		return true
	default:
		return false
	}
}

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeValue(value any) any {
	switch value := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(value)
	case time.Time:
		return value.UTC().Format(time.RFC3339Nano)
	default:
		return value
	}
}

func numericTextEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == right {
		return true
	}
	leftRat, leftOK := new(big.Rat).SetString(left)
	rightRat, rightOK := new(big.Rat).SetString(right)
	return leftOK && rightOK && leftRat.Cmp(rightRat) == 0
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

func numeric(value any) string {
	raw := text(value)
	if strings.TrimSpace(raw) == "" {
		return "0"
	}
	return raw
}

func validWave(wave string) bool {
	switch wave {
	case "stock", "gl", "history", "adjustments", "deleted-sale-items", "withholding", "payments", "party-adjustments", "return-allocations", "both", "all":
		return true
	default:
		return false
	}
}

func validateSource(raw string, allowCanonical bool) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	database := parsed.Query().Get("database")
	if strings.EqualFold(database, canonicalDatabase) {
		if allowCanonical {
			return nil
		}
		return errors.New("refusing canonical FazalDinPP19DataBaseV2; pass -allow-canonical with an explicit tenant and branch scope")
	}
	if !strings.EqualFold(database, "AbuzarLegacyReference") {
		return fmt.Errorf("source database must be AbuzarLegacyReference, got %q", database)
	}
	return nil
}

func validateUUIDScope(tenantID, branchID string) error {
	if !uuidPattern.MatchString(tenantID) {
		return fmt.Errorf("tenant scope %q is not a UUID", tenantID)
	}
	if !uuidPattern.MatchString(branchID) {
		return fmt.Errorf("branch scope %q is not a UUID", branchID)
	}
	return nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}

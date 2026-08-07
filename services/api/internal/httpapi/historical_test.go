package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoricalItemAndAdjustmentMigrationRetainsSourceRows(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "027_historical_item_history_adjustments.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read historical report migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS historical_item_changes",
		"CREATE TABLE IF NOT EXISTS historical_stock_adjustment_lines",
		"UNIQUE (tenant_id, branch_id, report_kind, source_legacy_id)",
		"source_table_row text NOT NULL",
		"payload jsonb NOT NULL",
		"REFERENCES master_items(tenant_id, id)",
		"REFERENCES master_godowns(tenant_id, id)",
		"ENABLE ROW LEVEL SECURITY",
		"CREATE POLICY",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("historical report migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DROP TABLE") || strings.Contains(strings.ToUpper(migration), "DELETE FROM") {
		t.Fatal("historical report migration contains a destructive table/data operation")
	}
}

func TestHistoricalDeletedSaleItemMigrationRetainsSourceRows(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "030_historical_deleted_sale_items.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read deleted-sale-item migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS historical_deleted_sale_items",
		"UNIQUE (tenant_id, branch_id, legacy_id)",
		"sale_invoice_code text NOT NULL",
		"source_table_row text NOT NULL",
		"payload jsonb NOT NULL",
		"REFERENCES master_items(tenant_id, id)",
		"ENABLE ROW LEVEL SECURITY",
		"CREATE POLICY historical_deleted_sale_items_branch_tenant_hardening",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("deleted-sale-item migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DROP TABLE") || strings.Contains(strings.ToUpper(migration), "DELETE FROM") {
		t.Fatal("deleted-sale-item migration contains a destructive table/data operation")
	}
}

func TestHistoricalWithholdingMigrationRetainsPaymentTaxFields(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "031_historical_withholding_tax.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read withholding migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS historical_withholding_tax_entries",
		"payment_code text NOT NULL",
		"purchase_invoice_code text NOT NULL",
		"taxable_base numeric(19, 4)",
		"rate numeric(19, 4)",
		"amount numeric(19, 4)",
		"source_table_row text NOT NULL",
		"payload jsonb NOT NULL",
		"ENABLE ROW LEVEL SECURITY",
		"CREATE POLICY historical_withholding_tax_entries_branch_tenant_hardening",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("withholding migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DROP TABLE") || strings.Contains(strings.ToUpper(migration), "DELETE FROM") {
		t.Fatal("withholding migration contains a destructive table/data operation")
	}
}

func TestHistoricalPartyPaymentMigrationRetainsCustomerAndSupplierSources(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "032_historical_party_payment_allocations.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read party payment migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS historical_party_payment_allocations",
		"counterparty_kind text NOT NULL CHECK (counterparty_kind IN ('customer', 'supplier'))",
		"party_legacy_id text NOT NULL",
		"source_document_legacy_id text NOT NULL",
		"payment_amount numeric(19, 4)",
		"source_table_row text NOT NULL",
		"source_legacy_id text NOT NULL",
		"payload jsonb NOT NULL",
		"CREATE POLICY historical_party_payment_allocations_branch_tenant_hardening",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("party payment migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DROP TABLE") || strings.Contains(strings.ToUpper(migration), "DELETE FROM") {
		t.Fatal("party payment migration contains a destructive table/data operation")
	}
}

func TestHistoricalPartyAdjustmentMigrationRetainsReceivableAdjustments(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "033_historical_party_ledger_adjustments.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read party adjustment migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS historical_party_ledger_adjustments",
		"counterparty_kind text NOT NULL CHECK (counterparty_kind IN ('customer', 'supplier'))",
		"debit_amount numeric(19, 4)",
		"credit_amount numeric(19, 4)",
		"occurred_at timestamptz",
		"source_table_row text NOT NULL",
		"source_legacy_id text NOT NULL",
		"payload jsonb NOT NULL",
		"CREATE POLICY historical_party_ledger_adjustments_branch_tenant_hardening",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("party adjustment migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DROP TABLE") || strings.Contains(strings.ToUpper(migration), "DELETE FROM") {
		t.Fatal("party adjustment migration contains a destructive table/data operation")
	}
}

func TestHistoricalPartyReturnAllocationMigrationRetainsSourceRows(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "034_historical_party_return_allocations.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read party return allocation migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS historical_party_return_allocations",
		"counterparty_kind text NOT NULL CHECK (counterparty_kind IN ('customer', 'supplier'))",
		"return_kind text NOT NULL CHECK (return_kind IN ('sale', 'purchase'))",
		"return_source_legacy_id text NOT NULL",
		"source_document_legacy_id text NOT NULL",
		"allocation_amount numeric(19, 4)",
		"outstanding_amount numeric(19, 4)",
		"source_table_row text NOT NULL",
		"source_legacy_id text NOT NULL",
		"payload jsonb NOT NULL",
		"CREATE POLICY historical_party_return_allocations_branch_tenant_hardening",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("party return allocation migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DROP TABLE") || strings.Contains(strings.ToUpper(migration), "DELETE FROM") {
		t.Fatal("party return allocation migration contains a destructive table/data operation")
	}
}

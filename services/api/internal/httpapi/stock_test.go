package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStockQuantityUsesExactDecimalScale(t *testing.T) {
	value, err := parseStockQuantity("1.2500")
	if err != nil {
		t.Fatalf("parse quantity: %v", err)
	}
	if got := formatStockQuantity(value); got != "1.25" {
		t.Fatalf("formatted quantity = %q, want 1.25", got)
	}
	if _, err := parseStockQuantity("0.00001"); err == nil {
		t.Fatal("quantity with more than four decimal places was accepted")
	}
}

func TestStockAllocationPolicyDefaultsToMovingAverage(t *testing.T) {
	// 2026-08-08 reconciliation against dbo.StockReport/Purdetail/Saledetail
	// (item ICode 3018) found legacy costs inventory with a moving
	// weighted-average, not FIFO batch consumption (see the doc comment on
	// stockAllocationPolicy). moving-average is therefore the default when
	// ABUZAR_STOCK_ALLOCATION_POLICY is unset.
	t.Setenv("ABUZAR_STOCK_ALLOCATION_POLICY", "")
	if policy, err := stockAllocationPolicy(); err != nil || policy != "moving-average" {
		t.Fatalf("default policy = %q, %v, want moving-average", policy, err)
	}
	t.Setenv("ABUZAR_STOCK_ALLOCATION_POLICY", "fifo")
	if policy, err := stockAllocationPolicy(); err != nil || policy != "fifo" {
		t.Fatalf("fifo policy = %q, %v", policy, err)
	}
	t.Setenv("ABUZAR_STOCK_ALLOCATION_POLICY", "moving-average")
	if policy, err := stockAllocationPolicy(); err != nil || policy != "moving-average" {
		t.Fatalf("moving-average policy = %q, %v", policy, err)
	}
	t.Setenv("ABUZAR_STOCK_ALLOCATION_POLICY", "MOVING-AVERAGE")
	if policy, err := stockAllocationPolicy(); err != nil || policy != "moving-average" {
		t.Fatalf("moving-average policy is not case-insensitive: %q, %v", policy, err)
	}
	t.Setenv("ABUZAR_STOCK_ALLOCATION_POLICY", "legacy")
	if _, err := stockAllocationPolicy(); err == nil || !strings.Contains(err.Error(), "unsupported stock allocation policy") {
		t.Fatalf("legacy is no longer a recognized policy name: %v", err)
	}
	t.Setenv("ABUZAR_STOCK_ALLOCATION_POLICY", "unknown")
	if _, err := stockAllocationPolicy(); err == nil {
		t.Fatal("unknown policy silently fell back")
	}
}

func TestInventoryAdjustmentSignIsExplicitAndBounded(t *testing.T) {
	negative := -1
	positive := 1
	invalid := 0
	if got, err := inventoryAdjustmentSign("adjustment", &negative); err != nil || got != -1 {
		t.Fatalf("negative adjustment sign = %d, %v", got, err)
	}
	if _, err := inventoryAdjustmentSign("adjustment", nil); err == nil {
		t.Fatal("missing adjustment sign was accepted")
	}
	if _, err := inventoryAdjustmentSign("adjustment", &invalid); err == nil {
		t.Fatal("invalid adjustment sign was accepted")
	}
	if got, err := inventoryAdjustmentSign("in", &positive); err != nil || got != 1 {
		t.Fatalf("positive in sign = %d, %v", got, err)
	}
	if _, err := inventoryAdjustmentSign("out", &negative); err == nil {
		t.Fatal("negative out sign was accepted")
	}
}

func TestStockMigrationDefinesImmutableBatchLedgerCacheAndRLS(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "db", "migrations", "012_stock_ledger.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stock migration: %v", err)
	}
	migration := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS stock_batches",
		"CREATE TABLE IF NOT EXISTS stock_ledger",
		"CREATE TABLE IF NOT EXISTS stock_allocations",
		"CREATE TABLE IF NOT EXISTS stock_balances",
		"CREATE TABLE IF NOT EXISTS stock_balance_rebuilds",
		"UNIQUE (tenant_id, source_event_id, source_line_key, batch_id, direction)",
		"CREATE TRIGGER stock_ledger_immutable",
		"ALTER TABLE stock_ledger ALTER COLUMN adjustment_sign DROP DEFAULT",
		"ALTER TABLE %I FORCE ROW LEVEL SECURITY",
		"COALESCE(expiry_date, DATE '9999-12-31')",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("stock migration is missing contract %q", required)
		}
	}
}

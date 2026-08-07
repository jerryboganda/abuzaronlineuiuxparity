package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateSourceRequiresExplicitCanonicalOptIn(t *testing.T) {
	canonical := "sqlserver://localhost?database=FazalDinPP19DataBaseV2&trusted_connection=yes"
	if err := validateSource(canonical, false); err == nil {
		t.Fatal("canonical source was accepted without explicit opt-in")
	}
	if err := validateSource(canonical, true); err != nil {
		t.Fatalf("canonical source was rejected after opt-in: %v", err)
	}
	if err := validateSource("sqlserver://localhost?database=AbuzarLegacyReference&trusted_connection=yes", false); err != nil {
		t.Fatalf("reviewed sandbox source was rejected: %v", err)
	}
	if err := validateSource("sqlserver://localhost?database=OtherDatabase&trusted_connection=yes", true); err == nil {
		t.Fatal("unreviewed source database was accepted")
	}
}

func TestValidateUUIDScope(t *testing.T) {
	validTenant := "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01"
	validBranch := "6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02"
	if err := validateUUIDScope(validTenant, validBranch); err != nil {
		t.Fatalf("valid scope rejected: %v", err)
	}
	if err := validateUUIDScope("sandbox", validBranch); err == nil {
		t.Fatal("invalid tenant scope was accepted")
	}
	if err := validateUUIDScope(validTenant, ""); err == nil {
		t.Fatal("empty branch scope was accepted")
	}
}

func TestValidWaveIncludesSourceBackedPaymentsAndWithholding(t *testing.T) {
	for _, wave := range []string{"stock", "gl", "history", "adjustments", "deleted-sale-items", "withholding", "payments", "party-adjustments", "return-allocations", "both", "all"} {
		if !validWave(wave) {
			t.Fatalf("wave %q was rejected", wave)
		}
	}
	if validWave("advance-tax") {
		t.Fatal("advance-tax was accepted as a withholding import wave")
	}
}

func TestStockSnapshotIdentityPreservesReviewedCompositeKey(t *testing.T) {
	date := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	if got, want := stockSnapshotLegacyID(date, "G-01", "I-77"), "2026-08-06T00:00:00Z:G-01:I-77"; got != want {
		t.Fatalf("stock snapshot identity = %q, want %q", got, want)
	}
}

func TestStockSnapshotImporterRejectsIdentityCollapse(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read historical importer: %v", err)
	}
	code := string(data)
	for _, required := range []string{
		"COUNT(DISTINCT legacy_id)",
		"duplicate composite identities; refusing silent overwrite",
		"'StockReport', s.legacy_id, s.payload",
	} {
		if !strings.Contains(code, required) {
			t.Errorf("stock importer is missing identity guard fragment %q", required)
		}
	}
}

func TestHistoricalGLIdentityPreservesReviewedCompositeKey(t *testing.T) {
	if got, want := historicalGLLegacyID("SALE-1", "4", "1100"), "SALE-1:4:1100"; got != want {
		t.Fatalf("historical GL identity = %q, want %q", got, want)
	}
}

func TestHistoricalGLImporterRejectsIdentityCollapse(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read historical importer: %v", err)
	}
	code := string(data)
	for _, required := range []string{
		"GL batch at %d contains %d duplicate reviewed identities; refusing silent overwrite",
		"historicalGLLegacyID(doc, vrow, acct)",
	} {
		if !strings.Contains(code, required) {
			t.Errorf("GL importer is missing identity guard fragment %q", required)
		}
	}
}

func TestPartyReturnAllocationImporterUsesReviewedSourceStreams(t *testing.T) {
	path := filepath.Join("main.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read historical importer: %v", err)
	}
	code := string(data)
	for _, required := range []string{
		"SRAllocationHeader",
		"SRAllocationDetail",
		"PRAllocationHeader",
		"PRAllocationDetail",
		"historical_party_return_allocations",
		"return-allocation",
		"return_source_table",
		"source_document_table",
	} {
		if !strings.Contains(code, required) {
			t.Errorf("return-allocation importer is missing reviewed fragment %q", required)
		}
	}
}

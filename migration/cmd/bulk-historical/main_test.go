package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

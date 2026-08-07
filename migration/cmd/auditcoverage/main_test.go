package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCoverageCountsUniqueManifestTablesAndOverlaps(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	mapsPath := filepath.Join(root, "maps")
	if err := os.Mkdir(mapsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{"columns": []map[string]string{
		{"schema": "dbo", "table": "Item"},
		{"schema": "dbo", "table": "Item"},
		{"schema": "dbo", "table": "Supplier"},
		{"schema": "dbo", "table": "StockReport"},
	}}
	writeJSON(t, manifestPath, manifest)
	writeJSON(t, filepath.Join(mapsPath, "first.json"), map[string]any{"tables": []map[string]any{
		{"source": map[string]string{"schema": "dbo", "table": "Item"}},
		{"source": map[string]string{"schema": "dbo", "table": "Supplier"}},
	}})
	writeJSON(t, filepath.Join(mapsPath, "second.json"), map[string]any{"tables": []map[string]any{
		{"source": map[string]string{"schema": "DBO", "table": "ITEM"}},
	}})

	result, err := buildCoverage(manifestPath, mapsPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.ManifestTableCount != 3 || result.ReviewedMappingEntryCount != 3 || result.MappedTableCount != 2 || result.UnmappedTableCount != 1 || result.OverlappingMappingEntryCount != 1 {
		t.Fatalf("unexpected coverage: %+v", result)
	}
	if result.UnmappedTables[0].Table != "StockReport" {
		t.Fatalf("unmapped table = %+v, want StockReport", result.UnmappedTables[0])
	}
}

func TestBuildCoverageRejectsMalformedMapSource(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	mapsPath := filepath.Join(root, "maps")
	if err := os.Mkdir(mapsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, manifestPath, map[string]any{"columns": []map[string]string{{"schema": "dbo", "table": "Item"}}})
	writeJSON(t, filepath.Join(mapsPath, "bad.json"), map[string]any{"tables": []map[string]any{{"source": map[string]string{"schema": "", "table": "Item"}}}})
	if _, err := buildCoverage(manifestPath, mapsPath); err == nil {
		t.Fatal("malformed source table was accepted")
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

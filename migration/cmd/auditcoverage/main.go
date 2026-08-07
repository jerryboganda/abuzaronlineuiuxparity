// Command auditcoverage compares the authoritative SQL Server schema manifest
// with every reviewed JSON mapping in migration/maps. It is read-only: it
// never opens either database and it never changes a mapping or source file.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type tableRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

type manifestColumn struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

type schemaManifest struct {
	Columns []manifestColumn `json:"columns"`
}

type mappingConfig struct {
	Tables []struct {
		Source tableRef `json:"source"`
	} `json:"tables"`
}

type mappingFileResult struct {
	File               string     `json:"file"`
	MappingEntries     int        `json:"mappingEntries"`
	UniqueSourceTables int        `json:"uniqueSourceTables"`
	SourceTables       []tableRef `json:"sourceTables,omitempty"`
}

type coverageReport struct {
	GeneratedAt                  string              `json:"generatedAt"`
	Manifest                     string              `json:"manifest"`
	MapsDirectory                string              `json:"mapsDirectory"`
	ManifestTableCount           int                 `json:"manifestTableCount"`
	ReviewedMappingEntryCount    int                 `json:"reviewedMappingEntryCount"`
	MappedTableCount             int                 `json:"mappedTableCount"`
	UnmappedTableCount           int                 `json:"unmappedTableCount"`
	OverlappingMappingEntryCount int                 `json:"overlappingMappingEntryCount"`
	MappedTables                 []tableRef          `json:"mappedTables"`
	UnmappedTables               []tableRef          `json:"unmappedTables"`
	Files                        []mappingFileResult `json:"files"`
}

func main() {
	manifestPath := flag.String("manifest", filepath.Join("tmp", "canonical-sqlserver-schema.json"), "authoritative SQL Server schema manifest")
	mapsDirectory := flag.String("maps", filepath.Join("migration", "maps"), "directory containing reviewed JSON maps")
	out := flag.String("out", filepath.Join("parity", "catalog", "phase-e-map-coverage.json"), "coverage report output path")
	failOnUnmapped := flag.Bool("fail-on-unmapped", false, "write the report and exit non-zero when any manifest table is unmapped")
	flag.Parse()

	report, err := buildCoverage(*manifestPath, *mapsDirectory)
	if err != nil {
		fatal(err)
	}
	if err := writeReport(*out, report); err != nil {
		fatal(err)
	}
	fmt.Printf("Wrote Phase E coverage for %d manifest tables: %d mapped, %d unmapped, %d overlapping mapping entries to %s\n", report.ManifestTableCount, report.MappedTableCount, report.UnmappedTableCount, report.OverlappingMappingEntryCount, *out)
	if *failOnUnmapped && report.UnmappedTableCount > 0 {
		fatal(errors.New("reviewed migration maps do not cover every manifest table"))
	}
}

func buildCoverage(manifestPath, mapsDirectory string) (coverageReport, error) {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return coverageReport{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest schemaManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return coverageReport{}, fmt.Errorf("decode manifest: %w", err)
	}

	manifestTables := make(map[string]tableRef)
	for _, column := range manifest.Columns {
		ref := tableRef{Schema: strings.TrimSpace(column.Schema), Table: strings.TrimSpace(column.Table)}
		if ref.Schema == "" || ref.Table == "" {
			return coverageReport{}, errors.New("manifest contains a table with an empty schema or table name")
		}
		manifestTables[tableKey(ref)] = ref
	}
	if len(manifestTables) == 0 {
		return coverageReport{}, errors.New("manifest contains no base-table columns")
	}

	entries := make(map[string][]string)
	fileResults := make([]mappingFileResult, 0)
	files, err := os.ReadDir(mapsDirectory)
	if err != nil {
		return coverageReport{}, fmt.Errorf("read maps directory: %w", err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".json") {
			continue
		}
		path := filepath.Join(mapsDirectory, file.Name())
		bytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return coverageReport{}, fmt.Errorf("read map %s: %w", file.Name(), readErr)
		}
		var config mappingConfig
		if err := json.Unmarshal(bytes, &config); err != nil {
			return coverageReport{}, fmt.Errorf("decode map %s: %w", file.Name(), err)
		}
		if len(config.Tables) == 0 {
			continue
		}
		fileTables := make(map[string]tableRef)
		for _, mapping := range config.Tables {
			ref := tableRef{Schema: strings.TrimSpace(mapping.Source.Schema), Table: strings.TrimSpace(mapping.Source.Table)}
			if ref.Schema == "" || ref.Table == "" {
				return coverageReport{}, fmt.Errorf("map %s contains a source table with an empty schema or table name", file.Name())
			}
			key := tableKey(ref)
			entries[key] = append(entries[key], file.Name())
			fileTables[key] = ref
		}
		fileRefs := refsFromMap(fileTables)
		fileResults = append(fileResults, mappingFileResult{
			File:               file.Name(),
			MappingEntries:     len(config.Tables),
			UniqueSourceTables: len(fileRefs),
			SourceTables:       fileRefs,
		})
	}

	mapped := make(map[string]tableRef)
	for key := range entries {
		if ref, ok := manifestTables[key]; ok {
			mapped[key] = ref
		}
	}
	unmapped := make(map[string]tableRef)
	for key, ref := range manifestTables {
		if _, ok := mapped[key]; !ok {
			unmapped[key] = ref
		}
	}
	allEntries := 0
	for _, files := range entries {
		allEntries += len(files)
	}
	report := coverageReport{
		GeneratedAt:                  time.Now().UTC().Format(time.RFC3339Nano),
		Manifest:                     filepath.Clean(manifestPath),
		MapsDirectory:                filepath.Clean(mapsDirectory),
		ManifestTableCount:           len(manifestTables),
		ReviewedMappingEntryCount:    allEntries,
		MappedTableCount:             len(mapped),
		UnmappedTableCount:           len(unmapped),
		OverlappingMappingEntryCount: allEntries - len(entries),
		MappedTables:                 refsFromMap(mapped),
		UnmappedTables:               refsFromMap(unmapped),
		Files:                        fileResults,
	}
	sort.Slice(report.Files, func(i, j int) bool { return report.Files[i].File < report.Files[j].File })
	return report, nil
}

func refsFromMap(values map[string]tableRef) []tableRef {
	refs := make([]tableRef, 0, len(values))
	for _, ref := range values {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		left := tableKey(refs[i])
		right := tableKey(refs[j])
		return left < right
	})
	return refs
}

func tableKey(ref tableRef) string {
	return strings.ToLower(strings.TrimSpace(ref.Schema) + "." + strings.TrimSpace(ref.Table))
}

func writeReport(path string, report coverageReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

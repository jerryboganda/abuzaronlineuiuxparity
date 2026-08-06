package httpapi

import (
	"strings"
	"testing"
)

func TestPhaseQRegistryCoversTheMappedRemainingLeaves(t *testing.T) {
	if got, want := len(phaseQReportRegistry), 28; got != want {
		t.Fatalf("Phase Q registry has %d leaves, want %d", got, want)
	}
	for _, kind := range []string{
		"accounts-reports-ledger-reports-accounts-ledger",
		"customer-sales-lp-ledger",
		"listing-group-rights-list",
		"reprinting-sale",
		"item-reports-deleted-sale-items-log",
	} {
		if kind == "customer-sales-lp-ledger" {
			spec, ok := reportSpecForKey(kind)
			if !ok || spec.financeMode != "party-customer" {
				t.Fatalf("%s was not promoted to the customer party-ledger projection: %+v", kind, spec)
			}
			continue
		}
		if _, ok := phaseQReportRegistry[kind]; !ok {
			t.Fatalf("Phase Q registry is missing %q", kind)
		}
	}
}

func TestPhaseQFinancialDefinitionsExposeTruthfulSourcesAndPrerequisites(t *testing.T) {
	tests := []struct {
		kind       string
		mode       string
		status     string
		noteParts  []string
		columnPart string
	}{
		{"gl-journal", "gl", "real", []string{"gl_journals", "gl_lines", "posted-only"}, "Debit"},
		{"trial-balance", "trial-balance", "real", []string{"opening", "historical"}, "Credit"},
		{"customer-statement", "party-customer", "real", []string{"party_ledger_entries"}, "Description"},
		{"payables-aging", "aging-payable", "real", []string{"due_date", "aging-bucket"}, "Aging Status"},
		{"tax-register", "tax-output", "real", []string{"tax snapshots", "not recomputed"}, "Tax Amount"},
		{"voucher-register", "voucher", "real", []string{"voucher_entries", "posted"}, "Status"},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			definition := reportDefinitionFor(test.kind)
			spec, ok := reportSpecForKey(test.kind)
			if !ok || spec.financeMode != test.mode {
				t.Fatalf("mode = %q, want %q", spec.financeMode, test.mode)
			}
			if definition.ProjectionStatus != test.status {
				t.Fatalf("projection status = %q, want %q", definition.ProjectionStatus, test.status)
			}
			for _, part := range test.noteParts {
				if !strings.Contains(strings.ToLower(definition.ProjectionNote), strings.ToLower(part)) {
					t.Errorf("projection note %q does not contain %q", definition.ProjectionNote, part)
				}
			}
			found := false
			for _, column := range definition.Columns {
				if column.Label == test.columnPart {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("columns do not contain %q: %+v", test.columnPart, definition.Columns)
			}
		})
	}
}

func TestPhaseQQueriesArePostedAndScopeBound(t *testing.T) {
	for _, mode := range []string{"gl", "trial-balance", "party-customer", "aging-payable", "tax-output", "voucher"} {
		query := financeReadModelQuery(mode, "LIMIT $6 OFFSET $7")
		for _, fragment := range []string{
			"$1::uuid",
			"$2::uuid",
			"$3::date",
			"$4::date",
			"$5",
			"LIMIT $6 OFFSET $7",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s query is missing %q", mode, fragment)
			}
		}
	}
	for _, query := range []string{
		financeReadModelQuery("gl", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("tax-output", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("voucher", "LIMIT $6 OFFSET $7"),
	} {
		if !strings.Contains(query, "posted") {
			t.Errorf("posted-only predicate missing from query:\n%s", query)
		}
	}
	if query := financeReadModelQuery("tax-withholding", "LIMIT $6 OFFSET $7"); !strings.Contains(query, "WHERE false") {
		t.Fatal("withholding query returned a fabricated source instead of an empty projection")
	}
	if query := compatibilityReportQuery("se.aggregate = 'quotation'", "LIMIT $6 OFFSET $7"); !strings.Contains(query, "COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'") {
		t.Fatal("compatibility report query is not explicitly posted-only")
	}
}

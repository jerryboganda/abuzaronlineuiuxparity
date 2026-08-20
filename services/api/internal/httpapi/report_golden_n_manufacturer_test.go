package httpapi

import (
	"strings"
	"testing"
)

// TestPhaseNGoldenManufacturerLeavesResolveToExpectedRegistryShape locks the
// registry wiring for the 6 Phase N "Manufacturer Wise" (Sales Reports)
// leaves verified in
// docs/evidence/PHASE_N_GOLDEN_VERIFICATION_MANUFACTURER_2026-08-09.md. All six sit in
// phaseNReportRegistry with salesReadModel=true and no salesMode case wired
// (mode stays "" — see the switch in reports.go around lines 284-317), so
// each dispatches to the plain 6-column generic salesReadModelQuery, not a
// manufacturer-grouped or manufacturer-columned projection.
func TestPhaseNGoldenManufacturerLeavesResolveToExpectedRegistryShape(t *testing.T) {
	tests := map[string]struct {
		title     string
		aggregate string
	}{
		"manufacturer-wise-sales":                                      {"Sales", reportSaleAggregate},
		"manufacturer-wise-sales-detail-and-summary":                   {"Sales Detail And Summary", reportSaleAggregate},
		"manufacturer-wise-net-sales":                                  {"Net Sales", reportSaleAggregate},
		"manufacturer-wise-item-sales-discount":                        {"Item Sales/Discount", reportSaleAggregate},
		"manufacturer-wise-manufacturer-wise-sales-and-return-summary": {"Manufacturer Wise Sales And Return Summary", reportSaleOrReturn},
		"manufacturer-wise-cnic-ntn-registered-customers-sales":        {"CNIC/NTN Registered Customers Sales", reportSaleAggregate},
	}
	for kind, want := range tests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if !spec.salesReadModel {
			t.Fatalf("%s: salesReadModel = false, want true", kind)
		}
		if spec.salesMode != "" {
			t.Fatalf("%s: salesMode = %q, want \"\" (no manufacturer-aware mode is wired today)", kind, spec.salesMode)
		}
		if spec.title != want.title {
			t.Fatalf("%s: title = %q, want %q", kind, spec.title, want.title)
		}
		if spec.aggregateCondition != want.aggregate {
			t.Fatalf("%s: aggregateCondition = %q, want %q", kind, spec.aggregateCondition, want.aggregate)
		}
	}
}

// TestPhaseNGoldenManufacturerLeavesReportEventLedgerProjectionStatus
// documents that all 6 leaves report ProjectionStatus == "event-ledger" and
// the generic 6-column reportEventLedgerColumns() shape (document,
// occurredAt, party, item, quantity, amount) — there is no manufacturer
// column and no manufacturer grouping in the API metadata or the underlying
// query, matching the same "no manufacturer join at all despite the name"
// finding already documented for the O-phase siblings
// (manufacturer-wise-detail, manufacturer-wise-monthly-stock-movement) in
// docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md.
func TestPhaseNGoldenManufacturerLeavesReportEventLedgerProjectionStatus(t *testing.T) {
	kinds := []string{
		"manufacturer-wise-sales",
		"manufacturer-wise-sales-detail-and-summary",
		"manufacturer-wise-net-sales",
		"manufacturer-wise-item-sales-discount",
		"manufacturer-wise-manufacturer-wise-sales-and-return-summary",
		"manufacturer-wise-cnic-ntn-registered-customers-sales",
	}
	wantColumns := reportEventLedgerColumns()
	for _, kind := range kinds {
		def := reportDefinitionFor(kind)
		if def.ProjectionStatus != "event-ledger" {
			t.Errorf("%s: ProjectionStatus = %q, want %q (update this test and the companion doc if a promotion pass changed this)", kind, def.ProjectionStatus, "event-ledger")
		}
		if len(def.Columns) != len(wantColumns) {
			t.Errorf("%s: got %d columns, want the generic %d-column event-ledger shape (no manufacturer column exists)", kind, len(def.Columns), len(wantColumns))
			continue
		}
		for i, col := range def.Columns {
			if col.Key != wantColumns[i].Key {
				t.Errorf("%s: column[%d].Key = %q, want %q", kind, i, col.Key, wantColumns[i].Key)
			}
		}
	}
}

// TestPhaseNGoldenManufacturerModeDispatchesToPlainGenericQuery documents at
// the code level that salesReadModelQueryMode with mode="" (the state of all
// 6 manufacturer-wise leaves today) falls through to the default branch and
// returns salesReadModelQuery's plain output, which selects only
// "document, occurred_at::text, party, item, quantity, amount" and never
// references master_manufacturers, master_items, or any ManfCode/Manufacturer
// payload key. This is the mechanical reason the leaf name promises a
// manufacturer breakdown that the query does not deliver.
func TestPhaseNGoldenManufacturerModeDispatchesToPlainGenericQuery(t *testing.T) {
	pagination := "LIMIT $3 OFFSET $4"
	for _, aggregate := range []string{reportSaleAggregate, reportSaleOrReturn} {
		got := salesReadModelQueryMode(aggregate, "", pagination)
		want := salesReadModelQuery(aggregate, pagination)
		if got != want {
			t.Fatalf("salesReadModelQueryMode(%q, \"\", ...) diverged from salesReadModelQuery(%q, ...); if a manufacturer-aware mode now exists, update this test and the companion doc.\ngot:\n%s\nwant:\n%s", aggregate, aggregate, got, want)
		}
		for _, unwanted := range []string{"master_manufacturers", "ManfCode", "Manufacturer"} {
			if strings.Contains(got, unwanted) {
				t.Errorf("salesReadModelQueryMode(%q, \"\", ...) unexpectedly references %q; a manufacturer join now exists — update the companion doc, this is good news", aggregate, unwanted)
			}
		}
	}
}

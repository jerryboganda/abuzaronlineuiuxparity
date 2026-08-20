package httpapi

import (
	"strings"
	"testing"
)

// TestPhaseNGoldenSaleReturnLeavesResolveToExpectedMode locks the registry
// wiring for the 5 Phase N "Daily Reports/Sales Return" + "Sales Reports"
// leaves verified in
// docs/evidence/PHASE_N_GOLDEN_VERIFICATION_SALE_RETURN_2026-08-09.md against
// independent psql cross-checks on business_documents/business_document_lines
// for the sandbox tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee, branch
// ffffffff-ffff-ffff-ffff-ffffffffffff.
func TestPhaseNGoldenSaleReturnLeavesResolveToExpectedMode(t *testing.T) {
	tests := map[string]struct {
		mode      string
		title     string
		aggregate string
	}{
		"selected-sales-and-summaries-report": {"", "Selected Sales and Summaries Report", reportSaleAggregate},
		"sales-return-detail":                 {"line-detail", "Sales Return detail", reportSaleReturnAggregate},
		"sales-return-summary":                {"invoice-summary", "Sales Return summary", reportSaleReturnAggregate},
		"sales-return-summary-inv-wise":       {"invoice-summary", "Sales Return Summary Inv.wise", reportSaleReturnAggregate},
		"sales-return-detail-inv-wise":        {"", "Sales Return Detail Inv.wise", reportSaleReturnAggregate},
	}
	for kind, want := range tests {
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s: not present in the report registry", kind)
		}
		if !spec.salesReadModel {
			t.Fatalf("%s: salesReadModel = false, want true", kind)
		}
		if spec.salesMode != want.mode {
			t.Fatalf("%s: salesMode = %q, want %q", kind, spec.salesMode, want.mode)
		}
		if spec.title != want.title {
			t.Fatalf("%s: title = %q, want %q", kind, spec.title, want.title)
		}
		if spec.aggregateCondition != want.aggregate {
			t.Fatalf("%s: aggregateCondition = %q, want %q", kind, spec.aggregateCondition, want.aggregate)
		}
	}
}

// TestPhaseNGoldenSaleReturnSummaryLeavesAreByteIdenticalSQL locks the
// documented finding that sales-return-summary and
// sales-return-summary-inv-wise both resolve to salesMode = "invoice-summary"
// with no other spec field differing, so saleReturnReadModelQueryMode
// produces byte-identical SQL for both leaves. This mirrors the wave-1
// "reprinting-*" precedent: two distinct legacy leaf names sharing one
// underlying query, not a bug. If a future promotion pass intentionally
// differentiates them, update this test alongside the companion doc.
func TestPhaseNGoldenSaleReturnSummaryLeavesAreByteIdenticalSQL(t *testing.T) {
	summarySpec, ok := reportSpecForKey("sales-return-summary")
	if !ok {
		t.Fatal("sales-return-summary: not present in the report registry")
	}
	invWiseSpec, ok := reportSpecForKey("sales-return-summary-inv-wise")
	if !ok {
		t.Fatal("sales-return-summary-inv-wise: not present in the report registry")
	}

	queryA := saleReturnReadModelQueryMode(summarySpec.salesMode, "LIMIT $6 OFFSET $7")
	queryB := saleReturnReadModelQueryMode(invWiseSpec.salesMode, "LIMIT $6 OFFSET $7")
	if queryA != queryB {
		t.Fatalf("expected sales-return-summary and sales-return-summary-inv-wise to produce byte-identical SQL, got different queries:\nA:\n%s\nB:\n%s", queryA, queryB)
	}
	if !strings.Contains(queryA, "GROUP BY document") {
		t.Fatalf("expected the shared invoice-summary query to GROUP BY document; query:\n%s", queryA)
	}
}

// TestPhaseNGoldenSaleReturnDetailInvWiseIsUngroupedRawView documents a
// naming/wiring gap: sales-return-detail-inv-wise's "Inv.wise" name implies
// per-invoice grouping (like sales-return-summary-inv-wise), but its
// salesMode is "" (unset), so it dispatches to the raw, ungrouped
// saleReturnReadModelQuery ("" mode) instead of the invoice-summary grouped
// query. Confirmed against the sandbox tenant: the raw view returns one row
// per business_document_lines row (44,579) across only 30,704 distinct
// invoices for the 'cash-sale-return' kind alone - i.e. it is NOT grouped by
// invoice despite the name. This is flagged for the promotion agent to
// decide (real invoice-summary wiring vs. intentional distinct raw-line
// "detail" semantics), not fixed here per the read-only reports.go
// constraint on this pass.
func TestPhaseNGoldenSaleReturnDetailInvWiseIsUngroupedRawView(t *testing.T) {
	spec, ok := reportSpecForKey("sales-return-detail-inv-wise")
	if !ok {
		t.Fatal("sales-return-detail-inv-wise: not present in the report registry")
	}
	if spec.salesMode != "" {
		t.Fatalf("sales-return-detail-inv-wise: salesMode = %q, want \"\" (update this test and the companion doc if this leaf was promoted to a real grouped mode)", spec.salesMode)
	}

	query := saleReturnReadModelQueryMode(spec.salesMode, "LIMIT $6 OFFSET $7")
	if strings.Contains(query, "GROUP BY document") {
		t.Fatalf("sales-return-detail-inv-wise now groups by document; update this test (and the companion doc's 'promotion needed' note) to reflect the fix; query:\n%s", query)
	}
	if query != saleReturnReadModelQuery("LIMIT $6 OFFSET $7") {
		t.Fatalf("expected sales-return-detail-inv-wise (empty salesMode) to dispatch to the raw saleReturnReadModelQuery, got a different query:\n%s", query)
	}
}

// TestPhaseNGoldenSaleReturnKindCoverage locks that both the line-detail
// (saleReturnDetailReadModelQuery) and default/summary
// (saleReturnReadModelQuery) sale-return queries scope their canonical
// business_documents branch to all 3 legacy kind spellings named in the
// wave-2 assignment (cash-sale-return, credit-sale-return, open-sale-return)
// plus the additional aliases (cash-return, credit-return, open-cash-return,
// open-credit-return) already present in the source. The sandbox tenant only
// has posted 'cash-sale-return' documents (30,704) - credit-sale-return and
// open-sale-return are 0 rows here, a data-availability gap in this sandbox,
// not a query gap; this test guards the query text itself so a future edit
// cannot silently narrow the kind list without a visible test failure.
func TestPhaseNGoldenSaleReturnKindCoverage(t *testing.T) {
	wantKinds := []string{
		"'cash-return'", "'credit-return'", "'open-cash-return'", "'open-credit-return'",
		"'cash-sale-return'", "'credit-sale-return'", "'open-sale-return'",
	}
	detailQuery := saleReturnDetailReadModelQuery("")
	summaryQuery := saleReturnReadModelQuery("")
	for _, k := range wantKinds {
		if !strings.Contains(detailQuery, k) {
			t.Errorf("saleReturnDetailReadModelQuery missing kind %s in its bd.kind IN (...) list", k)
		}
		if !strings.Contains(summaryQuery, k) {
			t.Errorf("saleReturnReadModelQuery missing kind %s in its bd.kind IN (...) list", k)
		}
	}
}

// TestPhaseNGoldenSaleReturnDetailSalesTaxCoalesceOrderBug documents a real,
// currently-unfixed bug found by independent psql cross-verification: for
// every one of the sandbox tenant's 44,579 posted cash-sale-return lines,
// business_document_lines.legacy_payload->>'SalesTax' is the literal string
// "0.00" (never empty), so
// COALESCE(NULLIF(legacy_payload->>'SalesTax', ''), ..., bl.tax_amount::text, '0.00')
// always stops at the first branch and the correctly-populated bl.tax_amount
// column (which is genuinely non-zero for 1,970 of those 44,579 lines,
// totaling Rs 311,703.05) is never reached. The "SalesTax Value" column on
// sales-return-detail therefore silently reports 0.00 for those 1,970 lines
// (4.42%) tenant-wide.
//
// This is the exact same defect class already found and fixed for the
// sibling sale leaf: dailySalesDetailReadModelQuery (used by sale-detail,
// see docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_CORE_2026-08-09.md leaf 1) now puts
// bl.tax_amount::numeric(19,2)::text FIRST in its COALESCE chain, with a
// comment explaining exactly this "legacy_payload SalesTax is always a stuck
// literal 0.00" data fact. That fix was never propagated to
// saleReturnDetailReadModelQuery, the sale-return counterpart used by
// sales-return-detail.
//
// This test was flipped 2026-08-09 (wave 2 follow-up) once the fix was
// applied to saleReturnDetailReadModelQuery: bl.tax_amount now comes first
// in the COALESCE chain, matching the already-fixed sibling
// dailySalesDetailReadModelQuery. It now asserts the corrected order so a
// regression back to the legacy_payload-first ordering fails loudly. The
// companion doc's verdict for sales-return-detail has been updated from
// MISMATCH-DOCUMENTED to MATCHED accordingly.
func TestPhaseNGoldenSaleReturnDetailSalesTaxCoalesceOrderBug(t *testing.T) {
	query := saleReturnDetailReadModelQuery("")

	fixedOrder := "COALESCE(bl.tax_amount::numeric(19,2)::text, NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), '0.00') AS sales_tax_value"
	if !strings.Contains(query, fixedOrder) {
		t.Fatalf("saleReturnDetailReadModelQuery's sales_tax_value COALESCE order changed - expected bl.tax_amount first (the 2026-08-09 fix), matching dailySalesDetailReadModelQuery. Query:\n%s", query)
	}

	// Sanity: the already-fixed sibling (sale-detail) query puts bl.tax_amount
	// first. This asserts that fixed query still looks the way the companion
	// doc describes, so a regression there doesn't go unnoticed by this file
	// even though sale-detail itself isn't in this pass's assignment.
	fixedSiblingOrder := "COALESCE(bl.tax_amount::numeric(19,2)::text, NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), '0.00') AS sales_tax_value"
	siblingQuery := dailySalesDetailReadModelQuery("")
	if !strings.Contains(siblingQuery, fixedSiblingOrder) {
		t.Fatalf("dailySalesDetailReadModelQuery (sale-detail) no longer matches the documented fixed COALESCE order - the wave-1 fix this test compares against may have regressed. Query:\n%s", siblingQuery)
	}
}

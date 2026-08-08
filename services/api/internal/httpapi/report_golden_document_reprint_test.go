package httpapi

import (
	"strings"
	"testing"
)

// TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart is a
// Go-level lock for the Phase Q golden-verification finding documented in
// docs/PHASE_Q_GOLDEN_VERIFICATION_DOCUMENT_REPRINT_2026-08-09.md: every
// reprinting-* leaf assigned to that pass resolves to the byte-identical SQL
// query text as its non-reprint counterpart leaf (sale-detail, sale-summary,
// purchase-detail). reprintMode only changes metadata/columns in
// reportDefinitionForKey; it is never consulted by the query-dispatch switch
// in the report handler. This test fails loudly if that ever changes without
// an accompanying update to the golden-verification doc.
func TestGoldenReprintingLeavesQueryTextIsIdenticalToNonReprintCounterpart(t *testing.T) {
	pagination := "LIMIT $6 OFFSET $7"
	tests := []struct {
		reprintKind    string
		counterpartKind string
	}{
		{"reprinting-sale", "sale-detail"},
		{"reprinting-sale-format-2", "sale-detail"},
		{"reprinting-sale-format-3", "sale-detail"},
		{"reprinting-sale-format-4", "sale-detail"},
		{"reprinting-sale-with-summary-reports", "sale-summary"},
		{"reprinting-sale-with-header-wise-summaries", "sale-summary"},
		{"reprinting-selected-sales-and-summaries", "sale-summary"},
	}
	for _, test := range tests {
		reprintSpec, ok := reportSpecForKey(test.reprintKind)
		if !ok {
			t.Fatalf("%s has no registered spec", test.reprintKind)
		}
		counterpartSpec, ok := reportSpecForKey(test.counterpartKind)
		if !ok {
			t.Fatalf("%s has no registered spec", test.counterpartKind)
		}
		if reprintSpec.aggregateCondition != counterpartSpec.aggregateCondition {
			t.Errorf("%s aggregateCondition = %q, counterpart %s = %q", test.reprintKind, reprintSpec.aggregateCondition, test.counterpartKind, counterpartSpec.aggregateCondition)
		}
		if reprintSpec.salesMode != counterpartSpec.salesMode {
			t.Errorf("%s salesMode = %q, counterpart %s = %q", test.reprintKind, reprintSpec.salesMode, test.counterpartKind, counterpartSpec.salesMode)
		}
		reprintQuery := salesReadModelQueryMode(reprintSpec.aggregateCondition, reprintSpec.salesMode, pagination)
		counterpartQuery := salesReadModelQueryMode(counterpartSpec.aggregateCondition, counterpartSpec.salesMode, pagination)
		if reprintQuery != counterpartQuery {
			t.Errorf("%s produces different SQL than %s; reprint leaves must read the same already-posted data as their non-reprint counterpart", test.reprintKind, test.counterpartKind)
		}
	}

	purchasePagination := "LIMIT $6 OFFSET $7"
	reprintPurchase, ok := reportSpecForKey("reprinting-purchase")
	if !ok {
		t.Fatal("reprinting-purchase has no registered spec")
	}
	purchaseDetail, ok := reportSpecForKey("purchase-detail")
	if !ok {
		t.Fatal("purchase-detail has no registered spec")
	}
	if reprintPurchase.aggregateCondition != purchaseDetail.aggregateCondition || reprintPurchase.purchaseMode != purchaseDetail.purchaseMode {
		t.Fatalf("reprinting-purchase spec (%+v) does not scope-match purchase-detail spec (%+v)", reprintPurchase, purchaseDetail)
	}
	reprintPurchaseQuery := purchaseLineDetailReadModelQuery(reprintPurchase.aggregateCondition, purchasePagination)
	purchaseDetailQuery := purchaseLineDetailReadModelQuery(purchaseDetail.aggregateCondition, purchasePagination)
	if reprintPurchaseQuery != purchaseDetailQuery {
		t.Error("reprinting-purchase produces different SQL than purchase-detail; reprint must read the same already-posted purchase data")
	}
}

// TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind locks the
// quotation-detail / quotation-summary wiring verified VACUOUS-NO-DATA in
// the Phase Q golden-verification pass (zero business_documents rows with
// kind='quotation' and zero sync_events rows with aggregate='quotation' in
// the sandbox tenant as of 2026-08-09) so the projection shape itself stays
// covered even though no sample data exists to replay.
func TestGoldenQuotationLeavesUseDocumentReadModelWithQuotationKind(t *testing.T) {
	for _, test := range []struct {
		kind string
		mode string
	}{
		{"quotation-detail", "line-detail"},
		{"quotation-summary", "invoice-summary"},
	} {
		spec, ok := reportSpecForKey(test.kind)
		if !ok {
			t.Fatalf("%s has no registered spec", test.kind)
		}
		if !spec.documentReadModel || spec.documentKind != "quotation" || spec.documentAggregate != "quotation" || spec.documentMode != test.mode {
			t.Fatalf("%s spec = %+v, want documentReadModel=true documentKind=quotation documentAggregate=quotation documentMode=%s", test.kind, spec, test.mode)
		}
		query := documentReadModelQuery(spec.documentKind, spec.documentAggregate, spec.documentMode, "LIMIT $6 OFFSET $7")
		if !strings.Contains(query, "bd.kind = 'quotation'") {
			t.Errorf("%s query does not scope canonical rows to kind='quotation':\n%s", test.kind, query)
		}
		if !strings.Contains(query, "se.aggregate = 'quotation'") {
			t.Errorf("%s query does not scope compatibility rows to aggregate='quotation':\n%s", test.kind, query)
		}
	}
}

// TestGoldenHeaderWiseTransactionSummaryIncludesMigratedSaleReturnKindSpelling
// locks the fix for a genuine data-coverage bug found during Phase Q golden
// verification (see docs/PHASE_Q_GOLDEN_VERIFICATION_DOCUMENT_REPRINT_2026-08-09.md):
// headerTransactionReadModelQuery's canonical kind IN-list previously used
// only the live-app sale-return kind spellings ('cash-return',
// 'credit-return', 'open-cash-return', 'open-credit-return') and omitted the
// migrated-historical spellings ('cash-sale-return', 'credit-sale-return',
// 'open-sale-return') that salesReadModelQuery/saleReturnReadModelQuery in
// this same file already include. In the sandbox tenant
// (eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee) this silently dropped all 30,704
// posted cash-sale-return documents (100% of the migrated sale_return
// family) from header-wise-transaction-summary with no compatibility-event
// fallback. The canonical kind IN-list now includes the migrated spellings,
// so all 30,704 documents surface in the summary. This test asserts the
// FIXED behavior so a future regression (dropping the migrated spellings
// again) is caught immediately.
func TestGoldenHeaderWiseTransactionSummaryIncludesMigratedSaleReturnKindSpelling(t *testing.T) {
	headerQuery := headerTransactionReadModelQuery("LIMIT $6 OFFSET $7")
	for _, present := range []string{"cash-sale-return", "credit-sale-return", "open-sale-return"} {
		if !strings.Contains(headerQuery, present) {
			t.Errorf("headerTransactionReadModelQuery is missing %q -- this reintroduces the bug documented in "+
				"PHASE_Q_GOLDEN_VERIFICATION_DOCUMENT_REPRINT_2026-08-09.md where migrated sale-return "+
				"documents (kind='cash-sale-return' etc.) are silently dropped from "+
				"header-wise-transaction-summary", present)
		}
	}
	// The sibling sale/return read models in this same file DO cover the
	// migrated spelling; confirm they still agree with the now-fixed header
	// query so all four queries stay consistent with each other.
	returnsQuery := saleReturnReadModelQuery("LIMIT $6 OFFSET $7")
	for _, present := range []string{"cash-sale-return", "credit-sale-return", "open-sale-return"} {
		if !strings.Contains(returnsQuery, present) {
			t.Errorf("saleReturnReadModelQuery unexpectedly lost %q; it previously included the migrated sale-return kind spelling", present)
		}
	}
}

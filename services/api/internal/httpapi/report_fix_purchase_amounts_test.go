package httpapi

import (
	"strings"
	"testing"
)

// This file locks the fix for two confirmed amount bugs in
// purchaseReadModelQuery's amountExpression, investigated and documented in
// docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md and
// docs/evidence/PHASE_N_O_GOLDEN_VERIFICATION_REMAINDER_2026-08-09.md.
//
// Ground truth, confirmed by hand against real sandbox-tenant documents
// (tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee) via direct psql inspection
// during this fix:
//
//   - purchase-order document #2810 (id 5bf93816-8840-5bac-ae15-325a037676d6,
//     branch ffffffff-ffff-ffff-ffff-ffffffffffff): business_documents.total_amount
//     = 0.0000, but its 10 business_document_lines rows, each independently
//     verified as quantity * unit_price (0 discounts/tax on this doc), sum to
//     20,502.4000 (e.g. line 129893: 10 * 589.0100 = 5,890.1000, matching
//     line_total exactly). ple.amount is NULL for every purchase-order (no
//     party_ledger_entries row exists yet - correct, since there's no
//     liability until goods are received). So COALESCE(ple.amount,
//     d.total_amount) always evaluates to 0 for purchase orders, even though
//     the real ordered value is clearly non-zero and computable from lines.
//
//   - purchase-return document #1856 (id d616768d-344c-543f-be93-cdd0e2a61abb,
//     branch ffffffff-ffff-ffff-ffff-ffffffffffff): business_documents.total_amount
//     = 20,052.0000 and the linked party_ledger_entries.debit_amount also
//     = 20,052.0000 (header and ledger agree with each other), but its 41
//     lines, each independently verified as quantity * unit_price (0
//     discounts/tax on this doc, e.g. line 2620: 154 * 15.67 = 2,413.1800
//     matching line_total exactly), sum to 85,897.1800 - a 4.3x gap from the
//     header/ledger figure. Tenant-wide, 566 of 634 posted purchase-return
//     documents (89.3%) show this same header-vs-line-sum disagreement (net
//     gap PKR 35,762.30), confirming the header/ledger amount is the stale
//     side (a migration-era value never reconciled after lines were
//     back-filled by migration/cmd/bulkorderlines), not the per-line sum.
//
// Fix: for the purchase_order and return aggregates only (receiving is
// unchanged - out of scope for this fix, and its header total, while also
// imperfect, was not part of the two bugs assigned here), non-detail modes
// (invoice-summary, day-summary, month-summary, item-summary,
// supplier-summary) now sum SUM(l.line_total) per document instead of
// trusting COALESCE(ple.amount, d.total_amount). detail mode already used
// l.line_total directly and needed no change - it was already reading the
// correct, hand-verified per-line source.

// TestPurchaseOrderAmountExpressionUsesLineTotalSum locks Bug 1's fix:
// purchase-order-summary / purchase-order / purchase-order-supplier-wise
// (aggregateCondition = se.aggregate = 'purchase_order', non-detail modes)
// must no longer rely solely on COALESCE(ple.amount, d.total_amount), which
// is 0.0000 for every one of the 2,810 posted purchase-order documents in
// the sandbox tenant (confirmed again directly against document #2810 during
// this fix: total_amount = 0, SUM(line_total) = 20,502.4000).
func TestPurchaseOrderAmountExpressionUsesLineTotalSum(t *testing.T) {
	for _, mode := range []string{"invoice-summary", "supplier-summary", "day-summary", "month-summary", "item-summary"} {
		sql := purchaseReadModelQuery("se.aggregate = 'purchase_order'", mode, "")
		if !strings.Contains(sql, "SUM(l.line_total)") {
			t.Fatalf("mode %q: purchaseReadModelQuery for purchase_order aggregate should sum l.line_total (business_documents.total_amount is 0.0000 for every sandbox purchase-order document, e.g. #2810); got:\n%s", mode, sql)
		}
	}
}

// TestPurchaseReturnAmountExpressionUsesLineTotalSum locks Bug 2's fix: the
// return aggregate's non-detail modes (purchase-return-summary,
// supplier-purchase-returns-summary) must now agree with the detail mode
// (supplier-purchase-returns-detail), which already summed l.line_total.
// Both now read the same canonical per-line source, eliminating the
// previously-documented PKR 35,762.30 net disagreement across 566/634
// documents (e.g. doc #1856: header 20,052.0000 vs line-sum 85,897.1800).
func TestPurchaseReturnAmountExpressionUsesLineTotalSum(t *testing.T) {
	for _, mode := range []string{"invoice-summary", "supplier-summary", "day-summary", "month-summary", "item-summary"} {
		sql := purchaseReadModelQuery("se.aggregate = 'return'", mode, "")
		if !strings.Contains(sql, "SUM(l.line_total)") {
			t.Fatalf("mode %q: purchaseReadModelQuery for return aggregate should sum l.line_total to agree with the detail-mode leaf (supplier-purchase-returns-detail already used l.line_total; e.g. doc #1856 header 20,052.0000 vs line-sum 85,897.1800 previously disagreed); got:\n%s", mode, sql)
		}
	}
}

// TestPurchaseReturnSummaryAndDetailNowAgreeOnAmountSource is the direct
// regression for Bug 2: purchase-return-summary (invoice-summary mode) and
// supplier-purchase-returns-detail (detail mode) must derive their Amount
// column from the identical canonical source (l.line_total) for return
// documents, so the two sibling leaves no longer disagree for the same
// underlying documents.
func TestPurchaseReturnSummaryAndDetailNowAgreeOnAmountSource(t *testing.T) {
	summarySQL := purchaseReadModelQuery("se.aggregate = 'return'", "invoice-summary", "")
	detailSQL := purchaseReadModelQuery("se.aggregate = 'return'", "detail", "")

	if !strings.Contains(summarySQL, "SUM(l.line_total)") {
		t.Fatalf("purchase-return-summary (invoice-summary mode) should now use SUM(l.line_total); got:\n%s", summarySQL)
	}
	if !strings.Contains(detailSQL, "l.line_total") {
		t.Fatalf("supplier-purchase-returns-detail (detail mode) should use l.line_total; got:\n%s", detailSQL)
	}
	// Neither should still fall back to the stale header/ledger amount as
	// the primary source for return documents.
	if strings.Contains(summarySQL, "COALESCE(ple.amount, d.total_amount)") {
		t.Fatalf("purchase-return-summary should no longer select COALESCE(ple.amount, d.total_amount) as its amount; got:\n%s", summarySQL)
	}
}

// TestPurchaseOrderSupplierWiseAmountExpressionUsesLineTotalSum locks the
// supplier-summary variant of Bug 1 (purchase-order-supplier-wise), which is
// built on top of purchaseReadModelQuery via purchaseSupplierSummaryReadModelQuery.
func TestPurchaseOrderSupplierWiseAmountExpressionUsesLineTotalSum(t *testing.T) {
	sql := purchaseSupplierSummaryReadModelQuery("se.aggregate = 'purchase_order'", "")
	if !strings.Contains(sql, "SUM(l.line_total)") {
		t.Fatalf("purchaseSupplierSummaryReadModelQuery for purchase_order aggregate should build on the line-total-sum base query; got:\n%s", sql)
	}
}

// TestReceivingAmountExpressionUnchanged confirms the fix is scoped strictly
// to purchase_order and return aggregates: the receiving aggregate (regular
// pack-purchase/loose-purchase/opening-purchase documents, used by
// purchase-summary and friends) is untouched by this fix and continues to
// use COALESCE(ple.amount, d.total_amount), matching the already-verified
// MATCHED result in docs/evidence/PHASE_O_GOLDEN_VERIFICATION_PURCHASE_2026-08-09.md
// (6,418 docs, amount 198,071,256.0000). Any further receiving-side amount
// issue is out of scope for this fix and is left for a dedicated
// investigation/agent.
func TestReceivingAmountExpressionUnchanged(t *testing.T) {
	sql := purchaseReadModelQuery("se.aggregate = 'receiving'", "invoice-summary", "")
	if !strings.Contains(sql, "COALESCE(ple.amount, d.total_amount)") {
		t.Fatalf("receiving aggregate's invoice-summary mode should still use COALESCE(ple.amount, d.total_amount) - this fix is scoped only to purchase_order/return aggregates; got:\n%s", sql)
	}
	if strings.Contains(sql, "SUM(l.line_total)") {
		t.Fatalf("receiving aggregate's invoice-summary mode should not have been changed to SUM(l.line_total) by this fix; got:\n%s", sql)
	}
}

// TestPurchaseDetailModeAmountExpressionUnchangedByFix confirms detail mode
// (already using l.line_total, the correct canonical source, for every
// aggregate) is unaffected by this fix - it needed no change.
func TestPurchaseDetailModeAmountExpressionUnchangedByFix(t *testing.T) {
	for _, agg := range []string{"se.aggregate = 'receiving'", "se.aggregate = 'return'", "se.aggregate = 'purchase_order'"} {
		sql := purchaseReadModelQuery(agg, "detail", "")
		if !strings.Contains(sql, "l.line_total` AS amount") && !strings.Contains(sql, "l.line_total::text AS amount") {
			// amountExpression is interpolated directly before "::text AS amount"
			if !strings.Contains(sql, "l.line_total") {
				t.Fatalf("aggregate %q: detail mode should select l.line_total as amount; got:\n%s", agg, sql)
			}
		}
	}
}

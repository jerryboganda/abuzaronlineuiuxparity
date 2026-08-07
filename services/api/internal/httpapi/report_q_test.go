package httpapi

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturedCatalogNonBlankReportLeavesResolveToExplicitDefinitions(t *testing.T) {
	type catalogItem struct {
		Path       string `json:"path"`
		HasSubmenu bool   `json:"hasSubmenu"`
	}
	var catalog struct {
		Items []catalogItem `json:"items"`
	}
	path := filepath.Join("..", "..", "..", "..", "parity", "catalog", "legacy-menu-tree-2026-08-05.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report catalog: %v", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("decode report catalog: %v", err)
	}
	leaves := 0
	for _, item := range catalog.Items {
		clean := strings.TrimSpace(strings.ReplaceAll(item.Path, "&", ""))
		if item.HasSubmenu || !strings.HasPrefix(clean, "Reports > ") || strings.HasSuffix(clean, " >") {
			continue
		}
		leaves++
		if got := reportRegistryKey("", item.Path); got == "" {
			t.Errorf("catalog leaf has no explicit registry definition: %q", item.Path)
		}
	}
	if leaves != 151 {
		t.Fatalf("catalog has %d non-blank report leaves, want 151", leaves)
	}
}

func TestPhaseQRegistryCoversTheMappedRemainingLeaves(t *testing.T) {
	if got, want := len(phaseQReportRegistry), 32; got != want {
		t.Fatalf("Phase Q registry has %d leaves, want %d", got, want)
	}
	for _, kind := range []string{
		"accounts-reports-ledger-reports-accounts-ledger",
		"customer-sales-lp-ledger",
		"listing-group-rights-list",
		"reprinting-sale",
		"item-reports-deleted-sale-items-log",
		"item-reports-history-sale-price-difference",
		"item-reports-history-item-basic-data-changes",
		"item-reports-history-item-sale-price-changes",
		"item-reports-history-new-item-s-created-defined",
		"item-reports-history-item-name-changes",
		"item-reports-stock-adjustments-stock-adjustments-detail",
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

func TestPhaseQReprintDefinitionsUseCanonicalReadModels(t *testing.T) {
	tests := map[string]struct {
		mode       string
		columnSize int
		firstLabel string
		notePart   string
	}{
		"reprinting-sale":                            {"sale-detail", 11, "Alias", "Canonical posted sale lines"},
		"reprinting-sale-format-2":                   {"sale-detail", 11, "Alias", "Canonical posted sale lines"},
		"reprinting-sale-format-3":                   {"sale-detail", 11, "Alias", "Canonical posted sale lines"},
		"reprinting-sale-format-4":                   {"sale-detail", 11, "Alias", "Canonical posted sale lines"},
		"reprinting-sale-with-summary-reports":       {"sale-summary", 6, "Invoice", "Canonical posted sale invoice summaries"},
		"reprinting-sale-with-header-wise-summaries": {"sale-summary", 6, "Invoice", "Canonical posted sale invoice summaries"},
		"reprinting-selected-sales-and-summaries":    {"sale-summary", 6, "Invoice", "Canonical posted sale invoice summaries"},
		"reprinting-purchase":                        {"purchase-detail", 12, "Document", "Canonical posted purchase lines"},
	}
	for kind, want := range tests {
		definition := reportDefinitionFor(kind)
		spec, ok := reportSpecForKey(kind)
		if !ok {
			t.Fatalf("%s has no explicit report definition", kind)
		}
		if spec.reprintMode != want.mode {
			t.Errorf("%s reprint mode = %q, want %q", kind, spec.reprintMode, want.mode)
		}
		if definition.ProjectionStatus != "real" {
			t.Errorf("%s projection status = %q, want real", kind, definition.ProjectionStatus)
		}
		if len(definition.Columns) != want.columnSize || definition.Columns[0].Label != want.firstLabel {
			t.Errorf("%s columns = %+v, want %d columns beginning with %q", kind, definition.Columns, want.columnSize, want.firstLabel)
		}
		if !strings.Contains(definition.ProjectionNote, want.notePart) || !strings.Contains(definition.ProjectionNote, "unverified") {
			t.Errorf("%s note does not disclose its source and remaining print boundary: %q", kind, definition.ProjectionNote)
		}
	}

	saleSpec, _ := reportSpecForKey("reprinting-sale")
	if saleSpec.salesMode != "line-detail" || !saleSpec.salesReadModel {
		t.Fatalf("sale reprint is not wired to the canonical line read model: %+v", saleSpec)
	}
	purchaseSpec, _ := reportSpecForKey("reprinting-purchase")
	if purchaseSpec.purchaseMode != "line-detail" || !purchaseSpec.purchaseReadModel {
		t.Fatalf("purchase reprint is not wired to the canonical line read model: %+v", purchaseSpec)
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
		{"gl-journal", "historical-gl", "real", []string{"historical_gl_entries", "VirtualGl", "gl_journals"}, "Document Code"},
		{"trial-balance", "trial-balance", "real", []string{"historical_gl_entries", "opening", "historical"}, "Credit"},
		{"customer-statement", "party-customer", "real", []string{"party_ledger_entries", "historical_party_payment_allocations"}, "Description"},
		{"receivables-aging", "aging-receivable", "real", []string{"DueDate", "aging", "unaged"}, "Aging Status"},
		{"payables-aging", "aging-payable", "real", []string{"CreditDays", "unaged"}, "Aging Status"},
		{"tax-register", "tax-output", "real", []string{"tax snapshots", "not recomputed"}, "Tax Amount"},
		{"customer-sales-customer-wise-advance-tax", "tax-advance", "real", []string{"advance-tax", "amount", "omitted"}, "Tax Amount"},
		{"supplier-wise-advance-income-tax", "tax-advance-input", "real", []string{"advance-tax", "amount", "omitted"}, "Tax Amount"},
		{"withholding-tax-deduction", "tax-withholding", "real", []string{"PurPayment", "advance tax", "grouping"}, "Withholding Amount"},
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
	for _, mode := range []string{"gl", "trial-balance", "party-customer", "aging-receivable", "aging-payable", "tax-output", "tax-advance", "tax-advance-input", "voucher"} {
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
		financeReadModelQuery("aging-receivable", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("aging-payable", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("tax-output", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("tax-advance", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("tax-advance-input", "LIMIT $6 OFFSET $7"),
		financeReadModelQuery("voucher", "LIMIT $6 OFFSET $7"),
	} {
		if !strings.Contains(query, "posted") {
			t.Errorf("posted-only predicate missing from query:\n%s", query)
		}
	}
	trialBalance := financeReadModelQuery("trial-balance", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{"WITH balances AS", "FROM historical_gl_entries h", "Legacy account", "UNION ALL"} {
		if !strings.Contains(trialBalance, fragment) {
			t.Errorf("trial-balance query is missing historical source fragment %q", fragment)
		}
	}
	accountsLedger := financeReadModelQuery("gl", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{"WITH ledger_rows AS", "FROM historical_gl_entries h", "historical:' || h.legacy_id", "UNION ALL"} {
		if !strings.Contains(accountsLedger, fragment) {
			t.Errorf("accounts-ledger query is missing historical source fragment %q", fragment)
		}
	}
	if strings.Contains(financeReadModelQuery("gl-cash", "LIMIT $6 OFFSET $7"), "FROM historical_gl_entries h") {
		t.Fatal("cash-account ledger unexpectedly inferred historical VirtualGl rows")
	}
	for _, mode := range []string{"party-customer", "party-supplier"} {
		query := financeReadModelQuery(mode, "LIMIT $6 OFFSET $7")
		for _, fragment := range []string{
			"FROM historical_party_payment_allocations p",
			"FROM historical_party_ledger_adjustments a",
			"FROM historical_party_return_allocations r",
			"UNION ALL",
			"p.payment_amount",
			"p.posted = true",
			"a.occurred_at IS NOT NULL",
			"historical_d.legacy_source_table <> ''",
			"source_document_legacy_id",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s query is missing source-backed payment fragment %q", mode, fragment)
			}
		}
	}
	receivableAging := financeReadModelQuery("aging-receivable", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"JOIN business_documents d",
		"d.legacy_payload->>'DueDate'",
		"d.pricing_result->>'dueDate'",
		"SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE 0 END)",
		"SUM(CASE WHEN ple.credit_amount > 0 THEN d.balance_amount ELSE 0 END)",
		"HAVING SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE -d.balance_amount END) <> 0",
		"0-30 days",
		"91+ days",
		"GROUP BY ple.party_id, mp.name, due.due_date",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(receivableAging, fragment) {
			t.Errorf("receivables-aging query is missing %q", fragment)
		}
	}
	payableAging := financeReadModelQuery("aging-payable", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"JOIN business_documents d",
		"d.legacy_payload->>'CreditDays'",
		"d.pricing_result->>'creditDays'",
		"SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE 0 END)",
		"SUM(CASE WHEN ple.credit_amount > 0 THEN d.balance_amount ELSE 0 END)",
		"HAVING SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE -d.balance_amount END) <> 0",
		"d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return')",
		"0-30 days",
		"91+ days",
		"GROUP BY ple.party_id, mp.name, due.due_date",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(payableAging, fragment) {
			t.Errorf("payables-aging query is missing %q", fragment)
		}
	}
	historicalGL := historicalGLReadModelQuery("LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"FROM historical_gl_entries h",
		"UNION ALL",
		"FROM gl_journals j",
		"h.tenant_id = $1::uuid AND h.branch_id = $2::uuid",
		"j.status = 'posted'",
		"occurred_at >= $3::date",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(historicalGL, fragment) {
			t.Errorf("historical GL query is missing %q", fragment)
		}
	}
	withholding := financeReadModelQuery("tax-withholding", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"FROM historical_withholding_tax_entries h",
		"h.posted = true",
		"h.amount <> 0",
		"JOIN business_documents d",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(withholding, fragment) {
			t.Errorf("withholding query is missing %q", fragment)
		}
	}
	advanceTax := financeReadModelQuery("tax-advance", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"tax.value->>'kind' = 'advance_tax'",
		"advance_tax.amount > 0",
		"l.advance_tax_rate > 0",
		"d.legacy_payload->>'AdvanceTaxAmt'",
		"SELECT MIN(l2.line_number)",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(advanceTax, fragment) {
			t.Errorf("advance-tax query is missing %q", fragment)
		}
	}
	if strings.Contains(advanceTax, "l.tax_amount::text") || strings.Contains(advanceTax, "l.tax_amount > 0") {
		t.Fatal("advance-tax query reinterpreted the aggregate line tax amount")
	}
	explicitAmount := strings.Index(advanceTax, "SUM(CASE WHEN NULLIF(tax.value->>'amount'")
	legacyAmount := strings.Index(advanceTax, "d.legacy_payload->>'AdvanceTaxAmt'")
	if explicitAmount < 0 || legacyAmount < 0 || explicitAmount > legacyAmount {
		t.Fatal("advance-tax query does not prioritize explicit snapshot amount over the legacy fallback")
	}
	advanceTaxInput := financeReadModelQuery("tax-advance-input", "LIMIT $6 OFFSET $7")
	if !strings.Contains(advanceTaxInput, "d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return')") {
		t.Fatal("supplier advance-tax query is not scoped to purchase document kinds")
	}
	for _, mode := range []string{"tax-output", "tax-input"} {
		query := financeReadModelQuery(mode, "LIMIT $6 OFFSET $7")
		if !strings.Contains(query, "l.tax_amount::text") {
			t.Errorf("%s query lost its aggregate tax amount projection", mode)
		}
		if strings.Contains(query, "tax.value->>'kind' = 'advance_tax'") {
			t.Errorf("%s query unexpectedly uses the advance-tax snapshot branch", mode)
		}
	}
	if query := emptyReportQuery("LIMIT $6 OFFSET $7"); !strings.Contains(query, "WHERE false") {
		t.Fatal("item-history fallback returned a fabricated source instead of an empty projection")
	}
	if query := compatibilityReportQuery("se.aggregate = 'quotation'", "LIMIT $6 OFFSET $7"); !strings.Contains(query, "COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'") {
		t.Fatal("compatibility report query is not explicitly posted-only")
	}
	sales := salesReadModelQuery(reportSaleAggregate, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"sd.status = 'posted'",
		"bd.status = 'posted'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
	} {
		if !strings.Contains(sales, fragment) {
			t.Errorf("sales read model is missing posted/duplicate guard %q", fragment)
		}
	}
	returns := saleReturnReadModelQuery("LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"bd.status = 'posted'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"NOT EXISTS",
	} {
		if !strings.Contains(returns, fragment) {
			t.Errorf("sale-return read model is missing posted/duplicate guard %q", fragment)
		}
	}
}

func TestPartyStatementDefinitionsDiscloseReturnAllocations(t *testing.T) {
	for _, test := range []struct {
		key      string
		fragment string
	}{
		{key: "customer-statement", fragment: "SRAllocationDetail return allocations"},
		{key: "supplier-statement", fragment: "PRAllocationDetail return allocations"},
	} {
		definition := reportDefinitionForKey("finance", test.key)
		if !strings.Contains(definition.Retrieval.Scope, test.fragment) {
			t.Fatalf("%s retrieval scope = %q; missing %q", test.key, definition.Retrieval.Scope, test.fragment)
		}
	}
}

func TestPhaseQItemHistoryDefinitionsUseSourceBackedProjections(t *testing.T) {
	tests := map[string]struct {
		title string
		mode  string
	}{
		"item-reports-history-sale-price-difference":              {"Sale Price Difference", "item-price-difference"},
		"item-reports-history-item-basic-data-changes":            {"Item Basic Data Changes", "item-basic-data"},
		"item-reports-history-item-sale-price-changes":            {"Item Sale Price Changes", "item-sale-price"},
		"item-reports-history-new-item-s-created-defined":         {"New Item(s) Created/Defined", "item-first-observed"},
		"item-reports-history-item-name-changes":                  {"Item Name Changes", "item-name"},
		"item-reports-stock-adjustments-stock-adjustments-detail": {"Stock Adjustments Detail", "stock-adjustments"},
		"item-reports-deleted-sale-items-log":                     {"Deleted Sale Items Log", "deleted-sale-items"},
	}
	for kind, test := range tests {
		definition := reportDefinitionFor(kind)
		spec, ok := reportSpecForKey(kind)
		if !ok || spec.historyMode != test.mode {
			t.Errorf("%s history mode = %q, want %q", kind, spec.historyMode, test.mode)
		}
		if definition.Title != test.title {
			t.Errorf("%s title = %q, want %q", kind, definition.Title, test.title)
		}
		if definition.ProjectionStatus != "real" {
			t.Errorf("%s projection status = %q, want real source-backed projection", kind, definition.ProjectionStatus)
		}
		if !strings.Contains(definition.ProjectionNote, "source-backed") || !strings.Contains(definition.ProjectionNote, "unverified") {
			t.Errorf("%s does not disclose source-backed and remaining parity boundaries: %q", kind, definition.ProjectionNote)
		}
	}
}

func TestPhaseQHistoricalQueriesAreScopeBoundAndPaginated(t *testing.T) {
	itemQuery := historicalReportQuery("item-name", "LIMIT $7 OFFSET $8")
	for _, fragment := range []string{
		"FROM historical_item_changes h",
		"h.tenant_id = $1::uuid AND h.branch_id = $2::uuid",
		"h.report_kind = $6",
		"h.occurred_at >= $3::date",
		"LIMIT $7 OFFSET $8",
	} {
		if !strings.Contains(itemQuery, fragment) {
			t.Errorf("item-history query is missing %q", fragment)
		}
	}
	adjustmentQuery := historicalReportQuery("stock-adjustments", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"WITH adjustment_rows AS (",
		"FROM historical_stock_adjustment_lines h",
		"FROM stock_ledger l",
		"l.direction = 'adjustment'",
		"JOIN sync_events se",
		"l.quantity * l.adjustment_sign",
		"h.tenant_id = $1::uuid AND h.branch_id = $2::uuid",
		"h.occurred_at < ($4::date + INTERVAL '1 day')",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(adjustmentQuery, fragment) {
			t.Errorf("adjustment query is missing %q", fragment)
		}
	}
	deletedQuery := historicalReportQuery("deleted-sale-items", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"FROM historical_deleted_sale_items h",
		"LEFT JOIN master_items mi",
		"h.tenant_id = $1::uuid AND h.branch_id = $2::uuid",
		"h.occurred_at >= $3::date",
		"h.payload::text ILIKE",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(deletedQuery, fragment) {
			t.Errorf("deleted-sale-items query is missing %q", fragment)
		}
	}
}

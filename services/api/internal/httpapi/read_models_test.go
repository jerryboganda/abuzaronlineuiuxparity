package httpapi

import (
	"strings"
	"testing"
)

func TestSalesReadModelIncludesCanonicalLinesAndPartyScope(t *testing.T) {
	query := salesReadModelQuery(reportSaleAggregate, "LIMIT $6")
	for _, fragment := range []string{
		"FROM business_documents bd",
		"business_document_lines bl",
		"master_parties mp",
		"bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid",
		"bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id",
		"bd.total_amount::text AS amount",
		"bd.status = 'posted'",
		"sd.status = 'posted'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("canonical sales read model is missing %q", fragment)
		}
	}
}

func TestSalesReadModelDoesNotDuplicateCompatibilityDocuments(t *testing.T) {
	query := salesReadModelQuery(reportSaleAggregate, "LIMIT $6")
	for _, fragment := range []string{
		"UNION ALL",
		"NOT EXISTS (",
		"FROM business_documents bd",
		"FROM sales_documents sd",
		"bd.id = sd.id OR bd.document_number = sd.document_number",
		"sd.id = se.aggregate_id",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("sales read model is missing duplicate guard %q", fragment)
		}
	}
	if strings.Contains(query, "se.aggregate IN ('sale', 'sale_return')") {
		t.Fatal("sale-only read model unexpectedly included sale returns")
	}
	if !strings.Contains(salesReadModelQuery(reportSaleOrReturn, "LIMIT $6"), "se.aggregate IN ('sale', 'sale_return')") {
		t.Fatal("sale/return read model did not include both compatibility aggregates")
	}
	combined := salesReadModelQuery(reportSaleOrReturn, "LIMIT $6")
	if !strings.Contains(combined, "'open-cash-return', 'open-credit-return'") {
		t.Fatal("sale/return read model did not include canonical open returns")
	}
}

func TestSaleReturnReadModelIncludesCanonicalAndCompatibilitySources(t *testing.T) {
	query := saleReturnReadModelQuery("LIMIT $6")
	for _, fragment := range []string{
		"FROM business_documents bd",
		"bd.kind IN ('cash-return', 'credit-return'",
		"'open-cash-return', 'open-credit-return'",
		"business_document_lines bl",
		"FROM sync_events se",
		"se.aggregate IN ('sale_return', 'sale-return')",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"bd.id = se.aggregate_id",
		"bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("sale-return read model is missing %q", fragment)
		}
	}
}

func TestInvoiceSummaryReadModelsGroupRowsOncePerDocument(t *testing.T) {
	for name, query := range map[string]string{
		"sale":   salesReadModelQueryMode(reportSaleAggregate, "invoice-summary", "LIMIT $6 OFFSET $7"),
		"return": saleReturnReadModelQueryMode("invoice-summary", "LIMIT $6 OFFSET $7"),
	} {
		for _, fragment := range []string{
			"WITH invoice_rows AS (",
			"GROUP BY document",
			"SUM(CASE WHEN quantity ~",
			"MAX(CASE WHEN amount ~",
			"::numeric(19,4)::text",
			"LIMIT $6 OFFSET $7",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s invoice summary is missing %q", name, fragment)
			}
		}
	}
	sale, saleOK := reportSpecForKey("sale-summary")
	if !saleOK || !sale.salesReadModel || sale.salesMode != "invoice-summary" {
		t.Fatalf("sale-summary spec = %+v (ok=%v), want invoice-summary read model", sale, saleOK)
	}
	saleReturn, saleReturnOK := reportSpecForKey("sales-return-summary")
	if !saleReturnOK || !saleReturn.salesReadModel || saleReturn.salesMode != "invoice-summary" {
		t.Fatalf("sales-return-summary spec = %+v (ok=%v), want invoice-summary read model", saleReturn, saleReturnOK)
	}
	customerInvoice, customerInvoiceOK := reportSpecForKey("customer-sales-invoice-summary")
	if !customerInvoiceOK || !customerInvoice.salesReadModel || customerInvoice.salesMode != "invoice-summary" {
		t.Fatalf("customer-sales-invoice-summary spec = %+v (ok=%v), want invoice-summary read model", customerInvoice, customerInvoiceOK)
	}
	customerDefinition := reportDefinitionFor("customer-sales-invoice-summary")
	if !strings.Contains(customerDefinition.ProjectionNote, "once per invoice") {
		t.Fatalf("customer invoice summary note = %q, want de-duplicated invoice semantics", customerDefinition.ProjectionNote)
	}
}

func TestCustomerSalesSummaryReadModelsUseExplicitBuckets(t *testing.T) {
	cases := map[string]struct {
		mode     string
		fragment string
		label    string
	}{
		"customer-sales-days-summary": {
			mode:     "day-summary",
			fragment: "GROUP BY period, party",
			label:    "Day",
		},
		"customer-sales-items-summary": {
			mode:     "item-summary",
			fragment: "GROUP BY item_name, party",
			label:    "Item",
		},
		"customer-sales-monthly-net-sales": {
			mode:     "month-summary",
			fragment: "date_trunc('month', occurred_at::date)::date",
			label:    "Month",
		},
		"customer-sales-hourly-graph": {
			mode:     "hour-summary",
			fragment: "date_trunc('hour', occurred_at)",
			label:    "Hour",
		},
		"customer-sales-customer-category-wise-sales-customer-wise-summary": {
			mode:     "customer-summary",
			fragment: "GROUP BY customer",
			label:    "Customer",
		},
		"customer-sales-customer-category-wise-sales-net-sales-and-volume": {
			mode:     "customer-summary",
			fragment: "GROUP BY customer",
			label:    "Customer",
		},
		"customer-sales-customer-category-wise-net-sales": {
			mode:     "customer-category-summary",
			fragment: "GROUP BY category",
			label:    "Customer Category",
		},
		"customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report": {
			mode:     "customer-category-summary",
			fragment: "GROUP BY category",
			label:    "Customer Category",
		},
		"customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report": {
			mode:     "customer-category-summary",
			fragment: "GROUP BY category",
			label:    "Customer Category",
		},
		"customer-sales-customer-wise-category-net-sales": {
			mode:     "customer-wise-category-summary",
			fragment: "GROUP BY customer, category",
			label:    "Customer",
		},
		"monthly-net-sales-summary": {
			mode:     "month-summary",
			fragment: "date_trunc('month', occurred_at::date)::date",
			label:    "Month",
		},
	}
	for kind, test := range cases {
		spec, ok := reportSpecForKey(kind)
		if !ok || !spec.salesReadModel || spec.salesMode != test.mode {
			t.Fatalf("%s spec = %+v (ok=%v), want %s sales projection", kind, spec, ok, test.mode)
		}
		query := salesReadModelQueryMode(reportSaleAggregate, test.mode, "LIMIT $6 OFFSET $7")
		for _, fragment := range []string{test.fragment, "numeric(19,4)::text", "LIMIT $6 OFFSET $7"} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s %s query is missing %q", kind, test.mode, fragment)
			}
		}
		definition := reportDefinitionFor(kind)
		if len(definition.Columns) != 6 || definition.Columns[0].Label != test.label {
			t.Errorf("%s definition columns = %+v, want %s summary columns", kind, definition.Columns, test.label)
		}
		if !strings.Contains(definition.ProjectionNote, test.mode) {
			t.Errorf("%s definition note = %q, want explicit %s mode", kind, definition.ProjectionNote, test.mode)
		}
	}
}

func TestCustomerSalesCategorySummaryUsesCustomerCategoryPayload(t *testing.T) {
	query := salesReadModelQueryMode(reportSaleAggregate, "customer-category-summary", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"master_parties mp",
		"mp.payload->>'Category'",
		"mp.payload->>'CustomerCategory'",
		"mp.payload->>'category'",
		"'Unspecified'",
		"FROM sales_documents sd",
		"FROM sync_events se",
		"jsonb_array_elements",
		"sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid",
		"se.tenant_id = $1::uuid AND se.branch_id = $2::uuid",
		"GROUP BY category",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("customer-category summary query is missing %q", fragment)
		}
	}
	definition := reportDefinitionFor("customer-sales-customer-category-wise-net-sales")
	if len(definition.Columns) != 6 || definition.Columns[0].Label != "Customer Category" || definition.Columns[4].Label != "Volume" {
		t.Errorf("customer-category summary definition columns = %+v", definition.Columns)
	}
}

func TestCustomerWiseCategorySummaryGroupsByCustomerAndCategory(t *testing.T) {
	query := salesReadModelQueryMode(reportSaleAggregate, "customer-wise-category-summary", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"customer, MAX(occurred_at)::text, customer, category",
		"GROUP BY customer, category",
		"ORDER BY customer, category",
		"customer ILIKE '%' || $5 || '%'",
		"master_parties mp",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("customer-wise category summary query is missing %q", fragment)
		}
	}
	definition := reportDefinitionFor("customer-sales-customer-wise-category-net-sales")
	if len(definition.Columns) != 6 || definition.Columns[0].Label != "Customer" || definition.Columns[3].Label != "Category" {
		t.Errorf("customer-wise category summary definition columns = %+v", definition.Columns)
	}
}

func TestCustomerCategorySalesDetailReportUsesLineDetailProjection(t *testing.T) {
	spec, ok := reportSpecForKey("customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report")
	if !ok || !spec.salesReadModel || spec.salesMode != "line-detail" {
		t.Fatalf("customer-category detail spec = %+v (ok=%v), want line-detail sales projection", spec, ok)
	}
	query := salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"business_document_lines",
		"AliasName",
		"sales_detail_read_model",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("customer-category detail query is missing %q", fragment)
		}
	}
	definition := reportDefinitionFor("customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report")
	if len(definition.Columns) != 11 || definition.Columns[0].Label != "Alias" {
		t.Errorf("customer-category detail definition columns = %+v", definition.Columns)
	}
}

func TestCustomerSalesProfitMarginReadModelUsesAllocatedCost(t *testing.T) {
	spec, ok := reportSpecForKey("customer-sales-invoice-wise-profit-margin-detail")
	if !ok || !spec.salesReadModel || spec.salesMode != "profit-margin-detail" {
		t.Fatalf("profit-margin spec = %+v (ok=%v), want explicit sales projection", spec, ok)
	}
	query := salesReadModelQueryMode(reportSaleAggregate, spec.salesMode, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"stock_allocations",
		"sa.quantity * sa.unit_cost",
		"gross_profit",
		"margin_percent",
		"allocated_cost",
		"NOT EXISTS",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("profit-margin query is missing %q", fragment)
		}
	}
	definition := reportDefinitionFor("customer-sales-invoice-wise-profit-margin-detail")
	if len(definition.Columns) != 11 || definition.Columns[0].Label != "Invoice" || definition.Columns[8].Label != "Gross Profit" {
		t.Fatalf("profit-margin definition columns = %+v, want 11-field invoice margin contract", definition.Columns)
	}
	if !strings.Contains(definition.ProjectionNote, "stock allocation cost") {
		t.Fatalf("profit-margin definition note = %q, want allocated-cost boundary", definition.ProjectionNote)
	}
}

func TestDailySalesProfitSummaryAggregatesCompleteCostRows(t *testing.T) {
	spec, ok := reportSpecForKey("daily-sales-summary-with-profit-day-wise-grouping")
	if !ok || !spec.salesReadModel || spec.salesMode != "profit-day-summary" {
		t.Fatalf("profit-day-summary spec = %+v (ok=%v), want explicit sales projection", spec, ok)
	}
	query := salesReadModelQueryMode(reportSaleAggregate, spec.salesMode, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"WITH profit_rows AS (",
		"GROUP BY period, party",
		"cost_count = row_count",
		"amount / quantity",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("profit day summary query is missing %q", fragment)
		}
	}
	definition := reportDefinitionFor("daily-sales-summary-with-profit-day-wise-grouping")
	if len(definition.Columns) != 11 || definition.Columns[0].Label != "Day" || definition.Columns[8].Label != "Gross Profit" {
		t.Fatalf("profit day summary definition columns = %+v, want 11-field day margin contract", definition.Columns)
	}
}

func TestCustomerSalesGrossProfitSummaryGroupsByCustomer(t *testing.T) {
	const kind = "customer-sales-customer-category-wise-sales-customer-wise-gross-profit"
	spec, ok := reportSpecForKey(kind)
	if !ok || !spec.salesReadModel || spec.salesMode != "profit-customer-summary" {
		t.Fatalf("customer gross-profit spec = %+v (ok=%v), want explicit customer profit projection", spec, ok)
	}
	query := salesReadModelQueryMode(reportSaleAggregate, spec.salesMode, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"WITH profit_rows AS (",
		"GROUP BY customer",
		"'Customer total'",
		"cost_count = row_count",
		"amount / quantity",
		"LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("customer profit summary query is missing %q", fragment)
		}
	}
	definition := reportDefinitionFor(kind)
	if len(definition.Columns) != 11 || definition.Columns[0].Label != "Customer" || definition.Columns[8].Label != "Gross Profit" {
		t.Fatalf("customer profit definition columns = %+v, want 11-field customer margin contract", definition.Columns)
	}
	if !strings.Contains(definition.ProjectionNote, "profit-customer-summary") {
		t.Fatalf("customer profit definition note = %q, want explicit customer profit mode", definition.ProjectionNote)
	}
}

func TestSalesProfitSummaryModesUseExtendedScanner(t *testing.T) {
	for _, mode := range []string{"profit-margin-detail", "profit-day-summary", "profit-customer-summary"} {
		if !salesSummaryUsesProfitMarginScanner(mode) {
			t.Fatalf("sales mode %q did not select the 11-field profit scanner", mode)
		}
	}
	if salesSummaryUsesProfitMarginScanner("customer-summary") {
		t.Fatal("six-field customer summary selected the 11-field profit scanner")
	}
}

func TestSalesReadModelAlwaysCarriesTenantAndBranchPredicates(t *testing.T) {
	for _, condition := range []string{reportSaleAggregate, reportSaleOrReturn} {
		query := salesReadModelQuery(condition, "LIMIT $6")
		if strings.Count(query, "se.tenant_id = $1::uuid AND se.branch_id = $2::uuid") == 0 {
			t.Fatalf("compatibility event branch is not tenant/branch scoped for %q", condition)
		}
		if strings.Count(query, "sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid") == 0 {
			t.Fatalf("compatibility projection is not tenant/branch scoped for %q", condition)
		}
	}
}

func TestSalesReportDefinitionsDescribeCanonicalUnionAndReturnCompatibility(t *testing.T) {
	sale := reportDefinitionFor("sale-detail")
	if !strings.Contains(sale.ProjectionNote, "business_documents and lines") {
		t.Fatalf("sale definition note does not describe canonical union: %q", sale.ProjectionNote)
	}
	if spec, ok := reportSpecForKey("sale-detail"); !ok || spec.salesMode != "line-detail" {
		t.Fatalf("sale-detail spec = %+v (ok=%v), want line-detail read model", spec, ok)
	}
	if len(sale.Columns) != 11 || !strings.Contains(sale.ProjectionNote, "line-detail contract") {
		t.Fatalf("sale-detail definition = %+v, want source-backed line-detail columns", sale)
	}
	saleReturn := reportDefinitionFor("sales-return-detail")
	if !strings.Contains(saleReturn.ProjectionNote, "business_documents") {
		t.Fatalf("sales-return definition does not describe canonical documents: %q", saleReturn.ProjectionNote)
	}
	if !strings.Contains(saleReturn.ProjectionNote, "compatibility sale_return events") {
		t.Fatalf("sales-return definition lost compatibility projection note: %q", saleReturn.ProjectionNote)
	}
	if spec, ok := reportSpecForKey("sales-return-detail"); !ok || spec.salesMode != "line-detail" {
		t.Fatalf("sales-return-detail spec = %+v (ok=%v), want line-detail read model", spec, ok)
	}
	if len(saleReturn.Columns) != 11 || !strings.Contains(saleReturn.ProjectionNote, "line-detail contract") {
		t.Fatalf("sales-return-detail definition = %+v, want source-backed line-detail columns", saleReturn)
	}
	for name, query := range map[string]string{
		"sale":   salesReadModelQueryMode(reportSaleAggregate, "line-detail", "LIMIT $6 OFFSET $7"),
		"return": saleReturnReadModelQueryMode("line-detail", "LIMIT $6 OFFSET $7"),
	} {
		for _, fragment := range []string{"legacy_payload", "sale_price", "discount_percent", "sales_tax_value", "batch_number", "LIMIT $6 OFFSET $7"} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s line-detail query is missing %q", name, fragment)
			}
		}
	}
}

func TestPurchaseLineDetailReadModelCarriesCanonicalAndCompatibilityFields(t *testing.T) {
	for name, condition := range map[string]string{
		"purchase": "se.aggregate = 'receiving'",
		"return":   "se.aggregate = 'return'",
	} {
		query := purchaseLineDetailReadModelQuery(condition, "LIMIT $6 OFFSET $7")
		for _, fragment := range []string{
			"FROM business_documents d",
			"JOIN business_document_lines l",
			"legacy_payload",
			"purchase_price",
			"discount_percent",
			"sales_tax_value",
			"expiry_date",
			"batch_number",
			"jsonb_array_elements",
			"NOT EXISTS (",
			"d.id = se.aggregate_id OR d.document_number = se.payload->>'documentNumber'",
			"LIMIT $6 OFFSET $7",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s purchase line-detail query is missing %q", name, fragment)
			}
		}
	}
	for kind := range map[string]struct{}{"purchase-detail": {}, "purchase-return-detail": {}} {
		spec, ok := reportSpecForKey(kind)
		if !ok || !spec.purchaseReadModel || spec.purchaseMode != "line-detail" {
			t.Fatalf("%s spec = %+v (ok=%v), want line-detail purchase read model", kind, spec, ok)
		}
		definition := reportDefinitionFor(kind)
		if len(definition.Columns) != 12 || definition.Columns[5].Key != "purchasePrice" {
			t.Fatalf("%s definition columns = %+v, want 12 source-backed purchase fields", kind, definition.Columns)
		}
		if !strings.Contains(definition.ProjectionNote, "tax") || !strings.Contains(definition.ProjectionNote, "profit") {
			t.Fatalf("%s definition note does not disclose open calculations: %q", kind, definition.ProjectionNote)
		}
	}
}

func TestHistoricalStockReadModelCarriesCapturedStockReportFields(t *testing.T) {
	query := historicalStockReadModelQuery("LIMIT $8 OFFSET $9")
	for _, fragment := range []string{
		"FROM historical_stock_snapshots h",
		"h.as_of >= $3::date",
		"h.godown_id = $6::uuid",
		"h.purchase_price::text",
		"h.sale_price::text",
		"h.average_price::text",
		"h.recent_purchase_price::text",
		"h.pack_units::text",
		"LIMIT $8 OFFSET $9",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("historical stock read model is missing %q", fragment)
		}
	}
	spec, ok := reportSpecForKey("stock-in-hand-back-date")
	if !ok || spec.stockMode != "historical-balance" {
		t.Fatalf("back-date stock spec = %+v (ok=%v), want historical-balance", spec, ok)
	}
	definition := reportDefinitionFor("stock-in-hand-back-date")
	if definition.ProjectionStatus != "real" || !strings.Contains(definition.ProjectionNote, "historical_stock_snapshots") {
		t.Fatalf("back-date definition = %+v, want source-backed historical projection", definition)
	}
	if len(definition.Columns) != 10 {
		t.Fatalf("back-date columns = %d, want captured StockReport fields: %+v", len(definition.Columns), definition.Columns)
	}
}

func TestStockLevelReadModelsUseItemThresholdPayloadAndPostedScope(t *testing.T) {
	for _, mode := range []string{"reorder-level", "optimum-level", "minimum-level", "reorder-optimum-level"} {
		query := stockLevelReadModelQuery(mode, "LIMIT $8 OFFSET $9")
		for _, fragment := range []string{
			"FROM stock_balances sb",
			"FROM stock_ledger l",
			"JOIN sync_events se",
			"i.payload->>'ReorderQty'",
			"i.payload->>'OptimumQty'",
			"i.payload->>'MinimumQty'",
			"i.payload->>'ReorderQuantity'",
			"i.payload->>'MinimumQuantity'",
			"sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid",
			"sb.updated_at >= $3::date",
			"$6::uuid",
			"$7",
			"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
			"LIMIT $8 OFFSET $9",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s stock-level query is missing %q", mode, fragment)
			}
		}
		if mode == "reorder-level" && !strings.Contains(query, "on_hand <= reorder_quantity") {
			t.Errorf("%s stock-level query lost reorder predicate", mode)
		}
		if mode == "optimum-level" && !strings.Contains(query, "on_hand <= optimum_quantity") {
			t.Errorf("%s stock-level query lost optimum predicate", mode)
		}
		if mode == "minimum-level" && !strings.Contains(query, "on_hand <= minimum_quantity") {
			t.Errorf("%s stock-level query lost minimum predicate", mode)
		}
		if mode == "reorder-optimum-level" && !strings.Contains(query, "GREATEST(reorder_quantity, optimum_quantity)") {
			t.Errorf("%s stock-level query lost combined predicate", mode)
		}
	}
}

func TestStockItemSummaryReadModelAggregatesPostedLedgerByItemDay(t *testing.T) {
	query := stockReadModelQuery("item-summary", "LIMIT $8 OFFSET $9")
	for _, fragment := range []string{
		"FROM stock_ledger l",
		"JOIN sync_events se",
		"l.tenant_id = $1::uuid AND l.branch_id = $2::uuid",
		"l.occurred_at >= $3::date",
		"b.godown_id = $6::uuid",
		"b.batch_number ILIKE '%' || $7 || '%'",
		"WHEN l.direction = 'adjustment' THEN l.quantity * l.adjustment_sign",
		"SUM(signed_quantity)::text",
		"SUM(signed_quantity * unit_cost)::text",
		"GROUP BY item_legacy_id, movement_date, godown, item_name",
		"LIMIT $8 OFFSET $9",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("stock item summary query is missing %q", fragment)
		}
	}
	if strings.Contains(query, "FROM stock_balances sb") {
		t.Fatal("stock item summary unexpectedly reads the balance cache instead of stock_ledger")
	}
	definition := reportDefinitionFor("item-stock-register-summary")
	if len(definition.Columns) != 6 || definition.Columns[4].Label != "Net Quantity" || definition.Columns[5].Label != "Net Value" {
		t.Fatalf("item stock summary definition = %+v, want six aggregate columns", definition.Columns)
	}
}

func TestStockMovementSummaryReadModelsAggregatePostedInOutByDay(t *testing.T) {
	query := stockReadModelQuery("movement-summary", "LIMIT $8 OFFSET $9")
	for _, fragment := range []string{
		"FROM stock_ledger l",
		"JOIN sync_events se",
		"l.tenant_id = $1::uuid AND l.branch_id = $2::uuid",
		"l.occurred_at::date AS movement_date",
		"WHEN l.direction = 'out' THEN 'OUT'",
		"WHEN l.direction = 'adjustment' THEN 'ADJUSTMENT'",
		"WHEN l.direction = 'out' THEN -l.quantity",
		"l.occurred_at >= $3::date",
		"b.godown_id = $6::uuid",
		"b.batch_number ILIKE '%' || $7 || '%'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"SUM(signed_quantity)::text",
		"SUM(signed_quantity * unit_cost)::text",
		"GROUP BY movement_date, movement_direction, godown, item_name",
		"LIMIT $8 OFFSET $9",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("stock movement summary query is missing %q", fragment)
		}
	}
	if strings.Contains(query, "inventory_movements") {
		t.Fatal("stock movement summary query reintroduced the compatibility inventory fallback")
	}
	for _, kind := range []string{"daily-stock-in-out", "stock-in-out-date-wise"} {
		spec, ok := reportSpecForKey(kind)
		if !ok || spec.stockMode != "movement-summary" {
			t.Fatalf("%s spec = %+v (ok=%v), want movement-summary mode", kind, spec, ok)
		}
		definition := reportDefinitionFor(kind)
		if len(definition.Columns) != 6 || definition.Columns[0].Label != "Date" || definition.Columns[5].Label != "Net Value" {
			t.Fatalf("%s definition = %+v, want six day-wise aggregate columns", kind, definition.Columns)
		}
		if !strings.Contains(definition.Retrieval.Scope, "day/direction/godown/item aggregation") {
			t.Fatalf("%s retrieval scope = %q, want movement-summary scope", kind, definition.Retrieval.Scope)
		}
	}
}

func TestStockSupplierManufacturerReadModelUsesItemSuppliersAndPostedBalances(t *testing.T) {
	query := stockReadModelQuery("supplier-manufacturer", "LIMIT $8 OFFSET $9")
	for _, fragment := range []string{
		"FROM stock_balances sb",
		"FROM item_suppliers s",
		"LEFT JOIN master_parties mp",
		"i.payload->>'Manufacturer'",
		"string_agg(",
		"s.legacy_item_id = sb.item_legacy_id",
		"sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid",
		"sb.updated_at >= $3::date",
		"b.godown_id = $6::uuid",
		"b.batch_number ILIKE '%' || $7 || '%'",
		"FROM stock_ledger l",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"ORDER BY manufacturer",
		"LIMIT $8 OFFSET $9",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("supplier/manufacturer query is missing %q", fragment)
		}
	}
	if strings.Contains(query, "inventory_movements") {
		t.Fatal("supplier/manufacturer query reintroduced the compatibility inventory fallback")
	}
	spec, ok := reportSpecForKey("stock-in-hand-supplier-manufacturer-association")
	if !ok || spec.stockMode != "supplier-manufacturer" {
		t.Fatalf("supplier/manufacturer spec = %+v (ok=%v), want supplier-manufacturer mode", spec, ok)
	}
	definition := reportDefinitionFor("stock-in-hand-supplier-manufacturer-association")
	if len(definition.Columns) != 6 || definition.Columns[0].Label != "Manufacturer" || definition.Columns[1].Label != "Supplier(s)" {
		t.Fatalf("supplier/manufacturer definition = %+v, want association columns", definition)
	}
	if !strings.Contains(definition.Retrieval.Scope, "item_suppliers links") {
		t.Fatalf("supplier/manufacturer retrieval scope = %q, want item-supplier scope", definition.Retrieval.Scope)
	}
}

func TestStockSalesReadModelJoinsCanonicalPostedSaleAllocations(t *testing.T) {
	query := stockReadModelQuery("stock-sales", "LIMIT $8 OFFSET $9")
	for _, fragment := range []string{
		"WITH posted_sales AS (",
		"FROM stock_allocations sa",
		"JOIN business_documents bd",
		"JOIN business_document_lines bl",
		"bd.status = 'posted'",
		"bd.kind IN ('cash-sale', 'credit-sale')",
		"bd.occurred_at >= $3::date",
		"SUM(sa.quantity) AS sales_quantity",
		"FROM stock_balances sb",
		"COALESCE(ps.sales_quantity, 0)::text",
		"FROM stock_ledger l",
		"$6::uuid",
		"$7",
		"LIMIT $8 OFFSET $9",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("stock-and-sales query is missing %q", fragment)
		}
	}
	if strings.Contains(query, "sb.updated_at >= $3::date") || strings.Contains(query, "sb.updated_at < ($4::date") {
		t.Fatal("stock-and-sales query date-filtered the current stock balance instead of only the sales allocation period")
	}
	definition := reportDefinitionFor("stock-and-sales")
	if len(definition.Columns) != 6 || definition.Columns[5].Label != "Sales Qty" || definition.Columns[5].DataType != "number" {
		t.Fatalf("stock-and-sales definition = %+v, want On Hand and Sales Qty columns", definition.Columns)
	}
}

func TestStockNarcoticsReadModelsUseCapturedItemFlagsAndPostedScope(t *testing.T) {
	for _, mode := range []string{"narcotics-movement", "narcotics-generic"} {
		query := stockReadModelQuery(mode, "LIMIT $8 OFFSET $9")
		for _, fragment := range []string{
			"FROM stock_ledger l",
			"JOIN sync_events se",
			"l.tenant_id = $1::uuid AND l.branch_id = $2::uuid",
			"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
			"i.payload->>'Narcotics'",
			"i.payload->>'Narcotic'",
			"i.payload->>'GenericName'",
			"b.godown_id = $6::uuid",
			"b.batch_number ILIKE '%' || $7 || '%'",
			"LIMIT $8 OFFSET $9",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s narcotics query is missing %q", mode, fragment)
			}
		}
		if strings.Contains(query, "inventory_movements") {
			t.Fatalf("%s narcotics query reintroduced the compatibility inventory fallback", mode)
		}
	}

	genericDefinition := reportDefinitionFor("norcotics-stock-register-generic-type-wise")
	if len(genericDefinition.Columns) != 6 || genericDefinition.Columns[0].Label != "Generic Type" || genericDefinition.Columns[4].Label != "Net Quantity" {
		t.Fatalf("generic narcotics definition = %+v, want generic type aggregate columns", genericDefinition)
	}
	if !strings.Contains(genericDefinition.ProjectionNote, "Narcotics") || !strings.Contains(genericDefinition.ProjectionNote, "GenericName") {
		t.Fatalf("generic narcotics definition does not disclose captured item payload scope: %q", genericDefinition.ProjectionNote)
	}
	for kind, wantMode := range map[string]string{
		"stock-register-for-narcotics":               "narcotics-movement",
		"stock-register-narcotics-format2":           "narcotics-movement",
		"norcotics-stock-register-generic-type-wise": "narcotics-generic",
	} {
		spec, ok := reportSpecForKey(kind)
		if !ok || spec.stockMode != wantMode {
			t.Fatalf("%s spec = %+v (ok=%v), want stock mode %q", kind, spec, ok, wantMode)
		}
	}
}

func TestStockExpiryClassReadModelUsesTypedExpiryAndItemClass(t *testing.T) {
	query := stockReadModelQuery("expiry-class", "LIMIT $8 OFFSET $9")
	for _, fragment := range []string{
		"FROM stock_balances sb",
		"FROM stock_ledger l",
		"JOIN sync_events se",
		"b.expiry_date IS NOT NULL",
		"b.expiry_date BETWEEN $3::date AND $4::date",
		"i.payload->>'Class'",
		"i.payload->>'ItemClass'",
		"sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid",
		"b.godown_id = $6::uuid",
		"b.batch_number ILIKE '%' || $7 || '%'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"ORDER BY item_class",
		"LIMIT $8 OFFSET $9",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("expiry-class query is missing %q", fragment)
		}
	}
	if strings.Contains(query, "inventory_movements") {
		t.Fatal("expiry-class query reintroduced the compatibility inventory fallback")
	}
	spec, ok := reportSpecForKey("expiry-report-class-wise")
	if !ok || spec.stockMode != "expiry-class" {
		t.Fatalf("expiry class spec = %+v (ok=%v), want expiry-class mode", spec, ok)
	}
	definition := reportDefinitionFor("expiry-report-class-wise")
	if len(definition.Columns) != 6 || definition.Columns[0].Label != "Class" || definition.Columns[1].Label != "Expiry Date" {
		t.Fatalf("expiry class definition = %+v, want six typed class/expiry columns", definition.Columns)
	}
}

func TestStockClassificationReadModelsUseCapturedItemGroups(t *testing.T) {
	for mode, payloadKey := range map[string]string{
		"manufacturer-balance": "i.payload->>'Manufacturer'",
		"category-balance":     "i.payload->>'Category'",
		"class-balance":        "i.payload->>'Class'",
	} {
		query := stockReadModelQuery(mode, "LIMIT $8 OFFSET $9")
		for _, fragment := range []string{
			"FROM stock_balances sb",
			"FROM stock_ledger l",
			"JOIN sync_events se",
			payloadKey,
			"sb.updated_at >= $3::date",
			"sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid",
			"b.godown_id = $6::uuid",
			"b.batch_number ILIKE '%' || $7 || '%'",
			"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
			"ORDER BY classification",
			"LIMIT $8 OFFSET $9",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s classification query is missing %q", mode, fragment)
			}
		}
		if strings.Contains(query, "inventory_movements") {
			t.Fatalf("%s classification query reintroduced the compatibility inventory fallback", mode)
		}
	}
	for kind, wantLabel := range map[string]string{
		"stock-in-hand-manufacturer-wise":         "Manufacturer",
		"stock-in-hand-manufacturer-wise-format2": "Manufacturer",
		"stock-in-hand-category-wise":             "Category",
		"stock-in-hand-class-wise":                "Class",
	} {
		definition := reportDefinitionFor(kind)
		if len(definition.Columns) != 6 || definition.Columns[0].Label != wantLabel || definition.Columns[4].Label != "On Hand" {
			t.Fatalf("%s definition = %+v, want six %s classification columns", kind, definition.Columns, wantLabel)
		}
	}
}

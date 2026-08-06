package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type reportRow struct {
	Document   string `json:"document"`
	OccurredAt string `json:"occurredAt"`
	Party      string `json:"party"`
	Item       string `json:"item"`
	Quantity   string `json:"quantity"`
	Amount     string `json:"amount"`
}

type reportColumn struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	DataType string `json:"dataType"`
	Sortable bool   `json:"sortable"`
}

type reportFormat struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

type reportRetrieval struct {
	Title              string   `json:"title"`
	Areas              []string `json:"areas"`
	SupportsCashCredit bool     `json:"supportsCashCredit"`
	SupportsDateRange  bool     `json:"supportsDateRange"`
	SupportsTextFilter bool     `json:"supportsTextFilter"`
	Scope              string   `json:"scope"`
}

type reportLetterhead struct {
	Name   string `json:"name"`
	Line2  string `json:"line2"`
	Line3  string `json:"line3"`
	Phone  string `json:"phone"`
	Fax    string `json:"fax"`
	Source string `json:"source"`
}

type reportExportHook struct {
	Format  string `json:"format"`
	Status  string `json:"status"`
	Label   string `json:"label"`
	Message string `json:"message"`
}

type reportDefinition struct {
	Kind             string             `json:"kind"`
	Title            string             `json:"title"`
	ProjectionStatus string             `json:"projectionStatus"`
	ProjectionNote   string             `json:"projectionNote,omitempty"`
	Columns          []reportColumn     `json:"columns"`
	Formats          []reportFormat     `json:"formats"`
	Retrieval        reportRetrieval    `json:"retrieval"`
	Letterhead       reportLetterhead   `json:"letterhead"`
	Exports          []reportExportHook `json:"exports"`
}

const (
	defaultReportLetterheadName  = "Fazal Din's Pharma Plus"
	defaultReportLetterheadLine2 = "NRY Pacific"
	defaultReportLetterheadLine3 = "Franchise Fazal Din's"
	defaultReportLetterheadPhone = "055 3252501"
)

type reportSpec struct {
	title              string
	aggregateCondition string
	salesReadModel     bool
	salesMode          string
	purchaseReadModel  bool
	purchaseMode       string
	stockReadModel     bool
	stockMode          string
	financeMode        string
	adminKind          string
	compatibilityOnly  bool
}

const (
	reportSaleAggregate       = "se.aggregate = 'sale'"
	reportSaleReturnAggregate = "se.aggregate = 'sale_return'"
	reportRefusedAggregate    = "se.aggregate = 'refused_sale'"
	reportSaleOrReturn        = "se.aggregate IN ('sale', 'sale_return')"
)

// phaseNReportRegistry is deliberately limited to report leaves captured under
// Daily Reports/Sale, Daily Reports/Sales Return, and Sales Reports. The
// event-ledger status means the leaf is scoped to the right immutable
// aggregate, not that its legacy aggregation or numeric calculations are
// complete.
var phaseNReportRegistry = func() map[string]reportSpec {
	registry := make(map[string]reportSpec)
	add := func(kind, title, aggregateCondition string) {
		registry[kind] = reportSpec{title: title, aggregateCondition: aggregateCondition}
	}

	for _, report := range []struct {
		kind  string
		title string
	}{
		{"sale-detail", "Sale detail"},
		{"sale-summary", "Sale summary"},
		{"sale-summary-inv-wise", "Sale Summary Inv. Wise"},
		{"sale-summary-inv-cust-wise", "Sale Summary Inv. Cust Wise"},
		{"sale-detail-inv-wise", "Sale Detail Inv. Wise"},
		{"sale-detail-format-2", "Sale Detail (Format 2)"},
		{"refused-sales-detail", "Refused Sales Detail"},
		{"sale-detail-inv-wise-with-diff-col", "Sale Detail Inv. Wise(with diff.col.)"},
		{"sale-summary-invoice-wise", "Sale Summary - Invoice Wise"},
		{"sale-summary-machine-and-invoice-range-wise", "Sale Summary Machine and Invoice Range Wise"},
		{"selected-sales-and-summaries-report", "Selected Sales and Summaries Report"},
	} {
		condition := reportSaleAggregate
		if report.kind == "refused-sales-detail" {
			condition = reportRefusedAggregate
		}
		add(report.kind, report.title, condition)
		if condition == reportSaleAggregate {
			mode := ""
			switch report.kind {
			case "sale-summary", "sale-summary-inv-wise", "sale-summary-invoice-wise":
				mode = "invoice-summary"
			}
			registry[report.kind] = reportSpec{title: report.title, aggregateCondition: condition, salesReadModel: true, salesMode: mode}
		}
	}
	for _, report := range []struct {
		kind  string
		title string
	}{
		{"sales-return-detail", "Sales Return detail"},
		{"sales-return-summary", "Sales Return summary"},
		{"sales-return-summary-inv-wise", "Sales Return Summary Inv.wise"},
		{"sales-return-detail-inv-wise", "Sales Return Detail Inv.wise"},
	} {
		mode := ""
		if report.kind == "sales-return-summary" || report.kind == "sales-return-summary-inv-wise" {
			mode = "invoice-summary"
		}
		registry[report.kind] = reportSpec{
			title:              report.title,
			aggregateCondition: reportSaleReturnAggregate,
			salesReadModel:     true,
			salesMode:          mode,
		}
	}

	// Sales Reports. These are intentionally event-level views until captured
	// legacy output proves the corresponding grouping, tax, profit, or graph
	// calculation. They still use a precise sale/sale-return filter.
	for _, report := range []struct {
		kind  string
		title string
	}{
		{"customer-sales-detail", "Detail"},
		{"customer-sales-summary", "Summary"},
		{"customer-sales-days-summary", "Days Summary"},
		{"customer-sales-invoice-wise-profit-margin-detail", "Invoice Wise Profit Margin Detail"},
		{"customer-sales-items-summary", "Items Summary"},
		{"customer-sales-invoice-summary", "Invoice Summary"},
		{"customer-sales-hourly-graph", "Hourly Graph"},
		{"customer-sales-lp-ledger", "LP Ledger"},
		{"customer-sales-customer-category-wise-net-sales", "Customer Category Wise Net Sales"},
		{"customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report", "Customer Category Wise Sales Summary Report"},
		{"customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report", "Customer Category Wise Sales Detail Report"},
		{"customer-sales-customer-category-wise-sales-net-sales-and-volume", "Net Sales and Volume"},
		{"customer-sales-customer-category-wise-sales-customer-wise-summary", "Customer Wise Summary"},
		{"customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report", "Customer Category Wise Net Sales Report"},
		{"customer-sales-customer-category-wise-sales-output-sales-tax-report", "Output Sales Tax Report"},
		{"customer-sales-customer-category-wise-sales-customer-wise-gross-profit", "Customer Wise Gross Profit"},
		{"customer-sales-customer-wise-category-net-sales", "Customer Wise Category Net Sales"},
		{"customer-sales-monthly-net-sales", "Monthly Net Sales"},
		{"customer-sales-claimable-for-allowed-customers", "Claimable for Allowed Customers"},
		{"customer-sales-customer-ntn-wise-sales-tax-report", "Customer NTN Wise Sales Tax Report"},
		{"customer-sales-customer-wise-advance-tax", "Customer Wise Advance Tax"},
		{"category-wise-sale-and-return", "Sale And Return"},
		{"category-wise-sales", "Sales"},
		{"category-wise-deviated-items", "Deviated Items"},
		{"category-wise-monthly-sale", "Monthly Sale"},
		{"category-wise-net-sale", "Net Sale"},
		{"category-wise-gross-profit", "Gross Profit"},
		{"category-wise-item-wise-sale-discounts-detail", "Item Wise Sale Discounts Detail"},
		{"category-wise-item-category-wise-monthly-sales", "Item Category Wise Monthly Sales"},
		{"category-wise-category-wise-day-net-sale", "Category Wise Day Net Sale"},
		{"manufacturer-wise-sales", "Sales"},
		{"manufacturer-wise-sales-detail-and-summary", "Sales Detail And Summary"},
		{"manufacturer-wise-net-sales", "Net Sales"},
		{"manufacturer-wise-item-sales-discount", "Item Sales/Discount"},
		{"manufacturer-wise-manufacturer-wise-sales-and-return-summary", "Manufacturer Wise Sales And Return Summary"},
		{"manufacturer-wise-cnic-ntn-registered-customers-sales", "CNIC/NTN Registered Customers Sales"},
		{"user-wise-invoice-graph", "Invoice Graph"},
		{"user-wise-sales", "Sales"},
		{"user-wise-category-summary", "Category Summary"},
		{"user-wise-discount-report", "Discount Report"},
		{"user-wise-net-cash", "Net Cash"},
		{"user-wise-sales-commission", "Sales Commission"},
		{"user-wise-user-wise-sales-summary", "User Wise Sales Summary"},
		{"hourly-sales-graph", "Hourly Sales Graph"},
		{"slow-fast-moving-items", "Slow/Fast moving Items"},
		{"net-sale-summary", "Net Sale Summary"},
		{"item-wise-item-sale-and-return-activity", "Item Sale and Return Activity"},
		{"item-wise-item-wise-net-sales", "Item Wise Net Sales"},
		{"sale-return-summary-inv-type-wise", "Sale/Return Summary Inv. Type Wise"},
		{"daily-sales-summary-with-profit-day-wise-grouping", "Daily Sales Summary with Profit(Day wise grouping)"},
		{"monthly-net-sales-summary", "Monthly Net Sales Summary"},
		{"dead-item-list", "Dead Item List"},
		{"sales-tax-report", "Sales Tax Report"},
	} {
		condition := reportSaleAggregate
		if strings.Contains(strings.ToLower(report.title), "return") {
			condition = reportSaleOrReturn
		}
		add(report.kind, report.title, condition)
		registry[report.kind] = reportSpec{title: report.title, aggregateCondition: condition, salesReadModel: true}
	}
	return registry
}()

// phaseOReportRegistry contains the captured purchase, purchase-return,
// purchase-order, and supplier report leaves. These definitions use the
// canonical purchase read model where it has the required source data. They
// remain event-ledger projections because legacy grouping, tax, profit, and
// graph calculations have not been reconciled.
var phaseOReportRegistry = func() map[string]reportSpec {
	registry := make(map[string]reportSpec)
	add := func(kind, title, aggregateCondition, mode string) {
		registry[kind] = reportSpec{
			title:              title,
			aggregateCondition: aggregateCondition,
			purchaseReadModel:  true,
			purchaseMode:       mode,
		}
	}
	for _, report := range []struct {
		kind      string
		title     string
		aggregate string
		mode      string
	}{
		{"purchase-detail", "Purchase detail", "se.aggregate = 'receiving'", "detail"},
		{"purchase-summary", "Purchase summary", "se.aggregate = 'receiving'", "summary"},
		{"purchase-summary2", "Purchase Summary2", "se.aggregate = 'receiving'", "summary"},
		{"purchase-return-detail", "Purchase Return detail", "se.aggregate = 'return'", "detail"},
		{"purchase-return-summary", "Purchase Return summary", "se.aggregate = 'return'", "summary"},
		{"purchase-order-summary", "Purchase Order Summary", "se.aggregate = 'purchase_order'", "summary"},
		{"p-o-based-purchase-disparity", "P/O Based Purchase Disparity", "se.aggregate = 'purchase_order'", "detail"},
		{"periodic-purchases", "Periodic Purchases", "se.aggregate = 'receiving'", "summary"},
		{"purchase-order", "Purchase Order", "se.aggregate = 'purchase_order'", "summary"},
		{"supplier-wise-detail", "Detail", "se.aggregate = 'receiving'", "detail"},
		{"supplier-wise-purchase-detail", "Purchase Detail", "se.aggregate = 'receiving'", "detail"},
		{"supplier-wise-advance-income-tax", "Advance Income Tax", "se.aggregate = 'receiving'", "summary"},
		{"manufacturer-wise-detail", "Detail", "se.aggregate = 'receiving'", "detail"},
		{"manufacturer-wise-monthly-stock-movement", "Monthly Stock Movement", "se.aggregate = 'receiving'", "summary"},
		{"monthly-purchase-graph", "Monthly Purchase Graph", "se.aggregate = 'receiving'", "summary"},
		{"category-wise-purchase", "Category Wise Purchase", "se.aggregate = 'receiving'", "summary"},
		{"days-summary", "Days Summary", "se.aggregate = 'receiving'", "summary"},
		{"purchase-order-supplier-wise", "Purchase Order Supplier Wise", "se.aggregate = 'purchase_order'", "summary"},
		{"net-purchase-summary", "Net Purchase Summary", "se.aggregate = 'receiving'", "summary"},
		{"supplier-category-wise-input-sales-tax-report", "Input Sales Tax Report", "se.aggregate = 'receiving'", "summary"},
		{"withholding-tax-deduction", "Withholding Tax Deduction", "se.aggregate = 'receiving'", "summary"},
		{"supplier-manufacturer-wise-g-p", "Supplier/Manufacturer Wise G/P", "se.aggregate = 'receiving'", "summary"},
		{"supplier-purchase-returns-detail", "Detail", "se.aggregate = 'return'", "detail"},
		{"supplier-purchase-returns-summary", "Summary", "se.aggregate = 'return'", "summary"},
	} {
		add(report.kind, report.title, report.aggregate, report.mode)
	}
	return registry
}()

// phasePReportRegistry contains the captured Stock Reports leaves. These
// reports read normalized stock_ledger/stock_balances rows only; compatibility
// inventory movements are not silently mixed into this wave.
var phasePReportRegistry = func() map[string]reportSpec {
	registry := make(map[string]reportSpec)
	add := func(kind, title, mode string) {
		registry[kind] = reportSpec{
			title:              title,
			aggregateCondition: "se.aggregate = 'inventory'",
			stockReadModel:     true,
			stockMode:          mode,
		}
	}
	for _, report := range []struct {
		kind  string
		title string
		mode  string
	}{
		{"stock-in-hand-manufacturer-wise", "Manufacturer wise", "balance"},
		{"stock-in-hand-category-wise", "Category wise", "balance"},
		{"stock-in-hand-others", "Others", "balance"},
		{"stock-in-hand-class-wise", "Class Wise", "balance"},
		{"stock-in-hand-batch-priority-wise", "Batch, Priority Wise", "balance"},
		{"stock-in-hand-back-date", "Back Date", "balance"},
		{"stock-in-hand-manufacturer-wise-format2", "Manufacturer Wise (Format2)", "balance"},
		{"stock-in-hand-supplier-manufacturer-association", "Supplier Manufacturer Association", "balance"},
		{"stock-in-hand-stock-quantity-format", "Stock Quantity Format", "balance"},
		{"stock-in-hand-stock-in-hand-audit-purpose", "Stock in Hand - Audit Purpose", "balance"},
		{"stock-in-hand-batch-priority-wise-audit-purposes", "Batch, Priority Wise - Audit Purposes", "balance"},
		{"expiry-report", "Expiry Report", "expiry"},
		{"reorder-level-report", "Reorder Level Report", "balance"},
		{"stock-register", "Stock Register", "movement"},
		{"item-stock-register-summary", "Item Stock Register Summary", "summary"},
		{"stock-register-for-narcotics", "Stock Register(For Narcotics)", "movement"},
		{"stock-and-sales", "Stock and Sales", "balance"},
		{"optimum-level-report", "Optimum Level Report", "balance"},
		{"item-activity", "Item Activity", "movement"},
		{"stock-register-narcotics-format2", "Stock Register(Narcotics Format2)", "movement"},
		{"expiry-report-class-wise", "Expiry Report(Class Wise)", "expiry"},
		{"minimum-level-report", "Minimum Level Report", "balance"},
		{"reorder-optimum-level-report", "Reorder/Optimum Level Report", "balance"},
		{"daily-stock-in-out", "Daily Stock IN/OUT", "movement"},
		{"stock-in-out-date-wise", "Stock IN/OUT(Date Wise)", "movement"},
		{"stock-management-report", "Stock Management Report", "summary"},
		{"norcotics-stock-register-generic-type-wise", "Norcotics Stock Register-Generic Type Wise", "movement"},
	} {
		add(report.kind, report.title, report.mode)
	}
	return registry
}()

// phaseQReportRegistry closes the remaining mapped report leaves without
// pretending that an absent historical projection exists. Financial leaves
// read immutable posted journals, party entries, voucher rows, or document tax
// snapshots. Remaining administrative/reprint leaves use an explicitly named
// normalized or compatibility projection.
var phaseQReportRegistry = func() map[string]reportSpec {
	registry := make(map[string]reportSpec)
	add := func(kind, title string, spec reportSpec) {
		spec.title = title
		registry[kind] = spec
	}
	adjustments := []struct {
		kind  string
		title string
	}{
		{"adjustment-adjustment-summary", "Adjustment Summary"},
		{"adjustment-adjustment-detail", "Adjustment Detail"},
		{"adjustment-adjustment-summary-inv-wise", "Adjustment Summary Inv. Wise"},
		{"adjustment-adjustment-detail-inv-wise", "Adjustment Detail Inv. wise"},
		{"adjustment-adjustment-summary-detail", "Adjustment Summary/Detail"},
		{"adjustment-item-wise-adjustment-summary", "Item Wise Adjustment Summary"},
	}
	for _, report := range adjustments {
		add(report.kind, report.title, reportSpec{
			stockReadModel: true,
			stockMode:      "adjustment",
		})
	}
	for _, report := range []struct {
		kind  string
		title string
	}{
		{"quotation-detail", "Detail"},
		{"quotation-summary", "Summary"},
		{"header-wise-transaction-summary", "Header Wise Transaction Summary"},
	} {
		condition := "se.aggregate IN ('sale', 'sale_return', 'receiving', 'return', 'quotation', 'purchase_order', 'inventory')"
		if strings.HasPrefix(report.kind, "quotation-") {
			condition = "se.aggregate = 'quotation'"
		}
		add(report.kind, report.title, reportSpec{aggregateCondition: condition, compatibilityOnly: true})
	}
	add("accounts-reports-ledger-reports-accounts-ledger", "Accounts Ledger", reportSpec{
		financeMode: "gl",
	})
	add("trial-balance", "Trial Balance", reportSpec{
		financeMode: "trial-balance",
	})
	add("voucher-register", "Voucher Register", reportSpec{
		financeMode: "voucher",
	})
	for _, report := range []struct {
		kind      string
		title     string
		adminKind string
	}{
		{"listing-supplier-list", "Supplier List", "supplier"},
		{"listing-items-list", "Items List", "item"},
		{"listing-manufacturer-list", "Manufacturer List", "manufacturer"},
		{"listing-group-rights-list", "Group Rights List", "roles"},
		{"listing-item-list-class-wise", "Item List Class Wise", "item_class"},
		{"listing-groupwise-user-list", "GroupWise User List", "users"},
		{"listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict", "Manufacturer/Sub Area Wise Sales Person Conflict", "users"},
	} {
		add(report.kind, report.title, reportSpec{adminKind: report.adminKind})
	}
	for _, report := range []struct {
		kind  string
		title string
		mode  string
	}{
		{"reprinting-sale", "Sale", "sales"},
		{"reprinting-purchase", "Purchase", "purchases"},
		{"reprinting-sale-with-summary-reports", "Sale (with summary reports)", "sales"},
		{"reprinting-sale-format-2", "Sale Format(2)", "sales"},
		{"reprinting-sale-format-3", "Sale Format(3)", "sales"},
		{"reprinting-sale-format-4", "Sale Format(4)", "sales"},
		{"reprinting-sale-with-header-wise-summaries", "Sale (with header wise summaries)", "sales"},
		{"reprinting-selected-sales-and-summaries", "Selected Sales and Summaries", "sales"},
	} {
		spec := reportSpec{compatibilityOnly: true, aggregateCondition: reportSaleAggregate}
		if report.mode == "sales" {
			spec.salesReadModel = true
		} else {
			spec.purchaseReadModel = true
			spec.purchaseMode = "summary"
			spec.aggregateCondition = "se.aggregate = 'receiving'"
		}
		add(report.kind, report.title, spec)
	}
	add("item-reports-deleted-sale-items-log", "Deleted Sale Items Log", reportSpec{
		aggregateCondition: "se.aggregate IN ('sale', 'sale_return')",
		compatibilityOnly:  true,
	})
	return registry
}()

// These captured leaves already live in the N/O registries but have a
// normalized financial source available in the Q wave. Keeping the override
// separate avoids changing the N/O coverage accounting.
var phaseQFinancialOverrides = map[string]string{
	"customer-sales-lp-ledger": "party-customer",
	"customer-sales-customer-category-wise-sales-output-sales-tax-report": "tax-output",
	"customer-sales-customer-ntn-wise-sales-tax-report":                   "tax-output",
	"customer-sales-customer-wise-advance-tax":                            "tax-advance",
	"sales-tax-report":                              "tax-output",
	"supplier-wise-advance-income-tax":              "tax-advance",
	"supplier-category-wise-input-sales-tax-report": "tax-input",
	"withholding-tax-deduction":                     "tax-withholding",
	"user-wise-net-cash":                            "gl-cash",
}

// The report route also accepts stable direct links for the financial views.
// They are aliases, not additional legacy leaves, and make the normalized
// projections available to the report client without inventing menu entries.
var phaseQReportAliases = map[string]reportSpec{
	"gl-journal":         {title: "GL Journal", financeMode: "gl"},
	"trial-balance":      {title: "Trial Balance", financeMode: "trial-balance"},
	"customer-statement": {title: "Customer Statement", financeMode: "party-customer"},
	"supplier-statement": {title: "Supplier Statement", financeMode: "party-supplier"},
	"receivables-aging":  {title: "Receivables Aging", financeMode: "aging-receivable"},
	"payables-aging":     {title: "Payables Aging", financeMode: "aging-payable"},
	"tax-register":       {title: "Tax Register", financeMode: "tax-output"},
	"voucher-register":   {title: "Voucher Register", financeMode: "voucher"},
}

func reportSpecForKey(kind string) (reportSpec, bool) {
	if spec, ok := phaseQReportRegistry[kind]; ok {
		return spec, true
	}
	if spec, ok := phaseQReportAliases[kind]; ok {
		return spec, true
	}
	if spec, ok := phaseOReportRegistry[kind]; ok {
		if mode, promoted := phaseQFinancialOverrides[kind]; promoted {
			spec.financeMode = mode
		}
		return spec, true
	}
	if spec, ok := phasePReportRegistry[kind]; ok {
		return spec, true
	}
	spec, ok := phaseNReportRegistry[kind]
	if ok {
		if mode, promoted := phaseQFinancialOverrides[kind]; promoted {
			spec.financeMode = mode
		}
	}
	return spec, ok
}

func reportRegistryHasKey(kind string) bool {
	_, ok := reportSpecForKey(kind)
	return ok
}

func reportColumns() []reportColumn {
	return []reportColumn{
		{Key: "document", Label: "Document", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
		{Key: "party", Label: "Customer/Supplier", DataType: "text", Sortable: true},
		{Key: "item", Label: "Item", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
		{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
	}
}

func reportEventLedgerColumns() []reportColumn {
	return []reportColumn{
		{Key: "document", Label: "Event / Document", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Occurred", DataType: "date", Sortable: true},
		{Key: "party", Label: "Party", DataType: "text", Sortable: true},
		{Key: "item", Label: "Item (first payload line)", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Quantity (payload)", DataType: "number", Sortable: true},
		{Key: "amount", Label: "Amount (payload)", DataType: "currency", Sortable: true},
	}
}

func stockReportColumns(mode string) []reportColumn {
	if mode == "adjustment" {
		return []reportColumn{
			{Key: "document", Label: "Adjustment", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Direction", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Unit Cost", DataType: "currency", Sortable: true},
		}
	}
	if mode == "movement" {
		return []reportColumn{
			{Key: "document", Label: "Movement", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Direction", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Unit Cost", DataType: "currency", Sortable: true},
		}
	}
	if mode == "valuation" {
		return []reportColumn{
			{Key: "document", Label: "Batch", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "As Of", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Normalized Valuation", DataType: "currency", Sortable: true},
		}
	}
	return []reportColumn{
		{Key: "document", Label: "Batch", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Expiry/Updated", DataType: "date", Sortable: true},
		{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
		{Key: "item", Label: "Item", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
		{Key: "amount", Label: "Unit Cost", DataType: "currency", Sortable: true},
	}
}

func financeReportColumns(mode string) []reportColumn {
	switch mode {
	case "gl", "gl-cash":
		return []reportColumn{
			{Key: "document", Label: "Journal", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Account", DataType: "text", Sortable: true},
			{Key: "item", Label: "Memo", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Debit", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Credit", DataType: "currency", Sortable: true},
		}
	case "trial-balance":
		return []reportColumn{
			{Key: "document", Label: "Account", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "As Of", DataType: "date", Sortable: true},
			{Key: "party", Label: "Category", DataType: "text", Sortable: true},
			{Key: "item", Label: "Account Name", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Debit", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Credit", DataType: "currency", Sortable: true},
		}
	case "party-customer", "party-supplier":
		return []reportColumn{
			{Key: "document", Label: "Document", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer/Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Description", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Debit", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Credit", DataType: "currency", Sortable: true},
		}
	case "aging-receivable", "aging-payable":
		return []reportColumn{
			{Key: "document", Label: "Party", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "As Of", DataType: "date", Sortable: true},
			{Key: "party", Label: "Party Type", DataType: "text", Sortable: true},
			{Key: "item", Label: "Aging Status", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Debit Total", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Credit Total", DataType: "currency", Sortable: true},
		}
	case "tax-output", "tax-input", "tax-advance", "tax-withholding":
		return []reportColumn{
			{Key: "document", Label: "Document", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer/Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item / Tax Snapshot", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Taxable Base", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Tax Amount", DataType: "currency", Sortable: true},
		}
	case "voucher":
		return []reportColumn{
			{Key: "document", Label: "Voucher", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Created", DataType: "date", Sortable: true},
			{Key: "party", Label: "Category", DataType: "text", Sortable: true},
			{Key: "item", Label: "Description", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Amount", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Status", DataType: "text", Sortable: true},
		}
	}
	return reportColumns()
}

func financeProjectionNote(mode string) string {
	switch mode {
	case "gl", "gl-cash":
		return "Posted-only gl_journals and gl_lines joined to the tenant chart of accounts; no draft, void, or legacy VirtualGl rows are inferred."
	case "trial-balance":
		return "Posted-only gl_journals/gl_lines aggregated by account and scoped to tenant, branch, and date; opening or imported historical balances are not included."
	case "party-customer":
		return "Posted-only customer party_ledger_entries with running balances where recorded; payment allocation and legacy customer statement columns not present in the source are omitted."
	case "party-supplier":
		return "Posted-only supplier party_ledger_entries with running balances where recorded; payment allocation and legacy supplier statement columns not present in the source are omitted."
	case "aging-receivable", "aging-payable":
		return "Unaged outstanding party-ledger totals only. No due_date, payment allocation, or invoice aging-bucket prerequisite exists in the normalized source, so bucket values are not fabricated."
	case "tax-output":
		return "Posted business-document line tax snapshots (tax_amount, rates, and taxable line totals) for sales; values are not recomputed from current configuration."
	case "tax-input":
		return "Posted business-document line tax snapshots (tax_amount, rates, and taxable line totals) for purchases; values are not recomputed from current configuration."
	case "tax-advance":
		return "Posted line tax snapshots with an advance-tax rate; rows are omitted when the posted snapshot has no advance-tax evidence."
	case "tax-withholding":
		return "No normalized withholding-tax snapshot or posting source exists. Withholding amount, rate, certificate, and party allocation prerequisites are absent; no values are returned."
	case "voucher":
		return "Posted voucher_entries scoped to tenant and branch. Voucher-to-GL line posting is not assumed when no linked journal exists."
	default:
		return ""
	}
}

func reportAdminColumns(kind string) []reportColumn {
	if kind == "users" {
		return []reportColumn{
			{Key: "document", Label: "User", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Created", DataType: "date", Sortable: true},
			{Key: "party", Label: "Username", DataType: "text", Sortable: true},
			{Key: "item", Label: "Display Name", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Active", DataType: "text", Sortable: true},
			{Key: "amount", Label: "Group", DataType: "text", Sortable: true},
		}
	}
	if kind == "roles" {
		return []reportColumn{
			{Key: "document", Label: "Group", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Permission", DataType: "text", Sortable: true},
			{Key: "party", Label: "Name", DataType: "text", Sortable: true},
			{Key: "item", Label: "Right", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Allowed", DataType: "text", Sortable: true},
			{Key: "amount", Label: "Source", DataType: "text", Sortable: true},
		}
	}
	return []reportColumn{
		{Key: "document", Label: "Code", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Updated", DataType: "date", Sortable: true},
		{Key: "party", Label: "Kind", DataType: "text", Sortable: true},
		{Key: "item", Label: "Name", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Active", DataType: "text", Sortable: true},
		{Key: "amount", Label: "Legacy ID", DataType: "text", Sortable: true},
	}
}

func stockProjectionNote(mode string) string {
	switch mode {
	case "movement":
		return "Normalized posted stock_ledger movement projection joined to batch, item, and godown metadata; compatibility inventory_movements and unreconciled legacy grouping are not included."
	case "expiry":
		return "Normalized posted stock_balances grouped by stock batch expiry; expired and future batches are shown from typed expiry_date, while legacy class/group calculations are not implemented."
	case "valuation":
		return "Normalized valuation is on_hand multiplied by stock_batches.unit_cost; legacy FIFO/average valuation and exact historical valuation are not reconciled."
	default:
		return "Normalized posted stock_balances projection joined to stock batches, items, and godowns; legacy manufacturer/category/class/reorder/narcotics groupings and exact valuation are not implemented."
	}
}

func reportFormatID(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func reportTitle(kind string) string {
	if spec, ok := reportSpecForKey(kind); ok {
		return spec.title
	}
	switch kind {
	case "daily-sales-detail":
		return "Daily Sales Detail"
	case "stock":
		return "Stock Reports"
	case "item":
		return "Item Reports"
	case "purchase-return":
		return "Purchase Return Reports"
	case "sale-detail":
		return "Sale detail"
	default:
		return strings.ReplaceAll(kind, "-", " ")
	}
}

func salesProjectionModeNote(mode string) string {
	if mode == "invoice-summary" {
		return "canonical and compatibility rows are grouped once per invoice with summed quantity and authoritative document amount; legacy tax, profit, and format-specific columns remain open"
	}
	return "captured legacy grouping and calculated numeric fields are not implemented"
}

func reportDefinitionForKey(kind, registryKey string) reportDefinition {
	dailySaleDetail := kind == "daily-sales-detail"
	spec, explicitEventProjection := reportSpecForKey(registryKey)
	concreteProjection := dailySaleDetail || kind == "stock" || kind == "item" || kind == "purchase-return"
	formatNames := []string{"Event ledger projection"}
	if concreteProjection && !dailySaleDetail {
		formatNames = []string{"Standard"}
	}
	if spec.purchaseReadModel {
		formatNames = []string{"Standard"}
	}
	if spec.stockReadModel {
		formatNames = []string{"Standard"}
	}
	if spec.financeMode != "" || spec.adminKind != "" {
		formatNames = []string{"Standard"}
	}
	if dailySaleDetail {
		formatNames = []string{
			"Standard",
			"Standard Format2",
			"Standard Format3",
			"Daily Sales Detail with Pack Qty",
			"Standard (with Patient Column)",
			"A Specified Group's Sale + Tax Info.",
			"With Patient, Customer and Remarks",
			"Selected Patient",
			"Standard (pack qty, pack rate, etc)",
			"Format 6 (with CNIC without discounts)",
		}
	}
	projectionStatus := "generic-fallback"
	projectionNote := "Generic event-ledger fallback; exact legacy projection is not implemented."
	columns := reportColumns()
	retrievalScope := "tenant and branch scoped"
	if spec.financeMode != "" {
		projectionStatus = "real"
		projectionNote = financeProjectionNote(spec.financeMode)
		columns = financeReportColumns(spec.financeMode)
		retrievalScope = "tenant, branch, posted-only, date, text, and normalized " + spec.financeMode + " projection"
		if spec.financeMode == "tax-withholding" {
			projectionStatus = "generic-fallback"
		}
	}
	if spec.adminKind != "" {
		projectionStatus = "real"
		columns = reportAdminColumns(spec.adminKind)
		projectionNote = "Tenant-scoped administrative projection; branch scope is applied where the source table carries a branch assignment. Legacy-only fields not present in the normalized tables are omitted."
		retrievalScope = "tenant, branch assignment where available, date, text, and normalized administrative records"
	}
	if explicitEventProjection {
		projectionStatus = "event-ledger"
		projectionNote = "Scoped immutable event-ledger view; captured legacy grouping and calculated numeric fields are not implemented."
		columns = reportEventLedgerColumns()
		retrievalScope = "tenant, branch, date, text, and immutable " + spec.aggregateCondition
		if spec.salesReadModel {
			if spec.aggregateCondition == reportSaleReturnAggregate {
				projectionNote = "Canonical posted cash/credit/open sale-return business_documents and lines with compatibility sale_return events; " + salesProjectionModeNote(spec.salesMode)
				retrievalScope = "tenant, branch, date, text, canonical sale-return documents/lines, and compatibility sale_return events"
			} else {
				projectionNote = "Scoped union of canonical cash/credit business_documents and compatibility sale events; " + salesProjectionModeNote(spec.salesMode)
				retrievalScope = "tenant, branch, date, text, canonical sales documents, and compatibility sale events"
			}
		}
		if spec.purchaseReadModel {
			projectionNote = "Canonical posted purchase business_documents/lines with supplier party ledger and stock ledger values when available; posted compatibility events are included only when no canonical document matches. Legacy grouping and unreconciled tax, profit, graph, and disparity calculations are not implemented."
			retrievalScope = "tenant, branch, date, text, supplier, canonical purchase documents/lines, stock ledger, supplier party ledger, and posted compatibility events"
			columns = reportColumns()
		}
		if spec.stockReadModel {
			projectionStatus = "real"
			columns = stockReportColumns(spec.stockMode)
			projectionNote = stockProjectionNote(spec.stockMode)
			retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, and normalized stock_balances"
		}
		if spec.financeMode != "" {
			projectionStatus = "real"
			projectionNote = financeProjectionNote(spec.financeMode)
			columns = financeReportColumns(spec.financeMode)
			retrievalScope = "tenant, branch, posted-only, date, text, and normalized " + spec.financeMode + " projection"
			if spec.financeMode == "tax-withholding" {
				projectionStatus = "generic-fallback"
			}
		}
		if spec.adminKind != "" {
			projectionStatus = "real"
			columns = reportAdminColumns(spec.adminKind)
			projectionNote = "Tenant-scoped administrative projection; branch scope is applied where the source table carries a branch assignment. Legacy-only fields not present in the normalized tables are omitted."
			retrievalScope = "tenant, branch assignment where available, date, text, and normalized administrative records"
		}
		if spec.compatibilityOnly && spec.financeMode == "" && spec.adminKind == "" &&
			!spec.stockReadModel && !spec.purchaseReadModel && !spec.salesReadModel {
			projectionStatus = "event-ledger"
			projectionNote = "Explicit compatibility fallback over posted sync_events; no normalized canonical projection exists for this legacy leaf, so missing legacy calculations and columns are not inferred."
			retrievalScope = "tenant, branch, posted-only compatibility events, date, and text"
		}
	}
	if concreteProjection {
		projectionStatus = "real"
		projectionNote = ""
	}
	if kind == "stock" || kind == "item" {
		projectionNote = "Scoped stock_ledger movement projection with stock_balances as the normalized balance cache; inventory_movements is used only as a labeled compatibility fallback when no normalized rows exist for the requested item scope."
	}
	if dailySaleDetail {
		projectionNote = "Canonical cash/credit business_documents and lines with compatibility sales projection/event fallback; legacy format-specific calculations are not implemented."
		retrievalScope = "tenant, branch, date, text, canonical sales documents, and compatibility sale events"
	}
	formats := make([]reportFormat, 0, len(formatNames))
	for _, name := range formatNames {
		formats = append(formats, reportFormat{ID: reportFormatID(name), Name: name, Source: "default"})
	}
	return reportDefinition{
		Kind: kind,
		Title: func() string {
			if explicitEventProjection {
				return spec.title
			}
			return reportTitle(kind)
		}(),
		ProjectionStatus: projectionStatus,
		ProjectionNote:   projectionNote,
		Columns:          columns,
		Formats:          formats,
		Retrieval: reportRetrieval{
			Title:              "Specify Retrieval Arguements",
			Areas:              []string{"DEFAULT AREA", "ALL AREAS"},
			SupportsCashCredit: dailySaleDetail,
			SupportsDateRange:  true,
			SupportsTextFilter: true,
			Scope:              retrievalScope,
		},
		Letterhead: reportLetterhead{
			Name:   defaultReportLetterheadName,
			Line2:  defaultReportLetterheadLine2,
			Line3:  defaultReportLetterheadLine3,
			Phone:  defaultReportLetterheadPhone,
			Source: "default",
		},
		Exports: []reportExportHook{
			{Format: "csv", Status: "available", Label: "CSV", Message: "CSV export is available."},
			{Format: "pdf", Status: "available", Label: "PDF", Message: "PDF export uses the print-preview letterhead and browser Save as PDF."},
			{Format: "excel", Status: "available", Label: "Excel", Message: "Excel-compatible workbook download is available."},
		},
	}
}

func reportDefinitionFor(kind string) reportDefinition {
	return reportDefinitionForKey(kind, kind)
}

func reportRegistryKey(kind, legacyPath string) string {
	if reportRegistryHasKey(kind) || kind == "daily-sales-detail" {
		return kind
	}
	segments := strings.Split(strings.TrimSpace(legacyPath), " > ")
	if len(segments) < 3 || strings.TrimSpace(strings.ReplaceAll(segments[0], "&", "")) != "Reports" {
		return kind
	}
	segments = segments[1:]
	for index := range segments {
		segments[index] = strings.TrimSpace(strings.ReplaceAll(strings.Split(segments[index], "\t")[0], "&", ""))
	}
	if len(segments) > 1 && segments[0] == "Daily Reports" {
		segments = segments[1:]
		if len(segments) > 1 && (segments[0] == "Sale" || segments[0] == "Sales Return" ||
			segments[0] == "Purchase" || segments[0] == "Purchase Return" || segments[0] == "Purchase Order") {
			segments = segments[1:]
		}
	} else if len(segments) > 0 && (segments[0] == "Sales Reports" ||
		segments[0] == "Purchase Reports" || segments[0] == "Purchase Return Reports" ||
		segments[0] == "Stock Reports") {
		segments = segments[1:]
	}
	var builder strings.Builder
	for _, segment := range segments {
		if builder.Len() > 0 {
			builder.WriteByte('-')
		}
		builder.WriteString(reportFormatID(segment))
	}
	candidate := builder.String()
	if reportRegistryHasKey(candidate) {
		return candidate
	}
	return kind
}

func reportDefinitionForPath(kind, legacyPath string) reportDefinition {
	return reportDefinitionForKey(kind, reportRegistryKey(kind, legacyPath))
}

func applyReportPreferences(definition *reportDefinition, values map[string]map[string]string) {
	if letterhead, ok := values["report:letterhead"]; ok {
		setReportLetterheadValue := func(current *string, key string, maxLength int) {
			if value := strings.TrimSpace(letterhead[key]); value != "" && len(value) <= maxLength {
				*current = value
				definition.Letterhead.Source = "database"
			}
		}
		setReportLetterheadValue(&definition.Letterhead.Name, "name", 160)
		setReportLetterheadValue(&definition.Letterhead.Line2, "line2", 160)
		setReportLetterheadValue(&definition.Letterhead.Line3, "line3", 160)
		setReportLetterheadValue(&definition.Letterhead.Phone, "phone", 80)
		setReportLetterheadValue(&definition.Letterhead.Fax, "fax", 80)
	}
	if formats, ok := values["report:format:"+definition.Kind]; ok {
		loaded := make([]reportFormat, 0, len(formats))
		for name := range formats {
			name = strings.TrimSpace(name)
			if name == "" || len(name) > 160 {
				continue
			}
			loaded = append(loaded, reportFormat{ID: reportFormatID(name), Name: name, Source: "database"})
		}
		if len(loaded) > 0 {
			definition.Formats = loaded
		}
	}
}

func applyReportFormatNames(definition *reportDefinition, names []string) {
	loaded := make([]reportFormat, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 160 {
			continue
		}
		loaded = append(loaded, reportFormat{ID: reportFormatID(name), Name: name, Source: "database"})
	}
	if len(loaded) > 0 {
		definition.Formats = loaded
	}
}

// salesReadModelQuery returns the shared sale projection used by reports and
// transaction history. Canonical documents are primary; compatibility sales
// projections/events are included only when no canonical document represents
// the same tenant/branch document. This keeps old event rows visible without
// double-counting a document during a mixed rollout.
func salesReadModelQuery(aggregateCondition, pagination string) string {
	eventCondition := "se.aggregate = 'sale'"
	canonicalReturnUnion := ""
	if aggregateCondition == reportSaleOrReturn {
		eventCondition = "se.aggregate IN ('sale', 'sale_return')"
		canonicalReturnUnion = `

			UNION ALL

			SELECT bd.document_number,
			       bd.occurred_at,
			       COALESCE(mp.name,
			                CASE WHEN bd.kind IN ('cash-return', 'open-cash-return',
			                                       'cash-sale-return', 'open-sale-return')
			                     THEN 'CASH' ELSE '' END),
			       COALESCE(bl.item_name, ''),
			       COALESCE(bl.quantity::text, ''),
			       bd.total_amount::text
			FROM business_documents bd
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
			 AND mp.party_type = 'customer'
			LEFT JOIN business_document_lines bl
			  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
			 AND bl.document_id = bd.id
			WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
			  AND bd.kind IN ('cash-return', 'credit-return',
			                  'open-cash-return', 'open-credit-return',
			                  'cash-sale-return', 'credit-sale-return',
			                  'open-sale-return')
			  AND bd.status = 'posted'`
	}
	return `
	WITH sales_read_model AS (
			SELECT bd.document_number AS document,
			       bd.occurred_at,
			       COALESCE(mp.name, CASE WHEN bd.kind = 'cash-sale' THEN 'CASH' ELSE '' END) AS party,
			       COALESCE(bl.item_name, '') AS item,
			       COALESCE(bl.quantity::text, '') AS quantity,
			       bd.total_amount::text AS amount
			FROM business_documents bd
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
			 AND mp.party_type = 'customer'
			LEFT JOIN business_document_lines bl
			  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
			 AND bl.document_id = bd.id
			WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
			  AND bd.kind IN ('cash-sale', 'credit-sale')
			  AND bd.status = 'posted'
		` + canonicalReturnUnion + `

			UNION ALL

			SELECT sd.document_number,
			       sd.occurred_at,
			       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
			       COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName', ''),
			       COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
			       sd.total_amount::text
			FROM sales_documents sd
			LEFT JOIN sync_events se
			  ON se.tenant_id = sd.tenant_id AND se.branch_id = sd.branch_id
			 AND se.aggregate_id = sd.id AND se.aggregate = 'sale'
			WHERE sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid
			  AND sd.status = 'posted'
			  AND NOT EXISTS (
				SELECT 1
				FROM business_documents bd
				WHERE bd.tenant_id = sd.tenant_id AND bd.branch_id = sd.branch_id
				  AND (bd.id = sd.id OR bd.document_number = sd.document_number)
			  )

			UNION ALL

			SELECT COALESCE(se.payload->>'documentNumber', se.aggregate_id::text),
			       se.occurred_at,
			       COALESCE(se.payload->>'customerName', se.payload->>'customer',
			                se.payload->>'supplierName', se.payload->>'supplier', se.aggregate),
			       COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName', ''),
			       COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
			       COALESCE(se.payload->>'totalAmount', se.payload->>'amount', '')
			FROM sync_events se
			WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
			  AND ` + eventCondition + `
			  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
			  AND NOT EXISTS (
				SELECT 1
				FROM business_documents bd
				WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
				  AND (bd.id = se.aggregate_id
				       OR bd.document_number = se.payload->>'documentNumber')
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM sales_documents sd
				WHERE sd.tenant_id = se.tenant_id AND sd.branch_id = se.branch_id
				  AND sd.id = se.aggregate_id
			  )
		)
		SELECT document, occurred_at::text, party, item, quantity, amount
		FROM sales_read_model
		WHERE occurred_at >= $3::date
		  AND occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
		       OR party ILIKE '%' || $5 || '%'
		       OR item ILIKE '%' || $5 || '%')
		ORDER BY occurred_at DESC, document, item ` + pagination
}

// salesReadModelQueryMode keeps the detailed projection as the compatibility
// default while allowing captured invoice-summary leaves to aggregate each
// document once. The underlying read model is still the authority for
// document identity, date scope, and compatibility de-duplication.
func salesReadModelQueryMode(aggregateCondition, mode, pagination string) string {
	if mode != "invoice-summary" {
		return salesReadModelQuery(aggregateCondition, pagination)
	}
	return invoiceSummaryReadModelQuery(salesReadModelQuery(aggregateCondition, ""), pagination)
}

func invoiceSummaryReadModelQuery(baseQuery, pagination string) string {
	return `
	WITH invoice_rows AS (` + baseQuery + `)
	SELECT document,
	       MAX(occurred_at)::text,
	       MAX(party),
	       '',
	       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0)::text,
	       COALESCE(MAX(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0)::text
	FROM invoice_rows
	GROUP BY document
	ORDER BY MAX(occurred_at) DESC, document ` + pagination
}

// saleReturnReadModelQuery is the posted sale-return counterpart to
// salesReadModelQuery. Source-bound and open returns are both canonical
// business documents; compatibility sale_return events remain visible only
// when no canonical document with the same identity exists.
func saleReturnReadModelQuery(pagination string) string {
	return `
	WITH sale_return_read_model AS (
		SELECT bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(mp.name,
		                CASE WHEN bd.kind IN ('cash-return', 'open-cash-return',
		                                       'cash-sale-return', 'open-sale-return')
		                     THEN 'CASH' ELSE '' END) AS party,
		       COALESCE(bl.item_name, '') AS item,
		       COALESCE(bl.quantity::text, '') AS quantity,
		       bd.total_amount::text AS amount
		FROM business_documents bd
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
		 AND mp.party_type = 'customer'
		LEFT JOIN business_document_lines bl
		  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
		 AND bl.document_id = bd.id
		WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
		  AND bd.kind IN ('cash-return', 'credit-return',
		                  'open-cash-return', 'open-credit-return',
		                  'cash-sale-return', 'credit-sale-return',
		                  'open-sale-return')
		  AND bd.status = 'posted'

		UNION ALL

		SELECT COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName', ''),
		       COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
		       COALESCE(se.payload->>'totalAmount', se.payload->>'amount', '')
		FROM sync_events se
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND se.aggregate IN ('sale_return', 'sale-return')
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND (bd.id = se.aggregate_id
			       OR bd.document_number = se.payload->>'documentNumber')
		  )
	)
	SELECT document, occurred_at::text, party, item, quantity, amount
	FROM sale_return_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR item ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document, item ` + pagination
}

func saleReturnReadModelQueryMode(mode, pagination string) string {
	if mode != "invoice-summary" {
		return saleReturnReadModelQuery(pagination)
	}
	return invoiceSummaryReadModelQuery(saleReturnReadModelQuery(""), pagination)
}

// purchaseReadModelQuery is the shared purchase report/history projection.
// Canonical posted documents are authoritative. Stock movements provide the
// received/returned quantity, while the supplier party ledger supplies the
// posted document amount when it exists. Compatibility events remain visible
// during rollout but are suppressed by scoped document identity.
func purchaseReadModelQuery(aggregateCondition, mode, pagination string) string {
	canonicalKinds := "'pack-purchase', 'loose-purchase', 'opening-purchase'"
	eventAggregate := "receiving"
	if aggregateCondition == "se.aggregate = 'return'" {
		canonicalKinds = "'purchase-return'"
		eventAggregate = "return"
	} else if aggregateCondition == "se.aggregate = 'purchase_order'" {
		canonicalKinds = "'purchase-order'"
		eventAggregate = "purchase_order"
	}
	detail := mode == "detail"
	itemExpression := "''"
	quantityExpression := "SUM(COALESCE(stock.stock_quantity, l.quantity))"
	amountExpression := "COALESCE(ple.amount, d.total_amount)"
	groupBy := "d.document_number, d.occurred_at, mp.name, ple.amount, d.total_amount"
	if detail {
		itemExpression = "l.item_name"
		quantityExpression = "COALESCE(stock.stock_quantity, l.quantity)"
		amountExpression = "l.line_total"
		groupBy += ", l.line_number, l.item_name, l.line_total, l.quantity, stock.stock_quantity"
	}
	return `
		WITH canonical_purchase AS (
			SELECT d.document_number AS document,
			       d.occurred_at,
			       COALESCE(mp.name, '') AS party,
			       ` + itemExpression + ` AS item,
			       ` + quantityExpression + `::text AS quantity,
			       ` + amountExpression + `::text AS amount
			FROM business_documents d
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = d.tenant_id AND mp.id = d.supplier_id
			 AND mp.party_type = 'supplier'
			LEFT JOIN business_document_lines l
			  ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id
			 AND l.document_id = d.id
			LEFT JOIN LATERAL (
				SELECT SUM(sl.quantity * sl.adjustment_sign) AS stock_quantity
				FROM stock_ledger sl
				WHERE sl.tenant_id = d.tenant_id AND sl.branch_id = d.branch_id
				  AND sl.source_document_id = d.id
				  AND sl.source_document_line_id = l.id
			) stock ON true
			LEFT JOIN LATERAL (
				SELECT CASE WHEN d.kind = 'purchase-return'
				            THEN NULLIF(p.debit_amount, 0)
				            ELSE NULLIF(p.credit_amount, 0)
				       END AS amount
				FROM party_ledger_entries p
				WHERE p.tenant_id = d.tenant_id AND p.branch_id = d.branch_id
				  AND p.party_id = d.supplier_id
				  AND p.source_document_id = d.id
				  AND p.counterparty_kind = 'supplier'
				  AND p.entry_kind IN ('purchase', 'purchase-return')
				ORDER BY p.occurred_at DESC, p.id DESC
				LIMIT 1
			) ple ON true
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
			  AND d.kind IN (` + canonicalKinds + `)
			  AND d.status = 'posted'
			GROUP BY ` + groupBy + `

			UNION ALL

			SELECT compatibility.document, compatibility.occurred_at,
			       compatibility.party, compatibility.item,
			       compatibility.quantity, compatibility.amount
			FROM (
				SELECT DISTINCT ON (
					COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text)
				)
					COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text) AS document,
					se.occurred_at,
					COALESCE(se.payload->>'supplierName', se.payload->>'supplier', '') AS party,
					COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName', '') AS item,
					COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', '') AS quantity,
					COALESCE(se.payload->>'totalAmount', se.payload->>'amount', '') AS amount,
					se.aggregate_id,
					se.payload->>'documentNumber' AS payload_document_number
				FROM sync_events se
				WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
				  AND se.aggregate = '` + eventAggregate + `'
				  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
				ORDER BY
					COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
					se.occurred_at DESC, se.event_id DESC
			) compatibility
			WHERE NOT EXISTS (
				SELECT 1
				FROM business_documents d
				WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
				  AND d.status = 'posted'
				  AND (d.id = compatibility.aggregate_id
				       OR d.document_number = compatibility.payload_document_number)
			)
		)
		SELECT document, occurred_at::text, party, item, quantity, amount
		FROM canonical_purchase
		WHERE occurred_at >= $3::date
		  AND occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
		       OR party ILIKE '%' || $5 || '%'
		       OR item ILIKE '%' || $5 || '%')
		ORDER BY occurred_at DESC, document, item ` + pagination
}

// stockReadModelQuery keeps Phase P on the normalized stock authority. Ledger
// rows are joined to their immutable source events so draft/void compatibility
// payloads cannot enter a report. Balance reports use the rebuildable
// stock_balances cache, but require a posted ledger row for the batch.
func stockReadModelQuery(mode, pagination string) string {
	if mode == "movement" {
		return `
			WITH posted_stock_ledger AS (
				SELECT l.id::text AS movement_id, l.occurred_at, l.direction,
				       l.quantity, l.adjustment_sign, l.unit_cost,
				       b.item_legacy_id, b.batch_number,
				       COALESCE(g.name, '') AS godown,
				       COALESCE(i.name, '') AS item_name
				FROM stock_ledger l
				JOIN sync_events se
				  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
				JOIN stock_batches b
				  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id
				 AND b.id = l.batch_id
				LEFT JOIN master_godowns g
				  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
				LEFT JOIN master_items i
				  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
				WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
				  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
				  AND ($6 = '' OR b.godown_id = $6::uuid)
				  AND ($7 = '' OR b.batch_number ILIKE '%' || $7 || '%')
			)
			SELECT movement_id, occurred_at::text, direction,
			       COALESCE(NULLIF(item_name, ''), item_legacy_id),
			       (CASE
					WHEN direction = 'out' THEN -quantity
					WHEN direction = 'adjustment' THEN quantity * adjustment_sign
					ELSE quantity
				END)::text,
			       unit_cost::text
			FROM posted_stock_ledger
			WHERE occurred_at >= $3::date
			  AND occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR movement_id ILIKE '%' || $5 || '%'
			       OR item_legacy_id ILIKE '%' || $5 || '%'
			       OR item_name ILIKE '%' || $5 || '%'
			       OR godown ILIKE '%' || $5 || '%'
			       OR batch_number ILIKE '%' || $5 || '%')
			ORDER BY occurred_at DESC, movement_id
			` + pagination
	}
	if mode == "adjustment" {
		return `
			SELECT l.id::text, l.occurred_at::text, l.direction,
			       COALESCE(NULLIF(i.name, ''), b.item_legacy_id),
			       (l.quantity * l.adjustment_sign)::text, l.unit_cost::text
			FROM stock_ledger l
			JOIN sync_events se
			  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
			JOIN stock_batches b
			  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id AND b.id = l.batch_id
			LEFT JOIN master_items i
			  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
			WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
			  AND l.direction = 'adjustment'
			  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
			  AND l.occurred_at >= $3::date
			  AND l.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR l.id::text ILIKE '%' || $5 || '%'
			       OR b.item_legacy_id ILIKE '%' || $5 || '%'
			       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%')
			ORDER BY l.occurred_at DESC, l.id
			` + pagination
	}

	valuation := "unit_cost"
	if mode == "valuation" {
		valuation = "on_hand * unit_cost"
	}
	expiryFilter := "updated_at >= $3::date AND updated_at < ($4::date + INTERVAL '1 day')"
	if mode == "expiry" {
		expiryFilter = "expiry_date IS NOT NULL AND expiry_date BETWEEN $3::date AND $4::date"
	}
	return `
		SELECT b.batch_number,
		       COALESCE(b.expiry_date::text, sb.updated_at::text),
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id),
		       sb.on_hand::text,
		       (` + valuation + `)::text
		FROM stock_balances sb
		JOIN stock_batches b
		  ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id
		 AND b.id = sb.batch_id
		LEFT JOIN master_godowns g
		  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
		LEFT JOIN master_items i
		  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  AND sb.on_hand <> 0
		  AND ` + expiryFilter + `
		  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
		       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
		       OR i.name ILIKE '%' || $5 || '%'
		       OR g.name ILIKE '%' || $5 || '%')
		  AND ($6 = '' OR b.godown_id = $6::uuid)
		  AND ($7 = '' OR b.batch_number ILIKE '%' || $7 || '%')
		  AND EXISTS (
			SELECT 1
			FROM stock_ledger l
			JOIN sync_events se
			  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
			WHERE l.tenant_id = sb.tenant_id AND l.branch_id = sb.branch_id
			  AND l.batch_id = sb.batch_id
			  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  )
		ORDER BY sb.updated_at DESC, b.batch_number, sb.batch_id
		` + pagination
}

func financeReadModelQuery(mode, pagination string) string {
	switch mode {
	case "gl", "gl-cash":
		accountFilter := ""
		if mode == "gl-cash" {
			accountFilter = "AND a.system_key = 'cash'"
		}
		return `
			SELECT j.id::text, j.posted_at::text, a.code || ' - ' || a.name,
			       l.memo, l.debit_amount::text, l.credit_amount::text
			FROM gl_journals j
			JOIN gl_lines l
			  ON l.tenant_id = j.tenant_id AND l.branch_id = j.branch_id
			 AND l.journal_id = j.id
			JOIN finance_accounts a
			  ON a.tenant_id = l.tenant_id AND a.id = l.account_id
			WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
			  AND j.status = 'posted'
			  ` + accountFilter + `
			  AND j.posted_at >= $3::date
			  AND j.posted_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR j.id::text ILIKE '%' || $5 || '%'
			       OR a.code ILIKE '%' || $5 || '%'
			       OR a.name ILIKE '%' || $5 || '%'
			       OR l.memo ILIKE '%' || $5 || '%')
			ORDER BY j.posted_at DESC, j.id, l.line_number
			` + pagination
	case "trial-balance":
		return `
			SELECT a.code, MAX(j.posted_at)::text, c.name, a.name,
			       COALESCE(SUM(l.debit_amount), 0)::text,
			       COALESCE(SUM(l.credit_amount), 0)::text
			FROM gl_journals j
			JOIN gl_lines l
			  ON l.tenant_id = j.tenant_id AND l.branch_id = j.branch_id
			 AND l.journal_id = j.id
			JOIN finance_accounts a
			  ON a.tenant_id = l.tenant_id AND a.id = l.account_id
			JOIN finance_account_categories c
			  ON c.tenant_id = a.tenant_id AND c.code = a.category_code
			WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
			  AND j.status = 'posted'
			  AND j.posted_at >= $3::date
			  AND j.posted_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR a.code ILIKE '%' || $5 || '%'
			       OR a.name ILIKE '%' || $5 || '%'
			       OR c.name ILIKE '%' || $5 || '%')
			GROUP BY a.code, c.name, a.name
			ORDER BY a.code
			` + pagination
	case "party-customer", "party-supplier":
		partyKind := "customer"
		if mode == "party-supplier" {
			partyKind = "supplier"
		}
		return `
			SELECT ple.source_document_id::text, ple.occurred_at::text,
			       COALESCE(mp.name, ''), ple.description,
			       ple.debit_amount::text, ple.credit_amount::text
			FROM party_ledger_entries ple
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = ple.tenant_id AND mp.id = ple.party_id
			 AND mp.party_type = '` + partyKind + `'
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = '` + partyKind + `'
			  AND ple.occurred_at >= $3::date
			  AND ple.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR ple.source_document_id::text ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR ple.description ILIKE '%' || $5 || '%')
			ORDER BY ple.occurred_at, ple.id
			` + pagination
	case "aging-receivable", "aging-payable":
		partyKind := "customer"
		if mode == "aging-payable" {
			partyKind = "supplier"
		}
		return `
			SELECT ple.party_id::text, MAX(ple.occurred_at)::text,
			       COALESCE(mp.name, ''), 'UNAGED - due date unavailable',
			       COALESCE(SUM(ple.debit_amount), 0)::text,
			       COALESCE(SUM(ple.credit_amount), 0)::text
			FROM party_ledger_entries ple
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = ple.tenant_id AND mp.id = ple.party_id
			 AND mp.party_type = '` + partyKind + `'
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = '` + partyKind + `'
			  AND ple.occurred_at >= $3::date
			  AND ple.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR ple.party_id::text ILIKE '%' || $5 || '%')
			GROUP BY ple.party_id, mp.name
			HAVING SUM(ple.debit_amount - ple.credit_amount) <> 0
			ORDER BY mp.name, ple.party_id
			` + pagination
	case "tax-output", "tax-input", "tax-advance":
		kindFilter := "'cash-sale', 'credit-sale'"
		if mode == "tax-input" {
			kindFilter = "'pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return'"
		}
		advanceFilter := ""
		if mode == "tax-advance" {
			advanceFilter = "AND l.advance_tax_rate > 0"
		}
		return `
			SELECT d.document_number, d.occurred_at::text,
			       COALESCE(mp.name, ''), l.item_name,
			       l.line_total::text, l.tax_amount::text
			FROM business_documents d
			JOIN business_document_lines l
			  ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id
			 AND l.document_id = d.id
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = d.tenant_id
			 AND mp.id = CASE WHEN d.kind IN ('cash-sale', 'credit-sale')
			                  THEN d.customer_id ELSE d.supplier_id END
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
			  AND d.status = 'posted' AND d.kind IN (` + kindFilter + `)
			  AND l.tax_amount > 0 ` + advanceFilter + `
			  AND d.occurred_at >= $3::date
			  AND d.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR d.document_number ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR l.item_name ILIKE '%' || $5 || '%')
			ORDER BY d.occurred_at DESC, d.document_number, l.line_number
			` + pagination
	case "tax-withholding":
		return `
			SELECT ''::text, ''::text, ''::text, ''::text, ''::text, ''::text
			WHERE false
			` + pagination
	case "voucher":
		return `
			SELECT ve.id::text, ve.created_at::text, vc.name, ve.description,
			       ve.amount::text, ve.status
			FROM voucher_entries ve
			JOIN voucher_categories vc
			  ON vc.tenant_id = ve.tenant_id AND vc.code = ve.category_code
			WHERE ve.tenant_id = $1::uuid AND ve.branch_id = $2::uuid
			  AND ve.status = 'posted'
			  AND ve.created_at >= $3::date
			  AND ve.created_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR ve.id::text ILIKE '%' || $5 || '%'
			       OR vc.name ILIKE '%' || $5 || '%'
			       OR ve.description ILIKE '%' || $5 || '%')
			ORDER BY ve.created_at DESC, ve.id
			` + pagination
	}
	return ""
}

func adminReadModelQuery(kind, pagination string) string {
	switch kind {
	case "users":
		return `
			SELECT u.id::text, u.created_at::text, u.username, u.display_name,
			       CASE WHEN u.active THEN 'true' ELSE 'false' END,
			       COALESCE(string_agg(r.name, ', ' ORDER BY r.name), '')
			FROM users u
			LEFT JOIN user_memberships um
			  ON um.tenant_id = u.tenant_id AND um.user_id = u.id
			LEFT JOIN roles r
			  ON r.tenant_id = um.tenant_id AND r.id = um.role_id
			LEFT JOIN user_branch_assignments uba
			  ON uba.tenant_id = u.tenant_id AND uba.user_id = u.id
			WHERE u.tenant_id = $1::uuid
			  AND (uba.branch_id = $2::uuid OR uba.branch_id IS NULL)
			  AND u.created_at >= $3::date
			  AND u.created_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR u.username ILIKE '%' || $5 || '%'
			       OR u.display_name ILIKE '%' || $5 || '%')
			GROUP BY u.id, u.created_at, u.username, u.display_name, u.active
			ORDER BY u.created_at DESC, u.id
			` + pagination
	case "roles":
		return `
			SELECT r.id::text, COALESCE(rp.permission, ''), r.name, r.code,
			       CASE WHEN COALESCE(rp.allowed, true) THEN 'true' ELSE 'false' END,
			       'normalized role_permissions'
			FROM roles r
			LEFT JOIN role_permissions rp
			  ON rp.tenant_id = r.tenant_id AND rp.role_id = r.id
			WHERE r.tenant_id = $1::uuid
			  AND $3::date <= $4::date
			  AND ($5 = '' OR r.code ILIKE '%' || $5 || '%'
			       OR r.name ILIKE '%' || $5 || '%'
			       OR COALESCE(rp.permission, '') ILIKE '%' || $5 || '%')
			ORDER BY r.code, rp.permission
			` + pagination
	default:
		kind = strings.ReplaceAll(kind, "'", "''")
		return `
			SELECT COALESCE(mr.code, ''), mr.updated_at::text, mr.kind, mr.name,
			       CASE WHEN mr.active THEN 'true' ELSE 'false' END,
			       COALESCE(mr.legacy_id, '')
			FROM master_records mr
			WHERE mr.tenant_id = $1::uuid AND mr.kind = '` + kind + `'
			  AND mr.updated_at >= $3::date
			  AND mr.updated_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR mr.code ILIKE '%' || $5 || '%'
			       OR mr.name ILIKE '%' || $5 || '%'
			       OR COALESCE(mr.legacy_id, '') ILIKE '%' || $5 || '%')
			ORDER BY mr.updated_at DESC, mr.code
			` + pagination
	}
}

func compatibilityReportQuery(aggregateCondition, pagination string) string {
	return `
		SELECT se.event_id::text, se.occurred_at::text,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer',
		                se.payload->>'supplierName', se.payload->>'supplier', se.aggregate),
		       COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName',
		                se.payload->>'documentNumber', ''),
		       COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
		       COALESCE(se.payload->>'totalAmount', se.payload->>'amount', '')
		FROM sync_events se
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND se.occurred_at >= $3::date
		  AND se.occurred_at < ($4::date + INTERVAL '1 day')
		  AND ` + aggregateCondition + `
		  AND ($5 = '' OR se.aggregate ILIKE '%' || $5 || '%'
		       OR se.event_id::text ILIKE '%' || $5 || '%'
		       OR se.payload::text ILIKE '%' || $5 || '%')
		ORDER BY se.occurred_at DESC, se.event_id
		` + pagination
}

func (s *Server) loadReportDefinition(ctx context.Context, operator *sessionContext, kind, legacyPath string) reportDefinition {
	definition := reportDefinitionForPath(kind, legacyPath)
	if s.database == nil {
		return definition
	}
	tx, err := s.beginScopedTx(ctx, operator)
	if err != nil {
		return definition
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT category, caption, value FROM tenant_preferences WHERE tenant_id = $1::uuid AND category LIKE 'report:%' ORDER BY category, position, caption`, operator.TenantID)
	if err != nil {
		return definition
	}
	values := make(map[string]map[string]string)
	formatNames := make([]string, 0)
	for rows.Next() {
		var category, caption, value string
		if err := rows.Scan(&category, &caption, &value); err != nil {
			rows.Close()
			return definition
		}
		if values[category] == nil {
			values[category] = make(map[string]string)
		}
		values[category][strings.ToLower(strings.TrimSpace(caption))] = value
		if category == "report:format:"+kind {
			formatNames = append(formatNames, caption)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return definition
	}
	rows.Close()
	applyReportPreferences(&definition, values)
	applyReportFormatNames(&definition, formatNames)
	return definition
}

func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if kind == "" || len(kind) > 96 || strings.ContainsAny(kind, "/\\?&= ") {
		writeProblem(w, http.StatusBadRequest, "invalid_report_kind", "Invalid report", "The requested report is not supported.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "reports.read") {
		return
	}
	if !s.requireScope(r, w, operator, "report", kind) {
		return
	}
	if godownID := strings.TrimSpace(r.URL.Query().Get("godownId")); godownID != "" &&
		!s.requireScope(r, w, operator, "godown", godownID) {
		return
	}
	if priceID := strings.TrimSpace(r.URL.Query().Get("priceId")); priceID != "" &&
		!s.requireScope(r, w, operator, "price", priceID) {
		return
	}
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" {
		from = "1900-01-01"
	}
	if to == "" {
		to = "2999-12-31"
	}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_report_date", "Invalid report date", "The from date must use YYYY-MM-DD.")
		return
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_report_date", "Invalid report date", "The to date must use YYYY-MM-DD.")
		return
	}
	if from > to {
		writeProblem(w, http.StatusBadRequest, "invalid_report_date_range", "Invalid report date range", "The from date must be on or before the to date.")
		return
	}
	page, pageSize, err := reportPagination(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_report_pagination", "Invalid report pagination", err.Error())
		return
	}
	legacyPath := strings.TrimSpace(r.URL.Query().Get("legacyPath"))
	registryKey := reportRegistryKey(kind, legacyPath)
	reportTimeout := s.reportTimeout
	if reportTimeout <= 0 {
		reportTimeout = 5 * time.Second
	}
	reportCtx, cancel := context.WithTimeout(r.Context(), reportTimeout)
	defer cancel()
	definition := s.loadReportDefinition(reportCtx, operator, kind, legacyPath)
	tx, err := s.beginScopedTx(reportCtx, operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The report store could not be opened.")
		return
	}
	defer tx.Rollback()
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	rows := make([]reportRow, 0, pageSize+1)
	appendRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity, &row.Amount); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return result.Err()
	}

	var query string
	switch kind {
	case "daily-sales-detail":
		query = salesReadModelQuery(reportSaleAggregate, "LIMIT $6 OFFSET $7")
		if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The sales report query failed.")
			return
		}
	case "stock", "item":
		godownID := strings.TrimSpace(r.URL.Query().Get("godownId"))
		if godownID != "" && !documentUUIDPattern.MatchString(godownID) {
			writeProblem(w, http.StatusBadRequest, "invalid_godown", "Invalid godown", "The godownId must be a UUID.")
			return
		}
		query = `
			SELECT movement_id, occurred_at::text, direction, item_legacy_id, quantity, ''
			FROM (
				SELECT l.id::text AS movement_id, l.occurred_at, l.direction,
				       b.item_legacy_id, l.quantity::text AS quantity
				FROM stock_ledger l
				JOIN stock_batches b
				  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id AND b.id = l.batch_id
				WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
				  AND ($8 = '' OR b.godown_id = $8::uuid)
				  AND l.occurred_at >= $3::date
				  AND l.occurred_at < ($4::date + INTERVAL '1 day')
				  AND ($5 = '' OR b.item_legacy_id ILIKE '%' || $5 || '%')

				UNION ALL

				SELECT im.source_event_id::text, im.occurred_at, im.direction,
				       im.item_legacy_id, im.quantity::text
				FROM inventory_movements im
				JOIN sync_events se
				  ON se.tenant_id = im.tenant_id AND se.event_id = im.source_event_id
				WHERE im.tenant_id = $1::uuid AND im.branch_id = $2::uuid
				  AND im.occurred_at >= $3::date
				  AND im.occurred_at < ($4::date + INTERVAL '1 day')
				  AND ($5 = '' OR im.item_legacy_id ILIKE '%' || $5 || '%')
				  AND ($8 = '' OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements(
						CASE WHEN jsonb_typeof(se.payload->'rows') = 'array'
						     THEN se.payload->'rows' ELSE jsonb_build_array(se.payload) END
					) AS payload_row
					WHERE payload_row->>'godownId' = $8
				  ))
				  AND NOT EXISTS (
					SELECT 1
					FROM stock_ledger l2
					JOIN stock_batches b2
					  ON b2.tenant_id = l2.tenant_id AND b2.branch_id = l2.branch_id
					 AND b2.id = l2.batch_id
					WHERE l2.tenant_id = im.tenant_id AND l2.branch_id = im.branch_id
					  AND b2.item_legacy_id = im.item_legacy_id
					  AND ($8 = '' OR b2.godown_id = $8::uuid)
				  )
			) stock_read_model
			ORDER BY occurred_at DESC, movement_id
			LIMIT $6 OFFSET $7`
		if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize, godownID); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The stock report query failed.")
			return
		}
	case "purchase-return":
		query = purchaseReadModelQuery("se.aggregate = 'return'", "detail", "LIMIT $6 OFFSET $7")
		if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The purchase-return report query failed.")
			return
		}
	default:
		// Every captured report leaf is backed by the immutable event ledger. The
		// aggregate selection keeps sale, purchase, return, quotation, order, and
		// stock reports semantically scoped instead of returning an unrelated
		// catch-all event stream. Payload fields preserve the legacy party/item/
		// quantity/amount values when a workflow supplied them.
		aggregateCondition := reportAggregateCondition(registryKey)
		if spec, ok := reportSpecForKey(registryKey); ok && spec.stockReadModel {
			godownID := strings.TrimSpace(r.URL.Query().Get("godownId"))
			if godownID != "" && !documentUUIDPattern.MatchString(godownID) {
				writeProblem(w, http.StatusBadRequest, "invalid_godown", "Invalid godown", "The godownId must be a UUID.")
				return
			}
			batchNumber := strings.TrimSpace(r.URL.Query().Get("batchNumber"))
			query = stockReadModelQuery(spec.stockMode, "LIMIT $8 OFFSET $9")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, godownID, batchNumber, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The normalized stock report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.purchaseReadModel && spec.financeMode == "" {
			query = purchaseReadModelQuery(spec.aggregateCondition, spec.purchaseMode, "LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The purchase report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.salesReadModel && spec.financeMode == "" {
			if spec.aggregateCondition == reportSaleReturnAggregate {
				query = saleReturnReadModelQueryMode(spec.salesMode, "LIMIT $6 OFFSET $7")
			} else {
				query = salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
			}
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The sales report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.financeMode != "" {
			query = financeReadModelQuery(spec.financeMode, "LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The normalized financial report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.adminKind != "" {
			query = adminReadModelQuery(spec.adminKind, "LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The normalized administrative report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.compatibilityOnly {
			query = compatibilityReportQuery(spec.aggregateCondition, "LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The labeled compatibility report query failed.")
				return
			}
			break
		}
		query = `SELECT se.event_id::text, se.occurred_at::text,
			COALESCE(se.payload->>'customerName', se.payload->>'customer', se.payload->>'supplierName', se.payload->>'supplier', se.aggregate),
			COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName', se.payload->>'documentNumber', ''),
			COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
			COALESCE(se.payload->>'totalAmount', se.payload->>'amount', '')
			FROM sync_events se
			WHERE se.tenant_id = $1::uuid AND ($2 = '' OR se.branch_id::text = $2)
			AND se.occurred_at >= $3::date
			AND se.occurred_at < ($4::date + INTERVAL '1 day')
			AND ` + aggregateCondition + `
			AND ($5 = '' OR se.aggregate ILIKE '%' || $5 || '%' OR se.event_id::text ILIKE '%' || $5 || '%' OR se.payload::text ILIKE '%' || $5 || '%')
			ORDER BY se.occurred_at DESC LIMIT $6 OFFSET $7`
		if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The legacy report query failed.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The report transaction could not be committed.")
		return
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":       kind,
		"rows":       rows,
		"definition": definition,
		"page":       page,
		"pageSize":   pageSize,
		"hasMore":    hasMore,
	})
}

func reportPagination(r *http.Request) (int, int, error) {
	page, pageSize := 1, 1000
	if value := strings.TrimSpace(r.URL.Query().Get("page")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100000 {
			return 0, 0, fmt.Errorf("page must be a positive integer no greater than 100000")
		}
		page = parsed
	}
	if value := strings.TrimSpace(r.URL.Query().Get("pageSize")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1000 {
			return 0, 0, fmt.Errorf("pageSize must be between 1 and 1000")
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

// reportAggregateCondition maps registered legacy report leaves to the
// immutable event aggregates produced by the corresponding workflow. The
// default deliberately remains broad for uncatalogued leaves and is surfaced
// as generic-fallback metadata rather than presented as an exact projection.
func reportAggregateCondition(kind string) string {
	kind = strings.ToLower(kind)
	if spec, ok := reportSpecForKey(kind); ok {
		return spec.aggregateCondition
	}
	switch {
	case strings.Contains(kind, "purchase-return"):
		return "se.aggregate = 'return'"
	case strings.Contains(kind, "sale-return") || strings.Contains(kind, "sales-return"):
		return "se.aggregate = 'sale_return'"
	case strings.Contains(kind, "refused"):
		return "se.aggregate = 'refused_sale'"
	case strings.Contains(kind, "quotation"):
		return "se.aggregate = 'quotation'"
	case strings.Contains(kind, "purchase-order") || strings.Contains(kind, "purchase_order") || strings.Contains(kind, "order"):
		return "se.aggregate = 'purchase_order'"
	case strings.Contains(kind, "purchase") || strings.Contains(kind, "receiving"):
		return "se.aggregate = 'receiving'"
	case strings.Contains(kind, "adjustment") || strings.Contains(kind, "stock"):
		return "se.aggregate = 'inventory'"
	case strings.Contains(kind, "sale") || strings.Contains(kind, "invoice"):
		return "se.aggregate = 'sale'"
	default:
		return "se.aggregate IN ('sale', 'sale_return', 'refused_sale', 'receiving', 'return', 'purchase_order', 'quotation', 'inventory')"
	}
}

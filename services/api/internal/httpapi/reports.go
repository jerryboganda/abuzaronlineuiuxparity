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
	DocumentID           string `json:"documentId,omitempty"`
	Document             string `json:"document"`
	OccurredAt           string `json:"occurredAt"`
	Party                string `json:"party"`
	Item                 string `json:"item"`
	Quantity             string `json:"quantity"`
	Amount               string `json:"amount"`
	Alias                string `json:"alias,omitempty"`
	ItemDescription      string `json:"itemDescription,omitempty"`
	SalePrice            string `json:"salePrice,omitempty"`
	DiscountPercent      string `json:"discountPercent,omitempty"`
	DiscountValue        string `json:"discountValue,omitempty"`
	ItemDiscount         string `json:"itemDiscount,omitempty"`
	SalesTaxValue        string `json:"salesTaxValue,omitempty"`
	PurchasePrice        string `json:"purchasePrice,omitempty"`
	AveragePrice         string `json:"averagePrice,omitempty"`
	RecentPurchasePrice  string `json:"recentPurchasePrice,omitempty"`
	PackUnits            string `json:"packUnits,omitempty"`
	DocumentType         string `json:"documentType,omitempty"`
	AlternateAccountCode string `json:"alternateAccountCode,omitempty"`
	UserLegacyID         string `json:"userLegacyId,omitempty"`
	InvoiceCode          string `json:"invoiceCode,omitempty"`
	ExpiryDate           string `json:"expiryDate,omitempty"`
	BatchNumber          string `json:"batchNumber,omitempty"`
	GrossProfit          string `json:"grossProfit,omitempty"`
	MarginPercent        string `json:"marginPercent,omitempty"`
	ReorderQuantity      string `json:"reorderQuantity,omitempty"`
	OptimumQuantity      string `json:"optimumQuantity,omitempty"`
	MinimumQuantity      string `json:"minimumQuantity,omitempty"`
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
	SelectedFormat   string             `json:"selectedFormat,omitempty"`
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
	documentReadModel  bool
	documentKind       string
	documentAggregate  string
	documentMode       string
	headerReadModel    bool
	stockReadModel     bool
	stockMode          string
	financeMode        string
	adminKind          string
	compatibilityOnly  bool
	reprintMode        string
	historyMode        string
	emptyProjection    bool
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
			case "sale-detail":
				mode = "line-detail"
			}
			registry[report.kind] = reportSpec{title: report.title, aggregateCondition: condition, salesReadModel: true, salesMode: mode}
		} else if report.kind == "refused-sales-detail" {
			registry[report.kind] = reportSpec{
				title:              report.title,
				aggregateCondition: condition,
				documentReadModel:  true,
				documentKind:       "refused-sale",
				documentAggregate:  "refused_sale",
				documentMode:       "line-detail",
			}
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
		if report.kind == "sales-return-detail" {
			registry[report.kind] = reportSpec{
				title:              report.title,
				aggregateCondition: reportSaleReturnAggregate,
				salesReadModel:     true,
				salesMode:          "line-detail",
			}
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
		mode := ""
		switch report.kind {
		case "customer-sales-days-summary":
			mode = "day-summary"
		case "customer-sales-items-summary":
			mode = "item-summary"
		case "customer-sales-invoice-summary":
			mode = "invoice-summary"
		case "customer-sales-invoice-wise-profit-margin-detail":
			mode = "profit-margin-detail"
		case "customer-sales-hourly-graph":
			mode = "hour-summary"
		case "customer-sales-customer-category-wise-sales-customer-wise-gross-profit":
			mode = "profit-customer-summary"
		case "customer-sales-customer-category-wise-sales-customer-wise-summary",
			"customer-sales-customer-category-wise-sales-net-sales-and-volume":
			mode = "customer-summary"
		case "customer-sales-customer-category-wise-net-sales",
			"customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report",
			"customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report":
			mode = "customer-category-summary"
		case "customer-sales-customer-wise-category-net-sales":
			mode = "customer-wise-category-summary"
		case "customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report":
			mode = "line-detail"
		case "customer-sales-monthly-net-sales", "monthly-net-sales-summary":
			mode = "month-summary"
		case "daily-sales-summary-with-profit-day-wise-grouping":
			mode = "profit-day-summary"
		}
		registry[report.kind] = reportSpec{title: report.title, aggregateCondition: condition, salesReadModel: true, salesMode: mode}
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
		{"purchase-detail", "Purchase detail", "se.aggregate = 'receiving'", "line-detail"},
		{"purchase-summary", "Purchase summary", "se.aggregate = 'receiving'", "invoice-summary"},
		{"purchase-summary2", "Purchase Summary2", "se.aggregate = 'receiving'", "invoice-summary"},
		{"purchase-return-detail", "Purchase Return detail", "se.aggregate = 'return'", "line-detail"},
		{"purchase-return-summary", "Purchase Return summary", "se.aggregate = 'return'", "invoice-summary"},
		{"purchase-order-summary", "Purchase Order Summary", "se.aggregate = 'purchase_order'", "invoice-summary"},
		{"p-o-based-purchase-disparity", "P/O Based Purchase Disparity", "se.aggregate = 'purchase_order'", "detail"},
		{"periodic-purchases", "Periodic Purchases", "se.aggregate = 'receiving'", "month-summary"},
		{"purchase-order", "Purchase Order", "se.aggregate = 'purchase_order'", "invoice-summary"},
		{"supplier-wise-detail", "Detail", "se.aggregate = 'receiving'", "detail"},
		{"supplier-wise-purchase-detail", "Purchase Detail", "se.aggregate = 'receiving'", "detail"},
		{"supplier-wise-advance-income-tax", "Advance Income Tax", "se.aggregate = 'receiving'", "summary"},
		{"manufacturer-wise-detail", "Detail", "se.aggregate = 'receiving'", "detail"},
		{"manufacturer-wise-monthly-stock-movement", "Monthly Stock Movement", "se.aggregate = 'receiving'", "month-summary"},
		{"monthly-purchase-graph", "Monthly Purchase Graph", "se.aggregate = 'receiving'", "month-summary"},
		{"category-wise-purchase", "Category Wise Purchase", "se.aggregate = 'receiving'", "item-summary"},
		{"days-summary", "Days Summary", "se.aggregate = 'receiving'", "day-summary"},
		{"purchase-order-supplier-wise", "Purchase Order Supplier Wise", "se.aggregate = 'purchase_order'", "supplier-summary"},
		{"net-purchase-summary", "Net Purchase Summary", "se.aggregate = 'receiving'", "invoice-summary"},
		{"supplier-category-wise-input-sales-tax-report", "Input Sales Tax Report", "se.aggregate = 'receiving'", "invoice-summary"},
		{"withholding-tax-deduction", "Withholding Tax Deduction", "se.aggregate = 'receiving'", "invoice-summary"},
		{"supplier-manufacturer-wise-g-p", "Supplier/Manufacturer Wise G/P", "se.aggregate = 'receiving'", "supplier-summary"},
		{"supplier-purchase-returns-detail", "Detail", "se.aggregate = 'return'", "detail"},
		{"supplier-purchase-returns-summary", "Summary", "se.aggregate = 'return'", "supplier-summary"},
	} {
		add(report.kind, report.title, report.aggregate, report.mode)
	}
	return registry
}()

// phasePReportRegistry contains the captured Stock Reports leaves. Current
// stock/balance leaves read normalized stock_ledger/stock_balances rows;
// Back Date has a separate source-backed StockReport snapshot projection so
// its date argument means the captured historical observation date.
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
		{"stock-in-hand-manufacturer-wise", "Manufacturer wise", "manufacturer-balance"},
		{"stock-in-hand-category-wise", "Category wise", "category-balance"},
		{"stock-in-hand-others", "Others", "balance"},
		{"stock-in-hand-class-wise", "Class Wise", "class-balance"},
		{"stock-in-hand-batch-priority-wise", "Batch, Priority Wise", "balance"},
		{"stock-in-hand-back-date", "Back Date", "historical-balance"},
		{"stock-in-hand-manufacturer-wise-format2", "Manufacturer Wise (Format2)", "manufacturer-balance"},
		{"stock-in-hand-supplier-manufacturer-association", "Supplier Manufacturer Association", "supplier-manufacturer"},
		{"stock-in-hand-stock-quantity-format", "Stock Quantity Format", "balance"},
		{"stock-in-hand-stock-in-hand-audit-purpose", "Stock in Hand - Audit Purpose", "balance"},
		{"stock-in-hand-batch-priority-wise-audit-purposes", "Batch, Priority Wise - Audit Purposes", "balance"},
		{"expiry-report", "Expiry Report", "expiry"},
		{"reorder-level-report", "Reorder Level Report", "reorder-level"},
		{"stock-register", "Stock Register", "movement"},
		{"item-stock-register-summary", "Item Stock Register Summary", "item-summary"},
		{"stock-register-for-narcotics", "Stock Register(For Narcotics)", "narcotics-movement"},
		{"stock-and-sales", "Stock and Sales", "stock-sales"},
		{"optimum-level-report", "Optimum Level Report", "optimum-level"},
		{"item-activity", "Item Activity", "movement"},
		{"stock-register-narcotics-format2", "Stock Register(Narcotics Format2)", "narcotics-movement"},
		{"expiry-report-class-wise", "Expiry Report(Class Wise)", "expiry-class"},
		{"minimum-level-report", "Minimum Level Report", "minimum-level"},
		{"reorder-optimum-level-report", "Reorder/Optimum Level Report", "reorder-optimum-level"},
		{"daily-stock-in-out", "Daily Stock IN/OUT", "movement-summary"},
		{"stock-in-out-date-wise", "Stock IN/OUT(Date Wise)", "movement-summary"},
		{"stock-management-report", "Stock Management Report", "management-summary"},
		{"norcotics-stock-register-generic-type-wise", "Norcotics Stock Register-Generic Type Wise", "narcotics-generic"},
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
		if strings.HasPrefix(report.kind, "quotation-") {
			mode := "line-detail"
			if report.kind == "quotation-summary" {
				mode = "invoice-summary"
			}
			add(report.kind, report.title, reportSpec{
				aggregateCondition: condition,
				documentReadModel:  true,
				documentKind:       "quotation",
				documentAggregate:  "quotation",
				documentMode:       mode,
			})
		} else if report.kind == "header-wise-transaction-summary" {
			add(report.kind, report.title, reportSpec{
				aggregateCondition: condition,
				headerReadModel:    true,
			})
		} else {
			add(report.kind, report.title, reportSpec{aggregateCondition: condition, compatibilityOnly: true})
		}
	}
	add("accounts-reports-ledger-reports-accounts-ledger", "Accounts Ledger", reportSpec{
		financeMode: "gl",
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
		spec := reportSpec{
			compatibilityOnly:  true,
			aggregateCondition: reportSaleAggregate,
			reprintMode:        "sale-detail",
			salesMode:          "line-detail",
		}
		if report.kind == "reprinting-sale-with-summary-reports" ||
			report.kind == "reprinting-sale-with-header-wise-summaries" ||
			report.kind == "reprinting-selected-sales-and-summaries" {
			spec.reprintMode = "sale-summary"
			spec.salesMode = "invoice-summary"
		}
		if report.mode == "sales" {
			spec.salesReadModel = true
		} else {
			spec.reprintMode = "purchase-detail"
			spec.salesMode = ""
			spec.purchaseReadModel = true
			spec.purchaseMode = "line-detail"
			spec.aggregateCondition = "se.aggregate = 'receiving'"
		}
		add(report.kind, report.title, spec)
	}
	add("item-reports-deleted-sale-items-log", "Deleted Sale Items Log", reportSpec{
		historyMode: "deleted-sale-items",
	})
	for _, report := range []struct {
		kind  string
		title string
		mode  string
	}{
		{"item-reports-history-sale-price-difference", "Sale Price Difference", "item-price-difference"},
		{"item-reports-history-item-basic-data-changes", "Item Basic Data Changes", "item-basic-data"},
		{"item-reports-history-item-sale-price-changes", "Item Sale Price Changes", "item-sale-price"},
		{"item-reports-history-new-item-s-created-defined", "New Item(s) Created/Defined", "item-first-observed"},
		{"item-reports-history-item-name-changes", "Item Name Changes", "item-name"},
		{"item-reports-stock-adjustments-stock-adjustments-detail", "Stock Adjustments Detail", "stock-adjustments"},
	} {
		add(report.kind, report.title, reportSpec{historyMode: report.mode})
	}
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
	"supplier-wise-advance-income-tax":              "tax-advance-input",
	"supplier-category-wise-input-sales-tax-report": "tax-input",
	"withholding-tax-deduction":                     "tax-withholding",
	"user-wise-net-cash":                            "gl-cash",
}

// The report route also accepts stable direct links for the financial views.
// They are aliases, not additional legacy leaves, and make the normalized
// projections available to the report client without inventing menu entries.
var phaseQReportAliases = map[string]reportSpec{
	"gl-journal":         {title: "GL Journal", financeMode: "historical-gl"},
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

func documentReportColumns(mode string) []reportColumn {
	if mode == "invoice-summary" {
		return []reportColumn{
			{Key: "document", Label: "Document", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	}
	return reportColumns()
}

func headerReportColumns() []reportColumn {
	return []reportColumn{
		{Key: "document", Label: "Document", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
		{Key: "party", Label: "Customer/Supplier", DataType: "text", Sortable: true},
		{Key: "item", Label: "Transaction Type", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
		{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
	}
}

func dailySaleDetailColumns() []reportColumn {
	return []reportColumn{
		{Key: "alias", Label: "Alias", DataType: "text", Sortable: true},
		{Key: "itemDescription", Label: "Item Description", DataType: "text", Sortable: true},
		{Key: "salePrice", Label: "Sale Price", DataType: "currency", Sortable: true},
		{Key: "quantity", Label: "Qty", DataType: "number", Sortable: true},
		{Key: "discountPercent", Label: "Disc(%)", DataType: "number", Sortable: true},
		{Key: "discountValue", Label: "Discount Value", DataType: "currency", Sortable: true},
		{Key: "itemDiscount", Label: "Item Disc", DataType: "currency", Sortable: true},
		{Key: "salesTaxValue", Label: "SalesTax Value", DataType: "currency", Sortable: true},
		{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		{Key: "expiryDate", Label: "Expiry Date", DataType: "date", Sortable: true},
		{Key: "batchNumber", Label: "Batch Number", DataType: "text", Sortable: true},
	}
}

func purchaseLineDetailColumns() []reportColumn {
	return []reportColumn{
		{Key: "document", Label: "Document", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
		{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
		{Key: "item", Label: "Item", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
		{Key: "purchasePrice", Label: "Purchase Price", DataType: "currency", Sortable: true},
		{Key: "discountPercent", Label: "Disc(%)", DataType: "number", Sortable: true},
		{Key: "discountValue", Label: "Discount Value", DataType: "currency", Sortable: true},
		{Key: "salesTaxValue", Label: "Sales Tax", DataType: "currency", Sortable: true},
		{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		{Key: "expiryDate", Label: "Expiry Date", DataType: "date", Sortable: true},
		{Key: "batchNumber", Label: "Batch Number", DataType: "text", Sortable: true},
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

func salesInvoiceSummaryReportColumns() []reportColumn {
	return []reportColumn{
		{Key: "document", Label: "Invoice", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
		{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
		{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
		{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
	}
}

func salesSummaryReportColumns(mode string) []reportColumn {
	switch mode {
	case "profit-margin-detail":
		return []reportColumn{
			{Key: "document", Label: "Invoice", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "salePrice", Label: "Sale Price", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Sales Amount", DataType: "currency", Sortable: true},
			{Key: "purchasePrice", Label: "Cost", DataType: "currency", Sortable: true},
			{Key: "grossProfit", Label: "Gross Profit", DataType: "currency", Sortable: true},
			{Key: "marginPercent", Label: "Margin %", DataType: "number", Sortable: true},
			{Key: "salesTaxValue", Label: "Sales Tax", DataType: "currency", Sortable: true},
		}
	case "profit-day-summary":
		return []reportColumn{
			{Key: "document", Label: "Day", DataType: "date", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "salePrice", Label: "Average Sale Price", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Sales Amount", DataType: "currency", Sortable: true},
			{Key: "purchasePrice", Label: "Cost", DataType: "currency", Sortable: true},
			{Key: "grossProfit", Label: "Gross Profit", DataType: "currency", Sortable: true},
			{Key: "marginPercent", Label: "Margin %", DataType: "number", Sortable: true},
			{Key: "salesTaxValue", Label: "Sales Tax", DataType: "currency", Sortable: true},
		}
	case "profit-customer-summary":
		return []reportColumn{
			{Key: "document", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "salePrice", Label: "Average Sale Price", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Sales Amount", DataType: "currency", Sortable: true},
			{Key: "purchasePrice", Label: "Cost", DataType: "currency", Sortable: true},
			{Key: "grossProfit", Label: "Gross Profit", DataType: "currency", Sortable: true},
			{Key: "marginPercent", Label: "Margin %", DataType: "number", Sortable: true},
			{Key: "salesTaxValue", Label: "Sales Tax", DataType: "currency", Sortable: true},
		}
	case "customer-summary":
		return []reportColumn{
			{Key: "document", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Volume", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Net Sales", DataType: "currency", Sortable: true},
		}
	case "customer-category-summary":
		return []reportColumn{
			{Key: "document", Label: "Customer Category", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer Category", DataType: "text", Sortable: true},
			{Key: "item", Label: "Category Total", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Volume", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Net Sales", DataType: "currency", Sortable: true},
		}
	case "customer-wise-category-summary":
		return []reportColumn{
			{Key: "document", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Category", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Volume", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Net Sales", DataType: "currency", Sortable: true},
		}
	case "day-summary":
		return []reportColumn{
			{Key: "document", Label: "Day", DataType: "date", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "hour-summary":
		return []reportColumn{
			{Key: "document", Label: "Hour", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "item-summary":
		return []reportColumn{
			{Key: "document", Label: "Item", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "month-summary":
		return []reportColumn{
			{Key: "document", Label: "Month", DataType: "date", Sortable: true},
			{Key: "occurredAt", Label: "Month", DataType: "date", Sortable: true},
			{Key: "party", Label: "Customer", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	default:
		return salesInvoiceSummaryReportColumns()
	}
}

func historicalReportColumns(mode string) []reportColumn {
	if mode == "deleted-sale-items" {
		return []reportColumn{
			{Key: "document", Label: "Sale Invoice", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Deleted At", DataType: "date", Sortable: true},
			{Key: "party", Label: "Machine / User", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item / Godown", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Qty + Bonus", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Sale Price", DataType: "currency", Sortable: true},
		}
	}
	if mode == "stock-adjustments" {
		return []reportColumn{
			{Key: "document", Label: "Adjustment", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown / User", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item / Batch", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Loose Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Adjustment Price", DataType: "currency", Sortable: true},
		}
	}
	return []reportColumn{
		{Key: "document", Label: "Source Row", DataType: "text", Sortable: true},
		{Key: "occurredAt", Label: "Observed", DataType: "date", Sortable: true},
		{Key: "party", Label: "User / Reason", DataType: "text", Sortable: true},
		{Key: "item", Label: "Item", DataType: "text", Sortable: true},
		{Key: "quantity", Label: "Previous", DataType: "text", Sortable: true},
		{Key: "amount", Label: "Current", DataType: "text", Sortable: true},
	}
}

func historicalProjectionNote(mode string) string {
	if mode == "deleted-sale-items" {
		return "source-backed DeletedSaleItem rows retain the captured sale invoice, item, godown, quantity/bonus, sale price, discount/tax fields, machine, user, and raw source payload under tenant and branch scope. Exact PowerBuilder deleted-item columns, deletion-order semantics, and print layout remain unverified."
	}
	if mode == "stock-adjustments" {
		return "source-backed AdjHeader/AdjDetail rows retain the legacy payload, header status, item, godown, batch, and pricing fields, and posted normalized stock_ledger adjustments are included from the immutable inventory event. Exact PowerBuilder adjustment grouping, calculated columns, and print layout remain unverified."
	}
	return "Normalized source-backed ItemLog snapshots retain source identity, payload, prior-observed values, and change reason. The New Item report uses first-observed snapshots rather than asserting creation semantics; exact PowerBuilder columns, filtering, and print layout remain unverified."
}

func isStockLevelMode(mode string) bool {
	switch mode {
	case "reorder-level", "optimum-level", "minimum-level", "reorder-optimum-level":
		return true
	default:
		return false
	}
}

func isStockManagementMode(mode string) bool {
	return mode == "management-summary"
}

func isStockItemSummaryMode(mode string) bool {
	return mode == "item-summary"
}

func isStockMovementSummaryMode(mode string) bool {
	return mode == "movement-summary"
}

func isStockSupplierManufacturerMode(mode string) bool {
	return mode == "supplier-manufacturer"
}

func isStockSalesMode(mode string) bool {
	return mode == "stock-sales"
}

func isStockNarcoticsMode(mode string) bool {
	return mode == "narcotics-movement" || mode == "narcotics-generic"
}

func isStockExpiryClassMode(mode string) bool {
	return mode == "expiry-class"
}

func isStockClassificationMode(mode string) bool {
	switch mode {
	case "manufacturer-balance", "category-balance", "class-balance":
		return true
	default:
		return false
	}
}

func stockClassificationLabel(mode string) string {
	switch mode {
	case "manufacturer-balance":
		return "Manufacturer"
	case "category-balance":
		return "Category"
	case "class-balance":
		return "Class"
	default:
		return "Group"
	}
}

func stockReportColumns(mode string) []reportColumn {
	if mode == "historical-balance" {
		return []reportColumn{
			{Key: "document", Label: "Source Row", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "As Of", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Stock", DataType: "number", Sortable: true},
			{Key: "purchasePrice", Label: "Purchase Price", DataType: "currency", Sortable: true},
			{Key: "salePrice", Label: "Sale Price", DataType: "currency", Sortable: true},
			{Key: "averagePrice", Label: "Average Price", DataType: "currency", Sortable: true},
			{Key: "recentPurchasePrice", Label: "Recent Purchase Price", DataType: "currency", Sortable: true},
			{Key: "packUnits", Label: "Pack Units", DataType: "number", Sortable: true},
		}
	}
	if isStockLevelMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Batch", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Expiry/Updated", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "reorderQuantity", Label: "Reorder Qty", DataType: "number", Sortable: true},
			{Key: "optimumQuantity", Label: "Optimum Qty", DataType: "number", Sortable: true},
			{Key: "minimumQuantity", Label: "Minimum Qty", DataType: "number", Sortable: true},
		}
	}
	if isStockManagementMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Batch", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Expiry/Updated", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "reorderQuantity", Label: "Reorder Qty", DataType: "number", Sortable: true},
			{Key: "optimumQuantity", Label: "Optimum Qty", DataType: "number", Sortable: true},
			{Key: "minimumQuantity", Label: "Minimum Qty", DataType: "number", Sortable: true},
		}
	}
	if isStockItemSummaryMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Item Code", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Net Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Net Value", DataType: "currency", Sortable: true},
		}
	}
	if isStockSalesMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Batch", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Expiry/Updated", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Sales Qty", DataType: "number", Sortable: true},
		}
	}
	if isStockExpiryClassMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Class", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Expiry Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Unit Cost", DataType: "currency", Sortable: true},
		}
	}
	if isStockClassificationMode(mode) {
		return []reportColumn{
			{Key: "document", Label: stockClassificationLabel(mode), DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Expiry/Updated", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Unit Cost", DataType: "currency", Sortable: true},
		}
	}
	if mode == "narcotics-generic" {
		return []reportColumn{
			{Key: "document", Label: "Generic Type", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Net Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Net Value", DataType: "currency", Sortable: true},
		}
	}
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
	if isStockMovementSummaryMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Date", DataType: "date", Sortable: true},
			{Key: "occurredAt", Label: "Direction", DataType: "text", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Net Value", DataType: "currency", Sortable: true},
		}
	}
	if isStockSupplierManufacturerMode(mode) {
		return []reportColumn{
			{Key: "document", Label: "Manufacturer", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Supplier(s)", DataType: "text", Sortable: true},
			{Key: "party", Label: "Godown", DataType: "text", Sortable: true},
			{Key: "item", Label: "Item", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "On Hand", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Unit Cost", DataType: "currency", Sortable: true},
		}
	}
	if mode == "movement" || mode == "narcotics-movement" {
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
	case "historical-gl":
		return []reportColumn{
			{Key: "document", Label: "Document Code", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "documentType", Label: "Document Type", DataType: "text", Sortable: true},
			{Key: "party", Label: "Account Code", DataType: "text", Sortable: true},
			{Key: "alternateAccountCode", Label: "Alternate Account", DataType: "text", Sortable: true},
			{Key: "invoiceCode", Label: "Invoice Code", DataType: "text", Sortable: true},
			{Key: "userLegacyId", Label: "User Code", DataType: "text", Sortable: true},
			{Key: "item", Label: "Remarks", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Debit", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Credit", DataType: "currency", Sortable: true},
		}
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
	case "tax-withholding":
		return []reportColumn{
			{Key: "document", Label: "Payment", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Purchase Invoice / Certificate", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Withholding Base", DataType: "currency", Sortable: true},
			{Key: "amount", Label: "Withholding Amount", DataType: "currency", Sortable: true},
		}
	case "tax-output", "tax-input", "tax-advance", "tax-advance-input":
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
	case "historical-gl":
		return "Imported historical_gl_entries from dbo.VirtualGl expose document code/type, account and alternate account codes, invoice/user identifiers, remarks, and debit/credit source values. Newly posted normalized gl_journals are included as explicitly labeled canonical rows; exact legacy account naming, opening balances, and print layout remain unverified."
	case "gl":
		return "Posted-only gl_journals/gl_lines are joined to the tenant chart of accounts and unioned with imported historical_gl_entries from dbo.VirtualGl; historical account labels remain explicit and exact opening-balance/print semantics remain unverified."
	case "gl-cash":
		return "Posted-only cash-account gl_journals and gl_lines joined to the tenant chart of accounts; historical VirtualGl rows are not filtered as cash without a reviewed account mapping."
	case "trial-balance":
		return "Posted-only gl_journals/gl_lines and imported historical_gl_entries are aggregated by account and scoped to tenant, branch, and date; unmatched legacy accounts remain explicitly labeled Historical and opening-balance semantics remain unverified."
	case "party-customer":
		return "Posted-only customer party_ledger_entries, including posted historical documents without a canonical GL journal, plus source-backed historical_party_payment_allocations, historical_party_ledger_adjustments, and historical_party_return_allocations from SaleLedger, InstallmentReceiptDetail, SaleReceivableAdj, and SRAllocationDetail; unresolved legacy identities remain visible and exact legacy statement/print semantics remain unverified."
	case "party-supplier":
		return "Posted-only supplier party_ledger_entries, including posted historical documents without a canonical GL journal, plus source-backed historical_party_payment_allocations and historical_party_return_allocations from PurPayment, Purledger snapshots, and PRAllocationDetail; unresolved legacy identities remain visible and exact legacy statement/print semantics remain unverified."
	case "aging-receivable":
		return "Posted-only customer party-ledger entries use retained SaleLedger DueDate payloads and business-document open balances when available to expose bounded aging buckets; missing due dates remain explicitly unaged and individual payment allocation or exact legacy bucket semantics are not fabricated."
	case "aging-payable":
		return "Posted-only supplier party-ledger entries use retained Purledger CreditDays payloads and business-document open balances for bounded purchase due dates; purchase returns and rows without valid terms remain explicitly unaged, and individual payment allocation or exact legacy bucket semantics are not fabricated."
	case "tax-output":
		return "Posted business-document line tax snapshots (tax_amount, rates, and taxable line totals) for sales; values are not recomputed from current configuration."
	case "tax-input":
		return "Posted business-document line tax snapshots (tax_amount, rates, and taxable line totals) for purchases; values are not recomputed from current configuration."
	case "tax-advance", "tax-advance-input":
		return "Posted line snapshots use explicit advance-tax rate/base/amount evidence; numeric legacy AdvanceTaxAmt is used only as a guarded fallback, and rows without amount evidence are omitted."
	case "tax-withholding":
		return "Imported dbo.PurPayment rows expose posted payment-level withholding base, rate, amount, account, check/reference, supplier-invoice identity, and remarks. The report does not reinterpret purchase-line advance tax; exact legacy grouping and print output remain unverified."
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
	case "historical-balance":
		return "Source-backed historical_stock_snapshots imported from dbo.StockReport expose the captured as-of stock, purchase price, sale price, average price, recent purchase price, and pack-unit fields. Manufacturer/category/class/narcotics grouping and exact print calculations remain unverified."
	case "movement":
		return "Normalized posted stock_ledger movement projection joined to batch, item, and godown metadata; compatibility inventory_movements and unreconciled legacy grouping are not included."
	case "management-summary":
		return "Normalized stock_balances expose batch, godown, item, on-hand, and item-payload reorder/optimum/minimum thresholds with posted-ledger gating and without an alert predicate. This is a bounded Stock Management projection; exact PowerBuilder alert status, valuation, grouping, and print calculations remain unverified."
	case "expiry":
		return "Normalized posted stock_balances grouped by stock batch expiry; expired and future batches are shown from typed expiry_date, while legacy class/group calculations are not implemented."
	case "valuation":
		return "Normalized valuation is on_hand multiplied by stock_batches.unit_cost; legacy FIFO/average valuation and exact historical valuation are not reconciled."
	case "item-summary":
		return "Normalized posted stock_ledger rows are grouped by item, godown, and calendar day; net quantity applies direction and signed adjustments, and net value is the signed quantity multiplied by posted unit cost. Legacy opening balances, grouping labels, and exact report calculations remain unverified."
	case "movement-summary":
		return "Normalized posted stock_ledger rows are grouped by calendar day, IN/OUT/ADJUSTMENT direction, godown, and item. Quantities and values are signed from the posted ledger; opening balances, legacy date-wise grouping, and exact print calculations remain unverified."
	case "supplier-manufacturer":
		return "Normalized posted stock_balances join the captured Item Manufacturer payload and tenant-scoped item_suppliers supplier links. Supplier names are aggregated per item to avoid duplicating batch balances; exact legacy association, priority selection, valuation, and print calculations remain unverified."
	case "stock-sales":
		return "Normalized stock balances are joined to canonical posted cash/credit sale allocations for the requested period; On Hand is the current balance cache and Sales Qty is the allocated sale quantity. Compatibility-only sales, returns, opening balances, and exact Stock-and-Sales calculations remain unverified."
	case "narcotics-movement":
		return "Normalized posted stock_ledger movements are filtered by the captured Item master Narcotics flag in master_items.payload; batch, godown, direction, signed quantity, and unit cost remain explicit. Exact narcotics report formatting and legacy payload-key semantics remain unverified."
	case "narcotics-generic":
		return "Normalized posted stock_ledger movements are filtered by the captured Item master Narcotics flag and grouped by the captured GenericName/GenericCode payload, day, godown, and item. Exact generic-type grouping, return/opening treatment, and print calculations remain unverified."
	case "expiry-class":
		return "Normalized posted stock_balances use typed stock_batches.expiry_date and the captured Item Class payload for the class-wise expiry projection; zero-stock rows and batches without a typed expiry are excluded. Exact class code joins, date-window semantics, and print calculations remain unverified."
	case "manufacturer-balance", "category-balance", "class-balance":
		return "Normalized posted stock_balances are grouped by the captured Item " + stockClassificationLabel(mode) + " payload with posted-ledger gating, current on-hand, batch expiry/update, godown, item, and unit-cost fields. Exact legacy group joins, valuation, and print calculations remain unverified."
	case "reorder-level", "optimum-level", "minimum-level", "reorder-optimum-level":
		return "Normalized stock_balances expose on-hand plus item payload reorder/optimum/minimum thresholds, using ReorderQty/OptimumQty/MinimumQty with maintenance-field fallbacks. The below-threshold predicate, zero-stock inclusion, date semantics, and exact PowerBuilder calculations remain unverified."
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
	switch mode {
	case "invoice-summary":
		return "canonical and compatibility rows are grouped once per invoice with summed quantity and authoritative document amount; legacy tax, profit, and format-specific columns remain open"
	case "day-summary":
		return "canonical and compatibility invoice rows are grouped by calendar day and customer after invoice de-duplication; legacy day-summary calculated columns and return/net semantics remain open"
	case "item-summary":
		return "canonical and compatibility sale lines are grouped by item and customer with numeric quantity and amount totals; legacy item grouping, return treatment, and calculated columns remain open"
	case "month-summary":
		return "canonical and compatibility invoice rows are grouped by calendar month and customer after invoice de-duplication; legacy net-sales, return, tax, and calculated columns remain open"
	case "hour-summary":
		return "canonical and compatibility invoice rows are grouped by calendar hour and customer after invoice de-duplication; graph rendering, hourly pricing/tax treatment, and legacy calculated columns remain open"
	case "profit-margin-detail":
		return "canonical posted sale lines use stock allocation cost when available and compatibility rows use an explicitly supplied legacy cost; gross profit is sales amount less tax and cost, while exact PowerBuilder valuation, returns, discounts, and margin rules remain open"
	case "profit-day-summary":
		return "canonical and compatibility profit rows are grouped by calendar day and customer; average sale price, sales amount, tax, cost, gross profit, and margin are emitted only within the bounded numeric source contract, while exact PowerBuilder day grouping and calculations remain open"
	case "profit-customer-summary":
		return "canonical and compatibility profit rows are grouped by customer; average sale price, sales amount, tax, cost, gross profit, and margin are emitted only when every contributing row has numeric cost, while exact PowerBuilder customer grouping and calculations remain open"
	case "customer-summary":
		return "canonical and compatibility invoice rows are de-duplicated and grouped by customer with summed quantity and authoritative document amount; exact PowerBuilder customer/category grouping, net/return semantics, and calculated columns remain open"
	case "customer-category-summary":
		return "canonical and compatibility posted sale rows are grouped by the retained customer category payload with numeric quantity and line amount totals; rows without a source category use Unspecified, while exact PowerBuilder category joins, net/return semantics, and calculated columns remain open"
	case "customer-wise-category-summary":
		return "canonical and compatibility posted sale rows are grouped by retained customer and customer category payload with numeric quantity and line amount totals; rows without a source category use Unspecified, while exact PowerBuilder joins, net/return semantics, and calculated columns remain open"
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
	if spec.documentReadModel {
		formatNames = []string{"Standard"}
	}
	if spec.headerReadModel {
		formatNames = []string{"Standard"}
	}
	if spec.stockReadModel {
		formatNames = []string{"Standard"}
	}
	if spec.financeMode != "" || spec.adminKind != "" {
		formatNames = []string{"Standard"}
	}
	if spec.historyMode != "" {
		formatNames = []string{"Standard"}
	}
	if spec.reprintMode != "" {
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
		if spec.financeMode == "historical-gl" {
			retrievalScope = "tenant, branch, date, text, imported historical_gl_entries, and newly posted gl_journals"
		} else if spec.financeMode == "gl" {
			retrievalScope = "tenant, branch, posted-only, date, text, imported historical_gl_entries, and newly posted gl_journals"
		} else if spec.financeMode == "tax-withholding" {
			retrievalScope = "tenant, branch, posted-only, date, text, and imported historical_withholding_tax_entries from dbo.PurPayment"
		} else if spec.financeMode == "aging-receivable" {
			retrievalScope = "tenant, branch, posted-only, date, text, customer party-ledger entries, and retained SaleLedger DueDate payloads"
		} else if spec.financeMode == "party-customer" {
			retrievalScope = "tenant, branch, posted-only, date, text, party_ledger_entries, SaleLedger payment snapshots, InstallmentReceiptDetail allocations, SaleReceivableAdj adjustments, and SRAllocationDetail return allocations"
		} else if spec.financeMode == "party-supplier" {
			retrievalScope = "tenant, branch, posted-only, date, text, party_ledger_entries, PurPayment allocations, Purledger payment snapshots, and PRAllocationDetail return allocations"
		} else {
			retrievalScope = "tenant, branch, posted-only, date, text, and normalized " + spec.financeMode + " projection"
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
			if spec.salesMode == "line-detail" {
				columns = dailySaleDetailColumns()
				projectionNote = "Canonical posted sale/return business_documents and lines expose a source-backed line-detail contract with alias, item, price, quantity, discounts, tax, amount, expiry, and batch values; compatibility sale_return events/rows are expanded when no canonical identity matches. Exact legacy grouping, format calculations, and print output remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical sale/return lines, stock allocations, and compatibility sale/return events"
			} else if spec.salesMode == "invoice-summary" || spec.salesMode == "day-summary" ||
				spec.salesMode == "item-summary" || spec.salesMode == "month-summary" ||
				spec.salesMode == "hour-summary" ||
				spec.salesMode == "profit-margin-detail" || spec.salesMode == "profit-day-summary" ||
				spec.salesMode == "profit-customer-summary" || spec.salesMode == "customer-summary" ||
				spec.salesMode == "customer-category-summary" || spec.salesMode == "customer-wise-category-summary" {
				columns = salesSummaryReportColumns(spec.salesMode)
				projectionNote = "Canonical and compatibility sales rows use the explicit " + spec.salesMode + " projection; " + salesProjectionModeNote(spec.salesMode) + "."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical sales documents/lines, and compatibility sale events with " + spec.salesMode + " grouping"
			}
		}
		if spec.documentReadModel {
			projectionStatus = "real"
			columns = documentReportColumns(spec.documentMode)
			if spec.documentMode == "invoice-summary" {
				projectionNote = "Canonical posted " + spec.documentKind + " documents are grouped once per document with de-duplicated posted compatibility " + spec.documentAggregate + " events; exact PowerBuilder summary calculations and print output remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical " + spec.documentKind + " documents/lines, and de-duplicated compatibility " + spec.documentAggregate + " events grouped by document"
			} else {
				projectionNote = "Canonical posted " + spec.documentKind + " document lines are joined to customer metadata and unioned with de-duplicated posted compatibility " + spec.documentAggregate + " rows; exact PowerBuilder detail columns, calculations, and print output remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical " + spec.documentKind + " documents/lines, and de-duplicated compatibility " + spec.documentAggregate + " events"
			}
		}
		if spec.headerReadModel {
			projectionStatus = "real"
			columns = headerReportColumns()
			projectionNote = "Canonical posted business_documents across sales, returns, quotations, refusals, purchases, purchase returns, and purchase orders are grouped once per header; de-duplicated posted compatibility events cover documents not yet canonical. Exact PowerBuilder transaction-type, opening-balance, and print calculations remain unverified."
			retrievalScope = "tenant, branch, posted-only, date, text, canonical business_documents/lines, and de-duplicated compatibility transaction events grouped by header"
		}
		if spec.purchaseReadModel {
			projectionNote = "Canonical posted purchase business_documents/lines with supplier party ledger and stock ledger values when available; posted compatibility events are included only when no canonical document matches. Legacy grouping and unreconciled tax, profit, graph, and disparity calculations are not implemented."
			retrievalScope = "tenant, branch, date, text, supplier, canonical purchase documents/lines, stock ledger, supplier party ledger, and posted compatibility events"
			columns = reportColumns()
			if spec.purchaseMode == "line-detail" {
				projectionNote = "Canonical posted purchase business-document lines expose document, supplier, item, quantity, purchase price, discount, tax, amount, expiry, and batch values; compatibility receiving/return events are expanded when no canonical document identity matches. Exact legacy grouping, tax, profit, purchase-order, and print calculations remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, supplier, canonical purchase lines, stock allocations, and compatibility receiving/return events"
				columns = purchaseLineDetailColumns()
			} else if isPurchaseSummaryMode(spec.purchaseMode) {
				projectionNote = "Canonical and compatibility purchase rows use the explicit " + spec.purchaseMode + " projection; " + purchaseProjectionModeNote(spec.purchaseMode) + "."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical purchase documents/lines, supplier party ledger, and compatibility receiving/return events with " + spec.purchaseMode + " grouping"
				columns = purchaseSummaryReportColumns(spec.purchaseMode)
			}
		}
		if spec.stockReadModel {
			projectionStatus = "real"
			columns = stockReportColumns(spec.stockMode)
			projectionNote = stockProjectionNote(spec.stockMode)
			if spec.stockMode == "historical-balance" {
				retrievalScope = "tenant, branch, as-of date, text, godown, and imported historical_stock_snapshots"
			} else if isStockItemSummaryMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, and item/godown/day aggregation"
			} else if isStockMovementSummaryMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, and day/direction/godown/item aggregation"
			} else if isStockSupplierManufacturerMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, Item Manufacturer payload, and item_suppliers links"
			} else if isStockSalesMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and canonical posted sale allocations"
			} else if isStockLevelMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and item threshold payload"
			} else if isStockManagementMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and item threshold payload without an alert predicate"
			} else if isStockNarcoticsMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, and captured Item Narcotics/GenericName payload"
			} else if isStockExpiryClassMode(spec.stockMode) {
				retrievalScope = "tenant, branch, typed expiry date, text, godown, batch, posted stock_ledger, and captured Item Class payload"
			} else if isStockClassificationMode(spec.stockMode) {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and captured Item " + stockClassificationLabel(spec.stockMode) + " payload"
			} else {
				retrievalScope = "tenant, branch, date, text, godown, batch, posted stock_ledger, and normalized stock_balances"
			}
		}
		if spec.financeMode != "" {
			projectionStatus = "real"
			projectionNote = financeProjectionNote(spec.financeMode)
			columns = financeReportColumns(spec.financeMode)
			if spec.financeMode == "historical-gl" {
				retrievalScope = "tenant, branch, date, text, imported historical_gl_entries, and newly posted gl_journals"
			} else if spec.financeMode == "gl" {
				retrievalScope = "tenant, branch, posted-only, date, text, imported historical_gl_entries, and newly posted gl_journals"
			} else if spec.financeMode == "tax-withholding" {
				retrievalScope = "tenant, branch, posted-only, date, text, and imported historical_withholding_tax_entries from dbo.PurPayment"
			} else if spec.financeMode == "aging-receivable" {
				retrievalScope = "tenant, branch, posted-only, date, text, customer party-ledger entries, and retained SaleLedger DueDate payloads"
			} else if spec.financeMode == "party-customer" {
				retrievalScope = "tenant, branch, posted-only, date, text, party_ledger_entries, SaleLedger payment snapshots, InstallmentReceiptDetail allocations, SaleReceivableAdj adjustments, and SRAllocationDetail return allocations"
			} else if spec.financeMode == "party-supplier" {
				retrievalScope = "tenant, branch, posted-only, date, text, party_ledger_entries, PurPayment allocations, Purledger payment snapshots, and PRAllocationDetail return allocations"
			} else {
				retrievalScope = "tenant, branch, posted-only, date, text, and normalized " + spec.financeMode + " projection"
			}
		}
		if spec.adminKind != "" {
			projectionStatus = "real"
			columns = reportAdminColumns(spec.adminKind)
			projectionNote = "Tenant-scoped administrative projection; branch scope is applied where the source table carries a branch assignment. Legacy-only fields not present in the normalized tables are omitted."
			retrievalScope = "tenant, branch assignment where available, date, text, and normalized administrative records"
		}
		if spec.historyMode != "" {
			projectionStatus = "real"
			columns = historicalReportColumns(spec.historyMode)
			projectionNote = historicalProjectionNote(spec.historyMode)
			retrievalScope = "tenant, branch, date, text, retained source-backed historical rows, and raw source payload"
		}
		if spec.reprintMode != "" {
			projectionStatus = "real"
			switch spec.reprintMode {
			case "sale-summary":
				columns = salesSummaryReportColumns("invoice-summary")
				projectionNote = "Canonical posted sale invoice summaries are available for reprinting, with de-duplicated compatibility sale events retained when no canonical document identity matches. Exact PowerBuilder summary sections, selected-invoice semantics, and print format remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical sale documents/lines, and de-duplicated compatibility sale events grouped by invoice"
			case "purchase-detail":
				columns = purchaseLineDetailColumns()
				projectionNote = "Canonical posted purchase lines are available for reprinting, with de-duplicated compatibility receiving events retained when no canonical document identity matches. Exact PowerBuilder purchase reprint sections, selected-invoice semantics, and print format remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical purchase documents/lines, and de-duplicated compatibility receiving events"
			default:
				columns = dailySaleDetailColumns()
				projectionNote = "Canonical posted sale lines are available for reprinting, with retained legacy line payload and de-duplicated compatibility sale events when no canonical document identity matches. Exact PowerBuilder patient/pack/header sections, selected-invoice semantics, and print format remain unverified."
				retrievalScope = "tenant, branch, posted-only, date, text, canonical sale documents/lines, retained legacy line payload, and de-duplicated compatibility sale events"
			}
		}
		if spec.compatibilityOnly && spec.financeMode == "" && spec.adminKind == "" &&
			!spec.documentReadModel && !spec.headerReadModel &&
			!spec.stockReadModel && !spec.purchaseReadModel && !spec.salesReadModel {
			projectionStatus = "event-ledger"
			projectionNote = "Explicit compatibility fallback over posted sync_events; no normalized canonical projection exists for this legacy leaf, so missing legacy calculations and columns are not inferred."
			retrievalScope = "tenant, branch, posted-only compatibility events, date, and text"
		}
		if spec.emptyProjection {
			projectionStatus = "generic-fallback"
			projectionNote = "Exact legacy item-history label retained; no normalized item-history source or verified legacy columns are available, so no values are fabricated."
			columns = reportEventLedgerColumns()
			retrievalScope = "tenant, branch, date, and text; no normalized item-history source"
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
		columns = dailySaleDetailColumns()
		projectionNote = "Canonical posted cash/credit business_document lines expose the captured Alias, Item Description, Sale Price, Qty, discount, tax, amount, expiry, and batch columns; retained Saledetail payload values are preferred when available. Compatibility sale rows are expanded when no canonical document identity matches. Format-specific patient, pack, and print calculations remain unverified."
		retrievalScope = "tenant, branch, posted-only, date, text, canonical sale lines, retained legacy line payload, stock allocations, and compatibility sale events"
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
			SupportsCashCredit: dailySaleDetail || spec.reprintMode == "sale-detail" || spec.reprintMode == "sale-summary",
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

func selectReportFormat(definition reportDefinition, requested string) (string, bool) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if len(definition.Formats) == 0 {
			return "", false
		}
		return definition.Formats[0].Name, true
	}
	for _, option := range definition.Formats {
		if strings.EqualFold(option.Name, requested) || strings.EqualFold(option.ID, requested) {
			return option.Name, true
		}
	}
	return "", false
}

// salesReadModelQuery returns the shared sale projection used by reports and
// transaction history. Canonical documents are primary; compatibility sales
// projections/events are included only when no canonical document represents
// the same tenant/branch document. This keeps old event rows visible without
// double-counting a document during a mixed rollout.
func salesReadModelQuery(aggregateCondition, pagination string, includeIdentity ...bool) string {
	withIdentity := len(includeIdentity) > 0 && includeIdentity[0]
	selectColumns := "document, occurred_at::text, party, item, quantity, amount"
	if withIdentity {
		selectColumns = "document_id, " + selectColumns
	}
	eventCondition := "se.aggregate = 'sale'"
	canonicalReturnUnion := ""
	if aggregateCondition == reportSaleOrReturn {
		eventCondition = "se.aggregate IN ('sale', 'sale_return')"
		canonicalReturnUnion = `

			UNION ALL

			SELECT bd.id::text AS document_id,
			       bd.document_number,
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
			SELECT bd.id::text AS document_id,
			       bd.document_number AS document,
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

			SELECT NULL::text AS document_id,
			       sd.document_number,
			       sd.occurred_at,
			       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
			       COALESCE(se.payload->>'itemName', se.payload->'rows'->0->>'itemName', ''),
			       COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
			       sd.total_amount::text
			FROM sales_documents sd
			LEFT JOIN sync_events se
			  ON se.tenant_id = sd.tenant_id AND se.branch_id = sd.branch_id
			 AND se.aggregate_id = sd.id AND se.aggregate = 'sale'
			 AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
			WHERE sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid
			  AND sd.status = 'posted'
			  AND NOT EXISTS (
				SELECT 1
				FROM business_documents bd
				WHERE bd.tenant_id = sd.tenant_id AND bd.branch_id = sd.branch_id
				  AND bd.status = 'posted'
				  AND (bd.id = sd.id OR bd.document_number = sd.document_number)
			  )

			UNION ALL

			SELECT NULL::text AS document_id,
			       COALESCE(se.payload->>'documentNumber', se.aggregate_id::text),
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
				  AND bd.status = 'posted'
				  AND (bd.id = se.aggregate_id
				       OR bd.document_number = se.payload->>'documentNumber')
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM sales_documents sd
				WHERE sd.tenant_id = se.tenant_id AND sd.branch_id = se.branch_id
				  AND sd.status = 'posted'
				  AND sd.id = se.aggregate_id
			  )
		)
	SELECT ` + selectColumns + `
	FROM sales_read_model
		WHERE occurred_at >= $3::date
		  AND occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
		       OR party ILIKE '%' || $5 || '%'
		       OR item ILIKE '%' || $5 || '%')
		ORDER BY occurred_at DESC, document, item ` + pagination
}

// documentReadModelQuery covers the canonical no-stock document workflows
// (quotation and refused sale). Canonical posted business documents win by
// identity; compatibility events remain visible only when no canonical row
// represents the same document.
func documentReadModelQuery(documentKind, documentAggregate, mode, pagination string, includeIdentity ...bool) string {
	withIdentity := len(includeIdentity) > 0 && includeIdentity[0]
	documentKind = strings.ReplaceAll(documentKind, "'", "''")
	documentAggregate = strings.ReplaceAll(documentAggregate, "'", "''")
	selectColumns := "document, occurred_at::text, party, item, quantity, amount"
	if withIdentity {
		selectColumns = "document_id, " + selectColumns
	}
	baseQuery := `
	WITH document_read_model AS (
		SELECT bd.id::text AS document_id,
		       bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(mp.name, CASE WHEN bd.customer_id IS NULL THEN 'CASH' ELSE '' END),
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
		  AND bd.kind = '` + documentKind + `'
		  AND bd.status = 'posted'

		UNION ALL

		SELECT NULL::text AS document_id,
		       COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'quantity', se.payload->>'quantity', ''),
		       COALESCE(se.payload->>'totalAmount', payload_row.value->>'amount', payload_row.value->>'lineTotal', se.payload->>'amount', '')
		FROM sync_events se
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND se.aggregate = '` + documentAggregate + `'
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id OR bd.document_number = se.payload->>'documentNumber')
		  )
	)
	SELECT ` + selectColumns + `
	FROM document_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR item ILIKE '%' || $5 || '%')`
	if mode == "invoice-summary" {
		return invoiceSummaryReadModelQuery(baseQuery, pagination)
	}
	return baseQuery + `
	ORDER BY occurred_at DESC, document, item ` + pagination
}

// headerTransactionReadModelQuery provides one row per canonical or
// compatibility transaction header. Canonical lines contribute quantity,
// while the authoritative document total is retained once per header.
func headerTransactionReadModelQuery(pagination string) string {
	return `
	WITH canonical_headers AS (
		SELECT bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(mp.name, CASE WHEN bd.customer_id IS NULL AND bd.supplier_id IS NULL THEN 'CASH' ELSE '' END) AS party,
		       bd.kind AS transaction_type,
		       COALESCE(SUM(bl.quantity), 0)::text AS quantity,
		       bd.total_amount::text AS amount
		FROM business_documents bd
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = bd.tenant_id
		 AND mp.id = COALESCE(bd.customer_id, bd.supplier_id)
		LEFT JOIN business_document_lines bl
		  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
		 AND bl.document_id = bd.id
		WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
		  AND bd.status = 'posted'
		  AND bd.kind IN (
			'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
			'open-cash-return', 'open-credit-return', 'quotation', 'refused-sale',
			'pack-purchase', 'loose-purchase', 'opening-purchase',
			'purchase-return', 'purchase-order'
		  )
		GROUP BY bd.document_number, bd.occurred_at, mp.name,
		         bd.customer_id, bd.supplier_id, bd.kind, bd.total_amount
	), compatibility_headers AS (
		SELECT DISTINCT ON (COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text))
		       COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer',
		                se.payload->>'supplierName', se.payload->>'supplier', se.aggregate),
		       COALESCE(se.payload->>'kind', se.aggregate),
		       COALESCE(se.payload->>'quantity', se.payload->'rows'->0->>'quantity', ''),
		       COALESCE(se.payload->>'totalAmount', se.payload->>'amount', '')
		FROM sync_events se
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND se.aggregate IN (
			'sale', 'sale_return', 'refused_sale', 'receiving', 'return',
			'purchase_order', 'quotation', 'inventory'
		  )
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id OR bd.document_number = se.payload->>'documentNumber')
		  )
		ORDER BY COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
		         se.occurred_at DESC, se.event_id DESC
	), header_read_model AS (
		SELECT document, occurred_at, party, transaction_type, quantity, amount
		FROM canonical_headers
		UNION ALL
		SELECT document, occurred_at, party, transaction_type, quantity, amount
		FROM compatibility_headers
	)
	SELECT document, occurred_at::text, party, transaction_type, quantity, amount
	FROM header_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR transaction_type ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document
	` + pagination
}

// dailySalesDetailReadModelQuery preserves the captured PowerBuilder detail
// contract while retaining the shared document/party identity fields used by
// the API's filtering and audit responses. Canonical imported lines prefer
// retained Saledetail payload values where available; new documents use their
// typed line/pricing snapshots. Compatibility events are expanded one payload
// row at a time only when no canonical document identity exists.
func dailySalesDetailReadModelQuery(pagination string) string {
	return `
	WITH sales_detail_read_model AS (
		SELECT bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(mp.name, CASE WHEN bd.kind = 'cash-sale' THEN 'CASH' ELSE '' END) AS party,
		       COALESCE(bl.item_name, '') AS item,
		       COALESCE(bl.quantity::text, '') AS quantity,
		       COALESCE(NULLIF(bl.legacy_payload->>'Amount', ''), bl.line_total::text, bd.total_amount::text, '') AS amount,
		       COALESCE(NULLIF(bl.legacy_payload->>'AliasName', ''), NULLIF(mi.payload->>'AliasName', ''), NULLIF(bl.item_code, ''), bl.item_legacy_id, '') AS alias,
		       COALESCE(bl.item_name, '') AS item_description,
		       COALESCE(NULLIF(bl.legacy_payload->>'SalePrice', ''), NULLIF(bl.pricing->>'salePrice', ''), bl.unit_price::text, '') AS sale_price,
		       COALESCE(
			       NULLIF(bl.legacy_payload->>'DiscPerc', ''),
			       NULLIF(bl.pricing->>'discountPercent', ''),
			       NULLIF(bl.pricing->>'customerDiscountRate', ''),
			       CASE WHEN bl.line_gross > 0 AND bl.line_gross >= bl.line_total
			            THEN ((bl.line_gross - bl.line_total) / bl.line_gross * 100)::numeric(19,4)::text
			            ELSE '0.00' END,
			       '0.00'
		       ) AS discount_percent,
		       COALESCE(
			       NULLIF(bl.legacy_payload->>'DiscountValue', ''),
			       CASE WHEN bl.line_gross >= bl.line_total
			            THEN (bl.line_gross - bl.line_total)::numeric(19,2)::text
			            ELSE '0.00' END,
			       '0.00'
		       ) AS discount_value,
		       COALESCE(NULLIF(bl.legacy_payload->>'itemflatdisc', ''), NULLIF(bl.pricing->>'itemDiscount', ''), bl.item_discount::text, '0.00') AS item_discount,
		       COALESCE(NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), bl.tax_amount::text, '0.00') AS sales_tax_value,
		       COALESCE(NULLIF(allocation.expiry_date, ''), NULLIF(bl.expiry_date::text, ''), '') AS expiry_date,
		       COALESCE(NULLIF(allocation.batch_number, ''), NULLIF(bl.batch_number, ''), '') AS batch_number
		FROM business_documents bd
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
		 AND mp.party_type = 'customer'
		LEFT JOIN business_document_lines bl
		  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
		 AND bl.document_id = bd.id
		LEFT JOIN master_items mi
		  ON mi.tenant_id = bl.tenant_id AND mi.id = bl.item_id
		LEFT JOIN LATERAL (
			SELECT string_agg(DISTINCT sb.batch_number, ', ' ORDER BY sb.batch_number) AS batch_number,
			       string_agg(DISTINCT sb.expiry_date::text, ', ' ORDER BY sb.expiry_date::text) AS expiry_date
			FROM stock_allocations sa
			JOIN stock_batches sb
			  ON sb.tenant_id = sa.tenant_id AND sb.branch_id = sa.branch_id AND sb.id = sa.batch_id
			WHERE sa.tenant_id = bl.tenant_id AND sa.branch_id = bl.branch_id
			  AND sa.document_line_id = bl.id
		) allocation ON TRUE
		WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
		  AND bd.kind IN ('cash-sale', 'credit-sale')
		  AND bd.status = 'posted'

		UNION ALL

		SELECT sd.document_number,
		       sd.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(payload_row.value->>'amount', payload_row.value->>'lineTotal', se.payload->>'totalAmount', sd.total_amount::text, ''),
		       COALESCE(payload_row.value->>'alias', payload_row.value->>'aliasName', payload_row.value->>'itemLegacyId', ''),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'salePrice', payload_row.value->>'unitPrice', payload_row.value->>'rate', ''),
		       COALESCE(payload_row.value->>'discountPercent', payload_row.value->>'discPerc', '0.00'),
		       COALESCE(payload_row.value->>'discountValue', payload_row.value->>'discountAmount', '0.00'),
		       COALESCE(payload_row.value->>'itemDiscount', payload_row.value->>'itemDisc', payload_row.value->>'itemflatdisc', '0.00'),
		       COALESCE(payload_row.value->>'salesTaxValue', payload_row.value->>'salesTax', payload_row.value->>'taxAmount', '0.00'),
		       COALESCE(payload_row.value->>'expiryDate', ''),
		       COALESCE(payload_row.value->>'batchNumber', '')
		FROM sales_documents sd
		LEFT JOIN sync_events se
		  ON se.tenant_id = sd.tenant_id AND se.branch_id = sd.branch_id
		 AND se.aggregate_id = sd.id AND se.aggregate = 'sale'
		 AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid
		  AND sd.status = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = sd.tenant_id AND bd.branch_id = sd.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = sd.id OR bd.document_number = sd.document_number)
		  )

		UNION ALL

		SELECT COALESCE(se.payload->>'documentNumber', se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(payload_row.value->>'amount', payload_row.value->>'lineTotal', se.payload->>'totalAmount', se.payload->>'amount', ''),
		       COALESCE(payload_row.value->>'alias', payload_row.value->>'aliasName', payload_row.value->>'itemLegacyId', ''),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'salePrice', payload_row.value->>'unitPrice', payload_row.value->>'rate', ''),
		       COALESCE(payload_row.value->>'discountPercent', payload_row.value->>'discPerc', '0.00'),
		       COALESCE(payload_row.value->>'discountValue', payload_row.value->>'discountAmount', '0.00'),
		       COALESCE(payload_row.value->>'itemDiscount', payload_row.value->>'itemDisc', payload_row.value->>'itemflatdisc', '0.00'),
		       COALESCE(payload_row.value->>'salesTaxValue', payload_row.value->>'salesTax', payload_row.value->>'taxAmount', '0.00'),
		       COALESCE(payload_row.value->>'expiryDate', ''),
		       COALESCE(payload_row.value->>'batchNumber', '')
		FROM sync_events se
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND se.aggregate = 'sale'
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id OR bd.document_number = se.payload->>'documentNumber')
		  )
		  AND NOT EXISTS (
			SELECT 1
			FROM sales_documents sd
			WHERE sd.tenant_id = se.tenant_id AND sd.branch_id = se.branch_id
			  AND sd.status = 'posted'
			  AND sd.id = se.aggregate_id
		  )
	)
	SELECT document, occurred_at::text, party, item, quantity, amount,
	       alias, item_description, sale_price, discount_percent, discount_value,
	       item_discount, sales_tax_value, expiry_date, batch_number
	FROM sales_detail_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR item ILIKE '%' || $5 || '%'
	       OR alias ILIKE '%' || $5 || '%'
	       OR item_description ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document, item_description ` + pagination
}

// salesReadModelQueryMode keeps the detailed projection as the compatibility
// default while allowing captured invoice-summary leaves to aggregate each
// document once. The underlying read model is still the authority for
// document identity, date scope, and compatibility de-duplication.
func salesReadModelQueryMode(aggregateCondition, mode, pagination string) string {
	if mode == "line-detail" {
		if aggregateCondition == reportSaleReturnAggregate {
			return saleReturnDetailReadModelQuery(pagination)
		}
		return dailySalesDetailReadModelQuery(pagination)
	}
	if mode == "invoice-summary" {
		return invoiceSummaryReadModelQuery(salesReadModelQuery(aggregateCondition, ""), pagination)
	}
	if mode == "day-summary" || mode == "month-summary" || mode == "hour-summary" {
		return salesCalendarSummaryReadModelQuery(aggregateCondition, mode, pagination)
	}
	if mode == "item-summary" {
		return salesItemSummaryReadModelQuery(aggregateCondition, pagination)
	}
	if mode == "profit-margin-detail" {
		return salesProfitMarginReadModelQuery(aggregateCondition, pagination)
	}
	if mode == "profit-day-summary" {
		return salesProfitDaySummaryReadModelQuery(aggregateCondition, pagination)
	}
	if mode == "profit-customer-summary" {
		return salesProfitCustomerSummaryReadModelQuery(aggregateCondition, pagination)
	}
	if mode == "customer-summary" {
		return salesCustomerSummaryReadModelQuery(aggregateCondition, pagination)
	}
	if mode == "customer-category-summary" {
		return salesCustomerCategorySummaryReadModelQuery(aggregateCondition, pagination)
	}
	if mode == "customer-wise-category-summary" {
		return salesCustomerWiseCategorySummaryReadModelQuery(aggregateCondition, pagination)
	}
	return salesReadModelQuery(aggregateCondition, pagination)
}

func salesSummaryUsesProfitMarginScanner(mode string) bool {
	switch mode {
	case "profit-margin-detail", "profit-day-summary", "profit-customer-summary":
		return true
	default:
		return false
	}
}

func salesCalendarSummaryReadModelQuery(aggregateCondition, mode, pagination string) string {
	base := invoiceSummaryReadModelQuery(salesReadModelQuery(aggregateCondition, ""), "")
	bucket := "occurred_at::date"
	if mode == "month-summary" {
		bucket = "date_trunc('month', occurred_at::date)::date"
	} else if mode == "hour-summary" {
		bucket = "date_trunc('hour', occurred_at)"
	}
	return `
		WITH invoice_rows AS (` + base + `), calendar_rows AS (
			SELECT ` + bucket + ` AS period,
			       COALESCE(NULLIF(party, ''), 'CASH') AS party,
			       quantity, amount
			FROM invoice_rows
		)
		SELECT period::text,
		       period::text,
		       party,
		       '',
		       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0)::numeric(19,4)::text,
		       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0)::numeric(19,4)::text
		FROM calendar_rows
		GROUP BY period, party
		ORDER BY period DESC, party
		` + pagination
}

func salesItemSummaryReadModelQuery(aggregateCondition, pagination string) string {
	base := salesReadModelQuery(aggregateCondition, "")
	return `
		WITH sales_rows AS (` + base + `), item_rows AS (
			SELECT COALESCE(NULLIF(item, ''), 'Unspecified') AS item_name,
			       COALESCE(NULLIF(party, ''), 'CASH') AS party,
			       occurred_at, quantity, amount
			FROM sales_rows
		)
		SELECT item_name,
		       MAX(occurred_at)::text,
		       party,
		       '',
		       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0)::numeric(19,4)::text,
		       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0)::numeric(19,4)::text
		FROM item_rows
		GROUP BY item_name, party
		ORDER BY item_name, party
		` + pagination
}

func salesProfitMarginReadModelQuery(aggregateCondition, pagination string) string {
	eventCondition := "se.aggregate = 'sale'"
	canonicalKinds := "'cash-sale', 'credit-sale'"
	if aggregateCondition == reportSaleOrReturn {
		eventCondition = "se.aggregate IN ('sale', 'sale_return')"
		canonicalKinds = "'cash-sale', 'credit-sale', 'cash-return', 'credit-return', 'open-cash-return', 'open-credit-return'"
	}
	return `
	WITH profit_rows AS (
		SELECT bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(mp.name, CASE WHEN bd.kind IN ('cash-sale', 'cash-return', 'open-cash-return') THEN 'CASH' ELSE '' END) AS party,
		       COALESCE(bl.item_name, '') AS item,
		       bl.quantity::text AS quantity,
		       COALESCE(NULLIF(bl.legacy_payload->>'SalePrice', ''), NULLIF(bl.pricing->>'salePrice', ''), bl.unit_price::text, '') AS sale_price,
		       COALESCE(NULLIF(bl.legacy_payload->>'Amount', ''), bl.line_total::text, bd.total_amount::text, '') AS amount,
		       CASE WHEN allocated_cost.allocation_count > 0 THEN allocated_cost.cost::numeric(19,4)::text ELSE '' END AS cost,
		       COALESCE(bl.tax_amount, 0)::numeric(19,4)::text AS sales_tax_value
		FROM business_documents bd
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
		 AND mp.party_type = 'customer'
		JOIN business_document_lines bl
		  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
		 AND bl.document_id = bd.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(sa.quantity * sa.unit_cost), 0) AS cost,
			       COUNT(*) AS allocation_count
			FROM stock_allocations sa
			WHERE sa.tenant_id = bl.tenant_id AND sa.branch_id = bl.branch_id
			  AND sa.document_line_id = bl.id
		) allocated_cost ON TRUE
		WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
		  AND bd.kind IN (` + canonicalKinds + `)
		  AND bd.status = 'posted'

		UNION ALL

		SELECT sd.document_number,
		       sd.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(payload_row.value->>'salePrice', payload_row.value->>'unitPrice', payload_row.value->>'rate', ''),
		       COALESCE(payload_row.value->>'amount', payload_row.value->>'lineTotal', se.payload->>'totalAmount', sd.total_amount::text, ''),
		       COALESCE(NULLIF(payload_row.value->>'purchasePrice', ''), NULLIF(payload_row.value->>'costPrice', ''), NULLIF(payload_row.value->>'unitCost', ''), ''),
		       COALESCE(payload_row.value->>'salesTaxValue', payload_row.value->>'salesTax', payload_row.value->>'taxAmount', '0.00')
		FROM sales_documents sd
		LEFT JOIN sync_events se
		  ON se.tenant_id = sd.tenant_id AND se.branch_id = sd.branch_id
		 AND se.aggregate = 'sale' AND se.aggregate_id = sd.id
		 AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid
		  AND sd.status = 'posted'
		  AND NOT EXISTS (
			SELECT 1 FROM business_documents bd
			WHERE bd.tenant_id = sd.tenant_id AND bd.branch_id = sd.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = sd.id OR bd.document_number = sd.document_number)
		  )

		UNION ALL

		SELECT COALESCE(se.payload->>'documentNumber', se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(payload_row.value->>'salePrice', payload_row.value->>'unitPrice', payload_row.value->>'rate', ''),
		       COALESCE(payload_row.value->>'amount', payload_row.value->>'lineTotal', se.payload->>'totalAmount', se.payload->>'amount', ''),
		       COALESCE(NULLIF(payload_row.value->>'purchasePrice', ''), NULLIF(payload_row.value->>'costPrice', ''), NULLIF(payload_row.value->>'unitCost', ''), ''),
		       COALESCE(payload_row.value->>'salesTaxValue', payload_row.value->>'salesTax', payload_row.value->>'taxAmount', '0.00')
		FROM sync_events se
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND ` + eventCondition + `
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1 FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id OR bd.document_number = se.payload->>'documentNumber')
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM sales_documents sd
			WHERE sd.tenant_id = se.tenant_id AND sd.branch_id = se.branch_id
			  AND sd.id = se.aggregate_id
		  )
	)
	SELECT document,
	       occurred_at::text,
	       party,
	       item,
	       quantity,
	       sale_price,
	       amount,
	       cost,
	       CASE
			WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$'
			 AND sales_tax_value ~ '^-?[0-9]+(\.[0-9]+)?$'
			 AND cost ~ '^-?[0-9]+(\.[0-9]+)?$'
			THEN (amount::numeric - sales_tax_value::numeric - cost::numeric)::numeric(19,4)::text
			ELSE ''
	       END AS gross_profit,
	       CASE
			WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$'
			 AND sales_tax_value ~ '^-?[0-9]+(\.[0-9]+)?$'
			 AND cost ~ '^-?[0-9]+(\.[0-9]+)?$'
			 AND (amount::numeric - sales_tax_value::numeric) <> 0
			THEN ((amount::numeric - sales_tax_value::numeric - cost::numeric) * 100 /
			      (amount::numeric - sales_tax_value::numeric))::numeric(19,4)::text
			ELSE ''
	       END AS margin_percent,
	       sales_tax_value
	FROM profit_rows
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%' OR party ILIKE '%' || $5 || '%' OR item ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document, item
	` + pagination
}

func salesProfitDaySummaryReadModelQuery(aggregateCondition, pagination string) string {
	base := salesProfitMarginReadModelQuery(aggregateCondition, "")
	return `
	WITH profit_rows AS (` + base + `), day_rows AS (
		SELECT occurred_at::date AS period,
		       COALESCE(NULLIF(party, ''), 'CASH') AS party,
		       quantity,
		       amount,
		       cost,
		       gross_profit,
		       sales_tax_value
		FROM profit_rows
	), grouped_rows AS (
		SELECT period,
		       party,
		       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0) AS quantity,
		       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0) AS amount,
		       COALESCE(SUM(CASE WHEN cost ~ '^-?[0-9]+(\.[0-9]+)?$' THEN cost::numeric ELSE 0 END), 0) AS cost,
		       COALESCE(SUM(CASE WHEN gross_profit ~ '^-?[0-9]+(\.[0-9]+)?$' THEN gross_profit::numeric ELSE 0 END), 0) AS gross_profit,
		       COALESCE(SUM(CASE WHEN sales_tax_value ~ '^-?[0-9]+(\.[0-9]+)?$' THEN sales_tax_value::numeric ELSE 0 END), 0) AS sales_tax,
		       COUNT(*) AS row_count,
		       COUNT(*) FILTER (WHERE cost ~ '^-?[0-9]+(\.[0-9]+)?$') AS cost_count
		FROM day_rows
		GROUP BY period, party
	)
	SELECT period::text,
	       period::text,
	       party,
	       '',
	       quantity::numeric(19,4)::text,
	       CASE WHEN quantity <> 0 THEN (amount / quantity)::numeric(19,4)::text ELSE '' END,
	       amount::numeric(19,4)::text,
	       CASE WHEN cost_count = row_count THEN cost::numeric(19,4)::text ELSE '' END,
	       CASE WHEN cost_count = row_count THEN gross_profit::numeric(19,4)::text ELSE '' END,
	       CASE WHEN cost_count = row_count AND (amount - sales_tax) <> 0
	            THEN (gross_profit * 100 / (amount - sales_tax))::numeric(19,4)::text ELSE '' END,
	       sales_tax::numeric(19,4)::text
	FROM grouped_rows
	ORDER BY period DESC, party
	` + pagination
}

func salesProfitCustomerSummaryReadModelQuery(aggregateCondition, pagination string) string {
	base := salesProfitMarginReadModelQuery(aggregateCondition, "")
	return `
	WITH profit_rows AS (` + base + `), customer_rows AS (
		SELECT COALESCE(NULLIF(party, ''), 'CASH') AS customer,
		       MAX(occurred_at) AS last_occurred_at,
		       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0) AS quantity,
		       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0) AS amount,
		       COALESCE(SUM(CASE WHEN cost ~ '^-?[0-9]+(\.[0-9]+)?$' THEN cost::numeric ELSE 0 END), 0) AS cost,
		       COALESCE(SUM(CASE WHEN gross_profit ~ '^-?[0-9]+(\.[0-9]+)?$' THEN gross_profit::numeric ELSE 0 END), 0) AS gross_profit,
		       COALESCE(SUM(CASE WHEN sales_tax_value ~ '^-?[0-9]+(\.[0-9]+)?$' THEN sales_tax_value::numeric ELSE 0 END), 0) AS sales_tax,
		       COUNT(*) AS row_count,
		       COUNT(*) FILTER (WHERE cost ~ '^-?[0-9]+(\.[0-9]+)?$') AS cost_count
		FROM profit_rows
		GROUP BY customer
	)
	SELECT customer,
	       last_occurred_at::text,
	       customer,
	       'Customer total',
	       quantity::numeric(19,4)::text,
	       CASE WHEN quantity <> 0 THEN (amount / quantity)::numeric(19,4)::text ELSE '' END,
	       amount::numeric(19,4)::text,
	       CASE WHEN cost_count = row_count THEN cost::numeric(19,4)::text ELSE '' END,
	       CASE WHEN cost_count = row_count THEN gross_profit::numeric(19,4)::text ELSE '' END,
	       CASE WHEN cost_count = row_count AND (amount - sales_tax) <> 0
	            THEN (gross_profit * 100 / (amount - sales_tax))::numeric(19,4)::text ELSE '' END,
	       sales_tax::numeric(19,4)::text
	FROM customer_rows
	ORDER BY customer
	` + pagination
}

func salesCustomerSummaryReadModelQuery(aggregateCondition, pagination string) string {
	base := invoiceSummaryReadModelQuery(salesReadModelQuery(aggregateCondition, ""), "")
	return `
	WITH invoice_rows AS (` + base + `), customer_rows AS (
		SELECT COALESCE(NULLIF(party, ''), 'CASH') AS customer,
		       MAX(occurred_at) AS last_occurred_at,
		       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0) AS quantity,
		       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0) AS amount
		FROM invoice_rows
		GROUP BY customer
	)
	SELECT customer,
	       last_occurred_at::text,
	       customer,
	       'Customer total',
	       quantity::numeric(19,4)::text,
	       amount::numeric(19,4)::text
	FROM customer_rows
	ORDER BY customer
	` + pagination
}

func customerPartyLookupExpression(customerExpression string) string {
	return "lower(mp.name) = lower(" + customerExpression + ") OR lower(mp.code) = lower(" + customerExpression + ") OR lower(mp.legacy_id) = lower(" + customerExpression + ")"
}

func salesCustomerCategorySummaryReadModelQuery(aggregateCondition, pagination string) string {
	return salesCustomerCategorySummaryReadModelQueryMode(aggregateCondition, pagination, false)
}

func salesCustomerWiseCategorySummaryReadModelQuery(aggregateCondition, pagination string) string {
	return salesCustomerCategorySummaryReadModelQueryMode(aggregateCondition, pagination, true)
}

func salesCustomerCategorySummaryReadModelQueryMode(aggregateCondition, pagination string, groupCustomer bool) string {
	eventCondition := "se.aggregate = 'sale'"
	canonicalKinds := "'cash-sale', 'credit-sale'"
	if aggregateCondition == reportSaleOrReturn {
		eventCondition = "se.aggregate IN ('sale', 'sale_return')"
		canonicalKinds = "'cash-sale', 'credit-sale', 'cash-return', 'credit-return', 'open-cash-return', 'open-credit-return'"
	}
	groupColumns := "category"
	summaryColumns := "category, MAX(occurred_at)::text, category, 'Category total'"
	orderColumns := "category"
	customerFilter := ""
	if groupCustomer {
		groupColumns = "customer, category"
		summaryColumns = "customer, MAX(occurred_at)::text, customer, category"
		orderColumns = "customer, category"
		customerFilter = " OR customer ILIKE '%' || $5 || '%'"
	}
	return `
	WITH category_rows AS (
		SELECT bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(NULLIF(mp.name, ''), 'Unknown Customer') AS customer,
		       COALESCE(
		         NULLIF(mp.payload->>'Category', ''),
		         NULLIF(mp.payload->>'CustomerCategory', ''),
		         NULLIF(mp.payload->>'category', ''),
		         'Unspecified'
		       ) AS category,
		       COALESCE(NULLIF(bl.legacy_payload->>'Quantity', ''), bl.quantity::text, '') AS quantity,
		       COALESCE(NULLIF(bl.legacy_payload->>'Amount', ''), bl.line_total::text, '') AS amount
		FROM business_documents bd
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
		 AND mp.party_type = 'customer'
		JOIN business_document_lines bl
		  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
		 AND bl.document_id = bd.id
		WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
		  AND bd.kind IN (` + canonicalKinds + `)
		  AND bd.status = 'posted'

		UNION ALL

		SELECT sd.document_number,
		       sd.occurred_at,
		       COALESCE(NULLIF(customer_category.customer, ''), 'Unknown Customer'),
		       COALESCE(NULLIF(customer_category.category, ''), 'Unspecified'),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(
		         payload_row.value->>'amount',
		         payload_row.value->>'lineTotal',
		         CASE
		           WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array'
		                OR jsonb_array_length(se.payload->'rows') = 0
		           THEN COALESCE(se.payload->>'totalAmount', sd.total_amount::text)
		           ELSE ''
		         END,
		         ''
		       )
		FROM sales_documents sd
		LEFT JOIN sync_events se
		  ON se.tenant_id = sd.tenant_id AND se.branch_id = sd.branch_id
		 AND se.aggregate_id = sd.id AND se.aggregate = 'sale'
		 AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(NULLIF(mp.name, ''), 'Unknown Customer') AS customer,
			       COALESCE(
			         NULLIF(mp.payload->>'Category', ''),
			         NULLIF(mp.payload->>'CustomerCategory', ''),
			         NULLIF(mp.payload->>'category', ''),
			         'Unspecified'
			       ) AS category
			FROM master_parties mp
			WHERE mp.tenant_id = sd.tenant_id
			  AND mp.party_type = 'customer'
			  AND (` + customerPartyLookupExpression("COALESCE(payload_row.value->>'customerName', payload_row.value->>'customer', se.payload->>'customerName', se.payload->>'customer', '')") + `)
			ORDER BY mp.id
			LIMIT 1
		) customer_category ON TRUE
		WHERE sd.tenant_id = $1::uuid AND sd.branch_id = $2::uuid
		  AND sd.status = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = sd.tenant_id AND bd.branch_id = sd.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = sd.id OR bd.document_number = sd.document_number)
		  )

		UNION ALL

		SELECT COALESCE(se.payload->>'documentNumber', se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(NULLIF(customer_category.customer, ''), 'Unknown Customer'),
		       COALESCE(NULLIF(customer_category.category, ''), 'Unspecified'),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(
		         payload_row.value->>'amount',
		         payload_row.value->>'lineTotal',
		         CASE
		           WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array'
		                OR jsonb_array_length(se.payload->'rows') = 0
		           THEN COALESCE(se.payload->>'totalAmount', se.payload->>'amount')
		           ELSE ''
		         END,
		         ''
		       )
		FROM sync_events se
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(NULLIF(mp.name, ''), 'Unknown Customer') AS customer,
			       COALESCE(
			         NULLIF(mp.payload->>'Category', ''),
			         NULLIF(mp.payload->>'CustomerCategory', ''),
			         NULLIF(mp.payload->>'category', ''),
			         'Unspecified'
			       ) AS category
			FROM master_parties mp
			WHERE mp.tenant_id = se.tenant_id
			  AND mp.party_type = 'customer'
			  AND (` + customerPartyLookupExpression("COALESCE(payload_row.value->>'customerName', payload_row.value->>'customer', se.payload->>'customerName', se.payload->>'customer', '')") + `)
			ORDER BY mp.id
			LIMIT 1
		) customer_category ON TRUE
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND ` + eventCondition + `
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id OR bd.document_number = se.payload->>'documentNumber')
		  )
		  AND NOT EXISTS (
			SELECT 1
			FROM sales_documents sd
			WHERE sd.tenant_id = se.tenant_id AND sd.branch_id = se.branch_id
			  AND sd.status = 'posted'
			  AND sd.id = se.aggregate_id
		  )
	)
	SELECT ` + summaryColumns + `,
	       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0)::numeric(19,4)::text,
	       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0)::numeric(19,4)::text
	FROM category_rows
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%' OR category ILIKE '%' || $5 || '%'` + customerFilter + `)
	GROUP BY ` + groupColumns + `
	ORDER BY ` + orderColumns + `
	` + pagination
}

func saleReturnDetailReadModelQuery(pagination string) string {
	return `
	WITH sale_return_detail_read_model AS (
		SELECT bd.document_number AS document,
		       bd.occurred_at,
		       COALESCE(mp.name,
		                CASE WHEN bd.kind IN ('cash-return', 'open-cash-return',
		                                       'cash-sale-return', 'open-sale-return')
		                     THEN 'CASH' ELSE '' END) AS party,
		       COALESCE(bl.item_name, '') AS item,
		       COALESCE(bl.quantity::text, '') AS quantity,
		       COALESCE(NULLIF(bl.legacy_payload->>'Amount', ''), bl.line_total::text, bd.total_amount::text, '') AS amount,
		       COALESCE(NULLIF(bl.legacy_payload->>'AliasName', ''), NULLIF(mi.payload->>'AliasName', ''), NULLIF(bl.item_code, ''), bl.item_legacy_id, '') AS alias,
		       COALESCE(bl.item_name, '') AS item_description,
		       COALESCE(NULLIF(bl.legacy_payload->>'SalePrice', ''), NULLIF(bl.pricing->>'salePrice', ''), bl.unit_price::text, '') AS sale_price,
		       COALESCE(NULLIF(bl.legacy_payload->>'DiscPerc', ''), NULLIF(bl.pricing->>'discountPercent', ''), NULLIF(bl.pricing->>'customerDiscountRate', ''), '0.00') AS discount_percent,
		       COALESCE(NULLIF(bl.legacy_payload->>'DiscountValue', ''), '0.00') AS discount_value,
		       COALESCE(NULLIF(bl.legacy_payload->>'itemflatdisc', ''), NULLIF(bl.pricing->>'itemDiscount', ''), bl.item_discount::text, '0.00') AS item_discount,
		       COALESCE(NULLIF(bl.legacy_payload->>'SalesTax', ''), NULLIF(bl.pricing->'taxes'->0->>'amount', ''), bl.tax_amount::text, '0.00') AS sales_tax_value,
		       COALESCE(NULLIF(allocation.expiry_date, ''), NULLIF(bl.expiry_date::text, ''), '') AS expiry_date,
		       COALESCE(NULLIF(allocation.batch_number, ''), NULLIF(bl.batch_number, ''), '') AS batch_number
		FROM business_documents bd
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = bd.tenant_id AND mp.id = bd.customer_id
		 AND mp.party_type = 'customer'
		LEFT JOIN business_document_lines bl
		  ON bl.tenant_id = bd.tenant_id AND bl.branch_id = bd.branch_id
		 AND bl.document_id = bd.id
		LEFT JOIN master_items mi
		  ON mi.tenant_id = bl.tenant_id AND mi.id = bl.item_id
		LEFT JOIN LATERAL (
			SELECT string_agg(DISTINCT sb.batch_number, ', ' ORDER BY sb.batch_number) AS batch_number,
			       string_agg(DISTINCT sb.expiry_date::text, ', ' ORDER BY sb.expiry_date::text) AS expiry_date
			FROM stock_allocations sa
			JOIN stock_batches sb
			  ON sb.tenant_id = sa.tenant_id AND sb.branch_id = sa.branch_id AND sb.id = sa.batch_id
			WHERE sa.tenant_id = bl.tenant_id AND sa.branch_id = bl.branch_id
			  AND sa.document_line_id = bl.id
		) allocation ON TRUE
		WHERE bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid
		  AND bd.kind IN ('cash-return', 'credit-return', 'open-cash-return',
		                  'open-credit-return', 'cash-sale-return', 'credit-sale-return',
		                  'open-sale-return')
		  AND bd.status = 'posted'

		UNION ALL

		SELECT COALESCE(se.payload->>'documentNumber', se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(se.payload->>'customerName', se.payload->>'customer', 'CASH'),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'quantity', ''),
		       COALESCE(payload_row.value->>'amount', payload_row.value->>'lineTotal', se.payload->>'totalAmount', se.payload->>'amount', ''),
		       COALESCE(payload_row.value->>'alias', payload_row.value->>'aliasName', payload_row.value->>'itemLegacyId', ''),
		       COALESCE(payload_row.value->>'itemDescription', payload_row.value->>'itemName', payload_row.value->>'item', ''),
		       COALESCE(payload_row.value->>'salePrice', payload_row.value->>'unitPrice', payload_row.value->>'rate', ''),
		       COALESCE(payload_row.value->>'discountPercent', payload_row.value->>'discPerc', '0.00'),
		       COALESCE(payload_row.value->>'discountValue', payload_row.value->>'discountAmount', '0.00'),
		       COALESCE(payload_row.value->>'itemDiscount', payload_row.value->>'itemDisc', payload_row.value->>'itemflatdisc', '0.00'),
		       COALESCE(payload_row.value->>'salesTaxValue', payload_row.value->>'salesTax', payload_row.value->>'taxAmount', '0.00'),
		       COALESCE(payload_row.value->>'expiryDate', ''),
		       COALESCE(payload_row.value->>'batchNumber', '')
		FROM sync_events se
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND se.aggregate IN ('sale_return', 'sale-return')
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents bd
			WHERE bd.tenant_id = se.tenant_id AND bd.branch_id = se.branch_id
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id OR bd.document_number = se.payload->>'documentNumber')
		  )
	)
	SELECT document, occurred_at::text, party, item, quantity, amount,
	       alias, item_description, sale_price, discount_percent, discount_value,
	       item_discount, sales_tax_value, expiry_date, batch_number
	FROM sale_return_detail_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR item ILIKE '%' || $5 || '%'
	       OR alias ILIKE '%' || $5 || '%'
	       OR item_description ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document, item_description ` + pagination
}

func invoiceSummaryReadModelQuery(baseQuery, pagination string) string {
	return `
	WITH invoice_rows AS (` + baseQuery + `)
	SELECT document,
	       MAX(occurred_at)::text,
	       MAX(party),
	       '',
	       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+(\.[0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0)::numeric(19,4)::text,
	       COALESCE(MAX(CASE WHEN amount ~ '^-?[0-9]+(\.[0-9]+)?$' THEN amount::numeric ELSE 0 END), 0)::numeric(19,4)::text
	FROM invoice_rows
	GROUP BY document
	ORDER BY MAX(occurred_at) DESC, document ` + pagination
}

// saleReturnReadModelQuery is the posted sale-return counterpart to
// salesReadModelQuery. Source-bound and open returns are both canonical
// business documents; compatibility sale_return events remain visible only
// when no canonical document with the same identity exists.
func saleReturnReadModelQuery(pagination string, includeIdentity ...bool) string {
	withIdentity := len(includeIdentity) > 0 && includeIdentity[0]
	selectColumns := "document, occurred_at::text, party, item, quantity, amount"
	if withIdentity {
		selectColumns = "document_id, " + selectColumns
	}
	return `
	WITH sale_return_read_model AS (
		SELECT bd.id::text AS document_id,
		       bd.document_number AS document,
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

		SELECT NULL::text AS document_id,
		       COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
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
			  AND bd.status = 'posted'
			  AND (bd.id = se.aggregate_id
			       OR bd.document_number = se.payload->>'documentNumber')
		  )
	)
	SELECT ` + selectColumns + `
	FROM sale_return_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR item ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document, item ` + pagination
}

func saleReturnReadModelQueryMode(mode, pagination string) string {
	if mode == "line-detail" {
		return saleReturnDetailReadModelQuery(pagination)
	}
	if mode != "invoice-summary" {
		return saleReturnReadModelQuery(pagination)
	}
	return invoiceSummaryReadModelQuery(saleReturnReadModelQuery(""), pagination)
}

// purchaseLineDetailReadModelQuery expands canonical purchase lines and
// compatibility receiving/return payload rows into the source-backed detail
// contract. Canonical rows win by document identity, so a replayed event does
// not double-count an imported or newly posted document.
func purchaseLineDetailReadModelQuery(aggregateCondition, pagination string) string {
	canonicalKinds := "'pack-purchase', 'loose-purchase', 'opening-purchase'"
	eventAggregate := "receiving"
	if aggregateCondition == "se.aggregate = 'return'" {
		canonicalKinds = "'purchase-return'"
		eventAggregate = "return"
	} else if aggregateCondition == "se.aggregate = 'purchase_order'" {
		canonicalKinds = "'purchase-order'"
		eventAggregate = "purchase_order"
	}
	return `
	WITH purchase_line_detail_read_model AS (
		SELECT COALESCE(NULLIF(d.document_number, ''), d.id::text) AS document,
		       d.occurred_at,
		       COALESCE(mp.name, '') AS party,
		       COALESCE(NULLIF(l.item_name, ''), l.item_legacy_id, '') AS item,
		       COALESCE(stock.stock_quantity, l.quantity)::text AS quantity,
		       COALESCE(NULLIF(l.legacy_payload->>'PurPrice', ''),
		                NULLIF(l.legacy_payload->>'purchasePrice', ''),
		                NULLIF(l.pricing->>'purchasePrice', ''),
		                NULLIF(l.pricing->>'unitCost', ''),
		                NULLIF(l.unit_cost, 0)::text,
		                l.unit_cost::text,
		                l.unit_price::text, '') AS purchase_price,
		       COALESCE(NULLIF(l.legacy_payload->>'DiscPerc', ''),
		                NULLIF(l.legacy_payload->>'discountPercent', ''),
		                NULLIF(l.pricing->>'discountPercent', ''),
		                '0.00') AS discount_percent,
		       COALESCE(NULLIF(l.legacy_payload->>'DiscountValue', ''),
		                NULLIF(l.legacy_payload->>'discountAmount', ''),
		                NULLIF(l.pricing->>'discountValue', ''),
		                (COALESCE(l.supplier_discount, 0) + COALESCE(l.item_discount, 0) + COALESCE(l.customer_discount, 0))::text,
		                '0.00') AS discount_value,
		       COALESCE(NULLIF(l.legacy_payload->>'SalesTax', ''),
		                NULLIF(l.legacy_payload->>'taxAmount', ''),
		                NULLIF(l.pricing->'taxes'->0->>'amount', ''),
		                l.tax_amount::text,
		                '0.00') AS sales_tax_value,
		       COALESCE(NULLIF(l.legacy_payload->>'Amount', ''),
		                NULLIF(l.legacy_payload->>'LineTotal', ''),
		                l.line_total::text, '') AS amount,
		       COALESCE(NULLIF(l.legacy_payload->>'ExpiryDate', ''), l.expiry_date::text, '') AS expiry_date,
		       COALESCE(NULLIF(l.legacy_payload->>'BatchNumber', ''),
		                NULLIF(l.legacy_payload->>'Batch', ''),
		                NULLIF(l.batch_number, ''), '') AS batch_number
		FROM business_documents d
		LEFT JOIN master_parties mp
		  ON mp.tenant_id = d.tenant_id AND mp.id = d.supplier_id
		 AND mp.party_type = 'supplier'
		JOIN business_document_lines l
		  ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id
		 AND l.document_id = d.id
		LEFT JOIN LATERAL (
			SELECT SUM(sl.quantity * sl.adjustment_sign) AS stock_quantity
			FROM stock_ledger sl
			WHERE sl.tenant_id = d.tenant_id AND sl.branch_id = d.branch_id
			  AND sl.source_document_id = d.id
			  AND sl.source_document_line_id = l.id
		) stock ON TRUE
		WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
		  AND d.kind IN (` + canonicalKinds + `)
		  AND d.status = 'posted'

		UNION ALL

		SELECT COALESCE(NULLIF(se.payload->>'documentNumber', ''), se.aggregate_id::text),
		       se.occurred_at,
		       COALESCE(payload_row.value->>'supplierName', payload_row.value->>'supplier',
		                se.payload->>'supplierName', se.payload->>'supplier', ''),
		       COALESCE(payload_row.value->>'itemName', payload_row.value->>'itemDescription',
		                payload_row.value->>'item', payload_row.value->>'itemLegacyId', ''),
		       COALESCE(payload_row.value->>'quantity', se.payload->>'quantity', ''),
		       COALESCE(payload_row.value->>'purchasePrice', payload_row.value->>'unitCost',
		                payload_row.value->>'price', payload_row.value->>'rate',
		                se.payload->>'purchasePrice', se.payload->>'unitCost', ''),
		       COALESCE(payload_row.value->>'discountPercent', payload_row.value->>'discPerc', '0.00'),
		       COALESCE(payload_row.value->>'discountValue', payload_row.value->>'discountAmount',
		                payload_row.value->>'itemDiscount', '0.00'),
		       COALESCE(payload_row.value->>'salesTaxValue', payload_row.value->>'salesTax',
		                payload_row.value->>'taxAmount', '0.00'),
		       COALESCE(payload_row.value->>'amount', payload_row.value->>'lineTotal',
		                se.payload->>'totalAmount', se.payload->>'amount', ''),
		       COALESCE(payload_row.value->>'expiryDate', payload_row.value->>'expiry', ''),
		       COALESCE(payload_row.value->>'batchNumber', payload_row.value->>'batch', '')
		FROM sync_events se
		LEFT JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(se.payload->'rows') IS DISTINCT FROM 'array' THEN jsonb_build_array(se.payload)
				WHEN jsonb_array_length(se.payload->'rows') = 0 THEN jsonb_build_array(se.payload)
				ELSE se.payload->'rows'
			END
		) AS payload_row(value) ON TRUE
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid
		  AND se.aggregate = '` + eventAggregate + `'
		  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
		  AND NOT EXISTS (
			SELECT 1
			FROM business_documents d
			WHERE d.tenant_id = se.tenant_id AND d.branch_id = se.branch_id
			  AND d.status = 'posted'
			  AND (d.id = se.aggregate_id OR d.document_number = se.payload->>'documentNumber')
		  )
	)
	SELECT document, occurred_at::text, party, item, quantity, purchase_price,
	       discount_percent, discount_value, sales_tax_value, amount,
	       expiry_date, batch_number
	FROM purchase_line_detail_read_model
	WHERE occurred_at >= $3::date
	  AND occurred_at < ($4::date + INTERVAL '1 day')
	  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
	       OR party ILIKE '%' || $5 || '%'
	       OR item ILIKE '%' || $5 || '%'
	       OR batch_number ILIKE '%' || $5 || '%')
	ORDER BY occurred_at DESC, document, item, batch_number ` + pagination
}

func purchaseSummaryReportColumns(mode string) []reportColumn {
	switch mode {
	case "invoice-summary":
		return []reportColumn{
			{Key: "document", Label: "Document", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "day-summary":
		return []reportColumn{
			{Key: "document", Label: "Day", DataType: "date", Sortable: true},
			{Key: "occurredAt", Label: "Date", DataType: "date", Sortable: true},
			{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "month-summary":
		return []reportColumn{
			{Key: "document", Label: "Month", DataType: "date", Sortable: true},
			{Key: "occurredAt", Label: "Month", DataType: "date", Sortable: true},
			{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "item-summary":
		return []reportColumn{
			{Key: "document", Label: "Item", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	case "supplier-summary":
		return []reportColumn{
			{Key: "document", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "occurredAt", Label: "Last Posted", DataType: "date", Sortable: true},
			{Key: "party", Label: "Supplier", DataType: "text", Sortable: true},
			{Key: "item", Label: "Summary", DataType: "text", Sortable: true},
			{Key: "quantity", Label: "Quantity", DataType: "number", Sortable: true},
			{Key: "amount", Label: "Amount", DataType: "currency", Sortable: true},
		}
	default:
		return reportColumns()
	}
}

func isPurchaseSummaryMode(mode string) bool {
	switch mode {
	case "invoice-summary", "day-summary", "month-summary", "item-summary", "supplier-summary":
		return true
	default:
		return false
	}
}

func purchaseProjectionModeNote(mode string) string {
	switch mode {
	case "invoice-summary":
		return "canonical and compatibility purchase rows are de-duplicated and grouped once per posted document with numeric quantity and authoritative amount; exact PowerBuilder tax, profit, and format fields remain open"
	case "day-summary":
		return "canonical and compatibility purchase documents are grouped by calendar day and supplier after document de-duplication; exact PowerBuilder day, tax, return, profit, and calculated columns remain open"
	case "month-summary":
		return "canonical and compatibility purchase documents are grouped by calendar month and supplier after document de-duplication; graph rendering and exact PowerBuilder tax, return, profit, and calculated columns remain open"
	case "item-summary":
		return "canonical and compatibility purchase lines are grouped by item and supplier with numeric quantity and amount totals; exact category joins, tax, return, profit, and calculated columns remain open"
	case "supplier-summary":
		return "canonical and compatibility purchase documents are grouped by supplier with numeric quantity and amount totals; exact PowerBuilder supplier, manufacturer, tax, profit, and return calculations remain open"
	default:
		return "captured legacy purchase grouping and calculated numeric fields are not implemented"
	}
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

func purchaseItemSummaryReadModelQuery(aggregateCondition, pagination string) string {
	base := purchaseReadModelQuery(aggregateCondition, "detail", "")
	return `
		WITH purchase_rows AS (` + base + `),
		grouped_rows AS (
			SELECT item,
			       MAX(occurred_at) AS occurred_at,
			       party,
			       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+([.][0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0) AS quantity,
			       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+([.][0-9]+)?$' THEN amount::numeric ELSE 0 END), 0) AS amount
			FROM purchase_rows
			WHERE occurred_at >= $3::date
			  AND occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
			       OR party ILIKE '%' || $5 || '%'
			       OR item ILIKE '%' || $5 || '%')
			GROUP BY item, party
		)
		SELECT item,
		       occurred_at::text,
		       party,
		       item,
		       quantity::numeric(19,4)::text,
		       amount::numeric(19,4)::text
		FROM grouped_rows
		ORDER BY item, party
		` + pagination
}

func purchaseSupplierSummaryReadModelQuery(aggregateCondition, pagination string) string {
	base := purchaseReadModelQuery(aggregateCondition, "invoice-summary", "")
	return `
		WITH purchase_rows AS (` + base + `),
		grouped_rows AS (
			SELECT party,
			       MAX(occurred_at) AS occurred_at,
			       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+([.][0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0) AS quantity,
			       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+([.][0-9]+)?$' THEN amount::numeric ELSE 0 END), 0) AS amount
			FROM purchase_rows
			WHERE occurred_at >= $3::date
			  AND occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
			       OR party ILIKE '%' || $5 || '%'
			       OR item ILIKE '%' || $5 || '%')
			GROUP BY party
		)
		SELECT party,
		       occurred_at::text,
		       party,
		       'Supplier total',
		       quantity::numeric(19,4)::text,
		       amount::numeric(19,4)::text
		FROM grouped_rows
		ORDER BY party
		` + pagination
}

func purchaseReadModelQueryMode(aggregateCondition, mode, pagination string) string {
	switch mode {
	case "day-summary", "month-summary":
		return purchaseCalendarSummaryReadModelQuery(aggregateCondition, mode, pagination)
	case "item-summary":
		return purchaseItemSummaryReadModelQuery(aggregateCondition, pagination)
	case "supplier-summary":
		return purchaseSupplierSummaryReadModelQuery(aggregateCondition, pagination)
	default:
		return purchaseReadModelQuery(aggregateCondition, mode, pagination)
	}
}

func purchaseCalendarSummaryReadModelQuery(aggregateCondition, mode, pagination string) string {
	base := purchaseReadModelQuery(aggregateCondition, "invoice-summary", "")
	bucket := "occurred_at::date"
	if mode == "month-summary" {
		bucket = "date_trunc('month', occurred_at::date)::date"
	}
	return `
		WITH purchase_rows AS (` + base + `),
		grouped_rows AS (
			SELECT ` + bucket + ` AS period,
			       MAX(occurred_at) AS occurred_at,
			       party,
			       COALESCE(SUM(CASE WHEN quantity ~ '^-?[0-9]+([.][0-9]+)?$' THEN quantity::numeric ELSE 0 END), 0) AS quantity,
			       COALESCE(SUM(CASE WHEN amount ~ '^-?[0-9]+([.][0-9]+)?$' THEN amount::numeric ELSE 0 END), 0) AS amount
			FROM purchase_rows
			WHERE occurred_at >= $3::date
			  AND occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
			       OR party ILIKE '%' || $5 || '%'
			       OR item ILIKE '%' || $5 || '%')
			GROUP BY ` + bucket + `, party
		)
		SELECT period::text,
		       occurred_at::text,
		       party,
		       'Purchase summary',
		       quantity::numeric(19,4)::text,
		       amount::numeric(19,4)::text
		FROM grouped_rows
		ORDER BY period DESC, party
		` + pagination
}

// stockReadModelQuery keeps Phase P on the normalized stock authority. Ledger
// rows are joined to their immutable source events so draft/void compatibility
// payloads cannot enter a report. Balance reports use the rebuildable
// stock_balances cache, but require a posted ledger row for the batch.
func stockItemSummaryReadModelQuery(pagination string) string {
	return `
		WITH posted_stock_ledger AS (
			SELECT l.occurred_at::date AS movement_date,
			       b.item_legacy_id,
			       COALESCE(NULLIF(i.name, ''), b.item_legacy_id) AS item_name,
			       COALESCE(g.name, '') AS godown,
			       CASE
			         WHEN l.direction = 'out' THEN -l.quantity
			         WHEN l.direction = 'adjustment' THEN l.quantity * l.adjustment_sign
			         ELSE l.quantity
			       END AS signed_quantity,
			       l.unit_cost
			FROM stock_ledger l
			JOIN sync_events se
			  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
			JOIN stock_batches b
			  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id AND b.id = l.batch_id
			LEFT JOIN master_items i
			  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
			LEFT JOIN master_godowns g
			  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
			WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
			  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
			  AND l.occurred_at >= $3::date
			  AND l.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR b.item_legacy_id ILIKE '%' || $5 || '%'
			       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
			       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
			       OR b.batch_number ILIKE '%' || $5 || '%')
			  AND ($6 = '' OR b.godown_id = $6::uuid)
			  AND ($7 = '' OR b.batch_number ILIKE '%' || $7 || '%')
		)
		SELECT item_legacy_id,
		       movement_date::text,
		       godown,
		       item_name,
		       SUM(signed_quantity)::text,
		       SUM(signed_quantity * unit_cost)::text
		FROM posted_stock_ledger
		GROUP BY item_legacy_id, movement_date, godown, item_name
		ORDER BY movement_date DESC, item_name, godown, item_legacy_id
		` + pagination
}

func stockMovementSummaryReadModelQuery(pagination string) string {
	return `
		WITH posted_stock_ledger AS (
			SELECT l.occurred_at::date AS movement_date,
			       CASE
			         WHEN l.direction = 'out' THEN 'OUT'
			         WHEN l.direction = 'adjustment' THEN 'ADJUSTMENT'
			         ELSE 'IN'
			       END AS movement_direction,
			       CASE
			         WHEN l.direction = 'out' THEN -l.quantity
			         WHEN l.direction = 'adjustment' THEN l.quantity * l.adjustment_sign
			         ELSE l.quantity
			       END AS signed_quantity,
			       l.unit_cost,
			       b.item_legacy_id,
			       b.batch_number,
			       COALESCE(g.name, '') AS godown,
			       COALESCE(NULLIF(i.name, ''), b.item_legacy_id) AS item_name
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
			  AND l.occurred_at >= $3::date
			  AND l.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR l.direction ILIKE '%' || $5 || '%'
			       OR b.item_legacy_id ILIKE '%' || $5 || '%'
			       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
			       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
			       OR b.batch_number ILIKE '%' || $5 || '%')
			  AND ($6 = '' OR b.godown_id = $6::uuid)
			  AND ($7 = '' OR b.batch_number ILIKE '%' || $7 || '%')
		)
		SELECT movement_date::text,
		       movement_direction,
		       godown,
		       item_name,
		       SUM(signed_quantity)::text,
		       SUM(signed_quantity * unit_cost)::text
		FROM posted_stock_ledger
		GROUP BY movement_date, movement_direction, godown, item_name
		ORDER BY movement_date DESC, movement_direction, item_name, godown
		` + pagination
}

func stockSupplierManufacturerReadModelQuery(pagination string) string {
	return `
		SELECT COALESCE(
		         NULLIF(i.payload->>'Manufacturer', ''),
		         NULLIF(i.payload->>'ManfCode', ''),
		         NULLIF(i.payload->>'manufacturer', ''),
		         'Unspecified'
		       ) AS manufacturer,
		       COALESCE(suppliers.supplier_names, 'Unspecified'),
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id) AS item_name,
		       sb.on_hand::text,
		       b.unit_cost::text
		FROM stock_balances sb
		JOIN stock_batches b
		  ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id
		 AND b.id = sb.batch_id
		LEFT JOIN master_godowns g
		  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
		LEFT JOIN master_items i
		  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
		LEFT JOIN LATERAL (
			SELECT string_agg(
			         DISTINCT COALESCE(NULLIF(mp.name, ''), NULLIF(s.legacy_supplier_id, ''), 'Unspecified'),
			         ', ' ORDER BY COALESCE(NULLIF(mp.name, ''), NULLIF(s.legacy_supplier_id, ''), 'Unspecified')
			       ) AS supplier_names
			FROM item_suppliers s
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = s.tenant_id AND mp.id = s.supplier_id
			 AND mp.party_type = 'supplier'
			WHERE s.tenant_id = sb.tenant_id
			  AND s.legacy_item_id = sb.item_legacy_id
		) suppliers ON TRUE
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  AND sb.on_hand <> 0
		  AND sb.updated_at >= $3::date
		  AND sb.updated_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
		       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(i.payload::text, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(suppliers.supplier_names, '') ILIKE '%' || $5 || '%')
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
		ORDER BY manufacturer, item_name, suppliers.supplier_names, g.name, b.batch_number
		` + pagination
}

func stockSalesReadModelQuery(pagination string) string {
	return `
		WITH posted_sales AS (
			SELECT sa.batch_id, SUM(sa.quantity) AS sales_quantity
			FROM stock_allocations sa
			JOIN business_documents bd
			  ON bd.tenant_id = sa.tenant_id AND bd.branch_id = sa.branch_id
			 AND bd.id = sa.document_id
			JOIN business_document_lines bl
			  ON bl.tenant_id = sa.tenant_id AND bl.branch_id = sa.branch_id
			 AND bl.id = sa.document_line_id
			WHERE sa.tenant_id = $1::uuid AND sa.branch_id = $2::uuid
			  AND bd.status = 'posted'
			  AND bd.kind IN ('cash-sale', 'credit-sale')
			  AND bd.occurred_at >= $3::date
			  AND bd.occurred_at < ($4::date + INTERVAL '1 day')
			GROUP BY sa.batch_id
		)
		SELECT b.batch_number,
		       COALESCE(b.expiry_date::text, sb.updated_at::text),
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id),
		       sb.on_hand::text,
		       COALESCE(ps.sales_quantity, 0)::text
		FROM stock_balances sb
		JOIN stock_batches b
		  ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id AND b.id = sb.batch_id
		LEFT JOIN master_godowns g
		  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
		LEFT JOIN master_items i
		  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
		LEFT JOIN posted_sales ps ON ps.batch_id = sb.batch_id
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  -- Stock-and-Sales reports current balance; only posted_sales is
		  -- restricted to the requested sales period above.
		  AND (sb.on_hand <> 0 OR ps.sales_quantity IS NOT NULL)
		  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
		       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%')
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

// stockManagementReadModelQuery keeps Stock Management distinct from the
// alert-oriented level reports. It returns every scoped balance row carrying
// typed threshold fallbacks, so the client can show the source values without
// inventing the legacy alert/status calculation.
func stockManagementReadModelQuery(pagination string) string {
	return `
		SELECT b.batch_number,
		       COALESCE(b.expiry_date::text, sb.updated_at::text),
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id),
		       sb.on_hand::text,
		       CASE
		         WHEN NULLIF(i.payload->>'ReorderQty', '') ~ '^-?[0-9]+([.][0-9]+)?$'
		           THEN (i.payload->>'ReorderQty')::numeric
		         WHEN NULLIF(i.payload->>'ReorderQuantity', '') ~ '^-?[0-9]+([.][0-9]+)?$'
		           THEN (i.payload->>'ReorderQuantity')::numeric
		         ELSE 0::numeric
		       END::text,
		       CASE
		         WHEN NULLIF(i.payload->>'OptimumQty', '') ~ '^-?[0-9]+([.][0-9]+)?$'
		           THEN (i.payload->>'OptimumQty')::numeric
		         WHEN NULLIF(i.payload->>'OptimumQuantity', '') ~ '^-?[0-9]+([.][0-9]+)?$'
		           THEN (i.payload->>'OptimumQuantity')::numeric
		         ELSE 0::numeric
		       END::text,
		       CASE
		         WHEN NULLIF(i.payload->>'MinimumQty', '') ~ '^-?[0-9]+([.][0-9]+)?$'
		           THEN (i.payload->>'MinimumQty')::numeric
		         WHEN NULLIF(i.payload->>'MinimumQuantity', '') ~ '^-?[0-9]+([.][0-9]+)?$'
		           THEN (i.payload->>'MinimumQuantity')::numeric
		         ELSE 0::numeric
		       END::text
		FROM stock_balances sb
		JOIN stock_batches b
		  ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id
		 AND b.id = sb.batch_id
		LEFT JOIN master_godowns g
		  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
		LEFT JOIN master_items i
		  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
		WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
		  AND sb.updated_at >= $3::date
		  AND sb.updated_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
		       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
		       OR i.payload::text ILIKE '%' || $5 || '%')
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
		ORDER BY sb.updated_at DESC, COALESCE(NULLIF(i.name, ''), sb.item_legacy_id), b.batch_number
		` + pagination
}

func stockLevelReadModelQuery(mode, pagination string) string {
	levelPredicate := "reorder_quantity > 0 AND on_hand <= reorder_quantity"
	switch mode {
	case "optimum-level":
		levelPredicate = "optimum_quantity > 0 AND on_hand <= optimum_quantity"
	case "minimum-level":
		levelPredicate = "minimum_quantity > 0 AND on_hand <= minimum_quantity"
	case "reorder-optimum-level":
		levelPredicate = "(reorder_quantity > 0 OR optimum_quantity > 0) AND on_hand <= GREATEST(reorder_quantity, optimum_quantity)"
	}
	return `
		WITH stock_levels AS (
			SELECT b.batch_number,
			       COALESCE(b.expiry_date::text, sb.updated_at::text) AS observed_at,
			       COALESCE(g.name, '') AS godown,
			       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id) AS item_name,
			       sb.on_hand,
			       CASE
			         WHEN NULLIF(i.payload->>'ReorderQty', '') ~ '^-?[0-9]+([.][0-9]+)?$'
			           THEN (i.payload->>'ReorderQty')::numeric
			         WHEN NULLIF(i.payload->>'ReorderQuantity', '') ~ '^-?[0-9]+([.][0-9]+)?$'
			           THEN (i.payload->>'ReorderQuantity')::numeric
			         ELSE 0::numeric
			       END AS reorder_quantity,
			       CASE
			         WHEN NULLIF(i.payload->>'OptimumQty', '') ~ '^-?[0-9]+([.][0-9]+)?$'
			           THEN (i.payload->>'OptimumQty')::numeric
			         WHEN NULLIF(i.payload->>'OptimumQuantity', '') ~ '^-?[0-9]+([.][0-9]+)?$'
			           THEN (i.payload->>'OptimumQuantity')::numeric
			         ELSE 0::numeric
			       END AS optimum_quantity,
			       CASE
			         WHEN NULLIF(i.payload->>'MinimumQty', '') ~ '^-?[0-9]+([.][0-9]+)?$'
			           THEN (i.payload->>'MinimumQty')::numeric
			         WHEN NULLIF(i.payload->>'MinimumQuantity', '') ~ '^-?[0-9]+([.][0-9]+)?$'
			           THEN (i.payload->>'MinimumQuantity')::numeric
			         ELSE 0::numeric
			       END AS minimum_quantity,
			       sb.tenant_id, sb.branch_id, sb.batch_id, sb.item_legacy_id
			FROM stock_balances sb
			JOIN stock_batches b
			  ON b.tenant_id = sb.tenant_id AND b.branch_id = sb.branch_id
			 AND b.id = sb.batch_id
			LEFT JOIN master_godowns g
			  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
			LEFT JOIN master_items i
			  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
			WHERE sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid
			  AND sb.updated_at >= $3::date
			  AND sb.updated_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
			       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
			       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
			       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
			       OR i.payload::text ILIKE '%' || $5 || '%')
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
		)
		SELECT batch_number, observed_at, godown, item_name,
		       on_hand::text, reorder_quantity::text, optimum_quantity::text,
		       minimum_quantity::text
		FROM stock_levels
		WHERE ` + levelPredicate + `
		ORDER BY observed_at DESC, item_name, batch_number, batch_id
		` + pagination
}

// stockExpiryClassReadModelQuery retains the captured class field while using
// the typed batch expiry and normalized posted stock balance authority.
func stockExpiryClassReadModelQuery(pagination string) string {
	return `
		SELECT COALESCE(
		         NULLIF(i.payload->>'Class', ''),
		         NULLIF(i.payload->>'ItemClass', ''),
		         NULLIF(i.payload->>'class', ''),
		         'Unspecified'
		       ) AS item_class,
		       b.expiry_date::text,
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id) AS item_name,
		       sb.on_hand::text,
		       b.unit_cost::text
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
		  AND b.expiry_date IS NOT NULL
		  AND b.expiry_date BETWEEN $3::date AND $4::date
		  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
		       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(i.payload->>'Class', '') ILIKE '%' || $5 || '%'
		       OR COALESCE(i.payload->>'ItemClass', '') ILIKE '%' || $5 || '%')
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
		ORDER BY item_class, b.expiry_date, item_name, g.name, b.batch_number
		` + pagination
}

func stockClassificationReadModelQuery(mode, pagination string) string {
	classificationExpression := "COALESCE(NULLIF(i.payload->>'Class', ''), NULLIF(i.payload->>'ItemClass', ''), NULLIF(i.payload->>'class', ''), 'Unspecified')"
	switch mode {
	case "manufacturer-balance":
		classificationExpression = "COALESCE(NULLIF(i.payload->>'Manufacturer', ''), NULLIF(i.payload->>'ManfCode', ''), NULLIF(i.payload->>'manufacturer', ''), 'Unspecified')"
	case "category-balance":
		classificationExpression = "COALESCE(NULLIF(i.payload->>'Category', ''), NULLIF(i.payload->>'ICatCode', ''), NULLIF(i.payload->>'category', ''), 'Unspecified')"
	}
	return `
		SELECT ` + classificationExpression + ` AS classification,
		       COALESCE(b.expiry_date::text, sb.updated_at::text),
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), sb.item_legacy_id) AS item_name,
		       sb.on_hand::text,
		       b.unit_cost::text
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
		  AND sb.updated_at >= $3::date
		  AND sb.updated_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR b.batch_number ILIKE '%' || $5 || '%'
		       OR sb.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%'
		       OR (` + classificationExpression + `) ILIKE '%' || $5 || '%')
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
		ORDER BY classification, b.expiry_date NULLS LAST, item_name, g.name, b.batch_number
		` + pagination
}

// stockNarcoticsReadModelQuery preserves the captured Item-master narcotics
// boundary without treating every inventory row as narcotic. The source-shaped
// payload keeps the legacy flag and generic identifiers until their exact
// PowerBuilder code/value semantics are approved.
func stockNarcoticsReadModelQuery(mode, pagination string) string {
	const postedNarcoticsRows = `
		WITH posted_narcotics_rows AS (
			SELECT l.id::text AS movement_id,
			       l.occurred_at,
			       l.occurred_at::date AS movement_date,
			       l.direction,
			       l.quantity,
			       l.adjustment_sign,
			       l.unit_cost,
			       b.item_legacy_id,
			       b.batch_number,
			       COALESCE(g.name, '') AS godown,
			       COALESCE(NULLIF(i.name, ''), b.item_legacy_id) AS item_name,
			       COALESCE(
			         NULLIF(i.payload->>'GenericName', ''),
			         NULLIF(i.payload->>'GenericCode', ''),
			         NULLIF(i.payload->>'genericName', ''),
			         'Unspecified'
			       ) AS generic_type
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
			  AND LOWER(COALESCE(
			        NULLIF(i.payload->>'Narcotics', ''),
			        NULLIF(i.payload->>'Narcotic', ''),
			        NULLIF(i.payload->>'narcotics', ''),
			        ''
			      )) IN ('yes', 'y', 'true', '1')
			  AND ($6 = '' OR b.godown_id = $6::uuid)
			  AND ($7 = '' OR b.batch_number ILIKE '%' || $7 || '%')
		)
`
	if mode == "narcotics-generic" {
		return postedNarcoticsRows + `
		SELECT generic_type,
		       movement_date::text,
		       godown,
		       item_name,
		       SUM(CASE
				 WHEN direction = 'out' THEN -quantity
				 WHEN direction = 'adjustment' THEN quantity * adjustment_sign
				 ELSE quantity
			   END)::text,
		       SUM((CASE
				 WHEN direction = 'out' THEN -quantity
				 WHEN direction = 'adjustment' THEN quantity * adjustment_sign
				 ELSE quantity
			   END) * unit_cost)::text
		FROM posted_narcotics_rows
		WHERE occurred_at >= $3::date
		  AND occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR generic_type ILIKE '%' || $5 || '%'
		       OR movement_id ILIKE '%' || $5 || '%'
		       OR item_legacy_id ILIKE '%' || $5 || '%'
		       OR item_name ILIKE '%' || $5 || '%'
		       OR godown ILIKE '%' || $5 || '%'
		       OR batch_number ILIKE '%' || $5 || '%')
		GROUP BY generic_type, movement_date, godown, item_name
		ORDER BY movement_date DESC, generic_type, item_name, godown
		` + pagination
	}
	return postedNarcoticsRows + `
		SELECT movement_id,
		       occurred_at::text,
		       direction,
		       item_name,
		       (CASE
				 WHEN direction = 'out' THEN -quantity
				 WHEN direction = 'adjustment' THEN quantity * adjustment_sign
				 ELSE quantity
			   END)::text,
		       unit_cost::text
		FROM posted_narcotics_rows
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

func stockReadModelQuery(mode, pagination string) string {
	if isStockItemSummaryMode(mode) {
		return stockItemSummaryReadModelQuery(pagination)
	}
	if isStockMovementSummaryMode(mode) {
		return stockMovementSummaryReadModelQuery(pagination)
	}
	if isStockSupplierManufacturerMode(mode) {
		return stockSupplierManufacturerReadModelQuery(pagination)
	}
	if isStockSalesMode(mode) {
		return stockSalesReadModelQuery(pagination)
	}
	if isStockLevelMode(mode) {
		return stockLevelReadModelQuery(mode, pagination)
	}
	if isStockManagementMode(mode) {
		return stockManagementReadModelQuery(pagination)
	}
	if isStockExpiryClassMode(mode) {
		return stockExpiryClassReadModelQuery(pagination)
	}
	if isStockClassificationMode(mode) {
		return stockClassificationReadModelQuery(mode, pagination)
	}
	if isStockNarcoticsMode(mode) {
		return stockNarcoticsReadModelQuery(mode, pagination)
	}
	if mode == "movement" || mode == "narcotics-movement" {
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

// historicalStockReadModelQuery is the bounded StockReport projection used by
// the captured Back Date leaf. The source table has no batch identity, so the
// source row key is retained as the document column rather than inventing one.
func historicalStockReadModelQuery(pagination string) string {
	return `
		SELECT h.legacy_id,
		       h.as_of::text,
		       COALESCE(g.name, ''),
		       COALESCE(NULLIF(i.name, ''), h.item_legacy_id),
		       h.quantity::text,
		       h.purchase_price::text,
		       h.sale_price::text,
		       h.average_price::text,
		       h.recent_purchase_price::text,
		       h.pack_units::text
		FROM historical_stock_snapshots h
		LEFT JOIN master_godowns g
		  ON g.tenant_id = h.tenant_id AND g.id = h.godown_id
		LEFT JOIN master_items i
		  ON i.tenant_id = h.tenant_id AND i.id = h.item_id
		WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid
		  AND h.as_of >= $3::date
		  AND h.as_of < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR h.legacy_id ILIKE '%' || $5 || '%'
		       OR h.item_legacy_id ILIKE '%' || $5 || '%'
		       OR COALESCE(i.name, '') ILIKE '%' || $5 || '%'
		       OR COALESCE(g.name, '') ILIKE '%' || $5 || '%')
		  AND ($6 = '' OR h.godown_id = $6::uuid)
		  AND ($7 = '' OR h.legacy_id ILIKE '%' || $7 || '%')
		ORDER BY h.as_of DESC, h.legacy_id
		` + pagination
}

func financeReadModelQuery(mode, pagination string) string {
	switch mode {
	case "gl", "gl-cash":
		accountFilter := ""
		if mode == "gl-cash" {
			accountFilter = "AND a.system_key = 'cash'"
		}
		historicalRows := ""
		if mode == "gl" {
			historicalRows = `

				UNION ALL

				SELECT 'historical:' || h.legacy_id,
				       h.occurred_at,
				       CASE
				         WHEN NULLIF(a.code, '') IS NOT NULL
				           THEN a.code || ' - ' || COALESCE(NULLIF(a.name, ''), 'Historical account')
				         ELSE 'Historical ' || COALESCE(NULLIF(h.account_code, ''), '(unmapped)')
				       END,
				       h.remarks, h.debit_amount, h.credit_amount
				FROM historical_gl_entries h
				LEFT JOIN finance_accounts a
				  ON a.tenant_id = h.tenant_id AND a.code = h.account_code
				WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid`
		}
		return `
			WITH ledger_rows AS (
				SELECT j.id::text AS row_id, j.posted_at AS occurred_at,
				       a.code || ' - ' || a.name AS account_name,
				       l.memo, l.debit_amount, l.credit_amount
				FROM gl_journals j
				JOIN gl_lines l
				  ON l.tenant_id = j.tenant_id AND l.branch_id = j.branch_id
				 AND l.journal_id = j.id
				JOIN finance_accounts a
				  ON a.tenant_id = l.tenant_id AND a.id = l.account_id
				WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
				  AND j.status = 'posted'
				  ` + accountFilter + `
			` + historicalRows + `
			)
			SELECT row_id, occurred_at::text, account_name, memo,
			       debit_amount::text, credit_amount::text
			FROM ledger_rows
			WHERE occurred_at >= $3::date
			  AND occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR row_id ILIKE '%' || $5 || '%'
			       OR account_name ILIKE '%' || $5 || '%'
			       OR memo ILIKE '%' || $5 || '%')
			ORDER BY occurred_at DESC, row_id
			` + pagination
	case "trial-balance":
		return `
			WITH balances AS (
				SELECT a.code AS account_code, j.posted_at AS occurred_at, c.name AS category_name,
				       a.name AS account_name, l.debit_amount, l.credit_amount
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
				UNION ALL
				SELECT COALESCE(NULLIF(a.code, ''), NULLIF(h.account_code, ''), '(unmapped)') AS account_code,
				       h.occurred_at, COALESCE(c.name, 'Historical') AS category_name,
				       COALESCE(NULLIF(a.name, ''), 'Legacy account ' || COALESCE(NULLIF(h.account_code, ''), '(unmapped)')) AS account_name,
				       h.debit_amount, h.credit_amount
				FROM historical_gl_entries h
				LEFT JOIN finance_accounts a
				  ON a.tenant_id = h.tenant_id AND a.code = h.account_code
				LEFT JOIN finance_account_categories c
				  ON c.tenant_id = a.tenant_id AND c.code = a.category_code
				WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid
				  AND h.occurred_at >= $3::date
				  AND h.occurred_at < ($4::date + INTERVAL '1 day')
			)
			SELECT account_code, MAX(occurred_at)::text, category_name, account_name,
			       COALESCE(SUM(debit_amount), 0)::text,
			       COALESCE(SUM(credit_amount), 0)::text
			FROM balances
			WHERE ($5 = '' OR account_code ILIKE '%' || $5 || '%'
			       OR account_name ILIKE '%' || $5 || '%'
			       OR category_name ILIKE '%' || $5 || '%')
			GROUP BY account_code, category_name, account_name
			ORDER BY account_code
			` + pagination
	case "party-customer", "party-supplier":
		partyKind := "customer"
		if mode == "party-supplier" {
			partyKind = "supplier"
		}
		return `
			WITH entries AS (
				SELECT ple.id::text AS row_id,
				       ple.source_document_id::text AS document_id,
				       ple.occurred_at,
				       COALESCE(mp.name, '') AS party_name,
				       ple.description,
				       ple.debit_amount,
				       ple.credit_amount
				FROM party_ledger_entries ple
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = ple.tenant_id AND mp.id = ple.party_id
				 AND mp.party_type = '` + partyKind + `'
				WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
				  AND ple.counterparty_kind = '` + partyKind + `'
				  AND (
					EXISTS (
						SELECT 1
						FROM gl_journals j
						WHERE j.tenant_id = ple.tenant_id AND j.branch_id = ple.branch_id
						  AND j.source_document_id = ple.source_document_id
						  AND j.status = 'posted'
					)
					OR EXISTS (
						SELECT 1
						FROM business_documents historical_d
						WHERE historical_d.tenant_id = ple.tenant_id
						  AND historical_d.branch_id = ple.branch_id
						  AND historical_d.id = ple.source_document_id
						  AND historical_d.status = 'posted'
						  AND historical_d.legacy_source_table <> ''
					)
				  )
				  AND ple.occurred_at >= $3::date
				  AND ple.occurred_at < ($4::date + INTERVAL '1 day')
				UNION ALL
				SELECT p.id::text AS row_id,
				       COALESCE(p.source_document_id::text,
				                NULLIF(p.source_document_legacy_id, ''), p.payment_code) AS document_id,
				       p.occurred_at,
				       COALESCE(mp.name, p.party_legacy_id, '') AS party_name,
				       COALESCE(NULLIF(p.remarks, ''), 'Payment ' || p.payment_code) AS description,
				       CASE WHEN '` + partyKind + `' = 'supplier' THEN p.payment_amount ELSE 0 END AS debit_amount,
				       CASE WHEN '` + partyKind + `' = 'customer' THEN p.payment_amount ELSE 0 END AS credit_amount
				FROM historical_party_payment_allocations p
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = p.tenant_id AND mp.id = p.party_id
				 AND mp.party_type = '` + partyKind + `'
				WHERE p.tenant_id = $1::uuid AND p.branch_id = $2::uuid
				  AND p.counterparty_kind = '` + partyKind + `'
				  AND p.posted = true
				  AND p.payment_amount <> 0
				  AND p.occurred_at >= $3::date
				  AND p.occurred_at < ($4::date + INTERVAL '1 day')
				UNION ALL
				SELECT a.id::text AS row_id,
				       COALESCE(a.source_document_id::text,
				                NULLIF(a.source_document_legacy_id, ''), a.source_legacy_id) AS document_id,
				       a.occurred_at,
				       COALESCE(mp.name, a.party_legacy_id, '') AS party_name,
				       COALESCE(NULLIF(a.remarks, ''), 'Receivable adjustment ' || a.source_legacy_id) AS description,
				       a.debit_amount,
				       a.credit_amount
				FROM historical_party_ledger_adjustments a
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = a.tenant_id AND mp.id = a.party_id
				 AND mp.party_type = '` + partyKind + `'
				WHERE a.tenant_id = $1::uuid AND a.branch_id = $2::uuid
				  AND a.counterparty_kind = '` + partyKind + `'
				  AND a.posted = true
				  AND (a.debit_amount <> 0 OR a.credit_amount <> 0)
				  AND a.occurred_at IS NOT NULL
				  AND a.occurred_at >= $3::date
				  AND a.occurred_at < ($4::date + INTERVAL '1 day')
				UNION ALL
				SELECT r.id::text AS row_id,
				       COALESCE(r.source_document_id::text,
				                NULLIF(r.source_document_legacy_id, ''), r.return_source_legacy_id) AS document_id,
				       r.occurred_at,
				       COALESCE(mp.name, r.party_legacy_id, '') AS party_name,
				       'Return allocation ' || r.return_kind || ' ' || r.return_source_legacy_id AS description,
				       r.debit_amount,
				       r.credit_amount
				FROM historical_party_return_allocations r
				LEFT JOIN master_parties mp
				  ON mp.tenant_id = r.tenant_id AND mp.id = r.party_id
				 AND mp.party_type = '` + partyKind + `'
				WHERE r.tenant_id = $1::uuid AND r.branch_id = $2::uuid
				  AND r.counterparty_kind = '` + partyKind + `'
				  AND r.posted = true
				  AND r.allocation_amount <> 0
				  AND r.occurred_at >= $3::date
				  AND r.occurred_at < ($4::date + INTERVAL '1 day')
			)
			SELECT document_id, occurred_at::text, party_name, description,
			       debit_amount::text, credit_amount::text
			FROM entries
			WHERE ($5 = '' OR document_id ILIKE '%' || $5 || '%'
			       OR party_name ILIKE '%' || $5 || '%'
			       OR description ILIKE '%' || $5 || '%')
			ORDER BY occurred_at, row_id
			` + pagination
	case "aging-receivable", "aging-payable":
		partyKind := "customer"
		if mode == "aging-payable" {
			partyKind = "supplier"
		}
		if mode == "aging-receivable" {
			return `
			SELECT ple.party_id::text, MAX(ple.occurred_at)::text,
			       COALESCE(mp.name, ''),
			       CASE
			         WHEN due.due_date IS NULL THEN 'UNAGED - due date unavailable'
			         WHEN due.due_date > $4::date THEN 'NOT DUE'
			         WHEN ($4::date - due.due_date) <= 30 THEN '0-30 days'
			         WHEN ($4::date - due.due_date) <= 60 THEN '31-60 days'
			         WHEN ($4::date - due.due_date) <= 90 THEN '61-90 days'
			         ELSE '91+ days'
			       END,
			       COALESCE(SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE 0 END), 0)::text,
			       COALESCE(SUM(CASE WHEN ple.credit_amount > 0 THEN d.balance_amount ELSE 0 END), 0)::text
			FROM party_ledger_entries ple
			JOIN business_documents d
			  ON d.tenant_id = ple.tenant_id AND d.branch_id = ple.branch_id
			 AND d.id = ple.source_document_id
			 AND d.status = 'posted'
			LEFT JOIN LATERAL (
				SELECT CASE
				         WHEN COALESCE(NULLIF(d.legacy_payload->>'DueDate', ''), NULLIF(d.pricing_result->>'dueDate', '')) ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}'
				         THEN LEFT(COALESCE(NULLIF(d.legacy_payload->>'DueDate', ''), NULLIF(d.pricing_result->>'dueDate', '')), 10)::date
				       END AS due_date
			) due ON TRUE
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = ple.tenant_id AND mp.id = ple.party_id
			 AND mp.party_type = 'customer'
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = 'customer'
			  AND d.kind IN ('cash-sale', 'credit-sale', 'cash-sale-return', 'credit-sale-return', 'open-sale-return')
			  AND (
				EXISTS (
					SELECT 1
					FROM gl_journals j
					WHERE j.tenant_id = ple.tenant_id AND j.branch_id = ple.branch_id
					  AND j.source_document_id = ple.source_document_id
					  AND j.status = 'posted'
				)
				OR EXISTS (
					SELECT 1
					FROM business_documents historical_d
					WHERE historical_d.tenant_id = ple.tenant_id
					  AND historical_d.branch_id = ple.branch_id
					  AND historical_d.id = ple.source_document_id
					  AND historical_d.status = 'posted'
					  AND historical_d.legacy_source_table <> ''
				)
			  )
			  AND ple.occurred_at >= $3::date
			  AND ple.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR d.document_number ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR ple.party_id::text ILIKE '%' || $5 || '%'
			       OR ple.description ILIKE '%' || $5 || '%')
			GROUP BY ple.party_id, mp.name, due.due_date
			HAVING SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE -d.balance_amount END) <> 0
			ORDER BY mp.name, ple.party_id, due.due_date
			` + pagination
		}
		return `
			SELECT ple.party_id::text, MAX(ple.occurred_at)::text,
			       COALESCE(mp.name, ''),
			       CASE
			         WHEN due.due_date IS NULL THEN 'UNAGED - due date unavailable'
			         WHEN due.due_date > $4::date THEN 'NOT DUE'
			         WHEN ($4::date - due.due_date) <= 30 THEN '0-30 days'
			         WHEN ($4::date - due.due_date) <= 60 THEN '31-60 days'
			         WHEN ($4::date - due.due_date) <= 90 THEN '61-90 days'
			         ELSE '91+ days'
			       END,
		       COALESCE(SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE 0 END), 0)::text,
		       COALESCE(SUM(CASE WHEN ple.credit_amount > 0 THEN d.balance_amount ELSE 0 END), 0)::text
			FROM party_ledger_entries ple
			JOIN business_documents d
			  ON d.tenant_id = ple.tenant_id AND d.branch_id = ple.branch_id
			 AND d.id = ple.source_document_id
			 AND d.status = 'posted'
			LEFT JOIN LATERAL (
				SELECT CASE
					         WHEN d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase')
					              AND COALESCE(NULLIF(d.legacy_payload->>'CreditDays', ''), NULLIF(d.pricing_result->>'creditDays', '')) ~ '^-?[0-9]+$'
					              AND COALESCE(NULLIF(d.legacy_payload->>'CreditDays', ''), NULLIF(d.pricing_result->>'creditDays', ''))::numeric BETWEEN -36500 AND 36500
					         THEN d.occurred_at::date + COALESCE(NULLIF(d.legacy_payload->>'CreditDays', ''), NULLIF(d.pricing_result->>'creditDays', ''))::integer
				       END AS due_date
			) due ON TRUE
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = ple.tenant_id AND mp.id = ple.party_id
			 AND mp.party_type = '` + partyKind + `'
			WHERE ple.tenant_id = $1::uuid AND ple.branch_id = $2::uuid
			  AND ple.counterparty_kind = '` + partyKind + `'
			  AND d.kind IN ('pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return')
			  AND (
				EXISTS (
					SELECT 1
					FROM gl_journals j
					WHERE j.tenant_id = ple.tenant_id AND j.branch_id = ple.branch_id
					  AND j.source_document_id = ple.source_document_id
					  AND j.status = 'posted'
				)
				OR EXISTS (
					SELECT 1
					FROM business_documents historical_d
					WHERE historical_d.tenant_id = ple.tenant_id
					  AND historical_d.branch_id = ple.branch_id
					  AND historical_d.id = ple.source_document_id
					  AND historical_d.status = 'posted'
					  AND historical_d.legacy_source_table <> ''
				)
			  )
			  AND ple.occurred_at >= $3::date
			  AND ple.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR d.document_number ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR ple.party_id::text ILIKE '%' || $5 || '%')
			GROUP BY ple.party_id, mp.name, due.due_date
			HAVING SUM(CASE WHEN ple.debit_amount > 0 THEN d.balance_amount ELSE -d.balance_amount END) <> 0
			ORDER BY mp.name, ple.party_id, due.due_date
			` + pagination
	case "tax-output", "tax-input", "tax-advance", "tax-advance-input":
		kindFilter := "'cash-sale', 'credit-sale'"
		if mode == "tax-input" || mode == "tax-advance-input" {
			kindFilter = "'pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return'"
		}
		if mode == "tax-advance" || mode == "tax-advance-input" {
			return `
			SELECT d.document_number, d.occurred_at::text,
			       COALESCE(mp.name, ''), l.item_name,
			       COALESCE(advance_tax.base, l.line_total)::text,
			       advance_tax.amount::text
			FROM business_documents d
			JOIN business_document_lines l
			  ON l.tenant_id = d.tenant_id AND l.branch_id = d.branch_id
			 AND l.document_id = d.id
			LEFT JOIN LATERAL (
				SELECT COALESCE(
				           SUM(CASE WHEN NULLIF(tax.value->>'amount', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
				                    THEN (tax.value->>'amount')::numeric END),
				           CASE WHEN l.line_number = (
				                         SELECT MIN(l2.line_number)
				                         FROM business_document_lines l2
				                         WHERE l2.tenant_id = d.tenant_id
				                           AND l2.branch_id = d.branch_id
				                           AND l2.document_id = d.id
				                       )
				                     AND NULLIF(d.legacy_payload->>'AdvanceTaxAmt', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
				                THEN (d.legacy_payload->>'AdvanceTaxAmt')::numeric END,
				           0
				       ) AS amount,
				       COALESCE(MAX(CASE WHEN NULLIF(tax.value->>'rate', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
				                         THEN (tax.value->>'rate')::numeric END), l.advance_tax_rate) AS rate,
				       COALESCE(MAX(CASE WHEN NULLIF(tax.value->>'base', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
				                         THEN (tax.value->>'base')::numeric END), l.line_total) AS base
				FROM jsonb_array_elements(
					CASE WHEN jsonb_typeof(l.pricing->'taxes') = 'array'
					     THEN l.pricing->'taxes' ELSE '[]'::jsonb END
				) AS tax(value)
				WHERE tax.value->>'kind' = 'advance_tax'
			) advance_tax ON TRUE
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = d.tenant_id
			 AND mp.id = CASE WHEN d.kind IN ('cash-sale', 'credit-sale')
			                  THEN d.customer_id ELSE d.supplier_id END
			WHERE d.tenant_id = $1::uuid AND d.branch_id = $2::uuid
			  AND d.status = 'posted' AND d.kind IN (` + kindFilter + `)
			  AND advance_tax.amount > 0 AND l.advance_tax_rate > 0
			  AND d.occurred_at >= $3::date
			  AND d.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR d.document_number ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR l.item_name ILIKE '%' || $5 || '%')
			ORDER BY d.occurred_at DESC, d.document_number, l.line_number
			` + pagination
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
			  AND l.tax_amount > 0
			  AND d.occurred_at >= $3::date
			  AND d.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR d.document_number ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%'
			       OR l.item_name ILIKE '%' || $5 || '%')
			ORDER BY d.occurred_at DESC, d.document_number, l.line_number
			` + pagination
	case "tax-withholding":
		return `
			SELECT h.payment_code,
			       h.occurred_at::text,
			       COALESCE(NULLIF(mp.name, ''), NULLIF(h.supplier_legacy_id, ''), ''),
			       COALESCE(NULLIF(h.purchase_invoice_code, ''), NULLIF(h.check_number, ''), h.remarks),
			       h.taxable_base::text,
			       h.amount::text
			FROM historical_withholding_tax_entries h
			LEFT JOIN business_documents d
			  ON d.tenant_id = h.tenant_id AND d.branch_id = h.branch_id
			 AND d.legacy_source_table = 'Purledger'
			 AND d.legacy_id = h.purchase_invoice_code
			LEFT JOIN master_parties mp
			  ON mp.tenant_id = d.tenant_id AND mp.id = d.supplier_id
			WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid
			  AND h.posted = true AND h.amount <> 0
			  AND h.occurred_at >= $3::date
			  AND h.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR h.payment_code ILIKE '%' || $5 || '%'
			       OR h.purchase_invoice_code ILIKE '%' || $5 || '%'
			       OR h.supplier_legacy_id ILIKE '%' || $5 || '%'
			       OR h.check_number ILIKE '%' || $5 || '%'
			       OR h.remarks ILIKE '%' || $5 || '%'
			       OR COALESCE(mp.name, '') ILIKE '%' || $5 || '%')
			ORDER BY h.occurred_at DESC, h.payment_code
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

// historicalGLReadModelQuery joins the imported VirtualGl projection with
// newly posted normalized journals. The source fields remain explicit; no
// account-name or opening-balance semantics are inferred for VirtualGl rows.
func historicalGLReadModelQuery(pagination string) string {
	return `
		WITH gl_rows AS (
			SELECT h.legacy_id AS document,
			       h.occurred_at,
			       h.document_type,
			       h.account_code,
			       h.alternate_account_code,
			       h.invoice_code,
			       h.user_legacy_id,
			       h.remarks,
			       h.debit_amount::text AS debit_amount,
			       h.credit_amount::text AS credit_amount
			FROM historical_gl_entries h
			WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid

			UNION ALL

			SELECT 'canonical:' || j.id::text,
			       j.posted_at,
			       j.kind,
			       COALESCE(a.code, ''),
			       '',
			       COALESCE(j.source_document_id::text, ''),
			       '',
			       l.memo,
			       l.debit_amount::text,
			       l.credit_amount::text
			FROM gl_journals j
			JOIN gl_lines l
			  ON l.tenant_id = j.tenant_id AND l.branch_id = j.branch_id
			 AND l.journal_id = j.id
			JOIN finance_accounts a
			  ON a.tenant_id = l.tenant_id AND a.id = l.account_id
			WHERE j.tenant_id = $1::uuid AND j.branch_id = $2::uuid
			  AND j.status = 'posted'
		)
		SELECT document, occurred_at::text, document_type, account_code,
		       alternate_account_code, invoice_code, user_legacy_id, remarks,
		       debit_amount, credit_amount
		FROM gl_rows
		WHERE occurred_at >= $3::date
		  AND occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR document ILIKE '%' || $5 || '%'
		       OR document_type ILIKE '%' || $5 || '%'
		       OR account_code ILIKE '%' || $5 || '%'
		       OR alternate_account_code ILIKE '%' || $5 || '%'
		       OR invoice_code ILIKE '%' || $5 || '%'
		       OR user_legacy_id ILIKE '%' || $5 || '%'
		       OR remarks ILIKE '%' || $5 || '%')
		ORDER BY occurred_at DESC, document, account_code
		` + pagination
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

func historicalReportQuery(mode, pagination string) string {
	if mode == "deleted-sale-items" {
		return `
			SELECT COALESCE(NULLIF(h.sale_invoice_code, ''), h.source_table_row),
			       h.occurred_at::text,
			       concat_ws(' / ', NULLIF(h.machine_name, ''), NULLIF(h.user_legacy_id, '')),
			       COALESCE(NULLIF(mi.name, ''), h.item_legacy_id) ||
			         CASE WHEN h.godown_legacy_id = '' THEN '' ELSE ' / ' || h.godown_legacy_id END,
			       (h.quantity + h.bonus_quantity)::text,
			       h.sale_price::text
			FROM historical_deleted_sale_items h
			LEFT JOIN master_items mi
			  ON mi.tenant_id = h.tenant_id AND mi.id = h.item_id
			WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid
			  AND h.occurred_at >= $3::date
			  AND h.occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR h.sale_invoice_code ILIKE '%' || $5 || '%'
			       OR h.item_legacy_id ILIKE '%' || $5 || '%'
			       OR h.godown_legacy_id ILIKE '%' || $5 || '%'
			       OR h.machine_name ILIKE '%' || $5 || '%'
			       OR h.user_legacy_id ILIKE '%' || $5 || '%'
			       OR h.payload::text ILIKE '%' || $5 || '%')
			ORDER BY h.occurred_at DESC, h.source_table_row
			` + pagination
	}
	if mode == "stock-adjustments" {
		return `
			WITH adjustment_rows AS (
				SELECT h.adjustment_legacy_id AS document,
				       h.occurred_at,
				       concat_ws(' / ', NULLIF(h.godown_legacy_id, ''), NULLIF(h.user_legacy_id, '')) AS party,
				       h.item_legacy_id || CASE WHEN h.batch = '' THEN '' ELSE ' / ' || h.batch END AS item,
				       h.loose_quantity::text AS quantity,
				       h.price::text AS amount,
				       concat_ws(' ', h.payload::text, h.adjustment_legacy_id, h.item_legacy_id, h.godown_legacy_id, h.batch) AS search_text
				FROM historical_stock_adjustment_lines h
				WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid
				  AND h.occurred_at >= $3::date
				  AND h.occurred_at < ($4::date + INTERVAL '1 day')

				UNION ALL

				SELECT COALESCE(NULLIF(se.payload->>'reference', ''),
				               NULLIF(se.payload->>'documentNumber', ''), se.event_id::text),
				       l.occurred_at,
				       concat_ws(' / ', NULLIF(g.name, ''), NULLIF(COALESCE(u.display_name, u.username, se.payload->>'userLegacyId', ''), '')),
				       COALESCE(NULLIF(i.name, ''), b.item_legacy_id) || CASE WHEN b.batch_number = '' THEN '' ELSE ' / ' || b.batch_number END,
				       (l.quantity * l.adjustment_sign)::text,
				       l.unit_cost::text,
				       concat_ws(' ', se.payload::text, se.event_id::text, b.item_legacy_id, b.batch_number, g.name, i.name)
				FROM stock_ledger l
				JOIN sync_events se
				  ON se.tenant_id = l.tenant_id AND se.event_id = l.source_event_id
				JOIN stock_batches b
				  ON b.tenant_id = l.tenant_id AND b.branch_id = l.branch_id AND b.id = l.batch_id
				LEFT JOIN master_godowns g
				  ON g.tenant_id = b.tenant_id AND g.id = b.godown_id
				LEFT JOIN master_items i
				  ON i.tenant_id = b.tenant_id AND i.id = b.item_id
				LEFT JOIN users u
				  ON u.tenant_id = se.tenant_id AND u.id = se.operator_id
				WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
				  AND l.direction = 'adjustment'
				  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
				  AND l.occurred_at >= $3::date
				  AND l.occurred_at < ($4::date + INTERVAL '1 day')
			)
			SELECT document, occurred_at::text, party, item, quantity, amount
			FROM adjustment_rows
			WHERE occurred_at >= $3::date
			  AND occurred_at < ($4::date + INTERVAL '1 day')
			  AND ($5 = '' OR search_text ILIKE '%' || $5 || '%')
			ORDER BY occurred_at DESC, document, item
			` + pagination
	}
	return `
		SELECT h.source_legacy_id,
		       h.occurred_at::text,
		       concat_ws(' / ', NULLIF(h.user_legacy_id, ''), NULLIF(h.change_reason, '')),
		       h.item_legacy_id,
		       CASE WHEN h.report_kind IN ('item-price-difference', 'item-sale-price')
		            THEN COALESCE(h.old_sale_price::text, h.old_name, '')
		            ELSE h.old_name END,
		       CASE WHEN h.report_kind IN ('item-price-difference', 'item-sale-price')
		            THEN COALESCE(h.new_sale_price::text, h.new_name, '')
		            ELSE h.new_name END
		FROM historical_item_changes h
		WHERE h.tenant_id = $1::uuid AND h.branch_id = $2::uuid
		  AND h.report_kind = $6
		  AND h.occurred_at >= $3::date
		  AND h.occurred_at < ($4::date + INTERVAL '1 day')
		  AND ($5 = '' OR h.source_legacy_id ILIKE '%' || $5 || '%'
		       OR h.item_legacy_id ILIKE '%' || $5 || '%'
		       OR h.user_legacy_id ILIKE '%' || $5 || '%'
		       OR h.change_reason ILIKE '%' || $5 || '%'
		       OR h.payload::text ILIKE '%' || $5 || '%')
		ORDER BY h.occurred_at DESC, h.source_legacy_id
		` + pagination
}

func emptyReportQuery(pagination string) string {
	return `
		SELECT ''::text, ''::text, ''::text, ''::text, ''::text, ''::text
		WHERE false
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
	requestedFormat := strings.TrimSpace(r.URL.Query().Get("format"))
	selectedFormat, formatOK := selectReportFormat(definition, requestedFormat)
	if !formatOK {
		writeProblem(w, http.StatusBadRequest, "invalid_report_format", "Invalid report format", "The requested report format is not available for this report.")
		return
	}
	definition.SelectedFormat = selectedFormat
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
	appendDailyDetailRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(
				&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity, &row.Amount,
				&row.Alias, &row.ItemDescription, &row.SalePrice, &row.DiscountPercent,
				&row.DiscountValue, &row.ItemDiscount, &row.SalesTaxValue, &row.ExpiryDate, &row.BatchNumber,
			); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return result.Err()
	}
	appendSalesProfitMarginRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(
				&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity,
				&row.SalePrice, &row.Amount, &row.PurchasePrice, &row.GrossProfit,
				&row.MarginPercent, &row.SalesTaxValue,
			); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return result.Err()
	}
	appendPurchaseLineDetailRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(
				&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity,
				&row.PurchasePrice, &row.DiscountPercent, &row.DiscountValue,
				&row.SalesTaxValue, &row.Amount, &row.ExpiryDate, &row.BatchNumber,
			); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return result.Err()
	}
	appendHistoricalStockRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(
				&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity,
				&row.PurchasePrice, &row.SalePrice, &row.AveragePrice,
				&row.RecentPurchasePrice, &row.PackUnits,
			); scanErr != nil {
				return scanErr
			}
			row.Amount = row.AveragePrice
			rows = append(rows, row)
		}
		return result.Err()
	}
	appendStockLevelRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(
				&row.Document, &row.OccurredAt, &row.Party, &row.Item, &row.Quantity,
				&row.ReorderQuantity, &row.OptimumQuantity, &row.MinimumQuantity,
			); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return result.Err()
	}
	appendHistoricalGLRows := func(query string, args ...any) error {
		result, queryErr := tx.QueryContext(reportCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer result.Close()
		for result.Next() {
			var row reportRow
			if scanErr := result.Scan(
				&row.Document, &row.OccurredAt, &row.DocumentType, &row.Party,
				&row.AlternateAccountCode, &row.InvoiceCode, &row.UserLegacyID,
				&row.Item, &row.Quantity, &row.Amount,
			); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return result.Err()
	}

	var query string
	switch kind {
	case "daily-sales-detail":
		query = dailySalesDetailReadModelQuery("LIMIT $6 OFFSET $7")
		if err = appendDailyDetailRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
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
				JOIN sync_events se0
				  ON se0.tenant_id = l.tenant_id AND se0.event_id = l.source_event_id
				WHERE l.tenant_id = $1::uuid AND l.branch_id = $2::uuid
				  AND COALESCE(NULLIF(se0.payload->>'status', ''), 'posted') = 'posted'
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
				  AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
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
			if spec.stockMode == "historical-balance" {
				query = historicalStockReadModelQuery("LIMIT $8 OFFSET $9")
				err = appendHistoricalStockRows(query, operator.TenantID, operator.BranchID, from, to, filter, godownID, batchNumber, pageSize+1, (page-1)*pageSize)
			} else if isStockLevelMode(spec.stockMode) || isStockManagementMode(spec.stockMode) {
				query = stockLevelReadModelQuery(spec.stockMode, "LIMIT $8 OFFSET $9")
				if isStockManagementMode(spec.stockMode) {
					query = stockManagementReadModelQuery("LIMIT $8 OFFSET $9")
				}
				err = appendStockLevelRows(query, operator.TenantID, operator.BranchID, from, to, filter, godownID, batchNumber, pageSize+1, (page-1)*pageSize)
			} else {
				query = stockReadModelQuery(spec.stockMode, "LIMIT $8 OFFSET $9")
				err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, godownID, batchNumber, pageSize+1, (page-1)*pageSize)
			}
			if err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The normalized stock report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.documentReadModel {
			query = documentReadModelQuery(spec.documentKind, spec.documentAggregate, spec.documentMode, "LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The canonical document report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.headerReadModel {
			query = headerTransactionReadModelQuery("LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The canonical header transaction report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.purchaseReadModel && spec.financeMode == "" {
			query = purchaseReadModelQueryMode(spec.aggregateCondition, spec.purchaseMode, "LIMIT $6 OFFSET $7")
			appendPurchaseRows := appendRows
			if spec.purchaseMode == "line-detail" {
				query = purchaseLineDetailReadModelQuery(spec.aggregateCondition, "LIMIT $6 OFFSET $7")
				appendPurchaseRows = appendPurchaseLineDetailRows
			}
			if err = appendPurchaseRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The purchase report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.salesReadModel && spec.financeMode == "" {
			appendSalesRows := appendRows
			if spec.aggregateCondition == reportSaleReturnAggregate {
				query = saleReturnReadModelQueryMode(spec.salesMode, "LIMIT $6 OFFSET $7")
			} else {
				query = salesReadModelQueryMode(spec.aggregateCondition, spec.salesMode, "LIMIT $6 OFFSET $7")
			}
			if spec.salesMode == "line-detail" {
				appendSalesRows = appendDailyDetailRows
			} else if salesSummaryUsesProfitMarginScanner(spec.salesMode) {
				appendSalesRows = appendSalesProfitMarginRows
			}
			if err = appendSalesRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The sales report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.financeMode != "" {
			if spec.financeMode == "historical-gl" {
				query = historicalGLReadModelQuery("LIMIT $6 OFFSET $7")
				err = appendHistoricalGLRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize)
			} else {
				query = financeReadModelQuery(spec.financeMode, "LIMIT $6 OFFSET $7")
				err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize)
			}
			if err != nil {
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
		if spec, ok := reportSpecForKey(registryKey); ok && spec.historyMode != "" {
			query = historicalReportQuery(spec.historyMode, func() string {
				if spec.historyMode == "stock-adjustments" || spec.historyMode == "deleted-sale-items" {
					return "LIMIT $6 OFFSET $7"
				}
				return "LIMIT $7 OFFSET $8"
			}())
			if spec.historyMode == "stock-adjustments" || spec.historyMode == "deleted-sale-items" {
				err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize)
			} else {
				err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, spec.historyMode, pageSize+1, (page-1)*pageSize)
			}
			if err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The source-backed historical report query failed.")
				return
			}
			break
		}
		if spec, ok := reportSpecForKey(registryKey); ok && spec.emptyProjection {
			query = emptyReportQuery("LIMIT $6 OFFSET $7")
			if err = appendRows(query, operator.TenantID, operator.BranchID, from, to, filter, pageSize+1, (page-1)*pageSize); err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "report_read_failed", "Unable to read report", "The empty item-history fallback query failed.")
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
			AND COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'
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

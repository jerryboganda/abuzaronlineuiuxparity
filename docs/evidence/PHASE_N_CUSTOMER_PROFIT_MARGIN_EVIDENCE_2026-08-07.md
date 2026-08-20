# Customer Sales profit margin detail evidence - 2026-08-07

## Scope

This is a bounded source-backed projection for the captured Customer Sales
Invoice Wise Profit Margin Detail leaf. It does not claim exact PowerBuilder
valuation, return, discount, tax, or print semantics.

## Implemented

- Posted canonical cash/credit sale lines are read with sale price, line
  amount, tax, and FIFO stock-allocation cost when allocations exist.
- Compatibility sale rows are de-duplicated against canonical documents and
  use a cost only when the retained payload explicitly supplies purchase,
  cost, or unit-cost data.
- Gross profit is exposed as sales amount less tax and available cost;
  margin is gross profit divided by pre-tax sales amount. Rows without a
  source cost leave profit and margin blank rather than fabricating a value.
- Daily Sales Summary with Profit (Day wise grouping) reuses these rows by
  calendar day and customer, exposes an average sale price, and emits cost,
  profit, and margin only when all contributing rows have source cost.
- Customer Sales > Customer Category Wise Sales > Customer Wise Gross Profit
  now reuses the same rows grouped by customer, exposes the last-posted date
  and average sale price, and applies the same all-contributing-rows cost gate.
- Customer Sales > Customer Category Wise Sales > Customer Wise Summary and
  Net Sales and Volume now use de-duplicated invoice rows grouped by customer,
  exposing last-posted date, volume, and authoritative net-sales amount.
- Customer Sales > Customer Category Wise Sales > Customer Category Wise Net
  Sales and its two captured compatibility aliases now use a six-field
  category aggregate over canonical and de-duplicated compatibility rows.
  Category values prefer the retained customer master payload keys
  `Category`, `CustomerCategory`, and `category`, with `Unspecified` as the
  explicit no-category bucket.
- Customer Sales > Customer Wise Category Net Sales now uses the same bounded
  source rows grouped by both retained customer and category, with a six-field
  Customer/Date/Customer/Category/Volume/Net Sales contract.
- Customer Category Wise Sales Detail Report now resolves to the source-backed
  11-field sale line-detail projection, including retained alias, item,
  pricing, discount, tax, amount, expiry, and batch values.
- The report exposes an 11-field typed contract and keeps tenant, branch,
  posted-only, date, text, pagination, and compatibility de-duplication
  boundaries in the query.

## Verification evidence

Focused API contract checks passed:

    go test ./services/api/internal/httpapi -run 'Test(CustomerSalesProfitMarginReadModelUsesAllocatedCost|DailySalesProfitSummaryAggregatesCompleteCostRows|CustomerSalesGrossProfitSummaryGroupsByCustomer|SalesProfitSummaryModesUseExtendedScanner|CustomerSalesSummaryReadModelsUseExplicitBuckets|CustomerSalesCategorySummaryUsesCustomerCategoryPayload|CustomerWiseCategorySummaryGroupsByCustomerAndCategory|CustomerCategorySalesDetailReportUsesLineDetailProjection|PhaseNReportRegistryDefinitionsAndAggregateFilters|InvoiceSummaryReadModelsGroupRowsOncePerDocument)$' -count=1

The Svelte workspace check passed with 0 errors and 0 warnings:

    cmd /c pnpm --filter @abuzar/web check

No database-backed report replay was claimed because DATABASE_URL was unset.

## Remaining boundary

The original PowerBuilder margin source columns, FIFO versus average/legacy
valuation policy, discount inclusion, return/net treatment, category joins,
day grouping, tax basis, rounding, formatting, print/PDF/workbook output,
migrated golden data, and operator acceptance still require captured source
evidence. The category projection currently joins compatibility customer
names/codes/legacy IDs to the retained master-party category payload; exact
legacy category and return semantics are not claimed.

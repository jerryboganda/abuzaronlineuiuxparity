import type { ReportDefinition, ReportExportFormat } from '@abuzar/contracts';

const defaultLetterhead = {
  name: "Fazal Din's Pharma Plus",
  line2: 'NRY Pacific',
  line3: "Franchise Fazal Din's",
  phone: '055 3252501',
  fax: '',
  source: 'default' as const
};

const dailySaleFormats = [
  'Standard',
  'Standard Format2',
  'Standard Format3',
  'Daily Sales Detail with Pack Qty',
  'Standard (with Patient Column)',
  "A Specified Group's Sale + Tax Info.",
  'With Patient, Customer and Remarks',
  'Selected Patient',
  'Standard (pack qty, pack rate, etc)',
  'Format 6 (with CNIC without discounts)'
];

const columns = [
  { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Customer/Supplier', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
];

const dailySaleDetailColumns = [
  { key: 'alias', label: 'Alias', dataType: 'text' as const, sortable: true },
  { key: 'itemDescription', label: 'Item Description', dataType: 'text' as const, sortable: true },
  { key: 'salePrice', label: 'Sale Price', dataType: 'currency' as const, sortable: true },
  { key: 'quantity', label: 'Qty', dataType: 'number' as const, sortable: true },
  { key: 'discountPercent', label: 'Disc(%)', dataType: 'number' as const, sortable: true },
  { key: 'discountValue', label: 'Discount Value', dataType: 'currency' as const, sortable: true },
  { key: 'itemDiscount', label: 'Item Disc', dataType: 'currency' as const, sortable: true },
  { key: 'salesTaxValue', label: 'SalesTax Value', dataType: 'currency' as const, sortable: true },
  { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true },
  { key: 'expiryDate', label: 'Expiry Date', dataType: 'date' as const, sortable: true },
  { key: 'batchNumber', label: 'Batch Number', dataType: 'text' as const, sortable: true }
];

const purchaseLineDetailColumns = [
  { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
  { key: 'purchasePrice', label: 'Purchase Price', dataType: 'currency' as const, sortable: true },
  { key: 'discountPercent', label: 'Disc(%)', dataType: 'number' as const, sortable: true },
  { key: 'discountValue', label: 'Discount Value', dataType: 'currency' as const, sortable: true },
  { key: 'salesTaxValue', label: 'Sales Tax', dataType: 'currency' as const, sortable: true },
  { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true },
  { key: 'expiryDate', label: 'Expiry Date', dataType: 'date' as const, sortable: true },
  { key: 'batchNumber', label: 'Batch Number', dataType: 'text' as const, sortable: true }
];

const purchaseSummaryColumns = (mode: string) => {
  if (mode === 'invoice-summary') return [
    { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'day-summary') return [
    { key: 'document', label: 'Day', dataType: 'date' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'month-summary') return [
    { key: 'document', label: 'Month', dataType: 'date' as const, sortable: true },
    { key: 'occurredAt', label: 'Month', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'item-summary') return [
    { key: 'document', label: 'Item', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'supplier-summary') return [
    { key: 'document', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  return salesInvoiceSummaryColumns;
};

const eventLedgerColumns = [
  { key: 'document', label: 'Event / Document', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Occurred', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Party', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item (first payload line)', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity (payload)', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Amount (payload)', dataType: 'currency' as const, sortable: true }
];

const documentReportColumns = (mode: string) => mode === 'document-invoice-summary'
  ? [
    { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ]
  : columns;

const headerReportColumns = [
  { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Customer/Supplier', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Transaction Type', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
];

const salesInvoiceSummaryColumns = [
  { key: 'document', label: 'Invoice', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
];

const salesSummaryColumns = (mode: string) => {
  if (mode === 'profit-margin-detail') return [
    { key: 'document', label: 'Invoice', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'salePrice', label: 'Sale Price', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Sales Amount', dataType: 'currency' as const, sortable: true },
    { key: 'purchasePrice', label: 'Cost', dataType: 'currency' as const, sortable: true },
    { key: 'grossProfit', label: 'Gross Profit', dataType: 'currency' as const, sortable: true },
    { key: 'marginPercent', label: 'Margin %', dataType: 'number' as const, sortable: true },
    { key: 'salesTaxValue', label: 'Sales Tax', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'profit-day-summary') return [
    { key: 'document', label: 'Day', dataType: 'date' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'salePrice', label: 'Average Sale Price', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Sales Amount', dataType: 'currency' as const, sortable: true },
    { key: 'purchasePrice', label: 'Cost', dataType: 'currency' as const, sortable: true },
    { key: 'grossProfit', label: 'Gross Profit', dataType: 'currency' as const, sortable: true },
    { key: 'marginPercent', label: 'Margin %', dataType: 'number' as const, sortable: true },
    { key: 'salesTaxValue', label: 'Sales Tax', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'profit-customer-summary') return [
    { key: 'document', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'salePrice', label: 'Average Sale Price', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Sales Amount', dataType: 'currency' as const, sortable: true },
    { key: 'purchasePrice', label: 'Cost', dataType: 'currency' as const, sortable: true },
    { key: 'grossProfit', label: 'Gross Profit', dataType: 'currency' as const, sortable: true },
    { key: 'marginPercent', label: 'Margin %', dataType: 'number' as const, sortable: true },
    { key: 'salesTaxValue', label: 'Sales Tax', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'customer-summary') return [
    { key: 'document', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Volume', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Net Sales', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'customer-category-summary') return [
    { key: 'document', label: 'Customer Category', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer Category', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Category Total', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Volume', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Net Sales', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'customer-wise-category-summary') return [
    { key: 'document', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Category', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Volume', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Net Sales', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'day-summary') return [
    { key: 'document', label: 'Day', dataType: 'date' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'hour-summary') return [
    { key: 'document', label: 'Hour', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'item-summary') return [
    { key: 'document', label: 'Item', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Last Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'month-summary') return [
    { key: 'document', label: 'Month', dataType: 'date' as const, sortable: true },
    { key: 'occurredAt', label: 'Month', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Summary', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Amount', dataType: 'currency' as const, sortable: true }
  ];
  return salesInvoiceSummaryColumns;
};

const phaseNReportKinds = new Set([
  'sale-detail', 'sale-summary', 'sale-summary-inv-wise', 'sale-summary-inv-cust-wise',
  'sale-detail-inv-wise', 'sale-detail-format-2', 'refused-sales-detail',
  'sale-detail-inv-wise-with-diff-col', 'sale-summary-invoice-wise',
  'sale-summary-machine-and-invoice-range-wise', 'selected-sales-and-summaries-report',
  'sales-return-detail', 'sales-return-summary', 'sales-return-summary-inv-wise',
  'sales-return-detail-inv-wise',
  'customer-sales-detail', 'customer-sales-summary', 'customer-sales-days-summary',
  'customer-sales-invoice-wise-profit-margin-detail', 'customer-sales-items-summary',
  'customer-sales-invoice-summary', 'customer-sales-hourly-graph', 'customer-sales-lp-ledger',
  'customer-sales-customer-category-wise-net-sales',
  'customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report',
  'customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report',
  'customer-sales-customer-category-wise-sales-net-sales-and-volume',
  'customer-sales-customer-category-wise-sales-customer-wise-summary',
  'customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report',
  'customer-sales-customer-category-wise-sales-output-sales-tax-report',
  'customer-sales-customer-category-wise-sales-customer-wise-gross-profit',
  'customer-sales-customer-wise-category-net-sales', 'customer-sales-monthly-net-sales',
  'customer-sales-claimable-for-allowed-customers',
  'customer-sales-customer-ntn-wise-sales-tax-report', 'customer-sales-customer-wise-advance-tax',
  'category-wise-sale-and-return', 'category-wise-sales', 'category-wise-deviated-items',
  'category-wise-monthly-sale', 'category-wise-net-sale', 'category-wise-gross-profit',
  'category-wise-item-wise-sale-discounts-detail', 'category-wise-item-category-wise-monthly-sales',
  'category-wise-category-wise-day-net-sale', 'manufacturer-wise-sales',
  'manufacturer-wise-sales-detail-and-summary', 'manufacturer-wise-net-sales',
  'manufacturer-wise-item-sales-discount',
  'manufacturer-wise-manufacturer-wise-sales-and-return-summary',
  'manufacturer-wise-cnic-ntn-registered-customers-sales', 'user-wise-invoice-graph',
  'user-wise-sales', 'user-wise-category-summary', 'user-wise-discount-report', 'user-wise-net-cash',
  'user-wise-sales-commission', 'user-wise-user-wise-sales-summary', 'hourly-sales-graph',
  'slow-fast-moving-items', 'net-sale-summary', 'item-wise-item-sale-and-return-activity',
  'item-wise-item-wise-net-sales', 'sale-return-summary-inv-type-wise',
  'daily-sales-summary-with-profit-day-wise-grouping', 'monthly-net-sales-summary',
  'dead-item-list', 'sales-tax-report'
]);

const phaseOReportKinds = new Set([
  'purchase-detail', 'purchase-summary', 'purchase-summary2',
  'purchase-return-detail', 'purchase-return-summary',
  'purchase-order-summary', 'p-o-based-purchase-disparity',
  'periodic-purchases', 'purchase-order', 'supplier-wise-detail',
  'supplier-wise-purchase-detail', 'supplier-wise-advance-income-tax',
  'manufacturer-wise-detail', 'manufacturer-wise-monthly-stock-movement',
  'monthly-purchase-graph', 'category-wise-purchase', 'days-summary',
  'purchase-order-supplier-wise', 'net-purchase-summary',
  'supplier-category-wise-input-sales-tax-report', 'withholding-tax-deduction',
  'supplier-manufacturer-wise-g-p', 'supplier-purchase-returns-detail',
  'supplier-purchase-returns-summary'
]);

const phaseOReportTitles: Record<string, string> = {
  'purchase-detail': 'Purchase detail',
  'purchase-summary': 'Purchase summary',
  'purchase-summary2': 'Purchase Summary2',
  'purchase-return-detail': 'Purchase Return detail',
  'purchase-return-summary': 'Purchase Return summary',
  'purchase-order-summary': 'Purchase Order Summary',
  'p-o-based-purchase-disparity': 'P/O Based Purchase Disparity',
  'periodic-purchases': 'Periodic Purchases',
  'purchase-order': 'Purchase Order',
  'supplier-wise-detail': 'Detail',
  'supplier-wise-purchase-detail': 'Purchase Detail',
  'supplier-wise-advance-income-tax': 'Advance Income Tax',
  'manufacturer-wise-detail': 'Detail',
  'manufacturer-wise-monthly-stock-movement': 'Monthly Stock Movement',
  'monthly-purchase-graph': 'Monthly Purchase Graph',
  'category-wise-purchase': 'Category Wise Purchase',
  'days-summary': 'Days Summary',
  'purchase-order-supplier-wise': 'Purchase Order Supplier Wise',
  'net-purchase-summary': 'Net Purchase Summary',
  'supplier-category-wise-input-sales-tax-report': 'Input Sales Tax Report',
  'withholding-tax-deduction': 'Withholding Tax Deduction',
  'supplier-manufacturer-wise-g-p': 'Supplier/Manufacturer Wise G/P',
  'supplier-purchase-returns-detail': 'Detail',
  'supplier-purchase-returns-summary': 'Summary'
};

const purchaseSummaryReportModes: Record<string, string> = {
  'purchase-summary': 'invoice-summary',
  'purchase-summary2': 'invoice-summary',
  'purchase-return-summary': 'invoice-summary',
  'purchase-order-summary': 'invoice-summary',
  'purchase-order': 'invoice-summary',
  'periodic-purchases': 'month-summary',
  'manufacturer-wise-monthly-stock-movement': 'month-summary',
  'monthly-purchase-graph': 'month-summary',
  'category-wise-purchase': 'item-summary',
  'days-summary': 'day-summary',
  'purchase-order-supplier-wise': 'supplier-summary',
  'net-purchase-summary': 'invoice-summary',
  'supplier-category-wise-input-sales-tax-report': 'invoice-summary',
  'withholding-tax-deduction': 'invoice-summary',
  'supplier-manufacturer-wise-g-p': 'supplier-summary',
  'supplier-purchase-returns-summary': 'supplier-summary'
};

const phasePReportKinds = new Set([
  'stock-in-hand-manufacturer-wise', 'stock-in-hand-category-wise', 'stock-in-hand-others',
  'stock-in-hand-class-wise', 'stock-in-hand-batch-priority-wise', 'stock-in-hand-back-date',
  'stock-in-hand-manufacturer-wise-format2', 'stock-in-hand-supplier-manufacturer-association',
  'stock-in-hand-stock-quantity-format', 'stock-in-hand-stock-in-hand-audit-purpose',
  'stock-in-hand-batch-priority-wise-audit-purposes', 'expiry-report', 'reorder-level-report',
  'stock-register', 'item-stock-register-summary', 'stock-register-for-narcotics',
  'stock-and-sales', 'optimum-level-report', 'item-activity',
  'stock-register-narcotics-format2', 'expiry-report-class-wise', 'minimum-level-report',
  'reorder-optimum-level-report', 'daily-stock-in-out', 'stock-in-out-date-wise',
  'stock-management-report', 'norcotics-stock-register-generic-type-wise'
]);

const stockLevelReportKinds = new Set([
  'reorder-level-report', 'optimum-level-report', 'minimum-level-report', 'reorder-optimum-level-report'
]);

const stockManagementReportKinds = new Set(['stock-management-report']);

const stockNarcoticsGenericReportKinds = new Set([
  'norcotics-stock-register-generic-type-wise'
]);

const stockMovementSummaryReportKinds = new Set([
  'daily-stock-in-out', 'stock-in-out-date-wise'
]);

const stockSupplierManufacturerReportKinds = new Set([
  'stock-in-hand-supplier-manufacturer-association'
]);

const stockExpiryClassReportKinds = new Set([
  'expiry-report-class-wise'
]);

const stockClassificationReportLabels: Record<string, string> = {
  'stock-in-hand-manufacturer-wise': 'Manufacturer',
  'stock-in-hand-manufacturer-wise-format2': 'Manufacturer',
  'stock-in-hand-category-wise': 'Category',
  'stock-in-hand-class-wise': 'Class'
};

const phasePReportTitles: Record<string, string> = {
  'stock-in-hand-manufacturer-wise': 'Manufacturer wise',
  'stock-in-hand-category-wise': 'Category wise',
  'stock-in-hand-others': 'Others',
  'stock-in-hand-class-wise': 'Class Wise',
  'stock-in-hand-batch-priority-wise': 'Batch, Priority Wise',
  'stock-in-hand-back-date': 'Back Date',
  'stock-in-hand-manufacturer-wise-format2': 'Manufacturer Wise (Format2)',
  'stock-in-hand-supplier-manufacturer-association': 'Supplier Manufacturer Association',
  'stock-in-hand-stock-quantity-format': 'Stock Quantity Format',
  'stock-in-hand-stock-in-hand-audit-purpose': 'Stock in Hand - Audit Purpose',
  'stock-in-hand-batch-priority-wise-audit-purposes': 'Batch, Priority Wise - Audit Purposes',
  'expiry-report': 'Expiry Report',
  'reorder-level-report': 'Reorder Level Report',
  'stock-register': 'Stock Register',
  'item-stock-register-summary': 'Item Stock Register Summary',
  'stock-register-for-narcotics': 'Stock Register(For Narcotics)',
  'stock-and-sales': 'Stock and Sales',
  'optimum-level-report': 'Optimum Level Report',
  'item-activity': 'Item Activity',
  'stock-register-narcotics-format2': 'Stock Register(Narcotics Format2)',
  'expiry-report-class-wise': 'Expiry Report(Class Wise)',
  'minimum-level-report': 'Minimum Level Report',
  'reorder-optimum-level-report': 'Reorder/Optimum Level Report',
  'daily-stock-in-out': 'Daily Stock IN/OUT',
  'stock-in-out-date-wise': 'Stock IN/OUT(Date Wise)',
  'stock-management-report': 'Stock Management Report',
  'norcotics-stock-register-generic-type-wise': 'Norcotics Stock Register-Generic Type Wise'
};

const phaseQReportModes: Record<string, string> = {
  'adjustment-adjustment-summary': 'adjustment',
  'adjustment-adjustment-detail': 'adjustment',
  'adjustment-adjustment-summary-inv-wise': 'adjustment',
  'adjustment-adjustment-detail-inv-wise': 'adjustment',
  'adjustment-adjustment-summary-detail': 'adjustment',
  'adjustment-item-wise-adjustment-summary': 'adjustment',
  'quotation-detail': 'document-line-detail',
  'quotation-summary': 'document-invoice-summary',
  'header-wise-transaction-summary': 'header-summary',
  'accounts-reports-ledger-reports-accounts-ledger': 'gl',
  'listing-supplier-list': 'admin',
  'listing-items-list': 'admin',
  'listing-manufacturer-list': 'admin',
  'listing-group-rights-list': 'admin',
  'listing-item-list-class-wise': 'admin',
  'listing-groupwise-user-list': 'admin',
  'listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict': 'admin',
  'reprinting-sale': 'sales',
  'reprinting-purchase': 'purchases',
  'reprinting-sale-with-summary-reports': 'sales',
  'reprinting-sale-format-2': 'sales',
  'reprinting-sale-format-3': 'sales',
  'reprinting-sale-format-4': 'sales',
  'reprinting-sale-with-header-wise-summaries': 'sales',
  'reprinting-selected-sales-and-summaries': 'sales',
  'item-reports-deleted-sale-items-log': 'history-deleted-sale-items',
  'item-reports-history-sale-price-difference': 'history-item-price-difference',
  'item-reports-history-item-basic-data-changes': 'history-item-basic-data',
  'item-reports-history-item-sale-price-changes': 'history-item-sale-price',
  'item-reports-history-new-item-s-created-defined': 'history-item-first-observed',
  'item-reports-history-item-name-changes': 'history-item-name',
  'item-reports-stock-adjustments-stock-adjustments-detail': 'history-stock-adjustments'
};

const reprintSaleDetailReportKinds = new Set([
  'reprinting-sale', 'reprinting-sale-format-2', 'reprinting-sale-format-3', 'reprinting-sale-format-4'
]);

const reprintSaleSummaryReportKinds = new Set([
  'reprinting-sale-with-summary-reports',
  'reprinting-sale-with-header-wise-summaries',
  'reprinting-selected-sales-and-summaries'
]);

const reprintPurchaseDetailReportKinds = new Set(['reprinting-purchase']);

const salesSummaryReportModes: Record<string, string> = {
  'customer-sales-days-summary': 'day-summary',
  'customer-sales-items-summary': 'item-summary',
  'customer-sales-invoice-summary': 'invoice-summary',
  'customer-sales-invoice-wise-profit-margin-detail': 'profit-margin-detail',
  'customer-sales-hourly-graph': 'hour-summary',
  'customer-sales-customer-category-wise-sales-customer-wise-gross-profit': 'profit-customer-summary',
  'customer-sales-customer-category-wise-sales-customer-wise-summary': 'customer-summary',
  'customer-sales-customer-category-wise-sales-net-sales-and-volume': 'customer-summary',
  'customer-sales-customer-category-wise-net-sales': 'customer-category-summary',
  'customer-sales-customer-category-wise-sales-customer-category-wise-sales-summary-report': 'customer-category-summary',
  'customer-sales-customer-category-wise-sales-customer-category-wise-net-sales-report': 'customer-category-summary',
  'customer-sales-customer-wise-category-net-sales': 'customer-wise-category-summary',
  'daily-sales-summary-with-profit-day-wise-grouping': 'profit-day-summary',
  'customer-sales-monthly-net-sales': 'month-summary',
  'monthly-net-sales-summary': 'month-summary'
};

const salesLineDetailReportKinds = new Set([
  'sale-detail',
  'sales-return-detail',
  'customer-sales-customer-category-wise-sales-customer-category-wise-sales-detail-report'
]);

const phaseQReportTitles: Record<string, string> = {
  'gl-journal': 'GL Journal',
  'trial-balance': 'Trial Balance',
  'customer-statement': 'Customer Statement',
  'supplier-statement': 'Supplier Statement',
  'receivables-aging': 'Receivables Aging',
  'payables-aging': 'Payables Aging',
  'tax-register': 'Tax Register',
  'voucher-register': 'Voucher Register',
  'adjustment-adjustment-summary': 'Adjustment Summary',
  'adjustment-adjustment-detail': 'Adjustment Detail',
  'adjustment-adjustment-summary-inv-wise': 'Adjustment Summary Inv. Wise',
  'adjustment-adjustment-detail-inv-wise': 'Adjustment Detail Inv. wise',
  'adjustment-adjustment-summary-detail': 'Adjustment Summary/Detail',
  'adjustment-item-wise-adjustment-summary': 'Item Wise Adjustment Summary',
  'quotation-detail': 'Detail',
  'quotation-summary': 'Summary',
  'header-wise-transaction-summary': 'Header Wise Transaction Summary',
  'accounts-reports-ledger-reports-accounts-ledger': 'Accounts Ledger',
  'listing-supplier-list': 'Supplier List',
  'listing-items-list': 'Items List',
  'listing-manufacturer-list': 'Manufacturer List',
  'listing-group-rights-list': 'Group Rights List',
  'listing-item-list-class-wise': 'Item List Class Wise',
  'listing-groupwise-user-list': 'GroupWise User List',
  'listing-sale-person-scope-manufacturer-sub-area-wise-sales-person-conflict': 'Manufacturer/Sub Area Wise Sales Person Conflict',
  'reprinting-sale': 'Sale',
  'reprinting-purchase': 'Purchase',
  'reprinting-sale-with-summary-reports': 'Sale (with summary reports)',
  'reprinting-sale-format-2': 'Sale Format(2)',
  'reprinting-sale-format-3': 'Sale Format(3)',
  'reprinting-sale-format-4': 'Sale Format(4)',
  'reprinting-sale-with-header-wise-summaries': 'Sale (with header wise summaries)',
  'reprinting-selected-sales-and-summaries': 'Selected Sales and Summaries',
  'item-reports-deleted-sale-items-log': 'Deleted Sale Items Log',
  'item-reports-history-sale-price-difference': 'Sale Price Difference',
  'item-reports-history-item-basic-data-changes': 'Item Basic Data Changes',
  'item-reports-history-item-sale-price-changes': 'Item Sale Price Changes',
  'item-reports-history-new-item-s-created-defined': 'New Item(s) Created/Defined',
  'item-reports-history-item-name-changes': 'Item Name Changes',
  'item-reports-stock-adjustments-stock-adjustments-detail': 'Stock Adjustments Detail'
};

const phaseQFinancialOverrides: Record<string, string> = {
  'customer-sales-lp-ledger': 'party-customer',
  'customer-sales-customer-category-wise-sales-output-sales-tax-report': 'tax-output',
  'customer-sales-customer-ntn-wise-sales-tax-report': 'tax-output',
  'customer-sales-customer-wise-advance-tax': 'tax-advance',
  'sales-tax-report': 'tax-output',
  'supplier-wise-advance-income-tax': 'tax-advance-input',
  'supplier-category-wise-input-sales-tax-report': 'tax-input',
  'withholding-tax-deduction': 'tax-withholding',
  'user-wise-net-cash': 'gl-cash'
};

const phaseQAliases: Record<string, string> = {
  'gl-journal': 'historical-gl',
  'trial-balance': 'trial-balance',
  'customer-statement': 'party-customer',
  'supplier-statement': 'party-supplier',
  'receivables-aging': 'aging-receivable',
  'payables-aging': 'aging-payable',
  'tax-register': 'tax-output',
  'voucher-register': 'voucher'
};

const financeColumns = (mode: string) => {
  if (mode === 'historical-gl') return [
    { key: 'document', label: 'Document Code', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'documentType', label: 'Document Type', dataType: 'text' as const, sortable: true },
    { key: 'party', label: 'Account Code', dataType: 'text' as const, sortable: true },
    { key: 'alternateAccountCode', label: 'Alternate Account', dataType: 'text' as const, sortable: true },
    { key: 'invoiceCode', label: 'Invoice Code', dataType: 'text' as const, sortable: true },
    { key: 'userLegacyId', label: 'User Code', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Remarks', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Debit', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Credit', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'gl' || mode === 'gl-cash') return [
    { key: 'document', label: 'Journal', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Posted', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Account', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Memo', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Debit', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Credit', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'trial-balance') return [
    { key: 'document', label: 'Account', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'As Of', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Category', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Account Name', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Debit', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Credit', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'party-customer' || mode === 'party-supplier') return [
    { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer/Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Description', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Debit', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Credit', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'aging-receivable' || mode === 'aging-payable') return [
    { key: 'document', label: 'Party', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'As Of', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Party Type', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Aging Status', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Debit Total', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Credit Total', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'tax-withholding') return [
    { key: 'document', label: 'Payment', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Purchase Invoice / Certificate', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Withholding Base', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Withholding Amount', dataType: 'currency' as const, sortable: true }
  ];
  if (mode === 'voucher') return [
    { key: 'document', label: 'Voucher', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Created', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Category', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Description', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Amount', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Status', dataType: 'text' as const, sortable: true }
  ];
  return [
    { key: 'document', label: 'Document', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Customer/Supplier', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Item / Tax Snapshot', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Taxable Base', dataType: 'currency' as const, sortable: true },
    { key: 'amount', label: 'Tax Amount', dataType: 'currency' as const, sortable: true }
  ];
};

const stockBalanceColumns = [
  { key: 'document', label: 'Batch', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Expiry/Updated', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'On Hand', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Unit Cost', dataType: 'currency' as const, sortable: true }
];

const stockMovementColumns = [
  { key: 'document', label: 'Movement', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Direction', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Unit Cost', dataType: 'currency' as const, sortable: true }
];

const stockLevelColumns = [
  { key: 'document', label: 'Batch', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Expiry/Updated', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'On Hand', dataType: 'number' as const, sortable: true },
  { key: 'reorderQuantity', label: 'Reorder Qty', dataType: 'number' as const, sortable: true },
  { key: 'optimumQuantity', label: 'Optimum Qty', dataType: 'number' as const, sortable: true },
  { key: 'minimumQuantity', label: 'Minimum Qty', dataType: 'number' as const, sortable: true }
];

const stockItemSummaryColumns = [
  { key: 'document', label: 'Item Code', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Net Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Net Value', dataType: 'currency' as const, sortable: true }
];

const stockMovementSummaryColumns = [
  { key: 'document', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'occurredAt', label: 'Direction', dataType: 'text' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Net Value', dataType: 'currency' as const, sortable: true }
];

const stockSupplierManufacturerColumns = [
  { key: 'document', label: 'Manufacturer', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Supplier(s)', dataType: 'text' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'On Hand', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Unit Cost', dataType: 'currency' as const, sortable: true }
];

const stockSalesColumns = [
  { key: 'document', label: 'Batch', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Expiry/Updated', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'On Hand', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Sales Qty', dataType: 'number' as const, sortable: true }
];

const stockNarcoticsGenericColumns = [
  { key: 'document', label: 'Generic Type', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Net Quantity', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Net Value', dataType: 'currency' as const, sortable: true }
];

const stockExpiryClassColumns = [
  { key: 'document', label: 'Class', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Expiry Date', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'On Hand', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Unit Cost', dataType: 'currency' as const, sortable: true }
];

const stockClassificationColumns = (label: string) => [
  { key: 'document', label, dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Expiry/Updated', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'On Hand', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Unit Cost', dataType: 'currency' as const, sortable: true }
];

const stockHistoricalColumns = [
  { key: 'document', label: 'Source Row', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'As Of', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Godown', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Stock', dataType: 'number' as const, sortable: true },
  { key: 'purchasePrice', label: 'Purchase Price', dataType: 'currency' as const, sortable: true },
  { key: 'salePrice', label: 'Sale Price', dataType: 'currency' as const, sortable: true },
  { key: 'averagePrice', label: 'Average Price', dataType: 'currency' as const, sortable: true },
  { key: 'recentPurchasePrice', label: 'Recent Purchase Price', dataType: 'currency' as const, sortable: true },
  { key: 'packUnits', label: 'Pack Units', dataType: 'number' as const, sortable: true }
];

const historicalColumns = (mode: string) => mode === 'history-deleted-sale-items'
  ? [
    { key: 'document', label: 'Sale Invoice', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Deleted At', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Machine / User', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Item / Godown', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Qty + Bonus', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Sale Price', dataType: 'currency' as const, sortable: true }
  ]
  : mode === 'history-stock-adjustments'
  ? [
    { key: 'document', label: 'Adjustment', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Date', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Godown / User', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Item / Batch', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Loose Quantity', dataType: 'number' as const, sortable: true },
    { key: 'amount', label: 'Adjustment Price', dataType: 'currency' as const, sortable: true }
  ]
  : [
    { key: 'document', label: 'Source Row', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Observed', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'User / Reason', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Item', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Previous', dataType: 'text' as const, sortable: true },
    { key: 'amount', label: 'Current', dataType: 'text' as const, sortable: true }
  ];

function reportRegistryKey(kind: string, legacyPath = ''): string {
  if (phaseNReportKinds.has(kind) || phaseOReportKinds.has(kind) || phasePReportKinds.has(kind) || Object.prototype.hasOwnProperty.call(phaseQReportModes, kind) || Object.prototype.hasOwnProperty.call(phaseQAliases, kind) || kind === 'daily-sales-detail') return kind;
  const segments = legacyPath.split(' > ').map((segment) => segment.replace(/\t.*$/, '').replace(/&/g, '').trim());
  if (segments.length < 3 || segments[0] !== 'Reports') return kind;
  let reportSegments = segments.slice(1);
  if (reportSegments[0] === 'Daily Reports') {
    reportSegments = reportSegments.slice(1);
    if (reportSegments.length > 1 && ['Sale', 'Sales Return', 'Purchase', 'Purchase Return', 'Purchase Order'].includes(reportSegments[0])) {
      reportSegments = reportSegments.slice(1);
    }
  } else if (['Sales Reports', 'Purchase Reports', 'Purchase Return Reports', 'Stock Reports'].includes(reportSegments[0])) {
    reportSegments = reportSegments.slice(1);
  }
  const candidate = reportSegments.join(' ').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  return phaseNReportKinds.has(candidate) || phaseOReportKinds.has(candidate) || phasePReportKinds.has(candidate) || Object.prototype.hasOwnProperty.call(phaseQReportModes, candidate) ? candidate : kind;
}

export function defaultReportDefinition(kind: string, title?: string, legacyPath = ''): ReportDefinition {
  const isDailySaleDetail = kind === 'daily-sales-detail';
  const scopedKind = reportRegistryKey(kind, legacyPath);
  const isLineDetail = salesLineDetailReportKinds.has(scopedKind);
  const salesSummaryMode = salesSummaryReportModes[scopedKind];
  const isSalesSummaryProjection = Boolean(salesSummaryMode);
  const isPurchaseLineDetail = scopedKind === 'purchase-detail' || scopedKind === 'purchase-return-detail';
  const purchaseSummaryMode = purchaseSummaryReportModes[scopedKind];
  const isPurchaseSummaryProjection = Boolean(purchaseSummaryMode);
  const isReprintSaleDetailProjection = reprintSaleDetailReportKinds.has(scopedKind);
  const isReprintSaleSummaryProjection = reprintSaleSummaryReportKinds.has(scopedKind);
  const isReprintPurchaseDetailProjection = reprintPurchaseDetailReportKinds.has(scopedKind);
  const hasReprintProjection = isReprintSaleDetailProjection || isReprintSaleSummaryProjection || isReprintPurchaseDetailProjection;
  const hasEventProjection = phaseNReportKinds.has(scopedKind) || phaseOReportKinds.has(scopedKind);
  const hasStockProjection = phasePReportKinds.has(scopedKind);
  const isStockLevelReport = stockLevelReportKinds.has(scopedKind);
  const isStockManagementReport = stockManagementReportKinds.has(scopedKind);
  const isStockItemSummaryReport = scopedKind === 'item-stock-register-summary';
  const isStockMovementSummaryReport = stockMovementSummaryReportKinds.has(scopedKind);
  const isStockSupplierManufacturerReport = stockSupplierManufacturerReportKinds.has(scopedKind);
  const isStockSalesReport = scopedKind === 'stock-and-sales';
  const isStockNarcoticsGenericReport = stockNarcoticsGenericReportKinds.has(scopedKind);
  const isStockExpiryClassReport = stockExpiryClassReportKinds.has(scopedKind);
  const stockClassificationLabel = stockClassificationReportLabels[scopedKind];
  const isStockClassificationReport = Boolean(stockClassificationLabel);
  const hasPurchaseProjection = phaseOReportKinds.has(scopedKind);
  const qMode = phaseQReportModes[scopedKind] ?? phaseQAliases[scopedKind] ?? phaseQFinancialOverrides[scopedKind];
  const isDocumentProjection = qMode === 'document-line-detail' || qMode === 'document-invoice-summary';
  const isHeaderProjection = qMode === 'header-summary';
  const hasHistoricalProjection = Boolean(qMode?.startsWith('history-'));
  const hasFinanceProjection = Boolean(qMode && !['compatibility', 'sales', 'purchases', 'admin', 'adjustment', 'empty-fallback', 'document-line-detail', 'document-invoice-summary', 'header-summary'].includes(qMode) && !hasHistoricalProjection);
  const hasQProjection = Object.prototype.hasOwnProperty.call(phaseQReportModes, scopedKind);
  const hasConcreteProjection = isDailySaleDetail || kind === 'stock' || kind === 'item' || kind === 'purchase-return' || hasStockProjection || hasFinanceProjection || qMode === 'admin' || qMode === 'adjustment' || hasHistoricalProjection || isDocumentProjection || isHeaderProjection || hasReprintProjection;
  const reportTitle = title ?? ({
    'daily-sales-detail': 'Daily Sales Detail',
    stock: 'Stock Reports',
    item: 'Item Reports',
    'purchase-return': 'Purchase Return Reports'
  }[kind] ?? phaseOReportTitles[scopedKind] ?? phasePReportTitles[scopedKind] ?? phaseQReportTitles[scopedKind] ?? kind.replace(/-/g, ' '));
  const financeMode = hasFinanceProjection
    ? qMode
    : undefined;
  const financeNote = financeMode === 'historical-gl'
    ? 'Imported historical_gl_entries from dbo.VirtualGl expose document code/type, account and alternate account codes, invoice/user identifiers, remarks, and debit/credit source values. Newly posted normalized gl_journals are included as explicitly labeled canonical rows; exact legacy account naming, opening balances, and print layout remain unverified.'
    : financeMode === 'gl' || financeMode === 'gl-cash'
    ? 'Posted-only gl_journals and gl_lines joined to the tenant chart of accounts; no draft, void, or legacy VirtualGl rows are inferred.'
    : financeMode === 'trial-balance'
      ? 'Posted-only gl_journals/gl_lines plus imported historical_gl_entries are aggregated by account; unmatched legacy accounts remain explicitly labeled Historical and opening-balance semantics remain unverified.'
      : financeMode === 'party-customer'
        ? 'Posted-only customer party_ledger_entries, including posted historical documents without a canonical GL journal, plus source-backed SaleLedger/InstallmentReceiptDetail payment allocations, SaleReceivableAdj adjustments, and SRAllocationDetail return allocations; unresolved legacy identities remain visible and exact legacy statement/print semantics remain unverified.'
        : financeMode === 'party-supplier'
          ? 'Posted-only supplier party_ledger_entries, including posted historical documents without a canonical GL journal, plus source-backed PurPayment/Purledger payment allocations and PRAllocationDetail return allocations; unresolved legacy identities remain visible and exact legacy statement/print semantics remain unverified.'
          : financeMode === 'aging-receivable'
            ? 'Posted customer party-ledger entries use retained SaleLedger DueDate payloads when available for bounded aging buckets; missing due dates remain explicitly unaged and exact legacy bucket semantics remain unverified.'
            : financeMode === 'aging-payable'
              ? 'Posted supplier party-ledger entries use retained Purledger CreditDays payloads for bounded purchase due dates; purchase returns and rows without valid terms remain explicitly unaged and exact legacy bucket semantics remain unverified.'
            : financeMode === 'tax-withholding'
              ? 'Imported dbo.PurPayment rows expose posted payment-level withholding base, rate, amount, account, check/reference, supplier-invoice identity, and remarks. Purchase-line advance tax is not reclassified as withholding; exact legacy grouping and print output remain unverified.'
              : financeMode === 'voucher'
                ? 'Posted voucher_entries scoped to tenant and branch; voucher-to-GL posting is not assumed without a linked journal.'
                : financeMode === 'tax-advance' || financeMode === 'tax-advance-input'
                  ? 'Posted document line snapshots use explicit advance-tax rate/base/amount evidence; rows without advance-tax amount evidence are omitted and values are not recomputed from current tax configuration.'
                : financeMode?.startsWith('tax-')
                  ? 'Posted document line tax snapshots are used; values are not recomputed from current tax configuration.'
                  : undefined;
  const adminColumns = qMode === 'admin' ? [
    { key: 'document', label: 'Code', dataType: 'text' as const, sortable: true },
    { key: 'occurredAt', label: 'Updated', dataType: 'date' as const, sortable: true },
    { key: 'party', label: 'Kind', dataType: 'text' as const, sortable: true },
    { key: 'item', label: 'Name', dataType: 'text' as const, sortable: true },
    { key: 'quantity', label: 'Active', dataType: 'text' as const, sortable: true },
    { key: 'amount', label: 'Legacy ID', dataType: 'text' as const, sortable: true }
  ] : undefined;
  return {
    kind,
    title: reportTitle,
    projectionStatus: hasConcreteProjection ? 'real' : hasEventProjection || qMode === 'sales' || qMode === 'purchases' ? 'event-ledger' : 'generic-fallback',
    projectionNote: financeNote ?? (qMode === 'empty-fallback'
      ? 'Exact legacy item-history label retained; no normalized item-history source or verified legacy columns are available, so no values are fabricated.'
      : hasHistoricalProjection
      ? qMode === 'history-deleted-sale-items'
		? 'source-backed DeletedSaleItem rows retain the captured sale invoice, item, godown, quantity/bonus, sale price, discount/tax fields, machine, user, and raw source payload under tenant and branch scope. Exact PowerBuilder deleted-item columns, deletion-order semantics, and print layout remain unverified.'
      : qMode === 'history-stock-adjustments'
        ? 'Source-backed AdjHeader/AdjDetail rows retain legacy payload, header status, item, godown, batch, and pricing fields, and posted normalized stock adjustments are included from the immutable inventory event. Exact PowerBuilder adjustment grouping, calculated columns, and print layout remain unverified.'
        : 'Normalized source-backed ItemLog snapshots retain source identity, payload, prior-observed values, and change reason. The New Item report uses first-observed snapshots rather than asserting creation semantics; exact PowerBuilder columns, filtering, and print layout remain unverified.'
      : qMode === 'adjustment'
      ? 'Normalized posted stock_ledger adjustment projection; legacy adjustment grouping and unreconciled calculations are not implemented.'
      : hasReprintProjection
      ? isReprintPurchaseDetailProjection
        ? 'Canonical posted purchase lines are available for reprinting, with de-duplicated compatibility receiving events retained when no canonical document identity matches. Exact PowerBuilder purchase reprint sections, selected-invoice semantics, and print format remain unverified.'
        : isReprintSaleSummaryProjection
          ? 'Canonical posted sale invoice summaries are available for reprinting, with de-duplicated compatibility sale events retained when no canonical document identity matches. Exact PowerBuilder summary sections, selected-invoice semantics, and print format remain unverified.'
          : 'Canonical posted sale lines are available for reprinting, with retained legacy line payload and de-duplicated compatibility sale events when no canonical document identity matches. Exact PowerBuilder patient/pack/header sections, selected-invoice semantics, and print format remain unverified.'
      : isDocumentProjection
        ? 'Canonical posted quotation/refused-sale document rows are joined to customer metadata and de-duplicated against posted compatibility events; exact PowerBuilder detail/summary calculations and print output remain unverified.'
      : isHeaderProjection
        ? 'Canonical posted business-document headers are grouped across sales, returns, quotations, refusals, purchases, purchase returns, and purchase orders, with de-duplicated compatibility events for documents not yet canonical. Exact PowerBuilder transaction-type and print calculations remain unverified.'
      : qMode === 'admin'
        ? 'Tenant-scoped administrative projection; branch scope is applied where the source table carries a branch assignment. Legacy-only fields are omitted.'
      : qMode === 'compatibility'
        ? 'Explicit compatibility fallback over posted sync_events; no normalized canonical projection exists for this legacy leaf.'
        : qMode === 'sales' || qMode === 'purchases'
          ? 'Canonical posted read model with explicitly labeled compatibility fallback when no canonical document identity matches.'
      : isDailySaleDetail
        ? 'Canonical posted sale lines expose the captured Alias, Item Description, Sale Price, Qty, discount, tax, amount, expiry, and batch columns; retained legacy line payload values are preferred when available. Compatibility sale rows are expanded when no canonical document identity matches. Format-specific patient, pack, and print calculations remain unverified.'
      : isLineDetail
        ? 'Canonical posted sale/return business_document lines expose a source-backed line-detail contract with alias, item, price, quantity, discounts, tax, amount, expiry, and batch values; compatibility rows are expanded when no canonical identity matches. Exact legacy grouping, format calculations, and print output remain unverified.'
      : isPurchaseLineDetail
        ? 'Canonical posted purchase business-document lines expose document, supplier, item, quantity, purchase price, discount, tax, amount, expiry, and batch values; compatibility receiving/return events are expanded when no canonical document identity matches. Exact legacy grouping, tax, profit, purchase-order, and print calculations remain unverified.'
      : isPurchaseSummaryProjection
        ? 'Canonical and compatibility purchase rows use the explicit ' + (purchaseSummaryMode ?? 'summary') + ' projection; exact PowerBuilder grouping, tax, profit, return, graph, and print calculations remain unverified.'
      : isSalesSummaryProjection
        ? ['profit-margin-detail', 'profit-day-summary', 'profit-customer-summary'].includes(salesSummaryMode ?? '')
          ? 'Canonical posted sale lines use allocated stock cost when available and compatibility rows use explicitly supplied legacy cost; gross profit and margin are bounded calculations, while exact PowerBuilder valuation, returns, discounts, and print calculations remain unverified.'
          : salesSummaryMode === 'customer-category-summary'
            ? 'Canonical and compatibility sales rows are grouped by the retained customer master category payload with an explicit Unspecified bucket; exact PowerBuilder category joins, net/return treatment, and print calculations remain unverified.'
          : salesSummaryMode === 'customer-wise-category-summary'
            ? 'Canonical and compatibility sales rows are grouped by retained customer and customer category payload with an explicit Unspecified bucket; exact PowerBuilder joins, net/return treatment, and print calculations remain unverified.'
          : 'Canonical and compatibility sales rows use the explicit ' + (salesSummaryMode ?? 'summary') + ' projection; exact legacy grouping, tax, profit, return treatment, and print calculations remain unverified.'
      : hasConcreteProjection && !hasStockProjection
      ? undefined
      : hasStockProjection
      ? (scopedKind === 'stock-in-hand-back-date'
          ? 'Source-backed historical_stock_snapshots imported from dbo.StockReport expose the captured as-of stock, purchase price, sale price, average price, recent purchase price, and pack-unit fields. Manufacturer/category/class/narcotics grouping and exact print calculations remain unverified.'
        : isStockManagementReport
          ? 'Normalized stock_balances expose batch, godown, item, on-hand, and item-payload reorder/optimum/minimum thresholds with posted-ledger gating. This is a bounded Stock Management projection; exact PowerBuilder alert status, valuation, grouping, and print calculations remain unverified.'
        : isStockLevelReport
          ? 'Normalized stock_balances expose on-hand plus item payload reorder/optimum/minimum thresholds, using ReorderQty/OptimumQty/MinimumQty with maintenance-field fallbacks. The below-threshold predicate, zero-stock inclusion, date semantics, and exact PowerBuilder calculations remain unverified.'
        : isStockItemSummaryReport
          ? 'Normalized posted stock_ledger rows are grouped by item, godown, and calendar day; net quantity applies direction and signed adjustments, and net value is the signed quantity multiplied by posted unit cost. Legacy opening balances, grouping labels, and exact report calculations remain unverified.'
        : isStockMovementSummaryReport
          ? 'Normalized posted stock_ledger rows are grouped by calendar day, IN/OUT/ADJUSTMENT direction, godown, and item. Quantities and values are signed from the posted ledger; opening balances, legacy date-wise grouping, and exact print calculations remain unverified.'
        : isStockSupplierManufacturerReport
          ? 'Normalized posted stock_balances join the captured Item Manufacturer payload and tenant-scoped item_suppliers supplier links. Supplier names are aggregated per item to avoid duplicating batch balances; exact legacy association, priority selection, valuation, and print calculations remain unverified.'
        : isStockSalesReport
          ? 'Normalized stock balances are joined to canonical posted cash/credit sale allocations for the requested period; On Hand is the current balance cache and Sales Qty is the allocated sale quantity. Compatibility-only sales, returns, opening balances, and exact Stock-and-Sales calculations remain unverified.'
        : isStockNarcoticsGenericReport
          ? 'Normalized posted stock_ledger movements are filtered by the captured Item master Narcotics flag and grouped by the captured GenericName/GenericCode payload, day, godown, and item. Exact generic-type grouping, return/opening treatment, and print calculations remain unverified.'
        : isStockExpiryClassReport
          ? 'Normalized posted stock_balances use typed stock_batches.expiry_date and the captured Item Class payload for the class-wise expiry projection; zero-stock rows and batches without a typed expiry are excluded. Exact class code joins, date-window semantics, and print calculations remain unverified.'
        : isStockClassificationReport
          ? `Normalized posted stock_balances are grouped by the captured Item ${stockClassificationLabel} payload with posted-ledger gating, current on-hand, batch expiry/update, godown, item, and unit-cost fields. Exact legacy group joins, valuation, and print calculations remain unverified.`
        : scopedKind.includes('expiry')
          ? 'Normalized posted stock_balances grouped by stock batch expiry; expired and future batches are shown from typed expiry_date, while legacy class/group calculations are not implemented.'
          : scopedKind.includes('stock-register') || scopedKind.includes('activity') || scopedKind.includes('daily-stock') || scopedKind.includes('stock-in-out')
            ? 'Normalized posted stock_ledger movement projection joined to batch, item, and godown metadata; compatibility inventory_movements and unreconciled legacy grouping are not included.'
            : 'Normalized posted stock_balances projection joined to stock batches, items, and godowns; legacy manufacturer/category/class/reorder/narcotics groupings and exact valuation are not implemented.')
      : hasPurchaseProjection
        ? 'Canonical posted purchase business_documents/lines with supplier party ledger and stock ledger values when available; posted compatibility events are included only when no canonical document matches. Legacy grouping and unreconciled tax, profit, graph, and disparity calculations are not implemented.'
      : hasEventProjection
        ? 'Scoped immutable event-ledger view; captured legacy grouping and calculated numeric fields are not implemented.'
        : 'Generic event-ledger fallback; exact legacy projection is not implemented.'),
    columns: isReprintSaleDetailProjection
      ? dailySaleDetailColumns
      : isReprintSaleSummaryProjection
        ? salesSummaryColumns('invoice-summary')
      : isReprintPurchaseDetailProjection
        ? purchaseLineDetailColumns
      : isDailySaleDetail || isLineDetail
      ? dailySaleDetailColumns
      : isPurchaseLineDetail
        ? purchaseLineDetailColumns
      : isPurchaseSummaryProjection
        ? purchaseSummaryColumns(purchaseSummaryMode ?? 'invoice-summary')
      : isSalesSummaryProjection
        ? salesSummaryColumns(salesSummaryMode ?? 'invoice-summary')
      : financeMode
      ? financeColumns(financeMode)
      : isDocumentProjection
        ? documentReportColumns(qMode ?? 'document-line-detail')
      : isHeaderProjection
        ? headerReportColumns
      : qMode === 'adjustment'
        ? stockMovementColumns
      : hasHistoricalProjection
        ? historicalColumns(qMode)
      : qMode === 'empty-fallback'
        ? eventLedgerColumns
      : adminColumns ?? (hasStockProjection
      ? (scopedKind === 'stock-in-hand-back-date'
        ? stockHistoricalColumns
        : isStockManagementReport
          ? stockLevelColumns
        : isStockLevelReport
          ? stockLevelColumns
        : isStockItemSummaryReport
          ? stockItemSummaryColumns
        : isStockMovementSummaryReport
          ? stockMovementSummaryColumns
        : isStockSupplierManufacturerReport
          ? stockSupplierManufacturerColumns
        : isStockSalesReport
          ? stockSalesColumns
        : isStockNarcoticsGenericReport
          ? stockNarcoticsGenericColumns
        : isStockExpiryClassReport
          ? stockExpiryClassColumns
        : isStockClassificationReport
          ? stockClassificationColumns(stockClassificationLabel ?? 'Group')
        : scopedKind.includes('stock-register') || scopedKind.includes('activity') || scopedKind.includes('daily-stock') || scopedKind.includes('stock-in-out') ? stockMovementColumns : stockBalanceColumns)
      : hasPurchaseProjection ? columns : hasEventProjection ? eventLedgerColumns : columns),
    formats: (isDailySaleDetail ? dailySaleFormats : isLineDetail || isSalesSummaryProjection || isPurchaseSummaryProjection || hasConcreteProjection || hasPurchaseProjection || hasStockProjection || (hasQProjection && qMode !== 'empty-fallback') ? ['Standard'] : ['Event ledger projection']).map((name) => ({
      id: name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
      name,
      source: 'default' as const
    })),
    retrieval: {
      title: 'Specify Retrieval Arguements',
      areas: ['DEFAULT AREA', 'ALL AREAS'],
      supportsCashCredit: isDailySaleDetail || isLineDetail || isReprintSaleDetailProjection || isReprintSaleSummaryProjection,
      supportsDateRange: true,
      supportsTextFilter: true,
      scope: hasFinanceProjection
        ? financeMode === 'historical-gl'
          ? 'tenant, branch, date, text, imported historical_gl_entries, newly posted gl_journals, and account/category aggregation'
          : financeMode === 'aging-receivable'
            ? 'tenant, branch, posted-only, date, text, customer party-ledger entries, and retained SaleLedger DueDate payloads'
          : financeMode === 'aging-payable'
            ? 'tenant, branch, posted-only, date, text, supplier party-ledger entries, and retained Purledger CreditDays payloads'
          : financeMode === 'party-customer'
            ? 'tenant, branch, posted-only, date, text, party_ledger_entries, SaleLedger payment snapshots, InstallmentReceiptDetail allocations, SaleReceivableAdj adjustments, and SRAllocationDetail return allocations'
          : financeMode === 'party-supplier'
            ? 'tenant, branch, posted-only, date, text, party_ledger_entries, PurPayment/Purledger payment snapshots, and PRAllocationDetail return allocations'
          : `tenant, branch, posted-only, date, text, and normalized ${financeMode} projection`
        : hasHistoricalProjection
          ? 'tenant, branch, date, text, retained source-backed historical rows, and raw source payload'
        : hasReprintProjection
          ? isReprintPurchaseDetailProjection
            ? 'tenant, branch, posted-only, date, text, canonical purchase documents/lines, and de-duplicated compatibility receiving events'
            : isReprintSaleSummaryProjection
              ? 'tenant, branch, posted-only, date, text, canonical sale documents/lines, and de-duplicated compatibility sale events grouped by invoice'
              : 'tenant, branch, posted-only, date, text, canonical sale documents/lines, retained legacy line payload, and de-duplicated compatibility sale events'
        : qMode === 'admin'
          ? 'tenant, branch assignment where available, date, text, and normalized administrative records'
          : isDocumentProjection
            ? 'tenant, branch, posted-only, date, text, canonical quotation/refused-sale documents/lines, and de-duplicated compatibility events'
          : isHeaderProjection
            ? 'tenant, branch, posted-only, date, text, canonical business_documents/lines, and de-duplicated compatibility transaction events grouped by header'
          : qMode === 'compatibility'
            ? 'tenant, branch, posted-only compatibility events, date, and text'
        : hasStockProjection
        ? scopedKind === 'stock-in-hand-back-date'
          ? 'tenant, branch, as-of date, text, godown, and imported historical_stock_snapshots'
          : isStockManagementReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and item threshold payload without an alert predicate'
          : isStockLevelReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and item threshold payload'
          : isStockItemSummaryReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, and item/godown/day aggregation'
          : isStockMovementSummaryReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, and day/direction/godown/item aggregation'
          : isStockSupplierManufacturerReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, Item Manufacturer payload, and item_suppliers links'
          : isStockSalesReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and canonical posted sale allocations'
          : isStockNarcoticsGenericReport
            ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, and captured Item Narcotics/GenericName payload'
          : isStockExpiryClassReport
            ? 'tenant, branch, typed expiry date, text, godown, batch, posted stock_ledger, and captured Item Class payload'
          : isStockClassificationReport
            ? `tenant, branch, date, text, godown, batch, posted stock_ledger, normalized stock_balances, and captured Item ${stockClassificationLabel} payload`
          : 'tenant, branch, date, text, godown, batch, posted stock_ledger, and normalized stock_balances'
        : isPurchaseSummaryProjection
        ? 'tenant, branch, posted-only, date, text, canonical purchase documents/lines, supplier party ledger, and compatibility receiving/return events with ' + (purchaseSummaryMode ?? 'summary') + ' grouping'
        : hasPurchaseProjection
        ? 'tenant, branch, date, text, supplier, canonical purchase documents/lines, stock ledger, supplier party ledger, and posted compatibility events'
        : hasEventProjection ? 'tenant, branch, date, text, and immutable event aggregate' : 'tenant and branch scoped'
    },
    letterhead: { ...defaultLetterhead },
    exports: [
      { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
      { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
      { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
    ]
  };
}

export function exportHook(definition: ReportDefinition, format: ReportExportFormat) {
  return definition.exports.find((hook) => hook.format === format);
}

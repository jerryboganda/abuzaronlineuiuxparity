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

const eventLedgerColumns = [
  { key: 'document', label: 'Event / Document', dataType: 'text' as const, sortable: true },
  { key: 'occurredAt', label: 'Occurred', dataType: 'date' as const, sortable: true },
  { key: 'party', label: 'Party', dataType: 'text' as const, sortable: true },
  { key: 'item', label: 'Item (first payload line)', dataType: 'text' as const, sortable: true },
  { key: 'quantity', label: 'Quantity (payload)', dataType: 'number' as const, sortable: true },
  { key: 'amount', label: 'Amount (payload)', dataType: 'currency' as const, sortable: true }
];

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

function reportRegistryKey(kind: string, legacyPath = ''): string {
  if (phaseNReportKinds.has(kind) || phaseOReportKinds.has(kind) || kind === 'daily-sales-detail') return kind;
  const segments = legacyPath.split(' > ').map((segment) => segment.replace(/\t.*$/, '').replace(/&/g, '').trim());
  if (segments.length < 3 || segments[0] !== 'Reports') return kind;
  let reportSegments = segments.slice(1);
  if (reportSegments[0] === 'Daily Reports') {
    reportSegments = reportSegments.slice(1);
    if (reportSegments.length > 1 && ['Sale', 'Sales Return', 'Purchase', 'Purchase Return', 'Purchase Order'].includes(reportSegments[0])) {
      reportSegments = reportSegments.slice(1);
    }
  } else if (['Sales Reports', 'Purchase Reports', 'Purchase Return Reports'].includes(reportSegments[0])) {
    reportSegments = reportSegments.slice(1);
  }
  const candidate = reportSegments.join(' ').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  return phaseNReportKinds.has(candidate) || phaseOReportKinds.has(candidate) ? candidate : kind;
}

export function defaultReportDefinition(kind: string, title?: string, legacyPath = ''): ReportDefinition {
  const isDailySaleDetail = kind === 'daily-sales-detail';
  const scopedKind = reportRegistryKey(kind, legacyPath);
  const hasEventProjection = phaseNReportKinds.has(scopedKind) || phaseOReportKinds.has(scopedKind);
  const hasPurchaseProjection = phaseOReportKinds.has(scopedKind);
  const hasConcreteProjection = isDailySaleDetail || kind === 'stock' || kind === 'item' || kind === 'purchase-return';
  const reportTitle = title ?? ({
    'daily-sales-detail': 'Daily Sales Detail',
    stock: 'Stock Reports',
    item: 'Item Reports',
    'purchase-return': 'Purchase Return Reports'
  }[kind] ?? phaseOReportTitles[scopedKind] ?? kind.replace(/-/g, ' '));
  return {
    kind,
    title: reportTitle,
    projectionStatus: hasConcreteProjection ? 'real' : hasEventProjection ? 'event-ledger' : 'generic-fallback',
    projectionNote: hasConcreteProjection
      ? undefined
      : hasPurchaseProjection
        ? 'Canonical posted purchase business_documents/lines with supplier party ledger and stock ledger values when available; posted compatibility events are included only when no canonical document matches. Legacy grouping and unreconciled tax, profit, graph, and disparity calculations are not implemented.'
      : hasEventProjection
        ? 'Scoped immutable event-ledger view; captured legacy grouping and calculated numeric fields are not implemented.'
        : 'Generic event-ledger fallback; exact legacy projection is not implemented.',
    columns: hasPurchaseProjection ? columns : hasEventProjection ? eventLedgerColumns : columns,
    formats: (isDailySaleDetail || hasConcreteProjection || hasPurchaseProjection ? ['Standard'] : ['Event ledger projection']).map((name) => ({
      id: name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
      name,
      source: 'default' as const
    })),
    retrieval: {
      title: 'Specify Retrieval Arguements',
      areas: ['DEFAULT AREA', 'ALL AREAS'],
      supportsCashCredit: isDailySaleDetail,
      supportsDateRange: true,
      supportsTextFilter: true,
      scope: hasPurchaseProjection
        ? 'tenant, branch, date, text, supplier, canonical purchase documents/lines, stock ledger, supplier party ledger, and posted compatibility events'
        : hasEventProjection ? 'tenant, branch, date, text, and immutable event aggregate' : 'tenant and branch scoped'
    },
    letterhead: { ...defaultLetterhead },
    exports: [
      { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
      { format: 'pdf', status: 'not_implemented', label: 'PDF', message: 'PDF export is not implemented yet.' },
      { format: 'excel', status: 'not_implemented', label: 'Excel', message: 'Excel export is not implemented yet.' }
    ]
  };
}

export function exportHook(definition: ReportDefinition, format: ReportExportFormat) {
  return definition.exports.find((hook) => hook.format === format);
}

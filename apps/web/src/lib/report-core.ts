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
  'quotation-detail': 'compatibility',
  'quotation-summary': 'compatibility',
  'header-wise-transaction-summary': 'compatibility',
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
  'item-reports-deleted-sale-items-log': 'compatibility'
};

const phaseQReportTitles: Record<string, string> = {
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
  'item-reports-deleted-sale-items-log': 'Deleted Sale Items Log'
};

const phaseQFinancialOverrides: Record<string, string> = {
  'customer-sales-lp-ledger': 'party-customer',
  'customer-sales-customer-category-wise-sales-output-sales-tax-report': 'tax-output',
  'customer-sales-customer-ntn-wise-sales-tax-report': 'tax-output',
  'customer-sales-customer-wise-advance-tax': 'tax-advance',
  'sales-tax-report': 'tax-output',
  'supplier-wise-advance-income-tax': 'tax-advance',
  'supplier-category-wise-input-sales-tax-report': 'tax-input',
  'withholding-tax-deduction': 'tax-withholding',
  'user-wise-net-cash': 'gl-cash'
};

const phaseQAliases: Record<string, string> = {
  'gl-journal': 'gl',
  'trial-balance': 'trial-balance',
  'customer-statement': 'party-customer',
  'supplier-statement': 'party-supplier',
  'receivables-aging': 'aging-receivable',
  'payables-aging': 'aging-payable',
  'tax-register': 'tax-output',
  'voucher-register': 'voucher'
};

const financeColumns = (mode: string) => {
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
  const hasEventProjection = phaseNReportKinds.has(scopedKind) || phaseOReportKinds.has(scopedKind);
  const hasStockProjection = phasePReportKinds.has(scopedKind);
  const hasPurchaseProjection = phaseOReportKinds.has(scopedKind);
  const qMode = phaseQReportModes[scopedKind] ?? phaseQAliases[scopedKind] ?? phaseQFinancialOverrides[scopedKind];
  const hasFinanceProjection = Boolean(qMode && !['compatibility', 'sales', 'purchases', 'admin', 'adjustment'].includes(qMode));
  const hasQProjection = Object.prototype.hasOwnProperty.call(phaseQReportModes, scopedKind);
  const hasConcreteProjection = isDailySaleDetail || kind === 'stock' || kind === 'item' || kind === 'purchase-return' || hasStockProjection || hasFinanceProjection || qMode === 'admin' || qMode === 'adjustment';
  const reportTitle = title ?? ({
    'daily-sales-detail': 'Daily Sales Detail',
    stock: 'Stock Reports',
    item: 'Item Reports',
    'purchase-return': 'Purchase Return Reports'
  }[kind] ?? phaseOReportTitles[scopedKind] ?? phasePReportTitles[scopedKind] ?? phaseQReportTitles[scopedKind] ?? kind.replace(/-/g, ' '));
  const financeMode = hasFinanceProjection
    ? qMode
    : undefined;
  const financeNote = financeMode === 'gl' || financeMode === 'gl-cash'
    ? 'Posted-only gl_journals and gl_lines joined to the tenant chart of accounts; no draft, void, or legacy VirtualGl rows are inferred.'
    : financeMode === 'trial-balance'
      ? 'Posted-only gl_journals/gl_lines aggregated by account; opening or imported historical balances are not included.'
      : financeMode === 'party-customer'
        ? 'Posted-only customer party_ledger_entries with running balances where recorded; unavailable legacy statement columns are omitted.'
        : financeMode === 'party-supplier'
          ? 'Posted-only supplier party_ledger_entries with running balances where recorded; unavailable legacy statement columns are omitted.'
          : financeMode === 'aging-receivable' || financeMode === 'aging-payable'
            ? 'Unaged outstanding party-ledger totals only. No due_date, payment allocation, or invoice aging-bucket prerequisite exists, so bucket values are not fabricated.'
            : financeMode === 'tax-withholding'
              ? 'No normalized withholding-tax snapshot or posting source exists; rate, amount, certificate, and party allocation prerequisites are absent.'
              : financeMode === 'voucher'
                ? 'Posted voucher_entries scoped to tenant and branch; voucher-to-GL posting is not assumed without a linked journal.'
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
    projectionStatus: financeMode === 'tax-withholding' ? 'generic-fallback' : hasConcreteProjection ? 'real' : hasEventProjection || qMode === 'sales' || qMode === 'purchases' ? 'event-ledger' : 'generic-fallback',
    projectionNote: financeNote ?? (qMode === 'adjustment'
      ? 'Normalized posted stock_ledger adjustment projection; legacy adjustment grouping and unreconciled calculations are not implemented.'
      : qMode === 'admin'
        ? 'Tenant-scoped administrative projection; branch scope is applied where the source table carries a branch assignment. Legacy-only fields are omitted.'
      : qMode === 'compatibility'
        ? 'Explicit compatibility fallback over posted sync_events; no normalized canonical projection exists for this legacy leaf.'
        : qMode === 'sales' || qMode === 'purchases'
          ? 'Canonical posted read model with explicitly labeled compatibility fallback when no canonical document identity matches.'
        : hasConcreteProjection && !hasStockProjection
      ? undefined
      : hasStockProjection
        ? (scopedKind.includes('expiry')
          ? 'Normalized posted stock_balances grouped by stock batch expiry; expired and future batches are shown from typed expiry_date, while legacy class/group calculations are not implemented.'
          : scopedKind.includes('stock-register') || scopedKind.includes('activity') || scopedKind.includes('daily-stock') || scopedKind.includes('stock-in-out')
            ? 'Normalized posted stock_ledger movement projection joined to batch, item, and godown metadata; compatibility inventory_movements and unreconciled legacy grouping are not included.'
            : 'Normalized posted stock_balances projection joined to stock batches, items, and godowns; legacy manufacturer/category/class/reorder/narcotics groupings and exact valuation are not implemented.')
      : hasPurchaseProjection
        ? 'Canonical posted purchase business_documents/lines with supplier party ledger and stock ledger values when available; posted compatibility events are included only when no canonical document matches. Legacy grouping and unreconciled tax, profit, graph, and disparity calculations are not implemented.'
      : hasEventProjection
        ? 'Scoped immutable event-ledger view; captured legacy grouping and calculated numeric fields are not implemented.'
        : 'Generic event-ledger fallback; exact legacy projection is not implemented.'),
    columns: financeMode
      ? financeColumns(financeMode)
      : qMode === 'adjustment'
        ? stockMovementColumns
      : adminColumns ?? (hasStockProjection
      ? (scopedKind.includes('stock-register') || scopedKind.includes('activity') || scopedKind.includes('daily-stock') || scopedKind.includes('stock-in-out') ? stockMovementColumns : stockBalanceColumns)
      : hasPurchaseProjection ? columns : hasEventProjection ? eventLedgerColumns : columns),
    formats: (isDailySaleDetail ? dailySaleFormats : hasConcreteProjection || hasPurchaseProjection || hasStockProjection || hasQProjection ? ['Standard'] : ['Event ledger projection']).map((name) => ({
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
      scope: hasFinanceProjection
        ? `tenant, branch, posted-only, date, text, and normalized ${financeMode} projection`
        : qMode === 'admin'
          ? 'tenant, branch assignment where available, date, text, and normalized administrative records'
          : qMode === 'compatibility'
            ? 'tenant, branch, posted-only compatibility events, date, and text'
            : hasStockProjection
        ? 'tenant, branch, date, text, godown, batch, posted stock_ledger, and normalized stock_balances'
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

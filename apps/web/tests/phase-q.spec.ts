import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
});

test('Phase Q representative financial and labeled fallback leaves expose their sources', async ({ page }) => {
  const cases: Record<string, { title: string; labels: string[]; note: string; row: string }> = {
    'gl-journal': {
      title: 'GL Journal',
      labels: ['Document Code', 'Date', 'Document Type', 'Account Code', 'Alternate Account', 'Invoice Code', 'User Code', 'Remarks', 'Debit', 'Credit'],
      note: 'Imported historical_gl_entries from dbo.VirtualGl',
      row: 'JOURNAL-1'
    },
    'customer-statement': {
      title: 'Customer Statement',
      labels: ['Document', 'Customer/Supplier', 'Description'],
      note: 'Posted-only customer party_ledger_entries',
      row: 'SALE-1'
    },
    'receivables-aging': {
      title: 'Receivables Aging',
      labels: ['Party', 'Aging Status', 'Debit Total', 'Credit Total'],
      note: 'DueDate payloads when available',
      row: '0-30 days'
    },
    'payables-aging': {
      title: 'Payables Aging',
      labels: ['Party', 'Aging Status', 'Debit Total', 'Credit Total'],
      note: 'Purledger CreditDays',
      row: '0-30 days'
    },
    'tax-register': {
      title: 'Tax Register',
      labels: ['Document', 'Taxable Base', 'Tax Amount'],
      note: 'Posted document line tax snapshots',
      row: 'INV-TAX-1'
    },
    'customer-sales-customer-wise-advance-tax': {
      title: 'Customer Wise Advance Tax',
      labels: ['Document', 'Date', 'Customer/Supplier', 'Item / Tax Snapshot', 'Taxable Base', 'Tax Amount'],
      note: 'explicit advance-tax rate/base/amount evidence',
      row: 'ADVANCE-TAX-1'
    },
    'supplier-wise-advance-income-tax': {
      title: 'Advance Income Tax',
      labels: ['Document', 'Date', 'Customer/Supplier', 'Item / Tax Snapshot', 'Taxable Base', 'Tax Amount'],
      note: 'explicit advance-tax rate/base/amount evidence',
      row: 'ADVANCE-TAX-INPUT-1'
    },
    'withholding-tax-deduction': {
      title: 'Withholding Tax Deduction',
      labels: ['Payment', 'Date', 'Supplier', 'Purchase Invoice / Certificate', 'Withholding Base', 'Withholding Amount'],
      note: 'Imported dbo.PurPayment rows expose posted payment-level withholding',
      row: 'PAYMENT-WHT-1'
    },
    'quotation-summary': {
      title: 'Summary',
      labels: ['Document', 'Date', 'Customer', 'Summary', 'Quantity', 'Amount'],
      note: 'Canonical posted quotation/refused-sale document rows',
      row: 'QUOTE-CANONICAL-1'
    },
    'refused-sales-detail': {
      title: 'Refused Sales Detail',
      labels: ['Document', 'Date', 'Customer/Supplier', 'Item', 'Quantity', 'Amount'],
      note: 'Canonical posted quotation/refused-sale document rows',
      row: 'REFUSED-CANONICAL-1'
    },
    'header-wise-transaction-summary': {
      title: 'Header Wise Transaction Summary',
      labels: ['Document', 'Date', 'Customer/Supplier', 'Transaction Type', 'Quantity', 'Amount'],
      note: 'Canonical posted business-document headers',
      row: 'HEADER-CANONICAL-1'
    },
    'reprinting-sale': {
      title: 'Sale',
      labels: ['Alias', 'Item Description', 'Sale Price', 'Qty', 'Disc(%)', 'Discount Value', 'Item Disc', 'SalesTax Value', 'Amount', 'Expiry Date', 'Batch Number'],
      note: 'Canonical posted sale lines are available for reprinting',
      row: 'SALE-REPRINT-1'
    },
    'reprinting-sale-with-summary-reports': {
      title: 'Sale (with summary reports)',
      labels: ['Invoice', 'Date', 'Customer', 'Summary', 'Quantity', 'Amount'],
      note: 'Canonical posted sale invoice summaries are available for reprinting',
      row: 'SALE-SUMMARY-1'
    },
    'reprinting-purchase': {
      title: 'Purchase',
      labels: ['Document', 'Date', 'Supplier', 'Item', 'Quantity', 'Purchase Price', 'Disc(%)', 'Discount Value', 'Sales Tax', 'Amount', 'Expiry Date', 'Batch Number'],
      note: 'Canonical posted purchase lines are available for reprinting',
      row: 'PURCHASE-REPRINT-1'
    },
    'item-reports-history-item-name-changes': {
      title: 'Item Name Changes',
      labels: ['Source Row', 'User / Reason', 'Item', 'Previous', 'Current'],
      note: 'Normalized source-backed ItemLog snapshots',
      row: 'ITEMLOG-1'
    },
    'item-reports-stock-adjustments-stock-adjustments-detail': {
      title: 'Stock Adjustments Detail',
      labels: ['Adjustment', 'Godown / User', 'Item / Batch', 'Loose Quantity', 'Adjustment Price'],
      note: 'Normalized source-backed AdjHeader/AdjDetail rows',
      row: 'ADJ-1'
    },
    'item-reports-deleted-sale-items-log': {
      title: 'Deleted Sale Items Log',
      labels: ['Sale Invoice', 'Deleted At', 'Machine / User', 'Item / Godown', 'Qty + Bonus', 'Sale Price'],
      note: 'source-backed DeletedSaleItem rows',
      row: 'DELETED-SALE-1'
    }
  };

  await page.route('**/v1/reports/**', async (route) => {
    const kind = new URL(route.request().url()).pathname.split('/').pop() ?? '';
    const current = cases[kind] ?? cases['quotation-summary'];
    const columns = current.labels.map((label, index) => ({
      key: ['document', 'occurredAt', 'documentType', 'party', 'alternateAccountCode', 'invoiceCode', 'userLegacyId', 'item', 'quantity', 'amount'][index] ?? 'document',
      label,
      dataType: label.includes('Date') || label === 'Posted' ? 'date' : 'text',
      sortable: true
    }));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        kind,
        rows: [{ document: current.row, occurredAt: '2026-08-06', party: 'PARTY-1', item: current.row, quantity: '10.00', amount: '20.00' }],
        page: 1,
        pageSize: 50,
        hasMore: false,
        definition: {
          kind,
          title: current.title,
          projectionStatus: current.note.startsWith('Explicit') ? 'event-ledger' : 'real',
          projectionNote: current.note,
          columns,
          formats: [{ id: 'standard', name: 'Standard', source: 'default' }],
          retrieval: { title: 'Specify Retrieval Arguements', areas: ['DEFAULT AREA'], supportsCashCredit: false, supportsDateRange: true, supportsTextFilter: true, scope: 'tenant, branch, posted-only' },
          letterhead: { name: "Fazal Din's Pharma Plus", line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '055 3252501', fax: '', source: 'default' },
          exports: [
            { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
            { format: 'pdf', status: 'available', label: 'PDF', message: 'Print preview.' },
            { format: 'excel', status: 'available', label: 'Excel', message: 'Workbook.' }
          ]
        }
      })
    });
  });

  for (const [kind, current] of Object.entries(cases)) {
    await page.goto(`/app/report/${kind}`);
    await page.waitForTimeout(1000);
    await page.getByRole('button', { name: 'Refresh report' }).click();
    await expect(page.getByRole('heading', { name: current.title, exact: true })).toBeVisible();
    await expect(page.getByText(current.note)).toBeVisible();
    await expect(page.getByText(current.row).first()).toBeVisible();
    for (const label of current.labels) await expect(page.locator('th').filter({ hasText: label }).first()).toBeVisible();
  }
});

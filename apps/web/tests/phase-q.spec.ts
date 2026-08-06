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
      labels: ['Journal', 'Account', 'Debit', 'Credit'],
      note: 'Posted-only gl_journals and gl_lines',
      row: 'JOURNAL-1'
    },
    'customer-statement': {
      title: 'Customer Statement',
      labels: ['Document', 'Customer/Supplier', 'Description'],
      note: 'Posted-only customer party_ledger_entries',
      row: 'SALE-1'
    },
    'payables-aging': {
      title: 'Supplier Statement',
      labels: ['Party', 'Aging Status', 'Debit Total', 'Credit Total'],
      note: 'No due_date, payment allocation',
      row: 'UNAGED - due date unavailable'
    },
    'tax-register': {
      title: 'Tax Register',
      labels: ['Document', 'Taxable Base', 'Tax Amount'],
      note: 'Posted document line tax snapshots',
      row: 'INV-TAX-1'
    },
    'quotation-summary': {
      title: 'Summary',
      labels: ['Event / Document', 'Party', 'Amount (payload)'],
      note: 'Explicit compatibility fallback',
      row: 'QUOTE-EVENT-1'
    }
  };

  await page.route('**/v1/reports/**', async (route) => {
    const kind = new URL(route.request().url()).pathname.split('/').pop() ?? '';
    const current = cases[kind] ?? cases['quotation-summary'];
    const columns = current.labels.map((label, index) => ({
      key: ['document', 'occurredAt', 'party', 'item', 'quantity', 'amount'][index] ?? 'document',
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

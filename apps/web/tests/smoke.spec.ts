import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
});

test('landing page exposes the parity workspace entrypoint', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('landing-page')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open parity workspace' })).toBeVisible();
});

test('workspace renders the shared Chrome/Tauri shell', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.goto('/app');
  await expect(page.getByTestId('workspace-page')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
  await expect(page.getByText('Parity workspace foundation')).toBeVisible();
});

test('legacy route renders the captured maximized main-window frame', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: {
        tenantId: 'tenant-1',
        tenantCode: 'TENANT',
        branchId: 'branch-1',
        counterId: 'counter-1',
        operatorId: 'operator-1',
        displayName: 'ADMIN'
      }
    })
  }));
  await page.goto('/app/legacy');
  await expect(page.getByTestId('legacy-shell-page')).toBeVisible();
  await expect(page.getByRole('button', { name: 'File' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Help' })).toBeVisible();
  await expect(page.getByRole('status')).toHaveText('Ready');
});

test('main-shell Change User opens the captured confirmation before login navigation', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  let logoutRequests = 0;
  await page.route('**/v1/auth/logout', async (route) => {
    logoutRequests += 1;
    await route.fulfill({ status: 204, body: '' });
  });
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/legacy');
  await accessResponse;
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Change User', exact: true }).click();
  const dialog = page.getByRole('alertdialog', { name: 'Change User' });
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveClass(/legacy-change-user-captured/);
  await dialog.getByRole('button', { name: 'No', exact: true }).click();
  await expect(dialog).toBeHidden();

  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Change User', exact: true }).click();
  const loginNavigation = page.waitForURL(/\/login\?changeUser=1/);
  await page.getByRole('alertdialog', { name: 'Change User' }).getByRole('button', { name: 'Yes', exact: true }).click();
  await loginNavigation;
  expect(logoutRequests).toBe(1);
  const registryState = await page.evaluate(() => JSON.parse(sessionStorage.getItem('abuzar.legacy-window-registry.v1') ?? '{}') as { windows?: unknown[] });
  expect(registryState.windows).toEqual([]);
});

test('child-window Change User uses the same captured confirmation before login navigation', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  let logoutRequests = 0;
  await page.route('**/v1/auth/logout', async (route) => {
    logoutRequests += 1;
    await route.fulfill({ status: 204, body: '' });
  });
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/report/sale-detail');
  await accessResponse;
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Change User', exact: true }).click();
  const dialog = page.getByRole('alertdialog', { name: 'Change User' });
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveClass(/legacy-change-user-captured/);
  await dialog.getByRole('button', { name: 'No', exact: true }).click();
  await expect(dialog).toBeHidden();

  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Change User', exact: true }).click();
  const loginNavigation = page.waitForURL(/\/login\?changeUser=1/);
  await page.getByRole('alertdialog', { name: 'Change User' }).getByRole('button', { name: 'Yes', exact: true }).click();
  await loginNavigation;
  expect(logoutRequests).toBe(1);
  const registryState = await page.evaluate(() => JSON.parse(sessionStorage.getItem('abuzar.legacy-window-registry.v1') ?? '{}') as { windows?: unknown[] });
  expect(registryState.windows).toEqual([]);
});

test('captured main-window keyboard shortcuts route to their menu commands', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.goto('/app/legacy');
  await expect(page.getByRole('button', { name: 'File' })).toBeVisible();
  await page.waitForTimeout(1000);
  await page.keyboard.press('Control+Alt+m');
  await expect(page).toHaveURL(/\/app\/manage\/session-monitor/);
});

test('global Ctrl+X keyboard shortcut navigates to Exit', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/legacy');
  await accessResponse;
  const exitNavigation = page.waitForURL('/');
  await page.keyboard.press('Control+x');
  await exitNavigation;
});

test('global Ctrl+Q shortcut triggers Save And Post command on contextual sales surface', async ({ page }) => {
  const itemId = '11111111-1111-4111-8111-111111111111';
  const documentId = '33333333-3333-4333-8333-333333333333';
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/items/lookup*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'QUOTE ITEM', payload: { salePrice: '10.00' }, active: true, aliases: [] }] })
  }));
  await page.route('**/v1/transactions/preview', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '10.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '10.00', taxes: [], totalDiscount: '0.00', total: '10.00' })
  }));
  let postReceived = false;
  await page.route('**/v1/documents/quotation', async (route) => {
    postReceived = true;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '55555555-5555-4555-8555-555555555555', aggregateId: documentId, kind: 'quotation', action: 'save-and-post', status: 'posted', document: { id: documentId, documentNumber: 'Q-1', version: 1 } })
    });
  });
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/sales?kind=quotation');
  await accessResponse;
  await page.getByLabel('Item lookup query').fill('QUOTE');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'QUOTE ITEM' })).toBeVisible();
  await page.getByRole('button', { name: 'QUOTE ITEM' }).click();
  await page.keyboard.press('Control+q');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(postReceived).toBe(true);
});

test('MDI tab closing via tab close button removes window from registry', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/legacy');
  await accessResponse;
  await expect(page.getByRole('button', { name: 'Close Main Window' })).toBeVisible();
  await page.getByRole('button', { name: 'Close Main Window' }).click();
  const windowsCount = await page.evaluate(() => {
    const raw = sessionStorage.getItem('abuzar.legacy-window-registry.v1');
    if (!raw) return 0;
    return (JSON.parse(raw) as { windows?: unknown[] }).windows?.length ?? 0;
  });
  expect(windowsCount).toBe(0);
});

test('SessionStorage preserves and restores open MDI windows across reloads for all valid context strings', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/legacy');
  await accessResponse;

  await page.evaluate(() => {
    const raw = sessionStorage.getItem('abuzar.legacy-window-registry.v1');
    const existing = raw ? JSON.parse(raw) : { windows: [], activeId: '', layout: 'arrange' };
    existing.windows = [
      { id: 'purchase-return', label: 'Purchase Return', href: '/app/purchase/return', context: 'purchase-return' },
      { id: 'sales-credit', label: 'Credit Sale', href: '/app/sales?kind=credit', context: 'credit-sale' },
      { id: 'master-customer', label: 'Customer Master', href: '/app/master/customer', context: 'customer-master' },
      { id: 'preferences', label: 'Preferences', href: '/app/preferences', context: 'preferences' }
    ];
    existing.activeId = 'preferences';
    sessionStorage.setItem('abuzar.legacy-window-registry.v1', JSON.stringify(existing));
  });

  await page.reload();

  const restoredWindows = await page.evaluate(() => {
    const raw = sessionStorage.getItem('abuzar.legacy-window-registry.v1');
    if (!raw) return [];
    return (JSON.parse(raw) as { windows?: Array<{ id: string; context: string }> }).windows ?? [];
  });

  expect(restoredWindows).toHaveLength(4);
  expect(restoredWindows.map((w) => w.context)).toEqual(['purchase-return', 'credit-sale', 'customer-master', 'preferences']);
});

test('offline-capable sales surface exposes the transaction workflow', async ({ page }) => {
  await page.goto('/app/sales');
  await expect(page.getByRole('heading', { name: 'New sale' })).toBeVisible();
  await expect(page.getByRole('button', { name: /Post sale/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Sync queue/ })).toBeVisible();
});

test('sales List tab renders persisted transaction history rows', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/transactions/sale*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ kind: 'sale', rows: [{ document: 'QA-100', occurredAt: '2026-08-05', party: 'CASH', item: 'QA ITEM', quantity: '2', amount: '42.00' }] })
  }));
  await page.goto('/app/sales?kind=cash');
  await page.waitForTimeout(700);
  await page.getByTestId('sales-list-tab').click({ force: true });
  await expect(page.getByTestId('sales-list-tab')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('.legacy-sale-list')).toBeVisible();
  await expect(page.getByText('QA-100')).toBeVisible();
  await expect(page.getByText('QA ITEM')).toBeVisible();
});

test('captured nested report menu reaches a concrete report workflow', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.goto('/app/legacy');
  await expect(page.getByRole('button', { name: 'Reports' })).toBeVisible();
  await page.waitForTimeout(1000);
  await page.getByRole('button', { name: 'Reports' }).click();
  await expect(page.locator('.legacy-menu-dropdown')).toBeVisible();
  await page.locator('.legacy-menu-dropdown > .legacy-menu-item-wrap > button').filter({ hasText: /^Daily Reports/ }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Sale', exact: true }).click({ force: true });
  await page.locator('.legacy-menu-subdropdown .legacy-menu-subdropdown > .legacy-menu-item-wrap > button').filter({ hasText: /^Sale detail$/ }).click({ force: true });
  await expect(page).toHaveURL(/\/app\/report\/sale-detail/);
  await expect(page.getByRole('heading', { name: 'Sale detail', exact: true })).toBeVisible();
});

test('Daily Sale Detail retrieves through the report definition and exports preview/workbook output', async ({ page }) => {
  let lastRequestedFormat = '';
  const rows = Array.from({ length: 25 }, (_, index) => ({
    document: `INV-${100 + index}`, occurredAt: '2026-08-06', party: 'CASH', item: index === 0 ? 'PARACETAMOL' : `ITEM-${index}`,
    alias: index === 0 ? 'PCM-500' : `SKU-${index}`, itemDescription: index === 0 ? 'PARACETAMOL' : `ITEM-${index}`, salePrice: '10.00', quantity: '2',
    discountPercent: '5.00', discountValue: '1.00', itemDiscount: '0.50', salesTaxValue: '0.25', amount: '20.00', expiryDate: '2027-01-01', batchNumber: 'B-100'
  }));
  await page.route('**/v1/reports/daily-sales-detail*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      kind: 'daily-sales-detail',
      rows,
      page: 1,
      pageSize: 1000,
      hasMore: false,
      definition: (() => {
        lastRequestedFormat = new URL(route.request().url()).searchParams.get('format') ?? '';
        return {
          kind: 'daily-sales-detail',
          title: 'Daily Sales Detail',
          projectionStatus: 'real',
          selectedFormat: lastRequestedFormat || 'Standard',
          columns: [
            { key: 'alias', label: 'Alias', dataType: 'text', sortable: true },
            { key: 'itemDescription', label: 'Item Description', dataType: 'text', sortable: true },
            { key: 'salePrice', label: 'Sale Price', dataType: 'currency', sortable: true },
            { key: 'quantity', label: 'Qty', dataType: 'number', sortable: true },
            { key: 'discountPercent', label: 'Disc(%)', dataType: 'number', sortable: true },
            { key: 'discountValue', label: 'Discount Value', dataType: 'currency', sortable: true },
            { key: 'itemDiscount', label: 'Item Disc', dataType: 'currency', sortable: true },
            { key: 'salesTaxValue', label: 'SalesTax Value', dataType: 'currency', sortable: true },
            { key: 'amount', label: 'Amount', dataType: 'currency', sortable: true },
            { key: 'expiryDate', label: 'Expiry Date', dataType: 'date', sortable: true },
            { key: 'batchNumber', label: 'Batch Number', dataType: 'text', sortable: true }
          ],
          formats: [{ id: 'standard', name: 'Standard', source: 'database' }, { id: 'compact', name: 'Compact', source: 'database' }],
          retrieval: { title: 'Specify Retrieval Arguements', areas: ['DEFAULT AREA'], supportsCashCredit: true },
          letterhead: { name: 'Configured Pharmacy', line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '0300 1234567', fax: '', source: 'database' },
          exports: [
            { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
            { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
            { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
          ]
        };
      })()
    })
  }));
  await page.goto('/app/report/daily-sales-detail');
  await expect(page.getByRole('dialog', { name: 'Specify Retrieval Arguements' })).toBeVisible({ timeout: 4000 });
  await page.getByRole('dialog', { name: 'Specify Retrieval Arguements' }).getByRole('button', { name: 'Ok' }).evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.getByText('PARACETAMOL').first()).toBeVisible();
  for (const heading of ['Alias', 'Item Description', 'Sale Price', 'Qty', 'Disc(%)', 'Discount Value', 'Item Disc', 'SalesTax Value', 'Amount', 'Expiry Date', 'Batch Number']) {
    await expect(page.getByRole('columnheader', { name: heading, exact: true })).toBeVisible();
  }
  await expect(page.getByText('PCM-500')).toBeVisible();
  await expect(page.getByRole('cell', { name: 'B-100', exact: true }).first()).toBeVisible();
  expect(lastRequestedFormat).toBe('Standard');
  await page.getByRole('button', { name: 'Report settings' }).click();
  const formatDialog = page.getByRole('dialog', { name: 'Select Format' });
  await formatDialog.dispatchEvent('pointerdown');
  await formatDialog.getByRole('radio', { name: /2\) Compact/ }).check();
  await formatDialog.getByRole('button', { name: 'Ok' }).click();
  await expect.poll(() => lastRequestedFormat).toBe('Compact');
  await expect(page.getByRole('button', { name: 'Export report as PDF' })).toBeEnabled();
  await page.getByRole('button', { name: 'Export report as PDF' }).click();
  await expect(page.getByRole('dialog', { name: 'Print preview' })).toBeVisible();
  await expect(page.getByText('Configured Pharmacy')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Preview next page' })).toBeEnabled();
  await expect(page.getByText('Preview page 1 of 2')).toBeVisible();
  await page.getByRole('button', { name: 'Preview next page' }).click();
  await expect(page.getByText('Preview page 2 of 2')).toBeVisible();
  await expect(page.getByText('Compact', { exact: true }).first()).toBeVisible();
  await page.getByRole('button', { name: 'Close print preview', exact: true }).click();
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Export report as Excel' }).click();
  await expect((await download).suggestedFilename()).toMatch(/daily-sales-detail-.*\.xls$/);
});

test('fallback report identifies its projection and keeps workbook exports available', async ({ page }) => {
  await page.route('**/v1/reports/unclassified-captured-report*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      kind: 'unclassified-captured-report',
      rows: [],
      page: 1,
      pageSize: 1000,
      hasMore: false,
      definition: {
        kind: 'unclassified-captured-report',
        title: 'Sale detail',
        projectionStatus: 'generic-fallback',
        projectionNote: 'Generic event-ledger fallback; exact legacy projection is not implemented.',
        columns: [{ key: 'document', label: 'Document', dataType: 'text', sortable: true }],
        formats: [{ id: 'event-ledger-projection', name: 'Event ledger projection', source: 'default' }],
        retrieval: { title: 'Specify Retrieval Arguements', areas: ['DEFAULT AREA'], supportsCashCredit: false },
        letterhead: { name: "Fazal Din's Pharma Plus", line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '055 3252501', fax: '', source: 'default' },
        exports: [
          { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
          { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
          { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
        ]
      }
    })
  }));
  await page.goto('/app/report/unclassified-captured-report');
  await page.getByRole('button', { name: 'Retrieve', exact: true }).click();
  await expect(page.getByText('Generic event-ledger fallback; exact legacy projection is not implemented.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Export report as Excel' })).toBeEnabled();
});

test('Sales Return detail uses the scoped sale-return projection', async ({ page }) => {
  await page.route('**/v1/reports/sales-return-detail*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      kind: 'sales-return-detail',
      rows: [{ document: 'RET-100', occurredAt: '2026-08-06', party: 'CASH', item: 'PARACETAMOL', quantity: '1', amount: '10.00' }],
      page: 1,
      pageSize: 50,
      hasMore: false,
      definition: {
        kind: 'sales-return-detail',
        title: 'Sales Return detail',
        projectionStatus: 'event-ledger',
        projectionNote: 'Scoped immutable event-ledger view; captured legacy grouping and calculated numeric fields are not implemented.',
        columns: [
          { key: 'document', label: 'Event / Document', dataType: 'text', sortable: true },
          { key: 'occurredAt', label: 'Occurred', dataType: 'date', sortable: true },
          { key: 'party', label: 'Party', dataType: 'text', sortable: true },
          { key: 'item', label: 'Item (first payload line)', dataType: 'text', sortable: true },
          { key: 'quantity', label: 'Quantity (payload)', dataType: 'number', sortable: true },
          { key: 'amount', label: 'Amount (payload)', dataType: 'currency', sortable: true }
        ],
        formats: [{ id: 'event-ledger-projection', name: 'Event ledger projection', source: 'default' }],
        retrieval: { title: 'Specify Retrieval Arguements', areas: ['DEFAULT AREA'], supportsCashCredit: false, supportsDateRange: true, supportsTextFilter: true, scope: 'tenant, branch, date, text, and immutable se.aggregate = sale_return' },
        letterhead: { name: "Fazal Din's Pharma Plus", line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '055 3252501', fax: '', source: 'default' },
        exports: [
          { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
          { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
          { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
        ]
      }
    })
  }));
  await page.goto('/app/report/sales-return-detail');
  await page.waitForTimeout(2000);
  await page.getByRole('button', { name: 'Refresh report' }).click();
  await expect(page.getByText('RET-100')).toBeVisible();
  await expect(page.getByText('Scoped immutable event-ledger view; captured legacy grouping and calculated numeric fields are not implemented.')).toBeVisible();
});

test('Sales Return fallback renders the optional API projection note', async ({ page }) => {
  const projectionNote = 'Sales return output is still a generic immutable-event fallback.';
  await page.route('**/v1/reports/sales-return-detail*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      kind: 'sales-return-detail',
      rows: [],
      page: 1,
      pageSize: 50,
      hasMore: false,
      definition: {
        kind: 'sales-return-detail',
        title: 'Sales Return detail',
        projectionStatus: 'generic-fallback',
        projectionNote,
        columns: [{ key: 'document', label: 'Document', dataType: 'text', sortable: true }],
        formats: [{ id: 'event-ledger-projection', name: 'Event ledger projection', source: 'default' }],
        retrieval: { title: 'Specify Retrieval Arguements', areas: ['DEFAULT AREA'], supportsCashCredit: false, supportsDateRange: true, supportsTextFilter: true, scope: 'tenant and branch scoped' },
        letterhead: { name: "Fazal Din's Pharma Plus", line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '055 3252501', fax: '', source: 'default' },
        exports: [
          { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
          { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
          { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
        ]
      }
    })
  }));
  await page.goto('/app/report/sales-return-detail');
  await page.waitForTimeout(2000);
  await page.getByRole('button', { name: 'Refresh report' }).click();
  await expect(page.getByText(projectionNote)).toBeVisible();
});

function purchaseReportDefinition(kind: string, title: string) {
  return {
    kind,
    title,
    projectionStatus: 'event-ledger',
    projectionNote: 'Canonical posted purchase business_documents/lines with supplier party ledger and stock ledger values when available; posted compatibility events are included only when no canonical document matches. Legacy grouping and unreconciled tax, profit, graph, and disparity calculations are not implemented.',
    columns: [
      { key: 'document', label: 'Document', dataType: 'text', sortable: true },
      { key: 'occurredAt', label: 'Date', dataType: 'date', sortable: true },
      { key: 'party', label: 'Customer/Supplier', dataType: 'text', sortable: true },
      { key: 'item', label: 'Item', dataType: 'text', sortable: true },
      { key: 'quantity', label: 'Quantity', dataType: 'number', sortable: true },
      { key: 'amount', label: 'Amount', dataType: 'currency', sortable: true }
    ],
    formats: [{ id: 'standard', name: 'Standard', source: 'default' }],
    retrieval: {
      title: 'Specify Retrieval Arguements',
      areas: ['DEFAULT AREA', 'ALL AREAS'],
      supportsCashCredit: false,
      supportsDateRange: true,
      supportsTextFilter: true,
      scope: 'tenant, branch, date, text, supplier, canonical purchase documents/lines, stock ledger, supplier party ledger, and posted compatibility events'
    },
    letterhead: { name: "Fazal Din's Pharma Plus", line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '055 3252501', fax: '', source: 'default' },
    exports: [
      { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
          { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
          { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
    ]
  };
}

test('purchase detail and summary reports use the purchase read-model contract', async ({ page }) => {
  await page.route('**/v1/reports/purchase-*', async (route) => {
    const kind = new URL(route.request().url()).pathname.split('/').pop() ?? 'purchase-detail';
    const title = kind === 'purchase-summary' ? 'Purchase summary' : 'Purchase detail';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        kind,
        rows: [{ document: 'PUR-100', occurredAt: '2026-08-06', party: 'SUPPLIER 1', item: 'CANONICAL ITEM', quantity: '2', amount: '20.00' }],
        page: 1,
        pageSize: 50,
        hasMore: false,
        definition: purchaseReportDefinition(kind, title)
      })
    });
  });
  for (const [path, title] of [['purchase-detail', 'Purchase detail'], ['purchase-summary', 'Purchase summary']] as const) {
    await page.goto(`/app/report/${path}`);
    await page.waitForTimeout(2000);
    await page.getByRole('button', { name: 'Refresh report' }).click({ force: true });
    await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
    await expect(page.getByText('SUPPLIER 1')).toBeVisible();
    await expect(page.getByText(/Canonical posted purchase business_documents/)).toBeVisible();
    await page.getByRole('button', { name: 'Preview report' }).click({ force: true });
    await expect(page.getByRole('dialog', { name: 'Print preview' })).toBeVisible();
    await expect(page.getByText("Fazal Din's Pharma Plus")).toBeVisible();
    await page.getByRole('button', { name: 'Close print preview', exact: true }).click({ force: true });
  }
});

test('purchase return, supplier, and purchase-order report leaves retain mapped navigation', async ({ page }) => {
  await page.route('**/v1/reports/*', async (route) => {
    const kind = new URL(route.request().url()).pathname.split('/').pop() ?? 'purchase-return-detail';
    const titles: Record<string, string> = {
      'purchase-return-detail': 'Purchase Return detail',
      'supplier-wise-detail': 'Detail',
      'purchase-order': 'Purchase Order'
    };
    const title = titles[kind] ?? kind;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        kind,
        rows: [{ document: 'PUR-RET-100', occurredAt: '2026-08-06', party: 'SUPPLIER 1', item: 'CANONICAL ITEM', quantity: '-1', amount: '10.00' }],
        page: 1,
        pageSize: 50,
        hasMore: false,
        definition: purchaseReportDefinition(kind, title)
      })
    });
  });
  for (const [path, legacyPath, title] of [
    ['purchase-return-detail', '&Reports > Daily Reports > Purchase Return > Purchase Return detail', 'Purchase Return detail'],
    ['supplier-wise-detail', '&Reports > Purchase Reports > Supplier Wise > Detail', 'Detail'],
    ['purchase-order', '&Reports > Purchase Reports > Purchase Order', 'Purchase Order']
  ] as const) {
    await page.goto(`/app/report/${path}?legacyPath=${encodeURIComponent(legacyPath)}`);
    await page.waitForTimeout(2000);
    await page.getByRole('button', { name: 'Refresh report' }).click({ force: true });
    await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
    await expect(page.getByText('PUR-RET-100')).toBeVisible();
  }
});

function stockReportDefinition(kind: string, title: string, movement = false) {
  const historical = kind === 'stock-in-hand-back-date';
  return {
    kind,
    title,
    projectionStatus: 'real',
    projectionNote: historical
      ? 'Source-backed historical_stock_snapshots imported from dbo.StockReport expose the captured as-of stock, purchase price, sale price, average price, recent purchase price, and pack-unit fields. Manufacturer/category/class/narcotics grouping and exact print calculations remain unverified.'
      : movement
      ? 'Normalized posted stock_ledger movement projection joined to batch, item, and godown metadata; compatibility inventory_movements and unreconciled legacy grouping are not included.'
      : 'Normalized posted stock_balances projection joined to stock batches, items, and godowns; legacy manufacturer/category/class/reorder/narcotics groupings and exact valuation are not implemented.',
    columns: historical
      ? [
          { key: 'document', label: 'Source Row', dataType: 'text', sortable: true },
          { key: 'occurredAt', label: 'As Of', dataType: 'date', sortable: true },
          { key: 'party', label: 'Godown', dataType: 'text', sortable: true },
          { key: 'item', label: 'Item', dataType: 'text', sortable: true },
          { key: 'quantity', label: 'Stock', dataType: 'number', sortable: true },
          { key: 'purchasePrice', label: 'Purchase Price', dataType: 'currency', sortable: true },
          { key: 'salePrice', label: 'Sale Price', dataType: 'currency', sortable: true },
          { key: 'averagePrice', label: 'Average Price', dataType: 'currency', sortable: true },
          { key: 'recentPurchasePrice', label: 'Recent Purchase Price', dataType: 'currency', sortable: true },
          { key: 'packUnits', label: 'Pack Units', dataType: 'number', sortable: true }
        ]
      : movement
      ? [
          { key: 'document', label: 'Movement', dataType: 'text', sortable: true },
          { key: 'occurredAt', label: 'Date', dataType: 'date', sortable: true },
          { key: 'party', label: 'Direction', dataType: 'text', sortable: true },
          { key: 'item', label: 'Item', dataType: 'text', sortable: true },
          { key: 'quantity', label: 'Quantity', dataType: 'number', sortable: true },
          { key: 'amount', label: 'Unit Cost', dataType: 'currency', sortable: true }
        ]
      : [
          { key: 'document', label: 'Batch', dataType: 'text', sortable: true },
          { key: 'occurredAt', label: 'Expiry/Updated', dataType: 'date', sortable: true },
          { key: 'party', label: 'Godown', dataType: 'text', sortable: true },
          { key: 'item', label: 'Item', dataType: 'text', sortable: true },
          { key: 'quantity', label: 'On Hand', dataType: 'number', sortable: true },
          { key: 'amount', label: 'Unit Cost', dataType: 'currency', sortable: true }
        ],
    formats: [{ id: 'standard', name: 'Standard', source: 'default' }],
    retrieval: {
      title: 'Specify Retrieval Arguements',
      areas: ['DEFAULT AREA', 'ALL AREAS'],
      supportsCashCredit: false,
      supportsDateRange: true,
      supportsTextFilter: true,
      scope: historical
        ? 'tenant, branch, as-of date, text, godown, and imported historical_stock_snapshots'
        : 'tenant, branch, date, text, godown, batch, posted stock_ledger, and normalized stock_balances'
    },
    letterhead: { name: "Fazal Din's Pharma Plus", line2: 'NRY Pacific', line3: "Franchise Fazal Din's", phone: '055 3252501', fax: '', source: 'default' },
    exports: [
      { format: 'csv', status: 'available', label: 'CSV', message: 'CSV export is available.' },
      { format: 'pdf', status: 'available', label: 'PDF', message: 'PDF export uses the print-preview letterhead and browser Save as PDF.' },
      { format: 'excel', status: 'available', label: 'Excel', message: 'Excel-compatible workbook download is available.' }
    ]
  };
}

test('stock report leaves use scoped normalized and historical stock metadata', async ({ page }) => {
  const requestedFilters: string[] = [];
  const requestedGodowns: string[] = [];
  const requestedBatches: string[] = [];
  await page.route('**/v1/reports/*', async (route) => {
    const requestURL = new URL(route.request().url());
    requestedFilters.push(requestURL.searchParams.get('filter') ?? '');
    requestedGodowns.push(requestURL.searchParams.get('godownId') ?? '');
    requestedBatches.push(requestURL.searchParams.get('batchNumber') ?? '');
    const kind = requestURL.pathname.split('/').pop() ?? 'stock-in-hand-batch-priority-wise';
    const historical = kind === 'stock-in-hand-back-date';
    const movement = kind === 'daily-stock-in-out';
    const title = historical ? 'Back Date' : kind === 'expiry-report' ? 'Expiry Report' : kind === 'daily-stock-in-out' ? 'Daily Stock IN/OUT' : 'Batch, Priority Wise';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        kind,
        rows: [{ document: historical ? 'STOCK-2026-08-06' : 'B-001', occurredAt: historical ? '2026-08-06' : '2030-01-01', party: 'GODOWN1', item: 'CANONICAL ITEM', quantity: movement ? '2' : '7', amount: '4.00', purchasePrice: '3.00', salePrice: '5.00', averagePrice: '4.00', recentPurchasePrice: '3.50', packUnits: '10' }],
        page: 1,
        pageSize: 50,
        hasMore: false,
        definition: stockReportDefinition(kind, title, movement)
      })
    });
  });
  for (const [path, legacyPath, title] of [
    ['stock-in-hand-batch-priority-wise', 'Reports > Stock Reports > Stock In hand > Batch, Priority Wise', 'Batch, Priority Wise'],
    ['stock-in-hand-back-date', 'Reports > Stock Reports > Stock In hand > Back Date', 'Back Date'],
    ['expiry-report', 'Reports > Stock Reports > Expiry Report', 'Expiry Report'],
    ['daily-stock-in-out', 'Reports > Stock Reports > Daily Stock IN/OUT', 'Daily Stock IN/OUT']
  ] as const) {
    const scope = path === 'stock-in-hand-batch-priority-wise'
      ? '&godownId=33333333-3333-4333-8333-333333333333&batchNumber=B-001'
      : '';
    await page.goto(`/app/report/${path}?legacyPath=${encodeURIComponent(legacyPath)}${scope}`);
    await page.waitForTimeout(1500);
    if (path === 'stock-in-hand-batch-priority-wise') {
      await page.getByLabel('Filter:').fill('B-001');
    }
    await page.getByRole('button', { name: 'Refresh report' }).click({ force: true });
    await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
    await expect(page.getByText('GODOWN1')).toBeVisible();
    await expect(page.getByText(path === 'stock-in-hand-back-date' ? /Source-backed historical_stock_snapshots/ : /Normalized posted stock_/)).toBeVisible();
  }
  expect(requestedFilters).toContain('B-001');
  expect(requestedGodowns).toContain('33333333-3333-4333-8333-333333333333');
  expect(requestedBatches).toContain('B-001');
});

test('parity workflow surfaces are reachable for transactions, master data, reports, and maintenance', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: {
        tenantId: 'tenant-1',
        tenantCode: 'TENANT',
        branchId: 'branch-1',
        counterId: 'counter-1',
        operatorId: 'operator-1',
        displayName: 'ADMIN'
      }
    })
  }));
  for (const [path, label] of [
    ['/app/purchase/pack', 'Pack Purchase'],
    ['/app/master/customer', 'Customer'],
    ['/app/report/daily-sales-detail', 'Daily Sales Detail'],
    ['/app/preferences', 'Preferences'],
    ['/app/maintenance/check-database-integrity', 'Check Database Integrity'],
    ['/app/manage/groups', 'Groups']
  ] as const) {
    await page.goto(path);
    await expect(page.locator('main')).toBeVisible();
    await expect(page.locator(`section[aria-label="${label}"]`)).toBeVisible();
    if (path === '/app/preferences' || path.includes('/maintenance/') || path.includes('/manage/')) {
      await expect(page.getByRole('button', { name: 'File', exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Window', exact: true })).toBeVisible();
    }
  }
});

test('backup request reports deployment policy status instead of claiming a backup', async ({ page }) => {
  await page.addInitScript(() => localStorage.removeItem('abuzar.apiBaseUrl'));
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN', roles: ['tenant_admin'], permissions: ['maintenance.write'] } })
  }));
  await page.route('**/v1/maintenance/backup-database', async (route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ kind: 'backup-database', items: [], operations: [] }) });
    }
    return route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ kind: 'backup-database', operationId: 'op-1', status: 'not_configured', message: 'No deployment backup/restore adapter is configured; no database backup or restore was performed.' })
    });
  });
  await page.goto('/app/maintenance/backup-database');
  await page.waitForTimeout(1000);
  await page.getByRole('button', { name: 'BackUp Now' }).evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.getByText(/No deployment backup\/restore adapter is configured/)).toBeVisible();
  await expect(page.getByText('Operation op-1: not_configured')).toBeVisible();
});

test('session monitor displays only the authenticated branch session set', async ({ page }) => {
  await page.addInitScript(() => localStorage.removeItem('abuzar.apiBaseUrl'));
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-a', counterId: 'counter-a', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN', roles: ['tenant_admin'], permissions: ['manage.users'] } })
  }));
  await page.route('**/v1/session-monitor', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      tenantId: 'tenant-1',
      branchId: 'branch-a',
      sessions: [{ userId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN', branchId: 'branch-a', counterId: 'counter-a', createdAt: '2026-08-06T01:00:00Z', lastSeenAt: '2026-08-06T01:05:00Z', expiresAt: '2026-08-06T09:00:00Z', current: true }]
    })
  }));
  await page.goto('/app/manage/session-monitor');
  await expect(page.getByRole('heading', { name: 'Session Monitor', exact: true })).toBeVisible();
  await expect(page.getByText('branch-a')).toBeVisible();
  await expect(page.getByText('branch-b')).not.toBeVisible();
});

test('generic catalog fallback pages survive direct SSR navigation', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: {
        tenantId: 'tenant-1',
        tenantCode: 'TENANT',
        branchId: 'branch-1',
        counterId: 'counter-1',
        operatorId: 'operator-1',
        displayName: 'ADMIN'
      }
    })
  }));
  for (const path of ['/app/module/about', '/app/module/purchases', '/app/module/purchase-orders', '/app/maintenance/change-items-price', '/app/manage/group-wise-header-setting']) {
    const response = await page.goto(path);
    expect(response?.status()).toBe(200);
    await expect(page.locator('main')).toBeVisible();
    if (path.startsWith('/app/module/')) {
      await expect(page.getByRole('button', { name: 'File', exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Window', exact: true })).toBeVisible();
    }
  }
});

test('critical deep links SSR at HTTP 200 with a visible main surface', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: {
        tenantId: 'tenant-1',
        tenantCode: 'TENANT',
        branchId: 'branch-1',
        counterId: 'counter-1',
        operatorId: 'operator-1',
        displayName: 'ADMIN'
      }
    })
  }));
  for (const path of ['/login', '/app/legacy', '/app/sales', '/app/module/purchase-orders']) {
    const response = await page.goto(path, { waitUntil: 'domcontentloaded' });
    expect(response?.status(), `${path} should return HTTP 200`).toBe(200);
    await expect(page.locator('main'), `${path} should expose a visible main surface`).toBeVisible();
  }
});

test('route-specific maintenance fields replace the generic fallback after interaction', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/maintenance/**', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ kind: 'change-items-price', items: [] })
  }));
  await page.goto('/app/maintenance/change-items-price');
  await page.locator('main').click({ position: { x: 30, y: 120 }, force: true });
  await expect(page.getByText('Item Code:', { exact: true })).toBeVisible();
  await expect(page.getByRole('combobox', { name: 'Price Type:' })).toBeVisible();
});

test('Change Items Price submits the captured fields to the canonical maintenance endpoint', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  let requestBody: Record<string, unknown> | undefined;
  await page.route('**/v1/maintenance/change-items-price', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ kind: 'change-items-price', items: [] }) });
      return;
    }
    requestBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ kind: 'change-items-price', operationId: 'op-price-1', status: 'completed', message: 'Sale Price for item ITEM-1 updated in the canonical item master.' })
    });
  });
  const stateResponse = page.waitForResponse((response) => response.url().includes('/v1/maintenance/change-items-price') && response.request().method() === 'GET');
  await page.goto('/app/maintenance/change-items-price');
  await stateResponse;
  await page.getByLabel('Item Code:').fill('ITEM-1');
  await page.getByRole('combobox', { name: 'Price Type:' }).selectOption({ label: 'Sale Price' });
  await page.getByLabel('New Price:').fill('12.75');
  const maintenanceRequest = page.waitForRequest((request) => request.url().includes('/v1/maintenance/change-items-price') && request.method() === 'POST');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await maintenanceRequest;
  await expect(page.getByText('Sale Price for item ITEM-1 updated in the canonical item master.')).toBeVisible();
  expect(requestBody).toMatchObject({ itemCode: 'ITEM-1', priceType: 'Sale Price', price: 12.75 });
});

test('Lock Item Batches submits the captured batch state to the maintenance endpoint', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  let requestBody: Record<string, unknown> | undefined;
  await page.route('**/v1/maintenance/lock-item-batches', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ kind: 'lock-item-batches', items: [] }) });
      return;
    }
    requestBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ kind: 'lock-item-batches', operationId: 'op-lock-1', status: 'completed', message: '1 batch row(s) for item ITEM-1 were locked in the current branch.' })
    });
  });
  const stateResponse = page.waitForResponse((response) => response.url().includes('/v1/maintenance/lock-item-batches') && response.request().method() === 'GET');
  await page.goto('/app/maintenance/lock-item-batches');
  await stateResponse;
  await page.getByLabel('Item Code:').fill('ITEM-1');
  await page.getByLabel('Batch:').fill('B-1');
  await page.getByRole('combobox', { name: 'Locked:' }).selectOption({ label: 'Yes' });
  const maintenanceRequest = page.waitForRequest((request) => request.url().includes('/v1/maintenance/lock-item-batches') && request.method() === 'POST');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await maintenanceRequest;
  await expect(page.getByText('1 batch row(s) for item ITEM-1 were locked in the current branch.')).toBeVisible();
  expect(requestBody).toMatchObject({ itemCode: 'ITEM-1', batch: 'B-1', locked: 'Yes' });
});

test('Opening Stock posts a canonical inbound inventory event', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: '22222222-2222-4222-8222-222222222222', kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] })
  }));
  await page.route('**/v1/items/lookup**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [{ id: '11111111-1111-4111-8111-111111111111', legacyId: 'ITEM-1', code: 'ITEM-1', name: 'Canonical Item', payload: {}, active: true, aliases: [] }] })
    });
  });
  await page.route('**/v1/maintenance/opening-stock', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ kind: 'opening-stock', items: [] }) });
      return;
    }
    throw new Error('Opening Stock must not use the generic maintenance endpoint.');
  });
  let requestBody: Record<string, unknown> | undefined;
  await page.route('**/v1/transactions/inventory', async (route) => {
    requestBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ accepted: true }) });
  });
  const godownResponse = page.waitForResponse((res) => res.url().includes('/v1/master/godown'));
  await page.goto('/app/maintenance/opening-stock');
  await godownResponse;
  const itemInput = page.locator('#adjustment-item-input');
  await expect(itemInput).toBeVisible();
  await itemInput.fill('Canonical');
  await page.getByRole('button', { name: 'Lookup adjustment item' }).click();
  await expect(page.getByRole('button', { name: 'Canonical Item (ITEM-1)' })).toBeVisible();
  await page.getByRole('button', { name: 'Canonical Item (ITEM-1)' }).click();
  await page.getByLabel('Quantity:').fill('2.5');
  await expect(page.getByLabel('Item legacy ID:')).toHaveValue('ITEM-1');
  await page.getByLabel('Godown ID:').selectOption('22222222-2222-4222-8222-222222222222');
  await page.getByLabel('Batch:').fill('OPEN-1');
  await page.getByLabel('Unit cost:').fill('1.25');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Opening Stock posted successfully.')).toBeVisible();
  expect(requestBody).toMatchObject({ payload: { itemLegacyId: 'ITEM-1', quantity: '2.5', direction: 'in', godownId: '22222222-2222-4222-8222-222222222222', batchNumber: 'OPEN-1' } });
});

test('captured preference form tabs expose their native legacy layouts after interaction', async ({ page }) => {
  await page.goto('/app/preferences');
  await page.waitForTimeout(500);
  await page.locator('.legacy-preferences-tabs button').nth(8).evaluate((element) => (element as HTMLButtonElement).click());
  await page.locator('main').click({ position: { x: 30, y: 120 }, force: true });
  await expect(page.getByLabel('Schedule preferences')).toBeVisible();
  await page.locator('.legacy-preferences-tabs button').nth(14).evaluate((element) => (element as HTMLButtonElement).click());
  await page.locator('main').click({ position: { x: 30, y: 120 }, force: true });
  await expect(page.getByLabel('Email preferences')).toBeVisible();
});

test('Users list selection reopens the detail form and persists an operator update', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/operators', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ operators: [{ id: 'user-1', username: 'CASHIER', displayName: 'Cashier One', active: true, roles: ['tenant_admin'] }] })
  }));
  await page.route('**/v1/roles', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ roles: [{ id: 'role-1', code: 'tenant_admin', name: 'Administrator', memberCount: 1, permissions: [] }] })
  }));
  await page.route('**/v1/branches', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ branches: [{ id: 'branch-1', code: 'MAIN', name: 'Main', timezone: 'Asia/Karachi', active: true }] }) }));
  await page.route('**/v1/counters*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ counters: [{ id: 'counter-1', branchId: 'branch-1', code: 'C1', name: 'Counter 1', active: true }] }) }));
  let updateBody = '';
  await page.route('**/v1/operators/user-1', async (route) => {
    updateBody = route.request().postData() ?? '';
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: 'user-1', username: 'CASHIER', displayName: 'Cashier Updated', active: true, roles: ['tenant_admin'] }) });
  });
  await page.goto('/app/master/user');
  await page.waitForTimeout(500);
  await page.getByRole('button', { name: 'CASHIER', exact: true }).click({ force: true });
  await expect(page.getByLabel('User Name:')).toHaveValue('Cashier One');
  await page.getByLabel('User Name:').fill('Cashier Updated');
  await page.locator('.legacy-master-actions button[type="submit"]').click();
  await expect.poll(() => updateBody).toContain('Cashier Updated');
});

test('Item detail exposes and persists the supplier grid', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/master/item**', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: 'item-1', kind: 'item', legacyId: 'I-1', code: 'I-1', name: 'Imported Item', payload: {}, active: true, suppliers: [{ id: 'link-1', legacySupplierId: 'SUP-1', priority: 1, rate: '10.00', discountPercent: '2.00', quantity: '5', bonus: '1', days: 30 }] }] }) });
      return;
    }
    if (route.request().method() === 'PATCH') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: 'item-1', kind: 'item', legacyId: 'I-1', code: 'I-1', name: 'Imported Item', payload: {}, active: true, suppliers: [] }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ suppliers: [{ id: 'link-1', legacySupplierId: 'SUP-1', priority: 1, rate: '10.00', discountPercent: '2.50', quantity: '5', bonus: '1', days: 30 }] }) });
  });
  let supplierBody = '';
  await page.route('**/v1/master/item/item-1/suppliers', async (route) => {
    supplierBody = route.request().postData() ?? '';
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ suppliers: [{ id: 'link-1', legacySupplierId: 'SUP-1', priority: 1, rate: '10.00', discountPercent: '2.50', quantity: '5', bonus: '1', days: 30 }] }) });
  });
  await page.goto('/app/master/item');
  await page.waitForTimeout(400);
  await page.getByRole('button', { name: 'I-1', exact: true }).click({ force: true });
  await expect(page.getByLabel('Supplier legacy id 1')).toHaveValue('SUP-1');
  await page.getByLabel('Supplier discount percent 1').fill('2.50');
  await page.locator('.legacy-master-actions button[type="submit"]').click({ force: true });
  await expect.poll(() => supplierBody).toContain('2.50');
});

test('Groups editor loads permissions and persists the selected permission set', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', displayName: 'ADMIN', roles: ['tenant_admin'] } })
  }));
  let roleBody = '';
  await page.route('**/v1/roles**', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ roles: [{ id: 'role-1', code: 'operator', name: 'Operator', memberCount: 1, permissions: ['sales.read'] }] }) });
      return;
    }
    roleBody = route.request().postData() ?? '';
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ id: 'role-2', code: 'manager', name: 'Manager', memberCount: 0, permissions: ['reports.read', 'sales.read'] }) });
  });
  await page.goto('/app/manage/groups');
  await page.waitForTimeout(500);
  await expect(page.getByLabel('Group Code:')).toBeVisible();
  await page.getByLabel('Group Code:').fill('manager');
  await page.getByLabel('Group Name:').fill('Manager');
  const reportsPermission = page.getByLabel('Reports - view');
  await reportsPermission.check();
  await expect(reportsPermission).toBeChecked();
  await page.getByRole('button', { name: 'Save group' }).click({ force: true });
  await expect(page.locator('section[aria-label="Groups"] footer [role="status"]')).toHaveText('Manager created successfully.');
  expect(JSON.parse(roleBody)).toMatchObject({ code: 'manager', name: 'Manager', permissions: ['reports.read'] });
});

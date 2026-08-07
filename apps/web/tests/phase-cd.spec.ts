import { expect, test } from '@playwright/test';
import {
  buildLegacyMenusForContext,
  contextualCatalogCounts,
  findShortcutAction,
  type LegacyMenu
} from '../src/lib/legacy-menu';
import { contextualLegacyMenuCatalog as contextualCatalog } from '../src/lib/legacy-menu-contextual-catalog';
import { createLegacyWindowRegistry } from '../src/lib/legacy-window-registry';

test.beforeEach(async ({ page }) => {
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
});
function menuLabels(menus: LegacyMenu[], label: string): string[] {
  return menus.find((menu) => menu.label === label)?.actions.map((action) => action.label) ?? [];
}

test('contextual catalog imports through the browser-safe TS module', () => {
  expect(contextualCatalog.catalogVersion).toBe('2026-08-06-contextual-1');
  expect(contextualCatalog.contexts.map((context) => context.windowType)).toEqual([
    'pack-purchase',
    'cash-sale',
    'item-master',
    'report-sale-detail',
    'manage-groups'
  ]);
});

test('versioned contextual catalog contains every captured item for each context', () => {
  const expected = {
    'pack-purchase': [325, 10],
    'cash-sale': [326, 10],
    'item-master': [314, 10],
    'report-sale-detail': [306, 9],
    'manage-groups': [295, 9]
  };
  expect(contextualCatalog.schemaVersion).toBe(1);
  expect(contextualCatalog.contexts).toHaveLength(5);
  for (const context of contextualCatalog.contexts) {
    expect(context.items).toHaveLength(expected[context.windowType as keyof typeof expected][0]);
    expect(context.topLevelCount).toBe(expected[context.windowType as keyof typeof expected][1]);
    expect(context.items).toHaveLength(context.menuItemCount);
  }
  expect(contextualCatalogCounts()['pack-purchase']).toEqual({ capturedItems: 325, topLevelMenus: 10 });
});

test('active-window menu swaps preserve captured contextual actions', () => {
  const purchase = buildLegacyMenusForContext('pack-purchase');
  const sales = buildLegacyMenusForContext('cash-sale');
  const item = buildLegacyMenusForContext('item-master');
  const report = buildLegacyMenusForContext('report-sale-detail');
  const groups = buildLegacyMenusForContext('manage-groups');

  expect(menuLabels(purchase, 'Item')).toEqual(['New Item', 'Delete Item', 'Restore Item']);
  expect(menuLabels(purchase, 'File')).toContain('Auto Batch Generation');
  expect(menuLabels(sales, 'File')).toContain('Item Sale History');
  expect(menuLabels(sales, 'File')).toContain('Sale Slip');
  expect(menuLabels(sales, 'File')).not.toContain('Item Purchase History');
  expect(menuLabels(item, 'File')).toContain('Select Models');
  const alternateAliases = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Set Alternate Item Alias Names');
  expect(alternateAliases?.implementation).toBe('implemented');
  expect(alternateAliases?.requiredPermission).toBe('master.write');
  const itemImages = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Set Item Image(s)');
  expect(itemImages?.implementation).toBe('implemented');
  expect(itemImages?.requiredPermission).toBe('master.write');
  const itemNotes = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Set Item Notes');
  expect(itemNotes?.implementation).toBe('implemented');
  expect(itemNotes?.requiredPermission).toBe('master.write');
  const itemAssociations = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Set Item Associations');
  expect(itemAssociations?.implementation).toBe('implemented');
  expect(itemAssociations?.requiredPermission).toBe('master.write');
  const itemAuthors = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Set Item Author(s)');
  expect(itemAuthors?.implementation).toBe('implemented');
  expect(itemAuthors?.requiredPermission).toBe('master.write');
  const itemModels = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Select Models');
  expect(itemModels?.implementation).toBe('implemented');
  expect(itemModels?.requiredPermission).toBe('master.write');
  const itemPricePolicy = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Set Item Price Policy');
  expect(itemPricePolicy?.implementation).toBe('implemented');
  expect(itemPricePolicy?.requiredPermission).toBe('master.write');
  const registrationRequest = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Populate Item Registration Request');
  expect(registrationRequest?.implementation).toBe('implemented');
  expect(registrationRequest?.requiredPermission).toBe('master.write');
  const populateItem = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Populate Item');
  expect(populateItem?.implementation).toBe('implemented');
  expect(populateItem?.requiredPermission).toBe('master.read');
  const unpostedTransactions = buildLegacyMenusForContext('item-master').find((menu) => menu.label === 'File')?.actions.find((action) => action.label === 'Show Un-Posted Transaction Report');
  expect(unpostedTransactions?.implementation).toBe('implemented');
  expect(unpostedTransactions?.requiredPermission).toBe('master.read');
  expect(menuLabels(report, 'File')).toContain('Retrieve');
  expect(menuLabels(groups, 'File')).toContain('Detail');
  expect(menuLabels(report, 'Item')).toEqual([]);
});

test('contextual accelerators resolve against the active menu set', () => {
  expect(findShortcutAction(buildLegacyMenusForContext('pack-purchase'), 'Ctrl+Q')?.label).toBe('Save And Post');
  expect(findShortcutAction(buildLegacyMenusForContext('item-master'), 'Ctrl+I')?.label).toBe('New Item');
  expect(findShortcutAction(buildLegacyMenusForContext('report-sale-detail'), 'Ctrl+R')?.label).toBe('Retrieve');
  expect(findShortcutAction(buildLegacyMenusForContext('manage-groups'), 'Ctrl+Q')).toBeUndefined();
});

test('window registry has safe 0, 1, and n-window states', () => {
  const registry = createLegacyWindowRegistry();
  expect(registry.snapshot().windows).toHaveLength(0);
  registry.open({ id: 'main', label: 'Main Window', href: '/app/legacy', context: 'base' });
  expect(registry.snapshot().windows).toHaveLength(1);
  registry.open({ id: 'sales-cash', label: 'Cash Sale', href: '/app/sales?kind=cash', context: 'cash-sale' });
  registry.open({ id: 'purchase-pack', label: 'Pack Purchase', href: '/app/purchase/pack', context: 'pack-purchase' });
  expect(registry.snapshot().windows).toHaveLength(3);
  registry.command('tile');
  expect(registry.snapshot().layout).toBe('tile');
  registry.command('refresh');
  expect(registry.snapshot().refreshToken).toBe(1);
});

test('Window menu preserves MDI entries across client-side document navigation', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' }
    })
  }));
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/legacy');
  await accessResponse;
  await expect(page.getByRole('button', { name: 'Sales', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Sales', exact: true }).click();
  await page.getByRole('menuitem', { name: 'CashSale', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/sales\?kind=cash/);

  await page.getByRole('button', { name: 'Window', exact: true }).click();
  await expect(page.getByRole('menuitem', { name: '1 Main Window', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: '2 Cash Sale', exact: true })).toBeVisible();
  await page.getByRole('menuitem', { name: '1 Main Window', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/legacy$/);

  await page.getByRole('button', { name: 'Window', exact: true }).click();
  await expect(page.getByRole('menuitem', { name: '2 Cash Sale', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('tab', { name: '2. Cash Sale', exact: true })).toBeVisible();
  await page.getByRole('tab', { name: '2. Cash Sale', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/sales\?kind=cash/);
});

test('cash sale dispatches Save And Post from its contextual menu', async ({ page }) => {
  const itemId = '11111111-1111-4111-8111-111111111111';
  const godownId = '22222222-2222-4222-8222-222222222222';
  const documentId = '33333333-3333-4333-8333-333333333333';
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' }
    })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] })
  }));
  await page.route('**/v1/items/lookup*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'MENU ITEM', payload: { salePrice: '10.00' }, active: true, aliases: [] }] })
  }));
  await page.route('**/v1/inventory/availability*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', itemLegacyId: 'ITEM-1', godownId, batches: [{ batchId: '44444444-4444-4444-8444-444444444444', batchNumber: 'B-1', quantity: '10', unitCost: '1.00' }] })
  }));
  await page.route('**/v1/transactions/preview', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '10.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '10.00', taxes: [], totalDiscount: '0.00', total: '10.00' })
  }));
  let postedPayload: Record<string, unknown> | undefined;
  await page.route('**/v1/documents/cash-sale', async (route) => {
    postedPayload = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ accepted: true, duplicate: false, eventId: '55555555-5555-4555-8555-555555555555', aggregateId: documentId, kind: 'cash-sale', action: 'save-and-post', status: 'posted', document: { id: documentId, documentNumber: 'CS-1', version: 1 } })
    });
  });
  await page.goto('/app/sales?kind=cash');
  await page.waitForTimeout(500);
  await expect(page.getByLabel('User:')).toHaveValue('ADMIN');
  await page.getByLabel('Item lookup query').fill('MENU');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'MENU ITEM' })).toBeVisible();
  await page.getByRole('button', { name: 'MENU ITEM' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await expect(page.getByLabel('Stock')).toHaveValue('10.00');
  await expect(page.getByRole('button', { name: 'File' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Item', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Window' })).toBeVisible();
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.keyboard.press('Control+q');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  const document = postedPayload?.document as Record<string, unknown>;
  expect(postedPayload?.kind).toBe('cash-sale');
  expect(postedPayload?.action).toBe('save-and-post');
  expect(document?.godownId).toBe(godownId);
  expect((document?.lines as Array<Record<string, unknown>>)?.[0]?.itemId).toBe(itemId);
});

test('canonical cash sale uses draft then post document lifecycle', async ({ page }) => {
  const itemId = '00000000-0000-0000-0000-000000000101';
  const godownId = '00000000-0000-0000-0000-000000000102';
  const documentId = '00000000-0000-0000-0000-000000000112';
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' }
    })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] })
  }));
  await page.route('**/v1/items/lookup*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-101', code: 'ITEM-101', name: 'CANONICAL ITEM', active: true, aliases: [], payload: { salePrice: '10.00' } }] })
  }));
  await page.route('**/v1/inventory/availability*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', itemLegacyId: 'ITEM-101', godownId, batches: [{ batchId: '00000000-0000-0000-0000-000000000103', batchNumber: 'B-1', quantity: '10', unitCost: '1.00' }] })
  }));
  await page.route('**/v1/transactions/preview', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '10.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '10.00', taxes: [], totalDiscount: '0.00', total: '10.00' })
  }));
  const commands: Array<Record<string, unknown>> = [];
  await page.route('**/v1/documents/cash-sale', async (route) => {
    const command = route.request().postDataJSON() as Record<string, unknown>;
    commands.push(command);
    const action = command.action;
    const posted = action !== 'save' && action !== 'void';
    const voided = action === 'void';
    await route.fulfill({
      status: posted || voided ? 200 : 201,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '00000000-0000-0000-0000-000000000111', aggregateId: documentId, kind: 'cash-sale', action, status: voided ? 'void' : posted ? 'posted' : 'draft', document: { id: documentId, documentNumber: 'BRANCH-000001', status: voided ? 'void' : posted ? 'posted' : 'draft', version: voided ? 3 : posted ? 2 : 1 } })
    });
  });
  await page.goto('/app/sales?kind=cash');
  await page.waitForTimeout(500);
  await page.getByLabel('Item lookup query').fill('CANONICAL');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'CANONICAL ITEM' })).toBeVisible({ timeout: 10000 });
  await page.getByRole('button', { name: 'CANONICAL ITEM' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Save', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('saved as draft', { timeout: 7000 });
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Post', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  await page.getByRole('button', { name: 'Void sale' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('voided successfully', { timeout: 7000 });
  expect(commands.map((command) => command.action)).toEqual(['save', 'post', 'void']);
  expect((commands[1].document as Record<string, unknown>).id).toBe(documentId);
});

test('purchase order uses the canonical document API and remains stock/GL-neutral', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' }
    })
  }));
  await page.route('**/v1/items/lookup*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: '11111111-1111-4111-8111-111111111111', legacyId: 'ITEM-1', code: 'ITEM-1', name: 'ORDER ITEM', active: true, aliases: [], payload: {} }] })
  }));
  await page.route('**/v1/master/supplier', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: '22222222-2222-4222-8222-222222222222', legacyId: 'SUP-1', code: 'SUP-1', name: 'SUPPLIER 1', active: true, payload: {} }] })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: '33333333-3333-4333-8333-333333333333', legacyId: 'G-1', code: 'G-1', name: 'GODOWN 1', active: true, payload: {} }] })
  }));
  await page.route('**/v1/documents/purchase-order', async (route) => {
    const command = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '55555555-5555-4555-8555-555555555555', aggregateId: '44444444-4444-4444-8444-444444444444', kind: 'purchase-order', action: command.action, status: 'draft', document: { id: '44444444-4444-4444-8444-444444444444', kind: 'purchase-order', documentNumber: 'PO-1', status: 'draft', version: 1 } })
    });
  });
  const supplierLoad = page.waitForResponse('**/v1/master/supplier');
  await page.goto('/app/purchase/order');
  await supplierLoad;
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ORDER ITEM');
  await page.getByRole('button', { name: 'Lookup item 1' }).click();
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByLabel('Purchase price 1').fill('4.00');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('stock/GL-neutral', { timeout: 7000 });
});

test('pack purchase Ctrl+B generates a deterministic batch identifier', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' }
    })
  }));
  await page.goto('/app/purchase/pack');
  await page.waitForTimeout(700);
  await page.getByRole('combobox', { name: 'Item name 1' }).fill('BATCH ITEM');
  await expect(page.getByRole('button', { name: 'File', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Auto Batch Generation', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('batch identifier generated');
  await expect(page.getByLabel('Batch 1')).toHaveValue(/^AUTO-\d{8}-001$/);
});

test('pack purchase contextual GST and expense commands update the live draft', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      tenantAdmin: true,
      permissions: [],
      legacyRights: [],
      scopes: {},
      scopeRows: [],
      exceptions: []
    })
  }));
  await page.route('**/v1/master/supplier', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [] })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [] })
  }));
  await page.goto('/app/purchase/pack');
  await page.waitForTimeout(700);
  await page.getByLabel('Item GST percent').fill('18');
  await expect(page.getByRole('button', { name: 'File', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Apply Item GST %', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('no populated lines');
  await page.getByLabel('Item name 1').fill('BATCH ITEM');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Apply Item GST %', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('18% applied');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Show Purchase Expenses Window', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Purchase Expenses' })).toBeVisible();
});

test('purchase invoice population hydrates canonical order lines and batch detail', async ({ page }) => {
  const documentId = '99999999-9999-4999-8999-999999999999';
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await page.route('**/v1/master/supplier', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: 'supplier-1', code: 'SUP-1', name: 'Canonical Supplier', active: true, payload: {} }] })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: 'godown-1', code: 'G-1', name: 'Main Godown', active: true, payload: {} }] })
  }));
  await page.route('**/v1/transactions/purchase-order*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ kind: 'purchase-order', rows: [{ documentId, document: 'PO-1', occurredAt: '2026-08-06T12:00:00Z', party: 'Canonical Supplier', item: 'Hydrated Item', quantity: '2', amount: '9.00' }] })
  }));
  await page.route(`**/v1/documents/${documentId}`, async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      id: documentId,
      kind: 'purchase-order',
      status: 'posted',
      documentNumber: 'PO-1',
      tenantId: 'tenant-1',
      branchId: 'branch-1',
      counterId: 'counter-1',
      operatorId: 'operator-1',
      supplierId: 'supplier-1',
      godownId: 'godown-1',
      occurredAt: '2026-08-06T12:00:00Z',
      lines: [{
        id: 'line-1',
        lineNumber: 1,
        itemId: 'item-1',
        itemLegacyId: 'ITEM-1',
        itemCode: 'ITEM-1',
        itemName: 'Hydrated Item',
        quantity: '2',
        price: { priceTier: 1, unitPrice: '4.50', grossAmount: '9.00', discountPercent: '0.00', discountAmount: '0.00', netAmount: '9.00' },
        tax: { taxableAmount: '9.00', amount: '0.00', lines: [] },
        allocations: [],
        batchNumber: 'PO-BATCH-1',
        expiryDate: '2027-08-06',
        unitCost: '4.50',
        taxAmount: '0.00',
        stock: { direction: 'none', quantity: '2' },
        lineTotal: '9.00'
      }],
      totals: { subtotal: '9.00', discountAmount: '0.00', miscAmount: '0.00', taxAmount: '0.00', totalAmount: '9.00', paidAmount: '0.00', balanceAmount: '9.00' },
      version: 1,
      createdAt: '2026-08-06T12:00:00Z',
      updatedAt: '2026-08-06T12:00:00Z'
    })
  }));
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/purchase/pack');
  await accessResponse;
  await page.waitForTimeout(200);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Populate Purchase Invoice', exact: true }).click();
  await expect(page.getByRole('button', { name: 'PO-1', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'PO-1', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('lines populated from canonical history');
  await expect(page.getByLabel('Item name 1')).toHaveValue('Hydrated Item');
  await expect(page.getByLabel('Batch 1')).toHaveValue('PO-BATCH-1');
  await expect(page.getByLabel('Expiry 1')).toHaveValue('2027-08-06');
  await expect(page.getByLabel('Quantity 1')).toHaveValue('2');
  await expect(page.getByLabel('Purchase price 1')).toHaveValue('4.50');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Fetch Purchase Invoice From Other Sources', exact: true }).click();
  await expect(page.getByRole('button', { name: 'PO-1', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'PO-1', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('lines populated from canonical history');
});

test('purchase return population preserves the canonical source document and batch', async ({ page }) => {
  const sourceDocumentId = '88888888-8888-4888-8888-888888888888';
  let returnCommand: Record<string, unknown> | null = null;
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await page.route('**/v1/master/supplier', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: 'supplier-1', code: 'SUP-1', name: 'Canonical Supplier', active: true, payload: {} }] })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: 'godown-1', code: 'G-1', name: 'Main Godown', active: true, payload: {} }] })
  }));
  await page.route('**/v1/transactions/pack-purchase*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ kind: 'pack-purchase', rows: [{ documentId: sourceDocumentId, document: 'PUR-1', occurredAt: '2026-08-06T12:00:00Z', party: 'Canonical Supplier', item: 'Returnable Item', quantity: '2', amount: '9.00' }] })
  }));
  await page.route(`**/v1/documents/${sourceDocumentId}`, async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      id: sourceDocumentId,
      kind: 'pack-purchase',
      status: 'posted',
      documentNumber: 'PUR-1',
      supplierId: 'supplier-1',
      godownId: 'godown-1',
      occurredAt: '2026-08-06T12:00:00Z',
      lines: [{
        id: 'line-1',
        lineNumber: 1,
        itemId: '11111111-1111-4111-8111-111111111111',
        itemLegacyId: 'ITEM-1',
        itemCode: 'ITEM-1',
        itemName: 'Returnable Item',
        quantity: '2',
        price: { priceTier: 1, unitPrice: '4.50', grossAmount: '9.00', discountPercent: '0.00', discountAmount: '0.00', netAmount: '9.00' },
        tax: { taxableAmount: '9.00', amount: '0.00', lines: [] },
        allocations: [{ batchId: '22222222-2222-4222-8222-222222222222', batchNumber: 'PUR-BATCH-1', expiryDate: '2027-08-06', quantity: '2' }],
        batchNumber: 'PUR-BATCH-1',
        expiryDate: '2027-08-06',
        unitCost: '4.50',
        taxAmount: '0.00',
        stock: { direction: 'in', quantity: '2' },
        lineTotal: '9.00'
      }],
      totals: { subtotal: '9.00', discountAmount: '0.00', miscAmount: '0.00', taxAmount: '0.00', totalAmount: '9.00', paidAmount: '0.00', balanceAmount: '9.00' },
      version: 1,
      createdAt: '2026-08-06T12:00:00Z',
      updatedAt: '2026-08-06T12:00:00Z'
    })
  }));
  await page.route('**/v1/documents/purchase-return', async (route) => {
    returnCommand = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '77777777-7777-4777-8777-777777777777', aggregateId: '66666666-6666-4666-8666-666666666666', kind: 'purchase-return', action: 'save-and-post', status: 'posted', document: { id: '66666666-6666-4666-8666-666666666666', documentNumber: 'PR-1', version: 1 } })
    });
  });
  const accessResponse = page.waitForResponse('**/v1/access');
  await page.goto('/app/purchase/return');
  await accessResponse;
  await page.waitForTimeout(200);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Populate Purchase Return Invoice', exact: true }).click();
  await expect(page.getByRole('button', { name: 'PUR-1', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'PUR-1', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('lines populated from canonical history');
  await expect(page.getByLabel('Item name 1')).toHaveValue('Returnable Item');
  await expect(page.getByLabel('Source document ID')).toHaveValue(sourceDocumentId);
  await expect(page.getByLabel('Source batch ID 1')).toHaveValue('22222222-2222-4222-8222-222222222222');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect.poll(() => returnCommand).not.toBeNull();
  const capturedCommand = returnCommand as unknown as Record<string, unknown>;
  const document = capturedCommand.document as Record<string, unknown>;
  expect(document.sourceDocumentId).toBe(sourceDocumentId);
  expect((document.lines as Array<Record<string, unknown>>)[0].allocations).toEqual([{
    batchId: '22222222-2222-4222-8222-222222222222',
    batchNumber: 'PUR-BATCH-1',
    quantity: '2'
  }]);
});

test('cash sale contextual item tax and document commands update the live draft', async ({ page }) => {
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
    tenantAdmin: true,
    permissions: [],
    legacyRights: [],
    scopes: {},
    scopeRows: [],
    exceptions: []
    })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [] })
  }));
  await page.goto('/app/sales?kind=cash');
  await page.waitForTimeout(700);
  await page.getByLabel('Item GST percent').fill('18');
  await page.locator('.legacy-sale-grid').getByLabel('Item name 1').fill('SALE ITEM');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Apply Item GST %', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('18% applied');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Show Document Gallery', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('no attachments selected');
});

test('quotation uses the canonical no-stock document lifecycle', async ({ page }) => {
  const itemId = '77777777-7777-4777-8777-777777777777';
  const documentId = '88888888-8888-4888-8888-888888888888';
  let postedPayload: Record<string, unknown> | undefined;
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [] })
  }));
  await page.route('**/v1/items/lookup*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'QUOTE-ITEM', code: 'QUOTE-ITEM', name: 'QUOTE ITEM', payload: { salePrice: '12.00' }, active: true, aliases: [] }] })
  }));
  await page.route('**/v1/transactions/preview', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '12.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '12.00', taxes: [], totalDiscount: '0.00', total: '12.00' })
  }));
  await page.route('**/v1/documents/quotation', async (route) => {
    postedPayload = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '99999999-9999-4999-8999-999999999999', aggregateId: documentId, kind: 'quotation', action: 'save-and-post', status: 'posted', document: { id: documentId, documentNumber: 'Q-1', version: 1 } })
    });
  });
  await page.goto('/app/sales?kind=quotation');
  await page.waitForTimeout(700);
  await page.getByLabel('Item lookup query').fill('QUOTE');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'QUOTE ITEM' })).toBeVisible();
  await page.getByRole('button', { name: 'QUOTE ITEM' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.keyboard.press('Control+q');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(postedPayload?.kind).toBe('quotation');
  expect((postedPayload?.document as Record<string, unknown>)?.godownId).toBeUndefined();
});

test('cash sale return posts through the canonical source-bound lifecycle', async ({ page }) => {
  const itemId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
  const godownId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
  const sourceDocumentId = 'cccccccc-cccc-4ccc-8ccc-cccccccccccc';
  const sourceLineId = 'eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee';
  const documentId = 'dddddddd-dddd-4ddd-8ddd-dddddddddddd';
  let postedPayload: Record<string, unknown> | undefined;
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', active: true, payload: {} }] })
  }));
  await page.route('**/v1/items/lookup**', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'RETURN-ITEM', code: 'RETURN-ITEM', name: 'RETURN ITEM', payload: { salePrice: '8.00' }, active: true, aliases: [] }] })
  }));
  await page.route('**/v1/transactions/preview', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '8.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '8.00', taxes: [], totalDiscount: '0.00', total: '8.00' })
  }));
  await page.route('**/v1/documents/cash-return', async (route) => {
    postedPayload = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: 'ffffffff-ffff-4fff-8fff-ffffffffffff', aggregateId: documentId, kind: 'cash-return', action: 'save-and-post', status: 'posted', document: { id: documentId, documentNumber: 'CR-1', version: 1 } })
    });
  });
  await page.goto('/app/sales?kind=cash-return');
  await page.waitForTimeout(700);
  await page.getByLabel('Item lookup query').fill('RETURN');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'RETURN ITEM' })).toBeVisible();
  await page.getByRole('button', { name: 'RETURN ITEM' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByLabel('Source document ID').fill(sourceDocumentId);
  await page.getByLabel('Source sale line ID 1').fill(sourceLineId);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.keyboard.press('Control+q');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(postedPayload?.kind).toBe('cash-return');
  expect((postedPayload?.document as Record<string, unknown>)?.sourceDocumentId).toBe(sourceDocumentId);
  expect(((postedPayload?.document as Record<string, unknown>)?.lines as Array<Record<string, unknown>>)[0].sourceLineId).toBe(sourceLineId);
  expect((postedPayload?.document as Record<string, unknown>)?.godownId).toBe(godownId);
});

test('open cash sale return posts without a source invoice', async ({ page }) => {
  const itemId = '11111111-1111-4111-8111-111111111111';
  const godownId = '22222222-2222-4222-8222-222222222222';
  let postedPayload: Record<string, unknown> | undefined;
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/master/godown', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', active: true, payload: {} }] })
  }));
  await page.route('**/v1/items/lookup**', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'OPEN-ITEM', code: 'OPEN-ITEM', name: 'OPEN RETURN ITEM', payload: { salePrice: '5.00' }, active: true, aliases: [] }] })
  }));
  await page.route('**/v1/transactions/preview', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '5.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '5.00', taxes: [], totalDiscount: '0.00', total: '5.00' })
  }));
  await page.route('**/v1/documents/open-cash-return', async (route) => {
    postedPayload = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '33333333-3333-4333-8333-333333333333', aggregateId: '44444444-4444-4444-8444-444444444444', kind: 'open-cash-return', action: 'save-and-post', status: 'posted', document: { id: '44444444-4444-4444-8444-444444444444', documentNumber: 'OCR-1', version: 1 } })
    });
  });
  await page.goto('/app/sales?kind=open-cash-return');
  await page.waitForTimeout(500);
  const lookupInput = page.getByLabel('Item lookup query');
  await lookupInput.fill('OPEN');
  await lookupInput.press('Enter');
  await expect(page.getByRole('button', { name: 'OPEN RETURN ITEM' })).toBeVisible({ timeout: 7000 });
  await page.getByRole('button', { name: 'OPEN RETURN ITEM' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByLabel('Batch 1').fill('OPEN-BATCH-1');
  await page.getByLabel('Expiry 1').fill('2027-01-01');
  await page.getByLabel('Unit cost 1').fill('2.25');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.keyboard.press('Control+q');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(postedPayload?.kind).toBe('open-cash-return');
  expect((postedPayload?.document as Record<string, unknown>)?.sourceDocumentId).toBeUndefined();
  expect((postedPayload?.document as Record<string, unknown>)?.godownId).toBe(godownId);
  const selectedDate = await page.getByLabel('Date').inputValue();
  const occurredAt = String((postedPayload?.document as Record<string, unknown>)?.occurredAt ?? '');
  expect(occurredAt.slice(0, 10)).toBe(selectedDate);
  expect(occurredAt).toContain('T12:00:00.000Z');
  const lines = (postedPayload?.document as Record<string, unknown>)?.lines as Array<Record<string, unknown>>;
  expect(lines[0]?.batchNumber).toBe('OPEN-BATCH-1');
  expect(lines[0]?.expiryDate).toBe('2027-01-01');
  expect(lines[0]?.unitCost).toBe('2.25');
});

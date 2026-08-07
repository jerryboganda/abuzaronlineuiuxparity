import { expect, test, type Page } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
});

const itemId = '11111111-1111-4111-8111-111111111111';
const supplierId = '22222222-2222-4222-8222-222222222222';
const godownId = '33333333-3333-4333-8333-333333333333';
const documentId = '44444444-4444-4444-8444-444444444444';
const batchId = '55555555-5555-4555-8555-555555555555';

async function mockCanonicalContext(page: Page, items = true) {
  await page.route('**/v1/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/items/lookup*', (route) => {
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: items ? [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'CANONICAL ITEM', active: true, payload: {}, aliases: [] }] : [] })
    });
  });
  await page.route('**/v1/master/supplier', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: supplierId, legacyId: 'SUP-1', code: 'SUP-1', name: 'SUPPLIER 1', active: true, payload: {} }] })
  }));
  await page.route('**/v1/master/godown', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: godownId, legacyId: 'G-1', code: 'G-1', name: 'GODOWN 1', active: true, payload: {} }] })
  }));
}

function accepted(kind: string, action: string, status: 'draft' | 'posted' | 'void') {
  return {
    accepted: true,
    duplicate: false,
    eventId: batchId,
    aggregateId: documentId,
    kind,
    action,
    status,
    document: { id: documentId, kind, documentNumber: 'PUR-000001', status, version: status === 'draft' ? 1 : 2 }
  };
}

async function waitForPurchaseReady(page: Page) {
  await expect(page.locator('.legacy-transaction-titlebar h1')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByRole('button', { name: 'File', exact: true })).toBeEnabled({ timeout: 10_000 });
  await expect(page.locator('.legacy-menu-bar')).toHaveAttribute('data-hydrated', 'true', { timeout: 10_000 });
}

async function fillReceipt(page: Page) {
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ITEM-1');
  const lookupButton = page.getByRole('button', { name: 'Lookup item 1' });
  await expect(lookupButton).toBeVisible({ timeout: 10_000 });
  await expect(lookupButton).toBeEnabled();
  await lookupButton.click({ force: true });
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByLabel('Godown 1').fill('GODOWN 1');
  await page.getByLabel('Batch 1').fill('PUR-001');
  await page.getByLabel('Expiry 1').fill('2027-08-06');
  await page.getByLabel('Purchase price 1').fill('4.00');
}

test('Edit Purchase Order routes a purchase window to the canonical order editor', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Edit Purchase Order', exact: true }).click();
  await page.waitForURL('**/app/purchase/order');
  await expect(page.locator('.legacy-transaction-titlebar h1')).toHaveText(/Purchase Order/);
});

test('Edit Purchase Order from the order window opens scoped canonical history', async ({ page }) => {
  await mockCanonicalContext(page);
  let requested = false;
  await page.route('**/v1/transactions/purchase-order**', async (route) => {
    requested = true;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ kind: 'purchase-order', rows: [{ document: 'PO-000001', occurredAt: '2026-08-06', party: 'SUPPLIER 1', item: 'CANONICAL ITEM', quantity: '2', amount: '8.00' }] })
    });
  });
  await page.goto('/app/purchase/order');
  await waitForPurchaseReady(page);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Edit Purchase Order', exact: true }).click();
  await expect(page.getByTestId('purchase-list-tab')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('.legacy-purchase-list')).toContainText('PO-000001');
  expect(requested).toBe(true);
});

test('Populate Items resolves purchase quick-search rows through canonical lookup', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('CANON');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Populate Items', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('1 active canonical item populated');
});

test('client-side purchase navigation preserves independent MDI workflow drafts', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await fillReceipt(page);

  await page.getByRole('button', { name: 'Purchase', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Purchase Return', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/purchase\/return/);
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('');

  await page.getByRole('button', { name: 'Window', exact: true }).click();
  await expect(page.getByRole('menuitem', { name: '1 Pack Purchase', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: '2 Purchase Return', exact: true })).toBeVisible();
  await page.getByRole('menuitem', { name: '1 Pack Purchase', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/purchase\/pack/);
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await expect(page.getByLabel('Batch 1')).toHaveValue('PUR-001');
});

test('Populate From Sale Template loads supported template lines into a new draft', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.route('**/v1/master/sale-template', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{
      id: '66666666-6666-4666-8666-666666666666',
      legacyId: 'TPL-1',
      code: 'TPL-1',
      name: 'Common purchase template',
      active: true,
      createdAt: '2026-08-06T00:00:00Z',
      updatedAt: '2026-08-06T00:00:00Z',
      payload: { rows: [{ itemName: 'CANONICAL ITEM', quantity: '2', purchasePrice: '4.00' }] }
    }] })
  }));
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Populate From Sale Template', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Sale Templates' })).toContainText('Common purchase template');
  await page.getByTestId('sale-template-66666666-6666-4666-8666-666666666666').click();
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await expect(page.getByLabel('Order Code:')).toHaveValue('TPL-1');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('loaded 1 line into a new draft');
});

test('supported purchases never fall back or show success after canonical failure', async ({ page }) => {
  await mockCanonicalContext(page);
  let transactionsCalled = false;
  page.on('request', (request) => { if (request.url().includes('/v1/transactions/')) transactionsCalled = true; });
  await page.route('**/v1/documents/pack-purchase', (route) => route.fulfill({ status: 422, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'canonical rejected' }) }));
  const supplierLoad = page.waitForResponse('**/v1/master/supplier');
  await page.goto('/app/purchase/pack');
  await supplierLoad;
  await expect(page.getByLabel('Supplier')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByRole('button', { name: 'Save document' })).toBeEnabled();
  await fillReceipt(page);
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('canonical rejected', { timeout: 7000 });
  await expect(page.locator('.legacy-transaction-footer')).not.toContainText('successfully');
  expect(transactionsCalled).toBe(false);
});

test('receipt sends canonical item, supplier, godown, batch, expiry, cost, and revision state', async ({ page }) => {
  await mockCanonicalContext(page);
  const commands: Array<Record<string, any>> = [];
  await page.route('**/v1/documents/pack-purchase', async (route) => {
    const command = route.request().postDataJSON() as Record<string, any>;
    commands.push(command);
    await route.fulfill({ status: command.action === 'save' ? 201 : 200, contentType: 'application/json', body: JSON.stringify(accepted('pack-purchase', command.action, command.action === 'save' ? 'draft' : 'posted')) });
  });
  await page.goto('/app/purchase/pack');
  await page.waitForTimeout(500);
  await fillReceipt(page);
  await page.getByLabel('Credit days').fill('30');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('saved as draft', { timeout: 7000 });
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Post', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(commands.map((command) => command.action)).toEqual(['save', 'post']);
  expect(commands[1].expectedVersion).toBe(1);
  expect(commands[1].document.supplierId).toBe(supplierId);
  expect(commands[1].document.godownId).toBe(godownId);
  expect(commands[1].document.creditDays).toBe('30');
  expect(commands[1].document.lines[0]).toMatchObject({
    itemId,
    batchNumber: 'PUR-001',
    expiryDate: '2027-08-06',
    unitCost: '4.00'
  });
  expect(commands[1].commandId).toBeTruthy();
  expect(commands[1].idempotencyKey).toBeTruthy();
});

test('free-text item rows fail closed without a canonical item identity', async ({ page }) => {
  await mockCanonicalContext(page, false);
  let commandCalled = false;
  await page.route('**/v1/documents/pack-purchase', (route) => {
    commandCalled = true;
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(accepted('pack-purchase', 'save', 'draft')) });
  });
  await page.goto('/app/purchase/pack');
  await page.waitForTimeout(500);
  await page.getByRole('combobox', { name: 'Item name 1' }).fill('FREE TEXT ONLY');
  await page.getByRole('button', { name: 'Lookup item 1' }).click();
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByLabel('Godown 1').fill('GODOWN 1');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('active canonical item', { timeout: 7000 });
  expect(commandCalled).toBe(false);
});

test('purchase orders use the canonical stock/GL-neutral document response', async ({ page }) => {
  await mockCanonicalContext(page);
  let transactionsCalled = false;
  page.on('request', (request) => { if (request.url().includes('/v1/transactions/')) transactionsCalled = true; });
  await page.route('**/v1/documents/purchase-order', async (route) => {
    const command = route.request().postDataJSON() as Record<string, any>;
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(accepted('purchase-order', command.action, 'draft')) });
  });
  await page.goto('/app/purchase/order');
  await page.waitForTimeout(500);
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ITEM-1');
  await page.getByRole('button', { name: 'Lookup item 1' }).click();
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByLabel('Purchase price 1').fill('4.00');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('stock/GL-neutral', { timeout: 7000 });
  expect(transactionsCalled).toBe(false);
});

test('Apply Item GST persists the canonical item assignment before updating purchase lines', async ({ page }) => {
  await mockCanonicalContext(page);
  let payload: Record<string, any> | undefined;
  await page.route('**/v1/tax-assignments/apply-item-gst', async (route) => {
    payload = route.request().postDataJSON() as Record<string, any>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ rateId: '66666666-6666-4666-8666-666666666666', itemsApplied: 1, effectiveFrom: '2026-08-07' })
    });
  });
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await fillReceipt(page);
  await page.getByLabel('Item GST percent').fill('18');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Apply Item GST %', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('18% assigned to 1 canonical item', { timeout: 7000 });
  expect(payload).toMatchObject({
    rate: '18',
    inclusive: false,
    effectiveFrom: '2026-08-07',
    itemIds: [itemId],
    sourceTable: 'PowerBuilder.FileCommand',
    sourceLegacyId: 'Apply Item GST %'
  });
});

test('File Delete submits a canonical purchase draft delete and clears the editor', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.route('**/v1/transactions/pack-purchase**', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ kind: 'pack-purchase', rows: [{ documentId, document: 'PUR-000001', occurredAt: '2026-08-07', party: 'SUPPLIER 1', item: 'CANONICAL ITEM', quantity: '1', amount: '4.00' }] })
  }));
  await page.route(`**/v1/documents/${documentId}`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      id: documentId,
      kind: 'pack-purchase',
      status: 'draft',
      documentNumber: 'PUR-000001',
      occurredAt: '2026-08-07T12:00:00.000Z',
      supplierId,
      supplier: { id: supplierId, name: 'SUPPLIER 1' },
      godownId,
      lines: [{
        id: '66666666-6666-4666-8666-666666666666', itemId, itemLegacyId: 'ITEM-1', itemCode: 'ITEM-1', itemName: 'CANONICAL ITEM', quantity: '1', unitCost: '4.00', batchNumber: 'PUR-001', expiryDate: '2027-08-06',
        price: { unitPrice: '4.00', grossAmount: '4.00', discountPercent: '0.00', discountAmount: '0.00', netAmount: '4.00' },
        tax: { lines: [], taxableAmount: '4.00', amount: '0.00' }, allocations: [], lineTotal: '4.00', stock: { direction: 'none', quantity: '1' }
      }],
      totals: { subtotal: '4.00', discountAmount: '0.00', miscAmount: '0.00', taxAmount: '0.00', totalAmount: '4.00', paidAmount: '0.00', balanceAmount: '4.00' },
      version: 1
    })
  }));
  let payload: Record<string, any> | undefined;
  await page.route('**/v1/documents/pack-purchase', async (route) => {
    payload = route.request().postDataJSON() as Record<string, any>;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
      accepted: true, duplicate: false, eventId: '77777777-7777-4777-8777-777777777777', aggregateId: documentId,
      kind: 'pack-purchase', action: 'delete', status: 'draft', document: { id: documentId, kind: 'pack-purchase', status: 'draft', version: 2, deletedAt: '2026-08-07T12:01:00.000Z' }
    }) });
  });
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByTestId('purchase-list-tab').click();
  await expect(page.getByRole('button', { name: 'PUR-000001' })).toBeVisible();
  await page.getByRole('button', { name: 'PUR-000001' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('Pack Purchase draft deleted successfully.', { timeout: 7000 });
  expect(payload).toMatchObject({ action: 'delete', documentId, expectedVersion: 1, reason: 'Deleted from purchase workflow' });
});

test('purchase returns require source document and explicit source batch allocation', async ({ page }) => {
  await mockCanonicalContext(page);
  let command: Record<string, any> | undefined;
  await page.route('**/v1/documents/purchase-return', async (route) => {
    command = route.request().postDataJSON() as Record<string, any>;
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(accepted('purchase-return', command.action, 'draft')) });
  });
  await page.goto('/app/purchase/return');
  await page.waitForTimeout(500);
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ITEM-1');
  await page.getByRole('button', { name: 'Lookup item 1' }).click();
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByLabel('Godown 1').fill('GODOWN 1');
  await page.getByLabel('Batch 1').fill('SOURCE-BATCH');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('source purchase document UUID', { timeout: 7000 });
  await page.getByLabel('Source document ID').fill(documentId);
  await page.getByLabel('Source purchase line ID 1').fill('88888888-8888-4888-8888-888888888888');
  await page.getByLabel('Source batch ID 1').fill(batchId);
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('saved as draft', { timeout: 7000 });
  expect(command?.document.sourceDocumentId).toBe(documentId);
  expect(command?.document.lines[0].sourceLineId).toBe('88888888-8888-4888-8888-888888888888');
  expect(command?.document.lines[0].allocations).toEqual([{ batchId, batchNumber: 'SOURCE-BATCH', quantity: '1' }]);
});

test('purchase returns serialize multiple source batch allocations', async ({ page }) => {
  await mockCanonicalContext(page);
  const firstBatchId = '66666666-6666-4666-8666-666666666666';
  const secondBatchId = '77777777-7777-4777-8777-777777777777';
  let command: Record<string, any> | undefined;
  await page.route('**/v1/inventory/availability*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ itemLegacyId: 'ITEM-1', godownId, batches: [
      { batchId: firstBatchId, batchNumber: 'SOURCE-BATCH-1', quantity: '4', expiryDate: '2027-08-06' },
      { batchId: secondBatchId, batchNumber: 'SOURCE-BATCH-2', quantity: '4', expiryDate: '2027-09-06' }
    ] })
  }));
  await page.route('**/v1/documents/purchase-return', async (route) => {
    command = route.request().postDataJSON() as Record<string, any>;
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(accepted('purchase-return', command.action, 'draft')) });
  });
  await page.goto('/app/purchase/return');
  await page.waitForTimeout(500);
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ITEM-1');
  await page.getByRole('button', { name: 'Lookup item 1' }).click();
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByLabel('Godown 1').fill('GODOWN 1');
  await page.getByLabel('Quantity 1', { exact: true }).fill('2');
  await page.getByLabel('Source document ID').fill(documentId);
  await page.getByLabel('Source purchase line ID 1').fill('88888888-8888-4888-8888-888888888888');
  await expect(page.getByLabel('Source batch allocation 1-1')).toBeVisible();
  await page.getByLabel('Source batch allocation 1-1').selectOption(firstBatchId);
  await page.getByLabel('Source allocation quantity 1-1').fill('1');
  await page.getByRole('button', { name: 'Add source allocation 1' }).click();
  await page.getByLabel('Source batch allocation 1-2').selectOption(secondBatchId);
  await page.getByLabel('Source allocation quantity 1-2').fill('1');
  await page.getByRole('button', { name: 'Save document' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('saved as draft', { timeout: 7000 });
  expect(command?.document.lines[0].allocations).toEqual([
    { batchId: firstBatchId, batchNumber: 'SOURCE-BATCH-1', quantity: '1' },
    { batchId: secondBatchId, batchNumber: 'SOURCE-BATCH-2', quantity: '1' }
  ]);
});

test('purchase List loads scoped canonical history and restores a document', async ({ page }) => {
  await mockCanonicalContext(page);
  let requested = false;
  await page.route('**/v1/transactions/pack-purchase**', async (route) => {
    requested = true;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ kind: 'pack-purchase', rows: [{ document: 'PUR-000001', occurredAt: '2026-08-06', party: 'SUPPLIER 1', item: 'CANONICAL ITEM', quantity: '2', amount: '8.00' }] })
    });
  });
  await page.goto('/app/purchase/pack');
  await page.waitForTimeout(700);
  await page.getByTestId('purchase-list-tab').click({ force: true });
  await expect(page.getByTestId('purchase-list-tab')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('.legacy-purchase-list')).toContainText('PUR-000001');
  await expect(page.locator('.legacy-purchase-list')).toContainText('CANONICAL ITEM');
  expect(requested).toBe(true);
  await page.locator('.legacy-purchase-list button').click();
  await expect(page.getByLabel('Invoice No:')).toHaveValue('PUR-000001');
});

test('Item Purchase History filters by the populated canonical item identity', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.route('**/v1/transactions/pack-purchase*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ kind: 'pack-purchase', rows: [] })
  }));
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ITEM-1');
  await page.getByRole('button', { name: 'Lookup item 1' }).click({ force: true });
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  const filteredRequest = page.waitForRequest((request) => {
    if (!request.url().includes('/v1/transactions/pack-purchase')) return false;
    return new URL(request.url()).searchParams.get('filter') === 'ITEM-1';
  });
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Item Purchase History', exact: true }).click();
  await filteredRequest;
  await expect(page.locator('.legacy-transaction-footer')).toContainText('filtered transaction list ready for ITEM-1');
});

test('View Item Info carries the populated canonical item identity to Item master', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.route('**/v1/master/item*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'CANONICAL ITEM', active: true, payload: {}, suppliers: [] }] })
  }));
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByRole('combobox', { name: 'Quick search 1' }).fill('ITEM-1');
  await page.getByRole('button', { name: 'Lookup item 1' }).click({ force: true });
  await expect(page.getByRole('combobox', { name: 'Item name 1' })).toHaveValue('CANONICAL ITEM');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'View Item Info', exact: true }).click();
  await page.waitForURL('**/app/master/item?legacyId=ITEM-1');
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('CANONICAL ITEM');
});

test('Supplier Info carries the active canonical supplier identity to Supplier master', async ({ page }) => {
  await mockCanonicalContext(page);
  await page.route('**/v1/master/supplier*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: supplierId, legacyId: 'SUP-1', code: 'SUP-1', name: 'SUPPLIER 1', active: true, payload: {} }] })
  }));
  await page.goto('/app/purchase/pack');
  await waitForPurchaseReady(page);
  await page.getByLabel('Supplier').fill('SUPPLIER 1');
  await page.getByRole('button', { name: 'File', exact: true }).click({ force: true });
  await page.getByRole('menuitem', { name: 'Supplier Info.', exact: true }).click();
  await page.waitForURL('**/app/master/supplier?legacyId=SUP-1');
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('SUPPLIER 1');
});

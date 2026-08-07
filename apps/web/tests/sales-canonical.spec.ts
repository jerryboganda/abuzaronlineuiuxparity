import { expect, test } from '@playwright/test';

const itemId = '11111111-1111-4111-8111-111111111111';
const godownId = '22222222-2222-4222-8222-222222222222';
const customerId = '33333333-3333-4333-8333-333333333333';

async function mockSession(page: import('@playwright/test').Page) {
  await page.route('**/v1/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      context: { tenantId: 'tenant-1', tenantCode: 'TENANT', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' }
    })
  }));
}

async function waitForSalesReady(page: import('@playwright/test').Page) {
  await expect(page.getByLabel('User:')).toHaveValue('ADMIN', { timeout: 10_000 });
}

test('cash sale has no demo fallback and searches the canonical item lookup', async ({ page }) => {
  await mockSession(page);
  let lookupQuery = '';
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [] }) }));
  await page.route('**/v1/items/lookup*', async (route) => {
    lookupQuery = new URL(route.request().url()).searchParams.get('q') ?? '';
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [] }) });
  });
  await page.goto('/app/sales?kind=cash');
  await waitForSalesReady(page);
  await expect(page.getByText(/No demo items are available/)).toBeVisible();
  await page.getByLabel('Item lookup query').fill('PARA');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByText(/No demo items are available/)).toBeVisible();
  await expect.poll(() => lookupQuery).toBe('PARA');
  await expect(page.getByText('SACHETS 10S')).toHaveCount(0);
});

test('sales history hydrates a canonical return with source identity and saved batch allocations', async ({ page }) => {
  const documentId = '88888888-8888-4888-8888-888888888888';
  const sourceDocumentId = '99999999-9999-4999-8999-999999999999';
  const sourceLineId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
  const returnLineId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
  const batchId = 'cccccccc-cccc-4ccc-8ccc-cccccccccccc';
  const returnDocument = {
    id: documentId,
    kind: 'cash-return',
    status: 'posted',
    documentNumber: 'RET-42',
    tenantId: 'tenant-1',
    branchId: 'branch-1',
    operatorId: 'operator-1',
    occurredAt: '2026-08-07T12:00:00.000Z',
    sourceDocumentId,
    sourceDocumentNumber: 'SALE-11',
    godownId,
    lines: [{
      id: returnLineId,
      lineNumber: 1,
      itemId,
      itemLegacyId: 'ITEM-1',
      itemCode: 'ITEM-1',
      itemName: 'Canonical Return Item',
      quantity: '2',
      price: { priceTier: 1, unitPrice: '8.50', grossAmount: '17.00', discountPercent: '0.00', discountAmount: '0.00', netAmount: '17.00' },
      tax: { taxableAmount: '17.00', amount: '0.00', lines: [] },
      allocations: [{ batchId, batchNumber: 'B-RETURN', expiryDate: '2027-01-01', quantity: '2', unitCost: '2.50' }],
      sourceLineId,
      batchNumber: 'B-RETURN',
      expiryDate: '2027-01-01',
      unitCost: '2.50',
      stock: { direction: 'in', quantity: '2' },
      lineTotal: '17.00'
    }],
    totals: { subtotal: '17.00', discountAmount: '0.00', miscAmount: '0.00', taxAmount: '0.00', totalAmount: '17.00', paidAmount: '17.00', balanceAmount: '0.00' },
    pricing: { tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '17.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '17.00', taxes: [], totalDiscount: '0.00', total: '17.00' },
    version: 3,
    createdAt: '2026-08-07T12:00:00.000Z',
    updatedAt: '2026-08-07T12:00:00.000Z'
  };
  await mockSession(page);
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] }) }));
  await page.route('**/v1/transactions/sale_return*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ kind: 'sale_return', rows: [{ documentId, document: 'RET-42', occurredAt: '2026-08-07', party: 'CASH', item: 'Summary Item', quantity: '2', amount: '17.00' }] }) }));
  await page.route(`**/v1/documents/${documentId}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(returnDocument) }));
  await page.goto('/app/sales?kind=cash-return');
  await waitForSalesReady(page);
  await page.getByTestId('sales-list-tab').click();
  await expect(page.getByRole('button', { name: 'RET-42' })).toBeVisible();
  await page.getByRole('button', { name: 'RET-42' }).click();
  await expect(page.getByLabel('Item name 1')).toHaveValue('Canonical Return Item');
  await expect(page.getByLabel('Quantity 1', { exact: true })).toHaveValue('2');
  await expect(page.getByLabel('Sale price 1')).toHaveValue('8.50');
  await expect(page.getByLabel('Source document ID')).toHaveValue(sourceDocumentId);
  await expect(page.getByLabel('Source document number')).toHaveValue('SALE-11');
  await expect(page.getByLabel('Source sale line ID 1')).toHaveValue(sourceLineId);
  await expect(page.getByLabel('Batch 1')).toHaveValue(batchId);
  await expect(page.getByLabel('Batch quantity 1-1')).toHaveValue('2');
});

test('cash sale sends canonical item, godown, pricing, lifecycle, and idempotency fields', async ({ page }) => {
  await mockSession(page);
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] }) }));
  await page.route('**/v1/items/lookup*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'Canonical Item', payload: { salePrice: '12.50' }, active: true, aliases: [] }] }) }));
  await page.route('**/v1/inventory/availability*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', itemLegacyId: 'ITEM-1', godownId, batches: [{ batchId: '44444444-4444-4444-8444-444444444444', batchNumber: 'B-1', quantity: '5', unitCost: '1.00' }, { batchId: '66666666-6666-4666-8666-666666666666', batchNumber: 'B-2', quantity: '5', unitCost: '1.25' }] }) }));
  await page.route('**/v1/transactions/preview', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '12.50', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '1.00', taxableBase: '13.50', taxes: [], totalDiscount: '0.00', total: '13.50' }) }));
  const payloads: Array<Record<string, unknown>> = [];
  let attempts = 0;
  await page.route('**/v1/documents/cash-sale', async (route) => {
    attempts += 1;
    const payload = route.request().postDataJSON() as Record<string, unknown>;
    payloads.push(payload);
    if (attempts === 1) {
      await route.abort();
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ accepted: true, duplicate: false, eventId: '55555555-5555-4555-8555-555555555555', aggregateId: itemId, kind: 'cash-sale', action: 'save-and-post', status: 'posted', document: { id: itemId, documentNumber: 'CS-1', version: 1 } }) });
  });
  await page.goto('/app/sales?kind=cash');
  await waitForSalesReady(page);
  await page.getByLabel('Item lookup query').fill('Canonical');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'Canonical Item' })).toBeVisible();
  await page.getByRole('button', { name: 'Canonical Item' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByLabel('Quantity 1').fill('2');
  await expect(page.getByLabel('Sale total')).toHaveValue('13.50');
  await page.getByLabel('Cash tendered').fill('20.00');
  await expect(page.getByLabel('Cash back')).toHaveValue('6.50');
  await page.getByLabel('Batch 1').selectOption('44444444-4444-4444-8444-444444444444');
  await page.getByLabel('Batch quantity 1-1').fill('1');
  await page.getByRole('button', { name: 'Add batch allocation 1' }).click();
  await page.getByLabel('Batch 1-2').selectOption('66666666-6666-4666-8666-666666666666');
  await page.getByLabel('Batch quantity 1-2').fill('1');
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect.poll(() => attempts).toBe(1);
  await expect(page.getByRole('button', { name: 'Post sale' })).toBeEnabled();
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(payloads).toHaveLength(2);
  expect(payloads[0].kind).toBe('cash-sale');
  expect(payloads[0].action).toBe('save-and-post');
  expect(typeof payloads[0].commandId).toBe('string');
  expect(typeof payloads[0].idempotencyKey).toBe('string');
  expect(payloads[0].commandId).toBe(payloads[1].commandId);
  expect(payloads[0].idempotencyKey).toBe(payloads[1].idempotencyKey);
  expect((payloads[0].document as Record<string, unknown>).godownId).toBe(godownId);
  expect(((payloads[0].document as Record<string, unknown>).lines as Array<Record<string, unknown>>)[0].itemId).toBe(itemId);
  const allocations = (((payloads[0].document as Record<string, unknown>).lines as Array<Record<string, unknown>>)[0].allocations as Array<Record<string, unknown>>);
  expect(allocations).toHaveLength(2);
  expect(allocations[0].batchId).toBe('44444444-4444-4444-8444-444444444444');
  expect(allocations[0].quantity).toBe('1');
  expect(allocations[1].batchId).toBe('66666666-6666-4666-8666-666666666666');
  expect(allocations[1].quantity).toBe('1');
  const payment = (payloads[0].document as Record<string, unknown>).payment as Record<string, unknown>;
  expect(payment.mode).toBe('cash');
  expect(payment.received).toBe('13.50');
  expect(payment.tendered).toBe('20.00');
  expect(payment.change).toBe('6.50');
});

test('canonical sale rejects an unselected item and then a missing godown without posting', async ({ page }) => {
  await mockSession(page);
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] }) }));
  await page.route('**/v1/items/lookup*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'Canonical Item', payload: {}, active: true, aliases: [] }] }) }));
  await page.route('**/v1/transactions/preview', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '0.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '0.00', taxes: [], totalDiscount: '0.00', total: '0.00' }) }));
  let documentRequests = 0;
  await page.route('**/v1/documents/cash-sale', (route) => { documentRequests += 1; return route.fulfill({ status: 500, body: '' }); });
  await page.goto('/app/sales?kind=cash');
  await waitForSalesReady(page);
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect(page.getByRole('alert')).toContainText('active canonical item');
  await page.getByLabel('Item lookup query').fill('Canonical');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'Canonical Item' })).toBeVisible();
  await page.getByRole('button', { name: 'Canonical Item' }).click();
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect(page.getByRole('alert')).toContainText('active godown');
  expect(documentRequests).toBe(0);
});

test('credit sale requires and submits the selected canonical customer', async ({ page }) => {
  await mockSession(page);
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] }) }));
  await page.route('**/v1/master/customer', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: customerId, kind: 'customer', code: 'C1', name: 'Canonical Customer', payload: {}, active: true }] }) }));
  await page.route('**/v1/items/lookup*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'Canonical Item', payload: { salePrice: '8.00' }, active: true, aliases: [] }] }) }));
  await page.route('**/v1/inventory/availability*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', itemLegacyId: 'ITEM-1', godownId, batches: [{ batchId: '44444444-4444-4444-8444-444444444444', batchNumber: 'B-1', quantity: '2', unitCost: '1.00' }] }) }));
  await page.route('**/v1/transactions/preview', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '8.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '1.00', taxableBase: '9.00', taxes: [], totalDiscount: '0.00', total: '9.00' }) }));
  let payload: Record<string, unknown> | undefined;
  await page.route('**/v1/documents/credit-sale', async (route) => {
    payload = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ accepted: true, duplicate: false, eventId: '55555555-5555-4555-8555-555555555555', aggregateId: itemId, kind: 'credit-sale', action: 'save-and-post', status: 'posted', document: { id: itemId, documentNumber: 'CR-1', version: 1 } }) });
  });
  await page.goto('/app/sales?kind=credit');
  await waitForSalesReady(page);
  await page.getByLabel('Item lookup query').fill('Canonical');
  await page.getByLabel('Item lookup query').press('Enter');
  await expect(page.getByRole('button', { name: 'Canonical Item' })).toBeVisible();
  await page.getByRole('button', { name: 'Canonical Item' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByLabel('Customer').selectOption(customerId);
  await page.getByLabel('Due date').fill('2026-08-31');
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect((payload?.document as Record<string, unknown>).customerId).toBe(customerId);
  expect((payload?.document as Record<string, unknown>).dueDate).toBe('2026-08-31');
  expect((payload?.document as Record<string, unknown>).customer).toBeUndefined();
});

test('client-side navigation from cash to credit loads canonical customers', async ({ page }) => {
  await mockSession(page);
  let customerRequests = 0;
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], scopes: {} })
  }));
  await page.route('**/v1/master/godown', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] })
  }));
  await page.route('**/v1/master/customer', (route) => {
    customerRequests += 1;
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        records: [{ id: customerId, kind: 'customer', code: 'C1', name: 'Canonical Customer', payload: {}, active: true }]
      })
    });
  });
  await page.route('**/v1/items/lookup*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'Canonical Item', payload: { salePrice: '8.00' }, active: true, aliases: [] }] })
  }));
  await page.route('**/v1/inventory/availability*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ batches: [{ batchId: '88888888-8888-4888-8888-888888888888', batchNumber: 'B-1', quantity: '5', unitCost: '1.00' }], tenantId: 'tenant-1', branchId: 'branch-1', itemLegacyId: 'ITEM-1', godownId })
  }));
  await page.route('**/v1/transactions/preview', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '8.00', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '1.00', taxableBase: '9.00', taxes: [], totalDiscount: '0.00', total: '9.00' })
  }));
  let releaseCashResponse = () => {};
  let cashRequests = 0;
  const cashResponseGate = new Promise<void>((resolve) => { releaseCashResponse = resolve; });
  await page.route('**/v1/documents/cash-sale', async (route) => {
    cashRequests += 1;
    await cashResponseGate;
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '44444444-4444-4444-8444-444444444444', aggregateId: '55555555-5555-4555-8555-555555555555', kind: 'cash-sale', action: 'save-and-post', status: 'posted', document: { id: '55555555-5555-4555-8555-555555555555', documentNumber: 'CS-1', version: 1 } })
    });
  });
  let creditPayload: Record<string, unknown> | undefined;
  await page.route('**/v1/documents/credit-sale', (route) => {
    creditPayload = route.request().postDataJSON() as Record<string, unknown>;
    return route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ accepted: true, duplicate: false, eventId: '66666666-6666-4666-8666-666666666666', aggregateId: '77777777-7777-4777-8777-777777777777', kind: 'credit-sale', action: 'save-and-post', status: 'posted', document: { id: '77777777-7777-4777-8777-777777777777', documentNumber: 'CR-1', version: 1 } })
    });
  });

  await page.goto('/app/sales?kind=cash');
  await waitForSalesReady(page);
  expect(customerRequests).toBe(0);
  await page.getByLabel('Item lookup query').fill('Canonical');
  await page.getByLabel('Item lookup query').press('Enter');
  await page.getByRole('button', { name: 'Canonical Item' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  const cashRequest = page.waitForRequest('**/v1/documents/cash-sale');
  await page.getByRole('button', { name: 'Post sale' }).click();
  await cashRequest;
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Post', exact: true }).click();
  expect(cashRequests).toBe(1);
  await page.getByRole('button', { name: 'Sales', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Credit Sale', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/sales\?kind=cash/);
  releaseCashResponse();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully');
  await page.getByRole('button', { name: 'Sales', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Credit Sale', exact: true }).click();

  await expect(page).toHaveURL(/\/app\/sales\?kind=credit/);
  await expect(page.getByLabel('Customer')).toContainText('Canonical Customer');
  expect(customerRequests).toBe(1);
  await expect(page.getByLabel('Item name 1')).toHaveValue('');
  await page.getByLabel('Customer').selectOption(customerId);
  await page.getByLabel('Item lookup query').fill('Canonical');
  await page.getByLabel('Item lookup query').press('Enter');
  await page.getByRole('button', { name: 'Canonical Item' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect.poll(() => creditPayload).toBeDefined();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully');
  expect(creditPayload?.expectedVersion).toBeUndefined();
  expect((creditPayload?.document as Record<string, unknown>).id).toBeUndefined();
  expect((creditPayload?.document as Record<string, unknown>).documentNumber).toBe('');
  await page.getByRole('button', { name: 'Window', exact: true }).click();
  await expect(page.getByRole('menuitem', { name: '1 Cash Sale', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: '2 Credit Sale', exact: true })).toBeVisible();
  await page.getByRole('menuitem', { name: '1 Cash Sale', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/sales\?kind=cash/);
  await expect(page.getByLabel('Item name 1')).toHaveValue('Canonical Item');
  await expect(page.getByLabel('Inv. No:')).toHaveValue('CS-1');
});

test('closed sale return requires source line identity and submits a canonical command', async ({ page }) => {
  await mockSession(page);
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] }) }));
  await page.route('**/v1/items/lookup*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-1', code: 'ITEM-1', name: 'Canonical Item', payload: { salePrice: '12.50' }, active: true, aliases: [] }] }) }));
  await page.route('**/v1/inventory/availability*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ batches: [] }) }));
  await page.route('**/v1/transactions/preview', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '12.50', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '12.50', taxes: [], totalDiscount: '0.00', total: '12.50' }) }));
  let payload: Record<string, unknown> | undefined;
  await page.route('**/v1/documents/cash-return', async (route) => {
    payload = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ accepted: true, duplicate: false, eventId: '55555555-5555-4555-8555-555555555555', aggregateId: itemId, kind: 'cash-return', action: 'save-and-post', status: 'posted', document: { id: itemId, documentNumber: 'RET-1', version: 1 } }) });
  });
  await page.goto('/app/sales?kind=cash-return');
  await waitForSalesReady(page);
  await page.getByLabel('Item lookup query').fill('Canonical');
  await page.getByLabel('Item lookup query').press('Enter');
  await page.getByRole('button', { name: 'Canonical Item' }).click();
  await page.getByLabel('Godown').selectOption(godownId);
  await page.getByLabel('Source document ID').fill('66666666-6666-4666-8666-666666666666');
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect(page.getByRole('alert')).toContainText('source sale line ID');
  expect(payload).toBeUndefined();
  await page.getByLabel('Source sale line ID 1').fill('77777777-7777-4777-8777-777777777777');
  await page.getByRole('button', { name: 'Post sale' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('posted successfully', { timeout: 7000 });
  expect(payload?.kind).toBe('cash-return');
  expect((payload?.document as Record<string, unknown>).sourceDocumentId).toBe('66666666-6666-4666-8666-666666666666');
  expect(((payload?.document as Record<string, unknown>).lines as Array<Record<string, unknown>>)[0].sourceLineId).toBe('77777777-7777-4777-8777-777777777777');
});

test('SalePrice selector reprices selected rows and sends captured tiers to preview', async ({ page }) => {
  await mockSession(page);
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [{ id: godownId, kind: 'godown', code: 'G1', name: 'Godown 1', payload: {}, active: true }] }) }));
  await page.route('**/v1/items/lookup*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-TIERED', code: 'ITEM-TIERED', name: 'Tiered Item', payload: { SalePrice1: '10.00', SalePrice2: '12.50', SalePrice3: '15.00' }, active: true, aliases: [] }] }) }));
  await page.route('**/v1/inventory/availability*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ batches: [] }) }));
  const previewPayloads: Array<Record<string, unknown>> = [];
  await page.route('**/v1/transactions/preview', async (route) => {
    previewPayloads.push(route.request().postDataJSON() as Record<string, unknown>);
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tenantId: 'tenant-1', branchId: 'branch-1', priceLevel: 1, lines: [], subtotal: '12.50', lineDiscountTotal: '0.00', documentPercentDiscount: '0.00', flatDiscount: '0.00', documentDiscountTotal: '0.00', misc: '0.00', taxableBase: '12.50', taxes: [], totalDiscount: '0.00', total: '12.50' }) });
  });
  await page.goto('/app/sales?kind=cash');
  await waitForSalesReady(page);
  await page.getByLabel('Item lookup query').fill('Tiered');
  await page.getByLabel('Item lookup query').press('Enter');
  await page.getByRole('button', { name: 'Tiered Item' }).click();
  await expect(page.getByLabel('Sale price 1')).toHaveValue('10.00');
  await page.getByLabel('Sale price tier').selectOption({ label: 'Sale Price 2' });
  await expect(page.getByLabel('Sale price 1')).toHaveValue('12.50');
  await expect.poll(() => (previewPayloads.at(-1)?.lines as Array<Record<string, unknown>> | undefined)?.[0]?.prices).toEqual(['10.00', '12.50']);
});

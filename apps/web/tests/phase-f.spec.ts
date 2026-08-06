import { expect, test, type Page } from '@playwright/test';

const itemId = '11111111-1111-4111-8111-111111111111';
const supplierId = '22222222-2222-4222-8222-222222222222';

function record(kind: string, code: string, name: string, payload: Record<string, unknown> = {}) {
  return { id: `${kind}-id`, kind, legacyId: code, code, name, payload, active: true };
}

async function mockItemPage(page: Page, suppliers: unknown[] = []) {
  let search = '';
  await page.route('**/v1/master/item/*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
      ...record('item', 'ITEM-1', 'CANONICAL ITEM', { AliasName: 'CANONICAL-ALIAS', SalePrice: '12.50' }),
      id: itemId,
      suppliers
    }) });
  });
  await page.route('**/v1/master/item*', async (route) => {
    if (new URL(route.request().url()).pathname.endsWith(`/${itemId}`)) {
      await route.fallback();
      return;
    }
    search = new URL(route.request().url()).searchParams.get('search') ?? '';
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
      records: [{ ...record('item', 'ITEM-1', 'CANONICAL ITEM', { AliasName: 'CANONICAL-ALIAS', SalePrice: '12.50' }), id: itemId }]
    }) });
  });
  return () => search;
}

test('item master searches canonical records and loads the detail payload', async ({ page }) => {
  const getSearch = await mockItemPage(page, []);
  await page.goto('/app/master/item');
  await page.getByLabel('Master search').fill('CANONICAL');
  await page.getByRole('button', { name: 'Filter / Retrieve' }).click();
  await expect.poll(getSearch).toBe('CANONICAL');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('CANONICAL ITEM');
  await expect(page.getByLabel('Alias Name:')).toHaveValue('CANONICAL-ALIAS');
});

test('item supplier grid saves and reloads through the canonical API', async ({ page }) => {
  const savedSupplier = { id: 'link-1', legacySupplierId: 'SUP-1', supplierId, priority: 1, rate: '4.00', discountPercent: '2.00', quantity: '10', bonus: '1', days: 30 };
  await mockItemPage(page, []);
  let replaceBody: Record<string, unknown> | undefined;
  await page.route(`**/v1/master/item/${itemId}`, async (route) => {
    if (route.request().method() === 'PATCH') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...record('item', 'ITEM-1', 'CANONICAL ITEM'), id: itemId, suppliers: [] }) });
      return;
    }
    await route.continue();
  });
  await page.route(`**/v1/master/item/${itemId}/suppliers`, async (route) => {
    if (route.request().method() === 'PUT') {
      replaceBody = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ suppliers: [savedSupplier] }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ suppliers: [savedSupplier] }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'Add supplier' }).click();
  await page.getByLabel('Supplier legacy id 1').fill('SUP-1');
  await page.getByLabel('Supplier priority 1').fill('1');
  await page.getByLabel('Supplier rate 1').fill('4.00');
  await page.locator('form.legacy-master-form').getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('saved', { timeout: 7000 });
  expect(replaceBody?.suppliers).toEqual([expect.objectContaining({ legacySupplierId: 'SUP-1', rate: '4.00', priority: 1 })]);
  await page.getByRole('button', { name: 'Refresh records' }).click();
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await expect(page.getByLabel('Supplier legacy id 1')).toHaveValue('SUP-1');
});

test('customer and supplier forms use canonical CRUD endpoints', async ({ page }) => {
  page.on('request', (request) => { if (request.url().includes('/v1/master/')) console.log(`CRUD REQUEST ${request.method()} ${request.url()}`); });
  page.on('console', (message) => console.log(`BROWSER ${message.text()}`));
  for (const kind of ['customer', 'supplier']) {
    const code = kind === 'customer' ? 'C-NEW' : 'S-NEW';
    await page.route(`**/v1/master/${kind}*`, async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [] }) });
        return;
      }
      await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(record(kind, code, `${kind} record`)) });
    });
    await page.goto(`/app/master/${kind}`);
    await page.getByRole('textbox', { name: 'Code:', exact: true }).fill(code);
    await page.getByRole('textbox', { name: 'Name:', exact: true }).fill(`${kind} record`);
    const saveButton = page.locator('form.legacy-master-form').getByRole('button', { name: 'Save' });
    await expect(saveButton).toBeEnabled();
    await saveButton.click({ force: true });
    await expect(page.locator('.legacy-transaction-footer')).toContainText('created', { timeout: 7000 });
  }
});

test('master API errors are shown without a generic success fallback', async ({ page }) => {
  await page.route('**/v1/master/manufacturer', (route) => route.fulfill({
    status: 403,
    contentType: 'application/problem+json',
    body: JSON.stringify({ status: 403, detail: 'Branch context is not allowed for this tenant.' })
  }));
  await page.goto('/app/master/manufacturer');
  await expect(page.getByRole('alert')).toContainText('Branch context is not allowed');
  await expect(page.locator('.legacy-transaction-footer')).not.toContainText('saved successfully');
});

test('empty canonical masters have no demo rows and unsupported kinds are truthful read-only', async ({ page }) => {
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [] }) }));
  await page.goto('/app/master/godown');
  await expect(page.getByText('No records in the current tenant scope.')).toBeVisible();
  await expect(page.getByText(/SACHETS|DEMO|DEFAULT GENERIC/i)).toHaveCount(0);

  await page.goto('/app/master/price-policy');
  await expect(page.getByText(/read-only; no canonical API is available/i)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save' }).last()).toBeDisabled();
});

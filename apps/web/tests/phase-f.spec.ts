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

test('item master captured File commands drive the canonical editor state', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await page.route('**/v1/session', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, context: { tenantId: 'tenant-1', branchId: 'branch-1', counterId: 'counter-1', operatorId: 'operator-1', username: 'ADMIN', displayName: 'ADMIN' } })
  }));
  await page.route('**/v1/master/item*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [{ ...record('item', 'ITEM-1', 'CANONICAL ITEM'), id: itemId }] })
  }));
  await page.goto('/app/master/item');
  await expect(page.locator('.legacy-menu-bar')).toHaveAttribute('data-hydrated', 'true');

  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'New', exact: true }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('New record ready.');

  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'List', exact: true }).click();
  await expect(page.locator('main.legacy-master-list-tab')).toBeVisible();
  await expect(page.getByRole('button', { name: 'ITEM-1' })).toBeVisible();
});

test('item master searches canonical records and loads the detail payload', async ({ page }) => {
  const getSearch = await mockItemPage(page, []);
  const initialList = page.waitForResponse((response) => response.url().endsWith('/v1/master/item') && response.request().method() === 'GET');
  await page.goto('/app/master/item');
  await initialList;
  await page.getByLabel('Master search').fill('CANONICAL');
  await page.getByRole('button', { name: 'Filter / Retrieve' }).click({ force: true });
  await expect.poll(getSearch).toBe('CANONICAL');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('CANONICAL ITEM');
  await expect(page.getByLabel('Alias Name:')).toHaveValue('CANONICAL-ALIAS');
});

test('item master preselects the canonical item requested by legacy identity', async ({ page }) => {
  await mockItemPage(page, []);
  await page.goto('/app/master/item?legacyId=ITEM-1');
  await expect(page.getByRole('textbox', { name: 'Code/No.:', exact: true })).toHaveValue('ITEM-1');
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('CANONICAL ITEM');
});

test('supplier master preselects the canonical supplier requested by legacy identity', async ({ page }) => {
  await page.route('**/v1/master/supplier*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ records: [record('supplier', 'SUP-1', 'SUPPLIER 1')] })
  }));
  await page.goto('/app/master/supplier?legacyId=SUP-1');
  await expect(page.getByRole('textbox', { name: 'Code:', exact: true })).toHaveValue('SUP-1');
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('SUPPLIER 1');
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
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...record('item', 'ITEM-1', 'CANONICAL ITEM'), id: itemId, suppliers: [savedSupplier] }) });
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

test('item master captured alternate-alias command replaces only alternate aliases', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedAliases: string[] | undefined;
  await page.route(`**/v1/master/item/${itemId}/aliases`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedAliases = (route.request().postDataJSON() as { aliases: string[] }).aliases;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ aliases: savedAliases }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ aliases: ['BOX-1'] }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Set Alternate Item Alias Names', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Alternate Item Alias Names' })).toBeVisible();
  await expect(page.locator('input[aria-label="Alternate alias 1"]')).toHaveValue('BOX-1');
  await page.getByRole('button', { name: 'Add alias' }).click();
  await page.locator('input[aria-label="Alternate alias 2"]').fill('BOX-2');
  await page.getByRole('dialog', { name: 'Alternate Item Alias Names' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('2 alternate item aliases saved');
  expect(savedAliases).toEqual(['BOX-1', 'BOX-2']);
});

test('item master captured image command loads and replaces ItemImage rows', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedImages: Array<Record<string, unknown>> | undefined;
  await page.route(`**/v1/master/item/${itemId}/images`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedImages = (route.request().postDataJSON() as { images: Array<Record<string, unknown>> }).images;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ images: savedImages }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ images: [{ id: 'image-1', rowId: 1, imageDescription: 'Existing front', imageData: 'aW1hZ2UtYnl0ZXM=', imageType: 'image/png' }] }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Set Item Image(s)', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Set Item Images' })).toBeVisible();
  await expect(page.locator('input[aria-label="Image description 1"]')).toHaveValue('Existing front');
  await page.getByLabel('Add item image file').setInputFiles('..\\..\\tmp\\legacy-item-command3.png');
  await expect(page.locator('input[aria-label="Image description 2"]')).toHaveValue('legacy-item-command3.png');
  await page.getByRole('dialog', { name: 'Set Item Images' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('2 item images saved');
  expect(savedImages).toHaveLength(2);
  expect(savedImages?.[1]).toEqual(expect.objectContaining({ imageDescription: 'legacy-item-command3.png', imageType: 'image/png' }));
  expect(String(savedImages?.[1]?.imageData ?? '')).not.toHaveLength(0);
});

test('item master captured notes command round-trips UTF-8 ItemNotes bytes', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedNotes: string | undefined;
  await page.route(`**/v1/master/item/${itemId}/notes`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedNotes = (route.request().postDataJSON() as { notesData: string }).notesData;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ notesData: savedNotes }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ notesData: 'TGVnYWN5IG5vdGU=' }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Set Item Notes', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Set Item Notes' })).toBeVisible();
  await expect(page.getByLabel('Item notes text')).toHaveValue('Legacy note');
  await page.getByLabel('Item notes text').fill('Updated note');
  await page.getByRole('dialog', { name: 'Set Item Notes' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('Item notes saved');
  expect(savedNotes).toBe('VXBkYXRlZCBub3Rl');
});

test('item master captured association command replaces ItemAssociation pairs', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedAssociations: string[] | undefined;
  await page.route(`**/v1/master/item/${itemId}/associations`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedAssociations = (route.request().postDataJSON() as { legacyItemIds: string[] }).legacyItemIds;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ associations: savedAssociations.map((legacyItemId) => ({ legacyItemId, code: legacyItemId, name: `Item ${legacyItemId}` })) }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ associations: [{ id: 'association-1', legacyItemId: 'ITEM-2', code: 'ITEM-2', name: 'Associated Item 2' }] }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Set Item Associations', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Set Item Associations' })).toBeVisible();
  await expect(page.getByLabel('Associated item legacy id 1')).toHaveValue('ITEM-2');
  await page.getByRole('button', { name: 'Add association' }).click();
  await page.getByLabel('Associated item legacy id 2').fill('ITEM-3');
  await page.getByRole('dialog', { name: 'Set Item Associations' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('2 item associations saved');
  expect(savedAssociations).toEqual(['ITEM-2', 'ITEM-3']);
});

test('item master captured author command replaces ItemAuthor rows', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedAuthors: Array<Record<string, unknown>> | undefined;
  await page.route(`**/v1/master/item/${itemId}/authors`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedAuthors = (route.request().postDataJSON() as { authors: Array<Record<string, unknown>> }).authors;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authors: savedAuthors }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authors: [{ id: 'author-link-1', authorCode: 12, priority: 2, rowId: 1 }] }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Set Item Author(s)', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Set Item Authors' })).toBeVisible();
  await expect(page.getByLabel('Item author code 1')).toHaveValue('12');
  await page.getByRole('button', { name: 'Add author' }).click();
  await page.getByLabel('Item author code 2').fill('13');
  await page.getByLabel('Item author priority 2').fill('1');
  await page.getByRole('dialog', { name: 'Set Item Authors' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('2 item authors saved');
  expect(savedAuthors).toEqual([
    expect.objectContaining({ authorCode: 12, priority: 2, rowId: 1 }),
    expect.objectContaining({ authorCode: 13, priority: 1, rowId: 2 })
  ]);
});

test('item master captured model command replaces ItemInModel membership', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedModels: number[] | undefined;
  await page.route(`**/v1/master/item/${itemId}/models`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedModels = (route.request().postDataJSON() as { modelCodes: number[] }).modelCodes;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ models: savedModels.map((modelCode) => ({ modelCode })) }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ models: [{ id: 'model-link-1', modelCode: 12 }] }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Select Models', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Select Models' })).toBeVisible();
  await expect(page.getByLabel('Item model code 1')).toHaveValue('12');
  await page.getByRole('button', { name: 'Add model' }).click();
  await page.getByLabel('Item model code 2').fill('-4');
  await page.getByRole('dialog', { name: 'Select Models' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('2 item models saved');
  expect(savedModels).toEqual([12, -4]);
});

test('item master captured price-policy command replaces PricePolicyDetail tiers', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let savedPolicy: { policyCode: string; tiers: Array<Record<string, unknown>> } | undefined;
  await page.route(`**/v1/master/item/${itemId}/price-policy`, async (route) => {
    if (route.request().method() === 'PUT') {
      savedPolicy = route.request().postDataJSON() as { policyCode: string; tiers: Array<Record<string, unknown>> };
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          policy: { policyCode: 'PP-1', name: 'Retail Policy', legacyItemId: 'ITEM-1' },
          tiers: savedPolicy.tiers
        })
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        policy: { policyCode: 'PP-1', name: 'Retail Policy', legacyItemId: 'ITEM-1' },
        tiers: [{ id: 'tier-1', quantityLimit: 1, price: '12.5000', expiryDate: '2026-12-31', flatDiscount: '0.00', discountPercent: '2.00' }]
      })
    });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Set Item Price Policy', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Set Item Price Policy' });
  await expect(dialog).toBeVisible();
  await expect(page.getByLabel('Price policy price 1')).toHaveValue('12.5000');
  await page.getByRole('button', { name: 'Add tier' }).click();
  await page.getByLabel('Price policy quantity limit 2').fill('10');
  await page.getByLabel('Price policy price 2').fill('11.7500');
  await page.getByLabel('Price policy expiry 2').fill('2027-12-31');
  await page.getByRole('dialog', { name: 'Set Item Price Policy' }).getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('.legacy-transaction-footer')).toContainText('2 price-policy tiers saved');
  expect(savedPolicy?.policyCode).toBe('PP-1');
  expect(savedPolicy?.tiers).toEqual([
    expect.objectContaining({ id: 'tier-1', quantityLimit: 1, price: '12.5000' }),
    expect.objectContaining({ quantityLimit: 10, price: '11.7500', expiryDate: '2027-12-31' })
  ]);
});

test('item master captured registration command populates a source-shaped request snapshot', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  let populateBody: Record<string, unknown> | undefined;
  const request = {
    id: 'registration-request-1',
    requestCode: 42,
    legacyItemId: 'ITEM-1',
    requestedAt: '2026-08-07T12:00:00Z',
    serverName: '',
    machineName: '',
    sent: 'N',
    sentOn: '',
    payload: { ICode: 'ITEM-1', Name: 'CANONICAL ITEM', SalePrice: '12.50', ItemRegReqCode: 42 }
  };
  await page.route(`**/v1/master/item/${itemId}/registration-request`, async (route) => {
    if (route.request().method() === 'POST') {
      populateBody = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ request }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ request: null }) });
  });
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Populate Item Registration Request', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Populate Item Registration Request' });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('No registration request has been populated');
  await dialog.getByRole('button', { name: 'Populate' }).click();
  await expect(dialog).toContainText('Request code');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('request 42 populated');
  expect(populateBody).toEqual({});
});

test('item master captured Populate Item command hydrates an active canonical lookup result', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  await page.route('**/v1/items/lookup*', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [{ id: itemId, legacyId: 'ITEM-2', code: 'ITEM-2', name: 'POPULATED ITEM', payload: { AliasName: 'POP-ALIAS' }, active: true, aliases: ['POP-ALIAS'] }] })
  }));
  await page.route(`**/v1/master/item/${itemId}`, async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ ...record('item', 'ITEM-2', 'POPULATED ITEM', { AliasName: 'POP-ALIAS' }), id: itemId, legacyId: 'ITEM-2', suppliers: [] })
  }));
  await page.goto('/app/master/item');
  await expect(page.locator('.legacy-menu-bar')).toHaveAttribute('data-hydrated', 'true');
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Populate Item', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Populate Item' });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel('Populate item lookup').fill('ITEM-2');
  await dialog.getByRole('button', { name: 'Search' }).click();
  await expect(dialog.locator('.legacy-item-populate-result')).toContainText('POPULATED ITEM');
  await dialog.locator('.legacy-item-populate-result').click();
  await expect(page.getByRole('textbox', { name: 'Code/No.:', exact: true })).toHaveValue('ITEM-2');
  await expect(page.getByRole('textbox', { name: 'Name:', exact: true })).toHaveValue('POPULATED ITEM');
  await expect(page.locator('.legacy-transaction-footer')).toContainText('populated from the active canonical item lookup');
});

test('item master captured unposted transaction report shows scoped draft lines', async ({ page }) => {
  await page.route('**/v1/access', async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await mockItemPage(page, []);
  await page.route(`**/v1/master/item/${itemId}/unposted-transactions`, async (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      itemId,
      truncated: false,
      transactions: [{
        id: 'draft-line-1',
        kind: 'quotation',
        documentNumber: 'Q-17',
        status: 'draft',
        occurredAt: '2026-08-07T12:00:00Z',
        lineNumber: 1,
        itemLegacyId: 'ITEM-1',
        itemName: 'CANONICAL ITEM',
        quantity: '2.0000',
        unitPrice: '12.5000',
        lineTotal: '25.0000'
      }]
    })
  }));
  await page.goto('/app/master/item');
  await page.getByRole('button', { name: 'ITEM-1' }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Show Un-Posted Transaction Report', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Show Un-Posted Transaction Report' });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('Q-17');
  await expect(dialog).toContainText('quotation');
  await expect(dialog).toContainText('12.5000');
  await dialog.getByRole('button', { name: 'Close' }).click();
  await expect(dialog).toBeHidden();
});

test('customer and supplier forms use canonical CRUD endpoints', async ({ page }) => {
  for (const kind of ['customer', 'supplier']) {
    const code = kind === 'customer' ? 'C-NEW' : 'S-NEW';
    await page.route(`**/v1/master/${kind}`, async (route) => {
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
    const response = await page.evaluate(async ({ masterKind, masterCode }) => {
      const result = await fetch(`/v1/master/${masterKind}`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ code: masterCode, name: `${masterKind} record`, payload: { source: 'phase-f-ui' }, active: true })
      });
      return { status: result.status, body: await result.json() };
    }, { masterKind: kind, masterCode: code });
    expect(response.status).toBe(201);
    expect(response.body.code).toBe(code);
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

test('empty canonical masters have no demo rows and auxiliary masters are writable', async ({ page }) => {
  await page.route('**/v1/master/godown', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [] }) }));
  await page.goto('/app/master/godown');
  await expect(page.getByText('No records in the current tenant scope.')).toBeVisible();
  await expect(page.getByText(/SACHETS|DEMO|DEFAULT GENERIC/i)).toHaveCount(0);

  let savedBody: Record<string, unknown> | undefined;
  await page.route('**/v1/master/price-policy', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: [] }) });
      return;
    }
    if (route.request().method() === 'DELETE') {
      await route.fulfill({ status: 204 });
      return;
    }
    savedBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({
      ...record('price-policy', 'PP-1', 'Retail Policy', { PricePolicyCode: 'PP-1', ICode: 'ITEM-1' }),
      id: 'price-policy-created'
    }) });
  });
  await page.route('**/v1/master/price-policy/*', async (route) => {
    if (route.request().method() === 'DELETE') {
      await route.fulfill({ status: 204 });
      return;
    }
    await route.fallback();
  });
  const pricePolicyList = page.waitForResponse((response) => response.url().includes('/v1/master/price-policy') && response.request().method() === 'GET');
  await page.goto('/app/master/price-policy');
  await pricePolicyList;
  await expect(page.getByLabel('Item Code:')).toBeVisible();
  await page.getByRole('textbox', { name: 'Code:', exact: true }).fill('PP-1');
  await page.getByRole('textbox', { name: 'Name:', exact: true }).fill('Retail Policy');
  await page.getByRole('textbox', { name: 'Item Code:', exact: true }).fill('ITEM-1');
  const saveRequest = page.waitForRequest((request) => request.url().includes('/v1/master/price-policy') && request.method() === 'POST');
  await page.locator('form.legacy-master-form').getByRole('button', { name: 'Save' }).click();
  await saveRequest;
  await expect(page.locator('.legacy-transaction-footer')).toContainText('created in the current tenant scope');
  expect(savedBody).toEqual(expect.objectContaining({ code: 'PP-1', name: 'Retail Policy', active: true }));
  expect(savedBody?.payload).toEqual(expect.objectContaining({ PricePolicyCode: 'PP-1', ICode: 'ITEM-1' }));
  page.once('dialog', (dialog) => dialog.accept());
  const deleteRequest = page.waitForRequest((request) => request.url().includes('/v1/master/price-policy/') && request.method() === 'DELETE');
  await page.locator('form.legacy-master-form').getByRole('button', { name: 'Delete' }).click();
  await deleteRequest;
  await expect(page.locator('.legacy-transaction-footer')).toContainText('deleted from the current tenant scope');

  await page.goto('/app/master/unsupported-legacy-master');
  await expect(page.getByText(/read-only; no canonical API is available/i)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save' }).last()).toBeDisabled();
});

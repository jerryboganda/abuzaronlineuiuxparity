import { expect, test } from '@playwright/test';
import { legacyMenuCatalog } from '../src/lib/legacy-menu-catalog';
import { buildLegacyMenus, cleanLabel, hrefFor, normalizeCatalogPath, type MenuAction } from '../src/lib/legacy-menu';

function catalogPath(rawPath: string): string[] {
  return normalizeCatalogPath(rawPath).split(' > ').map(cleanLabel).filter(Boolean);
}

function leaves(actions: MenuAction[]): MenuAction[] {
  return actions.flatMap((action) => action.children ? leaves(action.children) : [action]);
}

test('key web source files have no known double-encoded UTF-8 markers', async ({ request }) => {
  const sourcePaths = [
    'lib/LegacyWorkflowSurface.svelte',
    'lib/legacy-menu.ts',
    'lib/legacy-title.ts',
    'routes/+layout.svelte',
    'routes/app/legacy/+page.svelte',
    'routes/app/sales/+page.svelte'
  ];
  const mojibake = /[ÂÃâ]/;
  const offenders: string[] = [];
  for (const sourcePath of sourcePaths) {
    const response = await request.get(`/src/${sourcePath}`);
    expect(response.ok(), sourcePath).toBeTruthy();
    if (mojibake.test(await response.text())) offenders.push(sourcePath);
  }

  expect(offenders).toEqual([]);
});

test('catalog leaves normalize labels and resolve to concrete routes', () => {
  const catalogLeaves = legacyMenuCatalog
    .filter((item) => !item.hasSubmenu && item.commandId !== 0)
    .map((item) => ({ item, path: catalogPath(item.path) }));

  expect(catalogLeaves).toHaveLength(221);
  for (const { item, path } of catalogLeaves) {
    const href = hrefFor(path, item.commandId);
    expect(href, item.path).toBeDefined();
    if (path[0] !== 'Help') expect(href, item.path).not.toContain('/app/module/');
  }

  for (const action of leaves(buildLegacyMenus().flatMap((menu) => menu.actions))) {
    expect(action.label).toBe(action.label.trim());
  }
});

test('Open Sale Return leaves use distinct workflow kinds', () => {
  expect(hrefFor(['Sales', 'Sale Return', 'Cash Sale Return'], 10033)).toContain('kind=cash-return');
  expect(hrefFor(['Sales', 'Sale Return', 'Credit Sale Return'], 10034)).toContain('kind=credit-return');
  expect(hrefFor(['Sales', 'Open Sale Return', 'Cash Sale Return'], 10035)).toContain('kind=open-cash-return');
  expect(hrefFor(['Sales', 'Open Sale Return', 'Credit Sale Return'], 10036)).toContain('kind=open-credit-return');
});

test('legacy title uses the authenticated username and a live session clock', async ({ page }) => {
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
        username: 'CENTRAL_ADMIN',
        displayName: 'Local Administrator'
      }
    })
  }));
  await page.route('**/v1/operators', (route) => route.abort());

  await page.goto('/app/legacy');
  const title = page.locator('.legacy-window-titlebar h1');
  await expect(title).toContainText(/WASEELA\s+ABUZAR V3 01\.01\.2025 : CENTRAL_ADMIN : \d{2}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}/);
  const firstTitle = await title.textContent();
  await page.waitForTimeout(2200);
  await expect.poll(() => title.textContent()).not.toBe(firstTitle);
});

test('legacy title safely falls back when older sessions omit username', async ({ page }) => {
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
        displayName: 'Local Administrator'
      }
    })
  }));
  await page.route('**/v1/operators', (route) => route.abort());

  await page.goto('/app/legacy');
  await expect(page.locator('.legacy-window-titlebar h1')).toContainText(/: ADMIN : \d{2}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}/);
});

import { expect, test } from '@playwright/test';

function session(roles: string[] = ['operator'], permissions: string[] = ['manage.groups']) {
  return {
    authenticated: true,
    context: {
      tenantId: 'tenant-1',
      tenantCode: 'TENANT',
      branchId: 'branch-a',
      counterId: 'counter-a',
      operatorId: 'operator-1',
      username: 'OPERATOR',
      displayName: 'Operator',
      roles,
      permissions
    }
  };
}

function access(overrides: Record<string, unknown> = {}) {
  return {
    tenantAdmin: false,
    permissions: ['manage.groups'],
    legacyRights: [{ rightCode: 'LEGACY-SALE', permission: 'sales.read', allowed: true, mapping: 'explicit' }],
    scopes: { branch: { 'branch-a': true } },
    scopeRows: [],
    exceptions: [],
    ...overrides
  };
}

test('Groups rights matrix edits imported denies without losing normalized permissions', async ({ page }) => {
  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session()) }));
  await page.route('**/v1/access', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(access()) }));
  let rightsBody: Record<string, unknown> | undefined;
  await page.route('**/v1/roles/role-1/rights', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          roleId: 'role-1',
          permissions: ['sales.read'],
          legacyRights: [{ rightCode: 'LEGACY-SALE', permission: 'sales.read', allowed: true, mapping: 'explicit' }],
          scopes: [{ scopeKind: 'branch', scopeKey: 'branch-a', scopeLabel: 'Branch A', allowed: true, legacyTable: 'GroupAllowedBranch' }]
        })
      });
      return;
    }
    rightsBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ roleId: 'role-1', permissions: ['sales.read'], legacyRights: [], scopes: [] })
    });
  });
  await page.route('**/v1/roles', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ roles: [{ id: 'role-1', code: 'SALES OFFICER', name: 'Sales Officer', memberCount: 1, permissions: ['sales.read'] }] })
  }));
  await page.route('**/v1/roles/role-1', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ id: 'role-1', code: 'SALES OFFICER', name: 'Sales Officer', memberCount: 1, permissions: ['sales.read'] })
  }));

  await page.goto('/app/manage/groups');
  await page.getByRole('button', { name: 'SALES OFFICER', exact: true }).click();
  await expect(page.getByLabel('Imported legacy rights')).toBeVisible();
  await page.getByLabel(/LEGACY-SALE/).uncheck();
  await page.getByLabel(/branch: Branch A/).uncheck();
  await page.getByRole('button', { name: 'Save group' }).click();
  await expect(page.getByRole('status')).toContainText('saved successfully');
  expect(rightsBody?.permissions).toEqual(['sales.read']);
  expect(rightsBody?.legacyRights).toEqual([{ rightCode: 'LEGACY-SALE', allowed: false }]);
  expect(rightsBody?.scopes).toEqual([{ scopeKind: 'branch', scopeKey: 'branch-a', allowed: false }]);
});

test('revoked contextual command is disabled while tenant admin retains it', async ({ page }) => {
  let admin = false;
  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(admin ? session(['tenant_admin'], []) : session(['operator'], ['sales.read'])) }));
  await page.route('**/v1/access', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(admin ? access({ tenantAdmin: true, permissions: [] }) : access({ permissions: ['sales.read'] })) }));
  await page.goto('/app/sales?kind=cash');
  await page.waitForTimeout(1000);
  const fileMenu = page.getByRole('button', { name: 'File', exact: true });
  await expect(fileMenu).toBeVisible();
  await fileMenu.evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.getByRole('menuitem', { name: 'Save And Post', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Save And Post', exact: true })).toBeDisabled();

  admin = true;
  await page.reload();
  await page.waitForTimeout(1000);
  const adminFileMenu = page.getByRole('button', { name: 'File', exact: true });
  await expect(adminFileMenu).toBeVisible();
  await adminFileMenu.click({ force: true });
  await expect(page.getByRole('menuitem', { name: 'Save And Post', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Save And Post', exact: true })).toBeEnabled();
});

test('report menu applies the imported report scope filter', async ({ page }) => {
  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session(['operator'], ['reports.read'])) }));
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(access({ permissions: ['reports.read'], scopes: { report: { 'sale-detail': true, 'sale-summary': false } } }))
  }));
  await page.goto('/app/legacy');
  await page.waitForTimeout(1000);
  const reportsMenu = page.getByRole('button', { name: 'Reports', exact: true });
  await expect(reportsMenu).toBeVisible();
  await reportsMenu.evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.getByRole('menuitem', { name: 'Daily Reports', exact: true })).toBeVisible();
  await page.getByRole('menuitem', { name: 'Daily Reports', exact: true }).evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.getByRole('menuitem', { name: 'Sale', exact: true })).toBeVisible();
  await page.getByRole('menuitem', { name: 'Sale', exact: true }).evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.locator('button[data-legacy-path="Reports > Daily Reports > Sale > Sale detail"]')).toBeEnabled();
  await expect(page.locator('button[data-legacy-path="Reports > Daily Reports > Sale > Sale summary"]')).toBeDisabled();
});

import { expect, test } from '@playwright/test';
import { execFileSync } from 'node:child_process';

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

test('Groups captured File commands drive the canonical role editor', async ({ page }) => {
  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session()) }));
  await page.route('**/v1/access', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(access()) }));
  await page.route('**/v1/roles', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ roles: [{ id: 'role-1', code: 'OPERATOR', name: 'Operator', memberCount: 1, permissions: ['sales.read'] }] })
  }));
  await page.goto('/app/manage/groups');
  await expect(page.locator('.legacy-menu-bar')).toHaveAttribute('data-hydrated', 'true');

  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'New', exact: true }).click();
  await expect(page.locator('section[aria-label="Groups"] footer [role="status"]')).toHaveText('New group ready.');

  await page.getByRole('button', { name: 'OPERATOR', exact: true }).click();
  await page.getByRole('button', { name: 'File', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Detail', exact: true }).click();
  await expect(page.locator('section[aria-label="Groups"] footer [role="status"]')).toHaveText('Group detail is ready.');
});

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
  await expect(page.locator('section[aria-label="Groups"] footer [role="status"]')).toContainText('saved successfully');
  expect(rightsBody?.permissions).toEqual(['sales.read']);
  expect(rightsBody?.legacyRights).toEqual([{ rightCode: 'LEGACY-SALE', allowed: false }]);
  expect(rightsBody?.scopes).toEqual([{ scopeKind: 'branch', scopeKey: 'branch-a', allowed: false }]);
});

test('Group Allowed Price Setting edits retained composite GroupAllowedPrice scopes', async ({ page }) => {
  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session(['operator'], ['manage.groups'])) }));
  await page.route('**/v1/access', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(access()) }));
  await page.route('**/v1/roles', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ roles: [{ id: 'role-1', code: 'SALES OFFICER', name: 'Sales Officer', memberCount: 1, permissions: ['sales.read'] }] })
  }));
  let rightsBody: Record<string, unknown> | undefined;
  await page.route('**/v1/roles/role-1/rights', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          roleId: 'role-1',
          permissions: ['sales.read'],
          legacyRights: [],
          scopes: [
            { scopeKind: 'price', scopeKey: '2:1:1', allowed: true, legacyTable: 'GroupAllowedPrice' },
            { scopeKind: 'price', scopeKey: '2:1:2', allowed: false, legacyTable: 'GroupAllowedPrice' },
            { scopeKind: 'godown', scopeKey: '2:10', allowed: true, legacyTable: 'GroupAllowedGodown' }
          ]
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

  await page.goto('/app/manage/group-allowed-price-setting');
  await page.getByLabel('Group:').selectOption('role-1');
  await expect(page.getByText('GroupCode 2 - Module 1 - PriceTypeCode 1', { exact: false })).toBeVisible();
  await page.getByLabel(/GroupCode 2 - Module 1 - PriceTypeCode 1/).uncheck();
  await page.getByRole('button', { name: 'Save group price access' }).click();
  await expect(page.locator('section[aria-label="Group Allowed Price Setting"] footer [role="status"]')).toHaveText('Sales Officer price access saved.');
  expect(rightsBody?.scopes).toEqual([
    { scopeKind: 'price', scopeKey: '2:1:1', allowed: false },
    { scopeKind: 'price', scopeKey: '2:1:2', allowed: false }
  ]);
});

test('imported group scope setting leaves isolate their approved scope kinds', async ({ page }) => {
  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session(['operator'], ['manage.groups', 'maintenance.write'])) }));
  await page.route('**/v1/access', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(access({ permissions: ['manage.groups', 'maintenance.write'] })) }));
  await page.route('**/v1/roles', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ roles: [{ id: 'role-1', code: 'SALES OFFICER', name: 'Sales Officer', memberCount: 1, permissions: ['sales.read'] }] })
  }));
  const importedScopes = [
    { scopeKind: 'header', scopeKey: '2:10:1:1', allowed: true, legacyTable: 'GroupAllowedHeader' },
    { scopeKind: 'godown', scopeKey: '2:10:1:1', allowed: true, legacyTable: 'GroupAllowedGodown' },
    { scopeKind: 'cash_account', scopeKey: '2:1:100:7', allowed: true, legacyTable: 'GroupCashAccount' },
    { scopeKind: 'service_category', scopeKey: '2:9:1', allowed: true, legacyTable: 'GroupAllowedServiceCategory' }
  ];
  let rightsBody: Record<string, unknown> | undefined;
  await page.route('**/v1/roles/role-1/rights', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ roleId: 'role-1', permissions: ['sales.read'], legacyRights: [], scopes: importedScopes }) });
      return;
    }
    rightsBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ roleId: 'role-1', permissions: ['sales.read'], legacyRights: [], scopes: importedScopes }) });
  });

  const leaves = [
    { section: 'manage', kind: 'group-wise-header-setting', title: 'Group Wise Header Setting', scopeKind: 'header', scopeKey: '2:10:1:1', source: 'GroupAllowedHeader' },
    { section: 'maintenance', kind: 'group-wise-godown-setting', title: 'Group Wise Godown Setting', scopeKind: 'godown', scopeKey: '2:10:1:1', source: 'GroupAllowedGodown' },
    { section: 'manage', kind: 'group-wise-cash-account-setting', title: 'Group Wise Cash Account Setting', scopeKind: 'cash_account', scopeKey: '2:1:100:7', source: 'GroupCashAccount' },
    { section: 'manage', kind: 'group-wise-supplier-category', title: 'Group Wise Supplier Category', scopeKind: 'service_category', scopeKey: '2:9:1', source: 'GroupAllowedServiceCategory' }
  ];
  for (const leaf of leaves) {
    await page.goto(`/app/${leaf.section}/${leaf.kind}`);
    await page.getByLabel('Group:').selectOption('role-1');
    await expect(page.getByLabel(`${leaf.source} [${leaf.scopeKey}]`, { exact: true })).toBeVisible();
    await page.getByLabel(`${leaf.source} [${leaf.scopeKey}]`, { exact: true }).uncheck();
    await page.getByRole('button', { name: 'Save group scope access' }).click();
    await expect(page.locator(`section[aria-label="${leaf.title}"] footer [role="status"]`)).toContainText(`${leaf.title} access saved.`);
    expect(rightsBody?.scopes).toEqual([{ scopeKind: leaf.scopeKind, scopeKey: leaf.scopeKey, allowed: false }]);
  }
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

// Every test above drives page.route with hand-typed fixture permissions.
// That is fine for exercising the editor/menu components in isolation, but
// none of it proves the fixtures still match the real migrated group_rights
// table (see db/migrations/009_legacy_security_rights.sql and
// 019_security_data_import_adaptation.sql, and
// services/api/internal/httpapi/access_integration_test.go for the
// server-side equivalent). A fixture can silently drift from the 726-row
// import with no test noticing. fetchRealEffectivePermissions closes that
// gap for one group by reading the same tenant's live table directly and
// feeding what it actually contains into the mocked /v1/access response, so
// this file has at least one test pinned to real migrated data end to end.
const LEGACY_GROUP_RIGHTS_SOURCE_TENANT_ID = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';

function fetchRealEffectivePermissions(groupCode: string): string[] | undefined {
  const dsn = process.env.DATABASE_URL;
  if (!dsn) return undefined;
  try {
    const escapedGroupCode = groupCode.replace(/'/g, "''");
    const sql = `SELECT DISTINCT lower(coalesce(gr.permission, gr.right_code)) `
      + `FROM group_rights gr JOIN roles r ON r.id = gr.role_id AND r.tenant_id = gr.tenant_id `
      + `WHERE gr.tenant_id = '${LEGACY_GROUP_RIGHTS_SOURCE_TENANT_ID}' AND r.code = '${escapedGroupCode}' AND gr.allowed `
      + `ORDER BY 1;`;
    const output = execFileSync('psql', [dsn, '-t', '-A', '-c', sql], { encoding: 'utf8' });
    return output.split('\n').map((line) => line.trim()).filter(Boolean);
  } catch {
    return undefined;
  }
}

test('REMOTE group menu-gating reflects the live migrated group_rights matrix, not a hardcoded fixture', async ({ page }) => {
  const permissions = fetchRealEffectivePermissions('REMOTE');
  test.skip(!permissions, 'DATABASE_URL / psql was not reachable from the test runner; see fetchRealEffectivePermissions.');
  if (!permissions) return;

  // Pin the finding this test exists to catch: the migrated REMOTE right
  // codes are raw legacy numeric identifiers (e.g. "1", "5256") with no
  // `permission` column populated (see
  // docs/PHASE_R_SECURITY_RIGHTS_VERIFICATION_2026-08-08.md), so today none
  // of them equal a permission name legacy-menu.ts's requirementFor() gates
  // on. If this assertion ever fails, the migration finally populated the
  // mapping and the menu-gating expectation below must be revisited too.
  expect(permissions.length).toBeGreaterThan(0);
  expect(permissions).not.toContain('sales.write');

  await page.route('**/v1/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session(['operator'], permissions)) }));
  await page.route('**/v1/access', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(access({ permissions, legacyRights: [], scopes: {} })) }));
  await page.goto('/app/sales?kind=cash');
  await page.waitForTimeout(1000);
  const fileMenu = page.getByRole('button', { name: 'File', exact: true });
  await expect(fileMenu).toBeVisible();
  await fileMenu.evaluate((element) => (element as HTMLButtonElement).click());
  await expect(page.getByRole('menuitem', { name: 'Save And Post', exact: true })).toBeVisible();
  // requiredPermission for File > Save And Post under the cash-sale context
  // is 'sales.write' (src/lib/legacy-menu.ts requirementFor). The live
  // migrated REMOTE matrix never grants it, so the command must stay
  // disabled — driven by the real table, not by a hardcoded expectation.
  await expect(page.getByRole('menuitem', { name: 'Save And Post', exact: true })).toBeDisabled();
});

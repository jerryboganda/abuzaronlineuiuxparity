import { expect, test } from '@playwright/test';

const generalRegistry = [{
  caption: 'Enable Alias Name:',
  type: 'boolean',
  default: 'No',
  value: 'No',
  allowed: ['Yes', 'No'],
  behavior: 'Controls alias lookup.',
  runtimeStatus: 'partial',
  position: 0
}];

test('preferences round-trip, cancel, and ellipsis editing remain branch-scoped', async ({ page }) => {
  let saved: Record<string, unknown> | undefined;
  await page.route('**/v1/preferences*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      category: 'General',
      scope: { tenantId: 'tenant-a', branchId: 'branch-a' },
      items: [{ caption: 'Enable Alias Name:', value: 'No', position: 0 }],
      registry: generalRegistry,
      divergences: [],
      registryCount: 400
    })
  }));
  await page.route('**/v1/preferences', async (route) => {
    saved = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ category: 'General', saved: 40, scope: { tenantId: 'tenant-a', branchId: 'branch-a' }, divergences: [] }) });
  });

  await page.goto('/app/preferences');
  await page.waitForResponse((response) => response.url().includes('/v1/preferences?category=General'));
  await expect.poll(() => page.locator('select').count()).toBeGreaterThan(0);
  const alias = page.getByRole('combobox', { name: 'Enable Alias Name:' });
  await expect(alias).toHaveValue('No');
  await alias.selectOption('Yes');
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(alias).toHaveValue('No');
  await alias.selectOption('Yes');
  await page.getByRole('button', { name: 'Edit Enable Alias Name:' }).click();
  await expect(alias).toBeFocused();
  await page.getByRole('button', { name: 'Save' }).click();
  expect(saved?.category).toBe('General');
  expect((saved?.items as Array<{ caption: string; fieldKey: string; value: string }>).find((item) => item.caption === 'Enable Alias Name:')?.value).toBe('Yes');
  expect((saved?.items as Array<{ caption: string; fieldKey: string; value: string }>).find((item) => item.caption === 'Enable Alias Name:')?.fieldKey).toBe('general.enable-alias-name');
  await expect(page.getByRole('status')).toContainText('current branch');
});

test('preferences reports validation and permission denial instead of false success', async ({ page }) => {
  await page.route('**/v1/preferences*', (route) => route.fulfill({
    status: 403,
    contentType: 'application/problem+json',
    body: JSON.stringify({ status: 403, code: 'permission_required', detail: 'You do not have permission to read preferences.' })
  }));
  const deniedResponse = page.waitForResponse((response) => response.url().includes('/v1/preferences?category=General'));
  await page.goto('/app/preferences');
  expect((await deniedResponse).status()).toBe(403);
  await expect(page.getByRole('alert')).toContainText('permission');
});

test('schedule tab documents SQL-Agent divergence', async ({ page }) => {
  await page.route('**/v1/preferences*', (route) => {
    const category = new URL(route.request().url()).searchParams.get('category');
    if (category === 'Schedule') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          category: 'Schedule',
          items: [],
          registry: [{ caption: 'Activate:', type: 'boolean', default: 'No', value: 'No', allowed: ['Yes', 'No'], behavior: 'SQL-Agent/msdb is not configured.', runtimeStatus: 'not_configured', position: 0 }],
          divergences: [{ category: 'Schedule', status: 'not_configured', detail: 'Legacy SQL-Agent/msdb jobs are not reported as configured.' }]
        })
      });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ category: 'General', items: [], registry: [], divergences: [] }) });
  });
  const generalResponse = page.waitForResponse((response) => response.url().includes('/v1/preferences?category=General'));
  await page.goto('/app/preferences');
  await generalResponse;
  const scheduleResponse = page.waitForResponse((response) => response.url().includes('/v1/preferences?category=Schedule'));
  await page.getByRole('tab', { name: 'Schedule' }).click();
  await scheduleResponse;
  await expect(page.getByRole('note')).toContainText('SQL-Agent/msdb');
});

import { expect, test } from '@playwright/test';

const viewport = { width: 1936, height: 1048 };

test('live parity surfaces stay semantic and bounded at 1936x1048', async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  await page.setViewportSize(viewport);
  await page.route('**/v1/session', (route) => route.fulfill({
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
        username: 'ADMIN',
        displayName: 'ADMIN'
      }
    })
  }));
  await page.route('**/v1/access', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ tenantAdmin: true, permissions: [], legacyRights: [], scopes: {}, scopeRows: [], exceptions: [] })
  }));
  await page.route('**/v1/preferences*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: [] })
  }));
  await page.route('**/v1/roles', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ roles: [] })
  }));

  const surfaces = {
    customer: '/app/master/customer',
    item: '/app/master/item',
    cashSale: '/app/sales?kind=cash',
    packPurchase: '/app/purchase/pack',
    dailySalesDetail: '/app/report/daily-sales-detail',
    preferences: '/app/preferences',
    groups: '/app/manage/groups'
  };
  const captures: Array<Record<string, unknown>> = [];

  for (const [name, url] of Object.entries(surfaces)) {
    await page.goto(url);
    await page.waitForTimeout(name === 'dailySalesDetail' ? 2000 : 250);
    const state = await page.evaluate(() => {
      const main = document.querySelector('main');
      const root = document.documentElement;
      const visible = [...document.querySelectorAll('main *')].filter((element) => {
        const box = element.getBoundingClientRect();
        return box.width > 0 && box.height > 0 && getComputedStyle(element).opacity !== '0';
      });
      const backgrounds = [...document.querySelectorAll('main')].map((element) => getComputedStyle(element).backgroundImage);
      return {
        viewport: { width: innerWidth, height: innerHeight },
        mainClass: main?.className ?? '',
        scrollHeight: root.scrollHeight,
        liveElementCount: visible.length,
        hasSubstrateBackground: backgrounds.some((background) => background !== 'none'),
        hiddenChildCount: [...document.querySelectorAll('main > section > *')].filter((element) => getComputedStyle(element).opacity === '0').length
      };
    });
    const capturePath = testInfo.outputPath(`${name}.png`);
    await page.screenshot({ path: capturePath, fullPage: false });
    captures.push({
      name,
      url,
      ...state,
      capture: capturePath,
      comparison: {
        status: 'not-compared',
        differentPixels: null,
        maxChannelDelta: null,
        exception: 'No fresh independent legacy capture was available in this run; existing 1922x970/1536x972 substrates are not used as acceptance baselines.'
      }
    });
    expect(state.viewport).toEqual(viewport);
    expect(state.scrollHeight).toBeLessThanOrEqual(viewport.height);
    expect(state.liveElementCount).toBeGreaterThan(10);
    expect(state.hasSubstrateBackground).toBe(false);
    expect(state.hiddenChildCount).toBe(0);
  }
  expect(captures).toHaveLength(Object.keys(surfaces).length);
});

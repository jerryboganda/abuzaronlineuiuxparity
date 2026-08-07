import { test, expect } from '@playwright/test';

test.describe('Purchase Inventory Picker Parity', () => {
  test('displays real-time inventory lookup and populates item details on purchase routes', async ({ page }) => {
    await page.route(/\/v1\/items\/lookup/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          query: 'Panadol',
          items: [
            {
              id: '00000000-0000-0000-0000-000000000101',
              code: 'PND-500',
              legacyId: 'PND-500',
              name: 'PANADOL 500MG TAB',
              aliases: ['Panadol'],
              active: true,
              payload: {
                PurchasePrice: '12.50',
                SalePrice1: '15.00',
                Manufacturer: 'GSK',
                PackUnits: '100',
                Location: 'SHELF-A1'
              }
            }
          ]
        })
      });
    });

    await page.goto('/app/purchase/pack');
    await expect(page.locator('.legacy-purchase-lookup')).toBeVisible();
    const searchInput = page.locator('input[aria-label="Item lookup query"]');
    await searchInput.fill('Panadol');
    await expect(page.locator('.legacy-purchase-lookup table')).toContainText('PANADOL 500MG TAB', { timeout: 10000 });
    await page.locator('.legacy-purchase-lookup table button').first().click();
    await expect(page.locator('input[aria-label="Item name 1"]')).toHaveValue('PANADOL 500MG TAB');
  });
});

import { test, expect } from '@playwright/test';

test.describe('Purchase Inventory Picker Parity', () => {
  test('displays real-time inventory lookup and populates item details on purchase routes', async ({ page }) => {
    await page.goto('/app/purchase/pack');
    await expect(page.locator('.legacy-purchase-lookup')).toBeVisible();
    const searchInput = page.locator('input[aria-label="Item lookup query"]');
    await searchInput.fill('Panadol');
    await expect(page.locator('.legacy-purchase-lookup table')).toContainText('Panadol');
    await page.locator('.legacy-purchase-lookup button').first().click();
    await expect(page.locator('input[aria-label="Item name 1"]')).not.toHaveValue('');
  });
});

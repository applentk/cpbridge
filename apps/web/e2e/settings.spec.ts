import { test, expect } from '@playwright/test';
import { setupApiMocks } from './fixtures/api-mock';

test.describe('Platform Integrations Settings', () => {
  test('renders integrations page, zero-cookie notice, extension guide, and platform cards when active', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/settings/integrations');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Platform Integrations');
    await expect(page.locator('text=Zero-Cookie Privacy Guarantee')).toBeVisible();

    // Extension active status
    await expect(page.locator('text=Browser Extension Active')).toBeVisible();
    await expect(page.locator('text=v1.0.0')).toBeVisible();

    // Download button
    const downloadBtn = page.locator('a[download="cpbridge-extension.zip"]');
    await expect(downloadBtn).toBeVisible();

    // Platform cards with exact heading role
    await expect(page.getByRole('heading', { name: 'Codeforces', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'AtCoder', exact: true })).toBeVisible();
  });

  test('renders Extension Not Detected warning and 4-step installation guide when extension is absent', async ({ page }) => {
    await setupApiMocks(page, { disableExtension: true });

    await page.goto('/settings/integrations');
    await page.waitForLoadState('networkidle');

    // Warning banner
    await expect(page.locator('text=Extension Not Detected')).toBeVisible();
    await expect(page.locator('text=Quick 4-Step Installation Guide')).toBeVisible();
    await expect(page.locator('text=Download & Unzip')).toBeVisible();
  });
});

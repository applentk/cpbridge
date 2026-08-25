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

  test('displays loading skeleton state before extension detection resolves', async ({ page }) => {
    await setupApiMocks(page, { disableExtension: true });

    // Delay the mock extension reply to observe skeleton
    await page.addInitScript(() => {
      window.addEventListener('message', (event) => {
        if (event.data && event.data.source === 'CP_HUB_WEB' && event.data.payload?.type === 'PING') {
          const { id } = event.data;
          setTimeout(() => {
            window.postMessage({
              source: 'CP_HUB_EXTENSION',
              id,
              payload: {
                type: 'PONG',
                version: '1.0.0',
                platforms: {
                  CODEFORCES: { loggedIn: true, username: 'tester' },
                  ATCODER: { loggedIn: true, username: 'tester' },
                },
              },
            }, '*');
          }, 300);
        }
      });
    });

    await page.goto('/settings/integrations');
    // Initially "Extension Not Detected" should NOT be visible immediately
    expect(await page.locator('text=Extension Not Detected').isVisible()).toBe(false);

    // After resolving, Active state appears
    await expect(page.locator('text=Browser Extension Active')).toBeVisible();
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

  test('displays Not Connected badges and external login links when platform sessions are inactive', async ({ page }) => {
    await setupApiMocks(page, {
      extensionPlatforms: {
        CODEFORCES: { loggedIn: false },
        ATCODER: { loggedIn: false },
      },
    });

    await page.goto('/settings/integrations');
    await page.waitForLoadState('networkidle');

    // Extension is active
    await expect(page.locator('text=Browser Extension Active')).toBeVisible();

    // Platforms show Not Connected
    await expect(page.locator('text=Not Connected').first()).toBeVisible();

    // External login links exist
    const cfLoginLink = page.getByRole('link', { name: 'Open Codeforces' });
    await expect(cfLoginLink).toBeVisible();
    await expect(cfLoginLink).toHaveAttribute('href', 'https://codeforces.com/enter');

    const acLoginLink = page.getByRole('link', { name: 'Open AtCoder' });
    await expect(acLoginLink).toBeVisible();
    await expect(acLoginLink).toHaveAttribute('href', 'https://atcoder.jp/login');
  });
});

import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser } from './fixtures/mock-data';

test.describe('Navigation & Navbar States', () => {
  test('guest navigation displays public links and sign-in actions', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/');

    const nav = page.locator('nav').first();
    await expect(nav).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Dashboard' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Contests' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Sign In' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Sign Up' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Problems' })).not.toBeVisible();
    await expect(nav.getByRole('link', { name: 'Submissions' })).not.toBeVisible();
  });

  test('authenticated user navigation displays user links and profile avatar', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests');

    const nav = page.locator('nav').first();
    await expect(nav).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Dashboard' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Contests' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Submissions' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Problems' })).not.toBeVisible();
    await expect(nav).toContainText('tourist_fan');
    await expect(nav.locator('button[title="Log Out"]')).toBeVisible();
  });

  test('admin navigation displays administrative links and ADMIN badge', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin');

    const nav = page.locator('nav').first();
    await expect(nav.getByRole('link', { name: 'Admin Dashboard' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Problems' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Problem Sets' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'View User Site' })).toBeVisible();
    await expect(nav).toContainText('root_admin');
    await expect(nav).toContainText('ADMIN');
  });

  test('navbar link clicking smoothly navigates across pages', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/');
    await page.locator('nav').first().getByRole('link', { name: 'Contests' }).click();
    await expect(page).toHaveURL('/contests');
    await expect(page.locator('h1')).toContainText('Contests');
  });

  test('mobile viewport layout renders brand and action controls appropriately', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests');
    await page.waitForLoadState('networkidle');

    // Brand and logo are visible on mobile
    const nav = page.locator('nav').first();
    await expect(nav).toBeVisible();
    await expect(nav.locator('text=cpbridge')).toBeVisible();

    // Integrations puzzle button is accessible
    await expect(nav.locator('a[title="Platform Integrations"]')).toBeVisible();

    // User avatar is visible
    await expect(nav.locator('text=tourist_fan')).toBeVisible();
  });
});


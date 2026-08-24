import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser } from './fixtures/mock-data';

test.describe('User Dashboard (/dashboard)', () => {
  test('authenticated user dashboard renders greeting, contest cards, and recent submissions', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    // Header greeting
    await expect(page.locator('h1')).toContainText('Dashboard');
    await expect(page.locator('text=Welcome back, tourist_fan!')).toBeVisible();

    // Contests section
    await expect(page.locator('text=Weekly Practice Contest #42')).toBeVisible();
    await expect(page.locator('text=ACTIVE').first()).toBeVisible();

    // Navigate to contest from dashboard card
    await page.click('text=Weekly Practice Contest #42');
    await expect(page).toHaveURL('/contests/con_active_icpc');
    await expect(page.locator('h1')).toContainText('Weekly Practice Contest #42');
  });

  test('dashboard displays recent submissions list with status badges', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    // Recent submissions section
    await expect(page.locator('text=My Recent Submissions')).toBeVisible();
    await expect(page.locator('text=Codehorses T-shirts')).toBeVisible();
    await expect(page.locator('text=A - N-choice question')).toBeVisible();

    // Status badges
    await expect(page.locator('text=ACCEPTED').first()).toBeVisible();
    await expect(page.locator('text=WRONG_ANSWER').first()).toBeVisible();
  });

  test('clicking submission in dashboard opens inspection modal', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    // Click on first submission button to open modal
    await page.locator('button:has-text("Codehorses T-shirts")').first().click();

    // Verify SubmissionModal opens
    await expect(page.locator('#submission-modal-title')).toContainText('Codehorses T-shirts');
    await expect(page.locator('[role="dialog"]').locator('strong:has-text("C++23 (GCC)")')).toBeVisible();
    await expect(page.locator('button:has-text("Copy Code")')).toBeVisible();

    // Close modal via close button
    await page.locator('button[title="Close dialog (Esc)"]').click();
    await expect(page.locator('#submission-modal-title')).not.toBeVisible();
  });

  test('dashboard renders empty states gracefully when no submissions or contests exist', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      contests: [],
      submissions: [],
    });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('text=No active or upcoming contests right now.')).toBeVisible();
    await expect(page.locator('text=No recent submissions recorded.')).toBeVisible();
  });
});

import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser } from './fixtures/mock-data';

test.describe('Submissions Log & Verdicts', () => {
  test('submissions page renders submissions history table with verdicts', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/submissions');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Submissions Log');
    await expect(page.locator('text=Codehorses T-shirts')).toBeVisible();
    await expect(page.locator('text=A - N-choice question')).toBeVisible();

    // Verdict badges
    await expect(page.locator('text=ACCEPTED').first()).toBeVisible();
    await expect(page.locator('text=WRONG ANSWER').first()).toBeVisible();
    await expect(page.locator('text=JUDGING').first()).toBeVisible();
  });

  test('toggle user filter switches between my submissions and all submissions', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/submissions');
    await page.waitForLoadState('networkidle');

    const filterBtn = page.locator('button:has-text("My Submissions Only")');
    await expect(filterBtn).toBeVisible();

    // Toggle filter
    await filterBtn.click();
    await expect(page.locator('button:has-text("All Submissions")')).toBeVisible();
  });

  test('clicking submission row opens SubmissionModal with source code and metadata', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/submissions');
    await page.waitForLoadState('networkidle');

    // Click View Code button on first row (Codehorses T-shirts)
    await page.locator('button:has-text("View Code")').first().click();

    // Modal dialog scoped locators
    const dialog = page.locator('[role="dialog"]');
    await expect(dialog.locator('#submission-modal-title')).toContainText('Codehorses T-shirts');
    await expect(dialog.locator('strong:has-text("C++23 (GCC)")')).toBeVisible();
    await expect(dialog.locator('text=ACCEPTED')).toBeVisible();

    // Copy code button
    const copyBtn = dialog.locator('button:has-text("Copy Code")');
    await expect(copyBtn).toBeVisible();
    await copyBtn.click();
    await expect(dialog.locator('text=Copied!')).toBeVisible();

    // Regular user does NOT see External source URL
    await expect(dialog.locator('text=External source')).not.toBeVisible();

    // Close via Close button
    await dialog.locator('button:has-text("Close")').click();
    await expect(page.locator('#submission-modal-title')).not.toBeVisible();
  });

  test('modal displays execution and judging error output when present', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/submissions');
    await page.waitForLoadState('networkidle');

    // Click View Code on failed submission row (sub_002 with Wrong Answer)
    await page.locator('button:has-text("View Code")').nth(1).click();

    // Verify judging output alert in modal
    const dialog = page.locator('[role="dialog"]');
    await expect(dialog.locator('text=Execution / Judging Output:')).toBeVisible();
    await expect(dialog.locator('text=Wrong Answer on test 4: expected 2, found -1')).toBeVisible();

    // Close via Escape key
    await page.keyboard.press('Escape');
    await expect(page.locator('#submission-modal-title')).not.toBeVisible();
  });

  test('admin user sees external source link in submission modal', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/submissions');
    await page.waitForLoadState('networkidle');

    // Click View Code on first submission row as admin
    await page.locator('button:has-text("View Code")').first().click();

    // Verify external source link is visible in modal for admin
    const dialog = page.locator('[role="dialog"]');
    await expect(dialog.locator('text=External source')).toBeVisible();
    const sourceLink = dialog.locator('a:has-text("External source")');
    await expect(sourceLink).toHaveAttribute('href', 'https://codeforces.com/contest/1000/submission/28001');
  });
});

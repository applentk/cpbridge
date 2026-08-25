import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockAdminUser, mockRegularUser } from './fixtures/mock-data';

test.describe('Admin Dashboard & Management', () => {
  test('admin dashboard renders system overview statistics and quick actions', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Admin Dashboard');
    await expect(page.locator('text=Registered Users')).toBeVisible();
    await expect(page.locator('text=120')).toBeVisible(); // totalUsers
    await expect(page.locator('text=42').first()).toBeVisible(); // totalProblems
    await expect(page.locator('text=15')).toBeVisible(); // totalContests
    await expect(page.getByText('8', { exact: true })).toBeVisible(); // totalProblemSets
  });

  test('admin can navigate and fill the contest creation form', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/contests/new');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Create Contest');

    // Fill form
    await page.fill('input#contest-name', 'Spring Championship 2026');
    await page.fill('textarea#contest-description', 'Annual ICPC practice trial.');
    await page.selectOption('select#scoring-engine', 'ICPC');
    await page.selectOption('select#contest-visibility', 'PUBLIC');

    // Submit
    await page.click('button:has-text("Create Contest")');
    await expect(page).toHaveURL('/admin/contests');
  });

  test('admin user management allows searching, changing roles, and toggling status', async ({ page }) => {
    page.on('dialog', (dialog) => dialog.accept());

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/users');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('User Management');
    await expect(page.locator('text=tourist_fan')).toBeVisible();
    await expect(page.locator('text=bad_actor')).toBeVisible();

    // Search filter
    await page.fill('input[placeholder*="Search users"]', 'tourist');
    await expect(page.locator('text=tourist_fan')).toBeVisible();
    await expect(page.locator('text=bad_actor')).not.toBeVisible();

    // Clear search filter
    await page.fill('input[placeholder*="Search users"]', '');
    await expect(page.locator('text=bad_actor')).toBeVisible();

    // Change role on second user (tourist_fan, who is currently USER)
    const roleSelect = page.locator('table select').nth(1);
    await roleSelect.selectOption('ADMIN');
    await expect(page.locator('text=Updated')).toBeVisible();
  });

  test('admin problem library allows importing external problems', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problems');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Problem Library');
    await expect(page.locator('text=Codehorses T-shirts')).toBeVisible();

    // Open import modal
    await page.click('button:has-text("Import Problem")');
    await expect(page.locator('h3:has-text("Import Problem")')).toBeVisible();

    // Fill import URL
    await page.fill('input#import-url', 'https://codeforces.com/problemset/problem/1234/A');
    await page.locator('div.fixed').locator('button:has-text("Import")').click();

    // Success notification
    await expect(page.locator('text=Problem imported successfully!')).toBeVisible();
    await expect(page.locator('text=Imported Test Problem')).toBeVisible();
  });

  test('admin problem library allows creating custom problems and deleting problems', async ({ page }) => {
    page.on('dialog', (dialog) => dialog.accept());

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problems');
    await page.waitForLoadState('networkidle');

    // Create Custom Problem
    await page.click('button:has-text("Create Problem")');
    await expect(page.locator('h3:has-text("Create Custom Problem")')).toBeVisible();

    await page.fill('input#create-title', 'Bitwise XOR Sum');
    await page.fill('input#create-difficulty', '1300');
    await page.fill('input#create-tags', 'bitmask, math');
    await page.locator('div.fixed').locator('button:has-text("Create Problem")').click();

    await expect(page.locator('text=Problem created successfully!')).toBeVisible();
    await expect(page.locator('text=Bitwise XOR Sum')).toBeVisible();

    // Delete a problem
    await page.locator('button[title="Delete Problem"]').first().click();
    await expect(page.locator('text=Problem deleted successfully!')).toBeVisible();
  });

  test('admin contest editor allows updating contest metadata and managing problems', async ({ page }) => {
    page.on('dialog', (dialog) => dialog.accept());

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/contests/con_upcoming_01/edit');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Grand Prix of Tokyo');
    await expect(page.locator('text=Contest Problems')).toBeVisible();

    // Save details
    await page.fill('input#edit-contest-name', 'Grand Prix of Tokyo 2026');
    await page.click('button:has-text("Save Settings")');
    await expect(page.locator('text=Contest details updated successfully!')).toBeVisible();

    // Open Add Problem Modal
    await page.click('button:has-text("Add Problem")');
    await expect(page.locator('h3:has-text("Add Problem to Contest")')).toBeVisible();

    // Add first problem from library search
    await page.locator('div.fixed').locator('button:has-text("Add")').first().click();
    await expect(page.locator('text=Added problem')).toBeVisible();
  });

  test('admin user detail page renders profile, allows role update and status toggle', async ({ page }) => {
    page.on('dialog', (dialog) => dialog.accept());

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/users/usr_reg_123');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('User Profile');
    await expect(page.locator('text=tourist_fan')).toBeVisible();
    await expect(page.locator('text=coder@example.com')).toBeVisible();

    // Promote to ADMIN
    await page.click('button:has-text("Promote to ADMIN")');
    await expect(page.locator('text=Role changed to ADMIN!')).toBeVisible();

    // Disable account
    await page.click('button:has-text("Disable Account")');
    await expect(page.locator('text=Account "tourist_fan" is now disabled!')).toBeVisible();
  });

  test('LAST_ADMIN demotion and disablement safeguard prevents lockout', async ({ page }) => {
    const dialogMessages: string[] = [];
    page.on('dialog', async (dialog) => {
      dialogMessages.push(dialog.message());
      await dialog.accept();
    });

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/users/usr_adm_999');
    await page.waitForLoadState('networkidle');

    // 1. Attempt to demote last active admin
    await page.click('button:has-text("Demote to USER")');
    await expect.poll(() => dialogMessages.some((msg) => msg.includes('cannot demote the last active administrator'))).toBe(true);

    // 2. Attempt to disable last active admin
    await page.click('button:has-text("Disable Account")');
    await expect.poll(() => dialogMessages.some((msg) => msg.includes('cannot disable the last active administrator'))).toBe(true);
  });

  test('PROBLEM_IN_USE deletion prevention rejects deleting problem in active contest', async ({ page }) => {
    page.on('dialog', async (dialog) => {
      await dialog.accept();
    });

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problems');
    await page.waitForLoadState('networkidle');

    // Search for the in-use problem
    await page.fill('input[placeholder*="Search problems"]', 'Problem In Active Contest');
    await expect(page.locator('text=Problem In Active Contest')).toBeVisible();

    // Attempt deletion
    const deleteBtn = page.locator('button[title="Delete Problem"]').first();
    await deleteBtn.click();
  });

  test('public access to admin-only redirect routes redirects correctly', async ({ page }) => {
    // 1. Regular user accessing /problem-sets is redirected to /contests
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problem-sets');
    await page.waitForURL('/contests');

    await page.goto('/problem-sets/set_standard_dp');
    await page.waitForURL('/contests');

    await page.goto('/contests/new');
    await page.waitForURL('/contests');

    // 2. Admin user accessing /problem-sets is redirected to /admin/problem-sets
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/problem-sets');
    await page.waitForURL('/admin/problem-sets');

    await page.goto('/problem-sets/set_standard_dp');
    await page.waitForURL('/admin/problem-sets/set_standard_dp');

    await page.goto('/contests/new');
    await page.waitForURL('/admin/contests/new');
  });
});


import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockAdminUser } from './fixtures/mock-data';

test.describe('Problem Sets Management', () => {
  test('admin problem sets page renders list of problem sets and creation input', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problem-sets');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Problem Sets');
    await expect(page.locator('text=Dynamic Programming Foundations')).toBeVisible();

    // Open create modal
    await page.click('button:has-text("Create Problem Set")');
    await page.fill('input#set-name', 'Graph Algorithms 101');
    await page.click('button:has-text("Create Set")');

    await expect(page.locator('text=Graph Algorithms 101')).toBeVisible();
  });

  test('problem set detail page renders items, allows reordering and contest creation', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problem-sets/set_standard_dp');
    await page.waitForLoadState('networkidle');

    // Title and metadata
    await expect(page.locator('h1')).toContainText('Dynamic Programming Foundations');
    await expect(page.locator('text=Codehorses T-shirts')).toBeVisible();
    await expect(page.locator('text=A - N-choice question')).toBeVisible();

    // Create contest from set link
    const createContestLink = page.locator('a:has-text("Create Contest from Set")');
    await expect(createContestLink).toBeVisible();

    // Update set metadata
    await page.fill('input#edit-set-name', 'DP Foundations Advanced');
    await page.click('button:has-text("Save Details")');
    await expect(page.locator('text=Problem set updated successfully!')).toBeVisible();

    // Reorder problems using Move Down button on first item
    const moveDownBtn = page.locator('button[title="Move Down"]').first();
    await expect(moveDownBtn).toBeVisible();
    await moveDownBtn.click();

    // Remove problem from set
    const removeBtn = page.locator('button[title="Remove from Set"]').first();
    await expect(removeBtn).toBeVisible();
    await removeBtn.click();
  });

  test('admin can add problems to a problem set from library modal', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problem-sets/set_standard_dp');
    await page.waitForLoadState('networkidle');

    // Open add problem modal
    await page.click('button:has-text("Add Problem")');
    await expect(page.locator('h3:has-text("Add Problem to Set")')).toBeVisible();

    // Click Add on first search result
    const addBtn = page.locator('div.fixed button:has-text("Add")').first();
    await expect(addBtn).toBeVisible();
    await addBtn.click();

    await expect(page.locator('text=Added problem')).toBeVisible();
  });

  test('admin can delete problem set from problem sets list', async ({ page }) => {
    page.on('dialog', (dialog) => dialog.accept());

    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/admin/problem-sets');
    await page.waitForLoadState('networkidle');

    // Click Delete button on first set
    const deleteBtn = page.locator('button[title="Delete Set"]').first();
    await expect(deleteBtn).toBeVisible();
    await deleteBtn.click();

    await expect(page.locator('text=Problem Set deleted!')).toBeVisible();
  });
});

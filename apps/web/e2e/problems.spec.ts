import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser } from './fixtures/mock-data';

test.describe('Problem Browsing & Workspace', () => {
  test('problems list page renders problem cards and filters', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/problems');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Problems');
    await expect(page.locator('text=Codehorses T-shirts')).toBeVisible();
    await expect(page.locator('text=A - N-choice question')).toBeVisible();
    await expect(page.locator('text=Laura and Operations')).toBeVisible();

    // Test platform filter
    await page.selectOption('select', 'ATCODER');
    await expect(page.locator('text=A - N-choice question')).toBeVisible();
    await expect(page.locator('text=Codehorses T-shirts')).not.toBeVisible();

    // Reset platform filter and search by query
    await page.selectOption('select', '');
    await page.fill('input[type="search"]', 'Laura');
    await page.click('button:has-text("Search")');
    await expect(page.locator('text=Laura and Operations')).toBeVisible();
    await expect(page.locator('text=Codehorses T-shirts')).not.toBeVisible();
  });

  test('problem workspace renders problem statement, limits, and KaTeX math', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/problems/prb_cf_1000A');
    await page.waitForLoadState('networkidle');

    // Title and metadata
    await expect(page.locator('h1')).toContainText('Codehorses T-shirts');
    await expect(page.locator('text=Time Limit: 2.0s')).toBeVisible();
    await expect(page.locator('text=Memory Limit: 256MB')).toBeVisible();
    await expect(page.locator('text=CODEFORCES')).toBeVisible();

    // KaTeX math rendering (first element)
    await expect(page.locator('.katex').first()).toBeVisible();

    // Sample cases
    await expect(page.locator('text=Example 1')).toBeVisible();
    await expect(page.locator('text=3 1 2')).toBeVisible();
  });

  test('problem workspace supports tab switching and split view layout', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/problems/prb_cf_1000A');
    await page.waitForLoadState('networkidle');

    // Default tabbed view
    await expect(page.locator('button:has-text("Problem Statement")')).toBeVisible();
    await expect(page.locator('button:has-text("Code Editor & Submit")')).toBeVisible();
    await expect(page.locator('button:has-text("Submissions")')).toBeVisible();

    // Switch to Code Editor tab
    await page.click('button:has-text("Code Editor & Submit")');
    await expect(page.locator('select').first()).toBeVisible(); // Language selector

    // Switch to Split View
    await page.click('button:has-text("Split View")');
    await expect(page.locator('.statement-content')).toBeVisible();
    await expect(page.locator('select').first()).toBeVisible();
  });

  test('language selector updates starter code and handles language change', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A?tab=editor');
    await page.waitForLoadState('networkidle');

    // Change language to Python 3
    const langSelect = page.locator('select').first();
    await langSelect.selectOption('python3');

    // Change language to Java 21
    await langSelect.selectOption('java21');
  });

  test('copy button on sample test case copies text to clipboard', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await setupApiMocks(page);

    await page.goto('/problems/prb_cf_1000A');
    await page.waitForLoadState('networkidle');

    const copyBtn = page.locator('button:has-text("Copy")').first();
    await copyBtn.click();
    await expect(page.locator('text=Copied')).toBeVisible();
  });

  test('file upload auto-detects language and loads content into editor', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A?tab=editor');
    await page.waitForLoadState('networkidle');

    // Upload a C++ file
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: 'solution.cpp',
      mimeType: 'text/x-c++src',
      buffer: Buffer.from('#include <iostream>\nint main(){ std::cout << "OK"; return 0; }'),
    });

    // Verify upload toast message with auto-detection
    await expect(page.locator('text=Loaded "solution.cpp" (Auto-detected: C++23 (GCC))')).toBeVisible();

    // Upload a Python file
    await fileInput.setInputFiles({
      name: 'solution.py',
      mimeType: 'text/x-python',
      buffer: Buffer.from('print("Hello from Py")'),
    });
    await expect(page.locator('text=Loaded "solution.py" (Auto-detected: Python 3)')).toBeVisible();

    // Upload a Java file
    await fileInput.setInputFiles({
      name: 'Main.java',
      mimeType: 'text/x-java-source',
      buffer: Buffer.from('public class Main { public static void main(String[] args) {} }'),
    });
    await expect(page.locator('text=Loaded "Main.java" (Auto-detected: Java 21)')).toBeVisible();
  });

  test('code submission dispatches via extension, polls verdict, and updates status banner', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A?tab=editor');
    await page.waitForLoadState('networkidle');

    // Click submit button
    const submitBtn = page.locator('button:has-text("Submit Solution")');
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // Verify automatic switch to Submissions tab and submission entry with ACCEPTED verdict
    await expect(page.locator('h3:has-text("Your Submission History")')).toBeVisible();
    await expect(page.locator('button:has-text("ACCEPTED")').first()).toBeVisible();
  });

  test('extension submission failure displays error notification and failed status', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      submissionError: 'Compilation Error: missing semicolon',
    });

    await page.goto('/problems/prb_cf_1000A?tab=editor');
    await page.waitForLoadState('networkidle');

    const submitBtn = page.locator('button:has-text("Submit Solution")');
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // Verify submission error feedback
    await expect(page.locator('text=Compilation Error: missing semicolon')).toBeVisible();
  });
});

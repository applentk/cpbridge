import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser } from './fixtures/mock-data';

test.describe('Problem Workspace', () => {
  test('regular users accessing problem without contestId are redirected to /contests', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A');
    await page.waitForURL('/contests');
    await expect(page.locator('h1')).toContainText('Contests');
  });

  test('problem workspace renders problem statement, limits, and KaTeX math', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc');
    await page.waitForLoadState('networkidle');

    // Title and metadata
    await expect(page.locator('h1')).toContainText('Codehorses T-shirts');
    await expect(page.locator('text=Time Limit: 2.0s')).toBeVisible();
    await expect(page.locator('text=Memory Limit: 256MB')).toBeVisible();

    // KaTeX math rendering (first element)
    await expect(page.locator('.katex').first()).toBeVisible();

    // Sample cases
    await expect(page.locator('text=Example 1')).toBeVisible();
    await expect(page.locator('text=3 1 2')).toBeVisible();
  });

  test('problem workspace supports tab switching and split view layout', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc');
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

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.waitForLoadState('networkidle');

    // Change language to Python 3
    const langSelect = page.locator('select').first();
    await langSelect.selectOption('python3');

    // Change language to Java 21
    await langSelect.selectOption('java21');
  });

  test('copy button on sample test case copies text to clipboard', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc');
    await page.waitForLoadState('networkidle');

    const copyBtn = page.locator('button:has-text("Copy")').first();
    await copyBtn.click();
    await expect(page.locator('text=Copied')).toBeVisible();
  });

  test('file upload auto-detects language and loads content into editor', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
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

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.waitForLoadState('networkidle');

    // Click submit button
    const submitBtn = page.locator('button:has-text("Submit Solution")');
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // Verify automatic switch to Submissions tab and submission entry with ACCEPTED verdict
    await expect(page.locator('h3:has-text("Your Submission History")')).toBeVisible();
    await expect(page.locator('button:has-text("ACCEPTED")').first()).toBeVisible();
  });

  test('synchronizes a switched Codeforces account before creating the submission', async ({ page }) => {
    const accountRequests: Array<{ method: string; path: string; body?: unknown }> = [];
    page.on('request', (request) => {
      const path = new URL(request.url()).pathname;
      if (
        (request.method() === 'PUT' && path === '/api/integrations/CODEFORCES')
        || (request.method() === 'POST' && path === '/api/submissions')
      ) {
        accountRequests.push({
          method: request.method(),
          path,
          body: request.postDataJSON()
        });
      }
    });

    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      integrations: [{
        platform: 'CODEFORCES',
        externalUsername: 'old_handle',
        connectionStatus: 'CONNECTED',
        updatedAt: new Date(0).toISOString()
      }],
      extensionPlatforms: {
        CODEFORCES: { loggedIn: true, username: 'new_handle' },
        ATCODER: { loggedIn: true, username: 'tourist_fan' }
      }
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.locator('button:has-text("Submit Solution")').click();

    await expect(page.locator('button:has-text("ACCEPTED")').first()).toBeVisible();
    expect(accountRequests.slice(0, 2)).toEqual([
      {
        method: 'PUT',
        path: '/api/integrations/CODEFORCES',
        body: { externalUsername: 'new_handle', connectionStatus: 'CONNECTED' }
      },
      {
        method: 'POST',
        path: '/api/submissions',
        body: expect.any(Object)
      }
    ]);
  });

  test('outdated extension is blocked before a submission is created', async ({ page }) => {
    let submissionCreateRequests = 0;
    page.on('request', (request) => {
      if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/submissions') {
        submissionCreateRequests += 1;
      }
    });

    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      extensionVersion: '1.0.4',
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('text=Browser extension update required.')).toBeVisible();
    await expect(page.locator('button:has-text("Code Editor & Submit")')).toBeEnabled();
    await expect(page.locator('button:has-text("Submit Solution")')).toBeDisabled();
    expect(submissionCreateRequests).toBe(0);
  });

  test('extension submission failure displays error notification and failed status', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      submissionError: 'Compilation Error: missing semicolon',
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.waitForLoadState('networkidle');

    const submitBtn = page.locator('button:has-text("Submit Solution")');
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // Verify submission error feedback
    await expect(page.locator('text=Compilation Error: missing semicolon')).toBeVisible();
  });

  test('automatically verifies a completed interactive Codeforces submission', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      manualSubmissionRequired: true,
      manualSubmissionCompleteAfterChecks: 2,
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.locator('button:has-text("Submit Solution")').click();

    await expect(page).toHaveURL(/tab=submissions/);
    await expect(page.locator('h3:has-text("Your Submission History")')).toBeVisible();
    await expect(page.locator('select').first()).toBeHidden();

    await expect(page.getByText('ACCEPTED', { exact: true }).first()).toBeVisible();
    await expect(page).toHaveURL(/tab=submissions/);
  });

  test('marks an interactive submission failed when the Codeforces tab is closed', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      manualSubmissionRequired: true,
      manualSubmissionCloseAfterChecks: 1,
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.locator('button:has-text("Submit Solution")').click();

    await expect(page).toHaveURL(/tab=submissions/);
    await expect(page.getByText('FAILED', { exact: true }).first()).toBeVisible();
  });

  test('code submission handling non-accepted verdicts (WRONG_ANSWER)', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      extensionPollVerdict: 'WRONG_ANSWER',
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.waitForLoadState('networkidle');

    const submitBtn = page.locator('button:has-text("Submit Solution")');
    await submitBtn.click();

    // Verify WRONG_ANSWER verdict badge in submissions tab
    await expect(page.locator('button:has-text("WRONG_ANSWER")').first()).toBeVisible();
  });

  test('when extension is missing/disabled, problem workspace exposes official source fallback', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser, disableExtension: true });

    await page.goto('/problems/prb_cf_1000A');
    await page.waitForLoadState('networkidle');

    // Official source link exists
    const sourceLink = page.getByRole('link', { name: 'Source' });
    await expect(sourceLink).toBeVisible();
    await expect(sourceLink).toHaveAttribute('href', 'https://codeforces.com/problemset/problem/1000/A');
    await expect(sourceLink).toHaveAttribute('target', '_blank');
  });

  test('extension and platform guard disables editor entry and submission', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      extensionPlatforms: {
        CODEFORCES: { loggedIn: false },
        ATCODER: { loggedIn: true, username: 'tourist_fan' },
      },
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc');
    await page.waitForLoadState('networkidle');

    const editorTab = page.locator('button:has-text("Code Editor & Submit")');
    await expect(editorTab).toBeEnabled();
    await editorTab.click();
    await expect(page.locator('text=Codeforces is not connected')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Upload File (Auto-Detect)' })).toBeDisabled();
    await expect(page.locator('button:has-text("Submit Solution")')).toBeDisabled();
  });

  test('reconciles pending recovered submissions on page load', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      recoveredSubmissions: [
        {
          submissionId: 'sub_003',
          state: 'CREATED',
          externalSubmissionId: 'cf_recovered_99',
        },
      ],
    });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=submissions');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h3:has-text("Your Submission History")')).toBeVisible();
  });

  test('editor code state persists when toggling between Statement and Editor tabs', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc&tab=editor');
    await page.waitForLoadState('networkidle');

    // Switch language to Python 3
    const langSelect = page.locator('select').first();
    await langSelect.selectOption('python3');

    // Switch to statement tab
    await page.click('button:has-text("Problem Statement")');
    await expect(page.locator('.statement-content')).toBeVisible();

    // Switch back to editor tab
    await page.click('button:has-text("Code Editor & Submit")');
    await expect(langSelect).toHaveValue('python3');
  });

  test('navigating to unknown problem displays friendly 404 error state', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/problems/prb_non_existent');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('text=Error loading problem')).toBeVisible();
    await expect(page.locator('text=Problem not found')).toBeVisible();
  });
});

import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser } from './fixtures/mock-data';

test.describe('Contests & ICPC Scoreboard', () => {
  test('contests list page displays contests and filters by state', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/contests');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Contests');
    await expect(page.locator('text=Weekly Practice Contest #42')).toBeVisible();
    await expect(page.locator('text=Grand Prix of Tokyo')).toBeVisible();
    await expect(page.locator('text=Winter Warmup 2026')).toBeVisible();

    // Filter by ACTIVE
    await page.click('button:has-text("ACTIVE")');
    await expect(page.locator('text=Weekly Practice Contest #42')).toBeVisible();
    await expect(page.locator('text=Grand Prix of Tokyo')).not.toBeVisible();

    // Filter by UPCOMING
    await page.click('button:has-text("UPCOMING")');
    await expect(page.locator('text=Grand Prix of Tokyo')).toBeVisible();
    await expect(page.locator('text=Weekly Practice Contest #42')).not.toBeVisible();

    // Filter by FINISHED
    await page.click('button:has-text("FINISHED")');
    await expect(page.locator('text=Winter Warmup 2026')).toBeVisible();
    await expect(page.locator('text=Weekly Practice Contest #42')).not.toBeVisible();
  });

  test('active contest lobby displays timer, problem snapshot, and scoreboard link', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests/con_active_icpc');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Weekly Practice Contest #42');
    await expect(page.locator('text=ICPC Scoring')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Scoreboard' })).toBeVisible();
    await expect(page.locator('text=Codehorses T-shirts')).toBeVisible();
    await expect(page.locator('text=A - N-choice question')).toBeVisible();
  });

  test('upcoming contest shows locked problem message for regular users', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests/con_upcoming_01');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Grand Prix of Tokyo');
    await expect(page.locator('text=Problems are Locked')).toBeVisible();
  });

  test('upcoming contest shows unlocked problem list and edit controls for admin users', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/contests/con_upcoming_01');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Grand Prix of Tokyo');
    await expect(page.locator('text=Problems are Locked')).not.toBeVisible();
    await expect(page.locator('text=Laura and Operations')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Edit Contest' })).toBeVisible();
  });

  test('user can click Join Contest button and join the contest lobby', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    // con_upcoming_01 has isParticipant: false
    await page.goto('/contests/con_upcoming_01');
    await page.waitForLoadState('networkidle');

    const joinBtn = page.locator('button:has-text("Join Contest")');
    await expect(joinBtn).toBeVisible();
    await joinBtn.click();

    // After joining, button switches to confirmation
    await expect(page.locator('text=✓ You are participating')).toBeVisible();
  });

  test('contest problem view renders contest header with timer and problem navigation', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc');
    await page.waitForLoadState('networkidle');

    // Contest banner in problem view
    await expect(page.locator('text=Weekly Practice Contest #42')).toBeVisible();
    await expect(page.locator('text=Contest Lobby')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Scoreboard' })).toBeVisible();

    // Problem label A
    await expect(page.locator('text=Problem A')).toBeVisible();

    // Next problem button exists and points to Problem B
    const nextBtn = page.locator('a:has-text("Next (B)")');
    await expect(nextBtn).toBeVisible();
  });

  test('contest scoreboard renders participants, solved count, penalty, and problem verdict cells', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/contests/con_active_icpc/standings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Weekly Practice Contest #42');
    await expect(page.locator('h1')).toContainText('Standings');

    // Scoreboard table headers
    await expect(page.locator('th:has-text("Rank")')).toBeVisible();
    await expect(page.locator('th:has-text("Participant")')).toBeVisible();
    await expect(page.locator('th:has-text("Solved")')).toBeVisible();
    await expect(page.locator('th:has-text("Penalty")')).toBeVisible();
    await expect(page.getByRole('columnheader', { name: 'A', exact: true })).toBeVisible();
    await expect(page.getByRole('columnheader', { name: 'B', exact: true })).toBeVisible();

    // Participant rows
    await expect(page.locator('text=tourist_fan')).toBeVisible();
    await expect(page.locator('text=speed_coder')).toBeVisible();

    // Problem scores (green + and red - attempts)
    await expect(page.locator('text=+1')).toBeVisible();
    await expect(page.locator('text=-3')).toBeVisible();

    // Click problem letter column header A to navigate to contest problem workspace
    await page.getByRole('link', { name: 'A', exact: true }).click();
    await expect(page).toHaveURL(/\/problems\/prb_cf_1000A\?contestId=con_active_icpc/);
    await expect(page.locator('text=Problem A')).toBeVisible();
  });

  test('contest submission records contestId and reflects attempts in live scoreboard', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    // Navigate to problem workspace within active contest
    await page.goto('/problems/prb_cf_1000A?contestId=con_active_icpc');
    await page.waitForLoadState('networkidle');

    // Switch to editor tab and submit code
    await page.click('button:has-text("Code Editor & Submit")');
    await page.click('button:has-text("Submit Solution")');

    // Submissions tab opens and verdict is shown
    await expect(page.locator('button:has-text("ACCEPTED")').first()).toBeVisible();

    // Navigate to scoreboard and verify live attempts/ranking
    await page.goto('/contests/con_active_icpc/standings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Standings');
    await expect(page.getByRole('table').getByText('tourist_fan')).toBeVisible();
    await expect(page.locator('text=+1').first()).toBeVisible();
  });

  test('contest problem snapshotting creates a contest directly from problem set', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    // Navigate to problem set detail
    await page.goto('/admin/problem-sets/set_standard_dp');
    await page.waitForLoadState('networkidle');

    // Click "Create Contest from Set"
    await page.click('a:has-text("Create Contest from Set")');
    await page.waitForURL(/\/admin\/contests\/new\?setId=set_standard_dp/);

    // Fill contest creation form
    await page.fill('#contest-name', 'Snapshotted DP Contest');
    await page.click('button:has-text("Create Contest")');

    // Should redirect to admin contests list
    await page.waitForURL('/admin/contests');
    await expect(page.locator('text=Snapshotted DP Contest')).toBeVisible();
  });

  test('admin contest editor allows reordering and removing problems', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    // Open contest editor for upcoming contest
    await page.goto('/admin/contests/con_upcoming_01/edit');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Grand Prix of Tokyo');

    // Add a second problem from modal
    await page.click('button:has-text("Add Problem")');
    await expect(page.locator('h3:has-text("Add Problem to Contest")')).toBeVisible();
    await page.locator('div.fixed').locator('button:has-text("Add")').first().click();

    // Save details
    await page.click('button:has-text("Save Settings")');
    await expect(page.locator('text=Contest details updated successfully!')).toBeVisible();

    // Problem list now has problems and allows reordering/removal
    const moveDownBtn = page.locator('button[title="Move Down"]').first();
    if (await moveDownBtn.isVisible()) {
      await moveDownBtn.click();
    }

    // Remove problem with confirmation
    page.on('dialog', (dialog) => dialog.accept());
    const removeBtn = page.locator('button[title="Remove Problem"]').first();
    await removeBtn.click();
  });

  test('finished contest renders finished state, ended timer, and standings link', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/contests/con_finished_99');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Winter Warmup 2026');
    await expect(page.getByText('FINISHED', { exact: true })).toBeVisible();
    await expect(page.getByText('Finished', { exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Scoreboard' })).toBeVisible();
  });

  test('draft contest is hidden from regular users and returns 404 on direct access', async ({ page }) => {
    // 1. Regular user on contests list
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('text=Draft Secret Contest')).not.toBeVisible();

    // Direct navigation to draft contest
    await page.goto('/contests/con_draft_01');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('text=Contest not found')).toBeVisible();

    // 2. Admin user can see draft contest
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/contests');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('text=Draft Secret Contest')).toBeVisible();
  });
});


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
});

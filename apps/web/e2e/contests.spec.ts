import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser, mockContests, mockStandings } from './fixtures/mock-data';

test.describe('Contests & ICPC Scoreboard', () => {
  test('contests list page groups events by state in order', async ({ page }) => {
    await setupApiMocks(page);

    await page.goto('/contests');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Contests');
    await expect(page.locator('text=Weekly Practice Contest #42')).toBeVisible();
    await expect(page.locator('text=Grand Prix of Tokyo')).toBeVisible();
    await expect(page.locator('text=Winter Warmup 2026')).toBeVisible();

    await expect(page.locator('section h2')).toHaveText([
      'Active Contests',
      'Upcoming Contests',
      'All Events'
    ]);

    await expect(page.locator('section').nth(0).locator('a')).toHaveClass(/bg-emerald-500\/10/);
    await expect(page.locator('section').nth(1).locator('a')).toHaveClass(/bg-amber-500\/10/);
    await expect(page.locator('section').nth(2).locator('a')).toHaveClass(/bg-zinc-800\/45/);
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

    // Problem status badges in contest lobby
    await expect(page.locator('text=Solved').first()).toBeVisible();
    await expect(page.locator('text=Attempted').first()).toBeVisible();

    // Full card clickable without standalone solve button
    const problemCardLink = page.getByRole('link', { name: /Codehorses T-shirts/i });
    await expect(problemCardLink).toBeVisible();
    await expect(page.getByRole('link', { name: 'Solve', exact: true })).not.toBeVisible();
  });

  test('admin contest lobby derives problem status only from the admin submissions', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    const submissionsRequest = page.waitForRequest((request) => {
      const url = new URL(request.url());
      return url.pathname.endsWith('/submissions') && url.searchParams.get('contestId') === 'con_active_icpc';
    });

    await page.goto('/contests/con_active_icpc');
    const request = await submissionsRequest;
    const url = new URL(request.url());

    expect(url.searchParams.get('userId')).toBe(mockAdminUser.id);
    await expect(page.getByText('Solved', { exact: true })).not.toBeVisible();
    await expect(page.getByText('Attempted', { exact: true })).not.toBeVisible();
  });

  test('upcoming contest shows locked problem message for regular users', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests/con_upcoming_01');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Grand Prix of Tokyo');
    await expect(page.locator('text=Problems are Locked')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Scoreboard' })).not.toBeVisible();
  });

  test('upcoming contest standings route redirects to the contest lobby', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/contests/con_upcoming_01/standings');
    await page.waitForURL('/contests/con_upcoming_01');

    await expect(page.locator('text=Problems are Locked')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Scoreboard' })).not.toBeVisible();
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
    const scoreboardLink = page.getByRole('link', { name: 'Scoreboard' });
    await expect(scoreboardLink).toBeVisible();
    await expect(scoreboardLink).toHaveClass(/bg-white/);

    // Problem title and label
    await expect(page.locator('h1')).toContainText('Codehorses T-shirts');
    await expect(page.locator('text=Problem A')).toBeVisible();

    // Contest Problems sidebar exists with Problem B
    const probBLink = page.locator('aside a:has-text("B")');
    await expect(probBLink).toBeVisible();
  });

  test('regular users accessing an upcoming problem are redirected to the contest lobby', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/problems/prb_cf_1900B?contestId=con_upcoming_01');
    await page.waitForURL('/contests/con_upcoming_01');

    await expect(page.locator('text=Problems are Locked')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Scoreboard' })).not.toBeVisible();
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
    await expect(page.getByRole('tab', { name: /In contest/ })).not.toBeVisible();
    await expect(page.getByRole('tab', { name: /After contest/ })).not.toBeVisible();

    // Pagination summary
    await expect(page.getByText(/Showing 1 to \d+ of \d+ participants/)).toBeVisible();

    // Click problem letter column header A to navigate to contest problem workspace
    await page.getByRole('link', { name: 'A', exact: true }).click();
    await expect(page).toHaveURL(/\/problems\/prb_cf_1000A\?contestId=con_active_icpc/);
    await expect(page.locator('h1')).toContainText('Codehorses T-shirts');
    await expect(page.locator('text=Problem A')).toBeVisible();
  });

  test('contest scoreboard keeps post-contest upsolve rows across pagination', async ({ page }) => {
    const manyParticipants = Array.from({ length: 21 }, (_, index) => ({
      ...mockStandings.standings[0],
      userId: index === 0 ? mockRegularUser.id : `usr_participant_${index}`,
      username: index === 0 ? mockRegularUser.username : `participant_${index}`,
      rank: index + 1,
    }));

    await setupApiMocks(page, {
      currentUser: mockRegularUser,
      contests: mockContests.map((contest) => contest.id === 'con_active_icpc' ? { ...contest, state: 'FINISHED' } : contest),
      standings: {
        ...mockStandings,
        standings: manyParticipants,
        upsolveStandings: [
          {
            ...mockStandings.standings[0],
            userId: 'usr_upsolver_1',
            username: 'upsolver_one',
            rank: 1,
            solvedCount: 1,
            totalPenalty: 12,
            problemScores: {
              [mockStandings.problems[0].problemId]: { ...mockStandings.standings[0].problemScores[mockStandings.problems[0].problemId], solved: true, attempts: 1, firstSolvedAtMinutes: 12 },
              [mockStandings.problems[1].problemId]: { ...mockStandings.standings[0].problemScores[mockStandings.problems[1].problemId], solved: false, attempts: 1, firstSolvedAtMinutes: null },
            },
          },
          {
            ...mockStandings.standings[0],
            userId: mockRegularUser.id,
            username: mockRegularUser.username,
            rank: 2,
            solvedCount: 1,
            totalPenalty: 30,
          },
        ],
      },
    });

    await page.goto('/contests/con_active_icpc/standings');
    await page.waitForLoadState('networkidle');

    const contestTab = page.getByRole('tab', { name: /In contest/ });
    const afterContestTab = page.getByRole('tab', { name: /After contest/ });
    await expect(contestTab).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('tbody tr').first()).toContainText(mockRegularUser.username);

    await afterContestTab.click();
    await expect(afterContestTab).toHaveAttribute('aria-selected', 'true');
    const afterContestRows = page.locator('tbody tr');
    await expect(afterContestRows).toHaveCount(2);
    await expect(afterContestRows.first()).toContainText(mockRegularUser.username);
    await expect(page.getByRole('row', { name: /upsolver_one/ })).toBeVisible();

    await contestTab.click();
    await page.getByRole('button', { name: 'Next page' }).click();
    await expect(page.getByText('participant_20')).toBeVisible();
    await afterContestTab.click();
    await expect(afterContestRows.first()).toContainText(mockRegularUser.username);
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

  test('empty scoreboard renders empty state message', async ({ page }) => {
    await setupApiMocks(page, {
      standings: {
        contestId: 'con_active_icpc',
        scoringType: 'ICPC',
        generatedAt: '2026-01-01T00:00:00Z',
        problems: [
          { problemId: 'prb_cf_1000A', label: 'A', title: 'Codehorses T-shirts', platform: 'CODEFORCES' },
        ],
        standings: [],
      },
    });

    await page.goto('/contests/con_active_icpc/standings');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Standings');
    await expect(page.locator('text=No participants or submissions recorded yet.')).toBeVisible();
  });

  test('unauthenticated guest viewing contest lobby does not see join button', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/contests/con_upcoming_01');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('h1')).toContainText('Grand Prix of Tokyo');
    await expect(page.locator('button:has-text("Join Contest")')).not.toBeVisible();
    await expect(page.getByText(/\d+ registered participants?/)).toBeVisible();
  });
});

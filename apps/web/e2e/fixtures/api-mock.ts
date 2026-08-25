import type { Page, Route } from '@playwright/test';
import type { User, Problem, Contest, Submission, ProblemSet, Standings, PlatformIntegration } from '@cpbridge/contracts';
import {
  mockAdminUser,
  mockRegularUser,
  mockProblems,
  mockStatement,
  mockProblemSets,
  mockContests,
  mockStandings,
  mockSubmissions,
  mockAdminStats,
  mockUsersList,
} from './mock-data';

const LATEST_EXTENSION_VERSION = '1.0.7';

export interface SetupApiMocksOptions {
  currentUser?: User | null;
  users?: User[];
  problems?: Problem[];
  contests?: Contest[];
  problemSets?: ProblemSet[];
  submissions?: Submission[];
  standings?: Standings;
  disableExtension?: boolean;
  submissionError?: string;
  extensionPlatforms?: Record<string, { loggedIn: boolean; username?: string }>;
  extensionVersion?: string;
  extensionPollVerdict?: string;
  recoveredSubmissions?: unknown[];
  integrations?: PlatformIntegration[];
}

export async function setupApiMocks(page: Page, options: SetupApiMocksOptions = {}) {
  let currentUser = options.currentUser !== undefined ? options.currentUser : null;
  const usersList = options.users ? [...options.users] : [...mockUsersList];
  let problemsList = options.problems ? [...options.problems] : [...mockProblems];
  const contestsList = options.contests ? [...options.contests] : [...mockContests];
  const problemSetsList = options.problemSets ? [...options.problemSets] : [...mockProblemSets];
  const submissionsList = options.submissions ? [...options.submissions] : [...mockSubmissions];
  const integrationsList: PlatformIntegration[] = options.integrations
    ? options.integrations.map((integration) => ({ ...integration }))
    : [];
  const currentStandings = options.standings ? JSON.parse(JSON.stringify(options.standings)) : JSON.parse(JSON.stringify(mockStandings));

  // Provide mock Chrome Extension postMessage responder unless disabled
  if (!options.disableExtension) {
    await page.addInitScript((opts) => {
      window.addEventListener('message', (event) => {
        if (event.data && event.data.source === 'CP_HUB_WEB') {
          const { id, payload } = event.data;
          if (payload?.type === 'PING') {
            window.postMessage({
              source: 'CP_HUB_EXTENSION',
              id,
              payload: {
                type: 'PONG',
                version: opts.extensionVersion,
                platforms: opts.extensionPlatforms || {
                  CODEFORCES: { loggedIn: true, username: 'tourist_fan' },
                  ATCODER: { loggedIn: true, username: 'tourist_fan' },
                },
              },
            }, '*');
          } else if (payload?.type === 'SUBMIT') {
            if (opts.submissionError) {
              window.postMessage({
                source: 'CP_HUB_EXTENSION',
                id,
                payload: {
                  type: 'SUBMISSION_FAILED',
                  submissionId: payload.submissionId,
                  error: 'SUBMISSION_FAILED',
                  message: opts.submissionError,
                },
              }, '*');
            } else {
              window.postMessage({
                source: 'CP_HUB_EXTENSION',
                id,
                payload: {
                  type: 'SUBMISSION_CREATED',
                  submissionId: payload.submissionId,
                  externalSubmissionId: `cf_${Date.now()}`,
                },
              }, '*');
            }
          } else if (payload?.type === 'POLL_STATUS') {
            window.postMessage({
              source: 'CP_HUB_EXTENSION',
              id,
              payload: {
                type: 'POLL_STATUS_RESULT',
                externalSubmissionId: payload.externalSubmissionId,
                status: opts.extensionPollVerdict || 'ACCEPTED',
              },
            }, '*');
          } else if (payload?.type === 'RECOVER_SUBMISSIONS') {
            window.postMessage({
              source: 'CP_HUB_EXTENSION',
              id,
              payload: {
                type: 'RECOVER_SUBMISSIONS_RESULT',
                submissions: opts.recoveredSubmissions || [],
              },
            }, '*');
          }
        }
      });
    }, {
      submissionError: options.submissionError,
      extensionPlatforms: options.extensionPlatforms,
      extensionVersion: options.extensionVersion || LATEST_EXTENSION_VERSION,
      extensionPollVerdict: options.extensionPollVerdict,
      recoveredSubmissions: options.recoveredSubmissions,
    });
  }

  // Helper to reply with JSON
  const jsonResponse = (route: Route, data: unknown, status = 200) => {
    return route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(data),
    });
  };

  await page.route('/api/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace(/^\/api/, '');
    const method = route.request().method();

    // 1. Auth routes
    if (path === '/auth/me' && method === 'GET') {
      if (currentUser) {
        return jsonResponse(route, { user: currentUser });
      }
      return jsonResponse(route, { error: 'Unauthorized' }, 401);
    }

    if (path === '/auth/login' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      if (body.password === 'wrongpassword') {
        return jsonResponse(route, { error: 'Invalid username or password' }, 401);
      }
      const user = body.emailOrUsername?.includes('admin') ? mockAdminUser : mockRegularUser;
      currentUser = user;
      return jsonResponse(route, { user, token: `mock_jwt_token_${user.id}` });
    }

    if (path === '/auth/register' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      if (body.password && body.password.length < 6) {
        return jsonResponse(route, { error: 'Password must be at least 6 characters' }, 400);
      }
      const newUser: User = {
        id: `usr_${Date.now()}`,
        email: body.email || 'new@example.com',
        username: body.username || 'newuser',
        role: 'USER',
        isActive: true,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      currentUser = newUser;
      return jsonResponse(route, { user: newUser, token: `mock_jwt_token_${newUser.id}` });
    }

    if (path === '/integrations' && method === 'GET') {
      return jsonResponse(route, integrationsList);
    }

    const integrationMatch = path.match(/^\/integrations\/([^/]+)$/);
    if (integrationMatch && method === 'PUT') {
      const platform = integrationMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const integration: PlatformIntegration = {
        platform: platform as PlatformIntegration['platform'],
        externalUsername: body.externalUsername,
        connectionStatus: body.connectionStatus,
        updatedAt: new Date().toISOString(),
      };
      const index = integrationsList.findIndex((item) => item.platform === platform);
      if (index === -1) integrationsList.push(integration);
      else integrationsList[index] = integration;
      return jsonResponse(route, integration);
    }

    // 2. Problems routes
    if ((path === '/problems' || path === '/admin/problems') && method === 'GET') {
      if (path === '/problems' && currentUser?.role !== 'ADMIN') {
        return jsonResponse(route, { error: 'only administrators can browse the global problem library' }, 403);
      }
      const query = url.searchParams.get('query')?.toLowerCase();
      const platform = url.searchParams.get('platform');
      let list = [...problemsList];
      if (platform) {
        list = list.filter((p) => p.platform === platform);
      }
      if (query) {
        list = list.filter(
          (p) =>
            p.title.toLowerCase().includes(query) ||
            p.externalId.toLowerCase().includes(query)
        );
      }
      return jsonResponse(route, { problems: list, total: list.length });
    }

    if (path === '/admin/problems/import' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const imported: Problem = {
        id: `prb_imp_${Date.now()}`,
        platform: body.url?.includes('atcoder') ? 'ATCODER' : 'CODEFORCES',
        externalId: '1234A',
        title: 'Imported Test Problem',
        url: body.url || 'https://codeforces.com/problemset/problem/1234/A',
        difficulty: 1200,
        tags: ['implementation'],
        metadata: {},
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      problemsList.unshift(imported);
      return jsonResponse(route, imported);
    }

    if (path === '/admin/problems' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const created: Problem = {
        id: `prb_${Date.now()}`,
        platform: body.platform || 'CODEFORCES',
        externalId: body.externalId || 'custom_01',
        title: body.title || 'New Custom Problem',
        url: body.url || '',
        difficulty: body.difficulty || 1000,
        tags: body.tags || [],
        metadata: {},
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      problemsList.unshift(created);
      return jsonResponse(route, created);
    }

    const problemStatementMatch = path.match(/^\/problems\/([^/]+)\/statement$/);
    if (problemStatementMatch && method === 'GET') {
      const pId = problemStatementMatch[1];
      const contestId = url.searchParams.get('contestId');
      if (currentUser?.role !== 'ADMIN' && !contestId) {
        return jsonResponse(route, { error: 'contestId parameter is required to access statement' }, 400);
      }
      if (pId === 'prb_non_existent') {
        return jsonResponse(route, { error: 'Problem not found' }, 404);
      }
      return jsonResponse(route, mockStatement);
    }

    const problemDetailMatch = path.match(/^\/(?:admin\/)?problems\/([^/]+)$/);
    if (problemDetailMatch && method === 'GET') {
      const pId = problemDetailMatch[1];
      const contestId = url.searchParams.get('contestId');
      if (!path.startsWith('/admin') && currentUser?.role !== 'ADMIN' && !contestId) {
        return jsonResponse(route, { error: 'contestId parameter is required to access problem' }, 400);
      }
      const found = problemsList.find((p) => p.id === pId);
      if (!found) {
        return jsonResponse(route, { error: 'Problem not found' }, 404);
      }
      return jsonResponse(route, found);
    }

    if (problemDetailMatch && method === 'PATCH') {
      const pId = problemDetailMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const idx = problemsList.findIndex((p) => p.id === pId);
      if (idx !== -1) {
        problemsList[idx] = { ...problemsList[idx], ...body };
        return jsonResponse(route, problemsList[idx]);
      }
      return jsonResponse(route, { error: 'Not found' }, 404);
    }

    if (problemDetailMatch && method === 'DELETE') {
      const pId = problemDetailMatch[1];
      if (pId === 'prb_in_use') {
        return jsonResponse(route, { error: 'PROBLEM_IN_USE', message: 'PROBLEM_IN_USE' }, 400);
      }
      problemsList = problemsList.filter((p) => p.id !== pId);
      return jsonResponse(route, { success: true });
    }

    // 3. Problem Sets routes
    if ((path === '/problem-sets' || path === '/admin/problem-sets') && method === 'GET') {
      return jsonResponse(route, problemSetsList);
    }

    if (path === '/admin/problem-sets' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const newSet = {
        id: `set_${Date.now()}`,
        ownerId: currentUser?.id || 'usr_adm_999',
        name: body.name,
        description: body.description || '',
        visibility: body.visibility || 'PUBLIC',
        problemCount: 0,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        items: [],
      };
      problemSetsList.push(newSet as ProblemSet);
      return jsonResponse(route, newSet);
    }

    const problemSetDeleteProblemMatch = path.match(/^\/admin\/problem-sets\/([^/]+)\/problems\/([^/]+)$/);
    if (problemSetDeleteProblemMatch && method === 'DELETE') {
      const [, setId, pId] = problemSetDeleteProblemMatch;
      const set = problemSetsList.find((s) => s.id === setId);
      if (set && set.items) {
        set.items = set.items.filter((it) => it.problemId !== pId);
        set.problemCount = set.items.length;
      }
      return jsonResponse(route, { success: true });
    }

    const problemSetOrderMatch = path.match(/^\/admin\/problem-sets\/([^/]+)\/order$/);
    if (problemSetOrderMatch && method === 'PATCH') {
      return jsonResponse(route, { success: true });
    }

    const problemSetDetailMatch = path.match(/^\/(?:admin\/)?problem-sets\/([^/]+)$/);
    if (problemSetDetailMatch && method === 'GET') {
      const setId = problemSetDetailMatch[1];
      const found = problemSetsList.find((s) => s.id === setId) || problemSetsList[0];
      return jsonResponse(route, found);
    }

    if (problemSetDetailMatch && method === 'PATCH') {
      const setId = problemSetDetailMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const idx = problemSetsList.findIndex((s) => s.id === setId);
      if (idx !== -1) {
        problemSetsList[idx] = { ...problemSetsList[idx], ...body };
        return jsonResponse(route, problemSetsList[idx]);
      }
      return jsonResponse(route, { error: 'Not found' }, 404);
    }

    // 4. Contests routes
    if ((path === '/contests' || path === '/admin/contests') && method === 'GET') {
      let list = [...contestsList];
      if (!path.startsWith('/admin') && currentUser?.role !== 'ADMIN') {
        list = list.filter((c) => c.publicationStatus !== 'DRAFT');
      }
      return jsonResponse(route, list);
    }

    const contestStandingsMatch = path.match(/^\/contests\/([^/]+)\/standings$/);
    if (contestStandingsMatch && method === 'GET') {
      return jsonResponse(route, currentStandings);
    }

    const contestJoinMatch = path.match(/^\/contests\/([^/]+)\/join$/);
    if (contestJoinMatch && method === 'POST') {
      const cId = contestJoinMatch[1];
      const idx = contestsList.findIndex((c) => c.id === cId);
      if (idx !== -1) {
        contestsList[idx] = {
          ...contestsList[idx],
          isParticipant: true,
          participantCount: (contestsList[idx].participantCount || 0) + 1,
        };
      }
      return jsonResponse(route, { success: true });
    }

    const contestDetailMatch = path.match(/^\/(?:admin\/)?contests\/([^/]+)$/);
    if (contestDetailMatch && method === 'GET') {
      const cId = contestDetailMatch[1];
      const found = contestsList.find((c) => c.id === cId);
      if (!found) {
        return jsonResponse(route, { error: 'Contest not found' }, 404);
      }
      if (found.publicationStatus === 'DRAFT' && currentUser?.role !== 'ADMIN') {
        return jsonResponse(route, { error: 'Contest not found' }, 404);
      }
      return jsonResponse(route, found);
    }

    if (contestDetailMatch && method === 'PATCH') {
      const cId = contestDetailMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const idx = contestsList.findIndex((c) => c.id === cId);
      if (idx !== -1) {
        contestsList[idx] = { ...contestsList[idx], ...body };
        return jsonResponse(route, contestsList[idx]);
      }
      return jsonResponse(route, { error: 'Not found' }, 404);
    }

    // Contest problem management
    const contestAddProblemMatch = path.match(/^\/admin\/contests\/([^/]+)\/problems$/);
    if (contestAddProblemMatch && method === 'POST') {
      const cId = contestAddProblemMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const contest = contestsList.find((c) => c.id === cId);
      if (contest) {
        const problemToAdd = problemsList.find((p) => p.id === body.problemId) || problemsList[0];
        contest.problems = contest.problems || [];
        contest.problems.push({
          contestId: cId,
          problemId: problemToAdd.id,
          position: contest.problems.length + 1,
          label: String.fromCharCode(65 + contest.problems.length),
          points: body.points || 100,
          problem: problemToAdd,
        });
      }
      return jsonResponse(route, { success: true });
    }

    const contestProblemOrderMatch = path.match(/^\/admin\/contests\/([^/]+)\/problem-order$/);
    if (contestProblemOrderMatch && method === 'PATCH') {
      const cId = contestProblemOrderMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const contest = contestsList.find((c) => c.id === cId);
      if (contest && contest.problems && Array.isArray(body.problemIds)) {
        const reordered: NonNullable<Contest['problems']> = [];
        body.problemIds.forEach((pid: string, idx: number) => {
          const cp = contest.problems?.find((p) => p.problemId === pid);
          if (cp) {
            reordered.push({
              ...cp,
              position: idx + 1,
              label: String.fromCharCode(65 + idx),
            });
          }
        });
        contest.problems = reordered;
      }
      return jsonResponse(route, { success: true });
    }

    const contestDeleteProblemMatch = path.match(/^\/admin\/contests\/([^/]+)\/problems\/([^/]+)$/);
    if (contestDeleteProblemMatch && method === 'DELETE') {
      const [, cId, pId] = contestDeleteProblemMatch;
      const contest = contestsList.find((c) => c.id === cId);
      if (contest && contest.problems) {
        contest.problems = contest.problems.filter((cp) => cp.problemId !== pId);
        contest.problems.forEach((cp, idx) => {
          cp.position = idx + 1;
          cp.label = String.fromCharCode(65 + idx);
        });
      }
      return jsonResponse(route, { success: true });
    }

    if (path === '/admin/contests' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      let snapshottedProblems: NonNullable<Contest['problems']> = [];
      if (body.problemSetId) {
        const sourceSet = problemSetsList.find((s) => s.id === body.problemSetId);
        if (sourceSet && sourceSet.items) {
          snapshottedProblems = sourceSet.items.map((item, idx) => ({
            contestId: `con_${Date.now()}`,
            problemId: item.problemId,
            position: idx + 1,
            label: String.fromCharCode(65 + idx),
            points: 100,
            problem: item.problem,
          }));
        }
      }

      const createdContest: Contest = {
        id: `con_${Date.now()}`,
        ownerId: currentUser?.id || 'usr_adm_999',
        ownerUsername: currentUser?.username || 'root_admin',
        name: body.name || 'New Contest',
        description: body.description || '',
        startAt: body.startAt,
        endAt: body.endAt,
        visibility: body.visibility || 'PUBLIC',
        scoringType: body.scoringType || 'ICPC',
        publicationStatus: body.publicationStatus || 'PUBLISHED',
        state: 'UPCOMING',
        problems: snapshottedProblems,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      contestsList.push(createdContest);
      return jsonResponse(route, createdContest);
    }

    // 5. Submissions routes
    if (path === '/submissions' && method === 'GET') {
      const pId = url.searchParams.get('problemId');
      const cId = url.searchParams.get('contestId');
      const uId = url.searchParams.get('userId');
      const myOnly = url.searchParams.get('myOnly');
      let list = [...submissionsList];
      if (pId) list = list.filter((s) => s.problemId === pId);
      if (cId) list = list.filter((s) => s.contestId === cId);
      if (uId) list = list.filter((s) => s.userId === uId);
      if (myOnly === 'true' && currentUser) {
        list = list.filter((s) => s.userId === currentUser?.id);
      }
      // If user is not admin, strip sourceUrl as per project invariants
      if (currentUser?.role !== 'ADMIN') {
        list = list.map((s) => ({ ...s, sourceUrl: undefined }));
      }
      return jsonResponse(route, list);
    }

    if (path === '/submissions' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const newSub: Submission = {
        id: `sub_${Date.now()}`,
        userId: currentUser?.id || 'usr_anonymous',
        username: currentUser?.username || 'anonymous',
        problemId: body.problemId,
        contestId: body.contestId || null,
        platform: 'CODEFORCES',
        externalSubmissionId: `cf_${Date.now()}`,
        language: body.language,
        sourceCode: body.sourceCode || '// Solution submitted in test',
        status: (options.extensionPollVerdict && options.extensionPollVerdict !== 'ACCEPTED' ? options.extensionPollVerdict : 'ACCEPTED') as Submission['status'],
        submittedAt: new Date().toISOString(),
        judgedAt: new Date().toISOString(),
        problemTitle: 'Codehorses T-shirts',
        metadata: {
          executionTimeMs: 30,
          memoryBytes: 2048576,
        },
      };
      submissionsList.unshift(newSub);

      // If contest submission, update mock standings
      if (body.contestId === 'con_active_icpc' && currentStandings) {
        const userRow = currentStandings.standings.find((s: { userId: string; problemScores: Record<string, { attempts: number }> }) => s.userId === (currentUser?.id || 'usr_reg_123'));
        if (userRow && userRow.problemScores[body.problemId]) {
          userRow.problemScores[body.problemId].attempts += 1;
        }
      }

      return jsonResponse(route, newSub);
    }

    const submissionDetailMatch = path.match(/^\/submissions\/([^/]+)$/);
    if (submissionDetailMatch && method === 'GET') {
      const subId = submissionDetailMatch[1];
      const found = submissionsList.find((s) => s.id === subId) || submissionsList[0];
      const payload = { ...found };
      if (currentUser?.role !== 'ADMIN') {
        payload.sourceUrl = undefined;
      }
      return jsonResponse(route, payload);
    }

    const submissionSyncMatch = path.match(/^\/submissions\/([^/]+)\/sync$/);
    if (submissionSyncMatch && method === 'POST') {
      const subId = submissionSyncMatch[1];
      const found = submissionsList.find((s) => s.id === subId) || submissionsList[0];
      if (options.extensionPollVerdict) {
        found.status = options.extensionPollVerdict as Submission['status'];
      } else {
        found.status = 'ACCEPTED';
      }
      return jsonResponse(route, found);
    }

    const submissionDispatchedMatch = path.match(/^\/submissions\/([^/]+)\/dispatched$/);
    if (submissionDispatchedMatch && (method === 'POST' || method === 'PATCH')) {
      return jsonResponse(route, { success: true });
    }

    const submissionResultMatch = path.match(/^\/submissions\/([^/]+)\/result$/);
    if (submissionResultMatch && (method === 'POST' || method === 'PATCH')) {
      const subId = submissionResultMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const found = submissionsList.find((s) => s.id === subId);
      if (found) {
        found.status = body.status;
        found.metadata = body.metadata;
      }
      return jsonResponse(route, { success: true });
    }

    // 6. Admin stats & users
    if (path === '/admin/stats' && method === 'GET') {
      return jsonResponse(route, mockAdminStats);
    }

    const adminUserDetailMatch = path.match(/^\/admin\/users\/([^/]+)$/);
    if (adminUserDetailMatch && method === 'GET') {
      const uId = adminUserDetailMatch[1];
      const found = usersList.find((u) => u.id === uId);
      if (found) {
        return jsonResponse(route, found);
      }
      return jsonResponse(route, { error: 'User not found' }, 404);
    }

    if (path.startsWith('/admin/users') && method === 'GET') {
      const search = url.searchParams.get('search')?.toLowerCase();
      let list = [...usersList];
      if (search) {
        list = list.filter(
          (u) =>
            u.username.toLowerCase().includes(search) ||
            u.email.toLowerCase().includes(search)
        );
      }
      return jsonResponse(route, list);
    }

    const userRoleMatch = path.match(/^\/admin\/users\/([^/]+)\/role$/);
    if (userRoleMatch && method === 'PATCH') {
      const uId = userRoleMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const uIndex = usersList.findIndex((u) => u.id === uId);
      if (uIndex !== -1) {
        if (usersList[uIndex].id === mockAdminUser.id && body.role === 'USER') {
          return jsonResponse(route, { error: 'LAST_ADMIN', message: 'LAST_ADMIN' }, 400);
        }
        usersList[uIndex] = { ...usersList[uIndex], role: body.role };
      }
      return jsonResponse(route, { success: true });
    }

    const userStatusMatch = path.match(/^\/admin\/users\/([^/]+)\/status$/);
    if (userStatusMatch && method === 'PATCH') {
      const uId = userStatusMatch[1];
      const body = JSON.parse(route.request().postData() || '{}');
      const uIndex = usersList.findIndex((u) => u.id === uId);
      if (uIndex !== -1) {
        if (usersList[uIndex].id === mockAdminUser.id && body.isActive === false) {
          return jsonResponse(route, { error: 'LAST_ADMIN', message: 'LAST_ADMIN' }, 400);
        }
        usersList[uIndex] = { ...usersList[uIndex], isActive: body.isActive };
      }
      return jsonResponse(route, { success: true });
    }

    // Default fallback
    return jsonResponse(route, {});
  });
}

export async function loginAs(page: Page, user: User) {
  await page.addInitScript((userData) => {
    localStorage.setItem('cp_token', `mock_jwt_token_${userData.id}`);
  }, user);
}

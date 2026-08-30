const CODEFORCES_SUBMISSION_DETECTION_ATTEMPTS = 12;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export interface CodeforcesProblemRef {
  contestId: string;
  problemIndex: string;
}

export function parseCodeforcesExternalId(externalId: string): CodeforcesProblemRef | undefined {
  const parts = externalId.trim().split('/');
  if (parts.length === 2 && /^\d+$/.test(parts[0]) && parts[1].trim()) {
    return { contestId: parts[0], problemIndex: parts[1].trim().toUpperCase() };
  }
  if (parts.length === 3 && parts[0].toLowerCase() === 'gym' && /^\d+$/.test(parts[1]) && parts[2].trim()) {
    return { contestId: `gym/${parts[1]}`, problemIndex: parts[2].trim().toUpperCase() };
  }
  return undefined;
}

function codeforcesContestPath(contestId: string): string {
  return contestId.startsWith('gym/') ? contestId : `contest/${contestId}`;
}

function codeforcesNumericContestId(contestId: string): string {
  return contestId.startsWith('gym/') ? contestId.slice('gym/'.length) : contestId;
}

function extractSubmissionIds(html: string, problemIndex?: string): string[] {
  const rows = [...html.matchAll(/<tr\b[^>]*>[\s\S]*?<\/tr>/gi)].map((match) => match[0]);
  const candidates = rows.length > 0 ? rows : [html];
  const escapedIndex = problemIndex?.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const problemPattern = escapedIndex ? new RegExp(`/problem/${escapedIndex}(?:["'/?]|$)`, 'i') : undefined;
  const ids: string[] = [];
  for (const row of candidates) {
    if (problemPattern && !problemPattern.test(row)) continue;
    for (const match of row.matchAll(/(?:data-submission-id=["']|\/submission\/)(\d+)/gi)) {
      if (!ids.includes(match[1])) ids.push(match[1]);
    }
  }
  return ids;
}

/**
 * Codeforces' `/my` page is scoped to the authenticated browser account.
 * Never identify a submission from the public contest-status page: its first
 * row can belong to an unrelated competitor.
 */
async function snapshotMyCodeforcesSubmissionIds(contestId: string, problemIndex: string): Promise<Set<string> | undefined> {
  try {
    const response = await fetch(`https://codeforces.com/${codeforcesContestPath(contestId)}/my?cpbridge_ts=${Date.now()}`, {
      method: 'GET',
      credentials: 'include',
      cache: 'no-store'
    });
    if (!response.ok) return undefined;
    return new Set(extractSubmissionIds(await response.text(), problemIndex));
  } catch {
    return undefined;
  }
}

function singleNewSubmissionId(knownIds: Set<string>, currentIds: Set<string>): string | undefined {
  const newIds = [...currentIds].filter((id) => !knownIds.has(id));
  // Several new rows mean another submission happened concurrently. Failing
  // the handoff is preferable to attaching a verdict to the wrong solution.
  return newIds.length === 1 ? newIds[0] : undefined;
}

async function waitForCodeforcesSubmission(contestId: string, problemIndex: string, knownIds: Set<string>): Promise<string | undefined> {
  for (let attempt = 0; attempt < CODEFORCES_SUBMISSION_DETECTION_ATTEMPTS; attempt++) {
    const currentIds = await snapshotMyCodeforcesSubmissionIds(contestId, problemIndex);
    const id = currentIds && singleNewSubmissionId(knownIds, currentIds);
    if (id) return id;
    if (attempt < CODEFORCES_SUBMISSION_DETECTION_ATTEMPTS - 1) await sleep(500);
  }
  return undefined;
}

function extractCodeforcesUsername(html: string): string | undefined {
  const patterns = [
    /href=["']\/profile\/([^"'/?#]+)["']/i,
    /handle\s*=\s*["']([^"']+)["']/i,
    /data-user=["']([^"']+)["']/i
  ];
  for (const pattern of patterns) {
    const username = html.match(pattern)?.[1]?.trim();
    if (username) return username;
  }
  return undefined;
}

export async function checkCodeforcesSession(): Promise<{ loggedIn: boolean; username?: string }> {
  try {
    // The settings page is account-scoped, so its profile link identifies the
    // active browser session without mistaking another user's public link for
    // the signed-in handle.
    const res = await fetch(`https://codeforces.com/settings/general?cpbridge_ts=${Date.now()}`, {
      method: 'GET',
      credentials: 'include',
      cache: 'no-store'
    });
    if (!res.ok || new URL(res.url).pathname.startsWith('/enter')) {
      return { loggedIn: false };
    }
    const username = extractCodeforcesUsername(await res.text());
    return username ? { loggedIn: true, username } : { loggedIn: false };
  } catch {
    return { loggedIn: false };
  }
}

export async function detectManualCodeforcesSubmission(
  contestId: string,
  problemIndex: string,
  knownSubmissionIds: Iterable<string>
): Promise<string | undefined> {
  return waitForCodeforcesSubmission(contestId, problemIndex, new Set(knownSubmissionIds));
}

export async function snapshotCodeforcesSubmissionIds(
  contestId: string,
  problemIndex: string
): Promise<string[] | undefined> {
  const ids = await snapshotMyCodeforcesSubmissionIds(contestId, problemIndex);
  return ids ? [...ids] : undefined;
}

export async function pollCodeforcesStatus(contestId: string, externalSubmissionId: string): Promise<{ status: 'JUDGING' | 'ACCEPTED' | 'WRONG_ANSWER' | 'TIME_LIMIT' | 'MEMORY_LIMIT' | 'RUNTIME_ERROR' | 'COMPILE_ERROR' | 'FAILED' }> {
  if (externalSubmissionId.startsWith('cf_')) return { status: 'FAILED' };
  const numericContestId = codeforcesNumericContestId(contestId);
  try {
    const res = await fetch(`https://codeforces.com/api/contest.status?contestId=${numericContestId}&from=1&count=1000&cpbridge_ts=${Date.now()}`, { method: 'GET', credentials: 'include', cache: 'no-store' });
    if (res.ok) {
      const data = await res.json();
      const sub = data.status === 'OK' && Array.isArray(data.result) ? data.result.find((item: { id?: number | string; verdict?: string }) => String(item.id) === String(externalSubmissionId)) : undefined;
      if (sub?.verdict) {
        const verdicts: Record<string, 'JUDGING' | 'ACCEPTED' | 'WRONG_ANSWER' | 'TIME_LIMIT' | 'MEMORY_LIMIT' | 'RUNTIME_ERROR' | 'COMPILE_ERROR' | 'FAILED'> = {
          OK: 'ACCEPTED', WRONG_ANSWER: 'WRONG_ANSWER', TIME_LIMIT_EXCEEDED: 'TIME_LIMIT', MEMORY_LIMIT_EXCEEDED: 'MEMORY_LIMIT',
          COMPILATION_ERROR: 'COMPILE_ERROR', RUNTIME_ERROR: 'RUNTIME_ERROR', CHALLENGED: 'FAILED', SKIPPED: 'FAILED', FAILED: 'FAILED',
          SECURITY_VIOLATED: 'FAILED', CRASHED: 'FAILED'
        };
        return { status: verdicts[sub.verdict] || 'JUDGING' };
      }
    }
  } catch {}

  try {
    const urls = contestId.startsWith('gym/')
      ? [`https://codeforces.com/${codeforcesContestPath(contestId)}/submission/${externalSubmissionId}`]
      : [`https://codeforces.com/${codeforcesContestPath(contestId)}/submission/${externalSubmissionId}`, `https://codeforces.com/problemset/submission/${contestId}/${externalSubmissionId}`];
    for (const url of urls) {
      const res = await fetch(url, { method: 'GET', credentials: 'include' });
      if (res.status === 404 || !res.ok) continue;
      const html = await res.text();
      if (html.includes('verdict-accepted') || html.includes('>Accepted<')) return { status: 'ACCEPTED' };
      if (html.includes('Compilation error') || html.includes('verdict-compilation-error')) return { status: 'COMPILE_ERROR' };
      if (html.includes('Time limit exceeded')) return { status: 'TIME_LIMIT' };
      if (html.includes('Memory limit exceeded')) return { status: 'MEMORY_LIMIT' };
      if (html.includes('Runtime error')) return { status: 'RUNTIME_ERROR' };
      if (html.includes('Wrong answer') || html.includes('verdict-rejected')) return { status: 'WRONG_ANSWER' };
      if (html.includes('verdict-waiting') || html.includes('In queue') || html.includes('Running on test')) return { status: 'JUDGING' };
    }
  } catch {}
  return { status: 'JUDGING' };
}

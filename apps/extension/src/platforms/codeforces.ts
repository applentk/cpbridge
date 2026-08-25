import type { LanguageId } from '@cpbridge/contracts';

// Fallbacks only: Codeforces changes compiler IDs periodically. The submit form
// is the source of truth whenever it is available.
const CF_LANGUAGE_MAP: Record<LanguageId, string> = {
  cpp23: '91', python3: '70', java21: '87'
};

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

function cleanOptionText(value: string): string {
  return htmlText(value).toLowerCase();
}

function htmlText(value: string): string {
  return value
    .replace(/<[^>]+>/g, ' ')
    .replace(/(?:&nbsp;?|&#160;?|&#x0*a0;?)(?=\s|$|<)/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#(?:39|x0*27);/gi, "'")
    .replace(/\s+/g, ' ')
    .trim();
}

function getOptionValue(html: string, language: LanguageId): string | undefined {
  const wanted: Record<LanguageId, RegExp[]> = {
    cpp23: [/c\+\+\s*23/, /gnu c\+\+\s*23/, /c\+\+\s*20/],
    python3: [/python.*3/, /pypy.*3/],
    // Some contests expose only an older Java runtime. Prefer Java 21, then
    // fall back to another real Java compiler without matching JavaScript.
    java21: [/^java\s*21\b/, /^java\s*17\b/, /^java(?:\s*\d+|\s*\()/]
  };
  const options = [...html.matchAll(/<option\b[^>]*value=["']([^"']+)["'][^>]*>([\s\S]*?)<\/option>/gi)];
  for (const pattern of wanted[language]) {
    const option = options.find((match) => pattern.test(cleanOptionText(match[2])));
    if (option) return option[1];
  }
  return undefined;
}

function getInputValue(html: string, name: string): string | undefined {
  const escapedName = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return html.match(new RegExp(`<input\\b[^>]*name=["']${escapedName}["'][^>]*value=["']([^"']*)["']`, 'i'))?.[1];
}

function getCsrfToken(html: string): string | undefined {
  return html.match(/csrf_token\s*=\s*["']([^"']+)["']/i)?.[1]
    || html.match(/name=["']csrf_token["'][^>]*value=["']([^"']+)["']/i)?.[1]
    || html.match(/data-csrf=["']([^"']+)["']/i)?.[1];
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
    const response = await fetch(`https://codeforces.com/contest/${contestId}/my`, {
      method: 'GET',
      credentials: 'include'
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
  for (let attempt = 0; attempt < 6; attempt++) {
    const currentIds = await snapshotMyCodeforcesSubmissionIds(contestId, problemIndex);
    const id = currentIds && singleNewSubmissionId(knownIds, currentIds);
    if (id) return id;
    if (attempt < 5) await sleep(500);
  }
  return undefined;
}

function platformError(html: string): string | undefined {
  const match = html.match(/<(?:span|div|p)[^>]*class=["'][^"']*(?:error|alert-danger)[^"']*["'][^>]*>([\s\S]*?)<\/(?:span|div|p)>/i);
  const message = match?.[1] ? htmlText(match[1]) : '';
  return message || undefined;
}

class RetryableEndpointRejection extends Error {}

export class CodeforcesUserActionRequired extends Error {
  readonly knownSubmissionIds?: string[];

  constructor(message: string, knownSubmissionIds?: Iterable<string>) {
    super(message);
    this.name = 'CodeforcesUserActionRequired';
    this.knownSubmissionIds = knownSubmissionIds ? [...knownSubmissionIds] : undefined;
  }
}

function endpointRejection(html: string, error?: string): { kind: 'ANTIBOT' | 'NOT_REGISTERED'; message: string } | undefined {
  const text = error || htmlText(html);
  if (/anti[- ]?bot|complete (?:the )?verification|verify (?:that )?you are (?:a )?human|confirm (?:that )?you are not a robot|captcha|cf-turnstile|challenge-platform|just a moment/i.test(text)) {
    return {
      kind: 'ANTIBOT',
      message: 'Codeforces requires an interactive anti-bot check. Complete the verification and submit in the Codeforces tab that cpbridge opened.'
    };
  }
  if (/not registered (?:for|in) (?:the )?contest/i.test(text)) {
    return {
      kind: 'NOT_REGISTERED',
      message: 'This Codeforces account is not registered for the original contest.'
    };
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
    const res = await fetch('https://codeforces.com/settings/general', { method: 'GET', credentials: 'include' });
    if (!res.ok || new URL(res.url).pathname.startsWith('/enter')) {
      return { loggedIn: false };
    }
    const username = extractCodeforcesUsername(await res.text());
    return username ? { loggedIn: true, username } : { loggedIn: false };
  } catch {
    return { loggedIn: false };
  }
}

async function getCodeforcesTTA(): Promise<string> {
  try {
    const cookie = await chrome.cookies.get({ url: 'https://codeforces.com', name: '39c3' });
    if (cookie?.value) {
      let result = 0;
      for (let i = 0; i < cookie.value.length; i++) result = (result * 31 + cookie.value.charCodeAt(i)) % 10007;
      return String(result);
    }
  } catch {}
  return '176';
}

export async function submitCodeforces(contestId: string, index: string, language: LanguageId, sourceCode: string): Promise<{ externalSubmissionId: string }> {
  const submitUrls = /^\d+$/.test(contestId)
    ? [`https://codeforces.com/contest/${contestId}/submit`, `https://codeforces.com/problemset/submit`]
    : ['https://codeforces.com/problemset/submit'];
  let lastError = '';

  for (const submitUrl of submitUrls) {
    let postSent = false;
    try {
      const pageRes = await fetch(submitUrl, { method: 'GET', credentials: 'include' });
      const html = await pageRes.text();
      const csrfToken = getCsrfToken(html);
      if (!csrfToken) {
        if (html.includes('Enter Codeforces') || html.includes('handleOrEmail')) throw new Error('NOT_LOGGED_IN');
        const rejection = endpointRejection(html);
        if (rejection?.kind === 'ANTIBOT') {
          const knownIds = await snapshotMyCodeforcesSubmissionIds(contestId, index);
          throw new CodeforcesUserActionRequired(rejection.message, knownIds);
        }
        lastError = `Could not find the Codeforces CSRF token at ${submitUrl}`;
        continue;
      }

      const knownIds = await snapshotMyCodeforcesSubmissionIds(contestId, index);
      if (!knownIds) {
        throw new Error('Could not read your Codeforces submissions before submitting; refusing to match a verdict unsafely.');
      }
      const formData = new URLSearchParams({
        csrf_token: csrfToken, action: 'submitSolutionFormSubmitted', submittedProblemIndex: index,
        submittedProblemCode: `${contestId}${index}`, programTypeId: getOptionValue(html, language) || CF_LANGUAGE_MAP[language] || '89',
        source: sourceCode, tabSize: '4', _tta: getInputValue(html, '_tta') || await getCodeforcesTTA()
      });
      const submitRes = await fetch(`${submitUrl}?csrf_token=${encodeURIComponent(csrfToken)}`, {
        method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: formData.toString()
      });
      postSent = true;
      const responseHtml = await submitRes.text();
      if (!submitRes.ok) throw new Error(`Codeforces returned HTTP ${submitRes.status}`);
      const error = platformError(responseHtml);
      if (error && !/has been submitted|success/i.test(error)) {
        const rejection = endpointRejection(responseHtml, error);
        if (rejection?.kind === 'ANTIBOT') throw new CodeforcesUserActionRequired(rejection.message, knownIds);
        if (rejection?.kind === 'NOT_REGISTERED') throw new RetryableEndpointRejection(rejection.message);
        throw new Error(error);
      }
      const rejection = endpointRejection(responseHtml);
      if (rejection?.kind === 'ANTIBOT') throw new CodeforcesUserActionRequired(rejection.message, knownIds);
      if (rejection?.kind === 'NOT_REGISTERED') throw new RetryableEndpointRejection(rejection.message);

      const id = await waitForCodeforcesSubmission(contestId, index, knownIds);
      if (id) return { externalSubmissionId: id };
      throw new Error('Codeforces accepted the request, but it could not uniquely identify your new submission. Check the Codeforces submissions page.');
    } catch (err: unknown) {
      const errObj = err instanceof Error ? err : undefined;
      if (errObj?.message === 'NOT_LOGGED_IN') throw err;
      if (err instanceof CodeforcesUserActionRequired) throw err;
      // Some browser/network exceptions have an empty `message` (or are
      // thrown as strings). Never surface a blank "Codeforces: " error.
      const errorText = typeof err === 'string' ? err.trim() : String(errObj?.message || '').trim();
      lastError = errorText || `Request failed while ${postSent ? 'sending the submission to' : 'opening'} Codeforces`;
      // Codeforces may reject newer/unregistered accounts at the contest
      // endpoint before creating a submission. Only those explicit rejections
      // are safe to retry through the problemset endpoint without risking a
      // duplicate solution.
      if (postSent && !(err instanceof RetryableEndpointRejection)) {
        throw new Error(`Codeforces: ${lastError}`, { cause: err });
      }
    }
  }
  throw new Error(`Codeforces: ${lastError || 'Failed to open the submission form.'}`);
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
  try {
    const res = await fetch(`https://codeforces.com/api/contest.status?contestId=${contestId}&from=1&count=1000&cpbridge_ts=${Date.now()}`, { method: 'GET', credentials: 'include', cache: 'no-store' });
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
    const urls = [`https://codeforces.com/contest/${contestId}/submission/${externalSubmissionId}`, `https://codeforces.com/problemset/submission/${contestId}/${externalSubmissionId}`];
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

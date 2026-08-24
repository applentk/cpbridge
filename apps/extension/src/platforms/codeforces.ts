import type { LanguageId } from '@cpbridge/contracts';

// Fallbacks only: Codeforces changes compiler IDs periodically. The submit form
// is the source of truth whenever it is available.
const CF_LANGUAGE_MAP: Record<LanguageId, string> = {
  cpp23: '89', python3: '70', java21: '87', go: '32', rust: '75'
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
    python3: [/python.*3/, /pypy.*3/], java21: [/java\s*21/, /java\s*17/],
    go: [/\bgo\b.*1\./], rust: [/\brust\b.*1\./]
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

export async function checkCodeforcesSession(): Promise<{ loggedIn: boolean; username?: string }> {
  try {
    const res = await fetch('https://codeforces.com/enter', { method: 'GET', credentials: 'include' });
    const text = await res.text();
    const handleMatch = text.match(/handle\s*=\s*["']([^"']+)["']/) || text.match(/\/profile\/([a-zA-Z0-9_\-]+)/);
    return { loggedIn: !text.includes('Enter Codeforces') || !!handleMatch, username: handleMatch?.[1] };
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
      if (error && !/has been submitted|success/i.test(error)) throw new Error(error);

      const id = await waitForCodeforcesSubmission(contestId, index, knownIds);
      if (id) return { externalSubmissionId: id };
      throw new Error('Codeforces accepted the request, but it could not uniquely identify your new submission. Check the Codeforces submissions page.');
    } catch (err: any) {
      if (err.message === 'NOT_LOGGED_IN') throw err;
      // Some browser/network exceptions have an empty `message` (or are
      // thrown as strings). Never surface a blank "Codeforces: " error.
      const errorText = typeof err === 'string' ? err.trim() : String(err?.message || '').trim();
      lastError = errorText || `Request failed while ${postSent ? 'sending the submission to' : 'opening'} Codeforces`;
      if (postSent) throw new Error(`Codeforces: ${lastError}`);
    }
  }
  throw new Error(`Codeforces: ${lastError || 'Failed to open the submission form.'}`);
}

export async function pollCodeforcesStatus(contestId: string, externalSubmissionId: string): Promise<{ status: 'JUDGING' | 'ACCEPTED' | 'WRONG_ANSWER' | 'TIME_LIMIT' | 'MEMORY_LIMIT' | 'RUNTIME_ERROR' | 'COMPILE_ERROR' | 'FAILED' }> {
  if (externalSubmissionId.startsWith('cf_')) return { status: 'FAILED' };
  try {
    const res = await fetch(`https://codeforces.com/api/contest.status?contestId=${contestId}&from=1&count=100`, { method: 'GET', credentials: 'include' });
    if (res.ok) {
      const data = await res.json();
      const sub = data.status === 'OK' && Array.isArray(data.result) ? data.result.find((item: any) => String(item.id) === String(externalSubmissionId)) : undefined;
      if (sub?.verdict) {
        const verdicts: Record<string, any> = {
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

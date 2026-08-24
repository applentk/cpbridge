import type { LanguageId } from '@cpbridge/contracts';

// Fallbacks only. AtCoder's language IDs change when compiler versions change;
// submitAtCoder reads the current IDs from the submit form first.
const AC_LANGUAGE_MAP: Record<LanguageId, string> = {
  cpp23: '5052', python3: '5078', java21: '5005', go: '5025', rust: '5054'
};

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

function cleanOptionText(value: string): string {
  return value.replace(/<[^>]+>/g, ' ').replace(/&amp;/g, '&').replace(/\s+/g, ' ').trim().toLowerCase();
}

function getOptionValue(html: string, language: LanguageId): string | undefined {
  const wanted: Record<LanguageId, RegExp[]> = {
    cpp23: [/c\+\+\s*23/, /c\+\+\s*20/, /c\+\+/],
    python3: [/python.*3/, /pypy.*3/], java21: [/java\s*21/, /java\s*17/, /java/],
    go: [/\bgo\b.*1\./, /\bgo\b/], rust: [/\brust\b.*1\./, /\brust\b/]
  };
  const options = [...html.matchAll(/<option\b[^>]*value=["']([^"']+)["'][^>]*>([\s\S]*?)<\/option>/gi)];
  for (const pattern of wanted[language]) {
    const option = options.find((match) => pattern.test(cleanOptionText(match[2])));
    if (option) return option[1];
  }
  return undefined;
}

function getCsrfToken(html: string): string | undefined {
  return html.match(/name=["']csrf_token["'][^>]*value=["']([^"']+)["']/i)?.[1]
    || html.match(/csrf_token\s*=\s*["']([^"']+)["']/i)?.[1];
}

function extractSubmissionIds(html: string, taskId?: string): string[] {
  const rows = [...html.matchAll(/<tr\b[^>]*>[\s\S]*?<\/tr>/gi)].map((match) => match[0]);
  const candidates = rows.length > 0 ? rows : [html];
  const escapedTask = taskId?.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const taskPattern = escapedTask ? new RegExp(`(?:/tasks/|data-task-screen-name=["'])${escapedTask}(?:["'/?]|$)`, 'i') : undefined;
  const ids: string[] = [];
  for (const row of candidates) {
    if (taskPattern && !taskPattern.test(row)) continue;
    for (const match of row.matchAll(/\/submissions\/(\d+)/gi)) {
      if (!ids.includes(match[1])) ids.push(match[1]);
    }
  }
  return ids;
}

async function snapshotMyAtCoderSubmissionIds(contestId: string, taskId: string): Promise<Set<string> | undefined> {
  try {
    const response = await fetch(`https://atcoder.jp/contests/${contestId}/submissions/me`, { method: 'GET', credentials: 'include' });
    if (!response.ok) return undefined;
    const html = await response.text();
    return new Set(extractSubmissionIds(html, taskId));
  } catch {
    return undefined;
  }
}

function singleNewSubmissionId(knownIds: Set<string>, currentIds: Set<string>): string | undefined {
  const newIds = [...currentIds].filter((id) => !knownIds.has(id));
  // Do not guess when a second submission appeared concurrently.
  return newIds.length === 1 ? newIds[0] : undefined;
}

async function waitForAtCoderSubmission(contestId: string, taskId: string, knownIds: Set<string>): Promise<string | undefined> {
  for (let attempt = 0; attempt < 8; attempt++) {
    const currentIds = await snapshotMyAtCoderSubmissionIds(contestId, taskId);
    const id = currentIds && singleNewSubmissionId(knownIds, currentIds);
    if (id) return id;
    if (attempt < 7) await sleep(400);
  }
  return undefined;
}

function platformError(html: string): string | undefined {
  const match = html.match(/<(?:div|span|p)[^>]*class=["'][^"']*(?:alert-danger|error)[^"']*["'][^>]*>([\s\S]*?)<\/(?:div|span|p)>/i);
  const message = match?.[1]?.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  return message || undefined;
}

type AtCoderPageSubmitResult = {
  externalSubmissionId?: string;
  error?: string;
};

async function submitAtCoderFromSameOriginPage(
  contestId: string,
  taskId: string,
  languageId: string,
  sourceCode: string,
  knownIds: Set<string>
): Promise<string> {
  const tab = await chrome.tabs.create({
    url: `https://atcoder.jp/contests/${contestId}/submit`,
    active: false
  });
  if (!tab.id) throw new Error('AtCoder: could not open the submit page');

  const tabId = tab.id;
  try {
    await new Promise<void>((resolve, reject) => {
      let settled = false;
      const finish = () => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      };
      const timeout = setTimeout(() => {
        if (settled) return;
        settled = true;
        chrome.tabs.onUpdated.removeListener(listener);
        reject(new Error('AtCoder: submit page did not finish loading'));
      }, 10000);
      const listener = (updatedTabId: number, changeInfo: chrome.tabs.TabChangeInfo) => {
        if (updatedTabId === tabId && changeInfo.status === 'complete') finish();
      };
      chrome.tabs.onUpdated.addListener(listener);
      chrome.tabs.get(tabId).then((currentTab) => {
        if (currentTab.status === 'complete') finish();
      }).catch(reject);
    });

    const [execution] = await chrome.scripting.executeScript({
      target: { tabId },
      func: async (args): Promise<AtCoderPageSubmitResult> => {
        const csrfToken = document.querySelector<HTMLInputElement>('input[name="csrf_token"]')?.value;
        if (!csrfToken) return { error: 'NOT_LOGGED_IN' };

        const body = new URLSearchParams({
          'data.TaskScreenName': args.taskId,
          'data.LanguageId': args.languageId,
          sourceCode: args.sourceCode,
          csrf_token: csrfToken
        });
        const response = await fetch(`/contests/${args.contestId}/submit`, {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body
        });
        const responseHtml = await response.text();
        if (!response.ok) {
          const message = responseHtml.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 240);
          return { error: `AtCoder returned HTTP ${response.status}${message ? `: ${message}` : ''}` };
        }

        for (let attempt = 0; attempt < 8; attempt++) {
          const submissionsResponse = await fetch(`/contests/${args.contestId}/submissions/me`, {
            credentials: 'same-origin'
          });
          if (submissionsResponse.ok) {
            const submissionsDoc = new DOMParser().parseFromString(await submissionsResponse.text(), 'text/html');
            const currentIds = new Set<string>();
            for (const row of [...submissionsDoc.querySelectorAll('tr')]) {
              const taskLink = [...row.querySelectorAll<HTMLAnchorElement>('a[href*="/tasks/"]')]
                .some((link) => link.href.includes(`/tasks/${args.taskId}`));
              if (!taskLink) continue;
              for (const candidate of [...row.querySelectorAll<HTMLAnchorElement>('a[href*="/submissions/"]')]
                .map((link) => link.href.match(/\/submissions\/(\d+)/)?.[1])) {
                if (candidate) currentIds.add(candidate);
              }
            }
            const newIds = [...currentIds].filter((id) => !args.knownIds.includes(id));
            if (newIds.length === 1) return { externalSubmissionId: newIds[0] };
          }
          if (attempt < 7) await new Promise((resolve) => setTimeout(resolve, 400));
        }
        return {};
      },
      args: [{ contestId, taskId, languageId, sourceCode, knownIds: [...knownIds] }]
    });

    const result = execution?.result as AtCoderPageSubmitResult | undefined;
    if (result?.externalSubmissionId) return result.externalSubmissionId;
    if (result?.error === 'NOT_LOGGED_IN') throw new Error('NOT_LOGGED_IN');
    throw new Error(result?.error || 'AtCoder accepted the request but the new submission ID was not visible yet.');
  } finally {
    await chrome.tabs.remove(tabId).catch(() => undefined);
  }
}

export async function checkAtCoderSession(): Promise<{ loggedIn: boolean; username?: string }> {
  try {
    const res = await fetch('https://atcoder.jp/home', { method: 'GET', credentials: 'include' });
    const text = await res.text();
    const userMatch = text.match(/\/users\/([a-zA-Z0-9_\-]+)/);
    return { loggedIn: !!userMatch && !text.includes('Sign In'), username: userMatch?.[1] };
  } catch {
    return { loggedIn: false };
  }
}

export async function submitAtCoder(contestId: string, taskId: string, language: LanguageId, sourceCode: string): Promise<{ externalSubmissionId: string }> {
  const submitPageUrl = `https://atcoder.jp/contests/${contestId}/submit`;
  const pageRes = await fetch(submitPageUrl, {
    method: 'GET',
    credentials: 'include',
    referrer: submitPageUrl,
    referrerPolicy: 'strict-origin-when-cross-origin'
  });
  const html = await pageRes.text();
  const csrfToken = getCsrfToken(html);
  if (!csrfToken || html.includes('Sign In')) throw new Error('NOT_LOGGED_IN');

  const knownIds = await snapshotMyAtCoderSubmissionIds(contestId, taskId);
  if (!knownIds) {
    throw new Error('Could not read your AtCoder submissions before submitting; refusing to match a verdict unsafely.');
  }
  const formData = new URLSearchParams({
    'data.TaskScreenName': taskId,
    'data.LanguageId': getOptionValue(html, language) || AC_LANGUAGE_MAP[language] || '5052',
    sourceCode,
    csrf_token: csrfToken
  });
  const submitRes = await fetch(submitPageUrl, {
    method: 'POST',
    credentials: 'include',
    referrer: submitPageUrl,
    referrerPolicy: 'strict-origin-when-cross-origin',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: formData.toString()
  });
  const responseHtml = await submitRes.text();
  if (!submitRes.ok) {
    if (submitRes.status === 403) {
      return {
        externalSubmissionId: await submitAtCoderFromSameOriginPage(
          contestId,
          taskId,
          getOptionValue(html, language) || AC_LANGUAGE_MAP[language] || '5052',
          sourceCode,
          knownIds
        )
      };
    }
    const detail = platformError(responseHtml);
    throw new Error(`AtCoder returned HTTP ${submitRes.status}${detail ? `: ${detail}` : ''}`);
  }
  const error = platformError(responseHtml);
  if (error && !/success|submitted/i.test(error)) throw new Error(error);

  const id = await waitForAtCoderSubmission(contestId, taskId, knownIds);
  if (id) return { externalSubmissionId: id };
  throw new Error('AtCoder accepted the request, but it could not uniquely identify your new submission. Check the AtCoder submissions page.');
}

export async function pollAtCoderStatus(contestId: string, externalSubmissionId: string): Promise<{ status: 'JUDGING' | 'ACCEPTED' | 'WRONG_ANSWER' | 'TIME_LIMIT' | 'MEMORY_LIMIT' | 'RUNTIME_ERROR' | 'COMPILE_ERROR' | 'FAILED' }> {
  if (externalSubmissionId.startsWith('ac_')) return { status: 'FAILED' };
  try {
    const res = await fetch(`https://atcoder.jp/contests/${contestId}/submissions/${externalSubmissionId}`, { method: 'GET', credentials: 'include' });
    if (res.status === 404 || !res.ok) return { status: 'JUDGING' };
    const html = await res.text();
    if (html.includes('>AC</span>') || html.includes('label-success')) return { status: 'ACCEPTED' };
    if (html.includes('>WA</span>')) return { status: 'WRONG_ANSWER' };
    if (html.includes('>TLE</span>')) return { status: 'TIME_LIMIT' };
    if (html.includes('>MLE</span>')) return { status: 'MEMORY_LIMIT' };
    if (html.includes('>RE</span>')) return { status: 'RUNTIME_ERROR' };
    if (html.includes('>CE</span>')) return { status: 'COMPILE_ERROR' };
    if (html.includes('>OLE</span>') || html.includes('>QLE</span>')) return { status: 'FAILED' };
    if (html.includes('>WJ</span>') || html.includes('>WR</span>') || html.includes('label-default')) return { status: 'JUDGING' };
  } catch {}
  return { status: 'JUDGING' };
}

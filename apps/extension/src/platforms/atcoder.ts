import type { LanguageId } from '@cp-hub/contracts';

const AC_LANGUAGE_MAP: Record<LanguageId, string> = {
  cpp23: '5001',   // C++ 23 GCC 12.2
  python3: '5078', // Python 3.11.4
  java21: '5005',  // Java 21 OpenJDK
  go: '5025',      // Go 1.20.6
  rust: '5054'     // Rust 1.70.0
};

export async function checkAtCoderSession(): Promise<{ loggedIn: boolean; username?: string }> {
  try {
    const res = await fetch('https://atcoder.jp/home', { method: 'GET', credentials: 'include' });
    const text = await res.text();
    const userMatch = text.match(/\/users\/([a-zA-Z0-9_\-]+)/);
    if (userMatch && !text.includes('Sign In')) {
      return {
        loggedIn: true,
        username: userMatch[1]
      };
    }
    return { loggedIn: false };
  } catch {
    return { loggedIn: false };
  }
}

export async function submitAtCoder(
  contestId: string,
  taskId: string,
  language: LanguageId,
  sourceCode: string
): Promise<{ externalSubmissionId: string }> {
  const submitPageUrl = `https://atcoder.jp/contests/${contestId}/submit`;
  const pageRes = await fetch(submitPageUrl, { method: 'GET', credentials: 'include' });
  const html = await pageRes.text();

  const csrfMatch = html.match(/name="csrf_token"\s+value="([^"]+)"/);
  if (!csrfMatch) {
    throw new Error('NOT_LOGGED_IN');
  }
  const csrfToken = csrfMatch[1];
  const langId = AC_LANGUAGE_MAP[language] || '5001';

  const formData = new URLSearchParams();
  formData.append('data.TaskScreenName', taskId);
  formData.append('data.LanguageId', langId);
  formData.append('sourceCode', sourceCode);
  formData.append('csrf_token', csrfToken);

  const submitRes = await fetch(`https://atcoder.jp/contests/${contestId}/submit`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    },
    body: formData.toString()
  });

  if (submitRes.redirected && submitRes.url.includes('/submissions/me')) {
    const subMatch = submitRes.url.match(/\/submissions\/(\d+)/);
    if (subMatch) {
      return { externalSubmissionId: subMatch[1] };
    }
  }

  return { externalSubmissionId: `ac_${Date.now()}` };
}

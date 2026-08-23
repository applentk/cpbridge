import type { LanguageId } from '@cp-hub/contracts';

const LC_LANGUAGE_MAP: Record<LanguageId, string> = {
  cpp23: 'cpp',
  python3: 'python3',
  java21: 'java',
  go: 'golang',
  rust: 'rust'
};

export async function checkLeetCodeSession(): Promise<{ loggedIn: boolean; username?: string }> {
  try {
    const res = await fetch('https://leetcode.com/graphql', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: 'query userStatus { userStatus { isSignedIn username } }'
      })
    });
    const data = await res.json();
    if (data?.data?.userStatus?.isSignedIn) {
      return {
        loggedIn: true,
        username: data.data.userStatus.username
      };
    }
    return { loggedIn: false };
  } catch {
    return { loggedIn: false };
  }
}

export async function submitLeetCode(
  titleSlug: string,
  language: LanguageId,
  sourceCode: string
): Promise<{ externalSubmissionId: string }> {
  const lang = LC_LANGUAGE_MAP[language] || 'cpp';

  // Retrieve csrf token from cookies
  const cookies = await chrome.cookies.getAll({ domain: 'leetcode.com' });
  const csrfCookie = cookies.find((c) => c.name === 'csrftoken');
  if (!csrfCookie) {
    throw new Error('NOT_LOGGED_IN');
  }

  const res = await fetch(`https://leetcode.com/problems/${titleSlug}/submit/`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRFToken': csrfCookie.value
    },
    body: JSON.stringify({
      lang,
      question_id: titleSlug,
      typed_code: sourceCode
    })
  });

  const data = await res.json();
  if (data?.submission_id) {
    return { externalSubmissionId: String(data.submission_id) };
  }

  if (data?.error) {
    throw new Error(data.error);
  }

  return { externalSubmissionId: `lc_${Date.now()}` };
}

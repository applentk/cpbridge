import assert from 'node:assert/strict';
import { afterEach, describe, test } from 'node:test';

import {
  checkCodeforcesSession,
  parseCodeforcesExternalId,
  pollCodeforcesStatus,
  snapshotCodeforcesSubmissionIds
} from '../dist/test/codeforces.js';
import {
  checkAtCoderSession,
  pollAtCoderStatus,
  submitAtCoder
} from '../dist/test/atcoder.js';
import { isCodeforcesSubmissionForm } from '../dist/test/codeforces-submit-form.js';

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function htmlResponse(body, status = 200, url) {
  const response = new Response(body, {
    status,
    headers: { 'Content-Type': 'text/html' }
  });
  if (url) Object.defineProperty(response, 'url', { value: url });
  return response;
}

function codeforcesSubmissionRow(id, problemIndex = 'A') {
  return `<tr data-submission-id="${id}"><td><a href="/contest/123/problem/${problemIndex}">${problemIndex}</a></td></tr>`;
}

function atCoderSubmissionRow(id, taskId = 'abc123_a') {
  return `<tr><td><a href="/contests/abc123/tasks/${taskId}">${taskId}</a><a href="/contests/abc123/submissions/${id}">${id}</a></td></tr>`;
}

describe('Codeforces adapter', () => {
  test('parses regular and Gym problem external IDs', () => {
    assert.deepEqual(parseCodeforcesExternalId('123/A'), { contestId: '123', problemIndex: 'A' });
    assert.deepEqual(parseCodeforcesExternalId('gym/105053/a'), { contestId: 'gym/105053', problemIndex: 'A' });
    assert.equal(parseCodeforcesExternalId('gym/105053'), undefined);
  });

  test('recognizes the dynamic Codeforces problemset submission form', () => {
    const form = {
      classList: { contains: (name) => name === 'submit-form' },
      querySelector: () => null
    };

    assert.equal(isCodeforcesSubmissionForm(form), true);
  });

  test('recognizes the Codeforces submission action field without relying on the form URL', () => {
    const form = {
      classList: { contains: () => false },
      querySelector: () => ({ value: 'submitSolutionFormSubmitted' })
    };

    assert.equal(isCodeforcesSubmissionForm(form), true);
  });

  test('detects the authenticated account from the settings page', async () => {
    globalThis.fetch = async () => htmlResponse(
      '<a href="/profile/tourist">tourist</a>',
      200,
      'https://codeforces.com/settings/general'
    );

    assert.deepEqual(await checkCodeforcesSession(), {
      loggedIn: true,
      username: 'tourist'
    });
  });

  test('snapshots only matching submissions before the interactive handoff', async () => {
    let request;
    globalThis.fetch = async (input, init = {}) => {
      request = { url: String(input), init };
      return htmlResponse(codeforcesSubmissionRow('100', 'A') + codeforcesSubmissionRow('101', 'B'));
    };

    assert.deepEqual(await snapshotCodeforcesSubmissionIds('123', 'A'), ['100']);
    assert.match(request.url, /\/contest\/123\/my\?cpbridge_ts=/);
    assert.equal(request.init.method, 'GET');
    assert.equal(request.init.credentials, 'include');
    assert.equal(request.init.cache, 'no-store');
  });

  test('normalizes API verdicts', async () => {
    globalThis.fetch = async () => Response.json({
      status: 'OK',
      result: [{ id: 456, verdict: 'TIME_LIMIT_EXCEEDED' }]
    });

    assert.deepEqual(await pollCodeforcesStatus('123', '456'), { status: 'TIME_LIMIT' });
  });
});

describe('AtCoder adapter', () => {
  test('detects the authenticated account from the home page', async () => {
    globalThis.fetch = async () => htmlResponse('<a href="/users/chokudai">chokudai</a>');

    assert.deepEqual(await checkAtCoderSession(), {
      loggedIn: true,
      username: 'chokudai'
    });
  });

  test('selects Java 24 without matching JavaScript and identifies the new submission', async () => {
    let submittedForm;
    globalThis.fetch = async (input, init = {}) => {
      const url = String(input);
      if (url.includes('/submissions/me?')) {
        return htmlResponse(atCoderSubmissionRow('300'));
      }
      if (url.endsWith('/submit') && init.method === 'GET') {
        return htmlResponse(`
          <input name="csrf_token" value="csrf-atcoder">
          <select>
            <option value="9999">JavaScript Node.js</option>
            <option value="6056">Java 24</option>
          </select>
        `);
      }
      if (url.endsWith('/submit') && init.method === 'POST') {
        submittedForm = new URLSearchParams(String(init.body));
        return htmlResponse(atCoderSubmissionRow('301'));
      }
      throw new Error(`Unexpected request: ${init.method || 'GET'} ${url}`);
    };

    const result = await submitAtCoder('abc123', 'abc123_a', 'java21', 'class Main {}');

    assert.deepEqual(result, { externalSubmissionId: '301' });
    assert.equal(submittedForm?.get('data.LanguageId'), '6056');
    assert.equal(submittedForm?.get('sourceCode'), 'class Main {}');
  });

  test('uses the current Python fallback ID when the form has no matching option', async () => {
    let submittedForm;
    globalThis.fetch = async (input, init = {}) => {
      const url = String(input);
      if (url.includes('/submissions/me?')) {
        return htmlResponse(atCoderSubmissionRow('400'));
      }
      if (url.endsWith('/submit') && init.method === 'GET') {
        return htmlResponse('<input name="csrf_token" value="csrf-atcoder">');
      }
      if (url.endsWith('/submit') && init.method === 'POST') {
        submittedForm = new URLSearchParams(String(init.body));
        return htmlResponse(atCoderSubmissionRow('401'));
      }
      throw new Error(`Unexpected request: ${init.method || 'GET'} ${url}`);
    };

    await submitAtCoder('abc123', 'abc123_a', 'python3', 'print(1)');

    assert.equal(submittedForm?.get('data.LanguageId'), '6082');
  });

  test('normalizes submission-page verdicts', async () => {
    globalThis.fetch = async () => htmlResponse('<span class="label">WA</span>');

    assert.deepEqual(await pollAtCoderStatus('abc123', '789'), { status: 'WRONG_ANSWER' });
  });
});

import assert from 'node:assert/strict';
import { afterEach, describe, test } from 'node:test';

import {
  checkCodeforcesSession,
  pollCodeforcesStatus,
  submitCodeforces
} from '../dist/test/codeforces.js';
import {
  checkAtCoderSession,
  pollAtCoderStatus,
  submitAtCoder
} from '../dist/test/atcoder.js';

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

  test('selects a Java compiler without matching JavaScript', async () => {
    let myPageReads = 0;
    let submittedForm;
    globalThis.fetch = async (input, init = {}) => {
      const url = String(input);
      if (url.endsWith('/contest/123/submit') && init.method === 'GET') {
        return htmlResponse(`
          <input name="_tta" value="1234">
          <script>var csrf_token = "csrf-codeforces";</script>
          <select>
            <option value="55">JavaScript V8</option>
            <option value="87">Java 17</option>
          </select>
        `);
      }
      if (url.endsWith('/contest/123/my')) {
        myPageReads += 1;
        return htmlResponse(codeforcesSubmissionRow(myPageReads === 1 ? '100' : '101'));
      }
      if (url.includes('/contest/123/submit?') && init.method === 'POST') {
        submittedForm = new URLSearchParams(String(init.body));
        return htmlResponse('<div class="success">has been submitted</div>');
      }
      throw new Error(`Unexpected request: ${init.method || 'GET'} ${url}`);
    };

    const result = await submitCodeforces('123', 'A', 'java21', 'class Main {}');

    assert.deepEqual(result, { externalSubmissionId: '101' });
    assert.equal(submittedForm?.get('programTypeId'), '87');
    assert.equal(submittedForm?.get('source'), 'class Main {}');
  });

  test('uses the current C++ fallback ID when the form has no matching option', async () => {
    let myPageReads = 0;
    let submittedForm;
    globalThis.fetch = async (input, init = {}) => {
      const url = String(input);
      if (url.endsWith('/contest/123/submit') && init.method === 'GET') {
        return htmlResponse('<input name="_tta" value="1234"><script>var csrf_token = "csrf";</script>');
      }
      if (url.endsWith('/contest/123/my')) {
        myPageReads += 1;
        return htmlResponse(codeforcesSubmissionRow(myPageReads === 1 ? '200' : '201'));
      }
      if (url.includes('/contest/123/submit?') && init.method === 'POST') {
        submittedForm = new URLSearchParams(String(init.body));
        return htmlResponse('submitted');
      }
      throw new Error(`Unexpected request: ${init.method || 'GET'} ${url}`);
    };

    await submitCodeforces('123', 'A', 'cpp23', '#include <iostream>');

    assert.equal(submittedForm?.get('programTypeId'), '91');
  });

  test('falls back to problemset submission when a new account is blocked at the contest endpoint', async () => {
    let myPageReads = 0;
    const postUrls = [];
    globalThis.fetch = async (input, init = {}) => {
      const url = String(input);
      if (url.endsWith('/contest/123/submit') && init.method === 'GET') {
        return htmlResponse('<input name="_tta" value="1234"><script>var csrf_token = "contest-csrf";</script>');
      }
      if (url.endsWith('/problemset/submit') && init.method === 'GET') {
        return htmlResponse('<input name="_tta" value="1234"><script>var csrf_token = "problemset-csrf";</script>');
      }
      if (url.endsWith('/contest/123/my')) {
        myPageReads += 1;
        return htmlResponse(codeforcesSubmissionRow(myPageReads <= 2 ? '500' : '501'));
      }
      if (url.includes('/contest/123/submit?') && init.method === 'POST') {
        postUrls.push(url);
        return htmlResponse('<span class="error">Please complete anti-bot verification to submit a solution</span>');
      }
      if (url.includes('/problemset/submit?') && init.method === 'POST') {
        postUrls.push(url);
        return htmlResponse('<div class="success">has been submitted</div>');
      }
      throw new Error(`Unexpected request: ${init.method || 'GET'} ${url}`);
    };

    const result = await submitCodeforces('123', 'A', 'cpp23', '#include <iostream>');

    assert.deepEqual(result, { externalSubmissionId: '501' });
    assert.equal(postUrls.length, 2);
    assert.match(postUrls[0], /\/contest\/123\/submit\?/);
    assert.match(postUrls[1], /\/problemset\/submit\?/);
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

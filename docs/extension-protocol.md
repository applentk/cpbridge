# cpbridge — Chrome Extension Protocol

The extension is a Manifest V3 Chrome extension built from `apps/extension`. It is a browser-session bridge: the cpbridge backend never receives Codeforces or AtCoder cookies.

## Components

- `apps/extension/src/bridge.ts`: content script injected into allowed cpbridge origins; forwards messages between the page and `chrome.runtime`.
- `apps/extension/src/background.ts`: service worker; checks sessions, submits code, and polls verdicts.
- `apps/extension/src/platforms/codeforces.ts`: Codeforces form/API/HTML submission logic.
- `apps/extension/src/platforms/atcoder.ts`: AtCoder form/API/HTML submission logic.
- `apps/web/src/lib/extension/bridge.ts`: web-side request/response wrapper.
- `packages/contracts/src/protocol.ts`: shared message and response types.

## Transport and origin checks

The web page sends a message with `source: CP_HUB_WEB` using `window.postMessage`. The content script accepts only messages from the same window and an allowlisted origin. It forwards the payload to the background worker and posts the response back with `source: CP_HUB_EXTENSION`.

Allowed origins include the local development hosts and `https://cphub.dev` / `https://app.cphub.dev`.

Each request has a generated ID. The web bridge keeps a callback map and times out a request after 10 seconds.

## Messages

### `PING` → `PONG`

Checks Codeforces and AtCoder sessions.

```json
{
  "type": "PING"
}
```

```json
{
  "type": "PONG",
  "version": "1.0.0",
  "platforms": {
    "CODEFORCES": { "loggedIn": true, "username": "tourist" },
    "ATCODER": { "loggedIn": false }
  }
}
```

### `SUBMIT` → `SUBMISSION_CREATED` or `SUBMISSION_FAILED`

The web app sends the cpbridge submission ID, normalized platform problem ID, language, and source code.

```json
{
  "type": "SUBMIT",
  "submissionId": "sub_018f9...",
  "platform": "CODEFORCES",
  "problem": {
    "externalId": "1900/A",
    "url": "https://codeforces.com/problemset/problem/1900/A"
  },
  "language": "cpp23",
  "source": "#include <iostream>..."
}
```

On success, the extension returns the external judge submission ID:

```json
{
  "type": "SUBMISSION_CREATED",
  "submissionId": "sub_018f9...",
  "externalSubmissionId": "234891238"
}
```

On failure, it returns an error code such as `NOT_LOGGED_IN`, `SUBMISSION_FAILED`, or `PLATFORM_UNAVAILABLE`.

### `POLL_STATUS` → `POLL_STATUS_RESULT`

The web app can ask the extension to poll a submission using the user's browser session:

```json
{
  "type": "POLL_STATUS",
  "platform": "ATCODER",
  "externalSubmissionId": "123456",
  "problem": {
    "externalId": "abc350/abc350_f",
    "url": "https://atcoder.jp/contests/abc350/tasks/abc350_f"
  }
}
```

The response normalizes platform verdicts to `JUDGING`, `ACCEPTED`, `WRONG_ANSWER`, `TIME_LIMIT`, `MEMORY_LIMIT`, `RUNTIME_ERROR`, `COMPILE_ERROR`, or `FAILED`.

## Submission lifecycle

The API creates the database row first. After the extension returns an external ID, the web app calls `/api/submissions/{id}/dispatched`. The API changes the row to `JUDGING` and enqueues an Asynq polling task. The web page also polls while the submission is active and may request an extension-side status check.

## Required Chrome permissions

The manifest requests `cookies`, `activeTab`, and `scripting`, plus host permissions for Codeforces, AtCoder, local cpbridge development hosts, and the production cpbridge hosts. These permissions are what allow the extension to use the user's existing platform session and, for AtCoder fallback submission, open a same-origin submit page and execute the form request there.

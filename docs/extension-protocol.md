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

Allowed origins include local development hosts, `https://cpbridge.applentk.com`, and `https://*.applentk.com`.

Each request has a generated ID. The web bridge keeps a callback map. The default timeout is 10 seconds, submission dispatch uses 30 seconds, and recovery uses 20 seconds.

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
  "version": "1.0.4",
  "platforms": {
    "CODEFORCES": { "loggedIn": true, "username": "tourist" },
    "ATCODER": { "loggedIn": false }
  }
}
```

The response version comes from `chrome.runtime.getManifest()`. The example above matches the current manifest.

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

On failure, it returns one of the shared error codes: `NOT_LOGGED_IN`, `PLATFORM_UNAVAILABLE`, `RATE_LIMITED`, `UNSUPPORTED_LANGUAGE`, `PROBLEM_NOT_FOUND`, `SUBMISSION_FAILED`, or `UNKNOWN`. Current platform-dispatch failures are generally mapped to `NOT_LOGGED_IN` or `SUBMISSION_FAILED`; bridge/runtime failures use `PLATFORM_UNAVAILABLE` or `UNKNOWN`.

The background worker deduplicates concurrent `SUBMIT` messages by cpbridge submission ID. It also persists a compact handoff record under `cp_hub_dispatch:{submissionId}` in `chrome.storage.local`. The stored record contains the cpbridge ID, state, and external ID or error; it does not contain source code or platform credentials.

### `RECOVER_SUBMISSIONS` → `RECOVER_SUBMISSIONS_RESULT`

After a reload, the web app asks for dispatches whose API handoff may not have completed:

```json
{
  "type": "RECOVER_SUBMISSIONS"
}
```

```json
{
  "type": "RECOVER_SUBMISSIONS_RESULT",
  "submissions": [
    {
      "submissionId": "sub_018f9...",
      "state": "CREATED",
      "externalSubmissionId": "234891238"
    }
  ]
}
```

States are `DISPATCHING`, `CREATED`, or `FAILED`. If the original in-memory dispatch is still running, recovery waits up to 15 seconds for that same operation; it never starts a duplicate platform submission.

### `ACK_SUBMISSION` → `ACK_SUBMISSION_RESULT`

After the web app successfully applies a recovered result to the API, it acknowledges the record so the extension can remove it:

```json
{
  "type": "ACK_SUBMISSION",
  "submissionId": "sub_018f9..."
}
```

```json
{
  "type": "ACK_SUBMISSION_RESULT",
  "submissionId": "sub_018f9...",
  "acknowledged": true
}
```

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

The API creates the database row first. The web app then sends `SUBMIT`, retrying the same submission ID up to three times if the bridge itself times out. Because the extension deduplicates by ID and retains a successful result, those retries do not intentionally resubmit the source.

After the extension returns an external ID, the web app calls `/api/submissions/{id}/dispatched`. The API changes the row to `JUDGING` and enqueues an Asynq polling task. The web page also polls while the submission is active and may request an extension-side status check.

On page startup and before submission-history loads, the web app runs the recovery handshake. A recovered `CREATED` result completes the `/dispatched` call; a recovered `FAILED` result updates `/result`; acknowledgement happens only after the API call succeeds.

Codeforces submits directly from the service worker and identifies the new ID from the signed-in user's `/my` page. AtCoder normally submits directly too, but browser verification can require an inactive same-origin submit tab. That tab may briefly appear and is always closed after the attempt.

## Required Chrome permissions

The manifest requests `cookies`, `activeTab`, `scripting`, and `storage`, plus host permissions for Codeforces, AtCoder, local cpbridge development hosts, and the production cpbridge hosts. These permissions allow the extension to use the user's existing platform session, retain recoverable handoff state, and, for AtCoder fallback submission, open a same-origin submit page and execute the form request there.

`pnpm --filter @cpbridge/extension build` writes the two scripts under `apps/extension/dist` and packages the manifest plus `dist/` into ZIP files for local installation and web download.

# Chrome Extension Bridge Protocol

The CP Hub Chrome Extension communicates with the web application via `window.postMessage` and handles background submission dispatches using browser cookies.

---

## Protocol Messages

### `PING` -> `PONG`
Checks if the extension is installed and queries the active session state of each platform.

**Request**:
```json
{
  "type": "PING"
}
```

**Response**:
```json
{
  "type": "PONG",
  "version": "1.0.0",
  "platforms": {
    "CODEFORCES": { "loggedIn": true, "username": "tourist" },
    "ATCODER": { "loggedIn": true, "username": "chokudai" },
    "LEETCODE": { "loggedIn": false }
  }
}
```

---

### `SUBMIT` -> `SUBMISSION_CREATED` | `SUBMISSION_FAILED`
Dispatches code to the external platform judge.

**Request**:
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

**Success Response**:
```json
{
  "type": "SUBMISSION_CREATED",
  "submissionId": "sub_018f9...",
  "externalSubmissionId": "234891238"
}
```

**Error Response**:
```json
{
  "type": "SUBMISSION_FAILED",
  "submissionId": "sub_018f9...",
  "error": "NOT_LOGGED_IN",
  "message": "Please log in to Codeforces in your browser"
}
```

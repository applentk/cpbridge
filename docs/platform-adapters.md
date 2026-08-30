# cpbridge — Platform Adapters

Platform adapters live under `apps/api/internal/platform`. They normalize external problem metadata and verdicts for the rest of the application. The current interface is defined in `apps/api/internal/platform/platform.go`:

```go
type Platform interface {
    Type() Type
    MatchURL(rawURL string) (externalID string, matched bool)
    GetProblem(ctx context.Context, externalID string) (*NormalizedProblem, error)
    GetStatement(ctx context.Context, externalID string) (*ProblemStatement, error)
    GetSubmission(ctx context.Context, externalSubmissionID string) (*SubmissionStatus, error)
}
```

The registry is created in `apps/api/cmd/server/main.go` and currently registers:

```go
registry.Register(codeforces.New())
registry.Register(atcoder.New())
```

## Normalized types

`NormalizedProblem` contains the platform, external ID, title, official URL, optional difficulty, tags, and a metadata map.

`ProblemStatement` contains HTML, optional time/memory limits, and extracted sample cases.

`SubmissionStatus` contains the external ID, normalized verdict, optional execution time, memory, failed testcase, compiler output, and raw platform payload.

## Codeforces adapter

Source: `apps/api/internal/platform/codeforces/codeforces.go`

Recognized URLs include:

- `codeforces.com/problemset/problem/{contest}/{index}`
- `codeforces.com/contest/{contest}/problem/{index}`
- `codeforces.com/gym/{contest}/problem/{index}`

The normalized external ID is `{contest}/{INDEX}`. Problem metadata first comes from the Codeforces `contest.standings` API. If that is unavailable or omits the problem, the adapter scrapes the official problem page title and finally creates a generic placeholder title.

The adapter also implements the optional `ContestProvider` capability. Administrators can supply a numeric contest ID, a regular `codeforces.com/contest/{id}` URL, or a public `codeforces.com/gym/{id}` URL. Regular contests use an anonymous `contest.standings?contestId={id}` request. Because the anonymous API does not expose most gyms, public Gym dashboards are parsed for their ordered problem tables. Private, restricted, and unrevealed contests fail without storing partial data.

Statements are scraped from the official HTML page. The adapter extracts limits and sample input/output blocks, removes redundant headers/sample sections, and falls back to a link to the official statement when parsing fails.

Verdicts are read from Codeforces `contest.status`; the adapter also has an HTML-page fallback for individual submissions. IDs with the mock `cf_` prefix are treated as invalid and become `FAILED`.

## AtCoder adapter

Source: `apps/api/internal/platform/atcoder/atcoder.go`

Recognized URLs are:

```text
https://atcoder.jp/contests/{contest}/tasks/{task}
```

The normalized external ID is `{contest}/{task}`. Metadata and statements are parsed from the AtCoder task page, including the English statement, limits, and sample cases. The adapter removes the Japanese section and common footer content.

AtCoder also implements `ContestProvider`. A contest slug or `atcoder.jp/contests/{slug}` URL is resolved through its public tasks page, preserving the displayed task order and normalizing every task into the existing problem model.

AtCoder submission status is read from the individual submission page. IDs with the mock `ac_` prefix are treated as invalid and become `FAILED`.

## Submission responsibility

The Go adapters do not submit code. They only fetch metadata/statements and poll verdicts. Actual submission is implemented in the Chrome extension because the extension can use the user's active browser session.

The API, shared contracts, web editor, and extension support these language IDs:

| ID | Display name |
| --- | --- |
| `cpp23` | C++23 (GCC) |
| `python3` | Python 3 |
| `java21` | Java 21 |

The extension parses the current platform submit form to choose a compiler. Its numeric Codeforces and AtCoder language maps are fallbacks because judge compiler IDs can change.

Before submitting, the extension snapshots the authenticated user's matching submissions. Codeforces uses `/contest/{contest}/my`; AtCoder uses `/contests/{contest}/submissions/me`. After the form request it accepts an ID only when one new submission can be identified, avoiding accidental association with another concurrent submission.

For every Codeforces submission, the extension opens the official problemset submit page and waits until it can safely snapshot the signed-in account. It then prefills the form but leaves review, any verification challenge, and the final Submit click to the user. cpbridge verifies the result against that snapshot before attaching the external submission ID and closing the tab.

AtCoder's submit form can include browser-generated verification data. When that protection is present (or a direct form POST returns HTTP 403), the extension opens an inactive, short-lived AtCoder submit tab, serializes the real same-origin form after verification completes, submits it, and closes the tab in a `finally` path.

## Adding another platform

1. Add a package under `apps/api/internal/platform/<name>` implementing the interface above.
2. Add a new `Type` constant in `platform.go`.
3. Register the adapter in `apps/api/cmd/server/main.go` and `apps/api/cmd/seed-demo/main.go` if the demo should import it.
4. Update `packages/contracts/src/problem.ts` and the extension protocol/adapter if the web client can submit to the platform.
5. Add URL, normalization, statement, and verdict tests.

The web client and extension currently support only the language and platform values declared in `packages/contracts/src/problem.ts`; `packages/contracts/src/protocol.ts` builds its messages from those shared types.

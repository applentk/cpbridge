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

Statements are scraped from the official HTML page. The adapter extracts limits and sample input/output blocks, removes redundant headers/sample sections, and falls back to a link to the official statement when parsing fails.

Verdicts are read from Codeforces `contest.status`; the adapter also has an HTML-page fallback for individual submissions. IDs with the mock `cf_` prefix are treated as invalid and become `FAILED`.

## AtCoder adapter

Source: `apps/api/internal/platform/atcoder/atcoder.go`

Recognized URLs are:

```text
https://atcoder.jp/contests/{contest}/tasks/{task}
```

The normalized external ID is `{contest}/{task}`. Metadata and statements are parsed from the AtCoder task page, including the English statement, limits, and sample cases. The adapter removes the Japanese section and common footer content.

AtCoder submission status is read from the individual submission page. IDs with the mock `ac_` prefix are treated as invalid and become `FAILED`.

## Submission responsibility

The Go adapters do not submit code. They only fetch metadata/statements and poll verdicts. Actual submission is implemented in the Chrome extension because the extension can use the user's active browser session.

## Adding another platform

1. Add a package under `apps/api/internal/platform/<name>` implementing the interface above.
2. Add a new `Type` constant in `platform.go`.
3. Register the adapter in `apps/api/cmd/server/main.go` and `apps/api/cmd/seed-demo/main.go` if the demo should use it.
4. Update `packages/contracts/src/problem.ts` and the extension protocol if the web client can submit to the platform.
5. Add URL, normalization, statement, and verdict tests.

The web client and extension currently support only the languages and platform values declared in `packages/contracts/src/problem.ts` and `packages/contracts/src/protocol.ts`.

# External Platform Adapters

Platform adapters isolate external HTTP endpoints, regex matching, and response normalization from CP Hub's core domain.

---

## Go Interface Definition

```go
type Platform interface {
    Type() Type
    MatchURL(rawURL string) (externalID string, matched bool)
    GetProblem(ctx context.Context, externalID string) (*NormalizedProblem, error)
    GetSubmission(ctx context.Context, externalSubmissionID string) (*SubmissionStatus, error)
}
```

---

## Adapter Specifications

### 1. Codeforces (`internal/platform/codeforces`)
- **URL Matches**:
  - `https://codeforces.com/problemset/problem/:contestId/:index`
  - `https://codeforces.com/contest/:contestId/problem/:index`
  - `https://codeforces.com/gym/:contestId/problem/:index`
- **External ID**: `:contestId/:index` (e.g. `1900/A`)
- **Metadata Source**: Codeforces public REST API `https://codeforces.com/api/contest.standings` with resilient fallback.

### 2. AtCoder (`internal/platform/atcoder`)
- **URL Matches**:
  - `https://atcoder.jp/contests/:contestId/tasks/:taskId`
- **External ID**: `:contestId/:taskId` (e.g. `abc350/abc350_f`)
- **Metadata Source**: AtCoder task page scraping & metadata extractor.

### 3. LeetCode (`internal/platform/leetcode`)
- **URL Matches**:
  - `https://leetcode.com/problems/:titleSlug/`
  - `https://leetcode.com/problems/:titleSlug/description/`
- **External ID**: `:titleSlug` (e.g. `two-sum`)
- **Metadata Source**: LeetCode GraphQL `questionData` query.

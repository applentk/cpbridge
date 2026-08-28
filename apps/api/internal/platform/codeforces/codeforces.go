package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	htmllib "html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/platform"
)

var (
	urlPattern1       = regexp.MustCompile(`codeforces\.com/(?:problemset/)?problem/(\d+)/([A-Za-z0-9]+)`)
	urlPattern2       = regexp.MustCompile(`codeforces\.com/contest/(\d+)/problem/([A-Za-z0-9]+)`)
	urlPattern3       = regexp.MustCompile(`codeforces\.com/gym/(\d+)/problem/([A-Za-z0-9]+)`)
	titlePrefixRegex  = regexp.MustCompile(`(?i)^[a-z](?:\s*[.\-:]\s*|\s+)`)
	problemTitleRegex = regexp.MustCompile(`(?is)<div[^>]*class=["']title["'][^>]*>(.*?)</div>`)
	htmlTitleRegex    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

	headerDivRegex        = regexp.MustCompile(`(?is)<div class="header">.*?</div>\s*</div>`)
	sampleDivRegex        = regexp.MustCompile(`(?is)<div class="sample-tests?">.*?</div>\s*</div>`)
	divTagRegex           = regexp.MustCompile(`(?is)<(/?)div\b[^>]*>`)
	classAttributeRegex   = regexp.MustCompile(`(?is)\bclass\s*=\s*["']([^"']+)["']`)
	noteTitleRegex        = regexp.MustCompile(`(?is)^\s*<div\b[^>]*class\s*=\s*["'][^"']*\bsection-title\b[^"']*["'][^>]*>\s*Note\s*:?\s*</div>`)
	timeLimitRegex        = regexp.MustCompile(`(?s)<div class="time-limit"[^>]*>.*?<div class="property-title">time limit per test</div>(.*?)</div>`)
	memoryLimitRegex      = regexp.MustCompile(`(?s)<div class="memory-limit"[^>]*>.*?<div class="property-title">memory limit per test</div>(.*?)</div>`)
	sampleInputRegex      = regexp.MustCompile(`(?s)<div class="input"><div class="title">Input</div><pre>(.*?)</pre></div>`)
	sampleOutputRegex     = regexp.MustCompile(`(?s)<div class="output"><div class="title">Output</div><pre>(.*?)</pre></div>`)
	submissionSourceRegex = regexp.MustCompile(`(?is)<pre[^>]*id=["']program-source-text["'][^>]*>(.*?)</pre>`)
)

type Adapter struct {
	client *http.Client
}

func New() *Adapter {
	return &Adapter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (a *Adapter) Type() platform.Type {
	return platform.Codeforces
}

func (a *Adapter) MatchURL(rawURL string) (string, bool) {
	if m := urlPattern1.FindStringSubmatch(rawURL); len(m) == 3 {
		return fmt.Sprintf("%s/%s", m[1], strings.ToUpper(m[2])), true
	}
	if m := urlPattern2.FindStringSubmatch(rawURL); len(m) == 3 {
		return fmt.Sprintf("%s/%s", m[1], strings.ToUpper(m[2])), true
	}
	if m := urlPattern3.FindStringSubmatch(rawURL); len(m) == 3 {
		return fmt.Sprintf("%s/%s", m[1], strings.ToUpper(m[2])), true
	}
	return "", false
}

type cfApiResponse struct {
	Status string `json:"status"`
	Result struct {
		Problems []struct {
			ContestID int      `json:"contestId"`
			Index     string   `json:"index"`
			Name      string   `json:"name"`
			Type      string   `json:"type"`
			Rating    *int     `json:"rating"`
			Tags      []string `json:"tags"`
		} `json:"problems"`
	} `json:"result"`
	Comment string `json:"comment"`
}

func (a *Adapter) GetProblem(ctx context.Context, externalID string) (*platform.NormalizedProblem, error) {
	parts := strings.Split(externalID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid codeforces external id: %s", externalID)
	}
	contestIDStr, index := parts[0], strings.ToUpper(parts[1])
	officialURL := fmt.Sprintf("https://codeforces.com/problemset/problem/%s/%s", contestIDStr, index)

	apiURL := fmt.Sprintf("https://codeforces.com/api/contest.standings?contestId=%s&from=1&count=1", contestIDStr)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		resp, err := a.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var cfResp cfApiResponse
			if err := json.NewDecoder(resp.Body).Decode(&cfResp); err == nil && cfResp.Status == "OK" {
				for _, p := range cfResp.Result.Problems {
					if strings.EqualFold(p.Index, index) {
						return &platform.NormalizedProblem{
							Platform:   platform.Codeforces,
							ExternalID: externalID,
							Title:      normalizeProblemTitle(p.Name),
							URL:        officialURL,
							Difficulty: p.Rating,
							Tags:       p.Tags,
							Metadata: map[string]any{
								"contestId": p.ContestID,
								"index":     p.Index,
							},
						}, nil
					}
				}
			}
		}
	}

	// The public API can be unavailable or can omit a problem temporarily. Use
	// the official problem page before falling back to a generic placeholder.
	if title, timeLimit, memoryLimit, ok := a.fetchProblemDetails(ctx, officialURL); ok {
		meta := map[string]any{
			"contestId": contestIDStr,
			"index":     index,
		}
		if timeLimit != "" {
			meta["timeLimit"] = timeLimit
		}
		if memoryLimit != "" {
			meta["memoryLimit"] = memoryLimit
		}
		return &platform.NormalizedProblem{
			Platform:   platform.Codeforces,
			ExternalID: externalID,
			Title:      title,
			URL:        officialURL,
			Difficulty: nil,
			Tags:       []string{"codeforces"},
			Metadata:   meta,
		}, nil
	}

	// Final fallback when both Codeforces API and page scraping are unavailable.
	contestIDNum, _ := strconv.Atoi(contestIDStr)
	return &platform.NormalizedProblem{
		Platform:   platform.Codeforces,
		ExternalID: externalID,
		Title:      fmt.Sprintf("Problem %s (%s)", index, contestIDStr),
		URL:        officialURL,
		Difficulty: nil,
		Tags:       []string{"codeforces"},
		Metadata: map[string]any{
			"contestId": contestIDNum,
			"index":     index,
		},
	}, nil
}

func (a *Adapter) fetchProblemDetails(ctx context.Context, officialURL string) (string, string, string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, officialURL, nil)
	if err != nil {
		return "", "", "", false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", "", "", false
	}
	htmlStr := string(body)
	title, ok := extractProblemTitle(htmlStr)
	if !ok {
		return "", "", "", false
	}
	var timeLimit, memoryLimit string
	if m := timeLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		timeLimit = strings.TrimSpace(cleanHTMLTags(m[1]))
	}
	if m := memoryLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		memoryLimit = strings.TrimSpace(cleanHTMLTags(m[1]))
	}
	return title, timeLimit, memoryLimit, true
}

func extractProblemTitle(html string) (string, bool) {
	if match := problemTitleRegex.FindStringSubmatch(html); len(match) > 1 {
		if title := normalizeProblemTitle(htmllib.UnescapeString(cleanHTMLTags(match[1]))); title != "" {
			return title, true
		}
	}
	if match := htmlTitleRegex.FindStringSubmatch(html); len(match) > 1 {
		title := strings.TrimSpace(htmllib.UnescapeString(cleanHTMLTags(match[1])))
		title = strings.TrimSuffix(title, " - Codeforces")
		if title := normalizeProblemTitle(title); title != "" && !strings.EqualFold(title, "problemset") {
			return title, true
		}
	}
	return "", false
}

func normalizeProblemTitle(title string) string {
	return strings.TrimSpace(titlePrefixRegex.ReplaceAllString(strings.TrimSpace(title), ""))
}

func (a *Adapter) GetStatement(ctx context.Context, externalID string) (*platform.ProblemStatement, error) {
	parts := strings.Split(externalID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid codeforces external id: %s", externalID)
	}
	contestID, index := parts[0], strings.ToUpper(parts[1])
	officialURL := fmt.Sprintf("https://codeforces.com/problemset/problem/%s/%s", contestID, index)

	req, err := http.NewRequestWithContext(ctx, "GET", officialURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch problem statement: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024*2))
	if err != nil {
		return nil, err
	}
	htmlStr := string(bodyBytes)

	statementHTML, _ := extractDivContentByClass(htmlStr, "problem-statement")

	var timeLimit, memoryLimit string
	if m := timeLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		timeLimit = strings.TrimSpace(cleanHTMLTags(m[1]))
	}
	if m := memoryLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		memoryLimit = strings.TrimSpace(cleanHTMLTags(m[1]))
	}

	// Extract sample cases
	var sampleCases []platform.SampleCase
	inputs := sampleInputRegex.FindAllStringSubmatch(htmlStr, -1)
	outputs := sampleOutputRegex.FindAllStringSubmatch(htmlStr, -1)
	minLen := min(len(inputs), len(outputs))

	for i := range minLen {
		inText := cleanSampleCode(inputs[i][1])
		outText := cleanSampleCode(outputs[i][1])
		sampleCases = append(sampleCases, platform.SampleCase{
			Input:  inText,
			Output: outText,
		})
	}

	if sampleCases == nil {
		sampleCases = []platform.SampleCase{}
	}

	var noteHTML string
	// Codeforces places notes after its sample-tests block. Samples are rendered
	// separately by the web client, so move the note out as well to preserve
	// the intended order there.
	if statementHTML != "" {
		noteHTML, statementHTML = extractNote(statementHTML)
	}

	// Strip redundant header div and duplicate sample tests from statementHTML
	if statementHTML != "" {
		statementHTML = headerDivRegex.ReplaceAllString(statementHTML, "")
		statementHTML = sampleDivRegex.ReplaceAllString(statementHTML, "")
	}

	if statementHTML == "" {
		statementHTML = fmt.Sprintf(`<p>Please refer to the official Codeforces problem statement at <a href="%s" target="_blank">%s</a>.</p>`, officialURL, officialURL)
	}

	return &platform.ProblemStatement{
		HTML:        statementHTML,
		TimeLimit:   timeLimit,
		MemoryLimit: memoryLimit,
		SampleCases: sampleCases,
		Note:        noteHTML,
	}, nil
}

func extractDivContentByClass(html, className string) (string, bool) {
	tags := divTagRegex.FindAllStringIndex(html, -1)
	for tagIndex, tagRange := range tags {
		tag := html[tagRange[0]:tagRange[1]]
		parts := divTagRegex.FindStringSubmatch(tag)
		classMatch := classAttributeRegex.FindStringSubmatch(tag)
		if len(parts) != 2 || parts[1] != "" || len(classMatch) != 2 || !hasCSSClass(classMatch[1], className) {
			continue
		}

		depth := 1
		for _, candidateRange := range tags[tagIndex+1:] {
			candidate := html[candidateRange[0]:candidateRange[1]]
			candidateParts := divTagRegex.FindStringSubmatch(candidate)
			if len(candidateParts) != 2 {
				continue
			}
			if candidateParts[1] == "" {
				depth++
				continue
			}

			depth--
			if depth == 0 {
				return html[tagRange[1]:candidateRange[0]], true
			}
		}
	}

	return "", false
}

func extractNote(statementHTML string) (string, string) {
	tags := divTagRegex.FindAllStringIndex(statementHTML, -1)
	for tagIndex, tagRange := range tags {
		tag := statementHTML[tagRange[0]:tagRange[1]]
		parts := divTagRegex.FindStringSubmatch(tag)
		classMatch := classAttributeRegex.FindStringSubmatch(tag)
		if len(parts) != 2 || parts[1] != "" || len(classMatch) != 2 || !hasCSSClass(classMatch[1], "note") {
			continue
		}

		depth := 1
		for _, closingRange := range tags[tagIndex+1:] {
			candidate := statementHTML[closingRange[0]:closingRange[1]]
			candidateParts := divTagRegex.FindStringSubmatch(candidate)
			if len(candidateParts) != 2 {
				continue
			}
			if candidateParts[1] == "" {
				depth++
				continue
			}

			depth--
			if depth != 0 {
				continue
			}

			noteHTML := statementHTML[tagRange[1]:closingRange[0]]
			noteHTML = noteTitleRegex.ReplaceAllString(noteHTML, "")
			end := closingRange[1]
			withoutNote := strings.TrimSpace(statementHTML[:tagRange[0]] + statementHTML[end:])
			return strings.TrimSpace(noteHTML), withoutNote
		}
	}

	return "", statementHTML
}

func hasCSSClass(classes, want string) bool {
	for _, className := range strings.Fields(classes) {
		if className == want {
			return true
		}
	}
	return false
}

type cfSubmissionResult struct {
	ID        int64 `json:"id"`
	ContestID int   `json:"contestId"`
	Problem   struct {
		ContestID int    `json:"contestId"`
		Index     string `json:"index"`
	} `json:"problem"`
	Author struct {
		Members []struct {
			Handle string `json:"handle"`
		} `json:"members"`
	} `json:"author"`
	ProgrammingLanguage string  `json:"programmingLanguage"`
	CreationTimeSeconds int64   `json:"creationTimeSeconds"`
	Verdict             *string `json:"verdict"`
	PassedTestCount     int     `json:"passedTestCount"`
	TimeConsumedMillis  int     `json:"timeConsumedMillis"`
	MemoryConsumedBytes int64   `json:"memoryConsumedBytes"`
}

type cfStatusApiResponse struct {
	Status  string               `json:"status"`
	Result  []cfSubmissionResult `json:"result"`
	Comment string               `json:"comment"`
}

func (a *Adapter) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) {
	if strings.HasPrefix(externalSubmissionID, "cf_") {
		return &platform.SubmissionStatus{
			ExternalSubmissionID: externalSubmissionID,
			Status:               "FAILED",
			RawPayload: map[string]any{
				"error": "Submission was never created on Codeforces (invalid or mock ID)",
			},
		}, nil
	}

	var contestID, subID string
	if strings.Contains(externalSubmissionID, "/") {
		parts := strings.Split(externalSubmissionID, "/")
		contestID, subID = parts[0], parts[1]
	} else {
		subID = externalSubmissionID
	}

	// 1. Try Codeforces Public REST API (contest.status)
	if contestID != "" {
		apiURL := fmt.Sprintf("https://codeforces.com/api/contest.status?contestId=%s&from=1&count=100", contestID)
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			resp, err := a.client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var statusResp cfStatusApiResponse
				if err := json.NewDecoder(resp.Body).Decode(&statusResp); err == nil && statusResp.Status == "OK" {
					for _, sub := range statusResp.Result {
						if strconv.FormatInt(sub.ID, 10) == subID {
							status := mapCFVerdict(sub.Verdict)
							timeMs := sub.TimeConsumedMillis
							memBytes := sub.MemoryConsumedBytes
							testcase := sub.PassedTestCount
							if status != "ACCEPTED" && status != "JUDGING" {
								testcase++
							}
							statusObj := &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               status,
								ProblemExternalID:    fmt.Sprintf("%d/%s", sub.Problem.ContestID, strings.ToUpper(sub.Problem.Index)),
								Language:             sub.ProgrammingLanguage,
								PlatformUsername:     firstCFHandle(sub.Author.Members),
								SubmittedAt:          unixTime(sub.CreationTimeSeconds),
								ExecutionTimeMs:      &timeMs,
								MemoryBytes:          &memBytes,
								FailedTestcase:       &testcase,
								RawPayload: map[string]any{
									"cfSubmissionId":  sub.ID,
									"verdict":         sub.Verdict,
									"passedTestCount": sub.PassedTestCount,
								},
							}
							if source, ok := a.fetchSubmissionSource(ctx, contestID, subID); ok {
								statusObj.SourceCode = source
							}
							return statusObj, nil
						}
					}
				}
			}
		}
	}

	// 2. Fallback to scraping Codeforces submission page
	if contestID != "" && subID != "" {
		submissionURLs := []string{
			fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", contestID, subID),
			fmt.Sprintf("https://codeforces.com/problemset/submission/%s/%s", contestID, subID),
		}

		for _, subURL := range submissionURLs {
			req, err := http.NewRequestWithContext(ctx, "GET", subURL, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			resp, err := a.client.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return &platform.SubmissionStatus{
					ExternalSubmissionID: externalSubmissionID,
					Status:               "FAILED",
					RawPayload: map[string]any{
						"error": "Submission not found on Codeforces (404 Not Found)",
					},
				}, nil
			}

			if resp.StatusCode == http.StatusOK {
				bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*500))
				if err == nil {
					htmlStr := string(bodyBytes)
					source := extractSubmissionSource(htmlStr)
					if strings.Contains(htmlStr, "verdict-accepted") || strings.Contains(htmlStr, ">Accepted<") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "ACCEPTED",
							SourceCode:           source,
						}, nil
					}
					if strings.Contains(htmlStr, "Compilation error") || strings.Contains(htmlStr, "verdict-compilation-error") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "COMPILE_ERROR",
							SourceCode:           source,
						}, nil
					}
					if strings.Contains(htmlStr, "Time limit exceeded") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "TIME_LIMIT",
							SourceCode:           source,
						}, nil
					}
					if strings.Contains(htmlStr, "Memory limit exceeded") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "MEMORY_LIMIT",
							SourceCode:           source,
						}, nil
					}
					if strings.Contains(htmlStr, "Runtime error") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "RUNTIME_ERROR",
							SourceCode:           source,
						}, nil
					}
					if strings.Contains(htmlStr, "Wrong answer") || strings.Contains(htmlStr, "verdict-rejected") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "WRONG_ANSWER",
							SourceCode:           source,
						}, nil
					}
					if strings.Contains(htmlStr, "verdict-waiting") || strings.Contains(htmlStr, "In queue") || strings.Contains(htmlStr, "Running on test") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "JUDGING",
							SourceCode:           source,
						}, nil
					}
				}
			}
		}
	}

	return &platform.SubmissionStatus{
		ExternalSubmissionID: externalSubmissionID,
		Status:               "JUDGING",
	}, nil
}

func (a *Adapter) fetchSubmissionSource(ctx context.Context, contestID, submissionID string) (string, bool) {
	for _, submissionURL := range []string{
		fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", contestID, submissionID),
		fmt.Sprintf("https://codeforces.com/problemset/submission/%s/%s", contestID, submissionID),
	} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, submissionURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		resp, err := a.client.Do(req)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*500))
		resp.Body.Close()
		if readErr == nil && resp.StatusCode == http.StatusOK {
			if source := extractSubmissionSource(string(body)); source != "" {
				return source, true
			}
		}
	}
	return "", false
}

func extractSubmissionSource(htmlStr string) string {
	match := submissionSourceRegex.FindStringSubmatch(htmlStr)
	if len(match) < 2 {
		return ""
	}
	return htmllib.UnescapeString(match[1])
}

func firstCFHandle(members []struct {
	Handle string `json:"handle"`
}) string {
	if len(members) == 0 {
		return ""
	}
	return members[0].Handle
}

func unixTime(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

func mapCFVerdict(verdict *string) string {
	if verdict == nil {
		return "JUDGING"
	}
	switch *verdict {
	case "OK":
		return "ACCEPTED"
	case "WRONG_ANSWER":
		return "WRONG_ANSWER"
	case "TIME_LIMIT_EXCEEDED":
		return "TIME_LIMIT"
	case "MEMORY_LIMIT_EXCEEDED":
		return "MEMORY_LIMIT"
	case "COMPILATION_ERROR":
		return "COMPILE_ERROR"
	case "RUNTIME_ERROR":
		return "RUNTIME_ERROR"
	case "CHALLENGED", "SKIPPED", "FAILED", "SECURITY_VIOLATED", "CRASHED", "INPUT_PREPARATION_CRASHED":
		return "FAILED"
	case "TESTING", "SUBMITTED", "PENDING":
		return "JUDGING"
	default:
		return "JUDGING"
	}
}

func cleanHTMLTags(s string) string {
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	return tagRegex.ReplaceAllString(s, "")
}

func cleanSampleCode(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, `<div class="test-example-line">`, "")
	s = strings.ReplaceAll(s, `</div>`, "\n")
	return strings.TrimSpace(cleanHTMLTags(s))
}

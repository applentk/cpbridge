package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cp-hub/api/internal/platform"
)

var (
	urlPattern1 = regexp.MustCompile(`codeforces\.com/(?:problemset/)?problem/(\d+)/([A-Za-z0-9]+)`)
	urlPattern2 = regexp.MustCompile(`codeforces\.com/contest/(\d+)/problem/([A-Za-z0-9]+)`)
	urlPattern3 = regexp.MustCompile(`codeforces\.com/gym/(\d+)/problem/([A-Za-z0-9]+)`)

	statementRegex   = regexp.MustCompile(`(?s)<div class="problem-statement">(.*?)</div>\s*<!--\s*end problem statement`)
	statementRegex2  = regexp.MustCompile(`(?s)<div class="problem-statement">(.*)`)
	headerDivRegex   = regexp.MustCompile(`(?is)<div class="header">.*?</div>\s*</div>`)
	sampleDivRegex   = regexp.MustCompile(`(?is)<div class="sample-tests?">.*?</div>\s*</div>`)
	timeLimitRegex   = regexp.MustCompile(`(?s)<div class="time-limit"[^>]*>.*?<div class="property-title">time limit per test</div>(.*?)</div>`)
	memoryLimitRegex = regexp.MustCompile(`(?s)<div class="memory-limit"[^>]*>.*?<div class="property-title">memory limit per test</div>(.*?)</div>`)
	sampleInputRegex = regexp.MustCompile(`(?s)<div class="input"><div class="title">Input</div><pre>(.*?)</pre></div>`)
	sampleOutputRegex = regexp.MustCompile(`(?s)<div class="output"><div class="title">Output</div><pre>(.*?)</pre></div>`)
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
		req.Header.Set("User-Agent", "Mozilla/5.0 CPHub/1.0")
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
							Title:      fmt.Sprintf("%s. %s", index, p.Name),
							URL:        officialURL,
							Difficulty: p.Rating,
							Tags:       p.Tags,
							Metadata: map[string]interface{}{
								"contestId": p.ContestID,
								"index":     p.Index,
							},
						}, nil
					}
				}
			}
		}
	}

	// Fallback when Codeforces API is slow/unavailable
	contestIDNum, _ := strconv.Atoi(contestIDStr)
	return &platform.NormalizedProblem{
		Platform:   platform.Codeforces,
		ExternalID: externalID,
		Title:      fmt.Sprintf("Problem %s (%s)", index, contestIDStr),
		URL:        officialURL,
		Difficulty: nil,
		Tags:       []string{"codeforces"},
		Metadata: map[string]interface{}{
			"contestId": contestIDNum,
			"index":     index,
		},
	}, nil
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

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

	var statementHTML string
	if m := statementRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		statementHTML = m[1]
	} else if m := statementRegex2.FindStringSubmatch(htmlStr); len(m) > 1 {
		statementHTML = m[1]
	}

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
	minLen := len(inputs)
	if len(outputs) < minLen {
		minLen = len(outputs)
	}

	for i := 0; i < minLen; i++ {
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
	}, nil
}

type cfSubmissionResult struct {
	ID                  int64   `json:"id"`
	ContestID           int     `json:"contestId"`
	Verdict             *string `json:"verdict"`
	PassedTestCount     int     `json:"passedTestCount"`
	TimeConsumedMillis  int     `json:"timeConsumedMillis"`
	MemoryConsumedBytes int64   `json:"memoryConsumedBytes"`
}

type cfStatusApiResponse struct {
	Status string               `json:"status"`
	Result []cfSubmissionResult `json:"result"`
	Comment string              `json:"comment"`
}

func (a *Adapter) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) {
	if strings.HasPrefix(externalSubmissionID, "cf_") {
		return &platform.SubmissionStatus{
			ExternalSubmissionID: externalSubmissionID,
			Status:               "JUDGING",
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
			req.Header.Set("User-Agent", "Mozilla/5.0 CPHub/1.0")
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
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               status,
								ExecutionTimeMs:      &timeMs,
								MemoryBytes:          &memBytes,
								FailedTestcase:       &testcase,
								RawPayload: map[string]interface{}{
									"cfSubmissionId":  sub.ID,
									"verdict":         sub.Verdict,
									"passedTestCount": sub.PassedTestCount,
								},
							}, nil
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
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
			resp, err := a.client.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*500))
				if err == nil {
					htmlStr := string(bodyBytes)
					if strings.Contains(htmlStr, "verdict-accepted") || strings.Contains(htmlStr, ">Accepted<") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "ACCEPTED",
						}, nil
					}
					if strings.Contains(htmlStr, "verdict-rejected") || strings.Contains(htmlStr, "Wrong answer") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "WRONG_ANSWER",
						}, nil
					}
					if strings.Contains(htmlStr, "Time limit exceeded") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "TIME_LIMIT",
						}, nil
					}
					if strings.Contains(htmlStr, "Memory limit exceeded") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "MEMORY_LIMIT",
						}, nil
					}
					if strings.Contains(htmlStr, "Runtime error") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "RUNTIME_ERROR",
						}, nil
					}
					if strings.Contains(htmlStr, "Compilation error") {
						return &platform.SubmissionStatus{
							ExternalSubmissionID: externalSubmissionID,
							Status:               "COMPILE_ERROR",
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

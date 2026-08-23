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

func (a *Adapter) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) {
	return &platform.SubmissionStatus{
		ExternalSubmissionID: externalSubmissionID,
		Status:               "JUDGING",
	}, nil
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

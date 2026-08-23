package atcoder

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cp-hub/api/internal/platform"
)

var (
	urlPattern         = regexp.MustCompile(`atcoder\.jp/contests/([a-zA-Z0-9_\-]+)/tasks/([a-zA-Z0-9_\-]+)`)
	titleRegex         = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)
	titlePrefixRegex   = regexp.MustCompile(`(?i)^[a-z0-9]+\s*[-.:)]\s*`)
	contestSuffixRegex = regexp.MustCompile(`(?i)\s+-\s+atcoder.*$`)

	taskStatementRegex  = regexp.MustCompile(`(?s)<div id="task-statement">(.*?)</div>\s*<span class="center-block`)
	taskStatementRegex2 = regexp.MustCompile(`(?s)<div id="task-statement">(.*)`)
	langEnRegex         = regexp.MustCompile(`(?s)<span class="lang-en">(.*?)</span>\s*</span>`)
	langJaRegex         = regexp.MustCompile(`(?is)<span class="lang-ja">.*?</span>`)
	timeLimitRegex      = regexp.MustCompile(`Time Limit:\s*([0-9\.]+\s*sec)`)
	memoryLimitRegex    = regexp.MustCompile(`Memory Limit:\s*([0-9\.]+\s*MB)`)
	sampleInputRegex    = regexp.MustCompile(`(?s)<h3>\s*Sample Input\s*\d*\s*</h3>\s*<pre>(.*?)</pre>`)
	sampleOutputRegex   = regexp.MustCompile(`(?s)<h3>\s*Sample Output\s*\d*\s*</h3>\s*<pre>(.*?)</pre>`)

	// Cleaners for footer and duplicate sample blocks in statement html
	sampleSectionRegex = regexp.MustCompile(`(?is)<hr\s*/?>\s*<div class="part">\s*<section>\s*<h3>\s*Sample (?:Input|Output).*`)
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
	return platform.AtCoder
}

func (a *Adapter) MatchURL(rawURL string) (string, bool) {
	if m := urlPattern.FindStringSubmatch(rawURL); len(m) == 3 {
		return fmt.Sprintf("%s/%s", m[1], m[2]), true
	}
	return "", false
}

func (a *Adapter) GetProblem(ctx context.Context, externalID string) (*platform.NormalizedProblem, error) {
	parts := strings.Split(externalID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid atcoder external id: %s", externalID)
	}
	contestID, taskID := parts[0], parts[1]
	officialURL := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", contestID, taskID)

	title := fmt.Sprintf("%s (%s)", taskID, contestID)
	req, err := http.NewRequestWithContext(ctx, "GET", officialURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 CPHub/1.0")
		resp, err := a.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*100))
			if m := titleRegex.FindStringSubmatch(string(bodyBytes)); len(m) == 2 {
				cleanTitle := normalizeProblemTitle(m[1])
				if cleanTitle != "" {
					title = cleanTitle
				}
			}
		}
	}

	return &platform.NormalizedProblem{
		Platform:   platform.AtCoder,
		ExternalID: externalID,
		Title:      title,
		URL:        officialURL,
		Difficulty: nil,
		Tags:       []string{"atcoder", contestID},
		Metadata: map[string]interface{}{
			"contestId": contestID,
			"taskId":    taskID,
		},
	}, nil
}

func normalizeProblemTitle(title string) string {
	title = html.UnescapeString(strings.TrimSpace(title))
	title = contestSuffixRegex.ReplaceAllString(title, "")
	title = titlePrefixRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func (a *Adapter) GetStatement(ctx context.Context, externalID string) (*platform.ProblemStatement, error) {
	parts := strings.Split(externalID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid atcoder external id: %s", externalID)
	}
	contestID, taskID := parts[0], parts[1]
	officialURL := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", contestID, taskID)

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
	if m := langEnRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		statementHTML = m[1]
	} else if m := taskStatementRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		statementHTML = m[1]
	} else if m := taskStatementRegex2.FindStringSubmatch(htmlStr); len(m) > 1 {
		statementHTML = m[1]
	}

	var timeLimit, memoryLimit string
	if m := timeLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		timeLimit = m[1]
	}
	if m := memoryLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		memoryLimit = m[1]
	}

	var sampleCases []platform.SampleCase
	inputs := sampleInputRegex.FindAllStringSubmatch(htmlStr, -1)
	outputs := sampleOutputRegex.FindAllStringSubmatch(htmlStr, -1)
	minLen := len(inputs)
	if len(outputs) < minLen {
		minLen = len(outputs)
	}

	for i := 0; i < minLen; i++ {
		sampleCases = append(sampleCases, platform.SampleCase{
			Input:  strings.TrimSpace(inputs[i][1]),
			Output: strings.TrimSpace(outputs[i][1]),
		})
	}

	if sampleCases == nil {
		sampleCases = []platform.SampleCase{}
	}

	// Clean statement HTML
	if statementHTML != "" {
		statementHTML = langJaRegex.ReplaceAllString(statementHTML, "")
		statementHTML = sampleSectionRegex.ReplaceAllString(statementHTML, "")
	}

	if statementHTML == "" {
		statementHTML = fmt.Sprintf(`<p>Please refer to the official AtCoder statement at <a href="%s" target="_blank">%s</a>.</p>`, officialURL, officialURL)
	}

	return &platform.ProblemStatement{
		HTML:        statementHTML,
		TimeLimit:   timeLimit,
		MemoryLimit: memoryLimit,
		SampleCases: sampleCases,
	}, nil
}

func (a *Adapter) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) {
	if strings.HasPrefix(externalSubmissionID, "ac_") {
		return &platform.SubmissionStatus{
			ExternalSubmissionID: externalSubmissionID,
			Status:               "FAILED",
			RawPayload: map[string]interface{}{
				"error": "Submission was never created on AtCoder (invalid or mock ID)",
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

	if contestID != "" && subID != "" {
		submissionURL := fmt.Sprintf("https://atcoder.jp/contests/%s/submissions/%s", contestID, subID)
		req, err := http.NewRequestWithContext(ctx, "GET", submissionURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
			resp, err := a.client.Do(req)
			if err == nil {
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusNotFound {
					return &platform.SubmissionStatus{
						ExternalSubmissionID: externalSubmissionID,
						Status:               "FAILED",
						RawPayload: map[string]interface{}{
							"error": "Submission not found on AtCoder (404 Not Found)",
						},
					}, nil
				}

				if resp.StatusCode == http.StatusOK {
					bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*500))
					if err == nil {
						htmlStr := string(bodyBytes)
						if strings.Contains(htmlStr, ">AC</span>") || strings.Contains(htmlStr, "label-success") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "ACCEPTED",
							}, nil
						}
						if strings.Contains(htmlStr, ">WA</span>") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "WRONG_ANSWER",
							}, nil
						}
						if strings.Contains(htmlStr, ">TLE</span>") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "TIME_LIMIT",
							}, nil
						}
						if strings.Contains(htmlStr, ">MLE</span>") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "MEMORY_LIMIT",
							}, nil
						}
						if strings.Contains(htmlStr, ">RE</span>") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "RUNTIME_ERROR",
							}, nil
						}
						if strings.Contains(htmlStr, ">CE</span>") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "COMPILE_ERROR",
							}, nil
						}
						if strings.Contains(htmlStr, ">OLE</span>") || strings.Contains(htmlStr, ">QLE</span>") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "FAILED",
							}, nil
						}
						if strings.Contains(htmlStr, ">WJ</span>") || strings.Contains(htmlStr, ">WR</span>") || strings.Contains(htmlStr, "label-default") {
							return &platform.SubmissionStatus{
								ExternalSubmissionID: externalSubmissionID,
								Status:               "JUDGING",
							}, nil
						}
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

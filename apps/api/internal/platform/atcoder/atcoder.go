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

	"github.com/cpbridge/api/internal/platform"
)

var (
	urlPattern         = regexp.MustCompile(`atcoder\.jp/contests/([a-zA-Z0-9_\-]+)/tasks/([a-zA-Z0-9_\-]+)`)
	titleRegex         = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)
	titlePrefixRegex   = regexp.MustCompile(`(?i)^[a-z0-9]+\s*[-.:)]\s*`)
	contestSuffixRegex = regexp.MustCompile(`(?i)\s+-\s+atcoder.*$`)

	timeLimitRegex          = regexp.MustCompile(`Time Limit:\s*([0-9\.]+\s*sec)`)
	memoryLimitRegex        = regexp.MustCompile(`Memory Limit:\s*([0-9\.]+\s*(?:Mi?B|KB))`)
	htmlTagRegex            = regexp.MustCompile(`(?is)<(/?)([a-z][a-z0-9:-]*)\b[^>]*>`)
	idAttributeRegex        = regexp.MustCompile(`(?is)\bid\s*=\s*["']([^"']+)["']`)
	classAttributeRegex     = regexp.MustCompile(`(?is)\bclass\s*=\s*["']([^"']+)["']`)
	submissionTaskRegex     = regexp.MustCompile(`(?is)<th[^>]*>\s*Task\s*</th>\s*<td[^>]*>.*?/contests/([^/"']+)/tasks/([^"']+)`)
	submissionLanguageRegex = regexp.MustCompile(`(?is)<th[^>]*>\s*Language\s*</th>\s*<td[^>]*>(.*?)</td>`)
	submissionUserRegex     = regexp.MustCompile(`(?is)<a[^>]+href=["']/users/([^"']+)["']`)
	submissionTimeRegex     = regexp.MustCompile(`(?is)<th[^>]*>\s*(?:Submitted At|Submission Time)\s*</th>\s*<td[^>]*>(.*?)</td>`)
	sampleInputRegex        = regexp.MustCompile(`(?is)<h3\b[^>]*>\s*Sample Input\s*\d*\s*</h3>\s*<pre[^>]*>(.*?)</pre>`)
	sampleOutputRegex       = regexp.MustCompile(`(?is)<h3\b[^>]*>\s*Sample Output\s*\d*\s*</h3>\s*<pre[^>]*>(.*?)</pre>`)

	// Cleaners for footer and duplicate sample blocks in statement html
	sampleHeadingRegex           = regexp.MustCompile(`(?is)<h3\b[^>]*>\s*Sample (?:Input|Output)\s*\d*\s*</h3>`)
	trailingHorizontalRuleRegex  = regexp.MustCompile(`(?is)<hr\s*/?>\s*$`)
	atcoderScoreHeadingRegex     = regexp.MustCompile(`(?is)<h[1-6][^>]*>\s*Score\s*:\s*.*?</h[1-6]>`)
	atcoderScoreParagraphRegex   = regexp.MustCompile(`(?is)<p[^>]*>\s*Score\s*:.*?</p>`)
	atcoderStatementHeadingRegex = regexp.MustCompile(`(?is)<h[1-6][^>]*>\s*(?:###\s*)?Problem Statement\s*</h[1-6]>`)
	atcoderScoreTextRegex        = regexp.MustCompile(`(?im)^\s*Score\s*:\s*[^<\r\n]+\s*(?:\r?\n|$)`)
	atcoderStatementTextRegex    = regexp.MustCompile(`(?im)^\s*###\s*Problem Statement\s*(?:\r?\n|$)`)
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

	htmlStr, err := a.fetchTaskPage(ctx, officialURL)
	if err != nil {
		return nil, err
	}
	if _, ok := taskStatementHTML(htmlStr); !ok {
		return nil, fmt.Errorf("AtCoder response did not include a task statement")
	}

	title := ""
	if m := titleRegex.FindStringSubmatch(htmlStr); len(m) == 2 {
		title = normalizeProblemTitle(m[1])
	}
	if title == "" {
		return nil, fmt.Errorf("AtCoder response did not include a problem title")
	}

	meta := map[string]any{
		"contestId": contestID,
		"taskId":    taskID,
	}
	if m := timeLimitRegex.FindStringSubmatch(htmlStr); len(m) == 2 {
		if tl := strings.TrimSpace(m[1]); tl != "" {
			meta["timeLimit"] = tl
		}
	}
	if m := memoryLimitRegex.FindStringSubmatch(htmlStr); len(m) == 2 {
		if ml := strings.TrimSpace(m[1]); ml != "" {
			meta["memoryLimit"] = ml
		}
	}

	return &platform.NormalizedProblem{
		Platform:   platform.AtCoder,
		ExternalID: externalID,
		Title:      title,
		URL:        officialURL,
		Difficulty: nil,
		Tags:       []string{"atcoder", contestID},
		Metadata:   meta,
	}, nil
}

func normalizeProblemTitle(title string) string {
	title = html.UnescapeString(strings.TrimSpace(title))
	title = contestSuffixRegex.ReplaceAllString(title, "")
	title = titlePrefixRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func (a *Adapter) fetchTaskPage(ctx context.Context, officialURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, officialURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch AtCoder problem: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AtCoder returned %s for problem", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read AtCoder problem response: %w", err)
	}
	return string(body), nil
}

// extractTaskStatement gets the English statement when available. It finds the
// matching element boundary instead of relying on the next closing tag, since
// AtCoder statements contain nested div and span elements.
func extractTaskStatement(htmlStr string) string {
	taskStatement, ok := taskStatementHTML(htmlStr)
	if !ok {
		return ""
	}

	if english, ok := findElementInnerHTML(taskStatement, "span", func(openTag string) bool {
		return hasCSSClass(attributeValue(openTag, classAttributeRegex), "lang-en")
	}); ok {
		return english
	}

	return taskStatement
}

func taskStatementHTML(htmlStr string) (string, bool) {
	return findElementInnerHTML(htmlStr, "div", func(openTag string) bool {
		return strings.EqualFold(attributeValue(openTag, idAttributeRegex), "task-statement")
	})
}

func findElementInnerHTML(htmlStr, element string, match func(openTag string) bool) (string, bool) {
	tags := htmlTagRegex.FindAllStringIndex(htmlStr, -1)
	for tagIndex, tagRange := range tags {
		tag := htmlStr[tagRange[0]:tagRange[1]]
		parts := htmlTagRegex.FindStringSubmatch(tag)
		if len(parts) != 3 || parts[1] == "/" || !strings.EqualFold(parts[2], element) || !match(tag) {
			continue
		}

		depth := 1
		for _, closingRange := range tags[tagIndex+1:] {
			candidate := htmlStr[closingRange[0]:closingRange[1]]
			candidateParts := htmlTagRegex.FindStringSubmatch(candidate)
			if len(candidateParts) != 3 || !strings.EqualFold(candidateParts[2], element) {
				continue
			}
			if candidateParts[1] == "/" {
				depth--
				if depth == 0 {
					return htmlStr[tagRange[1]:closingRange[0]], true
				}
			} else if !strings.HasSuffix(strings.TrimSpace(candidate), "/>") {
				depth++
			}
		}
	}

	return "", false
}

func attributeValue(openTag string, attributeRegex *regexp.Regexp) string {
	if match := attributeRegex.FindStringSubmatch(openTag); len(match) == 2 {
		return match[1]
	}
	return ""
}

func hasCSSClass(classes, want string) bool {
	for _, class := range strings.Fields(classes) {
		if class == want {
			return true
		}
	}
	return false
}

func (a *Adapter) GetStatement(ctx context.Context, externalID string) (*platform.ProblemStatement, error) {
	parts := strings.Split(externalID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid atcoder external id: %s", externalID)
	}
	contestID, taskID := parts[0], parts[1]
	officialURL := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", contestID, taskID)

	htmlStr, err := a.fetchTaskPage(ctx, officialURL)
	if err != nil {
		return nil, err
	}

	statementHTML := extractTaskStatement(htmlStr)

	var timeLimit, memoryLimit string
	if m := timeLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		timeLimit = m[1]
	}
	if m := memoryLimitRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		memoryLimit = m[1]
	}

	var sampleCases []platform.SampleCase
	inputs := sampleInputRegex.FindAllStringSubmatch(statementHTML, -1)
	outputs := sampleOutputRegex.FindAllStringSubmatch(statementHTML, -1)
	minLen := min(len(inputs), len(outputs))

	for i := range minLen {
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
		statementHTML = cleanStatementHTML(statementHTML)
		statementHTML = removeSampleSections(statementHTML)
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

func cleanStatementHTML(statementHTML string) string {
	statementHTML = atcoderScoreHeadingRegex.ReplaceAllString(statementHTML, "")
	statementHTML = atcoderScoreParagraphRegex.ReplaceAllString(statementHTML, "")
	statementHTML = atcoderStatementHeadingRegex.ReplaceAllString(statementHTML, "")
	statementHTML = atcoderScoreTextRegex.ReplaceAllString(statementHTML, "")
	statementHTML = atcoderStatementTextRegex.ReplaceAllString(statementHTML, "")
	return strings.TrimSpace(statementHTML)
}

func removeSampleSections(statementHTML string) string {
	if sampleStart := sampleHeadingRegex.FindStringIndex(statementHTML); sampleStart != nil {
		statementHTML = statementHTML[:sampleStart[0]]
		statementHTML = trailingHorizontalRuleRegex.ReplaceAllString(statementHTML, "")
	}
	return strings.TrimSpace(statementHTML)
}

func (a *Adapter) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) {
	if strings.HasPrefix(externalSubmissionID, "ac_") {
		return &platform.SubmissionStatus{
			ExternalSubmissionID: externalSubmissionID,
			Status:               "FAILED",
			RawPayload: map[string]any{
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
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			resp, err := a.client.Do(req)
			if err == nil {
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusNotFound {
					return &platform.SubmissionStatus{
						ExternalSubmissionID: externalSubmissionID,
						Status:               "FAILED",
						RawPayload: map[string]any{
							"error": "Submission not found on AtCoder (404 Not Found)",
						},
					}, nil
				}

				if resp.StatusCode == http.StatusOK {
					bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*500))
					if err == nil {
						htmlStr := string(bodyBytes)
						verified := parseSubmissionMetadata(htmlStr, contestID)
						verified.ExternalSubmissionID = externalSubmissionID
						if strings.Contains(htmlStr, ">AC</span>") || strings.Contains(htmlStr, "label-success") {
							verified.Status = "ACCEPTED"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">WA</span>") {
							verified.Status = "WRONG_ANSWER"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">TLE</span>") {
							verified.Status = "TIME_LIMIT"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">MLE</span>") {
							verified.Status = "MEMORY_LIMIT"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">RE</span>") {
							verified.Status = "RUNTIME_ERROR"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">CE</span>") {
							verified.Status = "COMPILE_ERROR"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">OLE</span>") || strings.Contains(htmlStr, ">QLE</span>") {
							verified.Status = "FAILED"
							return verified, nil
						}
						if strings.Contains(htmlStr, ">WJ</span>") || strings.Contains(htmlStr, ">WR</span>") || strings.Contains(htmlStr, "label-default") {
							verified.Status = "JUDGING"
							return verified, nil
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

func parseSubmissionMetadata(htmlStr, contestID string) *platform.SubmissionStatus {
	status := &platform.SubmissionStatus{}
	if match := submissionTaskRegex.FindStringSubmatch(htmlStr); len(match) == 3 {
		status.ProblemExternalID = fmt.Sprintf("%s/%s", match[1], match[2])
	}
	if match := submissionLanguageRegex.FindStringSubmatch(htmlStr); len(match) == 2 {
		status.Language = cleanHTMLTags(match[1])
	}
	if match := submissionUserRegex.FindStringSubmatch(htmlStr); len(match) == 2 {
		status.PlatformUsername = html.UnescapeString(strings.TrimSpace(match[1]))
	}
	if match := submissionTimeRegex.FindStringSubmatch(htmlStr); len(match) == 2 {
		value := cleanHTMLTags(match[1])
		for _, layout := range []string{"2006-01-02 15:04:05-0700", "2006-01-02 15:04:05 MST", time.RFC3339} {
			if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
				parsed = parsed.UTC()
				status.SubmittedAt = &parsed
				break
			}
		}
	}
	if status.ProblemExternalID == "" && contestID != "" {
		status.RawPayload = map[string]any{"metadataIncomplete": true}
	}
	return status
}

func cleanHTMLTags(value string) string {
	return strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html.UnescapeString(value), ""))
}

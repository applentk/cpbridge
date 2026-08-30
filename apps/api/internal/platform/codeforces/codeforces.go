package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	htmllib "html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/platform"
)

var (
	urlPattern1             = regexp.MustCompile(`codeforces\.com/(?:problemset/)?problem/(\d+)/([A-Za-z0-9]+)`)
	urlPattern2             = regexp.MustCompile(`codeforces\.com/contest/(\d+)/problem/([A-Za-z0-9]+)`)
	urlPattern3             = regexp.MustCompile(`codeforces\.com/gym/(\d+)/problem/([A-Za-z0-9]+)`)
	contestURLPattern       = regexp.MustCompile(`(?i)^(?:https?://)?(?:www\.)?codeforces\.com/contest/(\d+)(?:/?(?:\?.*)?)?$`)
	gymContestURLPattern    = regexp.MustCompile(`(?i)^(?:https?://)?(?:www\.)?codeforces\.com/gym/(\d+)(?:/?(?:\?.*)?)?$`)
	contestIDPattern        = regexp.MustCompile(`^\d+$`)
	gymExternalIDPattern    = regexp.MustCompile(`^gym/(\d+)$`)
	contestTitleRegex       = regexp.MustCompile(`(?is)<a[^>]+href=["']/contest/(\d+)["'][^>]*>(.*?)</a>`)
	contestProblemLinkRegex = regexp.MustCompile(`(?is)<a[^>]+href=["']/contest/(\d+)/problem/([A-Za-z0-9]+)["'][^>]*>(.*?)</a>`)
	gymContestTitleRegex    = regexp.MustCompile(`(?is)<a[^>]+href=["']/gym/(\d+)["'][^>]*>(.*?)</a>`)
	gymProblemLinkRegex     = regexp.MustCompile(`(?is)<a[^>]+href=["']/gym/(\d+)/problem/([A-Za-z0-9]+)["'][^>]*>(.*?)</a>`)
	titlePrefixRegex        = regexp.MustCompile(`(?i)^[a-z](?:\s*[.\-:]\s*|\s+)`)
	problemTitleRegex       = regexp.MustCompile(`(?is)<div[^>]*class=["']title["'][^>]*>(.*?)</div>`)
	htmlTitleRegex          = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

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
	client  *http.Client
	baseURL string
}

func New() *Adapter {
	return &Adapter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://codeforces.com",
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
		return fmt.Sprintf("gym/%s/%s", m[1], strings.ToUpper(m[2])), true
	}
	return "", false
}

func (a *Adapter) MatchContestURL(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if match := contestURLPattern.FindStringSubmatch(rawURL); len(match) == 2 {
		return match[1], true
	}
	if match := gymContestURLPattern.FindStringSubmatch(rawURL); len(match) == 2 {
		return "gym/" + match[1], true
	}
	return "", false
}

func (a *Adapter) GetContest(ctx context.Context, externalID string) (*platform.ContestSnapshot, error) {
	if match := gymExternalIDPattern.FindStringSubmatch(externalID); len(match) == 2 {
		return a.getGymContest(ctx, match[1])
	}
	if !contestIDPattern.MatchString(externalID) {
		return nil, fmt.Errorf("invalid codeforces contest id: %s", externalID)
	}

	return a.getRegularContest(ctx, externalID)
}

func (a *Adapter) getRegularContest(ctx context.Context, contestID string) (*platform.ContestSnapshot, error) {
	pageURL := fmt.Sprintf("%s/contest/%s", a.baseURL, contestID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to create request for contest %s (%s): %v", contestID, pageURL, err)
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] HTTP request failed for contest %s (%s): %v", contestID, pageURL, err)
		return nil, fmt.Errorf("failed to fetch Codeforces contest: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Platform:Codeforces:Error] Contest %s (%s) returned status %d %s | Body: %s", contestID, pageURL, resp.StatusCode, resp.Status, previewBody(body, 1000))
		return nil, fmt.Errorf("Codeforces returned status %d", resp.StatusCode)
	}
	if readErr != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to read contest response body for %s: %v", contestID, readErr)
		return nil, fmt.Errorf("failed to read Codeforces contest: %w", readErr)
	}
	htmlStr := string(body)

	name := ""
	for _, match := range contestTitleRegex.FindAllStringSubmatch(htmlStr, -1) {
		if len(match) == 3 && match[1] == contestID {
			name = strings.TrimSpace(htmllib.UnescapeString(cleanHTMLTags(match[2])))
			if name != "" {
				break
			}
		}
	}
	if name == "" {
		if match := htmlTitleRegex.FindStringSubmatch(htmlStr); len(match) > 1 {
			title := strings.TrimSpace(htmllib.UnescapeString(cleanHTMLTags(match[1])))
			title = strings.TrimSuffix(title, " - Codeforces")
			if title != "" && !strings.EqualFold(title, "codeforces") {
				name = title
			}
		}
	}
	if name == "" {
		name = fmt.Sprintf("Codeforces Contest %s", contestID)
	}

	type problemItem struct {
		index string
		title string
	}
	ordered := make([]problemItem, 0)
	positions := make(map[string]int)
	for _, match := range contestProblemLinkRegex.FindAllStringSubmatch(htmlStr, -1) {
		if len(match) != 4 || match[1] != contestID {
			continue
		}
		index := strings.ToUpper(strings.TrimSpace(match[2]))
		text := strings.TrimSpace(htmllib.UnescapeString(cleanHTMLTags(match[3])))
		position, exists := positions[index]
		if !exists {
			positions[index] = len(ordered)
			ordered = append(ordered, problemItem{index: index})
			position = len(ordered) - 1
		}
		if text != "" && !strings.EqualFold(text, index) {
			ordered[position].title = text
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("Codeforces contest is private, unrevealed, or contains no importable problems")
	}

	contestIDNumber, _ := strconv.Atoi(contestID)
	problems := make([]platform.NormalizedProblem, 0, len(ordered))
	for _, item := range ordered {
		if item.title == "" {
			return nil, fmt.Errorf("Codeforces contest returned problem %s without a title", item.index)
		}
		problems = append(problems, platform.NormalizedProblem{
			Platform:   platform.Codeforces,
			ExternalID: fmt.Sprintf("%s/%s", contestID, item.index),
			Title:      item.title,
			URL:        fmt.Sprintf("https://codeforces.com/problemset/problem/%s/%s", contestID, item.index),
			Difficulty: nil,
			Tags:       []string{"codeforces"},
			Metadata: map[string]any{
				"contestId": contestIDNumber,
				"index":     item.index,
			},
		})
	}

	return &platform.ContestSnapshot{
		Platform:   platform.Codeforces,
		ExternalID: contestID,
		Name:       name,
		URL:        fmt.Sprintf("https://codeforces.com/contest/%s", contestID),
		Phase:      "AVAILABLE",
		Problems:   problems,
	}, nil
}

func (a *Adapter) getGymContest(ctx context.Context, gymID string) (*platform.ContestSnapshot, error) {
	pageURL := fmt.Sprintf("%s/gym/%s", a.baseURL, gymID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to create request for gym contest %s (%s): %v", gymID, pageURL, err)
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] HTTP request failed for gym contest %s (%s): %v", gymID, pageURL, err)
		return nil, fmt.Errorf("failed to fetch Codeforces Gym contest: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Platform:Codeforces:Error] Gym contest %s (%s) returned status %d %s | Body: %s", gymID, pageURL, resp.StatusCode, resp.Status, previewBody(body, 1000))
		return nil, fmt.Errorf("Codeforces Gym returned status %d", resp.StatusCode)
	}
	if readErr != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to read gym contest response body for %s: %v", gymID, readErr)
		return nil, fmt.Errorf("failed to read Codeforces Gym contest: %w", readErr)
	}
	htmlStr := string(body)

	name := ""
	for _, match := range gymContestTitleRegex.FindAllStringSubmatch(htmlStr, -1) {
		if len(match) == 3 && match[1] == gymID {
			name = strings.TrimSpace(htmllib.UnescapeString(cleanHTMLTags(match[2])))
			if name != "" {
				break
			}
		}
	}
	if name == "" {
		name = fmt.Sprintf("Codeforces Gym %s", gymID)
	}

	type gymProblem struct {
		index string
		title string
	}
	ordered := make([]gymProblem, 0)
	positions := make(map[string]int)
	for _, match := range gymProblemLinkRegex.FindAllStringSubmatch(htmlStr, -1) {
		if len(match) != 4 || match[1] != gymID {
			continue
		}
		index := strings.ToUpper(strings.TrimSpace(match[2]))
		text := strings.TrimSpace(htmllib.UnescapeString(cleanHTMLTags(match[3])))
		position, exists := positions[index]
		if !exists {
			positions[index] = len(ordered)
			ordered = append(ordered, gymProblem{index: index})
			position = len(ordered) - 1
		}
		if text != "" && !strings.EqualFold(text, index) {
			ordered[position].title = text
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("Codeforces Gym contest is private, unrevealed, or contains no importable problems")
	}

	gymIDNumber, _ := strconv.Atoi(gymID)
	problems := make([]platform.NormalizedProblem, 0, len(ordered))
	for _, item := range ordered {
		if item.title == "" {
			return nil, fmt.Errorf("Codeforces Gym returned problem %s without a title", item.index)
		}
		problems = append(problems, platform.NormalizedProblem{
			Platform:   platform.Codeforces,
			ExternalID: fmt.Sprintf("gym/%s/%s", gymID, item.index),
			Title:      item.title,
			URL:        fmt.Sprintf("https://codeforces.com/gym/%s/problem/%s", gymID, item.index),
			Difficulty: nil,
			Tags:       []string{"codeforces", "gym"},
			Metadata: map[string]any{
				"contestId": gymIDNumber,
				"index":     item.index,
				"gym":       true,
			},
		})
	}

	return &platform.ContestSnapshot{
		Platform:   platform.Codeforces,
		ExternalID: "gym/" + gymID,
		Name:       name,
		URL:        fmt.Sprintf("https://codeforces.com/gym/%s", gymID),
		Phase:      "AVAILABLE",
		Problems:   problems,
	}, nil
}

func (a *Adapter) GetProblem(ctx context.Context, externalID string) (*platform.NormalizedProblem, error) {
	contestIDStr, index, isGym, err := parseProblemRef(externalID)
	if err != nil {
		return nil, err
	}
	officialURL := codeforcesProblemURL(contestIDStr, index, isGym)
	if isGym {
		return a.getGymProblem(ctx, externalID, contestIDStr, index, officialURL)
	}

	// Fetch problem details from the official problem page.
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

func (a *Adapter) getGymProblem(ctx context.Context, externalID, gymID, index, officialURL string) (*platform.NormalizedProblem, error) {
	if title, timeLimit, memoryLimit, ok := a.fetchProblemDetails(ctx, officialURL); ok {
		metadata := map[string]any{
			"contestId": gymID,
			"index":     index,
			"gym":       true,
		}
		if timeLimit != "" {
			metadata["timeLimit"] = timeLimit
		}
		if memoryLimit != "" {
			metadata["memoryLimit"] = memoryLimit
		}
		return &platform.NormalizedProblem{
			Platform:   platform.Codeforces,
			ExternalID: externalID,
			Title:      title,
			URL:        officialURL,
			Difficulty: nil,
			Tags:       []string{"codeforces", "gym"},
			Metadata:   metadata,
		}, nil
	}

	return nil, fmt.Errorf("failed to fetch Codeforces Gym problem %s", externalID)
}

func parseProblemRef(externalID string) (contestID, index string, isGym bool, err error) {
	parts := strings.Split(strings.TrimSpace(externalID), "/")
	switch {
	case len(parts) == 2 && contestIDPattern.MatchString(parts[0]) && strings.TrimSpace(parts[1]) != "":
		return parts[0], strings.ToUpper(parts[1]), false, nil
	case len(parts) == 3 && strings.EqualFold(parts[0], "gym") && contestIDPattern.MatchString(parts[1]) && strings.TrimSpace(parts[2]) != "":
		return parts[1], strings.ToUpper(parts[2]), true, nil
	default:
		return "", "", false, fmt.Errorf("invalid codeforces external id: %s", externalID)
	}
}

func codeforcesProblemURL(contestID, index string, isGym bool) string {
	if isGym {
		return fmt.Sprintf("https://codeforces.com/gym/%s/problem/%s", contestID, index)
	}
	return fmt.Sprintf("https://codeforces.com/problemset/problem/%s/%s", contestID, index)
}

func (a *Adapter) fetchProblemDetails(ctx context.Context, officialURL string) (string, string, string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, officialURL, nil)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to create request for problem details %s: %v", officialURL, err)
		return "", "", "", false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] HTTP request failed for problem details %s: %v", officialURL, err)
		return "", "", "", false
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Platform:Codeforces:Error] Problem details %s returned status %d %s | Body: %s", officialURL, resp.StatusCode, resp.Status, previewBody(body, 1000))
		return "", "", "", false
	}
	if readErr != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to read problem details response body for %s: %v", officialURL, readErr)
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
	contestID, index, isGym, err := parseProblemRef(externalID)
	if err != nil {
		return nil, err
	}
	officialURL := codeforcesProblemURL(contestID, index, isGym)

	req, err := http.NewRequestWithContext(ctx, "GET", officialURL, nil)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to create request for statement %s: %v", officialURL, err)
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[Platform:Codeforces:Error] HTTP request failed for statement %s: %v", officialURL, err)
		return nil, fmt.Errorf("failed to fetch problem statement: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024*2))
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Platform:Codeforces:Error] Statement request %s returned status %d %s | Body: %s", officialURL, resp.StatusCode, resp.Status, previewBody(bodyBytes, 1000))
	}
	if readErr != nil {
		log.Printf("[Platform:Codeforces:Error] Failed to read statement response body for %s: %v", officialURL, readErr)
		return nil, readErr
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

	contestID, subID, isGym := parseSubmissionRef(externalSubmissionID)

	// 1. Try the Codeforces Public REST API. contest.status rejects Gym IDs,
	// while problemset.recentStatus includes both regular and Gym submissions.
	// Gym submissions are checked immediately after dispatch, so the latest 1000
	// global submissions provide a cookie-free verification and verdict source.
	if contestID != "" || isGym {
		apiMethod := "contest.status"
		apiURL := fmt.Sprintf("https://codeforces.com/api/contest.status?contestId=%s&from=1&count=100", contestID)
		apiBodyLimit := int64(1024 * 500)
		if isGym {
			apiMethod = "problemset.recentStatus"
			apiURL = "https://codeforces.com/api/problemset.recentStatus?count=1000"
			apiBodyLimit = 4 << 20
		}
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			log.Printf("[Platform:Codeforces:Error] Failed to create %s request for contest %s: %v", apiMethod, contestID, err)
		} else {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			resp, err := a.client.Do(req)
			if err != nil {
				log.Printf("[Platform:Codeforces:Error] HTTP request failed for %s (%s): %v", apiMethod, apiURL, err)
			} else {
				defer resp.Body.Close()
				apiBody, readErr := io.ReadAll(io.LimitReader(resp.Body, apiBodyLimit))
				if resp.StatusCode != http.StatusOK {
					log.Printf("[Platform:Codeforces:Error] %s API returned status %d %s | Body: %s", apiMethod, resp.StatusCode, resp.Status, previewBody(apiBody, 1000))
				} else if readErr != nil {
					log.Printf("[Platform:Codeforces:Error] Failed to read %s API response: %v", apiMethod, readErr)
				} else {
					var statusResp cfStatusApiResponse
					if err := json.Unmarshal(apiBody, &statusResp); err != nil {
						log.Printf("[Platform:Codeforces:Error] Failed to decode %s response: %v | Body: %s", apiMethod, err, previewBody(apiBody, 1000))
					} else if statusResp.Status != "OK" {
						log.Printf("[Platform:Codeforces:Error] %s API returned non-OK status: %s (comment: %s) | Body: %s", apiMethod, statusResp.Status, statusResp.Comment, previewBody(apiBody, 1000))
					} else {
						for _, sub := range statusResp.Result {
							if strconv.FormatInt(sub.ID, 10) == subID && (!isGym || contestID == "" || strconv.Itoa(sub.ContestID) == contestID) {
								status := mapCFVerdict(sub.Verdict)
								timeMs := sub.TimeConsumedMillis
								memBytes := sub.MemoryConsumedBytes
								testcase := sub.PassedTestCount
								if status != "ACCEPTED" && status != "JUDGING" {
									testcase++
								}
								problemExternalID := fmt.Sprintf("%d/%s", sub.Problem.ContestID, strings.ToUpper(sub.Problem.Index))
								if isGym {
									problemExternalID = "gym/" + problemExternalID
								}
								canonicalSubmissionID := externalSubmissionID
								if isGym {
									canonicalSubmissionID = fmt.Sprintf("gym/%d/%s", sub.ContestID, subID)
								}
								statusObj := &platform.SubmissionStatus{
									ExternalSubmissionID: canonicalSubmissionID,
									Status:               status,
									ProblemExternalID:    problemExternalID,
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
								if source, ok := a.fetchSubmissionSource(ctx, strconv.Itoa(sub.ContestID), subID, isGym); ok {
									statusObj.SourceCode = source
								}
								return statusObj, nil
							}
						}
					}
				}
			}
		}
	}

	// 2. Fallback to scraping Codeforces submission page
	if contestID != "" && subID != "" {
		submissionURLs := codeforcesSubmissionURLs(contestID, subID, isGym)

		for _, subURL := range submissionURLs {
			req, err := http.NewRequestWithContext(ctx, "GET", subURL, nil)
			if err != nil {
				log.Printf("[Platform:Codeforces:Error] Failed to create submission scrape request for %s: %v", subURL, err)
				continue
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			resp, err := a.client.Do(req)
			if err != nil {
				log.Printf("[Platform:Codeforces:Error] HTTP request failed for submission scrape %s: %v", subURL, err)
				continue
			}
			defer resp.Body.Close()

			bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*500))

			if resp.StatusCode == http.StatusNotFound {
				return &platform.SubmissionStatus{
					ExternalSubmissionID: externalSubmissionID,
					Status:               "FAILED",
					RawPayload: map[string]any{
						"error": "Submission not found on Codeforces (404 Not Found)",
						"body":  previewBody(bodyBytes, 500),
					},
				}, nil
			}

			if resp.StatusCode != http.StatusOK {
				log.Printf("[Platform:Codeforces:Error] Submission scrape %s returned status %d %s | Body: %s", subURL, resp.StatusCode, resp.Status, previewBody(bodyBytes, 1000))
				continue
			}
			if readErr != nil {
				log.Printf("[Platform:Codeforces:Error] Failed to read submission scrape body for %s: %v", subURL, readErr)
				continue
			}
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

	return &platform.SubmissionStatus{
		ExternalSubmissionID: externalSubmissionID,
		Status:               "JUDGING",
	}, nil
}

func parseSubmissionRef(externalSubmissionID string) (contestID, submissionID string, isGym bool) {
	parts := strings.Split(strings.TrimSpace(externalSubmissionID), "/")
	if len(parts) == 3 && strings.EqualFold(parts[0], "gym") {
		return parts[1], parts[2], true
	}
	// v1.0.15 briefly stored Gym dispatches as gym/<submission-id>.
	// Retain enough scope to recover those pending records through the public
	// recent-status feed, which supplies the missing contest ID.
	if len(parts) == 2 && strings.EqualFold(parts[0], "gym") {
		return "", parts[1], true
	}
	if len(parts) == 2 {
		return parts[0], parts[1], false
	}
	return "", strings.TrimSpace(externalSubmissionID), false
}

func codeforcesSubmissionURLs(contestID, submissionID string, isGym bool) []string {
	if isGym {
		return []string{fmt.Sprintf("https://codeforces.com/gym/%s/submission/%s", contestID, submissionID)}
	}
	return []string{
		fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", contestID, submissionID),
		fmt.Sprintf("https://codeforces.com/problemset/submission/%s/%s", contestID, submissionID),
	}
}

func (a *Adapter) fetchSubmissionSource(ctx context.Context, contestID, submissionID string, isGym bool) (string, bool) {
	for _, submissionURL := range codeforcesSubmissionURLs(contestID, submissionID, isGym) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, submissionURL, nil)
		if err != nil {
			log.Printf("[Platform:Codeforces:Error] Failed to create submission source request for %s: %v", submissionURL, err)
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		resp, err := a.client.Do(req)
		if err != nil {
			log.Printf("[Platform:Codeforces:Error] HTTP request failed for submission source %s: %v", submissionURL, err)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*500))
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Platform:Codeforces:Error] Failed to read submission source body for %s: %v", submissionURL, readErr)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			if source := extractSubmissionSource(string(body)); source != "" {
				return source, true
			}
		} else {
			log.Printf("[Platform:Codeforces:Error] Submission source request %s returned status %d %s | Body: %s", submissionURL, resp.StatusCode, resp.Status, previewBody(body, 1000))
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

func previewBody(body []byte, maxLen int) string {
	if len(body) == 0 {
		return "<empty>"
	}
	s := strings.TrimSpace(string(body))
	if maxLen > 0 && len(s) > maxLen {
		return s[:maxLen] + " ... (truncated)"
	}
	return s
}

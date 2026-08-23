package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cp-hub/api/internal/platform"
)

var (
	urlPattern = regexp.MustCompile(`leetcode\.com/problems/([a-zA-Z0-9_\-]+)`)
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
	return platform.LeetCode
}

func (a *Adapter) MatchURL(rawURL string) (string, bool) {
	if m := urlPattern.FindStringSubmatch(rawURL); len(m) == 2 {
		return strings.ToLower(m[1]), true
	}
	return "", false
}

type leetcodeGqlResponse struct {
	Data struct {
		Question struct {
			QuestionID          string   `json:"questionId"`
			QuestionFrontendID  string   `json:"questionFrontendId"`
			Title               string   `json:"title"`
			Difficulty          string   `json:"difficulty"`
			Content             string   `json:"content"`
			SampleTestCase      string   `json:"sampleTestCase"`
			ExampleTestcaseList []string `json:"exampleTestcaseList"`
			TopicTags           []struct {
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"topicTags"`
		} `json:"question"`
	} `json:"data"`
}

func (a *Adapter) GetProblem(ctx context.Context, titleSlug string) (*platform.NormalizedProblem, error) {
	titleSlug = strings.ToLower(titleSlug)
	officialURL := fmt.Sprintf("https://leetcode.com/problems/%s/", titleSlug)

	gqlBody := map[string]interface{}{
		"query": `query questionData($titleSlug: String!) {
			question(titleSlug: $titleSlug) {
				questionId
				questionFrontendId
				title
				difficulty
				topicTags {
					name
					slug
				}
			}
		}`,
		"variables": map[string]string{
			"titleSlug": titleSlug,
		},
	}

	bodyBytes, _ := json.Marshal(gqlBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://leetcode.com/graphql", bytes.NewReader(bodyBytes))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
		resp, err := a.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var gqlResp leetcodeGqlResponse
			if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err == nil && gqlResp.Data.Question.Title != "" {
				q := gqlResp.Data.Question
				tags := make([]string, 0, len(q.TopicTags))
				for _, t := range q.TopicTags {
					tags = append(tags, t.Slug)
				}

				var diffRating int
				switch strings.ToLower(q.Difficulty) {
				case "easy":
					diffRating = 800
				case "medium":
					diffRating = 1500
				case "hard":
					diffRating = 2100
				default:
					diffRating = 1000
				}

				return &platform.NormalizedProblem{
					Platform:   platform.LeetCode,
					ExternalID: titleSlug,
					Title:      fmt.Sprintf("%s. %s", q.QuestionFrontendID, q.Title),
					URL:        officialURL,
					Difficulty: &diffRating,
					Tags:       tags,
					Metadata: map[string]interface{}{
						"questionFrontendId": q.QuestionFrontendID,
						"rawDifficulty":      q.Difficulty,
					},
				}, nil
			}
		}
	}

	// Fallback to formatted title
	words := strings.Split(titleSlug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	formattedTitle := strings.Join(words, " ")

	return &platform.NormalizedProblem{
		Platform:   platform.LeetCode,
		ExternalID: titleSlug,
		Title:      formattedTitle,
		URL:        officialURL,
		Difficulty: nil,
		Tags:       []string{"leetcode"},
		Metadata: map[string]interface{}{
			"titleSlug": titleSlug,
		},
	}, nil
}

func (a *Adapter) GetStatement(ctx context.Context, titleSlug string) (*platform.ProblemStatement, error) {
	titleSlug = strings.ToLower(titleSlug)
	officialURL := fmt.Sprintf("https://leetcode.com/problems/%s/", titleSlug)

	gqlBody := map[string]interface{}{
		"query": `query questionData($titleSlug: String!) {
			question(titleSlug: $titleSlug) {
				title
				content
				sampleTestCase
				exampleTestcaseList
			}
		}`,
		"variables": map[string]string{
			"titleSlug": titleSlug,
		},
	}

	bodyBytes, _ := json.Marshal(gqlBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://leetcode.com/graphql", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leetcode statement: %w", err)
	}
	defer resp.Body.Close()

	var gqlResp leetcodeGqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err == nil && gqlResp.Data.Question.Content != "" {
		q := gqlResp.Data.Question
		var sampleCases []platform.SampleCase

		if len(q.ExampleTestcaseList) > 0 {
			for i, ex := range q.ExampleTestcaseList {
				sampleCases = append(sampleCases, platform.SampleCase{
					Input:  ex,
					Output: fmt.Sprintf("Example %d expected output", i+1),
				})
			}
		} else if q.SampleTestCase != "" {
			sampleCases = append(sampleCases, platform.SampleCase{
				Input:  q.SampleTestCase,
				Output: "",
			})
		}

		if sampleCases == nil {
			sampleCases = []platform.SampleCase{}
		}

		return &platform.ProblemStatement{
			HTML:        q.Content,
			TimeLimit:   "1.0 sec",
			MemoryLimit: "256 MB",
			SampleCases: sampleCases,
		}, nil
	}

	return &platform.ProblemStatement{
		HTML:        fmt.Sprintf(`<p>Please refer to the official LeetCode statement at <a href="%s" target="_blank">%s</a>.</p>`, officialURL, officialURL),
		TimeLimit:   "1.0 sec",
		MemoryLimit: "256 MB",
		SampleCases: []platform.SampleCase{},
	}, nil
}

func (a *Adapter) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) {
	return &platform.SubmissionStatus{
		ExternalSubmissionID: externalSubmissionID,
		Status:               "JUDGING",
	}, nil
}

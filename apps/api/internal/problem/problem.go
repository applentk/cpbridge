package problem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cp-hub/api/internal/idgen"
	"github.com/cp-hub/api/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Problem struct {
	ID         string                 `json:"id"`
	Platform   platform.Type          `json:"platform"`
	ExternalID string                 `json:"externalId"`
	Title      string                 `json:"title"`
	URL        string                 `json:"url"`
	Difficulty *int                   `json:"difficulty"`
	Tags       []string               `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"createdAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
}

type Filter struct {
	Platform      *platform.Type
	Query         string
	MinDifficulty *int
	MaxDifficulty *int
	Tag           string
	Limit         int
	Offset        int
}

type CreateCustomReq struct {
	Platform    platform.Type         `json:"platform"`
	ExternalID  string                `json:"externalId"`
	Title       string                `json:"title"`
	URL         string                `json:"url"`
	Difficulty  *int                  `json:"difficulty"`
	Tags        []string              `json:"tags"`
	Statement   string                `json:"statement"`
	TimeLimit   string                `json:"timeLimit"`
	MemoryLimit string                `json:"memoryLimit"`
	SampleCases []platform.SampleCase `json:"sampleCases"`
}

type Service struct {
	db       *sql.DB
	registry *platform.Registry
}

func NewService(db *sql.DB, registry *platform.Registry) *Service {
	return &Service{
		db:       db,
		registry: registry,
	}
}

func (s *Service) ImportByUrl(ctx context.Context, rawURL string) (*Problem, error) {
	pType, extID, adapter, err := s.registry.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}

	norm, err := adapter.GetProblem(ctx, extID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch problem from %s: %w", pType, err)
	}

	tagsJSON, err := json.Marshal(norm.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	metaJSON, err := json.Marshal(norm.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	id := idgen.New(idgen.PrefixProblem)
	now := time.Now().UTC()

	query := `
		INSERT INTO problems (id, platform, external_id, title, url, difficulty, tags, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (platform, external_id) DO UPDATE SET
			title = EXCLUDED.title,
			url = EXCLUDED.url,
			difficulty = EXCLUDED.difficulty,
			tags = EXCLUDED.tags,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
		RETURNING id, platform, external_id, title, url, difficulty, tags, metadata, created_at, updated_at
	`

	var p Problem
	var scannedTags []byte
	var scannedMeta []byte

	row := s.db.QueryRowContext(ctx, query, id, norm.Platform, norm.ExternalID, norm.Title, norm.URL, norm.Difficulty, tagsJSON, metaJSON, now, now)
	err = row.Scan(&p.ID, &p.Platform, &p.ExternalID, &p.Title, &p.URL, &p.Difficulty, &scannedTags, &scannedMeta, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save problem: %w", err)
	}

	_ = json.Unmarshal(scannedTags, &p.Tags)
	_ = json.Unmarshal(scannedMeta, &p.Metadata)

	return &p, nil
}

func (s *Service) CreateCustom(ctx context.Context, req CreateCustomReq) (*Problem, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.ExternalID = strings.TrimSpace(req.ExternalID)
	req.URL = strings.TrimSpace(req.URL)

	if req.Title == "" {
		return nil, errors.New("problem title is required")
	}
	if req.ExternalID == "" {
		req.ExternalID = fmt.Sprintf("custom_%d", time.Now().UnixNano()%1000000)
	}
	if req.Platform == "" {
		req.Platform = platform.Codeforces
	}
	if req.URL == "" {
		req.URL = fmt.Sprintf("https://cphub.dev/problems/%s", req.ExternalID)
	}
	if req.Tags == nil {
		req.Tags = []string{"custom"}
	}
	if req.SampleCases == nil {
		req.SampleCases = []platform.SampleCase{}
	}

	// Clean statement of any remaining boilerplate
	req.Statement = CleanBoilerplate(req.Statement)

	meta := map[string]interface{}{
		"statement":   req.Statement,
		"timeLimit":   req.TimeLimit,
		"memoryLimit": req.MemoryLimit,
		"sampleCases": req.SampleCases,
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	metaJSON, _ := json.Marshal(meta)

	id := idgen.New(idgen.PrefixProblem)
	now := time.Now().UTC()

	query := `
		INSERT INTO problems (id, platform, external_id, title, url, difficulty, tags, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (platform, external_id) DO UPDATE SET
			title = EXCLUDED.title,
			url = EXCLUDED.url,
			difficulty = EXCLUDED.difficulty,
			tags = EXCLUDED.tags,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
		RETURNING id, platform, external_id, title, url, difficulty, tags, metadata, created_at, updated_at
	`

	var p Problem
	var scannedTags []byte
	var scannedMeta []byte

	row := s.db.QueryRowContext(ctx, query, id, req.Platform, req.ExternalID, req.Title, req.URL, req.Difficulty, tagsJSON, metaJSON, now, now)
	err := row.Scan(&p.ID, &p.Platform, &p.ExternalID, &p.Title, &p.URL, &p.Difficulty, &scannedTags, &scannedMeta, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save custom problem: %w", err)
	}

	_ = json.Unmarshal(scannedTags, &p.Tags)
	_ = json.Unmarshal(scannedMeta, &p.Metadata)

	return &p, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*Problem, error) {
	query := `
		SELECT id, platform, external_id, title, url, difficulty, tags, metadata, created_at, updated_at
		FROM problems
		WHERE id = $1
	`
	var p Problem
	var scannedTags []byte
	var scannedMeta []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Platform, &p.ExternalID, &p.Title, &p.URL, &p.Difficulty, &scannedTags, &scannedMeta, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("problem not found")
		}
		return nil, err
	}

	_ = json.Unmarshal(scannedTags, &p.Tags)
	_ = json.Unmarshal(scannedMeta, &p.Metadata)

	return &p, nil
}

func (s *Service) GetStatement(ctx context.Context, id string) (*platform.ProblemStatement, error) {
	prob, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// If problem has pre-stored statement in metadata, return it directly!
	if stmtVal, ok := prob.Metadata["statement"].(string); ok && strings.TrimSpace(stmtVal) != "" {
		var timeLimit, memoryLimit string
		if tl, ok := prob.Metadata["timeLimit"].(string); ok {
			timeLimit = tl
		}
		if ml, ok := prob.Metadata["memoryLimit"].(string); ok {
			memoryLimit = ml
		}

		var sampleCases []platform.SampleCase
		if scList, ok := prob.Metadata["sampleCases"].([]interface{}); ok {
			for _, item := range scList {
				if scMap, ok := item.(map[string]interface{}); ok {
					in, _ := scMap["input"].(string)
					out, _ := scMap["output"].(string)
					exp, _ := scMap["explanation"].(string)
					sampleCases = append(sampleCases, platform.SampleCase{
						Input:       in,
						Output:      out,
						Explanation: exp,
					})
				}
			}
		}

		if sampleCases == nil {
			sampleCases = []platform.SampleCase{}
		}

		return &platform.ProblemStatement{
			HTML:        CleanBoilerplate(stmtVal),
			TimeLimit:   timeLimit,
			MemoryLimit: memoryLimit,
			SampleCases: sampleCases,
		}, nil
	}

	// Live fetch via platform adapter
	adapter, err := s.registry.Get(prob.Platform)
	if err != nil {
		return nil, err
	}

	return adapter.GetStatement(ctx, prob.ExternalID)
}

func (s *Service) List(ctx context.Context, f Filter) ([]Problem, int, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	var conditions []string
	var args []interface{}
	idx := 1

	if f.Platform != nil && *f.Platform != "" {
		conditions = append(conditions, fmt.Sprintf("platform = $%d", idx))
		args = append(args, *f.Platform)
		idx++
	}

	if f.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(title) LIKE $%d OR LOWER(external_id) LIKE $%d)", idx, idx))
		args = append(args, "%"+strings.ToLower(f.Query)+"%")
		idx++
	}

	if f.MinDifficulty != nil {
		conditions = append(conditions, fmt.Sprintf("difficulty >= $%d", idx))
		args = append(args, *f.MinDifficulty)
		idx++
	}

	if f.MaxDifficulty != nil {
		conditions = append(conditions, fmt.Sprintf("difficulty <= $%d", idx))
		args = append(args, *f.MaxDifficulty)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM problems %s", whereClause)
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, platform, external_id, title, url, difficulty, tags, metadata, created_at, updated_at
		FROM problems
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, idx, idx+1)

	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var problems []Problem
	for rows.Next() {
		var p Problem
		var scannedTags []byte
		var scannedMeta []byte

		if err := rows.Scan(&p.ID, &p.Platform, &p.ExternalID, &p.Title, &p.URL, &p.Difficulty, &scannedTags, &scannedMeta, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(scannedTags, &p.Tags)
		_ = json.Unmarshal(scannedMeta, &p.Metadata)
		problems = append(problems, p)
	}

	if problems == nil {
		problems = []Problem{}
	}

	return problems, total, nil
}

type UpdateProblemReq struct {
	Title      *string                `json:"title"`
	URL        *string                `json:"url"`
	Difficulty *int                   `json:"difficulty"`
	Tags       []string               `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
}

func (s *Service) Update(ctx context.Context, id string, req UpdateProblemReq) (*Problem, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		p.Title = strings.TrimSpace(*req.Title)
	}
	if req.URL != nil && strings.TrimSpace(*req.URL) != "" {
		p.URL = strings.TrimSpace(*req.URL)
	}
	if req.Difficulty != nil {
		p.Difficulty = req.Difficulty
	}
	if req.Tags != nil {
		p.Tags = req.Tags
	}
	if req.Metadata != nil {
		p.Metadata = req.Metadata
	}
	p.UpdatedAt = time.Now().UTC()

	tagsJSON, _ := json.Marshal(p.Tags)
	metaJSON, _ := json.Marshal(p.Metadata)

	query := `
		UPDATE problems
		SET title = $1, url = $2, difficulty = $3, tags = $4, metadata = $5, updated_at = $6
		WHERE id = $7
	`
	_, err = s.db.ExecContext(ctx, query, p.Title, p.URL, p.Difficulty, tagsJSON, metaJSON, p.UpdatedAt, p.ID)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	// Check if problem is in use by any contest
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM contest_problems WHERE problem_id = $1", id).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("PROBLEM_IN_USE")
	}

	// Delete from problem_set_items first
	_, _ = s.db.ExecContext(ctx, "DELETE FROM problem_set_items WHERE problem_id = $1", id)

	res, err := s.db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("problem not found")
	}

	return nil
}


// CleanBoilerplate removes redundant header metadata, footers, copyright, and server info from statement text/HTML
func CleanBoilerplate(text string) string {
	// 1. Remove Codeforces header div
	reCfHeader := regexp.MustCompile(`(?is)<div class="header">.*?</div>\s*</div>`)
	text = reCfHeader.ReplaceAllString(text, "")

	// 2. Remove Codeforces duplicate sample-tests div (since handled by sample card component)
	reCfSamples := regexp.MustCompile(`(?is)<div class="sample-tests?">.*?</div>\s*</div>`)
	text = reCfSamples.ReplaceAllString(text, "")

	// 3. Remove Codeforces plain text header block (e.g. "time limit per test\n2 seconds\nmemory limit...")
	reTextHeader := regexp.MustCompile(`(?is)^[A-Z0-9\.\s\-]+\n(?:time limit(?:\s+per test)?\n[^\n]+\n)?(?:memory limit(?:\s+per test)?\n[^\n]+\n)?(?:input\n[^\n]+\n)?(?:output\n[^\n]+\n)?`)
	text = reTextHeader.ReplaceAllString(text, "")

	// 4. Remove Codeforces & AtCoder copyright footers, server time, mobile version links
	reCfFooter := regexp.MustCompile(`(?is)(?:\[?Codeforces\]?|\(c\)\s*Copyright).*?(?:Mike Mirzayanov|Server time:|Desktop version|Privacy Policy|Supported by).*`)
	text = reCfFooter.ReplaceAllString(text, "")

	reServerTime := regexp.MustCompile(`(?is)Server time:.*`)
	text = reServerTime.ReplaceAllString(text, "")

	reAtCoderFooter := regexp.MustCompile(`(?is)(?:Copyright\s*\d+-\d+\s*AtCoder Inc\.|AtCoder is a trademark).*`)
	text = reAtCoderFooter.ReplaceAllString(text, "")

	// 5. Remove Japanese section in AtCoder if present
	reLangJa := regexp.MustCompile(`(?is)<span class="lang-ja">.*?</span>`)
	text = reLangJa.ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}

// ExtractFromRawContent parses copied text or HTML from Codeforces, AtCoder, or generic problem sources
func ExtractFromRawContent(raw string) (title, statement, timeLimit, memoryLimit string, sampleCases []platform.SampleCase) {
	raw = strings.TrimSpace(raw)

	// 1. Time Limit extraction
	tlRegex := regexp.MustCompile(`(?i)(?:time limit(?:\s+per test)?|Time Limit)[\s:]*([0-9\.]+\s*(?:s|sec|seconds|ms))`)
	if m := tlRegex.FindStringSubmatch(raw); len(m) > 1 {
		timeLimit = strings.TrimSpace(m[1])
	}

	// 2. Memory Limit extraction
	mlRegex := regexp.MustCompile(`(?i)(?:memory limit(?:\s+per test)?|Memory Limit)[\s:]*([0-9\.]+\s*(?:MB|megabytes|KB))`)
	if m := mlRegex.FindStringSubmatch(raw); len(m) > 1 {
		memoryLimit = strings.TrimSpace(m[1])
	}

	// 3. Title extraction
	titleRegex := regexp.MustCompile(`(?i)<title>(.*?)(?: - Codeforces| - AtCoder)?</title>`)
	if m := titleRegex.FindStringSubmatch(raw); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	if title == "" {
		h1Regex := regexp.MustCompile(`(?i)<div class="title">([^<]+)</div>`)
		if m := h1Regex.FindStringSubmatch(raw); len(m) > 1 {
			title = strings.TrimSpace(m[1])
		}
	}
	if title == "" {
		plainTitleRegex := regexp.MustCompile(`(?m)^([A-Z][0-9]?\.\s+[^\n]+)`)
		if m := plainTitleRegex.FindStringSubmatch(raw); len(m) > 1 {
			title = strings.TrimSpace(m[1])
		}
	}

	// 4. Sample cases extraction
	cfInputRegex := regexp.MustCompile(`(?is)<div class="input">\s*<div class="title">Input</div>\s*<pre>(.*?)</pre>\s*</div>`)
	cfOutputRegex := regexp.MustCompile(`(?is)<div class="output">\s*<div class="title">Output</div>\s*<pre>(.*?)</pre>\s*</div>`)
	cfInputs := cfInputRegex.FindAllStringSubmatch(raw, -1)
	cfOutputs := cfOutputRegex.FindAllStringSubmatch(raw, -1)
	if len(cfInputs) > 0 && len(cfOutputs) > 0 {
		count := len(cfInputs)
		if len(cfOutputs) < count {
			count = len(cfOutputs)
		}
		for i := 0; i < count; i++ {
			sampleCases = append(sampleCases, platform.SampleCase{
				Input:  cleanSample(cfInputs[i][1]),
				Output: cleanSample(cfOutputs[i][1]),
			})
		}
	}

	// AtCoder / Generic pattern: Sample Input / Output
	if len(sampleCases) == 0 {
		sampleBlockRegex := regexp.MustCompile(`(?is)<h3>\s*Sample Input\s*(\d*)\s*</h3>\s*<pre>(.*?)</pre>\s*<h3>\s*Sample Output\s*\1\s*</h3>\s*<pre>(.*?)</pre>`)
		matches := sampleBlockRegex.FindAllStringSubmatch(raw, -1)
		for _, m := range matches {
			if len(m) >= 4 {
				sampleCases = append(sampleCases, platform.SampleCase{
					Input:  cleanSample(m[2]),
					Output: cleanSample(m[3]),
				})
			}
		}
	}

	// Markdown Example pattern: Example 1: Input: ... Output: ...
	if len(sampleCases) == 0 {
		exRegex := regexp.MustCompile(`(?is)(?:Example\s*\d*[:\s]*|Sample\s*\d*[:\s]*)Input[:\s]*(.*?)(?:Output|Expected)[:\s]*(.*?)(?:Explanation|$|Example\s*\d+)`)
		matches := exRegex.FindAllStringSubmatch(raw, -1)
		for _, m := range matches {
			if len(m) >= 3 {
				sampleCases = append(sampleCases, platform.SampleCase{
					Input:  strings.TrimSpace(m[1]),
					Output: strings.TrimSpace(m[2]),
				})
			}
		}
	}

	if sampleCases == nil {
		sampleCases = []platform.SampleCase{}
	}

	// 5. Statement Body extraction
	stmtRegex := regexp.MustCompile(`(?is)<div class="problem-statement">(.*?)</div>\s*<!--\s*end problem statement`)
	if m := stmtRegex.FindStringSubmatch(raw); len(m) > 1 {
		statement = m[1]
	} else {
		taskRegex := regexp.MustCompile(`(?is)<div id="task-statement">(.*?)</div>\s*<span class="center-block`)
		if m := taskRegex.FindStringSubmatch(raw); len(m) > 1 {
			statement = m[1]
		} else {
			statement = raw
		}
	}

	statement = CleanBoilerplate(statement)

	return title, statement, timeLimit, memoryLimit, sampleCases
}

func cleanSample(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, `<div class="test-example-line">`, "")
	s = strings.ReplaceAll(s, `</div>`, "\n")
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(tagRegex.ReplaceAllString(s, ""))
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/statement", h.GetStatement)
	r.Post("/import", h.Import)
	r.Post("/extract-text", h.ExtractText)
	return r
}

type importReq struct {
	URL string `json:"url"`
}

func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	var req importReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	p, err := h.service.ImportByUrl(r.Context(), req.URL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCustomReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	p, err := h.service.CreateCustom(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

type extractReq struct {
	RawContent string `json:"rawContent"`
}

func (h *Handler) ExtractText(w http.ResponseWriter, r *http.Request) {
	var req extractReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	title, stmt, tl, ml, sc := ExtractFromRawContent(req.RawContent)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"title":       title,
		"statement":   stmt,
		"timeLimit":   tl,
		"memoryLimit": ml,
		"sampleCases": sc,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"problem not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (h *Handler) GetStatement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	st, err := h.service.GetStatement(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to fetch statement: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := Filter{
		Query: q.Get("query"),
	}

	if p := q.Get("platform"); p != "" {
		pt := platform.Type(strings.ToUpper(p))
		filter.Platform = &pt
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = l
		}
	}
	if offsetStr := q.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = o
		}
	}

	problems, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch problems"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"problems": problems,
		"total":    total,
	})
}

package submission

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cp-hub/api/internal/auth"
	"github.com/cp-hub/api/internal/contest"
	"github.com/cp-hub/api/internal/idgen"
	"github.com/cp-hub/api/internal/platform"
	"github.com/cp-hub/api/internal/problem"
	"github.com/go-chi/chi/v5"
)

type Status string

const (
	Pending      Status = "PENDING"
	Dispatching  Status = "DISPATCHING"
	Judging      Status = "JUDGING"
	Accepted     Status = "ACCEPTED"
	WrongAnswer  Status = "WRONG_ANSWER"
	TimeLimit    Status = "TIME_LIMIT"
	MemoryLimit  Status = "MEMORY_LIMIT"
	RuntimeError Status = "RUNTIME_ERROR"
	CompileError Status = "COMPILE_ERROR"
	Failed       Status = "FAILED"
)

type Submission struct {
	ID                   string                 `json:"id"`
	UserID               string                 `json:"userId"`
	Username             string                 `json:"username,omitempty"`
	ProblemID            string                 `json:"problemId"`
	ProblemTitle         string                 `json:"problemTitle,omitempty"`
	ContestID            *string                `json:"contestId,omitempty"`
	Platform             platform.Type          `json:"platform"`
	Language             string                 `json:"language"`
	SourceCode           string                 `json:"sourceCode"`
	Status               Status                 `json:"status"`
	ExternalSubmissionID *string                `json:"externalSubmissionId,omitempty"`
	SubmittedAt          time.Time              `json:"submittedAt"`
	JudgedAt             *time.Time             `json:"judgedAt,omitempty"`
	Metadata             map[string]interface{} `json:"metadata"`
}

type ProblemScore struct {
	ProblemID            string `json:"problemId"`
	Label                string `json:"label"`
	Solved               bool   `json:"solved"`
	Attempts             int    `json:"attempts"`
	PenaltyMinutes       int    `json:"penaltyMinutes"`
	FirstSolvedAtMinutes *int   `json:"firstSolvedAtMinutes,omitempty"`
}

type ParticipantScore struct {
	UserID        string                  `json:"userId"`
	Username      string                  `json:"username"`
	Rank          int                     `json:"rank"`
	SolvedCount   int                     `json:"solvedCount"`
	TotalPenalty  int                     `json:"totalPenalty"`
	ProblemScores map[string]ProblemScore `json:"problemScores"`
}

type StandingsResponse struct {
	ContestID   string             `json:"contestId"`
	ScoringType string             `json:"scoringType"`
	Standings   []ParticipantScore `json:"standings"`
	Problems    []ProblemHeader    `json:"problems"`
	GeneratedAt time.Time          `json:"generatedAt"`
}

type ProblemHeader struct {
	ProblemID string `json:"problemId"`
	Label     string `json:"label"`
	Title     string `json:"title"`
	Platform  string `json:"platform"`
}

type Service struct {
	db         *sql.DB
	contestSvc *contest.Service
	probSvc    *problem.Service
	timeClock  func() time.Time
}

func NewService(db *sql.DB, contestSvc *contest.Service, probSvc *problem.Service) *Service {
	return &Service{
		db:         db,
		contestSvc: contestSvc,
		probSvc:    probSvc,
		timeClock:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetClock(clock func() time.Time) {
	s.timeClock = clock
}

func (s *Service) Create(ctx context.Context, userID, problemID string, contestID *string, language, sourceCode string) (*Submission, error) {
	prob, err := s.probSvc.GetByID(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("problem not found: %w", err)
	}

	now := s.timeClock()

	// Validate contest window if contestId is present
	if contestID != nil && *contestID != "" {
		c, err := s.contestSvc.GetByID(ctx, *contestID, userID)
		if err != nil {
			return nil, fmt.Errorf("contest not found: %w", err)
		}

		if now.Before(c.StartAt) {
			return nil, errors.New("contest has not started yet")
		}
		if !now.Before(c.EndAt) {
			return nil, errors.New("contest has ended")
		}

		// Ensure user joined contest
		_ = s.contestSvc.Join(ctx, *contestID, userID)
	}

	sub := &Submission{
		ID:          idgen.New(idgen.PrefixSubmission),
		UserID:      userID,
		ProblemID:   problemID,
		ContestID:   contestID,
		Platform:    prob.Platform,
		Language:    language,
		SourceCode:  sourceCode,
		Status:      Pending,
		SubmittedAt: now,
		Metadata:    map[string]interface{}{},
	}

	query := `
		INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, status, submitted_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = s.db.ExecContext(ctx, query, sub.ID, sub.UserID, sub.ProblemID, sub.ContestID, sub.Platform, sub.Language, sub.SourceCode, sub.Status, sub.SubmittedAt, "{}")
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	return sub, nil
}

func (s *Service) GetByID(ctx context.Context, id, requestingUserID string) (*Submission, error) {
	query := `
		SELECT s.id, s.user_id, u.username, s.problem_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.submitted_at, s.judged_at, s.metadata
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		WHERE s.id = $1
	`
	var sub Submission
	var metaJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.ProblemTitle, &sub.ContestID,
		&sub.Platform, &sub.Language, &sub.SourceCode, &sub.Status, &sub.ExternalSubmissionID,
		&sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("submission not found")
		}
		return nil, err
	}

	_ = json.Unmarshal(metaJSON, &sub.Metadata)
	return &sub, nil
}

func (s *Service) List(ctx context.Context, userID, contestID, problemID string, limit, offset int) ([]Submission, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}
	idx := 1

	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("s.user_id = $%d", idx))
		args = append(args, userID)
		idx++
	}
	if contestID != "" {
		conditions = append(conditions, fmt.Sprintf("s.contest_id = $%d", idx))
		args = append(args, contestID)
		idx++
	}
	if problemID != "" {
		conditions = append(conditions, fmt.Sprintf("s.problem_id = $%d", idx))
		args = append(args, problemID)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.user_id, u.username, s.problem_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.submitted_at, s.judged_at, s.metadata
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		%s
		ORDER BY s.submitted_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, idx, idx+1)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Submission
	for rows.Next() {
		var sub Submission
		var metaJSON []byte
		err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.ProblemTitle, &sub.ContestID,
			&sub.Platform, &sub.Language, &sub.SourceCode, &sub.Status, &sub.ExternalSubmissionID,
			&sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaJSON, &sub.Metadata)
		subs = append(subs, sub)
	}

	if subs == nil {
		subs = []Submission{}
	}

	return subs, nil
}

func (s *Service) UpdateDispatched(ctx context.Context, id, userID, externalSubmissionID string) error {
	sub, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if sub.UserID != userID {
		return errors.New("unauthorized to update submission")
	}

	query := `
		UPDATE submissions
		SET status = $1, external_submission_id = $2
		WHERE id = $3
	`
	_, err = s.db.ExecContext(ctx, query, Judging, externalSubmissionID, id)
	return err
}

func (s *Service) UpdateResult(ctx context.Context, id, userID string, status Status, metadata map[string]interface{}) error {
	sub, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if sub.UserID != userID {
		return errors.New("unauthorized to update submission")
	}

	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	now := s.timeClock()
	query := `
		UPDATE submissions
		SET status = $1, judged_at = $2, metadata = $3
		WHERE id = $4
	`
	_, err = s.db.ExecContext(ctx, query, status, now, metaJSON, id)
	return err
}

func (s *Service) CalculateStandings(ctx context.Context, contestID string, requestingUserID string) (*StandingsResponse, error) {
	c, err := s.contestSvc.GetByID(ctx, contestID, requestingUserID)
	if err != nil {
		return nil, err
	}

	// Fetch contest problems
	problems, err := s.contestSvc.GetProblems(ctx, contestID, requestingUserID)
	if err != nil {
		return nil, err
	}

	headers := make([]ProblemHeader, len(problems))
	problemLabels := make(map[string]string)
	for i, p := range problems {
		headers[i] = ProblemHeader{
			ProblemID: p.ProblemID,
			Label:     p.Label,
			Title:     p.Problem.Title,
			Platform:  string(p.Problem.Platform),
		}
		problemLabels[p.ProblemID] = p.Label
	}

	// Fetch all participants
	partRows, err := s.db.QueryContext(ctx, `
		SELECT cp.user_id, u.username
		FROM contest_participants cp
		JOIN users u ON cp.user_id = u.id
		WHERE cp.contest_id = $1
	`, contestID)
	if err != nil {
		return nil, err
	}
	defer partRows.Close()

	scoresByParticipant := make(map[string]*ParticipantScore)
	for partRows.Next() {
		var uid, uname string
		if err := partRows.Scan(&uid, &uname); err != nil {
			return nil, err
		}
		scoresByParticipant[uid] = &ParticipantScore{
			UserID:        uid,
			Username:      uname,
			SolvedCount:   0,
			TotalPenalty:  0,
			ProblemScores: make(map[string]ProblemScore),
		}
		for _, p := range problems {
			scoresByParticipant[uid].ProblemScores[p.ProblemID] = ProblemScore{
				ProblemID:      p.ProblemID,
				Label:          p.Label,
				Solved:         false,
				Attempts:       0,
				PenaltyMinutes: 0,
			}
		}
	}

	// Fetch submissions during contest window only: contest.start_at <= submitted_at < contest.end_at
	subQuery := `
		SELECT s.id, s.user_id, s.problem_id, s.status, s.submitted_at
		FROM submissions s
		WHERE s.contest_id = $1 AND s.submitted_at >= $2 AND s.submitted_at < $3
		ORDER BY s.submitted_at ASC
	`
	rows, err := s.db.QueryContext(ctx, subQuery, contestID, c.StartAt, c.EndAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sid, uid, pid string
		var status Status
		var subTime time.Time
		if err := rows.Scan(&sid, &uid, &pid, &status, &subTime); err != nil {
			return nil, err
		}

		pScore, exists := scoresByParticipant[uid]
		if !exists {
			continue
		}
		pProbScore, pProbExists := pScore.ProblemScores[pid]
		if !pProbExists {
			continue
		}

		// If already solved, ignore further submissions for ICPC penalty
		if pProbScore.Solved {
			continue
		}

		pProbScore.Attempts++

		if status == Accepted {
			pProbScore.Solved = true
			elapsedMinutes := int(math.Floor(subTime.Sub(c.StartAt).Minutes()))
			if elapsedMinutes < 0 {
				elapsedMinutes = 0
			}
			pProbScore.FirstSolvedAtMinutes = &elapsedMinutes

			if c.ScoringType == contest.ICPC {
				// penalty = minutes from contest start + 20 * rejected before AC
				rejectedBeforeAC := pProbScore.Attempts - 1
				pProbScore.PenaltyMinutes = elapsedMinutes + (20 * rejectedBeforeAC)
			} else {
				pProbScore.PenaltyMinutes = elapsedMinutes
			}

			pScore.SolvedCount++
			pScore.TotalPenalty += pProbScore.PenaltyMinutes
		}

		pScore.ProblemScores[pid] = pProbScore
	}

	// Sort standings
	var standings []ParticipantScore
	for _, pScore := range scoresByParticipant {
		standings = append(standings, *pScore)
	}

	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].SolvedCount != standings[j].SolvedCount {
			return standings[i].SolvedCount > standings[j].SolvedCount
		}
		return standings[i].TotalPenalty < standings[j].TotalPenalty
	})

	for i := range standings {
		standings[i].Rank = i + 1
	}

	if standings == nil {
		standings = []ParticipantScore{}
	}

	return &StandingsResponse{
		ContestID:   contestID,
		ScoringType: string(c.ScoringType),
		Standings:   standings,
		Problems:    headers,
		GeneratedAt: s.timeClock(),
	}, nil
}

type Handler struct {
	service *Service
	authSvc *auth.Service
}

func NewHandler(service *Service, authSvc *auth.Service) *Handler {
	return &Handler{service: service, authSvc: authSvc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Group(func(pr chi.Router) {
		pr.Use(h.authSvc.AuthMiddleware(false))
		pr.Get("/", h.List)
		pr.Get("/{id}", h.Get)
	})

	r.Group(func(pr chi.Router) {
		pr.Use(h.authSvc.AuthMiddleware(true))
		pr.Post("/", h.Create)
		pr.Post("/{id}/dispatched", h.UpdateDispatched)
		pr.Post("/{id}/result", h.UpdateResult)
	})

	return r
}

type createSubReq struct {
	ProblemID  string  `json:"problemId"`
	ContestID  *string `json:"contestId"`
	Language   string  `json:"language"`
	SourceCode string  `json:"sourceCode"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	var req createSubReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	sub, err := h.service.Create(r.Context(), claims.UserID, req.ProblemID, req.ContestID, req.Language, req.SourceCode)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sub)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var uid string
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
	}

	sub, err := h.service.GetByID(r.Context(), id, uid)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sub)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	uid := q.Get("userId")
	cid := q.Get("contestId")
	pid := q.Get("problemId")

	subs, err := h.service.List(r.Context(), uid, cid, pid, 50, 0)
	if err != nil {
		http.Error(w, `{"error":"failed to list submissions"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(subs)
}

type dispatchedReq struct {
	ExternalSubmissionID string `json:"externalSubmissionId"`
}

func (h *Handler) UpdateDispatched(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req dispatchedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	err := h.service.UpdateDispatched(r.Context(), id, claims.UserID, req.ExternalSubmissionID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

type resultReq struct {
	Status   Status                 `json:"status"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (h *Handler) UpdateResult(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req resultReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	err := h.service.UpdateResult(r.Context(), id, claims.UserID, req.Status, req.Metadata)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

package contest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cp-hub/api/internal/auth"
	"github.com/cp-hub/api/internal/idgen"
	"github.com/cp-hub/api/internal/problem"
	"github.com/cp-hub/api/internal/problemset"
	"github.com/go-chi/chi/v5"
)

type State string

const (
	Upcoming State = "UPCOMING"
	Active   State = "ACTIVE"
	Finished State = "FINISHED"
)

type ScoringType string

const (
	Simple ScoringType = "SIMPLE"
	ICPC   ScoringType = "ICPC"
)

type ContestProblem struct {
	ContestID string           `json:"contestId"`
	ProblemID string           `json:"problemId"`
	Position  int              `json:"position"`
	Label     string           `json:"label"`
	Points    *int             `json:"points,omitempty"`
	Problem   *problem.Problem `json:"problem,omitempty"`
}

type Contest struct {
	ID               string           `json:"id"`
	OwnerID          string           `json:"ownerId"`
	OwnerUsername    string           `json:"ownerUsername,omitempty"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	StartAt          time.Time        `json:"startAt"`
	EndAt            time.Time        `json:"endAt"`
	Visibility       string           `json:"visibility"`
	ScoringType      ScoringType      `json:"scoringType"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	State            State            `json:"state"`
	Problems         []ContestProblem `json:"problems,omitempty"`
	ParticipantCount int              `json:"participantCount"`
	IsParticipant    bool             `json:"isParticipant"`
}

func CalculateState(now, startAt, endAt time.Time) State {
	now = now.UTC()
	startAt = startAt.UTC()
	endAt = endAt.UTC()

	if now.Before(startAt) {
		return Upcoming
	}
	if !now.Before(endAt) {
		return Finished
	}
	return Active
}

func GenerateLabel(index int) string {
	if index < 26 {
		return string(rune('A' + index))
	}
	return fmt.Sprintf("P%d", index+1)
}

type Service struct {
	db        *sql.DB
	setSvc    *problemset.Service
	timeClock func() time.Time
}

func NewService(db *sql.DB, setSvc *problemset.Service) *Service {
	return &Service{
		db:        db,
		setSvc:    setSvc,
		timeClock: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetClock(clock func() time.Time) {
	s.timeClock = clock
}

func (s *Service) CreateFromProblemSet(ctx context.Context, ownerID, problemSetID, name, description string, startAt, endAt time.Time, visibility string, scoring ScoringType) (*Contest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("contest name is required")
	}

	startAt = startAt.UTC()
	endAt = endAt.UTC()
	if !endAt.After(startAt) {
		return nil, errors.New("end time must be after start time")
	}

	if visibility == "" {
		visibility = "PUBLIC"
	}
	if scoring == "" {
		scoring = ICPC
	}

	// Fetch problem set to snapshot
	set, err := s.setSvc.GetByID(ctx, problemSetID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("problem set not found: %w", err)
	}

	if len(set.Items) == 0 {
		return nil, errors.New("cannot create contest from empty problem set")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	contestID := idgen.New(idgen.PrefixContest)
	now := s.timeClock()

	contestQuery := `
		INSERT INTO contests (id, owner_id, name, description, start_at, end_at, visibility, scoring_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.ExecContext(ctx, contestQuery, contestID, ownerID, name, description, startAt, endAt, visibility, scoring, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert contest: %w", err)
	}

	// Auto-join owner as participant
	_, err = tx.ExecContext(ctx, `INSERT INTO contest_participants (contest_id, user_id, joined_at) VALUES ($1, $2, $3)`, contestID, ownerID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add owner to participants: %w", err)
	}

	// Snapshot problems from problem set
	for i, item := range set.Items {
		label := GenerateLabel(i)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO contest_problems (contest_id, problem_id, position, label)
			VALUES ($1, $2, $3, $4)
		`, contestID, item.ProblemID, i, label)
		if err != nil {
			return nil, fmt.Errorf("failed to snapshot problem %s: %w", item.ProblemID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, contestID, ownerID)
}

func (s *Service) GetByID(ctx context.Context, contestID string, requestingUserID string) (*Contest, error) {
	query := `
		SELECT c.id, c.owner_id, u.username, c.name, c.description, c.start_at, c.end_at, c.visibility, c.scoring_type, c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM contest_participants cp WHERE cp.contest_id = c.id) as participant_count,
		       EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $2) as is_participant
		FROM contests c
		JOIN users u ON c.owner_id = u.id
		WHERE c.id = $1
	`
	var c Contest
	err := s.db.QueryRowContext(ctx, query, contestID, requestingUserID).Scan(
		&c.ID, &c.OwnerID, &c.OwnerUsername, &c.Name, &c.Description, &c.StartAt, &c.EndAt, &c.Visibility, &c.ScoringType, &c.CreatedAt, &c.UpdatedAt,
		&c.ParticipantCount, &c.IsParticipant,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("contest not found")
		}
		return nil, err
	}

	c.State = CalculateState(s.timeClock(), c.StartAt, c.EndAt)

	// Fetch contest problems
	problems, err := s.GetProblems(ctx, contestID, requestingUserID)
	if err == nil {
		c.Problems = problems
	}

	return &c, nil
}

func (s *Service) GetProblems(ctx context.Context, contestID string, requestingUserID string) ([]ContestProblem, error) {
	var startAt, endAt time.Time
	err := s.db.QueryRowContext(ctx, `SELECT start_at, end_at FROM contests WHERE id = $1`, contestID).Scan(&startAt, &endAt)
	if err != nil {
		return nil, errors.New("contest not found")
	}

	state := CalculateState(s.timeClock(), startAt, endAt)

	query := `
		SELECT cp.contest_id, cp.problem_id, cp.position, cp.label, cp.points,
		       p.id, p.platform, p.external_id, p.title, p.url, p.difficulty, p.tags, p.metadata, p.created_at, p.updated_at
		FROM contest_problems cp
		JOIN problems p ON cp.problem_id = p.id
		WHERE cp.contest_id = $1
		ORDER BY cp.position ASC
	`
	rows, err := s.db.QueryContext(ctx, query, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ContestProblem
	for rows.Next() {
		var cp ContestProblem
		var p problem.Problem
		var tagsJSON, metaJSON []byte

		err := rows.Scan(
			&cp.ContestID, &cp.ProblemID, &cp.Position, &cp.Label, &cp.Points,
			&p.ID, &p.Platform, &p.ExternalID, &p.Title, &p.URL, &p.Difficulty, &tagsJSON, &metaJSON, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if state == Upcoming {
			// Redact problem statement/title/external details before contest starts
			cp.Problem = &problem.Problem{
				ID:         cp.ProblemID,
				Title:      fmt.Sprintf("Problem %s", cp.Label),
				Platform:   "",
				ExternalID: "",
				URL:        "",
				Tags:       []string{},
				Metadata:   map[string]interface{}{},
			}
		} else {
			_ = json.Unmarshal(tagsJSON, &p.Tags)
			_ = json.Unmarshal(metaJSON, &p.Metadata)
			cp.Problem = &p
		}

		list = append(list, cp)
	}

	if list == nil {
		list = []ContestProblem{}
	}

	return list, nil
}

func (s *Service) List(ctx context.Context, requestingUserID string) ([]Contest, error) {
	query := `
		SELECT c.id, c.owner_id, u.username, c.name, c.description, c.start_at, c.end_at, c.visibility, c.scoring_type, c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM contest_participants cp WHERE cp.contest_id = c.id) as participant_count,
		       EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $1) as is_participant
		FROM contests c
		JOIN users u ON c.owner_id = u.id
		WHERE c.visibility = 'PUBLIC' OR c.owner_id = $1 OR EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $1)
		ORDER BY c.start_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, requestingUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := s.timeClock()
	var contests []Contest
	for rows.Next() {
		var c Contest
		err := rows.Scan(
			&c.ID, &c.OwnerID, &c.OwnerUsername, &c.Name, &c.Description, &c.StartAt, &c.EndAt, &c.Visibility, &c.ScoringType, &c.CreatedAt, &c.UpdatedAt,
			&c.ParticipantCount, &c.IsParticipant,
		)
		if err != nil {
			return nil, err
		}
		c.State = CalculateState(now, c.StartAt, c.EndAt)
		contests = append(contests, c)
	}

	if contests == nil {
		contests = []Contest{}
	}

	return contests, nil
}

func (s *Service) Join(ctx context.Context, contestID, userID string) error {
	c, err := s.GetByID(ctx, contestID, userID)
	if err != nil {
		return err
	}

	if c.State == Finished {
		return errors.New("cannot join finished contest")
	}

	query := `
		INSERT INTO contest_participants (contest_id, user_id, joined_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (contest_id, user_id) DO NOTHING
	`
	_, err = s.db.ExecContext(ctx, query, contestID, userID, s.timeClock())
	return err
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
		pr.Get("/{id}/problems", h.GetProblems)
	})

	r.Group(func(pr chi.Router) {
		pr.Use(h.authSvc.AuthMiddleware(true))
		pr.Post("/", h.Create)
		pr.Post("/{id}/join", h.Join)
	})

	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	var uid string
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
	}

	contests, err := h.service.List(r.Context(), uid)
	if err != nil {
		http.Error(w, `{"error":"failed to list contests"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contests)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var uid string
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
	}

	c, err := h.service.GetByID(r.Context(), id, uid)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (h *Handler) GetProblems(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var uid string
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
	}

	problems, err := h.service.GetProblems(r.Context(), id, uid)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(problems)
}

type createContestReq struct {
	ProblemSetID string      `json:"problemSetId"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	StartAt      string      `json:"startAt"`
	EndAt        string      `json:"endAt"`
	Visibility   string      `json:"visibility"`
	ScoringType  ScoringType `json:"scoringType"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	var req createContestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		http.Error(w, `{"error":"invalid startAt RFC3339 format"}`, http.StatusBadRequest)
		return
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		http.Error(w, `{"error":"invalid endAt RFC3339 format"}`, http.StatusBadRequest)
		return
	}

	contest, err := h.service.CreateFromProblemSet(
		r.Context(), claims.UserID, req.ProblemSetID, req.Name, req.Description, startAt, endAt, req.Visibility, req.ScoringType,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(contest)
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	err := h.service.Join(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

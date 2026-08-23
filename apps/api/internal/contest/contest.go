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

const (
	PublicationDraft     = "DRAFT"
	PublicationPublished = "PUBLISHED"
)

const (
	VisibilityPublic  = "PUBLIC"
	VisibilityPrivate = "PRIVATE"
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
	ID                string           `json:"id"`
	OwnerID           string           `json:"ownerId"`
	OwnerUsername     string           `json:"ownerUsername,omitempty"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	StartAt           time.Time        `json:"startAt"`
	EndAt             time.Time        `json:"endAt"`
	Visibility        string           `json:"visibility"`
	ScoringType       ScoringType      `json:"scoringType"`
	PublicationStatus string           `json:"publicationStatus"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	State             State            `json:"state"`
	Problems          []ContestProblem `json:"problems,omitempty"`
	ParticipantCount  int              `json:"participantCount"`
	IsParticipant     bool             `json:"isParticipant"`
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

type CreateContestParams struct {
	OwnerID           string
	ProblemSetID      string
	ProblemIDs        []string
	Name              string
	Description       string
	StartAt           time.Time
	EndAt             time.Time
	Visibility        string
	ScoringType       ScoringType
	PublicationStatus string
}

func (s *Service) Create(ctx context.Context, params CreateContestParams) (*Contest, error) {
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, errors.New("contest name is required")
	}

	params.StartAt = params.StartAt.UTC()
	params.EndAt = params.EndAt.UTC()
	if !params.EndAt.After(params.StartAt) {
		return nil, errors.New("end time must be after start time")
	}

	if params.Visibility == "" {
		params.Visibility = "PUBLIC"
	}
	if params.ScoringType == "" {
		params.ScoringType = ICPC
	}
	if params.PublicationStatus == "" {
		params.PublicationStatus = PublicationPublished
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	contestID := idgen.New(idgen.PrefixContest)
	now := s.timeClock()

	contestQuery := `
		INSERT INTO contests (id, owner_id, name, description, start_at, end_at, visibility, scoring_type, publication_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = tx.ExecContext(ctx, contestQuery, contestID, params.OwnerID, params.Name, params.Description, params.StartAt, params.EndAt, params.Visibility, params.ScoringType, params.PublicationStatus, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert contest: %w", err)
	}

	// Auto-join owner as participant
	_, err = tx.ExecContext(ctx, `INSERT INTO contest_participants (contest_id, user_id, joined_at) VALUES ($1, $2, $3)`, contestID, params.OwnerID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to add owner to participants: %w", err)
	}

	// Snapshot problems if problem set provided
	if params.ProblemSetID != "" {
		set, err := s.setSvc.GetByID(ctx, params.ProblemSetID, params.OwnerID)
		if err == nil && len(set.Items) > 0 {
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
		}
	} else if len(params.ProblemIDs) > 0 {
		for i, pid := range params.ProblemIDs {
			label := GenerateLabel(i)
			_, err := tx.ExecContext(ctx, `
				INSERT INTO contest_problems (contest_id, problem_id, position, label)
				VALUES ($1, $2, $3, $4)
			`, contestID, pid, i, label)
			if err != nil {
				return nil, fmt.Errorf("failed to snapshot problem %s: %w", pid, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, contestID, params.OwnerID, true)
}

func (s *Service) CreateFromProblemSet(ctx context.Context, ownerID, problemSetID, name, description string, startAt, endAt time.Time, visibility string, scoring ScoringType) (*Contest, error) {
	return s.Create(ctx, CreateContestParams{
		OwnerID:           ownerID,
		ProblemSetID:      problemSetID,
		Name:              name,
		Description:       description,
		StartAt:           startAt,
		EndAt:             endAt,
		Visibility:        visibility,
		ScoringType:       scoring,
		PublicationStatus: PublicationPublished,
	})
}

func (s *Service) GetByID(ctx context.Context, contestID string, requestingUserID string, isAdmin bool) (*Contest, error) {
	query := `
		SELECT c.id, c.owner_id, u.username, c.name, c.description, c.start_at, c.end_at, c.visibility, c.scoring_type, c.publication_status, c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM contest_participants cp WHERE cp.contest_id = c.id) as participant_count,
		       EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $2) as is_participant
		FROM contests c
		JOIN users u ON c.owner_id = u.id
		WHERE c.id = $1
	`
	var c Contest
	err := s.db.QueryRowContext(ctx, query, contestID, requestingUserID).Scan(
		&c.ID, &c.OwnerID, &c.OwnerUsername, &c.Name, &c.Description, &c.StartAt, &c.EndAt, &c.Visibility, &c.ScoringType, &c.PublicationStatus, &c.CreatedAt, &c.UpdatedAt,
		&c.ParticipantCount, &c.IsParticipant,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("contest not found")
		}
		return nil, err
	}

	if !isAdmin {
		if c.PublicationStatus == PublicationDraft && c.OwnerID != requestingUserID {
			return nil, errors.New("contest not found")
		}
		if c.Visibility == VisibilityPrivate && c.OwnerID != requestingUserID && !c.IsParticipant {
			return nil, errors.New("contest is private")
		}
	}

	now := s.timeClock()
	var dbNow time.Time
	if err := s.db.QueryRowContext(ctx, "SELECT NOW()").Scan(&dbNow); err == nil && !dbNow.IsZero() {
		now = dbNow.UTC()
	}
	c.State = CalculateState(now, c.StartAt, c.EndAt)

	// Fetch contest problems
	problems, err := s.GetProblems(ctx, contestID, requestingUserID, isAdmin)
	if err == nil {
		c.Problems = problems
	}

	return &c, nil
}

func (s *Service) GetProblems(ctx context.Context, contestID string, requestingUserID string, isAdmin bool) ([]ContestProblem, error) {
	var startAt, endAt time.Time
	var pubStatus, visibility, ownerID string
	var isParticipant bool
	err := s.db.QueryRowContext(ctx, `
		SELECT start_at, end_at, publication_status, visibility, owner_id,
		       EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = $1 AND cp.user_id = $2) as is_participant
		FROM contests WHERE id = $1
	`, contestID, requestingUserID).Scan(&startAt, &endAt, &pubStatus, &visibility, &ownerID, &isParticipant)
	if err != nil {
		return nil, errors.New("contest not found")
	}

	if !isAdmin {
		if pubStatus == PublicationDraft && ownerID != requestingUserID {
			return nil, errors.New("contest not found")
		}
		if visibility == VisibilityPrivate && ownerID != requestingUserID && !isParticipant {
			return nil, errors.New("contest is private")
		}
	}

	now := s.timeClock()
	var dbNow time.Time
	if err := s.db.QueryRowContext(ctx, "SELECT NOW()").Scan(&dbNow); err == nil && !dbNow.IsZero() {
		now = dbNow.UTC()
	}
	state := CalculateState(now, startAt, endAt)

	// If contest has not started and user is NOT admin/owner, return CONTEST_NOT_STARTED error
	if state == Upcoming && !isAdmin && ownerID != requestingUserID {
		return nil, errors.New("CONTEST_NOT_STARTED")
	}

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

		_ = json.Unmarshal(tagsJSON, &p.Tags)
		_ = json.Unmarshal(metaJSON, &p.Metadata)
		cp.Problem = &p
		list = append(list, cp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if list == nil {
		list = []ContestProblem{}
	}

	return list, nil
}

func (s *Service) List(ctx context.Context, requestingUserID string, isAdmin bool) ([]Contest, error) {
	var query string
	var args []any

	if isAdmin {
		query = `
			SELECT c.id, c.owner_id, u.username, c.name, c.description, c.start_at, c.end_at, c.visibility, c.scoring_type, c.publication_status, c.created_at, c.updated_at,
			       (SELECT COUNT(*) FROM contest_participants cp WHERE cp.contest_id = c.id) as participant_count,
			       EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $1) as is_participant
			FROM contests c
			JOIN users u ON c.owner_id = u.id
			ORDER BY c.start_at DESC
		`
		args = append(args, requestingUserID)
	} else {
		query = `
			SELECT c.id, c.owner_id, u.username, c.name, c.description, c.start_at, c.end_at, c.visibility, c.scoring_type, c.publication_status, c.created_at, c.updated_at,
			       (SELECT COUNT(*) FROM contest_participants cp WHERE cp.contest_id = c.id) as participant_count,
			       EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $1) as is_participant
			FROM contests c
			JOIN users u ON c.owner_id = u.id
			WHERE c.publication_status = 'PUBLISHED' AND (c.visibility = 'PUBLIC' OR c.owner_id = $1 OR EXISTS(SELECT 1 FROM contest_participants cp WHERE cp.contest_id = c.id AND cp.user_id = $1))
			ORDER BY c.start_at DESC
		`
		args = append(args, requestingUserID)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := s.timeClock()
	var contests []Contest
	for rows.Next() {
		var c Contest
		err := rows.Scan(
			&c.ID, &c.OwnerID, &c.OwnerUsername, &c.Name, &c.Description, &c.StartAt, &c.EndAt, &c.Visibility, &c.ScoringType, &c.PublicationStatus, &c.CreatedAt, &c.UpdatedAt,
			&c.ParticipantCount, &c.IsParticipant,
		)
		if err != nil {
			return nil, err
		}
		c.State = CalculateState(now, c.StartAt, c.EndAt)
		contests = append(contests, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if contests == nil {
		contests = []Contest{}
	}

	return contests, nil
}

func (s *Service) Join(ctx context.Context, contestID, userID string) error {
	c, err := s.GetByID(ctx, contestID, userID, false)
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

type UpdateContestParams struct {
	Name              *string
	Description       *string
	StartAt           *time.Time
	EndAt             *time.Time
	Visibility        *string
	ScoringType       *ScoringType
	PublicationStatus *string
}

func (s *Service) Update(ctx context.Context, contestID string, params UpdateContestParams) (*Contest, error) {
	c, err := s.GetByID(ctx, contestID, "", true)
	if err != nil {
		return nil, err
	}

	// Active contest rules
	if (c.State == Active || c.State == Finished) && params.StartAt != nil {
		if !params.StartAt.Equal(c.StartAt) {
			return nil, errors.New("cannot modify start time of active or finished contest")
		}
	}

	if params.Name != nil && strings.TrimSpace(*params.Name) != "" {
		c.Name = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		c.Description = *params.Description
	}
	if params.StartAt != nil {
		c.StartAt = params.StartAt.UTC()
	}
	if params.EndAt != nil {
		c.EndAt = params.EndAt.UTC()
	}
	if !c.EndAt.After(c.StartAt) {
		return nil, errors.New("end time must be after start time")
	}
	if params.Visibility != nil && *params.Visibility != "" {
		c.Visibility = *params.Visibility
	}
	if params.ScoringType != nil && *params.ScoringType != "" {
		c.ScoringType = *params.ScoringType
	}
	if params.PublicationStatus != nil && *params.PublicationStatus != "" {
		c.PublicationStatus = *params.PublicationStatus
	}
	c.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE contests
		SET name = $1, description = $2, start_at = $3, end_at = $4, visibility = $5, scoring_type = $6, publication_status = $7, updated_at = $8
		WHERE id = $9
	`
	_, err = s.db.ExecContext(ctx, query, c.Name, c.Description, c.StartAt, c.EndAt, c.Visibility, c.ScoringType, c.PublicationStatus, c.UpdatedAt, c.ID)
	if err != nil {
		return nil, err
	}

	return s.GetByID(ctx, contestID, "", true)
}

func (s *Service) Delete(ctx context.Context, contestID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM contests WHERE id = $1`, contestID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("contest not found")
	}
	return nil
}

func (s *Service) AddProblem(ctx context.Context, contestID, problemID string, position *int, label *string, points *int) error {
	c, err := s.GetByID(ctx, contestID, "", true)
	if err != nil {
		return err
	}
	if c.State == Active || c.State == Finished {
		return errors.New("cannot modify problems of active or finished contest")
	}

	pos := len(c.Problems)
	if position != nil && *position >= 0 {
		pos = *position
	}

	lbl := GenerateLabel(pos)
	if label != nil && strings.TrimSpace(*label) != "" {
		lbl = strings.TrimSpace(*label)
	}

	query := `
		INSERT INTO contest_problems (contest_id, problem_id, position, label, points)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (contest_id, problem_id) DO UPDATE SET position = EXCLUDED.position, label = EXCLUDED.label, points = EXCLUDED.points
	`
	_, err = s.db.ExecContext(ctx, query, contestID, problemID, pos, lbl, points)
	return err
}

func (s *Service) RemoveProblem(ctx context.Context, contestID, problemID string) error {
	c, err := s.GetByID(ctx, contestID, "", true)
	if err != nil {
		return err
	}
	if c.State == Active || c.State == Finished {
		return errors.New("cannot modify problems of active or finished contest")
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM contest_problems WHERE contest_id = $1 AND problem_id = $2`, contestID, problemID)
	return err
}

func (s *Service) ReorderProblems(ctx context.Context, contestID string, problemIDs []string) error {
	c, err := s.GetByID(ctx, contestID, "", true)
	if err != nil {
		return err
	}
	if c.State == Active || c.State == Finished {
		return errors.New("cannot modify problems of active or finished contest")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, pid := range problemIDs {
		lbl := GenerateLabel(i)
		_, err := tx.ExecContext(ctx, `
			UPDATE contest_problems
			SET position = $1, label = $2
			WHERE contest_id = $3 AND problem_id = $4
		`, i, lbl, contestID, pid)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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
		pr.Post("/{id}/join", h.Join)
	})

	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	var uid string
	isAdmin := false
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
		isAdmin = (claims.Role == auth.RoleAdmin)
	}

	contests, err := h.service.List(r.Context(), uid, isAdmin)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list contests"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contests)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var uid string
	isAdmin := false
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
		isAdmin = (claims.Role == auth.RoleAdmin)
	}

	c, err := h.service.GetByID(r.Context(), id, uid, isAdmin)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (h *Handler) GetProblems(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var uid string
	isAdmin := false
	if claims := auth.GetUserFromContext(r.Context()); claims != nil {
		uid = claims.UserID
		isAdmin = (claims.Role == auth.RoleAdmin)
	}

	problems, err := h.service.GetProblems(r.Context(), id, uid, isAdmin)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "CONTEST_NOT_STARTED" {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(problems)
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

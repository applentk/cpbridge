package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/contest"
	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/problem"
	"github.com/cpbridge/api/internal/problemset"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	db         *sql.DB
	authSvc    *auth.Service
	probSvc    *problem.Service
	setSvc     *problemset.Service
	contestSvc *contest.Service
}

func NewHandler(
	db *sql.DB,
	authSvc *auth.Service,
	probSvc *problem.Service,
	setSvc *problemset.Service,
	contestSvc *contest.Service,
) *Handler {
	return &Handler{
		db:         db,
		authSvc:    authSvc,
		probSvc:    probSvc,
		setSvc:     setSvc,
		contestSvc: contestSvc,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Stats
	r.Get("/stats", h.GetStats)

	// Problems
	r.Route("/problems", func(pr chi.Router) {
		pr.Get("/", h.ListProblems)
		pr.Post("/", h.CreateProblem)
		pr.Post("/import", h.ImportProblem)
		pr.Patch("/{id}", h.UpdateProblem)
		pr.Delete("/{id}", h.DeleteProblem)
	})

	// Problem Sets
	r.Route("/problem-sets", func(psr chi.Router) {
		psr.Get("/", h.ListProblemSets)
		psr.Post("/", h.CreateProblemSet)
		psr.Get("/{id}", h.GetProblemSet)
		psr.Patch("/{id}", h.UpdateProblemSet)
		psr.Delete("/{id}", h.DeleteProblemSet)
		psr.Post("/{id}/problems", h.AddProblemToSet)
		psr.Delete("/{id}/problems/{problemId}", h.RemoveProblemFromSet)
		psr.Patch("/{id}/order", h.ReorderProblemSet)
	})

	// Contests
	r.Route("/contests", func(cr chi.Router) {
		cr.Get("/", h.ListContests)
		cr.Post("/", h.CreateContest)
		cr.Get("/{id}", h.GetContest)
		cr.Patch("/{id}", h.UpdateContest)
		cr.Delete("/{id}", h.DeleteContest)
		cr.Post("/{id}/problems", h.AddContestProblem)
		cr.Delete("/{id}/problems/{problemId}", h.RemoveContestProblem)
		cr.Patch("/{id}/problem-order", h.ReorderContestProblems)
	})

	// Users
	r.Route("/users", func(ur chi.Router) {
		ur.Get("/", h.ListUsers)
		ur.Get("/{id}", h.GetUser)
		ur.Patch("/{id}/role", h.UpdateUserRole)
		ur.Patch("/{id}/status", h.UpdateUserStatus)
	})

	return r
}

// Stats
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var totalProblems, totalSets, totalUsers int
	_ = h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM problems").Scan(&totalProblems)
	_ = h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM problem_sets").Scan(&totalSets)
	_ = h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)

	contests, _ := h.contestSvc.List(ctx, "", true)
	totalContests := len(contests)
	activeContests := 0
	upcomingContests := 0
	for _, c := range contests {
		switch c.State {
		case contest.Active:
			activeContests++
		case contest.Upcoming:
			upcomingContests++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"totalProblems":    totalProblems,
		"totalProblemSets": totalSets,
		"totalContests":    totalContests,
		"activeContests":   activeContests,
		"upcomingContests": upcomingContests,
		"totalUsers":       totalUsers,
	})
}

// Problems
func (h *Handler) ListProblems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := problem.Filter{
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

	problems, total, err := h.probSvc.List(r.Context(), filter)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list problems"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"problems": problems,
		"total":    total,
	})
}

func (h *Handler) CreateProblem(w http.ResponseWriter, r *http.Request) {
	var req problem.CreateCustomReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	p, err := h.probSvc.CreateCustom(r.Context(), req)
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

type importProblemReq struct {
	URL string `json:"url"`
}

func (h *Handler) ImportProblem(w http.ResponseWriter, r *http.Request) {
	var req importProblemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	p, err := h.probSvc.ImportByUrl(r.Context(), req.URL)
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

func (h *Handler) UpdateProblem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req problem.UpdateProblemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	p, err := h.probSvc.Update(r.Context(), id, req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "problem not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (h *Handler) DeleteProblem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.probSvc.Delete(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "PROBLEM_IN_USE" {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "problem not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Problem Sets
func (h *Handler) ListProblemSets(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	sets, err := h.setSvc.List(r.Context(), "", claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list problem sets"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sets)
}

type createProblemSetReq struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Visibility  problemset.Visibility `json:"visibility"`
}

func (h *Handler) CreateProblemSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	var req createProblemSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	set, err := h.setSvc.Create(r.Context(), claims.UserID, req.Name, req.Description, req.Visibility)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(set)
}

func (h *Handler) GetProblemSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	set, err := h.setSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(set)
}

type updateProblemSetReq struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	Visibility  *problemset.Visibility `json:"visibility"`
}

func (h *Handler) UpdateProblemSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req updateProblemSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	// For admin update, allow updating even if owner differs or treat admin as owner
	set, err := h.setSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	updated, err := h.setSvc.Update(r.Context(), id, set.OwnerID, req.Name, req.Description, req.Visibility)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func (h *Handler) DeleteProblemSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	set, err := h.setSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	err = h.setSvc.Delete(r.Context(), id, set.OwnerID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type addProblemToSetReq struct {
	ProblemID string `json:"problemId"`
	Position  *int   `json:"position"`
}

func (h *Handler) AddProblemToSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req addProblemToSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	set, err := h.setSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	err = h.setSvc.AddProblem(r.Context(), id, set.OwnerID, req.ProblemID, req.Position)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) RemoveProblemFromSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	problemID := chi.URLParam(r, "problemId")

	set, err := h.setSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	err = h.setSvc.RemoveProblem(r.Context(), id, set.OwnerID, problemID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type reorderSetReq struct {
	ProblemIDs []string `json:"problemIds"`
}

func (h *Handler) ReorderProblemSet(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req reorderSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	set, err := h.setSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	err = h.setSvc.Reorder(r.Context(), id, set.OwnerID, req.ProblemIDs)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Contests
func (h *Handler) ListContests(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	contests, err := h.contestSvc.List(r.Context(), claims.UserID, true)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list contests"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contests)
}

type createContestReq struct {
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	ProblemSetID      string              `json:"problemSetId"`
	ProblemIDs        []string            `json:"problemIds"`
	StartAt           string              `json:"startAt"`
	EndAt             string              `json:"endAt"`
	Visibility        string              `json:"visibility"`
	ScoringType       contest.ScoringType `json:"scoringType"`
	PublicationStatus string              `json:"publicationStatus"`
}

func (h *Handler) CreateContest(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	var req createContestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid startAt format"})
		return
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid endAt format"})
		return
	}

	pubStatus := req.PublicationStatus
	if pubStatus == "" {
		pubStatus = contest.PublicationPublished
	}

	c, err := h.contestSvc.Create(r.Context(), contest.CreateContestParams{
		OwnerID:           claims.UserID,
		ProblemSetID:      req.ProblemSetID,
		ProblemIDs:        req.ProblemIDs,
		Name:              req.Name,
		Description:       req.Description,
		StartAt:           startAt,
		EndAt:             endAt,
		Visibility:        req.Visibility,
		ScoringType:       req.ScoringType,
		PublicationStatus: pubStatus,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

func (h *Handler) GetContest(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	c, err := h.contestSvc.GetByID(r.Context(), id, claims.UserID, true)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

type updateContestReq struct {
	Name              *string              `json:"name"`
	Description       *string              `json:"description"`
	StartAt           *string              `json:"startAt"`
	EndAt             *string              `json:"endAt"`
	Visibility        *string              `json:"visibility"`
	ScoringType       *contest.ScoringType `json:"scoringType"`
	PublicationStatus *string              `json:"publicationStatus"`
}

func (h *Handler) UpdateContest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateContestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	params := contest.UpdateContestParams{
		Name:              req.Name,
		Description:       req.Description,
		Visibility:        req.Visibility,
		ScoringType:       req.ScoringType,
		PublicationStatus: req.PublicationStatus,
	}

	if req.StartAt != nil {
		st, err := time.Parse(time.RFC3339, *req.StartAt)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid startAt format"})
			return
		}
		params.StartAt = &st
	}
	if req.EndAt != nil {
		et, err := time.Parse(time.RFC3339, *req.EndAt)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid endAt format"})
			return
		}
		params.EndAt = &et
	}

	c, err := h.contestSvc.Update(r.Context(), id, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(err.Error(), "active or finished") {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "contest not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (h *Handler) DeleteContest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.contestSvc.Delete(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "contest not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type addContestProblemReq struct {
	ProblemID string  `json:"problemId"`
	Position  *int    `json:"position"`
	Label     *string `json:"label"`
	Points    *int    `json:"points"`
}

func (h *Handler) AddContestProblem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req addContestProblemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	err := h.contestSvc.AddProblem(r.Context(), id, req.ProblemID, req.Position, req.Label, req.Points)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(err.Error(), "active or finished") {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "contest not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) RemoveContestProblem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	problemID := chi.URLParam(r, "problemId")

	err := h.contestSvc.RemoveProblem(r.Context(), id, problemID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(err.Error(), "active or finished") {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "contest not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type reorderContestProblemsReq struct {
	ProblemIDs []string `json:"problemIds"`
}

func (h *Handler) ReorderContestProblems(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req reorderContestProblemsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	err := h.contestSvc.ReorderProblems(r.Context(), id, req.ProblemIDs)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(err.Error(), "active or finished") {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "contest not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	users, err := h.authSvc.ListUsers(r.Context(), search)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list users"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.authSvc.GetUserByID(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

type updateRoleReq struct {
	Role string `json:"role"`
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req updateRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	user, err := h.authSvc.UpdateUserRole(r.Context(), id, req.Role)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "LAST_ADMIN" {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "user not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
	_ = claims // reference
}

type updateStatusReq struct {
	IsActive bool `json:"isActive"`
}

func (h *Handler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	user, err := h.authSvc.UpdateUserStatus(r.Context(), id, req.IsActive)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "LAST_ADMIN" {
			w.WriteHeader(http.StatusConflict)
		} else if err.Error() == "user not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

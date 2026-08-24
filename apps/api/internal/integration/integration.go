package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/platform"
	"github.com/go-chi/chi/v5"
)

type PlatformIntegration struct {
	Platform         platform.Type `json:"platform"`
	ExternalUsername string        `json:"externalUsername"`
	ConnectionStatus string        `json:"connectionStatus"`
	UpdatedAt        time.Time     `json:"updatedAt"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, userID string) ([]PlatformIntegration, error) {
	query := `
		SELECT platform, external_username, connection_status, updated_at
		FROM integrations
		WHERE user_id = $1
		ORDER BY platform ASC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var integrations []PlatformIntegration
	for rows.Next() {
		var pi PlatformIntegration
		if err := rows.Scan(&pi.Platform, &pi.ExternalUsername, &pi.ConnectionStatus, &pi.UpdatedAt); err != nil {
			return nil, err
		}
		integrations = append(integrations, pi)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if integrations == nil {
		integrations = []PlatformIntegration{}
	}

	return integrations, nil
}

func (s *Service) Upsert(ctx context.Context, userID string, pType platform.Type, username, status string) error {
	username = strings.TrimSpace(username)
	if status == "" {
		status = "CONNECTED"
	}

	query := `
		INSERT INTO integrations (user_id, platform, external_username, connection_status, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, platform) DO UPDATE SET
			external_username = EXCLUDED.external_username,
			connection_status = EXCLUDED.connection_status,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query, userID, pType, username, status, time.Now().UTC())
	return err
}

func (s *Service) Delete(ctx context.Context, userID string, pType platform.Type) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM integrations WHERE user_id = $1 AND platform = $2`, userID, pType)
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
	r.Use(h.authSvc.AuthMiddleware(true))

	r.Get("/", h.List)
	r.Put("/{platform}", h.Upsert)
	r.Delete("/{platform}", h.Delete)

	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	list, err := h.service.List(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch integrations"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

type upsertReq struct {
	ExternalUsername string `json:"externalUsername"`
	ConnectionStatus string `json:"connectionStatus"`
}

func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	plat := platform.Type(strings.ToUpper(chi.URLParam(r, "platform")))

	var req upsertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	err := h.service.Upsert(r.Context(), claims.UserID, plat, req.ExternalUsername, req.ConnectionStatus)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	plat := platform.Type(strings.ToUpper(chi.URLParam(r, "platform")))

	err := h.service.Delete(r.Context(), claims.UserID, plat)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

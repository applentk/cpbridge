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

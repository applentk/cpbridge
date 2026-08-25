package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/idgen"
	"github.com/cpbridge/api/internal/ratelimit"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserContextKey contextKey = "user"

const (
	RoleAdmin = "ADMIN"
	RoleUser  = "USER"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Claims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewService(db *sql.DB) *Service {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		// NewService is retained for package-level tests and local callers. The
		// server uses NewServiceFromEnv, which fails closed outside development.
		secret = randomSecret()
	}
	return &Service{db: db, jwtSecret: []byte(secret)}
}

// NewServiceFromEnv is the startup constructor. A deployment must explicitly
// opt into development mode to run without a configured JWT_SECRET.
func NewServiceFromEnv(db *sql.DB) (*Service, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("NODE_ENV")))
	}
	isDevelopment := env == "development" || env == "dev" || env == "test"
	if !isDevelopment && secret == "" {
		return nil, errors.New("JWT_SECRET is required unless ENV is explicitly development")
	}
	if secret != "" && len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters long")
	}
	if secret == "" {
		secret = randomSecret()
	}
	return &Service{db: db, jwtSecret: []byte(secret)}, nil
}

func randomSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("failed to generate JWT secret")
	}
	return fmt.Sprintf("%x", buf)
}

func (s *Service) SetJWTSecret(secret string) {
	s.jwtSecret = []byte(secret)
}

func (s *Service) Register(ctx context.Context, email, username, password string) (*User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if email == "" || username == "" || password == "" {
		return nil, "", errors.New("email, username, and password are required")
	}
	if len(password) < 6 {
		return nil, "", errors.New("password must be at least 6 characters")
	}
	if len(username) < 3 || len(username) > 30 {
		return nil, "", errors.New("username must be between 3 and 30 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		ID:           idgen.New(idgen.PrefixUser),
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		Role:         RoleUser,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	query := `
		INSERT INTO users (id, email, username, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = s.db.ExecContext(ctx, query, user.ID, user.Email, user.Username, user.PasswordHash, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, "", errors.New("email or username already in use")
		}
		return nil, "", fmt.Errorf("failed to insert user: %w", err)
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *Service) Login(ctx context.Context, emailOrUsername, password string) (*User, string, error) {
	emailOrUsername = strings.TrimSpace(emailOrUsername)
	if emailOrUsername == "" || password == "" {
		return nil, "", errors.New("credentials required")
	}

	query := `
		SELECT id, email, username, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)
		LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query, emailOrUsername)

	var user User
	err := row.Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", errors.New("invalid email/username or password")
		}
		return nil, "", fmt.Errorf("failed to query user: %w", err)
	}

	if !user.IsActive {
		return nil, "", errors.New("ACCOUNT_DISABLED")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("invalid email/username or password")
	}

	token, err := s.generateToken(&user)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

func (s *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, email, username, role, is_active, created_at, updated_at FROM users WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)
	var user User
	if err := row.Scan(&user.ID, &user.Email, &user.Username, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (s *Service) ListUsers(ctx context.Context, search string) ([]User, error) {
	var query string
	var args []any
	if search != "" {
		query = `
			SELECT id, email, username, role, is_active, created_at, updated_at
			FROM users
			WHERE LOWER(username) LIKE $1 OR LOWER(email) LIKE $1
			ORDER BY created_at DESC
		`
		args = append(args, "%"+strings.ToLower(search)+"%")
	} else {
		query = `
			SELECT id, email, username, role, is_active, created_at, updated_at
			FROM users
			ORDER BY created_at DESC
		`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if users == nil {
		users = []User{}
	}
	return users, nil
}

func (s *Service) UpdateUserRole(ctx context.Context, targetUserID, newRole string) (*User, error) {
	newRole = strings.ToUpper(strings.TrimSpace(newRole))
	if newRole != RoleAdmin && newRole != RoleUser {
		return nil, errors.New("invalid role: must be ADMIN or USER")
	}

	target, err := s.GetUserByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	// Safety check: last active admin demotion
	if target.Role == RoleAdmin && target.IsActive && newRole != RoleAdmin {
		var count int
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = $1 AND is_active = true", RoleAdmin).Scan(&count)
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, errors.New("LAST_ADMIN")
		}
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, "UPDATE users SET role = $1, updated_at = $2 WHERE id = $3", newRole, now, targetUserID)
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(ctx, targetUserID)
}

func (s *Service) UpdateUserStatus(ctx context.Context, targetUserID string, isActive bool) (*User, error) {
	target, err := s.GetUserByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	// Safety check: last active admin disabling
	if target.Role == RoleAdmin && target.IsActive && !isActive {
		var count int
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = $1 AND is_active = true", RoleAdmin).Scan(&count)
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, errors.New("LAST_ADMIN")
		}
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, "UPDATE users SET is_active = $1, updated_at = $2 WHERE id = $3", isActive, now, targetUserID)
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(ctx, targetUserID)
}

func (s *Service) generateToken(user *User) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *Service) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (s *Service) AuthMiddleware(required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			var tokenStr string
			if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
				tokenStr = token
			}

			if tokenStr == "" {
				if required {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			claims, err := s.ParseToken(tokenStr)
			if err != nil {
				if required {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Verify active status from database
			var role string
			var isActive bool
			err = s.db.QueryRowContext(r.Context(), "SELECT role, is_active FROM users WHERE id = $1", claims.UserID).Scan(&role, &isActive)
			if err != nil {
				if required {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if !isActive {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "ACCOUNT_DISABLED"})
				return
			}

			// Update role on claims with latest DB value
			claims.Role = role

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserFromContext(r.Context())
			if claims == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			if claims.Role != role {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(RoleAdmin)
}

func GetUserFromContext(ctx context.Context) *Claims {
	if val, ok := ctx.Value(UserContextKey).(*Claims); ok {
		return val
	}
	return nil
}

type Handler struct {
	service *Service
	limiter *ratelimit.Limiter
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service, limiter: ratelimit.New(10000)}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Group(func(pr chi.Router) {
		pr.Use(h.service.AuthMiddleware(true))
		pr.Get("/me", h.Me)
	})
	return r
}

type registerReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	EmailOrUsername string `json:"emailOrUsername"`
	Password        string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.allowAuthRequest(w, r, "register") {
		return
	}
	var req registerReq
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	user, token, err := h.service.Register(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user":  user,
		"token": token,
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.allowAuthRequest(w, r, "login") {
		return
	}
	var req loginReq
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}
	if !h.allowAuthRequest(w, r, "login:"+strings.ToLower(strings.TrimSpace(req.EmailOrUsername))) {
		return
	}

	user, token, err := h.service.Login(r.Context(), req.EmailOrUsername, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err.Error() == "ACCOUNT_DISABLED" {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user":  user,
		"token": token,
	})
}

func (h *Handler) allowAuthRequest(w http.ResponseWriter, r *http.Request, key string) bool {
	if h.limiter == nil {
		h.limiter = ratelimit.New(10000)
	}
	limit := 60
	if strings.HasPrefix(key, "login:") {
		limit = 10
	}
	ok, retryAfter := h.limiter.Allow(ratelimit.ClientIP(r)+"|"+key, limit, time.Minute, time.Now())
	if ok {
		return true
	}
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many authentication attempts"})
	return false
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r.Context())
	if claims == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	user, err := h.service.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": user,
	})
}

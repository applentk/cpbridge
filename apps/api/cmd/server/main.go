package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cpbridge/api/internal/admin"
	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/contest"
	"github.com/cpbridge/api/internal/db"
	"github.com/cpbridge/api/internal/integration"
	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/platform/atcoder"
	"github.com/cpbridge/api/internal/platform/codeforces"
	"github.com/cpbridge/api/internal/problem"
	"github.com/cpbridge/api/internal/problemset"
	"github.com/cpbridge/api/internal/queue"
	"github.com/cpbridge/api/internal/ratelimit"
	"github.com/cpbridge/api/internal/submission"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
)

func main() {
	log.Println("Starting cpbridge API Server...")

	// Database Connection
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.EnsureSchema(database); err != nil {
		log.Printf("Schema verification notice: %v", err)
	}

	// Redis & Asynq Setup
	redisOpt, err := redisOptions()
	if err != nil {
		log.Fatalf("Failed to configure Redis: %v", err)
	}

	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// Platform Adapters (Codeforces, AtCoder)
	platRegistry := platform.NewRegistry()
	platRegistry.Register(codeforces.New())
	platRegistry.Register(atcoder.New())

	// Domain Services
	authSvc, err := auth.NewServiceFromEnv(database)
	if err != nil {
		log.Fatalf("Failed to configure authentication: %v", err)
	}

	probSvc := problem.NewService(database, platRegistry)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)
	subSvc := submission.NewService(database, contestSvc, probSvc, platRegistry)
	subSvc.SetAsynqClient(asynqClient)
	intSvc := integration.NewService(database)

	// Asynq Worker Server
	subWorker := submission.NewWorker(database, probSvc, platRegistry)
	asynqServer := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				queue.QueueSubmissions: 6,
				"default":              3,
			},
			RetryDelayFunc: queue.CustomRetryDelay,
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				if retried >= maxRetry && task.Type() == queue.TypePollSubmissionVerdict {
					log.Printf("[Worker:Exhausted] Task %s exhausted all %d retries: %v", task.Type(), maxRetry, err)
					var p queue.PollVerdictPayload
					if unmarshalErr := json.Unmarshal(task.Payload(), &p); unmarshalErr == nil && p.SubmissionID != "" {
						meta := map[string]any{
							"error": "Judging polling timed out: exceeded maximum retries on platform",
						}
						metaJSON, _ := json.Marshal(meta)
						_, updateErr := database.ExecContext(context.Background(), `
							UPDATE submissions
							SET status = 'FAILED', judged_at = NOW(), metadata = $1
							WHERE id = $2 AND status IN ('PENDING', 'DISPATCHING', 'JUDGING')
						`, metaJSON, p.SubmissionID)
						if updateErr != nil {
							log.Printf("[Worker:ExhaustedError] Failed to update exhausted submission %s: %v", p.SubmissionID, updateErr)
						}
					}
				}
			}),
		},
	)

	asynqMux := asynq.NewServeMux()
	asynqMux.HandleFunc(queue.TypePollSubmissionVerdict, subWorker.ProcessPollVerdict)

	go func() {
		log.Println("Starting background Asynq worker for submission processing...")
		if err := asynqServer.Run(asynqMux); err != nil && !errors.Is(err, asynq.ErrServerClosed) {
			log.Printf("Asynq worker server notice: %v", err)
		}
	}()

	// Domain Handlers
	authHandler := auth.NewHandler(authSvc)
	probHandler := problem.NewHandler(probSvc, authSvc)
	contestHandler := contest.NewHandler(contestSvc, authSvc)
	subHandler := submission.NewHandler(subSvc, authSvc)
	intHandler := integration.NewHandler(intSvc, authSvc)
	adminHandler := admin.NewHandler(database, authSvc, probSvc, setSvc, contestSvc)

	// Main Router
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(requestProtection)

	// CORS Setup
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// API Routes
	r.Route("/api", func(api chi.Router) {
		api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "cpbridge-api"})
		})

		api.Mount("/auth", authHandler.Routes())
		api.Mount("/problems", probHandler.Routes())
		api.Mount("/contests", contestHandler.Routes())
		api.Mount("/submissions", subHandler.Routes())
		api.Mount("/integrations", intHandler.Routes())

		// Admin API Group (Requires Auth + Admin Role)
		api.Group(func(adminRouter chi.Router) {
			adminRouter.Use(authSvc.AuthMiddleware(true))
			adminRouter.Use(auth.RequireAdmin())
			adminRouter.Mount("/admin", adminHandler.Routes())
		})

		// Standings route
		api.Group(func(pr chi.Router) {
			pr.Use(authSvc.AuthMiddleware(false))
			pr.Get("/contests/{id}/standings", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				var uid string
				var isAdmin bool
				if claims := auth.GetUserFromContext(r.Context()); claims != nil {
					uid = claims.UserID
					isAdmin = claims.Role == auth.RoleAdmin
				}
				standings, err := subSvc.CalculateStandings(r.Context(), id, uid, isAdmin)
				if err != nil {
					http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(standings)
			})
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Server shutdown listener
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server stopped unexpectedly: %v", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down API server and worker gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	asynqServer.Shutdown()
	log.Println("cpbridge server and Asynq worker stopped cleanly.")
}

func requestProtection(next http.Handler) http.Handler {
	var limiter = ratelimit.New(20000)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, retryAfter := limiter.Allow(ratelimit.ClientIP(r), 240, time.Minute, time.Now()); !ok {
			seconds := int(retryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		// Keep the global ceiling below the largest source payload accepted by
		// the API. Individual sensitive handlers apply smaller limits as well.
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		next.ServeHTTP(w, r)
	})
}

func allowedOrigins() []string {
	value := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if value == "" {
		return []string{"http://localhost:3000"}
	}

	origins := make([]string, 0)
	for _, origin := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:3000"}
	}
	return origins
}

func redisOptions() (asynq.RedisClientOpt, error) {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}
		return asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: os.Getenv("REDIS_PASSWORD"),
		}, nil
	}

	parsed, err := url.Parse(redisURL)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return asynq.RedisClientOpt{}, fmt.Errorf("REDIS_URL must use redis:// or rediss://")
	}
	if parsed.Host == "" {
		return asynq.RedisClientOpt{}, fmt.Errorf("REDIS_URL must include a host")
	}

	opt := asynq.RedisClientOpt{Addr: parsed.Host}
	if parsed.User != nil {
		opt.Username = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			opt.Password = password
		}
	}
	if parsed.Scheme == "rediss" {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opt, nil
}

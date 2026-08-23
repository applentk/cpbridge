package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cp-hub/api/internal/admin"
	"github.com/cp-hub/api/internal/auth"
	"github.com/cp-hub/api/internal/contest"
	"github.com/cp-hub/api/internal/db"
	"github.com/cp-hub/api/internal/integration"
	"github.com/cp-hub/api/internal/platform"
	"github.com/cp-hub/api/internal/platform/atcoder"
	"github.com/cp-hub/api/internal/platform/codeforces"
	"github.com/cp-hub/api/internal/problem"
	"github.com/cp-hub/api/internal/problemset"
	"github.com/cp-hub/api/internal/queue"
	"github.com/cp-hub/api/internal/submission"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
)

func main() {
	log.Println("Starting Competitive Programming Hub API Server...")

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
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisOpt := asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
	}

	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// Platform Adapters (Codeforces, AtCoder)
	platRegistry := platform.NewRegistry()
	platRegistry.Register(codeforces.New())
	platRegistry.Register(atcoder.New())

	// Domain Services
	authSvc := auth.NewService(database)
	if err := authSvc.BootstrapInitialAdmin(context.Background()); err != nil {
		log.Printf("Initial admin bootstrap notice: %v", err)
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
						meta := map[string]interface{}{
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
	setHandler := problemset.NewHandler(setSvc, authSvc)
	contestHandler := contest.NewHandler(contestSvc, authSvc)
	subHandler := submission.NewHandler(subSvc, authSvc)
	intHandler := integration.NewHandler(intSvc, authSvc)
	adminHandler := admin.NewHandler(database, authSvc, probSvc, setSvc, contestSvc)

	// Main Router
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS Setup
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API Routes
	r.Route("/api", func(api chi.Router) {
		api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "cp-hub-api"})
		})

		api.Mount("/auth", authHandler.Routes())
		api.Mount("/problems", probHandler.Routes())
		api.Mount("/problem-sets", setHandler.Routes())
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
		Addr:    ":" + port,
		Handler: r,
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
	log.Println("CP Hub server and Asynq worker stopped cleanly.")
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cp-hub/api/internal/auth"
	"github.com/cp-hub/api/internal/contest"
	"github.com/cp-hub/api/internal/db"
	"github.com/cp-hub/api/internal/integration"
	"github.com/cp-hub/api/internal/platform"
	"github.com/cp-hub/api/internal/platform/atcoder"
	"github.com/cp-hub/api/internal/platform/codeforces"
	"github.com/cp-hub/api/internal/platform/leetcode"
	"github.com/cp-hub/api/internal/problem"
	"github.com/cp-hub/api/internal/problemset"
	"github.com/cp-hub/api/internal/submission"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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

	// Platform Adapters
	platRegistry := platform.NewRegistry()
	platRegistry.Register(codeforces.New())
	platRegistry.Register(atcoder.New())
	platRegistry.Register(leetcode.New())

	// Domain Services
	authSvc := auth.NewService(database)
	probSvc := problem.NewService(database, platRegistry)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)
	subSvc := submission.NewService(database, contestSvc, probSvc)
	intSvc := integration.NewService(database)

	// Domain Handlers
	authHandler := auth.NewHandler(authSvc)
	probHandler := problem.NewHandler(probSvc)
	setHandler := problemset.NewHandler(setSvc, authSvc)
	contestHandler := contest.NewHandler(contestSvc, authSvc)
	subHandler := submission.NewHandler(subSvc, authSvc)
	intHandler := integration.NewHandler(intSvc, authSvc)

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

		// Standings route
		api.Group(func(pr chi.Router) {
			pr.Use(authSvc.AuthMiddleware(false))
			pr.Get("/contests/{id}/standings", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				var uid string
				if claims := auth.GetUserFromContext(r.Context()); claims != nil {
					uid = claims.UserID
				}
				standings, err := subSvc.CalculateStandings(r.Context(), id, uid)
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

	log.Printf("Server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

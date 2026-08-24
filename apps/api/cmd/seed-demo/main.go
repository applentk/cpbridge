// Command seed-demo creates a small, repeatable local dataset for manual testing.
// It must not be run against a production database.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/contest"
	"github.com/cpbridge/api/internal/db"
	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/platform/atcoder"
	"github.com/cpbridge/api/internal/platform/codeforces"
	"github.com/cpbridge/api/internal/problem"
	"github.com/cpbridge/api/internal/problemset"
)

const (
	demoEmail    = "demo@cphub.local"
	demoUsername = "demo"
	demoPassword = "demo1234"
	setName      = "cpbridge Demo Problem Set"
	contestName  = "cpbridge Demo Contest"
)

var demoProblemURLs = []string{
	"https://codeforces.com/problemset/problem/4/A",
	"https://codeforces.com/problemset/problem/71/A",
	"https://codeforces.com/problemset/problem/158/A",
	"https://codeforces.com/problemset/problem/231/A",
	"https://codeforces.com/problemset/problem/282/A",
	"https://atcoder.jp/contests/abc086/tasks/abc086_a",
	"https://atcoder.jp/contests/abc081/tasks/abc081_a",
	"https://atcoder.jp/contests/abc081/tasks/abc081_b",
	"https://atcoder.jp/contests/abc083/tasks/abc083_a",
	"https://atcoder.jp/contests/abc087/tasks/abc087_a",
}

func main() {
	if isProduction() {
		log.Fatal("seed-demo refuses to run when ENV or NODE_ENV is production")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	database, err := db.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := db.EnsureSchema(database); err != nil {
		log.Fatal(err)
	}

	registry := platform.NewRegistry()
	registry.Register(codeforces.New())
	registry.Register(atcoder.New())

	authSvc := auth.NewService(database)
	owner, err := ensureDemoOwner(ctx, database, authSvc)
	if err != nil {
		log.Fatal(err)
	}

	problemSvc := problem.NewService(database, registry)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)

	problemIDs := make([]string, 0, len(demoProblemURLs))
	for _, rawURL := range demoProblemURLs {
		p, err := problemSvc.ImportByUrl(ctx, rawURL)
		if err != nil {
			log.Fatalf("import %s: %v", rawURL, err)
		}
		problemIDs = append(problemIDs, p.ID)
		log.Printf("problem: %-10s %-12s %s", p.Platform, p.ExternalID, p.Title)
	}

	setID, err := ensureProblemSet(ctx, database, setSvc, owner.ID)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `DELETE FROM problem_set_items WHERE problem_set_id = $1`, setID); err != nil {
		log.Fatalf("reset demo problem set: %v", err)
	}
	for i, problemID := range problemIDs {
		if err := setSvc.AddProblem(ctx, setID, owner.ID, problemID, &i); err != nil {
			log.Fatalf("add problem %s to demo set: %v", problemID, err)
		}
	}

	contestID, contestEndAt, created, err := ensureContest(ctx, database, contestSvc, owner.ID, setID)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("demo data is ready")
	log.Printf("login:    %s / %s", owner.Email, configuredPassword())
	log.Printf("set id:   %s", setID)
	log.Printf("contest:  %s%s", contestID, createdMessage(created))
	log.Printf("contest:  ends at %s (UTC)", contestEndAt.Format(time.RFC3339))
}

func isProduction() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("NODE_ENV")))
	}
	return env == "production" || env == "prod"
}

func configuredEmail() string {
	if value := strings.TrimSpace(os.Getenv("SEED_DEMO_EMAIL")); value != "" {
		return strings.ToLower(value)
	}
	return demoEmail
}

func configuredUsername() string {
	if value := strings.TrimSpace(os.Getenv("SEED_DEMO_USERNAME")); value != "" {
		return value
	}
	return demoUsername
}

func configuredPassword() string {
	if value := os.Getenv("SEED_DEMO_PASSWORD"); value != "" {
		return value
	}
	return demoPassword
}

func ensureDemoOwner(ctx context.Context, database *sql.DB, authSvc *auth.Service) (*auth.User, error) {
	var owner auth.User
	err := database.QueryRowContext(ctx, `
		SELECT id, email, username, role, is_active, created_at, updated_at
		FROM users WHERE LOWER(email) = LOWER($1)
	`, configuredEmail()).Scan(
		&owner.ID, &owner.Email, &owner.Username, &owner.Role, &owner.IsActive, &owner.CreatedAt, &owner.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		created, _, registerErr := authSvc.Register(ctx, configuredEmail(), configuredUsername(), configuredPassword())
		if registerErr != nil {
			return nil, fmt.Errorf("create demo user: %w", registerErr)
		}
		owner = *created
	} else if err != nil {
		return nil, fmt.Errorf("find demo user: %w", err)
	}

	if !owner.IsActive {
		return nil, errors.New("demo user exists but is disabled")
	}
	if _, err := database.ExecContext(ctx, `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`, auth.RoleAdmin, owner.ID); err != nil {
		return nil, fmt.Errorf("promote demo user: %w", err)
	}
	owner.Role = auth.RoleAdmin
	return &owner, nil
}

func ensureProblemSet(ctx context.Context, database *sql.DB, setSvc *problemset.Service, ownerID string) (string, error) {
	var setID string
	err := database.QueryRowContext(ctx, `SELECT id FROM problem_sets WHERE owner_id = $1 AND name = $2`, ownerID, setName).Scan(&setID)
	if errors.Is(err, sql.ErrNoRows) {
		set, createErr := setSvc.Create(ctx, ownerID, setName, "Ten Codeforces and AtCoder problems for local testing.", problemset.Public)
		if createErr != nil {
			return "", fmt.Errorf("create demo problem set: %w", createErr)
		}
		return set.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("find demo problem set: %w", err)
	}
	return setID, nil
}

func ensureContest(ctx context.Context, database *sql.DB, contestSvc *contest.Service, ownerID, setID string) (string, time.Time, bool, error) {
	var contestID string
	var endAt time.Time
	err := database.QueryRowContext(ctx, `SELECT id, end_at FROM contests WHERE owner_id = $1 AND name = $2`, ownerID, contestName).Scan(&contestID, &endAt)
	if err == nil {
		return contestID, endAt, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, false, fmt.Errorf("find demo contest: %w", err)
	}

	now := time.Now().UTC()
	created, err := contestSvc.CreateFromProblemSet(
		ctx,
		ownerID,
		setID,
		contestName,
		"Active ICPC contest seeded with five Codeforces and five AtCoder problems.",
		now.Add(-15*time.Minute),
		now.Add(105*time.Minute),
		contest.VisibilityPublic,
		contest.ICPC,
	)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("create demo contest: %w", err)
	}
	return created.ID, created.EndAt, true, nil
}

func createdMessage(created bool) string {
	if created {
		return " (created)"
	}
	return " (already existed)"
}

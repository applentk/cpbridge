package contest_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/contest"
	"github.com/cpbridge/api/internal/db"
	"github.com/cpbridge/api/internal/problemset"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetOutput(io.Discard)
}

func TestContestStateCalculation(t *testing.T) {
	startAt := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	// 1. Before start
	beforeTime := time.Date(2026, 8, 23, 9, 59, 59, 0, time.UTC)
	assert.Equal(t, contest.Upcoming, contest.CalculateState(beforeTime, startAt, endAt))

	// 2. Exactly at start
	atStart := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, contest.Active, contest.CalculateState(atStart, startAt, endAt))

	// 3. During contest
	duringTime := time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC)
	assert.Equal(t, contest.Active, contest.CalculateState(duringTime, startAt, endAt))

	// 4. Exactly at end
	atEnd := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, contest.Finished, contest.CalculateState(atEnd, startAt, endAt))

	// 5. After end
	afterTime := time.Date(2026, 8, 23, 12, 30, 0, 0, time.UTC)
	assert.Equal(t, contest.Finished, contest.CalculateState(afterTime, startAt, endAt))
}

func TestGenerateLabel(t *testing.T) {
	assert.Equal(t, "A", contest.GenerateLabel(0))
	assert.Equal(t, "B", contest.GenerateLabel(1))
	assert.Equal(t, "Z", contest.GenerateLabel(25))
	assert.Equal(t, "P27", contest.GenerateLabel(26))
}

func TestContestListVisibility(t *testing.T) {
	database, err := db.Connect()
	if err != nil {
		t.Skipf("Skipping db integration test: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.Ping(); err != nil {
		t.Skipf("Skipping db integration test: %v", err)
	}

	err = db.EnsureSchema(database)
	if err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	ctx := context.Background()
	suffix := time.Now().UnixNano()
	authSvc := auth.NewService(database)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)

	user, _, err := authSvc.Register(ctx, fmt.Sprintf("u_%d@test.com", suffix), fmt.Sprintf("user_%d", suffix), "password123")
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = database.ExecContext(ctx, "DELETE FROM contest_participants WHERE user_id = $1", user.ID)
		_, _ = database.ExecContext(ctx, "DELETE FROM contests WHERE owner_id = $1", user.ID)
		_, _ = database.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	now := time.Now().UTC()

	// 1. Finished contest
	cFinished, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           user.ID,
		Name:              fmt.Sprintf("Finished Contest %d", suffix),
		StartAt:           now.Add(-3 * time.Hour),
		EndAt:             now.Add(-1 * time.Hour),
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	if err != nil {
		t.Fatalf("Failed to create finished contest: %v", err)
	}

	// 2. Active contest
	cActive, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           user.ID,
		Name:              fmt.Sprintf("Active Contest %d", suffix),
		StartAt:           now.Add(-1 * time.Hour),
		EndAt:             now.Add(1 * time.Hour),
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	if err != nil {
		t.Fatalf("Failed to create active contest: %v", err)
	}

	// 3. Upcoming contest
	cUpcoming, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           user.ID,
		Name:              fmt.Sprintf("Upcoming Contest %d", suffix),
		StartAt:           now.Add(1 * time.Hour),
		EndAt:             now.Add(3 * time.Hour),
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	if err != nil {
		t.Fatalf("Failed to create upcoming contest: %v", err)
	}

	// For regular user (non-admin), List should return active, upcoming, and finished contests
	userList, err := contestSvc.List(ctx, user.ID, false)
	if err != nil {
		t.Fatalf("Failed to list contests: %v", err)
	}

	foundFinished := false
	foundActive := false
	foundUpcoming := false
	for _, c := range userList {
		if c.ID == cFinished.ID {
			foundFinished = true
		}
		if c.ID == cActive.ID {
			foundActive = true
		}
		if c.ID == cUpcoming.ID {
			foundUpcoming = true
		}
	}
	assert.True(t, foundFinished, "Regular user should see finished contests in list")
	assert.True(t, foundActive, "Regular user must see active contests")
	assert.True(t, foundUpcoming, "Regular user must see upcoming contests")

	// For admin, List should return ALL contests including finished
	adminList, err := contestSvc.List(ctx, user.ID, true)
	if err != nil {
		t.Fatalf("Failed to list contests as admin: %v", err)
	}
	foundFinishedAdmin := false
	for _, c := range adminList {
		if c.ID == cFinished.ID {
			foundFinishedAdmin = true
		}
	}
	assert.True(t, foundFinishedAdmin, "Admin user must see finished contests in list")
}
